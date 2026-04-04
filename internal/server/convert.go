// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server

import (
	"math"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"
)

// safeConvCount clamps an int to the int32 range for proto serialization.
func safeConvCount(v int) int32 {
	if v > math.MaxInt32 {
		return math.MaxInt32
	}
	if v < 0 {
		return 0
	}
	return int32(v)
}

// timeToProto converts a time.Time to a protobuf Timestamp, returning nil for zero values.
func timeToProto(t time.Time) *timestamppb.Timestamp {
	if t.IsZero() {
		return nil
	}
	return timestamppb.New(t)
}
