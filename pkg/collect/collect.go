package collect

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rinat1313/zakupki-parser/internal/archiveutil"
	"github.com/rinat1313/zakupki-parser/internal/detect"
	"github.com/rinat1313/zakupki-parser/internal/extract"
	"github.com/rinat1313/zakupki-parser/internal/filemeta"
	"github.com/rinat1313/zakupki-parser/internal/parser/fz223"
	"github.com/rinat1313/zakupki-parser/internal/parser/fz44"
	"github.com/rinat1313/zakupki-parser/internal/parser/pricereq"
	"github.com/rinat1313/zakupki-parser/internal/regnum"
	"github.com/rinat1313/zakupki-parser/models"
	"github.com/rinat1313/zakupki-parser/pkg/eis"
)

type DocResult struct {
	UID           string `json:"uid"`
	Filename      string `json:"filename"`
	SourceURL     string `json:"source_url"`
	GroupTitle    string `json:"group_title,omitempty"`
	Edition       string `json:"edition,omitempty"`
	ProcessStatus string `json:"process_status"` // processed | unprocessed
	TextContent   string `json:"text_content,omitempty"`
	ProcessError  string `json:"process_error,omitempty"`
	ContentHash   string `json:"content_hash,omitempty"`
}

type Result struct {
	RegNumber      string                     `json:"reg_number"`
	Law            string                     `json:"law"`
	ObjectName     string                     `json:"object_name"`
	Status         string                     `json:"status"`
	NMCK           *float64                   `json:"nmck,omitempty"`
	Currency       string                     `json:"currency,omitempty"`
	PublishedAt    *time.Time                 `json:"published_at,omitempty"`
	UpdatedOnSite  *time.Time                 `json:"updated_on_site,omitempty"`
	ApplicationEnd *time.Time                 `json:"application_end,omitempty"`
	Customer       models.Organization44      `json:"customer"`
	Customer223    *models.Organization223    `json:"customer_223,omitempty"`
	Payload        json.RawMessage            `json:"payload,omitempty"`
	Documents      []DocResult                `json:"documents"`
}

type Collector struct {
	Client *eis.Client
	Delay  time.Duration
}

func New(insecure bool) *Collector {
	return &Collector{Client: eis.NewClient(insecure), Delay: 800 * time.Millisecond}
}

func IsZakupki(site string) bool {
	u, err := url.Parse(strings.TrimSpace(site))
	if err != nil {
		return strings.Contains(strings.ToLower(site), "zakupki.gov.ru")
	}
	host := strings.ToLower(u.Hostname())
	return host == "zakupki.gov.ru" || strings.HasSuffix(host, ".zakupki.gov.ru") || host == ""
}

func (c *Collector) Collect(_ context.Context, regNumber, sourceSite string) (*Result, error) {
	if !IsZakupki(sourceSite) {
		return nil, fmt.Errorf("unsupported_source")
	}
	var lastErr error
	for _, id := range regnum.Candidates(regNumber) {
		res, err := c.collectOne(id)
		if err == nil {
			return res, nil
		}
		lastErr = err
		time.Sleep(c.Delay)
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("not found")
	}
	return nil, lastErr
}

func (c *Collector) collectOne(regNumber string) (*Result, error) {
	det, err := detect.DetectAttempts(c.Client, regNumber, 3)
	if err != nil {
		return nil, err
	}
	switch det.Law {
	case models.Law223:
		return c.collect223(regNumber, det.NoticeGUID)
	case models.LawPricereq:
		return c.collectPricereq(regNumber)
	default:
		nt := det.NoticeType
		if nt == "" {
			nt = "ea20"
		}
		return c.collect44(regNumber, nt)
	}
}

func (c *Collector) collect44(regNumber, noticeType string) (*Result, error) {
	noticeURL := eis.NoticeCommonInfoURL(c.Client.BaseURL, noticeType, regNumber)
	html, err := c.Client.Get(noticeURL)
	if err != nil {
		return nil, err
	}
	n, err := fz44.ParseNoticeHTML(html, noticeURL)
	if err != nil {
		return nil, err
	}
	cust := models.Organization44{
		OrganizationCode: n.OrganizationCode,
		FullName:         n.CustomerName,
		Email:            n.ContactEmail,
		Phone:            n.ContactPhone,
		ContactPerson:    n.ContactPerson,
		INN:              n.CustomerINN,
		KPP:              n.CustomerKPP,
	}
	if n.OrganizationCode != "" {
		time.Sleep(c.Delay)
		orgHTML, err := c.Client.Get(eis.OrganizationInfoURL(c.Client.BaseURL, n.OrganizationCode))
		if err == nil {
			inn, kpp, name, err := fz44.ParseOrganizationHTML(orgHTML)
			if err == nil {
				cust.INN, cust.KPP = inn, kpp
				if name != "" {
					cust.FullName = name
				}
			}
		}
	}
	var docs []DocResult
	time.Sleep(c.Delay)
	docsHTML, err := c.Client.Get(eis.NoticeDocumentsURL(c.Client.BaseURL, noticeType, regNumber))
	if err == nil {
		parsed, err := fz44.ParseDocumentsHTML(docsHTML)
		if err == nil {
			docs = c.fetchDocs(parsed)
		}
	}
	payload, _ := json.Marshal(map[string]any{"tender": n, "customer": cust})
	res := &Result{
		RegNumber:  regNumber,
		Law:        string(models.Law44),
		ObjectName: n.ObjectName,
		Status:     n.Status,
		Currency:   n.NMCK.Currency,
		Customer:   cust,
		Payload:    payload,
		Documents:  docs,
	}
	if n.NMCK.Amount > 0 {
		v := n.NMCK.Amount
		res.NMCK = &v
	}
	if !n.Published.At.IsZero() {
		t := n.Published.At
		res.PublishedAt = &t
	}
	if !n.Updated.At.IsZero() {
		t := n.Updated.At
		res.UpdatedOnSite = &t
	}
	if !n.ApplicationEnd.At.IsZero() {
		t := n.ApplicationEnd.At
		res.ApplicationEnd = &t
	}
	return res, nil
}

func (c *Collector) collect223(regNumber, guid string) (*Result, error) {
	noticeURL := eis.Notice223CommonInfoURL(c.Client.BaseURL, regNumber, guid)
	html, err := c.Client.Get(noticeURL)
	if err != nil {
		return nil, err
	}
	if guid == "" {
		guid = fz223.ExtractNoticeGUID(html)
	}
	n, err := fz223.ParseNoticeHTML(html, noticeURL)
	if err != nil {
		return nil, err
	}
	if n.NoticeGUID == "" {
		n.NoticeGUID = guid
	}
	var cust223 *models.Organization223
	cust := models.Organization44{
		FullName: n.CustomerName, INN: n.CustomerINN, KPP: n.CustomerKPP, OGRN: n.CustomerOGRN,
		Email: n.ContactEmail, ContactPerson: n.ContactPerson,
	}
	if n.CustomerAgencyID != "" || n.CustomerINN != "" {
		time.Sleep(c.Delay)
		orgHTML, err := c.Client.Get(eis.Organization223InfoURL(c.Client.BaseURL, n.CustomerAgencyID, n.CustomerINN, n.CustomerKPP, n.CustomerOGRN))
		if err == nil {
			if org, err := fz223.ParseOrganizationHTML(orgHTML); err == nil {
				cust223 = org
				cust.INN, cust.KPP, cust.OGRN = org.INN, org.KPP, org.OGRN
				if org.FullName != "" {
					cust.FullName = org.FullName
				}
			}
		}
	}
	var docs []DocResult
	time.Sleep(c.Delay)
	docsHTML, err := c.Client.Get(eis.Notice223DocumentsURL(c.Client.BaseURL, regNumber, n.NoticeGUID))
	if err == nil {
		parsed, err := fz223.ParseDocumentsHTML(docsHTML)
		if err == nil {
			docs = c.fetchDocs(parsed)
		}
	}
	payload, _ := json.Marshal(map[string]any{"tender": n, "customer": cust223})
	res := &Result{
		RegNumber:   regNumber,
		Law:         string(models.Law223),
		ObjectName:  n.ObjectName,
		Status:      n.Status,
		Currency:    n.InitialPrice.Currency,
		Customer:    cust,
		Customer223: cust223,
		Payload:     payload,
		Documents:   docs,
	}
	if n.InitialPrice.Amount > 0 {
		v := n.InitialPrice.Amount
		res.NMCK = &v
	}
	if !n.Published.At.IsZero() {
		t := n.Published.At
		res.PublishedAt = &t
	}
	if !n.Updated.At.IsZero() {
		t := n.Updated.At
		res.UpdatedOnSite = &t
	}
	return res, nil
}

func (c *Collector) collectPricereq(regNumber string) (*Result, error) {
	noticeURL := eis.PriceReqCommonInfoURL(c.Client.BaseURL, regNumber)
	html, err := c.Client.Get(noticeURL)
	if err != nil {
		return nil, err
	}
	n, err := pricereq.ParseCommonInfoHTML(html, noticeURL)
	if err != nil {
		searchURL := eis.SearchPricereqURL(c.Client.BaseURL, regNumber)
		time.Sleep(c.Delay)
		searchHTML, serr := c.Client.Get(searchURL)
		if serr != nil {
			return nil, err
		}
		n, err = pricereq.ParseCommonInfoHTML(searchHTML, searchURL)
		if err != nil {
			return nil, err
		}
	}
	cust := models.Organization44{
		OrganizationCode: n.OrganizationCode, FullName: n.OrgName,
		INN: n.CustomerINN, KPP: n.CustomerKPP, Email: n.ContactEmail, Phone: n.ContactPhone, ContactPerson: n.ContactPerson,
	}
	if n.OrganizationCode != "" {
		time.Sleep(c.Delay)
		orgHTML, err := c.Client.Get(eis.OrganizationInfoURL(c.Client.BaseURL, n.OrganizationCode))
		if err == nil {
			inn, kpp, name, err := fz44.ParseOrganizationHTML(orgHTML)
			if err == nil {
				cust.INN, cust.KPP = inn, kpp
				if name != "" {
					cust.FullName = name
				}
			}
		}
	}
	var docs []DocResult
	parsed := c.loadPricereqDocs(regNumber, n.PriceRequestInfoID)
	if len(parsed) > 0 {
		docs = c.fetchDocs(parsed)
	}
	payload, _ := json.Marshal(map[string]any{"price_request": n, "customer": cust})
	res := &Result{
		RegNumber:  regNumber,
		Law:        string(models.LawPricereq),
		ObjectName: n.ObjectName,
		Status:     n.Status,
		Customer:   cust,
		Payload:    payload,
		Documents:  docs,
		Currency:   "RUB",
	}
	if !n.Published.At.IsZero() {
		t := n.Published.At
		res.PublishedAt = &t
	}
	if !n.Updated.At.IsZero() {
		t := n.Updated.At
		res.UpdatedOnSite = &t
	}
	if !n.PriceInfoTo.At.IsZero() {
		t := n.PriceInfoTo.At
		res.ApplicationEnd = &t
	}
	return res, nil
}

func (c *Collector) loadPricereqDocs(regNumber, infoID string) []models.DocumentFile {
	urls := []string{
		eis.PriceReqDocumentsURL(c.Client.BaseURL, regNumber),
	}
	if infoID != "" {
		urls = append(urls,
			eis.PriceReqDocumentsURLByInfoID(c.Client.BaseURL, infoID),
			eis.PriceReqDocumentsURLByRequestID(c.Client.BaseURL, infoID),
		)
	}
	seen := map[string]struct{}{}
	var out []models.DocumentFile
	for _, u := range urls {
		time.Sleep(c.Delay)
		html, err := c.Client.Get(u)
		if err != nil {
			continue
		}
		parsed, err := pricereq.ParseDocumentsHTML(html)
		if err != nil || len(parsed) == 0 {
			parsed, _ = fz44.ParseDocumentsHTML(html)
		}
		for _, d := range parsed {
			key := d.UID
			if key == "" {
				key = d.URL
			}
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, d)
		}
		if len(out) > 0 {
			break
		}
	}
	return out
}

func (c *Collector) fetchDocs(files []models.DocumentFile) []DocResult {
	var out []DocResult
	for _, f := range files {
		time.Sleep(c.Delay)
		base := DocResult{
			UID: f.UID, Filename: f.Filename, SourceURL: f.URL,
			GroupTitle: f.GroupTitle, Edition: f.Edition, ProcessStatus: "unprocessed",
		}
		meta, err := c.Client.DownloadMeta(f.URL)
		if err != nil {
			base.ProcessError = err.Error()
			out = append(out, base)
			continue
		}
		hint := filemeta.ExtFromContentDisposition(meta.Header)
		sniffed := filemeta.SniffExt(meta.Body)
		name := filemeta.EnsureExt(f.Filename, hint, sniffed)
		base.Filename = name
		sum := sha256.Sum256(meta.Body)
		base.ContentHash = hex.EncodeToString(sum[:])

		tmp, err := os.MkdirTemp("", "zakupki-doc-*")
		if err != nil {
			base.ProcessError = err.Error()
			out = append(out, base)
			continue
		}
		path := filepath.Join(tmp, filepath.Base(name))
		if err := os.WriteFile(path, meta.Body, 0o644); err != nil {
			_ = os.RemoveAll(tmp)
			base.ProcessError = err.Error()
			out = append(out, base)
			continue
		}

		if archiveutil.IsArchive(path) {
			extractDir := filepath.Join(tmp, "unpacked")
			leaves, err := archiveutil.ExtractRecursive(path, extractDir, 6)
			_ = os.Remove(path)
			if err != nil {
				base.ProcessError = "archive: " + err.Error()
				out = append(out, base)
				_ = os.RemoveAll(tmp)
				continue
			}
			if len(leaves) == 0 {
				base.ProcessError = "archive empty"
				out = append(out, base)
				_ = os.RemoveAll(tmp)
				continue
			}
			for _, leaf := range leaves {
				rel, _ := filepath.Rel(extractDir, leaf)
				if rel == "" || strings.HasPrefix(rel, "..") {
					rel = filepath.Base(leaf)
				}
				child := DocResult{
					UID:       strings.TrimSpace(f.UID + "/" + rel),
					Filename:  filepath.Base(leaf),
					SourceURL: f.URL + "#" + rel,
					GroupTitle: f.GroupTitle,
					Edition:   f.Edition,
					ProcessStatus: "unprocessed",
				}
				if b, err := os.ReadFile(leaf); err == nil {
					s := sha256.Sum256(b)
					child.ContentHash = hex.EncodeToString(s[:])
				}
				child = c.extractFile(leaf, child)
				out = append(out, child)
			}
			_ = os.RemoveAll(tmp)
			continue
		}

		base = c.extractFile(path, base)
		_ = os.RemoveAll(tmp)
		out = append(out, base)
	}
	return out
}

func (c *Collector) extractFile(path string, dr DocResult) DocResult {
	txtPath := path + ".out.txt"
	r := extract.ToTextOut(path, txtPath)
	if r.Error != "" {
		dr.ProcessError = r.Error
		dr.ProcessStatus = "unprocessed"
		return dr
	}
	b, err := os.ReadFile(txtPath)
	if err != nil {
		dr.ProcessError = err.Error()
		dr.ProcessStatus = "unprocessed"
		return dr
	}
	dr.TextContent = string(b)
	dr.ProcessStatus = "processed"
	dr.ProcessError = ""
	return dr
}
