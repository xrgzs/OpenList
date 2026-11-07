package geegeng

import (
	"github.com/OpenListTeam/OpenList/v4/internal/driver"
	"github.com/OpenListTeam/OpenList/v4/internal/op"
)

type Addition struct {
	driver.RootID
	// define other
	Token    string `json:"token" required:"true"`
	Cookie   string `json:"cookie" required:"true"`
	CustomUA string `json:"custom_ua"`
}

var config = driver.Config{
	Name:        "Geegeng",
	LocalSort:   true,
	DefaultRoot: "-1",
	Alert:       "",
}

func init() {
	op.RegisterDriver(func() driver.Driver {
		return &Geegeng{}
	})
}
