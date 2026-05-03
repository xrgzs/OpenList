package geegeng

import (
	"github.com/OpenListTeam/OpenList/v4/internal/driver"
	"github.com/OpenListTeam/OpenList/v4/internal/op"
)

type Addition struct {
	// Usually one of two
	driver.RootPath
	driver.RootID
	// define other
	Field string `json:"field" type:"select" required:"true" options:"a,b,c" default:"a"`
}

var config = driver.Config{
	Name:        "Geegeng",
	LocalSort:   true,
	DefaultRoot: "root, / or other",
	Alert:       "",
}

func init() {
	op.RegisterDriver(func() driver.Driver {
		return &Geegeng{}
	})
}
