package models

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// SearcherConfig — параметры расширенного поиска ЕИС (контракт Gateway UI).
// Поле pp_rf615 (не pp_rf_615). records_per_page — целое 10|20|50.
type SearcherConfig struct {
	SearchString string `json:"search_string"`
	Morphology   bool   `json:"morphology"`
	StrictEqual  bool   `json:"strict_equal"`

	FZ44    bool `json:"fz44"`
	FZ223   bool `json:"fz223"`
	PPRF615 bool `json:"pp_rf615"`

	StageAF bool `json:"stage_af"`
	StageCA bool `json:"stage_ca"`
	StagePC bool `json:"stage_pc"`
	StagePA bool `json:"stage_pa"`

	PublishDateFrom *string `json:"publish_date_from"`
	PublishDateTo   *string `json:"publish_date_to"`
	ApplCloseFrom   *string `json:"appl_submission_close_date_from"`
	ApplCloseTo     *string `json:"appl_submission_close_date_to"`

	PriceFrom *float64 `json:"price_from"`
	PriceTo   *float64 `json:"price_to"`

	CurrencyCode string `json:"currency_code"`

	OKPD2Codes      string `json:"okpd2_codes"`
	OKPD2WithNested bool   `json:"okpd2_with_nested"`
	OKPD2Several    bool   `json:"okpd2_several"`

	CustomerTitle   string `json:"customer_title"`
	PlacingWayList  string `json:"placing_way_list"`
	SMPSono         bool   `json:"smp_sono"`
	JointPurchase   bool   `json:"joint_purchase"`

	SortBy         string         `json:"sort_by"`
	SortDirection  bool           `json:"sort_direction"`
	RecordsPerPage RecordsPerPage `json:"records_per_page"`
}

// RecordsPerPage accepts UI integers (10|20|50) or EIS strings (_10|_20|_50).
type RecordsPerPage int

func (r RecordsPerPage) EIS() string {
	switch int(r) {
	case 20:
		return "_20"
	case 50:
		return "_50"
	case 10:
		return "_10"
	default:
		if int(r) <= 0 {
			return "_50"
		}
		return "_10"
	}
}

func (r *RecordsPerPage) UnmarshalJSON(b []byte) error {
	b = bytesTrimSpace(b)
	if len(b) == 0 || string(b) == "null" {
		*r = 50
		return nil
	}
	if b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		s = strings.TrimSpace(s)
		s = strings.TrimPrefix(s, "_")
		n, err := strconv.Atoi(s)
		if err != nil {
			return fmt.Errorf("records_per_page: %w", err)
		}
		*r = RecordsPerPage(n)
		return nil
	}
	var n int
	if err := json.Unmarshal(b, &n); err != nil {
		return err
	}
	*r = RecordsPerPage(n)
	return nil
}

func (r RecordsPerPage) MarshalJSON() ([]byte, error) {
	n := int(r)
	if n <= 0 {
		n = 50
	}
	return json.Marshal(n)
}

func bytesTrimSpace(b []byte) []byte {
	return []byte(strings.TrimSpace(string(b)))
}

// DefaultSearcherConfig — стартовые значения как в UI searchers.js.
func DefaultSearcherConfig() SearcherConfig {
	return SearcherConfig{
		Morphology:      true,
		FZ44:            true,
		FZ223:           true,
		StageAF:         true,
		StageCA:         true,
		CurrencyCode:    "RUB",
		OKPD2WithNested: true,
		SortBy:          "UPDATE_DATE",
		SortDirection:   false,
		RecordsPerPage:  50,
	}
}

const EISResultsPath = "/epz/order/extendedsearch/results.html"

// QueryValues строит query-параметры для results.html.
func (c SearcherConfig) QueryValues() url.Values {
	q := url.Values{}
	if s := strings.TrimSpace(c.SearchString); s != "" {
		q.Set("searchString", s)
	}
	if c.Morphology && !c.StrictEqual {
		q.Set("morphology", "on")
	}
	if c.StrictEqual {
		q.Set("strictEqual", "true")
	}
	if c.FZ44 {
		q.Set("fz44", "on")
	}
	if c.FZ223 {
		q.Set("fz223", "on")
	}
	if c.PPRF615 {
		q.Set("ppRf615", "on")
	}
	if c.StageAF {
		q.Set("af", "on")
	}
	if c.StageCA {
		q.Set("ca", "on")
	}
	if c.StagePC {
		q.Set("pc", "on")
	}
	if c.StagePA {
		q.Set("pa", "on")
	}

	sortBy := c.SortBy
	if sortBy == "" {
		sortBy = "UPDATE_DATE"
	}
	q.Set("sortBy", sortBy)
	q.Set("sortDirection", strconv.FormatBool(c.SortDirection))
	q.Set("recordsPerPage", c.RecordsPerPage.EIS())
	q.Set("pageNumber", "1")

	if c.PriceFrom != nil {
		q.Set("priceFromGeneral", formatFloat(*c.PriceFrom))
	}
	if c.PriceTo != nil {
		q.Set("priceToGeneral", formatFloat(*c.PriceTo))
	}
	if id := currencyID(c.CurrencyCode); id != "" {
		q.Set("currencyIdGeneral", id)
	}
	if v := derefTrim(c.PublishDateFrom); v != "" {
		q.Set("publishDateFrom", v)
	}
	if v := derefTrim(c.PublishDateTo); v != "" {
		q.Set("publishDateTo", v)
	}
	if v := derefTrim(c.ApplCloseFrom); v != "" {
		q.Set("applSubmissionCloseDateFrom", v)
	}
	if v := derefTrim(c.ApplCloseTo); v != "" {
		q.Set("applSubmissionCloseDateTo", v)
	}
	if t := strings.TrimSpace(c.CustomerTitle); t != "" {
		q.Set("customerTitle", t)
	}
	if codes := strings.TrimSpace(c.OKPD2Codes); codes != "" {
		q.Set("okpd2IdsCodes", codes)
		if c.OKPD2WithNested {
			q.Set("okpdWithNested", "on")
		}
		if c.OKPD2Several {
			q.Set("okpd2IdsWithNestedAlways", "on")
		}
	}
	if pw := strings.TrimSpace(c.PlacingWayList); pw != "" {
		q.Set("placingWayList", pw)
	}
	if c.SMPSono {
		q.Set("selectedSmp", "on")
	}
	if c.JointPurchase {
		q.Set("jointPurchase", "on")
	}
	return q
}

// ResultsURL возвращает полный URL страницы поиска (pageNumber подставляется отдельно при пагинации).
func (c SearcherConfig) ResultsURL(base string) string {
	base = strings.TrimRight(base, "/")
	if base == "" {
		base = "https://zakupki.gov.ru"
	}
	return base + EISResultsPath + "?" + c.QueryValues().Encode()
}

// PageURL — URL конкретной страницы результатов.
func (c SearcherConfig) PageURL(base string, page int) string {
	if page < 1 {
		page = 1
	}
	q := c.QueryValues()
	q.Set("pageNumber", strconv.Itoa(page))
	base = strings.TrimRight(base, "/")
	if base == "" {
		base = "https://zakupki.gov.ru"
	}
	return base + EISResultsPath + "?" + q.Encode()
}

func formatFloat(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

func derefTrim(p *string) string {
	if p == nil {
		return ""
	}
	return strings.TrimSpace(*p)
}

func currencyID(code string) string {
	switch strings.ToUpper(strings.TrimSpace(code)) {
	case "", "ALL", "-1":
		return "-1"
	case "RUB", "RUR":
		return "1"
	case "USD":
		return "2"
	case "EUR":
		return "3"
	default:
		return ""
	}
}
