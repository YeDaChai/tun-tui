//go:build unix

package config

import (
	"os"
	"os/user"
	"strconv"
)

func chownToSudoUser(path string) error {
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
