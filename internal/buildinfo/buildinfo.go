// Package buildinfo resolves version metadata from the linker or the Go module.
package buildinfo

import (
	"runtime/debug"
	"strings"
)

const developmentVersion = "dev"

var (
	Version = developmentVersion
	Commit  = "unknown"
	Date    = "unknown"
)

// ResolvedVersion returns the linker-injected release version when available.
// Binaries installed with "go install module@version" fall back to the module
// version embedded by the Go toolchain. Local development builds remain "dev".
func ResolvedVersion() string {
	return resolveVersion(Version, debug.ReadBuildInfo)
}

func resolveVersion(injected string, readBuildInfo func() (*debug.BuildInfo, bool)) string {
	injected = strings.TrimSpace(injected)
	if injected != "" && injected != developmentVersion {
		return normalizeVersion(injected)
	}

	info, ok := readBuildInfo()
	if !ok || info == nil {
		return developmentVersion
	}
	if isLocalVCSBuild(info.Settings) {
		return developmentVersion
	}

	moduleVersion := strings.TrimSpace(info.Main.Version)
	if moduleVersion == "" || moduleVersion == "(devel)" {
		return developmentVersion
	}

	return normalizeVersion(moduleVersion)
}

func normalizeVersion(version string) string {
	return strings.TrimPrefix(version, "v")
}

func isLocalVCSBuild(settings []debug.BuildSetting) bool {
	for _, setting := range settings {
		if setting.Key == "vcs" {
			return true
		}
	}
	return false
}
