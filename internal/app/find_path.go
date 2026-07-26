// SPDX-License-Identifier: MIT

package app

import (
	"github.com/otfabric/iec61850ctl/pkg/service"
)

// FindPathInput specifies criteria for finding matching paths.
type FindPathInput struct {
	LNPattern  string // Regex pattern for logical node matching
	DOName     string // Exact data object name to match
	DAName     string // Exact data attribute name to match (optional)
	IncludeDAs bool   // If true, include all leaf data attributes
	Detailed   bool   // If true, include detailed DA information
}

// FindPathResult contains the results of a path search.
type FindPathResult struct {
	Matches   []service.PathMatch `json:"matches"`
	CallCount int                 `json:"call_count"`
}

// FindPath searches for paths matching logical node pattern and data object name.
// Returns all matching paths with optional data attributes详情.
func (a *App) FindPath(input FindPathInput) (*FindPathResult, error) {
	finder := service.NewFinder(a.conn)

	result, err := finder.FindPath(service.FindPathInput{
		LNPattern:  input.LNPattern,
		DoName:     input.DOName,
		DaName:     input.DAName,
		IncludeDas: input.IncludeDAs,
		Detailed:   input.Detailed,
	})
	if err != nil {
		return nil, err
	}

	return &FindPathResult{
		Matches:   result.Matches,
		CallCount: result.CallCount,
	}, nil
}
