package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/q1317540161/free5gc-MCP/pkg/auth"
	"github.com/q1317540161/free5gc-MCP/pkg/control"
	"github.com/q1317540161/free5gc-MCP/pkg/mcp"
)

var client *control.Free5GCClient

func SetupRouter(c *control.Free5GCClient, authCfg *auth.AuthConfig) *gin.Engine {
	client = c

	r := gin.Default()
	mcpServer := mcp.NewServer(c)

	// MCP JSON-RPC endpoint at root
	if authCfg != nil && authCfg.IsEnabled() {
		r.POST("/", authCfg.Middleware(), mcpServer.HandleJSONRPC)
		r.GET("/", authCfg.Middleware(), mcpServer.HandleSSE)
	} else {
		r.POST("/", mcpServer.HandleJSONRPC)
		r.GET("/", mcpServer.HandleSSE)
	}

	// health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	tools := r.Group("/tools")
	tools.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "pong"})
	})

	// apply auth middleware if configured
	if authCfg != nil && authCfg.IsEnabled() {
		tools.Use(authCfg.Middleware())
	}
	{
		// Tenant users endpoint - GET /tools/tenant/:tenantId/user
		tools.GET("/tenant/:tenantId/user", getTenantUsers)

		core := tools.Group("/core")
		{
			core.POST("/start", startCore)
			core.POST("/stop", stopCore)
			core.GET("/status", coreStatus)
		}
	}

	return r
}
