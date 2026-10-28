// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	iec61850 "github.com/otfabric/go-iec61850"
	"github.com/otfabric/go-iec61850/sv"
	"github.com/spf13/cobra"

	"github.com/otfabric/iec61850ctl/pkg/errs"
	"github.com/otfabric/iec61850ctl/pkg/formatter"
	"github.com/otfabric/iec61850ctl/pkg/gooseops"
	"github.com/otfabric/iec61850ctl/pkg/service"
	"github.com/otfabric/iec61850ctl/pkg/svops"
)

// svOpen opens a raw Ethernet SV transport; tests substitute an in-memory
// hub port.
var svOpen = sv.Open

var svFlags struct {
	scl           string
	iface         string
	svcb          string
	format        string
	count         int
	duration      time.Duration
	appIDs        []string
	dst           string
	svID          string
	layout        string
	sampleEvery   int
	statsInterval time.Duration
	timeout       time.Duration
	acceptSim     bool
	bareDomain    bool

	values    []string
	waveforms []string
	src       string
	smpSynch  uint8
	frequency float64
}

var svCmd = &cobra.Command{
	Use:   "sv",
	Short: "Sampled Values: list control blocks, sniff, subscribe, publish, read/write MSVCBs",
	Long: `Sampled Values (IEC 61850-9-2, including the UCA 9-2LE profile) over raw
Ethernet.

sniff, subscribe and publish open a raw socket on --iface: AF_PACKET on Linux
(needs CAP_NET_RAW) or /dev/bpf* on macOS/BSD (needs access to /dev/bpf*).
list reads an SCL file offline; svcb get/set use MMS (--host).

publish injects a sample stream into the process bus. It needs an explicit
--iface and --yes, and frames carry the simulation bit unless
--simulation=false is combined with --live.`,
}

var svListCmd = &cobra.Command{
	Use:   "list",
	Short: "List SV control blocks (SampledValueControl + SMV address + dataset) from SCL",
	Example: `  iec61850ctl sv list --scl station.scd
  iec61850ctl sv list --scl merging-unit.cid --format json`,
	RunE: runSVList,
}

var svSniffCmd = &cobra.Command{
	Use:   "sniff",
	Short: "Decode and print SV ASDUs seen on an interface (no supervision)",
	Example: `  iec61850ctl sv sniff --iface eth0 --count 4
  iec61850ctl sv sniff --iface eth0 --appid 0x4000 --layout 9-2LE --sample-every 4000
  iec61850ctl sv sniff --iface eth0 --scl merging-unit.cid --svcb MSVCB01 --count 10 --format jsonl`,
	RunE: runSVSniff,
}

var svSubscribeCmd = &cobra.Command{
	Use:   "subscribe",
	Short: "Subscribe to one SVCB from SCL with validation, supervision and live statistics",
	Example: `  iec61850ctl sv subscribe --iface eth0 --scl merging-unit.cid --svcb MSVCB01
  iec61850ctl sv subscribe --iface eth0 --scl merging-unit.cid --svcb MSVCB01 --sample-every 4000 --duration 10s
  iec61850ctl sv subscribe --iface eth0 --scl merging-unit.cid --svcb MSVCB02 --count 1000 --duration 5s --format jsonl`,
	RunE: runSVSubscribe,
}

var svPublishCmd = &cobra.Command{
	Use:   "publish",
	Short: "Publish an SVCB from SCL (simulation frames unless --simulation=false --live)",
	Example: `  iec61850ctl sv publish --iface eth0 --scl merging-unit.cid --svcb MSVCB01 --yes --duration 10s
  iec61850ctl sv publish --iface eth0 --scl merging-unit.cid --svcb MSVCB01 --yes \
      --waveform IA=100 --waveform IB=100,50,-120 --waveform IC=100,50,120 --waveform VA=230
  iec61850ctl sv publish --iface eth0 --scl merging-unit.cid --svcb MSVCB02 --yes --value 0=12.5 --value 1=test`,
	RunE: runSVPublish,
}

var svSvcbCmd = &cobra.Command{
	Use:   "svcb",
	Short: "Read or write a multicast SV control block (MSVCB) over MMS",
}

// svPublishSafety guards sv publish; frames are marked with the simulation bit.
var svPublishSafety = publishSafety{protocol: "SV", iface: &svFlags.iface, marker: "simulation"}

var svSvcbFlags struct {
	ld      string
	svcb    string
	format  string
	enable  bool
	disable bool
	msvID   string
	datSet  string
	smpRate uint16
	optFlds string
	smpMod  string
}

var svSvcbGetCmd = &cobra.Command{
	Use:     "get",
	Short:   "Read an MSVCB (SvEna, MsvID, DatSet, ConfRev, SmpRate, OptFlds, SmpMod, DstAddress, noASDU)",
	Example: `  iec61850ctl sv svcb get --host 192.0.2.10 --ld LD0 --svcb MSVCB01 --format json`,
	RunE:    runSVSvcbGet,
}

var svSvcbSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Write MSVCB attributes (SvEna written last)",
	Example: `  iec61850ctl sv svcb set --host 192.0.2.10 --ld LD0 --svcb MSVCB01 --disable
  iec61850ctl sv svcb set --host 192.0.2.10 --ld LD0 --svcb MSVCB01 --opt-flds refresh-time,sample-rate --enable`,
	RunE: runSVSvcbSet,
}

func init() {
	f := svListCmd.Flags()
	f.StringVar(&svFlags.scl, "scl", "", "SCL file (SCD/CID/ICD) (required)")
	f.StringVar(&svFlags.format, "format", "text", "Output format: text, json")
	_ = svListCmd.MarkFlagRequired("scl")

	f = svSniffCmd.Flags()
	f.StringVar(&svFlags.iface, "iface", "", "network interface to capture on (required)")
	f.StringSliceVar(&svFlags.appIDs, "appid", nil, "only frames with these APPIDs (hex, repeatable)")
	f.StringVar(&svFlags.dst, "dst", "", "only frames to this destination MAC (joins the multicast group)")
	f.StringVar(&svFlags.svID, "sv-id", "", "only ASDUs with this svID")
	f.StringVar(&svFlags.scl, "scl", "", "SCL file: with --svcb, filter on and decode that control block's stream")
	f.StringVar(&svFlags.svcb, "svcb", "", "control block in --scl: name, LD/name, IED/LD/name or SvCBRef")
	f.StringVar(&svFlags.layout, "layout", "", "decode samples with this layout: 9-2LE or a list such as FLOAT32,Quality (default: hex)")
	f.IntVar(&svFlags.sampleEvery, "sample-every", 1, "print every Nth matching ASDU")
	addSVRunFlags(svSniffCmd, "stop after this many matching ASDUs (0 = unlimited)")
	_ = svSniffCmd.MarkFlagRequired("iface")

	f = svSubscribeCmd.Flags()
	f.StringVar(&svFlags.iface, "iface", "", "network interface to receive on (required)")
	f.StringVar(&svFlags.scl, "scl", "", "SCL file describing the publisher (required)")
	f.StringVar(&svFlags.svcb, "svcb", "", "control block: name, LD/name, IED/LD/name or SvCBRef (required)")
	f.BoolVar(&svFlags.acceptSim, "accept-simulation", false, "accept frames with the simulation bit set")
	f.BoolVar(&svFlags.bareDomain, "bare-domain", false, "expect datSet with the bare LD as MMS domain (LD/LLN0$ds) instead of IED+LD")
	f.IntVar(&svFlags.sampleEvery, "sample-every", 0, "print every Nth accepted ASDU (0 = none)")
	f.DurationVar(&svFlags.statsInterval, "stats-interval", time.Second, "print stream statistics this often (0 = only at the end)")
	f.DurationVar(&svFlags.timeout, "timeout", sv.DefaultTimeout, "supervision timeout: a silent stream reports a timeout after this long")
	addSVRunFlags(svSubscribeCmd, "stop after this many accepted ASDUs (0 = unlimited)")
	for _, n := range []string{"iface", "scl", "svcb"} {
		_ = svSubscribeCmd.MarkFlagRequired(n)
	}

	f = svPublishCmd.Flags()
	f.StringVar(&svFlags.iface, "iface", "", "network interface to publish on (required, never defaulted)")
	f.StringVar(&svFlags.scl, "scl", "", "SCL file with the control block (required)")
	f.StringVar(&svFlags.svcb, "svcb", "", "control block: name, LD/name, IED/LD/name or SvCBRef (required)")
	f.StringArrayVar(&svFlags.values, "value", nil, "sample leaf as INDEX=VALUE or NAME=VALUE: member ref, REF#N, or 9-2LE channel IA..VN / qIA..qVN (repeatable; others are zero/good)")
	f.StringArrayVar(&svFlags.waveforms, "waveform", nil, "9-2LE sine as CHANNEL=PEAK[,HZ[,PHASE_DEG]] in A (IA..IN) or V (VA..VN) (repeatable)")
	svPublishSafety.register(svPublishCmd, "set the simulation bit in every frame")
	f.StringVar(&svFlags.src, "src", "", "source MAC (default: the interface address)")
	f.BoolVar(&svFlags.bareDomain, "bare-domain", false, "send datSet with the bare LD as MMS domain")
	f.Uint8Var(&svFlags.smpSynch, "smp-synch", 0, "smpSynch to send: 0 none, 1 local, 2 global, 5-254 PTP grandmaster")
	f.Float64Var(&svFlags.frequency, "frequency", 50, "nominal frequency in Hz (SmpPerPeriod rate and default waveform frequency)")
	f.DurationVar(&svFlags.duration, "duration", 0, "stop after this long (0 = until Ctrl+C)")
	f.StringVar(&svFlags.format, "format", "text", "Output format: text, jsonl")
	for _, n := range []string{"iface", "scl", "svcb"} {
		_ = svPublishCmd.MarkFlagRequired(n)
	}

	for _, c := range []*cobra.Command{svSvcbGetCmd, svSvcbSetCmd} {
		c.Flags().StringVar(&svSvcbFlags.ld, "ld", "", "logical device (required)")
		c.Flags().StringVar(&svSvcbFlags.svcb, "svcb", "", "MSVCB name (MSVCB01) or item (LLN0$MS$MSVCB01) (required)")
		c.Flags().StringVar(&svSvcbFlags.format, "format", "text", "Output format: text, json")
		_ = c.MarkFlagRequired("ld")
		_ = c.MarkFlagRequired("svcb")
	}
	sf := svSvcbSetCmd.Flags()
	sf.BoolVar(&svSvcbFlags.enable, "enable", false, "set SvEna=true")
	sf.BoolVar(&svSvcbFlags.disable, "disable", false, "set SvEna=false")
	sf.StringVar(&svSvcbFlags.msvID, "msv-id", "", "new MsvID (only while disabled)")
	sf.StringVar(&svSvcbFlags.datSet, "dat-set", "", "new DatSet reference (only while disabled)")
	sf.Uint16Var(&svSvcbFlags.smpRate, "smp-rate", 0, "new SmpRate (only while disabled)")
	sf.StringVar(&svSvcbFlags.optFlds, "opt-flds", "", "new OptFlds: comma list of refresh-time, sample-synchronized, sample-rate, data-set, security, or none (only while disabled)")
	sf.StringVar(&svSvcbFlags.smpMod, "smp-mod", "", "new SmpMod: SmpPerPeriod, SmpPerSec, SecPerSmp (only while disabled)")
	svSvcbSetCmd.MarkFlagsMutuallyExclusive("enable", "disable")
	svSvcbSetCmd.MarkFlagsOneRequired("enable", "disable", "msv-id", "dat-set", "smp-rate", "opt-flds", "smp-mod")

	svSvcbCmd.AddCommand(svSvcbGetCmd, svSvcbSetCmd)
	svCmd.AddCommand(svListCmd, svSniffCmd, svSubscribeCmd, svPublishCmd, svSvcbCmd)
	rootCmd.AddCommand(svCmd)
}

func addSVRunFlags(c *cobra.Command, countUsage string) {
	c.Flags().IntVar(&svFlags.count, "count", 0, countUsage)
	c.Flags().DurationVar(&svFlags.duration, "duration", 0, "stop after this long (0 = until Ctrl+C); with --count, exit 4 if not reached")
	c.Flags().StringVar(&svFlags.format, "format", "text", "Output format: text, jsonl")
}

func svOutput(cmd *cobra.Command) (svops.Output, error) {
	sf, err := formatter.ParseStreamFormat(svFlags.format)
	if err != nil {
		return svops.Output{}, err
	}
	return svops.Output{JSONL: sf == formatter.StreamFormatJSONL, Out: cmd.OutOrStdout(), Err: cmd.ErrOrStderr()}, nil
}

func validateSVRun() error {
	switch {
	case svFlags.count < 0:
		return errs.Invalidf("--count must not be negative")
	case svFlags.duration < 0:
		return errs.Invalidf("--duration must not be negative")
	case svFlags.sampleEvery < 0:
		return errs.Invalidf("--sample-every must not be negative")
	case svFlags.statsInterval < 0:
		return errs.Invalidf("--stats-interval must not be negative")
	case svFlags.timeout < 0:
		return errs.Invalidf("--timeout must not be negative")
	}
	return nil
}

func openSVTransport(iface string, opts sv.TransportOptions) (sv.Transport, error) {
	if strings.TrimSpace(iface) == "" {
		return nil, errs.Invalidf("--iface must name a network interface")
	}
	opts.Logger = cmdLogger()
	tr, err := svOpen(iface, opts)
	if err != nil {
		return nil, fmt.Errorf("open SV transport on %s: %w", iface, err)
	}
	return tr, nil
}

func loadSVControlBlock(cmd *cobra.Command) (*sv.ControlBlock, error) {
	cbs, warn, err := svops.LoadControlBlocks(svFlags.scl)
	if err != nil {
		return nil, err
	}
	cb, err := svops.SelectControlBlock(cbs, svFlags.svcb)
	if err != nil {
		if warn != nil {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "warning: %v\n", warn)
		}
		return nil, err
	}
	return cb, nil
}

func runSVList(cmd *cobra.Command, _ []string) error {
	of, ok := formatter.ParseOutputFormat(svFlags.format)
	if !ok || (of != formatter.OutputFormatText && of != formatter.OutputFormatJSON) {
		return errs.Invalidf("invalid --format %q (supported: text, json)", svFlags.format)
	}
	cbs, warn, err := svops.LoadControlBlocks(svFlags.scl)
	if err != nil {
		return err
	}
	if warn != nil {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "warning: %v\n", warn)
	}
	views := make([]svops.ControlBlockView, 0, len(cbs))
	for i := range cbs {
		views = append(views, svops.ViewControlBlock(&cbs[i]))
	}
	out := cmd.OutOrStdout()
	if of == formatter.OutputFormatJSON {
		return writeJSONIndent(out, views)
	}
	if len(views) == 0 {
		_, _ = fmt.Fprintln(out, "No SV control blocks found")
		return nil
	}
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "SVCBREF\tAPPID\tDST\tVLAN\tSVID\tSMPRATE\tNOASDU\tCONFREV\tPROFILE\tDATSET\tOCTETS")
	for _, v := range views {
		appID, dst, vlan := v.AppID, v.Dst, "-"
		if !v.Address {
			appID, dst = "-", "(no SMV address)"
		}
		if v.VLANID != nil {
			vlan = fmt.Sprintf("%d/%d", *v.VLANID, *v.VLANPrio)
		}
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%d %s\t%d\t%d\t%s\t%s\t%d\n", v.SvCBRef, appID, dst, vlan, v.SvID,
			v.SmpRate, v.SmpMod, v.NoASDU, v.ConfRev, v.Profile, v.DatSet, v.SampleOctets)
	}
	return tw.Flush()
}

func runSVSniff(cmd *cobra.Command, _ []string) error {
	if err := validateSVRun(); err != nil {
		return err
	}
	out, err := svOutput(cmd)
	if err != nil {
		return err
	}
	var (
		flt    svops.Filter
		layout sv.Layout
	)
	if svFlags.svcb != "" || svFlags.scl != "" {
		if svFlags.svcb == "" || svFlags.scl == "" {
			return errs.Invalidf("--scl and --svcb go together")
		}
		cb, err := loadSVControlBlock(cmd)
		if err != nil {
			return err
		}
		layout, flt.SvID = cb.Layout, cb.SvID
		if cb.HasAddress {
			flt.Dst, flt.AppIDs = cb.Dst, []uint16{cb.AppID}
		}
	}
	if svFlags.layout != "" {
		if layout, err = svops.ParseLayout(svFlags.layout); err != nil {
			return err
		}
	}
	if len(svFlags.appIDs) > 0 {
		flt.AppIDs = nil
		for _, s := range svFlags.appIDs {
			id, err := gooseops.ParseAppID(s)
			if err != nil {
				return err
			}
			flt.AppIDs = append(flt.AppIDs, id)
		}
	}
	if svFlags.dst != "" {
		if flt.Dst, err = sv.ParseMAC(svFlags.dst); err != nil {
			return errs.Mark(fmt.Errorf("--dst: %w", err), errs.ErrInvalidInput)
		}
	}
	if svFlags.svID != "" {
		flt.SvID = svFlags.svID
	}
	opts := sv.TransportOptions{Promiscuous: true}
	if !flt.Dst.IsZero() {
		opts = sv.TransportOptions{Multicast: []sv.MAC{flt.Dst}}
	}
	tr, err := openSVTransport(svFlags.iface, opts)
	if err != nil {
		return err
	}
	defer func() { _ = tr.Close() }()
	_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Sniffing SV on %s (Ctrl+C to stop)\n", svFlags.iface)
	_, err = svops.Sniff(cmd.Context(), svops.SniffConfig{
		Transport: tr, Filter: flt, Layout: layout, SampleEvery: svFlags.sampleEvery,
		Count: svFlags.count, Duration: svFlags.duration, Output: out,
	})
	return err
}

func runSVSubscribe(cmd *cobra.Command, _ []string) error {
	if err := validateSVRun(); err != nil {
		return err
	}
	out, err := svOutput(cmd)
	if err != nil {
		return err
	}
	cb, err := loadSVControlBlock(cmd)
	if err != nil {
		return err
	}
	sc := cb.SubscriptionConfig()
	sc.Timeout = svFlags.timeout
	if svFlags.bareDomain {
		sc.DatSet = gooseops.BareRef(sc.DatSet, cb.IEDName)
	}
	opts := sv.TransportOptions{Promiscuous: true}
	if cb.HasAddress {
		opts = sv.TransportOptions{Multicast: []sv.MAC{cb.Dst}}
	}
	tr, err := openSVTransport(svFlags.iface, opts)
	if err != nil {
		return err
	}
	defer func() { _ = tr.Close() }()
	_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Subscribing to %s (svID %s) on %s (Ctrl+C to stop)\n", cb.SvCBRef, cb.SvID, svFlags.iface)
	_, err = svops.Subscribe(cmd.Context(), svops.SubscribeConfig{
		Transport: tr, Subscription: sc, AcceptSimulation: svFlags.acceptSim,
		SampleEvery: svFlags.sampleEvery, StatsInterval: svFlags.statsInterval,
		Count: svFlags.count, Duration: svFlags.duration, Output: out,
	})
	return err
}

func runSVPublish(cmd *cobra.Command, _ []string) error {
	if err := svPublishSafety.check(cmd); err != nil {
		return err
	}
	if svFlags.duration < 0 {
		return errs.Invalidf("--duration must not be negative")
	}
	if svFlags.frequency <= 0 {
		return errs.Invalidf("--frequency must be positive")
	}
	sf, err := formatter.ParseStreamFormat(svFlags.format)
	if err != nil {
		return err
	}
	cb, err := loadSVControlBlock(cmd)
	if err != nil {
		return err
	}
	pc, err := cb.PublisherConfig()
	if err != nil {
		return err
	}
	values := svops.ZeroValues(cb.Layout)
	if err := svops.ApplyOverrides(cb, values, svFlags.values); err != nil {
		return err
	}
	if pc.Sample, err = cb.Layout.Encode(nil, values); err != nil {
		return errs.Mark(fmt.Errorf("encode sample: %w", err), errs.ErrInvalidInput)
	}
	pc.NominalFrequency = svFlags.frequency
	if len(svFlags.waveforms) > 0 {
		if !svops.IsLE(cb.Layout) {
			return errs.Invalidf("--waveform needs a 9-2LE control block; %s has layout %s", cb.Name, cb.Layout)
		}
		waves := make([]svops.Waveform, 0, len(svFlags.waveforms))
		for _, spec := range svFlags.waveforms {
			w, err := svops.ParseWaveform(spec, svFlags.frequency)
			if err != nil {
				return err
			}
			waves = append(waves, w)
		}
		rate := sv.SamplesPerSecond(pc.SmpRate, pc.SmpMod, svFlags.frequency)
		if pc.Source, err = svops.WaveformSource(values, waves, rate); err != nil {
			return err
		}
	}
	if svFlags.src != "" {
		if pc.Src, err = sv.ParseMAC(svFlags.src); err != nil {
			return errs.Mark(fmt.Errorf("--src: %w", err), errs.ErrInvalidInput)
		}
	}
	if svFlags.bareDomain {
		pc.DatSet = gooseops.BareRef(pc.DatSet, cb.IEDName)
	}
	pc.Simulation, pc.SmpSynch, pc.Logger = svPublishSafety.marked, svFlags.smpSynch, cmdLogger()

	tr, err := openSVTransport(svFlags.iface, sv.TransportOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tr.Close() }()
	diag := cmd.ErrOrStderr()
	svPublishSafety.warnLive(diag)
	_, _ = fmt.Fprintf(diag, "Publishing %s (svID %s) on %s -> %s appid %s simulation=%t (Ctrl+C to stop)\n",
		cb.SvCBRef, pc.SvID, svFlags.iface, pc.Dst, gooseops.FormatAppID(pc.AppID), pc.Simulation)
	_, err = svops.Publish(cmd.Context(), svops.PublishConfig{
		Transport: tr, Publisher: pc, Duration: svFlags.duration,
		Output: svops.Output{JSONL: sf == formatter.StreamFormatJSONL, Out: cmd.OutOrStdout(), Err: diag},
	})
	return err
}

// svcbItem maps "MSVCB01" to the MMS item "LLN0$MS$MSVCB01"; an item or a
// dotted form ("LLN0.MS.MSVCB01") is accepted as is.
func svcbItem(s string) string {
	s = strings.ReplaceAll(strings.TrimSpace(s), ".", "$")
	if strings.Contains(s, "$") {
		return s
	}
	return "LLN0$MS$" + s
}

// svcbView is the JSON form of an MSVCB.
type svcbView struct {
	Reference string   `json:"reference"`
	SvEna     bool     `json:"sv_ena"`
	MsvID     string   `json:"msv_id"`
	DatSet    string   `json:"dat_set"`
	ConfRev   uint32   `json:"conf_rev"`
	SmpRate   uint16   `json:"smp_rate"`
	SmpMod    string   `json:"smp_mod"`
	OptFlds   []string `json:"opt_flds"`
	Dst       string   `json:"dst"`
	Priority  uint8    `json:"vlan_priority"`
	VID       uint16   `json:"vlan_id"`
	AppID     string   `json:"app_id"`
	NoASDU    *uint16  `json:"no_asdu,omitempty"`
}

func writeSVCB(w io.Writer, asJSON bool, v svcbView) error {
	if asJSON {
		return writeJSONIndent(w, v)
	}
	opt := strings.Join(v.OptFlds, ",")
	if opt == "" {
		opt = "none"
	}
	_, _ = fmt.Fprintf(w, "MSVCB %s\n", v.Reference)
	_, _ = fmt.Fprintf(w, "  SvEna:      %t\n  MsvID:      %s\n  DatSet:     %s\n  ConfRev:    %d\n  SmpRate:    %d %s\n  OptFlds:    %s\n",
		v.SvEna, v.MsvID, v.DatSet, v.ConfRev, v.SmpRate, v.SmpMod, opt)
	_, _ = fmt.Fprintf(w, "  DstAddress: %s appid %s vlan %d prio %d\n", v.Dst, v.AppID, v.VID, v.Priority)
	if v.NoASDU != nil {
		_, _ = fmt.Fprintf(w, "  noASDU:     %d\n", *v.NoASDU)
	}
	return nil
}

func svcbFormat() (bool, error) {
	of, ok := formatter.ParseOutputFormat(svSvcbFlags.format)
	if !ok || (of != formatter.OutputFormatText && of != formatter.OutputFormatJSON) {
		return false, errs.Invalidf("invalid --format %q (supported: text, json)", svSvcbFlags.format)
	}
	return of == formatter.OutputFormatJSON, nil
}

func svcbConnOf(s *clientSession) (svcbConn, error) {
	m, ok := s.Conn().(svcbConn)
	if !ok {
		return nil, errs.Newf(errs.ErrUnsupported, "connection does not support MSVCB access")
	}
	return m, nil
}

func readSVCB(cmd *cobra.Command, m svcbConn) (svcbView, error) {
	item := svcbItem(svSvcbFlags.svcb)
	cb, err := m.GetMSVCB(cmd.Context(), svSvcbFlags.ld, item)
	if err != nil {
		return svcbView{}, fmt.Errorf("get MSVCB %s/%s: %w", svSvcbFlags.ld, item, err)
	}
	v := svcbView{
		Reference: cb.Reference, SvEna: cb.SvEna, MsvID: cb.MsvID, DatSet: cb.DatSet, ConfRev: cb.ConfRev,
		SmpRate: cb.SmpRate, SmpMod: cb.SmpMod.String(), OptFlds: svops.OptFldsNames(cb.OptFlds),
		Dst: cb.DstAddress.Addr.String(), Priority: cb.DstAddress.Priority, VID: cb.DstAddress.VID,
		AppID: gooseops.FormatAppID(cb.DstAddress.AppID),
	}
	if cb.NoASDU != 0 {
		n := cb.NoASDU
		v.NoASDU = &n
	}
	return v, nil
}

func runSVSvcbGet(cmd *cobra.Command, _ []string) error {
	asJSON, err := svcbFormat()
	if err != nil {
		return err
	}
	s, err := openClientSession(cmd, clientSessionOptions{})
	if err != nil {
		return err
	}
	defer s.Close()
	m, err := svcbConnOf(s)
	if err != nil {
		return err
	}
	v, err := readSVCB(cmd, m)
	if err != nil {
		return err
	}
	return writeSVCB(cmd.OutOrStdout(), asJSON, v)
}

func runSVSvcbSet(cmd *cobra.Command, _ []string) error {
	asJSON, err := svcbFormat()
	if err != nil {
		return err
	}
	var upd msvcbUpdate
	if svSvcbFlags.enable || svSvcbFlags.disable {
		on := svSvcbFlags.enable
		upd.SvEna = &on
	}
	fl := cmd.Flags()
	if fl.Changed("msv-id") {
		upd.MsvID = &svSvcbFlags.msvID
	}
	if fl.Changed("dat-set") {
		upd.DatSet = &svSvcbFlags.datSet
	}
	if fl.Changed("smp-rate") {
		upd.SmpRate = &svSvcbFlags.smpRate
	}
	if fl.Changed("opt-flds") {
		o, err := svops.ParseOptFlds(svSvcbFlags.optFlds)
		if err != nil {
			return err
		}
		upd.OptFlds = &o
	}
	if fl.Changed("smp-mod") {
		m, err := svops.ParseSmpMod(svSvcbFlags.smpMod)
		if err != nil {
			return err
		}
		upd.SmpMod = &m
	}
	s, err := openClientSession(cmd, clientSessionOptions{})
	if err != nil {
		return err
	}
	defer s.Close()
	m, err := svcbConnOf(s)
	if err != nil {
		return err
	}
	item := svcbItem(svSvcbFlags.svcb)
	if err := m.SetMSVCB(cmd.Context(), svSvcbFlags.ld, item, upd); err != nil {
		return errs.Mark(fmt.Errorf("set MSVCB %s/%s: %w", svSvcbFlags.ld, item, err), errs.ErrRejected)
	}
	v, err := readSVCB(cmd, m)
	if err != nil {
		return err
	}
	return writeSVCB(cmd.OutOrStdout(), asJSON, v)
}

type (
	svcbConn    = service.MSVCBConnection
	msvcbUpdate = iec61850.MSVCBUpdate
)
