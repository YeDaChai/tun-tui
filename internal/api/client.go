package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
		return nil, fmt.Errorf("api %s %s: %s", method, path, resp.Status)
	}

	return data, nil
}

func (c *Client) Version() (Version, error) {
	data, err := c.request(http.MethodGet, "/version", nil)
	if err != nil {
		return Version{}, err
	}

	var v Version
	return v, json.Unmarshal(data, &v)
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
	if group == DefaultProxyGroup {
		return c.selectProxy(GlobalProxyGroup, node)
	}
	return nil
}

func (c *Client) selectProxy(group, node string) error {
	_, err := c.request(http.MethodPut, "/proxies/"+group, map[string]string{"name": node})
	return err
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
	if ok && global.Now == proxy.Now {
		return nil
	}
	return c.selectProxy(GlobalProxyGroup, proxy.Now)
}

func (c *Client) PatchMode(mode string) error {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "global" {
		if err := c.SyncGlobalFromProxy(); err != nil {
			return err
		}
	}
	_, err := c.request(http.MethodPatch, "/configs", map[string]string{"mode": mode})
	return err
}

func (c *Client) Traffic() (Traffic, error) {
	req, err := http.NewRequest(http.MethodGet, c.base+"/traffic", nil)
	if err != nil {
		return Traffic{}, err
	}
	if c.secret != "" {
		req.Header.Set("Authorization", "Bearer "+c.secret)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return Traffic{}, err
	}
	defer resp.Body.Close()

	buf := make([]byte, 4096)
	n, err := resp.Body.Read(buf)
	if err != nil && err != io.EOF {
		return Traffic{}, err
	}

	var t Traffic
	return t, json.Unmarshal(buf[:n], &t)
}

func (c *Client) GroupDelay(group, testURL string) (map[string]uint16, error) {
	path := fmt.Sprintf("/group/%s/delay?url=%s&timeout=5000", group, testURL)
	data, err := c.request(http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var delays map[string]uint16
	return delays, json.Unmarshal(data, &delays)
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
	req, err := http.NewRequest(http.MethodPut, c.base+"/providers/proxies/"+name, nil)
	if err != nil {
		return err
	}
	if c.secret != "" {
		req.Header.Set("Authorization", "Bearer "+c.secret)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("api PUT /providers/proxies/%s: %s %s", name, resp.Status, strings.TrimSpace(string(body)))
	}
	return nil
}
