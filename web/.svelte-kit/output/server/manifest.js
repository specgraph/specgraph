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
		client: {start:"_app/immutable/entry/start.Do_MCiZt.js",app:"_app/immutable/entry/app.BR_6Fhxy.js",imports:["_app/immutable/entry/start.Do_MCiZt.js","_app/immutable/chunks/D2guxoj4.js","_app/immutable/chunks/BCH0nEjg.js","_app/immutable/entry/app.BR_6Fhxy.js","_app/immutable/chunks/BCH0nEjg.js","_app/immutable/chunks/DNegTBFU.js","_app/immutable/chunks/DQwDdmL6.js","_app/immutable/chunks/ac2rllyU.js","_app/immutable/chunks/IkwjGrrj.js","_app/immutable/chunks/CWoEj9vS.js"],stylesheets:[],fonts:[],uses_env_dynamic_public:false},
		nodes: [
			__memo(() => import('./nodes/0.js')),
			__memo(() => import('./nodes/1.js')),
			__memo(() => import('./nodes/2.js')),
			__memo(() => import('./nodes/3.js')),
			__memo(() => import('./nodes/4.js')),
			__memo(() => import('./nodes/5.js'))
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
			},
			{
				id: "/decision/[slug]",
				pattern: /^\/decision\/([^/]+?)\/?$/,
				params: [{"name":"slug","optional":false,"rest":false,"chained":false}],
				page: { layouts: [0,], errors: [1,], leaf: 3 },
				endpoint: null
			},
			{
				id: "/graph",
				pattern: /^\/graph\/?$/,
				params: [],
				page: { layouts: [0,], errors: [1,], leaf: 4 },
				endpoint: null
			},
			{
				id: "/spec/[slug]",
				pattern: /^\/spec\/([^/]+?)\/?$/,
				params: [{"name":"slug","optional":false,"rest":false,"chained":false}],
				page: { layouts: [0,], errors: [1,], leaf: 5 },
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
