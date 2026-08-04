package fz44

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

// ParseDocumentsHTML извлекает уникальные вложения filestore со вкладки «Документы».
func ParseDocumentsHTML(html []byte) ([]models.DocumentFile, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("parse documents html: %w", err)
	}

	seen := make(map[string]struct{})
	var out []models.DocumentFile

	doc.Find(`a[href*="filestore"][href*="uid="]`).Each(func(_ int, a *goquery.Selection) {
		href, ok := a.Attr("href")
		if !ok || href == "" {
			return
		}
		low := strings.ToLower(href)
		if strings.Contains(low, "signview") || strings.Contains(low, "crypto") {
			return
		}
		// часть ссылок без /download/ — всё равно filestore+uid
		if !strings.Contains(low, "download") && !strings.Contains(low, "file.html") && !strings.Contains(low, "get.html") {
			// keep if clearly filestore download endpoint variants
			if !strings.Contains(low, "filestore") {
				return
			}
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
		// иконка типа рядом со ссылкой (docx.svg / pdf.svg)
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
		// запасной поиск в ближайшем блоке строки
		if extHint == "" {
			row := a.Closest("tr, .attachment, .noticeDocsPad, .blockInfo__section")
			row.Find("img[src*='icons/type']").Each(func(_ int, img *goquery.Selection) {
				if extHint != "" {
					return
				}
				if src, ok := img.Attr("src"); ok {
					extHint = filemeta.ExtFromIconSrc(src)
				}
			})
		}
		name = filemeta.EnsureExt(sanitizeFilename(name), extHint, "")

		group := ""
		edition := ""
		block := a.Closest(".notice-documents, .blockInfo__section, .blockInfo")
		if block.Length() > 0 {
			group = textutil.CleanSpace(block.Closest(".blockInfo").Find("h2.blockInfo__title").First().Text())
			block.Find(".section__attrib, .section__value").Each(func(_ int, s *goquery.Selection) {
				t := textutil.CleanSpace(s.Text())
				if strings.Contains(t, "Действующая") || strings.Contains(t, "Недействующая") {
					edition = t
				}
			})
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
	q := u.Query().Get("uid")
	if q != "" {
		return strings.ToUpper(q)
	}
	// fallback path
	base := path.Base(u.Path)
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
	_ = base
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
