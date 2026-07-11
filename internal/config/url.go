package config

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// ValidateSubscriptionURL rejects non-http(s) and addresses that would let a
// root-privileged mihomo fetch hit loopback / private / link-local networks.
func ValidateSubscriptionURL(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fmt.Errorf("订阅地址不能为空")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("订阅地址无效")
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("仅支持 http/https 订阅地址")
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("订阅地址缺少主机名")
	}
	lower := strings.ToLower(host)
	switch lower {
	case "localhost", "metadata.google.internal", "metadata":
		return fmt.Errorf("不允许指向本机或云元数据的订阅地址")
	}
	if strings.HasSuffix(lower, ".local") || strings.HasSuffix(lower, ".localhost") {
		return fmt.Errorf("不允许指向本机或局域网的订阅地址")
	}

	if ip := net.ParseIP(host); ip != nil {
		if isBlockedIP(ip) {
			return fmt.Errorf("不允许指向内网或本机 IP 的订阅地址")
		}
		return nil
	}

	addrs, err := net.LookupIP(host)
	if err != nil {
		// DNS failure: allow save but mihomo fetch will fail later.
		// Blocking here would reject temporary DNS outages.
		return nil
	}
	for _, ip := range addrs {
		if isBlockedIP(ip) {
			return fmt.Errorf("订阅域名解析到内网或本机地址，已拒绝")
		}
	}
	return nil
}

func isBlockedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast() {
		return true
	}
	if ip4 := ip.To4(); ip4 != nil {
		// 169.254.0.0/16 link-local / cloud metadata
		if ip4[0] == 169 && ip4[1] == 254 {
			return true
		}
		// 100.64.0.0/10 CGNAT often used internally
		if ip4[0] == 100 && ip4[1] >= 64 && ip4[1] <= 127 {
			return true
		}
	}
	return false
}
