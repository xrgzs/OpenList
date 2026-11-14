package vk

import (
	"github.com/OpenListTeam/OpenList/v4/internal/driver"
	"github.com/OpenListTeam/OpenList/v4/internal/op"
)

type Addition struct {
	// Usually one of two
	// define other
	AccessToken string `json:"access_token" type:"string" required:"true"`
}

var config = driver.Config{
	Name:      "VK",
	LocalSort: true,
	NoUpload:  true,
}

func init() {
	op.RegisterDriver(func() driver.Driver {
		return &VK{}
	})
}
