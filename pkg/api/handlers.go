package api

import (
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
)

// getTenantUsers handles GET /tools/tenant/:tenantId/user
// Calls the webconsole backend to get all users for a tenant
func getTenantUsers(c *gin.Context) {
	tenantId := c.Param("tenantId")
	resp, err := client.GetTenantUsers(tenantId)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), b)
}

// Core control handlers (stubs) — still call control client in future
func startCore(c *gin.Context) {
	c.JSON(http.StatusAccepted, gin.H{"result": "start requested"})
}

func stopCore(c *gin.Context) {
	c.JSON(http.StatusAccepted, gin.H{"result": "stop requested"})
}

func coreStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "unknown", "detail": "not implemented"})
}
