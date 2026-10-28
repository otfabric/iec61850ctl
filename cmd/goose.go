// SPDX-License-Identifier: MIT

package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	iec61850 "github.com/otfabric/go-iec61850"
	"github.com/otfabric/go-iec61850/goose"
	"github.com/spf13/cobra"

	"github.com/otfabric/iec61850ctl/pkg/errs"
	"github.com/otfabric/iec61850ctl/pkg/formatter"
	"github.com/otfabric/iec61850ctl/pkg/gooseops"
	"github.com/otfabric/iec61850ctl/pkg/service"
)

// gooseOpen opens a raw Ethernet GOOSE transport; tests substitute an
// in-memory hub port.
var gooseOpen = goose.Open

var gooseFlags struct {
	scl        string
	iface      string
	gocb       string
	format     string
	count      int
	duration   time.Duration
	appIDs     []string
	dst        string
	gocbRef    string
	bareDomain bool
	onlyStates bool
	acceptSim  bool

	values     []string
	simulation bool
	src        string
	changes    int
	interval   time.Duration
}

var gooseCmd = &cobra.Command{
	Use:   "goose",
	Short: "GOOSE: list control blocks, sniff, subscribe, publish, read/write GoCBs",
	Long: `GOOSE (IEC 61850-8-1 clause 18) over raw Ethernet.

sniff, subscribe and publish open a raw socket on --iface: AF_PACKET on Linux
(needs CAP_NET_RAW) or /dev/bpf* on macOS/BSD (needs access to /dev/bpf*).
list reads an SCL file offline; gocb get/set use MMS (--host).

publish injects frames into the station bus. It needs an explicit --iface and
--yes, and frames carry test=true unless --test=false is combined with --live.`,
}

var gooseListCmd = &cobra.Command{
	Use:   "list",
	Short: "List GOOSE control blocks (GSEControl + GSE address + dataset) from SCL",
	Example: `  iec61850ctl goose list --scl station.scd
  iec61850ctl goose list --scl device.cid --format json`,
	RunE: runGooseList,
}

var gooseSniffCmd = &cobra.Command{
	Use:   "sniff",
	Short: "Decode and print GOOSE frames seen on an interface",
	Example: `  iec61850ctl goose sniff --iface eth0
  iec61850ctl goose sniff --iface eth0 --appid 0x1001 --count 10 --format jsonl`,
	RunE: runGooseSniff,
}

var gooseSubscribeCmd = &cobra.Command{
	Use:   "subscribe",
	Short: "Subscribe to one GoCB from SCL with validation and supervision",
	Example: `  iec61850ctl goose subscribe --iface eth0 --scl station.scd --gocb IED1/LD0/gcb1
  iec61850ctl goose subscribe --iface eth0 --scl device.cid --gocb gcb1 --count 5 --duration 30s --format jsonl`,
	RunE: runGooseSubscribe,
}

var goosePublishCmd = &cobra.Command{
	Use:   "publish",
	Short: "Publish a GoCB from SCL (test frames unless --test=false --live)",
	Example: `  iec61850ctl goose publish --iface eth0 --scl device.cid --gocb gcb1 --yes --duration 10s
  iec61850ctl goose publish --iface eth0 --scl device.cid --gocb gcb1 --value 0=true --changes 3 --interval 1s --yes`,
	RunE: runGoosePublish,
}

var gooseGocbCmd = &cobra.Command{
	Use:   "gocb",
	Short: "Read or write a GOOSE control block over MMS",
}

// goosePublishSafety guards goose publish; frames are marked with the test flag.
var goosePublishSafety = publishSafety{protocol: "GOOSE", iface: &gooseFlags.iface, marker: "test"}

var gooseGocbFlags struct {
	ld      string
	gocb    string
	format  string
	enable  bool
	disable bool
	goID    string
	datSet  string
}

var gooseGocbGetCmd = &cobra.Command{
	Use:     "get",
	Short:   "Read a GoCB (GoEna, GoID, DatSet, ConfRev, DstAddress, times)",
	Example: `  iec61850ctl goose gocb get --host 192.0.2.10 --ld LD0 --gocb gcb1 --format json`,
	RunE:    runGooseGocbGet,
}

var gooseGocbSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Write GoCB attributes (GoEna written last)",
	Example: `  iec61850ctl goose gocb set --host 192.0.2.10 --ld LD0 --gocb gcb1 --disable
  iec61850ctl goose gocb set --host 192.0.2.10 --ld LD0 --gocb gcb1 --dat-set LD0/LLN0\$ds2 --enable`,
	RunE: runGooseGocbSet,
}

func init() {
	f := gooseListCmd.Flags()
	f.StringVar(&gooseFlags.scl, "scl", "", "SCL file (SCD/CID/ICD) (required)")
	f.StringVar(&gooseFlags.format, "format", "text", "Output format: text, json")
	_ = gooseListCmd.MarkFlagRequired("scl")

	f = gooseSniffCmd.Flags()
	f.StringVar(&gooseFlags.iface, "iface", "", "network interface to capture on (required)")
	f.StringSliceVar(&gooseFlags.appIDs, "appid", nil, "only frames with these APPIDs (hex, repeatable)")
	f.StringVar(&gooseFlags.dst, "dst", "", "only frames to this destination MAC (joins the multicast group)")
	f.StringVar(&gooseFlags.gocbRef, "gocb-ref", "", "only frames with this gocbRef")
	addGooseRunFlags(gooseSniffCmd)
	_ = gooseSniffCmd.MarkFlagRequired("iface")

	f = gooseSubscribeCmd.Flags()
	f.StringVar(&gooseFlags.iface, "iface", "", "network interface to receive on (required)")
	f.StringVar(&gooseFlags.scl, "scl", "", "SCL file describing the publisher (required)")
	f.StringVar(&gooseFlags.gocb, "gocb", "", "control block: name, LD/name, IED/LD/name or gocbRef (required)")
	f.BoolVar(&gooseFlags.bareDomain, "bare-domain", false, "expect references with the bare LD as MMS domain (LD/LLN0$GO$gcb) instead of IED+LD")
	f.BoolVar(&gooseFlags.onlyStates, "only-changes", false, "print state changes only, not retransmissions")
	f.BoolVar(&gooseFlags.acceptSim, "accept-simulation", false, "accept frames with the simulation bit set")
	addGooseRunFlags(gooseSubscribeCmd)
	for _, n := range []string{"iface", "scl", "gocb"} {
		_ = gooseSubscribeCmd.MarkFlagRequired(n)
	}

	f = goosePublishCmd.Flags()
	f.StringVar(&gooseFlags.iface, "iface", "", "network interface to publish on (required, never defaulted)")
	f.StringVar(&gooseFlags.scl, "scl", "", "SCL file with the control block (required)")
	f.StringVar(&gooseFlags.gocb, "gocb", "", "control block: name, LD/name, IED/LD/name or gocbRef (required)")
	f.StringArrayVar(&gooseFlags.values, "value", nil, "dataset value as INDEX=VALUE or MEMBER=VALUE (repeatable; others are zero)")
	goosePublishSafety.register(goosePublishCmd, "set the test flag in every frame")
	f.BoolVar(&gooseFlags.simulation, "simulation", false, "set the Edition 2 simulation bit")
	f.StringVar(&gooseFlags.src, "src", "", "source MAC (default: the interface address)")
	f.BoolVar(&gooseFlags.bareDomain, "bare-domain", false, "publish references with the bare LD as MMS domain")
	f.IntVar(&gooseFlags.changes, "changes", 0, "additional state changes to publish after the initial state")
	f.DurationVar(&gooseFlags.interval, "interval", time.Second, "delay between state changes")
	f.DurationVar(&gooseFlags.duration, "duration", 0, "keep retransmitting this long after the last change (0 = until Ctrl+C)")
	f.StringVar(&gooseFlags.format, "format", "text", "Output format: text, jsonl")
	for _, n := range []string{"iface", "scl", "gocb"} {
		_ = goosePublishCmd.MarkFlagRequired(n)
	}

	for _, c := range []*cobra.Command{gooseGocbGetCmd, gooseGocbSetCmd} {
		c.Flags().StringVar(&gooseGocbFlags.ld, "ld", "", "logical device (required)")
		c.Flags().StringVar(&gooseGocbFlags.gocb, "gocb", "", "GoCB name (gcb1) or item (LLN0$GO$gcb1) (required)")
		c.Flags().StringVar(&gooseGocbFlags.format, "format", "text", "Output format: text, json")
		_ = c.MarkFlagRequired("ld")
		_ = c.MarkFlagRequired("gocb")
	}
	sf := gooseGocbSetCmd.Flags()
	sf.BoolVar(&gooseGocbFlags.enable, "enable", false, "set GoEna=true")
	sf.BoolVar(&gooseGocbFlags.disable, "disable", false, "set GoEna=false")
	sf.StringVar(&gooseGocbFlags.goID, "go-id", "", "new GoID (only while disabled)")
	sf.StringVar(&gooseGocbFlags.datSet, "dat-set", "", "new DatSet reference (only while disabled; increments ConfRev)")
	gooseGocbSetCmd.MarkFlagsMutuallyExclusive("enable", "disable")
	gooseGocbSetCmd.MarkFlagsOneRequired("enable", "disable", "go-id", "dat-set")

	gooseGocbCmd.AddCommand(gooseGocbGetCmd, gooseGocbSetCmd)
	gooseCmd.AddCommand(gooseListCmd, gooseSniffCmd, gooseSubscribeCmd, goosePublishCmd, gooseGocbCmd)
	rootCmd.AddCommand(gooseCmd)
}

func addGooseRunFlags(c *cobra.Command) {
	c.Flags().IntVar(&gooseFlags.count, "count", 0, "stop after this many frames/messages (0 = unlimited)")
	c.Flags().DurationVar(&gooseFlags.duration, "duration", 0, "stop after this long (0 = until Ctrl+C); with --count, exit 4 if not reached")
	c.Flags().StringVar(&gooseFlags.format, "format", "text", "Output format: text, jsonl")
}

func gooseOutput(cmd *cobra.Command) (gooseops.Output, error) {
	sf, err := formatter.ParseStreamFormat(gooseFlags.format)
	if err != nil {
		return gooseops.Output{}, err
	}
	return gooseops.Output{JSONL: sf == formatter.StreamFormatJSONL, Out: cmd.OutOrStdout(), Err: cmd.ErrOrStderr()}, nil
}

func validateGooseRun() error {
	if gooseFlags.count < 0 {
		return errs.Invalidf("--count must not be negative")
	}
	if gooseFlags.duration < 0 {
		return errs.Invalidf("--duration must not be negative")
	}
	return nil
}

func openGooseTransport(iface string, opts goose.TransportOptions) (goose.Transport, error) {
	if strings.TrimSpace(iface) == "" {
		return nil, errs.Invalidf("--iface must name a network interface")
	}
	opts.Logger = cmdLogger()
	tr, err := gooseOpen(iface, opts)
	if err != nil {
		return nil, fmt.Errorf("open GOOSE transport on %s: %w", iface, err)
	}
	return tr, nil
}

func loadGooseControlBlock(cmd *cobra.Command) (*goose.ControlBlock, error) {
	cbs, warn, err := gooseops.LoadControlBlocks(gooseFlags.scl)
	if err != nil {
		return nil, err
	}
	cb, err := gooseops.SelectControlBlock(cbs, gooseFlags.gocb)
	if err != nil {
		if warn != nil {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "warning: %v\n", warn)
		}
		return nil, err
	}
	return cb, nil
}

func runGooseList(cmd *cobra.Command, _ []string) error {
	of, ok := formatter.ParseOutputFormat(gooseFlags.format)
	if !ok || (of != formatter.OutputFormatText && of != formatter.OutputFormatJSON) {
		return errs.Invalidf("invalid --format %q (supported: text, json)", gooseFlags.format)
	}
	cbs, warn, err := gooseops.LoadControlBlocks(gooseFlags.scl)
	if err != nil {
		return err
	}
	if warn != nil {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "warning: %v\n", warn)
	}
	views := make([]gooseops.ControlBlockView, 0, len(cbs))
	for i := range cbs {
		views = append(views, gooseops.ViewControlBlock(&cbs[i]))
	}
	out := cmd.OutOrStdout()
	if of == formatter.OutputFormatJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(views)
	}
	if len(views) == 0 {
		_, _ = fmt.Fprintln(out, "No GOOSE control blocks found")
		return nil
	}
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "GOCBREF\tAPPID\tDST\tVLAN\tCONFREV\tGOID\tDATSET\tMEMBERS")
	for _, v := range views {
		appID, dst, vlan := v.AppID, v.Dst, "-"
		if !v.Address {
			appID, dst = "-", "(no GSE address)"
		}
		if v.VLANID != nil {
			vlan = fmt.Sprintf("%d/%d", *v.VLANID, *v.VLANPrio)
		}
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%d\t%s\t%s\t%d\n", v.GocbRef, appID, dst, vlan, v.ConfRev, v.GoID, v.DatSet, len(v.Members))
	}
	return tw.Flush()
}

func runGooseSniff(cmd *cobra.Command, _ []string) error {
	if err := validateGooseRun(); err != nil {
		return err
	}
	out, err := gooseOutput(cmd)
	if err != nil {
		return err
	}
	var flt gooseops.Filter
	for _, s := range gooseFlags.appIDs {
		id, err := gooseops.ParseAppID(s)
		if err != nil {
			return err
		}
		flt.AppIDs = append(flt.AppIDs, id)
	}
	opts := goose.TransportOptions{Promiscuous: true}
	if gooseFlags.dst != "" {
		if flt.Dst, err = goose.ParseMAC(gooseFlags.dst); err != nil {
			return errs.Mark(fmt.Errorf("--dst: %w", err), errs.ErrInvalidInput)
		}
		opts = goose.TransportOptions{Multicast: []goose.MAC{flt.Dst}}
	}
	flt.GocbRef = gooseFlags.gocbRef
	tr, err := openGooseTransport(gooseFlags.iface, opts)
	if err != nil {
		return err
	}
	defer func() { _ = tr.Close() }()
	_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Sniffing GOOSE on %s (Ctrl+C to stop)\n", gooseFlags.iface)
	_, err = gooseops.Sniff(cmd.Context(), gooseops.SniffConfig{
		Transport: tr, Filter: flt, Count: gooseFlags.count, Duration: gooseFlags.duration, Output: out,
	})
	return err
}

func runGooseSubscribe(cmd *cobra.Command, _ []string) error {
	if err := validateGooseRun(); err != nil {
		return err
	}
	out, err := gooseOutput(cmd)
	if err != nil {
		return err
	}
	cb, err := loadGooseControlBlock(cmd)
	if err != nil {
		return err
	}
	sc := cb.SubscriptionConfig()
	if gooseFlags.bareDomain {
		sc.GocbRef = gooseops.BareRef(sc.GocbRef, cb.IEDName)
		sc.DatSet = gooseops.BareRef(sc.DatSet, cb.IEDName)
	}
	sc.OnlyStateChanges = gooseFlags.onlyStates
	sc.AcceptSimulation = gooseFlags.acceptSim
	opts := goose.TransportOptions{Promiscuous: true}
	if cb.HasAddress {
		opts = goose.TransportOptions{Multicast: []goose.MAC{cb.Dst}}
	}
	tr, err := openGooseTransport(gooseFlags.iface, opts)
	if err != nil {
		return err
	}
	defer func() { _ = tr.Close() }()
	_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Subscribing to %s on %s (Ctrl+C to stop)\n", sc.GocbRef, gooseFlags.iface)
	_, err = gooseops.Subscribe(cmd.Context(), gooseops.SubscribeConfig{
		Transport: tr, Subscription: sc, Count: gooseFlags.count, Duration: gooseFlags.duration, Output: out,
	})
	return err
}

func runGoosePublish(cmd *cobra.Command, _ []string) error {
	if err := goosePublishSafety.check(cmd); err != nil {
		return err
	}
	if gooseFlags.changes < 0 || gooseFlags.interval < 0 || gooseFlags.duration < 0 {
		return errs.Invalidf("--changes, --interval and --duration must not be negative")
	}
	sf, err := formatter.ParseStreamFormat(gooseFlags.format)
	if err != nil {
		return err
	}
	cb, err := loadGooseControlBlock(cmd)
	if err != nil {
		return err
	}
	pc, err := cb.PublisherConfig()
	if err != nil {
		return err
	}
	if err := gooseops.ApplyOverrides(pc.Values, cb.Members, gooseFlags.values); err != nil {
		return err
	}
	if gooseFlags.src != "" {
		if pc.Src, err = goose.ParseMAC(gooseFlags.src); err != nil {
			return errs.Mark(fmt.Errorf("--src: %w", err), errs.ErrInvalidInput)
		}
	}
	if gooseFlags.bareDomain {
		pc.GocbRef = gooseops.BareRef(pc.GocbRef, cb.IEDName)
		pc.DatSet = gooseops.BareRef(pc.DatSet, cb.IEDName)
	}
	pc.Test, pc.Simulation, pc.Logger = goosePublishSafety.marked, gooseFlags.simulation, cmdLogger()

	tr, err := openGooseTransport(gooseFlags.iface, goose.TransportOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tr.Close() }()
	diag := cmd.ErrOrStderr()
	goosePublishSafety.warnLive(diag)
	_, _ = fmt.Fprintf(diag, "Publishing %s on %s -> %s appid %s test=%t (Ctrl+C to stop)\n",
		pc.GocbRef, gooseFlags.iface, pc.Dst, gooseops.FormatAppID(pc.AppID), pc.Test)
	_, err = gooseops.Publish(cmd.Context(), gooseops.PublishConfig{
		Transport: tr, Publisher: pc, Changes: gooseFlags.changes, Interval: gooseFlags.interval,
		Duration: gooseFlags.duration,
		Output:   gooseops.Output{JSONL: sf == formatter.StreamFormatJSONL, Out: cmd.OutOrStdout(), Err: diag},
	})
	return err
}

// gocbItem maps "gcb1" to the MMS item "LLN0$GO$gcb1"; an item or a dotted
// form ("LLN0.GO.gcb1") is accepted as is.
func gocbItem(s string) string {
	s = strings.ReplaceAll(strings.TrimSpace(s), ".", "$")
	if strings.Contains(s, "$") {
		return s
	}
	return "LLN0$GO$" + s
}

// gocbView is the JSON form of a GoCB.
type gocbView struct {
	Reference string  `json:"reference"`
	GoEna     bool    `json:"go_ena"`
	GoID      string  `json:"go_id"`
	DatSet    string  `json:"dat_set"`
	ConfRev   uint32  `json:"conf_rev"`
	NdsCom    bool    `json:"nds_com"`
	Dst       string  `json:"dst"`
	Priority  uint8   `json:"vlan_priority"`
	VID       uint16  `json:"vlan_id"`
	AppID     string  `json:"app_id"`
	MinTimeMs *uint32 `json:"min_time_ms,omitempty"`
	MaxTimeMs *uint32 `json:"max_time_ms,omitempty"`
	FixedOffs bool    `json:"fixed_offs"`
}

func writeGoCB(w io.Writer, asJSON bool, v gocbView) error {
	if asJSON {
		return writeJSONIndent(w, v)
	}
	_, _ = fmt.Fprintf(w, "GoCB %s\n", v.Reference)
	_, _ = fmt.Fprintf(w, "  GoEna:      %t\n  GoID:       %s\n  DatSet:     %s\n  ConfRev:    %d\n  NdsCom:     %t\n",
		v.GoEna, v.GoID, v.DatSet, v.ConfRev, v.NdsCom)
	_, _ = fmt.Fprintf(w, "  DstAddress: %s appid %s vlan %d prio %d\n", v.Dst, v.AppID, v.VID, v.Priority)
	if v.MinTimeMs != nil {
		_, _ = fmt.Fprintf(w, "  MinTime:    %dms\n  MaxTime:    %dms\n", *v.MinTimeMs, *v.MaxTimeMs)
	}
	_, _ = fmt.Fprintf(w, "  FixedOffs:  %t\n", v.FixedOffs)
	return nil
}

func writeJSONIndent(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func gocbFormat() (bool, error) {
	of, ok := formatter.ParseOutputFormat(gooseGocbFlags.format)
	if !ok || (of != formatter.OutputFormatText && of != formatter.OutputFormatJSON) {
		return false, errs.Invalidf("invalid --format %q (supported: text, json)", gooseGocbFlags.format)
	}
	return of == formatter.OutputFormatJSON, nil
}

func readGoCB(cmd *cobra.Command, s *clientSession) (gocbView, error) {
	g, ok := s.Conn().(gocbConn)
	if !ok {
		return gocbView{}, errs.Newf(errs.ErrUnsupported, "connection does not support GoCB access")
	}
	cb, err := g.GetGoCB(cmd.Context(), gooseGocbFlags.ld, gocbItem(gooseGocbFlags.gocb))
	if err != nil {
		return gocbView{}, fmt.Errorf("get GoCB %s/%s: %w", gooseGocbFlags.ld, gocbItem(gooseGocbFlags.gocb), err)
	}
	v := gocbView{
		Reference: cb.Reference, GoEna: cb.GoEna, GoID: cb.GoID, DatSet: cb.DatSet, ConfRev: cb.ConfRev,
		NdsCom: cb.NdsCom, Dst: cb.DstAddress.Addr.String(), Priority: cb.DstAddress.Priority,
		VID: cb.DstAddress.VID, AppID: gooseops.FormatAppID(cb.DstAddress.AppID), FixedOffs: cb.FixedOffs,
	}
	if cb.HasTimes {
		minT, maxT := cb.MinTime, cb.MaxTime
		v.MinTimeMs, v.MaxTimeMs = &minT, &maxT
	}
	return v, nil
}

func runGooseGocbGet(cmd *cobra.Command, _ []string) error {
	asJSON, err := gocbFormat()
	if err != nil {
		return err
	}
	s, err := openClientSession(cmd, clientSessionOptions{})
	if err != nil {
		return err
	}
	defer s.Close()
	v, err := readGoCB(cmd, s)
	if err != nil {
		return err
	}
	return writeGoCB(cmd.OutOrStdout(), asJSON, v)
}

func runGooseGocbSet(cmd *cobra.Command, _ []string) error {
	asJSON, err := gocbFormat()
	if err != nil {
		return err
	}
	var upd goCBUpdate
	if gooseGocbFlags.enable || gooseGocbFlags.disable {
		on := gooseGocbFlags.enable
		upd.GoEna = &on
	}
	if cmd.Flags().Changed("go-id") {
		upd.GoID = &gooseGocbFlags.goID
	}
	if cmd.Flags().Changed("dat-set") {
		upd.DatSet = &gooseGocbFlags.datSet
	}
	s, err := openClientSession(cmd, clientSessionOptions{})
	if err != nil {
		return err
	}
	defer s.Close()
	g, ok := s.Conn().(gocbConn)
	if !ok {
		return errs.Newf(errs.ErrUnsupported, "connection does not support GoCB access")
	}
	item := gocbItem(gooseGocbFlags.gocb)
	if err := g.SetGoCB(cmd.Context(), gooseGocbFlags.ld, item, upd); err != nil {
		return errs.Mark(fmt.Errorf("set GoCB %s/%s: %w", gooseGocbFlags.ld, item, err), errs.ErrRejected)
	}
	v, err := readGoCB(cmd, s)
	if err != nil {
		return err
	}
	if err := writeGoCB(cmd.OutOrStdout(), asJSON, v); err != nil {
		return err
	}
	return verifyEna("GoEna", upd.GoEna, v.GoEna)
}

// verifyEna returns ErrVerifyMismatch (exit 7) when a requested GoEna/SvEna
// write reads back with a different value: the server accepted the write but
// did not start or stop publishing. requested is nil when no enable or
// disable was asked for.
func verifyEna(attr string, requested *bool, readBack bool) error {
	if requested == nil || *requested == readBack {
		return nil
	}
	return errs.Newf(errs.ErrVerifyMismatch, "verification mismatch: %s reads back %t after writing %t", attr, readBack, *requested)
}

type (
	gocbConn   = service.GoCBConnection
	goCBUpdate = iec61850.GoCBUpdate
)
