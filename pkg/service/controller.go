// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"

	iec61850 "github.com/otfabric/go-iec61850"
	"github.com/otfabric/iec61850ctl/pkg/domain"
)

// Controller orchestrates atomic IEC 61850 control journeys on one association.
type Controller struct {
	conn ControlConnection
}

// NewController returns a Controller bound to conn.
func NewController(conn ControlConnection) *Controller {
	return &Controller{conn: conn}
}

var ctlNumCounter uint32

func nextCtlNum() uint8 {
	n := atomic.AddUint32(&ctlNumCounter, 1)
	if uint8(n) == 0 {
		n = atomic.AddUint32(&ctlNumCounter, 1)
	}
	return uint8(n)
}

// Inspect reads ctlModel for a controllable DO reference (no FC).
func (c *Controller) Inspect(ctx context.Context, object string) (*domain.ControlResult, error) {
	ref, err := parseControlObjectRef(object)
	if err != nil {
		return nil, err
	}
	model, err := c.conn.ReadCtlModel(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("read ctlModel: %w", err)
	}
	return &domain.ControlResult{
		Object: object,
		ControlModel: domain.ControlModelInfo{
			Code: int(model),
			Name: model.String(),
		},
		Controllable:   model.IsControllable(),
		RequiresSelect: model.IsSBO(),
		Enhanced:       model.IsEnhanced(),
	}, nil
}

// Operate executes an atomic control journey.
func (c *Controller) Operate(ctx context.Context, req domain.ControlRequest) (*domain.ControlResult, error) {
	if err := validateControlRequest(req); err != nil {
		return nil, err
	}

	ref, err := parseControlObjectRef(req.Object)
	if err != nil {
		return nil, err
	}

	model, err := c.conn.ReadCtlModel(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("read ctlModel: %w", err)
	}

	result := &domain.ControlResult{
		Object: req.Object,
		ControlModel: domain.ControlModelInfo{
			Code: int(model),
			Name: model.String(),
		},
		Controllable:   model.IsControllable(),
		RequiresSelect: model.IsSBO(),
		Enhanced:       model.IsEnhanced(),
		Mode:           req.Mode,
		RequestedValue: ScalarRequestedAny(req.Value),
	}

	if !model.IsControllable() {
		result.Status = domain.ControlStatusFailed
		result.Operations = []domain.ControlOperation{{
			Operation: "ctlmodel",
			OK:        false,
			Error:     "status-only: object is not controllable",
		}}
		return result, nil
	}

	resolvedMode, modeErr := resolveControlMode(req.Mode, model)
	if modeErr != nil {
		result.Status = domain.ControlStatusFailed
		result.Operations = []domain.ControlOperation{{
			Operation: "mode",
			OK:        false,
			Error:     modeErr.Error(),
		}}
		return controlOutcome(result)
	}
	result.Mode = resolvedMode

	ctlNum := req.CtlNum
	if ctlNum == 0 {
		ctlNum = nextCtlNum()
	}
	result.CtlNum = ctlNum

	ctlVal, err := ScalarToCtlVal(req.Value)
	if err != nil {
		return nil, err
	}

	orIdent := []byte(req.Origin.Ident)
	origin := &iec61850.Origin{
		OrCat:   iec61850.OrCat(req.Origin.Category.OrCatCode()),
		OrIdent: orIdent,
	}
	var check iec61850.CheckConditions
	if req.Check.Synchro {
		check |= iec61850.CheckSynchroCheck
	}
	if req.Check.Interlock {
		check |= iec61850.CheckInterlockCheck
	}
	params := iec61850.OperateParams{
		CtlVal: ctlVal,
		Origin: origin,
		CtlNum: ctlNum,
		Test:   req.Test,
		Check:  check,
	}

	planned := plannedOperations(resolvedMode)
	if req.DryRun {
		result.Operations = planned
		result.Status = domain.ControlStatusPlanned
		return controlOutcome(result)
	}

	ops := make([]domain.ControlOperation, 0, len(planned)+1)

	switch resolvedMode {
	case domain.ControlModeAuto:
		// Unreachable: resolveControlMode never returns auto.
		return nil, fmt.Errorf("internal: unresolved auto mode")

	case domain.ControlModeDirect:
		ops = append(ops, domain.ControlOperation{Operation: "operate"})
		if opErr := c.conn.Operate(ctx, ref, params); opErr != nil {
			ops[len(ops)-1].OK = false
			ops[len(ops)-1].Error = opErr.Error()
			result.Operations = ops
			result.Status = domain.ControlStatusFailed
			result.LastError = c.lookupLastApplError(ctx, ref, req.Object)
			return controlOutcome(result)
		}
		ops[len(ops)-1].OK = true

	case domain.ControlModeSBO:
		ops = append(ops, domain.ControlOperation{Operation: "select"})
		sbo, selErr := c.conn.Select(ctx, ref)
		if selErr != nil || sbo == "" {
			ops[len(ops)-1].OK = false
			if selErr != nil {
				ops[len(ops)-1].Error = selErr.Error()
			} else {
				ops[len(ops)-1].Error = "select denied (empty SBO)"
			}
			result.Operations = ops
			result.Status = domain.ControlStatusFailed
			result.LastError = c.lookupLastApplError(ctx, ref, req.Object)
			return controlOutcome(result)
		}
		ops[len(ops)-1].OK = true
		ops[len(ops)-1].Detail = sbo

		ops = append(ops, domain.ControlOperation{Operation: "operate"})
		if opErr := c.conn.Operate(ctx, ref, params); opErr != nil {
			ops[len(ops)-1].OK = false
			ops[len(ops)-1].Error = opErr.Error()
			result.Operations = ops
			result.Status = domain.ControlStatusFailed
			result.Cleanup = c.tryCancel(ctx, ref, cancelFromOperate(params))
			result.LastError = c.lookupLastApplError(ctx, ref, req.Object)
			return controlOutcome(result)
		}
		ops[len(ops)-1].OK = true

	case domain.ControlModeSBOw:
		ops = append(ops, domain.ControlOperation{Operation: "select-with-value"})
		if selErr := c.conn.SelectWithValue(ctx, ref, params); selErr != nil {
			ops[len(ops)-1].OK = false
			ops[len(ops)-1].Error = selErr.Error()
			result.Operations = ops
			result.Status = domain.ControlStatusFailed
			result.LastError = c.lookupLastApplError(ctx, ref, req.Object)
			return controlOutcome(result)
		}
		ops[len(ops)-1].OK = true

		ops = append(ops, domain.ControlOperation{Operation: "operate"})
		if opErr := c.conn.Operate(ctx, ref, params); opErr != nil {
			ops[len(ops)-1].OK = false
			ops[len(ops)-1].Error = opErr.Error()
			result.Operations = ops
			result.Status = domain.ControlStatusFailed
			result.Cleanup = c.tryCancel(ctx, ref, cancelFromOperate(params))
			result.LastError = c.lookupLastApplError(ctx, ref, req.Object)
			return controlOutcome(result)
		}
		ops[len(ops)-1].OK = true
	}

	result.Operations = ops

	wantConfirm := req.ConfirmRef != "" && !req.NoConfirm && !req.Test
	if !wantConfirm {
		result.Status = domain.ControlStatusOperated
		return controlOutcome(result)
	}

	conf := &domain.ControlConfirmation{Attempted: true, Object: req.ConfirmRef}
	confirmRef, parseErr := iec61850.ParseRef(req.ConfirmRef)
	if parseErr != nil {
		conf.Error = parseErr.Error()
		result.Confirmation = conf
		result.Status = domain.ControlStatusOperatedUnconfirmed
		return controlOutcome(result)
	}
	conf.FC = string(confirmRef.FC)
	got, readErr := c.conn.Read(ctx, confirmRef)
	if readErr != nil {
		conf.Error = readErr.Error()
		result.Confirmation = conf
		result.Status = domain.ControlStatusOperatedUnconfirmed
		return controlOutcome(result)
	}
	conf.Value = ScalarFromIECValue(got)
	if !ScalarMatchesMMS(req.Value, got.MMS()) {
		conf.Matched = false
		result.Confirmation = conf
		result.Status = domain.ControlStatusConfirmationMismatch
		return controlOutcome(result)
	}
	conf.Matched = true
	result.Confirmation = conf
	result.Status = domain.ControlStatusConfirmed
	return controlOutcome(result)
}

// controlOutcome returns a completed ControlResult with a nil error so callers
// can emit JSON and map non-zero exit from result.Status (not from Go error).
func controlOutcome(r *domain.ControlResult) (*domain.ControlResult, error) {
	return r, nil
}

func cancelFromOperate(params iec61850.OperateParams) iec61850.CancelParams {
	return iec61850.CancelParams{
		CtlVal: params.CtlVal,
		Origin: params.Origin,
		CtlNum: params.CtlNum,
		OperTm: params.OperTm,
	}
}

func (c *Controller) tryCancel(ctx context.Context, ref iec61850.Ref, params iec61850.CancelParams) *domain.ControlCleanup {
	cleanup := &domain.ControlCleanup{Operation: "cancel", Attempted: true}
	if err := c.conn.Cancel(ctx, ref, params); err != nil {
		cleanup.OK = false
		cleanup.Error = err.Error()
		return cleanup
	}
	cleanup.OK = true
	return cleanup
}

func (c *Controller) lookupLastApplError(ctx context.Context, ref iec61850.Ref, object string) *domain.LastApplicationError {
	lae, err := c.conn.ReadLastApplError(ctx, ref)
	if err != nil || lae == nil {
		return nil
	}
	matched := lastApplErrorMatches(lae.CntrlObj, object)
	if !matched {
		return &domain.LastApplicationError{
			ControlObject: lae.CntrlObj,
			Error:         lae.Error,
			AddCause:      lae.AddCause.String(),
			Matched:       false,
		}
	}
	return &domain.LastApplicationError{
		ControlObject: lae.CntrlObj,
		Error:         lae.Error,
		AddCause:      lae.AddCause.String(),
		Matched:       true,
	}
}

func lastApplErrorMatches(cntrlObj, object string) bool {
	a := strings.TrimSpace(cntrlObj)
	b := strings.TrimSpace(object)
	if a == "" || b == "" {
		return false
	}
	if a == b {
		return true
	}
	// Servers may return MMS item IDs or Oper sub-refs.
	return strings.Contains(a, b) || strings.Contains(b, a) ||
		strings.HasPrefix(a, b+".") || strings.HasPrefix(b, a+".")
}

func parseControlObjectRef(object string) (iec61850.Ref, error) {
	ref, err := iec61850.ParseRef(object)
	if err != nil {
		return iec61850.Ref{}, fmt.Errorf("parse object: %w", err)
	}
	if ref.FC != "" {
		return iec61850.Ref{}, fmt.Errorf("control object must not include FC; got %q", object)
	}
	if ref.LD == "" || ref.LN == "" || len(ref.Path) == 0 {
		return iec61850.Ref{}, fmt.Errorf("control object must be LD/LN.DO; got %q", object)
	}
	return ref, nil
}

func validateControlRequest(req domain.ControlRequest) error {
	if strings.TrimSpace(req.Object) == "" {
		return fmt.Errorf("object is required")
	}
	if req.CtlNum != 0 && req.CtlNum < 1 {
		return fmt.Errorf("ctl-num must be 1..255")
	}
	ident := req.Origin.Ident
	if len([]byte(ident)) > domain.MaxOrIdentBytes {
		return fmt.Errorf("or-ident exceeds %d bytes", domain.MaxOrIdentBytes)
	}
	if req.Value.Kind == "" {
		return fmt.Errorf("value type is required")
	}
	return nil
}

func resolveControlMode(mode domain.ControlMode, model iec61850.CtlModel) (domain.ControlMode, error) {
	if mode == "" || mode == domain.ControlModeAuto {
		switch model {
		case iec61850.CtlModelDirectNormal, iec61850.CtlModelDirectEnhanced:
			return domain.ControlModeDirect, nil
		case iec61850.CtlModelSBONormal:
			return domain.ControlModeSBO, nil
		case iec61850.CtlModelSBOEnhanced:
			return domain.ControlModeSBOw, nil
		default:
			return "", fmt.Errorf("unsupported ctlModel %d", int(model))
		}
	}
	switch mode {
	case domain.ControlModeDirect:
		if model != iec61850.CtlModelDirectNormal && model != iec61850.CtlModelDirectEnhanced {
			return "", fmt.Errorf("mode direct incompatible with ctlModel %s", model)
		}
	case domain.ControlModeSBO:
		if model != iec61850.CtlModelSBONormal {
			return "", fmt.Errorf("mode sbo incompatible with ctlModel %s", model)
		}
	case domain.ControlModeSBOw:
		if model != iec61850.CtlModelSBOEnhanced {
			return "", fmt.Errorf("mode sbow incompatible with ctlModel %s", model)
		}
	default:
		return "", fmt.Errorf("invalid mode %q", mode)
	}
	return mode, nil
}

func plannedOperations(mode domain.ControlMode) []domain.ControlOperation {
	switch mode {
	case domain.ControlModeDirect:
		return []domain.ControlOperation{{Operation: "operate", OK: true}}
	case domain.ControlModeSBO:
		return []domain.ControlOperation{
			{Operation: "select", OK: true},
			{Operation: "operate", OK: true},
		}
	case domain.ControlModeSBOw:
		return []domain.ControlOperation{
			{Operation: "select-with-value", OK: true},
			{Operation: "operate", OK: true},
		}
	default:
		return nil
	}
}
