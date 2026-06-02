# Koanf Config Loader — Global Config Layering

**Date:** 2026-06-02
**Status:** Design (revised after adversarial review)
**Scope:** Server + client global config (`~/.config/specgraph/config.yaml` / `GlobalConfig`)

## Problem

Global config is loaded by a plain `yaml.Unmarshal` into typed structs
(`internal/config/global.go`). Environment-variable support is an ad-hoc set of
six hand-written `os.Getenv` calls scattered across `cmd/` and `internal/`:

| Env var | Read site | Concern |
|---------|-----------|---------|
| `SPECGRAPH_API_KEY` | `cmd/specgraph/client.go:100` | client bearer token (also a public MCP contract) |
| `SPECGRAPH_PG_URL` | `cmd/specgraph/serve.go:76` | server postgres URL |
| `SPECGRAPH_DRIFT_NUDGE` | `cmd/specgraph/nudge.go:63` | behavioral toggle |
| `SPECGRAPH_FETCH_GITHUB_TOKEN` | `internal/constitution/fetch/fetch.go:128` | secret |
| `SPECGRAPH_DEV_SOURCE_ROOT` | `internal/config/managedfiles/source_dev.go:20` | dev build seam |
| `SPECGRAPH_E2E_COVERDIR` | `e2e/testutil/cli.go:41` | test-only |

This produces three problems:

1. **Inconsistency** — only the fields that happen to have an `os.Getenv` call
   are env-overridable. The rest of the config (e.g. `server.listen`,
   `server.mode`) cannot be set by env at all.
2. **No centralization** — config precedence is implicit and spread across the
   codebase; there is no single place that owns "flag beats env beats file beats
   default."
3. **Operability** — when deploying the server (containers / k8s), editing a
   mounted YAML file is more painful than setting env vars, but env-driven
   config is not a first-class citizen.

## Goal

Introduce a principled config layer for the **entire global config** with an
explicit precedence stack (flag > env > file > default), centralized in one
loader, using **koanf**. Make every global-config field env-overridable. Keep
the typed `GlobalConfig` struct as the contract consumed by the rest of the
codebase.

### Non-goals

- The project config (`.specgraph.yaml` / `Config` in `internal/config/config.go`)
  is **out of scope**. Its semantics (per-repo, checked-in-adjacent) differ and
  warrant a separate decision.
- **Four env vars stay as-is, by design** — they are not "structured config
  randomness" but a different category: a secret
  (`SPECGRAPH_FETCH_GITHUB_TOKEN`), a behavioral toggle
  (`SPECGRAPH_DRIFT_NUDGE`), a dev build seam (`SPECGRAPH_DEV_SOURCE_ROOT`), and a
  test seam (`SPECGRAPH_E2E_COVERDIR`).
- **`SPECGRAPH_API_KEY` is NOT renamed and NOT folded into the koanf struct**
  (see "API key: explicitly excluded").

## Library choice — koanf (not Viper)

The request originally named Viper; koanf was chosen instead for consistency
with the author's other codebases and because it is a better structural fit:

- **Instance-based, no global singleton.** koanf cannot tempt the "live global
  registry" anti-pattern (config reads sprinkled through handlers). Every loader
  builds its own `*koanf.Koanf`.
- **Explicit precedence.** Precedence is literally the order of `k.Load()` calls,
  last-wins, readable in one place.
- **Cobra/pflag binding retained.** `posflag.Provider` binds a `pflag.FlagSet`,
  so the flags tier still works despite Cobra already being a dependency.

Trade-off accepted: we forgo Viper's native Cobra integration, but
`posflag.Provider` covers it. (`spf13/pflag` and `spf13/cast`, which posflag
needs, are already indirect deps in `go.mod` — promotion to direct is trivial.)

## Approach — koanf at the edge, typed struct preserved

koanf is confined to the loader. Nothing outside `internal/config` touches it.

```go
func loadGlobalAt(path string, materializeDefaults bool, flags *pflag.FlagSet) (*GlobalConfig, error) {
    k := koanf.New(".")

    // 1. defaults — single source of truth, shared with writeGlobal
    _ = k.Load(structs.Provider(globalDefaults(), "koanf"), nil)

    // 2. file — skip+materialize when absent (see "Preserved behaviors")
    if err := k.Load(file.Provider(path), yaml.Parser()); err != nil {
        if !errors.Is(err, fs.ErrNotExist) {
            return nil, fmt.Errorf("parse config: %w", err) // preserve existing error prefix
        }
        if !materializeDefaults {
            return nil, fmt.Errorf("config file not found at %s", path)
        }
        if writeErr := writeGlobal(path, globalDefaults()); writeErr != nil {
            return nil, fmt.Errorf("write default config: %w", writeErr)
        }
        // continue: defaults + env + flags still resolve
    }

    // 3. env — mapped via known-key env-form lookup (see "Env mapping")
    _ = k.Load(env.Provider("SPECGRAPH_", ".", envKeyMapper(k)), nil)

    // 4. flags — highest precedence, but only for flags the user actually set
    if flags != nil {
        _ = k.Load(posflag.Provider(flags, ".", k), nil)
    }

    var cfg GlobalConfig
    if err := k.UnmarshalWithConf("", &cfg, decoderConf()); err != nil {
        return nil, fmt.Errorf("unmarshal config: %w", err)
    }

    applyPostLoad(&cfg) // OIDC migration + ProbesConfig validation
    return &cfg, nil
}
```

The typed `GlobalConfig` struct remains the contract. `serve.go` and handlers
keep consuming it. The `SPECGRAPH_PG_URL` `os.Getenv` call in `serve.go` is
deleted; its value now arrives through `cfg.Server.Postgres.URL`.

### Decoder config (mapstructure decode hooks) — fixes H2

koanf `Unmarshal` uses mapstructure, which does **not** decode YAML duration
strings (`interval: 5s`) into `time.Duration` by default. `ProbesConfig.Interval`
and `ProbesConfig.Timeout` (`global.go:55-56`) require an explicit hook:

```go
func decoderConf() koanf.UnmarshalConf {
    return koanf.UnmarshalConf{
        Tag: "koanf",
        DecoderConfig: &mapstructure.DecoderConfig{
            DecodeHook: mapstructure.ComposeDecodeHookFunc(
                mapstructure.StringToTimeDurationHookFunc(),
            ),
            WeaklyTypedInput: true,
            Result:           nil, // set by UnmarshalWithConf
        },
    }
}
```

### Tagging cost

koanf unmarshals via mapstructure-style `koanf:"..."` tags. We add them
alongside the existing `yaml:"..."` tags across the `GlobalConfig` struct family,
**including every slice-of-struct and nested type**: `APIKeyConfig`, `RoleConfig`,
`OIDCConfig`/`OIDCProviderConfig`, `ClaimMapping`, `Route`, and the JIT/policy
sub-structs. Round-trip tests (below) guard these — the tagging is mechanical but
**must not be assumed correct**; mapstructure squashing/embedding rules differ
from yaml.v3.

## Env mapping — fixes C1 (the underscore-collision bug)

A naive `_`→`.` replacement is **wrong** for this struct family: 11 keys contain
intra-segment underscores (`default_server`, `signing_key`, `client_id`,
`oidc_providers`, `default_role`, `api_keys`, `claims_mapping`, `extra_dirs`,
`jit_create`, `rate_limit_per_hour`, `email_domain_allowlist`). `SPECGRAPH_CLIENT_DEFAULT_SERVER`
would mangle to `client.default.server` instead of `client.default_server`.

Instead, `envKeyMapper` builds a lookup **from the known keys** (defaults are
loaded first, so `k.Keys()` enumerates every valid dotted path). For each known
key it computes the unambiguous env form by replacing `.` with `_` and
uppercasing (`client.default_server` → `CLIENT_DEFAULT_SERVER`), then maps the
incoming env var back:

```go
func envKeyMapper(k *koanf.Koanf) func(string) string {
    lookup := map[string]string{} // ENV_FORM -> dotted.key
    for _, key := range k.Keys() {
        lookup[strings.ToUpper(strings.ReplaceAll(key, ".", "_"))] = key
    }
    return func(envName string) string {
        trimmed := strings.TrimPrefix(envName, "SPECGRAPH_")
        if dotted, ok := lookup[trimmed]; ok {
            return dotted
        }
        return "" // unknown -> ignored
    }
}
```

This yields automatic, collision-free env-settability for every scalar key
(`SPECGRAPH_SERVER_LISTEN`, `SPECGRAPH_SERVER_POSTGRES_URL`,
`SPECGRAPH_CLIENT_DEFAULT_SERVER`, …). Slice/map keys (`auth.api_keys`) are not
env-settable; that is acceptable. (Distinct dotted keys cannot collide in the
env form unless one key embeds another's separator pattern — none of the current
keys do; a unit test asserts the lookup has no duplicate values.)

### The one clean-break rename

| Removed | Replacement | Maps to |
|---------|-------------|---------|
| `SPECGRAPH_PG_URL` | `SPECGRAPH_SERVER_POSTGRES_URL` | existing `server.postgres.url` |

This is the only env-var rename. A startup deprecation warning (below) covers it.

## API key: explicitly excluded — fixes C2 / M1

`SPECGRAPH_API_KEY` is **not** migrated. It is a public, documented contract:

- `specgraph init` writes `Bearer ${SPECGRAPH_API_KEY}` (and harness variants)
  into `.mcp.json`, `.cursor/mcp.json`, `opencode.json` via
  `internal/config/managedfiles/manifest.go:42,63,93` (with golden fixtures).
- It is documented in `docs/verification/{claude,cursor,opencode}.md` and the
  embedded `specgraph-troubleshooting/SKILL.md`.
- The **harness** (Claude/Cursor/OpenCode) resolves this env var itself to build
  the `Authorization` header — that path never goes through the CLI config
  loader, so renaming the CLI var would force users to set two names.

The existing CLI resolution stays exactly as-is: `resolveAPIKey()`
(`client.go:99`) keeps its `SPECGRAPH_API_KEY` env > credentials-file
(`auth.ReadDefaultKey`, `client.go:103`) precedence. We do **not** add an
`api_key` field to `ClientConfig`, so no secret is ever serialized into
`config.yaml` by `writeGlobal`.

## Flags tier — fixes H1 / L1 / L2

Bind two pflags on the **`serve` command only** (not `up` — see below):

- `--listen` → `server.listen`
- `--pg-url` → `server.postgres.url`

(`--mode` is dropped from the initial set as YAGNI; add later if needed.)

**posflag default discipline (H1):** the new flags MUST be registered with
**zero-value defaults** (`""`). Defaults live exclusively in the
`structs.Provider(globalDefaults())` tier. Passing `k` as the third arg to
`posflag.Provider` makes koanf fall back to the existing koanf value for flags
the user did not set, so an omitted flag does not clobber env/file. A precedence
test asserts: file/env sets `server.listen`, flag omitted → file/env value wins.

**Backend coercion (L1):** today `--pg-url` also sets `cfg.Server.Backend =
"postgres"` (`serve.go:79-81`). The koanf path reproduces this in `applyPostLoad`:
if `server.postgres.url` is set and backend is unset, default backend to
`postgres`.

**Scope to `serve` (L2):** `up` reads `cfg.Client.DefaultServer` for its health
check and does not bind these flags; adding `--listen` to `up` would be
semantically confusing. Flags are `serve`-only.

The existing `--config` flag stays special: it selects the *file path* before
the loader runs, so it is not part of the koanf layer.

## Preserved loader behaviors

koanf changes *how* config is read, not *what* the loader guarantees:

1. **Auto-create vs. fail-loudly.** `LoadGlobal` (default XDG path) still
   materializes a default file when absent; `LoadGlobalExplicit` (operator
   `--config`) still errors on a missing/typo'd path.
2. **OIDC migration warning.** The post-load transform
   (`auth.oidc_providers` → `auth.oidc.providers` with `slog.Warn`) runs in
   `applyPostLoad`, after `Unmarshal`, unchanged.
3. **Validation.** `ProbesConfig.Resolved()` stays a post-load step. Defaults
   come from the `structs.Provider(globalDefaults())` layer.
4. **Error-message contract (M2).** File-parse failures keep the `"parse config"`
   prefix so `TestLoadGlobal_MalformedYAML` (`global_test.go:84`) and
   `TestLoadGlobal_ReadOnlyParentDir` (`"write default config"`) assertions still
   hold.

### Defaults: single source of truth (H3.1)

`globalDefaults()` remains the **only** producer of defaults, feeding *both* the
`structs.Provider` layer and `writeGlobal`. A test asserts a freshly materialized
file round-trips back to `globalDefaults()` to prevent drift between the two
consumers.

### Materialize-on-env-only is pre-existing, not a regression (H3.2)

When the file is absent but config is fully supplied via env, `LoadGlobal` still
writes a default `config.yaml` (containing the dev-default Postgres URL). **This
is existing behavior** — today's loader also writes the default file on a missing
path before `serve.go` applies the env override. The koanf change preserves it
rather than introducing it. Decoupling materialization from load (making it an
`init`-time-only concern) is noted as a **possible follow-up**, out of scope here.

## Testing

- **Loader unit tests (table-driven):**
  - precedence: flag (set) > env > file > default for representative keys
  - **omitted flag does not clobber** env/file (the H1 guard)
  - env mapping: `SPECGRAPH_CLIENT_DEFAULT_SERVER` → `client.default_server`
    (the C1 guard — an underscore-bearing key, not just `server.listen`)
  - env-form lookup has no duplicate target keys (collision guard)
  - duration decode: `interval: 5s` decodes into `ProbesConfig.Interval` (H2 guard)
  - round-trip for every slice/nested type: `APIKeys`, `OIDC.Providers` +
    `ClaimsMapping`, `Routes` (H2 guard)
  - auto-create vs. fail-loudly both still hold
  - materialized file round-trips to `globalDefaults()` (H3.1 guard)
  - OIDC migration still warns and migrates
  - malformed YAML still surfaces `"parse config"` (M2 guard)
- **No global state** to reset between tests (koanf is instance-based). `t.Setenv`
  is confined to the env-provider tests.
- `go build ./...` and `task check` gate the `serve.go` caller update.

## Migration / breaking-change communication

Only `SPECGRAPH_PG_URL` is removed (renamed to `SPECGRAPH_SERVER_POSTGRES_URL`):

- Release-note / CHANGELOG entry with the rename.
- Update compose/k8s references that set `SPECGRAPH_PG_URL`.
- **Deprecation safety net (L3):** in the **`serve` command path only**, if
  `SPECGRAPH_PG_URL` is present in the environment, emit one `slog.Warn`
  ("`SPECGRAPH_PG_URL` is no longer read; use `SPECGRAPH_SERVER_POSTGRES_URL`").
  Scoped to server commands to avoid noise on every CLI invocation.

## New dependencies

- `github.com/knadh/koanf/v2`
- providers: `koanf/providers/structs`, `koanf/providers/file`,
  `koanf/providers/env`, `koanf/providers/posflag`
- parser: `koanf/parsers/yaml`
- `github.com/go-viper/mapstructure/v2` (decode hooks) — or koanf's bundled
  mapstructure, whichever the koanf v2 line vendors.

## Resolved review findings

- **C1** env underscore collision → known-key env-form lookup.
- **C2 / M1** `SPECGRAPH_API_KEY` rename dropped; key stays out of the struct.
- **H1** posflag default clobbering → zero-default flags + koanf fallback + test.
- **H2** duration/slice decode → explicit decode hook + round-trip tests.
- **H3** defaults double-source → single `globalDefaults()` + round-trip test;
  materialize-on-env-only documented as pre-existing.
- **M2** error-string contract → preserve `"parse config"`.
- **M3** dissolved (no api_key plumbing; only `serve.go` PG-URL read changes).
- **L1** backend coercion reproduced in `applyPostLoad`.
- **L2** flags scoped to `serve`.
- **L3** deprecation warn scoped to server commands.

## Open questions

- Should the `--mode` flag be included now, or deferred (design defers it)?
- Confirm the koanf v2 mapstructure import path used elsewhere in your codebases,
  for consistency.
