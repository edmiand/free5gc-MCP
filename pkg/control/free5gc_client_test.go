package control

import (
	"fmt"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestLogin_Success(t *testing.T) {
	fmt.Printf("\tTestLogin_Success: Expected successful login with credentials admin/free5gc, expected token='fake-token'\n")

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/login", func(c *gin.Context) {
		var req loginRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Bad Request"})
			return
		}

		if req.Username != "admin" || req.Password != "free5gc" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}

		resp := loginResponse{AccessToken: "fake-token"}
		c.JSON(http.StatusOK, resp)
	})

	server := httptest.NewServer(r)
	defer server.Close()

	client := NewFree5GCClient(server.URL, "admin", "free5gc", "")
	err := client.Login()
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	if client.getToken() != "fake-token" {
		t.Errorf("Expected token 'fake-token', got '%s'", client.getToken())
	}
}

func TestLogin_Failure(t *testing.T) {
	fmt.Printf("\tTestLogin_Failure: Expected login to fail with 401 status and error message containing 'login failed with status 401'\n")

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/login", func(c *gin.Context) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
	})

	server := httptest.NewServer(r)
	defer server.Close()

	client := NewFree5GCClient(server.URL, "admin", "wrong", "")
	err := client.Login()
	if err == nil {
		t.Error("Expected login to fail")
	}
	if !strings.Contains(err.Error(), "login failed with status 401") {
		t.Errorf("Expected error message to contain 'login failed with status 401', got: %v", err)
	}
}

func TestGetSubscribers(t *testing.T) {
	fmt.Printf("\tTestGetSubscribers: Expected GET /api/subscriber to return status 200 and response containing subscriber data\n")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/login" && r.Method == http.MethodPost {
			resp := loginResponse{AccessToken: "fake-token"}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
			return
		}

		if r.URL.Path == "/api/subscriber" && r.Method == http.MethodGet {
			if r.Header.Get("Token") != "fake-token" {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`[{"ueId": "imsi-208930000000001", "servingPlmnId": "20893"}]}`))
			return
		}

		http.NotFound(w, r)
	}))
	defer server.Close()

	client := NewFree5GCClient(server.URL, "admin", "free5gc", "")
	resp, err := client.GetSubscribers()
	if err != nil {
		t.Fatalf("GetSubscribers failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	if !strings.Contains(string(body), "imsi-208930000000001") {
		t.Errorf("Expected response to contain subscriber data, got: %s", string(body))
	}
}

func TestGetSubscriberByID(t *testing.T) {
	fmt.Printf("\tTestGetSubscriberByID: Expected GET /api/subscriber/{ueId}/{plmnId} to return status 200\n")

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/login", func(c *gin.Context) {
		resp := loginResponse{AccessToken: "fake-token"}
		c.JSON(http.StatusOK, resp)
	})
	r.GET("/api/subscriber/:ueId/:plmnId", func(c *gin.Context) {
		token := c.GetHeader("Token")
		if token != "fake-token" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}
		subscriber := gin.H{
			"ueId":           "imsi-208930000000001",
			"servingPlmnId": "20893",
		}
		c.JSON(http.StatusOK, subscriber)
	})

	server := httptest.NewServer(r)
	defer server.Close()

	client := NewFree5GCClient(server.URL, "admin", "free5gc", "")
	resp, err := client.GetSubscriberByID("imsi-208930000000001", "20893")
	if err != nil {
		t.Fatalf("GetSubscriberByID failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestCreateSubscriber(t *testing.T) {
	fmt.Printf("\tTestCreateSubscriber: Expected POST /api/subscriber/{ueId}/{plmnId} to return status 201\n")

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/login", func(c *gin.Context) {
		resp := loginResponse{AccessToken: "fake-token"}
		c.JSON(http.StatusOK, resp)
	})
	r.POST("/api/subscriber/:ueId/:plmnId", func(c *gin.Context) {
		token := c.GetHeader("Token")
		if token != "fake-token" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}
		contentType := c.GetHeader("Content-Type")
		if contentType != "application/json" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Bad Request"})
			return
		}
		c.Status(http.StatusCreated)
	})

	server := httptest.NewServer(r)
	defer server.Close()

	client := NewFree5GCClient(server.URL, "admin", "free5gc", "")
	subscriberData := `{"ueId": "imsi-208930000000001", "servingPlmnId": "20893"}`
	resp, err := client.CreateSubscriber("imsi-208930000000001", "20893", []byte(subscriberData))
	if err != nil {
		t.Fatalf("CreateSubscriber failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", resp.StatusCode)
	}
}

func TestUpdateSubscriber(t *testing.T) {
	fmt.Printf("\tTestUpdateSubscriber: Expected PUT /api/subscriber/{ueId}/{plmnId} to return status 200\n")

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/login", func(c *gin.Context) {
		resp := loginResponse{AccessToken: "fake-token"}
		c.JSON(http.StatusOK, resp)
	})
	r.PUT("/api/subscriber/:ueId/:plmnId", func(c *gin.Context) {
		token := c.GetHeader("Token")
		if token != "fake-token" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}
		c.Status(http.StatusOK)
	})

	server := httptest.NewServer(r)
	defer server.Close()

	client := NewFree5GCClient(server.URL, "admin", "free5gc", "")
	updateData := `{"ueId": "imsi-208930000000001", "servingPlmnId": "20893"}`
	resp, err := client.UpdateSubscriber("imsi-208930000000001", "20893", []byte(updateData))
	if err != nil {
		t.Fatalf("UpdateSubscriber failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestDeleteSubscriber(t *testing.T) {
	fmt.Printf("\tTestDeleteSubscriber: Expected DELETE /api/subscriber/{ueId}/{plmnId} to return status 204\n")

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/login", func(c *gin.Context) {
		resp := loginResponse{AccessToken: "fake-token"}
		c.JSON(http.StatusOK, resp)
	})
	r.DELETE("/api/subscriber/:ueId/:plmnId", func(c *gin.Context) {
		token := c.GetHeader("Token")
		if token != "fake-token" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}
		c.Status(http.StatusNoContent)
	})

	server := httptest.NewServer(r)
	defer server.Close()

	client := NewFree5GCClient(server.URL, "admin", "free5gc", "")
	resp, err := client.DeleteSubscriber("imsi-208930000000001", "20893")
	if err != nil {
		t.Fatalf("DeleteSubscriber failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("Expected status 204, got %d", resp.StatusCode)
	}
}

func TestPatchSubscriber(t *testing.T) {
	fmt.Printf("\tTestPatchSubscriber: Expected PATCH /api/subscriber/{ueId}/{plmnId} to return status 200\n")

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/login", func(c *gin.Context) {
		resp := loginResponse{AccessToken: "fake-token"}
		c.JSON(http.StatusOK, resp)
	})
	r.PATCH("/api/subscriber/:ueId/:plmnId", func(c *gin.Context) {
		token := c.GetHeader("Token")
		if token != "fake-token" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}
		c.Status(http.StatusOK)
	})

	server := httptest.NewServer(r)
	defer server.Close()

	client := NewFree5GCClient(server.URL, "admin", "free5gc", "")
	patchData := `{"servingPlmnId": "20893"}`
	resp, err := client.PatchSubscriber("imsi-208930000000001", "20893", []byte(patchData))
	if err != nil {
		t.Fatalf("PatchSubscriber failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestDeleteMultipleSubscribers(t *testing.T) {
	fmt.Printf("\tTestDeleteMultipleSubscribers: Expected DELETE /api/subscriber to return status 204\n")

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/login", func(c *gin.Context) {
		resp := loginResponse{AccessToken: "fake-token"}
		c.JSON(http.StatusOK, resp)
	})
	r.DELETE("/api/subscriber", func(c *gin.Context) {
		token := c.GetHeader("Token")
		if token != "fake-token" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}
		c.Status(http.StatusNoContent)
	})

	server := httptest.NewServer(r)
	defer server.Close()

	client := NewFree5GCClient(server.URL, "admin", "free5gc", "")
	deleteData := `[{"ueId": "imsi-208930000000001", "servingPlmnId": "20893"}]`
	resp, err := client.DeleteMultipleSubscribers([]byte(deleteData))
	if err != nil {
		t.Fatalf("DeleteMultipleSubscribers failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("Expected status 204, got %d", resp.StatusCode)
	}
}

func TestCreateMultipleSubscribers(t *testing.T) {
	fmt.Printf("\tTestCreateMultipleSubscribers: Expected POST /api/subscriber/{ueId}/{plmnId}/{userNumber} to return status 201\n")

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/login", func(c *gin.Context) {
		resp := loginResponse{AccessToken: "fake-token"}
		c.JSON(http.StatusOK, resp)
	})
	r.POST("/api/subscriber/:ueId/:plmnId/:userNumber", func(c *gin.Context) {
		token := c.GetHeader("Token")
		if token != "fake-token" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}
		c.Status(http.StatusCreated)
	})

	server := httptest.NewServer(r)
	defer server.Close()

	client := NewFree5GCClient(server.URL, "admin", "free5gc", "")
	subscriberData := `{"ueId": "imsi-208930000000001", "servingPlmnId": "20893"}`
	resp, err := client.CreateMultipleSubscribers("imsi-208930000000001", "20893", 5, []byte(subscriberData))
	if err != nil {
		t.Fatalf("CreateMultipleSubscribers failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", resp.StatusCode)
	}
}

func TestGetTenantUsers(t *testing.T) {
	fmt.Printf("\tTestGetTenantUsers: Expected GET /api/tenant/{tenantId}/user to return status 200\n")

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/login", func(c *gin.Context) {
		resp := loginResponse{AccessToken: "fake-token"}
		c.JSON(http.StatusOK, resp)
	})
	r.GET("/api/tenant/:tenantId/user", func(c *gin.Context) {
		token := c.GetHeader("Token")
		if token != "fake-token" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}
		users := []gin.H{
			{"username": "user1", "role": "admin"},
		}
		c.JSON(http.StatusOK, users)
	})

	server := httptest.NewServer(r)
	defer server.Close()

	client := NewFree5GCClient(server.URL, "admin", "free5gc", "")
	resp, err := client.GetTenantUsers("tenant1")
	if err != nil {
		t.Fatalf("GetTenantUsers failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}