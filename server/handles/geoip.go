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
	common.SuccessResp(c, op.GetIPInfo(ipStr))
}
