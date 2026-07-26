// SPDX-License-Identifier: MIT

// Package client provides connection management for IEC 61850 MMS clients
// built on github.com/otfabric/go-iec61850.
package client

import (
	"context"
	"fmt"
	"time"

	iec61850 "github.com/otfabric/go-iec61850"
	"github.com/otfabric/iec61850ctl/pkg/service"
)

// ConnectionInput defines parameters for creating an IEC 61850 client connection.
type ConnectionInput struct {
	Host           string // Server hostname or IP address
	Port           int    // Server port (typically 102)
	ConnectTimeout int    // Connection timeout in seconds
	RequestTimeout int    // Request timeout in seconds (reserved for future dial options)
}

// NewConnection dials an IEC 61850 MMS server and returns a service connection adapter.
func NewConnection(input ConnectionInput) (service.IEC61850Connection, error) {
	connectSec := input.ConnectTimeout
	if connectSec <= 0 {
		connectSec = 10
	}
	addr := fmt.Sprintf("%s:%d", input.Host, input.Port)
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(connectSec)*time.Second)
	defer cancel()

	c, err := iec61850.Dial(ctx, addr, iec61850.DialOptions{})
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", addr, err)
	}
	return service.NewClientAdapter(c), nil
}
