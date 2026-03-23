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
		client: {start:"_app/immutable/entry/start.CQiNHwTZ.js",app:"_app/immutable/entry/app.Cb5PUK4b.js",imports:["_app/immutable/entry/start.CQiNHwTZ.js","_app/immutable/chunks/RzXkAYVe.js","_app/immutable/chunks/CYzvPeNh.js","_app/immutable/chunks/CHK4LGn_.js","_app/immutable/entry/app.Cb5PUK4b.js","_app/immutable/chunks/CYzvPeNh.js","_app/immutable/chunks/CHK4LGn_.js"],stylesheets:[],fonts:[],uses_env_dynamic_public:false},
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
