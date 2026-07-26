// SPDX-License-Identifier: MIT

// Package cmd implements the Cobra-based CLI interface for iec61850ctl.
// get_ds.go implements 'get ds' (alias: dataset) to retrieve a single data set by name.
package cmd

import (
	"context"
	"fmt"

	"github.com/otfabric/iec61850ctl/internal/app"
	"github.com/otfabric/iec61850ctl/pkg/stack/client"

	"github.com/spf13/cobra"
)

var (
	getDsLdFlag       string
	getDsLnFlag       string
	getDsNameFlag     string
	getDsDetailedFlag bool
)

var getDsCmd = &cobra.Command{
	Use:     "ds",
	Aliases: []string{"dataset"},
	Short:   "Get a specific data set by name",
	Long: `Retrieve and display a data set (DS) by name within a logical device and logical node.
Use --detailed to also read and display current values for each member.

Example:
  iec61850ctl get ds --host <server> --ld ZS1550REX640LD0 --ln LLN0 --name MeasReg
  iec61850ctl get dataset --host <server> --ld ZS1550REX640LD0 --ln LLN0 --name MeasReg --detailed`,
	RunE: runGetDs,
}

func init() {
	getCmd.AddCommand(getDsCmd)
	getDsCmd.Flags().StringVar(&getDsLdFlag, "ld", "", "Logical device name (required)")
	getDsCmd.Flags().StringVar(&getDsLnFlag, "ln", "", "Logical node name (required)")
	getDsCmd.Flags().StringVar(&getDsNameFlag, "name", "", "Data set name (required)")
	getDsCmd.Flags().BoolVar(&getDsDetailedFlag, "detailed", false, "Read and show current values for each member")
	_ = getDsCmd.MarkFlagRequired("ld")
	_ = getDsCmd.MarkFlagRequired("ln")
	_ = getDsCmd.MarkFlagRequired("name")
}

func runGetDs(cmd *cobra.Command, args []string) error {
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

	dsView, err := a.GetDataSetWithValues(app.GetDataSetWithValuesInput{
		LD:         getDsLdFlag,
		LN:         getDsLnFlag,
		Name:       getDsNameFlag,
		ReadValues: getDsDetailedFlag,
	})
	if err != nil {
		return fmt.Errorf("failed to get data set: %w", err)
	}

	dsRef := fmt.Sprintf("%s/%s.%s", getDsLdFlag, getDsLnFlag, getDsNameFlag)
	fmt.Printf("Data set: %s\n", dsRef)
	fmt.Printf("Deletable: %t\n", dsView.IsDeletable)
	fmt.Printf("Members (%d):\n", dsView.MemberCount)

	for i, member := range dsView.Members {
		if getDsDetailedFlag && member.Value != "" {
			fmt.Printf("  %d. %s = %s\n", i+1, member.Ref, member.Value)
		} else {
			fmt.Printf("  %d. %s\n", i+1, member.Ref)
		}
	}

	return nil
}
