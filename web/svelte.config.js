import adapter from '@sveltejs/adapter-static';

/** @type {import('@sveltejs/kit').Config} */
const config = {
	compilerOptions: {
		runes: ({ filename }) => (filename.split(/[/\\]/).includes('node_modules') ? undefined : true)
	},
	kit: {
		// SPA: no SSR, serve a fallback index.html that the Go server returns
		// for every non-API route.
		adapter: adapter({ fallback: 'index.html', strict: false }),
		// Content-Security-Policy. SvelteKit hashes its own inline scripts so
		// script-src stays 'self' (no unsafe-inline for scripts, the real XSS
		// vector). Styles allow inline (low risk) and Google Fonts; images allow
		// https (Steam/Discord avatars).
		csp: {
			mode: 'hash',
			directives: {
				'default-src': ['self'],
				'script-src': ['self'],
				'style-src': ['self', 'unsafe-inline', 'https://fonts.googleapis.com'],
				'font-src': ['self', 'https://fonts.gstatic.com'],
				'img-src': ['self', 'data:', 'https:'],
				'connect-src': ['self'],
				'base-uri': ['self'],
				'form-action': ['self'],
				'object-src': ['none']
			}
		}
	}
};

export default config;
