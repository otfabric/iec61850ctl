// SPDX-License-Identifier: MIT

package app

import (
	"encoding/json"

	"github.com/otfabric/iec61850ctl/pkg/service"
)

// FindPathInput specifies criteria for finding matching paths. It is also
// the body of POST /api/find/path.
type FindPathInput struct {
	LNPattern  string `json:"ln_pattern"`  // Regex pattern for logical node matching
	DOName     string `json:"do_name"`     // Exact data object name to match
	DAName     string `json:"da_name"`     // Exact data attribute name to match (optional)
	IncludeDAs bool   `json:"include_das"` // If true, include all leaf data attributes
	Detailed   bool   `json:"detailed"`    // If true, include detailed DA information
}

// UnmarshalJSON decodes the snake_case keys and, for clients written
// against v0.4.0 and earlier, the deprecated PascalCase keys (LNPattern,
// DOName, DAName, IncludeDAs). A snake_case key wins when both are present.
func (in *FindPathInput) UnmarshalJSON(data []byte) error {
	type current FindPathInput
	var legacy struct {
		LNPattern  *string `json:"LNPattern"`
		DOName     *string `json:"DOName"`
		DAName     *string `json:"DAName"`
		IncludeDAs *bool   `json:"IncludeDAs"`
	}
	if err := json.Unmarshal(data, &legacy); err != nil {
		return err
	}
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(data, &keys); err != nil {
		return err
	}
	var out current
	if err := json.Unmarshal(data, &out); err != nil {
		return err
	}
	fallback := func(key string, dst *string, src *string) {
		if _, ok := keys[key]; !ok && src != nil {
			*dst = *src
		}
	}
	fallback("ln_pattern", &out.LNPattern, legacy.LNPattern)
	fallback("do_name", &out.DOName, legacy.DOName)
	fallback("da_name", &out.DAName, legacy.DAName)
	if _, ok := keys["include_das"]; !ok && legacy.IncludeDAs != nil {
		out.IncludeDAs = *legacy.IncludeDAs
	}
	*in = FindPathInput(out)
	return nil
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
