export const manifest = (() => {
function __memo(fn) {
	let value;
	return () => value ??= (value = fn());
}

return {
	appDir: "_app",
	appPath: "_app",
	assets: new Set([]),
	mimeTypes: {},
	_: {
		client: {start:"_app/immutable/entry/start.D8l_tbZ1.js",app:"_app/immutable/entry/app.B-VWunr8.js",imports:["_app/immutable/entry/start.D8l_tbZ1.js","_app/immutable/chunks/Cmk9Ek-L.js","_app/immutable/chunks/CHa7Rgnj.js","_app/immutable/chunks/B0dYN30k.js","_app/immutable/chunks/VGtfwmDZ.js","_app/immutable/entry/app.B-VWunr8.js","_app/immutable/chunks/CHa7Rgnj.js","_app/immutable/chunks/B0dYN30k.js","_app/immutable/chunks/VGtfwmDZ.js"],stylesheets:[],fonts:[],uses_env_dynamic_public:false},
		nodes: [
			__memo(() => import('./nodes/0.js')),
			__memo(() => import('./nodes/1.js')),
			__memo(() => import('./nodes/2.js'))
		],
		remotes: {
			
		},
		routes: [
			{
				id: "/",
				pattern: /^\/$/,
				params: [],
				page: { layouts: [0,], errors: [1,], leaf: 2 },
				endpoint: null
			}
		],
		prerendered_routes: new Set([]),
		matchers: async () => {
			
			return {  };
		},
		server_assets: {}
	}
}
})();
