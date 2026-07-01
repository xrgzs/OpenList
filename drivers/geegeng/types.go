package geegeng

import (
	"time"

	"github.com/OpenListTeam/OpenList/v4/internal/model"
)

type BaseResp struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

type UserInfo struct {
	ID        string `json:"ID"`
	CreatedAt string `json:"CreatedAt"`
	//Phone       string `json:"phone"`
	// Nickname    string `json:"nickname"`
	// Email       string `json:"email"`
	// Avatar      string `json:"avatar"`
	// CoverType   int    `json:"coverType"`
	Store int64 `json:"store"`
	// UsedStore   int64  `json:"usedStore"`
	VusedStore int64 `json:"vusedStore"`
	// SusedStore  int64  `json:"susedStore"`
	// Status      int    `json:"status"`
	// AccessSign  string `json:"accessSign"`
	// AccessSign2 string `json:"accessSign2"`
	// AccessToken string `json:"accessToken"`
}

type GetUserInfoResp struct {
	BaseResp
	Data struct {
		UserInfo UserInfo `json:"userInfo"`
	} `json:"data"`
}

type List struct {
	Id        string `json:"ID"`
	CreatedAt string `json:"CreatedAt"` //2026-05-03T13:46:51+08:00
	UpdatedAt string `json:"UpdatedAt"`
	Name      string `json:"name"`
	Type      int    `json:"type"`
	Size      int64  `json:"size"`
	Category  int    `json:"category"`
}

func (l List) toObj(parentID string) model.Obj {
	modified, _ := time.Parse(time.RFC3339, l.UpdatedAt)
	ctime, _ := time.Parse(time.RFC3339, l.CreatedAt)
	return &model.Object{
		ID:       l.Id,
		Name:     l.Name,
		Size:     l.Size,
		Modified: modified,
		Ctime:    ctime,
		IsFolder: l.Type == 2,
		Path:     parentID,
	}
}

type ListResp struct {
	BaseResp
	Data struct {
		List     []List `json:"list"`
		Page     int    `json:"page"`
		PageSize int    `json:"pageSize"`
		Total    int    `json:"total"`
	} `json:"data"`
}

type LinkResp struct {
	BaseResp
	Data struct {
		Url string `json:"url"`
	} `json:"data"`
}

type FindFile struct {
	Fid          string `json:"fid"`
	Sign         string `json:"sign"`
	UploadStatus bool   `json:"uploadStatus"`
	Url          string `json:"url"`
}

type FindFileResp struct {
	BaseResp
	Data struct {
		File FindFile `json:"file"`
	} `json:"data"`
}

type InitMultiUploadResp struct {
	BaseResp
	Data struct {
		FileName string `json:"fileName"`
		UploadId string `json:"uploadId"`
	} `json:"data"`
}

type GetUploadedPartsInfoResp struct {
	BaseResp
	Data struct {
		UploadedParts string `json:"uploadedParts"` // comma-separated part numbers, e.g. "1,2,3"
	} `json:"data"`
}

type UploadUrlItem struct {
	RequestUrl    string `json:"requestUrl"`
	RequestHeader string `json:"requestHeader"`
}

type GetMultiUploadUrlsResp struct {
	BaseResp
	Data struct {
		UploadUrls map[string]UploadUrlItem `json:"uploadUrls"` // key: "partNumber_1", "partNumber_2", ...
	} `json:"data"`
}
