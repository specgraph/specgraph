// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestJSONPointerSet_RoundTrip(t *testing.T) {
	cases := []struct {
		name    string
		base    string
		pointer string
		value   any
		want    string
	}{
		{
			name:    "top-level key",
			base:    `{}`,
			pointer: "/foo",
			value:   "bar",
			want:    `{"foo":"bar"}`,
		},
		{
			name:    "nested key creates intermediate object",
			base:    `{}`,
			pointer: "/a/b/c",
			value:   42.0,
			want:    `{"a":{"b":{"c":42}}}`,
		},
		{
			name:    "key with @ does not need escaping",
			base:    `{}`,
			pointer: "/enabledPlugins/specgraph@specgraph-local",
			value:   true,
			want:    `{"enabledPlugins":{"specgraph@specgraph-local":true}}`,
		},
		{
			name:    "RFC 6901 escape: ~1 means /",
			base:    `{}`,
			pointer: "/path~1with~1slash",
			value:   "v",
			want:    `{"path/with/slash":"v"}`,
		},
		{
			name:    "RFC 6901 escape: ~0 means ~",
			base:    `{}`,
			pointer: "/tilde~0key",
			value:   "v",
			want:    `{"tilde~key":"v"}`,
		},
		{
			name:    "overwrite existing value",
			base:    `{"foo":"old"}`,
			pointer: "/foo",
			value:   "new",
			want:    `{"foo":"new"}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var doc map[string]any
			if err := json.Unmarshal([]byte(tc.base), &doc); err != nil {
				t.Fatalf("unmarshal base: %v", err)
			}
			if err := jsonPointerSet(doc, tc.pointer, tc.value); err != nil {
				t.Fatalf("jsonPointerSet: %v", err)
			}
			got, err := json.Marshal(doc)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			var gotAny, wantAny any
			_ = json.Unmarshal(got, &gotAny)
			_ = json.Unmarshal([]byte(tc.want), &wantAny)
			if !reflect.DeepEqual(gotAny, wantAny) {
				t.Errorf("got %s, want %s", got, tc.want)
			}
		})
	}
}

func TestJSONPointerGet(t *testing.T) {
	doc := map[string]any{
		"enabledPlugins": map[string]any{
			"specgraph@specgraph-local": true,
		},
	}
	v, ok := jsonPointerGet(doc, "/enabledPlugins/specgraph@specgraph-local")
	if !ok {
		t.Fatal("expected key present, got missing")
	}
	if v != true {
		t.Errorf("got %v, want true", v)
	}
	if _, ok := jsonPointerGet(doc, "/nonexistent"); ok {
		t.Error("expected missing key to report not-present")
	}
}
