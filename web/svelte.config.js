import adapter from '@sveltejs/adapter-static';

/** @type {import('@sveltejs/kit').Config} */
const config = {
	compilerOptions: {
		runes: ({ filename }) => (filename.split(/[/\\]/).includes('node_modules') ? undefined : true)
	},
	kit: {
		// SPA: no SSR, serve a fallback index.html that the Go server returns
		// for every non-API route.
		adapter: adapter({ fallback: 'index.html', strict: false })
	}
};

export default config;
