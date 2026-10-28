// SPDX-License-Identifier: MIT

package client

import (
	"math/rand/v2"
	"time"
)

// equalJitter returns a delay uniformly distributed in [d/2, d], which keeps a
// lower bound on backoff while de-synchronising concurrent clients.
func equalJitter(d time.Duration) time.Duration {
	if d <= 1 {
		return d
	}
	half := d / 2
	return half + rand.N(d-half+1) //nolint:gosec // jitter, not security
}
