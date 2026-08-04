package models

import "time"

// Money — нормализованная сумма из HTML (пробелы/₽ убраны при парсинге).
type Money struct {
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"` // например "RUB"
	Raw      string  `json:"raw"`      // как на сайте
}

// Dated — дата/время с хвостом таймзоны из ЕИС («МСК+5»).
type Dated struct {
	At       time.Time `json:"at"`
	TZLabel  string    `json:"tz_label"`
	Raw      string    `json:"raw"`
}

// Law — федеральный закон закупки.
type Law string

const (
	Law44       Law = "44"
	Law223      Law = "223"
	LawPricereq Law = "pricereq" // запросы цен /epz/pricereq (не извещение)
)

// DocumentFile — вложение filestore.
type DocumentFile struct {
	UID          string `json:"uid"`
	Filename     string `json:"filename"`
	URL          string `json:"url"`
	DocumentGUID string `json:"document_guid,omitempty"` // чаще 223
	GroupTitle   string `json:"group_title"`
	Edition      string `json:"edition"` // Действующая / Недействующая
	PublishedRaw string `json:"published_raw"`
}

// NoticeEvent — строка журнала событий.
type NoticeEvent struct {
	At   Dated `json:"at"`
	Text string `json:"text"`
}
