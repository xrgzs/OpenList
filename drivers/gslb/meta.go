package gslb

import (
	"github.com/OpenListTeam/OpenList/v4/internal/driver"
	"github.com/OpenListTeam/OpenList/v4/internal/op"
)

type Addition struct {
	driver.RootPath
	Paths   string `json:"paths" type:"text" required:"true"`
	Timeout int    `json:"timeout" type:"number" default:"5"`
}

var config = driver.Config{
	Name:        "Gslb",
	LocalSort:   true,
	NoCache:     true,
	NoUpload:    true,
	DefaultRoot: "/",
}

func init() {
	op.RegisterDriver(func() driver.Driver {
		return &Gslb{}
	})
}
