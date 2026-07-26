// SPDX-License-Identifier: MIT

package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/otfabric/iec61850ctl/pkg/domain"
	"github.com/otfabric/iec61850ctl/pkg/stack/server"

	"github.com/spf13/cobra"
)

var (
	serverGenerateInput  string
	serverGenerateOutput string
	serverGenerateName   string
)

var serverGenerateConfigCmd = &cobra.Command{
	Use:   "generate-config",
	Short: "Generate libiec61850 .cfg from serialized IED JSON",
	Long:  `Read a JSON file produced by 'tree --serialize' and write a libiec61850 server .cfg file.`,
	RunE:  runServerGenerateConfig,
}

func init() {
	serverCmd.AddCommand(serverGenerateConfigCmd)
	serverGenerateConfigCmd.Flags().StringVarP(&serverGenerateInput, "input", "i", "", "path to serialized IED JSON (required)")
	serverGenerateConfigCmd.Flags().StringVarP(&serverGenerateOutput, "output", "o", "", "path to write .cfg file (required)")
	serverGenerateConfigCmd.Flags().StringVar(&serverGenerateName, "name", "", "MODEL name in .cfg (MMS domain = name+LD); empty = LD name only (match source device)")
	_ = serverGenerateConfigCmd.MarkFlagRequired("input")
	_ = serverGenerateConfigCmd.MarkFlagRequired("output")
}

func runServerGenerateConfig(cmd *cobra.Command, args []string) error {
	data, err := os.ReadFile(serverGenerateInput)
	if err != nil {
		return fmt.Errorf("read input: %w", err)
	}

	var ied domain.IED
	if err := json.Unmarshal(data, &ied); err != nil {
		return fmt.Errorf("decode IED JSON: %w", err)
	}

	cfg, err := server.IEDToCfg(&ied, serverGenerateName)
	if err != nil {
		return fmt.Errorf("generate config: %w", err)
	}

	outPath := serverGenerateOutput
	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("create output dir: %w", err)
	}
	if err := os.WriteFile(outPath, cfg, 0644); err != nil {
		return fmt.Errorf("write output: %w", err)
	}

	fmt.Printf("Wrote %s (%d bytes)\n", outPath, len(cfg))
	return nil
}
