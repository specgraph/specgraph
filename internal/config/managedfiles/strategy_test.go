// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import "testing"

func TestStrategyString(t *testing.T) {
	cases := []struct {
		s    Strategy
		want string
	}{
		{StrategyJSONKeyMerge, "JSONKeyMerge"},
		{StrategyMarkdownBlock, "MarkdownBlock"},
		{StrategyWholeFile, "WholeFile"},
		{Strategy(99), "Strategy(99)"},
	}
	for _, tc := range cases {
		if got := tc.s.String(); got != tc.want {
			t.Errorf("Strategy(%d).String() = %q, want %q", int(tc.s), got, tc.want)
		}
	}
}
