package fz44

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"github.com/rinat1313/zakupki-parser/internal/textutil"
	"github.com/rinat1313/zakupki-parser/models"
)

// ParseNoticeHTML разбирает HTML вкладки common-info 44-ФЗ в Notice44.
func ParseNoticeHTML(html []byte, sourceURL string) (*models.Notice44, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("parse html: %w", err)
	}

	n := &models.Notice44{
		Law:       models.Law44,
		SourceURL: sourceURL,
	}
	if sourceURL != "" {
		n.NoticeTypePath = noticeTypeFromURL(sourceURL)
	}

	// Шапка
	n.RegNumber = textutil.StripNoticeNumber(doc.Find(".cardMainInfo__purchaseLink a").First().Text())
	n.Status = textutil.CleanSpace(doc.Find(".cardMainInfo__state").First().Text())

	doc.Find(".cardMainInfo__section").Each(func(_ int, s *goquery.Selection) {
		title := textutil.CleanSpace(s.Find(".cardMainInfo__title").First().Text())
		content := s.Find(".cardMainInfo__content").First()
		val := textutil.CleanSpace(content.Text())
		switch {
		case textutil.ContainsLabel(title, "Объект закупки"):
			n.ObjectName = val
		case textutil.ContainsLabel(title, "Заказчик") || textutil.ContainsLabel(title, "Организация, осуществляющая размещение"):
			n.CustomerName = val
			if href, ok := content.Find("a").Attr("href"); ok {
				n.OrganizationCode = textutil.OrganizationCodeFromHref(href)
			}
			if textutil.ContainsLabel(title, "Уполномоченный") {
				n.PlacerIsAuthBody = true
			}
		}
	})

	// Цена в шапке (запасной вариант, если блок НМЦК пуст)
	headerPrice := textutil.CleanSpace(doc.Find(".cardMainInfo__content.cost").First().Text())
	if headerPrice != "" {
		n.NMCK = textutil.ParseMoney(headerPrice)
	}

	doc.Find(".cardMainInfo__section").Each(func(_ int, s *goquery.Selection) {
		title := textutil.CleanSpace(s.Find(".cardMainInfo__title").First().Text())
		val := textutil.CleanSpace(s.Find(".cardMainInfo__content").First().Text())
		switch {
		case textutil.ContainsLabel(title, "Размещено"):
			n.Published = textutil.ParseDated(val)
		case textutil.ContainsLabel(title, "Обновлено"):
			n.Updated = textutil.ParseDated(val)
		case textutil.ContainsLabel(title, "Окончание подачи заявок"):
			n.ApplicationEnd = textutil.ParseDated(val)
		}
	})

	// Поля по подписям section__title → section__info
	labels := mapSectionLabels(doc)

	if v := labels["способ определения поставщика подрядчика исполнителя"]; v != "" {
		n.Method = v
	}
	if v := firstLabel(labels, "наименование электронной площадки"); v != "" {
		n.ETPName = v
	}
	if v := firstLabel(labels, "адрес электронной площадки"); v != "" {
		n.ETPURL = v
	}
	if v := labels["наименование объекта закупки"]; v != "" {
		n.ObjectName = v
	}
	if v := labels["этап закупки"]; v != "" {
		n.Status = v
	}
	if v := firstLabel(labels, "сведения о связи с позицией плана графика"); v != "" {
		n.PlanPositionNumber = textutil.CleanSpace(strings.Split(v, "\n")[0])
	}
	if v := labels["ответственное должностное лицо"]; v != "" {
		n.ContactPerson = v
	}
	if v := labels["адрес электронной почты"]; v != "" {
		n.ContactEmail = v
	}
	if v := labels["номер контактного телефона"]; v != "" {
		n.ContactPhone = v
	}
	if v := labels["регион"]; v != "" {
		n.Region = v
	}
	if v := firstLabel(labels, "начальная максимальная цена контракта"); v != "" {
		n.NMCK = textutil.ParseMoney(v)
	}
	if v := labels["валюта"]; v != "" {
		n.NMCK.Currency = normalizeCurrency(v)
	}
	if v := firstLabel(labels, "идентификационный код закупки"); v != "" {
		n.IKZ = textutil.CleanSpace(strings.Split(v, "\n")[0])
	}

	// organizationCode из любого org-link, если ещё пусто
	if n.OrganizationCode == "" {
		doc.Find(`a[href*="organizationCode="]`).EachWithBreak(func(_ int, s *goquery.Selection) bool {
			if href, ok := s.Attr("href"); ok {
				if code := textutil.OrganizationCodeFromHref(href); code != "" {
					n.OrganizationCode = code
					if n.CustomerName == "" {
						n.CustomerName = textutil.CleanSpace(s.Text())
					}
					return false
				}
			}
			return true
		})
	}

	if n.RegNumber == "" {
		return nil, fmt.Errorf("reg_number not found in HTML")
	}
	return n, nil
}

func mapSectionLabels(doc *goquery.Document) map[string]string {
	out := make(map[string]string)
	doc.Find("section.blockInfo__section").Each(func(_ int, s *goquery.Selection) {
		title := textutil.CleanSpace(s.Find(".section__title").First().Text())
		info := textutil.CleanSpace(s.Find(".section__info").First().Text())
		if title == "" || info == "" {
			return
		}
		key := textutil.NormalizeLabel(title)
		// не перезаписывать уже найденное более длинным «мусором» из вложенных таблиц без нужды
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

func normalizeCurrency(s string) string {
	s = textutil.NormalizeLabel(s)
	if strings.Contains(s, "руб") {
		return "RUB"
	}
	return strings.ToUpper(textutil.CleanSpace(s))
}

func noticeTypeFromURL(u string) string {
	// .../notice/ea20/view/...
	const marker = "/notice/"
	i := strings.Index(u, marker)
	if i < 0 {
		return ""
	}
	rest := u[i+len(marker):]
	j := strings.Index(rest, "/")
	if j < 0 {
		return ""
	}
	return rest[:j]
}

// ParseOrganizationHTML достаёт ИНН/КПП/название из карточки организации 44-ФЗ.
func ParseOrganizationHTML(html []byte) (inn, kpp, fullName string, err error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(html))
	if err != nil {
		return "", "", "", err
	}
	// Шапка registry-entry
	doc.Find(".registry-entry__body-title").Each(func(_ int, s *goquery.Selection) {
		title := textutil.CleanSpace(s.Text())
		val := textutil.CleanSpace(s.Parent().Find(".registry-entry__body-value").First().Text())
		if textutil.ContainsLabel(title, "ИНН") && inn == "" {
			inn = val
		}
		if textutil.ContainsLabel(title, "КПП") && kpp == "" {
			kpp = val
		}
	})
	// Вкладка info — section labels (первый ИНН организации, не родителя)
	doc.Find("#tab-info section.blockInfo__section").Each(func(_ int, s *goquery.Selection) {
		title := textutil.CleanSpace(s.Find(".section__title").First().Text())
		info := textutil.CleanSpace(s.Find(".section__info").First().Text())
		switch {
		case textutil.ContainsLabel(title, "Полное наименование") && fullName == "":
			fullName = info
		case title == "ИНН" || textutil.NormalizeLabel(title) == "инн":
			if inn == "" {
				inn = info
			}
		case title == "КПП" || textutil.NormalizeLabel(title) == "кпп":
			if kpp == "" {
				kpp = info
			}
		}
	})
	// fallback: любой section ИНН в #tab-info
	if inn == "" {
		doc.Find("#tab-info .section__title").EachWithBreak(func(_ int, s *goquery.Selection) bool {
			if textutil.NormalizeLabel(s.Text()) == "инн" {
				inn = textutil.CleanSpace(s.Parent().Find(".section__info").First().Text())
				return false
			}
			return true
		})
	}
	return inn, kpp, fullName, nil
}
