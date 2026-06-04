package gslb

import (
	"fmt"

	rclonefs "github.com/rclone/rclone/fs"
	"gopkg.in/yaml.v3"
)

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
	// 排除特定国家/地区的用户（与 country_code 互斥）
	CountryCodeNot []string `yaml:"country_code_not"`
	// 作为List参考，避免请求过多后端存储
	Ref bool `yaml:"ref"`
	// 禁止下载时使用该 Ref 存储
	NoDown bool `yaml:"no_down"`
	// 过滤文件大小-小于该值的文件
	MinSize SizeSuffix `yaml:"min_size"`
	// 过滤文件大小-大于该值的文件
	MaxSize SizeSuffix `yaml:"max_size"`
	// 链接替换
	Replace map[string]string `yaml:"replace"`
	// 标记该节点参与同分负载均衡
	Balance bool `yaml:"balance"`
	// 标记该节点为万能节点，条件性提升优先级并参与负载均衡（隐含 balance: true）
	BalanceUniversal bool `yaml:"balance_universal"`
}

// Validate 校验配置合法性
func (s GslbStorage) Validate() error {
	if len(s.CountryCodeNot) > 0 && len(s.CountryCode) > 0 {
		return fmt.Errorf("gslb storage %s: country_code_not and country_code are mutually exclusive", s.Path)
	}
	return nil
}

// SizeSuffix 包装 rclone fs.SizeSuffix 以支持 YAML 序列化
type SizeSuffix struct {
	rclonefs.SizeSuffix
}

func (s *SizeSuffix) UnmarshalYAML(node *yaml.Node) error {
	var str string
	if err := node.Decode(&str); err != nil {
		return err
	}
	return s.SizeSuffix.Set(str)
}

func (s SizeSuffix) MarshalYAML() (interface{}, error) {
	return s.SizeSuffix.String(), nil
}

// Int64 返回字节数
func (s SizeSuffix) Int64() int64 {
	return int64(s.SizeSuffix)
}
