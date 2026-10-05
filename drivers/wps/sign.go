package wps

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"strconv"
	"time"

	"github.com/OpenListTeam/OpenList/v4/drivers/base"
	"github.com/OpenListTeam/OpenList/v4/pkg/utils"
)

func (d *Wps) dailySign(ctx context.Context) error {
	if d.isPersonal() {
		return d.dailySignPersonal(ctx)
	}
	return d.dailySignBusiness(ctx)
}

// 自动"企业签到"领福利
func (d *Wps) dailySignBusiness(ctx context.Context) error {
	var resp struct {
		UserSignFlag int `json:"user_sign_flag"`
	}
	_, err := d.request(ctx).
		SetBody(base.Json{
			"_t": time.Now().Unix(),
		}).
		SetResult(&resp).
		Post("https://plus.wps.cn/ops/activity/api/v1/activities/signin/user-sign")
	if err != nil {
		return err
	}
	// return 201 is successful, so don't check the status code
	if resp.UserSignFlag == 1 {
		utils.Log.Infof("[WPS-%s] 今日已签到", d.GetStorage().MountPath)
		fmt.Printf("[WPS-%s] 今日已签到\n", d.GetStorage().MountPath)
	} else {
		utils.Log.Infof("[WPS-%s] 签到成功, user_sign_flag: %d", d.GetStorage().MountPath, resp.UserSignFlag)
		fmt.Printf("[WPS-%s] 签到成功, user_sign_flag: %d\n", d.GetStorage().MountPath, resp.UserSignFlag)
	}
	return nil
}

// 自动"个人签到"领福利
func (d *Wps) dailySignPersonal(ctx context.Context) error {
	var keyResp struct {
		Result string `json:"result"`
		Msg    string `json:"msg"`
		Data   string `json:"data"`
	}
	keyReq, err := d.request(ctx).
		SetHeader("Origin", "https://personal-act.wps.cn").
		SetHeader("Referer", "https://personal-act.wps.cn/").
		SetResult(&keyResp).
		SetError(&keyResp).
		Get("https://personal-bus.wps.cn/sign_in/v1/encrypt/key")
	if err != nil {
		return err
	}

	if err := checkAPI(keyReq, apiResult{Result: keyResp.Result, Msg: keyResp.Msg}); err != nil {
		return err
	}

	aesKey, err := personalAESKey(32)
	if err != nil {
		return err
	}
	plain, _ := json.Marshal(base.Json{"user_id": d.login.UserID, "platform": 64})
	extra, err := personalAESEncrypt(string(plain), aesKey)
	if err != nil {
		return err
	}
	token, err := personalRSAEncrypt(aesKey, keyResp.Data)
	if err != nil {
		return fmt.Errorf("error rsa: %e", err)
	}

	var signResp struct {
		Data struct {
			PayOrigin            string        `json:"payOrigin"`
			BoostRewardIsPreview int           `json:"boost_reward_is_preview"`
			Badges               []interface{} `json:"badges"`
			Ids                  []interface{} `json:"ids"`
			Rewards              []struct {
				ID             int    `json:"id"`
				RewardDate     int    `json:"reward_date"`
				UserID         int    `json:"user_id"`
				Channel        string `json:"channel"`
				RewardType     string `json:"reward_type"`
				Sku            string `json:"sku"`
				MbID           int    `json:"mb_id"`
				RewardName     string `json:"reward_name"`
				Second         int    `json:"second"`
				Num            int    `json:"num"`
				RewardSrc      string `json:"reward_src"`
				ExtraRewardSrc string `json:"extra_reward_src"`
				State          int    `json:"state"`
				OrderID        string `json:"order_id"`
				ExpireTime     int    `json:"expire_time"`
				Ctime          int    `json:"ctime"`
			} `json:"rewards"`
			BoostRewards interface{} `json:"boost_rewards"`
		} `json:"data"`
		Result string `json:"result"`
		Code   int    `json:"code"`
		Msg    string `json:"msg"`
	}
	r, err := d.request(ctx).
		SetHeader("Content-Type", "application/json").
		SetHeader("Origin", "https://personal-act.wps.cn").
		SetHeader("Referer", "https://personal-act.wps.cn/").
		SetHeader("token", token).
		SetBody(base.Json{
			"encrypt":    true,
			"extra":      extra,
			"pay_origin": "pc_ucs_rwzx_sign",
		}).
		SetResult(&signResp).
		SetError(&signResp).
		Post("https://personal-bus.wps.cn/sign_in/v1/sign_in")
	if err != nil {
		return err
	}
	if r != nil && r.IsError() {
		if signResp.Msg == "has sign" {
			utils.Log.Infof("[WPS-%s] 今日已签到", d.GetStorage().MountPath)
			fmt.Printf("[WPS-%s] 今日已签到\n", d.GetStorage().MountPath)
			return nil
		}
		if signResp.Msg == "" {
			return fmt.Errorf("http error: %d", r.StatusCode())
		}
		return fmt.Errorf("%s", signResp.Msg)
	}
	if signResp.Result == "ok" || signResp.Msg == "has sign" {
		if signResp.Msg == "has sign" {
			utils.Log.Infof("[WPS-%s] 今日已签到", d.GetStorage().MountPath)
			fmt.Printf("[WPS-%s] 今日已签到\n", d.GetStorage().MountPath)
		} else {

			utils.Log.Infof("[WPS-%s] 签到成功", d.GetStorage().MountPath)
			fmt.Printf("[WPS-%s] 签到成功\n", d.GetStorage().MountPath)
			if len(signResp.Data.Rewards) > 0 {
				for _, reward := range signResp.Data.Rewards {
					utils.Log.Infof("[WPS-%s] 获得奖励: %s x%d", d.GetStorage().MountPath, reward.RewardName, reward.Num)
					fmt.Printf("[WPS-%s] 获得奖励: %s x%d\n", d.GetStorage().MountPath, reward.RewardName, reward.Num)
				}
			} else {
				utils.Log.Infof("[WPS-%s] 但未获得奖励", d.GetStorage().MountPath)
				fmt.Printf("[WPS-%s] 但未获得奖励\n", d.GetStorage().MountPath)
			}
		}
		return nil
	}
	if signResp.Msg == "" {
		return fmt.Errorf("签到失败，未知错误")
	}
	return fmt.Errorf("签到失败: %s", signResp.Msg)
}

func personalAESKey(n int) (string, error) {
	if n < 10 {
		n = 10
	}
	chars := "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n-10)
	for i := range b {
		idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		if err != nil {
			return "", err
		}
		b[i] = chars[idx.Int64()]
	}
	return string(b) + strconv.FormatInt(time.Now().Unix(), 10), nil
}

func personalAESEncrypt(plain, key string) (string, error) {
	keyBytes := make([]byte, 32)
	copy(keyBytes, []byte(key))
	iv := []byte(key)
	if len(iv) < aes.BlockSize {
		return "", fmt.Errorf("invalid aes key length")
	}

	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return "", err
	}
	data := pkcs7Pad([]byte(plain), aes.BlockSize)
	out := make([]byte, len(data))
	cipher.NewCBCEncrypter(block, iv[:aes.BlockSize]).CryptBlocks(out, data)
	return base64.StdEncoding.EncodeToString(out), nil
}

func personalRSAEncrypt(plain, pubKeyBase64 string) (string, error) {
	pemData, err := base64.StdEncoding.DecodeString(pubKeyBase64)
	if err != nil {
		return "", err
	}
	block, _ := pem.Decode(pemData)
	if block == nil {
		return "", fmt.Errorf("invalid public key")
	}
	pub, err := x509.ParsePKCS1PublicKey(block.Bytes)
	if err != nil {
		return "", err
	}
	// enc, err := rsa.EncryptPKCS1v15(rand.Reader, pub, []byte(plain))
	// if err != nil {
	// 	return "", err
	// }
	k := (pub.N.BitLen() + 7) / 8
	if len(plain) > k-11 {
		return "", fmt.Errorf("message too long")
	}
	psLen := k - len(plain) - 3
	eb := make([]byte, k)
	eb[0] = 0x00
	eb[1] = 0x02
	ps := eb[2 : 2+psLen]
	if _, err := rand.Read(ps); err != nil {
		return "", err
	}
	for i := range ps {
		for ps[i] == 0 {
			if _, err := rand.Read(ps[i : i+1]); err != nil {
				return "", err
			}
		}
	}
	eb[k-len(plain)-1] = 0x00
	copy(eb[k-len(plain):], []byte(plain))
	m := new(big.Int).SetBytes(eb)
	e := big.NewInt(int64(pub.E))
	c := new(big.Int).Exp(m, e, pub.N)
	enc := c.Bytes()
	if len(enc) < k {
		padded := make([]byte, k)
		copy(padded[k-len(enc):], enc)
		enc = padded
	}

	return base64.StdEncoding.EncodeToString(enc), nil
}

func pkcs7Pad(src []byte, blockSize int) []byte {
	padding := blockSize - len(src)%blockSize
	return append(src, bytes.Repeat([]byte{byte(padding)}, padding)...)
}
