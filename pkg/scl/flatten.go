// SPDX-License-Identifier: MIT

package scl

import (
	"fmt"
	"sort"
	"strings"
)

// FlattenEntry is one line of flattened output.
type FlattenEntry struct {
	Path     string // e.g. "IED/LD/LN.DO.DA"
	FC       string // function constraint: CF, ST, DC, CO, etc.
	Value    string
	BType    string // e.g. INT8, BOOL, UTC_TIME
	EnumVals string // comma-separated allowed values when type is Enum (e.g. "on,blocked,test,off")
}

// Flatten walks the SCL and produces flattened LD/LN/DO/DA entries with fc and type.
func (s *SCL) Flatten() ([]FlattenEntry, error) {
	// Build type lookups
	lnTypes := make(map[string]*LNodeType)
	for i := range s.DataTypeTemplates.LNodeType {
		lnTypes[s.DataTypeTemplates.LNodeType[i].ID] = &s.DataTypeTemplates.LNodeType[i]
	}
	doTypes := make(map[string]*DOType)
	for i := range s.DataTypeTemplates.DOType {
		doTypes[s.DataTypeTemplates.DOType[i].ID] = &s.DataTypeTemplates.DOType[i]
	}
	daTypes := make(map[string]*DAType)
	for i := range s.DataTypeTemplates.DAType {
		daTypes[s.DataTypeTemplates.DAType[i].ID] = &s.DataTypeTemplates.DAType[i]
	}
	// Build enum type lookup: id -> comma-separated allowed values (by ord)
	enumTypes := buildEnumTypes(s.DataTypeTemplates.EnumType)

	var out []FlattenEntry
	for i := range s.IED {
		ied := &s.IED[i]
		for ap := range ied.AccessPoint {
			acc := &ied.AccessPoint[ap]
			if acc.Server == nil {
				continue
			}
			for ld := range acc.Server.LDevice {
				ldev := &acc.Server.LDevice[ld]
				// LD in path is LDevice inst (e.g. LD0, CTRL), not AccessPoint name
				prefix := ied.Name + ldev.Inst // e.g. UNISECREF615CTRL
				// LN0
				if ldev.LN0 != nil {
					lnName := buildLNName(ldev.LN0.Prefix, ldev.LN0.LnClass, ldev.LN0.Inst)
					entries := flattenLN(prefix, lnName, ldev.LN0.LnType, nil, ldev.LN0.DOI, lnTypes, doTypes, daTypes, enumTypes)
					out = append(out, entries...)
				}
				// LNs
				for lnIdx := range ldev.LN {
					ln := &ldev.LN[lnIdx]
					lnName := buildLNName(ln.Prefix, ln.LnClass, ln.Inst)
					entries := flattenLN(prefix, lnName, ln.LnType, ln.DOI, nil, lnTypes, doTypes, daTypes, enumTypes)
					out = append(out, entries...)
				}
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

// buildEnumTypes returns a map from EnumType id to "ord(text),ord(text),..." for int→enum mapping.
func buildEnumTypes(enumTypes []EnumType) map[string]string {
	out := make(map[string]string)
	for i := range enumTypes {
		et := &enumTypes[i]
		if len(et.EnumVal) == 0 {
			continue
		}
		parts := make([]string, len(et.EnumVal))
		for j := range et.EnumVal {
			ord := strings.TrimSpace(et.EnumVal[j].Ord)
			text := strings.TrimSpace(et.EnumVal[j].Text)
			parts[j] = ord + "(" + text + ")"
		}
		out[et.ID] = strings.Join(parts, ",")
	}
	return out
}

func buildLNName(prefix, lnClass, inst string) string {
	return prefix + lnClass + inst
}

func flattenLN(prefix, lnName string, lnType string, doiList []DOI, doiList0 []DOI, lnTypes map[string]*LNodeType, doTypes map[string]*DOType, daTypes map[string]*DAType, enumTypes map[string]string) []FlattenEntry {
	lt := lnTypes[lnType]
	if lt == nil {
		return nil
	}
	doiMap := make(map[string]*DOI)
	for i := range doiList {
		doiMap[doiList[i].Name] = &doiList[i]
	}
	for i := range doiList0 {
		doiMap[doiList0[i].Name] = &doiList0[i]
	}

	var out []FlattenEntry
	for doIdx := range lt.DO {
		do := &lt.DO[doIdx]
		doType := doTypes[do.Type]
		if doType == nil {
			continue
		}
		doi := doiMap[do.Name]
		doPath := lnName + "." + do.Name
		entries := flattenDO(prefix, doPath, doType, doi, nil, doTypes, daTypes, enumTypes)
		out = append(out, entries...)
	}
	return out
}

func flattenDO(prefix, doPath string, doType *DOType, doi *DOI, sdiPath []string, doTypes map[string]*DOType, daTypes map[string]*DAType, enumTypes map[string]string) []FlattenEntry {
	var out []FlattenEntry
	// DA elements (leaf attributes)
	for i := range doType.DA {
		da := &doType.DA[i]
		fc := da.Fc
		if fc == "" {
			fc = inferFC(da.BType)
		}
		bType := normalizeBType(da.BType)
		if da.Type != "" && daTypes[da.Type] != nil {
			// Reference to DAType (e.g. Oper struct) — only recurse when type is a known DAType
			structPath := doPath + "." + da.Name
			structSDIPath := appendPath(sdiPath, da.Name)
			parentFC := da.Fc
			if parentFC == "" {
				parentFC = inferFC(da.BType)
			}
			entries := flattenDAType(prefix, structPath, da.Type, doi, structSDIPath, parentFC, daTypes, enumTypes)
			out = append(out, entries...)
		} else {
			// Leaf: no type, or type is Enum/other (e.g. BehaviourModeKind) — not in daTypes
			fullPath := doPath + "." + da.Name
			val := getDAIValue(doi, sdiPath, da.Name)
			e := FlattenEntry{
				Path:  prefix + "/" + fullPath,
				FC:    fc,
				Value: val,
				BType: bType,
			}
			if da.Type != "" && enumTypes[da.Type] != "" {
				e.EnumVals = enumTypes[da.Type]
			}
			out = append(out, e)
		}
	}
	// SDO elements (nested structures)
	for i := range doType.SDO {
		sdo := &doType.SDO[i]
		sdoType := doTypes[sdo.Type]
		if sdoType == nil {
			continue
		}
		subDoPath := doPath + "." + sdo.Name
		var subDOI *DOI
		var subSDIPath []string
		if doi != nil {
			for j := range doi.SDI {
				if doi.SDI[j].Name == sdo.Name {
					subDOI = &DOI{Name: sdo.Name, DAI: doi.SDI[j].DAI, SDI: doi.SDI[j].SDI}
					subSDIPath = appendPath(sdiPath, sdo.Name)
					break
				}
			}
		}
		entries := flattenDO(prefix, subDoPath, sdoType, subDOI, subSDIPath, doTypes, daTypes, enumTypes)
		out = append(out, entries...)
	}
	return out
}

// getDAIValue returns the value of a DAI after following sdiPath (e.g. ["Oper"] for Mod.Oper.ctlVal).
func getDAIValue(doi *DOI, sdiPath []string, daName string) string {
	if doi == nil {
		return ""
	}
	dais := doi.DAI
	sdis := doi.SDI
	for _, step := range sdiPath {
		found := false
		for i := range sdis {
			if sdis[i].Name == step {
				dais = sdis[i].DAI
				sdis = sdis[i].SDI
				found = true
				break
			}
		}
		if !found {
			return ""
		}
	}
	for i := range dais {
		if dais[i].Name == daName {
			return strings.TrimSpace(dais[i].Val)
		}
	}
	return ""
}

// flattenDAType recursively expands a DAType and returns leaf entries (path, fc, value, bType).
// parentFC is the fc of the containing DA (e.g. CO for Oper struct); used when BDA has no fc.
func flattenDAType(prefix, structPath string, typeID string, doi *DOI, sdiPath []string, parentFC string, daTypes map[string]*DAType, enumTypes map[string]string) []FlattenEntry {
	dt := daTypes[typeID]
	if dt == nil {
		return nil
	}
	var out []FlattenEntry
	for i := range dt.BDA {
		bda := &dt.BDA[i]
		subPath := structPath + "." + bda.Name
		fc := bda.Fc
		if fc == "" {
			fc = parentFC
		}
		if fc == "" {
			fc = inferFC(bda.BType)
		}
		if bda.Type != "" && daTypes[bda.Type] != nil {
			// Nested struct
			entries := flattenDAType(prefix, subPath, bda.Type, doi, appendPath(sdiPath, bda.Name), fc, daTypes, enumTypes)
			out = append(out, entries...)
		} else {
			val := getDAIValue(doi, sdiPath, bda.Name)
			e := FlattenEntry{
				Path:  prefix + "/" + subPath,
				FC:    fc,
				Value: val,
				BType: normalizeBType(bda.BType),
			}
			if bda.Type != "" && enumTypes[bda.Type] != "" {
				e.EnumVals = enumTypes[bda.Type]
			}
			out = append(out, e)
		}
	}
	for i := range dt.DA {
		dta := &dt.DA[i]
		subPath := structPath + "." + dta.Name
		val := getDAIValue(doi, sdiPath, dta.Name)
		fc := dta.Fc
		if fc == "" {
			fc = parentFC
		}
		if fc == "" {
			fc = inferFC(dta.BType)
		}
		e := FlattenEntry{
			Path:  prefix + "/" + subPath,
			FC:    fc,
			Value: val,
			BType: normalizeBType(dta.BType),
		}
		if dta.Type != "" && enumTypes[dta.Type] != "" {
			e.EnumVals = enumTypes[dta.Type]
		}
		out = append(out, e)
	}
	return out
}

func inferFC(bType string) string {
	switch bType {
	case "Struct":
		return "CO"
	default:
		return "ST"
	}
}

func normalizeBType(b string) string {
	switch b {
	case "BOOLEAN":
		return "BOOL"
	case "Timestamp":
		return "UTC_TIME"
	case "VisString255", "Unicode255":
		return "STRING"
	case "Quality":
		return "BIT_STRING"
	case "INT8", "INT16", "INT32", "INT64", "INT128":
		return b
	case "UINT8", "UINT16", "UINT32", "UINT64", "INT8U", "INT16U", "INT32U":
		if b == "INT8U" {
			return "UINT8"
		}
		if b == "INT16U" {
			return "UINT16"
		}
		if b == "INT32U" {
			return "UINT32"
		}
		return b
	case "Check", "Octet64":
		return "BIT_STRING"
	case "Enum":
		return "INT8" // often represented as INT8
	case "Float", "Double":
		return b
	case "":
		return "STRING"
	default:
		if b != "" && (strings.HasPrefix(b, "Enum") || strings.Contains(b, "Enum")) {
			return "INT8"
		}
		return b
	}
}

// FormatEntry formats one FlattenEntry. If detailed is false, output is "Path: Value" only.
// If detailed is true, includes [FC], [type:BType], and [enum:...].
func FormatEntry(e FlattenEntry, detailed bool) string {
	val := e.Value
	if val == "" {
		val = "?"
	}
	if !detailed {
		return fmt.Sprintf("%s: %s", e.Path, val)
	}
	line := fmt.Sprintf("%s[%s]: %s [type:%s]", e.Path, e.FC, val, e.BType)
	if e.EnumVals != "" {
		line += fmt.Sprintf(" [enum:%s]", e.EnumVals)
	}
	return line
}

// CSVRow returns the CSV columns for a FlattenEntry: Logical Device, Logical Node, Data Object, Data Attribute, FC, Value, Type, Enum.
// Path format is "LogicalDevice/LogicalNode.DataObject.DataAttribute" (Data Attribute can contain dots for nested paths).
func CSVRow(e FlattenEntry) (logicalDevice, logicalNode, dataObject, dataAttribute, fc, value, bType, enumVals string) {
	value = e.Value
	if value == "" {
		value = "?"
	}
	fc = e.FC
	bType = e.BType
	enumVals = e.EnumVals
	parts := strings.SplitN(e.Path, "/", 2)
	if len(parts) < 2 {
		logicalDevice = e.Path
		return
	}
	logicalDevice = parts[0]
	rest := parts[1]
	segments := strings.Split(rest, ".")
	switch len(segments) {
	case 0:
	case 1:
		logicalNode = rest
	case 2:
		logicalNode = segments[0]
		dataObject = segments[1]
	default:
		logicalNode = segments[0]
		dataObject = segments[1]
		dataAttribute = strings.Join(segments[2:], ".")
	}
	return
}
