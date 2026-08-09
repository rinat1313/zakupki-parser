package worker

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rinat1313/zakupki-parser/internal/searchsvc/config"
	"github.com/rinat1313/zakupki-parser/internal/searchsvc/db"
	"github.com/rinat1313/zakupki-parser/internal/searchsvc/models"
	"github.com/rinat1313/zakupki-parser/pkg/eis"
)

// Runner executes EIS search for a searcher and upserts hits.
type Runner struct {
	Store  *db.Store
	Cfg    config.Config
	Client *eis.Client
	HTTP   *http.Client
	Log    *log.Logger
}

func NewRunner(store *db.Store, cfg config.Config) *Runner {
	return &Runner{
		Store:  store,
		Cfg:    cfg,
		Client: eis.NewClient(cfg.InsecureTLS),
		HTTP:   &http.Client{Timeout: 30 * time.Second},
		Log:    log.Default(),
	}
}

// RunSearcher scrapes EIS pages, upserts hits, triggers parser for new ones.
func (r *Runner) RunSearcher(ctx context.Context, userID, searcherID string) (models.SearcherRun, error) {
	runID := newRunID()
	searcher, err := r.Store.TouchSearcherRun(ctx, userID, searcherID)
	if err != nil {
		return models.SearcherRun{}, err
	}

	maxPages := r.Cfg.MaxSearchPages
	if maxPages < 1 {
		maxPages = 3
	}

	found := 0
	newHits := make([]models.SearchHit, 0)
	var lastErr error

	for page := 1; page <= maxPages; page++ {
		pageURL := searcher.Config.PageURL(r.Cfg.EISBaseURL, page)
		body, err := r.Client.Get(pageURL)
		if err != nil {
			lastErr = err
			r.Log.Printf("searcher %s page %d: %v", searcherID, page, err)
			break
		}
		hits := ParseSearchHTML(body)
		if len(hits) == 0 {
			break
		}
		for _, hit := range hits {
			inserted, err := r.Store.UpsertSearcherTender(ctx, searcherID, hit)
			if err != nil {
				lastErr = err
				r.Log.Printf("upsert %s: %v", hit.RegNumber, err)
				continue
			}
			found++
			if inserted {
				newHits = append(newHits, hit)
			}
		}
	}

	status := "done"
	msg := fmt.Sprintf("Обработано хитов: %d, новых: %d", found, len(newHits))
	if lastErr != nil && found == 0 {
		status = "error"
		msg = lastErr.Error()
	}

	if len(newHits) > 0 {
		go r.enrichNewHits(searcher, newHits)
	}

	return models.SearcherRun{
		RunID:      runID,
		SearcherID: searcherID,
		Status:     status,
		FoundCount: found,
		Message:    msg,
	}, nil
}

func (r *Runner) enrichNewHits(searcher models.Searcher, hits []models.SearchHit) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	for _, hit := range hits {
		if r.Cfg.ParserURL != "" {
			if err := r.postParserFetch(ctx, searcher, hit.RegNumber); err != nil {
				r.Log.Printf("parser fetch %s: %v", hit.RegNumber, err)
			}
		}
		if r.Cfg.CoreURL != "" {
			if err := r.notifyCore(ctx, searcher, hit); err != nil {
				r.Log.Printf("core notify %s: %v", hit.RegNumber, err)
			}
			// AI включается на уровне поисковой настройки: после появления карточки
			// в core пробуем поставить анализ в очередь.
			if searcher.AutoAI {
				if err := r.triggerCoreAnalyze(ctx, hit.RegNumber); err != nil {
					r.Log.Printf("core analyze %s: %v", hit.RegNumber, err)
				}
			}
		}
	}
}

func (r *Runner) postParserFetch(ctx context.Context, searcher models.Searcher, regNumber string) error {
	payload, _ := json.Marshal(map[string]any{
		"reg_number":        regNumber,
		"source_site":       r.Cfg.EISBaseURL,
		"search_profile_id": searcher.ID,
		"auto_ai":           searcher.AutoAI,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.Cfg.ParserURL+"/api/v1/fetch", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := r.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("parser HTTP %d", resp.StatusCode)
	}
	return nil
}

func (r *Runner) notifyCore(ctx context.Context, searcher models.Searcher, hit models.SearchHit) error {
	// Best-effort: POST ingest-style payload if core exposes it; otherwise no-op list probe.
	payload, _ := json.Marshal(map[string]any{
		"reg_number":        hit.RegNumber,
		"source_site":       r.Cfg.EISBaseURL,
		"object_name":       hit.ObjectName,
		"notice_url":        hit.NoticeURL,
		"law":               hit.Law,
		"search_profile_id": searcher.ID,
		"searcher_id":       searcher.ID,
		"auto_ai":           searcher.AutoAI,
	})
	url := r.Cfg.CoreURL + "/api/v1/tenders"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := r.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	// 404/405 means core does not accept create this way — ignore.
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusMethodNotAllowed {
		return nil
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("core HTTP %d", resp.StatusCode)
	}
	return nil
}

func (r *Runner) triggerCoreAnalyze(ctx context.Context, reg string) error {
	id, ok := r.lookupCoreTenderID(ctx, reg)
	if !ok || id == "" {
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		r.Cfg.CoreURL+"/api/v1/tenders/"+id+"/analyze", bytes.NewReader([]byte("{}")))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := r.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusConflict {
		return nil
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("analyze HTTP %d", resp.StatusCode)
	}
	return nil
}

func (r *Runner) lookupCoreTenderID(ctx context.Context, reg string) (string, bool) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		r.Cfg.CoreURL+"/api/v1/tenders?q="+urlQueryEscape(reg), nil)
	if err != nil {
		return "", false
	}
	resp, err := r.HTTP.Do(req)
	if err != nil {
		return "", false
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil || resp.StatusCode != http.StatusOK {
		return "", false
	}
	var list []map[string]any
	if err := json.Unmarshal(body, &list); err != nil {
		var wrap struct {
			Items []map[string]any `json:"items"`
		}
		if err2 := json.Unmarshal(body, &wrap); err2 != nil {
			return "", false
		}
		list = wrap.Items
	}
	for _, item := range list {
		if strField(item, "reg_number") != reg {
			continue
		}
		if id := strField(item, "id"); id != "" {
			return id, true
		}
	}
	return "", false
}

// EnrichTendersFromCore fills progress fields from CORE_URL when available.
func (r *Runner) EnrichTendersFromCore(ctx context.Context, cards []models.TenderCard) []models.TenderCard {
	if r.Cfg.CoreURL == "" || len(cards) == 0 {
		return cards
	}
	for i := range cards {
		enriched, ok := r.fetchCoreTender(ctx, cards[i].RegNumber)
		if !ok {
			continue
		}
		mergeTender(&cards[i], enriched)
	}
	return cards
}

func (r *Runner) fetchCoreTender(ctx context.Context, reg string) (models.TenderCard, bool) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		r.Cfg.CoreURL+"/api/v1/tenders?q="+urlQueryEscape(reg), nil)
	if err != nil {
		return models.TenderCard{}, false
	}
	resp, err := r.HTTP.Do(req)
	if err != nil {
		return models.TenderCard{}, false
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil || resp.StatusCode != http.StatusOK {
		return models.TenderCard{}, false
	}

	var list []map[string]any
	if err := json.Unmarshal(body, &list); err != nil {
		var wrap struct {
			Items []map[string]any `json:"items"`
		}
		if err2 := json.Unmarshal(body, &wrap); err2 != nil {
			return models.TenderCard{}, false
		}
		list = wrap.Items
	}
	for _, item := range list {
		if strField(item, "reg_number") != reg {
			continue
		}
		return mapToCard(item), true
	}
	return models.TenderCard{}, false
}

func mergeTender(dst *models.TenderCard, src models.TenderCard) {
	if src.ObjectName != "" {
		dst.ObjectName = src.ObjectName
	}
	if src.ApplicationEnd != "" {
		dst.ApplicationEnd = src.ApplicationEnd
	}
	if src.NMCK != nil {
		dst.NMCK = src.NMCK
	}
	dst.DocsWithText = src.DocsWithText
	dst.DocsTotal = src.DocsTotal
	dst.CollectPct = src.CollectPct
	dst.AIPct = src.AIPct
	if src.AnalysisStatus != "" {
		dst.AnalysisStatus = src.AnalysisStatus
	}
	if src.Recommendation != "" {
		dst.Recommendation = src.Recommendation
	}
	if src.CardTone != "" {
		dst.CardTone = src.CardTone
	}
}

func mapToCard(m map[string]any) models.TenderCard {
	c := models.TenderCard{
		RegNumber:      strField(m, "reg_number"),
		ObjectName:     strField(m, "object_name"),
		ApplicationEnd: strField(m, "application_end"),
		AnalysisStatus: strField(m, "analysis_status"),
		Recommendation: strField(m, "recommendation"),
		CardTone:       strField(m, "card_tone"),
		DocsWithText:   intField(m, "docs_with_text"),
		DocsTotal:      intField(m, "docs_total"),
		CollectPct:     intField(m, "collect_pct"),
		AIPct:          intField(m, "ai_pct"),
	}
	if c.AnalysisStatus == "" {
		c.AnalysisStatus = "none"
	}
	if c.CardTone == "" {
		c.CardTone = "neutral"
	}
	if v, ok := m["nmck"].(float64); ok {
		c.NMCK = &v
	}
	return c
}

func strField(m map[string]any, k string) string {
	v, ok := m[k]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	default:
		return fmt.Sprint(t)
	}
}

func intField(m map[string]any, k string) int {
	v, ok := m[k]
	if !ok || v == nil {
		return 0
	}
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	case json.Number:
		n, _ := t.Int64()
		return int(n)
	default:
		n, _ := strconvAtoi(fmt.Sprint(t))
		return n
	}
}

func strconvAtoi(s string) (int, error) {
	var n int
	_, err := fmt.Sscanf(strings.TrimSpace(s), "%d", &n)
	return n, err
}

func urlQueryEscape(s string) string {
	return url.QueryEscape(s)
}

func newRunID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}
