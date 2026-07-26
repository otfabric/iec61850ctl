// SPDX-License-Identifier: MIT

package http

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	iec61850 "github.com/otfabric/go-iec61850"
	"github.com/otfabric/go-mms"
	"github.com/otfabric/iec61850ctl/internal/app"
	"github.com/otfabric/iec61850ctl/pkg/service"
)

type httpMockConn struct{}

func (m *httpMockConn) ListLogicalDevices(_ context.Context) ([]iec61850.LogicalDevice, error) {
	return []iec61850.LogicalDevice{{Name: "LD0"}}, nil
}
func (m *httpMockConn) ListLogicalNodes(_ context.Context, _ string) ([]iec61850.LogicalNode, error) {
	return []iec61850.LogicalNode{{Name: "LLN0"}, {Name: "MMXU1"}}, nil
}
func (m *httpMockConn) ListDataObjects(_ context.Context, _, _ string) ([]iec61850.DataObject, error) {
	return []iec61850.DataObject{{Name: "Hz"}, {Name: "Mod"}}, nil
}
func (m *httpMockConn) ListChildren(_ context.Context, ref iec61850.Ref) ([]iec61850.BrowseNode, error) {
	// Structure DO → mag → f (leaf), so ListDataAttributes can walk the tree.
	name := ref.String()
	switch {
	case endsWithSegment(name, "Hz"), endsWithSegment(name, "Mod"):
		c, _ := ref.Child("mag")
		return []iec61850.BrowseNode{{Name: "mag", Reference: c}}, nil
	case endsWithSegment(name, "mag"):
		c, _ := ref.Child("f")
		return []iec61850.BrowseNode{{Name: "f", Reference: c}}, nil
	default:
		return nil, nil
	}
}
func (m *httpMockConn) TreeWithOptions(_ context.Context, _ iec61850.TreeOptions) (*iec61850.ModelNode, error) {
	return &iec61850.ModelNode{Name: "root"}, nil
}
func (m *httpMockConn) FindPaths(_ context.Context, _ iec61850.FindQuery) ([]iec61850.Ref, error) {
	ref, _ := iec61850.ParseRef("LD0/MMXU1.Hz")
	return []iec61850.Ref{ref}, nil
}
func (m *httpMockConn) Read(_ context.Context, _ iec61850.Ref) (*iec61850.Value, error) {
	return iec61850.NewValue(mms.NewFloat(1)), nil
}
func (m *httpMockConn) Write(_ context.Context, _ iec61850.Ref, _ *mms.Value) error {
	return nil
}
func (m *httpMockConn) ReadCtlModel(_ context.Context, _ iec61850.Ref) (iec61850.CtlModel, error) {
	return iec61850.CtlModelStatusOnly, nil
}
func (m *httpMockConn) Select(_ context.Context, _ iec61850.Ref) (string, error) {
	return "", nil
}
func (m *httpMockConn) SelectWithValue(_ context.Context, _ iec61850.Ref, _ iec61850.OperateParams) error {
	return nil
}
func (m *httpMockConn) Operate(_ context.Context, _ iec61850.Ref, _ iec61850.OperateParams) error {
	return nil
}
func (m *httpMockConn) Cancel(_ context.Context, _ iec61850.Ref, _ iec61850.CancelParams) error {
	return nil
}
func (m *httpMockConn) ReadLastApplError(_ context.Context, _ iec61850.Ref) (*iec61850.LastApplError, error) {
	return nil, nil
}
func (m *httpMockConn) ReadMultiple(_ context.Context, _ []iec61850.Ref) ([]iec61850.ReadResult, error) {
	return nil, nil
}
func (m *httpMockConn) GetVariableType(_ context.Context, ref iec61850.Ref) (*mms.TypeSpec, error) {
	name := ref.String()
	switch {
	case endsWithSegment(name, "Hz"), endsWithSegment(name, "Mod"), endsWithSegment(name, "mag"):
		return &mms.TypeSpec{Type: mms.ValueTypeStructure}, nil
	default:
		return &mms.TypeSpec{Type: mms.ValueTypeFloat}, nil
	}
}
func (m *httpMockConn) ListDataSets(_ context.Context, _ string) ([]string, error) {
	return []string{"LLN0$Meas"}, nil
}
func (m *httpMockConn) GetDataSet(_ context.Context, _, _ string) (*iec61850.DataSet, error) {
	ref, _ := iec61850.ParseRef("LD0/MMXU1.Hz.mag.f[MX]")
	return &iec61850.DataSet{
		Reference: "LD0/LLN0.Meas",
		Members:   []iec61850.DataSetMember{{Ref: ref, DomainID: "LD0", ItemID: "MMXU1$MX$Hz$mag$f"}},
	}, nil
}
func (m *httpMockConn) ReadDataSet(_ context.Context, _, _ string) ([]iec61850.DataSetValue, error) {
	return []iec61850.DataSetValue{{Value: iec61850.NewValue(mms.NewFloat(50))}}, nil
}
func (m *httpMockConn) ListReports(_ context.Context, _ string) ([]string, error) {
	return []string{"LLN0$BR$rcb1", "LLN0$RP$urcb1"}, nil
}
func (m *httpMockConn) GetReportControlBlock(_ context.Context, _, _ string) (*iec61850.ReportControlBlock, error) {
	return &iec61850.ReportControlBlock{RptID: "r1", DatSet: "LD0/LLN0$Meas"}, nil
}
func (m *httpMockConn) SetReportControlBlock(_ context.Context, _, _ string, _ iec61850.RCBUpdate) error {
	return nil
}
func (m *httpMockConn) TriggerGI(_ context.Context, _, _ string) error { return nil }
func (m *httpMockConn) SubscribeReport(_ context.Context, _ string, _ iec61850.SubscribeReportOptions) (*iec61850.ReportSubscription, error) {
	return nil, nil
}
func (m *httpMockConn) ListFiles(_ context.Context, _ string) ([]iec61850.FileEntry, error) {
	return nil, nil
}
func (m *httpMockConn) DownloadFile(_ context.Context, _ string, _ io.Writer) (*iec61850.FileEntry, error) {
	return nil, nil
}
func (m *httpMockConn) GetFileAttributes(_ context.Context, _ string) (*iec61850.FileEntry, error) {
	return nil, nil
}
func (m *httpMockConn) ListJournals(_ context.Context, _ string) ([]string, error) {
	return []string{"LLN0$EventLog"}, nil
}
func (m *httpMockConn) ReadJournal(_ context.Context, _, _ string, _, _ time.Time) (*iec61850.JournalReadResult, error) {
	return &iec61850.JournalReadResult{}, nil
}
func (m *httpMockConn) ReadJournalAfter(_ context.Context, _, _ string, _ time.Time, _ []byte) (*iec61850.JournalReadResult, error) {
	return &iec61850.JournalReadResult{}, nil
}
func (m *httpMockConn) Close(_ context.Context) error { return nil }
func (m *httpMockConn) Abort(_ context.Context) error { return nil }

var _ service.IEC61850Connection = (*httpMockConn)(nil)

func endsWithSegment(ref, seg string) bool {
	base := ref
	if i := strings.LastIndexByte(base, '['); i >= 0 && strings.HasSuffix(base, "]") {
		base = base[:i]
	}
	return base == seg || strings.HasSuffix(base, "."+seg) || strings.HasSuffix(base, "/"+seg)
}

func newTestServer() *Server {
	return NewServerWithApp(":0", app.New(&httpMockConn{}))
}

func doReq(t *testing.T, s *Server, method, path string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	var r io.Reader
	if body != nil {
		r = bytes.NewReader(body)
	}
	req := httptest.NewRequest(method, path, r)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rr := httptest.NewRecorder()
	s.router.ServeHTTP(rr, req)
	return rr
}

func TestNewServerWithApp(t *testing.T) {
	s := NewServerWithApp(":0", app.New(&httpMockConn{}))
	if s == nil {
		t.Fatal("nil server")
	}
	if s.router == nil {
		t.Fatal("router is nil")
	}
	if s.app == nil {
		t.Fatal("app is nil")
	}
	if s.httpServer == nil {
		t.Fatal("httpServer is nil")
	}
}

func TestHealth(t *testing.T) {
	s := newTestServer()
	rr := doReq(t, s, http.MethodGet, "/health", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d", rr.Code)
	}
	rr = doReq(t, s, http.MethodPost, "/health", nil)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d", rr.Code)
	}
}

func TestLogicalDevices(t *testing.T) {
	s := newTestServer()

	rr := doReq(t, s, http.MethodGet, "/api/logical-devices", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var body map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["logical_devices"] == nil {
		t.Fatalf("missing logical_devices: %v", body)
	}

	rr = doReq(t, s, http.MethodGet, "/api/logical-devices/names", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("names status=%d", rr.Code)
	}

	rr = doReq(t, s, http.MethodPost, "/api/logical-devices", nil)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("method status=%d", rr.Code)
	}
	rr = doReq(t, s, http.MethodPost, "/api/logical-devices/names", nil)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("names method status=%d", rr.Code)
	}
}

func TestLogicalNodes(t *testing.T) {
	s := newTestServer()

	rr := doReq(t, s, http.MethodGet, "/api/logical-nodes", nil)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("missing ld status=%d", rr.Code)
	}
	rr = doReq(t, s, http.MethodGet, "/api/logical-nodes/names", nil)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("names missing ld status=%d", rr.Code)
	}

	rr = doReq(t, s, http.MethodGet, "/api/logical-nodes?ld=LD0", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	rr = doReq(t, s, http.MethodGet, "/api/logical-nodes/names?ld=LD0", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("names status=%d body=%s", rr.Code, rr.Body.String())
	}

	rr = doReq(t, s, http.MethodPost, "/api/logical-nodes?ld=LD0", nil)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("method status=%d", rr.Code)
	}
	rr = doReq(t, s, http.MethodPost, "/api/logical-nodes/names?ld=LD0", nil)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("names method status=%d", rr.Code)
	}
}

func TestDataObjects(t *testing.T) {
	s := newTestServer()

	rr := doReq(t, s, http.MethodGet, "/api/data-objects", nil)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("missing params status=%d", rr.Code)
	}
	rr = doReq(t, s, http.MethodGet, "/api/data-objects?ld=LD0", nil)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("missing ln status=%d", rr.Code)
	}
	rr = doReq(t, s, http.MethodGet, "/api/data-objects/names", nil)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("names missing params status=%d", rr.Code)
	}

	rr = doReq(t, s, http.MethodGet, "/api/data-objects?ld=LD0&ln=MMXU1", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	rr = doReq(t, s, http.MethodGet, "/api/data-objects/names?ld=LD0&ln=MMXU1", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("names status=%d body=%s", rr.Code, rr.Body.String())
	}

	rr = doReq(t, s, http.MethodDelete, "/api/data-objects?ld=LD0&ln=MMXU1", nil)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("method status=%d", rr.Code)
	}
	rr = doReq(t, s, http.MethodDelete, "/api/data-objects/names?ld=LD0&ln=MMXU1", nil)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("names method status=%d", rr.Code)
	}
}

func TestDataAttributes(t *testing.T) {
	s := newTestServer()

	rr := doReq(t, s, http.MethodGet, "/api/data-attributes", nil)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("missing params status=%d", rr.Code)
	}
	rr = doReq(t, s, http.MethodGet, "/api/data-attributes?ld=LD0&ln=MMXU1", nil)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("missing do status=%d", rr.Code)
	}

	rr = doReq(t, s, http.MethodGet, "/api/data-attributes?ld=LD0&ln=MMXU1&do=Hz", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var body map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["data_attributes"] == nil {
		t.Fatalf("missing data_attributes: %v", body)
	}

	rr = doReq(t, s, http.MethodGet, "/api/data-attributes?ld=LD0&ln=MMXU1&do=Hz&detailed=true", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("detailed status=%d body=%s", rr.Code, rr.Body.String())
	}

	rr = doReq(t, s, http.MethodPost, "/api/data-attributes?ld=LD0&ln=MMXU1&do=Hz", nil)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("method status=%d", rr.Code)
	}
}

func TestDataSets(t *testing.T) {
	s := newTestServer()

	rr := doReq(t, s, http.MethodGet, "/api/datasets", nil)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("missing params status=%d", rr.Code)
	}
	rr = doReq(t, s, http.MethodGet, "/api/datasets/names", nil)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("names missing params status=%d", rr.Code)
	}
	rr = doReq(t, s, http.MethodGet, "/api/datasets/details", nil)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("details missing params status=%d", rr.Code)
	}
	rr = doReq(t, s, http.MethodGet, "/api/datasets/with-values", nil)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("with-values missing params status=%d", rr.Code)
	}

	rr = doReq(t, s, http.MethodGet, "/api/datasets?ld=LD0&ln=LLN0", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	rr = doReq(t, s, http.MethodGet, "/api/datasets/names?ld=LD0&ln=LLN0", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("names status=%d body=%s", rr.Code, rr.Body.String())
	}
	rr = doReq(t, s, http.MethodGet, "/api/datasets/details?ld=LD0&ln=LLN0&name=Meas", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("details status=%d body=%s", rr.Code, rr.Body.String())
	}
	rr = doReq(t, s, http.MethodGet, "/api/datasets/with-values?ld=LD0&ln=LLN0&name=Meas", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("with-values status=%d body=%s", rr.Code, rr.Body.String())
	}
	rr = doReq(t, s, http.MethodGet, "/api/datasets/with-values?ld=LD0&ln=LLN0&name=Meas&read_values=false", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("with-values no-read status=%d body=%s", rr.Code, rr.Body.String())
	}

	for _, path := range []string{
		"/api/datasets?ld=LD0&ln=LLN0",
		"/api/datasets/names?ld=LD0&ln=LLN0",
		"/api/datasets/details?ld=LD0&ln=LLN0&name=Meas",
		"/api/datasets/with-values?ld=LD0&ln=LLN0&name=Meas",
	} {
		rr = doReq(t, s, http.MethodPost, path, nil)
		if rr.Code != http.StatusMethodNotAllowed {
			t.Fatalf("%s method status=%d", path, rr.Code)
		}
	}
}

func TestReports(t *testing.T) {
	s := newTestServer()

	rr := doReq(t, s, http.MethodGet, "/api/reports", nil)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("missing params status=%d", rr.Code)
	}
	rr = doReq(t, s, http.MethodGet, "/api/reports/names", nil)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("names missing params status=%d", rr.Code)
	}
	rr = doReq(t, s, http.MethodGet, "/api/reports/details", nil)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("details missing params status=%d", rr.Code)
	}

	rr = doReq(t, s, http.MethodGet, "/api/reports?ld=LD0&ln=LLN0", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	rr = doReq(t, s, http.MethodGet, "/api/reports/all", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("all status=%d body=%s", rr.Code, rr.Body.String())
	}
	rr = doReq(t, s, http.MethodGet, "/api/reports/names?ld=LD0&ln=LLN0", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("names status=%d body=%s", rr.Code, rr.Body.String())
	}
	rr = doReq(t, s, http.MethodGet, "/api/reports/details?ld=LD0&ln=LLN0&name=rcb1", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("details status=%d body=%s", rr.Code, rr.Body.String())
	}

	for _, path := range []string{
		"/api/reports?ld=LD0&ln=LLN0",
		"/api/reports/all",
		"/api/reports/names?ld=LD0&ln=LLN0",
		"/api/reports/details?ld=LD0&ln=LLN0&name=rcb1",
	} {
		rr = doReq(t, s, http.MethodPost, path, nil)
		if rr.Code != http.StatusMethodNotAllowed {
			t.Fatalf("%s method status=%d", path, rr.Code)
		}
	}
}

func TestJournals(t *testing.T) {
	s := newTestServer()

	rr := doReq(t, s, http.MethodGet, "/api/journals", nil)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("missing ld status=%d", rr.Code)
	}
	rr = doReq(t, s, http.MethodGet, "/api/journals/entries", nil)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("entries missing params status=%d", rr.Code)
	}
	rr = doReq(t, s, http.MethodGet, "/api/journals/entries?domain=LD0&journal=LLN0$EventLog&from=bad", nil)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("invalid from status=%d", rr.Code)
	}
	rr = doReq(t, s, http.MethodGet, "/api/journals/entries?domain=LD0&journal=LLN0$EventLog&from=1000&to=bad", nil)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("invalid to status=%d", rr.Code)
	}

	rr = doReq(t, s, http.MethodGet, "/api/journals?ld=LD0", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	rr = doReq(t, s, http.MethodGet, "/api/journals/entries?domain=LD0&journal=LLN0$EventLog&from=1000", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("entries status=%d body=%s", rr.Code, rr.Body.String())
	}
	rr = doReq(t, s, http.MethodGet, "/api/journals/entries?domain=LD0&journal=LLN0$EventLog&from=1000&to=2000", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("entries with to status=%d body=%s", rr.Code, rr.Body.String())
	}

	rr = doReq(t, s, http.MethodPost, "/api/journals?ld=LD0", nil)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("method status=%d", rr.Code)
	}
	rr = doReq(t, s, http.MethodPost, "/api/journals/entries?domain=LD0&journal=LLN0$EventLog&from=1000", nil)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("entries method status=%d", rr.Code)
	}
}

func TestFindPath(t *testing.T) {
	s := newTestServer()

	rr := doReq(t, s, http.MethodGet, "/api/find/path", nil)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET status=%d", rr.Code)
	}

	rr = doReq(t, s, http.MethodPost, "/api/find/path", []byte(`{`))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("invalid json status=%d", rr.Code)
	}

	payload, err := json.Marshal(app.FindPathInput{
		LNPattern:  "MMXU",
		DOName:     "Hz",
		IncludeDAs: true,
		Detailed:   true,
	})
	if err != nil {
		t.Fatal(err)
	}
	rr = doReq(t, s, http.MethodPost, "/api/find/path", payload)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}

	payload, err = json.Marshal(app.FindPathInput{
		LNPattern:  "MMXU",
		DOName:     "Hz",
		DAName:     "mag",
		IncludeDAs: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	rr = doReq(t, s, http.MethodPost, "/api/find/path", payload)
	if rr.Code != http.StatusOK {
		t.Fatalf("with DAName status=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestBulkFind(t *testing.T) {
	s := newTestServer()

	rr := doReq(t, s, http.MethodGet, "/api/find/bulk", nil)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET status=%d", rr.Code)
	}

	rr = doReq(t, s, http.MethodPost, "/api/find/bulk", []byte(`not-json`))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("invalid json status=%d", rr.Code)
	}

	payload, err := json.Marshal(app.BulkFindInput{
		Mappings: []service.BulkMappingEntry{
			{ControlledPropertyId: "freq", BaseLn: "MMXU", DoDaPath: "Hz.mag.f"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	rr = doReq(t, s, http.MethodPost, "/api/find/bulk", payload)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}

	// Empty mappings is a valid success path.
	rr = doReq(t, s, http.MethodPost, "/api/find/bulk", []byte(`{"mappings":[]}`))
	if rr.Code != http.StatusOK {
		t.Fatalf("empty mappings status=%d body=%s", rr.Code, rr.Body.String())
	}
}
