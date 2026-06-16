<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { api, type GameDetail } from '$lib/api';
	import { auth } from '$lib/stores/auth.svelte';
	import { formatDuration } from '$lib/format';
	import Avatar from '$lib/components/Avatar.svelte';
	import { t } from '$lib/i18n';
	import { wowClassColor, wowClassIcon } from '$lib/wow';

	let detail = $state<GameDetail | null>(null);
	let loading = $state(true);
	let error = $state('');
	let toggling = $state(false);

	const slug = $derived(page.params.slug ?? '');
	const isAdmin = $derived($auth.user?.role === 'admin');

	async function toggleHidden() {
		if (!detail || toggling) return;
		toggling = true;
		try {
			const res = await api.setGameHidden(detail.game.id, !detail.game.hidden);
			detail = { ...detail, game: { ...detail.game, hidden: res.hidden } };
		} catch (e) {
			error = e instanceof Error ? e.message : t('game.loadError');
		} finally {
			toggling = false;
		}
	}
	let topSeconds = $derived(
		Math.max(1, ...(detail?.leaderboard ?? []).map((e) => e.total_seconds))
	);

	onMount(load);
	async function load() {
		loading = true;
		error = '';
		try {
			detail = await api.getGame(slug);
		} catch (e) {
			error = e instanceof Error ? e.message : t('game.loadError');
		} finally {
			loading = false;
		}
	}
</script>

{#if loading}
	<p class="text-[var(--color-muted)]">{t('game.loading')}</p>
{:else if error}
	<p class="text-[var(--color-magenta)]">{error}</p>
{:else if detail}
	<!-- Cover banner -->
	<div class="pd-card pd-glow mb-6 overflow-hidden">
		{#if detail.game.cover_url}
			<div class="relative h-44 sm:h-64">
				<img src={detail.game.cover_url} alt="" class="h-full w-full object-cover" />
				<!-- Cinematic gradient overlay - stronger at bottom for text legibility -->
				<div class="absolute inset-0 bg-gradient-to-t from-[var(--color-bg)] via-[rgba(18,19,25,0.55)] to-transparent"></div>
				<!-- Subtle violet glow strip at the top edge -->
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
				<h1
					class="pd-heading text-3xl text-white"
					style="text-shadow: 0 0 24px rgba(154,108,255,0.7);"
				>
					{detail.game.name}
				</h1>
			</div>
		{/if}
		<!-- Stats row -->
		<div class="flex gap-8 border-t border-[var(--color-border)] bg-[var(--color-surface-2)] px-5 py-4">
			<div>
				<p class="font-display text-xl font-bold text-[var(--color-brand-bright)]">{formatDuration(detail.total_seconds)}</p>
				<p class="text-xs text-[var(--color-muted)] uppercase tracking-wide">{t('game.totalPlayed')}</p>
			</div>
			<div>
				<p class="font-display text-xl font-bold text-[var(--color-cyan)]">{detail.player_count}</p>
				<p class="text-xs text-[var(--color-muted)] uppercase tracking-wide">{t('game.players', { count: detail.player_count })}</p>
			</div>
			{#if isAdmin}
				<div class="ml-auto flex flex-col items-end justify-center gap-1">
					<button
						type="button"
						class="pd-cut-sm px-3 py-1.5 text-sm font-display border transition-colors disabled:opacity-50
							{detail.game.hidden
								? 'border-[var(--color-brand)] text-[var(--color-brand-bright)] hover:bg-[var(--color-surface)]'
								: 'border-[var(--color-magenta)] text-[var(--color-magenta)] hover:bg-[var(--color-surface)]'}"
						disabled={toggling}
						onclick={toggleHidden}
					>
						{detail.game.hidden ? t('game.show') : t('game.hide')}
					</button>
					{#if detail.game.hidden}
						<p class="max-w-xs text-right text-xs text-[var(--color-muted)]">{t('game.hiddenNotice')}</p>
					{/if}
				</div>
			{/if}
		</div>
	</div>

	<!-- Leaderboard -->
	<h2 class="pd-heading mb-4 flex items-center gap-2 text-sm text-[var(--color-brand-bright)]">
		<span class="inline-block h-4 w-1 bg-[var(--color-brand)]"></span>
		{t('game.leaderboard')}
	</h2>
	{#if detail.leaderboard.length === 0}
		<p class="text-sm text-[var(--color-muted)]">{t('game.noPlayers')}</p>
	{:else}
		<ul class="flex flex-col gap-2">
			{#each detail.leaderboard as e, i (e.user_id)}
				<a
					href="/players/{e.user_id}"
					class="pd-card group flex items-center gap-3 p-3 transition-all duration-150
						{i === 0
							? 'border-[var(--color-gold)] bg-[var(--color-surface-2)]'
							: 'hover:border-[var(--color-brand)]'}"
					style={i === 0 ? 'border-color: var(--color-gold); box-shadow: 0 0 18px -4px rgba(255,180,0,0.35);' : ''}
				>
					<!-- Rank badge -->
					<span
						class="pd-cut-sm flex h-7 w-7 shrink-0 items-center justify-center font-display text-sm font-bold
							{i === 0
								? 'bg-[var(--color-gold)] text-[#1a1200]'
								: i === 1
								? 'bg-[var(--color-surface-2)] text-[var(--color-muted)]'
								: i === 2
								? 'bg-[var(--color-surface-2)] text-[var(--color-muted)]'
								: 'bg-[var(--color-surface-2)] text-[var(--color-muted)]'}"
					>
						{i + 1}
					</span>

					<Avatar username={e.username} url={e.avatar_url} size={36} />

					<span class="flex-1 truncate font-display font-semibold
						{i === 0 ? 'text-[var(--color-gold)]' : 'text-[var(--color-text)]'}">
						{e.username}
					</span>

					<!-- Progress bar (desktop) -->
					<div class="hidden h-1.5 w-28 overflow-hidden sm:block" style="background-color: var(--color-bg); clip-path: polygon(4px 0, 100% 0, 100% calc(100% - 4px), calc(100% - 4px) 100%, 0 100%, 0 4px);">
						<div
							class="h-full transition-all duration-300"
							style="width:{Math.max(3, (e.total_seconds / topSeconds) * 100)}%;
								background-color: {i === 0 ? 'var(--color-gold)' : 'var(--color-brand)'};"
						></div>
					</div>

					<span class="w-20 text-right font-display text-sm
						{i === 0 ? 'text-[var(--color-gold)]' : 'text-[var(--color-muted)]'}">
						{formatDuration(e.total_seconds)}
					</span>
				</a>
			{/each}
		</ul>
	{/if}

	<!-- Achievements -->
	<h2 class="pd-heading mt-8 mb-4 flex items-center gap-2 text-sm text-[var(--color-brand-bright)]">
		<span class="inline-block h-4 w-1 bg-[var(--color-brand)]"></span>
		{t('game.achievements')}
	</h2>
	{#if !detail.achievements || detail.achievements.length === 0}
		<p class="text-sm text-[var(--color-muted)]">{t('game.noAchievements')}</p>
	{:else}
		<div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
			{#each detail.achievements as a (a.api_name)}
				<div class="pd-card flex items-center gap-3 p-3">
					{#if a.icon_url}
						<img
							src={a.icon_url}
							alt=""
							class="pd-cut-sm h-10 w-10 shrink-0 object-cover"
						/>
					{:else}
						<span class="pd-cut-sm flex h-10 w-10 shrink-0 items-center justify-center bg-[var(--color-surface)] text-xl" aria-hidden="true">🏆</span>
					{/if}
					<div class="min-w-0 flex-1">
						<p class="font-display truncate font-bold text-[var(--color-text)]">{a.display_name ?? a.api_name}</p>
						<p class="text-xs text-[var(--color-muted)]">{t('game.unlockedBy', { count: a.unlocks })}</p>
					</div>
				</div>
			{/each}
		</div>
	{/if}

	<!-- WoW Characters -->
	{#if detail.wow_characters?.length}
		<section class="mt-6">
			<h2 class="pd-heading mb-3 flex items-center gap-2 text-sm text-[var(--color-brand-bright)]">
				<span class="inline-block h-4 w-1 bg-[var(--color-brand)]"></span>
				{t('game.wowCharacters')}
			</h2>
			<div class="grid grid-cols-1 gap-2 sm:grid-cols-2">
				{#each detail.wow_characters as ch}
					<div class="pd-cut-sm flex items-center gap-3 px-3 py-2 border border-[var(--color-border)] bg-[var(--color-surface-2)]">
						{#if wowClassIcon(ch.class)}
							<img
								src={wowClassIcon(ch.class)}
								alt={ch.class ?? ''}
								class="h-8 w-8 shrink-0 rounded"
								loading="lazy"
								onerror={(e) => { (e.currentTarget as HTMLImageElement).style.display = 'none'; }}
							/>
						{/if}
						<div class="min-w-0 flex-1">
							<p class="font-display font-semibold text-[var(--color-text)]">{ch.name}{#if ch.realm}<span class="text-[var(--color-muted)]"> - {ch.realm}</span>{/if}</p>
							<p class="text-sm text-[var(--color-muted)]">
								{#if ch.level}{t('game.level')} {ch.level} · {/if}{ch.race ?? ''}{ch.race && ch.class ? ' ' : ''}<span style="color: {wowClassColor(ch.class)}">{ch.class ?? ''}</span>{#if ch.race || ch.class} · {/if}{t('game.ilvl')} {ch.item_level}{#if ch.mythic_rating} · M+ {Math.round(ch.mythic_rating)}{/if}{#if ch.achievement_points} · {t('game.achievementPoints')} {ch.achievement_points}{/if}
							</p>
						</div>
					</div>
				{/each}
			</div>
		</section>
	{/if}

	<!-- Battle.net Profiles -->
	{#if detail.bnet_profiles?.length}
		<section class="mt-6">
			<h2 class="pd-heading mb-3 text-xs text-[var(--color-brand-bright)]">{t('game.bnetProfiles')}</h2>
			<div class="grid grid-cols-1 gap-2 sm:grid-cols-2">
				{#each detail.bnet_profiles as p}
					<div class="pd-cut-sm px-3 py-2">
						<p class="font-display font-semibold text-[var(--color-text)]">{p.username}</p>
						{#if slug === 'diablo-iii'}
							<p class="text-sm text-[var(--color-muted)]">{t('game.paragon')} {Number(p.data.paragon ?? 0)}{#if Array.isArray(p.data.heroes)} · {(p.data.heroes as unknown[]).length} {t('game.heroes')}{/if}</p>
						{:else if slug === 'starcraft-ii-battle-chest'}
							<p class="text-sm text-[var(--color-muted)]">{String(p.data.race ?? '')}{#if p.data.league} · {String(p.data.league)}{/if}</p>
						{/if}
					</div>
				{/each}
			</div>
		</section>
	{/if}
{/if}
