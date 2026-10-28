// SPDX-License-Identifier: MIT

package client

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	iec61850 "github.com/otfabric/go-iec61850"
	"github.com/otfabric/go-mms"

	"github.com/otfabric/iec61850ctl/pkg/errs"
)

func TestRetryPolicyMMS(t *testing.T) {
	t.Parallel()
	p := RetryPolicy{Retries: 3, BaseDelay: 100 * time.Millisecond, MaxDelay: time.Second}.MMS(nil)
	if p.MaxAttempts != 4 || p.InitialBackoff != 100*time.Millisecond || p.MaxBackoff != time.Second || p.Jitter != 0.5 {
		t.Fatalf("policy = %+v", p)
	}
	want := []time.Duration{100, 200, 400, 800, 1000, 1000}
	for i, w := range want {
		if got := p.Backoff(i + 1); got != w*time.Millisecond {
			t.Errorf("Backoff(%d) = %v, want %v", i+1, got, w*time.Millisecond)
		}
	}
	d := RetryPolicy{Retries: -2}.MMS(nil)
	if d.MaxAttempts != 1 || d.InitialBackoff != DefaultRetryBaseDelay || d.MaxBackoff != DefaultRetryMaxDelay {
		t.Fatalf("defaults = %+v", d)
	}
}

func TestRetryable(t *testing.T) {
	t.Parallel()
	for _, err := range []error{mms.ErrAuthenticationFailed, iec61850.ErrInvalidArgument, errs.ErrInvalidInput, context.Canceled} {
		if retryable(err) {
			t.Errorf("%v should be permanent", err)
		}
	}
	if !retryable(errors.New("connection refused")) || !retryable(context.DeadlineExceeded) {
		t.Error("transient errors should be retried")
	}
}

func TestRetryDo(t *testing.T) {
	t.Parallel()
	transient := errors.New("connection refused")
	tests := []struct {
		name      string
		retries   int
		errs      []error // per attempt; nil = success
		wantCalls int
		wantErr   error
		wantMsg   string
	}{
		{"first try", 3, []error{nil}, 1, nil, ""},
		{"succeeds on third", 3, []error{transient, transient, nil}, 3, nil, ""},
		{"exhausted", 2, []error{transient, transient, transient}, 3, transient, "gave up after 3 attempt(s)"},
		{"no retry", 0, []error{transient}, 1, transient, ""},
		{"negative retries", -5, []error{transient}, 1, transient, ""},
		{"auth is permanent", 5, []error{mms.ErrAuthenticationFailed}, 1, mms.ErrAuthenticationFailed, "after 1 attempt(s)"},
		{"invalid input is permanent", 5, []error{errs.ErrInvalidInput}, 1, errs.ErrInvalidInput, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := RetryPolicy{Retries: tt.retries, BaseDelay: time.Millisecond, MaxDelay: 2 * time.Millisecond}
			calls := 0
			err := p.Do(context.Background(), nil, func(context.Context) error {
				calls++
				return tt.errs[calls-1]
			})
			if calls != tt.wantCalls {
				t.Errorf("calls = %d, want %d", calls, tt.wantCalls)
			}
			if !errors.Is(err, tt.wantErr) || (tt.wantErr == nil) != (err == nil) {
				t.Errorf("err = %v, want %v", err, tt.wantErr)
			}
			if tt.wantMsg != "" && !strings.Contains(err.Error(), tt.wantMsg) {
				t.Errorf("err %q lacks %q", err, tt.wantMsg)
			}
		})
	}
}

func TestRetryDoContext(t *testing.T) {
	t.Parallel()
	t.Run("canceled before first attempt", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		called := false
		err := RetryPolicy{Retries: 3}.Do(ctx, nil, func(context.Context) error { called = true; return nil })
		if called || !errors.Is(err, context.Canceled) {
			t.Fatalf("called=%v err=%v", called, err)
		}
	})
	t.Run("canceled during backoff", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(context.Background())
		p := RetryPolicy{Retries: 3, BaseDelay: time.Hour, MaxDelay: time.Hour}
		done := make(chan error, 1)
		go func() {
			done <- p.Do(ctx, nil, func(context.Context) error { return errors.New("refused") })
		}()
		time.Sleep(20 * time.Millisecond)
		cancel()
		select {
		case err := <-done:
			if !errors.Is(err, context.Canceled) || !strings.Contains(err.Error(), "after 1 attempt(s)") {
				t.Fatalf("err=%v", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("Do did not return after cancel")
		}
	})
}
