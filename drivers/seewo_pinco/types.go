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
		Capacity int64 `json:"capacity"`
		Used     int64 `json:"used"`
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

type PostV1DriveMaterialsMatchResp struct {
	BaseResp
	Data struct {
		Valid          bool                                         `json:"valid"`
		NeedToUpload   bool                                         `json:"needToUpload"`
		FormUploadMeta PostV1DriveMaterialsMatchResp_FormUploadMeta `json:"formUploadMeta"`
	} `json:"data"`
}

type PostV1DriveMaterialsMatchResp_FormUploadMeta struct {
	URL           string            `json:"url"`
	Headers       map[string]string `json:"headers"`
	Fields        map[string]string `json:"fields"`
	FileFieldName string            `json:"fileFieldName"`
}

type CStoreUploadResp struct {
	BaseResp
	Data CStoreUploadResp_Data `json:"data"`
}

type CStoreUploadResp_Data struct {
	FileSize    int    `json:"fileSize"`
	DownloadURL string `json:"downloadUrl"`
	FileKey     string `json:"fileKey"`
}

type PostV1DriveMaterialsCstoreWayResp struct {
	BaseResp
	Data struct {
		ID           string `json:"id"`
		Name         string `json:"name"`
		MimeType     string `json:"mimeType"`
		Size         int    `json:"size"`
		SpaceID      string `json:"spaceId"`
		CatalogID    int    `json:"catalogId"`
		CatalogName  string `json:"catalogName"`
		AppCode      string `json:"appCode"`
		Creator      string `json:"creator"`
		CreatorName  string `json:"creatorName"`
		CreateTime   int64  `json:"createTime"`
		ParentID     string `json:"parentId"`
		ThumbnailURL string `json:"thumbnailUrl"`
		UpdateTime   int64  `json:"updateTime"`
		TypeTag      int    `json:"typeTag"`
		File         bool   `json:"file"`
	} `json:"data"`
}

type PostV3CstoreUploadPolicyResp struct {
	BaseResp
	Data struct {
		AppID         string `json:"appId"`
		KeyPrefix     string `json:"keyPrefix"`
		ExpireSeconds int    `json:"expireSeconds"`
		PolicyList    []struct {
			Priority     int    `json:"priority"`
			Type         string `json:"type"`
			UploadURL    string `json:"uploadUrl"`
			HeaderFields []struct {
				Key   string `json:"key"`
				Value string `json:"value"`
			} `json:"headerFields"`
			FormFields []struct {
				Key   string `json:"key"`
				Value string `json:"value"`
			} `json:"formFields"`
			FileKey string `json:"fileKey"`
		} `json:"policyList"`
	} `json:"data"`
}

type PostV1CstoreMultipartUploadPolicyResp struct {
	BaseResp
	Data struct {
		AppID         int `json:"appId"`
		ExpireSeconds int `json:"expireSeconds"`
		PolicyList    []struct {
			FileKey             string `json:"fileKey"`
			PartSize            int64  `json:"partSize"`
			Priority            int    `json:"priority"`
			ServiceProviderName string `json:"serviceProviderName"`
			UploadID            string `json:"uploadId"`
			UploadMethod        string `json:"uploadMethod"`
			UploadMsgField      string `json:"uploadMsgField"`
			UploadMsgType       string `json:"uploadMsgType"`
		} `json:"policyList"`
	} `json:"data"`
}

type PostV1CstoreMultipartUploadRuleResp struct {
	BaseResp
	Data struct {
		HeaderFields []struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		} `json:"headerFields"`
		UploadURL string `json:"uploadUrl"`
	} `json:"data"`
}

type PostV1CstoreMultipartCompleteResp struct {
	BaseResp
	Data struct {
		FileKey       string `json:"fileKey"`
		DownloadURL   string `json:"downloadUrl"`
		MimeType      string `json:"mimeType"`
		FileSize      int    `json:"fileSize"`
		CustomeFields []any  `json:"customeFields"`
	} `json:"data"`
}

type MissionTodayRecordResp struct {
	Data struct {
		SignRecord struct {
			BeenSigned bool `json:"beenSigned"`
			CurrentDay int  `json:"currentDay"`
		} `json:"signRecord"`
	} `json:"data"`
	ErrorCode  int    `json:"error_code"`
	Message    string `json:"message"`
	ServerTime int64  `json:"serverTime"`
}

type MissionSignResp struct {
	Data struct {
		LotteryRecord struct {
			AwardName        string `json:"awardName"`
			LotteryType      string `json:"lotteryType"`
			PrizeDescription string `json:"prizeDescription"`
			PrizeName        string `json:"prizeName"`
			PrizeParam       string `json:"prizeParam"`
			PrizePictureURL  string `json:"prizePictureUrl"`
			PrizeType        string `json:"prizeType"`
			State            string `json:"state"`
		} `json:"lotteryRecord"`
		SignRecord struct {
			BeenSigned       bool   `json:"beenSigned"`
			CurrentDay       int    `json:"currentDay"`
			PrizeDescription string `json:"prizeDescription"`
			PrizeName        string `json:"prizeName"`
			PrizeParam       string `json:"prizeParam"`
			PrizePictureURL  string `json:"prizePictureUrl"`
			PrizeType        string `json:"prizeType"`
		} `json:"signRecord"`
	} `json:"data"`
	ErrorCode  int    `json:"error_code"`
	Message    string `json:"message"`
	ServerTime int64  `json:"serverTime"`
}
