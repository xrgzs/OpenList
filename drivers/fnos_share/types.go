package fnos_share

type BaseResp struct {
	Msg  string `json:"msg"`
	Code int    `json:"code"`
	Data any    `json:"data"`
}

type ShareDataResp struct {
	BaseResp
	Data *struct {
		Token string `json:"token"`
		Name  string `json:"name"`
	} `json:"data"`
}

type PwdReq struct {
	ShareID string `json:"shareId"`
	Passwd  string `json:"passwd"`
}

type FileInfo struct {
	FileID  int    `json:"fileId"`
	Path    string `json:"path"`
	IsDir   int    `json:"isDir"`
	File    string `json:"file"`
	Size    int64  `json:"size"`
	ModTime int64  `json:"modTime"`
}

type ListReq struct {
	ShareID string `json:"shareId"`
	Path    string `json:"path"`
	FileID  *int   `json:"fileId"`
}

type ListResp struct {
	BaseResp
	Data struct {
		Files []FileInfo `json:"files"`
	} `json:"data"`
}

type DownloadFile struct {
	Path   string `json:"path"`
	FileID int    `json:"fileId"`
}

type DownloadReq struct {
	Files            []DownloadFile `json:"files"`
	ShareID          string         `json:"shareId"`
	DownloadFilename string         `json:"downloadFilename"`
}

type DownloadResp struct {
	BaseResp
	Data struct {
		Path string `json:"path"`
	} `json:"data"`
}
