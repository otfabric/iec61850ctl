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
	wantJSON := `{"name":"iec61850ctl","version":"dev","tag":"","commit":"unknown","built":"unknown","go":"` +
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
		`","built":"2026-11-05T15:41:49Z","go":"` + runtime.Version() + `","platform":"` +
		runtime.GOOS + "/" + runtime.GOARCH + `"}`
	if got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
}
