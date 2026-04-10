# Inbound Webhooks Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build an inbound webhook system where GitHub and ADO can push events that update spec/slice state in SpecGraph.

**Architecture:** Raw HTTP handlers on the existing `http.ServeMux` (not ConnectRPC — external systems dictate wire format). Each provider handler verifies signatures and normalizes into a common `WebhookEvent`, which feeds a single `Processor` chain that maps events to state transitions. PostgreSQL stores event logs for dedup and audit.

**Tech Stack:** Go stdlib `net/http`, `crypto/hmac` (GitHub HMAC-SHA256), pgx v5 (storage), testcontainers (integration tests), Ginkgo/Gomega (e2e tests).

---

## File Structure

| Path | Responsibility |
|------|---------------|
| `internal/webhook/provider.go` | `WebhookProvider` interface, `WebhookEvent` type, sentinel errors, `ContentHash` computation |
| `internal/webhook/github.go` | GitHub provider: HMAC-SHA256 verification, PR/issue event normalization |
| `internal/webhook/github_test.go` | Unit tests for GitHub signature verification and normalization |
| `internal/webhook/ado.go` | ADO provider: shared-secret verification, service hook normalization |
| `internal/webhook/ado_test.go` | Unit tests for ADO verification and normalization |
| `internal/webhook/processor.go` | `Processor`: dedup check, event-to-action mapping, state transitions |
| `internal/webhook/processor_test.go` | Unit tests for Processor with mock storage |
| `internal/server/webhook_handler.go` | HTTP handler: payload size limit, provider dispatch, rate limiting |
| `internal/server/webhook_handler_test.go` | Handler tests with httptest |
| `internal/storage/webhook.go` | `WebhookBackend` interface, `WebhookEventRecord` domain type, sentinel errors |
| `internal/storage/postgres/webhook.go` | PostgreSQL implementation of `WebhookBackend` |
| `internal/storage/postgres/webhook_test.go` | Integration tests (shared container) |
| `internal/storage/postgres/migrations/004_webhook_events.sql` | Migration: `webhook_events` table with dedup indexes |
| `internal/config/global.go` | Add `WebhookConfig` to `ServerSection` (modify) |
| `internal/config/global_test.go` | Config validation tests (modify) |
| `internal/storage/scoper.go` | Add `WebhookBackend` to `ScopedBackend` (modify) |
| `cmd/specgraph/serve.go` | Register webhook handlers on mux (modify) |
| `e2e/api/webhook_test.go` | E2E tests: database verification after webhook processing |
| `e2e/testutil/server.go` | Register webhook handlers in test server (modify) |

---

## Task 1: Storage Layer — Domain Types, Interface, Migration

**Files:**
- Create: `internal/storage/webhook.go`
- Create: `internal/storage/postgres/migrations/004_webhook_events.sql`
- Modify: `internal/storage/scoper.go:10` (add `WebhookBackend` to `ScopedBackend`)
- Modify: `internal/storage/postgres/postgres.go:116-126` (add `webhook_events` to `ClearAll`)

### Steps

- [ ] **Step 1: Create the WebhookBackend interface and domain types**

```go
// internal/storage/webhook.go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package storage

import (
	"context"
	"errors"
	"time"
)

// WebhookEventRecord represents a persisted webhook event for audit and dedup.
type WebhookEventRecord struct {
	ID          string
	Provider    string
	DeliveryID  string
	ContentHash string
	EventType   string
	ExternalID  string
	ExternalURL string
	Repo        string
	Status      string // "processed", "duplicate", "no_match", "unsupported"
	Action      string // "slice_completed", "spec_updated", "no_match", "duplicate"
	SpecSlug    string
	SliceSlug   string
	RawPayload  []byte
	CreatedAt   time.Time
}

// WebhookBackend provides persistence for webhook event processing.
type WebhookBackend interface {
	// StoreWebhookEvent persists a webhook event record.
	StoreWebhookEvent(ctx context.Context, record *WebhookEventRecord) error

	// CheckDeliveryID returns true if a webhook event with this provider+deliveryID already exists.
	CheckDeliveryID(ctx context.Context, provider, deliveryID string) (bool, error)

	// CheckContentHash returns true if a webhook event with this content hash already exists.
	CheckContentHash(ctx context.Context, contentHash string) (bool, error)

	// ListWebhookEvents returns recent webhook events, newest first.
	ListWebhookEvents(ctx context.Context, limit int) ([]*WebhookEventRecord, error)
}

// Sentinel errors for webhook operations.
var (
	ErrDuplicateDelivery = errors.New("duplicate delivery")
	ErrDuplicateContent  = errors.New("duplicate content hash")
)
```

- [ ] **Step 2: Add WebhookBackend to ScopedBackend**

In `internal/storage/scoper.go`, add `WebhookBackend` to the `ScopedBackend` interface:

```go
type ScopedBackend interface {
	Backend
	GraphBackend
	DecisionBackend
	ClaimBackend
	ConstitutionBackend
	AuthoringBackend
	FindingsBackend
	ExecutionBackend
	LifecycleBackend
	SyncBackend
	ProjectBackend
	SliceBackend
	ConversationBackend
	ChangeLogBackend
	TransactionalBackend
	WebhookBackend
}
```

- [ ] **Step 3: Create the migration**

```sql
-- internal/storage/postgres/migrations/004_webhook_events.sql
-- SPDX-License-Identifier: MIT
-- Copyright 2026 Sean Brandt

-- +goose Up

CREATE TABLE webhook_events (
    id            TEXT NOT NULL,
    project_slug  TEXT NOT NULL REFERENCES projects(slug),
    provider      TEXT NOT NULL,
    delivery_id   TEXT NOT NULL DEFAULT '',
    content_hash  TEXT NOT NULL DEFAULT '',
    event_type    TEXT NOT NULL DEFAULT '',
    external_id   TEXT NOT NULL DEFAULT '',
    external_url  TEXT NOT NULL DEFAULT '',
    repo          TEXT NOT NULL DEFAULT '',
    status        TEXT NOT NULL DEFAULT 'processed',
    action        TEXT NOT NULL DEFAULT '',
    spec_slug     TEXT NOT NULL DEFAULT '',
    slice_slug    TEXT NOT NULL DEFAULT '',
    raw_payload   JSONB,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (project_slug, id)
);

-- Delivery ID dedup: unique per provider within a project.
CREATE UNIQUE INDEX idx_webhook_events_delivery
    ON webhook_events (project_slug, provider, delivery_id)
    WHERE delivery_id != '';

-- Content hash dedup: unique per project.
CREATE UNIQUE INDEX idx_webhook_events_content_hash
    ON webhook_events (project_slug, content_hash)
    WHERE content_hash != '';

-- Listing: newest first.
CREATE INDEX idx_webhook_events_created
    ON webhook_events (project_slug, created_at DESC);

-- +goose Down

DROP TABLE IF EXISTS webhook_events;
```

- [ ] **Step 4: Add webhook_events to ClearAll in postgres.go**

In `internal/storage/postgres/postgres.go`, update the `ClearAll` TRUNCATE list:

```go
func (s *Store) ClearAll(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `TRUNCATE
		webhook_events, sync_mappings, constitutions, execution_events, claims,
		conversation_logs, findings, changelog_entries, edges,
		slices, decisions, specs, projects
		CASCADE`)
```

- [ ] **Step 5: Verify the migration compiles**

Run: `task build`
Expected: Build succeeds (compilation will fail because `WebhookBackend` methods aren't implemented on `Store` yet — that's expected, we'll stub them in Task 2).

- [ ] **Step 6: Commit**

```bash
git add internal/storage/webhook.go internal/storage/scoper.go \
  internal/storage/postgres/migrations/004_webhook_events.sql \
  internal/storage/postgres/postgres.go
git commit -m "feat(webhook): add WebhookBackend interface, domain types, and migration"
```

---

## Task 2: PostgreSQL Implementation of WebhookBackend

**Files:**
- Create: `internal/storage/postgres/webhook.go`
- Create: `internal/storage/postgres/webhook_test.go`

### Steps

- [ ] **Step 1: Write the failing integration test for StoreWebhookEvent**

```go
// internal/storage/postgres/webhook_test.go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

//go:build integration

package postgres_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/specgraph/specgraph/internal/storage"
	"github.com/stretchr/testify/require"
)

func TestStoreWebhookEvent_Basic(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	record := &storage.WebhookEventRecord{
		ID:          "wh-test-001",
		Provider:    "github",
		DeliveryID:  "gh-delivery-abc",
		ContentHash: "abc123def456",
		EventType:   "pr_merged",
		ExternalID:  "12345",
		ExternalURL: "https://github.com/org/repo/pull/42",
		Repo:        "org/repo",
		Status:      "processed",
		Action:      "slice_completed",
		SpecSlug:    "my-spec",
		SliceSlug:   "my-spec/slice-1",
		RawPayload:  []byte(`{"action":"closed","merged":true}`),
	}
	err := store.StoreWebhookEvent(ctx, record)
	require.NoError(t, err)

	events, err := store.ListWebhookEvents(ctx, 10)
	require.NoError(t, err)
	require.Len(t, events, 1)
	require.Equal(t, "github", events[0].Provider)
	require.Equal(t, "gh-delivery-abc", events[0].DeliveryID)
	require.Equal(t, "abc123def456", events[0].ContentHash)
	require.Equal(t, "pr_merged", events[0].EventType)
	require.Equal(t, "slice_completed", events[0].Action)
	require.Equal(t, "my-spec", events[0].SpecSlug)
	require.False(t, events[0].CreatedAt.IsZero())
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -tags integration -run TestStoreWebhookEvent_Basic -v ./internal/storage/postgres/`
Expected: FAIL — `StoreWebhookEvent` method not found on `Store`.

- [ ] **Step 3: Implement WebhookBackend on postgres.Store**

```go
// internal/storage/postgres/webhook.go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package postgres

import (
	"context"
	"fmt"

	"github.com/specgraph/specgraph/internal/storage"
)

// StoreWebhookEvent persists a webhook event record.
func (s *Store) StoreWebhookEvent(ctx context.Context, record *storage.WebhookEventRecord) error {
	_, err := s.exec(ctx, `
		INSERT INTO webhook_events (
			id, project_slug, provider, delivery_id, content_hash,
			event_type, external_id, external_url, repo,
			status, action, spec_slug, slice_slug, raw_payload
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`,
		record.ID, s.project, record.Provider, record.DeliveryID, record.ContentHash,
		record.EventType, record.ExternalID, record.ExternalURL, record.Repo,
		record.Status, record.Action, record.SpecSlug, record.SliceSlug, record.RawPayload,
	)
	if err != nil {
		return fmt.Errorf("store webhook event: %w", err)
	}
	return nil
}

// CheckDeliveryID returns true if a webhook event with this provider+deliveryID already exists.
func (s *Store) CheckDeliveryID(ctx context.Context, provider, deliveryID string) (bool, error) {
	if deliveryID == "" {
		return false, nil
	}
	var exists bool
	err := s.queryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM webhook_events
			WHERE project_slug = $1 AND provider = $2 AND delivery_id = $3
		)`, s.project, provider, deliveryID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check delivery id: %w", err)
	}
	return exists, nil
}

// CheckContentHash returns true if a webhook event with this content hash already exists.
func (s *Store) CheckContentHash(ctx context.Context, contentHash string) (bool, error) {
	if contentHash == "" {
		return false, nil
	}
	var exists bool
	err := s.queryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM webhook_events
			WHERE project_slug = $1 AND content_hash = $2
		)`, s.project, contentHash).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check content hash: %w", err)
	}
	return exists, nil
}

// ListWebhookEvents returns recent webhook events, newest first.
func (s *Store) ListWebhookEvents(ctx context.Context, limit int) ([]*storage.WebhookEventRecord, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.query(ctx, `
		SELECT id, provider, delivery_id, content_hash, event_type,
		       external_id, external_url, repo, status, action,
		       spec_slug, slice_slug, raw_payload, created_at
		FROM webhook_events
		WHERE project_slug = $1
		ORDER BY created_at DESC
		LIMIT $2`, s.project, limit)
	if err != nil {
		return nil, fmt.Errorf("list webhook events: %w", err)
	}
	defer rows.Close()

	var events []*storage.WebhookEventRecord
	for rows.Next() {
		e := &storage.WebhookEventRecord{}
		if err := rows.Scan(
			&e.ID, &e.Provider, &e.DeliveryID, &e.ContentHash, &e.EventType,
			&e.ExternalID, &e.ExternalURL, &e.Repo, &e.Status, &e.Action,
			&e.SpecSlug, &e.SliceSlug, &e.RawPayload, &e.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan webhook event: %w", err)
		}
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate webhook events: %w", err)
	}
	return events, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -tags integration -run TestStoreWebhookEvent_Basic -v ./internal/storage/postgres/`
Expected: PASS

- [ ] **Step 5: Write dedup tests**

Add to `internal/storage/postgres/webhook_test.go`:

```go
func TestCheckDeliveryID_FindsDuplicate(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	record := &storage.WebhookEventRecord{
		ID:         "wh-dup-001",
		Provider:   "github",
		DeliveryID: "gh-delivery-dup",
		Status:     "processed",
	}
	require.NoError(t, store.StoreWebhookEvent(ctx, record))

	exists, err := store.CheckDeliveryID(ctx, "github", "gh-delivery-dup")
	require.NoError(t, err)
	require.True(t, exists)

	exists, err = store.CheckDeliveryID(ctx, "github", "gh-delivery-other")
	require.NoError(t, err)
	require.False(t, exists)
}

func TestCheckDeliveryID_EmptyIDReturnsFalse(t *testing.T) {
	store := newStore(t)
	ctx := context.Background()

	exists, err := store.CheckDeliveryID(ctx, "github", "")
	require.NoError(t, err)
	require.False(t, exists)
}

func TestCheckContentHash_FindsDuplicate(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	record := &storage.WebhookEventRecord{
		ID:          "wh-hash-001",
		Provider:    "github",
		ContentHash: "hash-dedup-test",
		Status:      "processed",
	}
	require.NoError(t, store.StoreWebhookEvent(ctx, record))

	exists, err := store.CheckContentHash(ctx, "hash-dedup-test")
	require.NoError(t, err)
	require.True(t, exists)

	exists, err = store.CheckContentHash(ctx, "different-hash")
	require.NoError(t, err)
	require.False(t, exists)
}

func TestListWebhookEvents_OrdersNewestFirst(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	for i, id := range []string{"wh-list-001", "wh-list-002", "wh-list-003"} {
		record := &storage.WebhookEventRecord{
			ID:        id,
			Provider:  "github",
			EventType: fmt.Sprintf("event-%d", i),
			Status:    "processed",
		}
		require.NoError(t, store.StoreWebhookEvent(ctx, record))
	}

	events, err := store.ListWebhookEvents(ctx, 10)
	require.NoError(t, err)
	require.Len(t, events, 3)
	// Newest first — last inserted should be first.
	require.Equal(t, "wh-list-003", events[0].ID)
}

func TestListWebhookEvents_RespectsLimit(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	for _, id := range []string{"wh-lim-001", "wh-lim-002", "wh-lim-003"} {
		record := &storage.WebhookEventRecord{
			ID:       id,
			Provider: "github",
			Status:   "processed",
		}
		require.NoError(t, store.StoreWebhookEvent(ctx, record))
	}

	events, err := store.ListWebhookEvents(ctx, 2)
	require.NoError(t, err)
	require.Len(t, events, 2)
}
```

- [ ] **Step 6: Run all webhook storage tests**

Run: `go test -tags integration -run "TestStoreWebhookEvent|TestCheckDelivery|TestCheckContentHash|TestListWebhookEvents" -v ./internal/storage/postgres/`
Expected: All PASS

- [ ] **Step 7: Update clearDatabase to include webhook_events**

In `internal/storage/postgres/postgres_test.go`, add `"webhook_events"` to the `tables` slice in `clearDatabase`:

```go
tables := []string{
	"webhook_events",
	"sync_mappings",
	"execution_events",
	// ... rest unchanged
}
```

- [ ] **Step 8: Commit**

```bash
git add internal/storage/postgres/webhook.go internal/storage/postgres/webhook_test.go \
  internal/storage/postgres/postgres_test.go
git commit -m "feat(webhook): implement PostgreSQL WebhookBackend with dedup"
```

---

## Task 3: WebhookProvider Interface and GitHub Provider

**Files:**
- Create: `internal/webhook/provider.go`
- Create: `internal/webhook/github.go`
- Create: `internal/webhook/github_test.go`

### Steps

- [ ] **Step 1: Create the provider interface and common types**

```go
// internal/webhook/provider.go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

// Package webhook implements inbound webhook handling for external systems.
package webhook

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/oklog/ulid/v2"
)

// WebhookProvider normalizes provider-specific webhook payloads into a common event.
type WebhookProvider interface {
	// Name returns the provider identifier (e.g., "github", "ado").
	Name() string
	// ValidateRequest verifies the request signature using the provider secret.
	ValidateRequest(secret string, header http.Header, body []byte) error
	// NormalizeEvent extracts a common WebhookEvent from the provider-specific payload.
	NormalizeEvent(header http.Header, body []byte) (*WebhookEvent, error)
}

// WebhookEvent is the provider-agnostic representation of an inbound webhook.
type WebhookEvent struct {
	Provider    string          // "github", "ado"
	DeliveryID  string          // idempotency key from provider
	ContentHash string          // hash of Provider+EventType+ExternalID+Repo
	EventType   string          // normalized: "pr_merged", "pr_closed", etc.
	ExternalID  string          // provider-specific resource ID
	ExternalURL string          // link back to the resource
	Repo        string          // owner/repo or project/repo
	RawPayload  json.RawMessage // original webhook payload
}

// ComputeContentHash produces a SHA-256 hex digest from the semantic fields.
func ComputeContentHash(provider, eventType, externalID, repo string) string {
	h := sha256.New()
	fmt.Fprintf(h, "%s\n%s\n%s\n%s", provider, eventType, externalID, repo)
	return fmt.Sprintf("%x", h.Sum(nil))
}

// mustULID generates a new ULID string.
func mustULID() string {
	return ulid.MustNew(ulid.Now(), rand.Reader).String()
}

// Sentinel errors.
var (
	ErrInvalidSignature = errors.New("invalid signature")
	ErrUnsupportedEvent = errors.New("unsupported event type")
	ErrMissingHeader    = errors.New("missing required header")
	ErrPayloadTooLarge  = errors.New("payload exceeds max size")
)
```

- [ ] **Step 2: Write failing test for GitHub signature verification**

```go
// internal/webhook/github_test.go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func signPayload(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func TestGitHub_ValidateRequest_ValidSignature(t *testing.T) {
	gh := &GitHubProvider{}
	secret := "test-secret"
	body := []byte(`{"action":"closed"}`)
	sig := signPayload(secret, body)

	header := http.Header{}
	header.Set("X-Hub-Signature-256", sig)
	header.Set("X-GitHub-Event", "pull_request")
	header.Set("X-GitHub-Delivery", "delivery-123")

	err := gh.ValidateRequest(secret, header, body)
	require.NoError(t, err)
}

func TestGitHub_ValidateRequest_InvalidSignature(t *testing.T) {
	gh := &GitHubProvider{}
	header := http.Header{}
	header.Set("X-Hub-Signature-256", "sha256=invalid")
	header.Set("X-GitHub-Event", "pull_request")
	header.Set("X-GitHub-Delivery", "delivery-123")

	err := gh.ValidateRequest("test-secret", header, []byte(`{"action":"closed"}`))
	require.ErrorIs(t, err, ErrInvalidSignature)
}

func TestGitHub_ValidateRequest_MissingSignatureHeader(t *testing.T) {
	gh := &GitHubProvider{}
	header := http.Header{}
	header.Set("X-GitHub-Event", "pull_request")

	err := gh.ValidateRequest("test-secret", header, []byte(`{}`))
	require.ErrorIs(t, err, ErrMissingHeader)
}

func TestGitHub_NormalizeEvent_PRMerged(t *testing.T) {
	gh := &GitHubProvider{}
	body := []byte(`{
		"action": "closed",
		"pull_request": {
			"merged": true,
			"number": 42,
			"html_url": "https://github.com/org/repo/pull/42"
		},
		"repository": {
			"full_name": "org/repo"
		}
	}`)
	header := http.Header{}
	header.Set("X-GitHub-Event", "pull_request")
	header.Set("X-GitHub-Delivery", "delivery-456")

	event, err := gh.NormalizeEvent(header, body)
	require.NoError(t, err)
	require.Equal(t, "github", event.Provider)
	require.Equal(t, "delivery-456", event.DeliveryID)
	require.Equal(t, "pr_merged", event.EventType)
	require.Equal(t, "42", event.ExternalID)
	require.Equal(t, "https://github.com/org/repo/pull/42", event.ExternalURL)
	require.Equal(t, "org/repo", event.Repo)
	require.NotEmpty(t, event.ContentHash)
	require.NotEmpty(t, event.RawPayload)
}

func TestGitHub_NormalizeEvent_PRClosed(t *testing.T) {
	gh := &GitHubProvider{}
	body := []byte(`{
		"action": "closed",
		"pull_request": {
			"merged": false,
			"number": 43,
			"html_url": "https://github.com/org/repo/pull/43"
		},
		"repository": {"full_name": "org/repo"}
	}`)
	header := http.Header{}
	header.Set("X-GitHub-Event", "pull_request")
	header.Set("X-GitHub-Delivery", "delivery-789")

	event, err := gh.NormalizeEvent(header, body)
	require.NoError(t, err)
	require.Equal(t, "pr_closed", event.EventType)
}

func TestGitHub_NormalizeEvent_UnsupportedEvent(t *testing.T) {
	gh := &GitHubProvider{}
	body := []byte(`{"action":"created"}`)
	header := http.Header{}
	header.Set("X-GitHub-Event", "star")
	header.Set("X-GitHub-Delivery", "delivery-star")

	_, err := gh.NormalizeEvent(header, body)
	require.ErrorIs(t, err, ErrUnsupportedEvent)
}

func TestGitHub_NormalizeEvent_UnknownFieldsSucceed(t *testing.T) {
	gh := &GitHubProvider{}
	body := []byte(`{
		"action": "closed",
		"pull_request": {
			"merged": true,
			"number": 44,
			"html_url": "https://github.com/org/repo/pull/44",
			"unknown_field": "should not break"
		},
		"repository": {"full_name": "org/repo"},
		"extra": {"nested": true}
	}`)
	header := http.Header{}
	header.Set("X-GitHub-Event", "pull_request")
	header.Set("X-GitHub-Delivery", "delivery-unknown")

	event, err := gh.NormalizeEvent(header, body)
	require.NoError(t, err)
	require.Equal(t, "pr_merged", event.EventType)
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test -run TestGitHub -v ./internal/webhook/`
Expected: FAIL — `GitHubProvider` not defined.

- [ ] **Step 4: Implement GitHubProvider**

```go
// internal/webhook/github.go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// GitHubProvider handles GitHub webhook signature verification and event normalization.
type GitHubProvider struct{}

// Name returns "github".
func (g *GitHubProvider) Name() string { return "github" }

// ValidateRequest verifies the X-Hub-Signature-256 HMAC.
func (g *GitHubProvider) ValidateRequest(secret string, header http.Header, body []byte) error {
	sig := header.Get("X-Hub-Signature-256")
	if sig == "" {
		return fmt.Errorf("X-Hub-Signature-256: %w", ErrMissingHeader)
	}

	prefix := "sha256="
	if !strings.HasPrefix(sig, prefix) {
		return fmt.Errorf("malformed signature: %w", ErrInvalidSignature)
	}

	expected := hmac.New(sha256.New, []byte(secret))
	expected.Write(body)
	expectedHex := hex.EncodeToString(expected.Sum(nil))

	actual := sig[len(prefix):]
	if !hmac.Equal([]byte(expectedHex), []byte(actual)) {
		return ErrInvalidSignature
	}
	return nil
}

// NormalizeEvent extracts a WebhookEvent from a GitHub webhook payload.
func (g *GitHubProvider) NormalizeEvent(header http.Header, body []byte) (*WebhookEvent, error) {
	eventType := header.Get("X-GitHub-Event")
	deliveryID := header.Get("X-GitHub-Delivery")

	switch eventType {
	case "pull_request":
		return g.normalizePR(deliveryID, body)
	default:
		return nil, fmt.Errorf("event type %q: %w", eventType, ErrUnsupportedEvent)
	}
}

// githubPRPayload captures only the fields we need — unknown fields are ignored.
type githubPRPayload struct {
	Action      string `json:"action"`
	PullRequest struct {
		Merged  bool   `json:"merged"`
		Number  int    `json:"number"`
		HTMLURL string `json:"html_url"`
	} `json:"pull_request"`
	Repository struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
}

func (g *GitHubProvider) normalizePR(deliveryID string, body []byte) (*WebhookEvent, error) {
	var payload githubPRPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("unmarshal PR payload: %w", err)
	}

	var normalized string
	switch {
	case payload.Action == "closed" && payload.PullRequest.Merged:
		normalized = "pr_merged"
	case payload.Action == "closed":
		normalized = "pr_closed"
	case payload.Action == "opened":
		normalized = "pr_opened"
	case payload.Action == "reopened":
		normalized = "pr_reopened"
	default:
		return nil, fmt.Errorf("PR action %q: %w", payload.Action, ErrUnsupportedEvent)
	}

	externalID := fmt.Sprintf("%d", payload.PullRequest.Number)
	repo := payload.Repository.FullName

	return &WebhookEvent{
		Provider:    "github",
		DeliveryID:  deliveryID,
		ContentHash: ComputeContentHash("github", normalized, externalID, repo),
		EventType:   normalized,
		ExternalID:  externalID,
		ExternalURL: payload.PullRequest.HTMLURL,
		Repo:        repo,
		RawPayload:  json.RawMessage(body),
	}, nil
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test -run TestGitHub -v ./internal/webhook/`
Expected: All PASS

- [ ] **Step 6: Commit**

```bash
git add internal/webhook/
git commit -m "feat(webhook): add WebhookProvider interface and GitHub provider"
```

---

## Task 4: ADO Provider

**Files:**
- Create: `internal/webhook/ado.go`
- Create: `internal/webhook/ado_test.go`

### Steps

- [ ] **Step 1: Write failing tests for ADO provider**

```go
// internal/webhook/ado_test.go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package webhook

import (
	"encoding/base64"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestADO_ValidateRequest_ValidSecret(t *testing.T) {
	ado := &ADOProvider{}
	secret := "ado-shared-secret"
	header := http.Header{}
	header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(":"+secret)))

	err := ado.ValidateRequest(secret, header, []byte(`{}`))
	require.NoError(t, err)
}

func TestADO_ValidateRequest_InvalidSecret(t *testing.T) {
	ado := &ADOProvider{}
	header := http.Header{}
	header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(":wrong-secret")))

	err := ado.ValidateRequest("ado-shared-secret", header, []byte(`{}`))
	require.ErrorIs(t, err, ErrInvalidSignature)
}

func TestADO_ValidateRequest_MissingHeader(t *testing.T) {
	ado := &ADOProvider{}
	header := http.Header{}

	err := ado.ValidateRequest("secret", header, []byte(`{}`))
	require.ErrorIs(t, err, ErrMissingHeader)
}

func TestADO_NormalizeEvent_PRCompleted(t *testing.T) {
	ado := &ADOProvider{}
	body := []byte(`{
		"eventType": "git.pullrequest.merged",
		"resource": {
			"pullRequestId": 101,
			"status": "completed",
			"repository": {
				"name": "my-repo",
				"project": {"name": "my-project"}
			},
			"_links": {
				"web": {"href": "https://dev.azure.com/org/project/_git/repo/pullrequest/101"}
			}
		}
	}`)
	header := http.Header{}

	event, err := ado.NormalizeEvent(header, body)
	require.NoError(t, err)
	require.Equal(t, "ado", event.Provider)
	require.Equal(t, "pr_merged", event.EventType)
	require.Equal(t, "101", event.ExternalID)
	require.Equal(t, "my-project/my-repo", event.Repo)
	require.NotEmpty(t, event.ContentHash)
}

func TestADO_NormalizeEvent_UnsupportedEvent(t *testing.T) {
	ado := &ADOProvider{}
	body := []byte(`{"eventType": "build.complete", "resource": {}}`)
	header := http.Header{}

	_, err := ado.NormalizeEvent(header, body)
	require.ErrorIs(t, err, ErrUnsupportedEvent)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -run TestADO -v ./internal/webhook/`
Expected: FAIL — `ADOProvider` not defined.

- [ ] **Step 3: Implement ADOProvider**

```go
// internal/webhook/ado.go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package webhook

import (
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// ADOProvider handles Azure DevOps service hook verification and normalization.
type ADOProvider struct{}

// Name returns "ado".
func (a *ADOProvider) Name() string { return "ado" }

// ValidateRequest verifies the ADO shared secret via Basic auth.
// ADO service hooks send Basic auth with an empty username and the secret as password.
func (a *ADOProvider) ValidateRequest(secret string, header http.Header, _ []byte) error {
	authHeader := header.Get("Authorization")
	if authHeader == "" {
		return fmt.Errorf("Authorization: %w", ErrMissingHeader)
	}

	const prefix = "Basic "
	if !strings.HasPrefix(authHeader, prefix) {
		return fmt.Errorf("unsupported auth scheme: %w", ErrInvalidSignature)
	}

	decoded, err := base64.StdEncoding.DecodeString(authHeader[len(prefix):])
	if err != nil {
		return fmt.Errorf("decode basic auth: %w", ErrInvalidSignature)
	}

	// ADO format: ":password" (empty username).
	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 {
		return fmt.Errorf("malformed basic auth: %w", ErrInvalidSignature)
	}

	if subtle.ConstantTimeCompare([]byte(parts[1]), []byte(secret)) != 1 {
		return ErrInvalidSignature
	}
	return nil
}

// NormalizeEvent extracts a WebhookEvent from an ADO service hook payload.
func (a *ADOProvider) NormalizeEvent(header http.Header, body []byte) (*WebhookEvent, error) {
	var envelope adoEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("unmarshal ADO payload: %w", err)
	}

	switch {
	case strings.HasPrefix(envelope.EventType, "git.pullrequest."):
		return a.normalizePR(envelope, body)
	default:
		return nil, fmt.Errorf("event type %q: %w", envelope.EventType, ErrUnsupportedEvent)
	}
}

type adoEnvelope struct {
	EventType string `json:"eventType"`
	Resource  struct {
		PullRequestID int    `json:"pullRequestId"`
		Status        string `json:"status"`
		Repository    struct {
			Name    string `json:"name"`
			Project struct {
				Name string `json:"name"`
			} `json:"project"`
		} `json:"repository"`
		Links struct {
			Web struct {
				Href string `json:"href"`
			} `json:"web"`
		} `json:"_links"`
	} `json:"resource"`
}

func (a *ADOProvider) normalizePR(envelope adoEnvelope, body []byte) (*WebhookEvent, error) {
	var normalized string
	switch envelope.EventType {
	case "git.pullrequest.merged":
		normalized = "pr_merged"
	case "git.pullrequest.updated":
		if envelope.Resource.Status == "completed" {
			normalized = "pr_merged"
		} else {
			normalized = "pr_updated"
		}
	case "git.pullrequest.created":
		normalized = "pr_opened"
	default:
		return nil, fmt.Errorf("ADO PR event %q: %w", envelope.EventType, ErrUnsupportedEvent)
	}

	externalID := fmt.Sprintf("%d", envelope.Resource.PullRequestID)
	repo := fmt.Sprintf("%s/%s",
		envelope.Resource.Repository.Project.Name,
		envelope.Resource.Repository.Name,
	)

	return &WebhookEvent{
		Provider:    "ado",
		DeliveryID:  "", // ADO doesn't provide a delivery ID header
		ContentHash: ComputeContentHash("ado", normalized, externalID, repo),
		EventType:   normalized,
		ExternalID:  externalID,
		ExternalURL: envelope.Resource.Links.Web.Href,
		Repo:        repo,
		RawPayload:  json.RawMessage(body),
	}, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -run TestADO -v ./internal/webhook/`
Expected: All PASS

- [ ] **Step 5: Commit**

```bash
git add internal/webhook/ado.go internal/webhook/ado_test.go
git commit -m "feat(webhook): add ADO provider with shared-secret verification"
```

---

## Task 5: Webhook Processor

**Files:**
- Create: `internal/webhook/processor.go`
- Create: `internal/webhook/processor_test.go`

### Steps

- [ ] **Step 1: Write failing test for processor happy path**

```go
// internal/webhook/processor_test.go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package webhook

import (
	"context"
	"testing"

	"github.com/specgraph/specgraph/internal/storage"
	"github.com/stretchr/testify/require"
)

// fakeWebhookStore is a minimal in-memory implementation for unit tests.
type fakeWebhookStore struct {
	events      []*storage.WebhookEventRecord
	deliveryIDs map[string]bool
	hashes      map[string]bool
}

func newFakeStore() *fakeWebhookStore {
	return &fakeWebhookStore{
		deliveryIDs: make(map[string]bool),
		hashes:      make(map[string]bool),
	}
}

func (f *fakeWebhookStore) StoreWebhookEvent(_ context.Context, record *storage.WebhookEventRecord) error {
	f.events = append(f.events, record)
	if record.DeliveryID != "" {
		f.deliveryIDs[record.Provider+":"+record.DeliveryID] = true
	}
	if record.ContentHash != "" {
		f.hashes[record.ContentHash] = true
	}
	return nil
}

func (f *fakeWebhookStore) CheckDeliveryID(_ context.Context, provider, deliveryID string) (bool, error) {
	return f.deliveryIDs[provider+":"+deliveryID], nil
}

func (f *fakeWebhookStore) CheckContentHash(_ context.Context, contentHash string) (bool, error) {
	return f.hashes[contentHash], nil
}

func (f *fakeWebhookStore) ListWebhookEvents(_ context.Context, _ int) ([]*storage.WebhookEventRecord, error) {
	return f.events, nil
}

func TestProcessor_Process_NewEvent(t *testing.T) {
	store := newFakeStore()
	proc := NewProcessor(store, nil)
	ctx := context.Background()

	event := &WebhookEvent{
		Provider:    "github",
		DeliveryID:  "delivery-1",
		ContentHash: "hash-1",
		EventType:   "pr_merged",
		ExternalID:  "42",
		Repo:        "org/repo",
		RawPayload:  []byte(`{}`),
	}

	result, err := proc.Process(ctx, event)
	require.NoError(t, err)
	require.Equal(t, "no_match", result.Action)
	require.Len(t, store.events, 1)
	require.Equal(t, "processed", store.events[0].Status)
}

func TestProcessor_Process_DuplicateDeliveryID(t *testing.T) {
	store := newFakeStore()
	store.deliveryIDs["github:delivery-dup"] = true
	proc := NewProcessor(store, nil)
	ctx := context.Background()

	event := &WebhookEvent{
		Provider:    "github",
		DeliveryID:  "delivery-dup",
		ContentHash: "hash-new",
		EventType:   "pr_merged",
	}

	result, err := proc.Process(ctx, event)
	require.NoError(t, err)
	require.Equal(t, "duplicate", result.Action)
}

func TestProcessor_Process_DuplicateContentHash(t *testing.T) {
	store := newFakeStore()
	store.hashes["hash-dup"] = true
	proc := NewProcessor(store, nil)
	ctx := context.Background()

	event := &WebhookEvent{
		Provider:    "github",
		DeliveryID:  "delivery-new",
		ContentHash: "hash-dup",
		EventType:   "pr_merged",
	}

	result, err := proc.Process(ctx, event)
	require.NoError(t, err)
	require.Equal(t, "duplicate", result.Action)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -run TestProcessor -v ./internal/webhook/`
Expected: FAIL — `Processor` not defined.

- [ ] **Step 3: Implement the Processor**

```go
// internal/webhook/processor.go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package webhook

import (
	"context"
	"log/slog"

	"github.com/specgraph/specgraph/internal/storage"
)

// ProcessResult describes the outcome of processing a webhook event.
type ProcessResult struct {
	Action    string // "slice_completed", "spec_updated", "no_match", "duplicate"
	SpecSlug  string // affected spec (empty if no_match)
	SliceSlug string // affected slice (empty if not slice-level)
}

// Processor handles webhook event dedup, mapping, and state transitions.
type Processor struct {
	store  storage.WebhookBackend
	logger *slog.Logger
}

// NewProcessor creates a Processor with the given storage backend.
func NewProcessor(store storage.WebhookBackend, logger *slog.Logger) *Processor {
	if logger == nil {
		logger = slog.Default()
	}
	return &Processor{store: store, logger: logger}
}

// Process checks for duplicates, maps the event to state transitions, and logs it.
func (p *Processor) Process(ctx context.Context, event *WebhookEvent) (*ProcessResult, error) {
	// Step 1: Delivery ID dedup.
	if event.DeliveryID != "" {
		dup, err := p.store.CheckDeliveryID(ctx, event.Provider, event.DeliveryID)
		if err != nil {
			return nil, err
		}
		if dup {
			p.logger.Info("duplicate delivery ID",
				"provider", event.Provider,
				"delivery_id", event.DeliveryID,
			)
			return &ProcessResult{Action: "duplicate"}, nil
		}
	}

	// Step 2: Content hash dedup.
	if event.ContentHash != "" {
		dup, err := p.store.CheckContentHash(ctx, event.ContentHash)
		if err != nil {
			return nil, err
		}
		if dup {
			p.logger.Info("duplicate content hash",
				"provider", event.Provider,
				"content_hash", event.ContentHash,
			)
			// Still log the event as duplicate for audit.
			record := p.eventToRecord(event, "duplicate", "duplicate", "", "")
			if storeErr := p.store.StoreWebhookEvent(ctx, record); storeErr != nil {
				p.logger.Error("store duplicate event", "error", storeErr)
			}
			return &ProcessResult{Action: "duplicate"}, nil
		}
	}

	// Step 3: Map event to action.
	// For now, no automatic state transitions — log and report no_match.
	// This is the extension point where event-to-action mapping will live.
	result := &ProcessResult{Action: "no_match"}

	// Step 4: Log the event.
	record := p.eventToRecord(event, "processed", result.Action, result.SpecSlug, result.SliceSlug)
	if err := p.store.StoreWebhookEvent(ctx, record); err != nil {
		return nil, err
	}

	p.logger.Info("webhook event processed",
		"provider", event.Provider,
		"event_type", event.EventType,
		"action", result.Action,
	)
	return result, nil
}

func (p *Processor) eventToRecord(
	event *WebhookEvent, status, action, specSlug, sliceSlug string,
) *storage.WebhookEventRecord {
	return &storage.WebhookEventRecord{
		ID:          "whe-" + mustULID(),
		Provider:    event.Provider,
		DeliveryID:  event.DeliveryID,
		ContentHash: event.ContentHash,
		EventType:   event.EventType,
		ExternalID:  event.ExternalID,
		ExternalURL: event.ExternalURL,
		Repo:        event.Repo,
		Status:      status,
		Action:      action,
		SpecSlug:    specSlug,
		SliceSlug:   sliceSlug,
		RawPayload:  event.RawPayload,
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -run TestProcessor -v ./internal/webhook/`
Expected: All PASS

- [ ] **Step 5: Commit**

```bash
git add internal/webhook/processor.go internal/webhook/processor_test.go
git commit -m "feat(webhook): add Processor with dedup and event logging"
```

---

## Task 6: Config and HTTP Handler

**Files:**
- Modify: `internal/config/global.go` (add `WebhookConfig`)
- Create: `internal/server/webhook_handler.go`
- Create: `internal/server/webhook_handler_test.go`
- Modify: `cmd/specgraph/serve.go` (register handler)

### Steps

- [ ] **Step 1: Add WebhookConfig to config**

In `internal/config/global.go`, add the webhook config struct and field:

```go
// ServerSection configures the specgraph server daemon.
type ServerSection struct {
	Listen   string         `yaml:"listen"`
	Mode     string         `yaml:"mode"`
	Backend  string         `yaml:"backend"`
	Postgres PostgresConfig `yaml:"postgres"`
	Docker   bool           `yaml:"docker"`
	Webhooks WebhookConfig  `yaml:"webhooks"`
}

// WebhookConfig configures inbound webhook handling.
type WebhookConfig struct {
	Enabled         bool                      `yaml:"enabled"`
	Providers       map[string]ProviderSecret `yaml:"providers"`
	RateLimit       WebhookRateLimit          `yaml:"rate_limit"`
	MaxPayloadBytes int                       `yaml:"max_payload_bytes"`
}

// ProviderSecret holds the shared secret for a webhook provider.
type ProviderSecret struct {
	Secret string `yaml:"secret"`
}

// WebhookRateLimit configures webhook rate limiting.
type WebhookRateLimit struct {
	RequestsPerMinute int `yaml:"requests_per_minute"`
}
```

- [ ] **Step 2: Write failing handler test**

```go
// internal/server/webhook_handler_test.go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server_test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/specgraph/specgraph/internal/config"
	"github.com/specgraph/specgraph/internal/server"
	"github.com/specgraph/specgraph/internal/storage"
	"github.com/specgraph/specgraph/internal/webhook"
	"github.com/stretchr/testify/require"
)

func signGitHub(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// discardStore satisfies WebhookBackend but discards everything.
type discardStore struct{}

func (d *discardStore) StoreWebhookEvent(_ context.Context, _ *storage.WebhookEventRecord) error {
	return nil
}
func (d *discardStore) CheckDeliveryID(_ context.Context, _, _ string) (bool, error) {
	return false, nil
}
func (d *discardStore) CheckContentHash(_ context.Context, _ string) (bool, error) {
	return false, nil
}
func (d *discardStore) ListWebhookEvents(_ context.Context, _ int) ([]*storage.WebhookEventRecord, error) {
	return nil, nil
}

func TestWebhookHandler_GitHubValidSignature(t *testing.T) {
	mux := http.NewServeMux()
	cfg := config.WebhookConfig{
		Enabled:         true,
		Providers:       map[string]config.ProviderSecret{"github": {Secret: "test-secret"}},
		MaxPayloadBytes: 1 << 20,
	}
	processor := webhook.NewProcessor(&discardStore{}, nil)
	server.RegisterWebhookHandlers(mux, cfg, processor)

	body := []byte(`{"action":"closed","pull_request":{"merged":true,"number":1,"html_url":"https://github.com/o/r/pull/1"},"repository":{"full_name":"o/r"}}`)
	req := httptest.NewRequest("POST", "/api/webhooks/github", bytes.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", signGitHub("test-secret", body))
	req.Header.Set("X-GitHub-Event", "pull_request")
	req.Header.Set("X-GitHub-Delivery", "test-delivery")

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
}

func TestWebhookHandler_GitHubInvalidSignature(t *testing.T) {
	mux := http.NewServeMux()
	cfg := config.WebhookConfig{
		Enabled:         true,
		Providers:       map[string]config.ProviderSecret{"github": {Secret: "test-secret"}},
		MaxPayloadBytes: 1 << 20,
	}
	processor := webhook.NewProcessor(&discardStore{}, nil)
	server.RegisterWebhookHandlers(mux, cfg, processor)

	body := []byte(`{"action":"closed"}`)
	req := httptest.NewRequest("POST", "/api/webhooks/github", bytes.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", "sha256=invalid")
	req.Header.Set("X-GitHub-Event", "pull_request")
	req.Header.Set("X-GitHub-Delivery", "test-delivery")

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestWebhookHandler_DisabledReturns404(t *testing.T) {
	mux := http.NewServeMux()
	cfg := config.WebhookConfig{Enabled: false}
	processor := webhook.NewProcessor(&discardStore{}, nil)
	server.RegisterWebhookHandlers(mux, cfg, processor)

	req := httptest.NewRequest("POST", "/api/webhooks/github", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestWebhookHandler_PayloadTooLarge(t *testing.T) {
	mux := http.NewServeMux()
	cfg := config.WebhookConfig{
		Enabled:         true,
		Providers:       map[string]config.ProviderSecret{"github": {Secret: "s"}},
		MaxPayloadBytes: 10, // tiny limit
	}
	processor := webhook.NewProcessor(&discardStore{}, nil)
	server.RegisterWebhookHandlers(mux, cfg, processor)

	body := bytes.Repeat([]byte("x"), 100)
	req := httptest.NewRequest("POST", "/api/webhooks/github", bytes.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", signGitHub("s", body))
	req.Header.Set("X-GitHub-Event", "pull_request")
	req.Header.Set("X-GitHub-Delivery", "test")

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	require.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test -run TestWebhookHandler -v ./internal/server/`
Expected: FAIL — `RegisterWebhookHandlers` not defined.

- [ ] **Step 4: Implement the webhook HTTP handler**

```go
// internal/server/webhook_handler.go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/specgraph/specgraph/internal/config"
	"github.com/specgraph/specgraph/internal/webhook"
)

// RegisterWebhookHandlers registers raw HTTP handlers for inbound webhooks.
// When webhooks are disabled in config, endpoints return 404.
func RegisterWebhookHandlers(mux *http.ServeMux, cfg config.WebhookConfig, processor *webhook.Processor) {
	if !cfg.Enabled {
		mux.HandleFunc("/api/webhooks/github", http.NotFound)
		mux.HandleFunc("/api/webhooks/ado", http.NotFound)
		return
	}

	maxBytes := cfg.MaxPayloadBytes
	if maxBytes <= 0 {
		maxBytes = 1 << 20 // 1 MiB default
	}

	providers := map[string]webhook.WebhookProvider{
		"github": &webhook.GitHubProvider{},
		"ado":    &webhook.ADOProvider{},
	}

	for name, provider := range providers {
		secret := ""
		if ps, ok := cfg.Providers[name]; ok {
			secret = ps.Secret
		}
		mux.Handle("/api/webhooks/"+name,
			newWebhookHandler(provider, secret, maxBytes, processor))
	}
}

func newWebhookHandler(
	provider webhook.WebhookProvider,
	secret string,
	maxBytes int,
	processor *webhook.Processor,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}

		// Enforce payload size limit before any processing.
		limited := http.MaxBytesReader(w, r.Body, int64(maxBytes))
		body, err := io.ReadAll(limited)
		if err != nil {
			http.Error(w, `{"error":"payload too large"}`, http.StatusRequestEntityTooLarge)
			return
		}

		// Verify signature before parsing.
		if err := provider.ValidateRequest(secret, r.Header, body); err != nil {
			if errors.Is(err, webhook.ErrInvalidSignature) || errors.Is(err, webhook.ErrMissingHeader) {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			http.Error(w, `{"error":"bad request"}`, http.StatusBadRequest)
			return
		}

		// Normalize.
		event, err := provider.NormalizeEvent(r.Header, body)
		if err != nil {
			if errors.Is(err, webhook.ErrUnsupportedEvent) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusAccepted)
				json.NewEncoder(w).Encode(map[string]string{"status": "unsupported_event"}) //nolint:errcheck
				return
			}
			http.Error(w, `{"error":"bad request"}`, http.StatusBadRequest)
			return
		}

		// Process.
		result, err := processor.Process(r.Context(), event)
		if err != nil {
			slog.Error("webhook processing failed",
				"provider", provider.Name(),
				"error", err,
			)
			http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{ //nolint:errcheck
			"status": "ok",
			"action": result.Action,
		})
	})
}
```

- [ ] **Step 5: Run handler tests**

Run: `go test -run TestWebhookHandler -v ./internal/server/`
Expected: All PASS

- [ ] **Step 6: Wire into serve.go**

In `cmd/specgraph/serve.go`, after `server.RegisterAuthHandlers(...)` (line 229), add:

```go
	// Webhook handlers (raw HTTP, not ConnectRPC).
	webhookProcessor := webhook.NewProcessor(store, slog.Default())
	server.RegisterWebhookHandlers(mux, cfg.Server.Webhooks, webhookProcessor)
```

And add the import:

```go
	"github.com/specgraph/specgraph/internal/webhook"
```

- [ ] **Step 7: Verify build**

Run: `task build`
Expected: Build succeeds.

- [ ] **Step 8: Commit**

```bash
git add internal/config/global.go internal/server/webhook_handler.go \
  internal/server/webhook_handler_test.go cmd/specgraph/serve.go
git commit -m "feat(webhook): add HTTP handler with payload limits and config"
```

---

## Task 7: E2E Tests

**Files:**
- Create: `e2e/api/webhook_test.go`
- Modify: `e2e/testutil/server.go` (register webhook handlers in test server)

### Steps

- [ ] **Step 1: Register webhook handlers in test server**

In `e2e/testutil/server.go`, after the `RegisterSyncService` call (line 60), add:

```go
	webhookCfg := config.WebhookConfig{
		Enabled: true,
		Providers: map[string]config.ProviderSecret{
			"github": {Secret: "e2e-test-secret"},
			"ado":    {Secret: "e2e-ado-secret"},
		},
		MaxPayloadBytes: 1 << 20,
	}
	webhookProcessor := webhook.NewProcessor(store, nil)
	server.RegisterWebhookHandlers(mux, webhookCfg, webhookProcessor)
```

Add imports for `config` and `webhook` packages.

- [ ] **Step 2: Write the E2E webhook test**

```go
// e2e/api/webhook_test.go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

//go:build e2e

package api_test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func signGitHubPayload(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

var _ = Describe("Webhook Endpoints", func() {
	const ghSecret = "e2e-test-secret"
	ctx := context.Background()

	It("accepts a valid GitHub PR merged webhook", func() {
		body := []byte(`{
			"action": "closed",
			"pull_request": {
				"merged": true,
				"number": 99,
				"html_url": "https://github.com/test/repo/pull/99"
			},
			"repository": {"full_name": "test/repo"}
		}`)
		req, err := http.NewRequest("POST", serverInfo.BaseURL+"/api/webhooks/github",
			bytes.NewReader(body))
		Expect(err).NotTo(HaveOccurred())
		req.Header.Set("X-Hub-Signature-256", signGitHubPayload(ghSecret, body))
		req.Header.Set("X-GitHub-Event", "pull_request")
		req.Header.Set("X-GitHub-Delivery", "e2e-delivery-1")

		resp, err := http.DefaultClient.Do(req)
		Expect(err).NotTo(HaveOccurred())
		defer resp.Body.Close()
		Expect(resp.StatusCode).To(Equal(http.StatusOK))
	})

	It("rejects a GitHub webhook with invalid signature", func() {
		body := []byte(`{"action":"closed"}`)
		req, err := http.NewRequest("POST", serverInfo.BaseURL+"/api/webhooks/github",
			bytes.NewReader(body))
		Expect(err).NotTo(HaveOccurred())
		req.Header.Set("X-Hub-Signature-256", "sha256=invalid")
		req.Header.Set("X-GitHub-Event", "pull_request")
		req.Header.Set("X-GitHub-Delivery", "e2e-delivery-bad")

		resp, err := http.DefaultClient.Do(req)
		Expect(err).NotTo(HaveOccurred())
		defer resp.Body.Close()
		Expect(resp.StatusCode).To(Equal(http.StatusUnauthorized))
	})

	It("returns 202 for unsupported GitHub events", func() {
		body := []byte(`{"action":"created"}`)
		req, err := http.NewRequest("POST", serverInfo.BaseURL+"/api/webhooks/github",
			bytes.NewReader(body))
		Expect(err).NotTo(HaveOccurred())
		req.Header.Set("X-Hub-Signature-256", signGitHubPayload(ghSecret, body))
		req.Header.Set("X-GitHub-Event", "star")
		req.Header.Set("X-GitHub-Delivery", "e2e-delivery-star")

		resp, err := http.DefaultClient.Do(req)
		Expect(err).NotTo(HaveOccurred())
		defer resp.Body.Close()
		Expect(resp.StatusCode).To(Equal(http.StatusAccepted))
	})

	It("handles duplicate delivery ID idempotently", func() {
		body := []byte(`{
			"action": "closed",
			"pull_request": {
				"merged": true,
				"number": 100,
				"html_url": "https://github.com/test/repo/pull/100"
			},
			"repository": {"full_name": "test/repo"}
		}`)

		for i := range 2 {
			req, err := http.NewRequest("POST", serverInfo.BaseURL+"/api/webhooks/github",
				bytes.NewReader(body))
			Expect(err).NotTo(HaveOccurred())
			req.Header.Set("X-Hub-Signature-256", signGitHubPayload(ghSecret, body))
			req.Header.Set("X-GitHub-Event", "pull_request")
			req.Header.Set("X-GitHub-Delivery", "e2e-delivery-dedup")

			resp, err := http.DefaultClient.Do(req)
			Expect(err).NotTo(HaveOccurred())
			respBody, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusOK),
				"attempt %d: status %d, body: %s", i, resp.StatusCode, string(respBody))
		}

		// Both should succeed — second is a dedup no-op.
		// Verify via listing (database content check).
		events, err := serverInfo.Store.ListWebhookEvents(ctx, 100)
		Expect(err).NotTo(HaveOccurred())
		// Only one event with this delivery ID should be stored as "processed".
		var processed int
		for _, e := range events {
			if e.DeliveryID == "e2e-delivery-dedup" && e.Status == "processed" {
				processed++
			}
		}
		Expect(processed).To(Equal(1))
	})
})
```

- [ ] **Step 3: Run E2E tests**

Run: `go test -tags e2e -run "Webhook Endpoints" -v ./e2e/api/`
Expected: All PASS (requires Docker for PostgreSQL container).

- [ ] **Step 4: Commit**

```bash
git add e2e/api/webhook_test.go e2e/testutil/server.go
git commit -m "test(webhook): add E2E tests for webhook endpoints"
```

---

## Task 8: Quality Gate and Final Verification

- [ ] **Step 1: Run task check**

Run: `task check`
Expected: All passing (fmt, lint, build, unit tests).

- [ ] **Step 2: Run task pr-prep**

Run: `task pr-prep`
Expected: All passing (includes integration and e2e tests with Docker).

- [ ] **Step 3: Fix any lint or formatting issues**

Address any issues from `task check`. Common ones:
- Missing license headers: `task license:add`
- Missing package comments: add `// Package webhook ...` doc comment
- Format drift: `task fmt`

- [ ] **Step 4: Commit fixes if any**

```bash
git add -A
git commit -m "fix(webhook): address lint and formatting issues"
```

- [ ] **Step 5: Report completion**

```bash
specgraph report-completion inbound-webhooks
```

---

## Notes

**What's deferred (steel-thread scope):** This plan implements slice 1 — the GitHub PR-merged steel thread. Event-to-action mapping (actually transitioning slice/spec state) returns `no_match` for now. Slices 2-4 (ADO provider, dedup hardening, config validation, rate limiting) are included in the plan but can be executed as follow-up work after the steel thread proves the architecture.

**ADO provider** is implemented in Task 4 but the E2E tests focus on GitHub. ADO E2E tests should follow the same pattern.

**Rate limiting** (`requests_per_minute`) is defined in config but not enforced in the handler yet — this is a hardening concern for a follow-up slice. The handler structure supports adding middleware easily.

**Config validation** (missing secret for enabled provider fails at startup) should be added as part of the config hardening slice.
