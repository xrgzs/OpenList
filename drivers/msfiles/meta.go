package msfiles

import (
	"github.com/OpenListTeam/OpenList/v4/internal/driver"
	"github.com/OpenListTeam/OpenList/v4/internal/op"
)

type Addition struct {
	driver.RootID
	DummyFile bool `json:"dummy_file" default:"false" help:"return a dummy file for rapid upload"`
}

var config = driver.Config{
	Name:        "Microsoft Files",
	LocalSort:   true,
	NoUpload:    true,
	DefaultRoot: "category",
}

func init() {
	op.RegisterDriver(func() driver.Driver {
		return &MSFiles{}
	})
}
