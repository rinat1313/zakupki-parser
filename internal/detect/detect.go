package detect

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"

	"github.com/rinat1313/zakupki-parser/models"
	"github.com/rinat1313/zakupki-parser/pkg/eis"
)

var (
	reNotice44  = regexp.MustCompile(`(?i)/epz/order/notice/([a-z0-9]+)/view/(?:common-info|documents)\.html`)
	reNotice223 = regexp.MustCompile(`(?i)/epz/order/notice/notice223/`)
)

// Result — закон и параметры URL для выгрузки.
type Result struct {
	Law        models.Law
	NoticeType string // 44: ea20, ok20, …
	NoticeGUID string // 223
	Source     string // search | search_pricereq | probe223 | probe44 | probe_pricereq
}

// Detect определяет 44/223/pricereq по живому ЕИС.
func Detect(client *eis.Client, regNumber string) (Result, error) {
	return DetectAttempts(client, regNumber, 1)
}

// DetectAttempts как Detect, но с повторами при временных сбоях ЕИС.
func DetectAttempts(client *eis.Client, regNumber string, attempts int) (Result, error) {
	regNumber = strings.TrimSpace(regNumber)
	if regNumber == "" {
		return Result{}, fmt.Errorf("empty reg_number")
	}
	if attempts < 1 {
		attempts = 1
	}

	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		if attempt > 1 {
			time.Sleep(time.Duration(attempt) * 2 * time.Second)
		}
		if r, ok := detectViaSearch(client, regNumber); ok {
			return r, nil
		}
		if r, ok := detectViaPricereqSearch(client, regNumber); ok {
			return r, nil
		}

		try223First := len(regNumber) <= 12
		if try223First {
			if r, ok := probe223(client, regNumber); ok {
				return r, nil
			}
			if r, ok := probe44(client, regNumber); ok {
				return r, nil
			}
		} else {
			if r, ok := probe44(client, regNumber); ok {
				return r, nil
			}
			if r, ok := probe223(client, regNumber); ok {
				return r, nil
			}
		}
		if r, ok := probePricereq(client, regNumber); ok {
			return r, nil
		}
		lastErr = fmt.Errorf("не удалось определить 44/223/pricereq для %s (попытка %d/%d: поиск и probe пусты)",
			regNumber, attempt, attempts)
	}
	return Result{}, lastErr
}

func detectViaSearch(client *eis.Client, regNumber string) (Result, bool) {
	u := eis.SearchByNumberURL(client.BaseURL, regNumber)
	html, err := client.Get(u)
	if err != nil {
		return Result{}, false
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(html)))
	if err != nil {
		return Result{}, false
	}

	var (
		found223 bool
		guid     string
		type44   string
	)

	doc.Find("a[href]").Each(func(_ int, a *goquery.Selection) {
		href, _ := a.Attr("href")
		if href == "" {
			return
		}
		abs := href
		if strings.HasPrefix(href, "/") {
			abs = "https://zakupki.gov.ru" + href
		}
		if strings.Contains(abs, "notice223") && strings.Contains(abs, regNumber) {
			found223 = true
			if g := guidFrom(abs); g != "" {
				guid = g
			}
		}
		if m := reNotice44.FindStringSubmatch(abs); len(m) > 1 {
			if strings.Contains(abs, "regNumber="+regNumber) || strings.Contains(a.Text(), regNumber) {
				t := m[1]
				if t != "notice223" && t != "printForm" {
					type44 = t
				}
			}
		}
	})

	if !found223 && reNotice223.Match(html) && strings.Contains(string(html), regNumber) {
		found223 = true
		guid = guidFrom(string(html))
	}
	if type44 == "" {
		if m := reNotice44.FindSubmatch(html); len(m) > 1 {
			t := string(m[1])
			if t != "notice223" && t != "printForm" && strings.Contains(string(html), "regNumber="+regNumber) {
				type44 = t
			}
		}
	}

	if found223 && type44 == "" {
		return Result{Law: models.Law223, NoticeGUID: guid, Source: "search"}, true
	}
	if type44 != "" && !found223 {
		return Result{Law: models.Law44, NoticeType: type44, Source: "search"}, true
	}
	if found223 && type44 != "" {
		if len(regNumber) <= 12 {
			return Result{Law: models.Law223, NoticeGUID: guid, Source: "search"}, true
		}
		return Result{Law: models.Law44, NoticeType: type44, Source: "search"}, true
	}
	return Result{}, false
}

func detectViaPricereqSearch(client *eis.Client, regNumber string) (Result, bool) {
	u := eis.SearchPricereqURL(client.BaseURL, regNumber)
	html, err := client.Get(u)
	if err != nil {
		return Result{}, false
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(html)))
	if err != nil {
		return Result{}, false
	}
	want := "reestrNumber=" + regNumber
	found := false
	doc.Find("a[href]").Each(func(_ int, a *goquery.Selection) {
		href, _ := a.Attr("href")
		if href == "" {
			return
		}
		if strings.Contains(href, "/epz/pricereq/card/") && strings.Contains(href, want) {
			found = true
		}
	})
	if found {
		return Result{Law: models.LawPricereq, Source: "search_pricereq"}, true
	}
	return Result{}, false
}

func probe223(client *eis.Client, regNumber string) (Result, bool) {
	u := eis.Notice223CommonInfoURL(client.BaseURL, regNumber, "")
	html, err := client.Get(u)
	if err != nil {
		return Result{}, false
	}
	s := string(html)
	if !looksLike223(s, regNumber) {
		return Result{}, false
	}
	return Result{
		Law:        models.Law223,
		NoticeGUID: guidFrom(s),
		Source:     "probe223",
	}, true
}

func probe44(client *eis.Client, regNumber string) (Result, bool) {
	types := []string{
		"eap20", "ea20", "zkp20", "okp20", "ok20", "za20",
		"ep44", "ef44", "ezk20", "ezt20", "ezp20", "inu44336",
		"pprf615", "pos504",
	}
	for _, t := range types {
		u := eis.NoticeCommonInfoURL(client.BaseURL, t, regNumber)
		html, err := client.Get(u)
		if err != nil {
			continue
		}
		if looksLike44(string(html), regNumber) {
			return Result{Law: models.Law44, NoticeType: t, Source: "probe44"}, true
		}
	}
	return Result{}, false
}

func probePricereq(client *eis.Client, regNumber string) (Result, bool) {
	u := eis.PriceReqCommonInfoURL(client.BaseURL, regNumber)
	html, err := client.Get(u)
	if err != nil {
		return Result{}, false
	}
	if !looksLikePricereq(string(html), regNumber) {
		return Result{}, false
	}
	return Result{Law: models.LawPricereq, Source: "probe_pricereq"}, true
}

func looksLike223(html, regNumber string) bool {
	low := strings.ToLower(html)
	if strings.Contains(low, "сведения не найдены") || strings.Contains(low, "извещение не найдено") {
		return false
	}
	if !strings.Contains(html, regNumber) {
		return false
	}
	if strings.Contains(html, "notice223") || strings.Contains(html, "223-ФЗ") {
		return strings.Contains(html, "registry-entry__header") ||
			strings.Contains(html, "Реестровый номер извещения") ||
			strings.Contains(html, "common-text__title")
	}
	return false
}

func looksLike44(html, regNumber string) bool {
	low := strings.ToLower(html)
	if strings.Contains(low, "сведения не найдены") || strings.Contains(low, "извещение не найдено") {
		return false
	}
	if !strings.Contains(html, regNumber) {
		return false
	}
	// не путать с запросом цен
	if strings.Contains(html, "/epz/pricereq/") && !strings.Contains(html, "/epz/order/notice/") {
		return false
	}
	return strings.Contains(html, "cardMainInfo") ||
		strings.Contains(html, "cardMainInfo__purchaseLink")
}

func looksLikePricereq(html, regNumber string) bool {
	low := strings.ToLower(html)
	if strings.Contains(low, "сведения не найдены") {
		return false
	}
	if !strings.Contains(html, regNumber) {
		return false
	}
	// только карточка, не страница поиска/фильтров
	if strings.Contains(html, "Общая информация запроса цен") {
		return true
	}
	if strings.Contains(html, "/epz/pricereq/card/") &&
		strings.Contains(html, "blockInfo__section") &&
		!strings.Contains(html, "search-registry-entrys-block") {
		return true
	}
	return false
}

func guidFrom(s string) string {
	re := regexp.MustCompile(`(?i)(?:noticeGuid|purchaseNoticeGuid)=([a-f0-9-]{36})`)
	if m := re.FindStringSubmatch(s); len(m) > 1 {
		return strings.ToLower(m[1])
	}
	if u, err := url.Parse(s); err == nil {
		if g := u.Query().Get("noticeGuid"); g != "" {
			return strings.ToLower(g)
		}
		if g := u.Query().Get("purchaseNoticeGuid"); g != "" {
			return strings.ToLower(g)
		}
	}
	return ""
}
