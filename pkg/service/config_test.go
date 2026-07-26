// SPDX-License-Identifier: MIT

package service_test

import (
	"errors"
	"testing"
	"time"

	"github.com/otfabric/iec61850ctl/pkg/domain"
	"github.com/otfabric/iec61850ctl/pkg/service"
)

func TestReporterConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  service.ReporterConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: service.ReporterConfig{
				ReportRef: "LD/LN.BR.rcb1",
				Duration:  10 * time.Second,
			},
			wantErr: false,
		},
		{
			name: "missing ReportRef",
			config: service.ReporterConfig{
				Duration: 10 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "sync without DatasetRef",
			config: service.ReporterConfig{
				ReportRef: "LD/LN.BR.rcb1",
				Sync:      true,
			},
			wantErr: true,
		},
		{
			name: "negative duration",
			config: service.ReporterConfig{
				ReportRef: "LD/LN.BR.rcb1",
				Duration:  -5 * time.Second,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && !errors.Is(err, service.ErrInvalidConfig) {
				t.Errorf("Validate() error should wrap ErrInvalidConfig, got: %v", err)
			}
		})
	}
}

func TestFindPathInputValidation(t *testing.T) {
	tests := []struct {
		name    string
		input   service.FindPathInput
		wantErr bool
	}{
		{
			name: "valid input",
			input: service.FindPathInput{
				LNPattern: ".*MMXU.*",
				DoName:    "Hz",
			},
			wantErr: false,
		},
		{
			name: "missing LNPattern",
			input: service.FindPathInput{
				DoName: "Hz",
			},
			wantErr: true,
		},
		{
			name: "missing DoName",
			input: service.FindPathInput{
				LNPattern: ".*MMXU.*",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestParseDataSetRef(t *testing.T) {
	tests := []struct {
		name   string
		datSet string
		wantLD string
		wantLN string
		wantDS string
		wantOK bool
	}{
		{
			name:   "domain format",
			datSet: "LD0/LN1$dataset1",
			wantLD: "LD0",
			wantLN: "LN1",
			wantDS: "dataset1",
			wantOK: true,
		},
		{
			name:   "directory format",
			datSet: "LD0/LN1.dataset1",
			wantLD: "LD0",
			wantLN: "LN1",
			wantDS: "dataset1",
			wantOK: true,
		},
		{
			name:   "empty string",
			datSet: "",
			wantOK: false,
		},
		{
			name:   "missing separator",
			datSet: "LD0LN1dataset1",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ld, ln, ds, ok := domain.ParseDataSetRef(tt.datSet)
			if ok != tt.wantOK {
				t.Errorf("ParseDataSetRef() ok = %v, want %v", ok, tt.wantOK)
			}
			if ok && (ld != tt.wantLD || ln != tt.wantLN || ds != tt.wantDS) {
				t.Errorf("ParseDataSetRef() = (%q, %q, %q), want (%q, %q, %q)",
					ld, ln, ds, tt.wantLD, tt.wantLN, tt.wantDS)
			}
		})
	}
}

func TestParseReportRef(t *testing.T) {
	tests := []struct {
		name        string
		reportRef   string
		wantLD      string
		wantLN      string
		wantFC      domain.FunctionalConstraint
		wantRcbName string
		wantOK      bool
	}{
		{
			name:        "valid buffered report",
			reportRef:   "LD0/LN1.BR.rcb1",
			wantLD:      "LD0",
			wantLN:      "LN1",
			wantFC:      "BR",
			wantRcbName: "rcb1",
			wantOK:      true,
		},
		{
			name:        "valid unbuffered report",
			reportRef:   "ZX2REX640A1LD0/LLN0.RP.urcb01",
			wantLD:      "ZX2REX640A1LD0",
			wantLN:      "LLN0",
			wantFC:      "RP",
			wantRcbName: "urcb01",
			wantOK:      true,
		},
		{
			name:      "empty string",
			reportRef: "",
			wantOK:    false,
		},
		{
			name:      "missing parts",
			reportRef: "LD0/LN1",
			wantOK:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ld, ln, fc, rcbName, ok := domain.ParseReportRef(tt.reportRef)
			if ok != tt.wantOK {
				t.Errorf("ParseReportRef() ok = %v, want %v", ok, tt.wantOK)
			}
			if ok && (ld != tt.wantLD || ln != tt.wantLN || fc != tt.wantFC || rcbName != tt.wantRcbName) {
				t.Errorf("ParseReportRef() = (%q, %q, %q, %q), want (%q, %q, %q, %q)",
					ld, ln, fc, rcbName, tt.wantLD, tt.wantLN, tt.wantFC, tt.wantRcbName)
			}
		})
	}
}

func TestParseTimeToUnixMs(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    uint64
		wantErr bool
	}{
		{
			name:  "raw milliseconds",
			input: "1698672000000",
			want:  1698672000000,
		},
		{
			name:  "RFC3339",
			input: "2024-10-30T12:00:00Z",
			want:  1730289600000,
		},
		{
			name:  "space-separated UTC",
			input: "2024-10-30 12:00:00",
			want:  1730289600000,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "invalid format",
			input:   "not-a-time",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := domain.ParseTimeToUnixMs(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseTimeToUnixMs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ParseTimeToUnixMs() = %v, want %v", got, tt.want)
			}
		})
	}
}
