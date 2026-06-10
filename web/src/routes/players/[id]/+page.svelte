<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { api, type Profile, type Session, type Achievement, type Game } from '$lib/api';
	import { formatDuration, timeAgo, formatDate } from '$lib/format';
	import { t } from '$lib/i18n';
	import Avatar from '$lib/components/Avatar.svelte';
	import StatusBadge from '$lib/components/StatusBadge.svelte';

	let profile = $state<Profile | null>(null);
	let sessions = $state<Session[]>([]);
	let nextCursor = $state<string | undefined>(undefined);
	let loading = $state(true);
	let error = $state('');
	let loadingMore = $state(false);

	let achievements = $state<Achievement[]>([]);
	let achGames = $state<{ game: Game; count: number }[]>([]);
	let achGameFilter = $state('');
	let achOffset = $state(0);
	let achHasMore = $state(false);
	let achLoading = $state(false);
	const ACH_LIMIT = 24;

	const id = $derived(page.params.id ?? '');
	let topSeconds = $derived(Math.max(1, ...(profile?.game_stats ?? []).map((g) => g.total_seconds)));

	onMount(load);

	async function load() {
		loading = true;
		error = '';
		try {
			profile = await api.getProfile(id);
			const res = await api.getSessions(id);
			sessions = res.sessions;
			nextCursor = res.next_cursor;
			await loadAchievements(true);
		} catch (e) {
			error = e instanceof Error ? e.message : t('profile.loadError');
		} finally {
			loading = false;
		}
	}

	async function loadAchievements(reset: boolean) {
		achLoading = true;
		try {
			const off = reset ? 0 : achOffset;
			const r = await api.getUserAchievements(id, { gameId: achGameFilter || undefined, offset: off, limit: ACH_LIMIT });
			achievements = reset ? r.achievements : [...achievements, ...r.achievements];
			achGames = r.games;
			achHasMore = r.has_more;
			achOffset = off + r.achievements.length;
		} finally {
			achLoading = false;
		}
	}

	async function loadMore() {
		if (!nextCursor) return;
		loadingMore = true;
		try {
			const res = await api.getSessions(id, nextCursor);
			sessions = [...sessions, ...res.sessions];
			nextCursor = res.next_cursor;
		} finally {
			loadingMore = false;
		}
	}
</script>

{#if loading}
	<p class="text-[var(--color-muted)]">{t('profile.loading')}</p>
{:else if error}
	<p class="text-[var(--color-magenta)]">{error}</p>
{:else if profile}
	<!-- Profile Header -->
	<div class="pd-card pd-glow mb-8 flex items-center gap-5 p-6">
		<div class="shrink-0">
			<Avatar username={profile.user.username} url={profile.user.avatar_url} size={80} />
		</div>
		<div class="min-w-0 flex-1">
			<div class="flex items-center gap-3">
				<h1 class="pd-heading text-3xl text-[var(--color-text)]">{profile.user.username}</h1>
				{#if profile.user.role === 'admin'}
					<span class="pd-cut-sm bg-[var(--color-brand)]/20 px-2 py-0.5 font-display text-xs font-bold italic uppercase tracking-widest text-[var(--color-brand-bright)]">admin</span>
				{/if}
			</div>
			<div class="mt-1.5 flex items-center gap-3 text-sm text-[var(--color-muted)]">
				<StatusBadge status={profile.presence.status} />
				<span>- {t('profile.memberSince')} {formatDate(profile.user.created_at)}</span>
			</div>
			{#if profile.presence.status === 'in_game' && profile.presence.game}
				<p class="mt-1 font-display text-sm font-bold italic text-[var(--color-brand-bright)]">
					{t('profile.playing')} {profile.presence.game.name}
				</p>
			{/if}
		</div>
		<div class="ml-auto flex shrink-0 gap-6 text-right">
			<div>
				<p class="font-display text-2xl font-bold italic text-[var(--color-brand-bright)]">{formatDuration(profile.total_seconds)}</p>
				<p class="text-xs uppercase tracking-wider text-[var(--color-muted)]">{t('profile.totalTracked')}</p>
			</div>
			{#if profile.achievement_count > 0}
				<div>
					<p class="font-display text-2xl font-bold italic text-[var(--color-gold)]">{profile.achievement_count}</p>
					<p class="text-xs uppercase tracking-wider text-[var(--color-muted)]">{t('profile.achievements', { count: profile.achievement_count })}</p>
				</div>
			{/if}
		</div>
	</div>

	{#if profile.connections.length > 0}
		<div class="mb-6 flex flex-wrap gap-2">
			{#each profile.connections as conn (conn.provider)}
				<a
					href={conn.profile_url ?? '#'}
					target="_blank"
					rel="noreferrer"
					class="btn-pd btn-pd-ghost pd-cut-sm inline-flex items-center gap-2 px-3 py-1.5 text-sm"
				>
					{#if conn.avatar_url}
						<img src={conn.avatar_url} alt="" class="h-5 w-5 rounded" />
					{/if}
					<span class="capitalize">{conn.provider}</span>
					{#if conn.display_name}
						<span class="text-[var(--color-muted)]">- {conn.display_name}</span>
					{/if}
				</a>
			{/each}
		</div>
	{/if}

	<!-- Hours per game -->
	<section class="mb-6">
		<h2 class="pd-heading mb-4 text-sm text-[var(--color-brand-bright)]">{t('profile.hoursPerGame')}</h2>
		{#if profile.game_stats.length === 0}
			<p class="text-sm text-[var(--color-muted)]">{t('profile.noSessionsYet')}</p>
		{:else}
			<div class="pd-card flex flex-col gap-1 p-4">
				{#each profile.game_stats.slice(0, 10) as stat, i (stat.game.id)}
					<a
						href="/games/{stat.game.slug}"
						class="flex items-center gap-3 px-2 py-1.5 transition-colors hover:bg-[var(--color-surface-2)]"
					>
						<span class="w-5 shrink-0 text-center font-display text-xs font-bold italic text-[var(--color-muted)]">{i + 1}</span>
						{#if stat.game.icon_url}
							<img src={stat.game.icon_url} alt="" class="h-6 w-6 shrink-0 rounded" />
						{/if}
						<span class="w-40 shrink-0 truncate text-sm">{stat.game.name}</span>
						<div class="h-2 flex-1 overflow-hidden bg-[var(--color-bg)]" style="clip-path: polygon(4px 0, 100% 0, calc(100% - 4px) 100%, 0 100%)">
							<div
								class="h-full bg-[var(--color-brand)]"
								style="width:{Math.max(2, (stat.total_seconds / topSeconds) * 100)}%; clip-path: polygon(4px 0, 100% 0, calc(100% - 4px) 100%, 0 100%)"
							></div>
						</div>
						<span class="w-20 shrink-0 text-right font-display text-sm font-bold italic text-[var(--color-text)]">
							{formatDuration(stat.total_seconds)}
						</span>
					</a>
				{/each}
			</div>
		{/if}
	</section>

	<!-- Achievements -->
	<section class="mb-6">
		<div class="mb-4 flex items-center gap-3">
			<h2 class="pd-heading text-sm text-[var(--color-gold)]">{t('profile.achievementsTitle')}</h2>
			<select
				bind:value={achGameFilter}
				onchange={() => loadAchievements(true)}
				class="pd-cut-sm ml-auto border border-[var(--color-border)] bg-[var(--color-bg)] px-2 py-1 text-xs text-[var(--color-text)]"
			>
				<option value="">{t('profile.allGames')}</option>
				{#each achGames as g (g.game.id)}
					<option value={g.game.id}>{g.game.name} ({g.count})</option>
				{/each}
			</select>
		</div>
		{#if achievements.length === 0 && !achLoading}
			<p class="text-sm text-[var(--color-muted)]">{t('profile.noAchievements')}</p>
		{:else}
			<div class="pd-card grid gap-3 p-4 sm:grid-cols-2">
				{#each achievements as a (a.game.id + a.api_name)}
					<div class="flex items-center gap-3 rounded bg-[var(--color-surface-2)] px-3 py-2">
						{#if a.icon_url}
							<img src={a.icon_url} alt="" class="h-9 w-9 shrink-0 pd-cut-sm object-cover" />
						{:else}
							<span class="grid h-9 w-9 shrink-0 pd-cut-sm place-items-center bg-[var(--color-bg)] font-display text-base text-[var(--color-gold)]">🏆</span>
						{/if}
						<div class="min-w-0">
							<p class="truncate text-sm font-display font-semibold">{a.display_name ?? a.api_name}</p>
							<p class="truncate text-xs text-[var(--color-muted)]">
								{a.game.name} - {timeAgo(a.unlocked_at)}
							</p>
						</div>
					</div>
				{/each}
			</div>
		{/if}
		{#if achHasMore}
			<button
				onclick={() => loadAchievements(false)}
				disabled={achLoading}
				class="btn-pd btn-pd-ghost mt-3 w-full py-2 text-sm disabled:opacity-60"
			>
				{achLoading ? t('profile.loadingMore') : t('profile.loadMore')}
			</button>
		{/if}
	</section>

	<!-- Recent sessions -->
	<section>
		<h2 class="pd-heading mb-4 text-sm text-[var(--color-cyan)]">{t('profile.recentSessions')}</h2>
		{#if sessions.length === 0}
			<p class="text-sm text-[var(--color-muted)]">{t('profile.noSessions')}</p>
		{:else}
			<div class="pd-card overflow-hidden">
				<ul class="divide-y divide-[var(--color-border)]">
					{#each sessions as s (s.id)}
						<li class="flex items-center gap-3 px-4 py-2.5 transition-colors hover:bg-[var(--color-surface-2)]">
							{#if s.game.icon_url}
								<img src={s.game.icon_url} alt="" class="h-5 w-5 shrink-0 rounded" />
							{/if}
							<span class="flex-1 truncate text-sm">{s.game.name}</span>
							{#if !s.ended_at}
								<span class="pd-cut-sm bg-[var(--color-online)]/15 px-2 py-0.5 font-display text-xs font-bold italic uppercase text-[var(--color-online)]">live</span>
							{:else}
								<span class="font-display text-sm font-bold italic text-[var(--color-text)]">{formatDuration(s.duration_seconds ?? 0)}</span>
							{/if}
							<span class="w-20 text-right text-xs text-[var(--color-muted)]">{timeAgo(s.started_at)}</span>
						</li>
					{/each}
				</ul>
			</div>
			{#if nextCursor}
				<button
					onclick={loadMore}
					disabled={loadingMore}
					class="btn-pd btn-pd-ghost mt-3 w-full py-2 text-sm disabled:opacity-60"
				>
					{loadingMore ? t('profile.loadingMore') : t('profile.loadMore')}
				</button>
			{/if}
		{/if}
	</section>
{/if}
