<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { api, type Profile, type Session, type Achievement, type Game } from '$lib/api';
	import { formatDuration, timeAgo, formatDate } from '$lib/format';
	import { t, getLocale } from '$lib/i18n';
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

	let libraryOpen = $state(false);
	let library = $state<{ game: Game; source: string }[]>([]);
	let libraryLoading = $state(false);

	const id = $derived(page.params.id ?? '');
	let topSeconds = $derived(Math.max(1, ...(profile?.game_stats ?? []).map((g) => g.total_seconds)));

	// Recent sessions grouped by local calendar day (the viewer's own timezone,
	// straight from the browser). Sessions arrive newest-first, so consecutive
	// runs of the same day stay contiguous and ordered.
	type SessionGroup = { key: string; iso: string; items: Session[] };
	const sessionGroups = $derived.by((): SessionGroup[] => {
		const groups: SessionGroup[] = [];
		let current: SessionGroup | null = null;
		for (const s of sessions) {
			const d = new Date(s.started_at);
			const key = `${d.getFullYear()}-${d.getMonth()}-${d.getDate()}`;
			if (!current || current.key !== key) {
				current = { key, iso: s.started_at, items: [] };
				groups.push(current);
			}
			current.items.push(s);
		}
		return groups;
	});

	const startOfLocalDay = (d: Date) => new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime();
	function dayLabel(iso: string): string {
		const d = new Date(iso);
		const diffDays = Math.round((startOfLocalDay(new Date()) - startOfLocalDay(d)) / 86400000);
		if (diffDays === 0) return t('profile.today');
		if (diffDays === 1) return t('profile.yesterday');
		return d.toLocaleDateString(getLocale(), { weekday: 'long', day: 'numeric', month: 'long' });
	}
	function timeOfDay(iso: string): string {
		return new Date(iso).toLocaleTimeString(getLocale(), { hour: '2-digit', minute: '2-digit' });
	}

	const PROVIDER_META: Record<string, { label: string; color: string; path: string }> = {
		steam: {
			label: 'Steam',
			color: '#66c0f4',
			path: 'M11.979 0C5.678 0 .511 4.86.022 11.037l6.432 2.658c.545-.371 1.203-.59 1.912-.59.063 0 .125.004.188.006l2.861-4.142V8.91c0-2.495 2.028-4.524 4.524-4.524 2.494 0 4.524 2.031 4.524 4.527s-2.03 4.525-4.524 4.525h-.105l-4.076 2.911c0 .052.004.105.004.159 0 1.875-1.515 3.396-3.39 3.396-1.635 0-3.016-1.173-3.331-2.727L.436 15.27C1.862 20.307 6.486 24 11.979 24c6.627 0 11.999-5.373 11.999-12S18.605 0 11.979 0zM7.54 18.21l-1.473-.61c.262.543.714.999 1.314 1.25 1.297.539 2.793-.076 3.332-1.375.263-.63.264-1.319.005-1.949s-.75-1.121-1.377-1.383c-.624-.26-1.29-.249-1.878-.03l1.523.63c.956.4 1.409 1.5 1.009 2.455-.397.957-1.497 1.41-2.454 1.012H7.54zm11.415-9.303c0-1.662-1.353-3.015-3.015-3.015-1.665 0-3.015 1.353-3.015 3.015 0 1.665 1.35 3.015 3.015 3.015 1.663 0 3.015-1.35 3.015-3.015zm-5.273-.005c0-1.252 1.013-2.266 2.265-2.266 1.249 0 2.266 1.014 2.266 2.266 0 1.251-1.017 2.265-2.266 2.265-1.253 0-2.265-1.014-2.265-2.265z'
		},
		battlenet: {
			label: 'Battle.net',
			color: '#148EFF',
			path: 'M18.94 8.296C15.9 6.892 11.534 6 7.426 6.332c.206-1.36.714-2.308 1.548-2.508 1.148-.275 2.4.48 3.594 1.854.782.102 1.71.28 2.355.429C12.747 2.013 9.828-.282 7.607.565c-1.688.644-2.553 2.97-2.448 6.094-2.2.468-3.915 1.3-5.013 2.495-.056.065-.181.227-.137.305.034.058.146-.008.194-.04 1.274-.89 2.904-1.373 5.027-1.676.303 3.333 1.713 7.56 4.055 10.952-1.28.502-2.356.536-2.946-.087-.812-.856-.784-2.318-.19-4.04a26.764 26.764 0 0 1-.807-2.254c-2.459 3.934-2.986 7.61-1.143 9.11 1.402 1.14 3.847.725 6.502-.926 1.505 1.672 3.083 2.74 4.667 3.094.084.015.287.043.332-.034.034-.06-.08-.124-.131-.149-1.408-.657-2.64-1.828-3.964-3.515 2.735-1.929 5.691-5.263 7.457-8.988 1.076.86 1.64 1.773 1.398 2.595-.336 1.131-1.615 1.84-3.403 2.185a27.697 27.697 0 0 1-1.548 1.826c4.634.16 8.08-1.22 8.458-3.565.286-1.786-1.295-3.696-4.053-5.17.696-2.139.832-4.04.346-5.588-.029-.08-.106-.27-.196-.27-.068 0-.067.13-.063.187.135 1.547-.263 3.2-1.062 5.19zm-8.533 9.869c-1.96-3.145-3.09-6.849-3.082-10.594 3.702-.124 7.474.748 10.714 2.627-1.743 3.269-4.385 6.1-7.633 7.966h.001z'
		},
		xbox: {
			label: 'Xbox',
			color: '#107C10',
			path: 'M4.102 21.033C6.211 22.881 8.977 24 12 24c3.026 0 5.789-1.119 7.902-2.967 1.877-1.912-4.316-8.709-7.902-11.417-3.582 2.708-9.779 9.505-7.898 11.417zm11.16-14.406c2.5 2.961 7.484 10.313 6.076 12.912C23.002 17.48 24 14.861 24 12.004c0-3.34-1.365-6.362-3.57-8.536 0 0-.027-.022-.082-.042-.063-.022-.152-.045-.281-.045-.592 0-1.985.434-4.805 3.246zM3.654 3.426c-.057.02-.082.041-.086.042C1.365 5.642 0 8.664 0 12.004c0 2.854.998 5.473 2.661 7.533-1.401-2.605 3.579-9.951 6.08-12.91-2.82-2.813-4.216-3.245-4.806-3.245-.131 0-.223.021-.281.046v-.002zM12 3.551S9.055 1.828 6.755 1.746c-.903-.033-1.454.295-1.521.339C7.379.646 9.659 0 11.984 0H12c2.334 0 4.605.646 6.766 2.085-.068-.046-.615-.372-1.52-.339C14.946 1.828 12 3.545 12 3.545v.006z'
		}
	};

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

	async function toggleLibrary() {
		if (libraryOpen) {
			libraryOpen = false;
			return;
		}
		libraryOpen = true;
		if (library.length > 0) return; // already loaded
		libraryLoading = true;
		try {
			const res = await api.userGames(id);
			library = res.games;
		} finally {
			libraryLoading = false;
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
				{@const meta = PROVIDER_META[conn.provider]}
				{#if conn.profile_url}
					<a
						href={conn.profile_url}
						target="_blank"
						rel="noreferrer noopener"
						class="btn-pd btn-pd-ghost pd-cut-sm inline-flex items-center gap-2 px-3 py-1.5 text-sm"
					>
						{#if meta}
							<svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true" class="h-5 w-5 shrink-0" style="color: {meta.color}"><path d={meta.path} /></svg>
						{/if}
						<span class="capitalize">{meta?.label ?? conn.provider}</span>
						{#if conn.display_name}
							<span class="text-[var(--color-muted)]">- {conn.display_name}</span>
						{/if}
					</a>
				{:else}
					<div class="pd-cut-sm inline-flex cursor-default items-center gap-2 border border-[var(--color-border)] px-3 py-1.5 text-sm">
						{#if meta}
							<svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true" class="h-5 w-5 shrink-0" style="color: {meta.color}"><path d={meta.path} /></svg>
						{/if}
						<span class="capitalize">{meta?.label ?? conn.provider}</span>
						{#if conn.display_name}
							<span class="text-[var(--color-muted)]">- {conn.display_name}</span>
						{/if}
					</div>
				{/if}
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

	<!-- Full owned game library -->
	<section class="mb-6">
		<button
			onclick={toggleLibrary}
			class="btn-pd btn-pd-ghost mb-4 flex w-full items-center justify-between px-4 py-2.5 text-left"
		>
			<span class="pd-heading text-sm text-[var(--color-brand-bright)]">{t('profile.allOwnedGames')}</span>
			<svg
				viewBox="0 0 24 24"
				fill="none"
				stroke="currentColor"
				stroke-width="2"
				class="h-4 w-4 shrink-0 text-[var(--color-muted)] transition-transform {libraryOpen ? 'rotate-180' : ''}"
				aria-hidden="true"
			>
				<path stroke-linecap="round" stroke-linejoin="round" d="M19 9l-7 7-7-7" />
			</svg>
		</button>
		{#if libraryOpen}
			{#if libraryLoading}
				<p class="text-sm text-[var(--color-muted)]">{t('common.loading')}</p>
			{:else if library.length === 0}
				<p class="text-sm text-[var(--color-muted)]">{t('profile.noSessionsYet')}</p>
			{:else}
				<div class="pd-card grid gap-2 p-4 sm:grid-cols-2 lg:grid-cols-3">
					{#each library as entry (entry.game.id)}
						{@const badge =
							entry.source === 'steam'
								? { label: 'Steam', color: '#66c0f4', bg: 'rgba(102,192,244,0.15)' }
								: entry.source === 'battlenet'
									? { label: 'Battle.net', color: '#148EFF', bg: 'rgba(20,142,255,0.15)' }
									: { label: t('profile.playedBadge'), color: 'var(--color-muted)', bg: 'var(--color-surface-2)' }}
						<a
							href="/players/{id}/games/{entry.game.slug}"
							class="flex items-center gap-3 rounded px-2 py-1.5 transition-colors hover:bg-[var(--color-surface-2)]"
						>
							{#if entry.game.icon_url}
								<img src={entry.game.icon_url} alt="" class="h-7 w-7 shrink-0 rounded" />
							{:else}
								<span class="grid h-7 w-7 shrink-0 place-items-center rounded bg-[var(--color-bg)] text-[var(--color-muted)]">
									<svg viewBox="0 0 24 24" fill="currentColor" class="h-4 w-4" aria-hidden="true"><path d="M21 6H3a1 1 0 0 0-1 1v10a1 1 0 0 0 1 1h18a1 1 0 0 0 1-1V7a1 1 0 0 0-1-1zm-1 10H4V8h16v8zm-8-6a2 2 0 1 0 0 4 2 2 0 0 0 0-4zm-5 2a1 1 0 1 0 0 2 1 1 0 0 0 0-2zm10 0a1 1 0 1 0 0 2 1 1 0 0 0 0-2z"/></svg>
								</span>
							{/if}
							<span class="min-w-0 flex-1 truncate text-sm">{entry.game.name}</span>
							<span
								class="shrink-0 pd-cut-sm px-1.5 py-0.5 font-display text-xs font-bold italic"
								style="background: {badge.bg}; color: {badge.color}"
							>{badge.label}</span>
						</a>
					{/each}
				</div>
			{/if}
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
				{#each sessionGroups as group (group.key)}
					<div class="bg-[var(--color-surface-2)] px-4 py-1.5 font-display text-xs font-bold uppercase tracking-wide text-[var(--color-muted)]">
						{dayLabel(group.iso)}
					</div>
					<ul class="divide-y divide-[var(--color-border)]">
						{#each group.items as s (s.id)}
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
								<span class="w-12 text-right text-xs text-[var(--color-muted)]">{timeOfDay(s.started_at)}</span>
							</li>
						{/each}
					</ul>
				{/each}
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
