package zbrowser

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-resty/resty/v2"
)

// do others that not defined in Driver interface

func (d *ZBrowser) apiRequest(ctx context.Context, path string, value any, resp any) (*resty.Response, error) {
	if d.client == nil {
		return nil, fmt.Errorf("[ZBrowser] client not initialized")
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("[ZBrowser] API request marshal error: %w", err)
	}
	cookie := fmt.Sprintf("__guid=%s; Q=%s; __NS_Q=%s; T=%s; __NS_T=%s", d.GUID, d.Q, d.Q, d.T, d.T)
	res, err := d.client.NewRequest().
		SetContext(ctx).
		SetHeader("Content-Type", "application/x-www-form-urlencoded").
		SetHeader("cookie", cookie).
		SetQueryParams(map[string]string{
			"mid":       d.MID,
			"m2":        d.MID,
			"ver":       "1.0.1182.0",
			"timestamp": fmt.Sprintf("%d", time.Now().UnixMilli()),
			"from":      "7",
		}).
		SetFormData(map[string]string{
			"d": string(data),
		}).Post(path)
	if err != nil {
		return nil, err
	}
	if res.IsError() {
		return nil, fmt.Errorf("[ZBrowser] API error: %s, body: %s", res.Status(), res.String())
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
