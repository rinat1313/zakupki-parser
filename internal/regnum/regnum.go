package regnum

import (
	"regexp"
	"strings"
)

var reTrailingUnderscoreDigits = regexp.MustCompile(`_\d+$`)

// Candidates возвращает варианты номера для поиска в ЕИС.
// Сначала полный (как в CSV), затем без суффикса _N (например _1).
func Candidates(raw string) []string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "№")
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	out := []string{raw}
	alt := StripEditionSuffix(raw)
	if alt != "" && alt != raw {
		out = append(out, alt)
	}
	return out
}

// StripEditionSuffix убирает хвост вида _1, _2, _12.
func StripEditionSuffix(raw string) string {
	raw = strings.TrimSpace(raw)
	return reTrailingUnderscoreDigits.ReplaceAllString(raw, "")
}

// HasEditionSuffix — есть ли хвост _цифры.
func HasEditionSuffix(raw string) bool {
	return StripEditionSuffix(raw) != strings.TrimSpace(raw)
}
