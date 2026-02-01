package qingque

import (
	"github.com/OpenListTeam/OpenList/v4/internal/driver"
	"github.com/OpenListTeam/OpenList/v4/internal/op"
)

type Addition struct {
	// Usually one of two
	driver.RootID
	// define other
	Cookie string `json:"cookie" type:"string" required:"true"`
}

var config = driver.Config{
	Name:              "Qingque",
	LocalSort:         true, // TODO: support cloud sort
	DefaultRoot:       "mine",
	NoOverwriteUpload: true,
}

func init() {
	op.RegisterDriver(func() driver.Driver {
		return &Qingque{}
	})
}
