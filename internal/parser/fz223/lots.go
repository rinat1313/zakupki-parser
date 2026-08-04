package fz223

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"github.com/rinat1313/zakupki-parser/internal/textutil"
	"github.com/rinat1313/zakupki-parser/models"
)

// ParseLotListHTML разбирает вкладку lot-list 223-ФЗ.
func ParseLotListHTML(html []byte, purchaseNoticeNumber string) ([]models.Lot223, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("parse lot-list html: %w", err)
	}

	var lots []models.Lot223
	doc.Find(`a[href*="lot-info.html"][href*="lotGuid="]`).Each(func(_ int, a *goquery.Selection) {
		href, ok := a.Attr("href")
		if !ok {
			return
		}
		lotGUID := textutil.LotGUIDFromHref(href)
		if lotGUID == "" {
			return
		}
		// dedupe by guid
		for _, existing := range lots {
			if existing.LotGUID == lotGUID {
				return
			}
		}

		tr := a.Closest("tr")
		rawName := textutil.CleanSpace(a.Text())
		lotNum := ""
		lotName := rawName
		if parts := strings.SplitN(rawName, "\n", 2); len(parts) == 2 {
			lotNum = textutil.CleanSpace(parts[0])
			lotName = textutil.CleanSpace(parts[1])
		} else if i := strings.Index(rawName, " "); i > 0 && looksLikeLotNumber(rawName[:i]) {
			// fallback: "1 название..."
			lotNum = rawName[:i]
			lotName = textutil.CleanSpace(rawName[i:])
		}

		lot := models.Lot223{
			LotGUID:              lotGUID,
			PurchaseNoticeNumber: purchaseNoticeNumber,
			LotNumber:            lotNum,
			LotName:              lotName,
		}

		tds := tr.Find("td")
		if tds.Length() >= 2 {
			central := textutil.NormalizeLabel(tds.Eq(1).Text())
			lot.Centralized = central == "да" || central == "yes"
		}
		if tds.Length() >= 3 {
			priceText := textutil.CleanSpace(tds.Eq(2).Text())
			// «Начальная (максимальная) цена договора: 115 000,00 ₽»
			if i := strings.Index(priceText, ":"); i >= 0 {
				priceText = textutil.CleanSpace(priceText[i+1:])
			}
			lot.Price = textutil.ParseMoney(priceText)
		}
		if tds.Length() >= 4 {
			lot.OKPD2 = splitCodes(tds.Eq(3).Text())
		}
		if tds.Length() >= 5 {
			lot.OKVED2 = splitCodes(tds.Eq(4).Text())
		}

		lots = append(lots, lot)
	})

	return lots, nil
}

func looksLikeLotNumber(s string) bool {
	s = textutil.CleanSpace(s)
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func splitCodes(raw string) []string {
	raw = strings.ReplaceAll(raw, "\r", "\n")
	var out []string
	for _, line := range strings.Split(raw, "\n") {
		line = textutil.CleanSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}
