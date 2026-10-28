// SPDX-License-Identifier: MIT

package cmd

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/otfabric/iec61850ctl/pkg/stack/client"
)

func TestBuildStructAttrPath(t *testing.T) {
	t.Parallel()
	cases := []struct{ object, parent, attr, want string }{
		{"LD/LN.DO", "", "", "LD/LN.DO"},
		{"LD/LN.DO", "", "stVal", "LD/LN.DO.stVal"},
		{"LD/LN.DO.Oper", "Oper", "", "LD/LN.DO.Oper"},
		{"LD/LN.DO.Oper", "Oper", "Oper", "LD/LN.DO.Oper"},
		{"LD/LN.DO.Oper", "Oper", "ctlVal", "LD/LN.DO.Oper.ctlVal"},
		{"LD/LN.DO.Oper", "Oper", "Oper.ctlVal", "LD/LN.DO.Oper.Oper.ctlVal"},
		{"LD/LN.DO", "Oper", "", "LD/LN.DO.Oper"},
		{"LD/LN.DO", "Oper", "Oper", "LD/LN.DO.Oper"},
		{"LD/LN.DO", "Oper", "ctlVal", "LD/LN.DO.Oper.ctlVal"},
		{"LD/LN.DO", "Oper", "Oper.ctlVal", "LD/LN.DO.Oper.ctlVal"},
	}
	for _, c := range cases {
		if got := buildStructAttrPath(c.object, c.parent, c.attr); got != c.want {
			t.Errorf("buildStructAttrPath(%q,%q,%q)=%q want %q", c.object, c.parent, c.attr, got, c.want)
		}
	}
}

func TestParseObjectRef(t *testing.T) {
	t.Parallel()
	cases := []struct{ in, ld, ln, do string }{
		{"noslash", "", "", ""},
		{"LD/LLN0", "LD", "LLN0", ""},
		{"LD/GGIO1.SPCSO1.stVal", "LD", "GGIO1", "SPCSO1.stVal"},
	}
	for _, c := range cases {
		ld, ln, do := parseObjectRef(c.in)
		if ld != c.ld || ln != c.ln || do != c.do {
			t.Errorf("parseObjectRef(%q)=%q,%q,%q", c.in, ld, ln, do)
		}
	}
}

func TestDADisplayName(t *testing.T) {
	t.Parallel()
	cases := []struct{ parent, name, want string }{
		{"q", "", "q"},
		{"q", "q", "q"},
		{"", "stVal", "stVal"},
		{"Oper", "Oper.ctlVal", "Oper.ctlVal"},
		{"Oper", "ctlVal", "Oper.ctlVal"},
	}
	for _, c := range cases {
		if got := daDisplayName(c.parent, c.name); got != c.want {
			t.Errorf("daDisplayName(%q,%q)=%q want %q", c.parent, c.name, got, c.want)
		}
	}
}

// closeConn fails Close and optionally Abort.
type closeConn struct {
	emptyConn
	abortErr error
}

func (closeConn) Close(context.Context) error   { return errors.New("close boom") }
func (c closeConn) Abort(context.Context) error { return c.abortErr }

func TestSessionClose(t *testing.T) {
	t.Parallel()
	var nilSession *clientSession
	nilSession.Close()
	if err := nilSession.CloseStrict(); err != nil {
		t.Fatal(err)
	}
	if err := (&clientSession{conn: emptyConn{}}).CloseStrict(); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	(&clientSession{conn: closeConn{abortErr: errors.New("abort boom")}, stderr: &buf}).Close()
	if !strings.Contains(buf.String(), "close boom") || !strings.Contains(buf.String(), "abort boom") {
		t.Fatalf("stderr=%q", buf.String())
	}

	err := (&clientSession{conn: closeConn{}}).CloseStrict()
	if err == nil || !strings.Contains(err.Error(), "aborted") {
		t.Fatalf("err=%v", err)
	}
	err = (&clientSession{conn: closeConn{abortErr: errors.New("abort boom")}}).CloseStrict()
	if err == nil || !strings.Contains(err.Error(), "abort boom") {
		t.Fatalf("err=%v", err)
	}
}

func TestLoggerAndTimeoutDefaults(t *testing.T) {
	cliMu.Lock()
	defer cliMu.Unlock()
	saved, savedTimeout := runState.logger, requestTimeoutFlag
	defer func() { runState.logger, requestTimeoutFlag = saved, savedTimeout }()

	runState.logger = nil
	if cmdLogger() == nil {
		t.Fatal("nil logger")
	}
	requestTimeoutFlag = 0
	if requestTimeout() != client.DefaultRequestTimeout {
		t.Fatal("default timeout")
	}
	requestTimeoutFlag = 3 * time.Second
	if requestTimeout() != 3*time.Second {
		t.Fatal("explicit timeout")
	}
}
