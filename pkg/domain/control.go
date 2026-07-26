// SPDX-License-Identifier: MIT

package domain

import (
	"fmt"
	"strings"
)

// ControlMode selects how a control operate journey is executed.
type ControlMode string

const (
	ControlModeAuto   ControlMode = "auto"
	ControlModeDirect ControlMode = "direct"
	ControlModeSBO    ControlMode = "sbo"
	ControlModeSBOw   ControlMode = "sbow"
)

// ParseControlMode parses a control mode flag value.
func ParseControlMode(s string) (ControlMode, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "auto":
		return ControlModeAuto, nil
	case "direct":
		return ControlModeDirect, nil
	case "sbo":
		return ControlModeSBO, nil
	case "sbow":
		return ControlModeSBOw, nil
	default:
		return "", fmt.Errorf("invalid control mode %q (want auto|direct|sbo|sbow)", s)
	}
}

// ControlStatus is the automation status of a control operate journey.
type ControlStatus string

const (
	ControlStatusPlanned              ControlStatus = "planned"
	ControlStatusOperated             ControlStatus = "operated"
	ControlStatusConfirmed            ControlStatus = "confirmed"
	ControlStatusOperatedUnconfirmed  ControlStatus = "operated-unconfirmed"
	ControlStatusConfirmationMismatch ControlStatus = "confirmation-mismatch"
	ControlStatusFailed               ControlStatus = "failed"
)

// ExitNonZero reports whether this status should cause a non-zero process exit.
func (s ControlStatus) ExitNonZero() bool {
	switch s {
	case ControlStatusOperatedUnconfirmed, ControlStatusConfirmationMismatch, ControlStatusFailed:
		return true
	default:
		return false
	}
}

// ControlModelInfo describes a ctlModel value.
type ControlModelInfo struct {
	Code int    `json:"code"`
	Name string `json:"name"`
}

// OriginCategory identifies the IEC 61850 orCat.
type OriginCategory string

const (
	OrCatNotSupported     OriginCategory = "not-supported"
	OrCatBayControl       OriginCategory = "bay-control"
	OrCatStationControl   OriginCategory = "station-control"
	OrCatRemoteControl    OriginCategory = "remote-control"
	OrCatAutomaticBay     OriginCategory = "automatic-bay"
	OrCatAutomaticStation OriginCategory = "automatic-station"
	OrCatAutomaticRemote  OriginCategory = "automatic-remote"
	OrCatMaintenance      OriginCategory = "maintenance"
	OrCatProcess          OriginCategory = "process"
)

// ParseOriginCategory parses an --or-cat flag value.
func ParseOriginCategory(s string) (OriginCategory, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "remote-control":
		return OrCatRemoteControl, nil
	case "not-supported":
		return OrCatNotSupported, nil
	case "bay-control":
		return OrCatBayControl, nil
	case "station-control":
		return OrCatStationControl, nil
	case "automatic-bay":
		return OrCatAutomaticBay, nil
	case "automatic-station":
		return OrCatAutomaticStation, nil
	case "automatic-remote":
		return OrCatAutomaticRemote, nil
	case "maintenance":
		return OrCatMaintenance, nil
	case "process":
		return OrCatProcess, nil
	default:
		return "", fmt.Errorf("invalid or-cat %q", s)
	}
}

// OrCatCode returns the IEC 61850 integer code for the category.
func (c OriginCategory) OrCatCode() int {
	switch c {
	case OrCatNotSupported:
		return 0
	case OrCatBayControl:
		return 1
	case OrCatStationControl:
		return 2
	case OrCatRemoteControl:
		return 3
	case OrCatAutomaticBay:
		return 4
	case OrCatAutomaticStation:
		return 5
	case OrCatAutomaticRemote:
		return 6
	case OrCatMaintenance:
		return 7
	case OrCatProcess:
		return 8
	default:
		return 3
	}
}

// MaxOrIdentBytes is the IEC 61850-8-1 limit for Origin.orIdent.
const MaxOrIdentBytes = 64

// CheckConditions holds synchrocheck / interlockCheck bits.
type CheckConditions struct {
	Synchro   bool
	Interlock bool
}

// ScalarKind is an explicit scalar type for control/write values.
type ScalarKind string

const (
	ScalarBool   ScalarKind = "bool"
	ScalarInt    ScalarKind = "int"
	ScalarUint   ScalarKind = "uint"
	ScalarFloat  ScalarKind = "float"
	ScalarEnum   ScalarKind = "enum"
	ScalarString ScalarKind = "string"
)

// ParseScalarKind parses a --type flag value.
func ParseScalarKind(s string) (ScalarKind, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "bool", "boolean":
		return ScalarBool, nil
	case "int", "integer":
		return ScalarInt, nil
	case "uint", "unsigned":
		return ScalarUint, nil
	case "float", "double":
		return ScalarFloat, nil
	case "enum":
		return ScalarEnum, nil
	case "string":
		return ScalarString, nil
	default:
		return "", fmt.Errorf("invalid type %q (want bool|int|uint|float|enum|string)", s)
	}
}

// ScalarValue is a typed scalar without interface{}.
type ScalarValue struct {
	Kind   ScalarKind
	Bool   bool
	Int    int64
	Uint   uint64
	Float  float64
	String string
}

// ControlOrigin is the originator of a control command.
type ControlOrigin struct {
	Category OriginCategory
	Ident    string // UTF-8; max MaxOrIdentBytes
}

// ControlRequest is an atomic control operate journey.
type ControlRequest struct {
	Object     string
	Mode       ControlMode
	Value      ScalarValue
	CtlNum     uint8 // 0 = auto-allocate non-zero
	Origin     ControlOrigin
	Test       bool
	Check      CheckConditions
	ConfirmRef string // empty = no confirmation
	NoConfirm  bool
	DryRun     bool
}

// ControlOperation records one step in the journey.
type ControlOperation struct {
	Operation string `json:"operation"`
	OK        bool   `json:"ok"`
	Error     string `json:"error,omitempty"`
	Detail    string `json:"detail,omitempty"`
}

// ControlConfirmation is an optional status read-back.
type ControlConfirmation struct {
	Attempted bool   `json:"attempted"`
	Object    string `json:"object,omitempty"`
	FC        string `json:"fc,omitempty"`
	Value     any    `json:"value,omitempty"`
	Matched   bool   `json:"matched,omitempty"`
	Error     string `json:"error,omitempty"`
}

// ControlCleanup records best-effort Cancel after a failed operate.
type ControlCleanup struct {
	Operation string `json:"operation"`
	Attempted bool   `json:"attempted"`
	OK        bool   `json:"ok"`
	Error     string `json:"error,omitempty"`
}

// LastApplicationError is a validated LastApplError projection.
type LastApplicationError struct {
	ControlObject string `json:"control_object"`
	Error         int    `json:"error"`
	AddCause      string `json:"add_cause"`
	Matched       bool   `json:"matched"`
}

// ControlResult is the outcome of Inspect or Operate.
type ControlResult struct {
	Object         string                `json:"object"`
	ControlModel   ControlModelInfo      `json:"control_model"`
	Controllable   bool                  `json:"controllable,omitempty"`
	RequiresSelect bool                  `json:"requires_select,omitempty"`
	Enhanced       bool                  `json:"enhanced_security,omitempty"`
	Mode           ControlMode           `json:"mode,omitempty"`
	RequestedValue any                   `json:"requested_value,omitempty"`
	CtlNum         uint8                 `json:"ctl_num,omitempty"`
	Operations     []ControlOperation    `json:"operations,omitempty"`
	Confirmation   *ControlConfirmation  `json:"confirmation,omitempty"`
	Cleanup        *ControlCleanup       `json:"cleanup,omitempty"`
	Status         ControlStatus         `json:"status,omitempty"`
	LastError      *LastApplicationError `json:"last_appl_error,omitempty"`
}

// WriteRequest is an explicit scalar MMS write.
type WriteRequest struct {
	Object string
	FC     FunctionalConstraint
	Value  ScalarValue
	Verify bool
}

// WriteVerification is an optional read-back after write.
type WriteVerification struct {
	Attempted bool   `json:"attempted"`
	Value     any    `json:"value,omitempty"`
	Matched   bool   `json:"matched,omitempty"`
	Error     string `json:"error,omitempty"`
}

// WriteResult is the outcome of set object.
type WriteResult struct {
	Object         string             `json:"object"`
	FC             string             `json:"fc"`
	Type           string             `json:"type"`
	RequestedValue any                `json:"requested_value"`
	WriteOK        bool               `json:"write_ok"`
	Error          string             `json:"error,omitempty"`
	Verification   *WriteVerification `json:"verification,omitempty"`
}
