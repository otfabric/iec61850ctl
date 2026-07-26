// SPDX-License-Identifier: MIT

// Package cmd provides the command-line interface for the iec61850ctl tool.
package cmd

import (
	"context"
	"fmt"

	"github.com/otfabric/iec61850ctl/internal/app"
	"github.com/otfabric/iec61850ctl/pkg/stack/client"

	"github.com/spf13/cobra"
)

var (
	dssLdFlag       string
	dssLnFlag       string
	dssDetailedFlag bool
)

// listDssCmd represents the 'list dss' command for listing data sets within a logical node.
var listDssCmd = &cobra.Command{
	Use:   "dss",
	Short: "List all data sets in a logical node",
	Long: `List all data sets (DS) within a specific logical node.

Data sets are named collections of data attributes that are used by report 
control blocks to define what data to include in reports.

Example:
  iec61850ctl list dss --host <server> --ld MNSREF615LD0 --ln LLN0`,
	RunE: runListDss,
}

func init() {
	listCmd.AddCommand(listDssCmd)
	listDssCmd.Flags().StringVar(&dssLdFlag, "ld", "", "Logical device name (required)")
	listDssCmd.Flags().StringVar(&dssLnFlag, "ln", "", "Logical node name (required)")
	listDssCmd.Flags().BoolVar(&dssDetailedFlag, "detailed", false, "Show detailed information including DataSet members")
	_ = listDssCmd.MarkFlagRequired("ld")
	_ = listDssCmd.MarkFlagRequired("ln")
}

func runListDss(cmd *cobra.Command, args []string) error {
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
		return fmt.Errorf("connection failed: %w", err)
	}
	defer func() { _ = conn.Close(context.Background()) }()

	a := app.New(conn)

	// Simple listing
	dataSets, err := a.ListDataSetNames(app.ListDataSetsInput{LD: dssLdFlag, LN: dssLnFlag})
	if err != nil {
		return err
	}

	if len(dataSets) == 0 {
		fmt.Printf("No data sets found in '%s/%s'\n", dssLdFlag, dssLnFlag)
		return nil
	}

	fmt.Printf("Found %d data set(s) in '%s/%s':\n", len(dataSets), dssLdFlag, dssLnFlag)

	// Simple listing
	if !dssDetailedFlag {
		for i, ds := range dataSets {
			fmt.Printf("  %d. %s\n", i+1, ds)
		}
		fmt.Println("\nUse --detailed flag to see DataSet members")
		return nil
	}

	// Detailed listing with members
	for i, ds := range dataSets {
		fmt.Printf("\n  %d. %s\n", i+1, ds)

		dsView, err := a.GetDataSet(app.GetDataSetInput{LD: dssLdFlag, LN: dssLnFlag, Name: ds})
		if err != nil {
			fmt.Printf("     Error getting details: %v\n", err)
			continue
		}

		fmt.Printf("     Deletable: %t\n", dsView.IsDeletable)
		fmt.Printf("     Members (%d):\n", dsView.MemberCount)
		for j, member := range dsView.Members {
			fmt.Printf("       %d. %s\n", j+1, member.Ref)
		}
	}

	return nil
}
