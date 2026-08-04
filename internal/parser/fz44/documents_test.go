package fz44_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rinat1313/zakupki-parser/internal/parser/fz44"
)

func TestParseDocumentsFixture(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "html", "44 фз", "Сведения закупки документы 44 фз.html")
	html, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	docs, err := fz44.ParseDocumentsHTML(html)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) < 5 {
		t.Fatalf("want >=5 unique files, got %d", len(docs))
	}
	var hasRAR bool
	for _, d := range docs {
		t.Logf("%s %s", d.UID, d.Filename)
		if filepath.Ext(d.Filename) == ".rar" {
			hasRAR = true
		}
	}
	if !hasRAR {
		t.Fatal("expected rar in fixture")
	}
}
