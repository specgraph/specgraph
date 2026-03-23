
// this file is generated — do not edit it


/// <reference types="@sveltejs/kit" />

/**
 * This module provides access to environment variables that are injected _statically_ into your bundle at build time and are limited to _private_ access.
 * 
 * |         | Runtime                                                                    | Build time                                                               |
 * | ------- | -------------------------------------------------------------------------- | ------------------------------------------------------------------------ |
 * | Private | [`$env/dynamic/private`](https://svelte.dev/docs/kit/$env-dynamic-private) | [`$env/static/private`](https://svelte.dev/docs/kit/$env-static-private) |
 * | Public  | [`$env/dynamic/public`](https://svelte.dev/docs/kit/$env-dynamic-public)   | [`$env/static/public`](https://svelte.dev/docs/kit/$env-static-public)   |
 * 
 * Static environment variables are [loaded by Vite](https://vitejs.dev/guide/env-and-mode.html#env-files) from `.env` files and `process.env` at build time and then statically injected into your bundle at build time, enabling optimisations like dead code elimination.
 * 
 * **_Private_ access:**
 * 
 * - This module cannot be imported into client-side code
 * - This module only includes variables that _do not_ begin with [`config.kit.env.publicPrefix`](https://svelte.dev/docs/kit/configuration#env) _and do_ start with [`config.kit.env.privatePrefix`](https://svelte.dev/docs/kit/configuration#env) (if configured)
 * 
 * For example, given the following build time environment:
 * 
 * ```env
 * ENVIRONMENT=production
 * PUBLIC_BASE_URL=http://site.com
 * ```
 * 
 * With the default `publicPrefix` and `privatePrefix`:
 * 
 * ```ts
 * import { ENVIRONMENT, PUBLIC_BASE_URL } from '$env/static/private';
 * 
 * console.log(ENVIRONMENT); // => "production"
 * console.log(PUBLIC_BASE_URL); // => throws error during build
 * ```
 * 
 * The above values will be the same _even if_ different values for `ENVIRONMENT` or `PUBLIC_BASE_URL` are set at runtime, as they are statically replaced in your code with their build time values.
 */
declare module '$env/static/private' {
	export const EZA_LAID_OPTIONS: string;
	export const OP_PLUGIN_ALIASES_SOURCED: string;
	export const MANPATH: string;
	export const STARSHIP_SHELL: string;
	export const EZA_LD_OPTIONS: string;
	export const GHOSTTY_RESOURCES_DIR: string;
	export const NoDefaultCurrentDirectoryInExePath: string;
	export const CLAUDE_CODE_ENTRYPOINT: string;
	export const TERM_PROGRAM: string;
	export const EZA_L_OPTIONS: string;
	export const NODE: string;
	export const INIT_CWD: string;
	export const EZA_LAA_OPTIONS: string;
	export const SHELL: string;
	export const TERM: string;
	export const EZA_LA_OPTIONS: string;
	export const __FISH_EZA_EXPANDED_OPT_NAME: string;
	export const EZA_LAAD_OPTIONS: string;
	export const HOMEBREW_REPOSITORY: string;
	export const TMPDIR: string;
	export const GOBIN: string;
	export const DIRENV_DIR: string;
	export const EZA_LL_OPTIONS: string;
	export const TERM_PROGRAM_VERSION: string;
	export const VAULT_ADDR: string;
	export const npm_config_npm_globalconfig: string;
	export const EZA_LID_OPTIONS: string;
	export const SDKMAN_PLATFORM: string;
	export const npm_config_registry: string;
	export const GIT_EDITOR: string;
	export const EZA_LO_OPTIONS: string;
	export const USER: string;
	export const EZA_LT_OPTIONS: string;
	export const COMMAND_MODE: string;
	export const SDKMAN_CANDIDATES_API: string;
	export const SDKMAN_ENV: string;
	export const npm_config_globalconfig: string;
	export const PNPM_SCRIPT_SRC_DIR: string;
	export const ENABLE_TOOL_SEARCH: string;
	export const KUBECONFIG: string;
	export const CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS: string;
	export const SSH_AUTH_SOCK: string;
	export const __CF_USER_TEXT_ENCODING: string;
	export const GIT_AUTHOR_NAME: string;
	export const QUARKUS_HOME: string;
	export const npm_execpath: string;
	export const PAGER: string;
	export const DIRENV_WATCHES: string;
	export const TMUX: string;
	export const npm_config_frozen_lockfile: string;
	export const npm_config_verify_deps_before_run: string;
	export const EZA_LC_OPTIONS: string;
	export const PATH: string;
	export const MICRONAUT_HOME: string;
	export const SDKMAN_OLD_PWD: string;
	export const GHOSTTY_SHELL_FEATURES: string;
	export const LaunchInstanceID: string;
	export const npm_package_json: string;
	export const __CFBundleIdentifier: string;
	export const fish_tmux_term: string;
	export const PWD: string;
	export const npm_command: string;
	export const JAVA_HOME: string;
	export const EDITOR: string;
	export const EZA_LE_OPTIONS: string;
	export const OTEL_EXPORTER_OTLP_METRICS_TEMPORALITY_PREFERENCE: string;
	export const npm_config__jsr_registry: string;
	export const npm_lifecycle_event: string;
	export const LANG: string;
	export const npm_package_name: string;
	export const NODE_PATH: string;
	export const TMUX_PANE: string;
	export const XPC_FLAGS: string;
	export const ATUIN_TMUX_POPUP: string;
	export const EZA_LG_OPTIONS: string;
	export const __FISH_EZA_EXPANDED: string;
	export const fish_tmux_autostarted: string;
	export const npm_config_node_gyp: string;
	export const DIRENV_FILE: string;
	export const XPC_SERVICE_NAME: string;
	export const npm_package_version: string;
	export const pnpm_config_verify_deps_before_run: string;
	export const fish_tmux_config: string;
	export const EZA_STANDARD_OPTIONS: string;
	export const HOME: string;
	export const SHLVL: string;
	export const __FISH_EZA_SORT_OPTIONS: string;
	export const EZA_LAI_OPTIONS: string;
	export const TERMINFO: string;
	export const __FISH_EZA_ALIASES: string;
	export const EZA_LI_OPTIONS: string;
	export const HOMEBREW_PREFIX: string;
	export const EZA_LAD_OPTIONS: string;
	export const LOGNAME: string;
	export const STARSHIP_SESSION_KEY: string;
	export const ZELLIJ_CONFIG_DIR: string;
	export const ATUIN_SESSION: string;
	export const SDKMAN_DIR: string;
	export const VISUAL: string;
	export const npm_lifecycle_script: string;
	export const EZA_LAAID_OPTIONS: string;
	export const EZA_LAAI_OPTIONS: string;
	export const XDG_DATA_DIRS: string;
	export const COREPACK_ENABLE_AUTO_PIN: string;
	export const GHOSTTY_BIN_DIR: string;
	export const TMUX_PLUGIN_MANAGER_PATH: string;
	export const BUN_INSTALL: string;
	export const GITHUB_TOKEN: string;
	export const GOPATH: string;
	export const __FISH_EZA_OPT_NAMES: string;
	export const npm_config_user_agent: string;
	export const HOMEBREW_CELLAR: string;
	export const INFOPATH: string;
	export const SDKMAN_CANDIDATES_DIR: string;
	export const GIT_AUTHOR_EMAIL: string;
	export const OSLogRateLimit: string;
	export const DIRENV_DIFF: string;
	export const __FISH_EZA_BASE_ALIASES: string;
	export const ATUIN_SHLVL: string;
	export const CLAUDECODE: string;
	export const SECURITYSESSIONID: string;
	export const SDKMAN_OFFLINE_MODE: string;
	export const COLORTERM: string;
	export const GH_TOKEN: string;
	export const npm_node_execpath: string;
	export const NODE_ENV: string;
}

/**
 * This module provides access to environment variables that are injected _statically_ into your bundle at build time and are _publicly_ accessible.
 * 
 * |         | Runtime                                                                    | Build time                                                               |
 * | ------- | -------------------------------------------------------------------------- | ------------------------------------------------------------------------ |
 * | Private | [`$env/dynamic/private`](https://svelte.dev/docs/kit/$env-dynamic-private) | [`$env/static/private`](https://svelte.dev/docs/kit/$env-static-private) |
 * | Public  | [`$env/dynamic/public`](https://svelte.dev/docs/kit/$env-dynamic-public)   | [`$env/static/public`](https://svelte.dev/docs/kit/$env-static-public)   |
 * 
 * Static environment variables are [loaded by Vite](https://vitejs.dev/guide/env-and-mode.html#env-files) from `.env` files and `process.env` at build time and then statically injected into your bundle at build time, enabling optimisations like dead code elimination.
 * 
 * **_Public_ access:**
 * 
 * - This module _can_ be imported into client-side code
 * - **Only** variables that begin with [`config.kit.env.publicPrefix`](https://svelte.dev/docs/kit/configuration#env) (which defaults to `PUBLIC_`) are included
 * 
 * For example, given the following build time environment:
 * 
 * ```env
 * ENVIRONMENT=production
 * PUBLIC_BASE_URL=http://site.com
 * ```
 * 
 * With the default `publicPrefix` and `privatePrefix`:
 * 
 * ```ts
 * import { ENVIRONMENT, PUBLIC_BASE_URL } from '$env/static/public';
 * 
 * console.log(ENVIRONMENT); // => throws error during build
 * console.log(PUBLIC_BASE_URL); // => "http://site.com"
 * ```
 * 
 * The above values will be the same _even if_ different values for `ENVIRONMENT` or `PUBLIC_BASE_URL` are set at runtime, as they are statically replaced in your code with their build time values.
 */
declare module '$env/static/public' {
	
}

/**
 * This module provides access to environment variables set _dynamically_ at runtime and that are limited to _private_ access.
 * 
 * |         | Runtime                                                                    | Build time                                                               |
 * | ------- | -------------------------------------------------------------------------- | ------------------------------------------------------------------------ |
 * | Private | [`$env/dynamic/private`](https://svelte.dev/docs/kit/$env-dynamic-private) | [`$env/static/private`](https://svelte.dev/docs/kit/$env-static-private) |
 * | Public  | [`$env/dynamic/public`](https://svelte.dev/docs/kit/$env-dynamic-public)   | [`$env/static/public`](https://svelte.dev/docs/kit/$env-static-public)   |
 * 
 * Dynamic environment variables are defined by the platform you're running on. For example if you're using [`adapter-node`](https://github.com/sveltejs/kit/tree/main/packages/adapter-node) (or running [`vite preview`](https://svelte.dev/docs/kit/cli)), this is equivalent to `process.env`.
 * 
 * **_Private_ access:**
 * 
 * - This module cannot be imported into client-side code
 * - This module includes variables that _do not_ begin with [`config.kit.env.publicPrefix`](https://svelte.dev/docs/kit/configuration#env) _and do_ start with [`config.kit.env.privatePrefix`](https://svelte.dev/docs/kit/configuration#env) (if configured)
 * 
 * > [!NOTE] In `dev`, `$env/dynamic` includes environment variables from `.env`. In `prod`, this behavior will depend on your adapter.
 * 
 * > [!NOTE] To get correct types, environment variables referenced in your code should be declared (for example in an `.env` file), even if they don't have a value until the app is deployed:
 * >
 * > ```env
 * > MY_FEATURE_FLAG=
 * > ```
 * >
 * > You can override `.env` values from the command line like so:
 * >
 * > ```sh
 * > MY_FEATURE_FLAG="enabled" npm run dev
 * > ```
 * 
 * For example, given the following runtime environment:
 * 
 * ```env
 * ENVIRONMENT=production
 * PUBLIC_BASE_URL=http://site.com
 * ```
 * 
 * With the default `publicPrefix` and `privatePrefix`:
 * 
 * ```ts
 * import { env } from '$env/dynamic/private';
 * 
 * console.log(env.ENVIRONMENT); // => "production"
 * console.log(env.PUBLIC_BASE_URL); // => undefined
 * ```
 */
declare module '$env/dynamic/private' {
	export const env: {
		EZA_LAID_OPTIONS: string;
		OP_PLUGIN_ALIASES_SOURCED: string;
		MANPATH: string;
		STARSHIP_SHELL: string;
		EZA_LD_OPTIONS: string;
		GHOSTTY_RESOURCES_DIR: string;
		NoDefaultCurrentDirectoryInExePath: string;
		CLAUDE_CODE_ENTRYPOINT: string;
		TERM_PROGRAM: string;
		EZA_L_OPTIONS: string;
		NODE: string;
		INIT_CWD: string;
		EZA_LAA_OPTIONS: string;
		SHELL: string;
		TERM: string;
		EZA_LA_OPTIONS: string;
		__FISH_EZA_EXPANDED_OPT_NAME: string;
		EZA_LAAD_OPTIONS: string;
		HOMEBREW_REPOSITORY: string;
		TMPDIR: string;
		GOBIN: string;
		DIRENV_DIR: string;
		EZA_LL_OPTIONS: string;
		TERM_PROGRAM_VERSION: string;
		VAULT_ADDR: string;
		npm_config_npm_globalconfig: string;
		EZA_LID_OPTIONS: string;
		SDKMAN_PLATFORM: string;
		npm_config_registry: string;
		GIT_EDITOR: string;
		EZA_LO_OPTIONS: string;
		USER: string;
		EZA_LT_OPTIONS: string;
		COMMAND_MODE: string;
		SDKMAN_CANDIDATES_API: string;
		SDKMAN_ENV: string;
		npm_config_globalconfig: string;
		PNPM_SCRIPT_SRC_DIR: string;
		ENABLE_TOOL_SEARCH: string;
		KUBECONFIG: string;
		CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS: string;
		SSH_AUTH_SOCK: string;
		__CF_USER_TEXT_ENCODING: string;
		GIT_AUTHOR_NAME: string;
		QUARKUS_HOME: string;
		npm_execpath: string;
		PAGER: string;
		DIRENV_WATCHES: string;
		TMUX: string;
		npm_config_frozen_lockfile: string;
		npm_config_verify_deps_before_run: string;
		EZA_LC_OPTIONS: string;
		PATH: string;
		MICRONAUT_HOME: string;
		SDKMAN_OLD_PWD: string;
		GHOSTTY_SHELL_FEATURES: string;
		LaunchInstanceID: string;
		npm_package_json: string;
		__CFBundleIdentifier: string;
		fish_tmux_term: string;
		PWD: string;
		npm_command: string;
		JAVA_HOME: string;
		EDITOR: string;
		EZA_LE_OPTIONS: string;
		OTEL_EXPORTER_OTLP_METRICS_TEMPORALITY_PREFERENCE: string;
		npm_config__jsr_registry: string;
		npm_lifecycle_event: string;
		LANG: string;
		npm_package_name: string;
		NODE_PATH: string;
		TMUX_PANE: string;
		XPC_FLAGS: string;
		ATUIN_TMUX_POPUP: string;
		EZA_LG_OPTIONS: string;
		__FISH_EZA_EXPANDED: string;
		fish_tmux_autostarted: string;
		npm_config_node_gyp: string;
		DIRENV_FILE: string;
		XPC_SERVICE_NAME: string;
		npm_package_version: string;
		pnpm_config_verify_deps_before_run: string;
		fish_tmux_config: string;
		EZA_STANDARD_OPTIONS: string;
		HOME: string;
		SHLVL: string;
		__FISH_EZA_SORT_OPTIONS: string;
		EZA_LAI_OPTIONS: string;
		TERMINFO: string;
		__FISH_EZA_ALIASES: string;
		EZA_LI_OPTIONS: string;
		HOMEBREW_PREFIX: string;
		EZA_LAD_OPTIONS: string;
		LOGNAME: string;
		STARSHIP_SESSION_KEY: string;
		ZELLIJ_CONFIG_DIR: string;
		ATUIN_SESSION: string;
		SDKMAN_DIR: string;
		VISUAL: string;
		npm_lifecycle_script: string;
		EZA_LAAID_OPTIONS: string;
		EZA_LAAI_OPTIONS: string;
		XDG_DATA_DIRS: string;
		COREPACK_ENABLE_AUTO_PIN: string;
		GHOSTTY_BIN_DIR: string;
		TMUX_PLUGIN_MANAGER_PATH: string;
		BUN_INSTALL: string;
		GITHUB_TOKEN: string;
		GOPATH: string;
		__FISH_EZA_OPT_NAMES: string;
		npm_config_user_agent: string;
		HOMEBREW_CELLAR: string;
		INFOPATH: string;
		SDKMAN_CANDIDATES_DIR: string;
		GIT_AUTHOR_EMAIL: string;
		OSLogRateLimit: string;
		DIRENV_DIFF: string;
		__FISH_EZA_BASE_ALIASES: string;
		ATUIN_SHLVL: string;
		CLAUDECODE: string;
		SECURITYSESSIONID: string;
		SDKMAN_OFFLINE_MODE: string;
		COLORTERM: string;
		GH_TOKEN: string;
		npm_node_execpath: string;
		NODE_ENV: string;
		[key: `PUBLIC_${string}`]: undefined;
		[key: `${string}`]: string | undefined;
	}
}

/**
 * This module provides access to environment variables set _dynamically_ at runtime and that are _publicly_ accessible.
 * 
 * |         | Runtime                                                                    | Build time                                                               |
 * | ------- | -------------------------------------------------------------------------- | ------------------------------------------------------------------------ |
 * | Private | [`$env/dynamic/private`](https://svelte.dev/docs/kit/$env-dynamic-private) | [`$env/static/private`](https://svelte.dev/docs/kit/$env-static-private) |
 * | Public  | [`$env/dynamic/public`](https://svelte.dev/docs/kit/$env-dynamic-public)   | [`$env/static/public`](https://svelte.dev/docs/kit/$env-static-public)   |
 * 
 * Dynamic environment variables are defined by the platform you're running on. For example if you're using [`adapter-node`](https://github.com/sveltejs/kit/tree/main/packages/adapter-node) (or running [`vite preview`](https://svelte.dev/docs/kit/cli)), this is equivalent to `process.env`.
 * 
 * **_Public_ access:**
 * 
 * - This module _can_ be imported into client-side code
 * - **Only** variables that begin with [`config.kit.env.publicPrefix`](https://svelte.dev/docs/kit/configuration#env) (which defaults to `PUBLIC_`) are included
 * 
 * > [!NOTE] In `dev`, `$env/dynamic` includes environment variables from `.env`. In `prod`, this behavior will depend on your adapter.
 * 
 * > [!NOTE] To get correct types, environment variables referenced in your code should be declared (for example in an `.env` file), even if they don't have a value until the app is deployed:
 * >
 * > ```env
 * > MY_FEATURE_FLAG=
 * > ```
 * >
 * > You can override `.env` values from the command line like so:
 * >
 * > ```sh
 * > MY_FEATURE_FLAG="enabled" npm run dev
 * > ```
 * 
 * For example, given the following runtime environment:
 * 
 * ```env
 * ENVIRONMENT=production
 * PUBLIC_BASE_URL=http://example.com
 * ```
 * 
 * With the default `publicPrefix` and `privatePrefix`:
 * 
 * ```ts
 * import { env } from '$env/dynamic/public';
 * console.log(env.ENVIRONMENT); // => undefined, not public
 * console.log(env.PUBLIC_BASE_URL); // => "http://example.com"
 * ```
 * 
 * ```
 * 
 * ```
 */
declare module '$env/dynamic/public' {
	export const env: {
		[key: `PUBLIC_${string}`]: string | undefined;
	}
}
