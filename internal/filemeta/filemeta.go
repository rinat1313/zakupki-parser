package filemeta

import (
	"bytes"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

var (
	reDispFilename = regexp.MustCompile(`(?i)filename\*?=(?:UTF-8''|")?([^";\r\n]+)`)
	reIconType     = regexp.MustCompile(`(?i)/icons/type/([a-z0-9]+)\.(?:svg|png|gif)`)
)

// knownExts — реальные расширения файлов (не путать с датами/инициалами в имени).
var knownExts = map[string]bool{
	".doc": true, ".docx": true, ".pdf": true, ".xls": true, ".xlsx": true,
	".rtf": true, ".odt": true, ".ods": true, ".csv": true,
	".zip": true, ".rar": true, ".7z": true,
	".txt": true, ".xml": true, ".html": true, ".htm": true,
	".ppt": true, ".pptx": true,
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".tif": true, ".tiff": true,
	".sig": true, ".p7s": true,
}

// IsKnownExt — true для .pdf/.docx/…; false для ".2026", ".Ю.", ". ТЕХНИЧЕСКАЯ ЧАСТЬ".
func IsKnownExt(ext string) bool {
	ext = strings.ToLower(strings.TrimSpace(ext))
	if ext == "" {
		return false
	}
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	return knownExts[ext]
}

// ExtFromContentDisposition достаёт расширение из заголовка Content-Disposition.
func ExtFromContentDisposition(h http.Header) string {
	cd := h.Get("Content-Disposition")
	if cd == "" {
		return ""
	}
	m := reDispFilename.FindStringSubmatch(cd)
	if len(m) < 2 {
		return ""
	}
	name := strings.Trim(m[1], `"' `)
	name, err := url.QueryUnescape(name)
	if err != nil {
		name = strings.Trim(m[1], `"' `)
	}
	return ExtFromFilename(name)
}

// ExtFromFilename возвращает известное расширение или "".
// filepath.Ext("отчет_27.07.2026") → ".2026" — это НЕ расширение файла.
func ExtFromFilename(name string) string {
	ext := strings.ToLower(filepath.Ext(strings.TrimSpace(name)))
	if !IsKnownExt(ext) {
		return ""
	}
	return ext
}

// ExtFromIconSrc: /epz/static/img/icons/type/docx.svg → .docx
func ExtFromIconSrc(src string) string {
	m := reIconType.FindStringSubmatch(src)
	if len(m) < 2 {
		return ""
	}
	t := strings.ToLower(m[1])
	switch t {
	case "doc", "docx", "pdf", "xls", "xlsx", "rtf", "odt", "ods", "csv",
		"zip", "rar", "7z", "txt", "xml", "html", "htm", "ppt", "pptx":
		return "." + t
	case "word":
		return ".docx"
	case "excel":
		return ".xlsx"
	case "acrobat", "adobe":
		return ".pdf"
	default:
		return ""
	}
}

// ExtFromAlt: "Microsoft Word Document" / "Adobe Acrobat Document".
func ExtFromAlt(alt string) string {
	low := strings.ToLower(alt)
	switch {
	case strings.Contains(low, "acrobat"), strings.Contains(low, "pdf"):
		return ".pdf"
	case strings.Contains(low, "word") && strings.Contains(low, "document"):
		if strings.Contains(low, "97") || strings.Contains(low, "2003") {
			return ".doc"
		}
		return ".docx"
	case strings.Contains(low, "excel"):
		if strings.Contains(low, "97") || strings.Contains(low, "2003") {
			return ".xls"
		}
		return ".xlsx"
	case strings.Contains(low, "rich text"), strings.Contains(low, "rtf"):
		return ".rtf"
	case strings.Contains(low, "zip"):
		return ".zip"
	case strings.Contains(low, "rar"):
		return ".rar"
	}
	return ""
}

// SniffExt определяет расширение по magic bytes.
func SniffExt(data []byte) string {
	if len(data) < 4 {
		return ""
	}
	trim := bytes.TrimSpace(data)
	if bytes.HasPrefix(data, []byte("%PDF")) {
		return ".pdf"
	}
	if bytes.HasPrefix(data, []byte{0xD0, 0xCF, 0x11, 0xE0}) {
		head := data
		if len(head) > 8192 {
			head = head[:8192]
		}
		low := bytes.ToLower(head)
		switch {
		case bytes.Contains(low, []byte("workbook")), bytes.Contains(low, []byte("worksheet")):
			return ".xls"
		case bytes.Contains(low, []byte("powerpoint")):
			return ".ppt"
		default:
			return ".doc"
		}
	}
	if bytes.HasPrefix(data, []byte("PK")) {
		head := data
		if len(head) > 8192 {
			head = head[:8192]
		}
		switch {
		case bytes.Contains(head, []byte("word/")):
			return ".docx"
		case bytes.Contains(head, []byte("xl/")):
			return ".xlsx"
		case bytes.Contains(head, []byte("ppt/")):
			return ".pptx"
		case bytes.Contains(head, []byte("content.xml")):
			if bytes.Contains(head, []byte("mimetype")) && bytes.Contains(head, []byte("opendocument.text")) {
				return ".odt"
			}
			return ".odt"
		default:
			return ".zip"
		}
	}
	if bytes.HasPrefix(data, []byte("Rar!")) {
		return ".rar"
	}
	if bytes.HasPrefix(data, []byte("7z\xBC\xAF\x27\x1C")) {
		return ".7z"
	}
	if bytes.HasPrefix(trim, []byte("<?xml")) {
		return ".xml"
	}
	if bytes.HasPrefix(trim, []byte("<!DOCTYPE html")) ||
		bytes.HasPrefix(trim, []byte("<html")) ||
		bytes.HasPrefix(bytes.ToLower(trim), []byte("<!doctype html")) {
		return ".html"
	}
	// простой текст / HTML-фрагмент без doctype — всё равно сохраняем как .txt для LLM
	if looksLikePlainText(trim) {
		return ".txt"
	}
	return ""
}

func looksLikePlainText(b []byte) bool {
	if len(b) == 0 {
		return false
	}
	// слишком бинарно
	nonPrint := 0
	n := len(b)
	if n > 4096 {
		n = 4096
	}
	for i := 0; i < n; i++ {
		c := b[i]
		if c == 0 {
			return false
		}
		if c < 0x09 || (c > 0x0d && c < 0x20) {
			nonPrint++
		}
	}
	return nonPrint*20 < n // <5% control chars
}

// EnsureExt добавляет известное расширение, если его ещё нет.
// Приоритет: уже есть known в name → hint → sniffed.
func EnsureExt(name, hintExt, sniffedExt string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "file"
	}
	if ExtFromFilename(name) != "" {
		return name
	}
	ext := strings.ToLower(strings.TrimSpace(hintExt))
	if !IsKnownExt(ext) {
		ext = strings.ToLower(strings.TrimSpace(sniffedExt))
	}
	if !IsKnownExt(ext) {
		return name
	}
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	// убрать хвостовые точки/подчёркивания перед добавлением расширения
	name = strings.TrimRightFunc(name, func(r rune) bool {
		return r == '.' || r == '_' || r == '-' || unicode.IsSpace(r)
	})
	if name == "" {
		name = "file"
	}
	return name + ext
}
