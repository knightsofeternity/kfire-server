<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { api, type PlayerGameDetail } from '$lib/api';
	import { formatDuration, timeAgo } from '$lib/format';
	import { t } from '$lib/i18n';

	let detail = $state<PlayerGameDetail | null>(null);
	let loading = $state(true);
	let error = $state('');

	const id = $derived(page.params.id ?? '');
	const slug = $derived(page.params.slug ?? '');

	onMount(load);
	async function load() {
		loading = true;
		error = '';
		try {
			detail = await api.userGameDetail(id, slug);
		} catch (e) {
			error = e instanceof Error ? e.message : t('playerGame.loadError');
		} finally {
			loading = false;
		}
	}
</script>

{#if loading}
	<p class="text-[var(--color-muted)]">{t('playerGame.loading')}</p>
{:else if error}
	<p class="text-[var(--color-magenta)]">{error}</p>
{:else if detail}
	<!-- Back link -->
	<a
		href="/players/{id}"
		class="mb-5 inline-flex items-center gap-1.5 text-sm text-[var(--color-muted)] transition-colors hover:text-[var(--color-brand-bright)]"
	>
		<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="h-4 w-4" aria-hidden="true">
			<path stroke-linecap="round" stroke-linejoin="round" d="M15 19l-7-7 7-7" />
		</svg>
		{t('playerGame.backToProfile')}
	</a>

	<!-- Game header -->
	<div class="pd-card pd-glow mb-6 overflow-hidden">
		{#if detail.game.cover_url}
			<div class="relative h-40 sm:h-56">
				<img src={detail.game.cover_url} alt="" class="h-full w-full object-cover" />
				<div class="absolute inset-0 bg-gradient-to-t from-[var(--color-bg)] via-[rgba(18,19,25,0.55)] to-transparent"></div>
				<div class="absolute inset-x-0 top-0 h-1 bg-[var(--color-brand-bright)] opacity-70"></div>
				<div class="absolute bottom-0 left-0 flex items-end gap-4 p-5">
					{#if detail.game.icon_url}
						<img
							src={detail.game.icon_url}
							alt=""
							class="pd-cut-sm h-14 w-14 shrink-0 object-cover shadow-lg ring-2 ring-[var(--color-brand)]"
						/>
					{/if}
					<h1
						class="pd-heading text-3xl text-white drop-shadow-lg"
						style="text-shadow: 0 0 24px rgba(154,108,255,0.7), 0 2px 8px rgba(0,0,0,0.8);"
					>
						{detail.game.name}
					</h1>
				</div>
			</div>
		{:else}
			<div class="flex items-center gap-4 p-5">
				{#if detail.game.icon_url}
					<img
						src={detail.game.icon_url}
						alt=""
						class="pd-cut-sm h-14 w-14 shrink-0 object-cover ring-2 ring-[var(--color-brand)]"
					/>
				{/if}
				<h1 class="pd-heading text-3xl text-white" style="text-shadow: 0 0 24px rgba(154,108,255,0.7);">
					{detail.game.name}
				</h1>
			</div>
		{/if}

		<!-- Playtime stats row -->
		<div class="flex gap-8 border-t border-[var(--color-border)] bg-[var(--color-surface-2)] px-5 py-4">
			{#if detail.total_seconds}
				<div>
					<p class="font-display text-xl font-bold text-[var(--color-brand-bright)]">{formatDuration(detail.total_seconds)}</p>
					<p class="text-xs text-[var(--color-muted)] uppercase tracking-wide">{t('playerGame.playtime')}</p>
				</div>
				{#if detail.session_count}
					<div>
						<p class="font-display text-xl font-bold text-[var(--color-cyan)]">{detail.session_count}</p>
						<p class="text-xs text-[var(--color-muted)] uppercase tracking-wide">{t('playerGame.sessions')}</p>
					</div>
				{/if}
				{#if detail.last_played_at}
					<div>
						<p class="font-display text-xl font-bold text-[var(--color-text)]">{timeAgo(detail.last_played_at)}</p>
						<p class="text-xs text-[var(--color-muted)] uppercase tracking-wide">last played</p>
					</div>
				{/if}
			{:else}
				<p class="text-sm text-[var(--color-muted)]">{t('playerGame.noPlaytime')}</p>
			{/if}
		</div>
	</div>

	<!-- WoW Characters -->
	{#if detail.wow_characters?.length}
		<section class="mb-6">
			<h2 class="pd-heading mb-3 flex items-center gap-2 text-sm text-[var(--color-brand-bright)]">
				<span class="inline-block h-4 w-1 bg-[var(--color-brand)]"></span>
				{t('game.wowCharacters')}
			</h2>
			<div class="grid grid-cols-1 gap-2 sm:grid-cols-2">
				{#each detail.wow_characters as ch}
					<div class="pd-cut-sm px-3 py-2 border border-[var(--color-border)] bg-[var(--color-surface-2)]">
						<p class="font-display font-semibold text-[var(--color-text)]">
							{ch.name}{#if ch.realm}<span class="text-[var(--color-muted)]"> - {ch.realm}</span>{/if}
						</p>
						<p class="text-sm text-[var(--color-muted)]">
							{ch.race ?? ''}{ch.race && ch.class ? ' ' : ''}{ch.class ?? ''}{#if ch.race || ch.class} · {/if}{t('game.ilvl')} {ch.item_level}{#if ch.mythic_rating} · M+ {Math.round(ch.mythic_rating)}{/if}
						</p>
					</div>
				{/each}
			</div>
		</section>
	{/if}

	<!-- Battle.net Profile (Diablo III / StarCraft II) -->
	{#if detail.bnet_profile}
		<section class="mb-6">
			<h2 class="pd-heading mb-3 flex items-center gap-2 text-sm text-[var(--color-brand-bright)]">
				<span class="inline-block h-4 w-1 bg-[var(--color-brand)]"></span>
				{t('game.bnetProfiles')}
			</h2>
			<div class="pd-cut-sm px-3 py-2 border border-[var(--color-border)] bg-[var(--color-surface-2)]">
				{#if slug === 'diablo-iii'}
					<p class="text-sm text-[var(--color-muted)]">
						{t('game.paragon')} {Number(detail.bnet_profile.paragon ?? 0)}{#if Array.isArray(detail.bnet_profile.heroes)} · {(detail.bnet_profile.heroes as unknown[]).length} {t('game.heroes')}{/if}
					</p>
				{:else if slug === 'starcraft-ii-battle-chest'}
					<p class="text-sm text-[var(--color-muted)]">
						{String(detail.bnet_profile.race ?? '')}{#if detail.bnet_profile.league} · {String(detail.bnet_profile.league)}{/if}
					</p>
				{:else}
					<p class="text-sm text-[var(--color-muted)] font-mono text-xs">{JSON.stringify(detail.bnet_profile)}</p>
				{/if}
			</div>
		</section>
	{/if}

	<!-- Achievements -->
	{#if detail.achievements?.length}
		<section class="mb-6">
			<h2 class="pd-heading mb-4 flex items-center gap-2 text-sm text-[var(--color-gold)]">
				<span class="inline-block h-4 w-1 bg-[var(--color-gold)]"></span>
				{t('game.achievements')}
			</h2>
			<div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
				{#each detail.achievements as a (a.api_name)}
					<div class="pd-card flex items-center gap-3 p-3">
						{#if a.icon_url}
							<img src={a.icon_url} alt="" class="pd-cut-sm h-10 w-10 shrink-0 object-cover" />
						{:else}
							<span class="pd-cut-sm flex h-10 w-10 shrink-0 items-center justify-center bg-[var(--color-surface)] text-xl" aria-hidden="true">🏆</span>
						{/if}
						<div class="min-w-0 flex-1">
							<p class="font-display truncate font-bold text-[var(--color-text)]">{a.display_name ?? a.api_name}</p>
							<p class="text-xs text-[var(--color-muted)]">{timeAgo(a.unlocked_at)}</p>
						</div>
					</div>
				{/each}
			</div>
		</section>
	{/if}
{/if}
