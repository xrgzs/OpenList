package seewo_pinco

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/OpenListTeam/OpenList/v4/drivers/base"
	"github.com/OpenListTeam/OpenList/v4/pkg/cookie"
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

const EN5_VERSION = "5.2.4.9454"

func (d *SeewoPinco) signLottery() error {
	token := cookie.GetCookie(d.client.Cookies, "x-token").Value
	if token == "" {
		return fmt.Errorf("sign lottery error: no x-token cookie found")
	}

	headers := map[string]string{
		"Host":               "easinote.seewo.com",
		"Connection":         "keep-alive",
		"sec-ch-ua":          "\"Chromium\";v=\"101\"",
		"Accept":             "*/*",
		"X-Requested-With":   "XMLHttpRequest",
		"sec-ch-ua-mobile":   "?0",
		"User-Agent":         "Mozilla/5.0 (Windows NT 10.0; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/101.0.4951.67 EasiNote5/" + EN5_VERSION + " Safari/537.36",
		"sec-ch-ua-platform": "\"Windows\"",
		"Accept-Language":    "zh-CN,zh;q=0.9",
		"Sec-Fetch-Site":     "same-origin`",
		"Sec-Fetch-Mode":     "cors",
		"Sec-Fetch-Dest":     "empty",
		"Referer":            "https://easinote.seewo.com/embedpc/mission?type=1",
		"Accept-Encoding":    "gzip, deflate, br",
		"Cookie":             "x-token=" + token + "; x-auth-token=" + token,
	}

	// Step 1: Check today's sign record
	var recordResp MissionTodayRecordResp
	client := base.NewRestyClient()
	req := client.R().
		SetQueryParam("api", "MISSION_TODAY_RECORD").
		SetQueryParam("_", fmt.Sprintf("%d", time.Now().UnixMicro())).
		SetHeaders(headers).
		SetResult(&recordResp)

	res, err := req.Get("https://easinote.seewo.com/embedpc/com/apis")
	if err != nil {
		return err
	}
	if res.IsError() {
		return fmt.Errorf("sign lottery error: %s", res.Status())
	}
	if recordResp.ErrorCode != 0 {
		return fmt.Errorf("sign lottery error: %d %s", recordResp.ErrorCode, recordResp.Message)
	}

	// Check if already signed today
	if recordResp.Data.SignRecord.BeenSigned {
		fmt.Printf("希沃白板今日已签到, 当前连续签到第%d天\n",
			recordResp.Data.SignRecord.CurrentDay)
		return nil
	}

	// Step 2: Perform sign in
	var signResp MissionSignResp
	signReq := client.R().
		SetQueryParam("api", "MISSION_SIGN_LOTTERY").
		SetQueryParam("_", fmt.Sprintf("%d", time.Now().UnixMicro())).
		SetHeaders(headers).
		SetResult(&signResp)

	signRes, err := signReq.Get("https://easinote.seewo.com/embedpc/com/apis")
	if err != nil {
		return err
	}
	if signRes.IsError() {
		return fmt.Errorf("sign lottery error: %s", signRes.Status())
	}
	if signResp.ErrorCode != 0 {
		return fmt.Errorf("sign lottery error: %d %s", signResp.ErrorCode, signResp.Message)
	}

	fmt.Printf("已为您在希沃白板签到, 当前连续签到第%d天, 获得%s和%s\n",
		signResp.Data.SignRecord.CurrentDay,
		signResp.Data.SignRecord.PrizeName,
		signResp.Data.LotteryRecord.PrizeName)
	return nil
}
