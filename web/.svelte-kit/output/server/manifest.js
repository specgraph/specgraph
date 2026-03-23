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
		client: {start:"_app/immutable/entry/start.B0A1g9qK.js",app:"_app/immutable/entry/app.Drc1z9ET.js",imports:["_app/immutable/entry/start.B0A1g9qK.js","_app/immutable/chunks/8W1mVzqm.js","_app/immutable/chunks/CYzvPeNh.js","_app/immutable/chunks/CHK4LGn_.js","_app/immutable/entry/app.Drc1z9ET.js","_app/immutable/chunks/CYzvPeNh.js","_app/immutable/chunks/CHK4LGn_.js"],stylesheets:[],fonts:[],uses_env_dynamic_public:false},
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
