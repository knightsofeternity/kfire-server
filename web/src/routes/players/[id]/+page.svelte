<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { api, type Profile, type Session } from '$lib/api';
	import { formatDuration, timeAgo, formatDate } from '$lib/format';
	import Avatar from '$lib/components/Avatar.svelte';
	import StatusBadge from '$lib/components/StatusBadge.svelte';

	let profile = $state<Profile | null>(null);
	let sessions = $state<Session[]>([]);
	let nextCursor = $state<string | undefined>(undefined);
	let loading = $state(true);
	let error = $state('');
	let loadingMore = $state(false);

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
		} catch (e) {
			error = e instanceof Error ? e.message : 'failed to load profile';
		} finally {
			loading = false;
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
	<p class="text-[var(--color-muted)]">Loading…</p>
{:else if error}
	<p class="text-red-500">{error}</p>
{:else if profile}
	<!-- Header -->
	<div class="mb-6 flex items-center gap-4">
		<Avatar username={profile.user.username} url={profile.user.avatar_url} size={72} />
		<div>
			<div class="flex items-center gap-3">
				<h1 class="text-2xl font-bold">{profile.user.username}</h1>
				{#if profile.user.role === 'admin'}
					<span class="rounded bg-[var(--color-brand)]/15 px-2 py-0.5 text-xs text-[var(--color-brand)]">admin</span>
				{/if}
			</div>
			<div class="mt-1 flex items-center gap-3 text-sm text-[var(--color-muted)]">
				<StatusBadge status={profile.presence.status} />
				<span>· member since {formatDate(profile.user.created_at)}</span>
			</div>
			{#if profile.presence.status === 'in_game' && profile.presence.game}
				<p class="mt-1 text-sm text-[var(--color-brand)]">Playing {profile.presence.game.name}</p>
			{/if}
		</div>
		<div class="ml-auto text-right">
			<p class="text-2xl font-bold">{formatDuration(profile.total_seconds)}</p>
			<p class="text-xs text-[var(--color-muted)]">total tracked</p>
		</div>
	</div>

	<!-- Hours per game -->
	<section class="mb-6">
		<h2 class="mb-3 text-sm font-semibold tracking-wide text-[var(--color-muted)] uppercase">
			Hours per game
		</h2>
		{#if profile.game_stats.length === 0}
			<p class="text-sm text-[var(--color-muted)]">No sessions recorded yet.</p>
		{:else}
			<div class="flex flex-col gap-2 rounded-xl border border-[var(--color-border)] bg-[var(--color-surface)] p-4">
				{#each profile.game_stats.slice(0, 10) as stat (stat.game.id)}
					<div class="flex items-center gap-3">
						{#if stat.game.icon_url}
							<img src={stat.game.icon_url} alt="" class="h-6 w-6 shrink-0 rounded" />
						{/if}
						<span class="w-40 shrink-0 truncate text-sm">{stat.game.name}</span>
						<div class="h-2.5 flex-1 overflow-hidden rounded-full bg-[var(--color-bg)]">
							<div
								class="h-full rounded-full bg-[var(--color-brand)]"
								style="width:{Math.max(2, (stat.total_seconds / topSeconds) * 100)}%"
							></div>
						</div>
						<span class="w-20 shrink-0 text-right text-sm text-[var(--color-muted)]">
							{formatDuration(stat.total_seconds)}
						</span>
					</div>
				{/each}
			</div>
		{/if}
	</section>

	<!-- Recent sessions -->
	<section>
		<h2 class="mb-3 text-sm font-semibold tracking-wide text-[var(--color-muted)] uppercase">
			Recent sessions
		</h2>
		{#if sessions.length === 0}
			<p class="text-sm text-[var(--color-muted)]">No sessions yet.</p>
		{:else}
			<ul class="divide-y divide-[var(--color-border)] overflow-hidden rounded-xl border border-[var(--color-border)] bg-[var(--color-surface)]">
				{#each sessions as s (s.id)}
					<li class="flex items-center gap-3 px-4 py-2.5">
						{#if s.game.icon_url}
							<img src={s.game.icon_url} alt="" class="h-5 w-5 rounded" />
						{/if}
						<span class="flex-1 truncate text-sm">{s.game.name}</span>
						{#if !s.ended_at}
							<span class="text-xs text-[var(--color-online)]">in progress</span>
						{:else}
							<span class="text-sm text-[var(--color-muted)]">{formatDuration(s.duration_seconds ?? 0)}</span>
						{/if}
						<span class="w-20 text-right text-xs text-[var(--color-muted)]">{timeAgo(s.started_at)}</span>
					</li>
				{/each}
			</ul>
			{#if nextCursor}
				<button
					onclick={loadMore}
					disabled={loadingMore}
					class="mt-3 w-full rounded-lg border border-[var(--color-border)] py-2 text-sm text-[var(--color-muted)] hover:text-[var(--color-text)] disabled:opacity-60"
				>
					{loadingMore ? 'Loading…' : 'Load more'}
				</button>
			{/if}
		{/if}
	</section>
{/if}
