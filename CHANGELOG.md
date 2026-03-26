# Changelog

## [0.2.1](https://github.com/specgraph/specgraph/compare/v0.2.1...v0.2.1) (2026-03-26)


### Features

* add code quality and lefthook setup ([#3](https://github.com/specgraph/specgraph/issues/3)) ([970664e](https://github.com/specgraph/specgraph/commit/970664ea5a5a44ece3557eff3c9e247e1e009a88))
* add constitution subsystem (Slice 2) ([#7](https://github.com/specgraph/specgraph/issues/7)) ([10c2ee3](https://github.com/specgraph/specgraph/commit/10c2ee3180a2bf11dd8c179cb4ea4e018f54ace7))
* add ConversationLog graph nodes for authoring audit trail (spgr-9mz) ([140341b](https://github.com/specgraph/specgraph/commit/140341be9a1986d07ef2a8c1b76d5f96184a1847))
* add duplicate slug check to spark skill ([#620](https://github.com/specgraph/specgraph/issues/620)) ([4655650](https://github.com/specgraph/specgraph/commit/4655650785be3436126e709bd0d8ba5b8f5980f2))
* add extended services (health, claim, decision, graph) ([#4](https://github.com/specgraph/specgraph/issues/4)) ([9fd18e5](https://github.com/specgraph/specgraph/commit/9fd18e5496d5d664c9be4f72e04a583d573f4d5e))
* add Murmur3-128 content hash for change detection ([#39](https://github.com/specgraph/specgraph/issues/39)) ([b3c10b2](https://github.com/specgraph/specgraph/commit/b3c10b2f37f3ab1a9de5a6553ce63a656e48bb52))
* add prompt templates for remaining analytical passes ([#572](https://github.com/specgraph/specgraph/issues/572)) ([e28ab1c](https://github.com/specgraph/specgraph/commit/e28ab1cae0ac3e1635a053b823e7f4b76b0333ab))
* add Zensical doc site with GitHub Pages deployment ([7a1410e](https://github.com/specgraph/specgraph/commit/7a1410e0ae39485c3f7540ddaf8affc21cfd6cbd))
* analytical pass system with unified findings ([#571](https://github.com/specgraph/specgraph/issues/571)) ([e8f455e](https://github.com/specgraph/specgraph/commit/e8f455e0198740bd0caf01cd024ed8f54b313f1a))
* **auth:** add authentication and authorization interceptor ([#38](https://github.com/specgraph/specgraph/issues/38)) ([f4fc6bf](https://github.com/specgraph/specgraph/commit/f4fc6bf2338020d521fe5ef626da2f8f5be2e1d5))
* batch improvements — dashboard stats, constitution view, status command, template overrides, bug fixes ([#670](https://github.com/specgraph/specgraph/issues/670)) ([c8aa122](https://github.com/specgraph/specgraph/commit/c8aa1224980209cb44317c8c4407c1ba8aa0284d))
* batch improvements + fix skill format references (spgr-r40, spgr-qdl, spgr-qu2, spgr-9n8, spgr-8ec, spgr-5sd, spgr-p9h) ([c8aa122](https://github.com/specgraph/specgraph/commit/c8aa1224980209cb44317c8c4407c1ba8aa0284d))
* batch improvements + fix skill format references (spgr-r40, spgr-qdl, spgr-qu2, spgr-9n8, spgr-8ec, spgr-5sd, spgr-p9h) ([b984856](https://github.com/specgraph/specgraph/commit/b984856ea422b28e03835bc8915fe033517cb62d))
* ChangeLog graph nodes for version tracking ([#41](https://github.com/specgraph/specgraph/issues/41)) ([e5c00dc](https://github.com/specgraph/specgraph/commit/e5c00dc2def9d8cd408e327afdf5b38f94b3c212))
* **cli:** add report-progress, report-blocker, report-completion commands ([#36](https://github.com/specgraph/specgraph/issues/36)) ([18b09bb](https://github.com/specgraph/specgraph/commit/18b09bb8fb6a6a878fb8c4cc87baad8d9acfb640))
* **cmd:** add comprehensive unit tests for CLI commands (spgr-dvb) ([#672](https://github.com/specgraph/specgraph/issues/672)) ([eb4d5f4](https://github.com/specgraph/specgraph/commit/eb4d5f4516830389275c4986af03b2fdd1bdfe15))
* content hash drift detection on DEPENDS_ON edges ([#43](https://github.com/specgraph/specgraph/issues/43)) ([6c86b33](https://github.com/specgraph/specgraph/commit/6c86b33fe59326557a309d1fcddf098bef0b5df3))
* ConversationLog graph nodes for authoring audit trail (spgr-9mz) ([#664](https://github.com/specgraph/specgraph/issues/664)) ([140341b](https://github.com/specgraph/specgraph/commit/140341be9a1986d07ef2a8c1b76d5f96184a1847))
* **docker:** add Memgraph sizing profiles and persistence ([#23](https://github.com/specgraph/specgraph/issues/23)) ([9a2ab3f](https://github.com/specgraph/specgraph/commit/9a2ab3f82367204c9c880086b0f69e4bdb810a6a))
* **execution:** Slice 4 — domain types consistency & execution bundles ([#26](https://github.com/specgraph/specgraph/issues/26)) ([9942813](https://github.com/specgraph/specgraph/commit/9942813353c8afeb930d5de68aec808079fc338b))
* expand specgraph show with full authoring stage detail (spgr-0dg) ([#665](https://github.com/specgraph/specgraph/issues/665)) ([8fc8412](https://github.com/specgraph/specgraph/commit/8fc8412dbc3fad04b59e1e83ecd9e47fdd06bda9))
* graph visualization UI with embedded SvelteKit SPA (spgr-p1l) ([#644](https://github.com/specgraph/specgraph/issues/644)) ([d66606a](https://github.com/specgraph/specgraph/commit/d66606adb84e2e4a5a9238691b11bac3c31e0f0f))
* include design docs as hidden pages on site ([3f986a1](https://github.com/specgraph/specgraph/commit/3f986a1753269629b69c8c2baf2cfc8cfde0abe5))
* initial ([a46c950](https://github.com/specgraph/specgraph/commit/a46c950af7c44cf0d101bb9895878698dd5bf0d1))
* **lifecycle:** Slice 5 — spec lifecycle operations ([#27](https://github.com/specgraph/specgraph/issues/27)) ([5adf681](https://github.com/specgraph/specgraph/commit/5adf6813d3bccc7bd16b7279a90e9f451a8dc634))
* markdown CLI output with --json flags ([#496](https://github.com/specgraph/specgraph/issues/496)) ([#642](https://github.com/specgraph/specgraph/issues/642)) ([0f3bbe8](https://github.com/specgraph/specgraph/commit/0f3bbe81779981eb11d56431117efab75324012c))
* plugin restructure, authoring skills, and demo runbook ([#609](https://github.com/specgraph/specgraph/issues/609)) ([b12f199](https://github.com/specgraph/specgraph/commit/b12f1992c22fe24789070dfa172c2852e08f83ea))
* **plugin:** evolve authoring skills from CLI reference cards to partner personas ([c260969](https://github.com/specgraph/specgraph/commit/c260969862c3e536765ec26a15700f5d39eed1a5))
* **plugin:** evolve authoring skills into partner personas ([#34](https://github.com/specgraph/specgraph/issues/34)) ([c260969](https://github.com/specgraph/specgraph/commit/c260969862c3e536765ec26a15700f5d39eed1a5))
* **plugin:** Slice 7 — global daemon and Claude Code plugin ([#31](https://github.com/specgraph/specgraph/issues/31)) ([a8a07b4](https://github.com/specgraph/specgraph/commit/a8a07b47ed18fcc5e52de4c7423a7be30e772914))
* **proto:** add notes field to Spec + JSON output for show ([#35](https://github.com/specgraph/specgraph/issues/35)) ([524b09c](https://github.com/specgraph/specgraph/commit/524b09c990999f6c8840c9ab171ccbc776fe042f))
* restructure SpecifyOutput with InterfaceSection, VerifyCriterion, FileTouch ([#619](https://github.com/specgraph/specgraph/issues/619)) ([5ba9999](https://github.com/specgraph/specgraph/commit/5ba9999d255ef556472713c4fe09386af4569cda))
* restructure SpecifyOutput with structured sub-messages ([5ba9999](https://github.com/specgraph/specgraph/commit/5ba9999d255ef556472713c4fe09386af4569cda))
* Slice 3 — Authoring Funnel ([#8](https://github.com/specgraph/specgraph/issues/8)) ([8d15fd1](https://github.com/specgraph/specgraph/commit/8d15fd19d9e3df1102c6a2f5e4a1b17b1a077fca))
* supply chain security — cosign, SBOMs, attestations, Trivy scan ([#562](https://github.com/specgraph/specgraph/issues/562)) ([070e014](https://github.com/specgraph/specgraph/commit/070e014ccf4f4aa5cef41fbc80ddeba9b2e1c267))
* **sync:** Slice 6 — sync adapters, tool injection, and CLI ([#30](https://github.com/specgraph/specgraph/issues/30)) ([c4c6ae7](https://github.com/specgraph/specgraph/commit/c4c6ae716dfc3bad7418085a75b42c1b1a81a93b))
* vertical slice — client/server architecture ([#1](https://github.com/specgraph/specgraph/issues/1)) ([50b504c](https://github.com/specgraph/specgraph/commit/50b504c67167cd52ab43fd956536a38ca8bacc08))
* **web:** expand spec detail page with stage outputs, edges, and conversations (spgr-zn1) ([2638edd](https://github.com/specgraph/specgraph/commit/2638edd9c8cccca1d24e261476fdb4ed8487e0bf))
* **web:** expand spec detail page with stage outputs, edges, conversations (spgr-zn1) ([#668](https://github.com/specgraph/specgraph/issues/668)) ([2638edd](https://github.com/specgraph/specgraph/commit/2638edd9c8cccca1d24e261476fdb4ed8487e0bf))


### Bug Fixes

* add explicit step gating to shape skill ([#618](https://github.com/specgraph/specgraph/issues/618)) ([ca2cedc](https://github.com/specgraph/specgraph/commit/ca2cedcdf2f2be0a7c81542f36c4aaddc6fa5a6a))
* approve self-approval guardrails + CLI usage dump silence (spgr-8ec, spgr-5sd) ([#669](https://github.com/specgraph/specgraph/issues/669)) ([b984856](https://github.com/specgraph/specgraph/commit/b984856ea422b28e03835bc8915fe033517cb62d))
* bump cosign-installer v3→v4, attest-build-provenance v2→v4, trivy-action v0.35.0 ([b55924d](https://github.com/specgraph/specgraph/commit/b55924d9b19fa5b7988a19006b42034b0727816c))
* **ci:** build web UI before goreleaser in release workflow ([#674](https://github.com/specgraph/specgraph/issues/674)) ([0d9b96e](https://github.com/specgraph/specgraph/commit/0d9b96e49b08deb4a868fcc0cba439b8924f9993))
* coordinate release-please and goreleaser — draft release handoff ([#521](https://github.com/specgraph/specgraph/issues/521)) ([fc299c4](https://github.com/specgraph/specgraph/commit/fc299c49d5bc91037cdaa955e734d6a5a3c42fd4))
* correct trivy-action version tag (v0.35.0) ([#564](https://github.com/specgraph/specgraph/issues/564)) ([b55924d](https://github.com/specgraph/specgraph/commit/b55924d9b19fa5b7988a19006b42034b0727816c))
* **deps:** update module github.com/testcontainers/testcontainers-go to v0.41.0 ([#28](https://github.com/specgraph/specgraph/issues/28)) ([2de880e](https://github.com/specgraph/specgraph/commit/2de880e92923fa4e8accb0a32793656ecd323db5))
* **deps:** update module golang.org/x/text to v0.35.0 ([#29](https://github.com/specgraph/specgraph/issues/29)) ([81fb5bf](https://github.com/specgraph/specgraph/commit/81fb5bff3ebaeffcfce4ea255444ee65a0841d09))
* Dockerfile for goreleaser — use pre-built binary ([#519](https://github.com/specgraph/specgraph/issues/519)) ([92c243b](https://github.com/specgraph/specgraph/commit/92c243bf2ae5626d70c6e0f84b4f9240b6a48275))
* **e2e:** address 4 open test suite findings ([#44](https://github.com/specgraph/specgraph/issues/44)) ([a029036](https://github.com/specgraph/specgraph/commit/a0290368fd4a56618187358b082fc8974aeff185))
* enforce slug uniqueness in CreateSpec ([#615](https://github.com/specgraph/specgraph/issues/615)) ([5d57110](https://github.com/specgraph/specgraph/commit/5d57110e5c58372833a5a2197fd34d85190abe1d))
* goreleaser Dockerfile + multi-arch Docker images + bump GH actions ([92c243b](https://github.com/specgraph/specgraph/commit/92c243bf2ae5626d70c6e0f84b4f9240b6a48275))
* let goreleaser own GitHub releases, release-please only creates tags ([56051cf](https://github.com/specgraph/specgraph/commit/56051cfe16c34f680ae457388a741578e872679e))
* let goreleaser own GitHub releases, release-please only tags ([#528](https://github.com/specgraph/specgraph/issues/528)) ([56051cf](https://github.com/specgraph/specgraph/commit/56051cfe16c34f680ae457388a741578e872679e))
* pin cosign-installer to v4.1.0 (no floating v4 tag) ([#567](https://github.com/specgraph/specgraph/issues/567)) ([009ce40](https://github.com/specgraph/specgraph/commit/009ce40262766254ca003cd6e08c4be537dbe06f))
* push git tag to remote before goreleaser (changelog needs it) ([2f40fd2](https://github.com/specgraph/specgraph/commit/2f40fd2fb0cfd71f2cbb03b01c88bea89b69158c))
* push git tag to remote before goreleaser changelog ([#533](https://github.com/specgraph/specgraph/issues/533)) ([2f40fd2](https://github.com/specgraph/specgraph/commit/2f40fd2fb0cfd71f2cbb03b01c88bea89b69158c))
* release-please creates release+tag, goreleaser replaces with assets ([#530](https://github.com/specgraph/specgraph/issues/530)) ([8439c3a](https://github.com/specgraph/specgraph/commit/8439c3a7c6a510a1442be5b35d2ca61b22365178))
* remove draft:true from release-please config ([#525](https://github.com/specgraph/specgraph/issues/525)) ([7f2f138](https://github.com/specgraph/specgraph/commit/7f2f1389aec66fa060d6e87290bab9a51670e353))
* remove draft:true from release-please, add workflow_dispatch trigger ([7f2f138](https://github.com/specgraph/specgraph/commit/7f2f1389aec66fa060d6e87290bab9a51670e353))
* resolve absolute binary path and quote slugs in tool commands ([#616](https://github.com/specgraph/specgraph/issues/616)) ([e2e08f4](https://github.com/specgraph/specgraph/commit/e2e08f4d69272a4f2687d8cd054c2afeb9d9311c))
* simple release flow — release-please creates release, goreleaser uploads assets ([#524](https://github.com/specgraph/specgraph/issues/524)) ([7f7b024](https://github.com/specgraph/specgraph/commit/7f7b024a5ea36acef6152778f821be00f0281112))
* unified release workflow — draft release + goreleaser in single pipeline ([8439c3a](https://github.com/specgraph/specgraph/commit/8439c3a7c6a510a1442be5b35d2ca61b22365178))
* wrap all multi-query write paths in RunInTransaction ([#42](https://github.com/specgraph/specgraph/issues/42)) ([04045e8](https://github.com/specgraph/specgraph/commit/04045e82e64d0cf49af5531c2cbf48d3cd2d4888))


### Documentation

* add CLAUDE.md for specgraph subproject ([b7f25f0](https://github.com/specgraph/specgraph/commit/b7f25f03230bd7e10ce0373ea0064b2429a44944))
* add implementation plans for Slices 3-7 ([72a8f6e](https://github.com/specgraph/specgraph/commit/72a8f6ee837f66e6b63807daba90f6b3e8c7641a))
* add implementation tracker and verification gates ([9261e5a](https://github.com/specgraph/specgraph/commit/9261e5a479af00b48236d737ed9a6cd4e2210607))
* add Slice 2 Constitution implementation plan ([fd8cda6](https://github.com/specgraph/specgraph/commit/fd8cda6759596eed4acf83afd83b9bd7b1cab984))
* add top-level README and align site docs ([#18](https://github.com/specgraph/specgraph/issues/18)) ([60e1437](https://github.com/specgraph/specgraph/commit/60e1437457511c18c0fd7ad63ec175664a2feea9))
* add vertical slice roadmap design for remaining implementation ([e736eb7](https://github.com/specgraph/specgraph/commit/e736eb7c1c442c5ba61fdc194519c4e3d663e05e))
* design for storage domain types and decision promotion ([f754076](https://github.com/specgraph/specgraph/commit/f7540767d0d116176e7ccb9255836f95b2f28bc7))
* implementation plan for storage domain types and decision promotion ([cfe9d63](https://github.com/specgraph/specgraph/commit/cfe9d63e8eadab66f574ec95e65ed55a2f50705d))
* mark slices 2-3 complete, remove stale rumdl exclude ([1a9c5c2](https://github.com/specgraph/specgraph/commit/1a9c5c22a40956316997932f624e688f4214d23d))
* Quick Start guide and documentation overhaul for 0.1.0 ([#515](https://github.com/specgraph/specgraph/issues/515)) ([a3c0766](https://github.com/specgraph/specgraph/commit/a3c07665fd825fca692b0bcac4752d04d9f3cff9))
* **site:** add example spec page ([#33](https://github.com/specgraph/specgraph/issues/33)) ([3c719f6](https://github.com/specgraph/specgraph/commit/3c719f65f2cc73492c2f4a9a59134467b3dad597))
* **site:** add example spec page with worked OAuth2 rotation scenario ([3c719f6](https://github.com/specgraph/specgraph/commit/3c719f65f2cc73492c2f4a9a59134467b3dad597))
* **site:** build out documentation site ([#9](https://github.com/specgraph/specgraph/issues/9)) ([66af3dc](https://github.com/specgraph/specgraph/commit/66af3dca602d5f926b20739c51c3775c319bbb16))
* sync site docs and README with current codebase ([bd71843](https://github.com/specgraph/specgraph/commit/bd7184358633c4f6e9dac63f9038acf878440079))
* update CLAUDE.md and add Claude Code automations ([9d17883](https://github.com/specgraph/specgraph/commit/9d1788359a70f05ea3ae71380d9778c3b7b36b8e))
* update CLAUDE.md with test gotchas, remove stale status ([3df0d54](https://github.com/specgraph/specgraph/commit/3df0d54cd153755cdd2fca13ec86e82a695e0acb))


### Performance

* share single memgraph container across integration tests ([#516](https://github.com/specgraph/specgraph/issues/516)) ([a95ac45](https://github.com/specgraph/specgraph/commit/a95ac459e6fe63457cb957ec44d0444edb891b87))
* share single memgraph container across integration tests (spgr-mfot) ([a95ac45](https://github.com/specgraph/specgraph/commit/a95ac459e6fe63457cb957ec44d0444edb891b87))


### Code Refactoring

* **proto:** wrap bare-entity RPC returns in Response messages ([#622](https://github.com/specgraph/specgraph/issues/622)) ([b724308](https://github.com/specgraph/specgraph/commit/b72430893fdf36f04ee9b54e6f3046c87f733faf))
* Slice 3.5 — Scanner removal & documentation cleanup ([#22](https://github.com/specgraph/specgraph/issues/22)) ([f06a476](https://github.com/specgraph/specgraph/commit/f06a47685fe1ce27ed5a265ff209448bd04b414c))
* split AnalyticalFinding into input/output types ([#617](https://github.com/specgraph/specgraph/issues/617)) ([12a558d](https://github.com/specgraph/specgraph/commit/12a558dc305dba503a3ad4b1a73be90c6c8e5afd))
* storage domain types and decision promotion ([#24](https://github.com/specgraph/specgraph/issues/24)) ([836abee](https://github.com/specgraph/specgraph/commit/836abeea8a96d04898d874aaddc6b4a574850690))


### Tests

* add comprehensive E2E test system ([#19](https://github.com/specgraph/specgraph/issues/19)) ([6ecf4e5](https://github.com/specgraph/specgraph/commit/6ecf4e585a21a252fdc18e16e4a6ebcfc109310c))
* **e2e:** implement 3-tier E2E test suite ([#32](https://github.com/specgraph/specgraph/issues/32)) ([de12bbc](https://github.com/specgraph/specgraph/commit/de12bbceaa1f437ee37d98f4d76e7f2f7817611f))
* **e2e:** implement 3-tier E2E test suite with full design doc coverage ([de12bbc](https://github.com/specgraph/specgraph/commit/de12bbceaa1f437ee37d98f4d76e7f2f7817611f))
* **integration:** add DISTINCT regression test for GetExecutionEvents ([#37](https://github.com/specgraph/specgraph/issues/37)) ([2b17445](https://github.com/specgraph/specgraph/commit/2b17445a8421f114d6f34ef3f1fca361afa32dcc))
* **spgr-g8i.16:** add diamond+cycle regression tests for detectCycles ([5adf681](https://github.com/specgraph/specgraph/commit/5adf6813d3bccc7bd16b7279a90e9f451a8dc634))


### CI

* add release-please + goreleaser infrastructure ([#46](https://github.com/specgraph/specgraph/issues/46)) ([1fd22d3](https://github.com/specgraph/specgraph/commit/1fd22d3d9ab3c80360a5e0d9117741192ddd26b8))
* exclude auto-generated CHANGELOG.md from markdown lint ([#517](https://github.com/specgraph/specgraph/issues/517)) ([7106861](https://github.com/specgraph/specgraph/commit/71068619c63a7a7f9749fa98e44287dceed001e3))
* exclude CHANGELOG.md from lint, use PAT for release-please to trigger CI on release PRs ([92805cf](https://github.com/specgraph/specgraph/commit/92805cfff0727fa5287349ab7faec36ecbef6c0f))
* use PAT for release-please to trigger CI on release PRs ([#518](https://github.com/specgraph/specgraph/issues/518)) ([92805cf](https://github.com/specgraph/specgraph/commit/92805cfff0727fa5287349ab7faec36ecbef6c0f))


### Build

* **deps:** bump golang.org/x/crypto from 0.43.0 to 0.45.0 ([#2](https://github.com/specgraph/specgraph/issues/2)) ([a4b88f8](https://github.com/specgraph/specgraph/commit/a4b88f82d2c7b71fbd89a48db4fb48a1d34b5b87))


### Miscellaneous

* add beads ([#5](https://github.com/specgraph/specgraph/issues/5)) ([d10d49d](https://github.com/specgraph/specgraph/commit/d10d49d4157b1376c5a646eff87bd13d63256ee2))
* add Claude Code plugin symlink for specgraph skills ([#608](https://github.com/specgraph/specgraph/issues/608)) ([90ac791](https://github.com/specgraph/specgraph/commit/90ac791835cd2f4aae321c5b0a52d335a68dbb83))
* beads updates ([#666](https://github.com/specgraph/specgraph/issues/666)) ([961da26](https://github.com/specgraph/specgraph/commit/961da262428e09ae05afc7ab02269da104295ea9))
* beads updates for sharing ([#667](https://github.com/specgraph/specgraph/issues/667)) ([ab0b1a6](https://github.com/specgraph/specgraph/commit/ab0b1a67f30016f46d5802f12b375346949960f0))
* begin 0.2.0 development ([#570](https://github.com/specgraph/specgraph/issues/570)) ([679732a](https://github.com/specgraph/specgraph/commit/679732a77a679ec96e2e2c83105052dc68bada4e))
* **ci:** pin actions to SHA, add govulncheck, codecov, semgrep, renovate, badges (spgr-o49) ([05f281f](https://github.com/specgraph/specgraph/commit/05f281f45f65bac015c9fe5f42374b0e36bfd8ae))
* **ci:** supply chain hardening — pin actions, govulncheck, codecov, semgrep, renovate (spgr-o49) ([#671](https://github.com/specgraph/specgraph/issues/671)) ([05f281f](https://github.com/specgraph/specgraph/commit/05f281f45f65bac015c9fe5f42374b0e36bfd8ae))
* Configure Renovate ([#6](https://github.com/specgraph/specgraph/issues/6)) ([0a627bf](https://github.com/specgraph/specgraph/commit/0a627bf4519521433eb9e151a33795148bced6c2))
* **deps:** update actions/cache action to v5 ([#25](https://github.com/specgraph/specgraph/issues/25)) ([13d90f5](https://github.com/specgraph/specgraph/commit/13d90f5a42e549a7b429b31e27a4c1373348384c))
* **deps:** update actions/checkout action to v6 ([#14](https://github.com/specgraph/specgraph/issues/14)) ([a6b4f1c](https://github.com/specgraph/specgraph/commit/a6b4f1ca68e896fc37e3598a9a910877a7ec769a))
* **deps:** update actions/setup-go action to v6 ([#21](https://github.com/specgraph/specgraph/issues/21)) ([7ecfca8](https://github.com/specgraph/specgraph/commit/7ecfca8babb52db21b16819005c6e3897189b852))
* **deps:** update actions/upload-pages-artifact action to v4 ([#15](https://github.com/specgraph/specgraph/issues/15)) ([f86df24](https://github.com/specgraph/specgraph/commit/f86df24a7140b5642883c44b7643312e0fe6f32a))
* **deps:** update alpine docker tag to v3.23 ([#10](https://github.com/specgraph/specgraph/issues/10)) ([55da31a](https://github.com/specgraph/specgraph/commit/55da31abfc77d132e30a0ad3872cab39e34d9aeb))
* **deps:** update astral-sh/setup-uv action to v7 ([#16](https://github.com/specgraph/specgraph/issues/16)) ([fa69828](https://github.com/specgraph/specgraph/commit/fa6982887065c9c81db416008791c9b4b551056a))
* **deps:** update dependency go to 1.26 ([#20](https://github.com/specgraph/specgraph/issues/20)) ([4e3718e](https://github.com/specgraph/specgraph/commit/4e3718e5568f31c2ad437679dd7b036237b20efe))
* **deps:** update golang docker tag to v1.26 ([#11](https://github.com/specgraph/specgraph/issues/11)) ([ebf12c5](https://github.com/specgraph/specgraph/commit/ebf12c5f0e781bd242b53cde75a486f89b26ed31))
* force 0.2.1 release ([#675](https://github.com/specgraph/specgraph/issues/675)) ([19d1172](https://github.com/specgraph/specgraph/commit/19d117282e6ef10f62e5b4e859a34ad81e626476))
* **main:** release 0.1.0 ([#49](https://github.com/specgraph/specgraph/issues/49)) ([fcd4b81](https://github.com/specgraph/specgraph/commit/fcd4b81df5000c6c4759a5f6cf6c0cad697a2532))
* **main:** release 0.1.1 ([#520](https://github.com/specgraph/specgraph/issues/520)) ([ef70ae7](https://github.com/specgraph/specgraph/commit/ef70ae7a1be886d6a5de2b43c4ad6f00a840c6fb))
* **main:** release 0.1.2 ([#522](https://github.com/specgraph/specgraph/issues/522)) ([b463d18](https://github.com/specgraph/specgraph/commit/b463d185ca6db602f593eaf30e69bfd4073d49a8))
* **main:** release 0.1.3 ([#527](https://github.com/specgraph/specgraph/issues/527)) ([7e1b255](https://github.com/specgraph/specgraph/commit/7e1b25579aa073eb919e2d1b0725ed818802f350))
* **main:** release 0.1.4 ([#529](https://github.com/specgraph/specgraph/issues/529)) ([dfbb73e](https://github.com/specgraph/specgraph/commit/dfbb73e98b8f61b8d556c34acdf9e8a81c129944))
* **main:** release 0.1.4 ([#531](https://github.com/specgraph/specgraph/issues/531)) ([4b2bc6c](https://github.com/specgraph/specgraph/commit/4b2bc6cff80ef111678f26b322f523b581703a01))
* **main:** release 0.1.5 ([#532](https://github.com/specgraph/specgraph/issues/532)) ([5cbcef5](https://github.com/specgraph/specgraph/commit/5cbcef5e47a60db7d3bc46a9ce7da78b0948ccf4))
* **main:** release 0.1.5 ([#563](https://github.com/specgraph/specgraph/issues/563)) ([2426b98](https://github.com/specgraph/specgraph/commit/2426b98d28113dc60b0e1106891efc68af3479f3))
* **main:** release 0.1.5 ([#565](https://github.com/specgraph/specgraph/issues/565)) ([8608023](https://github.com/specgraph/specgraph/commit/8608023bda623c0d8a59eebc7ed1f09d266075cc))
* **main:** release 0.1.5 ([#566](https://github.com/specgraph/specgraph/issues/566)) ([02c08bb](https://github.com/specgraph/specgraph/commit/02c08bbe66cac2f50a993371d9a2eaf8989acba2))
* **main:** release 0.1.6 ([8608023](https://github.com/specgraph/specgraph/commit/8608023bda623c0d8a59eebc7ed1f09d266075cc))
* **main:** release 0.1.6 ([#534](https://github.com/specgraph/specgraph/issues/534)) ([bc2ccd3](https://github.com/specgraph/specgraph/commit/bc2ccd3ba652c210608cf8c2f0edd00b23e2b38b))
* **main:** release 0.1.6 ([#568](https://github.com/specgraph/specgraph/issues/568)) ([86c7b44](https://github.com/specgraph/specgraph/commit/86c7b442e42b51344d540ac3bd885df5e1d18807))
* **main:** release 0.2.0 ([#569](https://github.com/specgraph/specgraph/issues/569)) ([85a3b89](https://github.com/specgraph/specgraph/commit/85a3b895c0950a15f6b80e9b0a134a6560146715))
* **main:** release 0.2.1 ([#673](https://github.com/specgraph/specgraph/issues/673)) ([3969d46](https://github.com/specgraph/specgraph/commit/3969d465501fa39b5536f3e7c7a5b881a8dbc02e))
* move module path to specgraph/specgraph ([#45](https://github.com/specgraph/specgraph/issues/45)) ([fb084cb](https://github.com/specgraph/specgraph/commit/fb084cb01a4c340e12a764d5464b43a75d2726e1))
* move repo from seanb4t/specgraph to specgraph/specgraph ([fb084cb](https://github.com/specgraph/specgraph/commit/fb084cb01a4c340e12a764d5464b43a75d2726e1))
* release 0.1.0 ([#48](https://github.com/specgraph/specgraph/issues/48)) ([31e695b](https://github.com/specgraph/specgraph/commit/31e695ba6b608b33248724154ff0fefb92c5b27e))
* trigger release 0.1.3 ([#526](https://github.com/specgraph/specgraph/issues/526)) ([4a92f1b](https://github.com/specgraph/specgraph/commit/4a92f1b33a8cde4b12070768d09a390443555115))

## [0.2.1](https://github.com/specgraph/specgraph/compare/v0.2.0...v0.2.1) (2026-03-26)


### Bug Fixes

* **ci:** build web UI before goreleaser in release workflow ([#674](https://github.com/specgraph/specgraph/issues/674)) ([0d9b96e](https://github.com/specgraph/specgraph/commit/0d9b96e49b08deb4a868fcc0cba439b8924f9993))


### Miscellaneous

* force 0.2.1 release ([#675](https://github.com/specgraph/specgraph/issues/675)) ([19d1172](https://github.com/specgraph/specgraph/commit/19d117282e6ef10f62e5b4e859a34ad81e626476))

## [0.2.0](https://github.com/specgraph/specgraph/compare/v0.1.6...v0.2.0) (2026-03-26)


### Features

* add ConversationLog graph nodes for authoring audit trail (spgr-9mz) ([140341b](https://github.com/specgraph/specgraph/commit/140341be9a1986d07ef2a8c1b76d5f96184a1847))
* add duplicate slug check to spark skill ([#620](https://github.com/specgraph/specgraph/issues/620)) ([4655650](https://github.com/specgraph/specgraph/commit/4655650785be3436126e709bd0d8ba5b8f5980f2))
* add prompt templates for remaining analytical passes ([#572](https://github.com/specgraph/specgraph/issues/572)) ([e28ab1c](https://github.com/specgraph/specgraph/commit/e28ab1cae0ac3e1635a053b823e7f4b76b0333ab))
* analytical pass system with unified findings ([#571](https://github.com/specgraph/specgraph/issues/571)) ([e8f455e](https://github.com/specgraph/specgraph/commit/e8f455e0198740bd0caf01cd024ed8f54b313f1a))
* batch improvements — dashboard stats, constitution view, status command, template overrides, bug fixes ([#670](https://github.com/specgraph/specgraph/issues/670)) ([c8aa122](https://github.com/specgraph/specgraph/commit/c8aa1224980209cb44317c8c4407c1ba8aa0284d))
* batch improvements + fix skill format references (spgr-r40, spgr-qdl, spgr-qu2, spgr-9n8, spgr-8ec, spgr-5sd, spgr-p9h) ([c8aa122](https://github.com/specgraph/specgraph/commit/c8aa1224980209cb44317c8c4407c1ba8aa0284d))
* batch improvements + fix skill format references (spgr-r40, spgr-qdl, spgr-qu2, spgr-9n8, spgr-8ec, spgr-5sd, spgr-p9h) ([b984856](https://github.com/specgraph/specgraph/commit/b984856ea422b28e03835bc8915fe033517cb62d))
* **cmd:** add comprehensive unit tests for CLI commands (spgr-dvb) ([#672](https://github.com/specgraph/specgraph/issues/672)) ([eb4d5f4](https://github.com/specgraph/specgraph/commit/eb4d5f4516830389275c4986af03b2fdd1bdfe15))
* ConversationLog graph nodes for authoring audit trail (spgr-9mz) ([#664](https://github.com/specgraph/specgraph/issues/664)) ([140341b](https://github.com/specgraph/specgraph/commit/140341be9a1986d07ef2a8c1b76d5f96184a1847))
* expand specgraph show with full authoring stage detail (spgr-0dg) ([#665](https://github.com/specgraph/specgraph/issues/665)) ([8fc8412](https://github.com/specgraph/specgraph/commit/8fc8412dbc3fad04b59e1e83ecd9e47fdd06bda9))
* graph visualization UI with embedded SvelteKit SPA (spgr-p1l) ([#644](https://github.com/specgraph/specgraph/issues/644)) ([d66606a](https://github.com/specgraph/specgraph/commit/d66606adb84e2e4a5a9238691b11bac3c31e0f0f))
* markdown CLI output with --json flags ([#496](https://github.com/specgraph/specgraph/issues/496)) ([#642](https://github.com/specgraph/specgraph/issues/642)) ([0f3bbe8](https://github.com/specgraph/specgraph/commit/0f3bbe81779981eb11d56431117efab75324012c))
* plugin restructure, authoring skills, and demo runbook ([#609](https://github.com/specgraph/specgraph/issues/609)) ([b12f199](https://github.com/specgraph/specgraph/commit/b12f1992c22fe24789070dfa172c2852e08f83ea))
* restructure SpecifyOutput with InterfaceSection, VerifyCriterion, FileTouch ([#619](https://github.com/specgraph/specgraph/issues/619)) ([5ba9999](https://github.com/specgraph/specgraph/commit/5ba9999d255ef556472713c4fe09386af4569cda))
* restructure SpecifyOutput with structured sub-messages ([5ba9999](https://github.com/specgraph/specgraph/commit/5ba9999d255ef556472713c4fe09386af4569cda))
* **web:** expand spec detail page with stage outputs, edges, and conversations (spgr-zn1) ([2638edd](https://github.com/specgraph/specgraph/commit/2638edd9c8cccca1d24e261476fdb4ed8487e0bf))
* **web:** expand spec detail page with stage outputs, edges, conversations (spgr-zn1) ([#668](https://github.com/specgraph/specgraph/issues/668)) ([2638edd](https://github.com/specgraph/specgraph/commit/2638edd9c8cccca1d24e261476fdb4ed8487e0bf))


### Bug Fixes

* add explicit step gating to shape skill ([#618](https://github.com/specgraph/specgraph/issues/618)) ([ca2cedc](https://github.com/specgraph/specgraph/commit/ca2cedcdf2f2be0a7c81542f36c4aaddc6fa5a6a))
* approve self-approval guardrails + CLI usage dump silence (spgr-8ec, spgr-5sd) ([#669](https://github.com/specgraph/specgraph/issues/669)) ([b984856](https://github.com/specgraph/specgraph/commit/b984856ea422b28e03835bc8915fe033517cb62d))
* enforce slug uniqueness in CreateSpec ([#615](https://github.com/specgraph/specgraph/issues/615)) ([5d57110](https://github.com/specgraph/specgraph/commit/5d57110e5c58372833a5a2197fd34d85190abe1d))
* resolve absolute binary path and quote slugs in tool commands ([#616](https://github.com/specgraph/specgraph/issues/616)) ([e2e08f4](https://github.com/specgraph/specgraph/commit/e2e08f4d69272a4f2687d8cd054c2afeb9d9311c))


### Code Refactoring

* **proto:** wrap bare-entity RPC returns in Response messages ([#622](https://github.com/specgraph/specgraph/issues/622)) ([b724308](https://github.com/specgraph/specgraph/commit/b72430893fdf36f04ee9b54e6f3046c87f733faf))
* split AnalyticalFinding into input/output types ([#617](https://github.com/specgraph/specgraph/issues/617)) ([12a558d](https://github.com/specgraph/specgraph/commit/12a558dc305dba503a3ad4b1a73be90c6c8e5afd))


### Miscellaneous

* add Claude Code plugin symlink for specgraph skills ([#608](https://github.com/specgraph/specgraph/issues/608)) ([90ac791](https://github.com/specgraph/specgraph/commit/90ac791835cd2f4aae321c5b0a52d335a68dbb83))
* beads updates ([#666](https://github.com/specgraph/specgraph/issues/666)) ([961da26](https://github.com/specgraph/specgraph/commit/961da262428e09ae05afc7ab02269da104295ea9))
* beads updates for sharing ([#667](https://github.com/specgraph/specgraph/issues/667)) ([ab0b1a6](https://github.com/specgraph/specgraph/commit/ab0b1a67f30016f46d5802f12b375346949960f0))
* begin 0.2.0 development ([#570](https://github.com/specgraph/specgraph/issues/570)) ([679732a](https://github.com/specgraph/specgraph/commit/679732a77a679ec96e2e2c83105052dc68bada4e))
* **ci:** pin actions to SHA, add govulncheck, codecov, semgrep, renovate, badges (spgr-o49) ([05f281f](https://github.com/specgraph/specgraph/commit/05f281f45f65bac015c9fe5f42374b0e36bfd8ae))
* **ci:** supply chain hardening — pin actions, govulncheck, codecov, semgrep, renovate (spgr-o49) ([#671](https://github.com/specgraph/specgraph/issues/671)) ([05f281f](https://github.com/specgraph/specgraph/commit/05f281f45f65bac015c9fe5f42374b0e36bfd8ae))

## [0.1.6](https://github.com/specgraph/specgraph/compare/v0.1.5...v0.1.6) (2026-03-21)


### Features

* supply chain security — cosign, SBOMs, attestations, Trivy scan ([#562](https://github.com/specgraph/specgraph/issues/562)) ([070e014](https://github.com/specgraph/specgraph/commit/070e014ccf4f4aa5cef41fbc80ddeba9b2e1c267))


### Bug Fixes

* bump cosign-installer v3→v4, attest-build-provenance v2→v4, trivy-action v0.35.0 ([b55924d](https://github.com/specgraph/specgraph/commit/b55924d9b19fa5b7988a19006b42034b0727816c))
* correct trivy-action version tag (v0.35.0) ([#564](https://github.com/specgraph/specgraph/issues/564)) ([b55924d](https://github.com/specgraph/specgraph/commit/b55924d9b19fa5b7988a19006b42034b0727816c))
* pin cosign-installer to v4.1.0 (no floating v4 tag) ([#567](https://github.com/specgraph/specgraph/issues/567)) ([009ce40](https://github.com/specgraph/specgraph/commit/009ce40262766254ca003cd6e08c4be537dbe06f))


### Miscellaneous

* **main:** release 0.1.5 ([#563](https://github.com/specgraph/specgraph/issues/563)) ([2426b98](https://github.com/specgraph/specgraph/commit/2426b98d28113dc60b0e1106891efc68af3479f3))
* **main:** release 0.1.5 ([#565](https://github.com/specgraph/specgraph/issues/565)) ([8608023](https://github.com/specgraph/specgraph/commit/8608023bda623c0d8a59eebc7ed1f09d266075cc))
* **main:** release 0.1.5 ([#566](https://github.com/specgraph/specgraph/issues/566)) ([02c08bb](https://github.com/specgraph/specgraph/commit/02c08bbe66cac2f50a993371d9a2eaf8989acba2))
* **main:** release 0.1.6 ([8608023](https://github.com/specgraph/specgraph/commit/8608023bda623c0d8a59eebc7ed1f09d266075cc))
* **main:** release 0.1.6 ([#534](https://github.com/specgraph/specgraph/issues/534)) ([bc2ccd3](https://github.com/specgraph/specgraph/commit/bc2ccd3ba652c210608cf8c2f0edd00b23e2b38b))

## [0.1.5](https://github.com/specgraph/specgraph/compare/v0.1.6...v0.1.5) (2026-03-21)


### Features

* add code quality and lefthook setup ([#3](https://github.com/specgraph/specgraph/issues/3)) ([970664e](https://github.com/specgraph/specgraph/commit/970664ea5a5a44ece3557eff3c9e247e1e009a88))
* add constitution subsystem (Slice 2) ([#7](https://github.com/specgraph/specgraph/issues/7)) ([10c2ee3](https://github.com/specgraph/specgraph/commit/10c2ee3180a2bf11dd8c179cb4ea4e018f54ace7))
* add extended services (health, claim, decision, graph) ([#4](https://github.com/specgraph/specgraph/issues/4)) ([9fd18e5](https://github.com/specgraph/specgraph/commit/9fd18e5496d5d664c9be4f72e04a583d573f4d5e))
* add Murmur3-128 content hash for change detection ([#39](https://github.com/specgraph/specgraph/issues/39)) ([b3c10b2](https://github.com/specgraph/specgraph/commit/b3c10b2f37f3ab1a9de5a6553ce63a656e48bb52))
* add Zensical doc site with GitHub Pages deployment ([7a1410e](https://github.com/specgraph/specgraph/commit/7a1410e0ae39485c3f7540ddaf8affc21cfd6cbd))
* **auth:** add authentication and authorization interceptor ([#38](https://github.com/specgraph/specgraph/issues/38)) ([f4fc6bf](https://github.com/specgraph/specgraph/commit/f4fc6bf2338020d521fe5ef626da2f8f5be2e1d5))
* ChangeLog graph nodes for version tracking ([#41](https://github.com/specgraph/specgraph/issues/41)) ([e5c00dc](https://github.com/specgraph/specgraph/commit/e5c00dc2def9d8cd408e327afdf5b38f94b3c212))
* **cli:** add report-progress, report-blocker, report-completion commands ([#36](https://github.com/specgraph/specgraph/issues/36)) ([18b09bb](https://github.com/specgraph/specgraph/commit/18b09bb8fb6a6a878fb8c4cc87baad8d9acfb640))
* content hash drift detection on DEPENDS_ON edges ([#43](https://github.com/specgraph/specgraph/issues/43)) ([6c86b33](https://github.com/specgraph/specgraph/commit/6c86b33fe59326557a309d1fcddf098bef0b5df3))
* **docker:** add Memgraph sizing profiles and persistence ([#23](https://github.com/specgraph/specgraph/issues/23)) ([9a2ab3f](https://github.com/specgraph/specgraph/commit/9a2ab3f82367204c9c880086b0f69e4bdb810a6a))
* **execution:** Slice 4 — domain types consistency & execution bundles ([#26](https://github.com/specgraph/specgraph/issues/26)) ([9942813](https://github.com/specgraph/specgraph/commit/9942813353c8afeb930d5de68aec808079fc338b))
* include design docs as hidden pages on site ([3f986a1](https://github.com/specgraph/specgraph/commit/3f986a1753269629b69c8c2baf2cfc8cfde0abe5))
* initial ([a46c950](https://github.com/specgraph/specgraph/commit/a46c950af7c44cf0d101bb9895878698dd5bf0d1))
* **lifecycle:** Slice 5 — spec lifecycle operations ([#27](https://github.com/specgraph/specgraph/issues/27)) ([5adf681](https://github.com/specgraph/specgraph/commit/5adf6813d3bccc7bd16b7279a90e9f451a8dc634))
* **plugin:** evolve authoring skills from CLI reference cards to partner personas ([c260969](https://github.com/specgraph/specgraph/commit/c260969862c3e536765ec26a15700f5d39eed1a5))
* **plugin:** evolve authoring skills into partner personas ([#34](https://github.com/specgraph/specgraph/issues/34)) ([c260969](https://github.com/specgraph/specgraph/commit/c260969862c3e536765ec26a15700f5d39eed1a5))
* **plugin:** Slice 7 — global daemon and Claude Code plugin ([#31](https://github.com/specgraph/specgraph/issues/31)) ([a8a07b4](https://github.com/specgraph/specgraph/commit/a8a07b47ed18fcc5e52de4c7423a7be30e772914))
* **proto:** add notes field to Spec + JSON output for show ([#35](https://github.com/specgraph/specgraph/issues/35)) ([524b09c](https://github.com/specgraph/specgraph/commit/524b09c990999f6c8840c9ab171ccbc776fe042f))
* Slice 3 — Authoring Funnel ([#8](https://github.com/specgraph/specgraph/issues/8)) ([8d15fd1](https://github.com/specgraph/specgraph/commit/8d15fd19d9e3df1102c6a2f5e4a1b17b1a077fca))
* supply chain security — cosign, SBOMs, attestations, Trivy scan ([#562](https://github.com/specgraph/specgraph/issues/562)) ([070e014](https://github.com/specgraph/specgraph/commit/070e014ccf4f4aa5cef41fbc80ddeba9b2e1c267))
* **sync:** Slice 6 — sync adapters, tool injection, and CLI ([#30](https://github.com/specgraph/specgraph/issues/30)) ([c4c6ae7](https://github.com/specgraph/specgraph/commit/c4c6ae716dfc3bad7418085a75b42c1b1a81a93b))
* vertical slice — client/server architecture ([#1](https://github.com/specgraph/specgraph/issues/1)) ([50b504c](https://github.com/specgraph/specgraph/commit/50b504c67167cd52ab43fd956536a38ca8bacc08))


### Bug Fixes

* bump cosign-installer v3→v4, attest-build-provenance v2→v4, trivy-action v0.35.0 ([b55924d](https://github.com/specgraph/specgraph/commit/b55924d9b19fa5b7988a19006b42034b0727816c))
* coordinate release-please and goreleaser — draft release handoff ([#521](https://github.com/specgraph/specgraph/issues/521)) ([fc299c4](https://github.com/specgraph/specgraph/commit/fc299c49d5bc91037cdaa955e734d6a5a3c42fd4))
* correct trivy-action version tag (v0.35.0) ([#564](https://github.com/specgraph/specgraph/issues/564)) ([b55924d](https://github.com/specgraph/specgraph/commit/b55924d9b19fa5b7988a19006b42034b0727816c))
* **deps:** update module github.com/testcontainers/testcontainers-go to v0.41.0 ([#28](https://github.com/specgraph/specgraph/issues/28)) ([2de880e](https://github.com/specgraph/specgraph/commit/2de880e92923fa4e8accb0a32793656ecd323db5))
* **deps:** update module golang.org/x/text to v0.35.0 ([#29](https://github.com/specgraph/specgraph/issues/29)) ([81fb5bf](https://github.com/specgraph/specgraph/commit/81fb5bff3ebaeffcfce4ea255444ee65a0841d09))
* Dockerfile for goreleaser — use pre-built binary ([#519](https://github.com/specgraph/specgraph/issues/519)) ([92c243b](https://github.com/specgraph/specgraph/commit/92c243bf2ae5626d70c6e0f84b4f9240b6a48275))
* **e2e:** address 4 open test suite findings ([#44](https://github.com/specgraph/specgraph/issues/44)) ([a029036](https://github.com/specgraph/specgraph/commit/a0290368fd4a56618187358b082fc8974aeff185))
* goreleaser Dockerfile + multi-arch Docker images + bump GH actions ([92c243b](https://github.com/specgraph/specgraph/commit/92c243bf2ae5626d70c6e0f84b4f9240b6a48275))
* let goreleaser own GitHub releases, release-please only creates tags ([56051cf](https://github.com/specgraph/specgraph/commit/56051cfe16c34f680ae457388a741578e872679e))
* let goreleaser own GitHub releases, release-please only tags ([#528](https://github.com/specgraph/specgraph/issues/528)) ([56051cf](https://github.com/specgraph/specgraph/commit/56051cfe16c34f680ae457388a741578e872679e))
* pin cosign-installer to v4.1.0 (no floating v4 tag) ([#567](https://github.com/specgraph/specgraph/issues/567)) ([009ce40](https://github.com/specgraph/specgraph/commit/009ce40262766254ca003cd6e08c4be537dbe06f))
* push git tag to remote before goreleaser (changelog needs it) ([2f40fd2](https://github.com/specgraph/specgraph/commit/2f40fd2fb0cfd71f2cbb03b01c88bea89b69158c))
* push git tag to remote before goreleaser changelog ([#533](https://github.com/specgraph/specgraph/issues/533)) ([2f40fd2](https://github.com/specgraph/specgraph/commit/2f40fd2fb0cfd71f2cbb03b01c88bea89b69158c))
* release-please creates release+tag, goreleaser replaces with assets ([#530](https://github.com/specgraph/specgraph/issues/530)) ([8439c3a](https://github.com/specgraph/specgraph/commit/8439c3a7c6a510a1442be5b35d2ca61b22365178))
* remove draft:true from release-please config ([#525](https://github.com/specgraph/specgraph/issues/525)) ([7f2f138](https://github.com/specgraph/specgraph/commit/7f2f1389aec66fa060d6e87290bab9a51670e353))
* remove draft:true from release-please, add workflow_dispatch trigger ([7f2f138](https://github.com/specgraph/specgraph/commit/7f2f1389aec66fa060d6e87290bab9a51670e353))
* simple release flow — release-please creates release, goreleaser uploads assets ([#524](https://github.com/specgraph/specgraph/issues/524)) ([7f7b024](https://github.com/specgraph/specgraph/commit/7f7b024a5ea36acef6152778f821be00f0281112))
* unified release workflow — draft release + goreleaser in single pipeline ([8439c3a](https://github.com/specgraph/specgraph/commit/8439c3a7c6a510a1442be5b35d2ca61b22365178))
* wrap all multi-query write paths in RunInTransaction ([#42](https://github.com/specgraph/specgraph/issues/42)) ([04045e8](https://github.com/specgraph/specgraph/commit/04045e82e64d0cf49af5531c2cbf48d3cd2d4888))


### Documentation

* add CLAUDE.md for specgraph subproject ([b7f25f0](https://github.com/specgraph/specgraph/commit/b7f25f03230bd7e10ce0373ea0064b2429a44944))
* add implementation plans for Slices 3-7 ([72a8f6e](https://github.com/specgraph/specgraph/commit/72a8f6ee837f66e6b63807daba90f6b3e8c7641a))
* add implementation tracker and verification gates ([9261e5a](https://github.com/specgraph/specgraph/commit/9261e5a479af00b48236d737ed9a6cd4e2210607))
* add Slice 2 Constitution implementation plan ([fd8cda6](https://github.com/specgraph/specgraph/commit/fd8cda6759596eed4acf83afd83b9bd7b1cab984))
* add top-level README and align site docs ([#18](https://github.com/specgraph/specgraph/issues/18)) ([60e1437](https://github.com/specgraph/specgraph/commit/60e1437457511c18c0fd7ad63ec175664a2feea9))
* add vertical slice roadmap design for remaining implementation ([e736eb7](https://github.com/specgraph/specgraph/commit/e736eb7c1c442c5ba61fdc194519c4e3d663e05e))
* design for storage domain types and decision promotion ([f754076](https://github.com/specgraph/specgraph/commit/f7540767d0d116176e7ccb9255836f95b2f28bc7))
* implementation plan for storage domain types and decision promotion ([cfe9d63](https://github.com/specgraph/specgraph/commit/cfe9d63e8eadab66f574ec95e65ed55a2f50705d))
* mark slices 2-3 complete, remove stale rumdl exclude ([1a9c5c2](https://github.com/specgraph/specgraph/commit/1a9c5c22a40956316997932f624e688f4214d23d))
* Quick Start guide and documentation overhaul for 0.1.0 ([#515](https://github.com/specgraph/specgraph/issues/515)) ([a3c0766](https://github.com/specgraph/specgraph/commit/a3c07665fd825fca692b0bcac4752d04d9f3cff9))
* **site:** add example spec page ([#33](https://github.com/specgraph/specgraph/issues/33)) ([3c719f6](https://github.com/specgraph/specgraph/commit/3c719f65f2cc73492c2f4a9a59134467b3dad597))
* **site:** add example spec page with worked OAuth2 rotation scenario ([3c719f6](https://github.com/specgraph/specgraph/commit/3c719f65f2cc73492c2f4a9a59134467b3dad597))
* **site:** build out documentation site ([#9](https://github.com/specgraph/specgraph/issues/9)) ([66af3dc](https://github.com/specgraph/specgraph/commit/66af3dca602d5f926b20739c51c3775c319bbb16))
* sync site docs and README with current codebase ([bd71843](https://github.com/specgraph/specgraph/commit/bd7184358633c4f6e9dac63f9038acf878440079))
* update CLAUDE.md and add Claude Code automations ([9d17883](https://github.com/specgraph/specgraph/commit/9d1788359a70f05ea3ae71380d9778c3b7b36b8e))
* update CLAUDE.md with test gotchas, remove stale status ([3df0d54](https://github.com/specgraph/specgraph/commit/3df0d54cd153755cdd2fca13ec86e82a695e0acb))


### Performance

* share single memgraph container across integration tests ([#516](https://github.com/specgraph/specgraph/issues/516)) ([a95ac45](https://github.com/specgraph/specgraph/commit/a95ac459e6fe63457cb957ec44d0444edb891b87))
* share single memgraph container across integration tests (spgr-mfot) ([a95ac45](https://github.com/specgraph/specgraph/commit/a95ac459e6fe63457cb957ec44d0444edb891b87))


### Code Refactoring

* Slice 3.5 — Scanner removal & documentation cleanup ([#22](https://github.com/specgraph/specgraph/issues/22)) ([f06a476](https://github.com/specgraph/specgraph/commit/f06a47685fe1ce27ed5a265ff209448bd04b414c))
* storage domain types and decision promotion ([#24](https://github.com/specgraph/specgraph/issues/24)) ([836abee](https://github.com/specgraph/specgraph/commit/836abeea8a96d04898d874aaddc6b4a574850690))


### Tests

* add comprehensive E2E test system ([#19](https://github.com/specgraph/specgraph/issues/19)) ([6ecf4e5](https://github.com/specgraph/specgraph/commit/6ecf4e585a21a252fdc18e16e4a6ebcfc109310c))
* **e2e:** implement 3-tier E2E test suite ([#32](https://github.com/specgraph/specgraph/issues/32)) ([de12bbc](https://github.com/specgraph/specgraph/commit/de12bbceaa1f437ee37d98f4d76e7f2f7817611f))
* **e2e:** implement 3-tier E2E test suite with full design doc coverage ([de12bbc](https://github.com/specgraph/specgraph/commit/de12bbceaa1f437ee37d98f4d76e7f2f7817611f))
* **integration:** add DISTINCT regression test for GetExecutionEvents ([#37](https://github.com/specgraph/specgraph/issues/37)) ([2b17445](https://github.com/specgraph/specgraph/commit/2b17445a8421f114d6f34ef3f1fca361afa32dcc))
* **spgr-g8i.16:** add diamond+cycle regression tests for detectCycles ([5adf681](https://github.com/specgraph/specgraph/commit/5adf6813d3bccc7bd16b7279a90e9f451a8dc634))


### CI

* add release-please + goreleaser infrastructure ([#46](https://github.com/specgraph/specgraph/issues/46)) ([1fd22d3](https://github.com/specgraph/specgraph/commit/1fd22d3d9ab3c80360a5e0d9117741192ddd26b8))
* exclude auto-generated CHANGELOG.md from markdown lint ([#517](https://github.com/specgraph/specgraph/issues/517)) ([7106861](https://github.com/specgraph/specgraph/commit/71068619c63a7a7f9749fa98e44287dceed001e3))
* exclude CHANGELOG.md from lint, use PAT for release-please to trigger CI on release PRs ([92805cf](https://github.com/specgraph/specgraph/commit/92805cfff0727fa5287349ab7faec36ecbef6c0f))
* use PAT for release-please to trigger CI on release PRs ([#518](https://github.com/specgraph/specgraph/issues/518)) ([92805cf](https://github.com/specgraph/specgraph/commit/92805cfff0727fa5287349ab7faec36ecbef6c0f))


### Build

* **deps:** bump golang.org/x/crypto from 0.43.0 to 0.45.0 ([#2](https://github.com/specgraph/specgraph/issues/2)) ([a4b88f8](https://github.com/specgraph/specgraph/commit/a4b88f82d2c7b71fbd89a48db4fb48a1d34b5b87))


### Miscellaneous

* add beads ([#5](https://github.com/specgraph/specgraph/issues/5)) ([d10d49d](https://github.com/specgraph/specgraph/commit/d10d49d4157b1376c5a646eff87bd13d63256ee2))
* Configure Renovate ([#6](https://github.com/specgraph/specgraph/issues/6)) ([0a627bf](https://github.com/specgraph/specgraph/commit/0a627bf4519521433eb9e151a33795148bced6c2))
* **deps:** update actions/cache action to v5 ([#25](https://github.com/specgraph/specgraph/issues/25)) ([13d90f5](https://github.com/specgraph/specgraph/commit/13d90f5a42e549a7b429b31e27a4c1373348384c))
* **deps:** update actions/checkout action to v6 ([#14](https://github.com/specgraph/specgraph/issues/14)) ([a6b4f1c](https://github.com/specgraph/specgraph/commit/a6b4f1ca68e896fc37e3598a9a910877a7ec769a))
* **deps:** update actions/setup-go action to v6 ([#21](https://github.com/specgraph/specgraph/issues/21)) ([7ecfca8](https://github.com/specgraph/specgraph/commit/7ecfca8babb52db21b16819005c6e3897189b852))
* **deps:** update actions/upload-pages-artifact action to v4 ([#15](https://github.com/specgraph/specgraph/issues/15)) ([f86df24](https://github.com/specgraph/specgraph/commit/f86df24a7140b5642883c44b7643312e0fe6f32a))
* **deps:** update alpine docker tag to v3.23 ([#10](https://github.com/specgraph/specgraph/issues/10)) ([55da31a](https://github.com/specgraph/specgraph/commit/55da31abfc77d132e30a0ad3872cab39e34d9aeb))
* **deps:** update astral-sh/setup-uv action to v7 ([#16](https://github.com/specgraph/specgraph/issues/16)) ([fa69828](https://github.com/specgraph/specgraph/commit/fa6982887065c9c81db416008791c9b4b551056a))
* **deps:** update dependency go to 1.26 ([#20](https://github.com/specgraph/specgraph/issues/20)) ([4e3718e](https://github.com/specgraph/specgraph/commit/4e3718e5568f31c2ad437679dd7b036237b20efe))
* **deps:** update golang docker tag to v1.26 ([#11](https://github.com/specgraph/specgraph/issues/11)) ([ebf12c5](https://github.com/specgraph/specgraph/commit/ebf12c5f0e781bd242b53cde75a486f89b26ed31))
* **main:** release 0.1.0 ([#49](https://github.com/specgraph/specgraph/issues/49)) ([fcd4b81](https://github.com/specgraph/specgraph/commit/fcd4b81df5000c6c4759a5f6cf6c0cad697a2532))
* **main:** release 0.1.1 ([#520](https://github.com/specgraph/specgraph/issues/520)) ([ef70ae7](https://github.com/specgraph/specgraph/commit/ef70ae7a1be886d6a5de2b43c4ad6f00a840c6fb))
* **main:** release 0.1.2 ([#522](https://github.com/specgraph/specgraph/issues/522)) ([b463d18](https://github.com/specgraph/specgraph/commit/b463d185ca6db602f593eaf30e69bfd4073d49a8))
* **main:** release 0.1.3 ([#527](https://github.com/specgraph/specgraph/issues/527)) ([7e1b255](https://github.com/specgraph/specgraph/commit/7e1b25579aa073eb919e2d1b0725ed818802f350))
* **main:** release 0.1.4 ([#529](https://github.com/specgraph/specgraph/issues/529)) ([dfbb73e](https://github.com/specgraph/specgraph/commit/dfbb73e98b8f61b8d556c34acdf9e8a81c129944))
* **main:** release 0.1.4 ([#531](https://github.com/specgraph/specgraph/issues/531)) ([4b2bc6c](https://github.com/specgraph/specgraph/commit/4b2bc6cff80ef111678f26b322f523b581703a01))
* **main:** release 0.1.5 ([#532](https://github.com/specgraph/specgraph/issues/532)) ([5cbcef5](https://github.com/specgraph/specgraph/commit/5cbcef5e47a60db7d3bc46a9ce7da78b0948ccf4))
* **main:** release 0.1.5 ([#563](https://github.com/specgraph/specgraph/issues/563)) ([2426b98](https://github.com/specgraph/specgraph/commit/2426b98d28113dc60b0e1106891efc68af3479f3))
* **main:** release 0.1.5 ([#565](https://github.com/specgraph/specgraph/issues/565)) ([8608023](https://github.com/specgraph/specgraph/commit/8608023bda623c0d8a59eebc7ed1f09d266075cc))
* **main:** release 0.1.6 ([8608023](https://github.com/specgraph/specgraph/commit/8608023bda623c0d8a59eebc7ed1f09d266075cc))
* **main:** release 0.1.6 ([#534](https://github.com/specgraph/specgraph/issues/534)) ([bc2ccd3](https://github.com/specgraph/specgraph/commit/bc2ccd3ba652c210608cf8c2f0edd00b23e2b38b))
* move module path to specgraph/specgraph ([#45](https://github.com/specgraph/specgraph/issues/45)) ([fb084cb](https://github.com/specgraph/specgraph/commit/fb084cb01a4c340e12a764d5464b43a75d2726e1))
* move repo from seanb4t/specgraph to specgraph/specgraph ([fb084cb](https://github.com/specgraph/specgraph/commit/fb084cb01a4c340e12a764d5464b43a75d2726e1))
* release 0.1.0 ([#48](https://github.com/specgraph/specgraph/issues/48)) ([31e695b](https://github.com/specgraph/specgraph/commit/31e695ba6b608b33248724154ff0fefb92c5b27e))
* trigger release 0.1.3 ([#526](https://github.com/specgraph/specgraph/issues/526)) ([4a92f1b](https://github.com/specgraph/specgraph/commit/4a92f1b33a8cde4b12070768d09a390443555115))

## [0.1.6](https://github.com/specgraph/specgraph/compare/v0.1.5...v0.1.6) (2026-03-21)


### Features

* supply chain security — cosign, SBOMs, attestations, Trivy scan ([#562](https://github.com/specgraph/specgraph/issues/562)) ([070e014](https://github.com/specgraph/specgraph/commit/070e014ccf4f4aa5cef41fbc80ddeba9b2e1c267))


### Bug Fixes

* bump cosign-installer v3→v4, attest-build-provenance v2→v4, trivy-action v0.35.0 ([b55924d](https://github.com/specgraph/specgraph/commit/b55924d9b19fa5b7988a19006b42034b0727816c))
* correct trivy-action version tag (v0.35.0) ([#564](https://github.com/specgraph/specgraph/issues/564)) ([b55924d](https://github.com/specgraph/specgraph/commit/b55924d9b19fa5b7988a19006b42034b0727816c))


### Miscellaneous

* **main:** release 0.1.5 ([#563](https://github.com/specgraph/specgraph/issues/563)) ([2426b98](https://github.com/specgraph/specgraph/commit/2426b98d28113dc60b0e1106891efc68af3479f3))
* **main:** release 0.1.6 ([#534](https://github.com/specgraph/specgraph/issues/534)) ([bc2ccd3](https://github.com/specgraph/specgraph/commit/bc2ccd3ba652c210608cf8c2f0edd00b23e2b38b))

## [0.1.5](https://github.com/specgraph/specgraph/compare/v0.1.6...v0.1.5) (2026-03-21)


### Features

* add code quality and lefthook setup ([#3](https://github.com/specgraph/specgraph/issues/3)) ([970664e](https://github.com/specgraph/specgraph/commit/970664ea5a5a44ece3557eff3c9e247e1e009a88))
* add constitution subsystem (Slice 2) ([#7](https://github.com/specgraph/specgraph/issues/7)) ([10c2ee3](https://github.com/specgraph/specgraph/commit/10c2ee3180a2bf11dd8c179cb4ea4e018f54ace7))
* add extended services (health, claim, decision, graph) ([#4](https://github.com/specgraph/specgraph/issues/4)) ([9fd18e5](https://github.com/specgraph/specgraph/commit/9fd18e5496d5d664c9be4f72e04a583d573f4d5e))
* add Murmur3-128 content hash for change detection ([#39](https://github.com/specgraph/specgraph/issues/39)) ([b3c10b2](https://github.com/specgraph/specgraph/commit/b3c10b2f37f3ab1a9de5a6553ce63a656e48bb52))
* add Zensical doc site with GitHub Pages deployment ([7a1410e](https://github.com/specgraph/specgraph/commit/7a1410e0ae39485c3f7540ddaf8affc21cfd6cbd))
* **auth:** add authentication and authorization interceptor ([#38](https://github.com/specgraph/specgraph/issues/38)) ([f4fc6bf](https://github.com/specgraph/specgraph/commit/f4fc6bf2338020d521fe5ef626da2f8f5be2e1d5))
* ChangeLog graph nodes for version tracking ([#41](https://github.com/specgraph/specgraph/issues/41)) ([e5c00dc](https://github.com/specgraph/specgraph/commit/e5c00dc2def9d8cd408e327afdf5b38f94b3c212))
* **cli:** add report-progress, report-blocker, report-completion commands ([#36](https://github.com/specgraph/specgraph/issues/36)) ([18b09bb](https://github.com/specgraph/specgraph/commit/18b09bb8fb6a6a878fb8c4cc87baad8d9acfb640))
* content hash drift detection on DEPENDS_ON edges ([#43](https://github.com/specgraph/specgraph/issues/43)) ([6c86b33](https://github.com/specgraph/specgraph/commit/6c86b33fe59326557a309d1fcddf098bef0b5df3))
* **docker:** add Memgraph sizing profiles and persistence ([#23](https://github.com/specgraph/specgraph/issues/23)) ([9a2ab3f](https://github.com/specgraph/specgraph/commit/9a2ab3f82367204c9c880086b0f69e4bdb810a6a))
* **execution:** Slice 4 — domain types consistency & execution bundles ([#26](https://github.com/specgraph/specgraph/issues/26)) ([9942813](https://github.com/specgraph/specgraph/commit/9942813353c8afeb930d5de68aec808079fc338b))
* include design docs as hidden pages on site ([3f986a1](https://github.com/specgraph/specgraph/commit/3f986a1753269629b69c8c2baf2cfc8cfde0abe5))
* initial ([a46c950](https://github.com/specgraph/specgraph/commit/a46c950af7c44cf0d101bb9895878698dd5bf0d1))
* **lifecycle:** Slice 5 — spec lifecycle operations ([#27](https://github.com/specgraph/specgraph/issues/27)) ([5adf681](https://github.com/specgraph/specgraph/commit/5adf6813d3bccc7bd16b7279a90e9f451a8dc634))
* **plugin:** evolve authoring skills from CLI reference cards to partner personas ([c260969](https://github.com/specgraph/specgraph/commit/c260969862c3e536765ec26a15700f5d39eed1a5))
* **plugin:** evolve authoring skills into partner personas ([#34](https://github.com/specgraph/specgraph/issues/34)) ([c260969](https://github.com/specgraph/specgraph/commit/c260969862c3e536765ec26a15700f5d39eed1a5))
* **plugin:** Slice 7 — global daemon and Claude Code plugin ([#31](https://github.com/specgraph/specgraph/issues/31)) ([a8a07b4](https://github.com/specgraph/specgraph/commit/a8a07b47ed18fcc5e52de4c7423a7be30e772914))
* **proto:** add notes field to Spec + JSON output for show ([#35](https://github.com/specgraph/specgraph/issues/35)) ([524b09c](https://github.com/specgraph/specgraph/commit/524b09c990999f6c8840c9ab171ccbc776fe042f))
* Slice 3 — Authoring Funnel ([#8](https://github.com/specgraph/specgraph/issues/8)) ([8d15fd1](https://github.com/specgraph/specgraph/commit/8d15fd19d9e3df1102c6a2f5e4a1b17b1a077fca))
* supply chain security — cosign, SBOMs, attestations, Trivy scan ([#562](https://github.com/specgraph/specgraph/issues/562)) ([070e014](https://github.com/specgraph/specgraph/commit/070e014ccf4f4aa5cef41fbc80ddeba9b2e1c267))
* **sync:** Slice 6 — sync adapters, tool injection, and CLI ([#30](https://github.com/specgraph/specgraph/issues/30)) ([c4c6ae7](https://github.com/specgraph/specgraph/commit/c4c6ae716dfc3bad7418085a75b42c1b1a81a93b))
* vertical slice — client/server architecture ([#1](https://github.com/specgraph/specgraph/issues/1)) ([50b504c](https://github.com/specgraph/specgraph/commit/50b504c67167cd52ab43fd956536a38ca8bacc08))


### Bug Fixes

* coordinate release-please and goreleaser — draft release handoff ([#521](https://github.com/specgraph/specgraph/issues/521)) ([fc299c4](https://github.com/specgraph/specgraph/commit/fc299c49d5bc91037cdaa955e734d6a5a3c42fd4))
* **deps:** update module github.com/testcontainers/testcontainers-go to v0.41.0 ([#28](https://github.com/specgraph/specgraph/issues/28)) ([2de880e](https://github.com/specgraph/specgraph/commit/2de880e92923fa4e8accb0a32793656ecd323db5))
* **deps:** update module golang.org/x/text to v0.35.0 ([#29](https://github.com/specgraph/specgraph/issues/29)) ([81fb5bf](https://github.com/specgraph/specgraph/commit/81fb5bff3ebaeffcfce4ea255444ee65a0841d09))
* Dockerfile for goreleaser — use pre-built binary ([#519](https://github.com/specgraph/specgraph/issues/519)) ([92c243b](https://github.com/specgraph/specgraph/commit/92c243bf2ae5626d70c6e0f84b4f9240b6a48275))
* **e2e:** address 4 open test suite findings ([#44](https://github.com/specgraph/specgraph/issues/44)) ([a029036](https://github.com/specgraph/specgraph/commit/a0290368fd4a56618187358b082fc8974aeff185))
* goreleaser Dockerfile + multi-arch Docker images + bump GH actions ([92c243b](https://github.com/specgraph/specgraph/commit/92c243bf2ae5626d70c6e0f84b4f9240b6a48275))
* let goreleaser own GitHub releases, release-please only creates tags ([56051cf](https://github.com/specgraph/specgraph/commit/56051cfe16c34f680ae457388a741578e872679e))
* let goreleaser own GitHub releases, release-please only tags ([#528](https://github.com/specgraph/specgraph/issues/528)) ([56051cf](https://github.com/specgraph/specgraph/commit/56051cfe16c34f680ae457388a741578e872679e))
* push git tag to remote before goreleaser (changelog needs it) ([2f40fd2](https://github.com/specgraph/specgraph/commit/2f40fd2fb0cfd71f2cbb03b01c88bea89b69158c))
* push git tag to remote before goreleaser changelog ([#533](https://github.com/specgraph/specgraph/issues/533)) ([2f40fd2](https://github.com/specgraph/specgraph/commit/2f40fd2fb0cfd71f2cbb03b01c88bea89b69158c))
* release-please creates release+tag, goreleaser replaces with assets ([#530](https://github.com/specgraph/specgraph/issues/530)) ([8439c3a](https://github.com/specgraph/specgraph/commit/8439c3a7c6a510a1442be5b35d2ca61b22365178))
* remove draft:true from release-please config ([#525](https://github.com/specgraph/specgraph/issues/525)) ([7f2f138](https://github.com/specgraph/specgraph/commit/7f2f1389aec66fa060d6e87290bab9a51670e353))
* remove draft:true from release-please, add workflow_dispatch trigger ([7f2f138](https://github.com/specgraph/specgraph/commit/7f2f1389aec66fa060d6e87290bab9a51670e353))
* simple release flow — release-please creates release, goreleaser uploads assets ([#524](https://github.com/specgraph/specgraph/issues/524)) ([7f7b024](https://github.com/specgraph/specgraph/commit/7f7b024a5ea36acef6152778f821be00f0281112))
* unified release workflow — draft release + goreleaser in single pipeline ([8439c3a](https://github.com/specgraph/specgraph/commit/8439c3a7c6a510a1442be5b35d2ca61b22365178))
* wrap all multi-query write paths in RunInTransaction ([#42](https://github.com/specgraph/specgraph/issues/42)) ([04045e8](https://github.com/specgraph/specgraph/commit/04045e82e64d0cf49af5531c2cbf48d3cd2d4888))


### Documentation

* add CLAUDE.md for specgraph subproject ([b7f25f0](https://github.com/specgraph/specgraph/commit/b7f25f03230bd7e10ce0373ea0064b2429a44944))
* add implementation plans for Slices 3-7 ([72a8f6e](https://github.com/specgraph/specgraph/commit/72a8f6ee837f66e6b63807daba90f6b3e8c7641a))
* add implementation tracker and verification gates ([9261e5a](https://github.com/specgraph/specgraph/commit/9261e5a479af00b48236d737ed9a6cd4e2210607))
* add Slice 2 Constitution implementation plan ([fd8cda6](https://github.com/specgraph/specgraph/commit/fd8cda6759596eed4acf83afd83b9bd7b1cab984))
* add top-level README and align site docs ([#18](https://github.com/specgraph/specgraph/issues/18)) ([60e1437](https://github.com/specgraph/specgraph/commit/60e1437457511c18c0fd7ad63ec175664a2feea9))
* add vertical slice roadmap design for remaining implementation ([e736eb7](https://github.com/specgraph/specgraph/commit/e736eb7c1c442c5ba61fdc194519c4e3d663e05e))
* design for storage domain types and decision promotion ([f754076](https://github.com/specgraph/specgraph/commit/f7540767d0d116176e7ccb9255836f95b2f28bc7))
* implementation plan for storage domain types and decision promotion ([cfe9d63](https://github.com/specgraph/specgraph/commit/cfe9d63e8eadab66f574ec95e65ed55a2f50705d))
* mark slices 2-3 complete, remove stale rumdl exclude ([1a9c5c2](https://github.com/specgraph/specgraph/commit/1a9c5c22a40956316997932f624e688f4214d23d))
* Quick Start guide and documentation overhaul for 0.1.0 ([#515](https://github.com/specgraph/specgraph/issues/515)) ([a3c0766](https://github.com/specgraph/specgraph/commit/a3c07665fd825fca692b0bcac4752d04d9f3cff9))
* **site:** add example spec page ([#33](https://github.com/specgraph/specgraph/issues/33)) ([3c719f6](https://github.com/specgraph/specgraph/commit/3c719f65f2cc73492c2f4a9a59134467b3dad597))
* **site:** add example spec page with worked OAuth2 rotation scenario ([3c719f6](https://github.com/specgraph/specgraph/commit/3c719f65f2cc73492c2f4a9a59134467b3dad597))
* **site:** build out documentation site ([#9](https://github.com/specgraph/specgraph/issues/9)) ([66af3dc](https://github.com/specgraph/specgraph/commit/66af3dca602d5f926b20739c51c3775c319bbb16))
* sync site docs and README with current codebase ([bd71843](https://github.com/specgraph/specgraph/commit/bd7184358633c4f6e9dac63f9038acf878440079))
* update CLAUDE.md and add Claude Code automations ([9d17883](https://github.com/specgraph/specgraph/commit/9d1788359a70f05ea3ae71380d9778c3b7b36b8e))
* update CLAUDE.md with test gotchas, remove stale status ([3df0d54](https://github.com/specgraph/specgraph/commit/3df0d54cd153755cdd2fca13ec86e82a695e0acb))


### Performance

* share single memgraph container across integration tests ([#516](https://github.com/specgraph/specgraph/issues/516)) ([a95ac45](https://github.com/specgraph/specgraph/commit/a95ac459e6fe63457cb957ec44d0444edb891b87))
* share single memgraph container across integration tests (spgr-mfot) ([a95ac45](https://github.com/specgraph/specgraph/commit/a95ac459e6fe63457cb957ec44d0444edb891b87))


### Code Refactoring

* Slice 3.5 — Scanner removal & documentation cleanup ([#22](https://github.com/specgraph/specgraph/issues/22)) ([f06a476](https://github.com/specgraph/specgraph/commit/f06a47685fe1ce27ed5a265ff209448bd04b414c))
* storage domain types and decision promotion ([#24](https://github.com/specgraph/specgraph/issues/24)) ([836abee](https://github.com/specgraph/specgraph/commit/836abeea8a96d04898d874aaddc6b4a574850690))


### Tests

* add comprehensive E2E test system ([#19](https://github.com/specgraph/specgraph/issues/19)) ([6ecf4e5](https://github.com/specgraph/specgraph/commit/6ecf4e585a21a252fdc18e16e4a6ebcfc109310c))
* **e2e:** implement 3-tier E2E test suite ([#32](https://github.com/specgraph/specgraph/issues/32)) ([de12bbc](https://github.com/specgraph/specgraph/commit/de12bbceaa1f437ee37d98f4d76e7f2f7817611f))
* **e2e:** implement 3-tier E2E test suite with full design doc coverage ([de12bbc](https://github.com/specgraph/specgraph/commit/de12bbceaa1f437ee37d98f4d76e7f2f7817611f))
* **integration:** add DISTINCT regression test for GetExecutionEvents ([#37](https://github.com/specgraph/specgraph/issues/37)) ([2b17445](https://github.com/specgraph/specgraph/commit/2b17445a8421f114d6f34ef3f1fca361afa32dcc))
* **spgr-g8i.16:** add diamond+cycle regression tests for detectCycles ([5adf681](https://github.com/specgraph/specgraph/commit/5adf6813d3bccc7bd16b7279a90e9f451a8dc634))


### CI

* add release-please + goreleaser infrastructure ([#46](https://github.com/specgraph/specgraph/issues/46)) ([1fd22d3](https://github.com/specgraph/specgraph/commit/1fd22d3d9ab3c80360a5e0d9117741192ddd26b8))
* exclude auto-generated CHANGELOG.md from markdown lint ([#517](https://github.com/specgraph/specgraph/issues/517)) ([7106861](https://github.com/specgraph/specgraph/commit/71068619c63a7a7f9749fa98e44287dceed001e3))
* exclude CHANGELOG.md from lint, use PAT for release-please to trigger CI on release PRs ([92805cf](https://github.com/specgraph/specgraph/commit/92805cfff0727fa5287349ab7faec36ecbef6c0f))
* use PAT for release-please to trigger CI on release PRs ([#518](https://github.com/specgraph/specgraph/issues/518)) ([92805cf](https://github.com/specgraph/specgraph/commit/92805cfff0727fa5287349ab7faec36ecbef6c0f))


### Build

* **deps:** bump golang.org/x/crypto from 0.43.0 to 0.45.0 ([#2](https://github.com/specgraph/specgraph/issues/2)) ([a4b88f8](https://github.com/specgraph/specgraph/commit/a4b88f82d2c7b71fbd89a48db4fb48a1d34b5b87))


### Miscellaneous

* add beads ([#5](https://github.com/specgraph/specgraph/issues/5)) ([d10d49d](https://github.com/specgraph/specgraph/commit/d10d49d4157b1376c5a646eff87bd13d63256ee2))
* Configure Renovate ([#6](https://github.com/specgraph/specgraph/issues/6)) ([0a627bf](https://github.com/specgraph/specgraph/commit/0a627bf4519521433eb9e151a33795148bced6c2))
* **deps:** update actions/cache action to v5 ([#25](https://github.com/specgraph/specgraph/issues/25)) ([13d90f5](https://github.com/specgraph/specgraph/commit/13d90f5a42e549a7b429b31e27a4c1373348384c))
* **deps:** update actions/checkout action to v6 ([#14](https://github.com/specgraph/specgraph/issues/14)) ([a6b4f1c](https://github.com/specgraph/specgraph/commit/a6b4f1ca68e896fc37e3598a9a910877a7ec769a))
* **deps:** update actions/setup-go action to v6 ([#21](https://github.com/specgraph/specgraph/issues/21)) ([7ecfca8](https://github.com/specgraph/specgraph/commit/7ecfca8babb52db21b16819005c6e3897189b852))
* **deps:** update actions/upload-pages-artifact action to v4 ([#15](https://github.com/specgraph/specgraph/issues/15)) ([f86df24](https://github.com/specgraph/specgraph/commit/f86df24a7140b5642883c44b7643312e0fe6f32a))
* **deps:** update alpine docker tag to v3.23 ([#10](https://github.com/specgraph/specgraph/issues/10)) ([55da31a](https://github.com/specgraph/specgraph/commit/55da31abfc77d132e30a0ad3872cab39e34d9aeb))
* **deps:** update astral-sh/setup-uv action to v7 ([#16](https://github.com/specgraph/specgraph/issues/16)) ([fa69828](https://github.com/specgraph/specgraph/commit/fa6982887065c9c81db416008791c9b4b551056a))
* **deps:** update dependency go to 1.26 ([#20](https://github.com/specgraph/specgraph/issues/20)) ([4e3718e](https://github.com/specgraph/specgraph/commit/4e3718e5568f31c2ad437679dd7b036237b20efe))
* **deps:** update golang docker tag to v1.26 ([#11](https://github.com/specgraph/specgraph/issues/11)) ([ebf12c5](https://github.com/specgraph/specgraph/commit/ebf12c5f0e781bd242b53cde75a486f89b26ed31))
* **main:** release 0.1.0 ([#49](https://github.com/specgraph/specgraph/issues/49)) ([fcd4b81](https://github.com/specgraph/specgraph/commit/fcd4b81df5000c6c4759a5f6cf6c0cad697a2532))
* **main:** release 0.1.1 ([#520](https://github.com/specgraph/specgraph/issues/520)) ([ef70ae7](https://github.com/specgraph/specgraph/commit/ef70ae7a1be886d6a5de2b43c4ad6f00a840c6fb))
* **main:** release 0.1.2 ([#522](https://github.com/specgraph/specgraph/issues/522)) ([b463d18](https://github.com/specgraph/specgraph/commit/b463d185ca6db602f593eaf30e69bfd4073d49a8))
* **main:** release 0.1.3 ([#527](https://github.com/specgraph/specgraph/issues/527)) ([7e1b255](https://github.com/specgraph/specgraph/commit/7e1b25579aa073eb919e2d1b0725ed818802f350))
* **main:** release 0.1.4 ([#529](https://github.com/specgraph/specgraph/issues/529)) ([dfbb73e](https://github.com/specgraph/specgraph/commit/dfbb73e98b8f61b8d556c34acdf9e8a81c129944))
* **main:** release 0.1.4 ([#531](https://github.com/specgraph/specgraph/issues/531)) ([4b2bc6c](https://github.com/specgraph/specgraph/commit/4b2bc6cff80ef111678f26b322f523b581703a01))
* **main:** release 0.1.5 ([#532](https://github.com/specgraph/specgraph/issues/532)) ([5cbcef5](https://github.com/specgraph/specgraph/commit/5cbcef5e47a60db7d3bc46a9ce7da78b0948ccf4))
* **main:** release 0.1.6 ([#534](https://github.com/specgraph/specgraph/issues/534)) ([bc2ccd3](https://github.com/specgraph/specgraph/commit/bc2ccd3ba652c210608cf8c2f0edd00b23e2b38b))
* move module path to specgraph/specgraph ([#45](https://github.com/specgraph/specgraph/issues/45)) ([fb084cb](https://github.com/specgraph/specgraph/commit/fb084cb01a4c340e12a764d5464b43a75d2726e1))
* move repo from seanb4t/specgraph to specgraph/specgraph ([fb084cb](https://github.com/specgraph/specgraph/commit/fb084cb01a4c340e12a764d5464b43a75d2726e1))
* release 0.1.0 ([#48](https://github.com/specgraph/specgraph/issues/48)) ([31e695b](https://github.com/specgraph/specgraph/commit/31e695ba6b608b33248724154ff0fefb92c5b27e))
* trigger release 0.1.3 ([#526](https://github.com/specgraph/specgraph/issues/526)) ([4a92f1b](https://github.com/specgraph/specgraph/commit/4a92f1b33a8cde4b12070768d09a390443555115))

## [0.1.6](https://github.com/specgraph/specgraph/compare/v0.1.5...v0.1.6) (2026-03-21)


### Features

* supply chain security — cosign, SBOMs, attestations, Trivy scan ([#562](https://github.com/specgraph/specgraph/issues/562)) ([070e014](https://github.com/specgraph/specgraph/commit/070e014ccf4f4aa5cef41fbc80ddeba9b2e1c267))

## [0.1.5](https://github.com/specgraph/specgraph/compare/v0.1.4...v0.1.5) (2026-03-21)


### Features

* add code quality and lefthook setup ([#3](https://github.com/specgraph/specgraph/issues/3)) ([970664e](https://github.com/specgraph/specgraph/commit/970664ea5a5a44ece3557eff3c9e247e1e009a88))
* add constitution subsystem (Slice 2) ([#7](https://github.com/specgraph/specgraph/issues/7)) ([10c2ee3](https://github.com/specgraph/specgraph/commit/10c2ee3180a2bf11dd8c179cb4ea4e018f54ace7))
* add extended services (health, claim, decision, graph) ([#4](https://github.com/specgraph/specgraph/issues/4)) ([9fd18e5](https://github.com/specgraph/specgraph/commit/9fd18e5496d5d664c9be4f72e04a583d573f4d5e))
* add Murmur3-128 content hash for change detection ([#39](https://github.com/specgraph/specgraph/issues/39)) ([b3c10b2](https://github.com/specgraph/specgraph/commit/b3c10b2f37f3ab1a9de5a6553ce63a656e48bb52))
* add Zensical doc site with GitHub Pages deployment ([7a1410e](https://github.com/specgraph/specgraph/commit/7a1410e0ae39485c3f7540ddaf8affc21cfd6cbd))
* **auth:** add authentication and authorization interceptor ([#38](https://github.com/specgraph/specgraph/issues/38)) ([f4fc6bf](https://github.com/specgraph/specgraph/commit/f4fc6bf2338020d521fe5ef626da2f8f5be2e1d5))
* ChangeLog graph nodes for version tracking ([#41](https://github.com/specgraph/specgraph/issues/41)) ([e5c00dc](https://github.com/specgraph/specgraph/commit/e5c00dc2def9d8cd408e327afdf5b38f94b3c212))
* **cli:** add report-progress, report-blocker, report-completion commands ([#36](https://github.com/specgraph/specgraph/issues/36)) ([18b09bb](https://github.com/specgraph/specgraph/commit/18b09bb8fb6a6a878fb8c4cc87baad8d9acfb640))
* content hash drift detection on DEPENDS_ON edges ([#43](https://github.com/specgraph/specgraph/issues/43)) ([6c86b33](https://github.com/specgraph/specgraph/commit/6c86b33fe59326557a309d1fcddf098bef0b5df3))
* **docker:** add Memgraph sizing profiles and persistence ([#23](https://github.com/specgraph/specgraph/issues/23)) ([9a2ab3f](https://github.com/specgraph/specgraph/commit/9a2ab3f82367204c9c880086b0f69e4bdb810a6a))
* **execution:** Slice 4 — domain types consistency & execution bundles ([#26](https://github.com/specgraph/specgraph/issues/26)) ([9942813](https://github.com/specgraph/specgraph/commit/9942813353c8afeb930d5de68aec808079fc338b))
* include design docs as hidden pages on site ([3f986a1](https://github.com/specgraph/specgraph/commit/3f986a1753269629b69c8c2baf2cfc8cfde0abe5))
* initial ([a46c950](https://github.com/specgraph/specgraph/commit/a46c950af7c44cf0d101bb9895878698dd5bf0d1))
* **lifecycle:** Slice 5 — spec lifecycle operations ([#27](https://github.com/specgraph/specgraph/issues/27)) ([5adf681](https://github.com/specgraph/specgraph/commit/5adf6813d3bccc7bd16b7279a90e9f451a8dc634))
* **plugin:** evolve authoring skills from CLI reference cards to partner personas ([c260969](https://github.com/specgraph/specgraph/commit/c260969862c3e536765ec26a15700f5d39eed1a5))
* **plugin:** evolve authoring skills into partner personas ([#34](https://github.com/specgraph/specgraph/issues/34)) ([c260969](https://github.com/specgraph/specgraph/commit/c260969862c3e536765ec26a15700f5d39eed1a5))
* **plugin:** Slice 7 — global daemon and Claude Code plugin ([#31](https://github.com/specgraph/specgraph/issues/31)) ([a8a07b4](https://github.com/specgraph/specgraph/commit/a8a07b47ed18fcc5e52de4c7423a7be30e772914))
* **proto:** add notes field to Spec + JSON output for show ([#35](https://github.com/specgraph/specgraph/issues/35)) ([524b09c](https://github.com/specgraph/specgraph/commit/524b09c990999f6c8840c9ab171ccbc776fe042f))
* Slice 3 — Authoring Funnel ([#8](https://github.com/specgraph/specgraph/issues/8)) ([8d15fd1](https://github.com/specgraph/specgraph/commit/8d15fd19d9e3df1102c6a2f5e4a1b17b1a077fca))
* **sync:** Slice 6 — sync adapters, tool injection, and CLI ([#30](https://github.com/specgraph/specgraph/issues/30)) ([c4c6ae7](https://github.com/specgraph/specgraph/commit/c4c6ae716dfc3bad7418085a75b42c1b1a81a93b))
* vertical slice — client/server architecture ([#1](https://github.com/specgraph/specgraph/issues/1)) ([50b504c](https://github.com/specgraph/specgraph/commit/50b504c67167cd52ab43fd956536a38ca8bacc08))


### Bug Fixes

* coordinate release-please and goreleaser — draft release handoff ([#521](https://github.com/specgraph/specgraph/issues/521)) ([fc299c4](https://github.com/specgraph/specgraph/commit/fc299c49d5bc91037cdaa955e734d6a5a3c42fd4))
* **deps:** update module github.com/testcontainers/testcontainers-go to v0.41.0 ([#28](https://github.com/specgraph/specgraph/issues/28)) ([2de880e](https://github.com/specgraph/specgraph/commit/2de880e92923fa4e8accb0a32793656ecd323db5))
* **deps:** update module golang.org/x/text to v0.35.0 ([#29](https://github.com/specgraph/specgraph/issues/29)) ([81fb5bf](https://github.com/specgraph/specgraph/commit/81fb5bff3ebaeffcfce4ea255444ee65a0841d09))
* Dockerfile for goreleaser — use pre-built binary ([#519](https://github.com/specgraph/specgraph/issues/519)) ([92c243b](https://github.com/specgraph/specgraph/commit/92c243bf2ae5626d70c6e0f84b4f9240b6a48275))
* **e2e:** address 4 open test suite findings ([#44](https://github.com/specgraph/specgraph/issues/44)) ([a029036](https://github.com/specgraph/specgraph/commit/a0290368fd4a56618187358b082fc8974aeff185))
* goreleaser Dockerfile + multi-arch Docker images + bump GH actions ([92c243b](https://github.com/specgraph/specgraph/commit/92c243bf2ae5626d70c6e0f84b4f9240b6a48275))
* let goreleaser own GitHub releases, release-please only creates tags ([56051cf](https://github.com/specgraph/specgraph/commit/56051cfe16c34f680ae457388a741578e872679e))
* let goreleaser own GitHub releases, release-please only tags ([#528](https://github.com/specgraph/specgraph/issues/528)) ([56051cf](https://github.com/specgraph/specgraph/commit/56051cfe16c34f680ae457388a741578e872679e))
* push git tag to remote before goreleaser (changelog needs it) ([2f40fd2](https://github.com/specgraph/specgraph/commit/2f40fd2fb0cfd71f2cbb03b01c88bea89b69158c))
* push git tag to remote before goreleaser changelog ([#533](https://github.com/specgraph/specgraph/issues/533)) ([2f40fd2](https://github.com/specgraph/specgraph/commit/2f40fd2fb0cfd71f2cbb03b01c88bea89b69158c))
* release-please creates release+tag, goreleaser replaces with assets ([#530](https://github.com/specgraph/specgraph/issues/530)) ([8439c3a](https://github.com/specgraph/specgraph/commit/8439c3a7c6a510a1442be5b35d2ca61b22365178))
* remove draft:true from release-please config ([#525](https://github.com/specgraph/specgraph/issues/525)) ([7f2f138](https://github.com/specgraph/specgraph/commit/7f2f1389aec66fa060d6e87290bab9a51670e353))
* remove draft:true from release-please, add workflow_dispatch trigger ([7f2f138](https://github.com/specgraph/specgraph/commit/7f2f1389aec66fa060d6e87290bab9a51670e353))
* simple release flow — release-please creates release, goreleaser uploads assets ([#524](https://github.com/specgraph/specgraph/issues/524)) ([7f7b024](https://github.com/specgraph/specgraph/commit/7f7b024a5ea36acef6152778f821be00f0281112))
* unified release workflow — draft release + goreleaser in single pipeline ([8439c3a](https://github.com/specgraph/specgraph/commit/8439c3a7c6a510a1442be5b35d2ca61b22365178))
* wrap all multi-query write paths in RunInTransaction ([#42](https://github.com/specgraph/specgraph/issues/42)) ([04045e8](https://github.com/specgraph/specgraph/commit/04045e82e64d0cf49af5531c2cbf48d3cd2d4888))


### Documentation

* add CLAUDE.md for specgraph subproject ([b7f25f0](https://github.com/specgraph/specgraph/commit/b7f25f03230bd7e10ce0373ea0064b2429a44944))
* add implementation plans for Slices 3-7 ([72a8f6e](https://github.com/specgraph/specgraph/commit/72a8f6ee837f66e6b63807daba90f6b3e8c7641a))
* add implementation tracker and verification gates ([9261e5a](https://github.com/specgraph/specgraph/commit/9261e5a479af00b48236d737ed9a6cd4e2210607))
* add Slice 2 Constitution implementation plan ([fd8cda6](https://github.com/specgraph/specgraph/commit/fd8cda6759596eed4acf83afd83b9bd7b1cab984))
* add top-level README and align site docs ([#18](https://github.com/specgraph/specgraph/issues/18)) ([60e1437](https://github.com/specgraph/specgraph/commit/60e1437457511c18c0fd7ad63ec175664a2feea9))
* add vertical slice roadmap design for remaining implementation ([e736eb7](https://github.com/specgraph/specgraph/commit/e736eb7c1c442c5ba61fdc194519c4e3d663e05e))
* design for storage domain types and decision promotion ([f754076](https://github.com/specgraph/specgraph/commit/f7540767d0d116176e7ccb9255836f95b2f28bc7))
* implementation plan for storage domain types and decision promotion ([cfe9d63](https://github.com/specgraph/specgraph/commit/cfe9d63e8eadab66f574ec95e65ed55a2f50705d))
* mark slices 2-3 complete, remove stale rumdl exclude ([1a9c5c2](https://github.com/specgraph/specgraph/commit/1a9c5c22a40956316997932f624e688f4214d23d))
* Quick Start guide and documentation overhaul for 0.1.0 ([#515](https://github.com/specgraph/specgraph/issues/515)) ([a3c0766](https://github.com/specgraph/specgraph/commit/a3c07665fd825fca692b0bcac4752d04d9f3cff9))
* **site:** add example spec page ([#33](https://github.com/specgraph/specgraph/issues/33)) ([3c719f6](https://github.com/specgraph/specgraph/commit/3c719f65f2cc73492c2f4a9a59134467b3dad597))
* **site:** add example spec page with worked OAuth2 rotation scenario ([3c719f6](https://github.com/specgraph/specgraph/commit/3c719f65f2cc73492c2f4a9a59134467b3dad597))
* **site:** build out documentation site ([#9](https://github.com/specgraph/specgraph/issues/9)) ([66af3dc](https://github.com/specgraph/specgraph/commit/66af3dca602d5f926b20739c51c3775c319bbb16))
* sync site docs and README with current codebase ([bd71843](https://github.com/specgraph/specgraph/commit/bd7184358633c4f6e9dac63f9038acf878440079))
* update CLAUDE.md and add Claude Code automations ([9d17883](https://github.com/specgraph/specgraph/commit/9d1788359a70f05ea3ae71380d9778c3b7b36b8e))
* update CLAUDE.md with test gotchas, remove stale status ([3df0d54](https://github.com/specgraph/specgraph/commit/3df0d54cd153755cdd2fca13ec86e82a695e0acb))


### Performance

* share single memgraph container across integration tests ([#516](https://github.com/specgraph/specgraph/issues/516)) ([a95ac45](https://github.com/specgraph/specgraph/commit/a95ac459e6fe63457cb957ec44d0444edb891b87))
* share single memgraph container across integration tests (spgr-mfot) ([a95ac45](https://github.com/specgraph/specgraph/commit/a95ac459e6fe63457cb957ec44d0444edb891b87))


### Code Refactoring

* Slice 3.5 — Scanner removal & documentation cleanup ([#22](https://github.com/specgraph/specgraph/issues/22)) ([f06a476](https://github.com/specgraph/specgraph/commit/f06a47685fe1ce27ed5a265ff209448bd04b414c))
* storage domain types and decision promotion ([#24](https://github.com/specgraph/specgraph/issues/24)) ([836abee](https://github.com/specgraph/specgraph/commit/836abeea8a96d04898d874aaddc6b4a574850690))


### Tests

* add comprehensive E2E test system ([#19](https://github.com/specgraph/specgraph/issues/19)) ([6ecf4e5](https://github.com/specgraph/specgraph/commit/6ecf4e585a21a252fdc18e16e4a6ebcfc109310c))
* **e2e:** implement 3-tier E2E test suite ([#32](https://github.com/specgraph/specgraph/issues/32)) ([de12bbc](https://github.com/specgraph/specgraph/commit/de12bbceaa1f437ee37d98f4d76e7f2f7817611f))
* **e2e:** implement 3-tier E2E test suite with full design doc coverage ([de12bbc](https://github.com/specgraph/specgraph/commit/de12bbceaa1f437ee37d98f4d76e7f2f7817611f))
* **integration:** add DISTINCT regression test for GetExecutionEvents ([#37](https://github.com/specgraph/specgraph/issues/37)) ([2b17445](https://github.com/specgraph/specgraph/commit/2b17445a8421f114d6f34ef3f1fca361afa32dcc))
* **spgr-g8i.16:** add diamond+cycle regression tests for detectCycles ([5adf681](https://github.com/specgraph/specgraph/commit/5adf6813d3bccc7bd16b7279a90e9f451a8dc634))


### CI

* add release-please + goreleaser infrastructure ([#46](https://github.com/specgraph/specgraph/issues/46)) ([1fd22d3](https://github.com/specgraph/specgraph/commit/1fd22d3d9ab3c80360a5e0d9117741192ddd26b8))
* exclude auto-generated CHANGELOG.md from markdown lint ([#517](https://github.com/specgraph/specgraph/issues/517)) ([7106861](https://github.com/specgraph/specgraph/commit/71068619c63a7a7f9749fa98e44287dceed001e3))
* exclude CHANGELOG.md from lint, use PAT for release-please to trigger CI on release PRs ([92805cf](https://github.com/specgraph/specgraph/commit/92805cfff0727fa5287349ab7faec36ecbef6c0f))
* use PAT for release-please to trigger CI on release PRs ([#518](https://github.com/specgraph/specgraph/issues/518)) ([92805cf](https://github.com/specgraph/specgraph/commit/92805cfff0727fa5287349ab7faec36ecbef6c0f))


### Build

* **deps:** bump golang.org/x/crypto from 0.43.0 to 0.45.0 ([#2](https://github.com/specgraph/specgraph/issues/2)) ([a4b88f8](https://github.com/specgraph/specgraph/commit/a4b88f82d2c7b71fbd89a48db4fb48a1d34b5b87))


### Miscellaneous

* add beads ([#5](https://github.com/specgraph/specgraph/issues/5)) ([d10d49d](https://github.com/specgraph/specgraph/commit/d10d49d4157b1376c5a646eff87bd13d63256ee2))
* Configure Renovate ([#6](https://github.com/specgraph/specgraph/issues/6)) ([0a627bf](https://github.com/specgraph/specgraph/commit/0a627bf4519521433eb9e151a33795148bced6c2))
* **deps:** update actions/cache action to v5 ([#25](https://github.com/specgraph/specgraph/issues/25)) ([13d90f5](https://github.com/specgraph/specgraph/commit/13d90f5a42e549a7b429b31e27a4c1373348384c))
* **deps:** update actions/checkout action to v6 ([#14](https://github.com/specgraph/specgraph/issues/14)) ([a6b4f1c](https://github.com/specgraph/specgraph/commit/a6b4f1ca68e896fc37e3598a9a910877a7ec769a))
* **deps:** update actions/setup-go action to v6 ([#21](https://github.com/specgraph/specgraph/issues/21)) ([7ecfca8](https://github.com/specgraph/specgraph/commit/7ecfca8babb52db21b16819005c6e3897189b852))
* **deps:** update actions/upload-pages-artifact action to v4 ([#15](https://github.com/specgraph/specgraph/issues/15)) ([f86df24](https://github.com/specgraph/specgraph/commit/f86df24a7140b5642883c44b7643312e0fe6f32a))
* **deps:** update alpine docker tag to v3.23 ([#10](https://github.com/specgraph/specgraph/issues/10)) ([55da31a](https://github.com/specgraph/specgraph/commit/55da31abfc77d132e30a0ad3872cab39e34d9aeb))
* **deps:** update astral-sh/setup-uv action to v7 ([#16](https://github.com/specgraph/specgraph/issues/16)) ([fa69828](https://github.com/specgraph/specgraph/commit/fa6982887065c9c81db416008791c9b4b551056a))
* **deps:** update dependency go to 1.26 ([#20](https://github.com/specgraph/specgraph/issues/20)) ([4e3718e](https://github.com/specgraph/specgraph/commit/4e3718e5568f31c2ad437679dd7b036237b20efe))
* **deps:** update golang docker tag to v1.26 ([#11](https://github.com/specgraph/specgraph/issues/11)) ([ebf12c5](https://github.com/specgraph/specgraph/commit/ebf12c5f0e781bd242b53cde75a486f89b26ed31))
* **main:** release 0.1.0 ([#49](https://github.com/specgraph/specgraph/issues/49)) ([fcd4b81](https://github.com/specgraph/specgraph/commit/fcd4b81df5000c6c4759a5f6cf6c0cad697a2532))
* **main:** release 0.1.1 ([#520](https://github.com/specgraph/specgraph/issues/520)) ([ef70ae7](https://github.com/specgraph/specgraph/commit/ef70ae7a1be886d6a5de2b43c4ad6f00a840c6fb))
* **main:** release 0.1.2 ([#522](https://github.com/specgraph/specgraph/issues/522)) ([b463d18](https://github.com/specgraph/specgraph/commit/b463d185ca6db602f593eaf30e69bfd4073d49a8))
* **main:** release 0.1.3 ([#527](https://github.com/specgraph/specgraph/issues/527)) ([7e1b255](https://github.com/specgraph/specgraph/commit/7e1b25579aa073eb919e2d1b0725ed818802f350))
* **main:** release 0.1.4 ([#529](https://github.com/specgraph/specgraph/issues/529)) ([dfbb73e](https://github.com/specgraph/specgraph/commit/dfbb73e98b8f61b8d556c34acdf9e8a81c129944))
* **main:** release 0.1.4 ([#531](https://github.com/specgraph/specgraph/issues/531)) ([4b2bc6c](https://github.com/specgraph/specgraph/commit/4b2bc6cff80ef111678f26b322f523b581703a01))
* move module path to specgraph/specgraph ([#45](https://github.com/specgraph/specgraph/issues/45)) ([fb084cb](https://github.com/specgraph/specgraph/commit/fb084cb01a4c340e12a764d5464b43a75d2726e1))
* move repo from seanb4t/specgraph to specgraph/specgraph ([fb084cb](https://github.com/specgraph/specgraph/commit/fb084cb01a4c340e12a764d5464b43a75d2726e1))
* release 0.1.0 ([#48](https://github.com/specgraph/specgraph/issues/48)) ([31e695b](https://github.com/specgraph/specgraph/commit/31e695ba6b608b33248724154ff0fefb92c5b27e))
* trigger release 0.1.3 ([#526](https://github.com/specgraph/specgraph/issues/526)) ([4a92f1b](https://github.com/specgraph/specgraph/commit/4a92f1b33a8cde4b12070768d09a390443555115))

## [0.1.4](https://github.com/specgraph/specgraph/compare/v0.1.4...v0.1.4) (2026-03-21)


### Features

* add code quality and lefthook setup ([#3](https://github.com/specgraph/specgraph/issues/3)) ([970664e](https://github.com/specgraph/specgraph/commit/970664ea5a5a44ece3557eff3c9e247e1e009a88))
* add constitution subsystem (Slice 2) ([#7](https://github.com/specgraph/specgraph/issues/7)) ([10c2ee3](https://github.com/specgraph/specgraph/commit/10c2ee3180a2bf11dd8c179cb4ea4e018f54ace7))
* add extended services (health, claim, decision, graph) ([#4](https://github.com/specgraph/specgraph/issues/4)) ([9fd18e5](https://github.com/specgraph/specgraph/commit/9fd18e5496d5d664c9be4f72e04a583d573f4d5e))
* add Murmur3-128 content hash for change detection ([#39](https://github.com/specgraph/specgraph/issues/39)) ([b3c10b2](https://github.com/specgraph/specgraph/commit/b3c10b2f37f3ab1a9de5a6553ce63a656e48bb52))
* add Zensical doc site with GitHub Pages deployment ([7a1410e](https://github.com/specgraph/specgraph/commit/7a1410e0ae39485c3f7540ddaf8affc21cfd6cbd))
* **auth:** add authentication and authorization interceptor ([#38](https://github.com/specgraph/specgraph/issues/38)) ([f4fc6bf](https://github.com/specgraph/specgraph/commit/f4fc6bf2338020d521fe5ef626da2f8f5be2e1d5))
* ChangeLog graph nodes for version tracking ([#41](https://github.com/specgraph/specgraph/issues/41)) ([e5c00dc](https://github.com/specgraph/specgraph/commit/e5c00dc2def9d8cd408e327afdf5b38f94b3c212))
* **cli:** add report-progress, report-blocker, report-completion commands ([#36](https://github.com/specgraph/specgraph/issues/36)) ([18b09bb](https://github.com/specgraph/specgraph/commit/18b09bb8fb6a6a878fb8c4cc87baad8d9acfb640))
* content hash drift detection on DEPENDS_ON edges ([#43](https://github.com/specgraph/specgraph/issues/43)) ([6c86b33](https://github.com/specgraph/specgraph/commit/6c86b33fe59326557a309d1fcddf098bef0b5df3))
* **docker:** add Memgraph sizing profiles and persistence ([#23](https://github.com/specgraph/specgraph/issues/23)) ([9a2ab3f](https://github.com/specgraph/specgraph/commit/9a2ab3f82367204c9c880086b0f69e4bdb810a6a))
* **execution:** Slice 4 — domain types consistency & execution bundles ([#26](https://github.com/specgraph/specgraph/issues/26)) ([9942813](https://github.com/specgraph/specgraph/commit/9942813353c8afeb930d5de68aec808079fc338b))
* include design docs as hidden pages on site ([3f986a1](https://github.com/specgraph/specgraph/commit/3f986a1753269629b69c8c2baf2cfc8cfde0abe5))
* initial ([a46c950](https://github.com/specgraph/specgraph/commit/a46c950af7c44cf0d101bb9895878698dd5bf0d1))
* **lifecycle:** Slice 5 — spec lifecycle operations ([#27](https://github.com/specgraph/specgraph/issues/27)) ([5adf681](https://github.com/specgraph/specgraph/commit/5adf6813d3bccc7bd16b7279a90e9f451a8dc634))
* **plugin:** evolve authoring skills from CLI reference cards to partner personas ([c260969](https://github.com/specgraph/specgraph/commit/c260969862c3e536765ec26a15700f5d39eed1a5))
* **plugin:** evolve authoring skills into partner personas ([#34](https://github.com/specgraph/specgraph/issues/34)) ([c260969](https://github.com/specgraph/specgraph/commit/c260969862c3e536765ec26a15700f5d39eed1a5))
* **plugin:** Slice 7 — global daemon and Claude Code plugin ([#31](https://github.com/specgraph/specgraph/issues/31)) ([a8a07b4](https://github.com/specgraph/specgraph/commit/a8a07b47ed18fcc5e52de4c7423a7be30e772914))
* **proto:** add notes field to Spec + JSON output for show ([#35](https://github.com/specgraph/specgraph/issues/35)) ([524b09c](https://github.com/specgraph/specgraph/commit/524b09c990999f6c8840c9ab171ccbc776fe042f))
* Slice 3 — Authoring Funnel ([#8](https://github.com/specgraph/specgraph/issues/8)) ([8d15fd1](https://github.com/specgraph/specgraph/commit/8d15fd19d9e3df1102c6a2f5e4a1b17b1a077fca))
* **sync:** Slice 6 — sync adapters, tool injection, and CLI ([#30](https://github.com/specgraph/specgraph/issues/30)) ([c4c6ae7](https://github.com/specgraph/specgraph/commit/c4c6ae716dfc3bad7418085a75b42c1b1a81a93b))
* vertical slice — client/server architecture ([#1](https://github.com/specgraph/specgraph/issues/1)) ([50b504c](https://github.com/specgraph/specgraph/commit/50b504c67167cd52ab43fd956536a38ca8bacc08))


### Bug Fixes

* coordinate release-please and goreleaser — draft release handoff ([#521](https://github.com/specgraph/specgraph/issues/521)) ([fc299c4](https://github.com/specgraph/specgraph/commit/fc299c49d5bc91037cdaa955e734d6a5a3c42fd4))
* **deps:** update module github.com/testcontainers/testcontainers-go to v0.41.0 ([#28](https://github.com/specgraph/specgraph/issues/28)) ([2de880e](https://github.com/specgraph/specgraph/commit/2de880e92923fa4e8accb0a32793656ecd323db5))
* **deps:** update module golang.org/x/text to v0.35.0 ([#29](https://github.com/specgraph/specgraph/issues/29)) ([81fb5bf](https://github.com/specgraph/specgraph/commit/81fb5bff3ebaeffcfce4ea255444ee65a0841d09))
* Dockerfile for goreleaser — use pre-built binary ([#519](https://github.com/specgraph/specgraph/issues/519)) ([92c243b](https://github.com/specgraph/specgraph/commit/92c243bf2ae5626d70c6e0f84b4f9240b6a48275))
* **e2e:** address 4 open test suite findings ([#44](https://github.com/specgraph/specgraph/issues/44)) ([a029036](https://github.com/specgraph/specgraph/commit/a0290368fd4a56618187358b082fc8974aeff185))
* goreleaser Dockerfile + multi-arch Docker images + bump GH actions ([92c243b](https://github.com/specgraph/specgraph/commit/92c243bf2ae5626d70c6e0f84b4f9240b6a48275))
* let goreleaser own GitHub releases, release-please only creates tags ([56051cf](https://github.com/specgraph/specgraph/commit/56051cfe16c34f680ae457388a741578e872679e))
* let goreleaser own GitHub releases, release-please only tags ([#528](https://github.com/specgraph/specgraph/issues/528)) ([56051cf](https://github.com/specgraph/specgraph/commit/56051cfe16c34f680ae457388a741578e872679e))
* release-please creates release+tag, goreleaser replaces with assets ([#530](https://github.com/specgraph/specgraph/issues/530)) ([8439c3a](https://github.com/specgraph/specgraph/commit/8439c3a7c6a510a1442be5b35d2ca61b22365178))
* remove draft:true from release-please config ([#525](https://github.com/specgraph/specgraph/issues/525)) ([7f2f138](https://github.com/specgraph/specgraph/commit/7f2f1389aec66fa060d6e87290bab9a51670e353))
* remove draft:true from release-please, add workflow_dispatch trigger ([7f2f138](https://github.com/specgraph/specgraph/commit/7f2f1389aec66fa060d6e87290bab9a51670e353))
* simple release flow — release-please creates release, goreleaser uploads assets ([#524](https://github.com/specgraph/specgraph/issues/524)) ([7f7b024](https://github.com/specgraph/specgraph/commit/7f7b024a5ea36acef6152778f821be00f0281112))
* unified release workflow — draft release + goreleaser in single pipeline ([8439c3a](https://github.com/specgraph/specgraph/commit/8439c3a7c6a510a1442be5b35d2ca61b22365178))
* wrap all multi-query write paths in RunInTransaction ([#42](https://github.com/specgraph/specgraph/issues/42)) ([04045e8](https://github.com/specgraph/specgraph/commit/04045e82e64d0cf49af5531c2cbf48d3cd2d4888))


### Documentation

* add CLAUDE.md for specgraph subproject ([b7f25f0](https://github.com/specgraph/specgraph/commit/b7f25f03230bd7e10ce0373ea0064b2429a44944))
* add implementation plans for Slices 3-7 ([72a8f6e](https://github.com/specgraph/specgraph/commit/72a8f6ee837f66e6b63807daba90f6b3e8c7641a))
* add implementation tracker and verification gates ([9261e5a](https://github.com/specgraph/specgraph/commit/9261e5a479af00b48236d737ed9a6cd4e2210607))
* add Slice 2 Constitution implementation plan ([fd8cda6](https://github.com/specgraph/specgraph/commit/fd8cda6759596eed4acf83afd83b9bd7b1cab984))
* add top-level README and align site docs ([#18](https://github.com/specgraph/specgraph/issues/18)) ([60e1437](https://github.com/specgraph/specgraph/commit/60e1437457511c18c0fd7ad63ec175664a2feea9))
* add vertical slice roadmap design for remaining implementation ([e736eb7](https://github.com/specgraph/specgraph/commit/e736eb7c1c442c5ba61fdc194519c4e3d663e05e))
* design for storage domain types and decision promotion ([f754076](https://github.com/specgraph/specgraph/commit/f7540767d0d116176e7ccb9255836f95b2f28bc7))
* implementation plan for storage domain types and decision promotion ([cfe9d63](https://github.com/specgraph/specgraph/commit/cfe9d63e8eadab66f574ec95e65ed55a2f50705d))
* mark slices 2-3 complete, remove stale rumdl exclude ([1a9c5c2](https://github.com/specgraph/specgraph/commit/1a9c5c22a40956316997932f624e688f4214d23d))
* Quick Start guide and documentation overhaul for 0.1.0 ([#515](https://github.com/specgraph/specgraph/issues/515)) ([a3c0766](https://github.com/specgraph/specgraph/commit/a3c07665fd825fca692b0bcac4752d04d9f3cff9))
* **site:** add example spec page ([#33](https://github.com/specgraph/specgraph/issues/33)) ([3c719f6](https://github.com/specgraph/specgraph/commit/3c719f65f2cc73492c2f4a9a59134467b3dad597))
* **site:** add example spec page with worked OAuth2 rotation scenario ([3c719f6](https://github.com/specgraph/specgraph/commit/3c719f65f2cc73492c2f4a9a59134467b3dad597))
* **site:** build out documentation site ([#9](https://github.com/specgraph/specgraph/issues/9)) ([66af3dc](https://github.com/specgraph/specgraph/commit/66af3dca602d5f926b20739c51c3775c319bbb16))
* sync site docs and README with current codebase ([bd71843](https://github.com/specgraph/specgraph/commit/bd7184358633c4f6e9dac63f9038acf878440079))
* update CLAUDE.md and add Claude Code automations ([9d17883](https://github.com/specgraph/specgraph/commit/9d1788359a70f05ea3ae71380d9778c3b7b36b8e))
* update CLAUDE.md with test gotchas, remove stale status ([3df0d54](https://github.com/specgraph/specgraph/commit/3df0d54cd153755cdd2fca13ec86e82a695e0acb))


### Performance

* share single memgraph container across integration tests ([#516](https://github.com/specgraph/specgraph/issues/516)) ([a95ac45](https://github.com/specgraph/specgraph/commit/a95ac459e6fe63457cb957ec44d0444edb891b87))
* share single memgraph container across integration tests (spgr-mfot) ([a95ac45](https://github.com/specgraph/specgraph/commit/a95ac459e6fe63457cb957ec44d0444edb891b87))


### Code Refactoring

* Slice 3.5 — Scanner removal & documentation cleanup ([#22](https://github.com/specgraph/specgraph/issues/22)) ([f06a476](https://github.com/specgraph/specgraph/commit/f06a47685fe1ce27ed5a265ff209448bd04b414c))
* storage domain types and decision promotion ([#24](https://github.com/specgraph/specgraph/issues/24)) ([836abee](https://github.com/specgraph/specgraph/commit/836abeea8a96d04898d874aaddc6b4a574850690))


### Tests

* add comprehensive E2E test system ([#19](https://github.com/specgraph/specgraph/issues/19)) ([6ecf4e5](https://github.com/specgraph/specgraph/commit/6ecf4e585a21a252fdc18e16e4a6ebcfc109310c))
* **e2e:** implement 3-tier E2E test suite ([#32](https://github.com/specgraph/specgraph/issues/32)) ([de12bbc](https://github.com/specgraph/specgraph/commit/de12bbceaa1f437ee37d98f4d76e7f2f7817611f))
* **e2e:** implement 3-tier E2E test suite with full design doc coverage ([de12bbc](https://github.com/specgraph/specgraph/commit/de12bbceaa1f437ee37d98f4d76e7f2f7817611f))
* **integration:** add DISTINCT regression test for GetExecutionEvents ([#37](https://github.com/specgraph/specgraph/issues/37)) ([2b17445](https://github.com/specgraph/specgraph/commit/2b17445a8421f114d6f34ef3f1fca361afa32dcc))
* **spgr-g8i.16:** add diamond+cycle regression tests for detectCycles ([5adf681](https://github.com/specgraph/specgraph/commit/5adf6813d3bccc7bd16b7279a90e9f451a8dc634))


### CI

* add release-please + goreleaser infrastructure ([#46](https://github.com/specgraph/specgraph/issues/46)) ([1fd22d3](https://github.com/specgraph/specgraph/commit/1fd22d3d9ab3c80360a5e0d9117741192ddd26b8))
* exclude auto-generated CHANGELOG.md from markdown lint ([#517](https://github.com/specgraph/specgraph/issues/517)) ([7106861](https://github.com/specgraph/specgraph/commit/71068619c63a7a7f9749fa98e44287dceed001e3))
* exclude CHANGELOG.md from lint, use PAT for release-please to trigger CI on release PRs ([92805cf](https://github.com/specgraph/specgraph/commit/92805cfff0727fa5287349ab7faec36ecbef6c0f))
* use PAT for release-please to trigger CI on release PRs ([#518](https://github.com/specgraph/specgraph/issues/518)) ([92805cf](https://github.com/specgraph/specgraph/commit/92805cfff0727fa5287349ab7faec36ecbef6c0f))


### Build

* **deps:** bump golang.org/x/crypto from 0.43.0 to 0.45.0 ([#2](https://github.com/specgraph/specgraph/issues/2)) ([a4b88f8](https://github.com/specgraph/specgraph/commit/a4b88f82d2c7b71fbd89a48db4fb48a1d34b5b87))


### Miscellaneous

* add beads ([#5](https://github.com/specgraph/specgraph/issues/5)) ([d10d49d](https://github.com/specgraph/specgraph/commit/d10d49d4157b1376c5a646eff87bd13d63256ee2))
* Configure Renovate ([#6](https://github.com/specgraph/specgraph/issues/6)) ([0a627bf](https://github.com/specgraph/specgraph/commit/0a627bf4519521433eb9e151a33795148bced6c2))
* **deps:** update actions/cache action to v5 ([#25](https://github.com/specgraph/specgraph/issues/25)) ([13d90f5](https://github.com/specgraph/specgraph/commit/13d90f5a42e549a7b429b31e27a4c1373348384c))
* **deps:** update actions/checkout action to v6 ([#14](https://github.com/specgraph/specgraph/issues/14)) ([a6b4f1c](https://github.com/specgraph/specgraph/commit/a6b4f1ca68e896fc37e3598a9a910877a7ec769a))
* **deps:** update actions/setup-go action to v6 ([#21](https://github.com/specgraph/specgraph/issues/21)) ([7ecfca8](https://github.com/specgraph/specgraph/commit/7ecfca8babb52db21b16819005c6e3897189b852))
* **deps:** update actions/upload-pages-artifact action to v4 ([#15](https://github.com/specgraph/specgraph/issues/15)) ([f86df24](https://github.com/specgraph/specgraph/commit/f86df24a7140b5642883c44b7643312e0fe6f32a))
* **deps:** update alpine docker tag to v3.23 ([#10](https://github.com/specgraph/specgraph/issues/10)) ([55da31a](https://github.com/specgraph/specgraph/commit/55da31abfc77d132e30a0ad3872cab39e34d9aeb))
* **deps:** update astral-sh/setup-uv action to v7 ([#16](https://github.com/specgraph/specgraph/issues/16)) ([fa69828](https://github.com/specgraph/specgraph/commit/fa6982887065c9c81db416008791c9b4b551056a))
* **deps:** update dependency go to 1.26 ([#20](https://github.com/specgraph/specgraph/issues/20)) ([4e3718e](https://github.com/specgraph/specgraph/commit/4e3718e5568f31c2ad437679dd7b036237b20efe))
* **deps:** update golang docker tag to v1.26 ([#11](https://github.com/specgraph/specgraph/issues/11)) ([ebf12c5](https://github.com/specgraph/specgraph/commit/ebf12c5f0e781bd242b53cde75a486f89b26ed31))
* **main:** release 0.1.0 ([#49](https://github.com/specgraph/specgraph/issues/49)) ([fcd4b81](https://github.com/specgraph/specgraph/commit/fcd4b81df5000c6c4759a5f6cf6c0cad697a2532))
* **main:** release 0.1.1 ([#520](https://github.com/specgraph/specgraph/issues/520)) ([ef70ae7](https://github.com/specgraph/specgraph/commit/ef70ae7a1be886d6a5de2b43c4ad6f00a840c6fb))
* **main:** release 0.1.2 ([#522](https://github.com/specgraph/specgraph/issues/522)) ([b463d18](https://github.com/specgraph/specgraph/commit/b463d185ca6db602f593eaf30e69bfd4073d49a8))
* **main:** release 0.1.3 ([#527](https://github.com/specgraph/specgraph/issues/527)) ([7e1b255](https://github.com/specgraph/specgraph/commit/7e1b25579aa073eb919e2d1b0725ed818802f350))
* **main:** release 0.1.4 ([#529](https://github.com/specgraph/specgraph/issues/529)) ([dfbb73e](https://github.com/specgraph/specgraph/commit/dfbb73e98b8f61b8d556c34acdf9e8a81c129944))
* move module path to specgraph/specgraph ([#45](https://github.com/specgraph/specgraph/issues/45)) ([fb084cb](https://github.com/specgraph/specgraph/commit/fb084cb01a4c340e12a764d5464b43a75d2726e1))
* move repo from seanb4t/specgraph to specgraph/specgraph ([fb084cb](https://github.com/specgraph/specgraph/commit/fb084cb01a4c340e12a764d5464b43a75d2726e1))
* release 0.1.0 ([#48](https://github.com/specgraph/specgraph/issues/48)) ([31e695b](https://github.com/specgraph/specgraph/commit/31e695ba6b608b33248724154ff0fefb92c5b27e))
* trigger release 0.1.3 ([#526](https://github.com/specgraph/specgraph/issues/526)) ([4a92f1b](https://github.com/specgraph/specgraph/commit/4a92f1b33a8cde4b12070768d09a390443555115))

## [0.1.4](https://github.com/specgraph/specgraph/compare/v0.1.3...v0.1.4) (2026-03-21)


### Bug Fixes

* let goreleaser own GitHub releases, release-please only creates tags ([56051cf](https://github.com/specgraph/specgraph/commit/56051cfe16c34f680ae457388a741578e872679e))
* let goreleaser own GitHub releases, release-please only tags ([#528](https://github.com/specgraph/specgraph/issues/528)) ([56051cf](https://github.com/specgraph/specgraph/commit/56051cfe16c34f680ae457388a741578e872679e))

## [0.1.3](https://github.com/specgraph/specgraph/compare/v0.1.2...v0.1.3) (2026-03-21)


### Features

* add code quality and lefthook setup ([#3](https://github.com/specgraph/specgraph/issues/3)) ([970664e](https://github.com/specgraph/specgraph/commit/970664ea5a5a44ece3557eff3c9e247e1e009a88))
* add constitution subsystem (Slice 2) ([#7](https://github.com/specgraph/specgraph/issues/7)) ([10c2ee3](https://github.com/specgraph/specgraph/commit/10c2ee3180a2bf11dd8c179cb4ea4e018f54ace7))
* add extended services (health, claim, decision, graph) ([#4](https://github.com/specgraph/specgraph/issues/4)) ([9fd18e5](https://github.com/specgraph/specgraph/commit/9fd18e5496d5d664c9be4f72e04a583d573f4d5e))
* add Murmur3-128 content hash for change detection ([#39](https://github.com/specgraph/specgraph/issues/39)) ([b3c10b2](https://github.com/specgraph/specgraph/commit/b3c10b2f37f3ab1a9de5a6553ce63a656e48bb52))
* add Zensical doc site with GitHub Pages deployment ([7a1410e](https://github.com/specgraph/specgraph/commit/7a1410e0ae39485c3f7540ddaf8affc21cfd6cbd))
* **auth:** add authentication and authorization interceptor ([#38](https://github.com/specgraph/specgraph/issues/38)) ([f4fc6bf](https://github.com/specgraph/specgraph/commit/f4fc6bf2338020d521fe5ef626da2f8f5be2e1d5))
* ChangeLog graph nodes for version tracking ([#41](https://github.com/specgraph/specgraph/issues/41)) ([e5c00dc](https://github.com/specgraph/specgraph/commit/e5c00dc2def9d8cd408e327afdf5b38f94b3c212))
* **cli:** add report-progress, report-blocker, report-completion commands ([#36](https://github.com/specgraph/specgraph/issues/36)) ([18b09bb](https://github.com/specgraph/specgraph/commit/18b09bb8fb6a6a878fb8c4cc87baad8d9acfb640))
* content hash drift detection on DEPENDS_ON edges ([#43](https://github.com/specgraph/specgraph/issues/43)) ([6c86b33](https://github.com/specgraph/specgraph/commit/6c86b33fe59326557a309d1fcddf098bef0b5df3))
* **docker:** add Memgraph sizing profiles and persistence ([#23](https://github.com/specgraph/specgraph/issues/23)) ([9a2ab3f](https://github.com/specgraph/specgraph/commit/9a2ab3f82367204c9c880086b0f69e4bdb810a6a))
* **execution:** Slice 4 — domain types consistency & execution bundles ([#26](https://github.com/specgraph/specgraph/issues/26)) ([9942813](https://github.com/specgraph/specgraph/commit/9942813353c8afeb930d5de68aec808079fc338b))
* include design docs as hidden pages on site ([3f986a1](https://github.com/specgraph/specgraph/commit/3f986a1753269629b69c8c2baf2cfc8cfde0abe5))
* initial ([a46c950](https://github.com/specgraph/specgraph/commit/a46c950af7c44cf0d101bb9895878698dd5bf0d1))
* **lifecycle:** Slice 5 — spec lifecycle operations ([#27](https://github.com/specgraph/specgraph/issues/27)) ([5adf681](https://github.com/specgraph/specgraph/commit/5adf6813d3bccc7bd16b7279a90e9f451a8dc634))
* **plugin:** evolve authoring skills from CLI reference cards to partner personas ([c260969](https://github.com/specgraph/specgraph/commit/c260969862c3e536765ec26a15700f5d39eed1a5))
* **plugin:** evolve authoring skills into partner personas ([#34](https://github.com/specgraph/specgraph/issues/34)) ([c260969](https://github.com/specgraph/specgraph/commit/c260969862c3e536765ec26a15700f5d39eed1a5))
* **plugin:** Slice 7 — global daemon and Claude Code plugin ([#31](https://github.com/specgraph/specgraph/issues/31)) ([a8a07b4](https://github.com/specgraph/specgraph/commit/a8a07b47ed18fcc5e52de4c7423a7be30e772914))
* **proto:** add notes field to Spec + JSON output for show ([#35](https://github.com/specgraph/specgraph/issues/35)) ([524b09c](https://github.com/specgraph/specgraph/commit/524b09c990999f6c8840c9ab171ccbc776fe042f))
* Slice 3 — Authoring Funnel ([#8](https://github.com/specgraph/specgraph/issues/8)) ([8d15fd1](https://github.com/specgraph/specgraph/commit/8d15fd19d9e3df1102c6a2f5e4a1b17b1a077fca))
* **sync:** Slice 6 — sync adapters, tool injection, and CLI ([#30](https://github.com/specgraph/specgraph/issues/30)) ([c4c6ae7](https://github.com/specgraph/specgraph/commit/c4c6ae716dfc3bad7418085a75b42c1b1a81a93b))
* vertical slice — client/server architecture ([#1](https://github.com/specgraph/specgraph/issues/1)) ([50b504c](https://github.com/specgraph/specgraph/commit/50b504c67167cd52ab43fd956536a38ca8bacc08))


### Bug Fixes

* coordinate release-please and goreleaser — draft release handoff ([#521](https://github.com/specgraph/specgraph/issues/521)) ([fc299c4](https://github.com/specgraph/specgraph/commit/fc299c49d5bc91037cdaa955e734d6a5a3c42fd4))
* **deps:** update module github.com/testcontainers/testcontainers-go to v0.41.0 ([#28](https://github.com/specgraph/specgraph/issues/28)) ([2de880e](https://github.com/specgraph/specgraph/commit/2de880e92923fa4e8accb0a32793656ecd323db5))
* **deps:** update module golang.org/x/text to v0.35.0 ([#29](https://github.com/specgraph/specgraph/issues/29)) ([81fb5bf](https://github.com/specgraph/specgraph/commit/81fb5bff3ebaeffcfce4ea255444ee65a0841d09))
* Dockerfile for goreleaser — use pre-built binary ([#519](https://github.com/specgraph/specgraph/issues/519)) ([92c243b](https://github.com/specgraph/specgraph/commit/92c243bf2ae5626d70c6e0f84b4f9240b6a48275))
* **e2e:** address 4 open test suite findings ([#44](https://github.com/specgraph/specgraph/issues/44)) ([a029036](https://github.com/specgraph/specgraph/commit/a0290368fd4a56618187358b082fc8974aeff185))
* goreleaser Dockerfile + multi-arch Docker images + bump GH actions ([92c243b](https://github.com/specgraph/specgraph/commit/92c243bf2ae5626d70c6e0f84b4f9240b6a48275))
* remove draft:true from release-please config ([#525](https://github.com/specgraph/specgraph/issues/525)) ([7f2f138](https://github.com/specgraph/specgraph/commit/7f2f1389aec66fa060d6e87290bab9a51670e353))
* remove draft:true from release-please, add workflow_dispatch trigger ([7f2f138](https://github.com/specgraph/specgraph/commit/7f2f1389aec66fa060d6e87290bab9a51670e353))
* simple release flow — release-please creates release, goreleaser uploads assets ([#524](https://github.com/specgraph/specgraph/issues/524)) ([7f7b024](https://github.com/specgraph/specgraph/commit/7f7b024a5ea36acef6152778f821be00f0281112))
* wrap all multi-query write paths in RunInTransaction ([#42](https://github.com/specgraph/specgraph/issues/42)) ([04045e8](https://github.com/specgraph/specgraph/commit/04045e82e64d0cf49af5531c2cbf48d3cd2d4888))


### Documentation

* add CLAUDE.md for specgraph subproject ([b7f25f0](https://github.com/specgraph/specgraph/commit/b7f25f03230bd7e10ce0373ea0064b2429a44944))
* add implementation plans for Slices 3-7 ([72a8f6e](https://github.com/specgraph/specgraph/commit/72a8f6ee837f66e6b63807daba90f6b3e8c7641a))
* add implementation tracker and verification gates ([9261e5a](https://github.com/specgraph/specgraph/commit/9261e5a479af00b48236d737ed9a6cd4e2210607))
* add Slice 2 Constitution implementation plan ([fd8cda6](https://github.com/specgraph/specgraph/commit/fd8cda6759596eed4acf83afd83b9bd7b1cab984))
* add top-level README and align site docs ([#18](https://github.com/specgraph/specgraph/issues/18)) ([60e1437](https://github.com/specgraph/specgraph/commit/60e1437457511c18c0fd7ad63ec175664a2feea9))
* add vertical slice roadmap design for remaining implementation ([e736eb7](https://github.com/specgraph/specgraph/commit/e736eb7c1c442c5ba61fdc194519c4e3d663e05e))
* design for storage domain types and decision promotion ([f754076](https://github.com/specgraph/specgraph/commit/f7540767d0d116176e7ccb9255836f95b2f28bc7))
* implementation plan for storage domain types and decision promotion ([cfe9d63](https://github.com/specgraph/specgraph/commit/cfe9d63e8eadab66f574ec95e65ed55a2f50705d))
* mark slices 2-3 complete, remove stale rumdl exclude ([1a9c5c2](https://github.com/specgraph/specgraph/commit/1a9c5c22a40956316997932f624e688f4214d23d))
* Quick Start guide and documentation overhaul for 0.1.0 ([#515](https://github.com/specgraph/specgraph/issues/515)) ([a3c0766](https://github.com/specgraph/specgraph/commit/a3c07665fd825fca692b0bcac4752d04d9f3cff9))
* **site:** add example spec page ([#33](https://github.com/specgraph/specgraph/issues/33)) ([3c719f6](https://github.com/specgraph/specgraph/commit/3c719f65f2cc73492c2f4a9a59134467b3dad597))
* **site:** add example spec page with worked OAuth2 rotation scenario ([3c719f6](https://github.com/specgraph/specgraph/commit/3c719f65f2cc73492c2f4a9a59134467b3dad597))
* **site:** build out documentation site ([#9](https://github.com/specgraph/specgraph/issues/9)) ([66af3dc](https://github.com/specgraph/specgraph/commit/66af3dca602d5f926b20739c51c3775c319bbb16))
* sync site docs and README with current codebase ([bd71843](https://github.com/specgraph/specgraph/commit/bd7184358633c4f6e9dac63f9038acf878440079))
* update CLAUDE.md and add Claude Code automations ([9d17883](https://github.com/specgraph/specgraph/commit/9d1788359a70f05ea3ae71380d9778c3b7b36b8e))
* update CLAUDE.md with test gotchas, remove stale status ([3df0d54](https://github.com/specgraph/specgraph/commit/3df0d54cd153755cdd2fca13ec86e82a695e0acb))


### Performance

* share single memgraph container across integration tests ([#516](https://github.com/specgraph/specgraph/issues/516)) ([a95ac45](https://github.com/specgraph/specgraph/commit/a95ac459e6fe63457cb957ec44d0444edb891b87))
* share single memgraph container across integration tests (spgr-mfot) ([a95ac45](https://github.com/specgraph/specgraph/commit/a95ac459e6fe63457cb957ec44d0444edb891b87))


### Code Refactoring

* Slice 3.5 — Scanner removal & documentation cleanup ([#22](https://github.com/specgraph/specgraph/issues/22)) ([f06a476](https://github.com/specgraph/specgraph/commit/f06a47685fe1ce27ed5a265ff209448bd04b414c))
* storage domain types and decision promotion ([#24](https://github.com/specgraph/specgraph/issues/24)) ([836abee](https://github.com/specgraph/specgraph/commit/836abeea8a96d04898d874aaddc6b4a574850690))


### Tests

* add comprehensive E2E test system ([#19](https://github.com/specgraph/specgraph/issues/19)) ([6ecf4e5](https://github.com/specgraph/specgraph/commit/6ecf4e585a21a252fdc18e16e4a6ebcfc109310c))
* **e2e:** implement 3-tier E2E test suite ([#32](https://github.com/specgraph/specgraph/issues/32)) ([de12bbc](https://github.com/specgraph/specgraph/commit/de12bbceaa1f437ee37d98f4d76e7f2f7817611f))
* **e2e:** implement 3-tier E2E test suite with full design doc coverage ([de12bbc](https://github.com/specgraph/specgraph/commit/de12bbceaa1f437ee37d98f4d76e7f2f7817611f))
* **integration:** add DISTINCT regression test for GetExecutionEvents ([#37](https://github.com/specgraph/specgraph/issues/37)) ([2b17445](https://github.com/specgraph/specgraph/commit/2b17445a8421f114d6f34ef3f1fca361afa32dcc))
* **spgr-g8i.16:** add diamond+cycle regression tests for detectCycles ([5adf681](https://github.com/specgraph/specgraph/commit/5adf6813d3bccc7bd16b7279a90e9f451a8dc634))


### CI

* add release-please + goreleaser infrastructure ([#46](https://github.com/specgraph/specgraph/issues/46)) ([1fd22d3](https://github.com/specgraph/specgraph/commit/1fd22d3d9ab3c80360a5e0d9117741192ddd26b8))
* exclude auto-generated CHANGELOG.md from markdown lint ([#517](https://github.com/specgraph/specgraph/issues/517)) ([7106861](https://github.com/specgraph/specgraph/commit/71068619c63a7a7f9749fa98e44287dceed001e3))
* exclude CHANGELOG.md from lint, use PAT for release-please to trigger CI on release PRs ([92805cf](https://github.com/specgraph/specgraph/commit/92805cfff0727fa5287349ab7faec36ecbef6c0f))
* use PAT for release-please to trigger CI on release PRs ([#518](https://github.com/specgraph/specgraph/issues/518)) ([92805cf](https://github.com/specgraph/specgraph/commit/92805cfff0727fa5287349ab7faec36ecbef6c0f))


### Build

* **deps:** bump golang.org/x/crypto from 0.43.0 to 0.45.0 ([#2](https://github.com/specgraph/specgraph/issues/2)) ([a4b88f8](https://github.com/specgraph/specgraph/commit/a4b88f82d2c7b71fbd89a48db4fb48a1d34b5b87))


### Miscellaneous

* add beads ([#5](https://github.com/specgraph/specgraph/issues/5)) ([d10d49d](https://github.com/specgraph/specgraph/commit/d10d49d4157b1376c5a646eff87bd13d63256ee2))
* Configure Renovate ([#6](https://github.com/specgraph/specgraph/issues/6)) ([0a627bf](https://github.com/specgraph/specgraph/commit/0a627bf4519521433eb9e151a33795148bced6c2))
* **deps:** update actions/cache action to v5 ([#25](https://github.com/specgraph/specgraph/issues/25)) ([13d90f5](https://github.com/specgraph/specgraph/commit/13d90f5a42e549a7b429b31e27a4c1373348384c))
* **deps:** update actions/checkout action to v6 ([#14](https://github.com/specgraph/specgraph/issues/14)) ([a6b4f1c](https://github.com/specgraph/specgraph/commit/a6b4f1ca68e896fc37e3598a9a910877a7ec769a))
* **deps:** update actions/setup-go action to v6 ([#21](https://github.com/specgraph/specgraph/issues/21)) ([7ecfca8](https://github.com/specgraph/specgraph/commit/7ecfca8babb52db21b16819005c6e3897189b852))
* **deps:** update actions/upload-pages-artifact action to v4 ([#15](https://github.com/specgraph/specgraph/issues/15)) ([f86df24](https://github.com/specgraph/specgraph/commit/f86df24a7140b5642883c44b7643312e0fe6f32a))
* **deps:** update alpine docker tag to v3.23 ([#10](https://github.com/specgraph/specgraph/issues/10)) ([55da31a](https://github.com/specgraph/specgraph/commit/55da31abfc77d132e30a0ad3872cab39e34d9aeb))
* **deps:** update astral-sh/setup-uv action to v7 ([#16](https://github.com/specgraph/specgraph/issues/16)) ([fa69828](https://github.com/specgraph/specgraph/commit/fa6982887065c9c81db416008791c9b4b551056a))
* **deps:** update dependency go to 1.26 ([#20](https://github.com/specgraph/specgraph/issues/20)) ([4e3718e](https://github.com/specgraph/specgraph/commit/4e3718e5568f31c2ad437679dd7b036237b20efe))
* **deps:** update golang docker tag to v1.26 ([#11](https://github.com/specgraph/specgraph/issues/11)) ([ebf12c5](https://github.com/specgraph/specgraph/commit/ebf12c5f0e781bd242b53cde75a486f89b26ed31))
* **main:** release 0.1.0 ([#49](https://github.com/specgraph/specgraph/issues/49)) ([fcd4b81](https://github.com/specgraph/specgraph/commit/fcd4b81df5000c6c4759a5f6cf6c0cad697a2532))
* **main:** release 0.1.1 ([#520](https://github.com/specgraph/specgraph/issues/520)) ([ef70ae7](https://github.com/specgraph/specgraph/commit/ef70ae7a1be886d6a5de2b43c4ad6f00a840c6fb))
* **main:** release 0.1.2 ([#522](https://github.com/specgraph/specgraph/issues/522)) ([b463d18](https://github.com/specgraph/specgraph/commit/b463d185ca6db602f593eaf30e69bfd4073d49a8))
* move module path to specgraph/specgraph ([#45](https://github.com/specgraph/specgraph/issues/45)) ([fb084cb](https://github.com/specgraph/specgraph/commit/fb084cb01a4c340e12a764d5464b43a75d2726e1))
* move repo from seanb4t/specgraph to specgraph/specgraph ([fb084cb](https://github.com/specgraph/specgraph/commit/fb084cb01a4c340e12a764d5464b43a75d2726e1))
* release 0.1.0 ([#48](https://github.com/specgraph/specgraph/issues/48)) ([31e695b](https://github.com/specgraph/specgraph/commit/31e695ba6b608b33248724154ff0fefb92c5b27e))
* trigger release 0.1.3 ([#526](https://github.com/specgraph/specgraph/issues/526)) ([4a92f1b](https://github.com/specgraph/specgraph/commit/4a92f1b33a8cde4b12070768d09a390443555115))

## [0.1.2](https://github.com/specgraph/specgraph/compare/v0.1.1...v0.1.2) (2026-03-21)


### Bug Fixes

* coordinate release-please and goreleaser — draft release handoff ([#521](https://github.com/specgraph/specgraph/issues/521)) ([fc299c4](https://github.com/specgraph/specgraph/commit/fc299c49d5bc91037cdaa955e734d6a5a3c42fd4))

## [0.1.1](https://github.com/specgraph/specgraph/compare/v0.1.0...v0.1.1) (2026-03-21)


### Bug Fixes

* Dockerfile for goreleaser — use pre-built binary ([#519](https://github.com/specgraph/specgraph/issues/519)) ([92c243b](https://github.com/specgraph/specgraph/commit/92c243bf2ae5626d70c6e0f84b4f9240b6a48275))
* goreleaser Dockerfile + multi-arch Docker images + bump GH actions ([92c243b](https://github.com/specgraph/specgraph/commit/92c243bf2ae5626d70c6e0f84b4f9240b6a48275))

## 0.1.0 (2026-03-21)


### Features

* add code quality and lefthook setup ([#3](https://github.com/specgraph/specgraph/issues/3)) ([970664e](https://github.com/specgraph/specgraph/commit/970664ea5a5a44ece3557eff3c9e247e1e009a88))
* add constitution subsystem (Slice 2) ([#7](https://github.com/specgraph/specgraph/issues/7)) ([10c2ee3](https://github.com/specgraph/specgraph/commit/10c2ee3180a2bf11dd8c179cb4ea4e018f54ace7))
* add extended services (health, claim, decision, graph) ([#4](https://github.com/specgraph/specgraph/issues/4)) ([9fd18e5](https://github.com/specgraph/specgraph/commit/9fd18e5496d5d664c9be4f72e04a583d573f4d5e))
* add Murmur3-128 content hash for change detection ([#39](https://github.com/specgraph/specgraph/issues/39)) ([b3c10b2](https://github.com/specgraph/specgraph/commit/b3c10b2f37f3ab1a9de5a6553ce63a656e48bb52))
* add Zensical doc site with GitHub Pages deployment ([7a1410e](https://github.com/specgraph/specgraph/commit/7a1410e0ae39485c3f7540ddaf8affc21cfd6cbd))
* **auth:** add authentication and authorization interceptor ([#38](https://github.com/specgraph/specgraph/issues/38)) ([f4fc6bf](https://github.com/specgraph/specgraph/commit/f4fc6bf2338020d521fe5ef626da2f8f5be2e1d5))
* ChangeLog graph nodes for version tracking ([#41](https://github.com/specgraph/specgraph/issues/41)) ([e5c00dc](https://github.com/specgraph/specgraph/commit/e5c00dc2def9d8cd408e327afdf5b38f94b3c212))
* **cli:** add report-progress, report-blocker, report-completion commands ([#36](https://github.com/specgraph/specgraph/issues/36)) ([18b09bb](https://github.com/specgraph/specgraph/commit/18b09bb8fb6a6a878fb8c4cc87baad8d9acfb640))
* content hash drift detection on DEPENDS_ON edges ([#43](https://github.com/specgraph/specgraph/issues/43)) ([6c86b33](https://github.com/specgraph/specgraph/commit/6c86b33fe59326557a309d1fcddf098bef0b5df3))
* **docker:** add Memgraph sizing profiles and persistence ([#23](https://github.com/specgraph/specgraph/issues/23)) ([9a2ab3f](https://github.com/specgraph/specgraph/commit/9a2ab3f82367204c9c880086b0f69e4bdb810a6a))
* **execution:** Slice 4 — domain types consistency & execution bundles ([#26](https://github.com/specgraph/specgraph/issues/26)) ([9942813](https://github.com/specgraph/specgraph/commit/9942813353c8afeb930d5de68aec808079fc338b))
* include design docs as hidden pages on site ([3f986a1](https://github.com/specgraph/specgraph/commit/3f986a1753269629b69c8c2baf2cfc8cfde0abe5))
* initial ([a46c950](https://github.com/specgraph/specgraph/commit/a46c950af7c44cf0d101bb9895878698dd5bf0d1))
* **lifecycle:** Slice 5 — spec lifecycle operations ([#27](https://github.com/specgraph/specgraph/issues/27)) ([5adf681](https://github.com/specgraph/specgraph/commit/5adf6813d3bccc7bd16b7279a90e9f451a8dc634))
* **plugin:** evolve authoring skills from CLI reference cards to partner personas ([c260969](https://github.com/specgraph/specgraph/commit/c260969862c3e536765ec26a15700f5d39eed1a5))
* **plugin:** evolve authoring skills into partner personas ([#34](https://github.com/specgraph/specgraph/issues/34)) ([c260969](https://github.com/specgraph/specgraph/commit/c260969862c3e536765ec26a15700f5d39eed1a5))
* **plugin:** Slice 7 — global daemon and Claude Code plugin ([#31](https://github.com/specgraph/specgraph/issues/31)) ([a8a07b4](https://github.com/specgraph/specgraph/commit/a8a07b47ed18fcc5e52de4c7423a7be30e772914))
* **proto:** add notes field to Spec + JSON output for show ([#35](https://github.com/specgraph/specgraph/issues/35)) ([524b09c](https://github.com/specgraph/specgraph/commit/524b09c990999f6c8840c9ab171ccbc776fe042f))
* Slice 3 — Authoring Funnel ([#8](https://github.com/specgraph/specgraph/issues/8)) ([8d15fd1](https://github.com/specgraph/specgraph/commit/8d15fd19d9e3df1102c6a2f5e4a1b17b1a077fca))
* **sync:** Slice 6 — sync adapters, tool injection, and CLI ([#30](https://github.com/specgraph/specgraph/issues/30)) ([c4c6ae7](https://github.com/specgraph/specgraph/commit/c4c6ae716dfc3bad7418085a75b42c1b1a81a93b))
* vertical slice — client/server architecture ([#1](https://github.com/specgraph/specgraph/issues/1)) ([50b504c](https://github.com/specgraph/specgraph/commit/50b504c67167cd52ab43fd956536a38ca8bacc08))


### Bug Fixes

* **deps:** update module github.com/testcontainers/testcontainers-go to v0.41.0 ([#28](https://github.com/specgraph/specgraph/issues/28)) ([2de880e](https://github.com/specgraph/specgraph/commit/2de880e92923fa4e8accb0a32793656ecd323db5))
* **deps:** update module golang.org/x/text to v0.35.0 ([#29](https://github.com/specgraph/specgraph/issues/29)) ([81fb5bf](https://github.com/specgraph/specgraph/commit/81fb5bff3ebaeffcfce4ea255444ee65a0841d09))
* **e2e:** address 4 open test suite findings ([#44](https://github.com/specgraph/specgraph/issues/44)) ([a029036](https://github.com/specgraph/specgraph/commit/a0290368fd4a56618187358b082fc8974aeff185))
* wrap all multi-query write paths in RunInTransaction ([#42](https://github.com/specgraph/specgraph/issues/42)) ([04045e8](https://github.com/specgraph/specgraph/commit/04045e82e64d0cf49af5531c2cbf48d3cd2d4888))


### Documentation

* add CLAUDE.md for specgraph subproject ([b7f25f0](https://github.com/specgraph/specgraph/commit/b7f25f03230bd7e10ce0373ea0064b2429a44944))
* add implementation plans for Slices 3-7 ([72a8f6e](https://github.com/specgraph/specgraph/commit/72a8f6ee837f66e6b63807daba90f6b3e8c7641a))
* add implementation tracker and verification gates ([9261e5a](https://github.com/specgraph/specgraph/commit/9261e5a479af00b48236d737ed9a6cd4e2210607))
* add Slice 2 Constitution implementation plan ([fd8cda6](https://github.com/specgraph/specgraph/commit/fd8cda6759596eed4acf83afd83b9bd7b1cab984))
* add top-level README and align site docs ([#18](https://github.com/specgraph/specgraph/issues/18)) ([60e1437](https://github.com/specgraph/specgraph/commit/60e1437457511c18c0fd7ad63ec175664a2feea9))
* add vertical slice roadmap design for remaining implementation ([e736eb7](https://github.com/specgraph/specgraph/commit/e736eb7c1c442c5ba61fdc194519c4e3d663e05e))
* design for storage domain types and decision promotion ([f754076](https://github.com/specgraph/specgraph/commit/f7540767d0d116176e7ccb9255836f95b2f28bc7))
* implementation plan for storage domain types and decision promotion ([cfe9d63](https://github.com/specgraph/specgraph/commit/cfe9d63e8eadab66f574ec95e65ed55a2f50705d))
* mark slices 2-3 complete, remove stale rumdl exclude ([1a9c5c2](https://github.com/specgraph/specgraph/commit/1a9c5c22a40956316997932f624e688f4214d23d))
* Quick Start guide and documentation overhaul for 0.1.0 ([#515](https://github.com/specgraph/specgraph/issues/515)) ([a3c0766](https://github.com/specgraph/specgraph/commit/a3c07665fd825fca692b0bcac4752d04d9f3cff9))
* **site:** add example spec page ([#33](https://github.com/specgraph/specgraph/issues/33)) ([3c719f6](https://github.com/specgraph/specgraph/commit/3c719f65f2cc73492c2f4a9a59134467b3dad597))
* **site:** add example spec page with worked OAuth2 rotation scenario ([3c719f6](https://github.com/specgraph/specgraph/commit/3c719f65f2cc73492c2f4a9a59134467b3dad597))
* **site:** build out documentation site ([#9](https://github.com/specgraph/specgraph/issues/9)) ([66af3dc](https://github.com/specgraph/specgraph/commit/66af3dca602d5f926b20739c51c3775c319bbb16))
* sync site docs and README with current codebase ([bd71843](https://github.com/specgraph/specgraph/commit/bd7184358633c4f6e9dac63f9038acf878440079))
* update CLAUDE.md and add Claude Code automations ([9d17883](https://github.com/specgraph/specgraph/commit/9d1788359a70f05ea3ae71380d9778c3b7b36b8e))
* update CLAUDE.md with test gotchas, remove stale status ([3df0d54](https://github.com/specgraph/specgraph/commit/3df0d54cd153755cdd2fca13ec86e82a695e0acb))


### Performance

* share single memgraph container across integration tests ([#516](https://github.com/specgraph/specgraph/issues/516)) ([a95ac45](https://github.com/specgraph/specgraph/commit/a95ac459e6fe63457cb957ec44d0444edb891b87))
* share single memgraph container across integration tests (spgr-mfot) ([a95ac45](https://github.com/specgraph/specgraph/commit/a95ac459e6fe63457cb957ec44d0444edb891b87))


### Code Refactoring

* Slice 3.5 — Scanner removal & documentation cleanup ([#22](https://github.com/specgraph/specgraph/issues/22)) ([f06a476](https://github.com/specgraph/specgraph/commit/f06a47685fe1ce27ed5a265ff209448bd04b414c))
* storage domain types and decision promotion ([#24](https://github.com/specgraph/specgraph/issues/24)) ([836abee](https://github.com/specgraph/specgraph/commit/836abeea8a96d04898d874aaddc6b4a574850690))


### Tests

* add comprehensive E2E test system ([#19](https://github.com/specgraph/specgraph/issues/19)) ([6ecf4e5](https://github.com/specgraph/specgraph/commit/6ecf4e585a21a252fdc18e16e4a6ebcfc109310c))
* **e2e:** implement 3-tier E2E test suite ([#32](https://github.com/specgraph/specgraph/issues/32)) ([de12bbc](https://github.com/specgraph/specgraph/commit/de12bbceaa1f437ee37d98f4d76e7f2f7817611f))
* **e2e:** implement 3-tier E2E test suite with full design doc coverage ([de12bbc](https://github.com/specgraph/specgraph/commit/de12bbceaa1f437ee37d98f4d76e7f2f7817611f))
* **integration:** add DISTINCT regression test for GetExecutionEvents ([#37](https://github.com/specgraph/specgraph/issues/37)) ([2b17445](https://github.com/specgraph/specgraph/commit/2b17445a8421f114d6f34ef3f1fca361afa32dcc))
* **spgr-g8i.16:** add diamond+cycle regression tests for detectCycles ([5adf681](https://github.com/specgraph/specgraph/commit/5adf6813d3bccc7bd16b7279a90e9f451a8dc634))


### CI

* add release-please + goreleaser infrastructure ([#46](https://github.com/specgraph/specgraph/issues/46)) ([1fd22d3](https://github.com/specgraph/specgraph/commit/1fd22d3d9ab3c80360a5e0d9117741192ddd26b8))
* exclude auto-generated CHANGELOG.md from markdown lint ([#517](https://github.com/specgraph/specgraph/issues/517)) ([7106861](https://github.com/specgraph/specgraph/commit/71068619c63a7a7f9749fa98e44287dceed001e3))
* exclude CHANGELOG.md from lint, use PAT for release-please to trigger CI on release PRs ([92805cf](https://github.com/specgraph/specgraph/commit/92805cfff0727fa5287349ab7faec36ecbef6c0f))
* use PAT for release-please to trigger CI on release PRs ([#518](https://github.com/specgraph/specgraph/issues/518)) ([92805cf](https://github.com/specgraph/specgraph/commit/92805cfff0727fa5287349ab7faec36ecbef6c0f))


### Build

* **deps:** bump golang.org/x/crypto from 0.43.0 to 0.45.0 ([#2](https://github.com/specgraph/specgraph/issues/2)) ([a4b88f8](https://github.com/specgraph/specgraph/commit/a4b88f82d2c7b71fbd89a48db4fb48a1d34b5b87))


### Miscellaneous

* add beads ([#5](https://github.com/specgraph/specgraph/issues/5)) ([d10d49d](https://github.com/specgraph/specgraph/commit/d10d49d4157b1376c5a646eff87bd13d63256ee2))
* Configure Renovate ([#6](https://github.com/specgraph/specgraph/issues/6)) ([0a627bf](https://github.com/specgraph/specgraph/commit/0a627bf4519521433eb9e151a33795148bced6c2))
* **deps:** update actions/cache action to v5 ([#25](https://github.com/specgraph/specgraph/issues/25)) ([13d90f5](https://github.com/specgraph/specgraph/commit/13d90f5a42e549a7b429b31e27a4c1373348384c))
* **deps:** update actions/checkout action to v6 ([#14](https://github.com/specgraph/specgraph/issues/14)) ([a6b4f1c](https://github.com/specgraph/specgraph/commit/a6b4f1ca68e896fc37e3598a9a910877a7ec769a))
* **deps:** update actions/setup-go action to v6 ([#21](https://github.com/specgraph/specgraph/issues/21)) ([7ecfca8](https://github.com/specgraph/specgraph/commit/7ecfca8babb52db21b16819005c6e3897189b852))
* **deps:** update actions/upload-pages-artifact action to v4 ([#15](https://github.com/specgraph/specgraph/issues/15)) ([f86df24](https://github.com/specgraph/specgraph/commit/f86df24a7140b5642883c44b7643312e0fe6f32a))
* **deps:** update alpine docker tag to v3.23 ([#10](https://github.com/specgraph/specgraph/issues/10)) ([55da31a](https://github.com/specgraph/specgraph/commit/55da31abfc77d132e30a0ad3872cab39e34d9aeb))
* **deps:** update astral-sh/setup-uv action to v7 ([#16](https://github.com/specgraph/specgraph/issues/16)) ([fa69828](https://github.com/specgraph/specgraph/commit/fa6982887065c9c81db416008791c9b4b551056a))
* **deps:** update dependency go to 1.26 ([#20](https://github.com/specgraph/specgraph/issues/20)) ([4e3718e](https://github.com/specgraph/specgraph/commit/4e3718e5568f31c2ad437679dd7b036237b20efe))
* **deps:** update golang docker tag to v1.26 ([#11](https://github.com/specgraph/specgraph/issues/11)) ([ebf12c5](https://github.com/specgraph/specgraph/commit/ebf12c5f0e781bd242b53cde75a486f89b26ed31))
* move module path to specgraph/specgraph ([#45](https://github.com/specgraph/specgraph/issues/45)) ([fb084cb](https://github.com/specgraph/specgraph/commit/fb084cb01a4c340e12a764d5464b43a75d2726e1))
* move repo from seanb4t/specgraph to specgraph/specgraph ([fb084cb](https://github.com/specgraph/specgraph/commit/fb084cb01a4c340e12a764d5464b43a75d2726e1))
* release 0.1.0 ([#48](https://github.com/specgraph/specgraph/issues/48)) ([31e695b](https://github.com/specgraph/specgraph/commit/31e695ba6b608b33248724154ff0fefb92c5b27e))
