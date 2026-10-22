package db

import (
	"net/netip"
	"path/filepath"

	"github.com/OpenListTeam/OpenList/v4/cmd/flags"
	"github.com/OpenListTeam/OpenList/v4/pkg/utils"
	"github.com/ipipdotnet/ipdb-go"
	"github.com/oschwald/geoip2-golang/v2"
	log "github.com/sirupsen/logrus"
)

type GeoIP2Reader struct {
	asn   *geoip2.Reader
	qqwry *ipdb.City
	// country *geoip2.Reader
}

func (g GeoIP2Reader) HasData() bool {
	return g.asn != nil || g.qqwry != nil
}

func (g GeoIP2Reader) ASN(ipAddress netip.Addr) (*geoip2.ASN, error) {
	return g.asn.ASN(ipAddress)
}

func (g GeoIP2Reader) QQWry(addr string, language string) (*ipdb.CityInfo, error) {
	return g.qqwry.FindInfo(addr, language)
}

var geo GeoIP2Reader

// need to check if geo is nil
func GetIPDB() GeoIP2Reader {
	return geo
}

func initIPDB() {
	asnPath := filepath.Join(flags.DataDir, "GeoLite2-ASN.mmdb")
	// https://github.com/P3TERX/GeoLite.mmdb/raw/download/GeoLite2-ASN.mmdb
	loadGeoDB(&geo.asn, "ASN", asnPath)
	// countryPath := filepath.Join(flags.DataDir, "GeoLite2-Country.mmdb")
	// // https://github.com/P3TERX/GeoLite.mmdb/raw/download/GeoLite2-Country.mmdb
	// // https://raw.githubusercontent.com/Loyalsoldier/geoip/release/Country-without-asn.mmdb
	// loadGeoDB(&geo.country, "Country", countryPath)
	qqwryPath := filepath.Join(flags.DataDir, "qqwry.ipdb")
	// https://cdn.jsdelivr.net/npm/qqwry.ipdb/qqwry.ipdb
	loadIPDB(&geo.qqwry, qqwryPath)
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

func loadIPDB(reader **ipdb.City, path string) bool {
	var err error
	if *reader != nil {
		return true
	}
	if utils.Exists(path) {
		*reader, err = ipdb.NewCity(path)
		if err != nil {
			log.Errorf("failed to open ipdb city database: %s", err.Error())
			return false
		}
		log.Infof("ipdb city database loaded from %s", path)
		return true
	}
	log.Warnf("can not find ipdb city database: %s", path)
	return false
}

func closeIPDB() {
	if geo.HasData() {
		log.Info("closing geoip2")
		if geo.asn != nil {
			err := geo.asn.Close()
			if err != nil {
				log.Errorf("failed to close geoip2 ASN: %s", err.Error())
			}
		}
		if geo.qqwry != nil {
			geo.qqwry = nil
		}
	}
}
