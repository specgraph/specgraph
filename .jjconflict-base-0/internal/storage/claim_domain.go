// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package storage

import "time"

// Claim represents an active claim/lease on a spec by an agent.
type Claim struct {
	Slug         string
	Agent        string
	LeaseExpires time.Time
	ClaimedAt    time.Time
}
