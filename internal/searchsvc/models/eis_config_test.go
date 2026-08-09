package models

import (
	"strings"
	"testing"
)

func TestQueryValuesRecordsPerPageAndPPRF615(t *testing.T) {
	cfg := DefaultSearcherConfig()
	cfg.SearchString = "тест"
	cfg.RecordsPerPage = 50
	cfg.PPRF615 = true
	cfg.FZ44 = false
	cfg.FZ223 = false

	q := cfg.QueryValues()
	if got := q.Get("recordsPerPage"); got != "_50" {
		t.Fatalf("recordsPerPage: want _50, got %q", got)
	}
	if got := q.Get("ppRf615"); got != "on" {
		t.Fatalf("ppRf615: want on, got %q", got)
	}
}

func TestRecordsPerPageUnmarshal(t *testing.T) {
	cases := []struct {
		raw  string
		want RecordsPerPage
		eis  string
	}{
		{`50`, 50, "_50"},
		{`"50"`, 50, "_50"},
		{`"_50"`, 50, "_50"},
		{`20`, 20, "_20"},
		{`"_10"`, 10, "_10"},
	}
	for _, tc := range cases {
		var r RecordsPerPage
		if err := r.UnmarshalJSON([]byte(tc.raw)); err != nil {
			t.Fatalf("unmarshal %s: %v", tc.raw, err)
		}
		if r != tc.want {
			t.Fatalf("%s: want %d got %d", tc.raw, tc.want, r)
		}
		if r.EIS() != tc.eis {
			t.Fatalf("%s EIS: want %s got %s", tc.raw, tc.eis, r.EIS())
		}
	}
}

func TestResultsURLContainsCoreParams(t *testing.T) {
	cfg := DefaultSearcherConfig()
	cfg.SearchString = "Разработка ПО"
	cfg.FZ223 = false
	cfg.RecordsPerPage = 10
	u := cfg.ResultsURL("https://zakupki.gov.ru")
	if !strings.Contains(u, "/epz/order/extendedsearch/results.html?") {
		t.Fatalf("path: %s", u)
	}
	for _, want := range []string{
		"searchString=",
		"fz44=on",
		"af=on",
		"recordsPerPage=_10",
		"sortBy=UPDATE_DATE",
	} {
		if !strings.Contains(u, want) {
			t.Fatalf("missing %q in %s", want, u)
		}
	}
	if strings.Contains(u, "fz223=on") {
		t.Fatalf("fz223 should be off: %s", u)
	}
	if strings.Contains(u, "ppRf615=on") {
		t.Fatalf("ppRf615 should be off: %s", u)
	}
}

func TestStrictEqualDisablesMorphologyFlag(t *testing.T) {
	cfg := DefaultSearcherConfig()
	cfg.SearchString = "0134300097526000797"
	cfg.StrictEqual = true
	q := cfg.QueryValues()
	if q.Get("strictEqual") != "true" {
		t.Fatal("strictEqual")
	}
	if q.Get("morphology") != "" {
		t.Fatalf("morphology should be empty, got %q", q.Get("morphology"))
	}
}
