// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"fmt"
	"strings"

	iec61850 "github.com/otfabric/go-iec61850"
	"github.com/otfabric/iec61850ctl/pkg/domain"
)

// Writer performs explicit scalar MMS writes.
type Writer struct {
	conn WriteConnection
}

// NewWriter returns a Writer bound to conn.
func NewWriter(conn WriteConnection) *Writer {
	return &Writer{conn: conn}
}

// SetObject writes a scalar attribute. FC=CO is rejected.
func (w *Writer) SetObject(ctx context.Context, req domain.WriteRequest) (*domain.WriteResult, error) {
	if strings.TrimSpace(req.Object) == "" {
		return nil, fmt.Errorf("object is required")
	}
	if !req.FC.IsValid() {
		return nil, fmt.Errorf("invalid functional constraint: %s", req.FC)
	}
	if req.FC == domain.FC_CO {
		return nil, fmt.Errorf("FC=CO is not allowed for set object; use control operate")
	}
	if req.Value.Kind == "" {
		return nil, fmt.Errorf("value type is required")
	}

	ref, err := iec61850.ParseRef(req.Object)
	if err != nil {
		return nil, fmt.Errorf("parse object: %w", err)
	}
	if ref.FC != "" && string(ref.FC) != string(req.FC) {
		return nil, fmt.Errorf("object FC %q conflicts with --fc %s", ref.FC, req.FC)
	}
	ref.FC = iec61850.FunctionalConstraint(req.FC)

	result := &domain.WriteResult{
		Object:         req.Object,
		FC:             string(req.FC),
		Type:           string(req.Value.Kind),
		RequestedValue: ScalarRequestedAny(req.Value),
	}

	mv, err := ScalarToMMS(req.Value)
	if err != nil {
		return nil, err
	}

	if writeErr := w.conn.Write(ctx, ref, mv); writeErr != nil {
		result.WriteOK = false
		result.Error = writeErr.Error()
		return writeOutcome(result)
	}
	result.WriteOK = true

	if !req.Verify {
		return writeOutcome(result)
	}

	ver := &domain.WriteVerification{Attempted: true}
	got, readErr := w.conn.Read(ctx, ref)
	if readErr != nil {
		ver.Error = readErr.Error()
		result.Verification = ver
		return writeOutcome(result)
	}
	ver.Value = ScalarFromIECValue(got)
	ver.Matched = ScalarMatchesMMS(req.Value, got.MMS())
	result.Verification = ver
	return writeOutcome(result)
}

// writeOutcome returns a completed WriteResult with a nil error so callers can
// emit JSON and map non-zero exit from WriteOK / verification (not Go error).
func writeOutcome(r *domain.WriteResult) (*domain.WriteResult, error) {
	return r, nil
}
