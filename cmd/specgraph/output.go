// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package main

import (
	"fmt"
	"io"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// printJSON marshals a proto message to pretty-printed JSON on the given writer.
func printJSON(w io.Writer, msg proto.Message) error {
	data, err := protojson.MarshalOptions{Multiline: true}.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}
	if _, err = w.Write(data); err != nil {
		return err
	}
	_, err = fmt.Fprintln(w)
	return err
}
