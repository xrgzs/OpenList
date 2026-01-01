package handles

import (
	"github.com/OpenListTeam/OpenList/v4/internal/op"
	"github.com/OpenListTeam/OpenList/v4/server/common"
	"github.com/gin-gonic/gin"
)

// return geoip2 ASN
func GeoIP2ASN(c *gin.Context) {
	ipStr := c.Query("ip")
	if ipStr == "" {
		ipStr = c.ClientIP()
	}
	ipinfo := op.GetIPInfo(ipStr)
	if !ipinfo.HasData() {
		common.ErrorStrResp(c, "invalid ip", 400)
		return
	}
	common.SuccessResp(c, ipinfo)
}
