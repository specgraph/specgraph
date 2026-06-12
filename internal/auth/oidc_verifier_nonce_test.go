// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

import "testing"

func TestNonceMatches(t *testing.T) {
	if !nonceMatches("abc", "abc") {
		t.Fatal("equal nonces should match")
	}
	if nonceMatches("abc", "xyz") {
		t.Fatal("different nonces must not match")
	}
	if nonceMatches("", "abc") {
		t.Fatal("empty received nonce must not match a non-empty expected nonce")
	}
	if nonceMatches("", "") {
		t.Fatal("both-empty must not match (the empty-want guard's purpose)")
	}
}
