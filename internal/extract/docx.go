package extract

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	reXMLTag   = regexp.MustCompile(`(?s)<[^>]+>`)
	reMultiWS  = regexp.MustCompile(`[ \t\r\f]+`)
	reMultiNL  = regexp.MustCompile(`\n{3,}`)
	reWTOpen   = regexp.MustCompile(`(?i)<w:t(\s[^>]*)?>`)
)

// ExtractDOCXNative читает DOCX как ZIP и собирает весь текст из document/header/footer/notes.
// Это надёжнее LibreOffice Text-экспорта для длинных ТЗ с таблицами.
func ExtractDOCXNative(path string) (string, int, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return "", 0, err
	}
	defer zr.Close()

	var parts []string
	var xmlTextRunes int
	for _, f := range zr.File {
		name := strings.ToLower(f.Name)
		if name == "word/document.xml" ||
			strings.HasPrefix(name, "word/header") && strings.HasSuffix(name, ".xml") ||
			strings.HasPrefix(name, "word/footer") && strings.HasSuffix(name, ".xml") ||
			strings.HasPrefix(name, "word/footnotes") && strings.HasSuffix(name, ".xml") ||
			strings.HasPrefix(name, "word/endnotes") && strings.HasSuffix(name, ".xml") {
			rc, err := f.Open()
			if err != nil {
				continue
			}
			raw, err := io.ReadAll(io.LimitReader(rc, 64<<20))
			rc.Close()
			if err != nil || len(raw) == 0 {
				continue
			}
			text, runesInXML := xmlOfficeText(raw)
			xmlTextRunes += runesInXML
			text = strings.TrimSpace(text)
			if text != "" {
				parts = append(parts, text)
			}
		}
	}
	if len(parts) == 0 {
		return "", 0, fmt.Errorf("docx: no text parts in zip")
	}
	out := strings.Join(parts, "\n\n")
	out = cleanupOfficeText(out)
	if UsefulRuneCount([]byte(out)) < MinUsefulRunes {
		return "", xmlTextRunes, fmt.Errorf("docx: extracted text too short")
	}
	return out, xmlTextRunes, nil
}

// CoverageOK — извлечённый текст покрывает ≥ minRatio полезных рун относительно XML.
func CoverageOK(extracted string, xmlTextRunes int, minRatio float64) bool {
	if xmlTextRunes < 200 {
		return true // мало текста в XML — не из чего судить
	}
	got := UsefulRuneCount([]byte(extracted))
	return float64(got) >= float64(xmlTextRunes)*minRatio
}

func xmlOfficeText(raw []byte) (text string, textRunesInTags int) {
	// Считаем руны внутри w:t / text:p для оценки покрытия.
	dec := xml.NewDecoder(bytes.NewReader(raw))
	dec.Strict = false
	dec.AutoClose = xml.HTMLAutoClose
	dec.Entity = xml.HTMLEntity

	var b strings.Builder
	var inT bool
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			local := strings.ToLower(t.Name.Local)
			switch local {
			case "t": // w:t
				inT = true
			case "tab":
				b.WriteByte('\t')
			case "br", "cr":
				b.WriteByte('\n')
			case "p", "h":
				if b.Len() > 0 {
					b.WriteByte('\n')
				}
			}
		case xml.EndElement:
			local := strings.ToLower(t.Name.Local)
			if local == "t" {
				inT = false
			}
			if local == "p" || local == "tr" {
				b.WriteByte('\n')
			}
		case xml.CharData:
			if inT {
				s := string([]byte(t))
				textRunesInTags += utf8.RuneCountInString(s)
				b.WriteString(s)
			}
		}
	}
	// Fallback: если decoder ничего не дал — грубый strip тегов по w:t.
	if textRunesInTags == 0 {
		return fallbackStripOOXML(raw)
	}
	return b.String(), textRunesInTags
}

func fallbackStripOOXML(raw []byte) (string, int) {
	s := string(raw)
	var b strings.Builder
	runes := 0
	for _, m := range reWTOpen.FindAllStringIndex(s, -1) {
		start := strings.Index(s[m[1]:], ">")
		if start < 0 {
			continue
		}
		from := m[1] + start + 1
		end := strings.Index(strings.ToLower(s[from:]), "</w:t>")
		if end < 0 {
			continue
		}
		chunk := s[from : from+end]
		chunk = reXMLTag.ReplaceAllString(chunk, "")
		chunk = xmlUnescape(chunk)
		runes += utf8.RuneCountInString(chunk)
		b.WriteString(chunk)
	}
	out := b.String()
	if out == "" {
		// last resort: strip all tags
		out = reXMLTag.ReplaceAllString(s, " ")
		out = xmlUnescape(out)
		runes = UsefulRuneCount([]byte(out))
	}
	return out, runes
}

func xmlUnescape(s string) string {
	r := strings.NewReplacer(
		"&lt;", "<",
		"&gt;", ">",
		"&amp;", "&",
		"&quot;", `"`,
		"&apos;", "'",
		"&#39;", "'",
	)
	return r.Replace(s)
}

func cleanupOfficeText(s string) string {
	s = strings.ReplaceAll(s, "\u00a0", " ")
	s = reMultiWS.ReplaceAllString(s, " ")
	lines := strings.Split(s, "\n")
	for i, ln := range lines {
		lines[i] = strings.TrimRightFunc(ln, unicode.IsSpace)
	}
	s = strings.Join(lines, "\n")
	s = reMultiNL.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}

// writeNativeDOCX пишет native-extract в txtPath.
func writeNativeDOCX(sourcePath, txtPath string) (Result, int, error) {
	res := Result{SourcePath: sourcePath, TextPath: txtPath, Engine: "docx-native"}
	text, xmlRunes, err := ExtractDOCXNative(sourcePath)
	if err != nil {
		res.Error = err.Error()
		return res, xmlRunes, err
	}
	if err := os.WriteFile(txtPath, []byte(text+"\n"), 0o644); err != nil {
		res.Error = err.Error()
		return res, xmlRunes, err
	}
	res = checkUsefulOrFail(&res)
	return res, xmlRunes, nil
}

// pickRicher выбирает результат с большим полезным объёмом текста.
func pickRicher(a, b Result) Result {
	if a.Error != "" && b.Error == "" {
		return b
	}
	if b.Error != "" && a.Error == "" {
		return a
	}
	if a.Error != "" && b.Error != "" {
		if a.Bytes >= b.Bytes {
			return a
		}
		return b
	}
	ua, ub := 0, 0
	if ba, err := os.ReadFile(a.TextPath); err == nil {
		ua = UsefulRuneCount(ba)
	}
	if bb, err := os.ReadFile(b.TextPath); err == nil {
		ub = UsefulRuneCount(bb)
	}
	if ub > ua {
		return b
	}
	return a
}

func absPath(p string) string {
	a, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	return a
}
