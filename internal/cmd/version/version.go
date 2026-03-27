package version

import (
	"fmt"
	"runtime"
)

var (
	// Set via -ldflags at release time; safe defaults keep local/module builds installable.
	gitCommit = "unknown"
	version   = "dev"
	buildDate = "1970-01-01 00:00:00 +0000"
)

// GoVersion returns the version of the go runtime used to compile the binary
var goVersion = runtime.Version()

// OsArch returns the os and arch used to build the binary
var osArch = fmt.Sprintf("%s %s", runtime.GOOS, runtime.GOARCH)

// generateOutput return the output of the version command
func generateOutput() string {
	return fmt.Sprintf(`prev - %s

Git Commit: %s
Build date: %s
Go version: %s
OS / Arch : %s
`, version, gitCommit, buildDate, goVersion, osArch)
}

// Print the current version
func Print() {
	fmt.Println(generateOutput())
}
