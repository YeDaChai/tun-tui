//go:build !darwin

package core

import "fmt"

func CleanupTunRoutes() error {
	return fmt.Errorf("自动清理路由目前仅支持 macOS；Linux/Windows 请重启网络，或手动删除指向 198.18.0.1 的残留路由")
}
