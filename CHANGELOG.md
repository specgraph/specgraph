# Changelog

> **Note:** v0.7.0 and earlier are documented in the historical block below.
> v0.8.0 through v0.13.0 were published on
> [GitHub Releases](https://github.com/specgraph/specgraph/releases) only and
> will not be backfilled here — expect a visible v0.7.0 → v0.14.0 gap below.
> Starting with v0.14.0, this file is bot-maintained by
> [release-please](https://github.com/googleapis/release-please) on every
> release.

All notable changes to this project will be documented in this file.
## [0.7.0](https://github.com/specgraph/specgraph/compare/v0.6.0...v0.7.0) - 2026-06-05

### Bug Fixes

- **ci:** Switch buf to local plugins to eliminate BSR rate-limit flakes (spgr-3zs0) (#977) ([#977](https://github.com/specgraph/specgraph/pull/977)) ([b2ffc36](https://github.com/specgraph/specgraph/commit/b2ffc367433d33abeaf3fa76e68684b30b90bf17))
- **auth:** Fail-closed role-downgrade clamp for custom/unranked roles (spgr-rjrt.9) (#974) ([#974](https://github.com/specgraph/specgraph/pull/974)) ([934f623](https://github.com/specgraph/specgraph/commit/934f623e85dfcc06ea8be9a5231e14f1aac48e8e))
- **bootstrap:** Recover a keyless bootstrap admin by minting a key on Ensure (spgr-rjrt.12) (#973) ([#973](https://github.com/specgraph/specgraph/pull/973)) ([a9380ec](https://github.com/specgraph/specgraph/commit/a9380ec4fcf50d2c84b83dd638ccd5ae88ec2b58))

### Features

- **identity:** [**breaking**] Api-key hardening — limit clamp, --expires-at, rotate contract fix, loose-creds warning (spgr-rjrt.7/.10/.11) (#971) ([a63ddd7](https://github.com/specgraph/specgraph/commit/a63ddd7639f61b7691127cc15819bbbba4acbbfe))
- **identity:** Bootstrap & UX — IdentityService RPC/CLI + bootstrap flows + protections (spgr-rjrt.5+.6) (#970) ([1a49b42](https://github.com/specgraph/specgraph/commit/1a49b42b6a28c03956d43c828a44737647da3580))

### Tests

- **authn:** OIDC coverage — full-stack interceptor, CLI oidc cmds, verifier branches (#976) ([#976](https://github.com/specgraph/specgraph/pull/976)) ([db715db](https://github.com/specgraph/specgraph/commit/db715dba32cba96b6e2183594c27ed29977421bd))
- **authn:** JWT/JIT end-to-end integration coverage for identity authn (spgr-rjrt.8) (#972) ([#972](https://github.com/specgraph/specgraph/pull/972)) ([9d739bb](https://github.com/specgraph/specgraph/commit/9d739bb5ff8f6857f2cfe6b080177f35bdbc98a8))

## [0.6.0](https://github.com/specgraph/specgraph/compare/v0.5.0...v0.6.0) - 2026-06-02

### Bug Fixes

- **mcp:** Pass conversation_exchanges through author tool (#952) ([c8a8a2b](https://github.com/specgraph/specgraph/commit/c8a8a2bf7897b3e11e1c2c39ed9d04d2c2419c04))
- Broken claude manifest setup (#949) ([f67d459](https://github.com/specgraph/specgraph/commit/f67d459b3a98450d28d45f0b3530a98cb9dddeed))
- **plugin/opencode:** Match author tool structurally; add E2E smoke procedure (#941) ([f68ebae](https://github.com/specgraph/specgraph/commit/f68ebae2651b8d77d9ce1c50ff011166097ea0af))
- **mcp:** Render constitution resource as markdown (#934) ([60dcd37](https://github.com/specgraph/specgraph/commit/60dcd37974cc6a6cd2391729d66296b863714cc5))
- **mcp:** Render constitution empty state nicely (#933) ([4f74281](https://github.com/specgraph/specgraph/commit/4f742811aa79308ea5a41a5f56b2002b5b00686f))
- **mcp:** Add project-wide findings API (#932) ([0a78a46](https://github.com/specgraph/specgraph/commit/0a78a4662b1f4446fb89b5f17d31133a53b0a316))
- **mcp:** Friendlier empty-state rendering in prime resource (#931) ([c784cc8](https://github.com/specgraph/specgraph/commit/c784cc8fe71d76db36b0a92d9c4e370db352a870))
- Change default server.listen to 0.0.0.0:9090 (#915) ([c3fed3b](https://github.com/specgraph/specgraph/commit/c3fed3be418de0a9683bea6b8ea57dc927b929c7))
- **cli:** Honor --config flag on server commands (#914) ([e8a3642](https://github.com/specgraph/specgraph/commit/e8a3642d8ac60eeb8743c647ea73a830d6bea9ca))
- **skills:** Make conversation recording structurally inseparable from stage persistence (#894) ([4b1de11](https://github.com/specgraph/specgraph/commit/4b1de119162820647686b9bffcdd16310da14600))
- Amend re-entry lands one stage before target so authoring commands succeed (#892) ([d3dc8b7](https://github.com/specgraph/specgraph/commit/d3dc8b74464737b590282545322214d6ef4a6571))
- **auth:** Add missing permission mappings for conversation RPCs (#829) ([5b3311f](https://github.com/specgraph/specgraph/commit/5b3311f3b15c1f98b3356f758a924a6e3b7e22a9))
- **auth:** CLI loads API key from credentials file (#828) ([274a05b](https://github.com/specgraph/specgraph/commit/274a05b033d06a93949bb4d78354c2b9e984f555))
- **auth:** Always show admin token path on dev server startup (#827) ([2988951](https://github.com/specgraph/specgraph/commit/298895138f6bb49483cd87666d5b93484d7cda0d))
- **storage:** Handle duplicate slug constraint in CreateDecision and CreateSpec (spgr-dn7) (#826) ([e2367c5](https://github.com/specgraph/specgraph/commit/e2367c54ca112ea635cb9ee6102bfd872a8f2673))
- Dev:reset removes stale compose file, fix XDG data paths (#823) ([fa19b20](https://github.com/specgraph/specgraph/commit/fa19b209baa0989586a76bbc2dd08bf56f9d692b))
- Pass ADR-003 fields through export engine import path (spgr-389) (#820) ([5a8eb45](https://github.com/specgraph/specgraph/commit/5a8eb45e10a46533731334efee87e75b3e83b951))
- Remove redundant .specgraph subdir in EnsureComposeFile path (spgr-2p5) (#808) ([2480161](https://github.com/specgraph/specgraph/commit/24801615a492bff390b40468ad128cc5a62eea8b))
- **ci:** Dockerfile TARGETPLATFORM for dockers_v2 + skip no-op re-runs (#795) ([cc0f547](https://github.com/specgraph/specgraph/commit/cc0f5477416b8f5a560b6064979278189a7e75be))
- **ci:** Skip release on no-op re-runs, switch to dockers_v2 (#794) ([fd915e2](https://github.com/specgraph/specgraph/commit/fd915e2f58555df56502eaf3447a19911a835f9b))
- Remove shadowing renovate.json root config (#793) ([52a356f](https://github.com/specgraph/specgraph/commit/52a356f350f15646cbbd90236dcf4a8401c565c7))
- **security:** Upgrade Go 1.26.1, add Trivy FS scan to CI (#792) ([9a88756](https://github.com/specgraph/specgraph/commit/9a887566e7c86da318bc6763aa2ab0cb5522b0e0))
- **ci:** Goreleaser Docker build — use dockers with buildx + multi-arch (#791) ([1e08acc](https://github.com/specgraph/specgraph/commit/1e08acc0a255794827b723160a61ef36d4b7cd28))
- **ci:** Make tag and release creation fully idempotent (#790) ([5dfb8d1](https://github.com/specgraph/specgraph/commit/5dfb8d18f4c2b2f964127af1e167674d1cc2c91a))
- **ci:** Make release workflow idempotent and advance past v0.3.0 (#789) ([98059cd](https://github.com/specgraph/specgraph/commit/98059cdc456a00dd28f880cb464fdbad3ac5af6c))
- **ci:** Skip changelog commit when nothing changed (idempotent re-runs) (#788) ([60f6a6e](https://github.com/specgraph/specgraph/commit/60f6a6e7bb42db87a19bf79cb99b6cfb4e1f4668))
- **ci:** Bump the version (#787) ([68fa212](https://github.com/specgraph/specgraph/commit/68fa2126da459848f0d9b77ee4ba23746fca6ecc))
- **ci:** Specify pnpm version in goreleaser release job (#786) ([d07f129](https://github.com/specgraph/specgraph/commit/d07f129c7c74f1db9c6dd6ee04fe7a5f15701d50))
- **ci:** Use cliff action output for release notes (#785) ([4227e68](https://github.com/specgraph/specgraph/commit/4227e680780eb2312015ea1b130cb55f4e6bee0e))
- **ci:** Disable persist-credentials so app token works for push (#784) ([94df4b4](https://github.com/specgraph/specgraph/commit/94df4b4bb5db4c8b0d81ef0522843085bb9c69d9))
- **ci:** Use app token via git remote URL for branch protection bypass (#783) ([76e1026](https://github.com/specgraph/specgraph/commit/76e1026a022504aadfb1812fec48837a984df86d))
- **ci:** Use GitHub App token for release workflow pushes (#782) ([d48a26b](https://github.com/specgraph/specgraph/commit/d48a26b702e6cef30245f12397b9596855fbb0d5))
- **ci:** Cap version bumps at 0.x while pre-1.0 (#781) ([73ad662](https://github.com/specgraph/specgraph/commit/73ad6624e49f70ecde37e366b94f068129cb96f1))
- **ci:** Git-cliff action output is 'version' not 'tag' (#780) ([b36e110](https://github.com/specgraph/specgraph/commit/b36e110c62b9124d845fee04716e7bd3a39307fb))
- **server:** Codebase review wave 1 — security fixes + foundations (spgr-dec) (#694) ([febd1b7](https://github.com/specgraph/specgraph/commit/febd1b72934eb233217873064f254e0815fabbd5))
- **web:** Dashboard counts all specs, remove slice filtering, UX label fixes (spgr-scd) (#685) ([961356e](https://github.com/specgraph/specgraph/commit/961356e9da388993050504533e0a637e792ddfc9))
- **ci:** Revert cancel-in-progress to false for release workflow (#679) ([7742983](https://github.com/specgraph/specgraph/commit/77429833a5656c68988af5211dd4877de18d2af9))
- **ci:** Set cancel-in-progress to unblock stuck release run (#678) ([8d74204](https://github.com/specgraph/specgraph/commit/8d7420471a95f6ce5a27b2c86a0a8f0e588a7ff7))
- **ci:** Build web UI before goreleaser in release workflow (#674) ([570fd75](https://github.com/specgraph/specgraph/commit/570fd75bb84d28fa6a9754f011b85599c316ccaa))
- Approve self-approval guardrails + CLI usage dump silence (spgr-8ec, spgr-5sd) (#669) ([8226d57](https://github.com/specgraph/specgraph/commit/8226d57a1bd3fac098d8b8c7a822f87ecd363d58))
- Add explicit step gating to shape skill (#618) ([50ada80](https://github.com/specgraph/specgraph/commit/50ada80fe8e15cd45ed5e2dcebe53c5340c0abc4))
- Resolve absolute binary path and quote slugs in tool commands (#616) ([4378fdb](https://github.com/specgraph/specgraph/commit/4378fdbf77fcdb1a47d6a442259373cb2d505719))
- Enforce slug uniqueness in CreateSpec (#615) ([ca4dd3f](https://github.com/specgraph/specgraph/commit/ca4dd3fd387b84061ff0beaa5480e87f92455c6c))
- Pin cosign-installer to v4.1.0 (no floating v4 tag) (#567) ([571a257](https://github.com/specgraph/specgraph/commit/571a25753ed18e7ac9293aa69959dda8fab95fbb))
- Correct trivy-action version tag (v0.35.0) (#564) ([e515a86](https://github.com/specgraph/specgraph/commit/e515a86d0d0275d3b55a20b8627d7d98a97171d9))
- Push git tag to remote before goreleaser changelog (#533) ([145491d](https://github.com/specgraph/specgraph/commit/145491d129218ad232aa2d2c8c7efe8205b9d7ca))
- Release-please creates release+tag, goreleaser replaces with assets (#530) ([c0c12dd](https://github.com/specgraph/specgraph/commit/c0c12dd79d5d9d19b12fcc52e6f5d5acd67582e0))
- Let goreleaser own GitHub releases, release-please only tags (#528) ([e1138c8](https://github.com/specgraph/specgraph/commit/e1138c88143ecfa2bbda2101f3a78b384d2a1b7a))
- Remove draft:true from release-please config (#525) ([9d9137b](https://github.com/specgraph/specgraph/commit/9d9137b93493a833c47d6bb6db098391f6723f37))
- Simple release flow — release-please creates release, goreleaser uploads assets (#524) ([338b5e8](https://github.com/specgraph/specgraph/commit/338b5e8bb206e87edddaabcc4956002230e4cd65))
- Coordinate release-please and goreleaser — draft release handoff (#521) ([0f8a803](https://github.com/specgraph/specgraph/commit/0f8a803f6efa079f079a623739848b511d62ee0b))
- Dockerfile for goreleaser — use pre-built binary (#519) ([f63f24a](https://github.com/specgraph/specgraph/commit/f63f24a4671d9b22b0102563d8c2d31f251bd327))
- **deps:** Update module golang.org/x/text to v0.35.0 (#29) ([17f6469](https://github.com/specgraph/specgraph/commit/17f6469c4f19d5c888e41f85abf48d0a62c2ff61))
- **deps:** Update module github.com/testcontainers/testcontainers-go to v0.41.0 (#28) ([a278f76](https://github.com/specgraph/specgraph/commit/a278f763ea2d762f47ed87a363121d739c85847f))
- **e2e:** Address 4 open test suite findings (#44) ([87613a5](https://github.com/specgraph/specgraph/commit/87613a5ecced3d3c66dfc829a666ff5e0b2cefd3))
- Wrap all multi-query write paths in RunInTransaction (#42) ([313dcfc](https://github.com/specgraph/specgraph/commit/313dcfce42ddd0449200621f97bd2069f2bc189a))

### Build

- **deps:** Bump github.com/go-jose/go-jose/v4 from 4.1.3 to 4.1.4 in the go_modules group across 1 directory (#825) ([a357d43](https://github.com/specgraph/specgraph/commit/a357d43ff45550f57b717a1c960adab3c26ad4f8))
- Migrate release pipeline from release-please to git-cliff + goreleaser v2 (#677) ([08f1b68](https://github.com/specgraph/specgraph/commit/08f1b68237d7765ad86d5ea15b2902d72cd4502d))
- **deps:** Bump golang.org/x/crypto from 0.43.0 to 0.45.0 (#2) ([2c17f36](https://github.com/specgraph/specgraph/commit/2c17f36145bdd049a03d3f490ab2f808f941c55c))

### CI

- Skip build and test for docs-only changes (#682) ([ae78a38](https://github.com/specgraph/specgraph/commit/ae78a3856ac5a51ed0faf4cee0bd9f965a5a9e85))
- Use PAT for release-please to trigger CI on release PRs (#518) ([20a9eb8](https://github.com/specgraph/specgraph/commit/20a9eb842a2b2a3f5952fc4a0c3cf72f853b34e7))
- Exclude auto-generated CHANGELOG.md from markdown lint (#517) ([1ef88eb](https://github.com/specgraph/specgraph/commit/1ef88eba0ff8d5fc91d59c26ef56355cd61f0121))
- Add release-please + goreleaser infrastructure (#46) ([60c82e7](https://github.com/specgraph/specgraph/commit/60c82e75ebac52c4043a28aa2d68369311797c72))

### Code Refactoring

- Broaden ChangeLogEntry.Stage from SpecStage to string (spgr-egu) (#813) ([57d2a49](https://github.com/specgraph/specgraph/commit/57d2a498559a7c00e7a556a53835155f82ed4a28))
- **server:** Remove redundant ConversationBackend type assertions (#798) ([e50a7ae](https://github.com/specgraph/specgraph/commit/e50a7aefff57c3dc9bab0d78ff0e846f03f78327))
- Codebase review wave 3 — cleanup + hardening (spgr-dec) (#696) ([2b1c24a](https://github.com/specgraph/specgraph/commit/2b1c24a63c21f66ac064df674dc0cb4f905b9176))
- Codebase review wave 2 — layering + type safety (spgr-dec) (#698) ([db2526c](https://github.com/specgraph/specgraph/commit/db2526ca84b5fce76f15fd72d47424f4afe1ad63))
- **proto:** Wrap bare-entity RPC returns in Response messages (#622) ([28efc1c](https://github.com/specgraph/specgraph/commit/28efc1ccb110b06e3fd214b2e826763d33192c24))
- Split AnalyticalFinding into input/output types (#617) ([ddc40af](https://github.com/specgraph/specgraph/commit/ddc40afbd83aeb8a6339b3b92abb8941c80d4a03))
- Storage domain types and decision promotion (#24) ([151b783](https://github.com/specgraph/specgraph/commit/151b783624e71f9d015c51b13e1786a0504ab86c))
- Slice 3.5 — Scanner removal & documentation cleanup (#22) ([a3ffca1](https://github.com/specgraph/specgraph/commit/a3ffca12da27297eae0c15d29662f481b77f4394))

### Features

- **config:** [**breaking**] Koanf config loader with flag>env>file>default precedence (#969) ([6eb6685](https://github.com/specgraph/specgraph/commit/6eb668595ee0ca7c16d259eeac14d1c415147d37))
- **auth:** [**breaking**] Identity Policy Engine — Cedar adoption (spgr-rjrt.4) (#968) ([1939428](https://github.com/specgraph/specgraph/commit/193942849b3e79b8efd766a25aa6b7cb1b3f2862))
- **authn:** Identity Authn — Resolver + JIT + Authorizer seam [spgr-rjrt.3] (#967) ([62cb353](https://github.com/specgraph/specgraph/commit/62cb3530c983433f29f64b65d80344bd464e67ca))
- **storage:** Identity Storage layer (UsersBackend + postgres) [spgr-rjrt.2] (#966) ([6e2b93d](https://github.com/specgraph/specgraph/commit/6e2b93d5c9fa126c41c90467dc8c49e5b94f65e6))
- [**breaking**] Replace SpecLifecycle with SpecProvenance model (#954) ([8fdec6d](https://github.com/specgraph/specgraph/commit/8fdec6d4eef7e63fe91ca94bf41571ce9017ab00))
- **skills:** Restore specgraph-constitution skill (#950) ([e11cae4](https://github.com/specgraph/specgraph/commit/e11cae4814be5b6c7685b82c9615186610928435))
- **harness:** Consolidate Claude / Cursor / OpenCode integration (spgr-cceg) (#939) ([e174d4c](https://github.com/specgraph/specgraph/commit/e174d4c019361d259ef3411f873d29879f793754))
- **cli:** Specgraph read-mcp-resource (Phase B Slice 5 Task 32) (#925) ([7d1c02c](https://github.com/specgraph/specgraph/commit/7d1c02c7c616a04805cca5c0eacbeed03652fcca))
- **authoring:** Composer adapter and MCP prompt delegation (phase B slice 4) (#924) ([aaa7cec](https://github.com/specgraph/specgraph/commit/aaa7cec9294f7cb0c89a69f47d62d499cc396e6d))
- **mcp:** [**breaking**] Drop stdio transport, HTTP-only (#923) ([b3275da](https://github.com/specgraph/specgraph/commit/b3275dad75b3866654c5cc0029148f3f39e1a147))
- **authoring:** Composer with embedded-content assembly and observability (phase B slice 3) (#922) ([c54c9c4](https://github.com/specgraph/specgraph/commit/c54c9c4c2076eaab1d782f5b00fc03d026ea0a80))
- **authoring:** Migrate workflow content to server-embedded files (phase B slice 2) (#920) ([0552cf5](https://github.com/specgraph/specgraph/commit/0552cf5b577cef6192b6051b094b52f7d1829fdf))
- **serve:** Add /livez and /readyz probe endpoints on dedicated listener (#916) ([981cd9a](https://github.com/specgraph/specgraph/commit/981cd9a7824ed791119baa515f2d3136155991c0))
- **cli:** [**breaking**] Split install/uninstall from up/down; retire --rm (#918) ([0128533](https://github.com/specgraph/specgraph/commit/0128533479f02ac368dee11b70b15af013ec1f90))
- **authoring:** Server-side polish for conversation coupling (phase B slice 1) (#917) ([3315c10](https://github.com/specgraph/specgraph/commit/3315c1010913d732dd47baae61bf1380a1cd8a50))
- **authoring:** Atomic conversation recording in stage handlers (phase A) (#910) ([20c69d6](https://github.com/specgraph/specgraph/commit/20c69d6fe9910980cad3f4a552f5ab46b15a1ee4))
- **mcp:** Add auth gating and profile-based tool filtering (#898) ([d300ac0](https://github.com/specgraph/specgraph/commit/d300ac0b9e0c83a9a1026410d25d1d0ebf045b4d))
- **mcp:** Add MCP server with tiered tool access (#897) ([ea35789](https://github.com/specgraph/specgraph/commit/ea357891012d65f6dad1bf541053f6b787880b60))
- Lifecycle nomenclature inversion — amend from in-flight, supersede from done (#889) ([7e7a4cd](https://github.com/specgraph/specgraph/commit/7e7a4cdc213121b6bc76defecf45c963604cc35a))
- Multi-layer constitution with strategic merge and provenance (#888) ([c52068d](https://github.com/specgraph/specgraph/commit/c52068dfd372ab5a3dd9f019db5d5118bce3bdca))
- Optional extra CA cert injection for Docker builds (#886) ([0759545](https://github.com/specgraph/specgraph/commit/075954528b89d1dae489f32f34df92f43af80600))
- Lifecycle amendment/supersede — diffs, changelog UI, docs, E2E (#885) ([33bf386](https://github.com/specgraph/specgraph/commit/33bf38671f29ae6e7ba861d413e571a69febef4f))
- **authoring:** Add steel thread decomposition strategy (spgr-47v) (#878) ([2cfdee2](https://github.com/specgraph/specgraph/commit/2cfdee2053a3ebec88a765b231999fa5a941b484))
- Auto-run analytical passes after authoring stage transitions (spgr-iap) (#830) ([9153e2a](https://github.com/specgraph/specgraph/commit/9153e2a9998ae6529ba338ae0dd2f6eeb2ce64e0))
- **auth:** Add dashboard authentication with cookie-based sessions (#824) ([610df15](https://github.com/specgraph/specgraph/commit/610df1594043e25ab0def65baf52fa039e6b9930))
- **storage:** Replace Memgraph with Postgres backend (spgr-khy) (#821) ([fbbad2e](https://github.com/specgraph/specgraph/commit/fbbad2e9c438cf2f5b8a049bff5e4255bfa9ef9b))
- Add optimistic concurrency version guard to UpdateDecision (spgr-ejd) (#819) ([1fca945](https://github.com/specgraph/specgraph/commit/1fca94569e6f39969a9d096d96c3c75e632d5c6e))
- Extend Decision type with ADR-003 fields (spgr-bk8) (#812) ([2947692](https://github.com/specgraph/specgraph/commit/2947692247f2ccb50d583dbe921a78ffcf2263c2))
- Add skipped-spec count to all-specs drift check (spgr-col) (#810) ([6d739e1](https://github.com/specgraph/specgraph/commit/6d739e1da8164ecc0f5d823209d608a77bdb9ba6))
- Impact notification service for spec changes (spgr-w6o) (#806) ([b87be62](https://github.com/specgraph/specgraph/commit/b87be62ec50097d0205694f340b1edf3cd5a07e9))
- **server:** Expose ListChanges via ConnectRPC API (spgr-fn5) (#804) ([a3f63c4](https://github.com/specgraph/specgraph/commit/a3f63c4a9e9bc85ebbe96e84ec1ce46c1dc09267))
- **e2e:** Coverage-instrumented CLI binary (spgr-6n8) (#803) ([d532977](https://github.com/specgraph/specgraph/commit/d5329775250dbf8305433226f8d8808d20474f24))
- **auth:** OIDC authentication with multi-provider support (spgr-0az) (#802) ([80004d5](https://github.com/specgraph/specgraph/commit/80004d5d19052c06d77cfd465705e497d0d60932))
- Export/import/verify — full project backup and restore (spgr-m56) (#699) ([2265aa4](https://github.com/specgraph/specgraph/commit/2265aa4585c2498be1f80bb81b6ab9f51820c948))
- **web:** Slice support — client, spec detail, graph pills (spgr-6sw.7) (#692) ([c6cea3b](https://github.com/specgraph/specgraph/commit/c6cea3bba55b8dafbae5e64e7ab78d47936f934a))
- **cli:** Add slice list/get/claim/complete commands (spgr-6sw.6) (#691) ([930beca](https://github.com/specgraph/specgraph/commit/930becae90552dfa285d87fc37189fac016e2cc8))
- **server:** SliceService handler + ClaimSlice/CompleteSlice return (*Slice, error) (spgr-6sw.4) (#690) ([461d26d](https://github.com/specgraph/specgraph/commit/461d26d0fe55d8ff9293c5386ff92fcf1b1b8972))
- **storage:** [**breaking**] StoreDecomposeOutput creates Slices + GetFullGraph includes Slices (spgr-6sw.3) (#689) ([4b17cea](https://github.com/specgraph/specgraph/commit/4b17cea52cad154ceca2ba8cf8e82161a0bb6e7d))
- **storage:** Implement Memgraph Slice CRUD + integration tests (spgr-6sw.2) (#688) ([6db9457](https://github.com/specgraph/specgraph/commit/6db9457d486aa496bd8a87be40025efe10ea81e4))
- **proto:** Add Slice message, SliceService, and storage interface (spgr-6sw.1) (#687) ([8fa786b](https://github.com/specgraph/specgraph/commit/8fa786b236a1fdfed9c9488825504870a2d5135b))
- **execution:** Redesign bundle as agent-actionable markdown launchpad (spgr-755) (#686) ([5be164b](https://github.com/specgraph/specgraph/commit/5be164b444b58f931494efc0b4fcf28e28699dc8))
- **skills:** Wire authoring skills to RecordConversation + enable conversation_count (spgr-cdd) (#684) ([b9cc58d](https://github.com/specgraph/specgraph/commit/b9cc58d1d884f447669a6ced63bc439febb666d6))
- **web:** Dashboard overhaul + detail page enhancements for demo-readiness (spgr-5re) (#683) ([4b0c5f2](https://github.com/specgraph/specgraph/commit/4b0c5f2ee34307fc5c87b8c25028773f125edf00))
- **sync:** Idempotent push — FindOrCreate to prevent orphaned external items (spgr-ylq) (#681) ([3f28794](https://github.com/specgraph/specgraph/commit/3f287947ce91b7efa5f8e918e7d553cf498972e1))
- **cmd:** Add comprehensive unit tests for CLI commands (spgr-dvb) (#672) ([c029edb](https://github.com/specgraph/specgraph/commit/c029edbd88143a8a4ef0a021c630518897b54c88))
- Batch improvements — dashboard stats, constitution view, status command, template overrides, bug fixes (#670) ([aa76826](https://github.com/specgraph/specgraph/commit/aa76826681ead827f53201f6b50accf8d195b262))
- **web:** Expand spec detail page with stage outputs, edges, conversations (spgr-zn1) (#668) ([36656fb](https://github.com/specgraph/specgraph/commit/36656fb7a8faba818eeb9857a1ac0cee3f8ab87b))
- Expand specgraph show with full authoring stage detail (spgr-0dg) (#665) ([063a523](https://github.com/specgraph/specgraph/commit/063a523b978f245023c65decca2d9e4394a8f36a))
- ConversationLog graph nodes for authoring audit trail (spgr-9mz) (#664) ([0fce64f](https://github.com/specgraph/specgraph/commit/0fce64ff40289e12983e0b563fa2fa11f82479c3))
- Graph visualization UI with embedded SvelteKit SPA (spgr-p1l) (#644) ([de84cd9](https://github.com/specgraph/specgraph/commit/de84cd95a7bd99322d5e63b2cb438126fa79e4f3))
- Markdown CLI output with --json flags (#496) (#642) ([9d766bd](https://github.com/specgraph/specgraph/commit/9d766bd17fe073052b1d55c25048c78e750a4212))
- Add duplicate slug check to spark skill (#620) ([5800234](https://github.com/specgraph/specgraph/commit/580023454779278801ad7f683a232cc217a3be6f))
- Restructure SpecifyOutput with InterfaceSection, VerifyCriterion, FileTouch (#619) ([c2241d9](https://github.com/specgraph/specgraph/commit/c2241d955d91d0898bec8fb42c3bf534a3113173))
- Plugin restructure, authoring skills, and demo runbook (#609) ([97b9654](https://github.com/specgraph/specgraph/commit/97b96549462421d9755552e7c0801ddb1863fd5a))
- Add prompt templates for remaining analytical passes (#572) ([a4810b5](https://github.com/specgraph/specgraph/commit/a4810b5d0a0112a45c36a74e04010da26f4196d0))
- Analytical pass system with unified findings (#571) ([b60e15a](https://github.com/specgraph/specgraph/commit/b60e15aa38a3f4e1c7b93748776e4cdf97247f2e))
- Supply chain security — cosign, SBOMs, attestations, Trivy scan (#562) ([3810dcf](https://github.com/specgraph/specgraph/commit/3810dcf2d17102c29724fca8d691f819afb378d3))
- Content hash drift detection on DEPENDS_ON edges (#43) ([b0752f1](https://github.com/specgraph/specgraph/commit/b0752f1a7d9b8f2a09bcd9914396e3207f01eb26))
- ChangeLog graph nodes for version tracking (#41) ([81aa512](https://github.com/specgraph/specgraph/commit/81aa5129dcdadfec3e7760ab3397f18f983c89ff))
- Add Murmur3-128 content hash for change detection (#39) ([5a7dfe6](https://github.com/specgraph/specgraph/commit/5a7dfe6a1e2494f7e92dcd47ea094a624d82ddd5))
- **auth:** Add authentication and authorization interceptor (#38) ([a5b1750](https://github.com/specgraph/specgraph/commit/a5b1750a79a4825d8322cd6f40dedc96b5270677))
- **cli:** Add report-progress, report-blocker, report-completion commands (#36) ([f62c02a](https://github.com/specgraph/specgraph/commit/f62c02a9927115474c8cae9a26893ebb229975bc))
- **proto:** Add notes field to Spec + JSON output for show (#35) ([a8cbdaf](https://github.com/specgraph/specgraph/commit/a8cbdafc924097318d1918fd502b94a6bf6ecfd6))
- **plugin:** Evolve authoring skills into partner personas (#34) ([030733d](https://github.com/specgraph/specgraph/commit/030733de892b823238863ee8d3f9c6406a6c9e24))
- **plugin:** Slice 7 — global daemon and Claude Code plugin (#31) ([260b58d](https://github.com/specgraph/specgraph/commit/260b58d4edddfdcb6a8704c084907f393d182bb0))
- **sync:** Slice 6 — sync adapters, tool injection, and CLI (#30) ([ed391a1](https://github.com/specgraph/specgraph/commit/ed391a1250f24b812b69c9951869f6726a3a21ee))
- **lifecycle:** Slice 5 — spec lifecycle operations (#27) ([4a430ca](https://github.com/specgraph/specgraph/commit/4a430caac9e664e686d5b5a1320ed5a72d6b8572))
- **execution:** Slice 4 — domain types consistency & execution bundles (#26) ([c13a9ae](https://github.com/specgraph/specgraph/commit/c13a9aef8fc24a7486989e22d3a6eae87ba46276))
- **docker:** Add Memgraph sizing profiles and persistence (#23) ([46e67d1](https://github.com/specgraph/specgraph/commit/46e67d1326115787216ae4a006c3839b047e6943))
- Slice 3 — Authoring Funnel (#8) ([ebc6a1e](https://github.com/specgraph/specgraph/commit/ebc6a1ea971e0a5b5899661f1c036469ade0c0b5))
- Add constitution subsystem (Slice 2) (#7) ([65be2d7](https://github.com/specgraph/specgraph/commit/65be2d7fe61a8adbf8511f07d3fb0754c5276852))
- Add extended services (health, claim, decision, graph) (#4) ([383c8c3](https://github.com/specgraph/specgraph/commit/383c8c3ec9ac38ec5fb647cdad960978b7bc4e36))
- Add code quality and lefthook setup (#3) ([ec56341](https://github.com/specgraph/specgraph/commit/ec56341fec798b0364f772b6c3c3899154cd836c))
- Vertical slice — client/server architecture (#1) ([649cd0e](https://github.com/specgraph/specgraph/commit/649cd0e78b9eea07f5f23748c7baedb9d66caa3b))
- Include design docs as hidden pages on site ([536ec6f](https://github.com/specgraph/specgraph/commit/536ec6f3ed53006bc3116cfc4a1456d6ba442efa))
- Add Zensical doc site with GitHub Pages deployment ([a9f677f](https://github.com/specgraph/specgraph/commit/a9f677fbf3824150746c2369eae6201d6a4ab8f1))
- Initial ([d402bc9](https://github.com/specgraph/specgraph/commit/d402bc9c3c72bd8a40f5ac4cb3c876df05f40b5b))

### Miscellaneous

- Wire active claims through SpecView via ClaimBackend.GetActiveClaim (#961) ([84209a9](https://github.com/specgraph/specgraph/commit/84209a9009ce6592f3858fd6c61ef0be1f398044))
- Deprecate specgraph inject in favor of MCP + extended init (#940) ([6ba53ba](https://github.com/specgraph/specgraph/commit/6ba53ba7ba1cb0a11467f176276ef209eb346d4c))
- Idempotent 'specgraph init' with managed per-harness MCP configs (#929) ([f3d44c8](https://github.com/specgraph/specgraph/commit/f3d44c8183fb1b0f74f671dc925014e42bcb02c9))

### Performance

- Share single memgraph container across integration tests (#516) ([1615001](https://github.com/specgraph/specgraph/commit/1615001aae2cc82a7339cb2ed0c0dd44e4c90b14))

### Tests

- **integration:** Concurrent duplicate-detection for CreateSyncMapping (spgr-5xt) (#809) ([fad9951](https://github.com/specgraph/specgraph/commit/fad99515f88ecebf0c9629ed8ae2dce7a3912697))
- **cli:** Add lifecycle RPC error tests (spgr-a7t.15) (#800) ([c1ea857](https://github.com/specgraph/specgraph/commit/c1ea857b081966d04c286530846179138692bb92))
- **e2e:** Add SliceService E2E tests + update decompose assertions (spgr-6sw.8) (#693) ([460a397](https://github.com/specgraph/specgraph/commit/460a397f63b92cc7f27f63a5497bb5467a27a30f))
- **integration:** Add DISTINCT regression test for GetExecutionEvents (#37) ([ca716b5](https://github.com/specgraph/specgraph/commit/ca716b593c223df5cee8379030653cd63c4c7785))
- **e2e:** Implement 3-tier E2E test suite (#32) ([b12e1f7](https://github.com/specgraph/specgraph/commit/b12e1f7e3d3ec45daf67b4dcfd984d630bcda754))
- Add comprehensive E2E test system (#19) ([7e99d69](https://github.com/specgraph/specgraph/commit/7e99d69be913325efac891a9d4d4a4b007aaaeef))


