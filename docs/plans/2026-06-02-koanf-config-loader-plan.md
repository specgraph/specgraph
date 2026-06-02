# Koanf Config Loader Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the global config's plain `yaml.Unmarshal` with a koanf loader that gives every field an explicit `flag > env > file > default` precedence stack, centralizing the scattered `os.Getenv` calls.

**Architecture:** koanf is confined to `internal/config` as an edge loader; the typed `GlobalConfig` struct stays the contract for all consumers. Layers load in order (defaults via `structs` → file via `yaml` parser → env via a known-key mapper → set flags via `confmap`), then a single `Unmarshal` with a duration decode hook produces the struct. Flags are injected with `flags.Visit` (only user-set flags), making default-clobbering impossible.

**Tech Stack:** Go, koanf/v2, mapstructure decode hooks, cobra/pflag, jj (commits).

**Design:** `docs/plans/2026-06-02-koanf-config-loader-design.md`

---

## File Structure

- `internal/config/global.go` — **modify.** Rewrite `loadGlobalAt`; add `LoadOption`/`WithFlags`, `envKeyMapper`, `decoderConf`, `applyPostLoad`, `flagKeyMap`. Add `koanf:` tags to the `GlobalConfig` struct family.
- `internal/config/config.go:143` — **modify.** Add `koanf:"url"` tag to `PostgresConfig.URL` (shared type).
- `internal/config/global_test.go` — **modify.** Existing tests are the regression gate; add precedence/env/duration/round-trip tests.
- `cmd/specgraph/serve.go` — **modify.** Register `--listen`; thread flags into the loader; delete the manual `SPECGRAPH_PG_URL` read; add the deprecation warning.
- `cmd/specgraph/main.go:73` — **modify.** `loadGlobalCfg` accepts `...config.LoadOption`.
- `go.mod` / `go.sum` — **modify.** Add koanf deps.
- `docs/verification/*.md`, compose references — **modify.** `SPECGRAPH_PG_URL` → `SPECGRAPH_SERVER_POSTGRES_URL`.

Each task ends with `jj --no-pager commit`. All commit messages MUST include the DCO trailer `Signed-off-by: Sean Brandt <SeBrandt@geico.com>` (repo requires it). Run `jj --no-pager commit -m "<msg>"` — it finalizes the working copy and opens a fresh one.

---

## Task 1: Add koanf dependencies

**Files:**

- Modify: `go.mod`, `go.sum`

- [ ] **Step 1: Add the modules**

Run:

```bash
go get github.com/knadh/koanf/v2@latest \
  github.com/knadh/koanf/providers/structs \
  github.com/knadh/koanf/providers/file \
  github.com/knadh/koanf/providers/env \
  github.com/knadh/koanf/providers/confmap \
  github.com/knadh/koanf/parsers/yaml \
  github.com/go-viper/mapstructure/v2
```

- [ ] **Step 2: Verify build (defer `go mod tidy`)**

Run: `go build ./...`
Expected: no errors. `go.sum` now contains `github.com/knadh/koanf/v2`.

Do **not** run `go mod tidy` yet — no code imports these packages, so tidy would strip them. They land as `// indirect` and become direct in Tasks 2–6; a final `go mod tidy` runs in Task 8. In this corporate environment (`packageregistry.geico.net` proxy) sumdb lookups can 404; if `go get`/`go mod tidy` fails on checksum verification, set `GOFLAGS=-mod=mod` and `GONOSUMCHECK`/`GOSUMDB=off` as needed.

**Env provider version:** pin `github.com/knadh/koanf/providers/env` to **v1.1.0**, NOT `@latest`. The latest tag (`v2.0.0+incompatible`) has a go.mod with no `go` directive, defaults to go1.16, and fails to compile (`any` unsupported). v1.1.0 exposes `Provider(prefix, delim string, cb func(string) string)`, which Task 4 uses. Run `go get github.com/knadh/koanf/providers/env@v1.1.0` explicitly.

- [ ] **Step 3: Confirm the env provider callback signature** (guards against v2 API drift)

Run: `go doc github.com/knadh/koanf/providers/env.Provider`
Expected: signature is `func Provider(prefix, delim string, cb func(string) string) *Env`. If the installed version differs (some lines use `env.ProviderWithValue`), note it — Task 4 Step 3's `envKeyMapper` must match the `func(string) string` callback shape.

- [ ] **Step 4: Commit**

```bash
jj --no-pager commit -m "build(deps): add koanf config loader dependencies

Signed-off-by: Sean Brandt <SeBrandt@geico.com>"
```

---

## Task 2: Add koanf struct tags

koanf `Unmarshal` reads `koanf:` tags (mapstructure-style), not `yaml:`. Tags mirror the existing yaml names exactly so on-disk YAML is unchanged.

**Files:**

- Modify: `internal/config/global.go:26-154`
- Modify: `internal/config/config.go:143-145`

- [ ] **Step 1: Tag the `GlobalConfig` family**

In `internal/config/global.go`, add a `koanf:"..."` tag alongside every `yaml:"..."` tag, using the same key. Apply to: `GlobalConfig`, `ExportConfig`, `ServerSection`, `ProbesConfig`, `ClientConfig`, `Route`, `OIDCConfig`, `JITCreateConfig`, `AuthConfig`, `PolicyConfig`, `APIKeyConfig`, `OIDCProviderConfig`, `ClaimMapping`. Example for `GlobalConfig` and `ClientConfig`:

```go
type GlobalConfig struct {
	Server ServerSection `yaml:"server" koanf:"server"`
	Client ClientConfig  `yaml:"client" koanf:"client"`
	Auth   AuthConfig    `yaml:"auth" koanf:"auth"`
	Export ExportConfig  `yaml:"export" koanf:"export"`
}

type ClientConfig struct {
	DefaultServer string  `yaml:"default_server" koanf:"default_server"`
	Routes        []Route `yaml:"routes,omitempty" koanf:"routes"`
}
```

For `ProbesConfig`, keep the `,omitempty` only on the yaml tag; koanf tags take no `omitempty`:

```go
type ProbesConfig struct {
	Listen   string        `yaml:"listen,omitempty" koanf:"listen"`
	Interval time.Duration `yaml:"interval,omitempty" koanf:"interval"`
	Timeout  time.Duration `yaml:"timeout,omitempty" koanf:"timeout"`
}
```

- [ ] **Step 2: Tag the shared `PostgresConfig`**

In `internal/config/config.go:143`:

```go
type PostgresConfig struct {
	URL string `yaml:"url" koanf:"url"`
}
```

- [ ] **Step 3: Verify build**

Run: `go build ./...`
Expected: no errors. (Behavior unverified until Task 5 round-trip tests; this step only confirms compilation.)

- [ ] **Step 4: Commit**

```bash
jj --no-pager commit -m "refactor(config): add koanf struct tags to GlobalConfig family

Signed-off-by: Sean Brandt <SeBrandt@geico.com>"
```

---

## Task 3: Env-key mapper (fixes the underscore-collision bug)

A naive `_`→`.` mapping mangles keys like `default_server`. Instead, build a lookup from the *known* keys and match incoming env names against it.

`envKeyMapper`/`globalDefaults` are unexported and `internal/config/global_test.go` is `package config_test` (external) — it cannot reach them. These unit tests therefore live in a **new internal test file** `internal/config/loader_internal_test.go` declared `package config`.

**Files:**

- Modify: `internal/config/global.go` (new functions)
- Create: `internal/config/loader_internal_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/config/loader_internal_test.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package config

import (
	"strings"
	"testing"

	"github.com/knadh/koanf/providers/structs"
	"github.com/knadh/koanf/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnvKeyMapper_MapsUnderscoreKeys(t *testing.T) {
	k := koanf.New(".")
	require.NoError(t, k.Load(structs.Provider(globalDefaults(), "koanf"), nil))
	m := envKeyMapper(k)

	// underscore-bearing key must NOT be mangled to client.default.server
	assert.Equal(t, "client.default_server", m("SPECGRAPH_CLIENT_DEFAULT_SERVER"))
	assert.Equal(t, "server.postgres.url", m("SPECGRAPH_SERVER_POSTGRES_URL"))
	assert.Equal(t, "server.listen", m("SPECGRAPH_SERVER_LISTEN"))
	assert.Equal(t, "", m("SPECGRAPH_UNKNOWN_KEY")) // unknown -> ignored
}

func TestEnvKeyMapper_NoEnvFormCollisions(t *testing.T) {
	k := koanf.New(".")
	require.NoError(t, k.Load(structs.Provider(globalDefaults(), "koanf"), nil))
	m := envKeyMapper(k)
	// Every env-settable (scalar) key must round-trip through its env form.
	// A collision would make one key fail to map back to itself.
	for _, key := range k.Keys() {
		if !isEnvSettable(k.Get(key)) {
			continue
		}
		form := "SPECGRAPH_" + strings.ToUpper(strings.ReplaceAll(key, ".", "_"))
		assert.Equal(t, key, m(form), "scalar key %q must round-trip via its env form", key)
	}
}
```

**Collision discovery (resolved):** the deprecated `auth.oidc_providers` and its replacement `auth.oidc.providers` both flatten to env-form `AUTH_OIDC_PROVIDERS`. Both are slice-of-struct, which cannot be expressed as a single env var — so the fix is to exclude non-scalar keys from env mapping (consistent with the design's "env handles scalars only"). `envKeyMapper` (Step 3) filters via `isEnvSettable`. This also means slice keys (`auth.roles`, `client.routes`, `auth.api_keys`, `*.extra_dirs`, `*.email_domain_allowlist`) are not env-settable, by design.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/ -run TestEnvKeyMapper -v`
Expected: FAIL — `envKeyMapper` undefined.

- [ ] **Step 3: Implement `envKeyMapper`**

Add to `internal/config/global.go` (and the koanf/structs imports):

```go
// envKeyMapper returns a callback that maps SPECGRAPH_-prefixed environment
// variable names to dotted koanf keys. It derives the mapping FROM the known
// keys (defaults are loaded first), avoiding the lossy `_`->`.` replacement
// that would mangle keys containing underscores (e.g. client.default_server).
// Only scalar keys participate; slice/map keys cannot be set from a single env
// var and would otherwise collide (e.g. auth.oidc_providers vs auth.oidc.providers).
func envKeyMapper(k *koanf.Koanf) func(string) string {
	lookup := make(map[string]string, len(k.Keys()))
	for _, key := range k.Keys() {
		if !isEnvSettable(k.Get(key)) {
			continue
		}
		lookup[strings.ToUpper(strings.ReplaceAll(key, ".", "_"))] = key
	}
	return func(envName string) string {
		trimmed := strings.TrimPrefix(envName, "SPECGRAPH_")
		return lookup[trimmed] // "" (ignored) when unknown
	}
}

// isEnvSettable reports whether a config value can be set from a single
// environment variable. Only scalars qualify; slices, maps, and nil-valued
// keys are excluded.
func isEnvSettable(v any) bool {
	switch reflect.ValueOf(v).Kind() {
	case reflect.Slice, reflect.Map, reflect.Array, reflect.Invalid:
		return false
	default:
		return true
	}
}
```

Add imports to `global.go`: `"reflect"`, `"strings"`, `"github.com/knadh/koanf/v2"`. (`structs` is imported only by the test file.)

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/config/ -run TestEnvKeyMapper -v`
Expected: PASS (both tests).

- [ ] **Step 5: Commit**

```bash
jj --no-pager commit -m "feat(config): add known-key env mapper for koanf loader

Signed-off-by: Sean Brandt <SeBrandt@geico.com>"
```

---

## Task 4: Rewrite `loadGlobalAt` on koanf

**Files:**

- Modify: `internal/config/global.go` (`loadGlobalAt`, `LoadGlobal`, `LoadGlobalExplicit`, new `decoderConf`, `applyPostLoad`, `flagKeyMap`)
- Modify: `cmd/specgraph/main.go:73` (`loadGlobalCfg` accepts options)

- [ ] **Step 1: Add the flags option type and flag→key map**

Add to `internal/config/global.go`:

```go
// LoadOption configures optional inputs to the global config loader.
type LoadOption func(*loadOptions)

type loadOptions struct {
	flags *pflag.FlagSet
}

// WithFlags supplies a command's flag set so set flags take highest precedence.
func WithFlags(flags *pflag.FlagSet) LoadOption {
	return func(o *loadOptions) { o.flags = flags }
}

// flagKeyMap maps serve-command flag names to dotted koanf keys. Flags absent
// from this map (e.g. cors-origin) are not config keys and are ignored.
var flagKeyMap = map[string]string{
	"listen": "server.listen",
	"pg-url": "server.postgres.url",
}
```

Add import `"github.com/spf13/pflag"`.

- [ ] **Step 2: Add `decoderConf` (duration decode hook)**

```go
// decoderConf wires the mapstructure decode hook so YAML/env duration strings
// (e.g. "5s") decode into time.Duration fields (ProbesConfig.Interval/Timeout).
func decoderConf(out *GlobalConfig) koanf.UnmarshalConf {
	return koanf.UnmarshalConf{
		Tag: "koanf",
		DecoderConfig: &mapstructure.DecoderConfig{
			DecodeHook:       mapstructure.StringToTimeDurationHookFunc(),
			WeaklyTypedInput: true,
			Result:           out,
		},
	}
}
```

Add import `"github.com/go-viper/mapstructure/v2"`.

- [ ] **Step 3: Extract `applyPostLoad`**

Move the existing OIDC migration here, and add the backend coercion (L1):

```go
// applyPostLoad runs transforms that must happen after unmarshal: the OIDC
// providers migration and postgres backend coercion.
func applyPostLoad(cfg *GlobalConfig) {
	if len(cfg.Auth.OIDCProviders) > 0 && len(cfg.Auth.OIDC.Providers) == 0 {
		cfg.Auth.OIDC.Providers = cfg.Auth.OIDCProviders
		slog.Warn("auth.oidc_providers is deprecated; move providers under auth.oidc.providers")
	}
	// A postgres URL supplied by any layer implies the postgres backend.
	if cfg.Server.Postgres.URL != "" && cfg.Server.Backend == "" {
		cfg.Server.Backend = "postgres"
	}
}
```

- [ ] **Step 4: Rewrite `loadGlobalAt` and the public loaders**

Replace the existing `loadGlobalAt`, `LoadGlobal`, `LoadGlobalExplicit`:

```go
func LoadGlobal(path string, opts ...LoadOption) (*GlobalConfig, error) {
	return loadGlobalAt(path, true, opts...)
}

func LoadGlobalExplicit(path string, opts ...LoadOption) (*GlobalConfig, error) {
	return loadGlobalAt(path, false, opts...)
}

func loadGlobalAt(path string, materializeDefaults bool, opts ...LoadOption) (*GlobalConfig, error) {
	var o loadOptions
	for _, opt := range opts {
		opt(&o)
	}

	k := koanf.New(".")

	// 1. defaults — single source of truth (shared with writeGlobal).
	if err := k.Load(structs.Provider(globalDefaults(), "koanf"), nil); err != nil {
		return nil, fmt.Errorf("load defaults: %w", err)
	}

	// 2. file — materialize defaults or fail loudly when absent.
	if _, statErr := os.Stat(path); statErr != nil {
		if !errors.Is(statErr, fs.ErrNotExist) {
			return nil, fmt.Errorf("read config: %w", statErr)
		}
		if !materializeDefaults {
			return nil, fmt.Errorf("config file not found at %s", path)
		}
		if writeErr := writeGlobal(path, globalDefaults()); writeErr != nil {
			return nil, fmt.Errorf("write default config: %w", writeErr)
		}
	} else if err := k.Load(file.Provider(path), yaml.Parser()); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	// 3. env — SPECGRAPH_* via known-key mapper.
	// NOTE: env provider is pinned to v1.1.0 (see Task 1 note). The v2.0.0
	// (+incompatible) tag ships a go.mod with no `go` directive, so it defaults
	// to go1.16 and rejects `any` — unbuildable. v1.1.0's API is
	// env.Provider(prefix, delim string, cb func(string) string), which matches
	// envKeyMapper's shape directly (returns "" to drop unrecognized vars).
	mapper := envKeyMapper(k)
	if err := k.Load(env.Provider("SPECGRAPH_", ".", mapper), nil); err != nil {
		return nil, fmt.Errorf("load env: %w", err)
	}

	// 4. set flags only (flags.Visit skips unset flags -> no default clobbering).
	if o.flags != nil {
		overrides := map[string]interface{}{}
		o.flags.Visit(func(f *pflag.Flag) {
			if key, ok := flagKeyMap[f.Name]; ok {
				overrides[key] = f.Value.String()
			}
		})
		if len(overrides) > 0 {
			if err := k.Load(confmap.Provider(overrides, "."), nil); err != nil {
				return nil, fmt.Errorf("load flags: %w", err)
			}
		}
	}

	var cfg GlobalConfig
	if err := k.UnmarshalWithConf("", &cfg, decoderConf(&cfg)); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	applyPostLoad(&cfg)
	return &cfg, nil
}
```

Add imports: `"github.com/knadh/koanf/providers/file"`, `"github.com/knadh/koanf/providers/env"`, `"github.com/knadh/koanf/providers/confmap"`, `"github.com/knadh/koanf/parsers/yaml"`. Remove the now-unused top-level `"gopkg.in/yaml.v3"` import **only if** `writeGlobal` no longer uses it — it does (`yaml.Marshal`), so keep `gopkg.in/yaml.v3` and alias the koanf parser import as `koanfyaml "github.com/knadh/koanf/parsers/yaml"`, using `koanfyaml.Parser()` in the file load.

- [ ] **Step 5: `loadGlobalCfg` accepts options**

In `cmd/specgraph/main.go:73`:

```go
func loadGlobalCfg(opts ...config.LoadOption) (*config.GlobalConfig, error) {
	if cfgFile != "" {
		return config.LoadGlobalExplicit(cfgFile, opts...)
	}
	return config.LoadGlobal(xdg.ConfigFile(), opts...)
}
```

- [ ] **Step 6: Run the existing config tests (regression gate)**

Run: `go test ./internal/config/ -v`
Expected: PASS — including `TestLoadGlobal_Defaults`, `TestLoadGlobal_ExistingFile`, `TestLoadGlobal_MalformedYAML` (still "parse config"), `TestLoadGlobalExplicit_ErrorsWhenMissing`, `TestLoadGlobal_ReadOnlyParentDir` ("write default config"), `TestLoadGlobal_ProbesYAML` (duration decode), `TestLoadGlobal_OIDCJITConfig`, `TestLoadGlobal_LegacyOIDCProvidersStillWorks`, `TestLoadGlobal_AuthConfig`.

If `TestLoadGlobal_ProbesYAML` fails on duration decode, confirm the decode hook is wired (Step 2). If any slice test fails, recheck koanf tags (Task 2).

- [ ] **Step 7: Build the whole tree**

Run: `go build ./...`
Expected: no errors (confirms `main.go` and `serve.go` still compile against the new signatures).

- [ ] **Step 8: Commit**

```bash
jj --no-pager commit -m "refactor(config): load global config via koanf with explicit precedence

Signed-off-by: Sean Brandt <SeBrandt@geico.com>"
```

---

## Task 5: Precedence, env, and round-trip tests

These tests use only exported APIs, so they go in the existing external-package file `internal/config/global_test.go` (`package config_test`) — every loader call is qualified with `config.`, and expected defaults use the known literals (not `globalDefaults()`, which is unexported).

**Files:**

- Modify: `internal/config/global_test.go` (add `"github.com/spf13/pflag"` to imports)

- [ ] **Step 1: Write the failing tests**

```go
func TestLoadGlobal_EnvOverridesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte("server:\n  listen: \"0.0.0.0:1111\"\n"), 0o600))
	t.Setenv("SPECGRAPH_SERVER_LISTEN", "0.0.0.0:2222")

	cfg, err := config.LoadGlobalExplicit(path)
	require.NoError(t, err)
	assert.Equal(t, "0.0.0.0:2222", cfg.Server.Listen) // env beats file
}

func TestLoadGlobal_SetFlagBeatsEnvAndFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte("server:\n  listen: \"0.0.0.0:1111\"\n"), 0o600))
	t.Setenv("SPECGRAPH_SERVER_LISTEN", "0.0.0.0:2222")

	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	fs.String("listen", "", "")
	require.NoError(t, fs.Parse([]string{"--listen", "0.0.0.0:3333"}))

	cfg, err := config.LoadGlobalExplicit(path, config.WithFlags(fs))
	require.NoError(t, err)
	assert.Equal(t, "0.0.0.0:3333", cfg.Server.Listen) // set flag wins
}

func TestLoadGlobal_UnsetFlagDoesNotClobber(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte("server:\n  listen: \"0.0.0.0:1111\"\n"), 0o600))

	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	fs.String("listen", "0.0.0.0:9999", "") // non-empty DEFAULT, not set on cmdline
	require.NoError(t, fs.Parse([]string{}))

	cfg, err := config.LoadGlobalExplicit(path, config.WithFlags(fs))
	require.NoError(t, err)
	assert.Equal(t, "0.0.0.0:1111", cfg.Server.Listen) // file wins; flag default ignored
}

func TestLoadGlobal_PgURLCoercesBackend(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte("server:\n  backend: \"\"\n"), 0o600))
	t.Setenv("SPECGRAPH_SERVER_POSTGRES_URL", "postgres://x/y")

	cfg, err := config.LoadGlobalExplicit(path)
	require.NoError(t, err)
	assert.Equal(t, "postgres://x/y", cfg.Server.Postgres.URL)
	assert.Equal(t, "postgres", cfg.Server.Backend) // coerced
}

func TestLoadGlobal_SliceRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	doc := "auth:\n  oidc:\n    providers:\n      - id: p1\n        client_id: cid\n        claims_mapping:\n          - claim: groups\n            value: admins\n            role: admin\n"
	require.NoError(t, os.WriteFile(path, []byte(doc), 0o600))

	cfg, err := config.LoadGlobalExplicit(path)
	require.NoError(t, err)
	require.Len(t, cfg.Auth.OIDC.Providers, 1)
	assert.Equal(t, "p1", cfg.Auth.OIDC.Providers[0].ID)
	assert.Equal(t, "cid", cfg.Auth.OIDC.Providers[0].ClientID)
	require.Len(t, cfg.Auth.OIDC.Providers[0].ClaimsMapping, 1)
	assert.Equal(t, "admin", cfg.Auth.OIDC.Providers[0].ClaimsMapping[0].Role)
}

func TestLoadGlobal_MaterializedFileMatchesDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	cfg, err := config.LoadGlobal(path) // missing -> materialize
	require.NoError(t, err)
	assert.Equal(t, "0.0.0.0:9090", cfg.Server.Listen) // matches globalDefaults()

	reread, err := config.LoadGlobalExplicit(path) // now exists
	require.NoError(t, err)
	assert.Equal(t, "postgres://specgraph:specgraph@localhost:5432/specgraph?sslmode=disable",
		reread.Server.Postgres.URL)
}
```

- [ ] **Step 2: Run to verify they fail (or pass) meaningfully**

Run: `go test ./internal/config/ -run TestLoadGlobal -v`
Expected: the new tests exercise behavior added in Task 4. If `TestLoadGlobal_UnsetFlagDoesNotClobber` fails, the `flags.Visit` mechanism is wrong (you used `VisitAll` or posflag defaults). If `TestLoadGlobal_SliceRoundTrip` fails, koanf tags on the OIDC/ClaimMapping structs are missing.

- [ ] **Step 3: Fix any failures inline, then re-run the full package**

Run: `go test ./internal/config/ -v`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
jj --no-pager commit -m "test(config): precedence, env, duration, and round-trip coverage

Signed-off-by: Sean Brandt <SeBrandt@geico.com>"
```

---

## Task 6: Wire serve flags + deprecation warning

**Files:**

- Modify: `cmd/specgraph/serve.go:54-83`

- [ ] **Step 1: Register `--listen` and pass flags to the loader; replace the manual pg-url block**

In `init()` (`serve.go:54`), add the listen flag:

```go
serveCmd.Flags().String("cors-origin", "", "Enable CORS for this origin (dev mode only)")
serveCmd.Flags().String("pg-url", "", "PostgreSQL connection URL (overrides config; env: SPECGRAPH_SERVER_POSTGRES_URL)")
serveCmd.Flags().String("listen", "", "Address to listen on (overrides config; env: SPECGRAPH_SERVER_LISTEN)")
```

In `runServe`, replace the load + manual pg-url/env block (`serve.go:61-83`) with:

```go
func runServe(cmd *cobra.Command, _ []string) error {
	if os.Getenv("SPECGRAPH_PG_URL") != "" {
		slog.Warn("SPECGRAPH_PG_URL is no longer read; use SPECGRAPH_SERVER_POSTGRES_URL")
	}

	cfg, err := loadGlobalCfg(config.WithFlags(cmd.Flags()))
	if err != nil {
		return fmt.Errorf("load global config: %w", err)
	}

	ctx, cancel := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// --pg-url / SPECGRAPH_SERVER_POSTGRES_URL and backend coercion are now
	// resolved inside the loader (cfg.Server.Postgres.URL, cfg.Server.Backend).
```

Delete the old `pgURL, err := cmd.Flags().GetString("pg-url")` ... `cfg.Server.Postgres.URL = pgURL` lines. Confirm nothing later in `runServe` references a local `pgURL`:

Run: `grep -n "pgURL" cmd/specgraph/serve.go`
Expected: no matches after the edit.

Add imports to `serve.go` if missing: `"log/slog"`, and the local config package (already imported as `config`). Remove `"os"` only if no other use remains — `os.Getenv` and `os.Stderr` are still used, so keep it.

- [ ] **Step 2: Verify the listen flag is actually consumed**

Confirm the listener uses `cfg.Server.Listen` (it does — `serve.go:298`/`addr := cfg.Server.Listen`). No change needed; the flag now flows through the loader into that field.

- [ ] **Step 3: Build and run serve unit tests**

Run: `go build ./... && go test ./cmd/specgraph/ -v`
Expected: PASS.

- [ ] **Step 4: Manual smoke (no DB needed — expect the no-URL error path or help)**

Run: `go run ./cmd/specgraph serve --listen 0.0.0.0:9099 --help`
Expected: help text shows `--listen` and `--pg-url` flags.

- [ ] **Step 5: Commit**

```bash
jj --no-pager commit -m "feat(serve): bind --listen/--pg-url via koanf; deprecate SPECGRAPH_PG_URL

Signed-off-by: Sean Brandt <SeBrandt@geico.com>"
```

---

## Task 7: Docs and migration references

**Files:**

- Modify: `docs/verification/claude.md`, `docs/verification/cursor.md`, `docs/verification/opencode.md` (only `SPECGRAPH_PG_URL` references, if any)
- Modify: any compose/k8s file setting `SPECGRAPH_PG_URL`

- [ ] **Step 1: Find every non-test `SPECGRAPH_PG_URL` reference**

Run: `grep -rn "SPECGRAPH_PG_URL" --include="*.md" --include="*.yaml" --include="*.yml" . | grep -v "docs/plans/2026-06-02-koanf"`
Expected: a list to update. (The deprecation-warn string in `serve.go` intentionally keeps the old name — do not change that.)

- [ ] **Step 2: Update each reference**

Replace `SPECGRAPH_PG_URL` with `SPECGRAPH_SERVER_POSTGRES_URL` in docs and compose/env files. Leave `SPECGRAPH_API_KEY` untouched everywhere (out of scope by design).

- [ ] **Step 3: Add a CHANGELOG/release note entry**

If a `CHANGELOG.md` exists, add under Unreleased:

```text
- **BREAKING:** server env var `SPECGRAPH_PG_URL` renamed to `SPECGRAPH_SERVER_POSTGRES_URL`. All global config fields are now env-overridable via `SPECGRAPH_<SECTION>_<KEY>`.
```

If no CHANGELOG exists, skip (note it in the PR description instead).

- [ ] **Step 4: Commit**

```bash
jj --no-pager commit -m "docs: rename SPECGRAPH_PG_URL to SPECGRAPH_SERVER_POSTGRES_URL

Signed-off-by: Sean Brandt <SeBrandt@geico.com>"
```

---

## Task 8: Full quality gate

- [ ] **Step 1: Tidy modules + license headers**

Run: `go mod tidy && go build ./...`
Expected: koanf deps move from `// indirect` to direct require; build passes. (If sumdb 404s, see Task 1 Step 2's proxy note.)

Run: `task license:check`
Expected: PASS. The new `internal/config/loader_internal_test.go` must carry the SPDX header (it does per Task 3). If anything is flagged, run `task license:add`.

- [ ] **Step 2: Run the full check**

Run: `task check`
Expected: fmt:check → license:check → lint → build → unit tests all PASS. Watch for `revive` (no new packages added, so no package-comment requirement) and `wrapcheck` (all koanf errors are wrapped with `fmt.Errorf("...: %w")` — confirm).

- [ ] **Step 3: Integration/e2e (Docker)**

Run: `task pr-prep`
Expected: PASS. The server still binds `cfg.Server.Listen`; e2e config loading exercises the koanf path.

- [ ] **Step 4: Final commit if `task check` reformatted anything**

```bash
jj --no-pager commit -m "chore: formatting from task check

Signed-off-by: Sean Brandt <SeBrandt@geico.com>"
```

(Skip if the working copy is clean.)

---

## Self-Review Notes (spec coverage)

- C1 (env underscore collision) → Task 3 + `TestEnvKeyMapper_*`.
- C2/M1 (no API-key rename) → Task 7 Step 2 explicitly leaves `SPECGRAPH_API_KEY`; no struct field added.
- H1 (flag default clobbering) → `flags.Visit` (Task 4 Step 4) + `TestLoadGlobal_UnsetFlagDoesNotClobber`.
- H2 (duration/slice decode) → `decoderConf` (Task 4 Step 2) + `TestLoadGlobal_ProbesYAML`/`TestLoadGlobal_SliceRoundTrip`.
- H3 (single default source + materialize) → `globalDefaults()` reused (Task 4 Step 4) + `TestLoadGlobal_MaterializedFileMatchesDefaults`.
- M2 (error strings) → `parse config` preserved (Task 4 Step 4) + existing `TestLoadGlobal_MalformedYAML`.
- L1 (backend coercion) → `applyPostLoad` (Task 4 Step 3) + `TestLoadGlobal_PgURLCoercesBackend`.
- L2 (serve-only flags) → flags registered only on `serveCmd` (Task 6).
- L3 (deprecation warn scope) → emitted in `runServe` only (Task 6 Step 1).
