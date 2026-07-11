//go:build !unix

package config

func chownToSudoUser(path string) error {
	return nil
}

// ChownToSudoUser is a no-op on non-unix platforms.
func ChownToSudoUser(path string) error {
	return nil
}
