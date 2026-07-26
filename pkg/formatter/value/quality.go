// SPDX-License-Identifier: MIT

package value

import (
	"fmt"
	"strings"

	"github.com/otfabric/iec61850ctl/pkg/domain"
)

// isQualityAttr returns true if the attribute name or path refers to the IEC 61850 quality attribute "q".
func isQualityAttr(nameOrPath string) bool {
	return nameOrPath == "q" || strings.HasSuffix(nameOrPath, ".q")
}

// formatBitStringRaw formats a bitstring value as hex without interpreting it as quality.
func formatBitStringRaw(value interface{}) string {
	var q uint16
	switch v := value.(type) {
	case uint16:
		q = v
	case int32:
		q = uint16(v)
	case uint32:
		q = uint16(v)
	case int:
		q = uint16(v)
	case []byte:
		if len(v) >= 2 {
			q = uint16(v[0])<<8 | uint16(v[1])
		} else if len(v) == 1 {
			q = uint16(v[0])
		}
	default:
		return fmt.Sprintf("%v", value)
	}
	return fmt.Sprintf("0x%04x", q)
}

// FormatQuality converts IEC 61850 Quality flags to a human-readable string.
// Returns empty string if quality is nil.
func FormatQuality(q *domain.Quality) string {
	if q == nil {
		return ""
	}

	quality := *q
	var result string

	switch quality.GetValidity() {
	case domain.ValidityGood:
		result = "good"
	case domain.ValidityInvalid:
		result = "invalid"
	case domain.ValidityReserved:
		result = "reserved"
	case domain.ValidityQuestionable:
		result = "questionable"
	}

	flags := []string{}
	if quality&domain.QualityOverflow != 0 {
		flags = append(flags, "overflow")
	}
	if quality&domain.QualityOutOfRange != 0 {
		flags = append(flags, "out-of-range")
	}
	if quality&domain.QualityBadReference != 0 {
		flags = append(flags, "bad-ref")
	}
	if quality&domain.QualityOscillatory != 0 {
		flags = append(flags, "oscillatory")
	}
	if quality&domain.QualityFailure != 0 {
		flags = append(flags, "failure")
	}
	if quality&domain.QualityOldData != 0 {
		flags = append(flags, "old-data")
	}
	if quality&domain.QualityInconsistent != 0 {
		flags = append(flags, "inconsistent")
	}
	if quality&domain.QualityInaccurate != 0 {
		flags = append(flags, "inaccurate")
	}
	if quality&domain.QualitySubstituted != 0 {
		flags = append(flags, "substituted")
	}
	if quality&domain.QualityTest != 0 {
		flags = append(flags, "test")
	}
	if quality&domain.QualityOperatorBlocked != 0 {
		flags = append(flags, "operator-blocked")
	}

	if len(flags) > 0 {
		for _, flag := range flags {
			result += "," + flag
		}
	}

	return result
}

// FormatQualityValue formats a quality bitfield value to human-readable flags.
func FormatQualityValue(value interface{}) string {
	var q uint16

	switch v := value.(type) {
	case uint16:
		q = v
	case int32:
		q = uint16(v)
	case uint32:
		q = uint16(v)
	case int:
		q = uint16(v)
	case []byte:
		if len(v) >= 2 {
			q = uint16(v[0])<<8 | uint16(v[1])
		} else if len(v) == 1 {
			q = uint16(v[0])
		}
	default:
		return fmt.Sprintf("%v (raw)", value)
	}

	quality := domain.Quality(q)
	var result string

	switch quality.GetValidity() {
	case domain.ValidityGood:
		result = "good"
	case domain.ValidityInvalid:
		result = "invalid"
	case domain.ValidityReserved:
		result = "reserved"
	case domain.ValidityQuestionable:
		result = "questionable"
	}

	flags := []string{}
	if quality&domain.QualityOverflow != 0 {
		flags = append(flags, "overflow")
	}
	if quality&domain.QualityOutOfRange != 0 {
		flags = append(flags, "out-of-range")
	}
	if quality&domain.QualityBadReference != 0 {
		flags = append(flags, "bad-ref")
	}
	if quality&domain.QualityOscillatory != 0 {
		flags = append(flags, "oscillatory")
	}
	if quality&domain.QualityFailure != 0 {
		flags = append(flags, "failure")
	}
	if quality&domain.QualityOldData != 0 {
		flags = append(flags, "old-data")
	}
	if quality&domain.QualityInconsistent != 0 {
		flags = append(flags, "inconsistent")
	}
	if quality&domain.QualityInaccurate != 0 {
		flags = append(flags, "inaccurate")
	}
	if quality&domain.QualitySubstituted != 0 {
		flags = append(flags, "substituted")
	}
	if quality&domain.QualityTest != 0 {
		flags = append(flags, "test")
	}
	if quality&domain.QualityOperatorBlocked != 0 {
		flags = append(flags, "operator-blocked")
	}

	detailStr := "none"
	if len(flags) > 0 {
		detailStr = strings.Join(flags, ", ")
	}
	return fmt.Sprintf("0x%04x [%s] | details: %s", q, result, detailStr)
}
