package zbrowser

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-resty/resty/v2"
)

// do others that not defined in Driver interface

func (d *ZBrowser) apiRequest(ctx context.Context, path string, value any, resp any) (*resty.Response, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("[ZBrowser] API request marshal error: %w", err)
	}
	res, err := d.client.NewRequest().
		SetContext(ctx).
		SetFormData(map[string]string{
			"d": string(data),
		}).Post(path)
	if err != nil {
		return nil, err
	}
	if res.IsError() {
		return nil, fmt.Errorf("[ZBrowser] API error: %s", res.Status())
	}
	if resp != nil {
		var base baseResp
		if err := json.Unmarshal(res.Body(), &base); err != nil {
			return nil, fmt.Errorf("[ZBrowser] API base response unmarshal error: %w", err)
		}
		if base.Code != 0 {
			return nil, fmt.Errorf("[ZBrowser] API error: %s", base.Msg)
		}
		if err := json.Unmarshal(res.Body(), resp); err != nil {
			return nil, fmt.Errorf("[ZBrowser] API response unmarshal error: %w", err)
		}
	}
	return res, nil
}
