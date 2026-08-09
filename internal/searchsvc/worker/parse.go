package worker

import (
	"bytes"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/rinat1313/zakupki-parser/internal/searchsvc/models"
)

var (
	reRegNumber  = regexp.MustCompile(`(?i)(?:regNumber|purchaseNoticeNumber|reestrNumber)=([0-9A-Za-z\-]+)`)
	reDigits     = regexp.MustCompile(`\d[\d\s]*([.,]\d+)?`)
	reLongDigits = regexp.MustCompile(`\d{10,}`)
)

// ParseSearchHTML extracts notice hits from EIS extendedsearch/results.html.
func ParseSearchHTML(body []byte) []models.SearchHit {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil
	}

	out := make([]models.SearchHit, 0)
	seen := map[string]struct{}{}

	blocks := doc.Find(".search-registry-entry-block, .registry-entry__form")
	if blocks.Length() == 0 {
		// fallback: any notice links on the page
		doc.Find("a[href*='regNumber='], a[href*='purchaseNoticeNumber=']").Each(func(_ int, a *goquery.Selection) {
			href, _ := a.Attr("href")
			reg := extractReg(href)
			if reg == "" {
				return
			}
			if _, ok := seen[reg]; ok {
				return
			}
			seen[reg] = struct{}{}
			out = append(out, models.SearchHit{
				RegNumber:  reg,
				ObjectName: strings.TrimSpace(a.Text()),
				NoticeURL:  absURL(href),
				Law:        guessLaw(href),
			})
		})
		return out
	}

	blocks.Each(func(_ int, s *goquery.Selection) {
		link := s.Find("a[href*='regNumber='], a[href*='purchaseNoticeNumber=']").First()
		href, _ := link.Attr("href")
		reg := extractReg(href)
		if reg == "" {
			text := s.Find(".registry-entry__header-mid__number").Text()
			reg = extractDigitsReg(text)
		}
		if reg == "" {
			return
		}
		if _, ok := seen[reg]; ok {
			return
		}
		seen[reg] = struct{}{}

		objectName := strings.TrimSpace(s.Find(".registry-entry__body-value").First().Text())
		if objectName == "" {
			objectName = strings.TrimSpace(s.Find(".registry-entry__body-href").First().Text())
		}

		var nmck *float64
		priceText := s.Find(".price-block__value, .cost").First().Text()
		if v, ok := parseMoney(priceText); ok {
			nmck = &v
		}

		appl := ""
		s.Find(".data-block__title").Each(func(_ int, title *goquery.Selection) {
			t := strings.ToLower(strings.TrimSpace(title.Text()))
			if strings.Contains(t, "окончани") || strings.Contains(t, "подач") {
				val := strings.TrimSpace(title.Parent().Find(".data-block__value").First().Text())
				if val != "" {
					appl = collapseWS(val)
				}
			}
		})

		out = append(out, models.SearchHit{
			RegNumber:      reg,
			ObjectName:     collapseWS(objectName),
			NoticeURL:      absURL(href),
			NMCK:           nmck,
			ApplicationEnd: appl,
			Law:            guessLaw(href),
		})
	})
	return out
}

func extractReg(href string) string {
	m := reRegNumber.FindStringSubmatch(href)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

func extractDigitsReg(text string) string {
	text = strings.TrimSpace(strings.ReplaceAll(text, "№", ""))
	text = collapseWS(text)
	return reLongDigits.FindString(text)
}

func absURL(href string) string {
	href = strings.TrimSpace(href)
	if href == "" {
		return ""
	}
	if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") {
		return href
	}
	if strings.HasPrefix(href, "/") {
		return "https://zakupki.gov.ru" + href
	}
	return href
}

func guessLaw(href string) string {
	h := strings.ToLower(href)
	switch {
	case strings.Contains(h, "notice223"):
		return "223"
	case strings.Contains(h, "pricereq"):
		return "pricereq"
	case strings.Contains(h, "/notice/"):
		return "44"
	default:
		return ""
	}
}

func parseMoney(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	m := reDigits.FindString(s)
	if m == "" {
		return 0, false
	}
	m = strings.ReplaceAll(m, " ", "")
	m = strings.ReplaceAll(m, "\u00a0", "")
	m = strings.ReplaceAll(m, ",", ".")
	v, err := strconv.ParseFloat(m, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

func collapseWS(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
