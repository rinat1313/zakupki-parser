package fz44_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rinat1313/zakupki-parser/internal/parser/fz44"
)

func TestParseNoticeFixture(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "html", "44 фз", "Сведения закупки 44 фз.html")
	html, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	n, err := fz44.ParseNoticeHTML(html, "https://zakupki.gov.ru/epz/order/notice/ea20/view/common-info.html?regNumber=0432200000826003978")
	if err != nil {
		t.Fatal(err)
	}
	if n.RegNumber != "0432200000826003978" {
		t.Fatalf("reg: %q", n.RegNumber)
	}
	if n.NMCK.Amount < 7000000 || n.NMCK.Amount > 7100000 {
		t.Fatalf("nmck: %+v", n.NMCK)
	}
	if n.ContactEmail != "ozp@gpnof.ru" {
		t.Fatalf("email: %q", n.ContactEmail)
	}
	if n.OrganizationCode != "04322000008" {
		t.Fatalf("org: %q", n.OrganizationCode)
	}
	if n.ObjectName == "" {
		t.Fatal("empty object name")
	}
	t.Logf("OK %s %s %.2f %s", n.RegNumber, n.ObjectName, n.NMCK.Amount, n.ContactEmail)
}

func TestParseOrgFixture(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "html", "44 фз", "Регистрационные данные организации учетная карточка 44 фз.html")
	html, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	inn, kpp, name, err := fz44.ParseOrganizationHTML(html)
	if err != nil {
		t.Fatal(err)
	}
	if inn != "6612008825" {
		t.Fatalf("inn=%q", inn)
	}
	if kpp != "661201001" {
		t.Fatalf("kpp=%q", kpp)
	}
	if name == "" {
		t.Fatal("empty name")
	}
	t.Logf("OK INN=%s KPP=%s name=%s", inn, kpp, name)
}
