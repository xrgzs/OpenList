package zbrowser

import (
	"github.com/OpenListTeam/OpenList/v4/internal/driver"
	"github.com/OpenListTeam/OpenList/v4/internal/op"
)

type Addition struct {
	driver.RootID
	GUID       string `json:"guid" required:"true"`
	MID        string `json:"mid" required:"true"`
	Q          string `json:"q" required:"true"`
	T          string `json:"t" required:"true"`
	DeleteMode string `json:"delete_mode" type:"select" options:"recycle,delete" default:"recycle"`
}

var config = driver.Config{
	Name:        "ZBrowser",
	LocalSort:   true,
	DefaultRoot: "0",
}

func init() {
	op.RegisterDriver(func() driver.Driver {
		return &ZBrowser{}
	})
}
