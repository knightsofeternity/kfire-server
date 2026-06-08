import { sveltekit } from '@sveltejs/kit/vite';
import tailwindcss from '@tailwindcss/vite';
import { defineConfig } from 'vite';

// In dev, proxy API + WebSocket to the Go server (default :8080) so the SPA
// and backend share an origin. In production the Go binary serves the built
// SPA itself, so no proxy is involved.
const API_TARGET = process.env.KFIRE_DEV_API ?? 'http://localhost:8080';

export default defineConfig({
	plugins: [tailwindcss(), sveltekit()],
	server: {
		proxy: {
			'/api': { target: API_TARGET, changeOrigin: true },
			'/ws': { target: API_TARGET, ws: true, changeOrigin: true }
		}
	}
});
