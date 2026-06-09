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
	<p class="text-[var(--color-muted)]">Loading...</p>
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
				<p class="text-xs text-[var(--color-muted)] uppercase tracking-wide">Total played in org</p>
			</div>
			<div>
				<p class="font-display text-xl font-bold text-[var(--color-cyan)]">{detail.player_count}</p>
				<p class="text-xs text-[var(--color-muted)] uppercase tracking-wide">{detail.player_count === 1 ? 'Player' : 'Players'}</p>
			</div>
		</div>
	</div>

	<!-- Leaderboard -->
	<h2 class="pd-heading mb-4 flex items-center gap-2 text-sm text-[var(--color-brand-bright)]">
		<span class="inline-block h-4 w-1 bg-[var(--color-brand)]"></span>
		Leaderboard
	</h2>
	{#if detail.leaderboard.length === 0}
		<p class="text-sm text-[var(--color-muted)]">Nobody has played this yet.</p>
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
{/if}
