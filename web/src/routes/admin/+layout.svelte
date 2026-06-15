<script lang="ts">
	import { page } from '$app/state';

	let { children } = $props();

	const adminNav = [
		{ href: '/admin', label: 'Administration' },
		{ href: '/admin/api-keys', label: 'Clés API' }
	];

	function isActive(href: string): boolean {
		return href === '/admin' ? page.url.pathname === '/admin' : page.url.pathname.startsWith(href);
	}
</script>

<nav class="mb-6 flex gap-1 border-b border-[var(--color-border)] pb-2">
	{#each adminNav as item (item.href)}
		<a
			href={item.href}
			class="font-display px-3 py-1.5 text-xs font-bold tracking-wider uppercase transition-colors {isActive(
				item.href
			)
				? 'text-[var(--color-brand-bright)]'
				: 'text-[var(--color-muted)] hover:text-[var(--color-text)]'}"
		>
			{item.label}
			{#if isActive(item.href)}
				<span class="mt-0.5 block h-0.5 w-full bg-[var(--color-brand)]"></span>
			{/if}
		</a>
	{/each}
</nav>

{@render children()}
