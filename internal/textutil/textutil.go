package textutil

import (
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/rinat1313/zakupki-parser/models"
)

var (
	reSpaces     = regexp.MustCompile(`[\s\x{00a0}]+`)
	reCurrency   = regexp.MustCompile(`(?i)(₽|руб\.?|российский рубль|&#8381;)`)
	reNonDigit   = regexp.MustCompile(`[^\d,.\-]`)
	reTZ         = regexp.MustCompile(`\((МСК[+-]?\d*)\)`)
	reOrgCode    = regexp.MustCompile(`organizationCode=([0-9]+)`)
	reAgencyID   = regexp.MustCompile(`(?i)agencyId=([0-9]+)`)
	reNoticeGUID = regexp.MustCompile(`(?i)(?:noticeGuid|purchaseNoticeGuid)=([a-f0-9-]{36})`)
	reLotGUID    = regexp.MustCompile(`(?i)lotGuid=([a-f0-9-]{36})`)
	reINN        = regexp.MustCompile(`(?i)[?&]inn=([0-9]+)`)
	reKPP        = regexp.MustCompile(`(?i)[?&]kpp=([0-9]+)`)
	reOGRN       = regexp.MustCompile(`(?i)[?&]ogrn=([0-9]+)`)
)

// CleanSpace схлопывает пробелы и NBSP.
func CleanSpace(s string) string {
	s = strings.ReplaceAll(s, "\u00a0", " ")
	s = reSpaces.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

// ParseMoney разбирает сумму вида «7 048 800,00 ₽».
func ParseMoney(raw string) models.Money {
	raw = CleanSpace(raw)
	m := models.Money{Raw: raw, Currency: "RUB"}
	s := reCurrency.ReplaceAllString(raw, "")
	s = CleanSpace(s)
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, ",", ".")
	s = reNonDigit.ReplaceAllString(s, "")
	if s == "" || s == "-" {
		return m
	}
	// оставить одну точку
	if i := strings.LastIndex(s, "."); i >= 0 {
		s = strings.ReplaceAll(s[:i], ".", "") + s[i:]
	}
	if v, err := strconv.ParseFloat(s, 64); err == nil {
		m.Amount = v
	}
	return m
}

// ParseDated разбирает «01.08.2026» или «01.08.2026 07:15 (МСК)».
func ParseDated(raw string) models.Dated {
	raw = CleanSpace(raw)
	d := models.Dated{Raw: raw}
	if m := reTZ.FindStringSubmatch(raw); len(m) > 1 {
		d.TZLabel = m[1]
	}
	base := reTZ.ReplaceAllString(raw, "")
	base = CleanSpace(base)
	layouts := []string{
		"02.01.2006 15:04",
		"02.01.2006",
	}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, base, time.Local); err == nil {
			d.At = t
			break
		}
	}
	return d
}

// StripNoticeNumber убирает «№» и пробелы.
func StripNoticeNumber(s string) string {
	s = CleanSpace(s)
	s = strings.TrimPrefix(s, "№")
	return CleanSpace(s)
}

// OrganizationCodeFromHref достаёт organizationCode из URL.
func OrganizationCodeFromHref(href string) string {
	m := reOrgCode.FindStringSubmatch(href)
	if len(m) > 1 {
		return m[1]
	}
	return ""
}

// AgencyIDFromHref достаёт agencyId из URL view223.
func AgencyIDFromHref(href string) string {
	m := reAgencyID.FindStringSubmatch(href)
	if len(m) > 1 {
		return m[1]
	}
	return ""
}

// NoticeGUIDFromHref достаёт noticeGuid / purchaseNoticeGuid.
func NoticeGUIDFromHref(href string) string {
	m := reNoticeGUID.FindStringSubmatch(href)
	if len(m) > 1 {
		return strings.ToLower(m[1])
	}
	return ""
}

// LotGUIDFromHref достаёт lotGuid.
func LotGUIDFromHref(href string) string {
	m := reLotGUID.FindStringSubmatch(href)
	if len(m) > 1 {
		return strings.ToLower(m[1])
	}
	return ""
}

// InnKppOgrnFromHref достаёт inn/kpp/ogrn из query view223.
func InnKppOgrnFromHref(href string) (inn, kpp, ogrn string) {
	if m := reINN.FindStringSubmatch(href); len(m) > 1 {
		inn = m[1]
	}
	if m := reKPP.FindStringSubmatch(href); len(m) > 1 {
		kpp = m[1]
	}
	if m := reOGRN.FindStringSubmatch(href); len(m) > 1 {
		ogrn = m[1]
	}
	return inn, kpp, ogrn
}

// NormalizeLabel для сравнения подписей полей.
func NormalizeLabel(s string) string {
	s = CleanSpace(s)
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == ' ' {
			b.WriteRune(r)
		}
	}
	return CleanSpace(b.String())
}

// ContainsLabel — мягкое сравнение подписи (без учёта регистра и лишней пунктуации).
func ContainsLabel(got, want string) bool {
	g := NormalizeLabel(got)
	w := NormalizeLabel(want)
	return g == w || strings.Contains(g, w)
}

// Truncate руны для логов.
func Truncate(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n]) + "…"
}
