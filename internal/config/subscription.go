package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const ProviderName = "subscription"

func SubscriptionFile(dataDir string) string {
	return filepath.Join(dataDir, "subscription.url")
}

func LoadSubscriptionURL(dataDir string) (string, error) {
	path := SubscriptionFile(dataDir)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}

	var url string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		url = line
		break
	}
	if url == "" || strings.HasPrefix(url, "#") {
		return "", nil
	}
	return url, nil
}

func SaveSubscriptionURL(dataDir, url string) error {
	url = strings.TrimSpace(url)
	if url == "" {
		return fmt.Errorf("订阅地址不能为空")
	}

	path := SubscriptionFile(dataDir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(url+"\n"), 0o600)
}
