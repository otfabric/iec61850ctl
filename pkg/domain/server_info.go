// SPDX-License-Identifier: MIT

package domain

// ServerIdentity holds MMS server identity information (Identify service).
type ServerIdentity struct {
	VendorName string
	ModelName  string
	Revision   string
}

// ServerStatus holds MMS server status information (Status service).
type ServerStatus struct {
	LogicalStatus  int32
	PhysicalStatus int32
	LocalDetail    int32
}

// ConnectionParams holds the negotiated MMS connection parameters.
type ConnectionParams struct {
	MaxServOutstandingCalling int32
	MaxServOutstandingCalled  int32
	NestingLevel              int32
	MaxPduSize                int32
}
