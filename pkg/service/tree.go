// SPDX-License-Identifier: MIT

// Package services provides business logic for IEC 61850 device exploration and data reading.
package service

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	iec61850 "github.com/otfabric/go-iec61850"
	"github.com/otfabric/go-mms"
	"github.com/otfabric/iec61850ctl/pkg/domain"
)

var probeFCs = []iec61850.FunctionalConstraint{
	iec61850.FCST, iec61850.FCMX, iec61850.FCCF, iec61850.FCDC,
	iec61850.FCSP, iec61850.FCSV, iec61850.FCCO, iec61850.FCSG, iec61850.FCSE,
}

// Tree provides methods for rendering the complete IEC 61850 device tree.
type Tree struct {
	conn         IEC61850Connection
	callCount    int
	callInterval time.Duration
}

// NewTree creates a new Tree service.
func NewTree(conn IEC61850Connection) *Tree {
	return &Tree{conn: conn}
}

// SetCallInterval sets a delay to apply after each MMS call.
func (t *Tree) SetCallInterval(d time.Duration) {
	t.callInterval = d
}

// WithCallInterval sets a delay to apply after each MMS call (fluent API).
func (t *Tree) WithCallInterval(d time.Duration) *Tree {
	t.callInterval = d
	return t
}

func (t *Tree) recordCall() {
	t.callCount++
	if t.callInterval > 0 {
		time.Sleep(t.callInterval)
	}
}

func (t *Tree) ctx() context.Context {
	return context.Background()
}

func parsePath(path string) (ld, ln, do string) {
	if path == "" {
		return "", "", ""
	}
	parts := strings.SplitN(path, "/", 2)
	ld = parts[0]
	if len(parts) > 1 {
		lnDoParts := strings.SplitN(parts[1], ".", 2)
		ln = lnDoParts[0]
		if len(lnDoParts) > 1 {
			do = lnDoParts[1]
		}
	}
	return ld, ln, do
}

func dedupeStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func lastSegment(ref string) string {
	if idx := strings.LastIndex(ref, "."); idx >= 0 {
		return ref[idx+1:]
	}
	return ref
}

func refWithFC(ref iec61850.Ref, fc iec61850.FunctionalConstraint) iec61850.Ref {
	r := ref
	r.FC = fc
	return r
}

func (t *Tree) resolveVariableType(ref iec61850.Ref) (iec61850.FunctionalConstraint, *mms.TypeSpec, error) {
	ctx := t.ctx()
	if ref.FC != "" {
		spec, err := t.conn.GetVariableType(ctx, ref)
		t.recordCall()
		if err == nil {
			return ref.FC, spec, nil
		}
	}
	for _, fc := range probeFCs {
		r := refWithFC(ref, fc)
		spec, err := t.conn.GetVariableType(ctx, r)
		t.recordCall()
		if err == nil {
			return fc, spec, nil
		}
	}
	return "", nil, fmt.Errorf("no FC found for %s", ref.String())
}

func formatTypeSpec(spec *mms.TypeSpec) string {
	if spec == nil {
		return "UNKNOWN"
	}
	switch spec.Type {
	case mms.ValueTypeStructure:
		if len(spec.Elements) > 0 {
			return fmt.Sprintf("STRUCT(%d)", len(spec.Elements))
		}
		return "STRUCT"
	case mms.ValueTypeArray:
		if spec.Count > 0 {
			return fmt.Sprintf("ARRAY[%d]", spec.Count)
		}
		return "ARRAY"
	default:
		return domain.FromMMSValueType(spec.Type).String()
	}
}

func formatValueDisplay(v *iec61850.Value) string {
	if v == nil {
		return "?"
	}
	dv := domain.ValueFromMMS(v.MMS())
	if dv == nil {
		return "?"
	}
	return dv.String()
}

func (t *Tree) listLogicalDeviceNames(pathLD string) ([]string, error) {
	if pathLD != "" {
		return []string{pathLD}, nil
	}
	debugf("ListLogicalDevices()")
	lds, err := t.conn.ListLogicalDevices(t.ctx())
	t.recordCall()
	if err != nil {
		return nil, fmt.Errorf("failed to get logical devices: %w", err)
	}
	names := make([]string, len(lds))
	for i, ld := range lds {
		names[i] = ld.Name
	}
	return names, nil
}

func (t *Tree) listLogicalNodeNames(ld, pathLN string) ([]string, error) {
	if pathLN != "" {
		return []string{pathLN}, nil
	}
	debugf("ListLogicalNodes(ld=%q)", ld)
	lns, err := t.conn.ListLogicalNodes(t.ctx(), ld)
	t.recordCall()
	if err != nil {
		return nil, err
	}
	names := make([]string, len(lns))
	for i, ln := range lns {
		names[i] = ln.Name
	}
	return names, nil
}

func (t *Tree) listDataObjectNames(ld, ln, pathDO string) ([]string, error) {
	if pathDO != "" {
		return []string{pathDO}, nil
	}
	debugf("ListDataObjects(ld=%q, ln=%q)", ld, ln)
	dos, err := t.conn.ListDataObjects(t.ctx(), ld, ln)
	t.recordCall()
	if err != nil {
		return nil, err
	}
	names := make([]string, len(dos))
	for i, d := range dos {
		names[i] = d.Name
	}
	return dedupeStrings(names), nil
}

// RenderDeviceTree renders the IEC 61850 device hierarchy to the writer.
func (t *Tree) RenderDeviceTree(w io.Writer, host string, port int, path string) (int, error) {
	t.callCount = 0
	_, _ = fmt.Fprintf(w, "IEC 61850 Device Tree for %s:%d\n", host, port)
	_, _ = fmt.Fprintln(w, strings.Repeat("=", 80))

	pathLD, pathLN, pathDO := parsePath(path)
	devices, err := t.listLogicalDeviceNames(pathLD)
	if err != nil {
		return 0, err
	}

	for _, ld := range devices {
		_, _ = fmt.Fprintf(w, "\n📦 %s\n", ld)

		lnNames, err := t.listLogicalNodeNames(ld, pathLN)
		if err != nil {
			_, _ = fmt.Fprintf(w, "   ⚠️  Error listing LNs: %v\n", err)
			continue
		}

		for _, ln := range lnNames {
			_, _ = fmt.Fprintf(w, "  ├─ 📋 %s\n", ln)

			doNames, err := t.listDataObjectNames(ld, ln, pathDO)
			if err != nil {
				_, _ = fmt.Fprintf(w, "     ⚠️  Error listing DOs: %v\n", err)
				continue
			}

			for doIdx, doName := range doNames {
				isLastDO := doIdx == len(doNames)-1
				doPrefix := "  │  ├─"
				if isLastDO {
					doPrefix = "  │  └─"
				}
				_, _ = fmt.Fprintf(w, "%s 📊 %s\n", doPrefix, doName)

				doRef := iec61850.Ref{LD: ld, LN: ln, Path: strings.Split(doName, ".")}
				if err := t.renderDataAttributes(w, doRef, isLastDO); err != nil {
					_, _ = fmt.Fprintf(w, "     │     ⚠️  Error listing DAs: %v\n", err)
				}
			}
		}
	}

	_, _ = fmt.Fprintln(w)
	return t.callCount, nil
}

// RenderDeviceTreeFlat renders the device hierarchy in flat format.
func (t *Tree) RenderDeviceTreeFlat(w io.Writer, host string, port int, path string) (int, error) {
	t.callCount = 0
	pathLD, pathLN, pathDO := parsePath(path)

	devices, err := t.listLogicalDeviceNames(pathLD)
	if err != nil {
		return 0, err
	}

	for _, ld := range devices {
		lnNames, err := t.listLogicalNodeNames(ld, pathLN)
		if err != nil {
			continue
		}
		for _, ln := range lnNames {
			doNames, err := t.listDataObjectNames(ld, ln, pathDO)
			if err != nil {
				continue
			}
			for _, doName := range doNames {
				doRef := iec61850.Ref{LD: ld, LN: ln, Path: strings.Split(doName, ".")}
				t.renderFlatAttributes(w, doRef, make(map[string]struct{}))
			}
		}
	}
	return t.callCount, nil
}

// BuildSerializableModel traverses the IED and returns a domain.IED.
func (t *Tree) BuildSerializableModel(host string, port int, path string, includeDataSets, includeReports bool) (*domain.IED, error) {
	t.callCount = 0
	pathLD, pathLN, pathDO := parsePath(path)

	devices, err := t.listLogicalDeviceNames(pathLD)
	if err != nil {
		return nil, err
	}

	dsService := NewDataSetService(t.conn)
	rsService := NewReportService(t.conn)
	var logicalDevices []domain.LogicalDevice
	var leaves []domain.LeafEntry

	for _, ld := range devices {
		lnNames, err := t.listLogicalNodeNames(ld, pathLN)
		if err != nil {
			continue
		}

		var logicalNodes []domain.LogicalNode
		for _, ln := range lnNames {
			doNames, err := t.listDataObjectNames(ld, ln, pathDO)
			if err != nil {
				continue
			}

			node := domain.LogicalNode{Name: ln}
			if includeDataSets {
				if ds, err := dsService.ListDataSets(ld, ln); err == nil {
					node.DataSets = ds
				}
			}
			if includeReports {
				if br, err := rsService.ListBufferedReports(ld, ln); err == nil {
					for _, n := range br {
						node.ReportControlBlocks = append(node.ReportControlBlocks, domain.ReportControlBlockRef{
							LD: ld, LN: ln, Name: n, Buffered: true,
							Ref: fmt.Sprintf("%s/%s.%s", ld, ln, n),
						})
					}
				}
				if ur, err := rsService.ListUnbufferedReports(ld, ln); err == nil {
					for _, n := range ur {
						node.ReportControlBlocks = append(node.ReportControlBlocks, domain.ReportControlBlockRef{
							LD: ld, LN: ln, Name: n, Buffered: false,
							Ref: fmt.Sprintf("%s/%s.%s", ld, ln, n),
						})
					}
				}
			}

			var dataObjects []domain.DataObject
			for _, doName := range doNames {
				doRef := iec61850.Ref{LD: ld, LN: ln, Path: strings.Split(doName, ".")}
				attrs := t.collectSerializableAttributes(doRef, make(map[string]struct{}), &leaves)
				dataObjects = append(dataObjects, domain.DataObject{Name: doName, Attributes: attrs})
			}
			node.DataObjects = dataObjects
			logicalNodes = append(logicalNodes, node)
		}
		logicalDevices = append(logicalDevices, domain.LogicalDevice{Name: ld, LogicalNodes: logicalNodes})
	}

	return &domain.IED{
		Meta: domain.IEDMeta{
			SourceHost:   host,
			SourcePort:   port,
			SerializedAt: time.Now().UTC().Format(time.RFC3339),
			Generator:    "iec61850ctl tree --serialize",
		},
		LogicalDevices: logicalDevices,
		Leaves:         leaves,
	}, nil
}

// Build traverses the device model and returns structured data without rendering.
func (t *Tree) Build(path string, withValues bool) (*domain.IED, error) {
	_ = withValues
	return t.BuildSerializableModel("", 0, path, false, false)
}

func (t *Tree) collectSerializableAttributes(ref iec61850.Ref, visited map[string]struct{}, leaves *[]domain.LeafEntry) []domain.DataAttribute {
	key := ref.String()
	if _, ok := visited[key]; ok {
		return nil
	}
	visited[key] = struct{}{}

	debugf("ListChildren(ref=%q)", ref.String())
	children, err := t.conn.ListChildren(t.ctx(), ref)
	t.recordCall()
	if err != nil {
		return nil
	}

	refTail := lastSegment(ref.String())
	seenChild := make(map[string]struct{}, len(children))
	var out []domain.DataAttribute

	for _, ch := range children {
		if _, ok := seenChild[ch.Name]; ok {
			continue
		}
		seenChild[ch.Name] = struct{}{}
		if ch.Name == refTail {
			continue
		}

		nextRef := ch.Reference
		nextKey := nextRef.String()
		if _, ok := visited[nextKey]; ok {
			continue
		}

		fc, spec, err := t.resolveVariableType(nextRef)
		refStr := nextRef.String()
		if err != nil {
			*leaves = append(*leaves, domain.LeafEntry{
				Ref:        refStr,
				FC:         domain.FromLibFC(fc),
				Type:       domain.TypeUnknown,
				ValueError: err.Error(),
			})
			out = append(out, domain.DataAttribute{
				Name:       ch.Name,
				Ref:        refStr,
				FC:         domain.FromLibFC(fc),
				Type:       domain.TypeUnknown,
				ValueError: err.Error(),
			})
			continue
		}

		fcModel := domain.FromLibFC(fc)
		readRef := refWithFC(nextRef, fc)

		if spec.Type == mms.ValueTypeStructure || spec.Type == mms.ValueTypeArray {
			childAttrs := t.collectSerializableAttributes(nextRef, visited, leaves)
			out = append(out, domain.DataAttribute{
				Name:     ch.Name,
				Type:     domain.TypeStructure,
				Children: childAttrs,
			})
			continue
		}

		typeModel := domain.FromMMSValueType(spec.Type)
		reader := NewReader(t.conn)
		value, readErr := reader.ReadLeafValue(readRef.String(), fc, spec.Type)
		t.recordCall()
		var modelValue *domain.Value
		var valueErr string
		if readErr != nil {
			valueErr = readErr.Error()
		} else {
			modelValue, valueErr = ValueToModel(value, spec.Type)
		}
		*leaves = append(*leaves, domain.LeafEntry{
			Ref:        readRef.String(),
			FC:         fcModel,
			Type:       typeModel,
			Value:      modelValue,
			ValueError: valueErr,
		})
		out = append(out, domain.DataAttribute{
			Name:       ch.Name,
			Ref:        readRef.String(),
			FC:         fcModel,
			Type:       typeModel,
			Value:      modelValue,
			ValueError: valueErr,
		})
	}
	return out
}

func (t *Tree) renderFlatAttributes(w io.Writer, ref iec61850.Ref, visited map[string]struct{}) {
	key := ref.String()
	if _, ok := visited[key]; ok {
		return
	}
	visited[key] = struct{}{}

	debugf("ListChildren(ref=%q)", ref.String())
	children, err := t.conn.ListChildren(t.ctx(), ref)
	t.recordCall()
	if err != nil {
		return
	}

	refTail := lastSegment(ref.String())
	seenChild := make(map[string]struct{}, len(children))

	for _, ch := range children {
		if _, ok := seenChild[ch.Name]; ok {
			continue
		}
		seenChild[ch.Name] = struct{}{}
		if ch.Name == refTail {
			continue
		}

		nextRef := ch.Reference
		if _, ok := visited[nextRef.String()]; ok {
			continue
		}

		fc, spec, err := t.resolveVariableType(nextRef)
		fcStr := domain.FromLibFC(fc).String()
		readRef := refWithFC(nextRef, fc)

		if err != nil {
			v, readErr := t.conn.Read(t.ctx(), readRef)
			t.recordCall()
			if readErr == nil {
				_, _ = fmt.Fprintf(w, "%s[%s]: %v [type:UNKNOWN]\n", readRef.String(), fcStr, formatValueDisplay(v))
			}
			continue
		}

		if spec.Type == mms.ValueTypeStructure || spec.Type == mms.ValueTypeArray {
			t.renderFlatAttributes(w, nextRef, visited)
		} else {
			typeStr := formatTypeSpec(spec)
			value := t.readValue(readRef, fc, spec.Type)
			_, _ = fmt.Fprintf(w, "%s[%s]: %s [type:%s]\n", readRef.String(), fcStr, value, typeStr)
		}
	}
}

func (t *Tree) renderDataAttributes(w io.Writer, doRef iec61850.Ref, isLastDO bool) error {
	debugf("ListChildren(ref=%q)", doRef.String())
	children, err := t.conn.ListChildren(t.ctx(), doRef)
	t.recordCall()
	if err != nil {
		return err
	}

	baseIndent := "  │  │  "
	if isLastDO {
		baseIndent = "  │     "
	}

	for daIdx, ch := range children {
		isLastDA := daIdx == len(children)-1
		fc, spec, typeErr := t.resolveVariableType(ch.Reference)

		fcStr := "?"
		if fc != "" {
			fcStr = domain.FromLibFC(fc).String()
		}

		daPrefix := baseIndent + "├─"
		if isLastDA {
			daPrefix = baseIndent + "└─"
		}
		_, _ = fmt.Fprintf(w, "%s [%s] %s\n", daPrefix, fcStr, ch.Name)

		readRef := refWithFC(ch.Reference, fc)
		childIndent := baseIndent + "│  "
		if isLastDA {
			childIndent = baseIndent + "   "
		}

		if typeErr != nil {
			t.renderLeafAttribute(w, readRef, fc, baseIndent, isLastDA)
			continue
		}

		t.renderAttributeValue(w, readRef, fc, spec, childIndent)
	}
	return nil
}

func (t *Tree) renderAttributeValue(w io.Writer, daRef iec61850.Ref, fc iec61850.FunctionalConstraint, spec *mms.TypeSpec, indent string) {
	if spec.Type == mms.ValueTypeStructure || spec.Type == mms.ValueTypeArray {
		children, err := t.conn.ListChildren(t.ctx(), daRef)
		t.recordCall()
		if err != nil {
			return
		}

		for subIdx, sub := range children {
			isLast := subIdx == len(children)-1
			subPrefix := indent + "├─"
			if isLast {
				subPrefix = indent + "└─"
			}

			subFC, subSpec, err := t.resolveVariableType(sub.Reference)
			subFcStr := domain.FromLibFC(subFC).String()
			subReadRef := refWithFC(sub.Reference, subFC)

			_, _ = fmt.Fprintf(w, "%s [%s] %s", subPrefix, subFcStr, sub.Name)

			if err != nil {
				_, _ = fmt.Fprintln(w)
				continue
			}

			if subSpec.Type == mms.ValueTypeStructure || subSpec.Type == mms.ValueTypeArray {
				_, _ = fmt.Fprintln(w)
				childIndent := indent + "│  "
				if isLast {
					childIndent = indent + "   "
				}
				t.renderAttributeValue(w, subReadRef, subFC, subSpec, childIndent)
			} else {
				typeStr := formatTypeSpec(subSpec)
				value := t.readValue(subReadRef, subFC, subSpec.Type)
				_, _ = fmt.Fprintf(w, " → %s = %s\n", typeStr, value)
			}
		}
	} else {
		typeStr := formatTypeSpec(spec)
		value := t.readValue(daRef, fc, spec.Type)
		leafName := daRef.String()
		if idx := strings.LastIndex(leafName, "."); idx >= 0 {
			leafName = leafName[idx+1:]
		}
		_, _ = fmt.Fprintf(w, "%s → %s = %s\n", indent, typeStr, value)
		_ = leafName
	}
}

func (t *Tree) renderLeafAttribute(w io.Writer, daRef iec61850.Ref, fc iec61850.FunctionalConstraint, baseIndent string, isLastParent bool) {
	if fc == "" {
		fc = iec61850.FCMX
	}
	readRef := refWithFC(daRef, fc)
	debugf("Read(ref=%q)", readRef.String())
	val, err := t.conn.Read(t.ctx(), readRef)
	t.recordCall()
	if err != nil {
		return
	}

	childIndent := baseIndent + "│  "
	if isLastParent {
		childIndent = baseIndent + "   "
	}
	_, _ = fmt.Fprintf(w, "%s└─ = %s\n", childIndent, formatValueDisplay(val))
}

func (t *Tree) readValue(ref iec61850.Ref, fc iec61850.FunctionalConstraint, varType mms.ValueType) string {
	reader := NewReader(t.conn)
	value, err := reader.ReadLeafValue(ref.String(), fc, varType)
	t.recordCall()
	if err != nil {
		return "?"
	}
	modelVal, errMsg := ValueToModel(value, varType)
	if errMsg != "" {
		return "?"
	}
	if modelVal == nil {
		return "?"
	}
	return modelVal.String()
}
