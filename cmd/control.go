// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/otfabric/iec61850ctl/internal/app"
	"github.com/otfabric/iec61850ctl/pkg/domain"
	"github.com/otfabric/iec61850ctl/pkg/formatter"
	"github.com/otfabric/iec61850ctl/pkg/service"
)

var (
	controlObjectFlag    string
	controlValueFlag     string
	controlTypeFlag      string
	controlModeFlag      string
	controlCtlNumFlag    int
	controlOrCatFlag     string
	controlOrIdentFlag   string
	controlTestFlag      bool
	controlSynchroFlag   bool
	controlInterlockFlag bool
	controlConfirmRef    string
	controlNoConfirm     bool
	controlDryRunFlag    bool
	controlFormatFlag    string
)

var controlCmd = &cobra.Command{
	Use:   "control",
	Short: "Inspect and operate IEC 61850 controllable objects",
	Long: `Control commands execute on a single MMS association.

control operate runs the complete select/operate sequence atomically.
Standalone select across separate CLI processes is not supported.`,
}

var controlInspectCmd = &cobra.Command{
	Use:   "inspect",
	Short: "Read the control model of a controllable object",
	RunE:  runControlInspect,
}

var controlOperateCmd = &cobra.Command{
	Use:   "operate",
	Short: "Atomically select (if required) and operate a controllable object",
	RunE:  runControlOperate,
}

func init() {
	controlInspectCmd.Flags().StringVar(&controlObjectFlag, "object", "", "controllable DO reference without FC (required)")
	controlInspectCmd.Flags().StringVar(&controlFormatFlag, "format", "text", "Output format: text, json")
	_ = controlInspectCmd.MarkFlagRequired("object")

	controlOperateCmd.Flags().StringVar(&controlObjectFlag, "object", "", "controllable DO reference without FC (required)")
	controlOperateCmd.Flags().StringVar(&controlValueFlag, "value", "", "control value (required unless --dry-run with inspect-only planning)")
	controlOperateCmd.Flags().StringVar(&controlTypeFlag, "type", "", "value type: bool|int|uint|float|enum|string (required)")
	controlOperateCmd.Flags().StringVar(&controlModeFlag, "mode", "auto", "control mode: auto|direct|sbo|sbow")
	controlOperateCmd.Flags().IntVar(&controlCtlNumFlag, "ctl-num", 0, "control sequence number 1..255 (0=auto)")
	controlOperateCmd.Flags().StringVar(&controlOrCatFlag, "or-cat", "remote-control", "originator category")
	controlOperateCmd.Flags().StringVar(&controlOrIdentFlag, "or-ident", "", "originator identifier (UTF-8, max 64 bytes)")
	controlOperateCmd.Flags().BoolVar(&controlTestFlag, "test", false, "set Test bit (also disables confirmation)")
	controlOperateCmd.Flags().BoolVar(&controlSynchroFlag, "check-synchro", false, "set synchrocheck bit")
	controlOperateCmd.Flags().BoolVar(&controlInterlockFlag, "check-interlock", false, "set interlockCheck bit")
	controlOperateCmd.Flags().StringVar(&controlConfirmRef, "confirm-ref", "", "complete reference for status read-back, e.g. LD/LN.DO.stVal[ST]")
	controlOperateCmd.Flags().BoolVar(&controlNoConfirm, "no-confirm", false, "skip confirmation even if --confirm-ref is set")
	controlOperateCmd.Flags().BoolVar(&controlDryRunFlag, "dry-run", false, "plan the sequence without writing SBO/SBOw/Oper")
	controlOperateCmd.Flags().StringVar(&controlFormatFlag, "format", "text", "Output format: text, json")
	_ = controlOperateCmd.MarkFlagRequired("object")
	_ = controlOperateCmd.MarkFlagRequired("type")
	_ = controlOperateCmd.MarkFlagRequired("value")

	controlCmd.AddCommand(controlInspectCmd)
	controlCmd.AddCommand(controlOperateCmd)
	rootCmd.AddCommand(controlCmd)
}

func runControlInspect(cmd *cobra.Command, _ []string) error {
	format, err := parseCLIFormatFlag(controlFormatFlag)
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

	res, err := app.New(session.Conn()).ControlInspect(ctx, controlObjectFlag)
	if err != nil {
		return err
	}

	if format == formatter.OutputFormatJSON {
		return writeJSON(cmd, res)
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "object: %s\n", res.Object)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "control_model: %d (%s)\n", res.ControlModel.Code, res.ControlModel.Name)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "controllable: %v\n", res.Controllable)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "requires_select: %v\n", res.RequiresSelect)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "enhanced_security: %v\n", res.Enhanced)
	return nil
}

func runControlOperate(cmd *cobra.Command, _ []string) error {
	format, err := parseCLIFormatFlag(controlFormatFlag)
	if err != nil {
		return err
	}

	mode, err := domain.ParseControlMode(controlModeFlag)
	if err != nil {
		return err
	}
	kind, err := domain.ParseScalarKind(controlTypeFlag)
	if err != nil {
		return err
	}
	val, err := service.ParseScalarValue(controlValueFlag, kind)
	if err != nil {
		return err
	}
	orCat, err := domain.ParseOriginCategory(controlOrCatFlag)
	if err != nil {
		return err
	}
	if controlCtlNumFlag != 0 && (controlCtlNumFlag < 1 || controlCtlNumFlag > 255) {
		return fmt.Errorf("ctl-num must be 1..255")
	}
	if len([]byte(controlOrIdentFlag)) > domain.MaxOrIdentBytes {
		return fmt.Errorf("or-ident exceeds %d bytes", domain.MaxOrIdentBytes)
	}

	session, err := openClientSession(cmd, clientSessionOptions{})
	if err != nil {
		return err
	}
	defer session.Close()

	ctx, cancel := context.WithTimeout(cmd.Context(), defaultRequestTimeout)
	defer cancel()

	req := domain.ControlRequest{
		Object: controlObjectFlag,
		Mode:   mode,
		Value:  val,
		CtlNum: uint8(controlCtlNumFlag),
		Origin: domain.ControlOrigin{
			Category: orCat,
			Ident:    controlOrIdentFlag,
		},
		Test:       controlTestFlag,
		Check:      domain.CheckConditions{Synchro: controlSynchroFlag, Interlock: controlInterlockFlag},
		ConfirmRef: controlConfirmRef,
		NoConfirm:  controlNoConfirm || controlTestFlag,
		DryRun:     controlDryRunFlag,
	}

	res, err := app.New(session.Conn()).ControlOperate(ctx, req)
	if err != nil {
		return err
	}

	if format == formatter.OutputFormatJSON {
		if err := writeJSON(cmd, res); err != nil {
			return err
		}
	} else {
		printControlResult(cmd, res)
	}

	if res.Status.ExitNonZero() {
		return fmt.Errorf("control status %s", res.Status)
	}
	return nil
}

func printControlResult(cmd *cobra.Command, res *domain.ControlResult) {
	out := cmd.OutOrStdout()
	_, _ = fmt.Fprintf(out, "object: %s\n", res.Object)
	_, _ = fmt.Fprintf(out, "control_model: %d (%s)\n", res.ControlModel.Code, res.ControlModel.Name)
	if res.Mode != "" {
		_, _ = fmt.Fprintf(out, "mode: %s\n", res.Mode)
	}
	if res.CtlNum != 0 {
		_, _ = fmt.Fprintf(out, "ctl_num: %d\n", res.CtlNum)
	}
	if res.RequestedValue != nil {
		_, _ = fmt.Fprintf(out, "requested_value: %v\n", res.RequestedValue)
	}
	for _, op := range res.Operations {
		line := fmt.Sprintf("operation %s: %s", op.Operation, strconv.FormatBool(op.OK))
		if op.Error != "" {
			line += " error=" + op.Error
		}
		_, _ = fmt.Fprintln(out, line)
	}
	if res.Confirmation != nil {
		_, _ = fmt.Fprintf(out, "confirmation attempted=%v matched=%v value=%v\n",
			res.Confirmation.Attempted, res.Confirmation.Matched, res.Confirmation.Value)
	}
	if res.Cleanup != nil {
		_, _ = fmt.Fprintf(out, "cleanup cancel attempted=%v ok=%v\n", res.Cleanup.Attempted, res.Cleanup.OK)
	}
	if res.Status != "" {
		_, _ = fmt.Fprintf(out, "status: %s\n", res.Status)
	}
}
