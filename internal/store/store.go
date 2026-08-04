package store

import (
	"encoding/json"
	"fmt"
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
	"github.com/rinat1313/zakupki-parser/models"
	"github.com/rinat1313/zakupki-parser/pkg/eis"
)

// Layout папок одного тендера result/{id}/.
const (
	DirOrigin    = "origin"     // оригинальные скачанные документы
	DirValidDoc  = "valid_doc"  // обработанные .txt
	DirHTML      = "html"       // оригинальные HTML страницы
	DirValidInfo = "valid_info" // JSON для LLM
	DirOverInfo  = "over_info"  // прочая информация
)

// Options настройки выгрузки одного тендера.
type Options struct {
	RootDir    string // DataCode/result
	NoticeType string
	NoticeGUID string // 223: опционально, иначе из HTML
	EnrichOrg  bool
	Download   bool
	ToText     bool // конвертировать вложения в .txt (LibreOffice/OCR)
	Delay      time.Duration
	DetectTries int // повторы Detect при пустом ответе ЕИС (по умолчанию 5)
}

type tenderDirs struct {
	root      string
	origin    string
	validDoc  string
	html      string
	validInfo string
	overInfo  string
}

func prepareDirs(root, regNumber string) (tenderDirs, error) {
	base := filepath.Join(root, regNumber)
	d := tenderDirs{
		root:      base,
		origin:    filepath.Join(base, DirOrigin),
		validDoc:  filepath.Join(base, DirValidDoc),
		html:      filepath.Join(base, DirHTML),
		validInfo: filepath.Join(base, DirValidInfo),
		overInfo:  filepath.Join(base, DirOverInfo),
	}
	for _, p := range []string{d.origin, d.validDoc, d.html, d.validInfo, d.overInfo} {
		if err := os.MkdirAll(p, 0o755); err != nil {
			return d, err
		}
	}
	return d, nil
}

// dirExisted — папка тендера уже была до текущей попытки выгрузки.
func dirExisted(root, regNumber string) bool {
	_, err := os.Stat(filepath.Join(root, regNumber))
	return err == nil
}

// cleanupIfNew удаляет папку тендера, если выгрузка упала и папки раньше не было.
func cleanupIfNew(d tenderDirs, existed bool, err error) {
	if err != nil && !existed {
		_ = os.RemoveAll(d.root)
	}
}

// ExportAuto определяет 44/223/pricereq на ЕИС и вызывает нужный экспорт.
func ExportAuto(client *eis.Client, regNumber string, opt Options) (*models.TenderExport, error) {
	tries := opt.DetectTries
	if tries < 1 {
		tries = 1
	}
	det, err := detect.DetectAttempts(client, regNumber, tries)
	if err != nil {
		return nil, err
	}
	fmt.Fprintf(os.Stderr, "  law=%s via %s", det.Law, det.Source)
	if det.NoticeType != "" {
		fmt.Fprintf(os.Stderr, " type=%s", det.NoticeType)
	}
	if det.NoticeGUID != "" {
		fmt.Fprintf(os.Stderr, " guid=%s", det.NoticeGUID)
	}
	fmt.Fprintln(os.Stderr)

	if det.NoticeType != "" {
		opt.NoticeType = det.NoticeType
	}
	if det.NoticeGUID != "" && opt.NoticeGUID == "" {
		opt.NoticeGUID = det.NoticeGUID
	}
	if det.Law == models.Law223 {
		return ExportTender223(client, regNumber, opt)
	}
	if det.Law == models.LawPricereq {
		return ExportPricereq(client, regNumber, opt)
	}
	return ExportTender(client, regNumber, opt)
}

// ExportTender качает карточку 44-ФЗ → result/{regNumber}/.
func ExportTender(client *eis.Client, regNumber string, opt Options) (exp *models.TenderExport, err error) {
	if opt.RootDir == "" {
		opt.RootDir = "result"
	}
	if opt.NoticeType == "" {
		opt.NoticeType = "ea20"
	}

	noticeURL := eis.NoticeCommonInfoURL(client.BaseURL, opt.NoticeType, regNumber)
	noticeHTML, err := client.Get(noticeURL)
	if err != nil {
		return nil, fmt.Errorf("notice: %w", err)
	}
	tender, err := fz44.ParseNoticeHTML(noticeHTML, noticeURL)
	if err != nil {
		return nil, fmt.Errorf("parse notice: %w", err)
	}

	existed := dirExisted(opt.RootDir, regNumber)
	d, err := prepareDirs(opt.RootDir, regNumber)
	if err != nil {
		return nil, err
	}
	defer func() { cleanupIfNew(d, existed, err) }()

	_ = os.WriteFile(filepath.Join(d.html, "notice.html"), noticeHTML, 0o644)

	var customer *models.Organization44
	if opt.EnrichOrg && tender.OrganizationCode != "" {
		time.Sleep(opt.Delay)
		orgURL := eis.OrganizationInfoURL(client.BaseURL, tender.OrganizationCode)
		orgHTML, errOrg := client.Get(orgURL)
		if errOrg != nil {
			fmt.Fprintf(os.Stderr, "  org warn: %v\n", errOrg)
		} else {
			_ = os.WriteFile(filepath.Join(d.html, "customer.html"), orgHTML, 0o644)
			inn, kpp, name, errOrg := fz44.ParseOrganizationHTML(orgHTML)
			if errOrg != nil {
				fmt.Fprintf(os.Stderr, "  org parse warn: %v\n", errOrg)
			} else {
				customer = &models.Organization44{
					OrganizationCode: tender.OrganizationCode,
					INN:              inn,
					KPP:              kpp,
					FullName:         name,
					Email:            tender.ContactEmail,
					Phone:            tender.ContactPhone,
					ContactPerson:    tender.ContactPerson,
				}
				if name != "" {
					customer.FullName = name
				} else {
					customer.FullName = tender.CustomerName
				}
				tender.CustomerINN = inn
				tender.CustomerKPP = kpp
			}
		}
	}
	if customer == nil {
		customer = &models.Organization44{
			OrganizationCode: tender.OrganizationCode,
			INN:              tender.CustomerINN,
			KPP:              tender.CustomerKPP,
			FullName:         tender.CustomerName,
			Email:            tender.ContactEmail,
			Phone:            tender.ContactPhone,
			ContactPerson:    tender.ContactPerson,
		}
	}

	var saved []models.SavedFile
	if opt.Download {
		time.Sleep(opt.Delay)
		docsURL := eis.NoticeDocumentsURL(client.BaseURL, opt.NoticeType, regNumber)
		docsHTML, errDocs := client.Get(docsURL)
		if errDocs != nil {
			fmt.Fprintf(os.Stderr, "  docs warn: %v\n", errDocs)
		} else {
			_ = os.WriteFile(filepath.Join(d.html, "documents.html"), docsHTML, 0o644)
			docs, errDocs := fz44.ParseDocumentsHTML(docsHTML)
			if errDocs != nil {
				fmt.Fprintf(os.Stderr, "  docs parse warn: %v\n", errDocs)
			} else {
				for _, doc := range docs {
					time.Sleep(opt.Delay)
					sf := downloadOne(client, doc, d)
					saved = append(saved, sf)
				}
			}
		}
	}

	var failed []models.FailedText
	if opt.ToText {
		saved, failed = applyTextConversion(d, saved)
	}

	exp = &models.TenderExport{
		RegNumber:   regNumber,
		Law:         models.Law44,
		FetchedAt:   time.Now().Format(time.RFC3339),
		Tender:      tender,
		Customer:    customer,
		Files:       saved,
		FailedTexts: failed,
	}
	err = writeExport(d, exp, tender, customer, nil, saved)
	return exp, err
}

// ExportTender223 качает карточку 223-ФЗ → result/{regNumber}/.
func ExportTender223(client *eis.Client, regNumber string, opt Options) (exp *models.TenderExport, err error) {
	if opt.RootDir == "" {
		opt.RootDir = "result"
	}

	guid := opt.NoticeGUID
	noticeURL := eis.Notice223CommonInfoURL(client.BaseURL, regNumber, guid)
	noticeHTML, err := client.Get(noticeURL)
	if err != nil {
		return nil, fmt.Errorf("notice223: %w", err)
	}
	if guid == "" {
		guid = fz223.ExtractNoticeGUID(noticeHTML)
	}
	tender, err := fz223.ParseNoticeHTML(noticeHTML, noticeURL)
	if err != nil {
		return nil, fmt.Errorf("parse notice223: %w", err)
	}
	if tender.NoticeGUID == "" {
		tender.NoticeGUID = guid
	}
	if guid == "" {
		guid = tender.NoticeGUID
	}

	existed := dirExisted(opt.RootDir, regNumber)
	d, err := prepareDirs(opt.RootDir, regNumber)
	if err != nil {
		return nil, err
	}
	defer func() { cleanupIfNew(d, existed, err) }()

	_ = os.WriteFile(filepath.Join(d.html, "notice.html"), noticeHTML, 0o644)

	var customer *models.Organization223
	if opt.EnrichOrg && (tender.CustomerAgencyID != "" || tender.CustomerINN != "") {
		time.Sleep(opt.Delay)
		orgURL := eis.Organization223InfoURL(client.BaseURL, tender.CustomerAgencyID,
			tender.CustomerINN, tender.CustomerKPP, tender.CustomerOGRN)
		orgHTML, errOrg := client.Get(orgURL)
		if errOrg != nil {
			fmt.Fprintf(os.Stderr, "  org223 warn: %v\n", errOrg)
		} else {
			_ = os.WriteFile(filepath.Join(d.html, "customer.html"), orgHTML, 0o644)
			org, errOrg := fz223.ParseOrganizationHTML(orgHTML)
			if errOrg != nil {
				fmt.Fprintf(os.Stderr, "  org223 parse warn: %v\n", errOrg)
			} else {
				customer = org
				if org.AgencyID != "" {
					tender.CustomerAgencyID = org.AgencyID
				}
				if org.INN != "" {
					tender.CustomerINN = org.INN
				}
				if org.KPP != "" {
					tender.CustomerKPP = org.KPP
				}
				if org.OGRN != "" {
					tender.CustomerOGRN = org.OGRN
				}
			}
		}
	}
	if customer == nil {
		customer = &models.Organization223{
			AgencyID:      tender.CustomerAgencyID,
			INN:           tender.CustomerINN,
			KPP:           tender.CustomerKPP,
			OGRN:          tender.CustomerOGRN,
			FullName:      tender.CustomerName,
			Address:       tender.CustomerAddress,
			Email:         tender.ContactEmail,
			ContactPerson: tender.ContactPerson,
		}
	}

	var lots []models.Lot223
	time.Sleep(opt.Delay)
	lotsURL := eis.Notice223LotListURL(client.BaseURL, regNumber, guid)
	lotsHTML, errLots := client.Get(lotsURL)
	if errLots != nil {
		fmt.Fprintf(os.Stderr, "  lots223 warn: %v\n", errLots)
	} else {
		_ = os.WriteFile(filepath.Join(d.html, "lots.html"), lotsHTML, 0o644)
		lots, errLots = fz223.ParseLotListHTML(lotsHTML, regNumber)
		if errLots != nil {
			fmt.Fprintf(os.Stderr, "  lots223 parse warn: %v\n", errLots)
		}
	}

	var saved []models.SavedFile
	if opt.Download {
		time.Sleep(opt.Delay)
		docsURL := eis.Notice223DocumentsURL(client.BaseURL, regNumber, guid)
		docsHTML, errDocs := client.Get(docsURL)
		if errDocs != nil {
			fmt.Fprintf(os.Stderr, "  docs223 warn: %v\n", errDocs)
		} else {
			_ = os.WriteFile(filepath.Join(d.html, "documents.html"), docsHTML, 0o644)
			docs, errDocs := fz223.ParseDocumentsHTML(docsHTML)
			if errDocs != nil {
				fmt.Fprintf(os.Stderr, "  docs223 parse warn: %v\n", errDocs)
			} else {
				for _, doc := range docs {
					time.Sleep(opt.Delay)
					sf := downloadOne(client, doc, d)
					saved = append(saved, sf)
				}
			}
		}
	}

	var failed []models.FailedText
	if opt.ToText {
		saved, failed = applyTextConversion(d, saved)
	}

	exp = &models.TenderExport{
		RegNumber:   regNumber,
		Law:         models.Law223,
		FetchedAt:   time.Now().Format(time.RFC3339),
		Tender223:   tender,
		Customer223: customer,
		Lots:        lots,
		Files:       saved,
		FailedTexts: failed,
	}
	err = writeExport(d, exp, tender, customer, lots, saved)
	return exp, err
}

// ExportPricereq качает карточку «Запросы цен» (/epz/pricereq) → result/{reestrNumber}/.
// Папки те же (origin/valid_doc/html/valid_info/over_info); в tender.json — PriceRequest.
func ExportPricereq(client *eis.Client, reestrNumber string, opt Options) (exp *models.TenderExport, err error) {
	if opt.RootDir == "" {
		opt.RootDir = "result"
	}

	noticeURL := eis.PriceReqCommonInfoURL(client.BaseURL, reestrNumber)
	noticeHTML, err := client.Get(noticeURL)
	if err != nil {
		return nil, fmt.Errorf("pricereq common-info: %w", err)
	}

	var searchHTML []byte
	searchURL := ""
	tender, err := pricereq.ParseCommonInfoHTML(noticeHTML, noticeURL)
	if err != nil {
		searchURL = eis.SearchPricereqURL(client.BaseURL, reestrNumber)
		time.Sleep(opt.Delay)
		var serr error
		searchHTML, serr = client.Get(searchURL)
		if serr != nil {
			return nil, fmt.Errorf("parse pricereq: %w", err)
		}
		tender, err = pricereq.ParseCommonInfoHTML(searchHTML, searchURL)
		if err != nil {
			return nil, fmt.Errorf("parse pricereq: %w", err)
		}
	}
	if tender.ReestrNumber == "" {
		tender.ReestrNumber = reestrNumber
	}

	existed := dirExisted(opt.RootDir, reestrNumber)
	d, err := prepareDirs(opt.RootDir, reestrNumber)
	if err != nil {
		return nil, err
	}
	defer func() { cleanupIfNew(d, existed, err) }()

	_ = os.WriteFile(filepath.Join(d.html, "common-info.html"), noticeHTML, 0o644)
	if len(searchHTML) > 0 {
		_ = os.WriteFile(filepath.Join(d.html, "search.html"), searchHTML, 0o644)
	}

	var customer *models.Organization44
	if opt.EnrichOrg && tender.OrganizationCode != "" {
		time.Sleep(opt.Delay)
		orgURL := eis.OrganizationInfoURL(client.BaseURL, tender.OrganizationCode)
		orgHTML, errOrg := client.Get(orgURL)
		if errOrg != nil {
			fmt.Fprintf(os.Stderr, "  org warn: %v\n", errOrg)
		} else {
			_ = os.WriteFile(filepath.Join(d.html, "customer.html"), orgHTML, 0o644)
			inn, kpp, name, errOrg := fz44.ParseOrganizationHTML(orgHTML)
			if errOrg != nil {
				fmt.Fprintf(os.Stderr, "  org parse warn: %v\n", errOrg)
			} else {
				customer = &models.Organization44{
					OrganizationCode: tender.OrganizationCode,
					INN:              inn,
					KPP:              kpp,
					FullName:         name,
					Email:            tender.ContactEmail,
					Phone:            tender.ContactPhone,
					ContactPerson:    tender.ContactPerson,
				}
				if name == "" {
					customer.FullName = tender.OrgName
				}
				tender.CustomerINN = inn
				tender.CustomerKPP = kpp
			}
		}
	}
	if customer == nil {
		customer = &models.Organization44{
			OrganizationCode: tender.OrganizationCode,
			INN:              tender.CustomerINN,
			KPP:              tender.CustomerKPP,
			FullName:         tender.OrgName,
			Email:            tender.ContactEmail,
			Phone:            tender.ContactPhone,
			ContactPerson:    tender.ContactPerson,
		}
	}

	var saved []models.SavedFile
	if opt.Download {
		time.Sleep(opt.Delay)
		docsURL := eis.PriceReqDocumentsURL(client.BaseURL, reestrNumber)
		docsHTML, errDocs := client.Get(docsURL)
		if errDocs != nil {
			fmt.Fprintf(os.Stderr, "  docs warn: %v\n", errDocs)
		} else {
			_ = os.WriteFile(filepath.Join(d.html, "documents.html"), docsHTML, 0o644)
			docs, errDocs := fz44.ParseDocumentsHTML(docsHTML)
			if errDocs != nil {
				fmt.Fprintf(os.Stderr, "  docs parse warn: %v\n", errDocs)
			} else {
				for _, doc := range docs {
					time.Sleep(opt.Delay)
					sf := downloadOne(client, doc, d)
					saved = append(saved, sf)
				}
			}
		}
	}

	var failed []models.FailedText
	if opt.ToText {
		saved, failed = applyTextConversion(d, saved)
	}

	exp = &models.TenderExport{
		RegNumber:    reestrNumber,
		Law:          models.LawPricereq,
		FetchedAt:    time.Now().Format(time.RFC3339),
		PriceRequest: tender,
		Customer:     customer,
		Files:        saved,
		FailedTexts:  failed,
	}
	if err = writeJSON(filepath.Join(d.validInfo, "items.json"), tender.Items); err != nil {
		return nil, err
	}
	err = writeExport(d, exp, tender, customer, nil, saved)
	return exp, err
}

func writeExport(d tenderDirs, exp *models.TenderExport, tender, customer any, lots any, saved []models.SavedFile) error {
	if err := writeJSON(filepath.Join(d.validInfo, "tender.json"), tender); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(d.validInfo, "customer.json"), customer); err != nil {
		return err
	}
	if lots != nil {
		if err := writeJSON(filepath.Join(d.validInfo, "lots.json"), lots); err != nil {
			return err
		}
	}
	if err := writeJSON(filepath.Join(d.validInfo, "files.json"), saved); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(d.validInfo, "export.json"), exp); err != nil {
		return err
	}
	return nil
}

func downloadOne(client *eis.Client, doc models.DocumentFile, d tenderDirs) models.SavedFile {
	sf := models.SavedFile{
		UID:          doc.UID,
		SourceURL:    doc.URL,
		OriginalName: doc.Filename,
		GroupTitle:   doc.GroupTitle,
		Edition:      doc.Edition,
	}
	localName := doc.Filename
	if localName == "" {
		localName = doc.UID
	}

	meta, err := client.DownloadMeta(doc.URL)
	if err != nil {
		sf.Error = err.Error()
		return sf
	}

	hintExt := filemeta.ExtFromContentDisposition(meta.Header)
	sniffed := filemeta.SniffExt(meta.Body)
	localName = filemeta.EnsureExt(localName, hintExt, sniffed)
	sf.OriginalName = localName

	dest := uniquePath(filepath.Join(d.origin, localName))
	if err := os.WriteFile(dest, meta.Body, 0o644); err != nil {
		sf.Error = err.Error()
		return sf
	}
	sf.SizeBytes = int64(len(meta.Body))
	if r, err := filepath.Rel(d.root, dest); err == nil {
		sf.LocalPath = r
	} else {
		sf.LocalPath = filepath.Join(DirOrigin, filepath.Base(dest))
	}

	if archiveutil.IsArchive(dest) {
		extractDir := filepath.Join(d.origin, strings.TrimSuffix(filepath.Base(dest), filepath.Ext(dest)))
		leaves, err := archiveutil.ExtractRecursive(dest, extractDir, 6)
		if err != nil {
			sf.Error = "extract: " + err.Error()
			return sf
		}
		_ = os.Remove(dest)
		sf.ArchiveRemoved = true
		sf.LocalPath = ""
		for _, leaf := range leaves {
			if rel, err := filepath.Rel(d.root, leaf); err == nil {
				sf.Extracted = append(sf.Extracted, rel)
			} else {
				sf.Extracted = append(sf.Extracted, leaf)
			}
		}
	}
	return sf
}

// applyTextConversion: origin → valid_doc, пишет failed_texts в over_info.
func applyTextConversion(d tenderDirs, saved []models.SavedFile) ([]models.SavedFile, []models.FailedText) {
	results := extract.Dir(d.origin, d.validDoc)
	bySrc := map[string]extract.Result{}
	var failed []models.FailedText
	regNumber := filepath.Base(d.root)
	now := time.Now().Format(time.RFC3339)

	for _, r := range results {
		bySrc[r.SourcePath] = r
		if r.Error != "" {
			fmt.Fprintf(os.Stderr, "  txt FAIL: %s: %s\n", filepath.Base(r.SourcePath), r.Error)
			rel := r.SourcePath
			if x, err := filepath.Rel(d.root, r.SourcePath); err == nil {
				rel = x
			}
			failed = append(failed, models.FailedText{
				RegNumber:  regNumber,
				SourcePath: rel,
				SourceName: filepath.Base(r.SourcePath),
				Engine:     r.Engine,
				RawBytes:   r.Bytes,
				Error:      r.Error,
				At:         now,
			})
		} else {
			fmt.Fprintf(os.Stderr, "  txt: %s → %s (%s, %d bytes)\n",
				filepath.Base(r.SourcePath), filepath.Base(r.TextPath), r.Engine, r.Bytes)
		}
	}

	for i := range saved {
		candidates := []string{}
		if saved[i].LocalPath != "" {
			candidates = append(candidates, filepath.Join(d.root, saved[i].LocalPath))
		}
		for _, e := range saved[i].Extracted {
			candidates = append(candidates, filepath.Join(d.root, e))
		}
		for _, c := range candidates {
			abs, _ := filepath.Abs(c)
			r, ok := bySrc[abs]
			if !ok {
				continue
			}
			if r.Error != "" {
				saved[i].TextError = r.Error
				saved[i].TextEngine = r.Engine
				continue
			}
			if rel, err := filepath.Rel(d.root, r.TextPath); err == nil {
				saved[i].TextPath = rel
			}
			saved[i].TextEngine = r.Engine
			saved[i].TextError = ""
		}
	}

	_ = writeJSON(filepath.Join(d.overInfo, "texts.json"), results)
	_ = writeJSON(filepath.Join(d.overInfo, "failed_texts.json"), failed)
	return saved, failed
}

func uniquePath(p string) string {
	if _, err := os.Stat(p); os.IsNotExist(err) {
		return p
	}
	ext := filepath.Ext(p)
	base := strings.TrimSuffix(p, ext)
	for i := 2; i < 1000; i++ {
		cand := fmt.Sprintf("%s_%d%s", base, i, ext)
		if _, err := os.Stat(cand); os.IsNotExist(err) {
			return cand
		}
	}
	return p
}

func writeJSON(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}
