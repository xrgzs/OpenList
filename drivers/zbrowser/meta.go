package zbrowser

import (
	"github.com/OpenListTeam/OpenList/v4/internal/driver"
	"github.com/OpenListTeam/OpenList/v4/internal/op"
)

type Addition struct {
	driver.RootID
	Cookie string `json:"cookie" required:"true"`
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
