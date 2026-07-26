// SPDX-License-Identifier: MIT

package service

import (
	"bytes"
	"testing"
)

func TestTree_RenderDeviceTreeFlat_Shallow(t *testing.T) {
	mock := &mockConnection{
		logicalDevices: []string{"LD0"},
		logicalNodes:   []string{"MMXU1"},
		dataObjects:    []string{"DO1"},
	}
	var buf bytes.Buffer
	calls, err := NewTree(mock).RenderDeviceTreeFlat(&buf, "127.0.0.1", 102, "LD0/MMXU1.DO1")
	if err != nil {
		t.Fatalf("RenderDeviceTreeFlat: %v", err)
	}
	if calls == 0 {
		t.Fatal("expected MMS calls")
	}
	if buf.Len() == 0 {
		t.Fatal("expected flat output")
	}
}

func TestTree_RenderDeviceTree_Shallow(t *testing.T) {
	mock := &mockConnection{
		logicalDevices: []string{"LD0"},
		logicalNodes:   []string{"MMXU1"},
		dataObjects:    []string{"DO1"},
	}
	var buf bytes.Buffer
	calls, err := NewTree(mock).RenderDeviceTree(&buf, "host", 102, "LD0")
	if err != nil {
		t.Fatalf("RenderDeviceTree: %v", err)
	}
	if calls == 0 || !bytes.Contains(buf.Bytes(), []byte("LD0")) {
		t.Fatalf("calls=%d out=%q", calls, buf.String())
	}
}

func TestTree_BuildSerializableModel_WithDSAndReports(t *testing.T) {
	mock := &mockConnection{
		logicalDevices: []string{"LD0"},
		logicalNodes:   []string{"LLN0"},
		dataObjects:    []string{"DO1"},
		dataSets:       []string{"LLN0$ds1"},
		reports:        []string{"LLN0$BR$rcb1", "LLN0$RP$urcb1"},
	}
	ied, err := NewTree(mock).BuildSerializableModel("h", 102, "LD0/LLN0.DO1", true, true)
	if err != nil {
		t.Fatalf("BuildSerializableModel: %v", err)
	}
	if len(ied.LogicalDevices) != 1 || len(ied.LogicalDevices[0].LogicalNodes) != 1 {
		t.Fatalf("ied=%+v", ied)
	}
	ln := ied.LogicalDevices[0].LogicalNodes[0]
	if len(ln.DataSets) == 0 || len(ln.ReportControlBlocks) < 2 {
		t.Fatalf("ln=%+v", ln)
	}
}

func TestTree_Build(t *testing.T) {
	mock := &mockConnection{logicalDevices: []string{"LD0"}, logicalNodes: []string{"LLN0"}, dataObjects: []string{"DO1"}}
	ied, err := NewTree(mock).Build("LD0", false)
	if err != nil || ied == nil {
		t.Fatalf("Build: %v %+v", err, ied)
	}
}
