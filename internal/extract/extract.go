package extract

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"golang.org/x/text/encoding/charmap"
)

// SupportedExtensions — форматы, которые умеем гнать в txt.
var SupportedExtensions = map[string]bool{
	".doc":  true,
	".docx": true,
	".odt":  true,
	".rtf":  true,
	".xls":  true,
	".xlsx": true,
	".ods":  true,
	".csv":  true,
	".pdf":  true,
	".txt":  true,
	".xml":  true,
	".html": true,
	".htm":  true,
}

// Result результат конвертации одного файла.
type Result struct {
	SourcePath string // абсолютный путь к исходному
	TextPath   string // абсолютный путь к .txt
	Bytes      int
	Engine     string // libreoffice | pdftotext | ocr | copy | skip
	Error      string
}

var (
	sofficeOnce sync.Once
	sofficePath string
	sofficeMu   sync.Mutex
)

// FindSoffice ищет бинарник LibreOffice.
func FindSoffice() string {
	sofficeOnce.Do(func() {
		candidates := []string{
			os.Getenv("SOFFICE_PATH"),
			"/Applications/LibreOffice.app/Contents/MacOS/soffice",
			"/usr/bin/soffice",
			"/usr/local/bin/soffice",
			"/opt/homebrew/bin/soffice",
			"soffice",
		}
		for _, c := range candidates {
			if c == "" {
				continue
			}
			if filepath.Base(c) == c {
				if p, err := exec.LookPath(c); err == nil {
					sofficePath = p
					return
				}
				continue
			}
			if st, err := os.Stat(c); err == nil && !st.IsDir() {
				sofficePath = c
				return
			}
		}
	})
	return sofficePath
}

// TextPathFor: «Проект контракта.docx» → «Проект контракта.txt» в той же папке.
func TextPathFor(sourcePath string) string {
	dir := filepath.Dir(sourcePath)
	base := filepath.Base(sourcePath)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	if name == "" {
		name = base
	}
	return filepath.Join(dir, name+".txt")
}

// TextPathMapped: origin/a/b.docx → valid_doc/a/b.txt
func TextPathMapped(sourcePath, originRoot, validRoot string) (string, error) {
	rel, err := filepath.Rel(originRoot, sourcePath)
	if err != nil {
		return "", err
	}
	ext := filepath.Ext(rel)
	name := strings.TrimSuffix(rel, ext)
	if name == "" {
		name = rel
	}
	return filepath.Join(validRoot, name+".txt"), nil
}

// ToText конвертирует файл в соседний .txt для LLM.
func ToText(sourcePath string) Result {
	return ToTextOut(sourcePath, TextPathFor(sourcePath))
}

// ToTextOut конвертирует sourcePath в указанный txtPath.
func ToTextOut(sourcePath, txtPath string) Result {
	abs, err := filepath.Abs(sourcePath)
	if err != nil {
		return Result{SourcePath: sourcePath, Error: err.Error()}
	}
	txtAbs, err := filepath.Abs(txtPath)
	if err != nil {
		return Result{SourcePath: abs, Error: err.Error()}
	}
	res := Result{SourcePath: abs, TextPath: txtAbs}
	ext := strings.ToLower(filepath.Ext(abs))

	if !SupportedExtensions[ext] {
		res.Engine = "skip"
		res.Error = "unsupported extension: " + ext
		return res
	}

	if err := os.MkdirAll(filepath.Dir(txtAbs), 0o755); err != nil {
		res.Error = err.Error()
		return res
	}

	if ext == ".txt" {
		if abs == txtAbs {
			res.Engine = "copy"
			if st, err := os.Stat(abs); err == nil {
				res.Bytes = int(st.Size())
			}
			return res
		}
		b, err := os.ReadFile(abs)
		if err != nil {
			res.Error = err.Error()
			return res
		}
		b = decodeToUTF8(b)
		if err := os.WriteFile(txtAbs, b, 0o644); err != nil {
			res.Error = err.Error()
			return res
		}
		res.Engine = "copy"
		res.Bytes = len(b)
		return checkUsefulOrFail(&res)
	}

	if ext == ".xml" || ext == ".html" || ext == ".htm" || ext == ".csv" {
		b, err := os.ReadFile(abs)
		if err != nil {
			res.Error = err.Error()
			return res
		}
		b = decodeToUTF8(b)
		if err := os.WriteFile(txtAbs, b, 0o644); err != nil {
			res.Error = err.Error()
			return res
		}
		res.Engine = "copy"
		res.Bytes = len(b)
		return checkUsefulOrFail(&res)
	}

	if ext == ".pdf" {
		if r := tryPDFToText(abs, txtAbs); r.Error == "" {
			return r
		} else {
			res.Error = r.Error
		}
		if r := tryPDFOCR(abs, txtAbs); r.Error == "" {
			return r
		} else {
			if res.Error != "" {
				res.Error = res.Error + "; ocr: " + r.Error
			} else {
				res.Error = "ocr: " + r.Error
			}
		}
	}

	// DOCX: сначала native ZIP/XML (полное тело + таблицы), затем LibreOffice; берём более полный.
	if ext == ".docx" {
		native, xmlRunes, _ := writeNativeDOCX(abs, txtAbs+".native.txt")
		loRes := Result{SourcePath: abs, TextPath: txtAbs + ".lo.txt"}
		if err := convertLibreOffice(abs, loRes.TextPath, ext); err != nil {
			loRes.Error = err.Error()
			loRes.Engine = "libreoffice"
		} else {
			loRes.Engine = "libreoffice"
			loRes = checkUsefulOrFail(&loRes)
		}
		best := pickRicher(native, loRes)
		if best.Error == "" {
			b, _ := os.ReadFile(best.TextPath)
			_ = os.WriteFile(txtAbs, b, 0o644)
			_ = os.Remove(txtAbs + ".native.txt")
			_ = os.Remove(txtAbs + ".lo.txt")
			out := Result{SourcePath: abs, TextPath: txtAbs, Engine: best.Engine, Bytes: len(b)}
			if xmlRunes > 0 && !CoverageOK(string(b), xmlRunes, 0.90) {
				// Повторно предпочитаем native, если LO обрезал.
				if native.Error == "" {
					nb, _ := os.ReadFile(native.TextPath)
					if UsefulRuneCount(nb) > UsefulRuneCount(b) {
						_ = os.WriteFile(txtAbs, nb, 0o644)
						out.Engine = "docx-native"
						out.Bytes = len(nb)
						b = nb
					}
				}
				if !CoverageOK(string(b), xmlRunes, 0.90) {
					out.Error = fmt.Sprintf("low coverage: useful=%d xml_runes=%d (<90%%); engine=%s",
						UsefulRuneCount(b), xmlRunes, out.Engine)
					// всё равно оставляем лучший текст — лучше частичный, чем ничего
					out.Error = ""
				}
			}
			return checkUsefulOrFail(&out)
		}
		if native.Error != "" && loRes.Error != "" {
			res.Error = "docx-native: " + native.Error + "; libreoffice: " + loRes.Error
			res.Engine = "docx"
			return res
		}
	}

	// DOC (binary): LibreOffice → DOCX → native XML extract; плюс прямой LO txt; берём лучшее.
	if ext == ".doc" || ext == ".rtf" || ext == ".odt" {
		if richer, err := convertViaDOCXNative(abs, txtAbs, ext); err == nil && richer.Error == "" {
			return richer
		}
	}

	if err := convertLibreOffice(abs, txtAbs, ext); err != nil {
		if res.Error != "" {
			res.Error = res.Error + "; libreoffice: " + err.Error()
		} else {
			res.Error = err.Error()
		}
		res.Engine = "libreoffice"
		return res
	}
	res.Engine = "libreoffice"
	return checkUsefulOrFail(&res)
}

func tryPDFToText(pdfPath, txtPath string) Result {
	res := Result{SourcePath: pdfPath, TextPath: txtPath, Engine: "pdftotext"}
	bin, err := exec.LookPath("pdftotext")
	if err != nil {
		res.Error = "pdftotext not found"
		return res
	}
	cmd := exec.Command(bin, "-layout", "-enc", "UTF-8", pdfPath, txtPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		res.Error = fmt.Sprintf("%v: %s", err, string(out))
		return res
	}
	return checkUsefulOrFail(&res)
}

func convertLibreOffice(sourcePath, finalTxtPath, ext string) error {
	so := FindSoffice()
	if so == "" {
		return fmt.Errorf("LibreOffice (soffice) not found; set SOFFICE_PATH")
	}
	sofficeMu.Lock()
	defer sofficeMu.Unlock()
	return convertLibreOfficeLocked(sourcePath, finalTxtPath, ext)
}

// convertViaDOCXNative: DOC/RTF/ODT → DOCX через LO → native XML extract + LO txt, выбрать полнее.
func convertViaDOCXNative(sourcePath, finalTxtPath, ext string) (Result, error) {
	so := FindSoffice()
	if so == "" {
		return Result{}, fmt.Errorf("LibreOffice not found")
	}
	sofficeMu.Lock()
	defer sofficeMu.Unlock()

	tmpRoot, err := os.MkdirTemp("", "eis-lo-docx-*")
	if err != nil {
		return Result{}, err
	}
	defer os.RemoveAll(tmpRoot)
	workDir := filepath.Join(tmpRoot, "work")
	outDir := filepath.Join(tmpRoot, "out")
	profile := filepath.Join(tmpRoot, "profile")
	_ = os.MkdirAll(workDir, 0o755)
	_ = os.MkdirAll(outDir, 0o755)
	_ = os.MkdirAll(profile, 0o755)

	inName := "input" + ext
	inPath := filepath.Join(workDir, inName)
	src, err := os.ReadFile(sourcePath)
	if err != nil {
		return Result{}, err
	}
	if err := os.WriteFile(inPath, src, 0o644); err != nil {
		return Result{}, err
	}

	profileURL := "file://" + profile
	cmd := exec.Command(so,
		"-env:UserInstallation="+profileURL,
		"--headless", "--norestore", "--nolockcheck",
		"--convert-to", "docx",
		"--outdir", outDir,
		inPath,
	)
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(), "LANG=C.UTF-8", "LC_ALL=C.UTF-8", "LC_CTYPE=C.UTF-8")
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Run(); err != nil {
		return Result{}, fmt.Errorf("soffice→docx: %v (%s)", err, strings.TrimSpace(buf.String()))
	}
	docxPath := filepath.Join(outDir, "input.docx")
	if _, err := os.Stat(docxPath); err != nil {
		// найти любой docx в outDir
		_ = filepath.WalkDir(outDir, func(path string, d os.DirEntry, err error) error {
			if err == nil && !d.IsDir() && strings.EqualFold(filepath.Ext(d.Name()), ".docx") {
				docxPath = path
			}
			return nil
		})
	}
	if _, err := os.Stat(docxPath); err != nil {
		return Result{}, fmt.Errorf("soffice→docx: no docx output (%s)", strings.TrimSpace(buf.String()))
	}

	nativePath := finalTxtPath + ".via-docx.txt"
	native, _, nerr := writeNativeDOCX(docxPath, nativePath)

	loTxt := finalTxtPath + ".lo.txt"
	loRes := Result{SourcePath: sourcePath, TextPath: loTxt, Engine: "libreoffice"}
	// отдельный txt export (нужен повторный LO — вызывающий уже держит lock; вызываем внутренне без lock)
	if err := convertLibreOfficeLocked(sourcePath, loTxt, ext); err != nil {
		loRes.Error = err.Error()
	} else {
		loRes = checkUsefulOrFail(&loRes)
	}
	_ = nerr
	best := pickRicher(native, loRes)
	if best.Error != "" {
		return best, fmt.Errorf("%s", best.Error)
	}
	b, err := os.ReadFile(best.TextPath)
	if err != nil {
		return best, err
	}
	if err := os.WriteFile(finalTxtPath, b, 0o644); err != nil {
		return best, err
	}
	_ = os.Remove(nativePath)
	_ = os.Remove(loTxt)
	out := Result{SourcePath: sourcePath, TextPath: finalTxtPath, Engine: best.Engine + "+via-docx", Bytes: len(b)}
	return checkUsefulOrFail(&out), nil
}

// convertLibreOfficeLocked — как convertLibreOffice, но без взятия sofficeMu (уже удерживается).
func convertLibreOfficeLocked(sourcePath, finalTxtPath, ext string) error {
	so := FindSoffice()
	if so == "" {
		return fmt.Errorf("LibreOffice (soffice) not found; set SOFFICE_PATH")
	}
	tmpRoot, err := os.MkdirTemp("", "eis-lo-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpRoot)
	workDir := filepath.Join(tmpRoot, "work")
	outDir := filepath.Join(tmpRoot, "out")
	profile := filepath.Join(tmpRoot, "profile")
	_ = os.MkdirAll(workDir, 0o755)
	_ = os.MkdirAll(outDir, 0o755)
	_ = os.MkdirAll(profile, 0o755)
	inName := "input" + ext
	inPath := filepath.Join(workDir, inName)
	src, err := os.ReadFile(sourcePath)
	if err != nil {
		return err
	}
	if err := os.WriteFile(inPath, src, 0o644); err != nil {
		return err
	}
	filter := "txt:Text (encoded):UTF8"
	wantExt := ".txt"
	switch ext {
	case ".xls", ".xlsx", ".ods":
		filter = `csv:Text - txt - csv (StarCalc):44,34,76,1`
		wantExt = ".csv"
	}
	profileURL := "file://" + profile
	cmd := exec.Command(so,
		"-env:UserInstallation="+profileURL,
		"--headless", "--norestore", "--nolockcheck",
		"--convert-to", filter,
		"--outdir", outDir,
		inPath,
	)
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(), "LANG=C.UTF-8", "LC_ALL=C.UTF-8", "LC_CTYPE=C.UTF-8")
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("soffice: %v (%s)", err, strings.TrimSpace(buf.String()))
	}
	data, err := readConvertedOutput(outDir, filepath.Join(outDir, "input"+wantExt), wantExt)
	if err != nil {
		return fmt.Errorf("%w; soffice out: %s", err, strings.TrimSpace(buf.String()))
	}
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	data = decodeToUTF8(data)
	if looksLikeEncodingLoss(data) {
		return fmt.Errorf("libreoffice produced encoding-loss text (Cyrillic became ?); check UTF-8 filter/locale")
	}
	return os.WriteFile(finalTxtPath, data, 0o644)
}

// decodeToUTF8 нормализует текст в UTF-8 (BOM / Windows-1251 для старых CSV).
func decodeToUTF8(b []byte) []byte {
	b = bytes.TrimPrefix(b, []byte{0xEF, 0xBB, 0xBF})
	if utf8.Valid(b) {
		return b
	}
	if s, err := charmap.Windows1251.NewDecoder().Bytes(b); err == nil && utf8.Valid(s) {
		return s
	}
	return b
}

func readConvertedOutput(outDir, preferred, wantExt string) ([]byte, error) {
	if st, err := os.Stat(preferred); err == nil && !st.IsDir() && st.Size() > 0 {
		return os.ReadFile(preferred)
	}
	var parts [][]byte
	_ = filepath.WalkDir(outDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(d.Name()))
		if wantExt != "" && ext != wantExt && ext != ".txt" && ext != ".csv" {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil || len(bytes.TrimSpace(b)) == 0 {
			return nil
		}
		parts = append(parts, b)
		return nil
	})
	if len(parts) == 0 {
		return nil, fmt.Errorf("read converted: no output in %s (expected %s)", outDir, preferred)
	}
	return bytes.Join(parts, []byte("\n\n")), nil
}

// Dir обходит originDir и пишет .txt в validDocDir с той же относительной структурой.
// Если validDocDir пуст — txt рядом с исходником (старое поведение).
func Dir(originDir string, validDocDir ...string) []Result {
	validRoot := ""
	if len(validDocDir) > 0 {
		validRoot = validDocDir[0]
	}
	var results []Result
	_ = filepath.WalkDir(originDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".txt" {
			return nil
		}
		if !SupportedExtensions[ext] {
			return nil
		}
		var r Result
		if validRoot != "" {
			out, mapErr := TextPathMapped(path, originDir, validRoot)
			if mapErr != nil {
				results = append(results, Result{SourcePath: path, Error: mapErr.Error()})
				return nil
			}
			r = ToTextOut(path, out)
		} else {
			r = ToText(path)
		}
		results = append(results, r)
		if r.Engine == "libreoffice" || r.Engine == "ocr" {
			time.Sleep(300 * time.Millisecond)
		}
		return nil
	})
	return results
}
