package vk

import "github.com/OpenListTeam/OpenList/v4/internal/model"

type DocGetResp struct {
	Response struct {
		Count      int     `json:"count"`
		TotalCount int     `json:"total_count"`
		Items      []Items `json:"items"`
	} `json:"response"`
}

type Items struct {
	ID         int    `json:"id"`
	OwnerID    int    `json:"owner_id"`
	Title      string `json:"title"`
	Size       int64  `json:"size"`
	Ext        string `json:"ext"`
	Date       int    `json:"date"`
	Type       int    `json:"type"`
	URL        string `json:"url"`
	IsLicensed int    `json:"is_licensed"`
	IsUnsafe   int    `json:"is_unsafe"`
	CanManage  bool   `json:"can_manage"`
	FolderID   int    `json:"folder_id"`
	PrivateURL string `json:"private_url"`
}

type Object struct {
	model.Object
	URL string
}
