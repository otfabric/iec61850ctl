// SPDX-License-Identifier: MIT

// Package cmd implements the Cobra-based CLI interface for iec61850ctl.
package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/otfabric/iec61850ctl/internal/buildinfo"
	"github.com/otfabric/iec61850ctl/pkg/errs"
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

var (
	versionJSONFlag   bool
	versionFormatFlag string
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Display version information",
	Long: `Display the version and build metadata of iec61850ctl on one line:

  iec61850ctl <version> (tag <tag>, commit <commit>, built <date>, <go>, <os>/<arch>)

The tag part is left out when the binary was not built from a tag. --json,
--format json or IEC61850CTL_FORMAT=json print the same fields as one JSON
object (name, version, tag, commit, built, go, platform). Other
IEC61850CTL_FORMAT values print the text line.`,
	Args: cobra.NoArgs,
	RunE: runVersion,
}

func runVersion(cmd *cobra.Command, _ []string) error {
	asJSON := versionJSONFlag
	switch f := strings.ToLower(strings.TrimSpace(versionFormatFlag)); f {
	case "json":
		asJSON = true
	case "", "text":
	default:
		// IEC61850CTL_FORMAT is shared by all commands; a value version has
		// no form for (table, csv, yaml, jsonl) prints the text line.
		if cmd.Flags().Changed("format") {
			return errs.Invalidf("invalid --format %q (supported: text, json)", versionFormatFlag)
		}
	}
	info := buildinfo.Resolve("iec61850ctl", buildMeta.version, buildMeta.tag, buildMeta.commit, buildMeta.buildDate)
	line := info.Text()
	if asJSON {
		line = info.JSON()
	}
	_, err := fmt.Fprintln(cmd.OutOrStdout(), line)
	return err
}

func init() {
	versionCmd.Flags().BoolVar(&versionJSONFlag, "json", false, "print the build metadata as one JSON object")
	versionCmd.Flags().StringVar(&versionFormatFlag, "format", "text", "Output format: text, json")
	rootCmd.AddCommand(versionCmd)
}
