# Changelog

All notable changes to this project will be documented in this file.
## [0.3.0](https://github.com/specgraph/specgraph/compare/v0.2.1...v0.3.0) - 2026-03-28

### Bug Fixes

- **ci:** Skip changelog commit when nothing changed (idempotent re-runs) (#788) ([#788](https://github.com/specgraph/specgraph/pull/788)) ([d95cb4d](https://github.com/specgraph/specgraph/commit/d95cb4dd96fb5782f53a2786591ebaaf3e8c70b2))
- **ci:** Bump the version (#787) ([#787](https://github.com/specgraph/specgraph/pull/787)) ([6dfd5da](https://github.com/specgraph/specgraph/commit/6dfd5da7884ab3e3e1363b998242fccc093b8f32))
- **ci:** Specify pnpm version in goreleaser release job (#786) ([#786](https://github.com/specgraph/specgraph/pull/786)) ([81d28d2](https://github.com/specgraph/specgraph/commit/81d28d20356a632b1f2f1a6fb24b10932ce3fdae))
- **ci:** Use cliff action output for release notes (#785) ([#785](https://github.com/specgraph/specgraph/pull/785)) ([1c4bac0](https://github.com/specgraph/specgraph/commit/1c4bac074513310bd7db44481ae3383ec0fbff16))
- **ci:** Disable persist-credentials so app token works for push (#784) ([#784](https://github.com/specgraph/specgraph/pull/784)) ([3bd38db](https://github.com/specgraph/specgraph/commit/3bd38dbad7cd7b66c12cd3f166e8b929b90d2769))
- **ci:** Use app token via git remote URL for branch protection bypass (#783) ([#783](https://github.com/specgraph/specgraph/pull/783)) ([8cebf51](https://github.com/specgraph/specgraph/commit/8cebf512326f3091831bd6bbf44d2ee8d80e0e87))
- **ci:** Use GitHub App token for release workflow pushes (#782) ([#782](https://github.com/specgraph/specgraph/pull/782)) ([138b9c5](https://github.com/specgraph/specgraph/commit/138b9c5d1ae8d68a34ca25622270590ffcca6dfb))
- **ci:** Cap version bumps at 0.x while pre-1.0 (#781) ([#781](https://github.com/specgraph/specgraph/pull/781)) ([72319ad](https://github.com/specgraph/specgraph/commit/72319ad0f942331cc18616433f9a2a3e08924a1a))
- **ci:** Git-cliff action output is 'version' not 'tag' (#780) ([#780](https://github.com/specgraph/specgraph/pull/780)) ([28f4cb7](https://github.com/specgraph/specgraph/commit/28f4cb78380633a60bd225a484fae9aedcc1f3eb))
- **server:** Codebase review wave 1 — security fixes + foundations (spgr-dec) (#694) ([#694](https://github.com/specgraph/specgraph/pull/694)) ([2869dce](https://github.com/specgraph/specgraph/commit/2869dceedb17983aa93201caee5fbc28bafd134d))
- **web:** Dashboard counts all specs, remove slice filtering, UX label fixes (spgr-scd) (#685) ([#685](https://github.com/specgraph/specgraph/pull/685)) ([8f225a9](https://github.com/specgraph/specgraph/commit/8f225a9a33f6c6df604e3342f859d139a2048819))
- **ci:** Revert cancel-in-progress to false for release workflow (#679) ([#679](https://github.com/specgraph/specgraph/pull/679)) ([c9ad431](https://github.com/specgraph/specgraph/commit/c9ad4315a8565b8aaa81033fc4e0bc1cd2d46efa))
- **ci:** Set cancel-in-progress to unblock stuck release run (#678) ([#678](https://github.com/specgraph/specgraph/pull/678)) ([e0f95fd](https://github.com/specgraph/specgraph/commit/e0f95fd94db808c51b26f4943f856bf5002f3c3a))

### Build

- Migrate release pipeline from release-please to git-cliff + goreleaser v2 (#677) ([#677](https://github.com/specgraph/specgraph/pull/677)) ([dd0ceec](https://github.com/specgraph/specgraph/commit/dd0ceecfe1bf9035dd4f9bdbfe0d69e53402de2d))

### CI

- Skip build and test for docs-only changes (#682) ([#682](https://github.com/specgraph/specgraph/pull/682)) ([b8a0978](https://github.com/specgraph/specgraph/commit/b8a0978b8efc85f740f087319fc60912fe0686b1))

### Code Refactoring

- Codebase review wave 3 — cleanup + hardening (spgr-dec) (#696) ([#696](https://github.com/specgraph/specgraph/pull/696)) ([697238e](https://github.com/specgraph/specgraph/commit/697238e7d9ec5d85e2c3d4e7aceb6ecf55945671))
- Codebase review wave 2 — layering + type safety (spgr-dec) (#698) ([#698](https://github.com/specgraph/specgraph/pull/698)) ([63d73fd](https://github.com/specgraph/specgraph/commit/63d73fd8c3768fcb0401d39881b56e994955c020))

### Features

- Export/import/verify — full project backup and restore (spgr-m56) (#699) ([#699](https://github.com/specgraph/specgraph/pull/699)) ([eabfac9](https://github.com/specgraph/specgraph/commit/eabfac954f2ee38f783974b4b6ce8def043f8ba9))
- **web:** Slice support — client, spec detail, graph pills (spgr-6sw.7) (#692) ([#692](https://github.com/specgraph/specgraph/pull/692)) ([f2e1d47](https://github.com/specgraph/specgraph/commit/f2e1d47fc003626c38be7f0e9c5ba6ce2156229e))
- **cli:** Add slice list/get/claim/complete commands (spgr-6sw.6) (#691) ([#691](https://github.com/specgraph/specgraph/pull/691)) ([10f2262](https://github.com/specgraph/specgraph/commit/10f2262a246587ee46d21c0a51d091a5efbc9fdb))
- **server:** SliceService handler + ClaimSlice/CompleteSlice return (*Slice, error) (spgr-6sw.4) (#690) ([#690](https://github.com/specgraph/specgraph/pull/690)) ([de9bd8d](https://github.com/specgraph/specgraph/commit/de9bd8d3f87fc622606ca2aa0ae1ec098b4c313c))
- **storage:** [**breaking**] StoreDecomposeOutput creates Slices + GetFullGraph includes Slices (spgr-6sw.3) (#689) ([#689](https://github.com/specgraph/specgraph/pull/689)) ([0a66af1](https://github.com/specgraph/specgraph/commit/0a66af1340fa24fa15e16be08420be5884d77a52))
- **storage:** Implement Memgraph Slice CRUD + integration tests (spgr-6sw.2) (#688) ([#688](https://github.com/specgraph/specgraph/pull/688)) ([576c746](https://github.com/specgraph/specgraph/commit/576c746ce754babdda34768ce2aa583215eb9239))
- **proto:** Add Slice message, SliceService, and storage interface (spgr-6sw.1) (#687) ([#687](https://github.com/specgraph/specgraph/pull/687)) ([5a87e84](https://github.com/specgraph/specgraph/commit/5a87e849d3ecf3153812fe337ce5d0b41671c631))
- **execution:** Redesign bundle as agent-actionable markdown launchpad (spgr-755) (#686) ([#686](https://github.com/specgraph/specgraph/pull/686)) ([87205ac](https://github.com/specgraph/specgraph/commit/87205ac1713f017a150f9491de16aba23110fd97))
- **skills:** Wire authoring skills to RecordConversation + enable conversation_count (spgr-cdd) (#684) ([#684](https://github.com/specgraph/specgraph/pull/684)) ([ef2ea4f](https://github.com/specgraph/specgraph/commit/ef2ea4f81b1668f15bcd59475f58d4f20694428e))
- **web:** Dashboard overhaul + detail page enhancements for demo-readiness (spgr-5re) (#683) ([#683](https://github.com/specgraph/specgraph/pull/683)) ([bdb2fa8](https://github.com/specgraph/specgraph/commit/bdb2fa8b5c5f9366d316eee5ce391b065ff06ced))
- **sync:** Idempotent push — FindOrCreate to prevent orphaned external items (spgr-ylq) (#681) ([#681](https://github.com/specgraph/specgraph/pull/681)) ([9fbdf29](https://github.com/specgraph/specgraph/commit/9fbdf29a663ba3e267208e7c5918365707a8499b))

### Tests

- **e2e:** Add SliceService E2E tests + update decompose assertions (spgr-6sw.8) (#693) ([#693](https://github.com/specgraph/specgraph/pull/693)) ([3bacc11](https://github.com/specgraph/specgraph/commit/3bacc110e2b0a771f657d2dd2c3ea488e4ec1f23))

## [0.2.1](https://github.com/specgraph/specgraph/compare/v0.2.0...v0.2.1) - 2026-03-26

### Bug Fixes

- **ci:** Build web UI before goreleaser in release workflow (#674) ([#674](https://github.com/specgraph/specgraph/pull/674)) ([0d9b96e](https://github.com/specgraph/specgraph/commit/0d9b96e49b08deb4a868fcc0cba439b8924f9993))

## [0.2.0](https://github.com/specgraph/specgraph/compare/v0.1.6...v0.2.0) - 2026-03-26

### Bug Fixes

- Approve self-approval guardrails + CLI usage dump silence (spgr-8ec, spgr-5sd) (#669) ([#669](https://github.com/specgraph/specgraph/pull/669)) ([b984856](https://github.com/specgraph/specgraph/commit/b984856ea422b28e03835bc8915fe033517cb62d))
- Add explicit step gating to shape skill (#618) ([#618](https://github.com/specgraph/specgraph/pull/618)) ([ca2cedc](https://github.com/specgraph/specgraph/commit/ca2cedcdf2f2be0a7c81542f36c4aaddc6fa5a6a))
- Resolve absolute binary path and quote slugs in tool commands (#616) ([#616](https://github.com/specgraph/specgraph/pull/616)) ([e2e08f4](https://github.com/specgraph/specgraph/commit/e2e08f4d69272a4f2687d8cd054c2afeb9d9311c))
- Enforce slug uniqueness in CreateSpec (#615) ([#615](https://github.com/specgraph/specgraph/pull/615)) ([5d57110](https://github.com/specgraph/specgraph/commit/5d57110e5c58372833a5a2197fd34d85190abe1d))

### Code Refactoring

- **proto:** Wrap bare-entity RPC returns in Response messages (#622) ([#622](https://github.com/specgraph/specgraph/pull/622)) ([b724308](https://github.com/specgraph/specgraph/commit/b72430893fdf36f04ee9b54e6f3046c87f733faf))
- Split AnalyticalFinding into input/output types (#617) ([#617](https://github.com/specgraph/specgraph/pull/617)) ([12a558d](https://github.com/specgraph/specgraph/commit/12a558dc305dba503a3ad4b1a73be90c6c8e5afd))

### Features

- **cmd:** Add comprehensive unit tests for CLI commands (spgr-dvb) (#672) ([#672](https://github.com/specgraph/specgraph/pull/672)) ([eb4d5f4](https://github.com/specgraph/specgraph/commit/eb4d5f4516830389275c4986af03b2fdd1bdfe15))
- Batch improvements — dashboard stats, constitution view, status command, template overrides, bug fixes (#670) ([#670](https://github.com/specgraph/specgraph/pull/670)) ([c8aa122](https://github.com/specgraph/specgraph/commit/c8aa1224980209cb44317c8c4407c1ba8aa0284d))
- **web:** Expand spec detail page with stage outputs, edges, conversations (spgr-zn1) (#668) ([#668](https://github.com/specgraph/specgraph/pull/668)) ([2638edd](https://github.com/specgraph/specgraph/commit/2638edd9c8cccca1d24e261476fdb4ed8487e0bf))
- Expand specgraph show with full authoring stage detail (spgr-0dg) (#665) ([#665](https://github.com/specgraph/specgraph/pull/665)) ([8fc8412](https://github.com/specgraph/specgraph/commit/8fc8412dbc3fad04b59e1e83ecd9e47fdd06bda9))
- ConversationLog graph nodes for authoring audit trail (spgr-9mz) (#664) ([#664](https://github.com/specgraph/specgraph/pull/664)) ([140341b](https://github.com/specgraph/specgraph/commit/140341be9a1986d07ef2a8c1b76d5f96184a1847))
- Graph visualization UI with embedded SvelteKit SPA (spgr-p1l) (#644) ([#644](https://github.com/specgraph/specgraph/pull/644)) ([d66606a](https://github.com/specgraph/specgraph/commit/d66606adb84e2e4a5a9238691b11bac3c31e0f0f))
- Markdown CLI output with --json flags (#496) (#642) ([#642](https://github.com/specgraph/specgraph/pull/642)) ([0f3bbe8](https://github.com/specgraph/specgraph/commit/0f3bbe81779981eb11d56431117efab75324012c))
- Add duplicate slug check to spark skill (#620) ([#620](https://github.com/specgraph/specgraph/pull/620)) ([4655650](https://github.com/specgraph/specgraph/commit/4655650785be3436126e709bd0d8ba5b8f5980f2))
- Restructure SpecifyOutput with InterfaceSection, VerifyCriterion, FileTouch (#619) ([#619](https://github.com/specgraph/specgraph/pull/619)) ([5ba9999](https://github.com/specgraph/specgraph/commit/5ba9999d255ef556472713c4fe09386af4569cda))
- Plugin restructure, authoring skills, and demo runbook (#609) ([#609](https://github.com/specgraph/specgraph/pull/609)) ([b12f199](https://github.com/specgraph/specgraph/commit/b12f1992c22fe24789070dfa172c2852e08f83ea))
- Add prompt templates for remaining analytical passes (#572) ([#572](https://github.com/specgraph/specgraph/pull/572)) ([e28ab1c](https://github.com/specgraph/specgraph/commit/e28ab1cae0ac3e1635a053b823e7f4b76b0333ab))
- Analytical pass system with unified findings (#571) ([#571](https://github.com/specgraph/specgraph/pull/571)) ([e8f455e](https://github.com/specgraph/specgraph/commit/e8f455e0198740bd0caf01cd024ed8f54b313f1a))

## [0.1.6](https://github.com/specgraph/specgraph/compare/v0.1.5...v0.1.6) - 2026-03-21

### Bug Fixes

- Pin cosign-installer to v4.1.0 (no floating v4 tag) (#567) ([#567](https://github.com/specgraph/specgraph/pull/567)) ([009ce40](https://github.com/specgraph/specgraph/commit/009ce40262766254ca003cd6e08c4be537dbe06f))
- Correct trivy-action version tag (v0.35.0) (#564) ([#564](https://github.com/specgraph/specgraph/pull/564)) ([b55924d](https://github.com/specgraph/specgraph/commit/b55924d9b19fa5b7988a19006b42034b0727816c))

### Features

- Supply chain security — cosign, SBOMs, attestations, Trivy scan (#562) ([#562](https://github.com/specgraph/specgraph/pull/562)) ([070e014](https://github.com/specgraph/specgraph/commit/070e014ccf4f4aa5cef41fbc80ddeba9b2e1c267))

## [0.1.5](https://github.com/specgraph/specgraph/compare/v0.1.3...v0.1.5) - 2026-03-21

### Bug Fixes

- Push git tag to remote before goreleaser changelog (#533) ([#533](https://github.com/specgraph/specgraph/pull/533)) ([2f40fd2](https://github.com/specgraph/specgraph/commit/2f40fd2fb0cfd71f2cbb03b01c88bea89b69158c))
- Release-please creates release+tag, goreleaser replaces with assets (#530) ([#530](https://github.com/specgraph/specgraph/pull/530)) ([8439c3a](https://github.com/specgraph/specgraph/commit/8439c3a7c6a510a1442be5b35d2ca61b22365178))
- Let goreleaser own GitHub releases, release-please only tags (#528) ([#528](https://github.com/specgraph/specgraph/pull/528)) ([56051cf](https://github.com/specgraph/specgraph/commit/56051cfe16c34f680ae457388a741578e872679e))

## [0.1.3](https://github.com/specgraph/specgraph/compare/v0.1.1...v0.1.3) - 2026-03-21

### Bug Fixes

- Remove draft:true from release-please config (#525) ([#525](https://github.com/specgraph/specgraph/pull/525)) ([7f2f138](https://github.com/specgraph/specgraph/commit/7f2f1389aec66fa060d6e87290bab9a51670e353))
- Simple release flow — release-please creates release, goreleaser uploads assets (#524) ([#524](https://github.com/specgraph/specgraph/pull/524)) ([7f7b024](https://github.com/specgraph/specgraph/commit/7f7b024a5ea36acef6152778f821be00f0281112))
- Coordinate release-please and goreleaser — draft release handoff (#521) ([#521](https://github.com/specgraph/specgraph/pull/521)) ([fc299c4](https://github.com/specgraph/specgraph/commit/fc299c49d5bc91037cdaa955e734d6a5a3c42fd4))

## [0.1.1](https://github.com/specgraph/specgraph/compare/v0.1.0...v0.1.1) - 2026-03-21

### Bug Fixes

- Dockerfile for goreleaser — use pre-built binary (#519) ([#519](https://github.com/specgraph/specgraph/pull/519)) ([92c243b](https://github.com/specgraph/specgraph/commit/92c243bf2ae5626d70c6e0f84b4f9240b6a48275))

## [0.1.0](https://github.com/specgraph/specgraph/compare/...v0.1.0) - 2026-03-21

### Bug Fixes

- **deps:** Update module golang.org/x/text to v0.35.0 (#29) ([#29](https://github.com/specgraph/specgraph/pull/29)) ([81fb5bf](https://github.com/specgraph/specgraph/commit/81fb5bff3ebaeffcfce4ea255444ee65a0841d09))
- **deps:** Update module github.com/testcontainers/testcontainers-go to v0.41.0 (#28) ([#28](https://github.com/specgraph/specgraph/pull/28)) ([2de880e](https://github.com/specgraph/specgraph/commit/2de880e92923fa4e8accb0a32793656ecd323db5))
- **e2e:** Address 4 open test suite findings (#44) ([#44](https://github.com/specgraph/specgraph/pull/44)) ([a029036](https://github.com/specgraph/specgraph/commit/a0290368fd4a56618187358b082fc8974aeff185))
- Wrap all multi-query write paths in RunInTransaction (#42) ([#42](https://github.com/specgraph/specgraph/pull/42)) ([04045e8](https://github.com/specgraph/specgraph/commit/04045e82e64d0cf49af5531c2cbf48d3cd2d4888))

### Build

- **deps:** Bump golang.org/x/crypto from 0.43.0 to 0.45.0 (#2) ([#2](https://github.com/specgraph/specgraph/pull/2)) ([a4b88f8](https://github.com/specgraph/specgraph/commit/a4b88f82d2c7b71fbd89a48db4fb48a1d34b5b87))

### CI

- Use PAT for release-please to trigger CI on release PRs (#518) ([#518](https://github.com/specgraph/specgraph/pull/518)) ([92805cf](https://github.com/specgraph/specgraph/commit/92805cfff0727fa5287349ab7faec36ecbef6c0f))
- Exclude auto-generated CHANGELOG.md from markdown lint (#517) ([#517](https://github.com/specgraph/specgraph/pull/517)) ([7106861](https://github.com/specgraph/specgraph/commit/71068619c63a7a7f9749fa98e44287dceed001e3))
- Add release-please + goreleaser infrastructure (#46) ([#46](https://github.com/specgraph/specgraph/pull/46)) ([1fd22d3](https://github.com/specgraph/specgraph/commit/1fd22d3d9ab3c80360a5e0d9117741192ddd26b8))

### Code Refactoring

- Storage domain types and decision promotion (#24) ([#24](https://github.com/specgraph/specgraph/pull/24)) ([836abee](https://github.com/specgraph/specgraph/commit/836abeea8a96d04898d874aaddc6b4a574850690))
- Slice 3.5 — Scanner removal & documentation cleanup (#22) ([#22](https://github.com/specgraph/specgraph/pull/22)) ([f06a476](https://github.com/specgraph/specgraph/commit/f06a47685fe1ce27ed5a265ff209448bd04b414c))

### Features

- Content hash drift detection on DEPENDS_ON edges (#43) ([#43](https://github.com/specgraph/specgraph/pull/43)) ([6c86b33](https://github.com/specgraph/specgraph/commit/6c86b33fe59326557a309d1fcddf098bef0b5df3))
- ChangeLog graph nodes for version tracking (#41) ([#41](https://github.com/specgraph/specgraph/pull/41)) ([e5c00dc](https://github.com/specgraph/specgraph/commit/e5c00dc2def9d8cd408e327afdf5b38f94b3c212))
- Add Murmur3-128 content hash for change detection (#39) ([#39](https://github.com/specgraph/specgraph/pull/39)) ([b3c10b2](https://github.com/specgraph/specgraph/commit/b3c10b2f37f3ab1a9de5a6553ce63a656e48bb52))
- **auth:** Add authentication and authorization interceptor (#38) ([#38](https://github.com/specgraph/specgraph/pull/38)) ([f4fc6bf](https://github.com/specgraph/specgraph/commit/f4fc6bf2338020d521fe5ef626da2f8f5be2e1d5))
- **cli:** Add report-progress, report-blocker, report-completion commands (#36) ([#36](https://github.com/specgraph/specgraph/pull/36)) ([18b09bb](https://github.com/specgraph/specgraph/commit/18b09bb8fb6a6a878fb8c4cc87baad8d9acfb640))
- **proto:** Add notes field to Spec + JSON output for show (#35) ([#35](https://github.com/specgraph/specgraph/pull/35)) ([524b09c](https://github.com/specgraph/specgraph/commit/524b09c990999f6c8840c9ab171ccbc776fe042f))
- **plugin:** Evolve authoring skills into partner personas (#34) ([#34](https://github.com/specgraph/specgraph/pull/34)) ([c260969](https://github.com/specgraph/specgraph/commit/c260969862c3e536765ec26a15700f5d39eed1a5))
- **plugin:** Slice 7 — global daemon and Claude Code plugin (#31) ([#31](https://github.com/specgraph/specgraph/pull/31)) ([a8a07b4](https://github.com/specgraph/specgraph/commit/a8a07b47ed18fcc5e52de4c7423a7be30e772914))
- **sync:** Slice 6 — sync adapters, tool injection, and CLI (#30) ([#30](https://github.com/specgraph/specgraph/pull/30)) ([c4c6ae7](https://github.com/specgraph/specgraph/commit/c4c6ae716dfc3bad7418085a75b42c1b1a81a93b))
- **lifecycle:** Slice 5 — spec lifecycle operations (#27) ([#27](https://github.com/specgraph/specgraph/pull/27)) ([5adf681](https://github.com/specgraph/specgraph/commit/5adf6813d3bccc7bd16b7279a90e9f451a8dc634))
- **execution:** Slice 4 — domain types consistency & execution bundles (#26) ([#26](https://github.com/specgraph/specgraph/pull/26)) ([9942813](https://github.com/specgraph/specgraph/commit/9942813353c8afeb930d5de68aec808079fc338b))
- **docker:** Add Memgraph sizing profiles and persistence (#23) ([#23](https://github.com/specgraph/specgraph/pull/23)) ([9a2ab3f](https://github.com/specgraph/specgraph/commit/9a2ab3f82367204c9c880086b0f69e4bdb810a6a))
- Slice 3 — Authoring Funnel (#8) ([#8](https://github.com/specgraph/specgraph/pull/8)) ([8d15fd1](https://github.com/specgraph/specgraph/commit/8d15fd19d9e3df1102c6a2f5e4a1b17b1a077fca))
- Add constitution subsystem (Slice 2) (#7) ([#7](https://github.com/specgraph/specgraph/pull/7)) ([10c2ee3](https://github.com/specgraph/specgraph/commit/10c2ee3180a2bf11dd8c179cb4ea4e018f54ace7))
- Add extended services (health, claim, decision, graph) (#4) ([#4](https://github.com/specgraph/specgraph/pull/4)) ([9fd18e5](https://github.com/specgraph/specgraph/commit/9fd18e5496d5d664c9be4f72e04a583d573f4d5e))
- Add code quality and lefthook setup (#3) ([#3](https://github.com/specgraph/specgraph/pull/3)) ([970664e](https://github.com/specgraph/specgraph/commit/970664ea5a5a44ece3557eff3c9e247e1e009a88))
- Vertical slice — client/server architecture (#1) ([#1](https://github.com/specgraph/specgraph/pull/1)) ([50b504c](https://github.com/specgraph/specgraph/commit/50b504c67167cd52ab43fd956536a38ca8bacc08))
- Include design docs as hidden pages on site ([3f986a1](https://github.com/specgraph/specgraph/commit/3f986a1753269629b69c8c2baf2cfc8cfde0abe5))
- Add Zensical doc site with GitHub Pages deployment ([7a1410e](https://github.com/specgraph/specgraph/commit/7a1410e0ae39485c3f7540ddaf8affc21cfd6cbd))
- Initial ([a46c950](https://github.com/specgraph/specgraph/commit/a46c950af7c44cf0d101bb9895878698dd5bf0d1))

### Performance

- Share single memgraph container across integration tests (#516) ([#516](https://github.com/specgraph/specgraph/pull/516)) ([a95ac45](https://github.com/specgraph/specgraph/commit/a95ac459e6fe63457cb957ec44d0444edb891b87))

### Tests

- **integration:** Add DISTINCT regression test for GetExecutionEvents (#37) ([#37](https://github.com/specgraph/specgraph/pull/37)) ([2b17445](https://github.com/specgraph/specgraph/commit/2b17445a8421f114d6f34ef3f1fca361afa32dcc))
- **e2e:** Implement 3-tier E2E test suite (#32) ([#32](https://github.com/specgraph/specgraph/pull/32)) ([de12bbc](https://github.com/specgraph/specgraph/commit/de12bbceaa1f437ee37d98f4d76e7f2f7817611f))
- Add comprehensive E2E test system (#19) ([#19](https://github.com/specgraph/specgraph/pull/19)) ([6ecf4e5](https://github.com/specgraph/specgraph/commit/6ecf4e585a21a252fdc18e16e4a6ebcfc109310c))


