package models

// PriceRequest — карточка «Запросы цен» (/epz/pricereq/...), не извещение 44/223.
// Используется для сбора ценовой информации (часто для расчёта НМЦК), без обязательств заказчика.
type PriceRequest struct {
	ReestrNumber         string `json:"reestr_number"`
	PriceRequestInfoID   string `json:"price_request_info_id,omitempty"`
	Law                  Law    `json:"law"` // LawPricereq
	Status               string `json:"status"`
	ObjectName           string `json:"object_name"`
	Region               string `json:"region,omitempty"`
	OrgName              string `json:"org_name"`
	OrganizationCode     string `json:"organization_code,omitempty"`
	OrgAuthority         string `json:"org_authority,omitempty"` // Заказчик / …
	CustomerINN          string `json:"customer_inn,omitempty"`
	CustomerKPP          string `json:"customer_kpp,omitempty"`
	Published            Dated  `json:"published"`
	Updated              Dated  `json:"updated"`
	PriceInfoFrom        Dated  `json:"price_info_from"`
	PriceInfoTo          Dated  `json:"price_info_to"`
	PriceInfoPeriodRaw   string `json:"price_info_period_raw,omitempty"`
	PurchasePeriodRaw    string `json:"purchase_period_raw,omitempty"`
	ContactPerson        string `json:"contact_person,omitempty"`
	ContactEmail         string `json:"contact_email,omitempty"`
	ContactPhone         string `json:"contact_phone,omitempty"`
	Requirements         string `json:"requirements,omitempty"`
	Items                []PriceRequestItem `json:"items,omitempty"`
	SourceURL            string `json:"source_url"`
}

// PriceRequestItem — позиция объекта запроса цен (ОКПД2/КТРУ).
type PriceRequestItem struct {
	Code string  `json:"code,omitempty"`
	Name string  `json:"name"`
	Unit string  `json:"unit,omitempty"`
	Qty  float64 `json:"qty,omitempty"`
}
