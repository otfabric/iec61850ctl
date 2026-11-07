// SPDX-License-Identifier: MIT

package buildinfo

import (
	"runtime"
	"runtime/debug"
	"testing"
)

const sha = "1802d0f9afe8642d1e278120b76fcdfac967cb9e"

var goPlat = runtime.Version() + ", " + runtime.GOOS + "/" + runtime.GOARCH

func withBuildInfo(t *testing.T, bi *debug.BuildInfo, ok bool) {
	t.Helper()
	prev := readBuildInfo
	readBuildInfo = func() (*debug.BuildInfo, bool) { return bi, ok }
	t.Cleanup(func() { readBuildInfo = prev })
}

func TestTextAllFields(t *testing.T) {
	withBuildInfo(t, nil, false)
	got := Resolve("iec61850ctl", "v0.11.0", "v0.11.0", sha, "2026-11-05T15:41:49Z").Text()
	want := "iec61850ctl 0.11.0 (tag v0.11.0, commit " + sha + ", built 2026-11-05T15:41:49Z, " + goPlat + ")"
	if got != want {
		t.Fatalf("got  %q\nwant %q", got, want)
	}
}

func TestTextWithoutTag(t *testing.T) {
	withBuildInfo(t, nil, false)
	got := Resolve("iec61850ctl", "0.10.0-3-gabc", "", sha, "2026-11-05T15:41:49Z").Text()
	want := "iec61850ctl 0.10.0-3-gabc (commit " + sha + ", built 2026-11-05T15:41:49Z, " + goPlat + ")"
	if got != want {
		t.Fatalf("got  %q\nwant %q", got, want)
	}
}

func TestDevFallbackNothingInjected(t *testing.T) {
	withBuildInfo(t, nil, false)
	i := Resolve("iec61850ctl", "", "", "", "")
	want := "iec61850ctl dev (commit unknown, built unknown, " + goPlat + ")"
	if got := i.Text(); got != want {
		t.Fatalf("got  %q\nwant %q", got, want)
	}
	wantJSON := `{"name":"iec61850ctl","version":"dev","tag":"","commit":"unknown","built":"unknown","libraries":[],"go":"` +
		runtime.Version() + `","platform":"` + runtime.GOOS + "/" + runtime.GOARCH + `"}`
	if got := i.JSON(); got != wantJSON {
		t.Fatalf("got  %s\nwant %s", got, wantJSON)
	}
}

func TestFallbackFromBuildInfo(t *testing.T) {
	withBuildInfo(t, &debug.BuildInfo{
		Main: debug.Module{Version: "(devel)"},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: sha},
			{Key: "vcs.modified", Value: "true"},
		},
	}, true)
	i := Resolve("iec61850ctl", "", "", "", "")
	if i.Version != "dev" || i.Commit != sha+"-dirty" || i.Built != Unknown || i.Tag != "" {
		t.Fatalf("devel build: %+v", i)
	}

	withBuildInfo(t, &debug.BuildInfo{
		Main:     debug.Module{Version: "v0.11.0"},
		Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: sha}, {Key: "vcs.modified", Value: "false"}},
	}, true)
	i = Resolve("iec61850ctl", "", "", "", "")
	if i.Version != "0.11.0" || i.Commit != sha {
		t.Fatalf("go install build: %+v", i)
	}

	// Injected values win over the build info.
	i = Resolve("iec61850ctl", "1.2.3", "", "abc", "")
	if i.Version != "1.2.3" || i.Commit != "abc" {
		t.Fatalf("injected: %+v", i)
	}
}

func TestJSONKeyOrder(t *testing.T) {
	withBuildInfo(t, nil, false)
	got := Resolve("iec61850ctl", "0.11.0", "v0.11.0", sha, "2026-11-05T15:41:49Z").JSON()
	want := `{"name":"iec61850ctl","version":"0.11.0","tag":"v0.11.0","commit":"` + sha +
		`","built":"2026-11-05T15:41:49Z","libraries":[],"go":"` + runtime.Version() + `","platform":"` +
		runtime.GOOS + "/" + runtime.GOARCH + `"}`
	if got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
}

func TestNonReleaseBuildInfoVersionsAreDev(t *testing.T) {
	for _, v := range []string{
		"v0.10.1-0.20261107214020-0978e46bfb72",
		"v0.10.1-0.20261107214020-0978e46bfb72+dirty",
		"v0.0.0-20261107214020-0978e46bfb72",
		"v1.2.0-rc.1.0.20261107214020-0978e46bfb72",
		"v0.10.0+dirty",
		"(devel)",
		"",
	} {
		withBuildInfo(t, &debug.BuildInfo{Main: debug.Module{Version: v}}, true)
		if got := Resolve("iec61850ctl", "", "", "", "").Version; got != "dev" {
			t.Errorf("build info version %q: got %q, want dev", v, got)
		}
	}
	for v, want := range map[string]string{"v0.11.0": "0.11.0", "v1.2.0-rc.1": "1.2.0-rc.1"} {
		withBuildInfo(t, &debug.BuildInfo{Main: debug.Module{Version: v}}, true)
		if got := Resolve("iec61850ctl", "", "", "", "").Version; got != want {
			t.Errorf("build info version %q: got %q, want %q", v, got, want)
		}
	}
}

func TestLibrariesFormatting(t *testing.T) {
	base := Info{Name: "iec61850ctl", Version: "0.11.0", Tag: "v0.11.0", Commit: "abc", Built: "2026-11-05T15:41:49Z", Go: "go1.26.10", Platform: "linux/amd64"}
	cases := []struct {
		libs     []Library
		text     string
		jsonLibs string
	}{
		{nil, "built 2026-11-05T15:41:49Z, go1.26.10", `[]`},
		{[]Library{{"go-iec61850", "1.7.0"}}, "built 2026-11-05T15:41:49Z, go-iec61850 1.7.0, go1.26.10", `[{"name":"go-iec61850","version":"1.7.0"}]`},
		{
			[]Library{{"go-iec61850", "1.7.0"}, {"go-mms", "1.4.0"}},
			"built 2026-11-05T15:41:49Z, go-iec61850 1.7.0, go-mms 1.4.0, go1.26.10",
			`[{"name":"go-iec61850","version":"1.7.0"},{"name":"go-mms","version":"1.4.0"}]`,
		},
	}
	for _, tc := range cases {
		i := base
		i.Libraries = tc.libs
		wantText := "iec61850ctl 0.11.0 (tag v0.11.0, commit abc, " + tc.text + ", linux/amd64)"
		if got := i.Text(); got != wantText {
			t.Errorf("text:\n got %q\nwant %q", got, wantText)
		}
		wantJSON := `{"name":"iec61850ctl","version":"0.11.0","tag":"v0.11.0","commit":"abc","built":"2026-11-05T15:41:49Z","libraries":` +
			tc.jsonLibs + `,"go":"go1.26.10","platform":"linux/amd64"}`
		if got := i.JSON(); got != wantJSON {
			t.Errorf("json:\n got %s\nwant %s", got, wantJSON)
		}
	}
}

func TestLibrariesFromBuildInfo(t *testing.T) {
	withBuildInfo(t, &debug.BuildInfo{
		Main: debug.Module{Path: "github.com/otfabric/go-self"},
		Deps: []*debug.Module{
			{Path: "github.com/otfabric/go-mms", Version: "v1.4.0"},
			{Path: "github.com/spf13/cobra", Version: "v1.10.2"},
			nil,
			{Path: "github.com/otfabric/go-self", Version: "v9.9.9"},
			{Path: "github.com/otfabric/go-iec61850", Version: "v1.7.0", Replace: &debug.Module{Path: "../go-iec61850", Version: "v1.7.1-0.20261107214020-0978e46bfb72"}},
			{Path: "github.com/otfabric/docsync", Version: "v0.3.1"},
		},
	}, true)
	got := Resolve("iec61850ctl", "0.11.0", "", "abc", "").Libraries
	want := []Library{{"go-iec61850", "1.7.1-0.20261107214020-0978e46bfb72"}, {"go-mms", "1.4.0"}}
	if len(got) != len(want) {
		t.Fatalf("libraries %v, want %v", got, want)
	}
	for k := range want {
		if got[k] != want[k] {
			t.Fatalf("libraries %v, want %v", got, want)
		}
	}
}
