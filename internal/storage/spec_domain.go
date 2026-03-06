// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package storage

import "time"

// Spec is the storage-layer domain type for specifications.
// Handlers convert between this type and the proto Spec message.
type Spec struct {
	ID         string
	Slug       string
	Intent     string
	Stage      string
	Priority   string
	Complexity string
	Version    int32
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
