//go:build !darwin

package core

func CleanupTunRoutes() error {
	return nil
}
