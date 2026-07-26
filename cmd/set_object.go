// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/otfabric/iec61850ctl/internal/app"
	"github.com/otfabric/iec61850ctl/pkg/domain"
	"github.com/otfabric/iec61850ctl/pkg/formatter"
	"github.com/otfabric/iec61850ctl/pkg/service"
)

var (
	setObjectFlag string
	setFCFlag     string
	setValueFlag  string
	setTypeFlag   string
	setVerifyFlag bool
	setFormatFlag string
)

var setCmd = &cobra.Command{
	Use:   "set",
	Short: "Write values to the IEC 61850 server",
}

var setObjectCmd = &cobra.Command{
	Use:   "object",
	Short: "Write a scalar data attribute",
	Long: `Write an explicit scalar MMS value to a data attribute.

FC=CO is rejected; use control operate for controllable objects.
Type is never inferred: --type must be provided.`,
	RunE: runSetObject,
}

func init() {
	setObjectCmd.Flags().StringVar(&setObjectFlag, "object", "", "object reference (required)")
	setObjectCmd.Flags().StringVar(&setFCFlag, "fc", "", "functional constraint (required)")
	setObjectCmd.Flags().StringVar(&setValueFlag, "value", "", "value to write (required)")
	setObjectCmd.Flags().StringVar(&setTypeFlag, "type", "", "value type: bool|int|uint|float|enum|string (required)")
	setObjectCmd.Flags().BoolVar(&setVerifyFlag, "verify", false, "read back the same reference after write")
	setObjectCmd.Flags().StringVar(&setFormatFlag, "format", "text", "Output format: text, json")
	_ = setObjectCmd.MarkFlagRequired("object")
	_ = setObjectCmd.MarkFlagRequired("fc")
	_ = setObjectCmd.MarkFlagRequired("value")
	_ = setObjectCmd.MarkFlagRequired("type")

	setCmd.AddCommand(setObjectCmd)
	rootCmd.AddCommand(setCmd)
}

func runSetObject(cmd *cobra.Command, _ []string) error {
	format, err := parseCLIFormatFlag(setFormatFlag)
	if err != nil {
		return err
	}
	fc := domain.ParseFC(setFCFlag)
	if !fc.IsValid() {
		return fmt.Errorf("invalid functional constraint: %s", setFCFlag)
	}
	if fc == domain.FC_CO {
		return fmt.Errorf("FC=CO is not allowed for set object; use control operate")
	}
	kind, err := domain.ParseScalarKind(setTypeFlag)
	if err != nil {
		return err
	}
	val, err := service.ParseScalarValue(setValueFlag, kind)
	if err != nil {
		return err
	}

	session, err := openClientSession(cmd, clientSessionOptions{})
	if err != nil {
		return err
	}
	defer session.Close()

	ctx, cancel := context.WithTimeout(cmd.Context(), defaultRequestTimeout)
	defer cancel()

	res, err := app.New(session.Conn()).SetObject(ctx, domain.WriteRequest{
		Object: setObjectFlag,
		FC:     fc,
		Value:  val,
		Verify: setVerifyFlag,
	})
	if err != nil {
		return err
	}

	if format == formatter.OutputFormatJSON {
		if err := writeJSON(cmd, res); err != nil {
			return err
		}
	} else {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "object: %s\n", res.Object)
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "fc: %s\n", res.FC)
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "type: %s\n", res.Type)
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "requested_value: %v\n", res.RequestedValue)
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "write_ok: %v\n", res.WriteOK)
		if res.Error != "" {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "error: %s\n", res.Error)
		}
		if res.Verification != nil {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "verification matched=%v value=%v\n",
				res.Verification.Matched, res.Verification.Value)
		}
	}

	if !res.WriteOK {
		return fmt.Errorf("write failed")
	}
	if res.Verification != nil && res.Verification.Attempted && !res.Verification.Matched {
		return fmt.Errorf("verification mismatch")
	}
	return nil
}
