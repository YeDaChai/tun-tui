package api

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	base   string
	secret string
	http   *http.Client
}

func New(base, secret string) *Client {
	return &Client{
		base:   base,
		secret: secret,
		http: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *Client) request(method, path string, body any) ([]byte, error) {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, c.base+path, reader)
	if err != nil {
		return nil, err
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.secret != "" {
		req.Header.Set("Authorization", "Bearer "+c.secret)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, &HTTPError{Method: method, Path: path, StatusCode: resp.StatusCode, Status: resp.Status}
	}

	return data, nil
}

type HTTPError struct {
	Method     string
	Path       string
	StatusCode int
	Status     string
}

func (e *HTTPError) Error() string {
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
	req, err := http.NewRequest(http.MethodGet, c.base+"/traffic", nil)
	if err != nil {
		return Traffic{}, err
	}
	if c.secret != "" {
		req.Header.Set("Authorization", "Bearer "+c.secret)
	}

	// Streaming endpoint: avoid the shared client timeout so we can read one frame.
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return Traffic{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return Traffic{}, &HTTPError{
			Method:     http.MethodGet,
			Path:       "/traffic",
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
		}
	}

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
func (c *Client) ProxyDelay(name string) (uint16, error) {
	path := fmt.Sprintf(
		"/proxies/%s/delay?url=%s&timeout=5000&expected=204",
		url.PathEscape(name),
		url.QueryEscape(delayTestURL),
	)
	req, err := http.NewRequest(http.MethodGet, c.base+path, nil)
	if err != nil {
		return 0, err
	}
	if c.secret != "" {
		req.Header.Set("Authorization", "Bearer "+c.secret)
	}

	client := *c.http
	client.Timeout = 8 * time.Second
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}
	if resp.StatusCode >= 400 {
		return 0, &HTTPError{
			Method:     http.MethodGet,
			Path:       path,
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
		}
	}

	var out struct {
		Delay uint16 `json:"delay"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
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
	path := "/providers/proxies/" + url.PathEscape(name)
	req, err := http.NewRequest(http.MethodPut, c.base+path, nil)
	if err != nil {
		return err
	}
	if c.secret != "" {
		req.Header.Set("Authorization", "Bearer "+c.secret)
	}

	client := *c.http
	client.Timeout = 30 * time.Second
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("api PUT %s: %s %s", path, resp.Status, strings.TrimSpace(string(body)))
	}
	return nil
}

type Configs struct {
	Mode     string `json:"mode"`
	LogLevel string `json:"log-level"`
}

type Proxy struct {
	Name  string   `json:"name"`
	Type  string   `json:"type"`
	Now   string   `json:"now"`
	All   []string `json:"all"`
	Alive bool     `json:"alive"`
}

type ProxiesResponse struct {
	Proxies map[string]Proxy `json:"proxies"`
}

type Traffic struct {
	Up        int64 `json:"up"`
	Down      int64 `json:"down"`
	UpTotal   int64 `json:"upTotal"`
	DownTotal int64 `json:"downTotal"`
}

type SubscriptionInfo struct {
	Upload   int64 `json:"upload"`
	Download int64 `json:"download"`
	Total    int64 `json:"total"`
	Expire   int64 `json:"expire"`
}

type ProxyProvider struct {
	Name             string            `json:"name"`
	Type             string            `json:"type"`
	VehicleType      string            `json:"vehicleType"`
	Proxies          []Proxy           `json:"proxies"`
	UpdatedAt        string            `json:"updatedAt"`
	SubscriptionInfo *SubscriptionInfo `json:"subscriptionInfo"`
}

type ProvidersResponse struct {
	Providers map[string]ProxyProvider `json:"providers"`
}
