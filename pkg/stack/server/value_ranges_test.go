// SPDX-License-Identifier: MIT

package server

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/otfabric/iec61850ctl/pkg/domain"
	"github.com/otfabric/iec61850ctl/pkg/errs"
)

// valuesFile writes a values file with one leaf of the given type and raw
// JSON value and returns its path.
func valuesFile(t *testing.T, ref, typ, raw string) string {
	t.Helper()
	doc := fmt.Sprintf(`{"Leaves":[{"Ref":%q,"FC":"ST","Type":%q,"Value":{"Raw":%s,"Type":%q}}]}`, ref, typ, raw, typ)
	path := filepath.Join(t.TempDir(), "values.json")
	if err := os.WriteFile(path, []byte(doc), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestValueRanges: a values file whose UTC time milliseconds (0..65535) or
// time quality (0..255), or whose bit string number (0..65535), lies outside
// its field is refused as invalid configuration (exit 3), and nothing of it
// is seeded; a value at the bound is seeded as written.
func TestValueRanges(t *testing.T) {
	const (
		tRef = "InteropLD/GGIO1.SPCSO1.t"
		tKey = "InteropIEDInteropLD/GGIO1$ST$SPCSO1$t"
		qRef = "InteropLD/GGIO1.SPCSO1.q"
		qKey = "InteropIEDInteropLD/GGIO1$ST$SPCSO1$q"
	)
	utc := func(ms, tq string) string {
		return fmt.Sprintf(`{"seconds":1700000000,"milliseconds":%s,"time_quality":%s}`, ms, tq)
	}
	for _, c := range []struct {
		name, ref, key, typ, raw string
		ok                       bool
	}{
		{"ms 0", tRef, tKey, "UTC_TIME", utc("0", "0"), true},
		{"ms 999", tRef, tKey, "UTC_TIME", utc("999", "10"), true},
		{"ms 65535", tRef, tKey, "UTC_TIME", utc("65535", "0"), true},
		{"ms 65536", tRef, tKey, "UTC_TIME", utc("65536", "0"), false},
		{"ms -1", tRef, tKey, "UTC_TIME", utc("-1", "0"), false},
		{"ms 1<<32+1", tRef, tKey, "UTC_TIME", utc("4294967297", "0"), false},
		{"tq 255", tRef, tKey, "UTC_TIME", utc("0", "255"), true},
		{"tq 256", tRef, tKey, "UTC_TIME", utc("0", "256"), false},
		{"tq -1", tRef, tKey, "UTC_TIME", utc("0", "-1"), false},
		{"bit string 0", qRef, qKey, "BIT_STRING", "0", true},
		{"bit string 65535", qRef, qKey, "BIT_STRING", "65535", true},
		{"bit string 65536", qRef, qKey, "BIT_STRING", "65536", false},
		{"bit string -1", qRef, qKey, "BIT_STRING", "-1", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			path := valuesFile(t, c.ref, c.typ, c.raw)
			srv := newInteropServer(t)
			before := srv.ValueStore().Get(c.key)
			err := seedValuesFromFile(srv, path)
			after := srv.ValueStore().Get(c.key)
			if c.ok {
				if err != nil {
					t.Fatal(err)
				}
				if after == nil || after == before {
					t.Fatalf("%s not seeded", c.key)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), "outside 0..") || !strings.Contains(err.Error(), c.ref) {
				t.Fatalf("err %v, want a range error naming %s", err, c.ref)
			}
			if code := errs.ExitCode(err); code != errs.ExitInvalidInput {
				t.Fatalf("exit code %d, want %d", code, errs.ExitInvalidInput)
			}
			if after != before {
				t.Fatalf("%s seeded from a refused file: %v", c.key, after)
			}
		})
	}
}

// TestValueRangesSeedSkips: SeedValues skips a leaf outside the range of
// its field rather than seeding a narrowed value.
func TestValueRangesSeedSkips(t *testing.T) {
	t.Parallel()
	srv := newInteropServer(t)
	const (
		tKey = "InteropIEDInteropLD/GGIO1$ST$SPCSO1$t"
		qKey = "InteropIEDInteropLD/GGIO1$ST$SPCSO1$q"
	)
	tBefore, qBefore := srv.ValueStore().Get(tKey), srv.ValueStore().Get(qKey)
	SeedValues(srv, &domain.IED{Leaves: []domain.LeafEntry{
		{Ref: "InteropLD/GGIO1.SPCSO1.t", FC: domain.FC_ST, Type: domain.TypeUtcTime,
			Value: domain.NewValue(map[string]any{"seconds": float64(1), "milliseconds": float64(65536)}, domain.TypeUtcTime)},
		{Ref: "InteropLD/GGIO1.SPCSO1.q", FC: domain.FC_ST, Type: domain.TypeBitString,
			Value: domain.NewValue(float64(65536), domain.TypeBitString)},
	}})
	if got := srv.ValueStore().Get(tKey); got != tBefore {
		t.Fatalf("%s seeded with %v", tKey, got)
	}
	if got := srv.ValueStore().Get(qKey); got != qBefore {
		t.Fatalf("%s seeded with %v", qKey, got)
	}
}

// TestStartRefusesValueOutOfRange: server start with such a values file
// fails before it listens, with exit 3.
func TestStartRefusesValueOutOfRange(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	path := valuesFile(t, "InteropLD/GGIO1.SPCSO1.q", "BIT_STRING", "65536")
	r, err := Start(ctx, RunConfig{SclPath: interopICD(t), Host: "127.0.0.1", ValuesPath: path})
	if err == nil {
		_ = r.Close()
		t.Fatal("server started with a value out of range")
	}
	if code := errs.ExitCode(err); code != errs.ExitInvalidInput || !errors.Is(err, errs.ErrInvalidConfig) {
		t.Fatalf("err %v: exit code %d, want %d", err, code, errs.ExitInvalidInput)
	}
}
