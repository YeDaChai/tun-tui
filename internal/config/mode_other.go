//go:build !unix

package config

func chownToSudoUser(path string) error {
	return nil
}
