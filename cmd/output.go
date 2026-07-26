// SPDX-License-Identifier: MIT

package cmd

import (
	"encoding/json"
	"io"

	"github.com/spf13/cobra"

	"github.com/otfabric/iec61850ctl/pkg/formatter"
)

func writeJSON(cmd *cobra.Command, v any) error {
	w := io.Writer(nil)
	if cmd != nil {
		w = cmd.OutOrStdout()
	}
	if w == nil {
		w = io.Discard
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func parseCLIFormatFlag(s string) (formatter.OutputFormat, error) {
	return formatter.ParseCLIFormat(s)
}
