package dsm_share

import (
	"github.com/OpenListTeam/OpenList/v4/internal/driver"
	"github.com/OpenListTeam/OpenList/v4/internal/op"
)

type Addition struct {
	driver.RootPath
	Address  string `json:"address" type:"string" required:"true" help:"Share host, e.g. https://nas.example.com:5001"`
	ShareID  string `json:"share_id" type:"string" required:"true" help:"Share ID, e.g. xxxxxx"`
	Password string `json:"password" type:"string"`
}

var config = driver.Config{
	Name:        "DSM Share",
	LocalSort:   true,
	NoUpload:    true,
	DefaultRoot: "/",
}

func init() {
	op.RegisterDriver(func() driver.Driver {
		return &DsmShare{}
	})
}
