// SPDX-License-Identifier: MIT

// Package buildinfo formats the build metadata of iec61850ctl: the values
// injected at link time (main.version, main.tag, main.commit, main.buildDate)
// with fallbacks from runtime/debug.ReadBuildInfo for go install / go build.
package buildinfo

import (
	"encoding/json"
	"runtime"
	"runtime/debug"
	"strings"
)

// Unknown is printed for a field that has no value.
const Unknown = "unknown"

// Info is the resolved build metadata. Every field has a value; Tag is
// empty when the binary was not built from a tag.
type Info struct {
	Name     string `json:"name"`
	Version  string `json:"version"`
	Tag      string `json:"tag"`
	Commit   string `json:"commit"`
	Built    string `json:"built"`
	Go       string `json:"go"`
	Platform string `json:"platform"`
}

// readBuildInfo is replaced in tests.
var readBuildInfo = debug.ReadBuildInfo

// Resolve builds Info from the link-time values. A missing version comes
// from the module version (go install …@vX.Y.Z), else "dev"; a missing
// commit from vcs.revision (with "-dirty" when vcs.modified=true), else
// "unknown"; a missing build date is "unknown".
func Resolve(name, version, tag, commit, buildDate string) Info {
	version = strings.TrimSpace(version)
	commit = strings.TrimSpace(commit)
	buildDate = strings.TrimSpace(buildDate)
	if version == "" || commit == "" {
		if bi, ok := readBuildInfo(); ok && bi != nil {
			if version == "" {
				if v := bi.Main.Version; v != "" && v != "(devel)" {
					version = v
				}
			}
			if commit == "" {
				commit = vcsCommit(bi)
			}
		}
	}
	version = strings.TrimPrefix(version, "v")
	if version == "" {
		version = "dev"
	}
	if commit == "" {
		commit = Unknown
	}
	if buildDate == "" {
		buildDate = Unknown
	}
	return Info{
		Name:     name,
		Version:  version,
		Tag:      strings.TrimSpace(tag),
		Commit:   commit,
		Built:    buildDate,
		Go:       runtime.Version(),
		Platform: runtime.GOOS + "/" + runtime.GOARCH,
	}
}

func vcsCommit(bi *debug.BuildInfo) string {
	var rev string
	var dirty bool
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			dirty = s.Value == "true"
		}
	}
	if rev != "" && dirty {
		rev += "-dirty"
	}
	return rev
}

// Text returns the one-line text form, without a trailing newline:
//
//	iec61850ctl 0.11.0 (tag v0.11.0, commit <sha>, built <date>, go1.26.1, linux/amd64)
func (i Info) Text() string {
	var b strings.Builder
	b.WriteString(i.Name + " " + i.Version + " (")
	if i.Tag != "" {
		b.WriteString("tag " + i.Tag + ", ")
	}
	b.WriteString("commit " + i.Commit + ", built " + i.Built + ", " + i.Go + ", " + i.Platform + ")")
	return b.String()
}

// JSON returns the one-line JSON form, without a trailing newline, with
// the keys name, version, tag, commit, built, go, platform in that order.
func (i Info) JSON() string {
	b, _ := json.Marshal(i) // a struct of strings always marshals
	return string(b)
}
