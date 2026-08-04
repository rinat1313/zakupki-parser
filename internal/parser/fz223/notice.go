package fz223

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"github.com/rinat1313/zakupki-parser/internal/textutil"
	"github.com/rinat1313/zakupki-parser/models"
)

// ParseNoticeHTML разбирает HTML вкладки common-info 223-ФЗ в Notice223.
func ParseNoticeHTML(html []byte, sourceURL string) (*models.Notice223, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("parse html: %w", err)
	}

	n := &models.Notice223{
		Law:       models.Law223,
		SourceURL: sourceURL,
	}

	if sourceURL != "" {
		n.NoticeGUID = textutil.NoticeGUIDFromHref(sourceURL)
	}
	if n.NoticeGUID == "" {
		doc.Find(`a[href*="noticeGuid="], a[href*="purchaseNoticeGuid="]`).EachWithBreak(func(_ int, s *goquery.Selection) bool {
			if href, ok := s.Attr("href"); ok {
				if g := textutil.NoticeGUIDFromHref(href); g != "" {
					n.NoticeGUID = g
					return false
				}
			}
			return true
		})
	}

	n.MethodHeader = textutil.CleanSpace(doc.Find(".registry-entry__header-top__title").First().Text())
	n.PurchaseNoticeNumber = textutil.StripNoticeNumber(doc.Find(".registry-entry__header-mid__number").First().Text())
	n.Status = textutil.CleanSpace(doc.Find(".registry-entry__header-mid__title").First().Text())

	doc.Find(".registry-entry__body-block").Each(func(_ int, s *goquery.Selection) {
		title := textutil.CleanSpace(s.Find(".registry-entry__body-title").First().Text())
		val := textutil.CleanSpace(s.Find(".registry-entry__body-value").First().Text())
		switch {
		case textutil.ContainsLabel(title, "Объект закупки"):
			n.ObjectName = val
		case textutil.ContainsLabel(title, "Заказчик"):
			n.CustomerName = val
			if href, ok := s.Find("a[href*='view223']").Attr("href"); ok {
				fillCustomerFromHref(n, href)
				if name := textutil.CleanSpace(s.Find("a").First().Text()); name != "" {
					n.CustomerName = name
				}
			}
		}
	})

	priceRaw := textutil.CleanSpace(doc.Find(".price-block__value").First().Text())
	if priceRaw != "" {
		n.InitialPrice = textutil.ParseMoney(priceRaw)
	}

	doc.Find(".data-block__title").Each(func(_ int, s *goquery.Selection) {
		title := textutil.CleanSpace(s.Text())
		val := textutil.CleanSpace(s.Parent().Find(".data-block__value").First().Text())
		switch {
		case textutil.ContainsLabel(title, "Размещено"):
			n.Published = textutil.ParseDated(val)
		case textutil.ContainsLabel(title, "Обновлено"):
			n.Updated = textutil.ParseDated(val)
		}
	})

	labels := mapCommonTextLabels(doc)

	if v := labels["реестровый номер извещения"]; v != "" {
		n.PurchaseNoticeNumber = textutil.StripNoticeNumber(v)
	}
	if v := labels["способ осуществления закупки"]; v != "" {
		n.MethodBody = v
	}
	if v := labels["наименование закупки"]; v != "" {
		n.ObjectName = v
	}
	if v := labels["редакция"]; v != "" {
		n.Revision = v
	}
	if v := labels["дата размещения извещения"]; v != "" {
		n.Published = textutil.ParseDated(v)
	}
	if v := labels["дата размещения текущей редакции извещения"]; v != "" {
		n.RevisionPublished = textutil.ParseDated(v)
	}
	if v := labels["наименование организации"]; v != "" {
		if n.CustomerName == "" {
			n.CustomerName = v
		}
	}
	if v := labels["место нахождения"]; v != "" {
		n.CustomerAddress = v
	}
	if v := labels["контактное лицо"]; v != "" {
		n.ContactPerson = v
	}
	if v := labels["адрес электронной почты"]; v != "" {
		n.ContactEmail = v
	}
	if v := labels["контактный телефон"]; v != "" {
		n.ContactPhone = v
	}
	if v := firstLabel(labels, "позиция плана"); v != "" {
		n.PlanRegNumber = textutil.CleanSpace(strings.Split(v, "\n")[0])
	}

	parseINNKPPOGRN(doc, n)

	if n.CustomerINN == "" {
		doc.Find(`a[href*="view223"]`).EachWithBreak(func(_ int, s *goquery.Selection) bool {
			if href, ok := s.Attr("href"); ok {
				fillCustomerFromHref(n, href)
				if n.CustomerName == "" {
					n.CustomerName = textutil.CleanSpace(s.Text())
				}
				return n.CustomerINN == ""
			}
			return true
		})
	}

	if n.PurchaseNoticeNumber == "" {
		return nil, fmt.Errorf("purchase_notice_number not found in HTML")
	}
	return n, nil
}

func fillCustomerFromHref(n *models.Notice223, href string) {
	if id := textutil.AgencyIDFromHref(href); id != "" {
		n.CustomerAgencyID = id
	}
	inn, kpp, ogrn := textutil.InnKppOgrnFromHref(href)
	if inn != "" {
		n.CustomerINN = inn
	}
	if kpp != "" {
		n.CustomerKPP = kpp
	}
	if ogrn != "" {
		n.CustomerOGRN = ogrn
	}
}

func parseINNKPPOGRN(doc *goquery.Document, n *models.Notice223) {
	doc.Find(".common-text__value--gray").Each(func(_ int, s *goquery.Selection) {
		label := textutil.NormalizeLabel(s.Text())
		val := textutil.CleanSpace(s.Parent().Find(".common-text__value").Not(".common-text__value--gray").First().Text())
		if val == "" {
			val = textutil.CleanSpace(s.Next().Text())
		}
		if val == "" {
			return
		}
		switch label {
		case "инн":
			if n.CustomerINN == "" {
				n.CustomerINN = val
			}
		case "кпп":
			if n.CustomerKPP == "" {
				n.CustomerKPP = val
			}
		case "огрн":
			if n.CustomerOGRN == "" {
				n.CustomerOGRN = val
			}
		}
	})
}

func mapCommonTextLabels(doc *goquery.Document) map[string]string {
	out := make(map[string]string)
	doc.Find(".common-text__title").Each(func(_ int, s *goquery.Selection) {
		title := textutil.CleanSpace(s.Text())
		parent := s.Parent()
		val := textutil.CleanSpace(parent.Find(".common-text__value").First().Text())
		if title == "" || val == "" {
			return
		}
		key := textutil.NormalizeLabel(title)
		if prev, ok := out[key]; ok && len(prev) > 0 && len(val) > len(prev)*3 {
			return
		}
		out[key] = val
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

// ExtractNoticeGUID достаёт GUID из уже скачанного HTML (если CSV без guid).
func ExtractNoticeGUID(html []byte) string {
	return textutil.NoticeGUIDFromHref(string(html))
}
