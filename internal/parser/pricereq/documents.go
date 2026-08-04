package pricereq

import (
	"bytes"
	"fmt"
	"net/url"
	"path"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"github.com/rinat1313/zakupki-parser/internal/filemeta"
	"github.com/rinat1313/zakupki-parser/internal/textutil"
	"github.com/rinat1313/zakupki-parser/models"
)

// ParseDocumentsHTML — вложения вкладки документов запроса цен.
// Логика шире, чем у 44-ФЗ: часть ссылок filestore без сегмента /download/.
func ParseDocumentsHTML(html []byte) ([]models.DocumentFile, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("parse pricereq documents html: %w", err)
	}

	seen := make(map[string]struct{})
	var out []models.DocumentFile

	doc.Find(`a[href]`).Each(func(_ int, a *goquery.Selection) {
		href, ok := a.Attr("href")
		if !ok || href == "" {
			return
		}
		low := strings.ToLower(href)
		if !strings.Contains(low, "filestore") || !strings.Contains(low, "uid=") {
			return
		}
		if strings.Contains(low, "signview") || strings.Contains(low, "crypto") || strings.Contains(low, "javascript:") {
			return
		}
		abs := href
		if strings.HasPrefix(href, "/") {
			abs = "https://zakupki.gov.ru" + href
		}
		uid := uidFromURL(abs)
		if uid == "" {
			return
		}
		if _, dup := seen[uid]; dup {
			return
		}
		seen[uid] = struct{}{}

		name := ""
		if t, ok := a.Attr("title"); ok {
			name = textutil.CleanSpace(t)
		}
		if name == "" {
			name = textutil.CleanSpace(a.Text())
		}
		if name == "" {
			name = uid
		}

		extHint := ""
		a.Parent().Find("img[src*='icons/type'], img[alt]").Each(func(_ int, img *goquery.Selection) {
			if extHint != "" {
				return
			}
			if src, ok := img.Attr("src"); ok {
				extHint = filemeta.ExtFromIconSrc(src)
			}
			if extHint == "" {
				if alt, ok := img.Attr("alt"); ok {
					extHint = filemeta.ExtFromAlt(alt)
				}
			}
		})
		if extHint == "" {
			row := a.Closest("tr, .attachment, .noticeDocsPad, .blockInfo__section, li, .card-attachments")
			row.Find("img[src*='icons/type'], img[alt]").Each(func(_ int, img *goquery.Selection) {
				if extHint != "" {
					return
				}
				if src, ok := img.Attr("src"); ok {
					extHint = filemeta.ExtFromIconSrc(src)
				}
				if extHint == "" {
					if alt, ok := img.Attr("alt"); ok {
						extHint = filemeta.ExtFromAlt(alt)
					}
				}
			})
		}
		name = filemeta.EnsureExt(sanitizeFilename(name), extHint, "")

		group := "Прикрепленные файлы"
		edition := ""
		block := a.Closest(".notice-documents, .blockInfo__section, .blockInfo")
		if block.Length() > 0 {
			if t := textutil.CleanSpace(block.Closest(".blockInfo").Find("h2.blockInfo__title").First().Text()); t != "" {
				group = t
			}
		}

		out = append(out, models.DocumentFile{
			UID:        uid,
			Filename:   name,
			URL:        abs,
			GroupTitle: group,
			Edition:    edition,
		})
	})

	return out, nil
}

func uidFromURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	if q := u.Query().Get("uid"); q != "" {
		return strings.ToUpper(q)
	}
	if strings.Contains(strings.ToLower(raw), "uid=") {
		parts := strings.Split(raw, "uid=")
		if len(parts) > 1 {
			id := parts[1]
			if i := strings.IndexAny(id, "&\"'"); i >= 0 {
				id = id[:i]
			}
			return strings.ToUpper(id)
		}
	}
	_ = path.Base(u.Path)
	return ""
}

func sanitizeFilename(name string) string {
	name = textutil.CleanSpace(name)
	repl := strings.NewReplacer(
		"/", "_", "\\", "_", ":", "_", "*", "_", "?", "_",
		"\"", "_", "<", "_", ">", "_", "|", "_",
	)
	name = repl.Replace(name)
	if name == "" {
		return "file"
	}
	return name
}
