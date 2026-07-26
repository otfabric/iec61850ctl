// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/otfabric/iec61850ctl/pkg/domain"
)

// DataSetService lists and reads named datasets.
type DataSetService struct {
	conn IEC61850Connection
}

func NewDataSetService(conn IEC61850Connection) *DataSetService {
	return &DataSetService{conn: conn}
}

func (s *DataSetService) ListDataSets(ldName, lnName string) ([]string, error) {
	ctx := context.Background()
	all, err := s.conn.ListDataSets(ctx, ldName)
	if err != nil {
		return nil, err
	}
	if lnName == "" {
		return all, nil
	}
	prefix := lnName + "$"
	var filtered []string
	for _, name := range all {
		if strings.HasPrefix(name, prefix) || !strings.Contains(name, "$") {
			// ListDataSets may return short names or LN$Name
			short := name
			if i := strings.LastIndex(name, "$"); i >= 0 {
				short = name[i+1:]
			}
			if strings.HasPrefix(name, prefix) || lnMatches(name, lnName) {
				filtered = append(filtered, short)
			}
		}
	}
	if len(filtered) == 0 {
		// Fallback: return all short names when LN filter yields nothing
		for _, name := range all {
			if i := strings.LastIndex(name, "$"); i >= 0 {
				filtered = append(filtered, name[i+1:])
			} else {
				filtered = append(filtered, name)
			}
		}
	}
	return filtered, nil
}

func lnMatches(dsName, ln string) bool {
	return strings.HasPrefix(dsName, ln+"$") || strings.Contains(dsName, "/"+ln+"$")
}

// DataSetDetails holds dataset metadata and optional values.
type DataSetDetails struct {
	Name      string
	LD        string
	LN        string
	Deletable bool
	Members   []DataSetMemberView
}

type DataSetMemberView struct {
	Ref   string
	Value *domain.Value
}

func (s *DataSetService) GetDataSet(ldName, lnName, dsName string, withValues bool) (*DataSetDetails, error) {
	ctx := context.Background()
	lookup := dsName
	if lnName != "" && !strings.Contains(dsName, "$") {
		lookup = lnName + "$" + dsName
	}
	ds, err := s.conn.GetDataSet(ctx, ldName, lookup)
	if err != nil {
		// try bare name
		ds, err = s.conn.GetDataSet(ctx, ldName, dsName)
		if err != nil {
			return nil, err
		}
	}
	out := &DataSetDetails{
		Name:      dsName,
		LD:        ldName,
		LN:        lnName,
		Deletable: ds.Deletable,
	}
	for _, m := range ds.Members {
		out.Members = append(out.Members, DataSetMemberView{Ref: m.Ref.String()})
	}
	if withValues {
		vals, err := s.conn.ReadDataSet(ctx, ldName, lookup)
		if err != nil {
			vals, err = s.conn.ReadDataSet(ctx, ldName, dsName)
		}
		if err != nil {
			return out, fmt.Errorf("dataset metadata ok but values failed: %w", err)
		}
		for i, v := range vals {
			if i < len(out.Members) && v.Value != nil {
				out.Members[i].Value = domain.ValueFromMMS(v.Value.MMS())
			}
		}
	}
	return out, nil
}

// GetDataSetDetails retrieves dataset directory members as a domain.DataSet.
func (s *DataSetService) GetDataSetDetails(ldName, lnName, dsName string) (*domain.DataSet, error) {
	details, err := s.GetDataSet(ldName, lnName, dsName, false)
	if err != nil {
		return nil, err
	}
	members := make([]domain.DataSetMember, len(details.Members))
	for i, m := range details.Members {
		members[i] = domain.DataSetMember{Ref: m.Ref, Value: m.Value}
	}
	return &domain.DataSet{
		Name:        details.Name,
		IsDeletable: details.Deletable,
		Members:     members,
	}, nil
}
