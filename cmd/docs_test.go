// SPDX-License-Identifier: MIT

package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// protocolDocs maps each raw-Ethernet command tree to its reference page.
var protocolDocs = map[string]string{"goose": "docs/GOOSE.md", "sv": "docs/SV.md"}

// TestDocsCoverCommands checks that docs/GOOSE.md and docs/SV.md mention
// every subcommand and every flag of their command tree, and that the README
// shows an example of each top-level subcommand and links the page.
func TestDocsCoverCommands(t *testing.T) {
	readme := readRepoFile(t, "README.md")
	for proto, page := range protocolDocs {
		doc := readRepoFile(t, page)
		root, _, err := rootCmd.Find([]string{proto})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(readme, page) {
			t.Errorf("README.md does not link %s", page)
		}
		for _, c := range root.Commands() {
			if ex := "iec61850ctl " + proto + " " + c.Name(); !strings.Contains(readme, ex) {
				t.Errorf("README.md has no %q example", ex)
			}
		}
		walkCommands(root, func(c *cobra.Command) {
			path := strings.TrimPrefix(c.CommandPath(), rootCmd.Name()+" ")
			if !strings.Contains(doc, path) {
				t.Errorf("%s does not mention %q", page, path)
			}
			c.LocalFlags().VisitAll(func(f *pflag.Flag) {
				if f.Name == "help" {
					return
				}
				if !strings.Contains(doc, "--"+f.Name) {
					t.Errorf("%s does not document %s --%s", page, path, f.Name)
				}
			})
		})
	}
}

// TestDocExamplesParse parses every iec61850ctl command line in the shell
// blocks of the GOOSE/SV pages and the README, and every cobra Example of the
// goose and sv trees: each must name a runnable command, use only defined
// flags, and set the required ones. A console session that shows exit code 2
// documents a usage error and may fail to parse.
func TestDocExamplesParse(t *testing.T) {
	var examples []docExample
	for _, page := range []string{"docs/GOOSE.md", "docs/SV.md", "README.md"} {
		examples = append(examples, shellExamples(t, page)...)
	}
	for proto := range protocolDocs {
		root, _, _ := rootCmd.Find([]string{proto})
		walkCommands(root, func(c *cobra.Command) {
			src := "Example of " + c.CommandPath()
			for _, line := range joinContinuations(strings.Split(c.Example, "\n")) {
				if args, ok := commandArgs(line); ok {
					examples = append(examples, docExample{src: src, line: line, args: args})
				}
			}
		})
	}
	if len(examples) < 40 {
		t.Fatalf("found only %d examples; is the parser broken?", len(examples))
	}
	for _, ex := range examples {
		if err := parseOnly(ex.args); err != nil && !ex.usageError {
			t.Errorf("%s: %q: %v", ex.src, ex.line, err)
		}
	}
}

type docExample struct {
	src        string
	line       string
	args       []string
	usageError bool // the session ends with exit code 2
}

func walkCommands(c *cobra.Command, fn func(*cobra.Command)) {
	fn(c)
	for _, sub := range c.Commands() {
		walkCommands(sub, fn)
	}
}

func readRepoFile(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot(), rel))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// shellExamples returns the iec61850ctl command lines of the sh, bash and
// console blocks of a Markdown page.
func shellExamples(t *testing.T, page string) []docExample {
	t.Helper()
	var out []docExample
	var block []string
	in := false
	sc := bufio.NewScanner(strings.NewReader(readRepoFile(t, page)))
	n := 0
	for sc.Scan() {
		n++
		line := sc.Text()
		fence := strings.TrimSpace(line)
		switch {
		case !in && (fence == "```sh" || fence == "```bash" || fence == "```console"):
			in, block = true, nil
		case in && fence == "```":
			in = false
			usage := len(block) >= 2 && block[len(block)-2] == "$ echo $?" && block[len(block)-1] == "2"
			for _, l := range joinContinuations(block) {
				if args, ok := commandArgs(l); ok {
					out = append(out, docExample{src: fmt.Sprintf("%s (block ending line %d)", page, n), line: l, args: args, usageError: usage})
				}
			}
		case in:
			block = append(block, line)
		}
	}
	return out
}

// joinContinuations joins lines ending in a backslash with the next one.
func joinContinuations(lines []string) []string {
	var out []string
	cur := ""
	for _, l := range lines {
		trimmed := strings.TrimRight(l, " ")
		if strings.HasSuffix(trimmed, "\\") {
			cur += strings.TrimSuffix(trimmed, "\\") + " "
			continue
		}
		out = append(out, cur+l)
		cur = ""
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}

// commandArgs splits an "iec61850ctl ..." line (optionally after a "$ "
// prompt) into its arguments, with POSIX quoting; other lines are not
// commands.
func commandArgs(line string) ([]string, bool) {
	s := strings.TrimSpace(line)
	s = strings.TrimPrefix(s, "$ ")
	if !strings.HasPrefix(s, "iec61850ctl ") {
		return nil, false
	}
	words, err := shellWords(strings.TrimPrefix(s, "iec61850ctl "))
	return words, err == nil
}

// shellWords splits s like a POSIX shell: single and double quotes, backslash
// escapes, and a # comment at the start of a word.
func shellWords(s string) ([]string, error) {
	var words []string
	var cur strings.Builder
	inWord := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == ' ' || c == '\t':
			if inWord {
				words, inWord = append(words, cur.String()), false
				cur.Reset()
			}
		case c == '#' && !inWord:
			return words, nil
		case c == '\'':
			j := strings.IndexByte(s[i+1:], '\'')
			if j < 0 {
				return nil, fmt.Errorf("unterminated quote")
			}
			cur.WriteString(s[i+1 : i+1+j])
			i, inWord = i+1+j, true
		case c == '"':
			i++
			for ; i < len(s) && s[i] != '"'; i++ {
				if s[i] == '\\' && i+1 < len(s) {
					i++
				}
				cur.WriteByte(s[i])
			}
			if i >= len(s) {
				return nil, fmt.Errorf("unterminated quote")
			}
			inWord = true
		case c == '\\' && i+1 < len(s):
			i++
			cur.WriteByte(s[i])
			inWord = true
		default:
			cur.WriteByte(c)
			inWord = true
		}
	}
	if inWord {
		words = append(words, cur.String())
	}
	return words, nil
}

// parseOnly resolves and parses args against the CLI without running it.
func parseOnly(args []string) error {
	cliMu.Lock()
	defer cliMu.Unlock()
	resetFlags(rootCmd)
	defer resetFlags(rootCmd)
	c, rest, err := rootCmd.Find(args)
	if err != nil {
		return err
	}
	if !c.Runnable() {
		return fmt.Errorf("%s is not runnable", c.CommandPath())
	}
	if err := c.ParseFlags(rest); err != nil {
		return err
	}
	if err := c.ValidateArgs(c.Flags().Args()); err != nil {
		return err
	}
	if err := c.ValidateRequiredFlags(); err != nil {
		return err
	}
	return c.ValidateFlagGroups()
}

func TestShellWords(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		`a 'b c' "d\"e" f\ g # comment`:       "a|b c|d\"e|f g",
		`--value 'LD0/GGIO1.Ind1.q[ST]=0000'`: "--value|LD0/GGIO1.Ind1.q[ST]=0000",
		`x\$y`:                                "x$y",
	} {
		got, err := shellWords(in)
		if err != nil || strings.Join(got, "|") != want {
			t.Errorf("shellWords(%q) = %q, %v", in, got, err)
		}
	}
	if _, err := shellWords(`'open`); err == nil {
		t.Error("unterminated quote accepted")
	}
}
