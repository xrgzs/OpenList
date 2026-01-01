package db

import (
	"path/filepath"

	"github.com/oschwald/geoip2-golang/v2"
	log "github.com/sirupsen/logrus"

	"github.com/OpenListTeam/OpenList/v4/cmd/flags"
	"github.com/OpenListTeam/OpenList/v4/internal/conf"
	"github.com/OpenListTeam/OpenList/v4/internal/model"
	"github.com/OpenListTeam/OpenList/v4/pkg/utils"
	"gorm.io/gorm"
)

var db *gorm.DB

var geo *geoip2.Reader

func Init(d *gorm.DB) {
	db = d
	err := AutoMigrate(new(model.Storage), new(model.User), new(model.Meta), new(model.SettingItem), new(model.SearchNode), new(model.TaskItem), new(model.SSHPublicKey), new(model.SharingDB))
	if err != nil {
		log.Fatalf("failed migrate database: %s", err.Error())
	}
	// https://github.com/P3TERX/GeoLite.mmdb/raw/download/GeoLite2-ASN.mmdb
	geoPath := filepath.Join(flags.DataDir, "GeoLite2-ASN.mmdb")
	if utils.Exists(geoPath) {
		geo, err = geoip2.Open(geoPath)
		if err != nil {
			log.Fatalf("failed to open geoip2 database: %s", err.Error())
		}
		return
	} else {
		log.Warnf("can not load geoip2 database: %s", geoPath)
	}
}

func AutoMigrate(dst ...interface{}) error {
	var err error
	if conf.Conf.Database.Type == "mysql" {
		err = db.Set("gorm:table_options", "ENGINE=InnoDB CHARSET=utf8mb4").AutoMigrate(dst...)
	} else {
		err = db.AutoMigrate(dst...)
	}
	return err
}

func GetDb() *gorm.DB {
	return db
}

// need to check if geo is nil
func GetGeoDb() *geoip2.Reader {
	return geo
}

func Close() {
	log.Info("closing db")
	sqlDB, err := db.DB()
	if err != nil {
		log.Errorf("failed to get db: %s", err.Error())
		return
	}
	err = sqlDB.Close()
	if err != nil {
		log.Errorf("failed to close db: %s", err.Error())
		return
	}
	if geo != nil {
		log.Info("closing geoip2")
		err = geo.Close()
		if err != nil {
			log.Errorf("failed to close geoip2: %s", err.Error())
			return
		}
	}
}
