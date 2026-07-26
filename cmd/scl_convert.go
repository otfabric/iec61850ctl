// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"
	"os"

	"github.com/otfabric/iec61850ctl/pkg/scl"

	"github.com/spf13/cobra"
)

var (
	sclConvertInput     string
	sclConvertOutput    string
	sclConvertSeparator string
)

var sclConvertCmd = &cobra.Command{
	Use:   "convert",
	Short: "Convert an SCL/CID file to CSV",
	Long:  `Parse an SCL XML file, flatten the data model, and write CSV to the given output file.`,
	RunE:  runSclConvert,
}

func init() {
	sclConvertCmd.Flags().StringVar(&sclConvertInput, "input", "", "path to SCL/CID file (required)")
	sclConvertCmd.Flags().StringVar(&sclConvertOutput, "output", "", "path to output CSV file (required)")
	sclConvertCmd.Flags().StringVar(&sclConvertSeparator, "separator", ",", "CSV field separator (e.g. \",\" or \"|\")")
	_ = sclConvertCmd.MarkFlagRequired("input")
	_ = sclConvertCmd.MarkFlagRequired("output")
	sclCmd.AddCommand(sclConvertCmd)
}

func runSclConvert(cmd *cobra.Command, args []string) error {
	f, err := os.Open(sclConvertInput)
	if err != nil {
		return fmt.Errorf("open input: %w", err)
	}
	defer func() { _ = f.Close() }()

	doc, err := scl.Parse(f)
	if err != nil {
		return fmt.Errorf("parse SCL: %w", err)
	}

	entries, err := doc.Flatten()
	if err != nil {
		return fmt.Errorf("flatten: %w", err)
	}

	out, err := os.Create(sclConvertOutput)
	if err != nil {
		return fmt.Errorf("create output: %w", err)
	}
	defer func() { _ = out.Close() }()

	if err := scl.WriteCSV(out, sclConvertSeparator, entries); err != nil {
		return fmt.Errorf("write CSV: %w", err)
	}

	_, _ = fmt.Fprintf(os.Stderr, "Wrote %d rows to %s\n", len(entries), sclConvertOutput)
	return nil
}
