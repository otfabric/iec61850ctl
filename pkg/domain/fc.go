// SPDX-License-Identifier: MIT

// Package domain provides canonical IEC 61850 types used by services and views.
package domain

import (
	"strings"

	iec61850 "github.com/otfabric/go-iec61850"
)

// FunctionalConstraint is a string-typed enum for IEC 61850 functional constraints.
type FunctionalConstraint string

const (
	FC_ST   FunctionalConstraint = "ST"
	FC_MX   FunctionalConstraint = "MX"
	FC_SP   FunctionalConstraint = "SP"
	FC_SV   FunctionalConstraint = "SV"
	FC_CF   FunctionalConstraint = "CF"
	FC_DC   FunctionalConstraint = "DC"
	FC_SG   FunctionalConstraint = "SG"
	FC_SE   FunctionalConstraint = "SE"
	FC_SR   FunctionalConstraint = "SR"
	FC_OR   FunctionalConstraint = "OR"
	FC_BL   FunctionalConstraint = "BL"
	FC_EX   FunctionalConstraint = "EX"
	FC_CO   FunctionalConstraint = "CO"
	FC_US   FunctionalConstraint = "US"
	FC_MS   FunctionalConstraint = "MS"
	FC_RP   FunctionalConstraint = "RP"
	FC_BR   FunctionalConstraint = "BR"
	FC_LG   FunctionalConstraint = "LG"
	FC_GO   FunctionalConstraint = "GO"
	FC_ALL  FunctionalConstraint = "ALL"
	FC_NONE FunctionalConstraint = ""
)

var allFCs = []FunctionalConstraint{
	FC_ST, FC_MX, FC_SP, FC_SV, FC_CF, FC_DC, FC_SG, FC_SE,
	FC_SR, FC_OR, FC_BL, FC_EX, FC_CO, FC_US, FC_MS, FC_RP,
	FC_BR, FC_LG, FC_GO,
}

// AllFCs returns all defined FC values (excluding NONE and ALL).
func AllFCs() []FunctionalConstraint {
	out := make([]FunctionalConstraint, len(allFCs))
	copy(out, allFCs)
	return out
}

// ParseFC parses a user string (case-insensitive) to a FunctionalConstraint.
func ParseFC(s string) FunctionalConstraint {
	upper := strings.ToUpper(strings.TrimSpace(s))
	switch upper {
	case "ST":
		return FC_ST
	case "MX":
		return FC_MX
	case "SP":
		return FC_SP
	case "SV":
		return FC_SV
	case "CF":
		return FC_CF
	case "DC":
		return FC_DC
	case "SG":
		return FC_SG
	case "SE":
		return FC_SE
	case "SR":
		return FC_SR
	case "OR":
		return FC_OR
	case "BL":
		return FC_BL
	case "EX":
		return FC_EX
	case "CO":
		return FC_CO
	case "US":
		return FC_US
	case "MS":
		return FC_MS
	case "RP":
		return FC_RP
	case "BR":
		return FC_BR
	case "LG":
		return FC_LG
	case "GO":
		return FC_GO
	case "ALL":
		return FC_ALL
	default:
		return FC_NONE
	}
}

func (fc FunctionalConstraint) String() string {
	if fc == FC_NONE {
		return "NONE"
	}
	return string(fc)
}

// IsValid returns true if the FC is a known, concrete value (not NONE).
func (fc FunctionalConstraint) IsValid() bool {
	return fc != FC_NONE && ParseFC(string(fc)) != FC_NONE
}

// ToLibFC converts to go-iec61850.FunctionalConstraint.
func (fc FunctionalConstraint) ToLibFC() iec61850.FunctionalConstraint {
	if fc == FC_ALL || fc == FC_NONE {
		return ""
	}
	parsed, err := iec61850.ParseFC(string(fc))
	if err != nil {
		return ""
	}
	return parsed
}

// FromLibFC converts go-iec61850.FunctionalConstraint to domain FC.
func FromLibFC(fc iec61850.FunctionalConstraint) FunctionalConstraint {
	return ParseFC(string(fc))
}
