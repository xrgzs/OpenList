package op

import (
	"net/netip"

	"github.com/OpenListTeam/OpenList/v4/internal/db"
	"github.com/OpenListTeam/OpenList/v4/internal/model"
)

func GetIPInfo(ip string) model.IPInfo {
	var out model.IPInfo
	out.IP = ip
	// parse ip for geoip2
	nip, err := netip.ParseAddr(ip)
	if err != nil {
		return out
	}
	geo := db.GetGeo()
	if !geo.HasData() {
		return out
	}
	out.Asn, err = geo.ASN(nip)
	if err != nil {
		return out
	}
	out.Country, err = geo.Country(nip)
	if err != nil {
		return out
	}
	return out
}
