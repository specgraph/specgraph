// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/specgraph/specgraph/internal/server"
	"github.com/specgraph/specgraph/internal/storage"
	"github.com/specgraph/specgraph/internal/storage/postgres"
	"github.com/stretchr/testify/require"
)

// failingOnSafetyFlagsStore wraps a *postgres.Store and forces StoreSafetyFlags
// to return an injected error, simulating a mid-transaction failure on the
// third op of the four-op Shape transaction.
type failingOnSafetyFlagsStore struct {
	*postgres.Store
}

// Compile-time assertion: failingOnSafetyFlagsStore must satisfy ScopedBackend.
var _ storage.ScopedBackend = (*failingOnSafetyFlagsStore)(nil)

func (f *failingOnSafetyFlagsStore) StoreSafetyFlags(_ context.Context, _ string, _ []storage.SafetyFlag) error {
	return errors.New("injected safety-flags failure")
}

// rollbackTestScoper is a storage.Scoper that always returns the same backend.
type rollbackTestScoper struct {
	backend storage.ScopedBackend
}

func (s *rollbackTestScoper) Scoped(_ context.Context, _ string) (storage.ScopedBackend, error) {
	return s.backend, nil
}

func (s *rollbackTestScoper) Subscribe(_ storage.ChangeSubscriber) {}

// TestShape_AtomicRollback_ConversationNotPersistedOnSafetyFail verifies that
// when StoreSafetyFlags (the third op) fails inside the four-op Shape
// transaction, the entire transaction — TransitionStage, StoreShapeOutput, and
// RecordConversation — is rolled back. After the error:
//
//   - The spec stage must still be "spark" (TransitionStage rolled back).
//   - No conversation log must exist for the shape stage (RecordConversation
//     rolled back).
func TestShape_AtomicRollback_ConversationNotPersistedOnSafetyFail(t *testing.T) {
	ctx := context.Background()

	// Real store used for setup and post-rollback assertions.
	realStore := newStore(t)
	clearDatabase(t, realStore)

	// Create a failing wrapper — embeds the real store, overrides StoreSafetyFlags.
	failingStore := &failingOnSafetyFlagsStore{Store: realStore}

	// Construct the handler via RegisterAuthoringService with a scoper that
	// always returns the failing store.
	scoper := &rollbackTestScoper{backend: failingStore}
	mux := http.NewServeMux()
	server.RegisterAuthoringService(mux, scoper)
	srv := httptest.NewServer(injectTestProject(mux))
	t.Cleanup(srv.Close)

	client := specgraphv1connect.NewAuthoringServiceClient(http.DefaultClient, srv.URL)

	// Step 1: drive the spec to spark stage via Spark RPC.
	_, err := client.Spark(ctx, connect.NewRequest(&specv1.SparkRequest{
		Slug: "oauth-refresh",
		Output: &specv1.SparkOutput{
			Seed:   "oauth refresh token flow",
			Signal: "user request",
		},
		// No conversation_exchanges — Spark allows omission.
	}))
	// Spark may fail because StoreSafetyFlags is always broken in the wrapper.
	// If the seed triggers a safety flag (unlikely for plain text), Spark would
	// fail too. In practice "oauth refresh token flow" does NOT trigger patterns.
	// But to be safe: if Spark fails, the spec won't be created and the test
	// should report a more actionable message.
	if err != nil {
		t.Fatalf("Spark unexpectedly failed (check whether seed triggers safety patterns): %v", err)
	}

	// Step 2: call Shape with a risk that triggers a safety flag ("credential"),
	// guaranteeing StoreSafetyFlags is called. The failing wrapper then returns
	// an error, rolling back the entire transaction.
	_, err = client.Shape(ctx, connect.NewRequest(&specv1.ShapeRequest{
		Slug: "oauth-refresh",
		Output: &specv1.ShapeOutput{
			ScopeIn:        []string{"token refresh endpoint"},
			ScopeOut:       []string{"initial login"},
			Approaches:     []*specv1.Approach{{Name: "jwt", Description: "JWT-based refresh", Tradeoffs: []string{"expiry management"}}},
			ChosenApproach: "jwt",
			// "credential" matches a warning security pattern — ensures safety flags are generated.
			Risks:       []string{"credential rotation required"},
			SuccessMust: []string{"refresh token accepted"},
		},
		ConversationExchanges: []*specv1.ConversationExchange{
			{Role: "probe", Content: "what is the shape?", Stage: "shape", Sequence: 1},
		},
	}))
	if err == nil {
		t.Fatal("expected Shape to return an error due to injected StoreSafetyFlags failure")
	}

	// Assert the error is a ConnectRPC CodeInternal (injected storage failure maps there).
	var connErr *connect.Error
	require.True(t, errors.As(err, &connErr), "expected connect.Error")
	require.Equal(t, connect.CodeInternal, connErr.Code(),
		"injected safety-flags failure must map to CodeInternal")

	// Step 3: assert rollback — spec must still be at spark stage.
	spec, specErr := realStore.GetSpec(ctx, "oauth-refresh")
	require.NoError(t, specErr, "GetSpec after rollback")
	if spec.Stage != storage.SpecStageSpark {
		t.Errorf("expected spec stage %q after rollback, got %q", storage.SpecStageSpark, spec.Stage)
	}

	// Verify ShapeOutput was also rolled back — op 2 (StoreShapeOutput) was
	// inside the same transaction as op 3 (StoreSafetyFlags), which failed.
	require.Nil(t, spec.ShapeOutput, "ShapeOutput should be nil after rollback")

	// Step 4: assert rollback — no conversation log for the shape stage.
	logs, logsErr := realStore.ListConversations(ctx, "oauth-refresh", "shape")
	require.NoError(t, logsErr, "ListConversations after rollback")
	if len(logs) > 0 {
		t.Errorf("expected no conversation logs for shape stage after rollback, got %d", len(logs))
	}
}

// injectTestProject mirrors the test project middleware used in server unit
// tests: it adds the X-Specgraph-Project header (using a valid project slug)
// when absent, then passes through server.ProjectMiddleware.
func injectTestProject(h http.Handler) http.Handler {
	withProject := server.ProjectMiddleware(h)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Specgraph-Project") == "" {
			r.Header.Set("X-Specgraph-Project", "test")
		}
		withProject.ServeHTTP(w, r)
	})
}
