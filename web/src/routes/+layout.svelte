<script lang="ts">
	import '../app.css';
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { auth } from '$lib/stores/auth.svelte';
	import Avatar from '$lib/components/Avatar.svelte';
	import Login from '$lib/components/Login.svelte';

	let { children } = $props();

	onMount(() => {
		auth.init();
	});

	let navItems = $derived([
		{ href: '/', label: 'Dashboard' },
		{ href: '/players', label: 'Players' },
		...($auth.user?.role === 'admin' ? [{ href: '/admin', label: 'Admin' }] : []),
		{ href: '/account', label: 'Account' }
	]);

	function isActive(href: string): boolean {
		return href === '/' ? page.url.pathname === '/' : page.url.pathname.startsWith(href);
	}
</script>

<svelte:head>
	<title>KFIRE</title>
</svelte:head>

{#if !$auth.ready}
	<div class="grid min-h-screen place-items-center text-[var(--color-muted)]">Loading…</div>
{:else if !$auth.user}
	<Login />
{:else}
	<div class="min-h-screen">
		<header class="border-b border-[var(--color-border)] bg-[var(--color-surface)]">
			<div class="mx-auto flex max-w-5xl items-center justify-between px-4 py-3">
				<div class="flex items-center gap-8">
					<a href="/" class="flex items-center gap-2">
						<img src="/favicon-96.png" alt="" class="h-7 w-7" />
						<span class="text-lg font-bold tracking-[0.12em] text-[var(--color-brand)]">KFIRE</span>
					</a>
					<nav class="flex gap-1">
						{#each navItems as item (item.href)}
							<a
								href={item.href}
								class="rounded-md px-3 py-1.5 text-sm transition-colors {isActive(item.href)
									? 'bg-[var(--color-surface-2)] text-[var(--color-text)]'
									: 'text-[var(--color-muted)] hover:text-[var(--color-text)]'}"
							>
								{item.label}
							</a>
						{/each}
					</nav>
				</div>
				<a href="/account" class="flex items-center gap-2">
					<span class="text-sm text-[var(--color-muted)]">{$auth.user.username}</span>
					<Avatar username={$auth.user.username} url={$auth.user.avatar_url} size={32} />
				</a>
			</div>
		</header>

		<main class="mx-auto max-w-5xl px-4 py-6">
			{@render children()}
		</main>
	</div>
{/if}
