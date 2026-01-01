package gslb

// GslbStorage 表示单个后端存储的信息
type GslbStorage struct {
	// 后端的存储路径
	Path string `yaml:"path"`
	// 优先匹配该后端存储的自治系统名字
	Aso string `yaml:"aso"`
	// 优先匹配该后端存储的自治系统号码
	Asn []uint `yaml:"asn"`
}
