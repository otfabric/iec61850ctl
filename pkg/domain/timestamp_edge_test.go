// SPDX-License-Identifier: MIT

package domain

import (
	"errors"
	"testing"

	"github.com/otfabric/iec61850ctl/pkg/errs"
)

// Regression tests for ParseTimeToUnixMs range handling (found by fuzzing):
// pre-epoch times used to wrap around to huge uint64 values, times after
// 2262 overflowed UnixNano, and out-of-range millisecond numbers were not
// classified as invalid input.
func TestParseTimeToUnixMs_Range(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in      string
		want    uint64
		wantErr bool
	}{
		{in: "1969-12-31T23:59:59Z", wantErr: true},
		{in: "0001-01-01T00:00:00Z", wantErr: true},
		{in: "1960-01-01 00:00:00", wantErr: true},
		{in: "18446744073709551616", wantErr: true},
		{in: "18446744073709551615", wantErr: true},
		{in: "9223372036854775807", want: 9223372036854775807},
		{in: "1970-01-01T00:00:00Z", want: 0},
		{in: "2262-04-12T00:00:00Z", want: 9223372800000},
		{in: "9999-12-31T23:59:59Z", want: 253402300799000},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			t.Parallel()
			got, err := ParseTimeToUnixMs(tt.in)
			if tt.wantErr {
				if !errors.Is(err, errs.ErrInvalidInput) {
					t.Fatalf("err = %v, want ErrInvalidInput", err)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("got %d, %v; want %d", got, err, tt.want)
			}
		})
	}
}
