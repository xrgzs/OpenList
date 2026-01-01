package model

import (
	"github.com/oschwald/geoip2-golang/v2"
)

type IPInfo struct {
	IP      string          `json:"ip"`
	Asn     *geoip2.ASN     `json:"asn"`
	Country *geoip2.Country `json:"country"`
}

func (i IPInfo) HasData() bool {
	return i.Asn != nil || i.Country != nil
}
