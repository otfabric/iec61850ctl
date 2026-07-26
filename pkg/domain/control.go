// SPDX-License-Identifier: MIT

package domain

// ControlModel describes the control model of a controllable data object.
type ControlModel string

const (
	ControlStatusOnly     ControlModel = "status-only"
	ControlDirectNormal   ControlModel = "direct-normal"
	ControlSBONormal      ControlModel = "sbo-normal"
	ControlDirectEnhanced ControlModel = "direct-enhanced"
	ControlSBOEnhanced    ControlModel = "sbo-enhanced"
)

// ControlBlock describes a controllable data object and its control model.
type ControlBlock struct {
	Ref          string
	ControlModel ControlModel
	CtlNum       int
}

// ControlParams holds the parameters for a control operation.
type ControlParams struct {
	CtlVal      interface{}
	OrIdent     string
	OrCat       int
	Test        bool
	Check       bool
	OperateTime uint64
}
