// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package fetch

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStripAuthOnCrossHost(t *testing.T) {
	mkReq := func(host string) *http.Request {
		return &http.Request{
			URL:    &url.URL{Host: host, Scheme: "https"},
			Header: http.Header{"Authorization": []string{"Bearer token123"}},
		}
	}

	t.Run("same-host preserves header", func(t *testing.T) {
		prev := mkReq("raw.githubusercontent.com")
		cur := mkReq("raw.githubusercontent.com")
		require.NoError(t, stripAuthOnCrossHost(cur, []*http.Request{prev}))
		assert.NotEmpty(t, cur.Header.Get("Authorization"))
	})

	t.Run("cross-host to non-allow-list strips header", func(t *testing.T) {
		prev := mkReq("raw.githubusercontent.com")
		cur := mkReq("attacker.example.com")
		require.NoError(t, stripAuthOnCrossHost(cur, []*http.Request{prev}))
		assert.Empty(t, cur.Header.Get("Authorization"),
			"Authorization must be stripped on cross-host redirect")
	})

	t.Run("cross-host to allow-list preserves header", func(t *testing.T) {
		prev := mkReq("raw.githubusercontent.com")
		cur := mkReq("api.github.com")
		require.NoError(t, stripAuthOnCrossHost(cur, []*http.Request{prev}))
		assert.NotEmpty(t, cur.Header.Get("Authorization"))
	})

	t.Run("no prior request preserves header", func(t *testing.T) {
		cur := mkReq("raw.githubusercontent.com")
		require.NoError(t, stripAuthOnCrossHost(cur, nil))
		assert.NotEmpty(t, cur.Header.Get("Authorization"))
	})
}

func TestHostAllowed(t *testing.T) {
	cases := map[string]bool{
		"raw.githubusercontent.com": true,
		"api.github.com":            true,
		"foo.githubusercontent.com": true,
		"github.com":                false,
		"example.com":               false,
		"":                          false,
		"attacker.com.fake.com":     false,
	}
	for host, want := range cases {
		assert.Equal(t, want, hostAllowed(host), "hostAllowed(%q)", host)
	}
}
