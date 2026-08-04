package models

// SavedFile — локально сохранённый документ тендера.
type SavedFile struct {
	UID            string   `json:"uid"`
	SourceURL      string   `json:"source_url"`
	OriginalName   string   `json:"original_name"`
	LocalPath      string   `json:"local_path"` // относительно папки тендера
	TextPath       string   `json:"text_path,omitempty"`
	SizeBytes      int64    `json:"size_bytes"`
	GroupTitle     string   `json:"group_title,omitempty"`
	Edition        string   `json:"edition,omitempty"`
	Extracted      []string `json:"extracted,omitempty"`
	ArchiveRemoved bool     `json:"archive_removed,omitempty"`
	TextEngine     string   `json:"text_engine,omitempty"`
	TextError      string   `json:"text_error,omitempty"` // не удалось получить полезный txt
	Error          string   `json:"error,omitempty"`
}

// FailedText — документ, из которого не удалось извлечь текст для LLM.
type FailedText struct {
	RegNumber  string `json:"reg_number"`
	SourcePath string `json:"source_path"`
	SourceName string `json:"source_name"`
	Engine     string `json:"engine,omitempty"`
	RawBytes   int    `json:"raw_bytes,omitempty"`
	Error      string `json:"error"`
	At         string `json:"at"`
}

// TenderExport — полный снимок тендера для result/{id}/.
type TenderExport struct {
	RegNumber    string            `json:"reg_number"`
	Law          Law               `json:"law"`
	FetchedAt    string            `json:"fetched_at"`
	Tender       *Notice44         `json:"tender,omitempty"`
	Tender223    *Notice223        `json:"tender_223,omitempty"`
	PriceRequest *PriceRequest     `json:"price_request,omitempty"`
	Customer     *Organization44   `json:"customer,omitempty"`
	Customer223  *Organization223  `json:"customer_223,omitempty"`
	Lots         []Lot223          `json:"lots,omitempty"`
	Files        []SavedFile       `json:"files"`
	FailedTexts  []FailedText      `json:"failed_texts,omitempty"`
}
