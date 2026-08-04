package models

// Notice223 — извещение 223-ФЗ (/epz/order/notice/notice223/...).
type Notice223 struct {
	PurchaseNoticeNumber string `json:"purchase_notice_number"` // = regNumber
	NoticeGUID           string `json:"notice_guid"`
	Law                  Law    `json:"law"` // Law223
	MethodHeader         string `json:"method_header"` // из шапки «Иной способ»
	MethodBody           string `json:"method_body"`   // из common-text
	Status               string `json:"status"`
	ObjectName           string `json:"object_name"`
	Revision             string `json:"revision"`
	InitialPrice         Money  `json:"initial_price"`
	Published            Dated  `json:"published"`
	RevisionPublished    Dated  `json:"revision_published"`
	Updated              Dated  `json:"updated"`
	TimezoneLabel        string `json:"timezone_label"`

	CustomerAgencyID string `json:"customer_agency_id"`
	CustomerINN      string `json:"customer_inn"`
	CustomerKPP      string `json:"customer_kpp"`
	CustomerOGRN     string `json:"customer_ogrn"`
	CustomerName     string `json:"customer_name"`
	CustomerAddress  string `json:"customer_address"`

	ContactPerson string `json:"contact_person"`
	ContactEmail  string `json:"contact_email"`
	ContactPhone  string `json:"contact_phone"`
	PlanRegNumber string `json:"plan_reg_number"`
	SourceURL     string `json:"source_url"`
}

// Lot223 — лот с вкладки lot-list / lot-info.
type Lot223 struct {
	LotGUID              string   `json:"lot_guid"`
	PurchaseNoticeNumber string   `json:"purchase_notice_number"`
	LotNumber            string   `json:"lot_number"`
	LotName              string   `json:"lot_name"`
	Centralized          bool     `json:"centralized"`
	Price                Money    `json:"price"`
	OKPD2                []string `json:"okpd2"`
	OKVED2               []string `json:"okved2"`
	DeliveryRegion       string   `json:"delivery_region"`
	DeliveryAddress      string   `json:"delivery_address"`
}
