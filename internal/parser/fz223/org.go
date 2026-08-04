package fz223

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"github.com/rinat1313/zakupki-parser/internal/textutil"
	"github.com/rinat1313/zakupki-parser/models"
)

// ParseOrganizationHTML разбирает учётную карточку view223 (tab=info).
func ParseOrganizationHTML(html []byte) (*models.Organization223, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("parse org html: %w", err)
	}

	org := &models.Organization223{}

	doc.Find(`a[href*="agencyId="]`).EachWithBreak(func(_ int, s *goquery.Selection) bool {
		if href, ok := s.Attr("href"); ok {
			if id := textutil.AgencyIDFromHref(href); id != "" {
				org.AgencyID = id
				return false
			}
		}
		return true
	})

	doc.Find(".registry-entry__body-title").Each(func(_ int, s *goquery.Selection) {
		title := textutil.CleanSpace(s.Text())
		val := textutil.CleanSpace(s.Parent().Find(".registry-entry__body-value").First().Text())
		switch {
		case textutil.ContainsLabel(title, "ИНН") && org.INN == "":
			org.INN = val
		case textutil.ContainsLabel(title, "КПП") && org.KPP == "":
			org.KPP = val
		case textutil.ContainsLabel(title, "ОГРН") && org.OGRN == "":
			org.OGRN = val
		}
	})

	labels := map[string]string{}
	doc.Find("section.blockInfo__section, section.section").Each(func(_ int, s *goquery.Selection) {
		title := textutil.CleanSpace(s.Find(".section__title").First().Text())
		info := textutil.CleanSpace(s.Find(".section__info").First().Text())
		if title == "" || info == "" {
			return
		}
		key := textutil.NormalizeLabel(title)
		if _, ok := labels[key]; !ok {
			labels[key] = info
		}
	})

	if v := labels["полное наименование"]; v != "" {
		org.FullName = v
	}
	if v := labels["сокращенное наименование"]; v != "" {
		org.ShortName = v
	}
	if v := labels["место нахождения"]; v != "" {
		org.Address = v
	}
	if v := labels["почтовый адрес"]; v != "" {
		org.PostAddress = v
	}
	if v := labels["код по окпо"]; v != "" {
		org.OKPO = v
	}
	if v := labels["код по окато"]; v != "" {
		org.OKATO = v
	}
	if v := labels["ику"]; v != "" {
		org.IKU = v
	}
	if v := labels["дата присвоения ику"]; v != "" {
		org.IKUAssigned = v
	}
	if v := labels["контактное лицо"]; v != "" {
		org.ContactPerson = v
	}
	if v := labels["адрес электронной почты"]; v != "" {
		org.Email = v
	}
	if v := labels["статус"]; v != "" {
		org.Status = v
	}
	if v := labels["сайт"]; v != "" {
		org.Website = v
	}
	if v, ok := labels["коды основного вида деятельности по оквед"]; ok {
		org.OKVEDMain = v
	}
	if v, ok := labels["коды дополнительного вида деятельности по оквед"]; ok {
		for _, part := range strings.Split(v, "\n") {
			part = textutil.CleanSpace(part)
			if part != "" {
				org.OKVEDExtra = append(org.OKVEDExtra, part)
			}
		}
	}
	if org.INN == "" {
		if v := labels["инн"]; v != "" {
			org.INN = v
		}
	}
	if org.KPP == "" {
		if v := labels["кпп"]; v != "" {
			org.KPP = v
		}
	}
	if org.OGRN == "" {
		if v := labels["огрн"]; v != "" {
			org.OGRN = v
		}
	}

	return org, nil
}
