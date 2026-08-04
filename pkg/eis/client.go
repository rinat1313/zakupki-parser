// Package eis — HTTP-клиент доступа к zakupki.gov.ru.
package eis

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const DefaultUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

const BaseURL = "https://zakupki.gov.ru"

// Client — HTTP-клиент ЕИС с retry и опциональным InsecureSkipVerify (для локальных тестов на Mac).
type Client struct {
	HTTP       *http.Client
	UserAgent  string
	MaxRetries int
	BaseURL    string
}

// NewClient создаёт клиент. insecureTLS=true соответствует curl -k.
func NewClient(insecureTLS bool) *Client {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	if insecureTLS {
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // локальные тесты ЕИС
	}
	return &Client{
		HTTP: &http.Client{
			Timeout:   90 * time.Second,
			Transport: tr,
		},
		UserAgent:  DefaultUserAgent,
		MaxRetries: 5,
		BaseURL:    BaseURL,
	}
}

// Get скачивает URL, делает retry на 429 / пустое тело / 5xx.
func (c *Client) Get(url string) ([]byte, error) {
	if c.HTTP == nil {
		c.HTTP = http.DefaultClient
	}
	if c.UserAgent == "" {
		c.UserAgent = DefaultUserAgent
	}
	if c.MaxRetries < 1 {
		c.MaxRetries = 1
	}

	var lastErr error
	for attempt := 0; attempt < c.MaxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(1<<uint(attempt-1)) * time.Second)
		}
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", c.UserAgent)
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

		resp, err := c.HTTP.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			continue
		}
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
			continue
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("HTTP %d for %s", resp.StatusCode, url)
		}
		if len(strings.TrimSpace(string(body))) == 0 {
			lastErr = fmt.Errorf("empty body")
			continue
		}
		return body, nil
	}
	return nil, fmt.Errorf("get %s: %w", url, lastErr)
}

// NoticeCommonInfoURL — URL вкладки общей информации 44-ФЗ.
func NoticeCommonInfoURL(base, noticeType, regNumber string) string {
	if base == "" {
		base = BaseURL
	}
	if noticeType == "" {
		noticeType = "ea20"
	}
	return fmt.Sprintf("%s/epz/order/notice/%s/view/common-info.html?regNumber=%s",
		strings.TrimRight(base, "/"), noticeType, regNumber)
}

// OrganizationInfoURL — карточка организации 44-ФЗ.
func OrganizationInfoURL(base, organizationCode string) string {
	if base == "" {
		base = BaseURL
	}
	return fmt.Sprintf("%s/epz/organization/view/info.html?organizationCode=%s&tab=info",
		strings.TrimRight(base, "/"), organizationCode)
}

// NoticeDocumentsURL — вкладка документов 44-ФЗ.
func NoticeDocumentsURL(base, noticeType, regNumber string) string {
	if base == "" {
		base = BaseURL
	}
	if noticeType == "" {
		noticeType = "ea20"
	}
	return fmt.Sprintf("%s/epz/order/notice/%s/view/documents.html?regNumber=%s",
		strings.TrimRight(base, "/"), noticeType, regNumber)
}

// Notice223CommonInfoURL — общая информация 223-ФЗ.
// noticeGuid можно оставить пустым: GUID часто есть в HTML вкладок.
func Notice223CommonInfoURL(base, regNumber, noticeGuid string) string {
	if base == "" {
		base = BaseURL
	}
	base = strings.TrimRight(base, "/")
	if noticeGuid != "" {
		return fmt.Sprintf("%s/epz/order/notice/notice223/common-info.html?noticeGuid=%s&regNumber=%s",
			base, noticeGuid, regNumber)
	}
	return fmt.Sprintf("%s/epz/order/notice/notice223/common-info.html?regNumber=%s",
		base, regNumber)
}

// Notice223DocumentsURL — документы 223-ФЗ (/223/filestore/...).
func Notice223DocumentsURL(base, regNumber, noticeGuid string) string {
	if base == "" {
		base = BaseURL
	}
	base = strings.TrimRight(base, "/")
	if noticeGuid != "" {
		return fmt.Sprintf("%s/epz/order/notice/notice223/documents.html?purchaseNoticeNumber=%s&noticeGuid=%s",
			base, regNumber, noticeGuid)
	}
	return fmt.Sprintf("%s/epz/order/notice/notice223/documents.html?purchaseNoticeNumber=%s",
		base, regNumber)
}

// Notice223LotListURL — список лотов 223-ФЗ.
func Notice223LotListURL(base, regNumber, noticeGuid string) string {
	if base == "" {
		base = BaseURL
	}
	base = strings.TrimRight(base, "/")
	if noticeGuid != "" {
		return fmt.Sprintf("%s/epz/order/notice/notice223/lot-list.html?purchaseNoticeNumber=%s&noticeGuid=%s",
			base, regNumber, noticeGuid)
	}
	return fmt.Sprintf("%s/epz/order/notice/notice223/lot-list.html?purchaseNoticeNumber=%s",
		base, regNumber)
}

// Organization223InfoURL — карточка организации 223-ФЗ.
// Предпочтительно agencyId; иначе inn+kpp+ogrn.
func Organization223InfoURL(base, agencyID, inn, kpp, ogrn string) string {
	if base == "" {
		base = BaseURL
	}
	base = strings.TrimRight(base, "/")
	if agencyID != "" {
		return fmt.Sprintf("%s/epz/organization/view223/info.html?agencyId=%s&tab=info", base, agencyID)
	}
	return fmt.Sprintf("%s/epz/organization/view223/info.html?inn=%s&kpp=%s&ogrn=%s&tab=info",
		base, inn, kpp, ogrn)
}

// SearchByNumberURL — точный поиск извещения (44+223) по реестровому номеру.
func SearchByNumberURL(base, regNumber string) string {
	if base == "" {
		base = BaseURL
	}
	q := url.Values{}
	q.Set("searchString", regNumber)
	q.Set("strictEqual", "true")
	q.Set("morphology", "on")
	q.Set("search-filter", "Дате размещения")
	q.Set("fz44", "on")
	q.Set("fz223", "on")
	q.Set("af", "on")
	q.Set("ca", "on")
	q.Set("pc", "on")
	q.Set("pa", "on")
	q.Set("currencyIdGeneral", "-1")
	q.Set("recordsPerPage", "_10")
	q.Set("pageNumber", "1")
	return strings.TrimRight(base, "/") + "/epz/order/extendedsearch/results.html?" + q.Encode()
}

// PriceReqCommonInfoURL — карточка запроса цен (reestrNumber).
func PriceReqCommonInfoURL(base, reestrNumber string) string {
	if base == "" {
		base = BaseURL
	}
	return fmt.Sprintf("%s/epz/pricereq/card/common-info.html?reestrNumber=%s",
		strings.TrimRight(base, "/"), reestrNumber)
}

// PriceReqDocumentsURL — вложения запроса цен.
func PriceReqDocumentsURL(base, reestrNumber string) string {
	if base == "" {
		base = BaseURL
	}
	return fmt.Sprintf("%s/epz/pricereq/card/docs.html?reestrNumber=%s",
		strings.TrimRight(base, "/"), reestrNumber)
}

// PriceReqDocumentsURLByInfoID — документы по priceRequestInfoId / priceRequestId.
func PriceReqDocumentsURLByInfoID(base, infoID string) string {
	if base == "" {
		base = BaseURL
	}
	id := strings.TrimSpace(infoID)
	return fmt.Sprintf("%s/epz/pricereq/card/docs.html?priceRequestInfoId=%s",
		strings.TrimRight(base, "/"), id)
}

// PriceReqDocumentsURLByRequestID — вариант с priceRequestId (как в карточках UI ЕИС).
func PriceReqDocumentsURLByRequestID(base, requestID string) string {
	if base == "" {
		base = BaseURL
	}
	return fmt.Sprintf("%s/epz/pricereq/card/docs.html?priceRequestId=%s",
		strings.TrimRight(base, "/"), strings.TrimSpace(requestID))
}

// SearchPricereqURL — поиск в реестре «Запросы цен».
func SearchPricereqURL(base, reestrNumber string) string {
	if base == "" {
		base = BaseURL
	}
	q := url.Values{}
	q.Set("searchString", reestrNumber)
	q.Set("morphology", "on")
	q.Set("published", "on")
	q.Set("proposed", "on")
	q.Set("ended", "on")
	q.Set("sortBy", "UPDATE_DATE")
	q.Set("pageNumber", "1")
	q.Set("sortDirection", "false")
	q.Set("recordsPerPage", "_10")
	return strings.TrimRight(base, "/") + "/epz/pricereq/search/results.html?" + q.Encode()
}

// Download сохраняет тело ответа в destPath (бинарные файлы filestore).
func (c *Client) Download(url, destPath string) (int64, error) {
	res, err := c.DownloadMeta(url)
	if err != nil {
		return 0, err
	}
	if err := writeFile(destPath, res.Body); err != nil {
		return 0, err
	}
	return int64(len(res.Body)), nil
}

// DownloadMeta скачивает файл и возвращает тело + заголовки (для имени/типа).
func (c *Client) DownloadMeta(url string) (*DownloadMeta, error) {
	if c.HTTP == nil {
		c.HTTP = http.DefaultClient
	}
	if c.UserAgent == "" {
		c.UserAgent = DefaultUserAgent
	}
	if c.MaxRetries < 1 {
		c.MaxRetries = 1
	}

	var lastErr error
	for attempt := 0; attempt < c.MaxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(1<<uint(attempt-1)) * time.Second)
		}
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", c.UserAgent)
		req.Header.Set("Accept", "*/*")

		resp, err := c.HTTP.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			continue
		}
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
			continue
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("HTTP %d for %s", resp.StatusCode, url)
		}
		if len(body) == 0 {
			lastErr = fmt.Errorf("empty body")
			continue
		}
		return &DownloadMeta{
			Body:                body,
			ContentType:         resp.Header.Get("Content-Type"),
			ContentDisposition:  resp.Header.Get("Content-Disposition"),
			Header:              resp.Header.Clone(),
		}, nil
	}
	return nil, fmt.Errorf("download %s: %w", url, lastErr)
}

// DownloadMeta — тело файла и заголовки ответа.
type DownloadMeta struct {
	Body               []byte
	ContentType        string
	ContentDisposition string
	Header             http.Header
}

