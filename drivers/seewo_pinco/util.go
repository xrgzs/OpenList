package seewo_pinco

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

// do others that not defined in Driver interface

func (d *SeewoPinco) api(ctx context.Context, actionName string, body interface{}, out interface{}) error {
	req := d.client.R()
	req.SetContext(ctx)
	req.SetBody(body)
	req.SetQueryParam("actionName", actionName)
	req.SetHeaders(map[string]string{
		"accept":        "*/*",
		"content-type":  "application/json;charset=UTF-8",
		"x-csrf-token":  "undefined",
		"x-language":    "zh_CHS",
		"x-req-traceid": uuid.NewString(),
		"x-server":      "default",
		"referrer":      "https://pinco.seewo.com/teacher/main/drive/resource",
	})
	req.SetResult(&out)
	res, err := req.Post("https://pinco.seewo.com/teacher/api.json")
	if err != nil {
		return err
	}
	if res.IsError() {
		return fmt.Errorf("api error: %s", res.Status())
	}
	var resp BaseResp
	err = json.Unmarshal(res.Body(), &resp)
	if err != nil {
		return err
	}
	if resp.StatusCode != 0 {
		return fmt.Errorf("api error: %d %s", resp.StatusCode, resp.Message)
	}
	return nil
}
