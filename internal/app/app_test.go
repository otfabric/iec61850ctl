// SPDX-License-Identifier: MIT

package app

import (
	"testing"

	"github.com/otfabric/iec61850ctl/pkg/domain"
	"github.com/otfabric/iec61850ctl/pkg/service"
)

// Compile-time check that App constructors accept IEC61850Connection.
func TestApp_NewAcceptsNilConn(t *testing.T) {
	a := New(nil)
	if a == nil {
		t.Fatal("New returned nil")
	}
	if a.Explorer() == nil || a.Reader() == nil || a.ReportService() == nil {
		t.Fatal("expected services to be constructible")
	}
}

func TestProjectLogicalDevices_Empty(t *testing.T) {
	out := service.ProjectLogicalDevices(nil)
	if len(out) != 0 {
		t.Fatalf("got %d", len(out))
	}
	out = service.ProjectLogicalDevices([]domain.LogicalDevice{{Name: "LD1", LNCount: 2, DSCount: 1}})
	if len(out) != 1 || out[0].Name != "LD1" || out[0].LNCount != 2 {
		t.Fatalf("unexpected projection: %+v", out)
	}
}
