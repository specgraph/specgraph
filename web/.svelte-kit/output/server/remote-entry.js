import { d as base, l as app_dir, t as prerendering } from "./chunks/environment.js";
import { C as MUTATIVE_METHODS, a as handle_error_and_jsonify, b as set_nested_value, g as deep_set, h as create_field_proxy, v as flatten_issues, y as normalize_issue } from "./chunks/utils.js";
import { a as stringify, o as stringify_remote_arg, r as create_remote_key, v as parse } from "./chunks/shared.js";
import { error, json } from "@sveltejs/kit";
import { HttpError, SvelteKitError, ValidationError } from "@sveltejs/kit/internal";
import { get_request_store, with_request_store } from "@sveltejs/kit/internal/server";
//#region node_modules/.pnpm/@sveltejs+kit@2.55.0_@sveltejs+vite-plugin-svelte@7.0.0_svelte@5.55.0_vite@8.0.2__svelt_c1620dc8e019698275e4944bc5517231/node_modules/@sveltejs/kit/src/runtime/app/server/remote/shared.js
/** @import { RequestEvent } from '@sveltejs/kit' */
/** @import { ServerHooks, MaybePromise, RequestState, RemoteInfo, RequestStore } from 'types' */
/**
* @param {any} validate_or_fn
* @param {(arg?: any) => any} [maybe_fn]
* @returns {(arg?: any) => MaybePromise<any>}
*/
function create_validator(validate_or_fn, maybe_fn) {
	if (!maybe_fn) return (arg) => {
		if (arg !== void 0) error(400, "Bad Request");
	};
	if (validate_or_fn === "unchecked") return (arg) => arg;
	if ("~standard" in validate_or_fn) return async (arg) => {
		const { event, state } = get_request_store();
		const result = await validate_or_fn["~standard"].validate(arg);
		if (result.issues) error(400, await state.handleValidationError({
			issues: result.issues,
			event
		}));
		return result.value;
	};
	throw new Error("Invalid validator passed to remote function. Expected \"unchecked\" or a Standard Schema (https://standardschema.dev)");
}
/**
* In case of a single remote function call, just returns the result.
*
* In case of a full page reload, returns the response for a remote function call,
* either from the cache or by invoking the function.
* Also saves an uneval'ed version of the result for later HTML inlining for hydration.
*
* @template {MaybePromise<any>} T
* @param {RemoteInfo} info
* @param {any} arg
* @param {RequestState} state
* @param {() => Promise<T>} get_result
* @returns {Promise<T>}
*/
async function get_response(info, arg, state, get_result) {
	await 0;
	const cache = get_cache(info, state);
	return cache[stringify_remote_arg(arg, state.transport)] ??= get_result();
}
/**
* @param {any} data
* @param {ServerHooks['transport']} transport
*/
function parse_remote_response(data, transport) {
	/** @type {Record<string, any>} */
	const revivers = {};
	for (const key in transport) revivers[key] = transport[key].decode;
	return parse(data, revivers);
}
/**
* Like `with_event` but removes things from `event` you cannot see/call in remote functions, such as `setHeaders`.
* @template T
* @param {RequestEvent} event
* @param {RequestState} state
* @param {boolean} allow_cookies
* @param {() => any} get_input
* @param {(arg?: any) => T} fn
*/
async function run_remote_function(event, state, allow_cookies, get_input, fn) {
	/** @type {RequestStore} */
	const store = {
		event: {
			...event,
			setHeaders: () => {
				throw new Error("setHeaders is not allowed in remote functions");
			},
			cookies: {
				...event.cookies,
				set: (name, value, opts) => {
					if (!allow_cookies) throw new Error("Cannot set cookies in `query` or `prerender` functions");
					if (opts.path && !opts.path.startsWith("/")) throw new Error("Cookies set in remote functions must have an absolute path");
					return event.cookies.set(name, value, opts);
				},
				delete: (name, opts) => {
					if (!allow_cookies) throw new Error("Cannot delete cookies in `query` or `prerender` functions");
					if (opts.path && !opts.path.startsWith("/")) throw new Error("Cookies deleted in remote functions must have an absolute path");
					return event.cookies.delete(name, opts);
				}
			}
		},
		state: {
			...state,
			is_in_remote_function: true
		}
	};
	const input = await with_request_store(store, get_input);
	return with_request_store(store, () => fn(input));
}
/**
* @param {RemoteInfo} info
* @param {RequestState} state
*/
function get_cache(info, state = get_request_store().state) {
	let cache = state.remote_data?.get(info);
	if (cache === void 0) {
		cache = {};
		(state.remote_data ??= /* @__PURE__ */ new Map()).set(info, cache);
	}
	return cache;
}
//#endregion
//#region node_modules/.pnpm/@sveltejs+kit@2.55.0_@sveltejs+vite-plugin-svelte@7.0.0_svelte@5.55.0_vite@8.0.2__svelt_c1620dc8e019698275e4944bc5517231/node_modules/@sveltejs/kit/src/runtime/app/server/remote/command.js
/** @import { RemoteCommand } from '@sveltejs/kit' */
/** @import { RemoteInfo, MaybePromise } from 'types' */
/** @import { StandardSchemaV1 } from '@standard-schema/spec' */
/**
* Creates a remote command. When called from the browser, the function will be invoked on the server via a `fetch` call.
*
* See [Remote functions](https://svelte.dev/docs/kit/remote-functions#command) for full documentation.
*
* @template Output
* @overload
* @param {() => Output} fn
* @returns {RemoteCommand<void, Output>}
* @since 2.27
*/
/**
* Creates a remote command. When called from the browser, the function will be invoked on the server via a `fetch` call.
*
* See [Remote functions](https://svelte.dev/docs/kit/remote-functions#command) for full documentation.
*
* @template Input
* @template Output
* @overload
* @param {'unchecked'} validate
* @param {(arg: Input) => Output} fn
* @returns {RemoteCommand<Input, Output>}
* @since 2.27
*/
/**
* Creates a remote command. When called from the browser, the function will be invoked on the server via a `fetch` call.
*
* See [Remote functions](https://svelte.dev/docs/kit/remote-functions#command) for full documentation.
*
* @template {StandardSchemaV1} Schema
* @template Output
* @overload
* @param {Schema} validate
* @param {(arg: StandardSchemaV1.InferOutput<Schema>) => Output} fn
* @returns {RemoteCommand<StandardSchemaV1.InferInput<Schema>, Output>}
* @since 2.27
*/
/**
* @template Input
* @template Output
* @param {any} validate_or_fn
* @param {(arg?: Input) => Output} [maybe_fn]
* @returns {RemoteCommand<Input, Output>}
* @since 2.27
*/
/* @__NO_SIDE_EFFECTS__ */
function command(validate_or_fn, maybe_fn) {
	/** @type {(arg?: Input) => Output} */
	const fn = maybe_fn ?? validate_or_fn;
	/** @type {(arg?: any) => MaybePromise<Input>} */
	const validate = create_validator(validate_or_fn, maybe_fn);
	/** @type {RemoteInfo} */
	const __ = {
		type: "command",
		id: "",
		name: ""
	};
	/** @type {RemoteCommand<Input, Output> & { __: RemoteInfo }} */
	const wrapper = (arg) => {
		const { event, state } = get_request_store();
		if (!state.allows_commands) {
			const disallowed_method = !MUTATIVE_METHODS.includes(event.request.method);
			throw new Error(`Cannot call a command (\`${__.name}(${maybe_fn ? "..." : ""})\`) ${disallowed_method ? `from a ${event.request.method} handler or ` : ""}during server-side rendering`);
		}
		state.refreshes ??= {};
		const promise = Promise.resolve(run_remote_function(event, state, true, () => validate(arg), fn));
		promise.updates = () => {
			throw new Error(`Cannot call '${__.name}(...).updates(...)' on the server`);
		};
		return promise;
	};
	Object.defineProperty(wrapper, "__", { value: __ });
	Object.defineProperty(wrapper, "pending", { get: () => 0 });
	return wrapper;
}
//#endregion
//#region node_modules/.pnpm/@sveltejs+kit@2.55.0_@sveltejs+vite-plugin-svelte@7.0.0_svelte@5.55.0_vite@8.0.2__svelt_c1620dc8e019698275e4944bc5517231/node_modules/@sveltejs/kit/src/runtime/app/server/remote/form.js
/** @import { RemoteFormInput, RemoteForm, InvalidField } from '@sveltejs/kit' */
/** @import { InternalRemoteFormIssue, MaybePromise, RemoteInfo } from 'types' */
/** @import { StandardSchemaV1 } from '@standard-schema/spec' */
/**
* Creates a form object that can be spread onto a `<form>` element.
*
* See [Remote functions](https://svelte.dev/docs/kit/remote-functions#form) for full documentation.
*
* @template Output
* @overload
* @param {() => MaybePromise<Output>} fn
* @returns {RemoteForm<void, Output>}
* @since 2.27
*/
/**
* Creates a form object that can be spread onto a `<form>` element.
*
* See [Remote functions](https://svelte.dev/docs/kit/remote-functions#form) for full documentation.
*
* @template {RemoteFormInput} Input
* @template Output
* @overload
* @param {'unchecked'} validate
* @param {(data: Input, issue: InvalidField<Input>) => MaybePromise<Output>} fn
* @returns {RemoteForm<Input, Output>}
* @since 2.27
*/
/**
* Creates a form object that can be spread onto a `<form>` element.
*
* See [Remote functions](https://svelte.dev/docs/kit/remote-functions#form) for full documentation.
*
* @template {StandardSchemaV1<RemoteFormInput, Record<string, any>>} Schema
* @template Output
* @overload
* @param {Schema} validate
* @param {(data: StandardSchemaV1.InferOutput<Schema>, issue: InvalidField<StandardSchemaV1.InferInput<Schema>>) => MaybePromise<Output>} fn
* @returns {RemoteForm<StandardSchemaV1.InferInput<Schema>, Output>}
* @since 2.27
*/
/**
* @template {RemoteFormInput} Input
* @template Output
* @param {any} validate_or_fn
* @param {(data_or_issue: any, issue?: any) => MaybePromise<Output>} [maybe_fn]
* @returns {RemoteForm<Input, Output>}
* @since 2.27
*/
/* @__NO_SIDE_EFFECTS__ */
function form(validate_or_fn, maybe_fn) {
	/** @type {any} */
	const fn = maybe_fn ?? validate_or_fn;
	/** @type {StandardSchemaV1 | null} */
	const schema = !maybe_fn || validate_or_fn === "unchecked" ? null : validate_or_fn;
	/**
	* @param {string | number | boolean} [key]
	*/
	function create_instance(key) {
		/** @type {RemoteForm<Input, Output>} */
		const instance = {};
		instance.method = "POST";
		Object.defineProperty(instance, "enhance", { value: () => {
			return {
				action: instance.action,
				method: instance.method
			};
		} });
		/** @type {RemoteInfo} */
		const __ = {
			type: "form",
			name: "",
			id: "",
			fn: async (data, meta, form_data) => {
				/** @type {{ submission: true, input?: Record<string, any>, issues?: InternalRemoteFormIssue[], result: Output }} */
				const output = {};
				output.submission = true;
				const { event, state } = get_request_store();
				const validated = await schema?.["~standard"].validate(data);
				if (meta.validate_only) return validated?.issues?.map((issue) => normalize_issue(issue, true)) ?? [];
				if (validated?.issues !== void 0) handle_issues(output, validated.issues, form_data);
				else {
					if (validated !== void 0) data = validated.value;
					state.refreshes ??= {};
					const issue = create_issues();
					try {
						output.result = await run_remote_function(event, state, true, () => data, (data) => !maybe_fn ? fn() : fn(data, issue));
					} catch (e) {
						if (e instanceof ValidationError) handle_issues(output, e.issues, form_data);
						else throw e;
					}
				}
				if (!event.isRemoteRequest) get_cache(__, state)[""] ??= output;
				return output;
			}
		};
		Object.defineProperty(instance, "__", { value: __ });
		Object.defineProperty(instance, "action", {
			get: () => `?/remote=${__.id}`,
			enumerable: true
		});
		Object.defineProperty(instance, "fields", { get() {
			return create_field_proxy({}, () => get_cache(__)?.[""]?.input ?? {}, (path, value) => {
				const cache = get_cache(__);
				const data = cache[""];
				if (data?.submission) return;
				if (path.length === 0) {
					(cache[""] ??= {}).input = value;
					return;
				}
				const input = data?.input ?? {};
				deep_set(input, path.map(String), value);
				(cache[""] ??= {}).input = input;
			}, () => flatten_issues(get_cache(__)?.[""]?.issues ?? []));
		} });
		Object.defineProperty(instance, "result", { get() {
			try {
				return get_cache(__)?.[""]?.result;
			} catch {
				return;
			}
		} });
		Object.defineProperty(instance, "pending", { get: () => 0 });
		Object.defineProperty(instance, "preflight", { value: () => instance });
		Object.defineProperty(instance, "validate", { value: () => {
			throw new Error("Cannot call validate() on the server");
		} });
		if (key == void 0) Object.defineProperty(instance, "for", { value: (key) => {
			const { state } = get_request_store();
			const cache_key = __.id + "|" + JSON.stringify(key);
			let instance = (state.form_instances ??= /* @__PURE__ */ new Map()).get(cache_key);
			if (!instance) {
				instance = create_instance(key);
				instance.__.id = `${__.id}/${encodeURIComponent(JSON.stringify(key))}`;
				instance.__.name = __.name;
				state.form_instances.set(cache_key, instance);
			}
			return instance;
		} });
		return instance;
	}
	return create_instance();
}
/**
* @param {{ issues?: InternalRemoteFormIssue[], input?: Record<string, any>, result: any }} output
* @param {readonly StandardSchemaV1.Issue[]} issues
* @param {FormData | null} form_data - null if the form is progressively enhanced
*/
function handle_issues(output, issues, form_data) {
	output.issues = issues.map((issue) => normalize_issue(issue, true));
	if (form_data) {
		output.input = {};
		for (let key of form_data.keys()) {
			if (/^[.\]]?_/.test(key)) continue;
			const is_array = key.endsWith("[]");
			const values = form_data.getAll(key).filter((value) => typeof value === "string");
			if (is_array) key = key.slice(0, -2);
			set_nested_value(output.input, key, is_array ? values : values[0]);
		}
	}
}
/**
* Creates an invalid function that can be used to imperatively mark form fields as invalid
* @returns {InvalidField<any>}
*/
function create_issues() {
	return new Proxy(
		/** @param {string} message */
		(message) => {
			if (typeof message !== "string") throw new Error("`invalid` should now be imported from `@sveltejs/kit` to throw validation issues. The second parameter provided to the form function (renamed to `issue`) is still used to construct issues, e.g. `invalid(issue.field('message'))`. For more info see https://github.com/sveltejs/kit/pulls/14768");
			return create_issue(message);
		},
		{ get(target, prop) {
			if (typeof prop === "symbol") return target[prop];
			return create_issue_proxy(prop, []);
		} }
	);
	/**
	* @param {string} message
	* @param {(string | number)[]} path
	* @returns {StandardSchemaV1.Issue}
	*/
	function create_issue(message, path = []) {
		return {
			message,
			path
		};
	}
	/**
	* Creates a proxy that builds up a path and returns a function to create an issue
	* @param {string | number} key
	* @param {(string | number)[]} path
	*/
	function create_issue_proxy(key, path) {
		const new_path = [...path, key];
		/**
		* @param {string} message
		* @returns {StandardSchemaV1.Issue}
		*/
		const issue_func = (message) => create_issue(message, new_path);
		return new Proxy(issue_func, { get(target, prop) {
			if (typeof prop === "symbol") return target[prop];
			if (/^\d+$/.test(prop)) return create_issue_proxy(parseInt(prop, 10), new_path);
			return create_issue_proxy(prop, new_path);
		} });
	}
}
//#endregion
//#region node_modules/.pnpm/@sveltejs+kit@2.55.0_@sveltejs+vite-plugin-svelte@7.0.0_svelte@5.55.0_vite@8.0.2__svelt_c1620dc8e019698275e4944bc5517231/node_modules/@sveltejs/kit/src/runtime/app/server/remote/prerender.js
/** @import { RemoteResource, RemotePrerenderFunction } from '@sveltejs/kit' */
/** @import { RemotePrerenderInputsGenerator, RemoteInfo, MaybePromise } from 'types' */
/** @import { StandardSchemaV1 } from '@standard-schema/spec' */
/**
* Creates a remote prerender function. When called from the browser, the function will be invoked on the server via a `fetch` call.
*
* See [Remote functions](https://svelte.dev/docs/kit/remote-functions#prerender) for full documentation.
*
* @template Output
* @overload
* @param {() => MaybePromise<Output>} fn
* @param {{ inputs?: RemotePrerenderInputsGenerator<void>, dynamic?: boolean }} [options]
* @returns {RemotePrerenderFunction<void, Output>}
* @since 2.27
*/
/**
* Creates a remote prerender function. When called from the browser, the function will be invoked on the server via a `fetch` call.
*
* See [Remote functions](https://svelte.dev/docs/kit/remote-functions#prerender) for full documentation.
*
* @template Input
* @template Output
* @overload
* @param {'unchecked'} validate
* @param {(arg: Input) => MaybePromise<Output>} fn
* @param {{ inputs?: RemotePrerenderInputsGenerator<Input>, dynamic?: boolean }} [options]
* @returns {RemotePrerenderFunction<Input, Output>}
* @since 2.27
*/
/**
* Creates a remote prerender function. When called from the browser, the function will be invoked on the server via a `fetch` call.
*
* See [Remote functions](https://svelte.dev/docs/kit/remote-functions#prerender) for full documentation.
*
* @template {StandardSchemaV1} Schema
* @template Output
* @overload
* @param {Schema} schema
* @param {(arg: StandardSchemaV1.InferOutput<Schema>) => MaybePromise<Output>} fn
* @param {{ inputs?: RemotePrerenderInputsGenerator<StandardSchemaV1.InferInput<Schema>>, dynamic?: boolean }} [options]
* @returns {RemotePrerenderFunction<StandardSchemaV1.InferInput<Schema>, Output>}
* @since 2.27
*/
/**
* @template Input
* @template Output
* @param {any} validate_or_fn
* @param {any} [fn_or_options]
* @param {{ inputs?: RemotePrerenderInputsGenerator<Input>, dynamic?: boolean }} [maybe_options]
* @returns {RemotePrerenderFunction<Input, Output>}
* @since 2.27
*/
/* @__NO_SIDE_EFFECTS__ */
function prerender(validate_or_fn, fn_or_options, maybe_options) {
	const maybe_fn = typeof fn_or_options === "function" ? fn_or_options : void 0;
	/** @type {typeof maybe_options} */
	const options = maybe_options ?? (maybe_fn ? void 0 : fn_or_options);
	/** @type {(arg?: Input) => MaybePromise<Output>} */
	const fn = maybe_fn ?? validate_or_fn;
	/** @type {(arg?: any) => MaybePromise<Input>} */
	const validate = create_validator(validate_or_fn, maybe_fn);
	/** @type {RemoteInfo} */
	const __ = {
		type: "prerender",
		id: "",
		name: "",
		has_arg: !!maybe_fn,
		inputs: options?.inputs,
		dynamic: options?.dynamic
	};
	/** @type {RemotePrerenderFunction<Input, Output> & { __: RemoteInfo }} */
	const wrapper = (arg) => {
		/** @type {Promise<Output> & Partial<RemoteResource<Output>>} */
		const promise = (async () => {
			const { event, state } = get_request_store();
			const payload = stringify_remote_arg(arg, state.transport);
			const url = `${base}/${app_dir}/remote/${__.id}${payload ? `/${payload}` : ""}`;
			if (!state.prerendering && !event.isRemoteRequest) try {
				return await get_response(__, arg, state, async () => {
					const key = stringify_remote_arg(arg, state.transport);
					const cache = get_cache(__, state);
					return parse_remote_response(await (cache[key] ??= fetch(new URL(url, event.url.origin).href).then(async (response) => {
						if (!response.ok) throw new Error("Prerendered response not found");
						const prerendered = await response.json();
						if (prerendered.type === "error") error(prerendered.status, prerendered.error);
						return prerendered.result;
					})), state.transport);
				});
			} catch {}
			if (state.prerendering?.remote_responses.has(url)) return state.prerendering.remote_responses.get(url);
			const promise = get_response(__, arg, state, () => run_remote_function(event, state, false, () => validate(arg), fn));
			if (state.prerendering) state.prerendering.remote_responses.set(url, promise);
			const result = await promise;
			if (state.prerendering) {
				const body = {
					type: "result",
					result: stringify(result, state.transport)
				};
				state.prerendering.dependencies.set(url, {
					body: JSON.stringify(body),
					response: json(body)
				});
			}
			return result;
		})();
		promise.catch(() => {});
		return promise;
	};
	Object.defineProperty(wrapper, "__", { value: __ });
	return wrapper;
}
//#endregion
//#region node_modules/.pnpm/@sveltejs+kit@2.55.0_@sveltejs+vite-plugin-svelte@7.0.0_svelte@5.55.0_vite@8.0.2__svelt_c1620dc8e019698275e4944bc5517231/node_modules/@sveltejs/kit/src/runtime/app/server/remote/query.js
/** @import { RemoteQuery, RemoteQueryFunction } from '@sveltejs/kit' */
/** @import { RemoteInfo, MaybePromise } from 'types' */
/** @import { StandardSchemaV1 } from '@standard-schema/spec' */
/**
* Creates a remote query. When called from the browser, the function will be invoked on the server via a `fetch` call.
*
* See [Remote functions](https://svelte.dev/docs/kit/remote-functions#query) for full documentation.
*
* @template Output
* @overload
* @param {() => MaybePromise<Output>} fn
* @returns {RemoteQueryFunction<void, Output>}
* @since 2.27
*/
/**
* Creates a remote query. When called from the browser, the function will be invoked on the server via a `fetch` call.
*
* See [Remote functions](https://svelte.dev/docs/kit/remote-functions#query) for full documentation.
*
* @template Input
* @template Output
* @overload
* @param {'unchecked'} validate
* @param {(arg: Input) => MaybePromise<Output>} fn
* @returns {RemoteQueryFunction<Input, Output>}
* @since 2.27
*/
/**
* Creates a remote query. When called from the browser, the function will be invoked on the server via a `fetch` call.
*
* See [Remote functions](https://svelte.dev/docs/kit/remote-functions#query) for full documentation.
*
* @template {StandardSchemaV1} Schema
* @template Output
* @overload
* @param {Schema} schema
* @param {(arg: StandardSchemaV1.InferOutput<Schema>) => MaybePromise<Output>} fn
* @returns {RemoteQueryFunction<StandardSchemaV1.InferInput<Schema>, Output>}
* @since 2.27
*/
/**
* @template Input
* @template Output
* @param {any} validate_or_fn
* @param {(args?: Input) => MaybePromise<Output>} [maybe_fn]
* @returns {RemoteQueryFunction<Input, Output>}
* @since 2.27
*/
/* @__NO_SIDE_EFFECTS__ */
function query(validate_or_fn, maybe_fn) {
	/** @type {(arg?: Input) => Output} */
	const fn = maybe_fn ?? validate_or_fn;
	/** @type {(arg?: any) => MaybePromise<Input>} */
	const validate = create_validator(validate_or_fn, maybe_fn);
	/** @type {RemoteInfo} */
	const __ = {
		type: "query",
		id: "",
		name: ""
	};
	/** @type {RemoteQueryFunction<Input, Output> & { __: RemoteInfo }} */
	const wrapper = (arg) => {
		if (prerendering) throw new Error(`Cannot call query '${__.name}' while prerendering, as prerendered pages need static data. Use 'prerender' from $app/server instead`);
		const { event, state } = get_request_store();
		const get_remote_function_result = () => run_remote_function(event, state, false, () => validate(arg), fn);
		/** @type {Promise<any> & Partial<RemoteQuery<any>>} */
		const promise = get_response(__, arg, state, get_remote_function_result);
		promise.catch(() => {});
		promise.set = (value) => update_refresh_value(get_refresh_context(__, "set", arg), value);
		promise.refresh = () => {
			const refresh_context = get_refresh_context(__, "refresh", arg);
			const is_immediate_refresh = !refresh_context.cache[refresh_context.cache_key];
			return update_refresh_value(refresh_context, is_immediate_refresh ? promise : get_remote_function_result(), is_immediate_refresh);
		};
		promise.withOverride = () => {
			throw new Error(`Cannot call '${__.name}.withOverride()' on the server`);
		};
		return promise;
	};
	Object.defineProperty(wrapper, "__", { value: __ });
	return wrapper;
}
/**
* Creates a batch query function that collects multiple calls and executes them in a single request
*
* See [Remote functions](https://svelte.dev/docs/kit/remote-functions#query.batch) for full documentation.
*
* @template Input
* @template Output
* @overload
* @param {'unchecked'} validate
* @param {(args: Input[]) => MaybePromise<(arg: Input, idx: number) => Output>} fn
* @returns {RemoteQueryFunction<Input, Output>}
* @since 2.35
*/
/**
* Creates a batch query function that collects multiple calls and executes them in a single request
*
* See [Remote functions](https://svelte.dev/docs/kit/remote-functions#query.batch) for full documentation.
*
* @template {StandardSchemaV1} Schema
* @template Output
* @overload
* @param {Schema} schema
* @param {(args: StandardSchemaV1.InferOutput<Schema>[]) => MaybePromise<(arg: StandardSchemaV1.InferOutput<Schema>, idx: number) => Output>} fn
* @returns {RemoteQueryFunction<StandardSchemaV1.InferInput<Schema>, Output>}
* @since 2.35
*/
/**
* @template Input
* @template Output
* @param {any} validate_or_fn
* @param {(args?: Input[]) => MaybePromise<(arg: Input, idx: number) => Output>} [maybe_fn]
* @returns {RemoteQueryFunction<Input, Output>}
* @since 2.35
*/
/* @__NO_SIDE_EFFECTS__ */
function batch(validate_or_fn, maybe_fn) {
	/** @type {(args?: Input[]) => MaybePromise<(arg: Input, idx: number) => Output>} */
	const fn = maybe_fn ?? validate_or_fn;
	/** @type {(arg?: any) => MaybePromise<Input>} */
	const validate = create_validator(validate_or_fn, maybe_fn);
	/** @type {RemoteInfo & { type: 'query_batch' }} */
	const __ = {
		type: "query_batch",
		id: "",
		name: "",
		run: async (args, options) => {
			const { event, state } = get_request_store();
			return run_remote_function(event, state, false, async () => Promise.all(args.map(validate)), async (input) => {
				const get_result = await fn(input);
				return Promise.all(input.map(async (arg, i) => {
					try {
						return {
							type: "result",
							data: get_result(arg, i)
						};
					} catch (error) {
						return {
							type: "error",
							error: await handle_error_and_jsonify(event, state, options, error),
							status: error instanceof HttpError || error instanceof SvelteKitError ? error.status : 500
						};
					}
				}));
			});
		}
	};
	/** @type {{ args: any[], resolvers: Array<{resolve: (value: any) => void, reject: (error: any) => void}> }} */
	let batching = {
		args: [],
		resolvers: []
	};
	/** @type {RemoteQueryFunction<Input, Output> & { __: RemoteInfo }} */
	const wrapper = (arg) => {
		if (prerendering) throw new Error(`Cannot call query.batch '${__.name}' while prerendering, as prerendered pages need static data. Use 'prerender' from $app/server instead`);
		const { event, state } = get_request_store();
		const get_remote_function_result = () => {
			return new Promise((resolve, reject) => {
				batching.args.push(arg);
				batching.resolvers.push({
					resolve,
					reject
				});
				if (batching.args.length > 1) return;
				setTimeout(async () => {
					const batched = batching;
					batching = {
						args: [],
						resolvers: []
					};
					try {
						return await run_remote_function(event, state, false, async () => Promise.all(batched.args.map(validate)), async (input) => {
							const get_result = await fn(input);
							for (let i = 0; i < batched.resolvers.length; i++) try {
								batched.resolvers[i].resolve(get_result(input[i], i));
							} catch (error) {
								batched.resolvers[i].reject(error);
							}
						});
					} catch (error) {
						for (const resolver of batched.resolvers) resolver.reject(error);
					}
				}, 0);
			});
		};
		/** @type {Promise<any> & Partial<RemoteQuery<any>>} */
		const promise = get_response(__, arg, state, get_remote_function_result);
		promise.catch(() => {});
		promise.set = (value) => update_refresh_value(get_refresh_context(__, "set", arg), value);
		promise.refresh = () => {
			const refresh_context = get_refresh_context(__, "refresh", arg);
			const is_immediate_refresh = !refresh_context.cache[refresh_context.cache_key];
			return update_refresh_value(refresh_context, is_immediate_refresh ? promise : get_remote_function_result(), is_immediate_refresh);
		};
		promise.withOverride = () => {
			throw new Error(`Cannot call '${__.name}.withOverride()' on the server`);
		};
		return promise;
	};
	Object.defineProperty(wrapper, "__", { value: __ });
	return wrapper;
}
Object.defineProperty(query, "batch", {
	value: batch,
	enumerable: true
});
/**
* @param {RemoteInfo} __
* @param {'set' | 'refresh'} action
* @param {any} [arg]
* @returns {{ __: RemoteInfo; state: any; refreshes: Record<string, Promise<any>>; cache: Record<string, Promise<any>>; refreshes_key: string; cache_key: string }}
*/
function get_refresh_context(__, action, arg) {
	const { state } = get_request_store();
	const { refreshes } = state;
	if (!refreshes) {
		const name = __.type === "query_batch" ? `query.batch '${__.name}'` : `query '${__.name}'`;
		throw new Error(`Cannot call ${action} on ${name} because it is not executed in the context of a command/form remote function`);
	}
	const cache = get_cache(__, state);
	const cache_key = stringify_remote_arg(arg, state.transport);
	return {
		__,
		state,
		refreshes,
		refreshes_key: create_remote_key(__.id, cache_key),
		cache,
		cache_key
	};
}
/**
* @param {{ __: RemoteInfo; refreshes: Record<string, Promise<any>>; cache: Record<string, Promise<any>>; refreshes_key: string; cache_key: string }} context
* @param {any} value
* @param {boolean} [is_immediate_refresh=false]
* @returns {Promise<void>}
*/
function update_refresh_value({ __, refreshes, refreshes_key, cache, cache_key }, value, is_immediate_refresh = false) {
	const promise = Promise.resolve(value);
	if (!is_immediate_refresh) cache[cache_key] = promise;
	if (__.id) refreshes[refreshes_key] = promise;
	return promise.then(() => {});
}
//#endregion
export { command, form, prerender, query };
