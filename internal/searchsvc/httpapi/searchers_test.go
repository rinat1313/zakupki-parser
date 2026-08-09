package httpapi

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/rinat1313/zakupki-parser/internal/searchsvc/models"
)

func TestSearcherWriteAcceptsUIPayload(t *testing.T) {
	raw := `{
		"name": "ПО",
		"auto_ai": true,
		"config": {
			"search_string": "Разработка ПО",
			"morphology": true,
			"fz44": true,
			"fz223": true,
			"pp_rf615": false,
			"stage_af": true,
			"stage_ca": true,
			"stage_pc": false,
			"stage_pa": false,
			"appl_submission_close_date_from": "01.01.2026",
			"price_from": 1000,
			"okpd2_codes": "62.01,62.02",
			"records_per_page": 50,
			"extra_ui_field": true
		}
	}`
	var req models.SearcherWrite
	dec := json.NewDecoder(bytes.NewReader([]byte(raw)))
	if err := dec.Decode(&req); err != nil {
		t.Fatal(err)
	}
	if req.Name != "ПО" || req.AutoAI == nil || !*req.AutoAI {
		t.Fatalf("write: %+v", req)
	}
	if req.Config == nil || req.Config.SearchString != "Разработка ПО" {
		t.Fatalf("config: %+v", req.Config)
	}
	if int(req.Config.RecordsPerPage) != 50 {
		t.Fatalf("records_per_page: %v", req.Config.RecordsPerPage)
	}
	if req.Config.PPRF615 {
		t.Fatal("pp_rf615 should be false")
	}
	if req.Config.ApplCloseFrom == nil || *req.Config.ApplCloseFrom != "01.01.2026" {
		t.Fatalf("appl close: %+v", req.Config.ApplCloseFrom)
	}
}

func TestSearcherJSONRoundTripForUI(t *testing.T) {
	s := models.Searcher{
		ID:           "0fc449d6-4063-4681-b193-458999155279",
		Name:         "ПО и разработка",
		Config:       models.DefaultSearcherConfig(),
		AutoAI:       true,
		TendersCount: 3,
	}
	s.Config.SearchString = "Разработка ПО"
	s.Config.RecordsPerPage = 50
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"id", "name", "config", "auto_ai", "tenders_count", "created_at", "updated_at", "last_run_at"} {
		if _, ok := m[key]; !ok {
			t.Fatalf("missing key %s in %s", key, string(b))
		}
	}
	cfg, _ := m["config"].(map[string]any)
	if _, ok := cfg["pp_rf615"]; !ok {
		t.Fatalf("config missing pp_rf615: %v", cfg)
	}
	if cfg["records_per_page"] != float64(50) {
		t.Fatalf("records_per_page want 50 got %v", cfg["records_per_page"])
	}
}
