package pricereq_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rinat1313/zakupki-parser/internal/parser/pricereq"
)

func TestParseCommonInfoCard(t *testing.T) {
	html, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "pricereq", "common-info.html"))
	if err != nil {
		t.Fatal(err)
	}
	n, err := pricereq.ParseCommonInfoHTML(html, "https://zakupki.gov.ru/epz/pricereq/card/common-info.html?reestrNumber=0303200000126000028")
	if err != nil {
		t.Fatal(err)
	}
	if n.ReestrNumber != "0303200000126000028" {
		t.Fatalf("reestr=%q", n.ReestrNumber)
	}
	if n.Status != "Подача предложений" {
		t.Fatalf("status=%q", n.Status)
	}
	if n.OrganizationCode != "03032000001" {
		t.Fatalf("orgCode=%q", n.OrganizationCode)
	}
	if n.PriceRequestInfoID != "3245926" {
		t.Fatalf("infoId=%q", n.PriceRequestInfoID)
	}
	if n.PriceInfoFrom.Raw == "" || n.PriceInfoTo.Raw == "" {
		t.Fatalf("price period empty: %+v %+v", n.PriceInfoFrom, n.PriceInfoTo)
	}
	if len(n.Items) != 1 || n.Items[0].Code != "62.02" {
		t.Fatalf("items=%+v", n.Items)
	}
	if n.Items[0].Name == "" {
		t.Fatal("item name empty")
	}
	if n.ContactEmail == "" {
		t.Fatal("email empty")
	}
}

func TestParseSearchResults(t *testing.T) {
	html, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "pricereq", "search-results.html"))
	if err != nil {
		t.Fatal(err)
	}
	n, err := pricereq.ParseCommonInfoHTML(html, "https://zakupki.gov.ru/epz/pricereq/search/results.html")
	if err != nil {
		t.Fatal(err)
	}
	if n.ReestrNumber != "0303200000126000028" {
		t.Fatalf("reestr=%q", n.ReestrNumber)
	}
	if n.OrganizationCode != "03032000001" {
		t.Fatalf("orgCode=%q", n.OrganizationCode)
	}
	if n.ObjectName == "" {
		t.Fatal("object empty")
	}
	if n.PriceRequestInfoID != "3245926" {
		t.Fatalf("infoId=%q", n.PriceRequestInfoID)
	}
	if n.PriceInfoPeriodRaw == "" {
		t.Fatal("period raw empty")
	}
}
