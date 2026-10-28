// SPDX-License-Identifier: MIT

package client

import (
	"context"
	"errors"
	"testing"
	"time"

	iec61850 "github.com/otfabric/go-iec61850"
	"github.com/otfabric/go-mms"

	"github.com/otfabric/iec61850ctl/pkg/errs"
)

func TestBackoff(t *testing.T) {
	t.Parallel()
	p := RetryPolicy{BaseDelay: 100 * time.Millisecond, MaxDelay: time.Second}
	want := []time.Duration{100, 200, 400, 800, 1000, 1000}
	for i, w := range want {
		if got := p.Backoff(i + 1); got != w*time.Millisecond {
			t.Errorf("Backoff(%d) = %v, want %v", i+1, got, w*time.Millisecond)
		}
	}
	d := RetryPolicy{}
	if got := d.Backoff(1); got != DefaultRetryBaseDelay {
		t.Errorf("default base = %v", got)
	}
	if got := d.Backoff(100); got != DefaultRetryMaxDelay {
		t.Errorf("default cap = %v", got)
	}
	tiny := RetryPolicy{BaseDelay: time.Second, MaxDelay: 10 * time.Millisecond}
	if got := tiny.Backoff(1); got != 10*time.Millisecond {
		t.Errorf("base above cap = %v", got)
	}
}

func TestEqualJitter(t *testing.T) {
	t.Parallel()
	for range 200 {
		got := equalJitter(100 * time.Millisecond)
		if got < 50*time.Millisecond || got > 100*time.Millisecond {
			t.Fatalf("jitter out of range: %v", got)
		}
	}
	if equalJitter(0) != 0 || equalJitter(1) != 1 {
		t.Fatal("degenerate jitter")
	}
}

type sleepRecorder struct{ delays []time.Duration }

func (s *sleepRecorder) sleep(ctx context.Context, d time.Duration) error {
	s.delays = append(s.delays, d)
	return ctx.Err()
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
		wantSleep int
	}{
		{"first try", 3, []error{nil}, 1, nil, 0},
		{"succeeds on third", 3, []error{transient, transient, nil}, 3, nil, 2},
		{"exhausted", 2, []error{transient, transient, transient}, 3, transient, 2},
		{"no retry", 0, []error{transient}, 1, transient, 0},
		{"negative retries", -5, []error{transient}, 1, transient, 0},
		{"auth is permanent", 5, []error{mms.ErrAuthenticationFailed}, 1, mms.ErrAuthenticationFailed, 0},
		{"invalid arg is permanent", 5, []error{iec61850.ErrInvalidArgument}, 1, iec61850.ErrInvalidArgument, 0},
		{"invalid input is permanent", 5, []error{errs.ErrInvalidInput}, 1, errs.ErrInvalidInput, 0},
		{"canceled is permanent", 5, []error{context.Canceled}, 1, context.Canceled, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rec := &sleepRecorder{}
			p := RetryPolicy{Retries: tt.retries, BaseDelay: 10 * time.Millisecond, MaxDelay: 25 * time.Millisecond,
				sleep: rec.sleep, jitter: func(d time.Duration) time.Duration { return d }}
			calls := 0
			err := p.Do(context.Background(), nil, func(attempt int) error {
				calls++
				if attempt != calls {
					t.Errorf("attempt=%d calls=%d", attempt, calls)
				}
				return tt.errs[calls-1]
			})
			if calls != tt.wantCalls {
				t.Errorf("calls = %d, want %d", calls, tt.wantCalls)
			}
			if !errors.Is(err, tt.wantErr) || (tt.wantErr == nil) != (err == nil) {
				t.Errorf("err = %v, want %v", err, tt.wantErr)
			}
			if len(rec.delays) != tt.wantSleep {
				t.Errorf("sleeps = %v, want %d", rec.delays, tt.wantSleep)
			}
			if tt.name == "exhausted" && (rec.delays[0] != 10*time.Millisecond || rec.delays[1] != 20*time.Millisecond) {
				t.Errorf("backoff sequence %v", rec.delays)
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
		err := RetryPolicy{Retries: 3}.Do(ctx, nil, func(int) error { called = true; return nil })
		if called || !errors.Is(err, context.Canceled) {
			t.Fatalf("called=%v err=%v", called, err)
		}
	})
	t.Run("canceled during backoff", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(context.Background())
		p := RetryPolicy{Retries: 3, BaseDelay: time.Hour, MaxDelay: time.Hour}
		calls := 0
		done := make(chan error, 1)
		go func() {
			done <- p.Do(ctx, nil, func(int) error { calls++; return errors.New("refused") })
		}()
		time.Sleep(20 * time.Millisecond)
		cancel()
		select {
		case err := <-done:
			if !errors.Is(err, context.Canceled) || calls != 1 {
				t.Fatalf("calls=%d err=%v", calls, err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("Do did not return after cancel")
		}
	})
	t.Run("canceled after failed attempt", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(context.Background())
		p := RetryPolicy{Retries: 3, sleep: func(context.Context, time.Duration) error { return nil }}
		err := p.Do(ctx, nil, func(int) error { cancel(); return errors.New("refused") })
		if !errors.Is(err, context.Canceled) && err.Error() == "" {
			t.Fatalf("err=%v", err)
		}
	})
}

func TestSleepCtx(t *testing.T) {
	t.Parallel()
	if err := sleepCtx(context.Background(), 0); err != nil {
		t.Fatal(err)
	}
	if err := sleepCtx(context.Background(), time.Millisecond); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := sleepCtx(ctx, time.Hour); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
