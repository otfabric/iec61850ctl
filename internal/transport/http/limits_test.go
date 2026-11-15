// SPDX-License-Identifier: MIT

package http

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"

	"github.com/otfabric/iec61850ctl/internal/app"
	"github.com/otfabric/iec61850ctl/pkg/service"
)

// serveAlloc serves one POST of body and returns the recorder and the bytes
// allocated while it was served. The body is built before the measurement.
// The tests that use it are not parallel, so other tests do not add to it.
func serveAlloc(t *testing.T, s *Server, path string, body []byte) (*httptest.ResponseRecorder, uint64) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	s.router.ServeHTTP(rr, req)
	runtime.ReadMemStats(&after)
	return rr, after.TotalAlloc - before.TotalAlloc
}

// findBody returns a /api/find/path body of exactly size bytes: the
// pattern, then an unknown field that pads it.
func findBody(t *testing.T, pattern string, size int) []byte {
	t.Helper()
	head := fmt.Sprintf(`{"ln_pattern":%q,"do_name":"Hz","pad":"`, pattern)
	if size < len(head)+2 {
		t.Fatalf("size %d below the %d bytes of the fields", size, len(head)+2)
	}
	return []byte(head + strings.Repeat("x", size-len(head)-2) + `"}`)
}

func errorCode(t *testing.T, rr *httptest.ResponseRecorder) string {
	t.Helper()
	var body errorBody
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("error body %q: %v", rr.Body.String(), err)
	}
	return body.Code
}

// TestFindBodyLimit: a body of up to maxFindBodyBytes is decoded, a larger
// one is answered with 413 after at most maxFindBodyBytes+1 bytes are read,
// for both find endpoints.
func TestFindBodyLimit(t *testing.T) {
	s := newTestServer()
	for _, c := range []struct {
		name   string
		size   int
		status int
	}{
		{"small", 64, http.StatusOK},
		{"limit-1", maxFindBodyBytes - 1, http.StatusOK},
		{"limit", maxFindBodyBytes, http.StatusOK},
		{"limit+1", maxFindBodyBytes + 1, http.StatusRequestEntityTooLarge},
		{"8 MiB", 8 << 20, http.StatusRequestEntityTooLarge},
	} {
		t.Run(c.name, func(t *testing.T) {
			rr, alloc := serveAlloc(t, s, "/api/find/path", findBody(t, "MMXU", c.size))
			if rr.Code != c.status {
				t.Fatalf("status=%d want %d body=%.200s", rr.Code, c.status, rr.Body.String())
			}
			if c.status != http.StatusOK && errorCode(t, rr) != codeInvalidInput {
				t.Fatalf("code %q want %q", errorCode(t, rr), codeInvalidInput)
			}
			// The decoder holds at most the limit; three decoding passes of
			// FindPathInput copy it a few times.
			if budget := uint64(6 * maxFindBodyBytes); alloc > budget {
				t.Fatalf("%d bytes allocated for a %d-byte body: more than %d", alloc, c.size, budget)
			}
		})
	}
	t.Run("bulk", func(t *testing.T) {
		body := []byte(`{"mappings":[{"baseLn":"` + strings.Repeat("M", maxFindBodyBytes) + `"}]}`)
		rr, alloc := serveAlloc(t, s, "/api/find/bulk", body)
		if rr.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("status=%d want 413", rr.Code)
		}
		if budget := uint64(6 * maxFindBodyBytes); alloc > budget {
			t.Fatalf("%d bytes allocated: more than %d", alloc, budget)
		}
	})
}

// TestFindPatternLimit: ln_pattern is compiled only up to maxLNPatternLen
// bytes; a pattern of that length that expands the most stays within a
// fixed allocation.
func TestFindPatternLimit(t *testing.T) {
	s := newTestServer()
	// Each "(M|X){1000}" compiles to about 600 KiB of program.
	costly := strings.Repeat("(M|X){1000}", maxLNPatternLen/len("(M|X){1000}"))
	costly += strings.Repeat("M", maxLNPatternLen-len(costly))
	for _, c := range []struct {
		name    string
		pattern string
		status  int
		budget  uint64
	}{
		{"1 byte", "M", http.StatusOK, 1 << 20},
		{"limit-1", strings.Repeat("M", maxLNPatternLen-1), http.StatusOK, 1 << 20},
		{"limit", strings.Repeat("M", maxLNPatternLen), http.StatusOK, 1 << 20},
		{"limit, largest program", costly, http.StatusOK, 32 << 20},
		{"limit+1", strings.Repeat("M", maxLNPatternLen+1), http.StatusBadRequest, 1 << 20},
		{"64 KiB", strings.Repeat("(M|X){1000}", 64<<10/11), http.StatusBadRequest, 2 << 20},
	} {
		t.Run(c.name, func(t *testing.T) {
			body, err := json.Marshal(app.FindPathInput{LNPattern: c.pattern, DOName: "Hz"})
			if err != nil {
				t.Fatal(err)
			}
			rr, alloc := serveAlloc(t, s, "/api/find/path", body)
			if rr.Code != c.status {
				t.Fatalf("status=%d want %d body=%.200s", rr.Code, c.status, rr.Body.String())
			}
			if c.status == http.StatusBadRequest && errorCode(t, rr) != codeInvalidInput {
				t.Fatalf("code %q", errorCode(t, rr))
			}
			if alloc > c.budget {
				t.Fatalf("%d bytes allocated for a %d-byte pattern: more than %d", alloc, len(c.pattern), c.budget)
			}
		})
	}
}

// TestBulkMappingsLimit: up to maxBulkMappings mappings are resolved, more
// are refused with 400 before the device is asked; distinct baseLn values
// cost no compiled pattern each.
func TestBulkMappingsLimit(t *testing.T) {
	s := newTestServer()
	mappings := func(n int) []service.BulkMappingEntry {
		m := make([]service.BulkMappingEntry, n)
		for i := range m {
			// Distinct baseLn values that fill the body: about 100 bytes of the body each.
			m[i] = service.BulkMappingEntry{ControlledPropertyId: "p", BaseLn: fmt.Sprintf("%040d", i), DoDaPath: "Hz"}
		}
		return m
	}
	for _, c := range []struct {
		name   string
		n      int
		status int
	}{
		{"1", 1, http.StatusOK},
		{"limit-1", maxBulkMappings - 1, http.StatusOK},
		{"limit", maxBulkMappings, http.StatusOK},
		{"limit+1", maxBulkMappings + 1, http.StatusBadRequest},
	} {
		t.Run(c.name, func(t *testing.T) {
			body, err := json.Marshal(app.BulkFindInput{Mappings: mappings(c.n)})
			if err != nil {
				t.Fatal(err)
			}
			if len(body) > maxFindBodyBytes {
				t.Fatalf("body of %d bytes exceeds the body limit; the case tests the mapping limit", len(body))
			}
			rr, alloc := serveAlloc(t, s, "/api/find/bulk", body)
			if rr.Code != c.status {
				t.Fatalf("status=%d want %d body=%.200s", rr.Code, c.status, rr.Body.String())
			}
			if c.status == http.StatusBadRequest && errorCode(t, rr) != codeInvalidInput {
				t.Fatalf("code %q", errorCode(t, rr))
			}
			if budget := uint64(16 << 20); alloc > budget {
				t.Fatalf("%d bytes allocated for %d mappings: more than %d", alloc, c.n, budget)
			}
		})
	}
}
