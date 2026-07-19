package api

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultTimeout = 10 * time.Second

type Client struct {
	base   string
	secret string
	http   *http.Client
}

func New(base, secret string) *Client {
	return &Client{
		base:   base,
		secret: secret,
		http:   &http.Client{Timeout: defaultTimeout},
	}
}

func (c *Client) SetSecret(secret string) {
	c.secret = secret
}

// newRequest builds an authenticated request; body is JSON-marshalled when non-nil.
func (c *Client) newRequest(ctx context.Context, method, path string, body any) (*http.Request, error) {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.base+path, reader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.secret != "" {
		req.Header.Set("Authorization", "Bearer "+c.secret)
	}
	return req, nil
}

// do runs req with the given timeout (0 = default). Status >= 400 becomes *HTTPError.
func (c *Client) do(req *http.Request, timeout time.Duration) (*http.Response, error) {
	client := c.http
	if timeout > 0 && timeout != defaultTimeout {
		clone := *c.http
		clone.Timeout = timeout
		client = &clone
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		_ = resp.Body.Close()
		return nil, &HTTPError{
			Method:     req.Method,
			Path:       req.URL.RequestURI(),
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			Body:       strings.TrimSpace(string(body)),
		}
	}
	return resp, nil
}

// request sends a request with the default timeout and returns the response body.
func (c *Client) request(method, path string, body any) ([]byte, error) {
	req, err := c.newRequest(context.Background(), method, path, body)
	if err != nil {
		return nil, err
	}
	resp, err := c.do(req, 0)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

type HTTPError struct {
	Method     string
	Path       string
	StatusCode int
	Status     string
	Body       string
}

func (e *HTTPError) Error() string {
	if e.Body != "" {
		return fmt.Sprintf("api %s %s: %s: %s", e.Method, e.Path, e.Status, e.Body)
	}
	return fmt.Sprintf("api %s %s: %s", e.Method, e.Path, e.Status)
}

func isTransientProxyErr(err error) bool {
	var he *HTTPError
	if errors.As(err, &he) {
		return he.StatusCode == http.StatusNotFound || he.StatusCode == http.StatusBadRequest
	}
	return false
}

func (c *Client) Configs() (Configs, error) {
	data, err := c.request(http.MethodGet, "/configs", nil)
	if err != nil {
		return Configs{}, err
	}

	var cfg Configs
	return cfg, json.Unmarshal(data, &cfg)
}

func (c *Client) Proxies() (ProxiesResponse, error) {
	data, err := c.request(http.MethodGet, "/proxies", nil)
	if err != nil {
		return ProxiesResponse{}, err
	}

	var resp ProxiesResponse
	return resp, json.Unmarshal(data, &resp)
}

const GlobalProxyGroup = "GLOBAL"
const DefaultProxyGroup = "PROXY"

func (c *Client) SelectProxy(group, node string) error {
	if err := c.selectProxy(group, node); err != nil {
		return err
	}
	// GLOBAL may not have subscription nodes yet during provider warm-up.
	if group == DefaultProxyGroup {
		_ = c.selectProxy(GlobalProxyGroup, node)
	}
	return nil
}

func (c *Client) selectProxy(group, node string) error {
	path := "/proxies/" + url.PathEscape(group)
	_, err := c.request(http.MethodPut, path, map[string]string{"name": node})
	return err
}

func proxyGroupHasNode(p Proxy, name string) bool {
	for _, n := range p.All {
		if n == name {
			return true
		}
	}
	return false
}

func (c *Client) SyncGlobalFromProxy() error {
	proxies, err := c.Proxies()
	if err != nil {
		return err
	}
	proxy, ok := proxies.Proxies[DefaultProxyGroup]
	if !ok || proxy.Now == "" {
		return nil
	}
	global, ok := proxies.Proxies[GlobalProxyGroup]
	if !ok {
		return nil
	}
	if global.Now == proxy.Now {
		return nil
	}
	// Subscription provider still loading — skip and retry later.
	if !proxyGroupHasNode(global, proxy.Now) {
		return nil
	}
	return c.selectProxy(GlobalProxyGroup, proxy.Now)
}

// SyncGlobalFromProxyRetry waits briefly for provider nodes, then syncs GLOBAL.
// Transient 400/404 during warm-up are ignored so mode switching still works.
func (c *Client) SyncGlobalFromProxyRetry() error {
	for i := 0; i < 5; i++ {
		err := c.SyncGlobalFromProxy()
		if err != nil {
			if isTransientProxyErr(err) {
				time.Sleep(300 * time.Millisecond)
				continue
			}
			return err
		}

		proxies, perr := c.Proxies()
		if perr != nil {
			return nil
		}
		proxy, ok := proxies.Proxies[DefaultProxyGroup]
		if !ok || proxy.Now == "" {
			return nil
		}
		global, ok := proxies.Proxies[GlobalProxyGroup]
		if !ok || global.Now == proxy.Now || !proxyGroupHasNode(global, proxy.Now) {
			return nil
		}
		time.Sleep(300 * time.Millisecond)
	}
	// Warm-up may still be in progress; don't fail the mode switch.
	return nil
}

func (c *Client) PatchMode(mode string) error {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "global" {
		_ = c.SyncGlobalFromProxyRetry()
	}
	_, err := c.request(http.MethodPatch, "/configs", map[string]string{"mode": mode})
	return err
}

// Traffic reads one snapshot from the streaming /traffic endpoint.
func (c *Client) Traffic() (Traffic, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, err := c.newRequest(ctx, http.MethodGet, "/traffic", nil)
	if err != nil {
		return Traffic{}, err
	}
	resp, err := c.do(req, 3*time.Second)
	if err != nil {
		return Traffic{}, err
	}
	defer resp.Body.Close()

	line, err := bufio.NewReader(resp.Body).ReadBytes('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return Traffic{}, err
	}
	line = bytes.TrimSpace(line)
	if len(line) == 0 {
		return Traffic{}, fmt.Errorf("empty traffic frame")
	}

	var t Traffic
	return t, json.Unmarshal(line, &t)
}

const delayTestURL = "https://www.gstatic.com/generate_204"

// ProxyDelay runs Mihomo URLTest for a single proxy node.
func (c *Client) ProxyDelay(ctx context.Context, name string) (uint16, error) {
	path := fmt.Sprintf(
		"/proxies/%s/delay?url=%s&timeout=5000&expected=204",
		url.PathEscape(name),
		url.QueryEscape(delayTestURL),
	)
	req, err := c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return 0, err
	}
	resp, err := c.do(req, 8*time.Second)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var out struct {
		Delay uint16 `json:"delay"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return 0, err
	}
	return out.Delay, nil
}

func (c *Client) Providers() (ProvidersResponse, error) {
	data, err := c.request(http.MethodGet, "/providers/proxies", nil)
	if err != nil {
		return ProvidersResponse{}, err
	}

	var resp ProvidersResponse
	return resp, json.Unmarshal(data, &resp)
}

func (c *Client) UpdateProvider(name string) error {
	req, err := c.newRequest(context.Background(), http.MethodPut, "/providers/proxies/"+url.PathEscape(name), nil)
	if err != nil {
		return err
	}
	resp, err := c.do(req, 30*time.Second)
	if err != nil {
		return err
	}
	return resp.Body.Close()
}

type Configs struct {
	Mode     string `json:"mode"`
	LogLevel string `json:"log-level"`
}

type Proxy struct {
	Name string   `json:"name"`
	Now  string   `json:"now"`
	All  []string `json:"all"`
}

type ProxiesResponse struct {
	Proxies map[string]Proxy `json:"proxies"`
}

type Traffic struct {
	Up   int64 `json:"up"`
	Down int64 `json:"down"`
}

type SubscriptionInfo struct {
	Upload   int64 `json:"upload"`
	Download int64 `json:"download"`
	Total    int64 `json:"total"`
}

type ProxyProvider struct {
	Name             string            `json:"name"`
	SubscriptionInfo *SubscriptionInfo `json:"subscriptionInfo"`
}

type ProvidersResponse struct {
	Providers map[string]ProxyProvider `json:"providers"`
}
