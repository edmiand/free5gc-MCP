package control

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"time"
)

// Free5GCClient is a small wrapper to call the free5GC WebUI/backend.
type Free5GCClient struct {
	BaseURL        string
	Token          string
	SubscribersPath string
	HTTPClient     *http.Client
}

func NewFree5GCClient(baseURL, token, subsPath string) *Free5GCClient {
	return &Free5GCClient{
		BaseURL: baseURL,
		Token: token,
		SubscribersPath: subsPath,
		HTTPClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (c *Free5GCClient) makeURL(p string) string {
	u, err := url.Parse(c.BaseURL)
	if err != nil {
		return c.BaseURL + p
	}
	// join path segments
	u.Path = path.Join(u.Path, p)
	return u.String()
}

func (c *Free5GCClient) doRequest(method, p string, body io.Reader, headers map[string]string) (*http.Response, error) {
	full := c.makeURL(p)
	req, err := http.NewRequest(method, full, body)
	if err != nil {
		return nil, err
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return c.HTTPClient.Do(req)
}

// Subscriber operations — these forward to the WebUI backend endpoints.
func (c *Free5GCClient) ListSubscribers() (*http.Response, error) {
	return c.doRequest(http.MethodGet, c.SubscribersPath, nil, nil)
}

func (c *Free5GCClient) CreateSubscriber(body io.Reader) (*http.Response, error) {
	headers := map[string]string{"Content-Type": "application/json"}
	return c.doRequest(http.MethodPost, c.SubscribersPath, body, headers)
}

func (c *Free5GCClient) GetSubscriber(id string) (*http.Response, error) {
	p := path.Join(c.SubscribersPath, id)
	return c.doRequest(http.MethodGet, p, nil, nil)
}

func (c *Free5GCClient) UpdateSubscriber(id string, body io.Reader) (*http.Response, error) {
	p := path.Join(c.SubscribersPath, id)
	headers := map[string]string{"Content-Type": "application/json"}
	return c.doRequest(http.MethodPut, p, body, headers)
}

func (c *Free5GCClient) DeleteSubscriber(id string) (*http.Response, error) {
	p := path.Join(c.SubscribersPath, id)
	return c.doRequest(http.MethodDelete, p, nil, nil)
}

// Simple control stubs (left for later implementation)
func (c *Free5GCClient) Start(component string) error {
	return fmt.Errorf("not implemented")
}

func (c *Free5GCClient) Stop(component string) error {
	return fmt.Errorf("not implemented")
}

func (c *Free5GCClient) Status(component string) (string, error) {
	return "unknown", fmt.Errorf("not implemented")
}
