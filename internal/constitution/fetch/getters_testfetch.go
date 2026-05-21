// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

//go:build testfetch

package fetch

import "github.com/hashicorp/go-getter"

// init runs only when built with -tags testfetch. It overrides the
// default getter registry to include the file:// scheme, used by tests.
// Production builds NEVER register file://.
func init() {
	defaultGetters = func(token string) map[string]getter.Getter {
		g := restrictedGetters(token)
		g["file"] = new(getter.FileGetter)
		return g
	}
}
