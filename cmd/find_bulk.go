// SPDX-License-Identifier: MIT

// Package cmd implements the Cobra-based CLI interface for iec61850ctl.
// This file implements the find bulk command.
package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/otfabric/iec61850ctl/internal/app"
	"github.com/otfabric/iec61850ctl/pkg/service"

	"github.com/spf13/cobra"
)

var (
	findBulkMappingFile string
	findBulkOutputFile  string
)

// mappingFileJSON matches the structure of mapping.json (iec61850Mappings array).
type mappingFileJSON struct {
	Iec61850Mappings []service.BulkMappingEntry `json:"iec61850Mappings"`
}

// resultsFileJSON matches the structure of results.json (iec61850Results array).
type resultsFileJSON struct {
	Iec61850Results []service.BulkResultEntry `json:"iec61850Results"`
}

var findBulkCmd = &cobra.Command{
	Use:   "bulk",
	Short: "Find paths for all entries in a mapping file with minimal device calls",
	Long: `Reads a mapping file (e.g. mapping.json) and discovers all device paths that match
each entry. Output is written to a JSON file (e.g. results.json) in the same format as
find path, but with one result per controlledPropertyId and minimal IEC 61850 calls.

Example:
  iec61850ctl find bulk --mapping mapping.json --output results.json`,
	RunE: runFindBulk,
}

func init() {
	findBulkCmd.Flags().StringVar(&findBulkMappingFile, "mapping", "", "path to mapping JSON file (required)")
	findBulkCmd.Flags().StringVar(&findBulkOutputFile, "output", "", "path to output results JSON file (required)")
	_ = findBulkCmd.MarkFlagRequired("mapping")
	_ = findBulkCmd.MarkFlagRequired("output")

	findCmd.AddCommand(findBulkCmd)
}

func runFindBulk(cmd *cobra.Command, args []string) error {
	data, err := os.ReadFile(findBulkMappingFile)
	if err != nil {
		return fmt.Errorf("read mapping file: %w", err)
	}

	var wrapper mappingFileJSON
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return fmt.Errorf("parse mapping file: %w", err)
	}
	mapping := wrapper.Iec61850Mappings
	if len(mapping) == 0 {
		// Write empty results and exit
		out := resultsFileJSON{Iec61850Results: []service.BulkResultEntry{}}
		b, _ := json.MarshalIndent(out, "", "  ")
		if err := os.WriteFile(findBulkOutputFile, b, 0644); err != nil {
			return fmt.Errorf("write output file: %w", err)
		}
		fmt.Printf("Total IEC61850 calls made: 0\n")
		return nil
	}

	session, err := openClientSession(cmd, clientSessionOptions{})
	if err != nil {
		return err
	}
	defer session.Close()
	conn := session.Conn()

	a := app.New(conn)
	result, err := a.BulkFind(app.BulkFindInput{Mappings: mapping})
	if err != nil {
		return err
	}

	out := resultsFileJSON{Iec61850Results: result.Entries}
	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return fmt.Errorf("encode results: %w", err)
	}
	if err := os.WriteFile(findBulkOutputFile, b, 0644); err != nil {
		return fmt.Errorf("write output file: %w", err)
	}

	fmt.Printf("Total IEC61850 calls made: %d\n", result.CallCount)
	return nil
}
