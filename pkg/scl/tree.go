// SPDX-License-Identifier: MIT

package scl

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

const (
	iconRoot   = "📦"
	iconLN     = "🔧"
	iconData   = "📊"
	iconReport = "📡"
	treeBar    = "│"
	treeBranch = "├─"
	treeLast   = "└─"
)

// WriteTree outputs the SCL in a tree format to w. If detailed is false, leaf lines are "path: value" only.
// If detailed is true, leaf lines include [FC], [type:BType], and [enum:...].
func (s *SCL) WriteTree(w io.Writer, detailed bool) error {
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
	enumTypes := buildEnumTypes(s.DataTypeTemplates.EnumType)

	for i := range s.IED {
		ied := &s.IED[i]
		for ap := range ied.AccessPoint {
			acc := &ied.AccessPoint[ap]
			if acc.Server == nil {
				continue
			}
			for ld := range acc.Server.LDevice {
				ldev := &acc.Server.LDevice[ld]
				rootName := ied.Name + ldev.Inst
				_, _ = fmt.Fprintf(w, "%s %s\n", iconRoot, rootName)
				_, _ = fmt.Fprintf(w, "%s\n", treeBar)

				// Collect LNs: LN0 first, then other LNs sorted by name
				var ln0 *LN0
				var lns []*LN
				if ldev.LN0 != nil {
					ln0 = ldev.LN0
				}
				for j := range ldev.LN {
					lns = append(lns, &ldev.LN[j])
				}
				sort.Slice(lns, func(a, b int) bool {
					na := buildLNName(lns[a].Prefix, lns[a].LnClass, lns[a].Inst)
					nb := buildLNName(lns[b].Prefix, lns[b].LnClass, lns[b].Inst)
					return na < nb
				})

				// LN0 first
				if ln0 != nil {
					lnName := buildLNName(ln0.Prefix, ln0.LnClass, ln0.Inst)
					lastLN := len(lns) == 0
					writeLNNode(w, lnName, "LLN0", ln0.LnType, ln0.DOI, "", ln0.DataSet, ln0.ReportControl, lastLN, lnTypes, doTypes, daTypes, enumTypes, detailed)
				}

				// Other LNs
				for idx, ln := range lns {
					lnName := buildLNName(ln.Prefix, ln.LnClass, ln.Inst)
					lastLN := (ln0 == nil && idx == len(lns)-1) || (ln0 != nil && idx == len(lns)-1)
					writeLNNode(w, lnName, ln.LnClass, ln.LnType, ln.DOI, ln.Desc, nil, nil, lastLN, lnTypes, doTypes, daTypes, enumTypes, detailed)
				}
			}
		}
	}
	return nil
}

func writeLNNode(w io.Writer, lnName, lnClass, lnType string, doiList []DOI, desc string, dataSets []DataSet, reports []ReportControl, isLast bool, lnTypes map[string]*LNodeType, doTypes map[string]*DOType, daTypes map[string]*DAType, enumTypes map[string]string, detailed bool) {
	prefix := treeBranch
	if isLast && len(dataSets) == 0 && len(reports) == 0 {
		prefix = treeLast
	}
	descPart := ""
	if desc != "" {
		descPart = " (" + desc + ")"
	}
	if lnClass == "LLN0" {
		descPart = " (Logical Node Zero)"
	}
	_, _ = fmt.Fprintf(w, "%s %s %s%s\n", prefix, iconLN, lnName, descPart)

	innerPrefix := "│  "
	if isLast && len(dataSets) == 0 && len(reports) == 0 {
		innerPrefix = "   "
	}

	lt := lnTypes[lnType]
	doiMap := make(map[string]*DOI)
	for i := range doiList {
		doiMap[doiList[i].Name] = &doiList[i]
	}

	if lt != nil {
		doNames := make([]string, 0, len(lt.DO))
		for i := range lt.DO {
			doNames = append(doNames, lt.DO[i].Name)
		}
		sort.Strings(doNames)

		for i, doName := range doNames {
			var do *DO
			for j := range lt.DO {
				if lt.DO[j].Name == doName {
					do = &lt.DO[j]
					break
				}
			}
			if do == nil {
				continue
			}
			doType := doTypes[do.Type]
			if doType == nil {
				continue
			}
			doi := doiMap[do.Name]
			doDesc := ""
			if doi != nil && doi.Desc != "" {
				doDesc = " (" + doi.Desc + ")"
			}
			lastDO := i == len(doNames)-1 && len(dataSets) == 0 && len(reports) == 0
			writeDONode(w, innerPrefix, doName, doDesc, doType, doi, nil, lastDO, doTypes, daTypes, enumTypes, detailed)
		}
	}

	// DataSets (LN0 only)
	if len(dataSets) > 0 {
		_, _ = fmt.Fprintf(w, "%s%s %s DataSets (%d)\n", innerPrefix, treeBranch, iconData, len(dataSets))
		dsPrefix := innerPrefix + "│  "
		for i, ds := range dataSets {
			last := i == len(dataSets)-1 && len(reports) == 0
			p := treeBranch
			if last && len(reports) == 0 {
				p = treeLast
			}
			_, _ = fmt.Fprintf(w, "%s%s %s\n", dsPrefix, p, ds.Name)
		}
	}

	// Reports (LN0 only) — always last child of LN when present
	if len(reports) > 0 {
		_, _ = fmt.Fprintf(w, "%s%s %s Reports (%d)\n", innerPrefix, treeLast, iconReport, len(reports))
		// No vertical bar under └─ so use spaces for the same width
		rptPrefix := strings.ReplaceAll(innerPrefix, treeBar, " ") + "   "
		for i, r := range reports {
			last := i == len(reports)-1
			br := treeBranch
			if last {
				br = treeLast
			}
			buf := ""
			if r.Buffered == "true" {
				buf = " [Buffered]"
			} else {
				buf = " [Unbuffered]"
			}
			_, _ = fmt.Fprintf(w, "%s%s %s%s\n", rptPrefix, br, r.Name, buf)
		}
	}
}

func writeDONode(w io.Writer, prefix, doName, doDesc string, doType *DOType, doi *DOI, sdiPath []string, isLast bool, doTypes map[string]*DOType, daTypes map[string]*DAType, enumTypes map[string]string, detailed bool) {
	br := treeBranch
	if isLast {
		br = treeLast
	}
	_, _ = fmt.Fprintf(w, "%s%s %s%s\n", prefix, br, doName, doDesc)

	childPrefix := prefix + "│  "
	if isLast {
		childPrefix = prefix + "   "
	}

	// Leaf DAs
	var leaves []string
	collectDOLeaves(doType, doi, sdiPath, "", doTypes, daTypes, enumTypes, detailed, &leaves)

	subPrefix := childPrefix
	for i, line := range leaves {
		last := i == len(leaves)-1
		b := treeBranch
		if last {
			b = treeLast
		}
		_, _ = fmt.Fprintf(w, "%s%s %s\n", subPrefix, b, line)
	}
}

// collectDOLeaves appends leaf lines (format depends on detailed) for all leaf attributes under a DO.
func collectDOLeaves(doType *DOType, doi *DOI, sdiPath []string, pathPrefix string, doTypes map[string]*DOType, daTypes map[string]*DAType, enumTypes map[string]string, detailed bool, out *[]string) {
	for i := range doType.DA {
		da := &doType.DA[i]
		fc := da.Fc
		if fc == "" {
			fc = inferFC(da.BType)
		}
		path := pathPrefix + da.Name
		if da.Type != "" && daTypes[da.Type] != nil {
			structPath := pathPrefix + da.Name + "."
			structSDI := appendPath(sdiPath, da.Name)
			parentFC := da.Fc
			if parentFC == "" {
				parentFC = fc
			}
			collectDATypeLeaves(daTypes[da.Type], doi, structSDI, structPath, parentFC, daTypes, enumTypes, detailed, out)
			continue
		}
		val := getDAIValue(doi, sdiPath, da.Name)
		if val == "" {
			val = "?"
		}
		*out = append(*out, formatLeafLine(path, fc, val, normalizeBType(da.BType), enumTypes[da.Type], detailed))
	}
	for i := range doType.SDO {
		sdo := &doType.SDO[i]
		sdoType := doTypes[sdo.Type]
		if sdoType == nil {
			continue
		}
		subPath := pathPrefix + sdo.Name + "."
		var subDOI *DOI
		var subSDI []string
		if doi != nil {
			for j := range doi.SDI {
				if doi.SDI[j].Name == sdo.Name {
					subDOI = &DOI{Name: sdo.Name, DAI: doi.SDI[j].DAI, SDI: doi.SDI[j].SDI}
					subSDI = appendPath(sdiPath, sdo.Name)
					break
				}
			}
		}
		collectDOLeaves(sdoType, subDOI, subSDI, subPath, doTypes, daTypes, enumTypes, detailed, out)
	}
}

func collectDATypeLeaves(dt *DAType, doi *DOI, sdiPath []string, pathPrefix string, parentFC string, daTypes map[string]*DAType, enumTypes map[string]string, detailed bool, out *[]string) {
	for i := range dt.BDA {
		bda := &dt.BDA[i]
		path := pathPrefix + bda.Name
		fc := bda.Fc
		if fc == "" {
			fc = parentFC
		}
		if fc == "" {
			fc = inferFC(bda.BType)
		}
		if bda.Type != "" && daTypes[bda.Type] != nil {
			collectDATypeLeaves(daTypes[bda.Type], doi, appendPath(sdiPath, bda.Name), path+".", fc, daTypes, enumTypes, detailed, out)
			continue
		}
		val := getDAIValue(doi, sdiPath, bda.Name)
		if val == "" {
			val = "?"
		}
		*out = append(*out, formatLeafLine(path, fc, val, normalizeBType(bda.BType), enumTypes[bda.Type], detailed))
	}
	for i := range dt.DA {
		dta := &dt.DA[i]
		path := pathPrefix + dta.Name
		fc := dta.Fc
		if fc == "" {
			fc = parentFC
		}
		if fc == "" {
			fc = inferFC(dta.BType)
		}
		val := getDAIValue(doi, sdiPath, dta.Name)
		if val == "" {
			val = "?"
		}
		*out = append(*out, formatLeafLine(path, fc, val, normalizeBType(dta.BType), enumTypes[dta.Type], detailed))
	}
}

// formatLeafLine returns "path: val" when detailed is false, else "path [FC]: val [type:BType] [enum:...]".
func formatLeafLine(path, fc, val, bType string, enumVals string, detailed bool) string {
	if !detailed {
		return fmt.Sprintf("%s: %s", path, val)
	}
	line := fmt.Sprintf("%s [%s]: %s [type:%s]", path, fc, val, bType)
	if enumVals != "" {
		line += fmt.Sprintf(" [enum:%s]", enumVals)
	}
	return line
}
