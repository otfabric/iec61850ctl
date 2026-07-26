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

	"github.com/spf13/cobra"
)

var (
	ldNameForDos    string
	lnNameForDos    string
	dosDetailedFlag bool
	dosFormatFlag   string
)

var listDosCmd = &cobra.Command{
	Use:   "dos",
	Short: "List all data objects within a specific logical node",
	Long: `Retrieves and displays the list of all data objects (DOs) within the specified logical node.
Requires both --ld and --ln flags to specify the logical device and logical node.`,
	RunE: runListDos,
}

func init() {
	listDosCmd.Flags().StringVar(&ldNameForDos, "ld", "", "logical device name (required)")
	listDosCmd.Flags().StringVar(&lnNameForDos, "ln", "", "logical node name (required)")
	listDosCmd.Flags().BoolVar(&dosDetailedFlag, "detailed", false, "show data attribute count for each data object")
	listDosCmd.Flags().StringVar(&dosFormatFlag, "format", "text", "Output format: text, json, csv, table, yaml")
	_ = listDosCmd.MarkFlagRequired("ld")
	_ = listDosCmd.MarkFlagRequired("ln")

	listCmd.AddCommand(listDosCmd)
}

// runListDos executes the 'list dos' command to display data objects within a logical node.
// It requires --ld and --ln flags and uses the explorer service to retrieve DO names.
// Returns an error if connection or listing fails.
func runListDos(cmd *cobra.Command, args []string) error {
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

	a := app.New(conn)

	// Structured output: fetch full metadata and render via Renderer
	outputFormat, ok := formatter.ParseOutputFormat(dosFormatFlag)
	if ok && outputFormat != formatter.OutputFormatText {
		objects, err := a.ListDataObjects(app.ListDataObjectsInput{LD: ldNameForDos, LN: lnNameForDos})
		if err != nil {
			return err
		}
		if len(objects) == 0 {
			if outputFormat == formatter.OutputFormatJSON {
				_, _ = os.Stdout.WriteString("[]\n")
			}
			return nil
		}
		renderer := formatter.NewRenderer(outputFormat)
		return renderer.RenderDataObjects(objects, os.Stdout)
	}

	// Text format: simple listing without metadata
	doNames, err := a.ListDataObjectNames(app.ListDataObjectsInput{LD: ldNameForDos, LN: lnNameForDos})
	if err != nil {
		return err
	}

	lnRef := fmt.Sprintf("%s/%s", ldNameForDos, lnNameForDos)
	if len(doNames) == 0 {
		fmt.Printf("No data objects found in '%s'\n", lnRef)
		return nil
	}

	fmt.Printf("Found %d data object(s) in '%s':\n", len(doNames), lnRef)

	if !dosDetailedFlag {
		// Simple mode: numbered list
		fmt.Println()
		for i, do := range doNames {
			fmt.Printf("  %d. %s\n", i+1, do)
		}
	} else {
		// Detailed mode: show data attribute count for each DO
		fmt.Println()
		for i, do := range doNames {
			// Get data attributes to count them
			attrsMap, err := a.ListDataAttributes(app.ListDataAttributesInput{
				LD:       ldNameForDos,
				LN:       lnNameForDos,
				DO:       do,
				Detailed: false,
			})

			daCount := 0
			if err == nil {
				for _, attrs := range attrsMap {
					daCount += len(attrs)
				}
			}

			if err != nil {
				fmt.Printf("  %d. %-30s [DAs: <error>]\n", i+1, do)
			} else {
				fmt.Printf("  %d. %-30s [DAs: %3d]\n", i+1, do, daCount)
			}
		}
	}

	return nil
}
