// SPDX-License-Identifier: MIT

package cmd

import (
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/otfabric/iec61850ctl/pkg/errs"
)

// complete runs the hidden __complete command and returns the candidates
// (descriptions stripped) and the directive line.
func complete(t *testing.T, args ...string) (values []string, directive string) {
	t.Helper()
	r := runCLI(t, append([]string{cobra.ShellCompRequestCmd}, args...)...)
	expectCode(t, r, errs.ExitOK)
	for line := range strings.SplitSeq(strings.TrimSpace(r.stdout), "\n") {
		if strings.HasPrefix(line, ":") {
			directive = line
			continue
		}
		name, _, _ := strings.Cut(line, "\t")
		values = append(values, name)
	}
	return values, directive
}

func TestCompletionShells(t *testing.T) {
	got, _ := complete(t, "completion", "")
	for _, sh := range []string{"bash", "zsh", "fish", "powershell"} {
		if !slices.Contains(got, sh) {
			t.Errorf("completion shells %v lack %s", got, sh)
		}
	}
	for _, sh := range []string{"bash", "zsh", "fish", "powershell"} {
		// Cobra binds the default completion command to the writer of the
		// invocation that created it; drop it so this run creates it anew.
		if c, _, err := rootCmd.Find([]string{"completion"}); err == nil && c != rootCmd {
			rootCmd.RemoveCommand(c)
		}
		r := runCLI(t, "completion", sh)
		expectCode(t, r, errs.ExitOK)
		if !strings.Contains(r.stdout, "iec61850ctl") {
			t.Errorf("completion %s: script does not name the tool: %.200q %q", sh, r.stdout, r.stderr)
		}
	}
}

func TestCompletionFlagValues(t *testing.T) {
	noFile := ":4" // ShellCompDirectiveNoFileComp
	cases := []struct {
		args      []string
		want      []string
		directive string
	}{
		{[]string{"--log-level", ""}, []string{"debug", "info", "warn", "error", "off"}, noFile},
		{[]string{"--log-format", ""}, []string{"text", "json"}, noFile},
		{[]string{"version", "--format", ""}, []string{"text", "json"}, noFile},
		{[]string{"list", "lds", "--format", ""}, []string{"text", "json", "csv", "table", "yaml"}, noFile},
		{[]string{"get", "object", "--fc", ""}, []string{"ST", "MX", "CO", "ALL"}, noFile},
		{[]string{"set", "object", "--type", ""}, []string{"bool", "int", "uint", "float", "enum", "string"}, noFile},
		{[]string{"control", "operate", "--mode", ""}, []string{"auto", "direct", "sbo", "sbow"}, noFile},
		{[]string{"control", "operate", "--or-cat", ""}, []string{"remote-control", "station-control", "process"}, noFile},
		{[]string{"control", "inspect", "--format", ""}, []string{"text", "json"}, noFile},
		{[]string{"subscribe", "report", "--type", ""}, []string{"BR", "RP"}, noFile},
		{[]string{"subscribe", "report", "--format", ""}, []string{"text", "jsonl"}, noFile},
		{[]string{"tree", "--include", ""}, []string{"data_sets", "report_control_blocks", "all"}, noFile},
		{[]string{"goose", "sniff", "--format", ""}, []string{"text", "jsonl"}, noFile},
		{[]string{"goose", "gocb", "get", "--format", ""}, []string{"text", "json"}, noFile},
		// --msv-id satisfies the one-required group: completing a command
		// without it makes cobra mark the whole group required for the
		// rest of this process.
		{[]string{"sv", "svcb", "set", "--msv-id", "x", "--smp-mod", ""}, []string{"SmpPerPeriod", "SmpPerSec", "SecPerSmp"}, noFile},
		{[]string{"sv", "sniff", "--layout", ""}, []string{"9-2LE"}, noFile},
		{[]string{"scl", "convert", "--input", ""}, []string{"scd", "cid", "icd"}, ":8"},
		{[]string{"scl", "convert", "--output", ""}, []string{"csv"}, ":8"},
		{[]string{"server", "start", "--scl", ""}, []string{"cid", "icd"}, ":8"},
		{[]string{"server", "generate-config", "--output", ""}, []string{"cfg"}, ":8"},
		{[]string{"find", "bulk", "--mapping", ""}, []string{"json"}, ":8"},
		{[]string{"server", "start", "--files-dir", ""}, nil, ":16"},
		{[]string{"--host", ""}, nil, noFile},
		{[]string{"list", "lns", "--ld", ""}, nil, noFile},
	}
	for _, tc := range cases {
		got, dir := complete(t, tc.args...)
		for _, w := range tc.want {
			if !slices.Contains(got, w) {
				t.Errorf("%v: completions %v lack %q", tc.args, got, w)
			}
		}
		if dir != tc.directive {
			t.Errorf("%v: directive %s, want %s", tc.args, dir, tc.directive)
		}
	}
}

func TestCompletionInterfaces(t *testing.T) {
	_, dir := complete(t, "goose", "sniff", "--iface", "")
	if dir != ":4" {
		t.Errorf("--iface directive %s, want :4", dir)
	}
	if got, _ := completeInterfaces(nil, nil, ""); len(got) == 0 {
		t.Error("no network interface completed (the loopback interface exists everywhere)")
	}
}

// valueSet matches a usage that lists a fixed value set: "a|b|c",
// "Output format: a, b", "one of …", "BR (…) or RP".
var valueSet = regexp.MustCompile(`\w\|\w|Output format:|one of |: \w+ \(.*\) or \w+`)

// TestCompletionEveryFlag fails when a flag that takes a value has no
// completion function, or when one whose usage lists a value set
// completes no values.
func TestCompletionEveryFlag(t *testing.T) {
	registerCompletions()
	walkCommands(rootCmd, func(c *cobra.Command) {
		c.LocalFlags().VisitAll(func(f *pflag.Flag) {
			if isBoolFlag(f) {
				return
			}
			fn, ok := c.GetFlagCompletionFunc(f.Name)
			if !ok {
				t.Errorf("%s --%s has no completion function", c.CommandPath(), f.Name)
				return
			}
			if valueSet.MatchString(f.Usage) {
				if vals, _ := fn(c, nil, ""); len(vals) == 0 {
					t.Errorf("%s --%s lists values (%q) but completes none", c.CommandPath(), f.Name, f.Usage)
				}
			}
		})
	})
}

func TestUsageValues(t *testing.T) {
	if got := usageValues("Output format: text, json, ,csv"); !slices.Equal(got, []string{"text", "json", "csv"}) {
		t.Errorf("usageValues = %v", got)
	}
	if got := usageValues("no list"); got != nil {
		t.Errorf("usageValues = %v", got)
	}
}

func TestRegisterPanicsOnUnknownFlag(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("register on an unknown flag did not panic")
		}
	}()
	register(versionCmd, "no-such-flag", cobra.NoFileCompletions)
}
