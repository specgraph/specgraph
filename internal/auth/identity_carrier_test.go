// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/specgraph/specgraph/internal/reqctx"
	"github.com/stretchr/testify/assert"
)

// stubResolver resolves any token to a fixed identity.
type stubResolver struct{ id *Identity }

func (s stubResolver) Resolve(context.Context, string) (*Identity, error) { return s.id, nil }

func (s stubResolver) HasAuth(context.Context) (bool, error) { return true, nil }

func TestRequireAuth_WritesIdentityToCarrier(t *testing.T) {
	res := stubResolver{id: &Identity{Subject: "apikey:k1"}}
	ctx, info := reqctx.NewContext(context.Background())

	var sawIdentity string
	h := RequireAuth(res)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		if got := reqctx.FromContext(r.Context()); got != nil {
			sawIdentity = got.Identity
		}
	}))
	req := httptest.NewRequest(http.MethodGet, "/x", http.NoBody).WithContext(ctx)
	req.Header.Set("Authorization", "Bearer t")
	h.ServeHTTP(httptest.NewRecorder(), req)

	assert.Equal(t, "apikey:k1", sawIdentity, "RequireAuth must write identity into the carrier")
	assert.Equal(t, "apikey:k1", info.Identity)
}
