package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/rinat1313/zakupki-parser/internal/extract"
	"github.com/rinat1313/zakupki-parser/internal/store"
	"github.com/rinat1313/zakupki-parser/models"
)

// cmd/extract — прогон уже скачанных result/{id}/origin → valid_doc/*.txt
func main() {
	var (
		resultDir = flag.String("result", "result", "корень DataCode/result")
		id        = flag.String("id", "", "только один reg_number (пусто = все папки)")
	)
	flag.Parse()

	root, _ := filepath.Abs(*resultDir)
	entries, err := os.ReadDir(root)
	if err != nil {
		fatal(err)
	}

	var allFailed []models.FailedText
	now := time.Now().Format(time.RFC3339)

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if *id != "" && name != *id {
			continue
		}

		tenderDir := filepath.Join(root, name)
		originDir := filepath.Join(tenderDir, store.DirOrigin)
		validDocDir := filepath.Join(tenderDir, store.DirValidDoc)
		overInfoDir := filepath.Join(tenderDir, store.DirOverInfo)

		// совместимость со старой структурой files/
		if st, err := os.Stat(originDir); err != nil || !st.IsDir() {
			legacy := filepath.Join(tenderDir, "files")
			if st, err := os.Stat(legacy); err == nil && st.IsDir() {
				originDir = legacy
				validDocDir = legacy // рядом со старыми файлами
				overInfoDir = tenderDir
			} else {
				continue
			}
		}
		_ = os.MkdirAll(validDocDir, 0o755)
		_ = os.MkdirAll(overInfoDir, 0o755)

		fmt.Fprintf(os.Stderr, "extract %s …\n", name)
		results := extract.Dir(originDir, validDocDir)
		ok, fail := 0, 0
		var failed []models.FailedText
		for _, r := range results {
			if r.Error != "" {
				fail++
				fmt.Fprintf(os.Stderr, "  FAIL %s: %s\n", filepath.Base(r.SourcePath), r.Error)
				rel := r.SourcePath
				if x, err := filepath.Rel(tenderDir, r.SourcePath); err == nil {
					rel = x
				}
				ft := models.FailedText{
					RegNumber:  name,
					SourcePath: rel,
					SourceName: filepath.Base(r.SourcePath),
					Engine:     r.Engine,
					RawBytes:   r.Bytes,
					Error:      r.Error,
					At:         now,
				}
				failed = append(failed, ft)
				allFailed = append(allFailed, ft)
			} else {
				ok++
				fmt.Fprintf(os.Stderr, "  OK %s → %s (%s, %d bytes)\n",
					filepath.Base(r.SourcePath), filepath.Base(r.TextPath), r.Engine, r.Bytes)
			}
		}
		_ = writeJSON(filepath.Join(overInfoDir, "texts.json"), results)
		_ = writeJSON(filepath.Join(overInfoDir, "failed_texts.json"), failed)
		fmt.Fprintf(os.Stderr, "  done ok=%d fail=%d\n", ok, fail)
	}

	_ = writeJSON(filepath.Join(root, "failed_texts.json"), allFailed)
	if len(allFailed) > 0 {
		fmt.Fprintf(os.Stderr, "FAILED TEXTS (%d) → %s\n", len(allFailed), filepath.Join(root, "failed_texts.json"))
		for _, f := range allFailed {
			fmt.Fprintf(os.Stderr, "  • %s / %s\n", f.RegNumber, f.SourceName)
		}
	}
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

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(1)
}
