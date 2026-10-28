// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"errors"
	"fmt"
	"testing"

	iec61850 "github.com/otfabric/go-iec61850"
	"github.com/otfabric/go-mms"

	"github.com/otfabric/iec61850ctl/pkg/domain"
	"github.com/otfabric/iec61850ctl/pkg/errs"
)

// fcErrConn fails GetVariableType per FC according to errByFC (missing FCs
// succeed with a float leaf).
type fcErrConn struct {
	mockConnection
	errByFC  map[iec61850.FunctionalConstraint]error
	allErr   error
	listErr  error
	children []iec61850.BrowseNode
}

func (m *fcErrConn) GetVariableType(_ context.Context, ref iec61850.Ref) (*mms.TypeSpec, error) {
	if m.allErr != nil {
		return nil, m.allErr
	}
	if err, ok := m.errByFC[ref.FC]; ok {
		return nil, err
	}
	if len(ref.Path) <= 1 && m.children != nil {
		return &mms.TypeSpec{Type: mms.ValueTypeStructure}, nil
	}
	return &mms.TypeSpec{Type: mms.ValueTypeFloat}, nil
}

func (m *fcErrConn) ListChildren(_ context.Context, _ iec61850.Ref) ([]iec61850.BrowseNode, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.children, nil
}

func allFCs(err error) map[iec61850.FunctionalConstraint]error {
	out := map[iec61850.FunctionalConstraint]error{}
	for _, fc := range []string{"ST", "MX", "CF", "DC", "SP", "SV", "CO", "SG", "SE", "EX"} {
		out[iec61850.FunctionalConstraint(fc)] = err
	}
	return out
}

func TestExplorer_GetDataAttributes_ErrorClassification(t *testing.T) {
	t.Parallel()
	timeout := fmt.Errorf("read: %w", errs.ErrTimeout)
	notFound := fmt.Errorf("gva: %w", iec61850.ErrNotFound)
	svcErr := &mms.ServiceError{Class: 2, Code: 2}

	partial := allFCs(notFound)
	delete(partial, iec61850.FCMX)

	partialHard := allFCs(timeout)
	delete(partialHard, iec61850.FCST)

	mixed := allFCs(notFound)
	mixed[iec61850.FCSP] = timeout

	tests := []struct {
		name      string
		conn      *fcErrConn
		wantErr   error // nil => success
		wantCount int
	}{
		{name: "device down on every FC", conn: &fcErrConn{allErr: timeout}, wantErr: errs.ErrTimeout},
		{name: "absent under every FC", conn: &fcErrConn{errByFC: allFCs(notFound)}, wantErr: errs.ErrNotFound},
		{name: "service error under every FC", conn: &fcErrConn{errByFC: allFCs(svcErr)}, wantErr: errs.ErrNotFound},
		{name: "mixed absent and hard prefers hard", conn: &fcErrConn{errByFC: mixed}, wantErr: errs.ErrTimeout},
		{name: "partial absent tolerated", conn: &fcErrConn{errByFC: partial}, wantCount: 1},
		{name: "partial hard tolerated", conn: &fcErrConn{errByFC: partialHard}, wantCount: 1},
		{
			name:    "structure list children fails",
			conn:    &fcErrConn{children: []iec61850.BrowseNode{}, listErr: timeout},
			wantErr: errs.ErrTimeout,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			attrs, err := NewExplorer(tt.conn).GetDataAttributes(ListDataAttributesInput{
				LogicalDevice: "LD0", LogicalNode: "MMXU1", DataObject: "TotW",
			})
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want errors.Is %v", err, tt.wantErr)
				}
				if attrs != nil {
					t.Fatalf("attrs = %+v, want nil", attrs)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(attrs) != tt.wantCount {
				t.Fatalf("got %d attrs, want %d: %+v", len(attrs), tt.wantCount, attrs)
			}
		})
	}
}

func TestExplorer_DataAttributeWrappersPropagateErrors(t *testing.T) {
	t.Parallel()
	conn := &fcErrConn{allErr: errs.ErrTimeout}
	ex := NewExplorer(conn)
	if _, err := ex.GetDataObjectDANames("LD0", "MMXU1", "TotW"); !errors.Is(err, errs.ErrTimeout) {
		t.Fatalf("GetDataObjectDANames err = %v", err)
	}
	if _, err := ex.ListDataAttributes(ListDataAttributesInput{LogicalDevice: "LD0", LogicalNode: "MMXU1", DataObject: "TotW"}); !errors.Is(err, errs.ErrTimeout) {
		t.Fatalf("ListDataAttributes err = %v", err)
	}
	if _, err := ex.GetDataAttributes(ListDataAttributesInput{}); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("validate err = %v", err)
	}
	if _, err := NewExplorer(nil).GetDataAttributes(ListDataAttributesInput{LogicalDevice: "a", LogicalNode: "b", DataObject: "c"}); err == nil {
		t.Fatal("expected nil-client error")
	}
	for _, in := range []ListDataAttributesInput{
		{LogicalDevice: "a"},
		{LogicalDevice: "a", LogicalNode: "b"},
	} {
		if err := in.Validate(); !errors.Is(err, ErrInvalidConfig) {
			t.Fatalf("Validate(%+v) = %v", in, err)
		}
	}
}

func TestExplorer_GetDataAttributes_NestedNames(t *testing.T) {
	t.Parallel()
	m := newTreeModel()
	attrs, err := NewExplorer(m).ListDataAttributes(ListDataAttributesInput{
		LogicalDevice: "LD0", LogicalNode: "MMXU1", DataObject: "TotW",
	})
	if err != nil {
		t.Fatal(err)
	}
	mag := attrs["mag"]
	if len(mag) == 0 {
		t.Fatalf("attrs=%+v", attrs)
	}
	var sawNested bool
	for _, a := range mag {
		if a.Name == "mag.inner.u" && a.FC == domain.FC_MX && a.Value != nil {
			sawNested = true
		}
	}
	if !sawNested {
		t.Fatalf("missing mag.inner.u in %+v", mag)
	}
}
