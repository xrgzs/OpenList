package fnos_share

import (
	"github.com/OpenListTeam/OpenList/v4/internal/driver"
	"github.com/OpenListTeam/OpenList/v4/internal/op"
)

type Addition struct {
	driver.RootPath
	Address    string `json:"address" type:"string" required:"true" help:"Share URL, e.g. https://nas.example.com:5667/s/xxxxxx"`
	Password   string `json:"password" type:"string"`
	CustomHost string `json:"custom_host" type:"string" help:"Custom download host, e.g. down.example.com"`
}

var config = driver.Config{
	Name:        "fnOS Share",
	LocalSort:   true,
	NoUpload:    true,
	OnlyProxy:   true,
	DefaultRoot: "/",
}

func init() {
	op.RegisterDriver(func() driver.Driver {
		return &FnOsShare{}
	})
}
