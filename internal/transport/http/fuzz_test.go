// SPDX-License-Identifier: MIT

package http

import (
	"encoding/json"
	"net/http"
	"testing"
)

// FuzzFindHandlers feeds arbitrary JSON bodies to POST /api/find/path and
// POST /api/find/bulk. Invariant: no panic, and the response is either 200
// with a JSON body or a 4xx/5xx with a JSON {"error": ...} body.
func FuzzFindHandlers(f *testing.F) {
	seeds := []string{
		`{}`,
		`{"LNPattern":"MMXU","DOName":"Hz"}`,
		`{"LNPattern":"MMXU","DOName":"Hz","DAName":"mag","IncludeDAs":true,"Detailed":true}`,
		`{"LNPattern":"(","DOName":"Hz"}`,
		`{"LNPattern":".*","DOName":""}`,
		`{"mappings":[]}`,
		`{"mappings":[{"controlledPropertyId":"f","baseLn":"MMXU","DoDaPath":"Hz.mag.f"}]}`,
		`{"mappings":[{"controlledPropertyId":"","baseLn":"","DoDaPath":""}]}`,
		`{"mappings":[{"baseLn":"(","DoDaPath":"..."}]}`,
		`{"mappings":null}`,
		`[]`,
		`null`,
		`{`,
		``,
	}
	for _, s := range seeds {
		f.Add(s)
	}
	srv := newTestServer()
	f.Fuzz(func(t *testing.T, body string) {
		for _, path := range []string{"/api/find/path", "/api/find/bulk"} {
			rr := doReq(t, srv, http.MethodPost, path, []byte(body))
			if rr.Code == http.StatusOK {
				if !json.Valid(rr.Body.Bytes()) {
					t.Fatalf("%s: 200 with invalid JSON body %q", path, rr.Body.String())
				}
				continue
			}
			if rr.Code < 400 || rr.Code > 599 {
				t.Fatalf("%s: unexpected status %d", path, rr.Code)
			}
			var eb errorBody
			if err := json.Unmarshal(rr.Body.Bytes(), &eb); err != nil || eb.Error == "" {
				t.Fatalf("%s: status %d without JSON error body: %q", path, rr.Code, rr.Body.String())
			}
		}
	})
}
