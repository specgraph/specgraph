// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package main

import (
	"fmt"
	"os"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// loadJSONFile reads a JSON file and unmarshals it into a proto message.
// T must be a proto.Message implementation (e.g. *specv1.ShapeOutput).
//
// Security note: path must be a user-supplied value from CLI context only.
// This function must NOT be used in server-side code paths where path could
// originate from untrusted network input.
func loadJSONFile[T proto.Message](path string, msg T) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	opts := protojson.UnmarshalOptions{DiscardUnknown: false}
	if err := opts.Unmarshal(data, msg); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	return nil
}
