package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const ProviderName = "subscription"

func subscriptionFile(dataDir string) string {
	return filepath.Join(dataDir, "subscription.url")
}

func subscriptionLinksFile(dataDir string) string {
	return filepath.Join(dataDir, "subscription.links")
}

func LoadSubscriptionURL(dataDir string) (string, error) {
	urls, active, err := LoadSubscriptionLinks(dataDir)
	if err != nil {
		return "", err
	}
	if active < 0 || active >= len(urls) {
		return "", nil
	}
	return urls[active], nil
}

func LoadSubscriptionLinks(dataDir string) ([]string, int, error) {
	linksPath := subscriptionLinksFile(dataDir)
	data, err := os.ReadFile(linksPath)
	if err != nil {
		if os.IsNotExist(err) {
			if url, legacyErr := loadLegacySubscriptionURL(dataDir); legacyErr == nil && url != "" {
				if saveErr := SaveSubscriptionLinks(dataDir, []string{url}, 0); saveErr != nil {
					return nil, -1, saveErr
				}
				return []string{url}, 0, nil
			}
			return nil, -1, nil
		}
		return nil, -1, err
	}
	return parseSubscriptionLinks(data)
}

func SaveSubscriptionLinks(dataDir string, urls []string, active int) error {
	clean := make([]string, 0, len(urls))
	for _, u := range urls {
		u = strings.TrimSpace(u)
		if u == "" {
			continue
		}
		if err := ValidateSubscriptionURL(u); err != nil {
			return err
		}
		clean = append(clean, u)
	}
	if len(clean) == 0 {
		active = -1
	} else {
		if active < 0 || active >= len(clean) {
			active = 0
		}
	}

	path := subscriptionLinksFile(dataDir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if len(clean) == 0 {
		_ = os.Remove(path)
		_ = os.Remove(subscriptionFile(dataDir))
		return nil
	}

	if err := os.WriteFile(path, []byte(formatSubscriptionLinks(clean, active)), 0o600); err != nil {
		return err
	}
	_ = ChownToSudoUser(path)

	// Keep legacy single-url file in sync for external tooling.
	legacyPath := subscriptionFile(dataDir)
	if err := os.WriteFile(legacyPath, []byte(clean[active]+"\n"), 0o600); err != nil {
		return err
	}
	_ = ChownToSudoUser(legacyPath)
	return nil
}

func AddSubscriptionLink(dataDir, url string) error {
	url = strings.TrimSpace(url)
	if err := ValidateSubscriptionURL(url); err != nil {
		return err
	}

	urls, _, err := LoadSubscriptionLinks(dataDir)
	if err != nil {
		return err
	}
	for _, u := range urls {
		if u == url {
			return fmt.Errorf("订阅地址已存在")
		}
	}

	urls = append(urls, url)
	return SaveSubscriptionLinks(dataDir, urls, len(urls)-1)
}

func SetActiveSubscriptionLink(dataDir string, index int) error {
	urls, _, err := LoadSubscriptionLinks(dataDir)
	if err != nil {
		return err
	}
	if index < 0 || index >= len(urls) {
		return fmt.Errorf("无效的订阅索引")
	}
	return SaveSubscriptionLinks(dataDir, urls, index)
}

func DeleteSubscriptionLink(dataDir string, index int) error {
	urls, active, err := LoadSubscriptionLinks(dataDir)
	if err != nil {
		return err
	}
	if index < 0 || index >= len(urls) {
		return fmt.Errorf("无效的订阅索引")
	}

	urls = append(urls[:index], urls[index+1:]...)
	if len(urls) == 0 {
		return SaveSubscriptionLinks(dataDir, nil, -1)
	}

	newActive := active
	switch {
	case index < active:
		newActive = active - 1
	case index == active:
		if newActive >= len(urls) {
			newActive = len(urls) - 1
		}
	}
	return SaveSubscriptionLinks(dataDir, urls, newActive)
}

func loadLegacySubscriptionURL(dataDir string) (string, error) {
	path := subscriptionFile(dataDir)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "*") {
			line = strings.TrimSpace(line[1:])
		}
		if line != "" {
			return line, nil
		}
	}
	return "", nil
}

func parseSubscriptionLinks(data []byte) ([]string, int, error) {
	var urls []string
	active := 0
	foundActive := false

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "*") {
			line = strings.TrimSpace(line[1:])
			active = len(urls)
			foundActive = true
		}
		if line == "" {
			continue
		}
		urls = append(urls, line)
	}

	if len(urls) == 0 {
		return nil, -1, nil
	}
	if !foundActive {
		active = 0
	}
	if active >= len(urls) {
		active = 0
	}
	return urls, active, nil
}

func formatSubscriptionLinks(urls []string, active int) string {
	var b strings.Builder
	for i, u := range urls {
		if i == active {
			b.WriteByte('*')
		}
		b.WriteString(u)
		b.WriteByte('\n')
	}
	return b.String()
}
