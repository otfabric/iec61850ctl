// SPDX-License-Identifier: MIT

package errs

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"testing"

	iec61850 "github.com/otfabric/go-iec61850"
	"github.com/otfabric/go-iec61850/goose"
	"github.com/otfabric/go-iec61850/sv"
	"github.com/otfabric/go-mms"
)

func TestMark(t *testing.T) {
	t.Parallel()
	if Mark(nil, ErrNotFound) != nil {
		t.Fatal("Mark(nil) must be nil")
	}
	base := errors.New("boom")
	m := Mark(base, ErrNotFound)
	if m.Error() != "boom" || !errors.Is(m, ErrNotFound) || !errors.Is(m, base) {
		t.Fatalf("Mark: %v", m)
	}
	if Mark(m, ErrNotFound) != m {
		t.Fatal("already-marked error must be returned unchanged")
	}
	if Mark(base, nil) != base {
		t.Fatal("nil sentinel must return err")
	}
	var pe *fs.PathError
	if !errors.As(Mark(&fs.PathError{Op: "open", Path: "x", Err: fs.ErrNotExist}, ErrInvalidInput), &pe) {
		t.Fatal("errors.As through Mark")
	}
	e := Invalidf("bad %d", 7)
	if e.Error() != "bad 7" || !errors.Is(e, ErrInvalidInput) {
		t.Fatalf("Invalidf: %v", e)
	}
	if n := Newf(ErrRejected, "x"); !errors.Is(n, ErrRejected) {
		t.Fatal("Newf")
	}
}

type timeoutErr struct{}

func (timeoutErr) Error() string   { return "i/o timeout" }
func (timeoutErr) Timeout() bool   { return true }
func (timeoutErr) Temporary() bool { return true }

func TestExitCode(t *testing.T) {
	t.Parallel()
	wrap := func(err error) error { return fmt.Errorf("ctx: %w", err) }
	tests := []struct {
		err  error
		want int
	}{
		{nil, ExitOK},
		{errors.New("x"), ExitFailure},
		{WithExitCode(errors.New("x"), 42), 42},
		{wrap(context.Canceled), ExitInterrupted},
		{wrap(context.DeadlineExceeded), ExitTimeout},
		{wrap(mms.ErrInvokeTimeout), ExitTimeout},
		{wrap(ErrTimeout), ExitTimeout},
		{&net.OpError{Op: "dial", Err: timeoutErr{}}, ExitTimeout},
		{Mark(wrap(context.DeadlineExceeded), ErrConnectionFailed), ExitTimeout},
		{wrap(ErrVerifyMismatch), ExitMismatch},
		{Invalidf("x"), ExitUsage},
		{wrap(ErrInvalidReference), ExitUsage},
		{wrap(ErrInvalidConfig), ExitUsage},
		{wrap(iec61850.ErrInvalidReference), ExitUsage},
		{wrap(iec61850.ErrInvalidFunctionalConstraint), ExitUsage},
		{wrap(iec61850.ErrInvalidArgument), ExitUsage},
		{wrap(iec61850.ErrSCLParse), ExitUsage},
		{wrap(goose.ErrInvalidConfig), ExitUsage},
		{wrap(goose.ErrInvalidValue), ExitUsage},
		{wrap(goose.ErrSCL), ExitUsage},
		{wrap(goose.ErrNotFound), ExitNotFound},
		{wrap(goose.ErrInterfaceNotFound), ExitNotFound},
		{wrap(goose.ErrUnsupportedPlatform), ExitUnsupported},
		{wrap(goose.ErrPermission), ExitRejected},
		{wrap(sv.ErrInvalidConfig), ExitUsage},
		{wrap(sv.ErrInvalidValue), ExitUsage},
		{wrap(sv.ErrSCL), ExitUsage},
		{wrap(sv.ErrValidation), ExitUsage},
		{wrap(sv.ErrNotFound), ExitNotFound},
		{wrap(ErrNotFound), ExitNotFound},
		{wrap(iec61850.ErrNotFound), ExitNotFound},
		{&fs.PathError{Op: "open", Path: "x", Err: os.ErrNotExist}, ExitNotFound},
		{wrap(ErrUnsupported), ExitUnsupported},
		{wrap(iec61850.ErrUnsupportedService), ExitUnsupported},
		{wrap(mms.ErrUnsupported), ExitUnsupported},
		{wrap(ErrPermissionDenied), ExitRejected},
		{wrap(ErrRejected), ExitRejected},
		{wrap(iec61850.ErrDataAccess), ExitRejected},
		{wrap(iec61850.ErrControlFailed), ExitRejected},
		{wrap(iec61850.ErrSelectFailed), ExitRejected},
		{wrap(iec61850.ErrOperateFailed), ExitRejected},
		{wrap(iec61850.ErrCancelFailed), ExitRejected},
		{wrap(iec61850.ErrNotControllable), ExitRejected},
		{wrap(mms.ErrDataAccess), ExitRejected},
		{wrap(mms.ErrServiceRejected), ExitRejected},
		{wrap(mms.ErrAuthenticationFailed), ExitRejected},
		{wrap(ErrConnectionFailed), ExitConnection},
		{wrap(iec61850.ErrClosed), ExitConnection},
		{wrap(mms.ErrClosed), ExitConnection},
		{wrap(mms.ErrConnectionRejected), ExitConnection},
		{wrap(mms.ErrAssociationFailed), ExitConnection},
		{wrap(mms.ErrNegotiationFailed), ExitConnection},
		{wrap(mms.ErrServerConnClosed), ExitConnection},
		{&net.OpError{Op: "dial", Err: errors.New("refused")}, ExitConnection},
		{&net.DNSError{Err: "no such host", Name: "x"}, ExitConnection},
	}
	for _, tt := range tests {
		if got := ExitCode(tt.err); got != tt.want {
			t.Errorf("ExitCode(%v) = %d, want %d", tt.err, got, tt.want)
		}
	}
}

func TestExitError(t *testing.T) {
	t.Parallel()
	if WithExitCode(nil, 3) != nil {
		t.Fatal("nil")
	}
	e := WithExitCode(ErrNotFound, 9)
	if e.Error() != ErrNotFound.Error() || !errors.Is(e, ErrNotFound) {
		t.Fatal(e)
	}
	if (&ExitError{Code: 1}).Error() != "exit status" {
		t.Fatal("empty ExitError message")
	}
	if IsTimeout(nil) {
		t.Fatal("IsTimeout(nil)")
	}
}
