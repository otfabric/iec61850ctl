// SPDX-License-Identifier: MIT

// Package cmd implements the Cobra-based CLI interface for iec61850ctl.
// It defines all commands (root, list, get, tree) and their flags, delegating
// business logic to the services package.
package cmd

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/otfabric/iec61850ctl/internal/app"
	"github.com/otfabric/iec61850ctl/pkg/formatter"
	"github.com/otfabric/iec61850ctl/pkg/view"

	"github.com/spf13/cobra"
)

var (
	ldNameForDas    string
	lnNameForDas    string
	doNameForDas    string
	dasDetailedFlag bool
	dasFormatFlag   string
)

var listDasCmd = &cobra.Command{
	Use:   "das",
	Short: "List all data attributes within a specific data object",
	Long: `Retrieves and displays the list of all data attributes (DAs) within the specified data object.
Requires --ld, --ln, and --do flags to specify the logical device, logical node, and data object.`,
	RunE: runListDas,
}

func init() {
	listDasCmd.Flags().StringVar(&ldNameForDas, "ld", "", "logical device name (required)")
	listDasCmd.Flags().StringVar(&lnNameForDas, "ln", "", "logical node name (required)")
	listDasCmd.Flags().StringVar(&doNameForDas, "do", "", "data object name (required)")
	listDasCmd.Flags().BoolVar(&dasDetailedFlag, "detailed", false, "show timestamp and quality metadata")
	listDasCmd.Flags().StringVar(&dasFormatFlag, "format", "text", "Output format: text, json, csv, table, yaml")
	_ = listDasCmd.MarkFlagRequired("ld")
	_ = listDasCmd.MarkFlagRequired("ln")
	_ = listDasCmd.MarkFlagRequired("do")

	listCmd.AddCommand(listDasCmd)
}

// runListDas executes the 'list das' command to display data attributes within a data object.
// It requires --ld, --ln, and --do flags and uses the explorer service to retrieve leaf DAs
// with their functional constraints, types, and values. Returns an error if connection or listing fails.
func runListDas(cmd *cobra.Command, args []string) error {
	session, err := openClientSession(cmd, clientSessionOptions{})
	if err != nil {
		return err
	}
	defer session.Close()
	conn := session.Conn()

	doRef := fmt.Sprintf("%s/%s.%s", ldNameForDas, lnNameForDas, doNameForDas)

	a := app.New(conn)

	attributesMap, err := a.ListDataAttributes(app.ListDataAttributesInput{
		LD:       ldNameForDas,
		LN:       lnNameForDas,
		DO:       doNameForDas,
		Detailed: dasDetailedFlag,
	})
	if err != nil {
		return err
	}

	if len(attributesMap) == 0 {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "No data attributes found in '%s'\n", doRef)
		return nil
	}

	// Structured output: flatten map to array and render via Renderer
	outputFormat, ok := formatter.ParseOutputFormat(dasFormatFlag)
	if ok && outputFormat != formatter.OutputFormatText {
		var flatAttrs []view.DataAttribute
		for _, parentName := range sortedKeys(attributesMap) {
			for _, attr := range attributesMap[parentName] {
				displayName := daDisplayName(parentName, attr.Name)
				flatAttrs = append(flatAttrs, view.DataAttribute{
					Name:    displayName,
					FC:      attr.FC,
					Type:    attr.Type,
					Value:   attr.Value,
					Time:    attr.Time,
					Quality: attr.Quality,
				})
			}
		}
		if len(flatAttrs) == 0 {
			if outputFormat == formatter.OutputFormatJSON {
				_, _ = io.WriteString(cmd.OutOrStdout(), "[]\n")
			}
			return nil
		}
		renderer := formatter.NewRenderer(outputFormat)
		return renderer.RenderDataAttributes(flatAttrs, cmd.OutOrStdout())
	}

	// Count total leaf attributes
	totalCount := 0
	for _, attributes := range attributesMap {
		totalCount += len(attributes)
	}

	if dasDetailedFlag {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Found %d leaf data attribute(s) in '%s':\n", totalCount, doRef)
	} else {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Found %d leaf data attribute(s) in '%s':\n", totalCount, doRef)
		_, _ = fmt.Fprintln(cmd.OutOrStdout())
	}

	index := 1
	for _, parentName := range sortedKeys(attributesMap) {
		attributes := attributesMap[parentName]
		if len(attributes) > 0 {
			if !dasDetailedFlag {
				// Simple mode: just list attribute names with numbering
				for _, attr := range attributes {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %d. %s\n", index, daDisplayName(parentName, attr.Name))
					index++
				}
			} else {
				// Detailed mode: show full metadata
				fcStr := "?"
				if attributes[0].FC != "" {
					fcStr = attributes[0].FC
				}
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\n  %d. %s [FC=%s]:\n", index, parentName, fcStr)
				index++

				for _, attr := range attributes {
					fcStr := "?"
					if attr.FC != "" {
						fcStr = attr.FC
					}
					valueStr := attr.Value
					if valueStr == "" {
						valueStr = "(nil)"
					}
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "     %-48s [FC=%-2s] Type: %-15s Value: %s\n",
						attr.Name, fcStr, attr.Type, valueStr)
				}
			}
		}
	}

	return nil
}

// daDisplayName joins a top-level DA name and a leaf name. Leaf names returned
// by the explorer may already carry the parent prefix ("Oper" -> "Oper.ctlVal")
// or equal the parent for scalar DAs ("q" -> "q"); never repeat the parent.
func daDisplayName(parent, name string) string {
	switch {
	case name == "" || name == parent:
		return parent
	case parent == "" || strings.HasPrefix(name, parent+"."):
		return name
	}
	return parent + "." + name
}

// sortedKeys returns the map keys in lexical order for deterministic output.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
