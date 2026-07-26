// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"
	"os"

	"github.com/otfabric/iec61850ctl/pkg/scl"

	"github.com/spf13/cobra"
)

var (
	sclParseInput   string
	sclParseDetail  bool
	sclParseFlatten bool
)

var sclParseCmd = &cobra.Command{
	Use:   "parse",
	Short: "Parse an SCL/CID file and print structure or flat list",
	Long:  `Parse an SCL XML file and output either a tree view or a flattened path list. Use --flatten for flat output, --detailed for FC/type/enum.`,
	RunE:  runSclParse,
}

func init() {
	sclParseCmd.Flags().StringVar(&sclParseInput, "input", "", "path to SCL/CID file (required)")
	_ = sclParseCmd.MarkFlagRequired("input")
	sclParseCmd.Flags().BoolVar(&sclParseDetail, "detailed", false, "include FC, type, and enum in output")
	sclParseCmd.Flags().BoolVar(&sclParseFlatten, "flatten", false, "output flat path list instead of tree")
	sclCmd.AddCommand(sclParseCmd)
}

func runSclParse(cmd *cobra.Command, args []string) error {
	f, err := os.Open(sclParseInput)
	if err != nil {
		return fmt.Errorf("open input: %w", err)
	}
	defer func() { _ = f.Close() }()

	doc, err := scl.Parse(f)
	if err != nil {
		return fmt.Errorf("parse SCL: %w", err)
	}

	if sclParseFlatten {
		entries, err := doc.Flatten()
		if err != nil {
			return fmt.Errorf("flatten: %w", err)
		}
		for _, e := range entries {
			fmt.Println(scl.FormatEntry(e, sclParseDetail))
		}
		return nil
	}

	return doc.WriteTree(os.Stdout, sclParseDetail)
}
