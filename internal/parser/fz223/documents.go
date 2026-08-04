package fz223

import (
	"bytes"
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"github.com/rinat1313/zakupki-parser/internal/filemeta"
	"github.com/rinat1313/zakupki-parser/internal/textutil"
	"github.com/rinat1313/zakupki-parser/models"
)

var reTooltipFile = regexp.MustCompile(`(?i)>([^<>]+\.(?:doc|docx|pdf|xls|xlsx|rtf|zip|rar|7z|odt|ods))<`)

// ParseDocumentsHTML извлекает вложения /223/filestore/ со вкладки «Документы».
func ParseDocumentsHTML(html []byte) ([]models.DocumentFile, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("parse documents html: %w", err)
	}

	seen := make(map[string]struct{})
	var out []models.DocumentFile

	doc.Find(`a[href*="/223/filestore/"][href*="uid="], a[href*="filestore"][href*="fz223"][href*="uid="]`).Each(func(_ int, a *goquery.Selection) {
		href, ok := a.Attr("href")
		if !ok || href == "" {
			return
		}
		if strings.Contains(strings.ToLower(href), "signview") || strings.Contains(strings.ToLower(href), "crypto") {
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

		name := filenameFromAnchor(a)
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
		name = filemeta.EnsureExt(sanitizeFilename(name), extHint, "")

		group := ""
		block := a.Closest(".card-attachments__block")
		if block.Length() > 0 {
			group = textutil.CleanSpace(block.Find(".title").First().Text())
		}

		docGUID := ""
		row := a.Closest(".attachment, .count, .row")
		if row.Length() > 0 {
			row.Find(`a[href*="documentGuid="]`).EachWithBreak(func(_ int, s *goquery.Selection) bool {
				if href, ok := s.Attr("href"); ok {
					if u, err := url.Parse(href); err == nil {
						if g := u.Query().Get("documentGuid"); g != "" {
							docGUID = g
							return false
						}
					}
				}
				return true
			})
		}

		out = append(out, models.DocumentFile{
			UID:          uid,
			Filename:     name,
			URL:          abs,
			DocumentGUID: docGUID,
			GroupTitle:   group,
		})
	})

	return out, nil
}

func filenameFromAnchor(a *goquery.Selection) string {
	if tip, ok := a.Attr("tip"); ok {
		if n := textutil.CleanSpace(tip); n != "" {
			return n
		}
	}
	if title, ok := a.Attr("title"); ok {
		if n := textutil.CleanSpace(title); n != "" && !strings.Contains(strings.ToLower(n), "microsoft") {
			return n
		}
	}
	if tip, ok := a.Attr("data-tooltip"); ok {
		if m := reTooltipFile.FindStringSubmatch(tip); len(m) > 1 {
			return textutil.CleanSpace(m[1])
		}
		// убрать html-теги
		plain := regexp.MustCompile(`<[^>]+>`).ReplaceAllString(tip, " ")
		plain = textutil.CleanSpace(plain)
		if plain != "" && strings.Contains(plain, ".") {
			return plain
		}
	}
	return textutil.CleanSpace(a.Text())
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
