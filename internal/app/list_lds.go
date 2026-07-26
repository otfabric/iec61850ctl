// SPDX-License-Identifier: MIT

package app

import (
	"github.com/otfabric/iec61850ctl/pkg/service"
	"github.com/otfabric/iec61850ctl/pkg/view"
)

// ListLogicalDevices returns all logical devices with summary counts.
func (a *App) ListLogicalDevices() ([]view.LogicalDevice, error) {
	explorer := a.Explorer()
	lds, err := explorer.GetLogicalDevices()
	if err != nil {
		return nil, err
	}
	return service.ProjectLogicalDevices(lds), nil
}

// ListLogicalDeviceNames returns just the logical device names (fast, no enrichment).
func (a *App) ListLogicalDeviceNames() ([]string, error) {
	return a.Explorer().ListLogicalDevices()
}
