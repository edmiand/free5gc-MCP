package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ory/dockertest/v3"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}

func TestE2E_ServerWithWebUI(t *testing.T) {
	fmt.Printf("\tTestE2E_ServerWithWebUI: Expected end-to-end test with MCP server and WebUI\n")

	// Get the current working directory
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Could not get working directory: %s", err)
	}
	projectRoot := filepath.Join(wd, "..", "..")

	// Setup dockertest pool
	pool, err := dockertest.NewPool("")
	if err != nil {
		t.Fatalf("Could not construct pool: %s", err)
	}

	// Start MongoDB container
	mongoResource, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "mongo",
		Tag:        "latest",
		Env:        []string{"MONGO_INITDB_DATABASE=free5gc"},
		Name:       "free5gc-test-mongo",
	})
	if err != nil {
		t.Fatalf("Could not start mongo resource: %s", err)
	}
	defer func() {
		if err := pool.Purge(mongoResource); err != nil {
			t.Logf("Could not purge mongo resource: %s", err)
		}
	}()

	// Get MongoDB host and port
	mongoHostAndPort := mongoResource.GetHostPort("27017/tcp")
	t.Logf("MongoDB is running at: %s", mongoHostAndPort)

	// Start WebUI container
	webuiResource, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "free5gc/webui",
		Tag:        "latest",
		Name:       "free5gc-test-webui",
		Env: []string{
			"GIN_MODE=release",
		},
		Mounts: []string{
			filepath.Join(projectRoot, "webui", "webuicfg.yaml") + ":/free5gc/config/webuicfg.yaml",
		},
		Cmd: []string{"./webui", "-c", "./config/webuicfg.yaml"},
		Links: []string{
			mongoResource.Container.Name + ":db",
		},
	})
	if err != nil {
		t.Fatalf("Could not start webui resource: %s", err)
	}
	defer func() {
		if err := pool.Purge(webuiResource); err != nil {
			t.Logf("Could not purge webui resource: %s", err)
		}
	}()

	// Wait for MongoDB to be ready
	if err := pool.Retry(func() error {
		conn, err := net.Dial("tcp", mongoHostAndPort)
		if err != nil {
			return err
		}
		conn.Close()
		return nil
	}); err != nil {
		t.Fatalf("Could not connect to mongo: %s", err)
	}

	// Get WebUI host and port
	webuiHostAndPort := webuiResource.GetHostPort("5000/tcp")
	t.Logf("WebUI is running at: %s", webuiHostAndPort)

	// Create test config with correct webui URL
	testConfigPath := filepath.Join(projectRoot, "config", "test-config.yaml")
	testConfigContent := fmt.Sprintf(`server:
  addr: ":8080"
free5gc:
  webui_base_url: "http://%s"
  username: "admin"
  password: "free5gc"
  free5gc_path: ""
infrastructure:
  use_microk8s: false
`, webuiHostAndPort)
	if err := os.WriteFile(testConfigPath, []byte(testConfigContent), 0644); err != nil {
		t.Fatalf("Failed to create test config: %s", err)
	}
	defer os.Remove(testConfigPath)

	// Wait for WebUI to be ready
	webuiURL := "http://" + webuiHostAndPort
	if err := waitForService(webuiURL+"/api/login", "webui"); err != nil {
		t.Fatalf("WebUI did not start: %s", err)
	}
	t.Logf("WebUI is ready at %s", webuiURL)

	// Build the MCP server binary
	mcpBinary := filepath.Join(projectRoot, "bin", "free5gc-mcp")
	if _, err := os.Stat(mcpBinary); os.IsNotExist(err) {
		t.Logf("Building MCP server binary...")
		if err := runCommand(projectRoot, "make", "build"); err != nil {
			t.Fatalf("Failed to build MCP server: %s", err)
		}
	}

	// Start MCP server process with config
	mcpCmd := exec.Command(mcpBinary, "--config", testConfigPath, "--addr", ":8080")
	mcpCmd.Stdout = os.Stdout
	mcpCmd.Stderr = os.Stderr
	if err := mcpCmd.Start(); err != nil {
		t.Fatalf("Failed to start MCP server: %s", err)
	}
	defer func() {
		if mcpCmd.Process != nil {
			mcpCmd.Process.Kill()
		}
	}()

	// Wait for MCP server to be ready
	mcpURL := "http://127.0.0.1:8080"
	if err := waitForService(mcpURL+"/health", "MCP server"); err != nil {
		t.Fatalf("MCP server did not start: %s", err)
	}
	t.Logf("MCP server is ready at %s", mcpURL)

	// Now run the actual e2e tests
	runE2ETestsWithWebUI(t, mcpURL, webuiURL)
}

func waitForService(url, serviceName string) error {
	client := &http.Client{Timeout: 2 * time.Second}
	for i := 0; i < 60; i++ { // Increased timeout for webui
		var resp *http.Response
		var err error

		if strings.Contains(url, "/api/login") {
			// For login endpoint, try POST request
			loginData := map[string]interface{}{
				"username": "admin",
				"password": "free5gc",
			}
			jsonData, _ := json.Marshal(loginData)
			resp, err = client.Post(url, "application/json", bytes.NewReader(jsonData))
		} else {
			resp, err = client.Get(url)
		}

		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
				return nil
			}
		}
		time.Sleep(1 * time.Second)
	}
	return fmt.Errorf("%s did not become ready", serviceName)
}

func runE2ETestsWithWebUI(t *testing.T, mcpURL, webuiURL string) {
	client := &http.Client{Timeout: 10 * time.Second}

	// First, authenticate with the WebUI to get a token
	authToken := authenticateWithWebUI(t, client, webuiURL)
	if authToken == "" {
		t.Fatalf("Failed to authenticate with WebUI")
	}
	t.Logf("Successfully authenticated with WebUI")

	// Test 1: Health check
	t.Run("HealthCheck", func(t *testing.T) {
		fmt.Printf("\t\tHealthCheck: Expected GET /health to return status 200\n")

		resp, err := client.Get(mcpURL + "/health")
		if err != nil {
			t.Fatalf("Health check failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("Failed to read response body: %v", err)
		}

		var health map[string]interface{}
		if err := json.Unmarshal(body, &health); err != nil {
			t.Fatalf("Failed to parse health response: %v", err)
		}

		if status, ok := health["status"].(string); !ok || status != "ok" {
			t.Errorf("Expected health status 'ok', got: %v", health)
		}
	})

	// Test 2: MCP Initialize
	t.Run("MCPInitialize", func(t *testing.T) {
		fmt.Printf("\t\tMCPInitialize: Expected initialize call to succeed\n")

		reqBody := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  "initialize",
			"params": map[string]interface{}{
				"protocolVersion": "2025-03-26",
			},
		}

		resp, err := makeJSONRPCRequest(client, mcpURL+"/", reqBody)
		if err != nil {
			t.Fatalf("Initialize request failed: %v", err)
		}

		if resp["error"] != nil {
			t.Errorf("Expected no error in initialize response, got: %v", resp["error"])
		}

		if result, ok := resp["result"].(map[string]interface{}); !ok {
			t.Errorf("Expected result in initialize response, got: %v", resp)
		} else {
			if protocolVersion, ok := result["protocolVersion"].(string); !ok || protocolVersion != "2025-03-26" {
				t.Errorf("Expected protocol version '2025-03-26', got: %v", result)
			}
		}
	})

	// Test 3: Tools List
	t.Run("ToolsList", func(t *testing.T) {
		fmt.Printf("\t\tToolsList: Expected tools/list to return available tools\n")

		reqBody := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      2,
			"method":  "tools/list",
			"params":  map[string]interface{}{},
		}

		resp, err := makeJSONRPCRequest(client, mcpURL+"/", reqBody)
		if err != nil {
			t.Fatalf("Tools list request failed: %v", err)
		}

		if resp["error"] != nil {
			t.Errorf("Expected no error in tools/list response, got: %v", resp["error"])
		}

		if result, ok := resp["result"].(map[string]interface{}); !ok {
			t.Errorf("Expected result in tools/list response, got: %v", resp)
		} else {
			if tools, ok := result["tools"].([]interface{}); !ok || len(tools) == 0 {
				t.Errorf("Expected tools array with content, got: %v", result)
			}
		}
	})

	// Test 4: Subscriber operations
	t.Run("SubscriberOperations", func(t *testing.T) {
		fmt.Printf("\t\tSubscriberOperations: Expected full CRUD operations on subscribers\n")

		// Create subscriber
		createReq := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      10,
			"method":  "tools/call",
			"params": map[string]interface{}{
				"name": "subscriber_create",
				"arguments": map[string]interface{}{
					"ueId":           "imsi-208930000000001",
					"servingPlmnId": "20893",
					"subscriberData": map[string]interface{}{
						"authType": "5g_aka",
						"ueId":     "imsi-208930000000001",
					},
				},
			},
		}

		resp, err := makeJSONRPCRequest(client, mcpURL+"/", createReq)
		if err != nil {
			t.Fatalf("Subscriber create request failed: %v", err)
		}

		if resp["error"] != nil {
			t.Errorf("Expected no error in subscriber_create response, got: %v", resp["error"])
		}

		// Get subscriber
		getReq := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      11,
			"method":  "tools/call",
			"params": map[string]interface{}{
				"name": "subscriber_get",
				"arguments": map[string]interface{}{
					"ueId":           "imsi-208930000000001",
					"servingPlmnId": "20893",
				},
			},
		}

		resp, err = makeJSONRPCRequest(client, mcpURL+"/", getReq)
		if err != nil {
			t.Fatalf("Subscriber get request failed: %v", err)
		}

		if resp["error"] != nil {
			t.Errorf("Expected no error in subscriber_get response, got: %v", resp["error"])
		}

		// List subscribers
		listReq := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      12,
			"method":  "tools/call",
			"params": map[string]interface{}{
				"name":      "subscriber_list",
				"arguments": map[string]interface{}{},
			},
		}

		resp, err = makeJSONRPCRequest(client, mcpURL+"/", listReq)
		if err != nil {
			t.Fatalf("Subscriber list request failed: %v", err)
		}

		if resp["error"] != nil {
			t.Errorf("Expected no error in subscriber_list response, got: %v", resp["error"])
		}

		// Delete subscriber
		deleteReq := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      13,
			"method":  "tools/call",
			"params": map[string]interface{}{
				"name": "subscriber_delete",
				"arguments": map[string]interface{}{
					"ueId":           "imsi-208930000000001",
					"servingPlmnId": "20893",
				},
			},
		}

		resp, err = makeJSONRPCRequest(client, mcpURL+"/", deleteReq)
		if err != nil {
			t.Fatalf("Subscriber delete request failed: %v", err)
		}

		if resp["error"] != nil {
			t.Errorf("Expected no error in subscriber_delete response, got: %v", resp["error"])
		}
	})
}

func authenticateWithWebUI(t *testing.T, client *http.Client, webuiURL string) string {
	loginReq := map[string]interface{}{
		"username": "admin",
		"password": "free5gc",
	}

	jsonData, err := json.Marshal(loginReq)
	if err != nil {
		t.Fatalf("Failed to marshal login request: %v", err)
	}

	resp, err := client.Post(webuiURL+"/api/login", "application/json", bytes.NewReader(jsonData))
	if err != nil {
		t.Fatalf("Login request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Login failed with status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read login response: %v", err)
	}

	var loginResp map[string]interface{}
	if err := json.Unmarshal(body, &loginResp); err != nil {
		t.Fatalf("Failed to parse login response: %v", err)
	}

	token, ok := loginResp["access_token"].(string)
	if !ok || token == "" {
		t.Fatalf("No access_token in login response: %v", loginResp)
	}

	return token
}

func makeJSONRPCRequest(client *http.Client, url string, reqBody interface{}) (map[string]interface{}, error) {
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	resp, err := client.Post(url, "application/json", bytes.NewReader(jsonData))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	return result, nil
}

func runCommand(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}