package models

import "time"

// Searcher — именованная поисковая настройка пользователя (UI contract).
type Searcher struct {
	ID           string         `json:"id"`
	UserID       string         `json:"user_id,omitempty"`
	Name         string         `json:"name"`
	Config       SearcherConfig `json:"config"`
	AutoAI       bool           `json:"auto_ai"`
	TendersCount int            `json:"tenders_count"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	LastRunAt    *time.Time     `json:"last_run_at"`
}

// SearcherWrite — тело POST/PUT /api/v1/searchers.
type SearcherWrite struct {
	Name   string          `json:"name"`
	Config *SearcherConfig `json:"config"`
	AutoAI *bool           `json:"auto_ai"`
}

// SearcherRun — ответ POST .../run.
type SearcherRun struct {
	RunID      string `json:"run_id"`
	SearcherID string `json:"searcher_id"`
	Status     string `json:"status"`
	FoundCount int    `json:"found_count"`
	Message    string `json:"message"`
}

// TenderCard — карточка тендера для UI списка поисковика.
type TenderCard struct {
	RegNumber       string   `json:"reg_number"`
	ObjectName      string   `json:"object_name"`
	ApplicationEnd  string   `json:"application_end,omitempty"`
	NMCK            *float64 `json:"nmck"`
	DocsWithText    int      `json:"docs_with_text"`
	DocsTotal       int      `json:"docs_total"`
	CollectPct      int      `json:"collect_pct"`
	AIPct           int      `json:"ai_pct"`
	AnalysisStatus  string   `json:"analysis_status"`
	Recommendation  string   `json:"recommendation,omitempty"`
	CardTone        string   `json:"card_tone"`
	NoticeURL       string   `json:"notice_url,omitempty"`
	Law             string   `json:"law,omitempty"`
}

// SearchHit — сырой хит со страницы ЕИС.
type SearchHit struct {
	RegNumber      string
	ObjectName     string
	NoticeURL      string
	NMCK           *float64
	ApplicationEnd string
	Law            string
}
