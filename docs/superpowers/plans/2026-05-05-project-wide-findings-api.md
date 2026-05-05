# Project-Wide Findings API Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a project-wide findings API so `specgraph://findings` and the `specgraph://prime` Open Findings section can list findings without making an invalid per-spec `ListFindings` request.

**Architecture:** Keep `ListFindings` as the per-spec API and add a separate `ListProjectFindings` RPC for project-wide reads. The server will adapt the existing `storage.ListAllFindings(ctx)` query into proto responses, including `spec_slug`, and MCP resources will consume the new RPC. For this issue, every stored finding is treated as open because the current findings model has no status field.

**Tech Stack:** Go, ConnectRPC, Protocol Buffers, PostgreSQL storage, MCP resource handlers, Taskfile

**Design Source:** `.cursor/plans/findings_api_fix_18de2513.plan.md`

---

## File Map

### Modified Files

| File | Responsibility |
|------|----------------|
| `proto/specgraph/v1/analytical_pass.proto` | Add `ListProjectFindings` RPC, request/response messages, and `AnalyticalFinding.spec_slug` |
| `gen/specgraph/v1/analytical_pass.pb.go` | Generated protobuf messages and descriptors |
| `gen/specgraph/v1/analytical_pass.connect.go` | Generated ConnectRPC client/server interfaces |
| `internal/server/analytical_pass_handler.go` | Implement project-wide handler and share finding conversion |
| `internal/server/analytical_pass_handler_test.go` | Test project-wide API behavior and spec slug propagation |
| `internal/mcp/resources.go` | Update `specgraph://findings` and `specgraph://prime` to call `ListProjectFindings` |
| `internal/mcp/resources_test.go` | Test MCP resource behavior and prime regression |
| `internal/mcp/testhelpers_test.go` | Add mock support for `ListProjectFindings` |

---

## Task 1: Start From Main and Claim the Issue

**Files:**

- Modify: `.beads/issues.jsonl`

- [ ] **Step 1: Verify the working copy is based on current `main`**

Run:

```bash
jj --no-pager status
jj --no-pager log -r '@ | @-' --no-graph
```

Expected: working copy has no code changes before implementation, and `@-` is the current `main` bookmark.

- [ ] **Step 2: Claim `spgr-vabz`**

Run:

```bash
bd update spgr-vabz --claim
```

Expected: the issue is in progress/claimed. Do not manually edit `.beads/issues.jsonl`.

---

## Task 2: Add Failing Server Tests for Project-Wide Findings

**Files:**

- Modify: `internal/server/analytical_pass_handler_test.go`

- [ ] **Step 1: Extend the in-memory analytical pass backend**

Add this method to `analyticalPassTestBackend` so tests can exercise `ListAllFindings` through the real handler:

```go
func (b *analyticalPassTestBackend) ListAllFindings(_ context.Context) ([]*storage.AnalyticalFinding, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	var all []*storage.AnalyticalFinding
	for key, findings := range b.findings {
		slug, _, ok := strings.Cut(key, ":")
		if !ok {
			continue
		}
		for i := range findings {
			f := findings[i]
			f.SpecSlug = slug
			all = append(all, &f)
		}
	}
	return all, nil
}
```

- [ ] **Step 2: Add the empty project-wide findings test**

Add:

```go
func TestListProjectFindings_EmptyProject(t *testing.T) {
	backend := newAnalyticalPassTestBackend()
	client := setupAnalyticalPassServer(t, backend)

	resp, err := client.ListProjectFindings(context.Background(), connect.NewRequest(&specv1.ListProjectFindingsRequest{}))
	require.NoError(t, err)
	require.Empty(t, resp.Msg.GetFindings())
}
```

- [ ] **Step 3: Add the one-spec-with-no-findings test**

Add:

```go
func TestListProjectFindings_OneSpecNoFindings(t *testing.T) {
	backend := newAnalyticalPassTestBackend()
	_, err := backend.CreateSpec(context.Background(), "spec-without-findings", "Test spec", "p1", "medium")
	require.NoError(t, err)
	client := setupAnalyticalPassServer(t, backend)

	resp, err := client.ListProjectFindings(context.Background(), connect.NewRequest(&specv1.ListProjectFindingsRequest{}))
	require.NoError(t, err)
	require.Empty(t, resp.Msg.GetFindings())
}
```

- [ ] **Step 4: Add the multiple-specs-with-findings test**

Add:

```go
func TestListProjectFindings_MultipleSpecs(t *testing.T) {
	backend := newAnalyticalPassTestBackend()
	_, err := backend.CreateSpec(context.Background(), "spec-a", "A", "p1", "medium")
	require.NoError(t, err)
	_, err = backend.CreateSpec(context.Background(), "spec-b", "B", "p1", "medium")
	require.NoError(t, err)
	client := setupAnalyticalPassServer(t, backend)

	_, err = client.StoreFindings(context.Background(), connect.NewRequest(&specv1.StoreFindingsRequest{
		Slug:     "spec-a",
		PassType: specv1.PassType_PASS_TYPE_CONSTITUTION_CHECK,
		Findings: []*specv1.AnalyticalFindingInput{{
			Severity: specv1.FindingSeverity_FINDING_SEVERITY_WARNING,
			Summary:  "A finding",
		}},
	}))
	require.NoError(t, err)
	_, err = client.StoreFindings(context.Background(), connect.NewRequest(&specv1.StoreFindingsRequest{
		Slug:     "spec-b",
		PassType: specv1.PassType_PASS_TYPE_RED_TEAM,
		Findings: []*specv1.AnalyticalFindingInput{{
			Severity: specv1.FindingSeverity_FINDING_SEVERITY_CRITICAL,
			Summary:  "B finding",
		}},
	}))
	require.NoError(t, err)

	resp, err := client.ListProjectFindings(context.Background(), connect.NewRequest(&specv1.ListProjectFindingsRequest{}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.GetFindings(), 2)

	bySlug := map[string]*specv1.AnalyticalFinding{}
	for _, f := range resp.Msg.GetFindings() {
		bySlug[f.GetSpecSlug()] = f
	}
	require.Equal(t, "A finding", bySlug["spec-a"].GetSummary())
	require.Equal(t, specv1.PassType_PASS_TYPE_CONSTITUTION_CHECK, bySlug["spec-a"].GetPassType())
	require.Equal(t, "B finding", bySlug["spec-b"].GetSummary())
	require.Equal(t, specv1.PassType_PASS_TYPE_RED_TEAM, bySlug["spec-b"].GetPassType())
}
```

- [ ] **Step 5: Add optional pass-type filter coverage**

Add:

```go
func TestListProjectFindings_FilterByPassType(t *testing.T) {
	backend := newAnalyticalPassTestBackend()
	_, err := backend.CreateSpec(context.Background(), "spec-a", "A", "p1", "medium")
	require.NoError(t, err)
	_, err = backend.CreateSpec(context.Background(), "spec-b", "B", "p1", "medium")
	require.NoError(t, err)
	client := setupAnalyticalPassServer(t, backend)

	_, err = client.StoreFindings(context.Background(), connect.NewRequest(&specv1.StoreFindingsRequest{
		Slug:     "spec-a",
		PassType: specv1.PassType_PASS_TYPE_CONSTITUTION_CHECK,
		Findings: []*specv1.AnalyticalFindingInput{{Severity: specv1.FindingSeverity_FINDING_SEVERITY_WARNING, Summary: "constitution"}},
	}))
	require.NoError(t, err)
	_, err = client.StoreFindings(context.Background(), connect.NewRequest(&specv1.StoreFindingsRequest{
		Slug:     "spec-b",
		PassType: specv1.PassType_PASS_TYPE_RED_TEAM,
		Findings: []*specv1.AnalyticalFindingInput{{Severity: specv1.FindingSeverity_FINDING_SEVERITY_CRITICAL, Summary: "red team"}},
	}))
	require.NoError(t, err)

	resp, err := client.ListProjectFindings(context.Background(), connect.NewRequest(&specv1.ListProjectFindingsRequest{
		PassType: specv1.PassType_PASS_TYPE_RED_TEAM,
	}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.GetFindings(), 1)
	require.Equal(t, "spec-b", resp.Msg.GetFindings()[0].GetSpecSlug())
	require.Equal(t, "red team", resp.Msg.GetFindings()[0].GetSummary())
}
```

- [ ] **Step 6: Verify the new tests fail because the API does not exist**

Run:

```bash
go test ./internal/server
```

Expected: compile failure mentioning missing `ListProjectFindingsRequest`, `ListProjectFindings`, or handler interface implementation.

---

## Task 3: Add the Project-Wide Findings Proto API

**Files:**

- Modify: `proto/specgraph/v1/analytical_pass.proto`
- Modify: `gen/specgraph/v1/analytical_pass.pb.go`
- Modify: `gen/specgraph/v1/analytical_pass.connect.go`

- [ ] **Step 1: Update the service and messages**

Change `analytical_pass.proto` to:

```protobuf
service AnalyticalPassService {
  rpc RunAnalyticalPass(RunAnalyticalPassRequest) returns (RunAnalyticalPassResponse);
  rpc StoreFindings(StoreFindingsRequest) returns (StoreFindingsResponse);
  rpc ListFindings(ListFindingsRequest) returns (ListFindingsResponse);
  rpc ListProjectFindings(ListProjectFindingsRequest) returns (ListProjectFindingsResponse);
}
```

Add `spec_slug` to `AnalyticalFinding`:

```protobuf
message AnalyticalFinding {
  string id = 1;
  PassType pass_type = 2;
  FindingSeverity severity = 3;
  string summary = 4;
  string detail = 5;
  string constraint = 6;
  string resolution = 7;
  int32 version = 8;
  string spec_slug = 9;
}
```

Add the project-wide request/response:

```protobuf
message ListProjectFindingsRequest {
  PassType pass_type = 1;
}

message ListProjectFindingsResponse {
  repeated AnalyticalFinding findings = 1;
}
```

- [ ] **Step 2: Regenerate protobuf code**

Run:

```bash
task proto
```

Expected: generated `gen/specgraph/v1/analytical_pass.pb.go` and `gen/specgraph/v1/analytical_pass.connect.go` update cleanly.

---

## Task 4: Implement the Server Handler

**Files:**

- Modify: `internal/server/analytical_pass_handler.go`

- [ ] **Step 1: Extract shared conversion helper**

Add this helper near `ListFindings`:

```go
func analyticalFindingToProto(f *storage.AnalyticalFinding) (*specv1.AnalyticalFinding, error) {
	protoSev, sevErr := findingSeverityToProto(f.Severity)
	if sevErr != nil {
		return nil, sevErr
	}
	protoPassType, ptErr := passTypeToProto(f.PassType)
	if ptErr != nil {
		return nil, ptErr
	}
	return &specv1.AnalyticalFinding{
		Id:         f.ID,
		SpecSlug:   f.SpecSlug,
		PassType:   protoPassType,
		Severity:   protoSev,
		Summary:    f.Summary,
		Detail:     f.Detail,
		Constraint: f.Constraint,
		Resolution: f.Resolution,
		Version:    f.Version,
	}, nil
}
```

- [ ] **Step 2: Refactor `ListFindings` to use the helper**

Replace the inline conversion loop with:

```go
protoFindings := make([]*specv1.AnalyticalFinding, len(findings))
for i := range findings {
	f := findings[i]
	if f.SpecSlug == "" {
		f.SpecSlug = msg.Slug
	}
	protoFinding, convErr := analyticalFindingToProto(&f)
	if convErr != nil {
		return nil, analyticalPassError(convErr)
	}
	protoFindings[i] = protoFinding
}

return connect.NewResponse(&specv1.ListFindingsResponse{
	Findings: protoFindings,
}), nil
```

- [ ] **Step 3: Implement `ListProjectFindings`**

Add:

```go
func (h *AnalyticalPassHandler) ListProjectFindings(ctx context.Context, req *connect.Request[specv1.ListProjectFindingsRequest]) (*connect.Response[specv1.ListProjectFindingsResponse], error) {
	store, err := scopeStore(ctx, h.scoper)
	if err != nil {
		return nil, err
	}

	var pt storage.PassType
	if req.Msg.GetPassType() != specv1.PassType_PASS_TYPE_UNSPECIFIED {
		var ptErr error
		pt, ptErr = passTypeFromProto(req.Msg.GetPassType())
		if ptErr != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, ptErr)
		}
	}

	findings, err := store.ListAllFindings(ctx)
	if err != nil {
		return nil, analyticalPassError(err)
	}

	protoFindings := make([]*specv1.AnalyticalFinding, 0, len(findings))
	for _, f := range findings {
		if f == nil {
			continue
		}
		if pt != "" && f.PassType != pt {
			continue
		}
		protoFinding, convErr := analyticalFindingToProto(f)
		if convErr != nil {
			return nil, analyticalPassError(convErr)
		}
		protoFindings = append(protoFindings, protoFinding)
	}

	return connect.NewResponse(&specv1.ListProjectFindingsResponse{
		Findings: protoFindings,
	}), nil
}
```

- [ ] **Step 4: Verify server tests pass**

Run:

```bash
go test ./internal/server
```

Expected: PASS.

---

## Task 5: Add Failing MCP Resource Tests

**Files:**

- Modify: `internal/mcp/testhelpers_test.go`
- Modify: `internal/mcp/resources_test.go`

- [ ] **Step 1: Extend the analytical pass mock**

Add a `listProjectFindings` function field to `mockAnalyticalPassService`:

```go
listProjectFindings func(req *specv1.ListProjectFindingsRequest) (*specv1.ListProjectFindingsResponse, error)
```

Add the method:

```go
func (m *mockAnalyticalPassService) ListProjectFindings(_ context.Context, req *connect.Request[specv1.ListProjectFindingsRequest]) (*connect.Response[specv1.ListProjectFindingsResponse], error) {
	if m.listProjectFindings == nil {
		panic("mockAnalyticalPassService.ListProjectFindings not configured")
	}
	resp, err := m.listProjectFindings(req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}
```

- [ ] **Step 2: Update `defaultAnalyticalPassMock`**

Set both old and new list methods so unrelated tests stay simple:

```go
func defaultAnalyticalPassMock() *mockAnalyticalPassService {
	return &mockAnalyticalPassService{
		listFindings: func(_ string) (*specv1.ListFindingsResponse, error) {
			return &specv1.ListFindingsResponse{}, nil
		},
		listProjectFindings: func(_ *specv1.ListProjectFindingsRequest) (*specv1.ListProjectFindingsResponse, error) {
			return &specv1.ListProjectFindingsResponse{}, nil
		},
	}
}
```

- [ ] **Step 3: Update `TestFindingsResource` to expect project-wide listing**

Configure `listProjectFindings` instead of `listFindings`:

```go
listProjectFindings: func(req *specv1.ListProjectFindingsRequest) (*specv1.ListProjectFindingsResponse, error) {
	require.Equal(t, specv1.PassType_PASS_TYPE_UNSPECIFIED, req.GetPassType())
	return &specv1.ListProjectFindingsResponse{
		Findings: []*specv1.AnalyticalFinding{
			{Id: "finding-1", SpecSlug: "spec-a", Summary: "missing constraint"},
		},
	}, nil
},
```

- [ ] **Step 4: Update prime findings tests to expect project-wide listing**

In `TestPrimeResource_FindingsSection` and `TestPrimeResource_SeverityOrdering`, configure `listProjectFindings` with `ListProjectFindingsResponse` and keep the existing severity assertions.

- [ ] **Step 5: Add the regression test for invalid-argument suppression removal**

Add:

```go
func TestPrimeResource_ProjectFindingsDoesNotCallPerSpecListWithoutSlug(t *testing.T) {
	c := &Client{
		Constitution: defaultConstitutionMock(),
		Spec: &mockSpecService{
			listSpecs: func() (*specv1.ListSpecsResponse, error) {
				return &specv1.ListSpecsResponse{}
			},
		},
		Graph: &mockGraphService{
			getReady: func() (*specv1.GetReadyResponse, error) {
				return &specv1.GetReadyResponse{}
			},
		},
		AnalyticalPass: &mockAnalyticalPassService{
			listFindings: func(_ string) (*specv1.ListFindingsResponse, error) {
				return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("slug is required"))
			},
			listProjectFindings: func(_ *specv1.ListProjectFindingsRequest) (*specv1.ListProjectFindingsResponse, error) {
				return &specv1.ListProjectFindingsResponse{
					Findings: []*specv1.AnalyticalFinding{
						{Id: "f1", SpecSlug: "spec-a", Severity: specv1.FindingSeverity_FINDING_SEVERITY_WARNING},
					},
				}, nil
			},
		},
	}

	content, err := primeResourceHandler(c)(context.Background(), "specgraph://prime")
	require.NoError(t, err)
	require.NotEmpty(t, content)
	require.Contains(t, content[0].Text, "## Open Findings")
	require.Contains(t, content[0].Text, "FINDING_SEVERITY_WARNING: 1")
	require.NotContains(t, content[0].Text, "slug is required")
}
```

- [ ] **Step 6: Verify MCP tests fail before resource code changes**

Run:

```bash
go test ./internal/mcp
```

Expected: tests fail because resources still call `ListFindings` or because mocks panic on missing `ListProjectFindings` calls.

---

## Task 6: Update MCP Resources to Use the New API

**Files:**

- Modify: `internal/mcp/resources.go`

- [ ] **Step 1: Update `findingsResourceHandler`**

Replace:

```go
resp, err := c.AnalyticalPass.ListFindings(ctx, connect.NewRequest(&specv1.ListFindingsRequest{}))
```

with:

```go
resp, err := c.AnalyticalPass.ListProjectFindings(ctx, connect.NewRequest(&specv1.ListProjectFindingsRequest{}))
```

Return `resourceJSON(uri, resp.Msg)` as before.

- [ ] **Step 2: Update the prime Open Findings section**

Replace the `ListFindings` call with:

```go
findingsResp, err := c.AnalyticalPass.ListProjectFindings(ctx, connect.NewRequest(&specv1.ListProjectFindingsRequest{}))
```

Remove the temporary `connect.CodeInvalidArgument` special case for `slug is required`; project-wide findings should use a valid project-wide RPC. Keep the generic error marker path for real RPC failures.

- [ ] **Step 3: Verify MCP tests pass**

Run:

```bash
go test ./internal/mcp
```

Expected: PASS.

---

## Task 7: Run Targeted and Full Validation

**Files:**

- No source changes expected unless validation surfaces failures.

- [ ] **Step 1: Run targeted package tests**

Run:

```bash
go test ./internal/server ./internal/mcp
```

Expected: PASS.

- [ ] **Step 2: Run repository quality gate**

Run:

```bash
task check
```

Expected: PASS.

- [ ] **Step 3: Inspect final diff**

Run:

```bash
jj --no-pager diff --git
jj --no-pager status
```

Expected: diff is limited to the proto/generated code, analytical pass handler/tests, MCP resources/tests, this implementation plan, and beads issue state.

---

## Self-Review

- Spec coverage: The plan covers the project-wide RPC, `spec_slug` propagation, server implementation, MCP resource updates, and tests for empty projects, one spec without findings, multiple specs with findings, and the prime regression.
- Placeholder scan: No task uses TBD/TODO/fill-in language.
- Type consistency: The new RPC is consistently named `ListProjectFindings`; request/response types are `ListProjectFindingsRequest` and `ListProjectFindingsResponse`; proto field name is `spec_slug` and generated Go accessor is `GetSpecSlug()`.
