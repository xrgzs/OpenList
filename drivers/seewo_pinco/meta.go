package seewo_pinco

import (
	"github.com/OpenListTeam/OpenList/v4/internal/driver"
	"github.com/OpenListTeam/OpenList/v4/internal/op"
)

type Addition struct {
	// Usually one of two
	// driver.RootPath
	driver.RootID
	// define other
	// Field string `json:"field" type:"select" required:"true" options:"a,b,c" default:"a"`
	Cookie string `json:"cookie" type:"text" required:"true"`
}

var config = driver.Config{
	Name:        "Seewo Pinco",
	LocalSort:   true,
	DefaultRoot: "0",
}

func init() {
	op.RegisterDriver(func() driver.Driver {
		return &SeewoPinco{}
	})
}
