//go:build e2e

// SPDX-License-Identifier: MIT

package e2e_test

import (
	"encoding/json"
	"testing"
)

// Phase 4 forward control journeys use isolated adapter servers (mutating).
//
// | Assertion                                      | libiec61850 | iec61850bean |
// |------------------------------------------------|-------------|--------------|
// | SPCSO1 direct operate confirmed                | required    | required     |
// | SPCSO2 SBO select+operate confirmed            | required    | required     |
// | SPCSO3 SBOw select-with-value+operate          | required    | known fail   |
// | Bean SPCSO3: non-zero exit, stVal unchanged    | n/a         | required     |

func TestControl_ClientOperate(t *testing.T) {
	type journey struct {
		name   string
		object string
		ops    []string
		beanOK bool // false => registered Bean SBOw failure
	}
	journeys := []journey{
		{name: "SPCSO1_Direct", object: "InteropLD/GGIO1.SPCSO1", ops: []string{"operate"}, beanOK: true},
		{name: "SPCSO2_SBO", object: "InteropLD/GGIO1.SPCSO2", ops: []string{"select", "operate"}, beanOK: true},
		{name: "SPCSO3_SBOw", object: "InteropLD/GGIO1.SPCSO3", ops: []string{"select-with-value", "operate"}, beanOK: false},
	}

	for _, stack := range []string{stackLibIEC61850, stackIEC61850Bean} {
		stack := stack
		for _, j := range journeys {
			j := j
			t.Run(stack+"/"+j.name, func(t *testing.T) {
				requireStack(t, stack)
				requireCtlBin(t)

				expectOK := stack != stackIEC61850Bean || j.beanOK

				readyTO := libReadyTimeout
				if stack == stackIEC61850Bean {
					readyTO = beanReadyTimeout
				}
				h, err := startDockerServer(stack, readyTO)
				if err != nil {
					t.Fatalf("start isolated %s: %v", stack, err)
				}
				t.Cleanup(h.stop)

				inspect := runCtl(t, h.ctlEnv(), commandTimeout,
					"control", "inspect",
					"--object", j.object,
					"--format", "json",
				)
				requireExitZero(t, inspect, h)

				confirmRef := j.object + ".stVal[ST]"
				op := runCtl(t, h.ctlEnv(), commandTimeout,
					"control", "operate",
					"--object", j.object,
					"--value", "true",
					"--type", "bool",
					"--mode", "auto",
					"--confirm-ref", confirmRef,
					"--format", "json",
				)

				var result struct {
					Status     string `json:"status"`
					CtlNum     int    `json:"ctl_num"`
					Operations []struct {
						Operation string `json:"operation"`
						OK        bool   `json:"ok"`
					} `json:"operations"`
				}
				if err := decodeExactJSON(op.Stdout, &result); err != nil {
					logCommandFailure(t, op, h)
					t.Fatalf("decode control operate JSON: %v", err)
				}

				if expectOK {
					requireExitZero(t, op, h)
					if result.Status != "confirmed" {
						t.Fatalf("status=%q want confirmed", result.Status)
					}
					gotOps := make([]string, 0, len(result.Operations))
					for _, o := range result.Operations {
						if !o.OK {
							t.Fatalf("operation %s not ok", o.Operation)
						}
						gotOps = append(gotOps, o.Operation)
					}
					if len(gotOps) != len(j.ops) {
						t.Fatalf("ops=%v want %v", gotOps, j.ops)
					}
					for i := range j.ops {
						if gotOps[i] != j.ops[i] {
							t.Fatalf("ops[%d]=%q want %q", i, gotOps[i], j.ops[i])
						}
					}
					if result.CtlNum == 0 {
						t.Fatal("ctl_num must be non-zero")
					}

					get := runCtl(t, h.ctlEnv(), commandTimeout,
						"get", "object",
						"--object", j.object+".stVal",
						"--fc", "ST",
						"--format", "json",
					)
					requireExitZero(t, get, h)
					var st struct {
						Value any `json:"value"`
					}
					if err := decodeExactJSON(get.Stdout, &st); err != nil {
						t.Fatalf("decode stVal: %v", err)
					}
					if b, ok := st.Value.(bool); !ok || !b {
						t.Fatalf("stVal=%#v want true", st.Value)
					}
					return
				}

				// Bean SBOw known limitation: deterministic failure, no false success.
				if op.ExitCode == 0 {
					t.Fatal("Bean SPCSO3 SBOw: expected non-zero exit (registered limitation)")
				}
				if result.Status == "confirmed" || result.Status == "operated" {
					t.Fatalf("Bean SPCSO3 SBOw: unexpected success status %q", result.Status)
				}
				get := runCtl(t, h.ctlEnv(), commandTimeout,
					"get", "object",
					"--object", j.object+".stVal",
					"--fc", "ST",
					"--format", "json",
				)
				requireExitZero(t, get, h)
				var st struct {
					Value any `json:"value"`
				}
				if err := json.Unmarshal(get.Stdout, &st); err != nil {
					t.Fatalf("decode stVal: %v", err)
				}
				if b, ok := st.Value.(bool); ok && b {
					t.Fatal("Bean SPCSO3 SBOw: stVal changed to true (false success)")
				}
			})
		}
	}
}
