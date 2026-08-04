package fz223

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	candidates := []string{
		filepath.Join("..", "..", "..", "..", "html", "223 фз", name),
		filepath.Join("..", "..", "..", "html", "223 фз", name),
	}
	for _, p := range candidates {
		b, err := os.ReadFile(p)
		if err == nil {
			return b
		}
	}
	t.Fatalf("fixture not found: %s", name)
	return nil
}

func TestParseNoticeHTML(t *testing.T) {
	html := fixture(t, "Карточка закупки 223 фз.html")
	n, err := ParseNoticeHTML(html, "https://zakupki.gov.ru/epz/order/notice/notice223/common-info.html?regNumber=32616257756")
	if err != nil {
		t.Fatal(err)
	}
	if n.PurchaseNoticeNumber != "32616257756" {
		t.Fatalf("number=%q", n.PurchaseNoticeNumber)
	}
	if n.NoticeGUID != "ddfc7c5e-bfbd-463a-894e-4f0f85263e27" {
		t.Fatalf("guid=%q", n.NoticeGUID)
	}
	if n.CustomerINN != "3919003130" {
		t.Fatalf("inn=%q", n.CustomerINN)
	}
	if n.InitialPrice.Amount != 115000 {
		t.Fatalf("price=%v", n.InitialPrice.Amount)
	}
	if n.ObjectName == "" {
		t.Fatal("empty object")
	}
	if n.MethodBody == "" {
		t.Fatal("empty method body")
	}
}

func TestParseDocumentsHTML(t *testing.T) {
	html := fixture(t, "Карточка закупки документы 223 фз.html")
	docs, err := ParseDocumentsHTML(html)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) < 2 {
		t.Fatalf("docs=%d", len(docs))
	}
	found := false
	for _, d := range docs {
		if d.UID == "45907B865F85415FB3D1A467E638A7A8" {
			found = true
			if !containsExt(d.Filename, ".doc") {
				t.Fatalf("filename=%q", d.Filename)
			}
			if d.GroupTitle == "" {
				t.Fatal("empty group")
			}
		}
	}
	if !found {
		t.Fatal("expected uid not found")
	}
}

func TestParseLotListHTML(t *testing.T) {
	html := fixture(t, "Карточка закупки список лотов 223 фз.html")
	lots, err := ParseLotListHTML(html, "32616257756")
	if err != nil {
		t.Fatal(err)
	}
	if len(lots) != 1 {
		t.Fatalf("lots=%d", len(lots))
	}
	if lots[0].LotGUID != "02b91e60-1065-443b-92fc-b7e5c4bc926f" {
		t.Fatalf("guid=%q", lots[0].LotGUID)
	}
	if lots[0].Price.Amount != 115000 {
		t.Fatalf("price=%v", lots[0].Price.Amount)
	}
	if len(lots[0].OKPD2) == 0 {
		t.Fatal("empty okpd2")
	}
}

func TestParseOrganizationHTML(t *testing.T) {
	html := fixture(t, "Регистрационные данные организации учётная карточка 223 фз.html")
	org, err := ParseOrganizationHTML(html)
	if err != nil {
		t.Fatal(err)
	}
	if org.AgencyID != "190768" {
		t.Fatalf("agency=%q", org.AgencyID)
	}
	if org.INN != "3919003130" {
		t.Fatalf("inn=%q", org.INN)
	}
	if org.FullName == "" {
		t.Fatal("empty name")
	}
}

func containsExt(name, ext string) bool {
	return strings.HasSuffix(strings.ToLower(name), strings.ToLower(ext))
}
