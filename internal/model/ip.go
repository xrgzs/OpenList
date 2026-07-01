package model

type IPInfo struct {
	IP string `json:"ip"`
	// geoip asn mmdb fields
	Asn uint   `json:"asn"`
	Aso string `json:"aso"`
	// qqwry fields
	Country       string `json:"country"`
	Region        string `json:"region"`
	City          string `json:"city"`
	District      string `json:"district"`
	Owner         string `json:"owner"`
	Isp           string `json:"isp"`
	CountryCode   string `json:"country_code"`
	ContinentCode string `json:"continent_code"`
}
