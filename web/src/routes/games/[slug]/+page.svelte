<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { api, type GameDetail } from '$lib/api';
	import { formatDuration } from '$lib/format';
	import Avatar from '$lib/components/Avatar.svelte';

	let detail = $state<GameDetail | null>(null);
	let loading = $state(true);
	let error = $state('');

	const slug = $derived(page.params.slug ?? '');
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
			error = e instanceof Error ? e.message : 'failed to load game';
		} finally {
			loading = false;
		}
	}
</script>

{#if loading}
	<p class="text-[var(--color-muted)]">Loading…</p>
{:else if error}
	<p class="text-red-500">{error}</p>
{:else if detail}
	<!-- Cover banner -->
	<div class="mb-6 overflow-hidden rounded-xl border border-[var(--color-border)] bg-[var(--color-surface)]">
		{#if detail.game.cover_url}
			<div class="relative h-44 sm:h-56">
				<img src={detail.game.cover_url} alt="" class="h-full w-full object-cover" />
				<div class="absolute inset-0 bg-gradient-to-t from-[var(--color-bg)] to-transparent"></div>
				<div class="absolute bottom-0 left-0 flex items-center gap-3 p-4">
					{#if detail.game.icon_url}
						<img src={detail.game.icon_url} alt="" class="h-12 w-12 rounded-lg shadow-lg" />
					{/if}
					<h1 class="text-2xl font-bold drop-shadow">{detail.game.name}</h1>
				</div>
			</div>
		{:else}
			<div class="flex items-center gap-3 p-5">
				{#if detail.game.icon_url}
					<img src={detail.game.icon_url} alt="" class="h-12 w-12 rounded-lg" />
				{/if}
				<h1 class="text-2xl font-bold">{detail.game.name}</h1>
			</div>
		{/if}
		<div class="flex gap-8 px-5 py-4">
			<div>
				<p class="text-xl font-bold">{formatDuration(detail.total_seconds)}</p>
				<p class="text-xs text-[var(--color-muted)]">total played in the org</p>
			</div>
			<div>
				<p class="text-xl font-bold">{detail.player_count}</p>
				<p class="text-xs text-[var(--color-muted)]">{detail.player_count === 1 ? 'player' : 'players'}</p>
			</div>
		</div>
	</div>

	<!-- Leaderboard -->
	<h2 class="mb-3 text-sm font-semibold tracking-wide text-[var(--color-muted)] uppercase">
		Leaderboard
	</h2>
	{#if detail.leaderboard.length === 0}
		<p class="text-sm text-[var(--color-muted)]">Nobody has played this yet.</p>
	{:else}
		<ul class="flex flex-col gap-2">
			{#each detail.leaderboard as e, i (e.user_id)}
				<a
					href="/players/{e.user_id}"
					class="flex items-center gap-3 rounded-xl border border-[var(--color-border)] bg-[var(--color-surface)] p-3 transition-colors hover:border-[var(--color-brand)]"
				>
					<span class="w-6 text-center text-sm font-bold {i < 3 ? 'text-[var(--color-brand)]' : 'text-[var(--color-muted)]'}">
						{i + 1}
					</span>
					<Avatar username={e.username} url={e.avatar_url} size={36} />
					<span class="flex-1 truncate font-medium">{e.username}</span>
					<div class="hidden h-2 w-32 overflow-hidden rounded-full bg-[var(--color-bg)] sm:block">
						<div
							class="h-full rounded-full bg-[var(--color-brand)]"
							style="width:{Math.max(3, (e.total_seconds / topSeconds) * 100)}%"
						></div>
					</div>
					<span class="w-20 text-right text-sm text-[var(--color-muted)]">
						{formatDuration(e.total_seconds)}
					</span>
				</a>
			{/each}
		</ul>
	{/if}
{/if}
