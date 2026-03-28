// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsLoopbackAddr(t *testing.T) {
	tests := []struct {
		addr     string
		loopback bool
	}{
		{"127.0.0.1:7890", true},
		{"localhost:7890", true},
		{"::1:7890", false}, // net.SplitHostPort sees "::1" as invalid host:port — edge case
		{"[::1]:7890", true},
		{"0.0.0.0:7890", false},
		{"192.168.1.100:7890", false},
		{"10.0.0.1:9090", false},
		{"localhost", true},
		{"127.0.0.1", true},
		{"::1", true},
	}
	for _, tt := range tests {
		t.Run(tt.addr, func(t *testing.T) {
			assert.Equal(t, tt.loopback, isLoopbackAddr(tt.addr))
		})
	}
}
