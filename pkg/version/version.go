// Package version provides build-time version information.
// These variables are set via ldflags at compile time.
//
// Example:
//
//	go build -ldflags "-X github.com/dev-dhg/yaocc/pkg/version.Version=v1.0.0 \
//	  -X github.com/dev-dhg/yaocc/pkg/version.Commit=abc1234 \
//	  -X github.com/dev-dhg/yaocc/pkg/version.BuildDate=2025-01-01T00:00:00Z"
package version

import "fmt"

// Version is the semantic version tag (e.g. "v1.2.3").
// Set to "dev" for local/untagged builds.
var Version = "dev"

// Commit is the short Git commit SHA of the build.
var Commit = "unknown"

// BuildDate is the ISO 8601 timestamp of when the binary was built.
var BuildDate = "unknown"

// String returns a formatted version string.
func String() string {
	return fmt.Sprintf("%s (commit: %s, built: %s)", Version, Commit, BuildDate)
}
