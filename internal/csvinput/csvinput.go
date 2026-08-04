package csvinput

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"
)

// Record — строка входного CSV (44 и/или 223 вперемешку).
type Record struct {
	RegNumber  string
	NoticeGUID string // только 223; можно пусто
	LawHint    string // опционально "44"/"223"; пусто = автодетект
}

// LoadRegNumbers читает CSV с колонкой reg_number (или первый столбец).
func LoadRegNumbers(path string) ([]string, error) {
	recs, err := LoadRecords(path)
	if err != nil {
		return nil, err
	}
	out := make([]string, len(recs))
	for i, r := range recs {
		out[i] = r.RegNumber
	}
	return out, nil
}

// LoadRecords читает CSV: reg_number[, notice_guid][, law].
func LoadRecords(path string) ([]Record, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.TrimLeadingSpace = true
	r.ReuseRecord = true
	r.FieldsPerRecord = -1
	r.Comment = '#'

	var (
		out        []Record
		regCol     = -1
		guidCol    = -1
		lawCol     = -1
		headerDone bool
	)
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("csv: %w", err)
		}
		if len(rec) == 0 {
			continue
		}
		if !headerDone {
			headerDone = true
			for i, h := range rec {
				h = strings.ToLower(strings.TrimSpace(h))
				switch h {
				case "reg_number", "regnumber", "номер", "id", "purchase_notice_number":
					regCol = i
				case "notice_guid", "noticeguid", "guid", "purchase_notice_guid":
					guidCol = i
				case "law", "fz", "фз":
					lawCol = i
				}
			}
			if regCol >= 0 {
				continue
			}
			regCol = 0
			if len(rec) > 1 {
				guidCol = 1
			}
			// fall through — первая строка без заголовка
		}
		if regCol >= len(rec) {
			continue
		}
		id := strings.TrimSpace(rec[regCol])
		id = strings.TrimPrefix(id, "№")
		id = strings.TrimSpace(id)
		if id == "" || strings.EqualFold(id, "reg_number") {
			continue
		}
		row := Record{RegNumber: id}
		if guidCol >= 0 && guidCol < len(rec) {
			row.NoticeGUID = strings.TrimSpace(rec[guidCol])
		}
		if lawCol >= 0 && lawCol < len(rec) {
			row.LawHint = normalizeLawHint(rec[lawCol])
		}
		out = append(out, row)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no reg_number values in %s", path)
	}
	return out, nil
}

func normalizeLawHint(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.ReplaceAll(s, "-", "")
	s = strings.ReplaceAll(s, " ", "")
	s = strings.TrimSuffix(s, "фз")
	switch s {
	case "44", "223", "pricereq", "price", "pricerequest":
		if s == "price" || s == "pricerequest" {
			return "pricereq"
		}
		return s
	default:
		return ""
	}
}
