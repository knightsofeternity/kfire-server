<script lang="ts">
	import ThemeToggle from '$lib/components/ThemeToggle.svelte';
	import LanguageToggle from '$lib/components/LanguageToggle.svelte';

	const year = new Date().getFullYear();
</script>

<footer class="border-t border-[var(--color-border)] bg-[var(--color-surface)]">
	<div
		class="mx-auto flex max-w-5xl flex-col items-center justify-between gap-3 px-4 py-5 text-sm text-[var(--color-muted)] sm:flex-row"
	>
		<a
			href="https://github.com/knightsofeternity"
			target="_blank"
			rel="noreferrer"
			class="font-display inline-flex items-center gap-2 font-bold tracking-wider uppercase transition-colors hover:text-[var(--color-brand)]"
		>
			<svg width="18" height="18" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
				<path
					d="M12 .5C5.37.5 0 5.87 0 12.5c0 5.3 3.44 9.8 8.21 11.39.6.11.82-.26.82-.58 0-.29-.01-1.05-.02-2.06-3.34.73-4.04-1.61-4.04-1.61-.55-1.39-1.34-1.76-1.34-1.76-1.09-.75.08-.73.08-.73 1.2.09 1.84 1.24 1.84 1.24 1.07 1.83 2.81 1.3 3.5.99.11-.78.42-1.3.76-1.6-2.67-.3-5.47-1.33-5.47-5.93 0-1.31.47-2.38 1.24-3.22-.13-.3-.54-1.52.12-3.18 0 0 1.01-.32 3.3 1.23a11.5 11.5 0 0 1 6 0c2.29-1.55 3.3-1.23 3.3-1.23.66 1.66.25 2.88.12 3.18.77.84 1.24 1.91 1.24 3.22 0 4.61-2.81 5.62-5.49 5.92.43.37.81 1.1.81 2.22 0 1.6-.01 2.89-.01 3.29 0 .32.22.7.83.58A12.01 12.01 0 0 0 24 12.5C24 5.87 18.63.5 12 .5z"
				/>
			</svg>
			Knights of Eternity
		</a>

		<a
			href="https://kfire.io"
			target="_blank"
			rel="noreferrer"
			class="font-display font-bold tracking-wider uppercase transition-colors hover:text-[var(--color-brand)]"
		>
			kfire.io
		</a>

		<div class="flex items-center gap-3">
			<LanguageToggle />
			<ThemeToggle />
			<p>
				&copy; {year}
				<a
					href="https://www.guilde-ke.fr"
					target="_blank"
					rel="noreferrer"
					class="text-[var(--color-text)] transition-colors hover:text-[var(--color-brand)]"
				>
					Knights of Eternity
				</a>
			</p>
		</div>
	</div>
</footer>
