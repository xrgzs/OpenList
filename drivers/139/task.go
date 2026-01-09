package _139

import (
	"fmt"
	"net/http"

	"github.com/OpenListTeam/OpenList/v4/pkg/utils"
)

// autoDailyTasks runs a set of daily automated tasks for the 139 driver.
func (d *Yun139) autoDailyTasks() error {
	// Ensure Authorization exists
	if d.getAuthorization() == "" {
		if _, err := d.loginWithPassword(); err != nil {
			return fmt.Errorf("loginWithPassword failed: %w", err)
		}
	}

	// Refresh token to make sure requestts will work
	if err := d.refreshToken(); err != nil {
		return err
	}

	// 参考: https://github.com/unify-z/caiyun-autosign/blob/master/main.py

	// Check sign status
	infoUrl := "https://caiyun.feixin.10086.cn/market/signin/page/infoV2?client=mini"
	body, err := d.request(infoUrl, http.MethodGet, nil, nil)
	if err != nil {
		utils.Log.Errorf("[139] autoDailyTasks failed to query sign info: %v", err)
	} else {
		today := utils.Json.Get(body, "result", "todaySignIn").ToBool()
		if today {
			utils.Log.Infof("[139] autoDailyTasks 今日已签到")
		} else {
			// perform sign
			signUrl := "https://caiyun.feixin.10086.cn/market/manager/commonMarketconfig/getByMarketRuleName?marketName=sign_in_3"
			sb, serr := d.request(signUrl, http.MethodGet, nil, nil)
			if serr != nil {
				utils.Log.Errorf("[139] autoDailyTasks signin request failed: %v", serr)
			} else {
				msg := utils.Json.Get(sb, "msg").ToString()
				if msg == "success" || utils.Json.Get(sb, "success").ToBool() {
					utils.Log.Infof("[139] autoDailyTasks 签到成功")
				} else {
					utils.Log.Errorf("[139] autoDailyTasks 签到响应: %s", string(sb))
				}
			}
		}
	}

	// Cloud status (待领取云朵)
	recvUrl := "https://caiyun.feixin.10086.cn/market/signin/page/receive"
	rb, rerr := d.request(recvUrl, http.MethodGet, nil, nil)
	if rerr != nil {
		utils.Log.Errorf("[139] autoDailyTasks query cloud status failed: %v", rerr)
	} else {
		clouds := utils.Json.Get(rb, "result", "receive").ToString()
		total := utils.Json.Get(rb, "result", "total").ToString()
		utils.Log.Infof("[139] autoDailyTasks 当前待领取云朵: %s", clouds)
		utils.Log.Infof("[139] autoDailyTasks 当前云朵数量: %s", total)
	}
	return nil
}
