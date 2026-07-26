// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"fmt"
	"strings"

	iec61850 "github.com/otfabric/go-iec61850"
	"github.com/otfabric/go-mms"
	"github.com/otfabric/iec61850ctl/pkg/domain"
)

// ListDataAttributesInput contains parameters for listing data attributes.
type ListDataAttributesInput struct {
	LogicalDevice string
	LogicalNode   string
	DataObject    string
	Detailed      bool
}

func (l ListDataAttributesInput) Validate() error {
	if l.LogicalDevice == "" {
		return fmt.Errorf("%w: LogicalDevice is required", ErrInvalidConfig)
	}
	if l.LogicalNode == "" {
		return fmt.Errorf("%w: LogicalNode is required", ErrInvalidConfig)
	}
	if l.DataObject == "" {
		return fmt.Errorf("%w: DataObject is required", ErrInvalidConfig)
	}
	return nil
}

// GetDataAttributes returns leaf data attributes for a data object.
func (e *Explorer) GetDataAttributes(input ListDataAttributesInput) ([]domain.DataAttribute, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}
	if e.conn == nil {
		return nil, fmt.Errorf("explorer: client is nil")
	}
	ctx := context.Background()
	doRef := fmt.Sprintf("%s/%s.%s", input.LogicalDevice, input.LogicalNode, input.DataObject)

	var result []domain.DataAttribute
	seen := map[string]bool{}
	for _, fc := range []string{"ST", "MX", "CF", "DC", "SP", "SV", "CO", "SG", "SE", "EX"} {
		leaves, err := e.collectLeaves(ctx, doRef, "", domain.ParseFC(fc))
		if err != nil {
			continue
		}
		for _, leaf := range leaves {
			key := leaf.Ref + "|" + string(leaf.FC)
			if seen[key] {
				continue
			}
			seen[key] = true
			result = append(result, leaf)
		}
	}
	return result, nil
}

// GetDataObjectDANames returns top-level data attribute names under a data object.
func (e *Explorer) GetDataObjectDANames(ldName, lnName, doName string) ([]string, error) {
	attrs, err := e.GetDataAttributes(ListDataAttributesInput{
		LogicalDevice: ldName,
		LogicalNode:   lnName,
		DataObject:    doName,
	})
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var names []string
	for _, attr := range attrs {
		top := attr.Name
		if i := strings.Index(attr.Name, "."); i >= 0 {
			top = attr.Name[:i]
		}
		if !seen[top] {
			seen[top] = true
			names = append(names, top)
		}
	}
	return names, nil
}

// ListDataAttributes groups leaf attributes by top-level parent DA name.
func (e *Explorer) ListDataAttributes(input ListDataAttributesInput) (map[string][]domain.DataAttribute, error) {
	attrs, err := e.GetDataAttributes(input)
	if err != nil {
		return nil, err
	}
	result := make(map[string][]domain.DataAttribute)
	for _, attr := range attrs {
		parent := attr.Name
		if i := strings.Index(attr.Name, "."); i >= 0 {
			parent = attr.Name[:i]
		}
		result[parent] = append(result[parent], attr)
	}
	return result, nil
}

func (e *Explorer) collectLeaves(ctx context.Context, baseRef, namePrefix string, fc domain.FunctionalConstraint) ([]domain.DataAttribute, error) {
	refStr := baseRef
	if fc != domain.FC_NONE {
		refStr = baseRef + "[" + string(fc) + "]"
	}
	ref, err := iec61850.ParseRef(refStr)
	if err != nil {
		return nil, err
	}

	spec, err := e.conn.GetVariableType(ctx, ref)
	if err != nil {
		return nil, err
	}

	displayName := namePrefix
	if displayName == "" {
		if i := strings.LastIndex(baseRef, "."); i >= 0 {
			displayName = baseRef[i+1:]
		} else {
			displayName = baseRef
		}
	}

	if spec != nil && (spec.Type == mms.ValueTypeStructure || spec.Type == mms.ValueTypeArray) {
		children, err := e.conn.ListChildren(ctx, ref)
		if err != nil || len(children) == 0 {
			return nil, err
		}
		var out []domain.DataAttribute
		for _, ch := range children {
			childName := ch.Name
			childRef := baseRef + "." + childName
			childPrefix := childName
			if namePrefix != "" {
				childPrefix = namePrefix + "." + childName
			}
			leaves, err := e.collectLeaves(ctx, childRef, childPrefix, fc)
			if err != nil {
				continue
			}
			out = append(out, leaves...)
		}
		return out, nil
	}

	da := domain.DataAttribute{
		Name: displayName,
		Ref:  baseRef,
		FC:   fc,
	}
	if spec != nil {
		da.Type = domain.FromMMSValueType(spec.Type)
	}
	if v, err := e.conn.Read(ctx, ref); err == nil && v != nil {
		da.Type = domain.FromMMSValueType(v.Type())
		da.Value = domain.ValueFromMMS(v.MMS())
	}
	return []domain.DataAttribute{da}, nil
}
