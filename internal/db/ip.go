package db

import (
	"net/netip"
	"path/filepath"

	"github.com/OpenListTeam/OpenList/v4/cmd/flags"
	"github.com/OpenListTeam/OpenList/v4/pkg/utils"
	"github.com/oschwald/geoip2-golang/v2"
	log "github.com/sirupsen/logrus"
)

type GeoIP2Reader struct {
	asn     *geoip2.Reader
	country *geoip2.Reader
}

func (g GeoIP2Reader) HasData() bool {
	return g.asn != nil || g.country != nil
}

func (g GeoIP2Reader) ASN(ipAddress netip.Addr) (*geoip2.ASN, error) {
	return g.asn.ASN(ipAddress)
}

func (g GeoIP2Reader) Country(ipAddress netip.Addr) (*geoip2.Country, error) {
	return g.country.Country(ipAddress)
}

var geo GeoIP2Reader

// need to check if geo is nil
func GetGeo() GeoIP2Reader {
	return geo
}

func initGeoDB() {
	asnPath := filepath.Join(flags.DataDir, "GeoLite2-ASN.mmdb")
	// https://github.com/P3TERX/GeoLite.mmdb/raw/download/GeoLite2-ASN.mmdb
	loadGeoDB(&geo.asn, "ASN", asnPath)
	countryPath := filepath.Join(flags.DataDir, "GeoLite2-Country.mmdb")
	// https://github.com/P3TERX/GeoLite.mmdb/raw/download/GeoLite2-Country.mmdb
	// https://raw.githubusercontent.com/Loyalsoldier/geoip/release/Country-without-asn.mmdb
	loadGeoDB(&geo.country, "Country", countryPath)
}

func loadGeoDB(reader **geoip2.Reader, name, path string) bool {
	var err error
	if *reader != nil {
		return true
	}
	if utils.Exists(path) {
		*reader, err = geoip2.Open(path)
		if err != nil {
			log.Errorf("failed to open geoip2 %s database: %s", name, err.Error())
			return false
		}
		log.Infof("geoip2 %s database loaded from %s", name, path)
		return true
	}
	log.Warnf("can not find geoip2 %s database: %s", name, path)
	return false
}

func closeGeoDB() {
	if geo.HasData() {
		log.Info("closing geoip2")
		if geo.asn != nil {
			err := geo.asn.Close()
			if err != nil {
				log.Errorf("failed to close geoip2 ASN: %s", err.Error())
			}
		}
		if geo.country != nil {
			err := geo.country.Close()
			if err != nil {
				log.Errorf("failed to close geoip2 Country: %s", err.Error())
			}
		}
	}
}
