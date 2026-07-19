//go:build unix

package config

import (
	"os"
	"os/user"
	"strconv"
)

// ChownToSudoUser reassigns ownership after sudo writes so the real user can manage files.
func ChownToSudoUser(path string) error {
	sudoUser := os.Getenv("SUDO_USER")
	if sudoUser == "" || os.Geteuid() != 0 {
		return nil
	}
	u, err := user.Lookup(sudoUser)
	if err != nil {
		return nil
	}
	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return nil
	}
	gid, err := strconv.Atoi(u.Gid)
	if err != nil {
		return nil
	}
	return os.Chown(path, uid, gid)
}
