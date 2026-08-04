package extract

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUsefulRuneCountEmptyFormFeed(t *testing.T) {
	if UsefulRuneCount([]byte{0x0c, 0x0c}) != 0 {
		t.Fatal("form-feed should be empty")
	}
	if UsefulRuneCount([]byte("  \n\t")) != 0 {
		t.Fatal("whitespace empty")
	}
	if !IsEmptyText([]byte{0x0c}) {
		t.Fatal("expected empty")
	}
	if IsEmptyText([]byte("Это нормальный текст документа для проверки")) {
		t.Fatal("expected non-empty")
	}
}

func TestScanPDFToTextOCR(t *testing.T) {
	pdf := filepath.Join("..", "..", "result", "32312323655", "files", "i.pdf")
	if _, err := os.Stat(pdf); err != nil {
		t.Skip("sample scan pdf not present:", pdf)
	}
	tmp := t.TempDir()
	dst := filepath.Join(tmp, "i.pdf")
	b, err := os.ReadFile(pdf)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, b, 0o644); err != nil {
		t.Fatal(err)
	}
	r := ToText(dst)
	if r.Error != "" {
		t.Fatalf("ToText: %s (engine=%s)", r.Error, r.Engine)
	}
	if r.Engine != "ocr" && r.Engine != "pdftotext" {
		t.Fatalf("unexpected engine %s", r.Engine)
	}
	if r.Bytes < 100 {
		t.Fatalf("too small txt: %d", r.Bytes)
	}
	out, _ := os.ReadFile(r.TextPath)
	if IsEmptyText(out) {
		t.Fatal("still empty after convert")
	}
}
