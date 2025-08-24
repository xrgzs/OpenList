package dsm_share

type BaseResp struct {
	Data    any  `json:"data"`
	Success bool `json:"success"`
}

//	type Owner struct {
//		Gid   int    `json:"gid"`
//		Group string `json:"group"`
//		UID   int    `json:"uid"`
//		User  string `json:"user"`
//	}
//
//	type ACL struct {
//		Append bool `json:"append"`
//		Del    bool `json:"del"`
//		Exec   bool `json:"exec"`
//		Read   bool `json:"read"`
//		Write  bool `json:"write"`
//	}
//
//	type Perm struct {
//		ACL       ACL  `json:"acl"`
//		IsACLMode bool `json:"is_acl_mode"`
//		Posix     int  `json:"posix"`
//	}
type Time struct {
	Atime  int64 `json:"atime"`
	Crtime int64 `json:"crtime"`
	Ctime  int64 `json:"ctime"`
	Mtime  int64 `json:"mtime"`
}
type Additional struct {
	MountPointType string `json:"mount_point_type"`
	Size           int64  `json:"size"`
	Time           Time   `json:"time"`
	Type           string `json:"type"`
	// Owner          Owner  `json:"owner"`
	// Perm           Perm   `json:"perm"`
}
type File struct {
	Additional Additional `json:"additional"`
	Isdir      bool       `json:"isdir"`
	Name       string     `json:"name"`
	Path       string     `json:"path"`
}

type ListResp struct {
	BaseResp
	Data struct {
		Files  []File `json:"files"`
		Offset int    `json:"offset"`
		Total  int    `json:"total"`
	} `json:"data"`
}

type LoginResp struct {
	BaseResp
	Data struct {
		SharingSid string `json:"sharing_sid"`
	} `json:"data"`
}

type Private struct {
	Filename string `json:"filename"`
}
type InitDataResp struct {
	BaseResp
	Data struct {
		Private Private `json:"Private"`
	} `json:"data"`
}
