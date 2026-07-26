// SPDX-License-Identifier: MIT

// Package cmd implements the Cobra-based CLI interface for iec61850ctl.
package cmd

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

// buildMeta holds link-time metadata set from main via SetBuildMeta.
var buildMeta struct {
	version, tag, commit, buildDate string
}

// SetBuildMeta is called from main before Execute; values come from
// ldflags -X main.version / main.tag / main.commit / main.buildDate.
func SetBuildMeta(version, tag, commit, buildDate string) {
	buildMeta.version = version
	buildMeta.tag = tag
	buildMeta.commit = commit
	buildMeta.buildDate = buildDate
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Display version information",
	Long:  `Display the current version of iec61850ctl.`,
	Run: func(cmd *cobra.Command, args []string) {
		out := cmd.OutOrStdout()
		ver := strings.TrimSpace(buildMeta.version)
		if ver == "" {
			ver = "dev"
		}
		_, _ = fmt.Fprintf(out, "iec61850ctl version: %s\n", ver)
		if s := strings.TrimSpace(buildMeta.tag); s != "" {
			_, _ = fmt.Fprintf(out, "tag:         %s\n", s)
		}
		if s := strings.TrimSpace(buildMeta.commit); s != "" {
			_, _ = fmt.Fprintf(out, "commit:      %s\n", s)
		}
		if s := strings.TrimSpace(buildMeta.buildDate); s != "" {
			_, _ = fmt.Fprintf(out, "build date:  %s\n", s)
		}
		_, _ = fmt.Fprintf(out, "go version:  %s\n", runtime.Version())
		_, _ = fmt.Fprintf(out, "platform:    %s/%s\n", runtime.GOOS, runtime.GOARCH)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
