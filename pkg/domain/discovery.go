// SPDX-License-Identifier: MIT

package domain

// DiscoveredTarget represents a host discovered during network scanning.
type DiscoveredTarget struct {
	IP           string
	Port         int
	TCPReachable bool
	IEC61850OK   bool
	MACAddress   string
	Error        string
}
