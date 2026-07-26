// SPDX-License-Identifier: MIT

// Package services provides business logic for IEC 61850 device exploration and data reading.
// It separates domain operations from CLI presentation, enabling reuse across interfaces (CLI, GUI, API).
package service

import (
	"fmt"
	"regexp"

	"github.com/otfabric/iec61850ctl/pkg/domain"
)

// Finder provides methods for finding paths matching criteria.
type Finder struct {
	explorer *Explorer
}

// NewFinder creates a new Finder service.
func NewFinder(conn IEC61850Connection) *Finder {
	return &Finder{
		explorer: NewExplorer(conn),
	}
}

// FindPathInput contains parameters for finding paths matching criteria.
type FindPathInput struct {
	LNPattern  string // Regex pattern for logical node matching
	DoName     string // Exact data object name to match
	DaName     string // Exact data attribute name to match (optional, filters DAs when IncludeDas is true)
	IncludeDas bool   // If true, include all leaf data attributes (auto-enabled if DaName is specified)
	Detailed   bool   // If true, include detailed DA information
}

// Validate checks if the configuration is valid.
func (f FindPathInput) Validate() error {
	if f.LNPattern == "" {
		return fmt.Errorf("%w: LNPattern is required", ErrInvalidConfig)
	}
	if f.DoName == "" {
		return fmt.Errorf("%w: DoName is required", ErrInvalidConfig)
	}
	// Auto-enable IncludeDas if DaName is specified
	if f.DaName != "" && !f.IncludeDas {
		return fmt.Errorf("%w: DaName requires IncludeDas to be true (auto-enabled)", ErrInvalidConfig)
	}
	return nil
}

// PathMatch represents a matched path with optional data attributes.
type PathMatch struct {
	LD             string
	LN             string
	DO             string
	DataAttributes map[string][]domain.DataAttribute
}

// FindPathResult contains the results of a path search operation.
type FindPathResult struct {
	Matches   []PathMatch
	CallCount int
}

// FindPath searches across all logical devices for logical nodes matching a pattern
// and data objects matching exactly. Returns all matching paths with optional data attributes.
func (f *Finder) FindPath(input FindPathInput) (*FindPathResult, error) {
	// Compile regex pattern for LN matching
	lnRegex, err := regexp.Compile(input.LNPattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex pattern for LN: %w", err)
	}

	callCount := 0
	var matches []PathMatch

	// List all logical devices
	ldNames, err := f.explorer.ListLogicalDevices()
	callCount++
	if err != nil {
		return nil, err
	}

	// Search through all LDs
	for _, ldName := range ldNames {
		// List all logical nodes in this LD
		lnNames, err := f.explorer.ListLogicalNodes(ldName)
		callCount++
		if err != nil {
			// Continue with next LD if this one fails
			continue
		}

		// Match LNs using regex
		for _, lnName := range lnNames {
			if !lnRegex.MatchString(lnName) {
				continue
			}

			// List data objects in this LN
			doNames, err := f.explorer.ListDataObjects(ldName, lnName)
			callCount++
			if err != nil {
				continue
			}

			// Find exact DO match
			for _, doName := range doNames {
				if doName == input.DoName {
					match := PathMatch{
						LD: ldName,
						LN: lnName,
						DO: doName,
					}

					// If including DAs, fetch them
					if input.IncludeDas {
						attributesMap, err := f.explorer.ListDataAttributes(ListDataAttributesInput{
							LogicalDevice: ldName,
							LogicalNode:   lnName,
							DataObject:    doName,
							Detailed:      input.Detailed,
						})
						callCount++
						if err == nil {
							// Filter by DA name if specified
							if input.DaName != "" {
								match.DataAttributes = filterDataAttributes(attributesMap, input.DaName)
								// Only include match if the DA actually exists
								if len(match.DataAttributes) == 0 || match.DataAttributes[input.DaName] == nil {
									continue
								}
							} else {
								match.DataAttributes = attributesMap
							}
						}
					}

					matches = append(matches, match)
				}
			}
		}
	}

	return &FindPathResult{
		Matches:   matches,
		CallCount: callCount,
	}, nil
}

// filterDataAttributes filters the data attributes map to only include entries
// where the parent DA name matches the specified DaName exactly.
// This allows drilling down into specific data attributes and their nested children.
func filterDataAttributes(attributesMap map[string][]domain.DataAttribute, daName string) map[string][]domain.DataAttribute {
	filtered := make(map[string][]domain.DataAttribute)

	// Check if the exact DA name exists
	if attrs, exists := attributesMap[daName]; exists {
		filtered[daName] = attrs
	}

	return filtered
}

// FindPathBuilder provides a fluent API for building FindPath queries.
type FindPathBuilder struct {
	finder *Finder
	input  FindPathInput
}

// MatchingLN sets the logical node regex pattern (required).
func (b *FindPathBuilder) MatchingLN(pattern string) *FindPathBuilder {
	b.input.LNPattern = pattern
	return b
}

// WithDO sets the exact data object name to match (required).
func (b *FindPathBuilder) WithDO(doName string) *FindPathBuilder {
	b.input.DoName = doName
	return b
}

// WithDA sets the exact data attribute name to match (optional, auto-enables IncludingAttributes).
func (b *FindPathBuilder) WithDA(daName string) *FindPathBuilder {
	b.input.DaName = daName
	b.input.IncludeDas = true // Auto-enable
	return b
}

// IncludingAttributes enables listing all data attributes for matched paths.
func (b *FindPathBuilder) IncludingAttributes() *FindPathBuilder {
	b.input.IncludeDas = true
	return b
}

// WithDetails enables detailed data attribute information.
func (b *FindPathBuilder) WithDetails() *FindPathBuilder {
	b.input.Detailed = true
	return b
}

// Find executes the path search with the configured parameters.
func (b *FindPathBuilder) Find() (*FindPathResult, error) {
	return b.finder.FindPath(b.input)
}

// Path creates a new FindPathBuilder for fluent query construction.
// Example: finder.Path().MatchingLN(".*MMXU.*").WithDO("Hz").IncludingAttributes().Find().
func (f *Finder) Path() *FindPathBuilder {
	return &FindPathBuilder{
		finder: f,
		input:  FindPathInput{},
	}
}
