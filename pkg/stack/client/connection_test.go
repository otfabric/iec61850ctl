// SPDX-License-Identifier: MIT

package client

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	iec61850 "github.com/otfabric/go-iec61850"

	"github.com/otfabric/iec61850ctl/pkg/errs"
	"github.com/otfabric/iec61850ctl/pkg/service"
	"github.com/otfabric/iec61850ctl/pkg/stack/server"
)

// callAll invokes every IEC61850Connection method once with ctx.
func callAll(c service.IEC61850Connection, ctx context.Context) {
	ref := iec61850.Ref{}
	_, _ = c.ReadCtlModel(ctx, ref)
	_, _ = c.Select(ctx, ref)
	_ = c.SelectWithValue(ctx, ref, iec61850.OperateParams{})
	_ = c.Operate(ctx, ref, iec61850.OperateParams{})
	_ = c.Cancel(ctx, ref, iec61850.CancelParams{})
	_, _ = c.ReadLastApplError(ctx, ref)
	_, _ = c.Read(ctx, ref)
	_ = c.Write(ctx, ref, nil)
	_, _ = c.ListLogicalDevices(ctx)
	_, _ = c.ListLogicalNodes(ctx, "LD")
	_, _ = c.ListDataObjects(ctx, "LD", "LN")
	_, _ = c.ListChildren(ctx, ref)
	_, _ = c.TreeWithOptions(ctx, iec61850.TreeOptions{})
	_, _ = c.FindPaths(ctx, iec61850.FindQuery{})
	_, _ = c.ReadMultiple(ctx, nil)
	_, _ = c.GetVariableType(ctx, ref)
	_, _ = c.ListDataSets(ctx, "LD")
	_, _ = c.GetDataSet(ctx, "LD", "DS")
	_, _ = c.ReadDataSet(ctx, "LD", "DS")
	_, _ = c.ListReports(ctx, "LD")
	_, _ = c.GetReportControlBlock(ctx, "LD", "RCB")
	_ = c.SetReportControlBlock(ctx, "LD", "RCB", iec61850.RCBUpdate{})
	_ = c.TriggerGI(ctx, "LD", "RCB")
	_, _ = c.SubscribeReport(ctx, "rpt", iec61850.SubscribeReportOptions{})
	_, _ = c.ListFiles(ctx, "*")
	_, _ = c.DownloadFile(ctx, "f", io.Discard)
	_, _ = c.GetFileAttributes(ctx, "f")
	_, _ = c.ListJournals(ctx, "LD")
	_, _ = c.ReadJournal(ctx, "LD", "J", time.Time{}, time.Time{})
	_, _ = c.ReadJournalAfter(ctx, "LD", "J", time.Time{}, nil)
	_ = c.Close(ctx)
	_ = c.Abort(ctx)
}

func TestTimeoutConnAppliesDeadlines(t *testing.T) {
	t.Parallel()
	noDeadline := map[string]bool{"SubscribeReport": true, "DownloadFile": true, "Close": true, "Abort": true}
	p := &probeConn{}
	base, cancel := context.WithCancel(context.Background())
	defer cancel()
	callAll(&timeoutConn{IEC61850Connection: p, base: base, timeout: time.Minute}, context.Background())
	for _, m := range probeMethods {
		got, ok := p.deadline[m]
		if !ok {
			t.Errorf("%s not forwarded", m)
			continue
		}
		if want := !noDeadline[m]; got != want {
			t.Errorf("%s: deadline=%v, want %v", m, got, want)
		}
	}
}

func TestTimeoutConnKeepsCallerDeadlineAndNilContext(t *testing.T) {
	t.Parallel()
	p := &probeConn{}
	tc := &timeoutConn{IEC61850Connection: p, timeout: 0}
	//nolint:staticcheck // nil context is tolerated on purpose
	callAll(tc, nil)
	if p.deadline["Read"] {
		t.Fatal("timeout 0 must not add a deadline")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Hour)
	defer cancel()
	tc.timeout = time.Millisecond
	_, _ = tc.Read(ctx, iec61850.Ref{})
	if !p.deadline["Read"] {
		t.Fatal("caller deadline lost")
	}
}

func TestTimeoutConnBoundsSilentPeer(t *testing.T) {
	t.Parallel()
	p := &probeConn{block: true}
	tc := &timeoutConn{IEC61850Connection: p, base: context.Background(), timeout: 20 * time.Millisecond}
	start := time.Now()
	_, err := tc.Read(context.Background(), iec61850.Ref{})
	if !errs.IsTimeout(err) || errs.ExitCode(err) != errs.ExitTimeout {
		t.Fatalf("err=%v", err)
	}
	if time.Since(start) > 2*time.Second {
		t.Fatal("timeout not applied")
	}
}

func TestTimeoutConnBaseCancellation(t *testing.T) {
	t.Parallel()
	for _, method := range []string{"Read", "DownloadFile", "SubscribeReport"} {
		t.Run(method, func(t *testing.T) {
			t.Parallel()
			p := &probeConn{block: true}
			base, cancel := context.WithCancel(context.Background())
			tc := &timeoutConn{IEC61850Connection: p, base: base, timeout: time.Hour}
			done := make(chan error, 1)
			go func() {
				var err error
				switch method {
				case "Read":
					_, err = tc.Read(context.Background(), iec61850.Ref{})
				case "DownloadFile":
					_, err = tc.DownloadFile(context.Background(), "f", io.Discard)
				default:
					_, err = tc.SubscribeReport(context.Background(), "r", iec61850.SubscribeReportOptions{})
				}
				done <- err
			}()
			time.Sleep(10 * time.Millisecond)
			cancel()
			select {
			case err := <-done:
				if !errors.Is(err, context.Canceled) || errs.ExitCode(err) != errs.ExitInterrupted {
					t.Fatalf("err=%v", err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("base cancel did not abort request")
			}
		})
	}
}

func TestConnectionInputAddr(t *testing.T) {
	t.Parallel()
	if got := (ConnectionInput{Host: "::1", Port: 102}).Addr(); got != "[::1]:102" {
		t.Fatalf("ipv6 addr %q", got)
	}
	if got := (ConnectionInput{Host: "10.0.0.1", Port: 8102}).Addr(); got != "10.0.0.1:8102" {
		t.Fatalf("ipv4 addr %q", got)
	}
}

func TestNewConnectionValidation(t *testing.T) {
	t.Parallel()
	for _, in := range []ConnectionInput{{Port: 102}, {Host: "h", Port: 0}, {Host: "h", Port: 70000}} {
		_, err := NewConnection(context.Background(), in)
		if errs.ExitCode(err) != errs.ExitUsage {
			t.Errorf("%+v: err=%v", in, err)
		}
	}
}

func TestNewConnectionDialFailures(t *testing.T) {
	t.Parallel()
	refused := errors.New("connection refused")
	t.Run("retries then classifies", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		calls := 0
		_, err := newConnection(context.Background(), ConnectionInput{
			Host: "h", Port: 1, Retry: RetryPolicy{Retries: 2, BaseDelay: time.Millisecond, MaxDelay: time.Millisecond},
			Logger: slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})),
		}, func(ctx context.Context, addr string, _ iec61850.DialOptions) (*iec61850.Client, error) {
			calls++
			if _, ok := ctx.Deadline(); !ok {
				t.Error("dial attempt without deadline")
			}
			if addr != "h:1" {
				t.Errorf("addr %q", addr)
			}
			return nil, refused
		})
		if calls != 3 || !errors.Is(err, refused) || !errors.Is(err, errs.ErrConnectionFailed) {
			t.Fatalf("calls=%d err=%v", calls, err)
		}
		if errs.ExitCode(err) != errs.ExitConnection {
			t.Fatalf("exit=%d", errs.ExitCode(err))
		}
		if !strings.Contains(buf.String(), "retrying") || !strings.Contains(err.Error(), "gave up after 3") {
			t.Fatalf("log=%q err=%q", buf.String(), err)
		}
	})
	t.Run("nil client", func(t *testing.T) {
		t.Parallel()
		//lint:ignore SA1012 nil context is tolerated on purpose
		_, err := newConnection(nil, //nolint:staticcheck // see lint:ignore above
			ConnectionInput{Host: "h", Port: 1},
			func(context.Context, string, iec61850.DialOptions) (*iec61850.Client, error) { return nil, nil })
		if !errors.Is(err, errs.ErrConnectionFailed) {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("timeout", func(t *testing.T) {
		t.Parallel()
		_, err := newConnection(context.Background(), ConnectionInput{Host: "h", Port: 1, ConnectTimeout: 10 * time.Millisecond},
			func(ctx context.Context, _ string, _ iec61850.DialOptions) (*iec61850.Client, error) {
				<-ctx.Done()
				return nil, ctx.Err()
			})
		if errs.ExitCode(err) != errs.ExitTimeout {
			t.Fatalf("err=%v exit=%d", err, errs.ExitCode(err))
		}
	})
	t.Run("refused port", func(t *testing.T) {
		t.Parallel()
		_, err := NewConnection(context.Background(), ConnectionInput{Host: "127.0.0.1", Port: 1, ConnectTimeout: 2 * time.Second})
		if code := errs.ExitCode(err); code != errs.ExitConnection {
			t.Fatalf("err=%v exit=%d", err, code)
		}
	})
}

func testdataSCL(t *testing.T) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "e2e", "testdata", "interop.icd")
}

func TestNewConnectionRealServer(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	srv, err := server.Start(ctx, server.RunConfig{SclPath: testdataSCL(t), Host: "127.0.0.1", Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = srv.Close() }()

	host, portStr, _ := strings.Cut(srv.Addr(), ":")
	var port int
	for _, c := range portStr {
		port = port*10 + int(c-'0')
	}
	conn, err := NewConnection(ctx, ConnectionInput{Host: host, Port: port, IEDName: srv.IEDName()})
	if err != nil {
		t.Fatal(err)
	}
	lds, err := conn.ListLogicalDevices(context.Background())
	if err != nil || len(lds) == 0 {
		t.Fatalf("lds=%v err=%v", lds, err)
	}
	if err := conn.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
}
