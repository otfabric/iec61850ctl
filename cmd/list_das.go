// SPDX-License-Identifier: MIT

// Package cmd implements the Cobra-based CLI interface for iec61850ctl.
// It defines all commands (root, list, get, tree) and their flags, delegating
// business logic to the services package.
package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/otfabric/iec61850ctl/internal/app"
	"github.com/otfabric/iec61850ctl/pkg/formatter"
	"github.com/otfabric/iec61850ctl/pkg/stack/client"
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
	finalHost, finalPort, err := getHostPort()
	if err != nil {
		return err
	}
	printConnectionTarget(finalHost, finalPort)

	conn, err := client.NewConnection(client.ConnectionInput{
		Host:           finalHost,
		Port:           finalPort,
		ConnectTimeout: 10,
		RequestTimeout: 10,
	})
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close(context.Background()) }()

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
		fmt.Printf("No data attributes found in '%s'\n", doRef)
		return nil
	}

	// Structured output: flatten map to array and render via Renderer
	outputFormat, ok := formatter.ParseOutputFormat(dasFormatFlag)
	if ok && outputFormat != formatter.OutputFormatText {
		var flatAttrs []view.DataAttribute
		for parentName, attributes := range attributesMap {
			for _, attr := range attributes {
				displayName := parentName
				if attr.Name != "" {
					displayName = parentName + "." + attr.Name
				}
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
				_, _ = os.Stdout.WriteString("[]\n")
			}
			return nil
		}
		renderer := formatter.NewRenderer(outputFormat)
		return renderer.RenderDataAttributes(flatAttrs, os.Stdout)
	}

	// Count total leaf attributes
	totalCount := 0
	for _, attributes := range attributesMap {
		totalCount += len(attributes)
	}

	if dasDetailedFlag {
		fmt.Printf("Found %d leaf data attribute(s) in '%s':\n", totalCount, doRef)
	} else {
		fmt.Printf("Found %d leaf data attribute(s) in '%s':\n", totalCount, doRef)
		fmt.Println()
	}

	index := 1
	for parentName, attributes := range attributesMap {
		if len(attributes) > 0 {
			if !dasDetailedFlag {
				// Simple mode: just list attribute names with numbering
				for _, attr := range attributes {
					if attr.Name == "" {
						fmt.Printf("  %d. %s\n", index, parentName)
					} else {
						fmt.Printf("  %d. %s.%s\n", index, parentName, attr.Name)
					}
					index++
				}
			} else {
				// Detailed mode: show full metadata
				fcStr := "?"
				if attributes[0].FC != "" {
					fcStr = attributes[0].FC
				}
				fmt.Printf("\n  %d. %s [FC=%s]:\n", index, parentName, fcStr)
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
					fmt.Printf("     %-48s [FC=%-2s] Type: %-15s Value: %s\n",
						attr.Name, fcStr, attr.Type, valueStr)
				}
			}
		}
	}

	return nil
}
