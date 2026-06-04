package zbrowser

import (
	"time"

	"github.com/OpenListTeam/OpenList/v4/internal/model"
	"github.com/OpenListTeam/OpenList/v4/pkg/utils"
)

type baseResp struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Flag int    `json:"flag"`
}

type Obj struct {
	model.Object
	ParentID string
}

type DirItem struct {
	ID        string `json:"id"`
	Pid       string `json:"pid"`
	Name      string `json:"name"`
	UpdatedAt int64  `json:"updatedAt"`
	Ver       int    `json:"ver"`
	HasSub    bool   `json:"hasSub"`
	Type      int    `json:"type"`
	LocName   string `json:"locName"`
}

type dirListRespV1 struct {
	baseResp
	Data struct {
		ID        string    `json:"id"`
		Pid       string    `json:"pid"`
		Name      string    `json:"name"`
		UpdatedAt int64     `json:"updatedAt"`
		Ver       int       `json:"ver"`
		Sub       []DirItem `json:"sub"`
		Type      int       `json:"type"`
	} `json:"data"`
}

func (d dirListRespV1) toObj() ([]model.Obj, error) {
	return utils.SliceConvert(d.Data.Sub, func(item DirItem) (model.Obj, error) {
		return &Obj{
			Object: model.Object{
				ID:       item.ID,
				Name:     item.Name,
				Modified: time.Unix(item.UpdatedAt, 0),
				IsFolder: true,
			},
			ParentID: d.Data.ID,
		}, nil
	})
}

type ListItem struct {
	ID        string `json:"id"`
	FileName  string `json:"fileName"`
	FileSize  int64  `json:"fileSize"`
	Hash      string `json:"hash"`
	CreatedAt int64  `json:"createdAt"`
	UpdatedAt int64  `json:"updatedAt"`
	SensCode  int    `json:"sensCode"`
	Sens      string `json:"sens"`
	CanPlay   int    `json:"canPlay"`
	Flag      int    `json:"flag"`
	Kind      int    `json:"kind"`
	Type      int    `json:"type"`
	Img       string `json:"img"`
	Thum      string `json:"thum"`
	CanAppeal int    `json:"canAppeal"`
	LocName   string `json:"locName"`
}

type fileListRespV1 struct {
	baseResp
	Data struct {
		Pid     string     `json:"pid"`
		Ver     int        `json:"ver"`
		Next    string     `json:"next"`
		HasMore int        `json:"hasMore"`
		Cnt     int        `json:"cnt"`
		Type    int        `json:"type"`
		List    []ListItem `json:"list"`
	} `json:"data"`
}

func (f fileListRespV1) toObj() ([]model.Obj, error) {
	return utils.SliceConvert(f.Data.List, func(item ListItem) (model.Obj, error) {
		return &Obj{
			Object: model.Object{
				ID:       item.ID,
				Name:     item.FileName,
				Size:     item.FileSize,
				Modified: time.Unix(item.UpdatedAt, 0),
				Ctime:    time.Unix(item.CreatedAt, 0),
				IsFolder: item.Type == 1, // type=1 is folder, type=2 is file
				HashInfo: utils.NewHashInfo(utils.SHA1, item.Hash),
			},
			ParentID: f.Data.Pid,
		}, nil
	})
}

type fileDlRespV3 struct {
	baseResp
	Data struct {
		List struct {
			ID    string        `json:"id"`
			Name  string        `json:"name"`
			Sub   []interface{} `json:"sub"`
			Files []struct {
				ID       string `json:"id"`
				Name     string `json:"name"`
				Ext      string `json:"ext"`
				FileSize int64  `json:"fileSize"`
				URL      string `json:"url"`
				SensCode int    `json:"sensCode"`
				Sens     string `json:"sens"`
			} `json:"files"`
		} `json:"list"`
		Error struct {
			SensCode int    `json:"sensCode"`
			Sens     string `json:"sens"`
		} `json:"error"`
	} `json:"data"`
}

type dirNewRespV3 struct {
	baseResp
	Data struct {
		Model struct {
			ID           string `json:"id"`
			Qid          int64  `json:"qid"`
			DirName      string `json:"dirName"`
			Pid          string `json:"pid"`
			Status       int    `json:"status"`
			CollectAt    int64  `json:"collectAt"`
			DelAt        int64  `json:"delAt"`
			CollectDirID int    `json:"collectDirId"`
			Ver          int    `json:"ver"`
			CreatedAt    int64  `json:"createdAt"`
			UpdatedAt    int64  `json:"updatedAt"`
		} `json:"model"`
	} `json:"data"`
}

type userSpaceResp struct {
	baseResp
	Data struct {
		Left  int64 `json:"left"`
		Total int64 `json:"total"`
		Used  int64 `json:"used"`
	} `json:"data"`
}
