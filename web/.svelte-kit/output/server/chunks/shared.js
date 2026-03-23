import { a as is_plain_object, c as stringify_string, i as get_type, l as valid_array_indices, n as enumerable_symbols, o as is_primitive, s as stringify_key, t as DevalueError } from "./utils2.js";
import { HttpError, SvelteKitError } from "@sveltejs/kit/internal";
//#region node_modules/.pnpm/devalue@5.6.4/node_modules/devalue/src/base64.js
/**
* Base64 Encodes an arraybuffer
* @param {ArrayBuffer} arraybuffer
* @returns {string}
*/
function encode64(arraybuffer) {
	const dv = new DataView(arraybuffer);
	let binaryString = "";
	for (let i = 0; i < arraybuffer.byteLength; i++) binaryString += String.fromCharCode(dv.getUint8(i));
	return binaryToAscii(binaryString);
}
/**
* Decodes a base64 string into an arraybuffer
* @param {string} string
* @returns {ArrayBuffer}
*/
function decode64(string) {
	const binaryString = asciiToBinary(string);
	const arraybuffer = new ArrayBuffer(binaryString.length);
	const dv = new DataView(arraybuffer);
	for (let i = 0; i < arraybuffer.byteLength; i++) dv.setUint8(i, binaryString.charCodeAt(i));
	return arraybuffer;
}
var KEY_STRING = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
/**
* Substitute for atob since it's deprecated in node.
* Does not do any input validation.
*
* @see https://github.com/jsdom/abab/blob/master/lib/atob.js
*
* @param {string} data
* @returns {string}
*/
function asciiToBinary(data) {
	if (data.length % 4 === 0) data = data.replace(/==?$/, "");
	let output = "";
	let buffer = 0;
	let accumulatedBits = 0;
	for (let i = 0; i < data.length; i++) {
		buffer <<= 6;
		buffer |= KEY_STRING.indexOf(data[i]);
		accumulatedBits += 6;
		if (accumulatedBits === 24) {
			output += String.fromCharCode((buffer & 16711680) >> 16);
			output += String.fromCharCode((buffer & 65280) >> 8);
			output += String.fromCharCode(buffer & 255);
			buffer = accumulatedBits = 0;
		}
	}
	if (accumulatedBits === 12) {
		buffer >>= 4;
		output += String.fromCharCode(buffer);
	} else if (accumulatedBits === 18) {
		buffer >>= 2;
		output += String.fromCharCode((buffer & 65280) >> 8);
		output += String.fromCharCode(buffer & 255);
	}
	return output;
}
/**
* Substitute for btoa since it's deprecated in node.
* Does not do any input validation.
*
* @see https://github.com/jsdom/abab/blob/master/lib/btoa.js
*
* @param {string} str
* @returns {string}
*/
function binaryToAscii(str) {
	let out = "";
	for (let i = 0; i < str.length; i += 3) {
		/** @type {[number, number, number, number]} */
		const groupsOfSix = [
			void 0,
			void 0,
			void 0,
			void 0
		];
		groupsOfSix[0] = str.charCodeAt(i) >> 2;
		groupsOfSix[1] = (str.charCodeAt(i) & 3) << 4;
		if (str.length > i + 1) {
			groupsOfSix[1] |= str.charCodeAt(i + 1) >> 4;
			groupsOfSix[2] = (str.charCodeAt(i + 1) & 15) << 2;
		}
		if (str.length > i + 2) {
			groupsOfSix[2] |= str.charCodeAt(i + 2) >> 6;
			groupsOfSix[3] = str.charCodeAt(i + 2) & 63;
		}
		for (let j = 0; j < groupsOfSix.length; j++) if (typeof groupsOfSix[j] === "undefined") out += "=";
		else out += KEY_STRING[groupsOfSix[j]];
	}
	return out;
}
//#endregion
//#region node_modules/.pnpm/devalue@5.6.4/node_modules/devalue/src/parse.js
/**
* Revive a value serialized with `devalue.stringify`
* @param {string} serialized
* @param {Record<string, (value: any) => any>} [revivers]
*/
function parse(serialized, revivers) {
	return unflatten(JSON.parse(serialized), revivers);
}
/**
* Revive a value flattened with `devalue.stringify`
* @param {number | any[]} parsed
* @param {Record<string, (value: any) => any>} [revivers]
*/
function unflatten(parsed, revivers) {
	if (typeof parsed === "number") return hydrate(parsed, true);
	if (!Array.isArray(parsed) || parsed.length === 0) throw new Error("Invalid input");
	const values = parsed;
	const hydrated = Array(values.length);
	/**
	* A set of values currently being hydrated with custom revivers,
	* used to detect invalid cyclical dependencies
	* @type {Set<number> | null}
	*/
	let hydrating = null;
	/**
	* @param {number} index
	* @returns {any}
	*/
	function hydrate(index, standalone = false) {
		if (index === -1) return void 0;
		if (index === -3) return NaN;
		if (index === -4) return Infinity;
		if (index === -5) return -Infinity;
		if (index === -6) return -0;
		if (standalone || typeof index !== "number") throw new Error(`Invalid input`);
		if (index in hydrated) return hydrated[index];
		const value = values[index];
		if (!value || typeof value !== "object") hydrated[index] = value;
		else if (Array.isArray(value)) if (typeof value[0] === "string") {
			const type = value[0];
			const reviver = revivers && Object.hasOwn(revivers, type) ? revivers[type] : void 0;
			if (reviver) {
				let i = value[1];
				if (typeof i !== "number") i = values.push(value[1]) - 1;
				hydrating ??= /* @__PURE__ */ new Set();
				if (hydrating.has(i)) throw new Error("Invalid circular reference");
				hydrating.add(i);
				hydrated[index] = reviver(hydrate(i));
				hydrating.delete(i);
				return hydrated[index];
			}
			switch (type) {
				case "Date":
					hydrated[index] = new Date(value[1]);
					break;
				case "Set":
					const set = /* @__PURE__ */ new Set();
					hydrated[index] = set;
					for (let i = 1; i < value.length; i += 1) set.add(hydrate(value[i]));
					break;
				case "Map":
					const map = /* @__PURE__ */ new Map();
					hydrated[index] = map;
					for (let i = 1; i < value.length; i += 2) map.set(hydrate(value[i]), hydrate(value[i + 1]));
					break;
				case "RegExp":
					hydrated[index] = new RegExp(value[1], value[2]);
					break;
				case "Object":
					const object = Object(value[1]);
					if (Object.hasOwn(object, "__proto__")) throw new Error("Cannot parse an object with a `__proto__` property");
					hydrated[index] = object;
					break;
				case "BigInt":
					hydrated[index] = BigInt(value[1]);
					break;
				case "null":
					const obj = Object.create(null);
					hydrated[index] = obj;
					for (let i = 1; i < value.length; i += 2) {
						if (value[i] === "__proto__") throw new Error("Cannot parse an object with a `__proto__` property");
						obj[value[i]] = hydrate(value[i + 1]);
					}
					break;
				case "Int8Array":
				case "Uint8Array":
				case "Uint8ClampedArray":
				case "Int16Array":
				case "Uint16Array":
				case "Int32Array":
				case "Uint32Array":
				case "Float32Array":
				case "Float64Array":
				case "BigInt64Array":
				case "BigUint64Array": {
					if (values[value[1]][0] !== "ArrayBuffer") throw new Error("Invalid data");
					const TypedArrayConstructor = globalThis[type];
					const typedArray = new TypedArrayConstructor(hydrate(value[1]));
					hydrated[index] = value[2] !== void 0 ? typedArray.subarray(value[2], value[3]) : typedArray;
					break;
				}
				case "ArrayBuffer": {
					const base64 = value[1];
					if (typeof base64 !== "string") throw new Error("Invalid ArrayBuffer encoding");
					hydrated[index] = decode64(base64);
					break;
				}
				case "Temporal.Duration":
				case "Temporal.Instant":
				case "Temporal.PlainDate":
				case "Temporal.PlainTime":
				case "Temporal.PlainDateTime":
				case "Temporal.PlainMonthDay":
				case "Temporal.PlainYearMonth":
				case "Temporal.ZonedDateTime": {
					const temporalName = type.slice(9);
					hydrated[index] = Temporal[temporalName].from(value[1]);
					break;
				}
				case "URL":
					hydrated[index] = new URL(value[1]);
					break;
				case "URLSearchParams":
					hydrated[index] = new URLSearchParams(value[1]);
					break;
				default: throw new Error(`Unknown type ${type}`);
			}
		} else if (value[0] === -7) {
			const len = value[1];
			if (!Number.isInteger(len) || len < 0) throw new Error("Invalid input");
			const array = new Array(len);
			hydrated[index] = array;
			for (let i = 2; i < value.length; i += 2) {
				const idx = value[i];
				if (!Number.isInteger(idx) || idx < 0 || idx >= len) throw new Error("Invalid input");
				array[idx] = hydrate(value[i + 1]);
			}
		} else {
			const array = new Array(value.length);
			hydrated[index] = array;
			for (let i = 0; i < value.length; i += 1) {
				const n = value[i];
				if (n === -2) continue;
				array[i] = hydrate(n);
			}
		}
		else {
			/** @type {Record<string, any>} */
			const object = {};
			hydrated[index] = object;
			for (const key of Object.keys(value)) {
				if (key === "__proto__") throw new Error("Cannot parse an object with a `__proto__` property");
				const n = value[key];
				object[key] = hydrate(n);
			}
		}
		return hydrated[index];
	}
	return hydrate(0);
}
//#endregion
//#region node_modules/.pnpm/devalue@5.6.4/node_modules/devalue/src/stringify.js
/**
* Turn a value into a JSON string that can be parsed with `devalue.parse`
* @param {any} value
* @param {Record<string, (value: any) => any>} [reducers]
*/
function stringify$1(value, reducers) {
	/** @type {any[]} */
	const stringified = [];
	/** @type {Map<any, number>} */
	const indexes = /* @__PURE__ */ new Map();
	/** @type {Array<{ key: string, fn: (value: any) => any }>} */
	const custom = [];
	if (reducers) for (const key of Object.getOwnPropertyNames(reducers)) custom.push({
		key,
		fn: reducers[key]
	});
	/** @type {string[]} */
	const keys = [];
	let p = 0;
	/** @param {any} thing */
	function flatten(thing) {
		if (thing === void 0) return -1;
		if (Number.isNaN(thing)) return -3;
		if (thing === Infinity) return -4;
		if (thing === -Infinity) return -5;
		if (thing === 0 && 1 / thing < 0) return -6;
		if (indexes.has(thing)) return indexes.get(thing);
		const index = p++;
		indexes.set(thing, index);
		for (const { key, fn } of custom) {
			const value = fn(thing);
			if (value) {
				stringified[index] = `["${key}",${flatten(value)}]`;
				return index;
			}
		}
		if (typeof thing === "function") throw new DevalueError(`Cannot stringify a function`, keys, thing, value);
		let str = "";
		if (is_primitive(thing)) str = stringify_primitive(thing);
		else {
			const type = get_type(thing);
			switch (type) {
				case "Number":
				case "String":
				case "Boolean":
					str = `["Object",${stringify_primitive(thing)}]`;
					break;
				case "BigInt":
					str = `["BigInt",${thing}]`;
					break;
				case "Date":
					str = `["Date","${!isNaN(thing.getDate()) ? thing.toISOString() : ""}"]`;
					break;
				case "URL":
					str = `["URL",${stringify_string(thing.toString())}]`;
					break;
				case "URLSearchParams":
					str = `["URLSearchParams",${stringify_string(thing.toString())}]`;
					break;
				case "RegExp":
					const { source, flags } = thing;
					str = flags ? `["RegExp",${stringify_string(source)},"${flags}"]` : `["RegExp",${stringify_string(source)}]`;
					break;
				case "Array": {
					let mostly_dense = false;
					str = "[";
					for (let i = 0; i < thing.length; i += 1) {
						if (i > 0) str += ",";
						if (Object.hasOwn(thing, i)) {
							keys.push(`[${i}]`);
							str += flatten(thing[i]);
							keys.pop();
						} else if (mostly_dense) str += -2;
						else {
							const populated_keys = valid_array_indices(thing);
							const population = populated_keys.length;
							const d = String(thing.length).length;
							if ((thing.length - population) * 3 > 4 + d + population * (d + 1)) {
								str = "[-7," + thing.length;
								for (let j = 0; j < populated_keys.length; j++) {
									const key = populated_keys[j];
									keys.push(`[${key}]`);
									str += "," + key + "," + flatten(thing[key]);
									keys.pop();
								}
								break;
							} else {
								mostly_dense = true;
								str += -2;
							}
						}
					}
					str += "]";
					break;
				}
				case "Set":
					str = "[\"Set\"";
					for (const value of thing) str += `,${flatten(value)}`;
					str += "]";
					break;
				case "Map":
					str = "[\"Map\"";
					for (const [key, value] of thing) {
						keys.push(`.get(${is_primitive(key) ? stringify_primitive(key) : "..."})`);
						str += `,${flatten(key)},${flatten(value)}`;
						keys.pop();
					}
					str += "]";
					break;
				case "Int8Array":
				case "Uint8Array":
				case "Uint8ClampedArray":
				case "Int16Array":
				case "Uint16Array":
				case "Int32Array":
				case "Uint32Array":
				case "Float32Array":
				case "Float64Array":
				case "BigInt64Array":
				case "BigUint64Array": {
					/** @type {import("./types.js").TypedArray} */
					const typedArray = thing;
					str = "[\"" + type + "\"," + flatten(typedArray.buffer);
					const a = thing.byteOffset;
					const b = a + thing.byteLength;
					if (a > 0 || b !== typedArray.buffer.byteLength) {
						const m = +/(\d+)/.exec(type)[1] / 8;
						str += `,${a / m},${b / m}`;
					}
					str += "]";
					break;
				}
				case "ArrayBuffer":
					str = `["ArrayBuffer","${encode64(thing)}"]`;
					break;
				case "Temporal.Duration":
				case "Temporal.Instant":
				case "Temporal.PlainDate":
				case "Temporal.PlainTime":
				case "Temporal.PlainDateTime":
				case "Temporal.PlainMonthDay":
				case "Temporal.PlainYearMonth":
				case "Temporal.ZonedDateTime":
					str = `["${type}",${stringify_string(thing.toString())}]`;
					break;
				default:
					if (!is_plain_object(thing)) throw new DevalueError(`Cannot stringify arbitrary non-POJOs`, keys, thing, value);
					if (enumerable_symbols(thing).length > 0) throw new DevalueError(`Cannot stringify POJOs with symbolic keys`, keys, thing, value);
					if (Object.getPrototypeOf(thing) === null) {
						str = "[\"null\"";
						for (const key of Object.keys(thing)) {
							if (key === "__proto__") throw new DevalueError(`Cannot stringify objects with __proto__ keys`, keys, thing, value);
							keys.push(stringify_key(key));
							str += `,${stringify_string(key)},${flatten(thing[key])}`;
							keys.pop();
						}
						str += "]";
					} else {
						str = "{";
						let started = false;
						for (const key of Object.keys(thing)) {
							if (key === "__proto__") throw new DevalueError(`Cannot stringify objects with __proto__ keys`, keys, thing, value);
							if (started) str += ",";
							started = true;
							keys.push(stringify_key(key));
							str += `${stringify_string(key)}:${flatten(thing[key])}`;
							keys.pop();
						}
						str += "}";
					}
			}
		}
		stringified[index] = str;
		return index;
	}
	const index = flatten(value);
	if (index < 0) return `${index}`;
	return `[${stringified.join(",")}]`;
}
/**
* @param {any} thing
* @returns {string}
*/
function stringify_primitive(thing) {
	const type = typeof thing;
	if (type === "string") return stringify_string(thing);
	if (thing instanceof String) return stringify_string(thing.toString());
	if (thing === void 0) return (-1).toString();
	if (thing === 0 && 1 / thing < 0) return (-6).toString();
	if (type === "bigint") return `["BigInt","${thing}"]`;
	return String(thing);
}
//#endregion
//#region node_modules/.pnpm/@sveltejs+kit@2.55.0_@sveltejs+vite-plugin-svelte@7.0.0_svelte@5.55.0_vite@8.0.2__svelt_c1620dc8e019698275e4944bc5517231/node_modules/@sveltejs/kit/src/runtime/utils.js
var text_encoder = new TextEncoder();
var text_decoder = new TextDecoder();
/**
* Like node's path.relative, but without using node
* @param {string} from
* @param {string} to
*/
function get_relative_path(from, to) {
	const from_parts = from.split(/[/\\]/);
	const to_parts = to.split(/[/\\]/);
	from_parts.pop();
	while (from_parts[0] === to_parts[0]) {
		from_parts.shift();
		to_parts.shift();
	}
	let i = from_parts.length;
	while (i--) from_parts[i] = "..";
	return from_parts.concat(to_parts).join("/");
}
/**
* @param {Uint8Array} bytes
* @returns {string}
*/
function base64_encode(bytes) {
	if (globalThis.Buffer) return globalThis.Buffer.from(bytes).toString("base64");
	let binary = "";
	for (let i = 0; i < bytes.length; i++) binary += String.fromCharCode(bytes[i]);
	return btoa(binary);
}
/**
* @param {string} encoded
* @returns {Uint8Array}
*/
function base64_decode(encoded) {
	if (globalThis.Buffer) {
		const buffer = globalThis.Buffer.from(encoded, "base64");
		return new Uint8Array(buffer);
	}
	const binary = atob(encoded);
	const bytes = new Uint8Array(binary.length);
	for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i);
	return bytes;
}
//#endregion
//#region node_modules/.pnpm/@sveltejs+kit@2.55.0_@sveltejs+vite-plugin-svelte@7.0.0_svelte@5.55.0_vite@8.0.2__svelt_c1620dc8e019698275e4944bc5517231/node_modules/@sveltejs/kit/src/utils/error.js
/**
* @param {unknown} err
* @return {Error}
*/
function coalesce_to_error(err) {
	return err instanceof Error || err && err.name && err.message ? err : new Error(JSON.stringify(err));
}
/**
* This is an identity function that exists to make TypeScript less
* paranoid about people throwing things that aren't errors, which
* frankly is not something we should care about
* @param {unknown} error
*/
function normalize_error(error) {
	return error;
}
/**
* @param {unknown} error
*/
function get_status(error) {
	return error instanceof HttpError || error instanceof SvelteKitError ? error.status : 500;
}
/**
* @param {unknown} error
*/
function get_message(error) {
	return error instanceof SvelteKitError ? error.text : "Internal Error";
}
//#endregion
//#region node_modules/.pnpm/@sveltejs+kit@2.55.0_@sveltejs+vite-plugin-svelte@7.0.0_svelte@5.55.0_vite@8.0.2__svelt_c1620dc8e019698275e4944bc5517231/node_modules/@sveltejs/kit/src/runtime/shared.js
/** @import { Transport } from '@sveltejs/kit' */
/**
* @param {string} route_id
* @param {string} dep
*/
function validate_depends(route_id, dep) {
	const match = /^(moz-icon|view-source|jar):/.exec(dep);
	if (match) console.warn(`${route_id}: Calling \`depends('${dep}')\` will throw an error in Firefox because \`${match[1]}\` is a special URI scheme`);
}
var INVALIDATED_PARAM = "x-sveltekit-invalidated";
var TRAILING_SLASH_PARAM = "x-sveltekit-trailing-slash";
/**
* @param {any} data
* @param {string} [location_description]
*/
function validate_load_response(data, location_description) {
	if (data != null && Object.getPrototypeOf(data) !== Object.prototype) throw new Error(`a load function ${location_description} returned ${typeof data !== "object" ? `a ${typeof data}` : data instanceof Response ? "a Response object" : Array.isArray(data) ? "an array" : "a non-plain object"}, but must return a plain object at the top level (i.e. \`return {...}\`)`);
}
/**
* Try to `devalue.stringify` the data object using the provided transport encoders.
* @param {any} data
* @param {Transport} transport
*/
function stringify(data, transport) {
	return stringify$1(data, Object.fromEntries(Object.entries(transport).map(([k, v]) => [k, v.encode])));
}
/**
* Stringifies the argument (if any) for a remote function in such a way that
* it is both a valid URL and a valid file name (necessary for prerendering).
* @param {any} value
* @param {Transport} transport
*/
function stringify_remote_arg(value, transport) {
	if (value === void 0) return "";
	const json_string = stringify(value, transport);
	return base64_encode(new TextEncoder().encode(json_string)).replaceAll("=", "").replaceAll("+", "-").replaceAll("/", "_");
}
/**
* Parses the argument (if any) for a remote function
* @param {string} string
* @param {Transport} transport
*/
function parse_remote_arg(string, transport) {
	if (!string) return void 0;
	return parse(text_decoder.decode(base64_decode(string.replaceAll("-", "+").replaceAll("_", "/"))), Object.fromEntries(Object.entries(transport).map(([k, v]) => [k, v.decode])));
}
/**
* @param {string} id
* @param {string} payload
*/
function create_remote_key(id, payload) {
	return id + "/" + payload;
}
//#endregion
export { stringify$1 as _, stringify as a, validate_load_response as c, get_status as d, normalize_error as f, text_encoder as g, text_decoder as h, parse_remote_arg as i, coalesce_to_error as l, get_relative_path as m, TRAILING_SLASH_PARAM as n, stringify_remote_arg as o, base64_encode as p, create_remote_key as r, validate_depends as s, INVALIDATED_PARAM as t, get_message as u, parse as v };
