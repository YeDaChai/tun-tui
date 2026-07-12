package config

import (
	"fmt"
	"os"
	"runtime"

	"gopkg.in/yaml.v3"
)

func BuildConfigBytes(dataDir, cfgPath, apiSecret string) ([]byte, error) {
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var root map[string]any
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	subURL, err := LoadSubscriptionURL(dataDir)
	if err != nil {
		return nil, err
	}
	if subURL != "" {
		if err := ValidateSubscriptionURL(subURL); err != nil {
			return nil, fmt.Errorf("订阅地址不安全: %w", err)
		}
		if err := SyncProviderCache(dataDir, subURL); err != nil {
			return nil, err
		}
		if err := applySubscriptionURL(root, subURL); err != nil {
			return nil, err
		}
	}

	applyTunSettings(root)
	applyController(root, apiSecret)
	applyRoutingRules(root)
	applyMode(dataDir, root)

	out, err := yaml.Marshal(root)
	if err != nil {
		return nil, fmt.Errorf("marshal config: %w", err)
	}
	return out, nil
}

func applySubscriptionURL(root map[string]any, subURL string) error {
	providers, ok := root["proxy-providers"].(map[string]any)
	if !ok {
		return fmt.Errorf("config.yaml 缺少 proxy-providers 配置")
	}

	provider, ok := providers[ProviderName].(map[string]any)
	if !ok {
		return fmt.Errorf("config.yaml 缺少 proxy-providers.%s 配置", ProviderName)
	}

	provider["type"] = "http"
	provider["path"] = "./providers/subscription.yaml"
	provider["url"] = subURL
	// 0 = load local cache on start; only refresh when the user presses u.
	provider["interval"] = 0
	providers[ProviderName] = provider
	root["proxy-providers"] = providers
	return nil
}

func applyController(root map[string]any, secret string) {
	root["external-controller"] = "127.0.0.1:9090"
	root["allow-lan"] = false
	if secret != "" {
		root["secret"] = secret
	}
}

// applyTunSettings overwrites tun / dns / sniffer / rules each run so TUN and
// RULE-mode routing stay consistent with the bundled geo databases. Treat those
// sections as app-managed.
func applyTunSettings(root map[string]any) {
	tun, _ := root["tun"].(map[string]any)
	if tun == nil {
		tun = map[string]any{}
	}

	tun["enable"] = true
	stack := "gvisor"
	if runtime.GOOS == "linux" {
		stack = "system"
	}
	tun["stack"] = stack
	tun["auto-route"] = true
	tun["auto-detect-interface"] = true
	tun["strict-route"] = true
	tun["dns-hijack"] = []any{"any:53", "tcp://any:53"}
	tun["inet4-address"] = []any{"198.18.0.1/30"}
	tun["mtu"] = 1500
	tun["endpoint-independent-nat"] = true
	tun["route-exclude-address"] = []any{
		"192.168.0.0/16",
		"10.0.0.0/8",
		"172.16.0.0/12",
		"127.0.0.0/8",
		"224.0.0.0/4",
		"255.255.255.255/32",
	}

	root["ipv6"] = false

	root["dns"] = map[string]any{
		"enable":             true,
		"enhanced-mode":      "fake-ip",
		"ipv6":               false,
		"fake-ip-range":      "198.18.0.1/16",
		"default-nameserver": []any{"223.5.5.5", "119.29.29.29"},
		"nameserver":         []any{"223.5.5.5", "119.29.29.29"},
		"fallback":           []any{},
		"fallback-filter": map[string]any{
			"geoip": false,
		},
		"fake-ip-filter": []any{
			"+.lan",
			"+.local",
			"localhost",
			"*.local",
		},
	}
	root["sniffer"] = map[string]any{
		"enable":               true,
		"parse-pure-ip":        true,
		"override-destination": true,
		"sniff": map[string]any{
			"TLS": map[string]any{
				"ports": []any{443, 8443},
			},
			"HTTP": map[string]any{
				"ports": []any{80, "8080-8880"},
			},
		},
	}
	// Use bundled geoip.metadb (MMDB). geodata-mode:true expects GeoIP.dat and
	// will block startup trying to download it when the file is missing/invalid.
	root["geodata-mode"] = false
	root["geo-auto-update"] = false
	root["log-level"] = "warning"
	root["unified-delay"] = true

	if providers, ok := root["proxy-providers"].(map[string]any); ok {
		if sub, ok := providers[ProviderName].(map[string]any); ok {
			hc, _ := sub["health-check"].(map[string]any)
			if hc == nil {
				hc = map[string]any{}
			}
			hc["enable"] = true
			hc["lazy"] = true
			if hc["interval"] == nil {
				hc["interval"] = 300
			}
			if hc["url"] == nil {
				hc["url"] = "https://www.gstatic.com/generate_204"
			}
			sub["type"] = "http"
			sub["path"] = "./providers/subscription.yaml"
			sub["interval"] = 0
			sub["health-check"] = hc
			providers[ProviderName] = sub
			root["proxy-providers"] = providers
		}
	}

	root["tun"] = tun
}

// applyRoutingRules installs a Clash/Mihomo-style split rule set that uses the
// bundled geoip.metadb + geosite.dat (no download). Same idea as Clash Verge /
// other Meta clients: LAN & CN direct, ads reject, everything else via PROXY.
func applyRoutingRules(root map[string]any) {
	root["rules"] = []any{
		"GEOSITE,private,DIRECT",
		"IP-CIDR,192.168.0.0/16,DIRECT,no-resolve",
		"IP-CIDR,10.0.0.0/8,DIRECT,no-resolve",
		"IP-CIDR,172.16.0.0/12,DIRECT,no-resolve",
		"IP-CIDR,127.0.0.0/8,DIRECT,no-resolve",
		"IP-CIDR,224.0.0.0/4,DIRECT,no-resolve",
		"IP-CIDR,255.255.255.255/32,DIRECT,no-resolve",
		"GEOSITE,category-ads-all,REJECT",
		"GEOSITE,cn,DIRECT",
		"GEOIP,CN,DIRECT",
		"MATCH,PROXY",
	}
}
