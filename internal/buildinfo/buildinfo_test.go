package buildinfo

import (
	"runtime/debug"
	"testing"
)

func TestResolveVersionPrefersInjectedVersion(t *testing.T) {
	readBuildInfo := func() (*debug.BuildInfo, bool) {
		t.Fatal("build information must not be read when a version is injected")
		return nil, false
	}

	for _, injected := range []string{"0.1.1", "v0.1.1"} {
		if got := resolveVersion(injected, readBuildInfo); got != "0.1.1" {
			t.Fatalf("resolveVersion(%q) = %q, want %q", injected, got, "0.1.1")
		}
	}
}

func TestResolveVersionFallsBackToModuleVersion(t *testing.T) {
	readBuildInfo := func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{
			Main: debug.Module{Version: "v0.1.1"},
		}, true
	}

	if got := resolveVersion(developmentVersion, readBuildInfo); got != "0.1.1" {
		t.Fatalf("resolveVersion() = %q, want %q", got, "0.1.1")
	}
}

func TestResolveVersionKeepsDevelopmentFallback(t *testing.T) {
	tests := []struct {
		name          string
		buildInfo     *debug.BuildInfo
		buildInfoRead bool
	}{
		{name: "unavailable"},
		{name: "nil", buildInfoRead: true},
		{name: "empty", buildInfo: &debug.BuildInfo{}, buildInfoRead: true},
		{
			name: "development",
			buildInfo: &debug.BuildInfo{
				Main: debug.Module{Version: "(devel)"},
			},
			buildInfoRead: true,
		},
		{
			name: "local vcs build",
			buildInfo: &debug.BuildInfo{
				Main: debug.Module{Version: "v0.1.0+dirty"},
				Settings: []debug.BuildSetting{
					{Key: "vcs", Value: "git"},
				},
			},
			buildInfoRead: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			readBuildInfo := func() (*debug.BuildInfo, bool) {
				return test.buildInfo, test.buildInfoRead
			}
			if got := resolveVersion(developmentVersion, readBuildInfo); got != developmentVersion {
				t.Fatalf("resolveVersion() = %q, want %q", got, developmentVersion)
			}
		})
	}
}
