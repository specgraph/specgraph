// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

//go:build integration

package postgres_test

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/specgraph/specgraph/internal/storage"
	"github.com/stretchr/testify/require"
)

func TestCreateDecision_AllFields(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	dec, err := store.CreateDecision(ctx, "adr-001", "Use Postgres", "We will use Postgres", "Mature and reliable",
		"Where to store data?",
		[]storage.RejectedAlternative{{Option: "MySQL", Reason: "less ecosystem"}, {Option: "SQLite", Reason: "not distributed"}},
		storage.DecisionConfidenceHigh,
		[]string{"storage", "database"},
		storage.DecisionScopeProject,
		"backend-spec", "specify")
	require.NoError(t, err)
	require.NotNil(t, dec)
	require.Equal(t, "adr-001", dec.Slug)
	require.Equal(t, "Use Postgres", dec.Title)
	require.Equal(t, storage.DecisionStatusProposed, dec.Status)
	require.Equal(t, "We will use Postgres", dec.Body)
	require.Equal(t, "Mature and reliable", dec.Rationale)
	require.Equal(t, "Where to store data?", dec.Question)
	require.Len(t, dec.RejectedAlternatives, 2)
	require.Equal(t, "MySQL", dec.RejectedAlternatives[0].Option)
	require.Equal(t, "less ecosystem", dec.RejectedAlternatives[0].Reason)
	require.Equal(t, "SQLite", dec.RejectedAlternatives[1].Option)
	require.Equal(t, storage.DecisionConfidenceHigh, dec.Confidence)
	require.Equal(t, []string{"storage", "database"}, dec.Tags)
	require.Equal(t, storage.DecisionScopeProject, dec.Scope)
	require.Equal(t, "backend-spec", dec.OriginSpec)
	require.Equal(t, "specify", dec.OriginStage)
	require.Equal(t, 1, dec.Version)
	require.NotEmpty(t, dec.ContentHash)
	require.Len(t, dec.ContentHash, 32, "content_hash should be 32-char hex")
	require.False(t, dec.CreatedAt.IsZero())
	require.False(t, dec.UpdatedAt.IsZero())
	require.NotEmpty(t, dec.ID)
	require.True(t, strings.HasPrefix(dec.ID, "dec-"), "ID should have dec- prefix")
}

func TestCreateDecision_MinimalFields(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	dec, err := store.CreateDecision(ctx, "adr-minimal", "Minimal Decision", "Body", "Rationale",
		"", nil, "", nil, "", "", "")
	require.NoError(t, err)
	require.Equal(t, "adr-minimal", dec.Slug)
	require.Equal(t, storage.DecisionStatusProposed, dec.Status)
	require.Equal(t, 1, dec.Version)
	require.Empty(t, dec.Question)
	require.Empty(t, dec.RejectedAlternatives)
	require.Empty(t, string(dec.Confidence))
	require.Empty(t, dec.Tags)
	require.Empty(t, string(dec.Scope))
	require.Empty(t, dec.OriginSpec)
	require.Empty(t, dec.OriginStage)
}

func TestCreateDecision_DuplicateSlug(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateDecision(ctx, "dup-dec", "Title", "Body", "Rationale",
		"", nil, "", nil, "", "", "")
	require.NoError(t, err)

	_, err = store.CreateDecision(ctx, "dup-dec", "Title2", "Body2", "Rationale2",
		"", nil, "", nil, "", "", "")
	require.ErrorIs(t, err, storage.ErrDecisionAlreadyExists)
}

func TestGetDecision_NotFound(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.GetDecision(ctx, "does-not-exist")
	require.ErrorIs(t, err, storage.ErrDecisionNotFound)
}

func TestGetDecision_RoundTrip(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	created, err := store.CreateDecision(ctx, "adr-get", "Get Test", "Body", "Rationale",
		"", nil, "", nil, "", "", "")
	require.NoError(t, err)

	got, err := store.GetDecision(ctx, "adr-get")
	require.NoError(t, err)
	require.Equal(t, created.Slug, got.Slug)
	require.Equal(t, created.Title, got.Title)
	require.Equal(t, created.ContentHash, got.ContentHash)
	require.Equal(t, created.Version, got.Version)
}

func TestListDecisions_StatusFilter(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateDecision(ctx, "dec-a", "First", "Body A", "Rationale A",
		"", nil, "", nil, "", "", "")
	require.NoError(t, err)
	_, err = store.CreateDecision(ctx, "dec-b", "Second", "Body B", "Rationale B",
		"", nil, "", nil, "", "", "")
	require.NoError(t, err)

	// No filter — all returned.
	all, err := store.ListDecisions(ctx, "", 0)
	require.NoError(t, err)
	require.Len(t, all, 2)

	// Filter by proposed — both match since that's the initial status.
	proposed, err := store.ListDecisions(ctx, storage.DecisionStatusProposed, 0)
	require.NoError(t, err)
	require.Len(t, proposed, 2)

	// Filter by accepted — none match.
	accepted, err := store.ListDecisions(ctx, storage.DecisionStatusAccepted, 0)
	require.NoError(t, err)
	require.Len(t, accepted, 0)
}

func TestListDecisions_Limit(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	for i := range 5 {
		slug := "list-dec-" + string(rune('a'+i))
		_, err := store.CreateDecision(ctx, slug, "Title", "Body", "Rationale",
			"", nil, "", nil, "", "", "")
		require.NoError(t, err)
	}

	limited, err := store.ListDecisions(ctx, "", 3)
	require.NoError(t, err)
	require.Len(t, limited, 3)
}

func TestUpdateDecision_BasicFields(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateDecision(ctx, "upd-dec", "Original Title", "Original body", "Original rationale",
		"", nil, "", nil, "", "", "")
	require.NoError(t, err)

	newStatus := storage.DecisionStatusAccepted
	updated, err := store.UpdateDecision(ctx, "upd-dec", 0, nil, &newStatus, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil)
	require.NoError(t, err)
	require.Equal(t, storage.DecisionStatusAccepted, updated.Status)
	require.Equal(t, "Original Title", updated.Title)
	require.Equal(t, 2, updated.Version)
}

func TestUpdateDecision_VersionGuard(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	dec, err := store.CreateDecision(ctx, "ver-guard-dec", "Title", "Body", "Rationale",
		"", nil, "", nil, "", "", "")
	require.NoError(t, err)
	require.Equal(t, 1, dec.Version)

	// Correct version succeeds.
	newTitle := "Updated"
	updated, err := store.UpdateDecision(ctx, "ver-guard-dec", 1, &newTitle, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil)
	require.NoError(t, err)
	require.Equal(t, "Updated", updated.Title)
	require.Equal(t, 2, updated.Version)

	// Stale version fails with ErrConcurrentModification.
	staleTitle := "Stale"
	_, err = store.UpdateDecision(ctx, "ver-guard-dec", 1, &staleTitle, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil)
	require.ErrorIs(t, err, storage.ErrConcurrentModification)

	// Version=0 skips check and succeeds.
	skipTitle := "NoCheck"
	noCheck, err := store.UpdateDecision(ctx, "ver-guard-dec", 0, &skipTitle, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil)
	require.NoError(t, err)
	require.Equal(t, "NoCheck", noCheck.Title)
	require.Equal(t, 3, noCheck.Version)
}

func TestUpdateDecision_ConditionalChangelog(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	dec, err := store.CreateDecision(ctx, "changelog-dec", "Title", "Body", "Rationale",
		"", nil, "", nil, "", "", "")
	require.NoError(t, err)
	origHash := dec.ContentHash

	// Update with a new title — hash changes, changelog entry created.
	newTitle := "Updated Title"
	updated, err := store.UpdateDecision(ctx, "changelog-dec", 0, &newTitle, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil)
	require.NoError(t, err)
	require.NotEqual(t, origHash, updated.ContentHash, "content hash should change when title changes")

	changes, err := store.ListChanges(ctx, "changelog-dec", storage.ChangeLogFilter{})
	require.NoError(t, err)
	// 1 from create + 1 from update.
	require.Len(t, changes, 2)
	require.Equal(t, "decision updated", changes[1].Summary)
}

func TestUpdateDecision_NoChangeSkipsChangelog(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateDecision(ctx, "noop-dec", "Same Title", "Same Body", "Same Rationale",
		"", nil, "", nil, "", "", "")
	require.NoError(t, err)

	// Update with the same title value — hash unchanged, no new changelog entry.
	sameTitle := "Same Title"
	updated, err := store.UpdateDecision(ctx, "noop-dec", 0, &sameTitle, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil)
	require.NoError(t, err)
	// Version still bumps.
	require.Equal(t, 2, updated.Version)

	changes, err := store.ListChanges(ctx, "noop-dec", storage.ChangeLogFilter{})
	require.NoError(t, err)
	// Only the initial create entry.
	require.Len(t, changes, 1)
}

func TestUpdateDecision_TagsAndRejectedAlts(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	dec, err := store.CreateDecision(ctx, "arr-dec", "Title", "Body", "Rationale",
		"", nil, "", nil, "", "", "")
	require.NoError(t, err)
	require.Empty(t, dec.Tags)
	require.Empty(t, dec.RejectedAlternatives)

	newTags := []string{"auth", "storage"}
	newAlts := []storage.RejectedAlternative{
		{Option: "Redis", Reason: "ops overhead"},
		{Option: "S3", Reason: "latency"},
	}
	updated, err := store.UpdateDecision(ctx, "arr-dec", 0,
		nil, nil, nil, nil, nil, nil,
		&newAlts, nil, &newTags, nil, nil, nil)
	require.NoError(t, err)
	require.Equal(t, []string{"auth", "storage"}, updated.Tags)
	require.Len(t, updated.RejectedAlternatives, 2)
	require.Equal(t, "Redis", updated.RejectedAlternatives[0].Option)
	require.Equal(t, "ops overhead", updated.RejectedAlternatives[0].Reason)
	require.Equal(t, "S3", updated.RejectedAlternatives[1].Option)

	// Round-trip via GetDecision.
	got, err := store.GetDecision(ctx, "arr-dec")
	require.NoError(t, err)
	require.Equal(t, []string{"auth", "storage"}, got.Tags)
	require.Len(t, got.RejectedAlternatives, 2)
}

func TestUpdateDecision_NotFound(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	newTitle := "x"
	_, err := store.UpdateDecision(ctx, "does-not-exist", 0, &newTitle, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil)
	require.ErrorIs(t, err, storage.ErrDecisionNotFound)
}

func TestUpdateDecision_SupersededWithoutSupersededBy_ReturnsError(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateDecision(ctx, "sup-dec", "Title", "Body", "Rationale",
		"", nil, "", nil, "", "", "")
	require.NoError(t, err)

	supersededStatus := storage.DecisionStatusSuperseded
	// Setting status=superseded without supersededBy must fail.
	_, err = store.UpdateDecision(ctx, "sup-dec", 0, nil, &supersededStatus, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil)
	require.ErrorIs(t, err, storage.ErrSupersededByRequired)
}

func TestUpdateDecision_SupersededWithSupersededBy_Succeeds(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateDecision(ctx, "sup-by-dec", "Title", "Body", "Rationale",
		"", nil, "", nil, "", "", "")
	require.NoError(t, err)

	supersededStatus := storage.DecisionStatusSuperseded
	supersededBy := "adr-new"
	updated, err := store.UpdateDecision(ctx, "sup-by-dec", 0, nil, &supersededStatus, nil, nil, &supersededBy,
		nil, nil, nil, nil, nil, nil, nil)
	require.NoError(t, err)
	require.Equal(t, storage.DecisionStatusSuperseded, updated.Status)
	require.Equal(t, "adr-new", updated.SupersededBy)
}

func TestUpdateDecision_NoFields_ReturnsCurrentDecision(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateDecision(ctx, "noop-dec2", "Title", "Body", "Rationale",
		"", nil, "", nil, "", "", "")
	require.NoError(t, err)

	// All nil fields — must return current decision unchanged.
	got, err := store.UpdateDecision(ctx, "noop-dec2", 0, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil)
	require.NoError(t, err)
	require.Equal(t, "noop-dec2", got.Slug)
	require.Equal(t, 1, got.Version)
}

func TestUpdateDecision_BodyRationaleQuestionOrigin(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateDecision(ctx, "full-dec", "Title", "Old body", "Old rationale",
		"Old question", nil, "", nil, "", "old-spec", "spark")
	require.NoError(t, err)

	newBody := "New body"
	newRationale := "New rationale"
	newQuestion := "New question?"
	newOriginSpec := "new-spec"
	newOriginStage := "specify"
	updated, err := store.UpdateDecision(ctx, "full-dec", 0, nil, nil,
		&newBody, &newRationale, nil, &newQuestion, nil, nil, nil, nil, &newOriginSpec, &newOriginStage)
	require.NoError(t, err)
	require.Equal(t, "New body", updated.Body)
	require.Equal(t, "New rationale", updated.Rationale)
	require.Equal(t, "New question?", updated.Question)
	require.Equal(t, "new-spec", updated.OriginSpec)
	require.Equal(t, "specify", updated.OriginStage)
}

func TestUpdateDecision_ContentHashChanges(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	dec, err := store.CreateDecision(ctx, "hash-dec", "Original", "Body", "Rationale",
		"", nil, "", nil, "", "", "")
	require.NoError(t, err)
	origHash := dec.ContentHash

	newTitle := "Updated Title"
	updated, err := store.UpdateDecision(ctx, "hash-dec", 0, &newTitle, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil)
	require.NoError(t, err)
	require.NotEqual(t, origHash, updated.ContentHash)
	require.Len(t, updated.ContentHash, 32)

	// Verify hash persisted.
	fetched, err := store.GetDecision(ctx, "hash-dec")
	require.NoError(t, err)
	require.Equal(t, updated.ContentHash, fetched.ContentHash)
}

// ---- Legacy status normalization tests ----
// These tests exercise normalizeDecisionStatus and legacyDecisionStatus by
// inserting rows with legacy proto-style status strings directly and verifying
// that ListDecisions + GetDecision normalise them correctly.

func TestListDecisions_LegacyProtoStatus_Proposed(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	// Create a decision and then overwrite its status with a legacy proto value.
	_, err := store.CreateDecision(ctx, "legacy-proposed", "Title", "Body", "Rationale",
		"", nil, "", nil, "", "", "")
	require.NoError(t, err)

	pool, poolErr := pgxpool.New(ctx, connString)
	require.NoError(t, poolErr)
	defer pool.Close()

	_, err = pool.Exec(ctx,
		`UPDATE decisions SET status = 'DECISION_STATUS_PROPOSED' WHERE slug = 'legacy-proposed'`)
	require.NoError(t, err)

	// ListDecisions with proposed filter must find this row.
	results, err := store.ListDecisions(ctx, storage.DecisionStatusProposed, 0)
	require.NoError(t, err)
	var found bool
	for _, d := range results {
		if d.Slug == "legacy-proposed" {
			found = true
			require.Equal(t, storage.DecisionStatusProposed, d.Status, "legacy DECISION_STATUS_PROPOSED should normalize to proposed")
		}
	}
	require.True(t, found, "expected legacy-proposed in results")
}

func TestListDecisions_LegacyProtoStatus_Accepted(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateDecision(ctx, "legacy-accepted", "Title", "Body", "Rationale",
		"", nil, "", nil, "", "", "")
	require.NoError(t, err)

	pool, poolErr := pgxpool.New(ctx, connString)
	require.NoError(t, poolErr)
	defer pool.Close()

	_, err = pool.Exec(ctx,
		`UPDATE decisions SET status = 'DECISION_STATUS_ACCEPTED' WHERE slug = 'legacy-accepted'`)
	require.NoError(t, err)

	// Filter by accepted must find it.
	results, err := store.ListDecisions(ctx, storage.DecisionStatusAccepted, 0)
	require.NoError(t, err)
	var found bool
	for _, d := range results {
		if d.Slug == "legacy-accepted" {
			found = true
			require.Equal(t, storage.DecisionStatusAccepted, d.Status)
		}
	}
	require.True(t, found, "expected legacy-accepted in results")
}

func TestListDecisions_LegacyProtoStatus_Superseded(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateDecision(ctx, "legacy-superseded", "Title", "Body", "Rationale",
		"", nil, "", nil, "", "", "")
	require.NoError(t, err)

	pool, poolErr := pgxpool.New(ctx, connString)
	require.NoError(t, poolErr)
	defer pool.Close()

	_, err = pool.Exec(ctx,
		`UPDATE decisions SET status = 'DECISION_STATUS_SUPERSEDED' WHERE slug = 'legacy-superseded'`)
	require.NoError(t, err)

	results, err := store.ListDecisions(ctx, storage.DecisionStatusSuperseded, 0)
	require.NoError(t, err)
	var found bool
	for _, d := range results {
		if d.Slug == "legacy-superseded" {
			found = true
			require.Equal(t, storage.DecisionStatusSuperseded, d.Status)
		}
	}
	require.True(t, found)
}

func TestListDecisions_LegacyProtoStatus_Deprecated(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateDecision(ctx, "legacy-deprecated", "Title", "Body", "Rationale",
		"", nil, "", nil, "", "", "")
	require.NoError(t, err)

	pool, poolErr := pgxpool.New(ctx, connString)
	require.NoError(t, poolErr)
	defer pool.Close()

	_, err = pool.Exec(ctx,
		`UPDATE decisions SET status = 'DECISION_STATUS_DEPRECATED' WHERE slug = 'legacy-deprecated'`)
	require.NoError(t, err)

	results, err := store.ListDecisions(ctx, storage.DecisionStatusDeprecated, 0)
	require.NoError(t, err)
	var found bool
	for _, d := range results {
		if d.Slug == "legacy-deprecated" {
			found = true
			require.Equal(t, storage.DecisionStatusDeprecated, d.Status)
		}
	}
	require.True(t, found)
}

func TestListDecisions_LegacyUnspecified_TreatedAsProposed(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateDecision(ctx, "legacy-unspecified", "Title", "Body", "Rationale",
		"", nil, "", nil, "", "", "")
	require.NoError(t, err)

	pool, poolErr := pgxpool.New(ctx, connString)
	require.NoError(t, poolErr)
	defer pool.Close()

	_, err = pool.Exec(ctx,
		`UPDATE decisions SET status = 'DECISION_STATUS_UNSPECIFIED' WHERE slug = 'legacy-unspecified'`)
	require.NoError(t, err)

	// UNSPECIFIED should appear under the proposed filter.
	results, err := store.ListDecisions(ctx, storage.DecisionStatusProposed, 0)
	require.NoError(t, err)
	var found bool
	for _, d := range results {
		if d.Slug == "legacy-unspecified" {
			found = true
			require.Equal(t, storage.DecisionStatusProposed, d.Status)
		}
	}
	require.True(t, found)
}
