package pricereq

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"github.com/rinat1313/zakupki-parser/internal/textutil"
	"github.com/rinat1313/zakupki-parser/models"
)

var (
	reInfoID = regexp.MustCompile(`(?i)priceRequestInfoId=(\d+)`)
	reQty    = regexp.MustCompile(`[\d]+(?:[.,]\d+)?`)
)

// ParseCommonInfoHTML разбирает карточку /epz/pricereq/card/common-info.html
// или сниппет выдачи поиска (search-registry-entry-block).
func ParseCommonInfoHTML(html []byte, sourceURL string) (*models.PriceRequest, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("parse html: %w", err)
	}

	n := &models.PriceRequest{
		Law:       models.LawPricereq,
		SourceURL: sourceURL,
	}

	// --- выдача поиска ---
	if entry := doc.Find(".search-registry-entry-block").First(); entry.Length() > 0 {
		fillFromSearchEntry(n, entry)
	}

	// --- шапка карточки (если есть) ---
	if n.ReestrNumber == "" {
		n.ReestrNumber = textutil.StripNoticeNumber(doc.Find(".cardMainInfo__purchaseLink a, .registry-entry__header-mid__number a").First().Text())
	}
	if n.Status == "" {
		n.Status = textutil.CleanSpace(doc.Find(".cardMainInfo__state, .registry-entry__header-mid__title").First().Text())
	}

	doc.Find(".cardMainInfo__section").Each(func(_ int, s *goquery.Selection) {
		title := textutil.CleanSpace(s.Find(".cardMainInfo__title").First().Text())
		content := s.Find(".cardMainInfo__content").First()
		val := textutil.CleanSpace(content.Text())
		switch {
		case textutil.ContainsLabel(title, "Объект закупки") || textutil.ContainsLabel(title, "Наименование объекта"):
			if n.ObjectName == "" {
				n.ObjectName = val
			}
		case textutil.ContainsLabel(title, "Организация") || textutil.ContainsLabel(title, "Заказчик"):
			if n.OrgName == "" {
				n.OrgName = val
			}
			if href, ok := content.Find("a").Attr("href"); ok && n.OrganizationCode == "" {
				n.OrganizationCode = textutil.OrganizationCodeFromHref(href)
			}
		case textutil.ContainsLabel(title, "Размещено"):
			n.Published = textutil.ParseDated(val)
		case textutil.ContainsLabel(title, "Обновлено"):
			n.Updated = textutil.ParseDated(val)
		}
	})

	labels := mapSectionLabels(doc)
	applyLabels(n, labels)

	// data-block из поиска / правой колонки
	doc.Find(".data-block__title").Each(func(_ int, t *goquery.Selection) {
		title := textutil.CleanSpace(t.Text())
		val := textutil.CleanSpace(t.Parent().Find(".data-block__value").First().Text())
		applyDataBlock(n, title, val)
	})

	if n.PriceRequestInfoID == "" {
		if m := reInfoID.FindSubmatch(html); len(m) > 1 {
			n.PriceRequestInfoID = string(m[1])
		}
	}
	if n.OrganizationCode == "" {
		doc.Find(`a[href*="organizationCode="]`).Each(func(_ int, a *goquery.Selection) {
			if n.OrganizationCode != "" {
				return
			}
			href, _ := a.Attr("href")
			n.OrganizationCode = textutil.OrganizationCodeFromHref(href)
		})
	}

	n.Items = parseItems(doc)

	if n.ReestrNumber == "" {
		// fallback: reestrNumber= из URL/HTML
		if m := regexp.MustCompile(`reestrNumber=([0-9]+)`).FindSubmatch(html); len(m) > 1 {
			n.ReestrNumber = string(m[1])
		}
	}
	if n.ReestrNumber == "" && n.ObjectName == "" {
		return nil, fmt.Errorf("pricereq: reestr_number/object not found in HTML")
	}
	return n, nil
}

func fillFromSearchEntry(n *models.PriceRequest, entry *goquery.Selection) {
	n.ReestrNumber = textutil.StripNoticeNumber(entry.Find(".registry-entry__header-mid__number a").First().Text())
	n.Status = textutil.CleanSpace(entry.Find(".registry-entry__header-mid__title").First().Text())

	entry.Find(".registry-entry__body-block").Each(func(_ int, b *goquery.Selection) {
		title := textutil.CleanSpace(b.Find(".registry-entry__body-title").First().Text())
		val := textutil.CleanSpace(b.Find(".registry-entry__body-value, .registry-entry__body-href").First().Text())
		switch {
		case textutil.ContainsLabel(title, "Организация, разместившая запрос цен"):
			n.OrgName = val
			if href, ok := b.Find("a").Attr("href"); ok {
				n.OrganizationCode = textutil.OrganizationCodeFromHref(href)
			}
		case textutil.ContainsLabel(title, "Наименование объекта закупки"):
			n.ObjectName = val
		}
	})

	entry.Find(".data-block__title").Each(func(_ int, t *goquery.Selection) {
		title := textutil.CleanSpace(t.Text())
		val := textutil.CleanSpace(t.Parent().Find(".data-block__value").First().Text())
		applyDataBlock(n, title, val)
	})

	if href, ok := entry.Find(`a[href*="priceRequestInfoId="]`).Attr("href"); ok {
		if m := reInfoID.FindStringSubmatch(href); len(m) > 1 {
			n.PriceRequestInfoID = m[1]
		}
	}
	if href, ok := entry.Find(`a[href*="common-info.html"]`).Attr("href"); ok {
		n.SourceURL = absURL(href)
	}
}

func applyDataBlock(n *models.PriceRequest, title, val string) {
	switch {
	case textutil.ContainsLabel(title, "Размещено"):
		n.Published = textutil.ParseDated(val)
	case textutil.ContainsLabel(title, "Обновлено"):
		n.Updated = textutil.ParseDated(val)
	case textutil.ContainsLabel(title, "Предоставление ценовой информации"):
		n.PriceInfoPeriodRaw = val
		from, to := splitPeriod(val)
		if from != "" {
			n.PriceInfoFrom = textutil.ParseDated(from)
		}
		if to != "" {
			n.PriceInfoTo = textutil.ParseDated(to)
		}
	case textutil.ContainsLabel(title, "Сроки проведения закупки"):
		n.PurchasePeriodRaw = val
	}
}

func applyLabels(n *models.PriceRequest, labels map[string]string) {
	if v := firstLabel(labels, "наименование объекта закупки"); v != "" {
		n.ObjectName = v
	}
	if v := firstLabel(labels, "наименование организации"); v != "" {
		n.OrgName = v
	}
	if v := firstLabel(labels, "полномочие организации"); v != "" {
		n.OrgAuthority = v
	}
	if v := firstLabel(labels, "субъект рф"); v != "" {
		n.Region = v
	}
	if v := firstLabel(labels, "дата и время начала предоставления ценовой"); v != "" {
		n.PriceInfoFrom = textutil.ParseDated(v)
	}
	if v := firstLabel(labels, "дата и время окончания предоставления ценовой"); v != "" {
		n.PriceInfoTo = textutil.ParseDated(v)
	}
	if v := firstLabel(labels, "предполагаемые сроки проведения закупки"); v != "" {
		n.PurchasePeriodRaw = v
	}
	if v := firstLabel(labels, "адрес электронной почты"); v != "" || firstLabel(labels, "электронной почты") != "" {
		if v == "" {
			v = firstLabel(labels, "электронной почты")
		}
		n.ContactEmail = v
	}
	if v := firstLabel(labels, "контактного телефона"); v != "" || firstLabel(labels, "телефон") != "" {
		if v == "" {
			v = firstLabel(labels, "телефон")
		}
		n.ContactPhone = v
	}
	if v := firstLabel(labels, "ответственное должностное лицо"); v != "" || firstLabel(labels, "контактное лицо") != "" {
		if v == "" {
			v = firstLabel(labels, "контактное лицо")
		}
		n.ContactPerson = v
	}
}

func parseItems(doc *goquery.Document) []models.PriceRequestItem {
	var out []models.PriceRequestItem
	doc.Find("table").Each(func(_ int, table *goquery.Selection) {
		nameIdx, codeIdx, qtyIdx, unitIdx := 0, 1, 2, -1
		header := table.Find("tr").First()
		if header.Find("th").Length() > 0 {
			header.Find("th").Each(func(i int, th *goquery.Selection) {
				h := textutil.NormalizeLabel(th.Text())
				switch {
				case strings.Contains(h, "наименование"):
					nameIdx = i
				case strings.Contains(h, "код"):
					codeIdx = i
				case strings.Contains(h, "количеств"):
					qtyIdx = i
				case strings.Contains(h, "ед"):
					unitIdx = i
				}
			})
		}
		table.Find("tr").Each(func(_ int, tr *goquery.Selection) {
			if tr.Find("th").Length() > 0 {
				return
			}
			cells := tr.Find("td")
			if cells.Length() < 2 {
				return
			}
			cell := func(idx int) string {
				if idx < 0 || idx >= cells.Length() {
					return ""
				}
				return textutil.CleanSpace(cells.Eq(idx).Text())
			}
			item := models.PriceRequestItem{
				Name: cell(nameIdx),
				Code: cell(codeIdx),
				Unit: cell(unitIdx),
			}
			q := cell(qtyIdx)
			if q != "" {
				if m := reQty.FindString(q); m != "" {
					m = strings.ReplaceAll(m, ",", ".")
					if v, err := strconv.ParseFloat(m, 64); err == nil {
						item.Qty = v
					}
				}
				if item.Unit == "" && strings.Contains(q, " ") {
					parts := strings.SplitN(q, " ", 2)
					if len(parts) == 2 && reQty.MatchString(parts[0]) {
						item.Unit = textutil.CleanSpace(parts[1])
					}
				}
			}
			if item.Name == "" && item.Code == "" {
				return
			}
			out = append(out, item)
		})
	})
	return out
}

func mapSectionLabels(doc *goquery.Document) map[string]string {
	out := make(map[string]string)
	doc.Find("section.blockInfo__section, .blockInfo__section.section").Each(func(_ int, s *goquery.Selection) {
		title := textutil.CleanSpace(s.Find(".section__title").First().Text())
		info := textutil.CleanSpace(s.Find(".section__info").First().Text())
		if title == "" || info == "" {
			return
		}
		key := textutil.NormalizeLabel(title)
		if prev, ok := out[key]; ok && len(prev) > 0 && len(info) > len(prev)*3 {
			return
		}
		out[key] = info
	})
	return out
}

func firstLabel(m map[string]string, substr string) string {
	want := textutil.NormalizeLabel(substr)
	for k, v := range m {
		if k == want || strings.Contains(k, want) {
			return v
		}
	}
	return ""
}

func splitPeriod(raw string) (from, to string) {
	raw = textutil.CleanSpace(raw)
	parts := regexp.MustCompile(`\s*[-–—]\s*`).Split(raw, 2)
	if len(parts) == 2 {
		return textutil.CleanSpace(parts[0]), textutil.CleanSpace(parts[1])
	}
	return raw, ""
}

func absURL(href string) string {
	if strings.HasPrefix(href, "http") {
		return href
	}
	if strings.HasPrefix(href, "/") {
		return "https://zakupki.gov.ru" + href
	}
	return href
}
