// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package main

import (
	"fmt"
	"os"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// printJSON marshals a proto message to pretty-printed JSON on stdout.
func printJSON(msg proto.Message) error {
	data, err := protojson.MarshalOptions{Multiline: true}.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}
	if _, err := os.Stdout.Write(data); err != nil {
		return err
	}
	fmt.Println()
	return nil
}
