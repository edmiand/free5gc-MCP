package control

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

)

// Free5GCClient is a small wrapper to call the free5GC WebUI/backend.
// Security note: Credentials (Username, Password) are stored in plaintext in memory.
// Ensure this struct is not logged or exposed. Change default passwords in production.
type Free5GCClient struct {
	BaseURL     string
	Username    string
	Password    string
	Token       string
	TokenMu     sync.RWMutex
	HTTPClient  *http.Client
	Free5GCPath string
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginResponse struct {
	AccessToken string `json:"access_token"`
}

func NewFree5GCClient(baseURL, username, password, free5gcPath string) *Free5GCClient {
	return &Free5GCClient{
		BaseURL:     baseURL,
		Username:    username,
		Password:    password,
		Free5GCPath: free5gcPath,
		HTTPClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// Login authenticates with the webconsole and stores the JWT token
func (c *Free5GCClient) Login() error {
	token, err := c.doLogin()
	if err != nil {
		return err
	}
	
	c.TokenMu.Lock()
	c.Token = token
	c.TokenMu.Unlock()
	
	return nil
}

// doLogin performs the actual login request and returns the token
// This internal method doesn't acquire the lock, allowing it to be called
// from within doRequestWithRetry which may already hold the lock
func (c *Free5GCClient) doLogin() (string, error) {
	loginData := loginRequest{
		Username: c.Username,
		Password: c.Password,
	}
	body, err := json.Marshal(loginData)
	if err != nil {
		return "", fmt.Errorf("failed to marshal login request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, c.BaseURL+"/api/login", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to create login request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("login request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("login failed with status %d: %s", resp.StatusCode, string(b))
	}

	var loginResp loginResponse
	if err := json.NewDecoder(resp.Body).Decode(&loginResp); err != nil {
		return "", fmt.Errorf("failed to decode login response: %w", err)
	}

	return loginResp.AccessToken, nil
}

func (c *Free5GCClient) getToken() string {
	c.TokenMu.RLock()
	defer c.TokenMu.RUnlock()
	return c.Token
}

func (c *Free5GCClient) doRequest(method, path string, body io.Reader, headers map[string]string) (*http.Response, error) {
	url := c.BaseURL + path
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	
	token := c.getToken()
	if token != "" {
		req.Header.Set("Token", token)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return c.HTTPClient.Do(req)
}

// doRequestWithRetry makes a request and retries with re-login if token is invalid
// Accepts bodyBytes as []byte to allow retry with a fresh reader if the first attempt fails
func (c *Free5GCClient) doRequestWithRetry(method, path string, bodyBytes []byte, headers map[string]string) (*http.Response, error) {
	// First, ensure we have a token (atomic check and login)
	c.TokenMu.Lock()
	if c.Token == "" {
		token, err := c.doLogin()
		if err != nil {
			c.TokenMu.Unlock()
			return nil, fmt.Errorf("initial login failed: %w", err)
		}
		c.Token = token
	}
	c.TokenMu.Unlock()

	var body io.Reader
	if bodyBytes != nil {
		body = bytes.NewReader(bodyBytes)
	}
	resp, err := c.doRequest(method, path, body, headers)
	if err != nil {
		return nil, err
	}

	// If we get a 401 or 403, try to re-login
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		resp.Body.Close()
		
		// Re-login (this will acquire the lock internally)
		if err := c.Login(); err != nil {
			return nil, fmt.Errorf("re-login failed: %w", err)
		}

		// Retry the request with new body reader
		if bodyBytes != nil {
			body = bytes.NewReader(bodyBytes)
		}
		return c.doRequest(method, path, body, headers)
	}

	return resp, nil
}

// GetTenantUsers calls GET /api/tenant/:tenantId/user on the webconsole backend
// to retrieve all users for a specific tenant.
func (c *Free5GCClient) GetTenantUsers(tenantId string) (*http.Response, error) {
	path := fmt.Sprintf("/api/tenant/%s/user", tenantId)
	return c.doRequestWithRetry(http.MethodGet, path, nil, nil)
}

// ============================================
// Subscriber Management API Methods
// ============================================

// GetSubscribers calls GET /api/subscriber on the webconsole backend
// to retrieve all subscribers.
func (c *Free5GCClient) GetSubscribers() (*http.Response, error) {
	return c.doRequestWithRetry(http.MethodGet, "/api/subscriber", nil, nil)
}

// DeleteMultipleSubscribers calls DELETE /api/subscriber on the webconsole backend
// to delete multiple subscribers. The bodyBytes should contain the list of subscribers to delete.
func (c *Free5GCClient) DeleteMultipleSubscribers(bodyBytes []byte) (*http.Response, error) {
	headers := map[string]string{"Content-Type": "application/json"}
	return c.doRequestWithRetry(http.MethodDelete, "/api/subscriber", bodyBytes, headers)
}

// GetSubscriberByID calls GET /api/subscriber/:ueId/:servingPlmnId on the webconsole backend
// to retrieve a specific subscriber by UE ID and PLMN ID.
func (c *Free5GCClient) GetSubscriberByID(ueId, servingPlmnId string) (*http.Response, error) {
	path := fmt.Sprintf("/api/subscriber/%s/%s", ueId, servingPlmnId)
	return c.doRequestWithRetry(http.MethodGet, path, nil, nil)
}

// CreateSubscriber calls POST /api/subscriber/:ueId/:servingPlmnId on the webconsole backend
// to create a new subscriber.
func (c *Free5GCClient) CreateSubscriber(ueId, servingPlmnId string, bodyBytes []byte) (*http.Response, error) {
	path := fmt.Sprintf("/api/subscriber/%s/%s", ueId, servingPlmnId)
	headers := map[string]string{"Content-Type": "application/json"}
	return c.doRequestWithRetry(http.MethodPost, path, bodyBytes, headers)
}

// CreateMultipleSubscribers calls POST /api/subscriber/:ueId/:servingPlmnId/:userNumber on the webconsole backend
// to create multiple subscribers at once.
func (c *Free5GCClient) CreateMultipleSubscribers(ueId, servingPlmnId string, userNumber int, bodyBytes []byte) (*http.Response, error) {
	path := fmt.Sprintf("/api/subscriber/%s/%s/%d", ueId, servingPlmnId, userNumber)
	headers := map[string]string{"Content-Type": "application/json"}
	return c.doRequestWithRetry(http.MethodPost, path, bodyBytes, headers)
}

// UpdateSubscriber calls PUT /api/subscriber/:ueId/:servingPlmnId on the webconsole backend
// to update a subscriber.
func (c *Free5GCClient) UpdateSubscriber(ueId, servingPlmnId string, bodyBytes []byte) (*http.Response, error) {
	path := fmt.Sprintf("/api/subscriber/%s/%s", ueId, servingPlmnId)
	headers := map[string]string{"Content-Type": "application/json"}
	return c.doRequestWithRetry(http.MethodPut, path, bodyBytes, headers)
}

// DeleteSubscriber calls DELETE /api/subscriber/:ueId/:servingPlmnId on the webconsole backend
// to delete a specific subscriber.
func (c *Free5GCClient) DeleteSubscriber(ueId, servingPlmnId string) (*http.Response, error) {
	path := fmt.Sprintf("/api/subscriber/%s/%s", ueId, servingPlmnId)
	return c.doRequestWithRetry(http.MethodDelete, path, nil, nil)
}

// PatchSubscriber calls PATCH /api/subscriber/:ueId/:servingPlmnId on the webconsole backend
// to partially update a subscriber.
func (c *Free5GCClient) PatchSubscriber(ueId, servingPlmnId string, bodyBytes []byte) (*http.Response, error) {
	path := fmt.Sprintf("/api/subscriber/%s/%s", ueId, servingPlmnId)
	headers := map[string]string{"Content-Type": "application/json"}
	return c.doRequestWithRetry(http.MethodPatch, path, bodyBytes, headers)
}

// ============================================
// Local Core Control Methods (Start/Stop/Status)
// ============================================

// NFResourceUsage represents CPU and memory usage for a single network function process
type NFResourceUsage struct {
	Name    string  `json:"name"`
	PID     int     `json:"pid,omitempty"`
	CPU     float64 `json:"cpu_percent,omitempty"`
	MemMB   float64 `json:"mem_mb,omitempty"`
	Status  string  `json:"status"` // "running", "sleeping", "not running", etc.
	Running bool    `json:"running"`
}

// CoreResources represents CPU and memory usage for all free5GC processes
type CoreResources struct {
	NFs        []NFResourceUsage `json:"network_functions"`
	Webconsole NFResourceUsage   `json:"webconsole"`
}

// NFStatus represents the status of a network function
type NFStatus struct {
	Name    string `json:"name"`
	Running bool   `json:"running"`
	PID     int    `json:"pid,omitempty"`
	IP      string `json:"ip,omitempty"`
	Port    int    `json:"port,omitempty"`
	Error   string `json:"error,omitempty"`
}

// CoreStatus represents the overall status of free5GC core
type CoreStatus struct {
	Overall    string     `json:"overall"` // "running", "stopped", "partial", "error"
	Message    string     `json:"message"`
	NFs        []NFStatus `json:"network_functions"`
	Webconsole NFStatus   `json:"webconsole"`
	StartTime  string     `json:"start_time,omitempty"`
}

// CoreStartResult represents the result of starting the core
type CoreStartResult struct {
	Success   bool       `json:"success"`
	Message   string     `json:"message"`
	Details   []string   `json:"details"`
	NFs       []NFStatus `json:"network_functions,omitempty"`
	Warnings  []string   `json:"warnings,omitempty"`
}

// CoreStopResult represents the result of stopping the core
type CoreStopResult struct {
	Success  bool     `json:"success"`
	Message  string   `json:"message"`
	Details  []string `json:"details"`
	Warnings []string `json:"warnings,omitempty"`
}

// NFList is the list of network functions to manage
var NFList = []string{"nrf", "amf", "smf", "udr", "pcf", "udm", "nssf", "ausf", "upf", "chf", "nef"}

// NFEndpoint holds the IP and port configuration for each network function
type NFEndpoint struct {
	IP   string
	Port int
}

// NFEndpoints maps network function names to their IP and port configuration
var NFEndpoints = map[string]NFEndpoint{
	"nrf":        {IP: "127.0.0.10", Port: 8000},   // NRF SBI
	"amf":        {IP: "127.0.0.18", Port: 8000},   // AMF SBI (also NGAP: 38412)
	"smf":        {IP: "127.0.0.2", Port: 8000},    // SMF SBI
	"udr":        {IP: "127.0.0.4", Port: 8000},    // UDR SBI
	"pcf":        {IP: "127.0.0.7", Port: 8000},    // PCF SBI
	"udm":        {IP: "127.0.0.3", Port: 8000},    // UDM SBI
	"nssf":       {IP: "127.0.0.31", Port: 8000},   // NSSF SBI
	"ausf":       {IP: "127.0.0.9", Port: 8000},    // AUSF SBI
	"upf":        {IP: "127.0.0.8", Port: 8805},    // UPF PFCP
	"chf":        {IP: "127.0.0.113", Port: 8000},  // CHF SBI
	"nef":        {IP: "127.0.0.5", Port: 8000},    // NEF SBI
	"webconsole": {IP: "0.0.0.0", Port: 30500},      // Webconsole HTTP (binds to all interfaces)
}

// extractPIDFromSSLine extracts the PID from an ss command output line
// Returns the PID if found and successfully parsed, 0 otherwise
func extractPIDFromSSLine(line string) int {
	if idx := strings.Index(line, "pid="); idx != -1 {
		pidStr := line[idx+4:]
		if endIdx := strings.Index(pidStr, ","); endIdx != -1 {
			pidStr = pidStr[:endIdx]
		}
		var pid int
		if n, err := fmt.Sscanf(pidStr, "%d", &pid); err == nil && n == 1 {
			return pid
		}
	}
	return 0
}

// checkPortListening checks if a specific IP:port is listening (TCP or UDP)
func checkPortListening(ip string, port int, checkUDP bool) (bool, int) {
	// For 0.0.0.0, ss shows it as *:port
	var addr string
	if ip == "0.0.0.0" {
		addr = fmt.Sprintf("*:%d", port)
	} else {
		addr = fmt.Sprintf("%s:%d", ip, port)
	}
	
	// Check TCP first
	cmd := exec.Command("ss", "-tlnp")
	output, err := cmd.Output()
	if err != nil {
		// ss command may not be available or failed to execute
		// Return false to indicate port is not confirmed as listening
		return false, 0
	}
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, addr) {
			pid := extractPIDFromSSLine(line)
			return true, pid
		}
	}
	
	// If checkUDP is true, also check UDP ports (for UPF PFCP)
	if checkUDP {
		cmd = exec.Command("ss", "-ulnp")
		output, err = cmd.Output()
		if err != nil {
			// ss command may not be available or failed to execute
			return false, 0
		}
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if strings.Contains(line, addr) {
				pid := extractPIDFromSSLine(line)
				return true, pid
			}
		}
	}
	
	return false, 0
}

// GetFree5GCStatus returns the current status of all free5GC network functions and webconsole
func (c *Free5GCClient) GetFree5GCStatus() (*CoreStatus, error) {
	status := &CoreStatus{
		NFs: make([]NFStatus, 0, len(NFList)),
	}
	
	runningCount := 0
	totalCount := len(NFList)
	
	// Check each NF by its IP:port
	for _, nf := range NFList {
		config := NFEndpoints[nf]
		// UPF uses UDP for PFCP, so check UDP for it
		checkUDP := (nf == "upf")
		running, pid := checkPortListening(config.IP, config.Port, checkUDP)
		nfStatus := NFStatus{
			Name:    nf,
			Running: running,
			PID:     pid,
			IP:      config.IP,
			Port:    config.Port,
		}
		status.NFs = append(status.NFs, nfStatus)
		if running {
			runningCount++
		}
	}
	
	// Check webconsole (TCP only)
	wcConfig := NFEndpoints["webconsole"]
	wcRunning, wcPid := checkPortListening(wcConfig.IP, wcConfig.Port, false)
	status.Webconsole = NFStatus{
		Name:    "webconsole",
		Running: wcRunning,
		PID:     wcPid,
		IP:      wcConfig.IP,
		Port:    wcConfig.Port,
	}
	
	// Determine overall status
	switch {
	case runningCount == 0:
		status.Overall = "stopped"
		status.Message = "All network functions are stopped"
	case runningCount == totalCount:
		status.Overall = "running"
		status.Message = fmt.Sprintf("All %d network functions are running", totalCount)
	default:
		status.Overall = "partial"
		status.Message = fmt.Sprintf("%d of %d network functions are running", runningCount, totalCount)
	}
	
	// Try to get start time from run.pid
	if c.Free5GCPath != "" {
		pidFile := filepath.Join(c.Free5GCPath, "run.pid")
		if info, err := os.Stat(pidFile); err == nil {
			status.StartTime = info.ModTime().Format(time.RFC3339)
		}
	}
	
	return status, nil
}

// StartFree5GC starts all free5GC network functions and webconsole
func (c *Free5GCClient) StartFree5GC(ctx context.Context) (*CoreStartResult, error) {
	result := &CoreStartResult{
		Details:  make([]string, 0),
		Warnings: make([]string, 0),
	}
	
	// Validate free5gc path
	if c.Free5GCPath == "" {
		result.Success = false
		result.Message = "free5gc_path is not configured"
		return result, nil
	}
	
	// Validate Free5GCPath exists and is a directory
	if stat, err := os.Stat(c.Free5GCPath); err != nil || !stat.IsDir() {
		result.Success = false
		result.Message = fmt.Sprintf("free5gc path does not exist or is not a directory: %v", err)
		return result, nil
	}
	
	result.Details = append(result.Details, fmt.Sprintf("Changing directory to: %s", c.Free5GCPath))
	result.Details = append(result.Details, "Executing: sudo ./run.sh (detached)")
	
	// Execute run.sh with sudo in a fully detached background process
	logFile := filepath.Join(c.Free5GCPath, "mcp_start.log")
	runShPath := filepath.Join(c.Free5GCPath, "run.sh")
	logF, err := os.Create(logFile)
	if err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("Failed to create log file: %v", err)
		return result, nil
	}
	defer logF.Close()
	
	cmd := exec.Command("sudo", runShPath)
	cmd.Dir = c.Free5GCPath
	cmd.Stdout = logF
	cmd.Stderr = logF
	cmd.Stdin = nil
	
	// Start and immediately release (do not wait for the process)
	if err := cmd.Start(); err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("Failed to execute run.sh: %v", err)
		return result, nil
	}
	// Detach: release the process to avoid zombie processes
	if err := cmd.Process.Release(); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to release run.sh process: %v", err))
	}
	
	result.Details = append(result.Details, "Started run.sh in background")
	
	// Start webconsole
	result.Details = append(result.Details, "Starting webconsole...")
	webconsolePath := filepath.Join(c.Free5GCPath, "webconsole")
	wcLogFile := filepath.Join(c.Free5GCPath, "mcp_webconsole.log")
	webconsoleBin := filepath.Join(webconsolePath, "bin", "webconsole")
	wcLogF, err := os.Create(wcLogFile)
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to create webconsole log file: %v", err))
	} else {
		defer wcLogF.Close()
		wcCmd := exec.Command(webconsoleBin)
		wcCmd.Dir = webconsolePath
		wcCmd.Stdout = wcLogF
		wcCmd.Stderr = wcLogF
		wcCmd.Stdin = nil
		
		if err := wcCmd.Start(); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to start webconsole: %v", err))
		} else {
			// Detach the process to avoid zombie if not waited on
			if err := wcCmd.Process.Release(); err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to release webconsole process: %v", err))
			} else {
				result.Details = append(result.Details, "Started webconsole in background")
			}
		}
	}
	
	result.Details = append(result.Details, "Waiting for network functions to initialize...")
	
	// Wait for NFs to start with context awareness
	select {
	case <-time.After(10 * time.Second):
		// Continue with status check
	case <-ctx.Done():
		result.Success = false
		result.Message = "Start operation timed out"
		result.Warnings = append(result.Warnings, "Context cancelled before NFs could be verified")
		return result, ctx.Err()
	}
	
	// Check status after starting
	newStatus, _ := c.GetFree5GCStatus()
	if newStatus != nil {
		result.NFs = newStatus.NFs
		result.Success = newStatus.Overall == "running"
		if newStatus.Webconsole.Running {
			result.Message = fmt.Sprintf("%s, webconsole is running", newStatus.Message)
		} else {
			result.Message = fmt.Sprintf("%s, webconsole may still be starting", newStatus.Message)
		}
	}
	
	return result, nil
}

// findProcessUsage scans pre-parsed ps aux lines for a process matching processName
// and returns its resource usage. Only the first matching non-grep process is returned.
func findProcessUsage(lines []string, processName string) NFResourceUsage {
	usage := NFResourceUsage{
		Name:    processName,
		Status:  "not running",
		Running: false,
	}
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 11 {
			continue
		}
		command := fields[10]
		if strings.Contains(command, "grep") {
			continue
		}
		if filepath.Base(command) != processName {
			continue
		}
		var pid int
		fmt.Sscanf(fields[1], "%d", &pid)
		var cpu float64
		fmt.Sscanf(fields[2], "%f", &cpu)
		var rssKB float64
		fmt.Sscanf(fields[5], "%f", &rssKB)
		stat := fields[7]
		status := "running"
		if strings.HasPrefix(stat, "Z") {
			status = "zombie"
		} else if strings.HasPrefix(stat, "T") {
			status = "stopped"
		}
		usage.PID = pid
		usage.CPU = cpu
		usage.MemMB = rssKB / 1024.0
		usage.Status = status
		usage.Running = true
		break
	}
	return usage
}

// GetFree5GCResources returns CPU and memory usage for all free5GC network function processes
func (c *Free5GCClient) GetFree5GCResources() (*CoreResources, error) {
	cmd := exec.Command("ps", "aux")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run ps aux: %w", err)
	}
	lines := strings.Split(string(output), "\n")

	resources := &CoreResources{
		NFs: make([]NFResourceUsage, 0, len(NFList)),
	}
	for _, nf := range NFList {
		resources.NFs = append(resources.NFs, findProcessUsage(lines, nf))
	}
	resources.Webconsole = findProcessUsage(lines, "webconsole")
	return resources, nil
}

// StopFree5GC stops all free5GC network functions and webconsole
func (c *Free5GCClient) StopFree5GC(ctx context.Context) (*CoreStopResult, error) {
	result := &CoreStopResult{
		Details:  make([]string, 0),
		Warnings: make([]string, 0),
	}
	
	// Validate free5gc path
	if c.Free5GCPath == "" {
		result.Success = false
		result.Message = "free5gc_path is not configured"
		return result, nil
	}
	
	// Validate path exists and is a directory
	if _, err := os.Stat(c.Free5GCPath); err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("free5gc path does not exist: %v", err)
		return result, nil
	}
	
	result.Details = append(result.Details, fmt.Sprintf("Changing directory to: %s", c.Free5GCPath))
	
	// Stop webconsole first - the binary is named 'webconsole' in webconsole/bin/
	result.Details = append(result.Details, "Stopping webconsole...")
	// Kill by matching the webconsole binary path
	webconsoleBinPath := filepath.Join(c.Free5GCPath, "webconsole", "bin", "webconsole")
	
	// Check if pkill is available
	pkillPath, pkillErr := exec.LookPath("pkill")
	fuserPath, fuserErr := exec.LookPath("fuser")
	
	if pkillErr == nil {
		wcCmd := exec.CommandContext(ctx, pkillPath, "-f", webconsoleBinPath)
		if err := wcCmd.Run(); err != nil {
			// Try fuser as fallback if available
			if fuserErr == nil {
				fuserCmd := exec.CommandContext(ctx, fuserPath, "-k", "30500/tcp")
				if fuserErr2 := fuserCmd.Run(); fuserErr2 != nil {
					result.Details = append(result.Details, "Webconsole was not running or already stopped")
				} else {
					result.Details = append(result.Details, "Webconsole stopped (via port kill)")
				}
			} else {
				result.Warnings = append(result.Warnings, "Neither 'pkill' nor 'fuser' commands are available to stop webconsole")
			}
		} else {
			result.Details = append(result.Details, "Webconsole stopped")
		}
	} else if fuserErr == nil {
		fuserCmd := exec.CommandContext(ctx, fuserPath, "-k", "30500/tcp")
		if fuserErr2 := fuserCmd.Run(); fuserErr2 != nil {
			result.Details = append(result.Details, "Webconsole was not running or already stopped")
		} else {
			result.Details = append(result.Details, "Webconsole stopped (via port kill)")
		}
	} else {
		result.Warnings = append(result.Warnings, "Neither 'pkill' nor 'fuser' commands are available to stop webconsole. Please install at least one of them.")
	}
	
	result.Details = append(result.Details, "Executing: ./force_kill.sh")
	
	// Execute force_kill.sh with context
	forceKillPath := filepath.Join(c.Free5GCPath, "force_kill.sh")
	cmd := exec.CommandContext(ctx, "bash", forceKillPath)
	cmd.Dir = c.Free5GCPath
	
	// Run the command and wait for it to complete
	output, err := cmd.CombinedOutput()
	if err != nil {
		result.Details = append(result.Details, fmt.Sprintf("force_kill.sh output: %s", string(output)))
	}
	
	result.Details = append(result.Details, "force_kill.sh executed")
	
	// Wait a moment for processes to terminate
	time.Sleep(2 * time.Second)
	
	// Verify all processes are stopped
	newStatus, _ := c.GetFree5GCStatus()
	if newStatus != nil {
		nfsStopped := newStatus.Overall == "stopped"
		wcStopped := !newStatus.Webconsole.Running
		result.Success = nfsStopped && wcStopped
		if result.Success {
			result.Message = "All network functions and webconsole are stopped"
		} else if nfsStopped && !wcStopped {
			result.Message = "Network functions stopped, but webconsole is still running"
		} else if !nfsStopped && wcStopped {
			result.Message = fmt.Sprintf("%s, webconsole is stopped", newStatus.Message)
		} else {
			result.Message = newStatus.Message
		}
	} else {
		result.Success = true
		result.Message = "Stop command executed"
	}
	
	return result, nil
}

