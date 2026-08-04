package extract

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractDOCXNative(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sample.docx")
	if err := writeMinimalDOCX(path, "Страница один. ", "Таблица два и длинный текст для покрытия. "); err != nil {
		t.Fatal(err)
	}
	text, runes, err := ExtractDOCXNative(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "Страница один") {
		t.Fatalf("missing text: %q", text)
	}
	if !strings.Contains(text, "Таблица два") {
		t.Fatalf("missing table text: %q", text)
	}
	if runes < 10 {
		t.Fatalf("xml runes too small: %d", runes)
	}
	if !CoverageOK(text, runes, 0.90) {
		t.Fatalf("coverage failed useful=%d xml=%d", UsefulRuneCount([]byte(text)), runes)
	}
}

func writeMinimalDOCX(path string, parts ...string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	contentTypes := `[Content_Types].xml`
	w, _ := zw.Create(contentTypes)
	_, _ = w.Write([]byte(`<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"></Types>`))
	var body strings.Builder
	body.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	body.WriteString(`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>`)
	for _, p := range parts {
		body.WriteString(`<w:p><w:r><w:t>`)
		body.WriteString(p)
		body.WriteString(`</w:t></w:r></w:p>`)
	}
	body.WriteString(`<w:tbl><w:tr><w:tc><w:p><w:r><w:t>ячейка таблицы</w:t></w:r></w:p></w:tc></w:tr></w:tbl>`)
	body.WriteString(`</w:body></w:document>`)
	w, _ = zw.Create("word/document.xml")
	_, _ = w.Write([]byte(body.String()))
	return zw.Close()
}
