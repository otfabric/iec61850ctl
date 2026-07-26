// SPDX-License-Identifier: MIT

package server

import (
	"path/filepath"
	"runtime"
	"testing"

	iec61850 "github.com/otfabric/go-iec61850"
	"github.com/otfabric/go-iec61850/scl"
)

func TestRegisterInteropControls(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	icd := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "e2e", "testdata", "interop.icd")

	sclData, err := scl.ParseFile(icd)
	if err != nil {
		t.Fatalf("parse ICD: %v", err)
	}
	model, err := iec61850.NewServerModelFromSCL(sclData, "InteropIED", "")
	if err != nil {
		t.Fatalf("model: %v", err)
	}
	srv, err := iec61850.NewServer(model, iec61850.ServerOptions{
		Identity: &iec61850.ServerIdentity{Vendor: "test", Model: "test", Revision: "1"},
	})
	if err != nil {
		t.Fatalf("server: %v", err)
	}
	t.Cleanup(func() { srv.Close() })

	if err := registerInteropControls(srv); err != nil {
		t.Fatalf("register: %v", err)
	}
	// Second registration of the same control refs must fail.
	if err := registerInteropControls(srv); err == nil {
		t.Fatal("expected duplicate registration error")
	}
}

func TestRun_MissingSCL(t *testing.T) {
	err := Run(RunConfig{SclPath: "/nonexistent/path.icd", Host: "127.0.0.1", Port: 0})
	if err == nil {
		t.Fatal("expected error for missing SCL")
	}
}
