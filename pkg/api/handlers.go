package api

import (
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
)

// These handlers forward subscriber operations to the configured free5GC WebUI backend

func listSubscribers(c *gin.Context) {
	resp, err := client.ListSubscribers()
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), b)
}

func createSubscriber(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	resp, err := client.CreateSubscriber(body)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), b)
}

func getSubscriber(c *gin.Context) {
	id := c.Param("id")
	resp, err := client.GetSubscriber(id)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), b)
}

func updateSubscriber(c *gin.Context) {
	id := c.Param("id")
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	resp, err := client.UpdateSubscriber(id, body)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), b)
}

func deleteSubscriber(c *gin.Context) {
	id := c.Param("id")
	resp, err := client.DeleteSubscriber(id)
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
