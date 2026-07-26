// SPDX-License-Identifier: MIT

package app

import (
	"io"
	"time"

	"github.com/otfabric/iec61850ctl/pkg/domain"
	"github.com/otfabric/iec61850ctl/pkg/service"
)

// TreeInput configures a device tree render or serialize.
type TreeInput struct {
	Host            string
	Port            int
	Path            string
	CallInterval    time.Duration
	IncludeDataSets bool
	IncludeReports  bool
}

// RenderTree writes the hierarchical device tree to w.
func (a *App) RenderTree(w io.Writer, input TreeInput) (int, error) {
	tree := service.NewTree(a.conn).WithCallInterval(input.CallInterval)
	return tree.RenderDeviceTree(w, input.Host, input.Port, input.Path)
}

// BuildSerializableTree builds a domain.IED model for JSON serialization.
func (a *App) BuildSerializableTree(input TreeInput) (*domain.IED, error) {
	tree := service.NewTree(a.conn).WithCallInterval(input.CallInterval)
	return tree.BuildSerializableModel(input.Host, input.Port, input.Path, input.IncludeDataSets, input.IncludeReports)
}
