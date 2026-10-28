// SPDX-License-Identifier: MIT

package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	iec61850 "github.com/otfabric/go-iec61850"
	"github.com/otfabric/go-mms"
	"github.com/otfabric/iec61850ctl/internal/app"
	"github.com/otfabric/iec61850ctl/pkg/errs"
)

// errMockConn fails every device call used by the HTTP handlers with err and
// counts Close/Abort calls.
type errMockConn struct {
	httpMockConn
	err      error
	closeErr error
	closed   atomic.Int32
	aborted  atomic.Int32
}

func (m *errMockConn) ListLogicalDevices(_ context.Context) ([]iec61850.LogicalDevice, error) {
	return nil, m.err
}
func (m *errMockConn) ListLogicalNodes(_ context.Context, _ string) ([]iec61850.LogicalNode, error) {
	return nil, m.err
}
func (m *errMockConn) ListDataObjects(_ context.Context, _, _ string) ([]iec61850.DataObject, error) {
	return nil, m.err
}
func (m *errMockConn) ListChildren(_ context.Context, _ iec61850.Ref) ([]iec61850.BrowseNode, error) {
	return nil, m.err
}
func (m *errMockConn) TreeWithOptions(_ context.Context, _ iec61850.TreeOptions) (*iec61850.ModelNode, error) {
	return nil, m.err
}
func (m *errMockConn) FindPaths(_ context.Context, _ iec61850.FindQuery) ([]iec61850.Ref, error) {
	return nil, m.err
}
func (m *errMockConn) Read(_ context.Context, _ iec61850.Ref) (*iec61850.Value, error) {
	return nil, m.err
}
func (m *errMockConn) ReadMultiple(_ context.Context, _ []iec61850.Ref) ([]iec61850.ReadResult, error) {
	return nil, m.err
}
func (m *errMockConn) GetVariableType(_ context.Context, _ iec61850.Ref) (*mms.TypeSpec, error) {
	return nil, m.err
}
func (m *errMockConn) ListDataSets(_ context.Context, _ string) ([]string, error) {
	return nil, m.err
}
func (m *errMockConn) GetDataSet(_ context.Context, _, _ string) (*iec61850.DataSet, error) {
	return nil, m.err
}
func (m *errMockConn) ReadDataSet(_ context.Context, _, _ string) ([]iec61850.DataSetValue, error) {
	return nil, m.err
}
func (m *errMockConn) ListReports(_ context.Context, _ string) ([]string, error) {
	return nil, m.err
}
func (m *errMockConn) GetReportControlBlock(_ context.Context, _, _ string) (*iec61850.ReportControlBlock, error) {
	return nil, m.err
}
func (m *errMockConn) ListJournals(_ context.Context, _ string) ([]string, error) {
	return nil, m.err
}
func (m *errMockConn) ReadJournal(_ context.Context, _, _ string, _, _ time.Time) (*iec61850.JournalReadResult, error) {
	return nil, m.err
}
func (m *errMockConn) ReadJournalAfter(_ context.Context, _, _ string, _ time.Time, _ []byte) (*iec61850.JournalReadResult, error) {
	return nil, m.err
}
func (m *errMockConn) Close(_ context.Context) error {
	m.closed.Add(1)
	return m.closeErr
}
func (m *errMockConn) Abort(_ context.Context) error {
	m.aborted.Add(1)
	return nil
}

func newErrServer(err error) *Server {
	return NewServerWithApp(":0", app.New(&errMockConn{err: err}))
}

func decodeErrorBody(t *testing.T, rr *httptest.ResponseRecorder) errorBody {
	t.Helper()
	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("content-type=%q", ct)
	}
	var body errorBody
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error body %q: %v", rr.Body.String(), err)
	}
	if body.Error == "" || body.Code == "" {
		t.Fatalf("incomplete error body: %+v", body)
	}
	return body
}

func TestStatusForError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"invalid input", errs.Invalidf("bad ld %q", "x"), http.StatusBadRequest, codeInvalidInput},
		{"invalid reference", fmt.Errorf("parse: %w", errs.ErrInvalidReference), http.StatusBadRequest, codeInvalidInput},
		{"not found", fmt.Errorf("ld: %w", errs.ErrNotFound), http.StatusNotFound, codeNotFound},
		{"lib not found", iec61850.ErrNotFound, http.StatusNotFound, codeNotFound},
		{"rejected", errs.ErrRejected, http.StatusForbidden, codeRejected},
		{"permission denied", errs.ErrPermissionDenied, http.StatusForbidden, codeRejected},
		{"unsupported", errs.ErrUnsupported, http.StatusNotImplemented, codeUnsupported},
		{"timeout", errs.ErrTimeout, http.StatusGatewayTimeout, codeTimeout},
		{"deadline", context.DeadlineExceeded, http.StatusGatewayTimeout, codeTimeout},
		{"connection", errs.Mark(errors.New("dial"), errs.ErrConnectionFailed), http.StatusBadGateway, codeConnection},
		{"interrupted", context.Canceled, http.StatusServiceUnavailable, codeInterrupted},
		{"mismatch", errs.ErrVerifyMismatch, http.StatusInternalServerError, codeInternal},
		{"unclassified", errors.New("boom"), http.StatusInternalServerError, codeInternal},
		{"forced exit code", errs.WithExitCode(errors.New("x"), errs.ExitNotFound), http.StatusNotFound, codeNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			status, code := statusForError(tt.err)
			if status != tt.status || code != tt.code {
				t.Fatalf("statusForError(%v) = %d,%q; want %d,%q", tt.err, status, code, tt.status, tt.code)
			}
		})
	}
}

func TestCodeForStatus(t *testing.T) {
	t.Parallel()
	tests := map[int]string{
		http.StatusBadRequest:          codeInvalidInput,
		http.StatusMethodNotAllowed:    codeMethodNotAllowed,
		http.StatusNotFound:            codeNotFound,
		http.StatusInternalServerError: codeInternal,
		http.StatusTeapot:              codeInternal,
	}
	for status, want := range tests {
		if got := codeForStatus(status); got != want {
			t.Errorf("codeForStatus(%d)=%q want %q", status, got, want)
		}
	}
}

// errorRoutes lists one valid request per handler that reaches the device.
// /api/data-attributes is absent: the explorer skips per-FC device errors
// and returns an empty result (see TestDataAttributesErrorBranch).
var errorRoutes = []struct {
	method string
	path   string
	body   string
}{
	{http.MethodGet, "/api/logical-devices", ""},
	{http.MethodGet, "/api/logical-devices/names", ""},
	{http.MethodGet, "/api/logical-nodes?ld=LD0", ""},
	{http.MethodGet, "/api/logical-nodes/names?ld=LD0", ""},
	{http.MethodGet, "/api/data-objects?ld=LD0&ln=MMXU1", ""},
	{http.MethodGet, "/api/data-objects/names?ld=LD0&ln=MMXU1", ""},
	{http.MethodGet, "/api/datasets?ld=LD0&ln=LLN0", ""},
	{http.MethodGet, "/api/datasets/names?ld=LD0&ln=LLN0", ""},
	{http.MethodGet, "/api/datasets/details?ld=LD0&ln=LLN0&name=Meas", ""},
	{http.MethodGet, "/api/datasets/with-values?ld=LD0&ln=LLN0&name=Meas", ""},
	{http.MethodGet, "/api/reports?ld=LD0&ln=LLN0", ""},
	{http.MethodGet, "/api/reports/all", ""},
	{http.MethodGet, "/api/reports/names?ld=LD0&ln=LLN0", ""},
	{http.MethodGet, "/api/reports/details?ld=LD0&ln=LLN0&name=rcb1", ""},
	{http.MethodGet, "/api/journals?ld=LD0", ""},
	{http.MethodGet, "/api/journals/entries?domain=LD0&journal=LLN0$EventLog&from=1000", ""},
	{http.MethodPost, "/api/find/path", `{"LNPattern":"MMXU","DOName":"Hz"}`},
	{http.MethodPost, "/api/find/bulk", `{"mappings":[{"controlledPropertyId":"f","baseLn":"MMXU","DoDaPath":"Hz.mag.f"}]}`},
}

func TestHandlerErrorStatusMapping(t *testing.T) {
	t.Parallel()
	kinds := []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"not_found", fmt.Errorf("lookup: %w", errs.ErrNotFound), http.StatusNotFound, codeNotFound},
		{"timeout", fmt.Errorf("read: %w", errs.ErrTimeout), http.StatusGatewayTimeout, codeTimeout},
		{"connection", fmt.Errorf("io: %w", errs.ErrConnectionFailed), http.StatusBadGateway, codeConnection},
		{"rejected", fmt.Errorf("srv: %w", errs.ErrRejected), http.StatusForbidden, codeRejected},
		{"unsupported", fmt.Errorf("srv: %w", errs.ErrUnsupported), http.StatusNotImplemented, codeUnsupported},
		{"internal", errors.New("boom"), http.StatusInternalServerError, codeInternal},
	}
	for _, k := range kinds {
		s := newErrServer(k.err)
		for _, rt := range errorRoutes {
			t.Run(k.name+" "+rt.path, func(t *testing.T) {
				t.Parallel()
				var body []byte
				if rt.body != "" {
					body = []byte(rt.body)
				}
				rr := doReq(t, s, rt.method, rt.path, body)
				if rr.Code != k.status {
					t.Fatalf("status=%d want %d body=%s", rr.Code, k.status, rr.Body.String())
				}
				eb := decodeErrorBody(t, rr)
				if eb.Code != k.code {
					t.Fatalf("code=%q want %q", eb.Code, k.code)
				}
			})
		}
	}
}

// TestDataAttributesErrorBranch covers the handler's error path, which the
// device mock cannot reach because the explorer tolerates per-FC failures.
func TestDataAttributesErrorBranch(t *testing.T) {
	t.Parallel()
	s := NewServerWithApp(":0", app.New(nil))
	rr := doReq(t, s, http.MethodGet, "/api/data-attributes?ld=LD0&ln=MMXU1&do=Hz", nil)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if eb := decodeErrorBody(t, rr); eb.Code != codeInternal {
		t.Fatalf("code=%q", eb.Code)
	}
}

func TestTransportErrorBodies(t *testing.T) {
	t.Parallel()
	s := newTestServer()
	tests := []struct {
		method, path, body string
		status             int
		code               string
	}{
		{http.MethodPost, "/health", "", http.StatusMethodNotAllowed, codeMethodNotAllowed},
		{http.MethodGet, "/api/logical-nodes", "", http.StatusBadRequest, codeInvalidInput},
		{http.MethodPost, "/api/find/path", "{", http.StatusBadRequest, codeInvalidInput},
		{http.MethodPost, "/api/find/bulk", "[]", http.StatusBadRequest, codeInvalidInput},
	}
	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			t.Parallel()
			var body []byte
			if tt.body != "" {
				body = []byte(tt.body)
			}
			rr := doReq(t, s, tt.method, tt.path, body)
			if rr.Code != tt.status {
				t.Fatalf("status=%d want %d", rr.Code, tt.status)
			}
			if eb := decodeErrorBody(t, rr); eb.Code != tt.code {
				t.Fatalf("code=%q want %q", eb.Code, tt.code)
			}
		})
	}
}

func TestJSONResponseEncodeError(t *testing.T) {
	t.Parallel()
	s := newTestServer()
	rr := httptest.NewRecorder()
	s.jsonResponse(rr, http.StatusOK, map[string]any{"bad": make(chan int)})
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d", rr.Code)
	}
}
