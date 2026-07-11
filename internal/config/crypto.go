package config

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ProxyCrypto describes protocol / cipher for a named outbound.
type ProxyCrypto struct {
	Type   string
	Cipher string
}

// LoadProxyCryptoMap reads providers/subscription.yaml for type + cipher hints.
// API /proxies only exposes protocol type; cipher (SS/VMess) lives in the provider file.
func LoadProxyCryptoMap(dataDir string) map[string]ProxyCrypto {
	path := filepath.Join(dataDir, "providers", "subscription.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var root struct {
		Proxies []struct {
			Name   string `yaml:"name"`
			Type   string `yaml:"type"`
			Cipher string `yaml:"cipher"`
		} `yaml:"proxies"`
	}
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil
	}

	out := make(map[string]ProxyCrypto, len(root.Proxies))
	for _, p := range root.Proxies {
		name := strings.TrimSpace(p.Name)
		if name == "" {
			continue
		}
		out[name] = ProxyCrypto{
			Type:   strings.TrimSpace(p.Type),
			Cipher: strings.TrimSpace(p.Cipher),
		}
	}
	return out
}

// FormatProxyCrypto builds a short label like "Shadowsocks · aes-256-gcm".
func FormatProxyCrypto(apiType, fileType, cipher string) string {
	typ := strings.TrimSpace(apiType)
	if typ == "" {
		typ = strings.TrimSpace(fileType)
	}
	typ = friendlyProxyType(typ)
	cipher = strings.TrimSpace(cipher)
	switch {
	case typ != "" && cipher != "":
		return typ + " · " + cipher
	case cipher != "":
		return cipher
	default:
		return typ
	}
}

func friendlyProxyType(t string) string {
	switch strings.ToLower(t) {
	case "ss", "shadowsocks":
		return "Shadowsocks"
	case "ssr", "shadowsocksr":
		return "ShadowsocksR"
	case "vmess":
		return "VMess"
	case "vless":
		return "VLESS"
	case "trojan":
		return "Trojan"
	case "hysteria":
		return "Hysteria"
	case "hysteria2", "hy2":
		return "Hysteria2"
	case "tuic":
		return "TUIC"
	case "wireguard":
		return "WireGuard"
	case "direct":
		return "DIRECT"
	case "reject":
		return "REJECT"
	default:
		if t == "" {
			return ""
		}
		return t
	}
}
