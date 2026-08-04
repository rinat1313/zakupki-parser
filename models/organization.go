package models

// Organization44 — /epz/organization/view/info.html
type Organization44 struct {
	OrganizationCode string `json:"organization_code"` // УУН
	OrganizationID   string `json:"organization_id"`
	INN              string `json:"inn"`
	KPP              string `json:"kpp"`
	OGRN             string `json:"ogrn"`
	FullName         string `json:"full_name"`
	ShortName        string `json:"short_name"`
	SvodnyReestrCode string `json:"svodny_reestr_code"`
	IKU              string `json:"iku"`
	IKO              string `json:"iko"`
	OKTMO            string `json:"oktmo"`
	Address          string `json:"address"`
	RegDateRaw       string `json:"reg_date_raw"`
	LastChangeRaw    string `json:"last_change_raw"`
	OwnershipCode    string `json:"ownership_code"`
	OKOPFCode        string `json:"okopf_code"`
	OKPO             string `json:"okpo"`
	Level            string `json:"level"`
	OrgType          string `json:"org_type"`
	Phone            string `json:"phone"`
	Email            string `json:"email"`
	ContactPerson    string `json:"contact_person"`
	Timezone         string `json:"timezone"`
	ParentINN        string `json:"parent_inn,omitempty"`
	ParentKPP        string `json:"parent_kpp,omitempty"`
	ParentName       string `json:"parent_name,omitempty"`
}

// Organization223 — /epz/organization/view223/info.html
type Organization223 struct {
	AgencyID      string   `json:"agency_id"`
	INN           string   `json:"inn"`
	KPP           string   `json:"kpp"`
	OGRN          string   `json:"ogrn"`
	FullName      string   `json:"full_name"`
	ShortName     string   `json:"short_name"`
	Address       string   `json:"address"`
	PostAddress   string   `json:"post_address"`
	Website       string   `json:"website"`
	Status        string   `json:"status"`
	Level         string   `json:"level"`
	OKPO          string   `json:"okpo"`
	OKATO         string   `json:"okato"`
	OKVEDMain     string   `json:"okved_main"`
	OKVEDExtra    []string `json:"okved_extra"`
	IKU           string   `json:"iku"`
	IKUAssigned   string   `json:"iku_assigned_raw"`
	ContactPerson string   `json:"contact_person"`
	Email         string   `json:"email"`
	ClauseReestr  string   `json:"purchase_clause_reestr"`
}
