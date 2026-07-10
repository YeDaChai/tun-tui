//go:build unix

package privilege

import "os"

func CanUseTUN() bool {
	return os.Geteuid() == 0
}
