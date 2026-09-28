<script lang="ts">
	import { presence } from '$lib/stores/presence.svelte';
	import { timeAgo } from '$lib/format';
	import Avatar from '$lib/components/Avatar.svelte';
	import StatusBadge from '$lib/components/StatusBadge.svelte';
	import { t } from '$lib/i18n';
	import { onMount } from 'svelte';

	// Hiding offline members is a per-viewer preference, kept in this browser.
	const HIDE_OFFLINE_KEY = 'kfire.dashboard.hideOffline';
	let hideOffline = $state(false);
	onMount(() => {
		try {
			hideOffline = localStorage.getItem(HIDE_OFFLINE_KEY) === '1';
		} catch {
			/* storage blocked: the default stays */
		}
	});
	function setHideOffline(on: boolean) {
		hideOffline = on;
		try {
			localStorage.setItem(HIDE_OFFLINE_KEY, on ? '1' : '0');
		} catch {
			/* storage blocked: the choice lasts until the page reloads */
		}
	}

	const rank = { in_game: 0, online: 1, offline: 2 };
	let sorted = $derived(
		presence.list.sort(
			(a, b) => rank[a.status] - rank[b.status] || a.username.localeCompare(b.username)
		)
	);
	let playing = $derived(sorted.filter((e) => e.status === 'in_game').length);
	let online = $derived(sorted.filter((e) => e.status !== 'offline').length);
	let shown = $derived(hideOffline ? sorted.filter((e) => e.status !== 'offline') : sorted);
</script>

<div class="mb-5 flex items-center justify-between">
	<div>
		<h1 class="pd-heading text-2xl">{t('dashboard.title')}</h1>
		<p class="text-sm text-[var(--color-muted)]">
			<span style="color: var(--color-in-game);">{t('dashboard.playing', { count: playing })}</span>
			&middot;
			<span style="color: var(--color-online);">{t('dashboard.online', { count: online })}</span>
		</p>
		<label class="mt-1 inline-flex items-center gap-2 text-xs text-[var(--color-muted)]">
			<input
				type="checkbox"
				checked={hideOffline}
				onchange={(e) => setHideOffline(e.currentTarget.checked)}
			/>
			{t('dashboard.hideOffline')}
		</label>
	</div>
	<span class="inline-flex items-center gap-1.5 text-xs text-[var(--color-muted)]" title={t('dashboard.liveConnection')}>
		<span
			class="h-2 w-2 rounded-full {presence.status === 'connected'
				? 'bg-[var(--color-online)]'
				: presence.status === 'connecting'
					? 'bg-yellow-500'
					: 'bg-[var(--color-muted)]'}"
		></span>
		{presence.status === 'connected' ? t('dashboard.live') : presence.status === 'connecting' ? t('dashboard.connecting') : t('dashboard.disconnected')}
	</span>
</div>

{#if !presence.loaded}
	<p class="text-[var(--color-muted)]">{t('dashboard.loading')}</p>
{:else if sorted.length === 0}
	<p class="text-[var(--color-muted)]">{t('dashboard.noMembers')}</p>
{:else if shown.length === 0}
	<p class="text-[var(--color-muted)]">{t('dashboard.nobodyOnline')}</p>
{:else}
	<div class="grid gap-3 sm:grid-cols-2">
		{#each shown as entry (entry.user_id)}
			<!-- The card holds two links, the member's profile and the game's Steam
			     page, so it is a div: a link cannot sit inside another link. -->
			<div
				class="pd-card flex items-center gap-3 p-4 transition-colors hover:border-[var(--color-brand)] {entry.status === 'in_game' ? 'pd-glow' : ''}"
				class:opacity-60={entry.status === 'offline'}
			>
				<a href="/players/{entry.user_id}" class="flex min-w-0 flex-1 items-center gap-3">
					<Avatar username={entry.username} url={entry.avatar_url} size={44} />
					<div class="min-w-0 flex-1">
						<div class="flex items-center gap-2">
							<span
								class="truncate font-semibold {entry.status === 'in_game' ? 'text-[var(--color-brand-bright)]' : ''}"
							>{entry.username}</span>
							<StatusBadge status={entry.status} />
						</div>
						{#if entry.status === 'in_game' && entry.game}
							<div class="mt-1 flex items-center gap-2">
								{#if entry.game.icon_url}
									<img src={entry.game.icon_url} alt="" class="h-5 w-5 pd-cut-sm" />
								{/if}
								<span class="truncate text-sm font-semibold text-[var(--color-brand)]">{entry.game.name}</span>
							</div>
							{#if entry.since}
								<p class="mt-0.5 text-xs text-[var(--color-muted)]">{t('dashboard.since', { time: timeAgo(entry.since) })}</p>
							{/if}
						{:else if entry.status === 'online'}
							<p class="mt-1 text-sm" style="color: var(--color-online);">{t('dashboard.onlineStatus')}</p>
						{/if}
					</div>
				</a>
				{#if entry.status === 'in_game' && entry.game?.steam_app_id}
					<a
						href="https://store.steampowered.com/app/{entry.game.steam_app_id}"
						target="_blank"
						rel="noopener noreferrer"
						title={t('dashboard.onSteam', { game: entry.game.name })}
						class="shrink-0 rounded border border-[var(--color-border)] px-2 py-1 text-xs text-[var(--color-muted)] hover:border-[var(--color-brand)] hover:text-[var(--color-text)]"
					>
						Steam ↗
					</a>
				{/if}
			</div>
		{/each}
	</div>
{/if}
