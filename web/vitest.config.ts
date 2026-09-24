import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vitest/config';

// hearthstone.ts imports $lib/i18n, which pulls in store.svelte.ts (runes):
// that file only compiles through the Svelte plugin, which sveltekit() wires
// in, so it must be present here too, even though this config was to stay
// minimal per the plan.
export default defineConfig({
	plugins: [sveltekit()],
	test: { include: ['src/**/*.test.ts'], environment: 'node' }
});
