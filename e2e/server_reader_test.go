//go:build e2e

// SPDX-License-Identifier: MIT

package e2e_test

import (
	"math"
	"testing"
)

// Reverse general-reader journeys (Phase 3B): mms-interop IED clients → iec61850ctl server.
func TestReverse_ServerReader(t *testing.T) {
	for _, tc := range []struct {
		stack      string
		entrypoint string
	}{
		{stackServerReaderLib, "libiec61850-ied-client"},
		{stackServerReaderBean, "iec61850bean-ied-client"},
	} {
		tc := tc
		t.Run(tc.stack, func(t *testing.T) {
			requireReverseStack(t, tc.stack)
			requireCtlBin(t)

			image := resolveImageForReverseStack(tc.stack)
			if image == "" {
				t.Fatalf("no image for stack %q", tc.stack)
			}
			validateAdapterCapabilities(t, image, tc.entrypoint, tc.entrypoint)

			srv := startCtlServer(t)
			res := runAdapterJSONL(t, tc.entrypoint, image, srv.hostPort)
			if res.ExitErr != nil {
				logAdapterFailure(t, res, srv)
				t.Fatalf("ied-client exited with error: %v", res.ExitErr)
			}

			assertReaderJourney(t, res.Results)
		})
	}
}

func assertReaderJourney(t *testing.T, results []adapterOpResult) {
	t.Helper()

	requireExactOpOK(t, results, "associate")
	requireExactOpOK(t, results, "conclude")

	dir := requireExactOpOK(t, results, "get-server-directory")
	if !containsString(dir.Names, "InteropLD") {
		t.Fatalf("get-server-directory names missing InteropLD: %v", dir.Names)
	}

	ld := requireExactOpOK(t, results, "get-ld-directory")
	for _, want := range []string{"LLN0", "GGIO1", "MMXU1", "MMTR1"} {
		if !containsString(ld.Names, want) {
			t.Fatalf("get-ld-directory missing %q: %v", want, ld.Names)
		}
	}

	ln := requireExactOpOK(t, results, "get-ln-directory")
	for _, want := range []string{"SPS1", "SPCSO1"} {
		if !containsString(ln.Names, want) {
			t.Fatalf("get-ln-directory missing %q: %v", want, ln.Names)
		}
	}

	st := requireReadOK(t, results, "InteropLD/GGIO1.SPS1.stVal[ST]")
	if b, ok := st.Value.(bool); !ok || b != false {
		t.Fatalf("ST read: want false, got %#v", st.Value)
	}

	mx := requireReadOK(t, results, "InteropLD/MMXU1.TotW.mag.f[MX]")
	f, ok := asFloat64(mx.Value)
	if !ok || math.Abs(f-1234.5) > floatCompareEpsilon {
		t.Fatalf("MX read: want 1234.5, got %#v", mx.Value)
	}

	cf := requireReadOK(t, results, "InteropLD/LLN0.Mod.ctlModel[CF]")
	n, ok := asInt64(cf.Value)
	if !ok || n != 1 {
		t.Fatalf("CF read: want 1, got %#v", cf.Value)
	}

	dc := requireReadOK(t, results, "InteropLD/LLN0.Mod.d[DC]")
	if s, ok := dc.Value.(string); !ok || s != "mode" {
		t.Fatalf("DC read: want %q, got %#v", "mode", dc.Value)
	}

	wr := findOpTarget(results, "write", "InteropLD/LLN0.Mod.stVal[ST]")
	if wr == nil || !wr.OK {
		t.Fatalf("write Mod.stVal missing or not ok")
	}

	ds := findOpTarget(results, "read-dataset", "InteropLD/LLN0$dsInterop")
	if ds == nil || !ds.OK {
		t.Fatalf("read-dataset missing or not ok")
	}
	if len(ds.Values) < 2 {
		t.Fatalf("dataset values length: want >= 2, got %d (%v)", len(ds.Values), ds.Values)
	}
	if b, ok := ds.Values[0].(bool); !ok || b != false {
		t.Fatalf("dataset[0] SPS1: want false, got %#v", ds.Values[0])
	}
	mod, ok := asInt64(ds.Values[1])
	if !ok || mod != 5 {
		t.Fatalf("dataset[1] Mod.stVal: want 5 after write, got %#v", ds.Values[1])
	}

	for _, r := range results {
		if !r.OK {
			t.Fatalf("operation %q target=%q ok=false error=%q", r.Operation, r.Target, r.Error)
		}
	}
}

func requireExactOpOK(t *testing.T, results []adapterOpResult, op string) adapterOpResult {
	t.Helper()
	for _, r := range results {
		if r.Operation == op {
			if !r.OK {
				t.Fatalf("%s: ok=false error=%q", op, r.Error)
			}
			return r
		}
	}
	t.Fatalf("%s: not found", op)
	return adapterOpResult{}
}

func requireReadOK(t *testing.T, results []adapterOpResult, target string) adapterOpResult {
	t.Helper()
	r := findOpTarget(results, "read", target)
	if r == nil {
		t.Fatalf("read %q: not found", target)
	}
	if !r.OK {
		t.Fatalf("read %q: ok=false error=%q", target, r.Error)
	}
	return *r
}

func findOpTarget(results []adapterOpResult, op, target string) *adapterOpResult {
	for i := range results {
		if results[i].Operation == op && results[i].Target == target {
			return &results[i]
		}
	}
	return nil
}

func containsString(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}
