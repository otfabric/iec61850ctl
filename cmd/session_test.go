// SPDX-License-Identifier: MIT

package cmd

import (
	"bytes"
	"os"
	"testing"
)

func TestGetIEDName_Precedence(t *testing.T) {
	origFlag := iedName
	origEnv := os.Getenv("IEC61850_IED_NAME")
	t.Cleanup(func() {
		iedName = origFlag
		_ = os.Setenv("IEC61850_IED_NAME", origEnv)
	})

	iedName = ""
	_ = os.Setenv("IEC61850_IED_NAME", "FromEnv")
	if got := getIEDName(); got != "FromEnv" {
		t.Fatalf("env: got %q", got)
	}

	iedName = "FromFlag"
	if got := getIEDName(); got != "FromFlag" {
		t.Fatalf("flag should win: got %q", got)
	}

	iedName = ""
	_ = os.Unsetenv("IEC61850_IED_NAME")
	if got := getIEDName(); got != "" {
		t.Fatalf("empty: got %q", got)
	}
}

func TestClientSession_CloseNilSafe(t *testing.T) {
	var s *clientSession
	s.Close()
	s = &clientSession{stderr: &bytes.Buffer{}}
	s.Close()
}
