package version

import "fmt"

var (
	Version   = "dev"
	Commit    = "none"
	BuildDate = "unknown"
)

func Full() string {
	if Commit == "none" {
		return fmt.Sprintf("%s (%s)", Version, BuildDate)
	}
	return fmt.Sprintf("%s (%s, %s)", Version, Commit, BuildDate)
}
