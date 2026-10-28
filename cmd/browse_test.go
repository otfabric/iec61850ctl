// SPDX-License-Identifier: MIT

package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/otfabric/iec61850ctl/pkg/errs"
)

// TestBrowseGolden drives read-only commands against the in-process server
// and compares stdout (normalised) with golden files, for every output format
// each command supports. Regenerate with `go test ./cmd -run TestBrowseGolden -update`.
func TestBrowseGolden(t *testing.T) {
	ld := []string{"--ld", "InteropLD"}
	lln0 := append(append([]string{}, ld...), "--ln", "LLN0")
	ggio := append(append([]string{}, ld...), "--ln", "GGIO1")
	cases := []struct {
		name string
		args []string
	}{
		{"list_lds_text", []string{"list", "lds"}},
		{"list_lds_detailed", []string{"list", "lds", "--detailed"}},
		{"list_lds_json", []string{"list", "lds", "--format", "json"}},
		{"list_lds_table", []string{"list", "lds", "--format", "table"}},
		{"list_lds_csv", []string{"list", "lds", "--format", "csv"}},
		{"list_lds_yaml", []string{"list", "domains", "--format", "yaml"}},
		{"list_lns_text", append([]string{"list", "lns"}, ld...)},
		{"list_lns_detailed", append([]string{"list", "lns", "--detailed"}, ld...)},
		{"list_lns_json", append([]string{"list", "lns", "--format", "json"}, ld...)},
		{"list_lns_table", append([]string{"list", "lns", "--format", "table"}, ld...)},
		{"list_lns_csv", append([]string{"list", "lns", "--format", "csv"}, ld...)},
		{"list_lns_yaml", append([]string{"list", "lns", "--format", "yaml"}, ld...)},
		{"list_dos_text", append([]string{"list", "dos"}, ggio...)},
		{"list_dos_detailed", append([]string{"list", "dos", "--detailed"}, ggio...)},
		{"list_dos_json", append([]string{"list", "dos", "--format", "json"}, ggio...)},
		{"list_dos_table", append([]string{"list", "dos", "--format", "table"}, ggio...)},
		{"list_dos_csv", append([]string{"list", "dos", "--format", "csv"}, ggio...)},
		{"list_dos_yaml", append([]string{"list", "dos", "--format", "yaml"}, ggio...)},
		{"list_das_text", append([]string{"list", "das", "--do", "SPS1"}, ggio...)},
		{"list_das_detailed", append([]string{"list", "das", "--do", "SPS1", "--detailed"}, ggio...)},
		{"list_das_json", append([]string{"list", "das", "--do", "SPS1", "--format", "json"}, ggio...)},
		{"list_das_table", append([]string{"list", "das", "--do", "SPS1", "--format", "table"}, ggio...)},
		{"list_das_csv", append([]string{"list", "das", "--do", "SPS1", "--format", "csv"}, ggio...)},
		{"list_das_yaml", append([]string{"list", "das", "--do", "SPS1", "--format", "yaml"}, ggio...)},
		{"list_dss_text", append([]string{"list", "dss"}, lln0...)},
		{"list_dss_detailed", append([]string{"list", "dss", "--detailed"}, lln0...)},
		{"list_dss_json", append([]string{"list", "dss", "--format", "json"}, lln0...)},
		{"list_dss_json_detailed", append([]string{"list", "dss", "--format", "json", "--detailed"}, lln0...)},
		{"list_reports_text", append([]string{"list", "reports"}, lln0...)},
		{"list_reports_detailed", append([]string{"list", "reports", "--detailed"}, lln0...)},
		{"list_reports_json", append([]string{"list", "reports", "--format", "json"}, lln0...)},
		{"list_reports_all", []string{"list", "reports", "--all"}},
		{"list_reports_all_json", []string{"list", "reports", "--all", "--format", "json"}},
		{"list_reports_all_detailed", []string{"list", "reports", "--all", "--detailed"}},
		{"list_files_text", []string{"list", "files"}},
		{"list_files_recursive", []string{"list", "files", "--recursive"}},
		{"list_files_pattern", []string{"list", "files", "--path", "*.cfg"}},
		{"get_object_text", []string{"get", "object", "--object", "InteropLD/MMXU1.TotW.mag.f"}},
		{"get_object_detailed", []string{"get", "object", "--object", "InteropLD/MMXU1.TotW.mag.f", "--detailed"}},
		{"get_object_json", []string{"get", "object", "--object", "InteropLD/MMXU1.TotW.mag.f", "--fc", "MX", "--format", "json"}},
		{"get_object_bool_json", []string{"get", "object", "--object", "InteropLD/GGIO1.SPS1.stVal", "--fc", "ST", "--format", "json"}},
		{"get_object_struct_detailed", []string{"get", "object", "--object", "InteropLD/GGIO1.SPS1", "--fc", "ST", "--detailed"}},
		{"get_object_fc_suffix", []string{"get", "object", "--object", "InteropLD/LLN0.Mod.stVal[ST]", "--fc", "ST", "--detailed"}},
		{"get_ds_text", append([]string{"get", "ds", "--name", "dsInterop"}, lln0...)},
		{"get_ds_detailed", append([]string{"get", "ds", "--name", "dsInterop", "--detailed"}, lln0...)},
		{"get_ds_json", append([]string{"get", "ds", "--name", "dsInterop", "--format", "json"}, lln0...)},
		{"get_ds_json_detailed", append([]string{"get", "ds", "--name", "dsInterop", "--format", "json", "--detailed"}, lln0...)},
		{"get_report_text", append([]string{"get", "report", "--report", "urcb01"}, lln0...)},
		{"get_report_detailed", append([]string{"get", "report", "--report", "brcb01", "--detailed"}, lln0...)},
		{"get_report_json", append([]string{"get", "report", "--report", "urcb01", "--format", "json"}, lln0...)},
		{"get_report_json_detailed", append([]string{"get", "report", "--report", "brcb01", "--format", "json", "--detailed"}, lln0...)},
		{"control_inspect_text", []string{"control", "inspect", "--object", "InteropLD/GGIO1.SPCSO3"}},
		{"control_inspect_json", []string{"control", "inspect", "--object", "InteropLD/GGIO1.SPCSO1", "--format", "json"}},
		{"control_dry_run", []string{"control", "operate", "--object", "InteropLD/GGIO1.SPCSO2", "--dry-run", "--type", "bool", "--value", "true", "--format", "json"}},
		{"find_path_text", []string{"find", "path", "--ln", "GGIO.*", "--path", "SPCSO1"}},
		{"find_path_detailed", []string{"find", "path", "--ln", "MMXU.*", "--path", "TotW.mag", "--include-das", "--detailed"}},
		{"tree_path", []string{"tree", "--path", "InteropLD/MMXU1"}},
		{"tree_path_do", []string{"tree", "--path", "InteropLD/MMXU1.TotW"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := runSrv(t, tc.args...)
			expectCode(t, r, errs.ExitOK)
			if !strings.Contains(r.stderr, "Connecting to 127.0.0.1:") {
				t.Errorf("stderr lacks connect line: %q", r.stderr)
			}
			if strings.Contains(tc.name, "json") {
				var v any
				dec := json.NewDecoder(strings.NewReader(r.stdout))
				if err := dec.Decode(&v); err != nil {
					t.Fatalf("stdout is not JSON: %v\n%s", err, r.stdout)
				}
			}
			golden(t, tc.name, r.stdout)
		})
	}
}
