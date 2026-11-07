// SPDX-License-Identifier: MIT

// Package buildinfo formats the build metadata of iec61850ctl: the values
// injected at link time (main.version, main.tag, main.commit, main.buildDate)
// with fallbacks from runtime/debug.ReadBuildInfo for go install / go build.
package buildinfo

import (
	"encoding/json"
	"regexp"
	"runtime"
	"runtime/debug"
	"slices"
	"strings"
)

// Unknown is printed for a field that has no value.
const Unknown = "unknown"

// Info is the resolved build metadata. Every field has a value; Tag is
// empty when the binary was not built from a tag.
type Info struct {
	Name      string    `json:"name"`
	Version   string    `json:"version"`
	Tag       string    `json:"tag"`
	Commit    string    `json:"commit"`
	Built     string    `json:"built"`
	Libraries []Library `json:"libraries"`
	Go        string    `json:"go"`
	Platform  string    `json:"platform"`
}

// Library is an OT Fabric library (github.com/otfabric/go-*) the binary is
// built on.
type Library struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// libraryPrefix selects the OT Fabric libraries among the build's modules.
const libraryPrefix = "github.com/otfabric/go-"

// pseudoVersion matches a Go pseudo-version (vX.Y.Z-yyyymmddhhmmss-abcdef123456,
// vX.Y.Z-pre.0.yyyymmddhhmmss-…, vX.Y.(Z+1)-0.yyyymmddhhmmss-…).
var pseudoVersion = regexp.MustCompile(`[-.]\d{14}-[0-9a-f]{12}$`)

// realVersion reports whether a build-info module version names a release:
// not (devel), not a pseudo-version and not +dirty.
func realVersion(v string) bool {
	if v == "" || v == "(devel)" || strings.Contains(v, "+dirty") {
		return false
	}
	base, _, _ := strings.Cut(v, "+")
	return !pseudoVersion.MatchString(base)
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
	var libs []Library
	if bi, ok := readBuildInfo(); ok && bi != nil {
		if version == "" && realVersion(bi.Main.Version) {
			version = bi.Main.Version
		}
		if commit == "" {
			commit = vcsCommit(bi)
		}
		libs = libraries(bi)
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
		Name:      name,
		Version:   version,
		Tag:       strings.TrimSpace(tag),
		Commit:    commit,
		Built:     buildDate,
		Libraries: libs,
		Go:        runtime.Version(),
		Platform:  runtime.GOOS + "/" + runtime.GOARCH,
	}
}

// libraries returns the github.com/otfabric/go-* modules of the build other
// than the main module, replacements applied, sorted by name.
func libraries(bi *debug.BuildInfo) []Library {
	out := []Library{}
	for _, d := range bi.Deps {
		if d == nil {
			continue
		}
		m := d
		if d.Replace != nil {
			m = d.Replace
		}
		if !strings.HasPrefix(d.Path, libraryPrefix) || d.Path == bi.Main.Path {
			continue
		}
		out = append(out, Library{Name: d.Path[strings.LastIndex(d.Path, "/")+1:], Version: strings.TrimPrefix(m.Version, "v")})
	}
	slices.SortFunc(out, func(a, b Library) int { return strings.Compare(a.Name, b.Name) })
	return out
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
//	iec61850ctl 0.11.0 (tag v0.11.0, commit <sha>, built <date>, go-iec61850 1.7.0, …, go1.26.1, linux/amd64)
func (i Info) Text() string {
	var b strings.Builder
	b.WriteString(i.Name + " " + i.Version + " (")
	if i.Tag != "" {
		b.WriteString("tag " + i.Tag + ", ")
	}
	b.WriteString("commit " + i.Commit + ", built " + i.Built + ", ")
	for _, l := range i.Libraries {
		b.WriteString(l.Name + " " + l.Version + ", ")
	}
	b.WriteString(i.Go + ", " + i.Platform + ")")
	return b.String()
}

// JSON returns the one-line JSON form, without a trailing newline, with
// the keys name, version, tag, commit, built, libraries, go, platform in
// that order; libraries is [] when there are none.
func (i Info) JSON() string {
	if i.Libraries == nil {
		i.Libraries = []Library{}
	}
	b, _ := json.Marshal(i) // a struct of strings always marshals
	return string(b)
}
