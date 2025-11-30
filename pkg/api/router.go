package api

import (
	"net/http"

	"github.com/q1317540161/free5gc-MCP/pkg/auth"
	"github.com/q1317540161/free5gc-MCP/pkg/control"
	"github.com/q1317540161/free5gc-MCP/pkg/mcp"
	"github.com/gin-gonic/gin"
)

var client *control.Free5GCClient

func SetupRouter(c *control.Free5GCClient, authCfg *auth.AuthConfig) *gin.Engine {
	client = c

	r := gin.Default()
	mcpServer := mcp.NewServer(c)
	// Protect MCP root with auth if configured
	if authCfg != nil && authCfg.IsEnabled() {
		r.POST("/", authCfg.Middleware(), mcpServer.HandleJSONRPC)
		r.GET("/", authCfg.Middleware(), mcpServer.HandleSSE)
	} else {
		r.POST("/", mcpServer.HandleJSONRPC)
		r.GET("/", mcpServer.HandleSSE)
	}

	// health
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
		tools.POST("/convert-time", convertTime)

		subs := tools.Group("/subscribers")
		{
			subs.GET("", listSubscribers)
			subs.POST("", createSubscriber)
				subs.GET("/:id", getSubscriber)
				subs.PUT("/:id", updateSubscriber)
				subs.DELETE("/:id", deleteSubscriber)
		}

		core := tools.Group("/core")
		{
			core.POST("/start", startCore)
			core.POST("/stop", stopCore)
			core.GET("/status", coreStatus)
		}
	}

	return r
}
