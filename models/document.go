package models

// NoticeDocuments — пакет документов одной закупки.
type NoticeDocuments struct {
	Law    Law            `json:"law"`
	Number string         `json:"number"` // regNumber или purchaseNoticeNumber
	GUID   string         `json:"guid,omitempty"`
	Files  []DocumentFile `json:"files"`
}

// CustomerIDAlias — связь разных ID одного заказчика (см. result/07, over/info.txt).
type CustomerIDAlias struct {
	INN              string `json:"inn"`
	OrganizationID   string `json:"organization_id,omitempty"`   // 44
	OrganizationCode string `json:"organization_code,omitempty"` // 44
	AgencyID         string `json:"agency_id,omitempty"`         // 223
}
