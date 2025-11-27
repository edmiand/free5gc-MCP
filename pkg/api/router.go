package api

import (
	"net/http"

	"github.com/Gthulhu/free5gc-MCP/pkg/api/auth"
	"github.com/Gthulhu/free5gc-MCP/pkg/control"
	"github.com/gin-gonic/gin"
)

var client *control.Free5GCClient

func SetupRouter(c *control.Free5GCClient, authCfg *auth.AuthConfig) *gin.Engine {
	client = c

	r := gin.Default()

	// health
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	tools := r.Group("/tools")
	// apply auth middleware if configured
	if authCfg != nil && authCfg.IsEnabled() {
		tools.Use(authCfg.Middleware())
	}
	{
		subs := tools.Group("/subscribers")
		{
			subs.GET("", listSubscribers)
			subs.POST("", createSubscriber)
			subs.GET(":id", getSubscriber)
			subs.PUT(":id", updateSubscriber)
			subs.DELETE(":id", deleteSubscriber)
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
