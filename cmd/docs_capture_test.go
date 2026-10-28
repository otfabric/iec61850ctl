// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/otfabric/go-iec61850/goose"
	"github.com/otfabric/go-iec61850/sv"

	"github.com/otfabric/iec61850ctl/pkg/stack/server"
	"github.com/otfabric/iec61850ctl/pkg/svops"
)

// TestCaptureDocExamples records the example sessions of docs/GOOSE.md and
// docs/SV.md: real runs of the CLI against the in-memory GOOSE/SV hub and
// an in-process server for the iec61850-interop fixtures. It only runs when
// IEC61850CTL_DOCS_CAPTURE names an output directory:
//
//	IEC61850CTL_DOCS_CAPTURE=/tmp/docs go test ./cmd -run TestCaptureDocExamples
//
// Each example is written to <dir>/<name>.txt as the command line, the
// exit code, stdout and stderr.
func TestCaptureDocExamples(t *testing.T) {
	dir := os.Getenv("IEC61850CTL_DOCS_CAPTURE")
	if dir == "" {
		t.Skip("IEC61850CTL_DOCS_CAPTURE unset")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	capture := func(name string, args ...string) {
		t.Helper()
		r := runCLI(t, args...)
		var b strings.Builder
		fmt.Fprintf(&b, "$ iec61850ctl %s\n# exit %d\n--- stdout\n%s--- stderr\n%s", shellJoin(args), r.code, r.stdout, r.stderr)
		if err := os.WriteFile(filepath.Join(dir, name+".txt"), []byte(b.String()), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// Fixture paths as a user would type them.
	t.Chdir(repoRoot())
	gcid, scid := "pkg/gooseops/testdata/interop-goose.cid", "pkg/svops/testdata/interop-sv.cid"

	// GOOSE: the server publishes its GoCBs (bare-LD references) on the hub.
	ghub := goose.NewHub()
	withHub(t, ghub)
	gport := ghub.NewPort(goose.MAC{0x02, 0, 0, 0, 0, 0x01})
	gsrv, err := server.Start(context.Background(), server.RunConfig{SclPath: gcid, Host: "127.0.0.1", Port: 10102, GOOSE: gport})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = gsrv.Close(); _ = gport.Close() })
	mms := []string{"--host", "127.0.0.1", "--port", "10102"}

	capture("goose_list", "goose", "list", "--scl", gcid)
	capture("goose_list_json", "goose", "list", "--scl", gcid, "--format", "json")
	capture("goose_sniff", "goose", "sniff", "--iface", "eth0", "--appid", "0x1002", "--count", "3")
	capture("goose_sniff_jsonl", "goose", "sniff", "--iface", "eth0", "--gocb-ref", "InteropLD/LLN0$GO$gcbBool", "--count", "1", "--format", "jsonl")
	capture("goose_subscribe", "goose", "subscribe", "--iface", "eth0", "--scl", gcid, "--gocb", "gcbBool", "--bare-domain", "--count", "3")
	capture("goose_subscribe_jsonl", "goose", "subscribe", "--iface", "eth0", "--scl", gcid, "--gocb", "gcbBool", "--bare-domain", "--only-changes", "--count", "1", "--format", "jsonl")
	capture("goose_subscribe_no_bare_domain", "goose", "subscribe", "--iface", "eth0", "--scl", gcid, "--gocb", "gcbBool", "--count", "1", "--duration", "2s")
	capture("goose_publish_no_yes", "goose", "publish", "--iface", "eth0", "--scl", gcid, "--gocb", "gcbBool")
	capture("goose_publish_no_live", "goose", "publish", "--iface", "eth0", "--scl", gcid, "--gocb", "gcbBool", "--yes", "--test=false")
	capture("goose_publish", "goose", "publish", "--iface", "eth0", "--scl", gcid, "--gocb", "gcbBool", "--yes",
		"--value", "0=true", "--changes", "2", "--interval", "500ms", "--duration", "2s")
	capture("goose_publish_live", "goose", "publish", "--iface", "eth0", "--scl", gcid, "--gocb", "gcbBool", "--yes",
		"--test=false", "--live", "--duration", "1s", "--format", "jsonl")
	capture("goose_gocb_get", append(mms, "goose", "gocb", "get", "--ld", "InteropLD", "--gocb", "gcbBool")...)
	capture("goose_gocb_get_json", append(mms, "goose", "gocb", "get", "--ld", "InteropLD", "--gocb", "gcbBool", "--format", "json")...)
	capture("goose_gocb_set_rejected", append(mms, "goose", "gocb", "set", "--ld", "InteropLD", "--gocb", "gcbBool", "--go-id", "renamed")...)
	capture("goose_gocb_set_disable", append(mms, "goose", "gocb", "set", "--ld", "InteropLD", "--gocb", "gcbBool", "--disable")...)
	capture("goose_gocb_set_enable", append(mms, "goose", "gocb", "set", "--ld", "InteropLD", "--gocb", "gcbBool", "--go-id", "renamed", "--enable")...)

	// SV: the server publishes MSVCB01 (9-2LE) and MSVCB02 (generic) on the hub.
	shub := sv.NewHub()
	withSVHub(t, shub)
	sport := shub.NewPort(sv.MAC{0x02, 0, 0, 0, 0, 0x01})
	ssrv, err := server.Start(context.Background(), server.RunConfig{SclPath: scid, Host: "127.0.0.1", Port: 10103, SV: sport})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ssrv.Close(); _ = sport.Close() })
	mms = []string{"--host", "127.0.0.1", "--port", "10103"}

	capture("sv_list", "sv", "list", "--scl", scid)
	capture("sv_list_json", "sv", "list", "--scl", scid, "--format", "json")
	capture("sv_sniff", "sv", "sniff", "--iface", "eth0", "--count", "3")
	capture("sv_sniff_scl", "sv", "sniff", "--iface", "eth0", "--scl", scid, "--svcb", "MSVCB01", "--sample-every", "1000", "--count", "4000")
	capture("sv_sniff_layout_jsonl", "sv", "sniff", "--iface", "eth0", "--appid", "0x4001", "--layout", "FLOAT32,Quality,INT32U,Quality,INT32", "--count", "1", "--format", "jsonl")
	capture("sv_subscribe", "sv", "subscribe", "--iface", "eth0", "--scl", scid, "--svcb", "MSVCB01", "--sample-every", "4000", "--duration", "2500ms")
	capture("sv_subscribe_jsonl", "sv", "subscribe", "--iface", "eth0", "--scl", scid, "--svcb", "MSVCB02", "--bare-domain",
		"--sample-every", "500", "--stats-interval", "1s", "--count", "1000", "--format", "jsonl")
	capture("sv_subscribe_invalid", "sv", "subscribe", "--iface", "eth0", "--scl", scid, "--svcb", "MSVCB02", "--count", "1", "--duration", "1s")

	// SV publish runs on a quiet hub, with a subscriber in the background.
	_ = ssrv.Close()
	pubHub := sv.NewHub()
	withSVHub(t, pubHub)
	capture("sv_publish_no_iface", "sv", "publish", "--scl", scid, "--svcb", "MSVCB01", "--yes")
	capture("sv_publish_no_live", "sv", "publish", "--iface", "eth0", "--scl", scid, "--svcb", "MSVCB01", "--yes", "--simulation=false")
	capture("sv_publish", "sv", "publish", "--iface", "eth0", "--scl", scid, "--svcb", "MSVCB02", "--yes",
		"--value", "0=1234.5", "--value", "InteropLD/MMTR1.TotVAh.actVal=42", "--value", "4=1", "--duration", "2s")
	capture("sv_publish_waveform_jsonl", "sv", "publish", "--iface", "eth0", "--scl", scid, "--svcb", "MSVCB01", "--yes",
		"--waveform", "IA=100", "--waveform", "IB=100,50,-120", "--waveform", "IC=100,50,120",
		"--waveform", "VA=325", "--waveform", "VB=325,50,-120", "--waveform", "VC=325,50,120",
		"--duration", "2s", "--format", "jsonl")
	// A waveform stream as a subscriber sees it: the same publisher, run
	// through svops in the background (CLI runs share global flags).
	cb := loadSVCB(t, "MSVCB01")
	pc, err := cb.PublisherConfig()
	if err != nil {
		t.Fatal(err)
	}
	var waves []svops.Waveform
	for _, spec := range []string{"IA=100", "IB=100,50,-120", "IC=100,50,120", "VA=325", "VB=325,50,-120", "VC=325,50,120"} {
		w, err := svops.ParseWaveform(spec, 50)
		if err != nil {
			t.Fatal(err)
		}
		waves = append(waves, w)
	}
	if pc.Source, err = svops.WaveformSource(svops.ZeroValues(cb.Layout), waves, 4000); err != nil {
		t.Fatal(err)
	}
	pc.Simulation = true
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	pubPort := pubHub.NewPort(sv.MAC{0x02, 0, 0, 0, 0, 0x01})
	go func() {
		defer close(done)
		_, _ = svops.Publish(ctx, svops.PublishConfig{Transport: pubPort, Publisher: pc, Output: svops.Output{Out: io.Discard}})
	}()
	time.Sleep(200 * time.Millisecond)
	capture("sv_subscribe_waveform", "sv", "subscribe", "--iface", "eth0", "--scl", scid, "--svcb", "MSVCB01",
		"--accept-simulation", "--sample-every", "10", "--stats-interval", "0", "--count", "80")
	cancel()
	<-done
	_ = pubPort.Close()

	// MSVCB over MMS.
	ssrv2, err := server.Start(context.Background(), server.RunConfig{SclPath: scid, Host: "127.0.0.1", Port: 10103, SV: shub.NewPort(sv.MAC{0x02, 0, 0, 0, 0, 0x01})})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ssrv2.Close() })
	capture("sv_svcb_get", append(mms, "sv", "svcb", "get", "--ld", "InteropLD", "--svcb", "MSVCB01")...)
	capture("sv_svcb_get_json", append(mms, "sv", "svcb", "get", "--ld", "InteropLD", "--svcb", "MSVCB02", "--format", "json")...)
	capture("sv_svcb_set_rejected", append(mms, "sv", "svcb", "set", "--ld", "InteropLD", "--svcb", "MSVCB01", "--opt-flds", "refresh-time,sample-rate")...)
	capture("sv_svcb_set_disable", append(mms, "sv", "svcb", "set", "--ld", "InteropLD", "--svcb", "MSVCB01", "--disable")...)
	capture("sv_svcb_set_enable", append(mms, "sv", "svcb", "set", "--ld", "InteropLD", "--svcb", "MSVCB01", "--opt-flds", "refresh-time,sample-rate", "--enable")...)
}

// shellJoin quotes the arguments that a POSIX shell would split or expand.
func shellJoin(args []string) string {
	out := make([]string, len(args))
	for i, a := range args {
		if strings.ContainsAny(a, "$ []*") {
			a = "'" + a + "'"
		}
		out[i] = a
	}
	return strings.Join(out, " ")
}
