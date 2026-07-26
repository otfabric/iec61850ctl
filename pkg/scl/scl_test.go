// SPDX-License-Identifier: MIT

package scl

import (
	"strings"
	"testing"
)

const minimalCID = `<?xml version="1.0"?>
<SCL xmlns="http://www.iec.ch/61850/2003/SCL">
  <IED name="TEST1">
    <AccessPoint name="LD0">
      <Server>
        <LDevice inst="LD0">
          <LN0 lnClass="LLN0" lnType="LLN0_Type">
            <DOI name="Beh"/>
          </LN0>
        </LDevice>
      </Server>
    </AccessPoint>
  </IED>
  <DataTypeTemplates>
    <LNodeType id="LLN0_Type" lnClass="LLN0">
      <DO name="Beh" type="ENS_Beh"/>
    </LNodeType>
    <DOType id="ENS_Beh" cdc="ENS">
      <DA name="stVal" bType="Enum" fc="ST"/>
      <DA name="q" bType="Quality" fc="ST"/>
      <DA name="t" bType="Timestamp" fc="ST"/>
    </DOType>
  </DataTypeTemplates>
</SCL>`

func TestParseMinimal(t *testing.T) {
	doc, err := Parse(strings.NewReader(minimalCID))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(doc.IED) != 1 {
		t.Fatalf("expected 1 IED, got %d", len(doc.IED))
	}
	if doc.IED[0].Name != "TEST1" {
		t.Errorf("IED name: got %q", doc.IED[0].Name)
	}
	if len(doc.DataTypeTemplates.LNodeType) != 1 {
		t.Fatalf("expected 1 LNodeType, got %d", len(doc.DataTypeTemplates.LNodeType))
	}
}

func TestFlattenMinimal(t *testing.T) {
	doc, err := Parse(strings.NewReader(minimalCID))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	entries, err := doc.Flatten()
	if err != nil {
		t.Fatalf("Flatten: %v", err)
	}
	// LLN0 with empty inst: LN name = "" + "LLN0" + "" = "LLN0"
	// Prefix = TEST1 + LD0 = TEST1LD0
	// We expect paths like TEST1LD0/LLN0.Beh.stVal, .Beh.q, .Beh.t
	if len(entries) < 3 {
		t.Errorf("expected at least 3 entries (Beh.stVal, q, t), got %d", len(entries))
	}
	var found bool
	for _, e := range entries {
		if strings.Contains(e.Path, "Beh.stVal") && e.FC == "ST" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected an entry for Beh.stVal[ST]; got %d entries: %v", len(entries), entries)
	}
}
