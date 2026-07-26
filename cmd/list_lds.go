// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/otfabric/iec61850ctl/internal/app"
	"github.com/otfabric/iec61850ctl/pkg/formatter"
)

var (
	ldsDetailedFlag bool
	ldsFormatFlag   string
)

var listLdsCmd = &cobra.Command{
	Use:     "lds",
	Aliases: []string{"domains"},
	Short:   "List all logical devices (alias: domains) available on the IEC 61850 server",
	Long:    `Retrieves and displays the list of all logical devices (LDs) from the connected IEC 61850 server.`,
	RunE:    runListLds,
}

func init() {
	listCmd.AddCommand(listLdsCmd)
	listLdsCmd.Flags().BoolVar(&ldsDetailedFlag, "detailed", false, "Show detailed breakdown of DataSets and Reports per LN")
	listLdsCmd.Flags().StringVar(&ldsFormatFlag, "format", "text", "Output format: text, json, csv, table, yaml")
}

func runListLds(cmd *cobra.Command, args []string) error {
	session, err := openClientSession(cmd, clientSessionOptions{})
	if err != nil {
		return err
	}
	defer session.Close()
	conn := session.Conn()

	a := app.New(conn)

	outputFormat, ok := formatter.ParseOutputFormat(ldsFormatFlag)
	if ok && outputFormat != formatter.OutputFormatText {
		devices, err := a.ListLogicalDevices()
		if err != nil {
			return err
		}
		if len(devices) == 0 {
			if outputFormat == formatter.OutputFormatJSON {
				_, _ = os.Stdout.WriteString("[]\n")
			}
			return nil
		}
		renderer := formatter.NewRenderer(outputFormat)
		return renderer.RenderLogicalDevices(devices, os.Stdout)
	}

	if !ldsDetailedFlag {
		devices, err := a.ListLogicalDeviceNames()
		if err != nil {
			return err
		}
		if len(devices) == 0 {
			fmt.Println("No logical devices found")
			return nil
		}
		fmt.Printf("Found %d logical device(s):\n", len(devices))
		for i, ld := range devices {
			fmt.Printf("  %d. %s\n", i+1, ld)
		}
		fmt.Println("\nUse --detailed flag to see DataSet/Report breakdown")
		return nil
	}

	devices, err := a.ListLogicalDevices()
	if err != nil {
		return err
	}
	if len(devices) == 0 {
		fmt.Println("No logical devices found")
		return nil
	}

	fmt.Printf("Found %d logical device(s):\n", len(devices))
	ctx := context.Background()
	for i, ld := range devices {
		totalReports := ld.URCBCount + ld.BRCBCount
		fmt.Printf("\n  %d. %-30s  [LNs: %3d, DataSets: %3d, Reports: %3d (UB:%d/B:%d)]\n",
			i+1, ld.Name, ld.LNCount, ld.DSCount, totalReports, ld.URCBCount, ld.BRCBCount)

		lnNames, err := a.Explorer().ListLogicalNodes(ld.Name)
		if err != nil {
			fmt.Printf("     Error getting LN details: %v\n", err)
			continue
		}
		fmt.Println("     Logical Nodes with DataSets/Reports:")
		hasContent := false
		dss, _ := conn.ListDataSets(ctx, ld.Name)
		reports, _ := conn.ListReports(ctx, ld.Name)
		for _, lnName := range lnNames {
			prefix := lnName + "$"
			lnDataSets, lnURCBs, lnBRCBs := 0, 0, 0
			for _, ds := range dss {
				if strings.HasPrefix(ds, prefix) {
					lnDataSets++
				}
			}
			for _, r := range reports {
				if !strings.HasPrefix(r, prefix) {
					continue
				}
				if strings.Contains(r, "$BR$") {
					lnBRCBs++
				} else {
					lnURCBs++
				}
			}
			if lnDataSets > 0 || lnURCBs > 0 || lnBRCBs > 0 {
				hasContent = true
				fmt.Printf("       %-20s  [DataSets: %2d, Reports: %2d (UB:%d/B:%d)]\n",
					lnName, lnDataSets, lnURCBs+lnBRCBs, lnURCBs, lnBRCBs)
			}
		}
		if !hasContent {
			fmt.Println("       (none)")
		}
	}
	return nil
}
