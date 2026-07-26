// SPDX-License-Identifier: MIT

package service

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	iec61850 "github.com/otfabric/go-iec61850"
	"github.com/otfabric/iec61850ctl/pkg/domain"
	"github.com/otfabric/iec61850ctl/pkg/formatter"
)

// jsonlEvent is one JSON Lines record for subscribe report --format jsonl.
type jsonlEvent struct {
	Event           string   `json:"event"`
	DataSet         string   `json:"data_set,omitempty"`
	RptID           string   `json:"rpt_id,omitempty"`
	SequenceNumber  *int64   `json:"sequence_number,omitempty"`
	EntryID         string   `json:"entry_id,omitempty"` // hex
	Values          []any    `json:"values,omitempty"`
	Reasons         []string `json:"reasons,omitempty"`
	ReportsReceived int64    `json:"reports_received,omitempty"`
	CleanDisable    *bool    `json:"clean_disable,omitempty"`
	DurationMS      int64    `json:"duration_ms,omitempty"`
}

func writeJSONL(w io.Writer, ev jsonlEvent) error {
	b, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "%s\n", b)
	return err
}

func reasonTokens(report *domain.Report) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, el := range report.Elements {
		tok := reasonToken(el.Reason)
		if tok == "" {
			continue
		}
		if _, ok := seen[tok]; ok {
			continue
		}
		seen[tok] = struct{}{}
		out = append(out, tok)
	}
	return out
}

func reasonToken(r domain.ReasonForInclusion) string {
	switch r {
	case domain.ReasonDataChange:
		return "dchg"
	case domain.ReasonQualityChange:
		return "qchg"
	case domain.ReasonDataUpdate:
		return "dupd"
	case domain.ReasonIntegrity:
		return "integrity"
	case domain.ReasonGI:
		return "gi"
	default:
		if s := string(r); s != "" && s != string(domain.ReasonUnknown) {
			return s
		}
		return ""
	}
}

func reportValuesJSON(report *domain.Report, show bool) []any {
	if !show {
		return nil
	}
	out := make([]any, 0, len(report.Elements))
	for _, el := range report.Elements {
		v, _ := formatter.ScalarJSONValue(el.Value)
		out = append(out, v)
	}
	return out
}

func baselineValuesJSON(values []iec61850.DataSetValue, show bool) []any {
	if !show {
		return nil
	}
	out := make([]any, 0, len(values))
	for _, dv := range values {
		if dv.Err != nil || dv.Value == nil {
			out = append(out, nil)
			continue
		}
		v, _ := formatter.ScalarJSONValue(domain.ValueFromMMS(dv.Value.MMS()))
		out = append(out, v)
	}
	return out
}

func writeJSONLSummary(w io.Writer, reports int64, clean bool, dur time.Duration) error {
	cd := clean
	return writeJSONL(w, jsonlEvent{
		Event:           "summary",
		ReportsReceived: reports,
		CleanDisable:    &cd,
		DurationMS:      dur.Milliseconds(),
	})
}
