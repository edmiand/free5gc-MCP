package control

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// Free5GCClient is a small wrapper to call the free5GC WebUI/backend.
type Free5GCClient struct {
	BaseURL    string
	Username   string
	Password   string
	Token      string
	TokenMu    sync.RWMutex
	HTTPClient *http.Client
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginResponse struct {
	AccessToken string `json:"access_token"`
}

func NewFree5GCClient(baseURL, username, password string) *Free5GCClient {
	return &Free5GCClient{
		BaseURL:  baseURL,
		Username: username,
		Password: password,
		HTTPClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// Login authenticates with the webconsole and stores the JWT token
func (c *Free5GCClient) Login() error {
	loginData := loginRequest{
		Username: c.Username,
		Password: c.Password,
	}
	body, err := json.Marshal(loginData)
	if err != nil {
		return fmt.Errorf("failed to marshal login request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, c.BaseURL+"/api/login", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create login request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("login request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("login failed with status %d: %s", resp.StatusCode, string(b))
	}

	var loginResp loginResponse
	if err := json.NewDecoder(resp.Body).Decode(&loginResp); err != nil {
		return fmt.Errorf("failed to decode login response: %w", err)
	}

	c.TokenMu.Lock()
	c.Token = loginResp.AccessToken
	c.TokenMu.Unlock()

	return nil
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
func (c *Free5GCClient) doRequestWithRetry(method, path string, body io.Reader, headers map[string]string) (*http.Response, error) {
	// First, ensure we have a token
	if c.getToken() == "" {
		if err := c.Login(); err != nil {
			return nil, fmt.Errorf("initial login failed: %w", err)
		}
	}

	resp, err := c.doRequest(method, path, body, headers)
	if err != nil {
		return nil, err
	}

	// If we get a 401 or the response indicates invalid token, try to re-login
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		resp.Body.Close()
		
		// Re-login
		if err := c.Login(); err != nil {
			return nil, fmt.Errorf("re-login failed: %w", err)
		}

		// Retry the request
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
// to delete multiple subscribers. The body should contain the list of subscribers to delete.
func (c *Free5GCClient) DeleteMultipleSubscribers(body io.Reader) (*http.Response, error) {
	headers := map[string]string{"Content-Type": "application/json"}
	return c.doRequestWithRetry(http.MethodDelete, "/api/subscriber", body, headers)
}

// GetSubscriberByID calls GET /api/subscriber/:ueId/:servingPlmnId on the webconsole backend
// to retrieve a specific subscriber by UE ID and PLMN ID.
func (c *Free5GCClient) GetSubscriberByID(ueId, servingPlmnId string) (*http.Response, error) {
	path := fmt.Sprintf("/api/subscriber/%s/%s", ueId, servingPlmnId)
	return c.doRequestWithRetry(http.MethodGet, path, nil, nil)
}

// CreateSubscriber calls POST /api/subscriber/:ueId/:servingPlmnId on the webconsole backend
// to create a new subscriber.
func (c *Free5GCClient) CreateSubscriber(ueId, servingPlmnId string, body io.Reader) (*http.Response, error) {
	path := fmt.Sprintf("/api/subscriber/%s/%s", ueId, servingPlmnId)
	headers := map[string]string{"Content-Type": "application/json"}
	return c.doRequestWithRetry(http.MethodPost, path, body, headers)
}

// CreateMultipleSubscribers calls POST /api/subscriber/:ueId/:servingPlmnId/:userNumber on the webconsole backend
// to create multiple subscribers at once.
func (c *Free5GCClient) CreateMultipleSubscribers(ueId, servingPlmnId string, userNumber int, body io.Reader) (*http.Response, error) {
	path := fmt.Sprintf("/api/subscriber/%s/%s/%d", ueId, servingPlmnId, userNumber)
	headers := map[string]string{"Content-Type": "application/json"}
	return c.doRequestWithRetry(http.MethodPost, path, body, headers)
}

// UpdateSubscriber calls PUT /api/subscriber/:ueId/:servingPlmnId on the webconsole backend
// to update a subscriber.
func (c *Free5GCClient) UpdateSubscriber(ueId, servingPlmnId string, body io.Reader) (*http.Response, error) {
	path := fmt.Sprintf("/api/subscriber/%s/%s", ueId, servingPlmnId)
	headers := map[string]string{"Content-Type": "application/json"}
	return c.doRequestWithRetry(http.MethodPut, path, body, headers)
}

// DeleteSubscriber calls DELETE /api/subscriber/:ueId/:servingPlmnId on the webconsole backend
// to delete a specific subscriber.
func (c *Free5GCClient) DeleteSubscriber(ueId, servingPlmnId string) (*http.Response, error) {
	path := fmt.Sprintf("/api/subscriber/%s/%s", ueId, servingPlmnId)
	return c.doRequestWithRetry(http.MethodDelete, path, nil, nil)
}

// PatchSubscriber calls PATCH /api/subscriber/:ueId/:servingPlmnId on the webconsole backend
// to partially update a subscriber.
func (c *Free5GCClient) PatchSubscriber(ueId, servingPlmnId string, body io.Reader) (*http.Response, error) {
	path := fmt.Sprintf("/api/subscriber/%s/%s", ueId, servingPlmnId)
	headers := map[string]string{"Content-Type": "application/json"}
	return c.doRequestWithRetry(http.MethodPatch, path, body, headers)
}
