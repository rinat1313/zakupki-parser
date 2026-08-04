package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rinat1313/zakupki-parser/internal/csvinput"
	"github.com/rinat1313/zakupki-parser/internal/parser/fz223"
	"github.com/rinat1313/zakupki-parser/internal/parser/fz44"
	"github.com/rinat1313/zakupki-parser/internal/parser/pricereq"
	"github.com/rinat1313/zakupki-parser/internal/regnum"
	"github.com/rinat1313/zakupki-parser/internal/store"
	"github.com/rinat1313/zakupki-parser/models"
	"github.com/rinat1313/zakupki-parser/pkg/eis"
)

type runStatus struct {
	CSVId      string `json:"csv_id"`
	ResolvedId string `json:"resolved_id,omitempty"`
	Status     string `json:"status"` // ok | bad
	Law        string `json:"law,omitempty"`
	Error      string `json:"error,omitempty"`
	Index      int    `json:"index"`
}

type jobResult struct {
	status runStatus
	exp    *models.TenderExport
}

func main() {
	var (
		csvPath    = flag.String("csv", "", "путь к CSV (по умолчанию data/tenders.csv)")
		fixture    = flag.String("fixture", "", "парсить локальный HTML notice (без выгрузки папки)")
		law        = flag.String("law", "auto", "закон: auto, 44, 223 или pricereq")
		noticeType = flag.String("type", "ea20", "тип извещения 44-ФЗ, если -law=44")
		enrichOrg  = flag.Bool("org", true, "добирать карточку заказчика")
		download   = flag.Bool("download", true, "скачивать документы")
		toText     = flag.Bool("txt", true, "конвертировать вложения в .txt")
		insecure   = flag.Bool("insecure", true, "TLS InsecureSkipVerify (как curl -k)")
		delay      = flag.Duration("delay", 1200*time.Millisecond, "пауза между запросами внутри одной закупки")
		limit      = flag.Int("limit", 0, "обработать только первые N номеров (0 = все)")
		workers    = flag.Int("workers", 1, "параллельных загрузок (1 рекомендуется; макс. 5)")
		retries    = flag.Int("retries", 5, "повторов при ошибке поиска/детекта одной закупки")
		resultDir  = flag.String("result", "result", "корневая папка выгрузки")
		outPath    = flag.String("o", "", "дополнительно записать сводный JSON-массив")
	)
	flag.Parse()

	mode := normalizeLawMode(*law)

	if *fixture != "" {
		runFixture(*fixture, mode, *outPath)
		return
	}

	if *workers < 1 {
		*workers = 1
	}
	if *workers > 5 {
		*workers = 5
	}
	if *retries < 1 {
		*retries = 1
	}

	path := *csvPath
	if path == "" {
		path = defaultCSVPath()
	}

	recs, err := csvinput.LoadRecords(path)
	if err != nil {
		fatal(err)
	}
	if *limit > 0 && *limit < len(recs) {
		recs = recs[:*limit]
	}

	root := *resultDir
	if !filepath.IsAbs(root) {
		root, _ = filepath.Abs(root)
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		fatal(err)
	}

	client := eis.NewClient(*insecure)

	jobs := make(chan int)
	results := make([]jobResult, len(recs))
	var wg sync.WaitGroup
	var logMu sync.Mutex
	var folderLocks sync.Map // reg_number → *sync.Mutex (не писать одну папку двумя воркерами)

	logf := func(format string, args ...any) {
		logMu.Lock()
		defer logMu.Unlock()
		fmt.Fprintf(os.Stderr, format, args...)
	}

	lockFolder := func(id string) func() {
		v, _ := folderLocks.LoadOrStore(id, &sync.Mutex{})
		m := v.(*sync.Mutex)
		m.Lock()
		return m.Unlock
	}

	workerFn := func() {
		defer wg.Done()
		for i := range jobs {
			rec := recs[i]
			lawForRow := mode
			if lawForRow == "auto" && rec.LawHint != "" {
				lawForRow = rec.LawHint
			}

			opt := store.Options{
				RootDir:     root,
				NoticeType:  *noticeType,
				NoticeGUID:  rec.NoticeGUID,
				EnrichOrg:   *enrichOrg,
				Download:    *download,
				ToText:      *toText,
				Delay:       *delay,
				DetectTries: 1, // повторы делает внешний цикл -retries
			}

			candidates := regnum.Candidates(rec.RegNumber)
			logf("[%d/%d] [%s] csv=%s candidates=%v\n",
				i+1, len(recs), lawForRow, rec.RegNumber, candidates)

			var (
				exp     *models.TenderExport
				usedID  string
				lastErr error
			)
			for _, id := range candidates {
				unlock := lockFolder(id)
				for attempt := 1; attempt <= *retries; attempt++ {
					if attempt > 1 {
						wait := time.Duration(attempt) * 2 * time.Second
						logf("  [%s] retry %d/%d for %s after %s\n", rec.RegNumber, attempt, *retries, id, wait)
						time.Sleep(wait)
					}
					exp, lastErr = exportOne(client, id, lawForRow, opt)
					if lastErr == nil {
						usedID = id
						if id != rec.RegNumber {
							logf("  [%s] OK after normalize → %s\n", rec.RegNumber, id)
						} else if attempt > 1 {
							logf("  [%s] OK on attempt %d\n", id, attempt)
						}
						break
					}
					logf("  [%s] try %s (attempt %d/%d): %v\n", rec.RegNumber, id, attempt, *retries, lastErr)
				}
				unlock()
				if lastErr == nil {
					break
				}
			}

			jr := jobResult{}
			if lastErr != nil {
				logf("  [%s] ERROR: %v\n", rec.RegNumber, lastErr)
				// папку result/{csv_id} при ошибке НЕ создаём — только статусы в корне
				jr.status = runStatus{
					CSVId:  rec.RegNumber,
					Status: "bad",
					Law:    lawForRow,
					Error:  lastErr.Error(),
					Index:  i,
				}
				jr.exp = &models.TenderExport{
					RegNumber: rec.RegNumber,
					Law:       models.Law(lawForRow),
					FetchedAt: time.Now().Format(time.RFC3339),
				}
			} else {
				printOKLocked(&logMu, exp)
				jr.status = runStatus{
					CSVId:      rec.RegNumber,
					ResolvedId: usedID,
					Status:     "ok",
					Law:        string(exp.Law),
					Index:      i,
				}
				if len(exp.FailedTexts) > 0 {
					logf("  [%s] failed_texts: %d\n", usedID, len(exp.FailedTexts))
				}
				jr.exp = exp
			}
			results[i] = jr
		}
	}

	wg.Add(*workers)
	for w := 0; w < *workers; w++ {
		go workerFn()
	}
	for i := range recs {
		jobs <- i
	}
	close(jobs)
	wg.Wait()

	var summary []*models.TenderExport
	var allFailed []models.FailedText
	statuses := make([]runStatus, len(results))
	okN, badN := 0, 0
	for i, jr := range results {
		statuses[i] = jr.status
		if jr.exp != nil {
			summary = append(summary, jr.exp)
			allFailed = append(allFailed, jr.exp.FailedTexts...)
		}
		if jr.status.Status == "ok" {
			okN++
		} else {
			badN++
		}
	}

	_ = writeJSONFile(filepath.Join(root, "summary.json"), summary)
	_ = writeJSONFile(filepath.Join(root, "failed_texts.json"), allFailed)
	_ = writeJSONFile(filepath.Join(root, "statuses.json"), statuses)
	statusCSV := filepath.Join(root, "statuses.csv")
	if err := writeStatusesCSV(statusCSV, statuses); err != nil {
		fatal(err)
	}

	fmt.Fprintf(os.Stderr, "summary → %s\n", filepath.Join(root, "summary.json"))
	fmt.Fprintf(os.Stderr, "statuses → %s  (ok=%d bad=%d)\n", statusCSV, okN, badN)
	if len(allFailed) > 0 {
		fmt.Fprintf(os.Stderr, "FAILED TEXTS (%d) → %s\n", len(allFailed), filepath.Join(root, "failed_texts.json"))
	} else {
		fmt.Fprintf(os.Stderr, "failed_texts → %s (пусто)\n", filepath.Join(root, "failed_texts.json"))
	}

	if *outPath != "" {
		writeJSON(*outPath, summary)
	}
}

func exportOne(client *eis.Client, id, lawForRow string, opt store.Options) (*models.TenderExport, error) {
	switch lawForRow {
	case "223":
		return store.ExportTender223(client, id, opt)
	case "44":
		return store.ExportTender(client, id, opt)
	case "pricereq":
		return store.ExportPricereq(client, id, opt)
	default:
		return store.ExportAuto(client, id, opt)
	}
}

func printOKLocked(mu *sync.Mutex, exp *models.TenderExport) {
	mu.Lock()
	defer mu.Unlock()
	switch {
	case exp.PriceRequest != nil:
		fmt.Fprintf(os.Stderr, "  OK [pricereq]: %s | status=%s | INN=%s | items=%d | files=%d\n",
			exp.PriceRequest.ObjectName, exp.PriceRequest.Status,
			exp.PriceRequest.CustomerINN, len(exp.PriceRequest.Items), len(exp.Files))
	case exp.Tender223 != nil:
		fmt.Fprintf(os.Stderr, "  OK [223]: %s | price=%.2f | INN=%s | lots=%d | files=%d\n",
			exp.Tender223.ObjectName, exp.Tender223.InitialPrice.Amount,
			exp.Tender223.CustomerINN, len(exp.Lots), len(exp.Files))
	case exp.Tender != nil:
		fmt.Fprintf(os.Stderr, "  OK [44]: %s | NMCK=%.2f | INN=%s | files=%d\n",
			exp.Tender.ObjectName, exp.Tender.NMCK.Amount, exp.Tender.CustomerINN, len(exp.Files))
	default:
		fmt.Fprintf(os.Stderr, "  OK: %s files=%d\n", exp.RegNumber, len(exp.Files))
	}
}

func writeStatusesCSV(path string, statuses []runStatus) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	_ = w.Write([]string{"csv_id", "resolved_id", "status", "law", "error"})
	for _, s := range statuses {
		_ = w.Write([]string{s.CSVId, s.ResolvedId, s.Status, s.Law, s.Error})
	}
	w.Flush()
	return w.Error()
}

func sanitizeFolderName(id string) string {
	repl := strings.NewReplacer("/", "_", "\\", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_")
	return repl.Replace(id)
}

func normalizeLawMode(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.ReplaceAll(s, "-", "")
	s = strings.ReplaceAll(s, " ", "")
	s = strings.TrimSuffix(s, "фз")
	switch s {
	case "223", "44", "auto", "pricereq", "price", "pricerequest":
		if s == "" {
			return "auto"
		}
		if s == "price" || s == "pricerequest" {
			return "pricereq"
		}
		return s
	case "":
		return "auto"
	default:
		return "auto"
	}
}

func runFixture(path, mode, outPath string) {
	html, err := os.ReadFile(path)
	if err != nil {
		fatal(err)
	}
	lowPath := strings.ToLower(path)
	usePricereq := mode == "pricereq" || (mode == "auto" && (strings.Contains(lowPath, "pricereq") || strings.Contains(lowPath, "запрос")))
	use223 := mode == "223" || (mode == "auto" && strings.Contains(lowPath, "223") && !usePricereq)
	if usePricereq {
		n, err := pricereq.ParseCommonInfoHTML(html, "fixture://"+filepath.Base(path))
		if err != nil {
			fatal(err)
		}
		writeJSON(outPath, n)
		fmt.Fprintf(os.Stderr, "fixture pricereq OK: %s status=%s org=%s items=%d\n",
			n.ReestrNumber, n.Status, n.OrganizationCode, len(n.Items))
		return
	}
	if use223 {
		n, err := fz223.ParseNoticeHTML(html, "fixture://"+filepath.Base(path))
		if err != nil {
			fatal(err)
		}
		writeJSON(outPath, n)
		fmt.Fprintf(os.Stderr, "fixture 223 OK: %s price=%.2f inn=%s guid=%s\n",
			n.PurchaseNoticeNumber, n.InitialPrice.Amount, n.CustomerINN, n.NoticeGUID)
		return
	}
	n, err := fz44.ParseNoticeHTML(html, "fixture://"+filepath.Base(path))
	if err != nil {
		fatal(err)
	}
	writeJSON(outPath, []*models.Notice44{n})
	fmt.Fprintf(os.Stderr, "fixture OK: %s NMCK=%.2f email=%s org=%s\n",
		n.RegNumber, n.NMCK.Amount, n.ContactEmail, n.OrganizationCode)
}

func writeJSON(outPath string, v any) {
	if outPath == "" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.SetEscapeHTML(false)
		_ = enc.Encode(v)
		return
	}
	if err := writeJSONFile(outPath, v); err != nil {
		fatal(err)
	}
	fmt.Fprintf(os.Stderr, "written %s\n", outPath)
}

func writeJSONFile(path string, v any) error {
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

func defaultCSVPath() string {
	candidates := []string{
		filepath.Join("data", "tenders.csv"),
		filepath.Join("..", "data", "tenders.csv"),
		filepath.Join("..", "..", "data", "tenders.csv"),
	}
	if wd, err := os.Getwd(); err == nil {
		candidates = append(candidates,
			filepath.Join(wd, "data", "tenders.csv"),
			filepath.Join(wd, "..", "data", "tenders.csv"),
		)
	}
	for _, p := range candidates {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			abs, _ := filepath.Abs(p)
			return abs
		}
	}
	return filepath.Join("data", "tenders.csv")
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(1)
}
