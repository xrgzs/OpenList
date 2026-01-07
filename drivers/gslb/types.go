package gslb

import rcfs "github.com/rclone/rclone/fs"

// GslbStorage 表示单个后端存储的信息
type GslbStorage struct {
	// 后端的存储路径
	Path string `yaml:"path"`
	// 优先匹配包含该后端存储的自治系统名字
	Aso []string `yaml:"aso"`
	// 优先匹配该后端存储的自治系统号码
	Asn []uint `yaml:"asn"`
	// 优先匹配该后端存储的ISP
	Isp []string `yaml:"isp"`
	// 优先匹配该后端存储的国家/地区代码
	CountryCode []string `yaml:"country_code"`
	// 作为List参考，避免请求过多后端存储
	Ref bool `yaml:"ref"`
	// 禁止下载时使用该 Ref 存储
	NoDown bool `yaml:"no_down"`
	// 过滤文件大小-小于该值的文件
	MinSize rcfs.SizeSuffix `yaml:"min_size"`
	// 过滤文件大小-大于该值的文件
	MaxSize rcfs.SizeSuffix `yaml:"max_size"`
}
