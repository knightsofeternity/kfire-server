<script lang="ts">
	import '../app.css';
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { auth } from '$lib/stores/auth.svelte';
	import { getConfig } from '$lib/api';
	import Avatar from '$lib/components/Avatar.svelte';
	import Login from '$lib/components/Login.svelte';
	import Footer from '$lib/components/Footer.svelte';
	import { initLocale, t } from '$lib/i18n';

	let { children } = $props();

	let hasLogo = $state(false);
	let orgName = $state('KFIRE');

	onMount(async () => {
		initLocale();
		auth.init();
		try {
			const cfg = await getConfig();
			// Apply the server's dominant accent (orange is the CSS default).
			if (cfg.accent && cfg.accent !== 'orange') {
				document.documentElement.dataset.accent = cfg.accent;
			} else {
				delete document.documentElement.dataset.accent;
			}
			hasLogo = cfg.has_logo;
			orgName = cfg.org_name;
		} catch (e) {
			/* keep defaults when config is unavailable */
		}
	});

	let navItems = $derived([
		{ href: '/', label: t('nav.dashboard') },
		{ href: '/players', label: t('nav.players') },
		{ href: '/games', label: t('nav.games') },
		{ href: '/download', label: t('nav.download') },
		...($auth.user?.role === 'admin' ? [{ href: '/admin', label: t('nav.admin') }] : []),
		{ href: '/account', label: t('nav.account') }
	]);

	function isActive(href: string): boolean {
		return href === '/' ? page.url.pathname === '/' : page.url.pathname.startsWith(href);
	}
</script>

<svelte:head>
	<title>KFIRE</title>
</svelte:head>

<div class="flex min-h-screen flex-col">
	{#if !$auth.ready}
		<div class="grid flex-1 place-items-center text-[var(--color-muted)]">{t('common.loading')}</div>
	{:else if !$auth.user}
		<Login />
	{:else}
		<header
			class="pd-header-shadow border-b-2 border-[var(--color-brand)]/60 bg-[var(--color-surface)]"
		>
			<div class="mx-auto flex max-w-5xl items-center justify-between px-4 py-3">
				<div class="flex items-center gap-8">
					<a href="/" class="flex items-center gap-3">
						<img src="/kfire-logo-640.png" alt="KFIRE" class="h-10 w-auto" />
						<span
							class="font-display text-2xl font-extrabold tracking-[0.08em] text-[var(--color-text)] italic"
						>
							K<span class="text-[var(--color-brand)]">FIRE</span>
						</span>
					</a>
					{#if hasLogo}
						<span class="h-8 w-px bg-[var(--color-border)]"></span>
						<img src="/img/org/logo" alt={orgName} title={orgName} class="h-9 w-auto" />
					{/if}
					<nav class="flex gap-1">
						{#each navItems as item (item.href)}
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
				</div>
				<div class="flex items-center gap-3">
					<a href="/account" class="flex items-center gap-2">
						<span class="text-sm text-[var(--color-muted)]">{$auth.user.username}</span>
						<Avatar username={$auth.user.username} url={$auth.user.avatar_url} size={32} />
					</a>
				</div>
			</div>
		</header>

		<main class="pd-grid-bg mx-auto w-full max-w-5xl flex-1 px-4 py-6">
			{@render children()}
		</main>
	{/if}
	<Footer />
</div>
