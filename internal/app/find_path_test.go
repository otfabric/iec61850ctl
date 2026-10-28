// SPDX-License-Identifier: MIT

package app

import (
	"encoding/json"
	"testing"
)

func TestFindPathInputJSON(t *testing.T) {
	t.Parallel()
	full := FindPathInput{LNPattern: "GGIO", DOName: "SPCSO1", DAName: "stVal", IncludeDAs: true, Detailed: true}
	tests := []struct {
		name string
		body string
		want FindPathInput
	}{
		{"snake_case", `{"ln_pattern":"GGIO","do_name":"SPCSO1","da_name":"stVal","include_das":true,"detailed":true}`, full},
		{"legacy PascalCase", `{"LNPattern":"GGIO","DOName":"SPCSO1","DAName":"stVal","IncludeDAs":true,"Detailed":true}`, full},
		{"snake_case wins", `{"ln_pattern":"MMXU","LNPattern":"GGIO","do_name":"Hz","DOName":"SPCSO1","include_das":false,"IncludeDAs":true}`,
			FindPathInput{LNPattern: "MMXU", DOName: "Hz"}},
		{"empty", `{}`, FindPathInput{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var got FindPathInput
			if err := json.Unmarshal([]byte(tt.body), &got); err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("got %+v, want %+v", got, tt.want)
			}
		})
	}

	b, err := json.Marshal(full)
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"ln_pattern":"GGIO","do_name":"SPCSO1","da_name":"stVal","include_das":true,"detailed":true}`; string(b) != want {
		t.Fatalf("marshal = %s, want %s", b, want)
	}
	for _, bad := range []string{`[]`, `{"ln_pattern":1}`, `{"LNPattern":1}`} {
		if err := json.Unmarshal([]byte(bad), &FindPathInput{}); err == nil {
			t.Errorf("%s: expected error", bad)
		}
	}
}
