// SPDX-License-Identifier: MIT

package domain

import (
	"fmt"
	"strings"

	iec61850 "github.com/otfabric/go-iec61850"
)

// Quality is the IEC 61850 quality bitfield (13 bits, IEC 61850-7-3).
type Quality uint16

// Validity represents the validity status encoded in the lower 2 bits of Quality.
type Validity uint8

const (
	ValidityGood         Validity = 0
	ValidityInvalid      Validity = 1
	ValidityReserved     Validity = 2
	ValidityQuestionable Validity = 3
)

const (
	QualityOverflow        Quality = 0x0004
	QualityOutOfRange      Quality = 0x0008
	QualityBadReference    Quality = 0x0010
	QualityOscillatory     Quality = 0x0020
	QualityFailure         Quality = 0x0040
	QualityOldData         Quality = 0x0080
	QualityInconsistent    Quality = 0x0100
	QualityInaccurate      Quality = 0x0200
	QualitySubstituted     Quality = 0x0400
	QualityTest            Quality = 0x0800
	QualityOperatorBlocked Quality = 0x1000
	QualityDerived         Quality = 0x2000
)

func (q Quality) GetValidity() Validity {
	return Validity(q & 0x3)
}

func (q Quality) IsGood() bool {
	return q.GetValidity() == ValidityGood
}

func (q Quality) DetailFlags() []string {
	var flags []string
	if q&QualityOverflow != 0 {
		flags = append(flags, "overflow")
	}
	if q&QualityOutOfRange != 0 {
		flags = append(flags, "out-of-range")
	}
	if q&QualityBadReference != 0 {
		flags = append(flags, "bad-reference")
	}
	if q&QualityOscillatory != 0 {
		flags = append(flags, "oscillatory")
	}
	if q&QualityFailure != 0 {
		flags = append(flags, "failure")
	}
	if q&QualityOldData != 0 {
		flags = append(flags, "old-data")
	}
	if q&QualityInconsistent != 0 {
		flags = append(flags, "inconsistent")
	}
	if q&QualityInaccurate != 0 {
		flags = append(flags, "inaccurate")
	}
	if q&QualitySubstituted != 0 {
		flags = append(flags, "substituted")
	}
	if q&QualityTest != 0 {
		flags = append(flags, "test")
	}
	if q&QualityOperatorBlocked != 0 {
		flags = append(flags, "operator-blocked")
	}
	if q&QualityDerived != 0 {
		flags = append(flags, "derived")
	}
	return flags
}

func (q Quality) String() string {
	var validity string
	switch q.GetValidity() {
	case ValidityGood:
		validity = "good"
	case ValidityInvalid:
		validity = "invalid"
	case ValidityReserved:
		validity = "reserved"
	case ValidityQuestionable:
		validity = "questionable"
	default:
		validity = fmt.Sprintf("unknown(%d)", q.GetValidity())
	}
	flags := q.DetailFlags()
	if len(flags) == 0 {
		return validity
	}
	return validity + " [" + strings.Join(flags, ", ") + "]"
}

// FromLibQuality converts go-iec61850.Quality to domain Quality.
func FromLibQuality(q iec61850.Quality) Quality {
	return Quality(q)
}
