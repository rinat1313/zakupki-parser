package models

// Notice44 — извещение 44-ФЗ (карточка /epz/order/notice/{type}/view/...).
type Notice44 struct {
	RegNumber      string `json:"reg_number"`
	NoticeTypePath string `json:"notice_type_path"` // ea20, ok20, ... из URL
	Law            Law    `json:"law"`              // всегда Law44
	Method         string `json:"method"`           // Электронный аукцион
	Status         string `json:"status"`           // этап
	ObjectName     string `json:"object_name"`
	NMCK           Money  `json:"nmck"`
	IKZ            string `json:"ikz"`
	Published      Dated  `json:"published"`
	Updated        Dated  `json:"updated"`
	ApplicationEnd Dated  `json:"application_end"`

	CustomerName         string `json:"customer_name"`
	OrganizationCode     string `json:"organization_code"`
	CustomerINN          string `json:"customer_inn,omitempty"` // с карточки org (добор)
	CustomerKPP          string `json:"customer_kpp,omitempty"`
	PlacerIsAuthBody     bool   `json:"placer_is_auth_body"` // уполномоченный орган vs заказчик
	ETPName              string `json:"etp_name"`
	ETPURL               string `json:"etp_url"`
	PlanPositionNumber   string `json:"plan_position_number"`
	Region               string `json:"region"`
	ContactPerson        string `json:"contact_person"`
	ContactEmail         string `json:"contact_email"`
	ContactPhone         string `json:"contact_phone"`

	SourceURL string `json:"source_url"`
}

// Notice44Result — вкладка supplier-results (если есть).
type Notice44Result struct {
	RegNumber           string  `json:"reg_number"`
	ProtocolName        string  `json:"protocol_name"`
	ProtocolURL         string  `json:"protocol_url"`
	ProtocolType        string  `json:"protocol_type"` // iea, ...
	ProtocolVersion     string  `json:"protocol_version"`
	WinnerParticipantID string  `json:"winner_participant_id"` // НЕ ИНН
	WinnerRankLabel     string  `json:"winner_rank_label"`
	BidPrice            Money   `json:"bid_price"`
	ResultFormedAt      Dated   `json:"result_formed_at"`
	ContractInfoPresent bool    `json:"contract_info_present"`
}

// PurchaseObject44 — строка таблицы объекта закупки / КТРУ.
type PurchaseObject44 struct {
	RegNumber  string  `json:"reg_number"`
	PositionCode string `json:"position_code"` // ОКПД2 / КТРУ
	Name       string  `json:"name"`
	Unit       string  `json:"unit"`
	Qty        float64 `json:"qty"`
	UnitPrice  Money   `json:"unit_price"`
	Amount     Money   `json:"amount"`
}
