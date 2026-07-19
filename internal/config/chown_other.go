//go:build !unix

package config

// ChownToSudoUser is a no-op on non-unix platforms.
func ChownToSudoUser(path string) error {
	return nil
}
