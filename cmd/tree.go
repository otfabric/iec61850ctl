// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/otfabric/iec61850ctl/internal/app"
	"github.com/otfabric/iec61850ctl/pkg/service"
	"github.com/otfabric/iec61850ctl/pkg/stack/client"
)

var (
	treeFlatten   bool
	treePath      string
	treeSerialize bool
	treeOutput    string
	treeInclude   string
	treeInterval  string
)

var treeCmd = &cobra.Command{
	Use:   "tree",
	Short: "Display the complete IEC 61850 device tree",
	Long: `Recursively traverses and displays the entire IEC 61850 device model hierarchy,
including all logical devices, logical nodes, data objects, and data attributes
with their functional constraints, types, and current values.`,
	RunE: runTree,
}

func init() {
	treeCmd.Flags().BoolVar(&treeFlatten, "flatten", false, "display output in flat format (LD/LN.DO.DA[FC]: value [type])")
	treeCmd.Flags().StringVar(&treePath, "path", "", "limit traversal to specific path (e.g., MNSREF615LD0/FMMXU1 or MNSREF615LD0/FMMXU1.Hz)")
	treeCmd.Flags().BoolVar(&treeSerialize, "serialize", false, "output JSON for server consumption (see SERVER.md); takes precedence over --flatten")
	treeCmd.Flags().StringVar(&treeOutput, "output", "", "write serialized JSON to file (default: stdout)")
	treeCmd.Flags().StringVar(&treeInclude, "include", "", "comma-separated: data_sets, report_control_blocks, or all (only with --serialize)")
	treeCmd.Flags().StringVar(&treeInterval, "interval", "0", "delay between each MMS call (e.g. 100ms, 1s); 0 = no delay")
	rootCmd.AddCommand(treeCmd)
}

func runTree(cmd *cobra.Command, args []string) error {
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

	d, err := time.ParseDuration(treeInterval)
	if err != nil {
		return fmt.Errorf("--interval: %w (e.g. 100ms, 1s)", err)
	}
	if d < 0 {
		return fmt.Errorf("--interval must be >= 0")
	}

	a := app.New(conn)
	input := app.TreeInput{
		Host:         finalHost,
		Port:         finalPort,
		Path:         treePath,
		CallInterval: d,
	}

	if treeSerialize {
		input.IncludeDataSets, input.IncludeReports = parseTreeInclude(treeInclude)
		model, err := a.BuildSerializableTree(input)
		if err != nil {
			return err
		}
		data, err := json.MarshalIndent(model, "", "  ")
		if err != nil {
			return fmt.Errorf("serialize: %w", err)
		}
		w := os.Stdout
		var outFile *os.File
		if treeOutput != "" {
			f, err := os.Create(treeOutput)
			if err != nil {
				return fmt.Errorf("output file: %w", err)
			}
			outFile = f
			w = f
		}
		if _, err = w.Write(data); err != nil {
			if outFile != nil {
				_ = outFile.Close()
			}
			return err
		}
		if outFile != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Wrote serialized IED to %s\n", treeOutput)
			return outFile.Close()
		}
		return nil
	}

	var callCount int
	if treeFlatten {
		callCount, err = service.NewTree(conn).WithCallInterval(d).RenderDeviceTreeFlat(os.Stdout, finalHost, finalPort, treePath)
	} else {
		callCount, err = a.RenderTree(os.Stdout, input)
	}
	if err != nil {
		return err
	}
	fmt.Printf("\nTotal IEC61850 calls made: %d\n", callCount)
	return nil
}

func parseTreeInclude(include string) (dataSets, reports bool) {
	for _, s := range strings.Split(include, ",") {
		s = strings.TrimSpace(strings.ToLower(s))
		switch s {
		case "all":
			return true, true
		case "data_sets":
			dataSets = true
		case "report_control_blocks":
			reports = true
		}
	}
	return dataSets, reports
}
