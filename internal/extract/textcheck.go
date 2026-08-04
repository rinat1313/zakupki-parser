package extract

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

// MinUsefulRunes — ниже этого после очистки считаем текст «пустым» (скан / сбой).
const MinUsefulRunes = 20

// UsefulRuneCount считает «полезные» руны (без пробелов и управляющих, в т.ч. \f от pdftotext).
func UsefulRuneCount(b []byte) int {
	n := 0
	for _, r := range string(b) {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			continue
		}
		n++
	}
	return n
}

// IsEmptyText — файл почти пустой / только form-feed.
func IsEmptyText(b []byte) bool {
	return UsefulRuneCount(b) < MinUsefulRunes
}

// ValidateTextFile читает .txt и проверяет полезный объём.
func ValidateTextFile(txtPath string) (bytes int, useful int, err error) {
	b, err := os.ReadFile(txtPath)
	if err != nil {
		return 0, 0, err
	}
	return len(b), UsefulRuneCount(b), nil
}

func markEmpty(res *Result, detail string) {
	res.Error = detail
	if res.TextPath != "" {
		_ = os.Remove(res.TextPath) // не оставляем мусор \f\f как «успех»
	}
	res.Bytes = 0
}

func checkUsefulOrFail(res *Result) Result {
	b, err := os.ReadFile(res.TextPath)
	if err != nil {
		res.Error = "txt missing after convert: " + err.Error()
		res.Bytes = 0
		return *res
	}
	useful := UsefulRuneCount(b)
	res.Bytes = len(b)
	if useful < MinUsefulRunes {
		markEmpty(res, fmt.Sprintf(
			"empty/unreadable text (raw=%d bytes, useful_runes=%d < %d)%s",
			len(b), useful, MinUsefulRunes, detailHint(res.Engine)))
		return *res
	}
	res.Error = ""
	return *res
}

func detailHint(engine string) string {
	switch engine {
	case "pdftotext", "ocr", "libreoffice":
		return "; вероятно скан/картинка без текстового слоя — нужен OCR или ручной разбор"
	default:
		return ""
	}
}

// tryPDFOCR: pdftoppm → tesseract (rus+eng) по страницам.
func tryPDFOCR(pdfPath, txtPath string) Result {
	res := Result{SourcePath: pdfPath, TextPath: txtPath, Engine: "ocr"}

	pdftoppm, err := exec.LookPath("pdftoppm")
	if err != nil {
		res.Error = "pdftoppm not found (brew install poppler)"
		return res
	}
	tesseract, err := exec.LookPath("tesseract")
	if err != nil {
		res.Error = "tesseract not found (brew install tesseract tesseract-lang)"
		return res
	}

	tmp, err := os.MkdirTemp("", "eis-ocr-*")
	if err != nil {
		res.Error = err.Error()
		return res
	}
	defer os.RemoveAll(tmp)

	prefix := filepath.Join(tmp, "page")
	cmd := exec.Command(pdftoppm, "-png", "-r", "200", pdfPath, prefix)
	if out, err := cmd.CombinedOutput(); err != nil {
		res.Error = fmt.Sprintf("pdftoppm: %v (%s)", err, strings.TrimSpace(string(out)))
		return res
	}

	entries, _ := os.ReadDir(tmp)
	var pages []string
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, "page") && strings.HasSuffix(strings.ToLower(name), ".png") {
			pages = append(pages, filepath.Join(tmp, name))
		}
	}
	sort.Strings(pages)
	if len(pages) == 0 {
		res.Error = "pdftoppm produced no pages"
		return res
	}

	var parts []string
	lang := "rus+eng"
	for i, page := range pages {
		outBase := filepath.Join(tmp, fmt.Sprintf("ocr-%d", i))
		cmd := exec.Command(tesseract, page, outBase, "-l", lang, "--psm", "3")
		if out, err := cmd.CombinedOutput(); err != nil {
			// fallback eng only
			cmd = exec.Command(tesseract, page, outBase, "-l", "eng", "--psm", "3")
			if out2, err2 := cmd.CombinedOutput(); err2 != nil {
				res.Error = fmt.Sprintf("tesseract page %d: %v (%s; %s)", i+1, err2,
					strings.TrimSpace(string(out)), strings.TrimSpace(string(out2)))
				return res
			}
		}
		b, err := os.ReadFile(outBase + ".txt")
		if err != nil {
			continue
		}
		parts = append(parts, string(b))
	}

	joined := strings.TrimSpace(strings.Join(parts, "\n\n"))
	if err := os.WriteFile(txtPath, []byte(joined+"\n"), 0o644); err != nil {
		res.Error = err.Error()
		return res
	}
	return checkUsefulOrFail(&res)
}
