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
func loadJSONFile[T proto.Message](path string, msg T) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	if err := protojson.Unmarshal(data, msg); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	return nil
}
