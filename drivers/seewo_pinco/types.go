package seewo_pinco

import (
	"time"

	"github.com/OpenListTeam/OpenList/v4/internal/model"
)

type BaseResp struct {
	StatusCode int    `json:"statusCode"`
	Message    string `json:"message"`
}

type GetV1DriveMaterialsResp struct {
	BaseResp
	Data struct {
		Content          []Content `json:"content"`
		TotalPages       int       `json:"totalPages"`
		TotalElements    int       `json:"totalElements"`
		Last             bool      `json:"last"`
		NumberOfElements int       `json:"numberOfElements"`
		First            bool      `json:"first"`
		Size             int       `json:"size"`
		Number           int       `json:"number"`
	} `json:"data"`
}

type Content struct {
	Name           string `json:"name"`
	Type           int    `json:"type"`
	Size           int64  `json:"size"`
	ID             string `json:"id"`
	CreateTime     int64  `json:"createTime"`
	UpdateTime     int64  `json:"updateTime"`
	ThumbnailURL   string `json:"thumbnailUrl"`
	DownloadURL    string `json:"downloadUrl"`
	Creator        string `json:"creator"`
	CreatorName    string `json:"creatorName"`
	ParentID       string `json:"parentId"`
	ParentIDPath   string `json:"parentIdPath"`
	ParentNamePath string `json:"parentNamePath"`
	StoreID        string `json:"storeId"`
	PreviewURL     string `json:"previewUrl"`
}

func contentToObj(c Content) *model.ObjThumbURL {
	return &model.ObjThumbURL{
		Object: model.Object{
			ID:       c.ID,
			Name:     c.Name,
			Size:     c.Size,
			Ctime:    time.UnixMilli(c.CreateTime),
			Modified: time.UnixMilli(c.UpdateTime),
			IsFolder: c.Type == 9,
		},
		Thumbnail: model.Thumbnail{Thumbnail: c.ThumbnailURL},
		Url: model.Url{
			Url: c.DownloadURL,
		},
	}
}

type GetV1DriveMaterialsCapacityResp struct {
	BaseResp
	Data struct {
		Capacity uint64 `json:"capacity"`
		Used     uint64 `json:"used"`
		// UsedDetail []struct {
		// 	AppCode         string `json:"appCode"`
		// 	AppName         string `json:"appName"`
		// 	TotalUsed       uint64 `json:"totalUsed"`
		// 	CatalogUsedList []struct {
		// 		ID   int    `json:"id"`
		// 		Name string `json:"name"`
		// 		Used uint64 `json:"used"`
		// 	} `json:"catalogUsedList"`
		// } `json:"usedDetail"`
	} `json:"data"`
}
