// SPDX-License-Identifier: MIT

// Package services provides business logic for IEC 61850 device exploration and data reading.
// This file implements bulk path discovery from a mapping file with minimal device calls.

package service

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// BulkMappingEntry is one entry from a mapping file (e.g. mapping.json).
type BulkMappingEntry struct {
	ControlledPropertyId string `json:"controlledPropertyId"`
	BaseLn               string `json:"baseLn"`
	DoDaPath             string `json:"DoDaPath"`
}

// BulkResultEntry is one entry in the bulk find output (e.g. results.json).
type BulkResultEntry struct {
	ControlledPropertyId string   `json:"controlledPropertyId"`
	Paths                []string `json:"paths"`
}

// BulkFindResult contains the results and metrics from a bulk find operation.
type BulkFindResult struct {
	Entries   []BulkResultEntry // The discovered paths grouped by controlledPropertyId
	CallCount int               // Number of IEC 61850 calls made during discovery
}

// wantKey groups mapping entries by (baseLn, doName) so we traverse each (LD, LN, DO) at most once.
type wantKey struct {
	baseLn string
	doName string
}

// wantEntry describes one output path we need: controlledPropertyId and optional DA name.
type wantEntry struct {
	controlledPropertyId string
	daName               string // empty means DO-level path only
}

// BulkFind discovers all device paths that match the mapping file, with minimal IEC 61850 calls.
// It parses the mapping, groups by baseLn and DO/DA, then traverses LD -> LN -> DO once per node,
// calling GetDataObjectDANames only when a DO has requested DAs and caching the result per (LD, LN, DO).
func (f *Finder) BulkFind(mapping []BulkMappingEntry) (*BulkFindResult, error) {
	if len(mapping) == 0 {
		return &BulkFindResult{Entries: nil, CallCount: 0}, nil
	}

	// wantMap: (baseLn, doName) -> list of (controlledPropertyId, daName)
	wantMap := make(map[wantKey][]wantEntry)
	// lnRegexMap: baseLn -> regex that matches LN names containing baseLn (e.g. MMXU matches FMMXU1)
	lnRegexMap := make(map[string]*regexp.Regexp)
	// ordered controlledPropertyIds for stable output (first occurrence order)
	ctrlIdOrder := make([]string, 0)
	seenCtrl := make(map[string]bool)

	for _, m := range mapping {
		doName, daName := parseDoDaPath(m.DoDaPath)
		k := wantKey{baseLn: m.BaseLn, doName: doName}
		wantMap[k] = append(wantMap[k], wantEntry{controlledPropertyId: m.ControlledPropertyId, daName: daName})
		if lnRegexMap[m.BaseLn] == nil {
			lnRegexMap[m.BaseLn] = regexp.MustCompile(".*" + regexp.QuoteMeta(m.BaseLn) + ".*")
		}
		if !seenCtrl[m.ControlledPropertyId] {
			seenCtrl[m.ControlledPropertyId] = true
			ctrlIdOrder = append(ctrlIdOrder, m.ControlledPropertyId)
		}
	}

	// results: controlledPropertyId -> list of paths (LD/LN.DO or LD/LN.DO.DA)
	results := make(map[string][]string)
	callCount := 0

	// Cache DA names per doRef so we never call GetDataObjectDANames twice for same (LD, LN, DO)
	daNamesCache := make(map[string][]string)

	ldNames, err := f.explorer.ListLogicalDevices()
	callCount++
	if err != nil {
		return &BulkFindResult{CallCount: callCount}, err
	}

	for _, ldName := range ldNames {
		lnNames, err := f.explorer.ListLogicalNodes(ldName)
		callCount++
		if err != nil {
			continue
		}

		for _, lnName := range lnNames {
			// Which baseLn patterns does this LN match?
			var matchedBaseLns []string
			for baseLn, re := range lnRegexMap {
				if re.MatchString(lnName) {
					matchedBaseLns = append(matchedBaseLns, baseLn)
				}
			}
			if len(matchedBaseLns) == 0 {
				continue
			}

			doNames, err := f.explorer.ListDataObjects(ldName, lnName)
			callCount++
			if err != nil {
				continue
			}

			doSet := make(map[string]bool)
			for _, d := range doNames {
				doSet[d] = true
			}

			for _, doName := range doNames {
				for _, baseLn := range matchedBaseLns {
					k := wantKey{baseLn: baseLn, doName: doName}
					entries, ok := wantMap[k]
					if !ok || len(entries) == 0 {
						continue
					}

					needDAInfo := false
					for _, e := range entries {
						if e.daName != "" {
							needDAInfo = true
							break
						}
					}

					var daNames []string
					if needDAInfo {
						cacheKey := fmt.Sprintf("%s/%s.%s", ldName, lnName, doName)
						if cached, ok := daNamesCache[cacheKey]; ok {
							daNames = cached
						} else {
							names, err := f.explorer.GetDataObjectDANames(ldName, lnName, doName)
							callCount++
							if err != nil {
								continue
							}
							daNamesCache[cacheKey] = names
							daNames = names
						}
					}

					daSet := make(map[string]bool)
					for _, n := range daNames {
						daSet[n] = true
					}

					basePath := fmt.Sprintf("%s/%s.%s", ldName, lnName, doName)
					for _, e := range entries {
						if e.daName == "" {
							results[e.controlledPropertyId] = append(results[e.controlledPropertyId], basePath)
						} else if daSet[e.daName] {
							results[e.controlledPropertyId] = append(results[e.controlledPropertyId], basePath+"."+e.daName)
						}
					}
				}
			}
		}
	}

	// Build output in stable order (same as mapping order for controlledPropertyIds)
	out := make([]BulkResultEntry, 0, len(ctrlIdOrder))
	for _, ctrlId := range ctrlIdOrder {
		paths := results[ctrlId]
		if paths == nil {
			paths = []string{}
		}
		sort.Strings(paths) // stable, readable output
		out = append(out, BulkResultEntry{ControlledPropertyId: ctrlId, Paths: paths})
	}

	return &BulkFindResult{Entries: out, CallCount: callCount}, nil
}

// parseDoDaPath splits "DO" or "DO.DA" into doName and daName (daName may be empty).
func parseDoDaPath(doDaPath string) (doName, daName string) {
	parts := strings.SplitN(doDaPath, ".", 2)
	doName = parts[0]
	if len(parts) > 1 {
		daName = parts[1]
	}
	return doName, daName
}
