package op

import (
	"net/netip"

	"github.com/OpenListTeam/OpenList/v4/internal/db"
	"github.com/OpenListTeam/OpenList/v4/internal/model"

	log "github.com/sirupsen/logrus"
)

func GetIPInfo(ip string) model.IPInfo {
	var out model.IPInfo
	out.IP = ip
	// parse ip for geoip2
	nip, err := netip.ParseAddr(ip)
	if err != nil {
		log.Warnf("cannot parse ip address %s: %s", ip, err.Error())
	} else {
		geo := db.GetIPDB()
		if !geo.HasData() {
			log.Warn("geoip2 database is not initialized")
		} else {
			// get geo info
			geoinfo, err := geo.ASN(nip)
			if err != nil || geoinfo == nil {
				log.Warnf("cannot get geoip2 ASN info for ip %s: %s", ip, err.Error())
			} else {
				out.Asn = geoinfo.AutonomousSystemNumber
				out.Aso = geoinfo.AutonomousSystemOrganization
			}
			qqinfo, err := geo.QQWry(ip, "CN")
			if err != nil {
				log.Warnf("cannot get qqwry info for ip %s: %s", ip, err.Error())
			} else {
				out.Country = qqinfo.CountryName
				out.Region = qqinfo.RegionName
				out.City = qqinfo.CityName
				out.District = qqinfo.DistrictName
				out.Owner = qqinfo.OwnerDomain
				out.Isp = qqinfo.IspDomain
				out.CountryCode = qqinfo.CountryCode
				out.ContinentCode = qqinfo.ContinentCode
			}
		}
	}
	return out
}
