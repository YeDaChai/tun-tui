package config

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed default_config.yaml
var defaultConfig []byte

const subscriptionExample = `# 把机场订阅链接粘贴到 subscription.url 文件中
# 或在 TUI 中按 l 管理
https://your-subscription-url-here
`

func Bootstrap(dataDir string) error {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(dataDir, "providers"), 0o755); err != nil {
		return fmt.Errorf("create providers dir: %w", err)
	}

	cfgPath := filepath.Join(dataDir, "config.yaml")
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		if err := os.WriteFile(cfgPath, defaultConfig, 0o644); err != nil {
			return fmt.Errorf("write default config: %w", err)
		}
	}

	examplePath := filepath.Join(dataDir, "subscription.url.example")
	if _, err := os.Stat(examplePath); os.IsNotExist(err) {
		if err := os.WriteFile(examplePath, []byte(subscriptionExample), 0o644); err != nil {
			return fmt.Errorf("write subscription example: %w", err)
		}
	}

	return nil
}
