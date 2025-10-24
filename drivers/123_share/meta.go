package _123Share

import (
	"github.com/OpenListTeam/OpenList/v4/internal/driver"
	"github.com/OpenListTeam/OpenList/v4/internal/op"
)

type Addition struct {
	ShareKey string `json:"sharekey" required:"true"`
	SharePwd string `json:"sharepassword"`
	driver.RootID
	//OrderBy        string `json:"order_by" type:"select" options:"file_name,size,update_at" default:"file_name"`
	//OrderDirection string `json:"order_direction" type:"select" options:"asc,desc" default:"asc"`
	AccessToken string `json:"accesstoken" type:"text"`
	TempDirID   int64  `json:"temp_dir_id" type:"number" default:"0" help:"Directory ID for transfer share files. (123Open Only)"`
}

var config = driver.Config{
	Name:          "123PanShare",
	LocalSort:     true,
	NoUpload:      true,
	DefaultRoot:   "0",
	LinkCacheMode: driver.LinkCacheIP,
}

func init() {
	op.RegisterDriver(func() driver.Driver {
		return &Pan123Share{}
	})
}
