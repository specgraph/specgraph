// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package main

import (
	"encoding/json"
	"fmt"
	"os"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// printSafetyFlags prints each safety flag in a standard format.
func printSafetyFlags(flags []*specv1.SafetyFlag) {
	for _, f := range flags {
		if f == nil {
			continue
		}
		fmt.Printf("  [%s] %s: %s\n", f.Severity, f.Category, f.Description)
	}
}

// loadJSONFile reads a JSON file and unmarshals it into a proto message.
// T must be a proto.Message implementation (e.g. *specv1.ShapeOutput).
//
// Security note: path must be a user-supplied value from CLI context only.
// This function must NOT be used in server-side code paths where path could
// originate from untrusted network input.

// loadJSONFileRaw reads a JSON file and unmarshals it into a plain Go value.
// Use this for non-proto structs; for proto messages use loadJSONFile.
//
// Security note: path must be a user-supplied value from CLI context only.
func loadJSONFileRaw(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	return nil
}

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
