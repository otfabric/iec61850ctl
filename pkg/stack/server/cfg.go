// SPDX-License-Identifier: MIT

// Package server generates libiec61850 .cfg from serialized IED JSON (export-only).
package server

import (
	"bytes"
	"fmt"
	"strconv"

	"github.com/otfabric/iec61850ctl/pkg/domain"
)

// IEDToCfg generates libiec61850 .cfg file content from domain.IED.
// iedName is the MODEL name (MMS domain = iedName+LDName per libiec61850). Use "" for domain = LD name only.
// DA format: DA(name arrayElements type fc triggerOptions sAddr). arrayElements=0 for non-array.
// See https://libiec61850.com/configuration-file-format and server_example_config_file/model.cfg.
func IEDToCfg(ied *domain.IED, iedName string) ([]byte, error) {
	if ied == nil || len(ied.LogicalDevices) == 0 {
		return nil, fmt.Errorf("IED has no logical devices")
	}
	var b bytes.Buffer
	fmt.Fprintf(&b, "MODEL(%s){\n", iedName)
	for _, ld := range ied.LogicalDevices {
		fmt.Fprintf(&b, "LD(%s){\n", ld.Name)
		for _, ln := range ld.LogicalNodes {
			fmt.Fprintf(&b, "LN(%s){\n", ln.Name)
			for _, do := range ln.DataObjects {
				writeDO(&b, &do)
			}
			b.WriteString("}\n")
		}
		b.WriteString("}\n")
	}
	b.WriteString("}\n")
	return b.Bytes(), nil
}

func writeDO(b *bytes.Buffer, do *domain.DataObject) {
	fmt.Fprintf(b, "DO(%s 0){\n", do.Name)
	for i := range do.Attributes {
		writeDA(b, &do.Attributes[i])
	}
	b.WriteString("}\n")
}

// daTriggerOption returns triggerOptions: 2=quality changed (q), 4=data update (ctlModel), 1=data changed, 0=other.
func daTriggerOption(daName string) int {
	switch daName {
	case "q":
		return 2
	case "ctlModel":
		return 4
	default:
		return 1
	}
}

// dataAttributeTypeCode returns libiec61850 DataAttributeType enum value (iec61850_model.h)
// for use in .cfg DA(type). Not the same as MmsType (client API).
const (
	daTypeBoolean     = 0
	daTypeInt8        = 1
	daTypeInt16       = 2
	daTypeInt32       = 3
	daTypeInt64       = 4
	daTypeInt8U       = 6
	daTypeInt16U      = 7
	daTypeInt32U      = 9
	daTypeFloat32     = 10
	daTypeEnum        = 12
	daTypeOctet8      = 15
	daTypeVisStr      = 20
	daTypeTimestamp   = 22
	daTypeQuality     = 23 // BIT_STRING for quality (q)
	daTypeBitString   = 26 // generic BIT_STRING
	daTypeConstructed = 27
)

func dataAttributeTypeCode(da *domain.DataAttribute) int {
	if len(da.Children) > 0 {
		return daTypeConstructed
	}
	// Quality attribute "q" uses IEC61850_QUALITY (23); other BIT_STRING use GENERIC_BITSTRING (26).
	if da.Type == domain.TypeBitString {
		if da.Name == "q" {
			return daTypeQuality
		}
		return daTypeBitString
	}
	// ctlModel is typically ENUMERATED in IEC 61850 (type 12).
	if da.Name == "ctlModel" && (da.Type == domain.TypeInt8 || da.Type == domain.TypeInt16 || da.Type == domain.TypeInt32) {
		return daTypeEnum
	}
	switch da.Type {
	case domain.TypeBoolean:
		return daTypeBoolean
	case domain.TypeInt8:
		return daTypeInt8
	case domain.TypeInt16:
		return daTypeInt16
	case domain.TypeInt32:
		return daTypeInt32
	case domain.TypeInt64:
		return daTypeInt64
	case domain.TypeUint8:
		return daTypeInt8U
	case domain.TypeUint16:
		return daTypeInt16U
	case domain.TypeUint32:
		return daTypeInt32U
	case domain.TypeFloat:
		return daTypeFloat32
	case domain.TypeVisibleString, domain.TypeMmsString:
		return daTypeVisStr
	case domain.TypeOctetString:
		return daTypeOctet8
	case domain.TypeUtcTime:
		return daTypeTimestamp
	case domain.TypeStructure:
		return daTypeConstructed
	default:
		return daTypeInt32 // fallback
	}
}

// libiec61850 FunctionalConstraint ordinals used by .cfg export.
func fcOrdinal(fc domain.FunctionalConstraint) int {
	switch fc {
	case domain.FC_ST:
		return 0
	case domain.FC_MX:
		return 1
	case domain.FC_SP:
		return 2
	case domain.FC_SV:
		return 3
	case domain.FC_CF:
		return 4
	case domain.FC_DC:
		return 5
	case domain.FC_SG:
		return 6
	case domain.FC_SE:
		return 7
	case domain.FC_SR:
		return 8
	case domain.FC_OR:
		return 9
	case domain.FC_BL:
		return 10
	case domain.FC_EX:
		return 11
	case domain.FC_CO:
		return 12
	case domain.FC_US:
		return 13
	case domain.FC_MS:
		return 14
	case domain.FC_RP:
		return 15
	case domain.FC_BR:
		return 16
	case domain.FC_LG:
		return 17
	default:
		return 0
	}
}

// writeDA emits one DA line. Type code is DataAttributeType (libiec61850 server), not MmsType.
func writeDA(b *bytes.Buffer, da *domain.DataAttribute) {
	fc := fcOrdinal(da.FC)
	typeCode := dataAttributeTypeCode(da)
	if len(da.Children) > 0 {
		fmt.Fprintf(b, "DA(%s 0 %d %d 1 0){\n", da.Name, typeCode, fc)
		for i := range da.Children {
			writeDA(b, &da.Children[i])
		}
		b.WriteString("}\n")
		return
	}
	trig := daTriggerOption(da.Name)
	line := fmt.Sprintf("DA(%s 0 %d %d %d 0)", da.Name, typeCode, fc, trig)
	if v := formatCfgValue(da); v != "" {
		line += "=" + v
	}
	line += ";\n"
	b.WriteString(line)
}

// toInt64 converts a JSON-unmarshalled number (float64/int/etc.) to int64.
func toInt64(v interface{}) int64 {
	if v == nil {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return int64(n)
	case int:
		return int64(n)
	case int64:
		return n
	case int32:
		return int64(n)
	case uint64:
		return int64(n)
	default:
		return 0
	}
}

func formatCfgValue(da *domain.DataAttribute) string {
	if da.Value == nil {
		return ""
	}
	raw := da.Value.Raw
	if raw == nil {
		return ""
	}
	// UTC_TIME is serialized as {"seconds", "milliseconds", "time_quality"}; combine for .cfg value (ms since epoch).
	if da.Type == domain.TypeUtcTime {
		m, ok := raw.(map[string]interface{})
		if !ok || (m["seconds"] == nil && m["milliseconds"] == nil) {
			return ""
		}
		sec := toInt64(m["seconds"])
		ms := toInt64(m["milliseconds"])
		epochMs := sec*1000 + ms
		return strconv.FormatInt(epochMs, 10)
	}
	switch v := raw.(type) {
	case bool:
		if v {
			return "1"
		}
		return "0"
	case float64:
		// Use integer form for whole numbers (e.g. UTC_TIME ms, enums)
		if v == float64(int64(v)) {
			return strconv.FormatInt(int64(v), 10)
		}
		return strconv.FormatFloat(v, 'g', -1, 64)
	case int:
		return strconv.Itoa(v)
	case int32:
		return strconv.FormatInt(int64(v), 10)
	case int64:
		return strconv.FormatInt(v, 10)
	case uint, uint32, uint64:
		return fmt.Sprintf("%d", v)
	case string:
		return strconv.Quote(v)
	default:
		// UTC_TIME and other numbers may unmarshal as float64
		if f, ok := raw.(float64); ok {
			return strconv.FormatFloat(f, 'f', 0, 64)
		}
		return fmt.Sprintf("%v", v)
	}
}
