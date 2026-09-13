package cli

import (
	"encoding/json"
	"runtime"
	"runtime/debug"
	"strings"
	"testing"
)

func TestBuildVersionInfo(t *testing.T) {
	testCases := []struct {
		name        string
		ldflags     string
		info        *debug.BuildInfo
		ok          bool
		wantVersion string
		wantCommit  string
		wantBuilt   string
	}{
		{
			name:        "ldflags wins over module version",
			ldflags:     "v0.1.0",
			info:        &debug.BuildInfo{Main: debug.Module{Version: "v9.9.9"}},
			ok:          true,
			wantVersion: "v0.1.0",
		},
		{
			name:        "module version fills in for go install",
			ldflags:     "dev",
			info:        &debug.BuildInfo{Main: debug.Module{Version: "v1.2.3"}},
			ok:          true,
			wantVersion: "v1.2.3",
		},
		{
			name:        "local build keeps dev",
			ldflags:     "dev",
			info:        &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}},
			ok:          true,
			wantVersion: "dev",
		},
		{
			name:    "commit is shortened and marked dirty",
			ldflags: "dev",
			info: &debug.BuildInfo{
				Main: debug.Module{Version: "(devel)"},
				Settings: []debug.BuildSetting{
					{Key: "vcs.revision", Value: "fff5392abcdef0123456789"},
					{Key: "vcs.time", Value: "2026-09-12T11:35:00Z"},
					{Key: "vcs.modified", Value: "true"},
				},
			},
			ok:          true,
			wantVersion: "dev",
			wantCommit:  "fff5392-dirty",
			wantBuilt:   "2026-09-12T11:35:00Z",
		},
		{
			name:    "clean commit has no suffix",
			ldflags: "dev",
			info: &debug.BuildInfo{
				Main: debug.Module{Version: "(devel)"},
				Settings: []debug.BuildSetting{
					{Key: "vcs.revision", Value: "fff5392abcdef0123456789"},
					{Key: "vcs.modified", Value: "false"},
				},
			},
			ok:          true,
			wantVersion: "dev",
			wantCommit:  "fff5392",
		},
		{
			name:        "missing build info still reports runtime details",
			ldflags:     "dev",
			info:        nil,
			ok:          false,
			wantVersion: "dev",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			got := buildVersionInfo(testCase.ldflags, testCase.info, testCase.ok)
			if got.Version != testCase.wantVersion {
				t.Errorf("version = %q, want %q", got.Version, testCase.wantVersion)
			}
			if got.Commit != testCase.wantCommit {
				t.Errorf("commit = %q, want %q", got.Commit, testCase.wantCommit)
			}
			if got.Built != testCase.wantBuilt {
				t.Errorf("built = %q, want %q", got.Built, testCase.wantBuilt)
			}
			if got.Go != runtime.Version() {
				t.Errorf("go = %q, want %q", got.Go, runtime.Version())
			}
			if want := runtime.GOOS + "/" + runtime.GOARCH; got.Platform != want {
				t.Errorf("platform = %q, want %q", got.Platform, want)
			}
		})
	}
}

func TestVersionCommand(t *testing.T) {
	output := executeCLI(t, &testConfigRepo{}, "version")
	if !strings.HasPrefix(output, "lazytodo ") {
		t.Fatalf("expected a single version line, got %q", output)
	}
	if strings.Contains(output, "Platform:") {
		t.Errorf("plain output should stay on one line, got %q", output)
	}
}

func TestVersionCommandVerbose(t *testing.T) {
	output := executeCLI(t, &testConfigRepo{}, "version", "--verbose")
	for _, want := range []string{"lazytodo ", "Go:", "Platform:"} {
		if !strings.Contains(output, want) {
			t.Errorf("expected %q in output, got %q", want, output)
		}
	}
}

func TestVersionCommandJSON(t *testing.T) {
	output := executeCLI(t, &testConfigRepo{}, "version", "--json")
	var payload map[string]any
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		t.Fatalf("unmarshal %q: %v", output, err)
	}
	for _, key := range []string{"version", "go", "platform"} {
		if _, found := payload[key]; !found {
			t.Errorf("expected key %q in %v", key, payload)
		}
	}
}
