// SPDX-License-Identifier: MIT

package cmd

import (
	"net"
	"strings"
	"sync"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/otfabric/iec61850ctl/pkg/domain"
)

// Shell completion for flag values. Cobra's default completion command
// generates the bash, zsh, fish and powershell scripts; this file registers
// what each flag value completes to:
//
//   - flags with a fixed value set complete those values (--format from its
//     "Output format: a, b" usage, the rest from the table below);
//   - flags naming a file of a known type complete files with that extension,
//     --files-dir completes directories, --iface the network interfaces;
//   - every other flag that takes a value completes nothing (no file names).

// sclExtensions are the SCL file types (IEC 61850-6) the SCL flags accept.
var sclExtensions = []string{"scd", "cid", "icd", "iid", "ssd", "sed", "scl", "xml"}

var completionOnce sync.Once

// registerCompletions registers the flag completion functions once, after
// every command's init has defined its flags.
func registerCompletions() {
	completionOnce.Do(func() {
		registerCompletionsOn(rootCmd)
	})
}

func fcValues(withAll bool) []string {
	var out []string
	for _, fc := range domain.AllFCs() {
		out = append(out, string(fc))
	}
	if withAll {
		out = append(out, string(domain.FC_ALL))
	}
	return out
}

func registerCompletionsOn(root *cobra.Command) {
	valueTypes := []string{"bool", "int", "uint", "float", "enum", "string"}
	enums := []struct {
		cmd    *cobra.Command
		flag   string
		values []string
	}{
		{root, "log-level", []string{"debug", "info", "warn", "error", "off"}},
		{root, "log-format", []string{"text", "json"}},
		{controlOperateCmd, "type", valueTypes},
		{controlOperateCmd, "mode", []string{"auto", "direct", "sbo", "sbow"}},
		{controlOperateCmd, "or-cat", []string{"not-supported", "bay-control", "station-control", "remote-control",
			"automatic-bay", "automatic-station", "automatic-remote", "maintenance", "process"}},
		{getObjectCmd, "fc", fcValues(true)},
		{setObjectCmd, "fc", fcValues(false)},
		{setObjectCmd, "type", valueTypes},
		{subscribeReportCmd, "type", []string{"BR", "RP"}},
		{treeCmd, "include", []string{"data_sets", "report_control_blocks", "all"}},
		{svSniffCmd, "layout", []string{"9-2LE"}},
		{svSvcbSetCmd, "opt-flds", []string{"refresh-time", "sample-synchronized", "sample-rate", "data-set", "security", "none"}},
		{svSvcbSetCmd, "smp-mod", []string{"SmpPerPeriod", "SmpPerSec", "SecPerSmp"}},
	}
	for _, e := range enums {
		register(e.cmd, e.flag, cobra.FixedCompletions(e.values, cobra.ShellCompDirectiveNoFileComp))
	}

	files := []struct {
		cmd  *cobra.Command
		flag string
		exts []string
	}{
		{findBulkCmd, "mapping", []string{"json"}},
		{findBulkCmd, "output", []string{"json"}},
		{sclParseCmd, "input", sclExtensions},
		{sclConvertCmd, "input", sclExtensions},
		{sclConvertCmd, "output", []string{"csv"}},
		{gooseListCmd, "scl", sclExtensions},
		{gooseSubscribeCmd, "scl", sclExtensions},
		{goosePublishCmd, "scl", sclExtensions},
		{svListCmd, "scl", sclExtensions},
		{svSniffCmd, "scl", sclExtensions},
		{svSubscribeCmd, "scl", sclExtensions},
		{svPublishCmd, "scl", sclExtensions},
		{serverStartCmd, "scl", sclExtensions},
		{serverStartCmd, "values", []string{"json"}},
		{serverGenerateConfigCmd, "input", []string{"json"}},
		{serverGenerateConfigCmd, "output", []string{"cfg"}},
		{treeCmd, "output", []string{"json"}},
	}
	for _, f := range files {
		register(f.cmd, f.flag, fileExtCompletion(f.exts))
	}
	register(serverStartCmd, "files-dir", func(*cobra.Command, []string, string) ([]cobra.Completion, cobra.ShellCompDirective) {
		return nil, cobra.ShellCompDirectiveFilterDirs
	})
	// get file --output saves any file.
	register(getFileCmd, "output", func(*cobra.Command, []string, string) ([]cobra.Completion, cobra.ShellCompDirective) {
		return nil, cobra.ShellCompDirectiveDefault
	})
	for _, c := range []*cobra.Command{gooseSniffCmd, gooseSubscribeCmd, goosePublishCmd, svSniffCmd, svSubscribeCmd, svPublishCmd} {
		register(c, "iface", completeInterfaces)
	}

	walkCommands(root, func(c *cobra.Command) {
		c.LocalFlags().VisitAll(func(f *pflag.Flag) {
			if _, ok := c.GetFlagCompletionFunc(f.Name); ok || isBoolFlag(f) {
				return
			}
			if f.Name == "format" {
				if vals := usageValues(f.Usage); len(vals) > 0 {
					register(c, f.Name, cobra.FixedCompletions(vals, cobra.ShellCompDirectiveNoFileComp))
					return
				}
			}
			register(c, f.Name, cobra.NoFileCompletions)
		})
	})
}

// register panics on a programming error (unknown flag, duplicate), which
// TestCompletionEveryFlag catches.
func register(c *cobra.Command, flag string, fn cobra.CompletionFunc) {
	if err := c.RegisterFlagCompletionFunc(flag, fn); err != nil {
		panic("completion for --" + flag + " on " + c.CommandPath() + ": " + err.Error())
	}
}

func fileExtCompletion(exts []string) cobra.CompletionFunc {
	return func(*cobra.Command, []string, string) ([]cobra.Completion, cobra.ShellCompDirective) {
		return exts, cobra.ShellCompDirectiveFilterFileExt
	}
}

// completeInterfaces lists the host's network interfaces.
func completeInterfaces(*cobra.Command, []string, string) ([]cobra.Completion, cobra.ShellCompDirective) {
	ifs, _ := net.Interfaces()
	var out []cobra.Completion
	for _, i := range ifs {
		out = append(out, i.Name)
	}
	return out, cobra.ShellCompDirectiveNoFileComp
}

// usageValues returns the values of an "Output format: a, b, c" usage.
func usageValues(usage string) []string {
	_, list, ok := strings.Cut(usage, "Output format:")
	if !ok {
		return nil
	}
	var out []string
	for v := range strings.SplitSeq(list, ",") {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func isBoolFlag(f *pflag.Flag) bool {
	return f.Value.Type() == "bool"
}

func walkCommands(c *cobra.Command, fn func(*cobra.Command)) {
	fn(c)
	for _, sub := range c.Commands() {
		walkCommands(sub, fn)
	}
}
