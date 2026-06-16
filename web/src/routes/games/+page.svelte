<script lang="ts">
	import { onMount } from 'svelte';
	import { api, type PlayedGame } from '$lib/api';
	import { formatDuration } from '$lib/format';
	import { t } from '$lib/i18n';

	let games = $state<PlayedGame[]>([]);
	let loading = $state(true);
	let query = $state('');

	// Sort by player count (most-played-in-common first), then by total hours.
	let filtered = $derived(
		(query.trim()
			? games.filter((g) => g.name.toLowerCase().includes(query.trim().toLowerCase()))
			: games
		)
			.slice()
			.sort((a, b) => b.player_count - a.player_count || b.total_seconds - a.total_seconds)
	);

	onMount(async () => {
		try {
			games = await api.getPlayedGames();
		} finally {
			loading = false;
		}
	});
</script>

<h1 class="pd-heading mb-5 text-2xl text-[var(--color-brand-bright)]">{t('gamesList.title')}</h1>

<input
	type="search"
	bind:value={query}
	placeholder={t('gamesList.search')}
	class="pd-cut-sm mb-6 w-full max-w-md border border-[var(--color-border)] bg-[var(--color-bg)] px-3 py-2 text-sm text-[var(--color-text)] outline-none focus:border-[var(--color-brand)]"
/>

{#if loading}
	<p class="text-[var(--color-muted)]">{t('gamesList.loading')}</p>
{:else if games.length === 0}
	<p class="text-[var(--color-muted)]">{t('gamesList.empty')}</p>
{:else if filtered.length === 0}
	<p class="text-[var(--color-muted)]">{t('gamesList.noMatch')}</p>
{:else}
	<ul class="grid gap-3 sm:grid-cols-2">
		{#each filtered as g (g.id)}
			<li>
				<a
					href={`/games/${g.slug}`}
					class="pd-card group flex items-center gap-3 p-3 transition-colors hover:border-[var(--color-brand)]"
				>
					{#if g.icon_url}
						<img src={g.icon_url} alt="" class="pd-cut-sm h-12 w-12 shrink-0 object-cover" />
					{:else}
						<span
							class="pd-cut-sm grid h-12 w-12 shrink-0 place-items-center bg-[var(--color-surface-2)] text-lg"
							>🎮</span
						>
					{/if}
					<div class="min-w-0 flex-1">
						<p
							class="font-display truncate font-bold text-[var(--color-text)] group-hover:text-[var(--color-brand-bright)]"
						>
							{g.name}
						</p>
						<p class="text-xs text-[var(--color-muted)]">
							{g.player_count === 1
								? t('gamesList.onePlayer')
								: t('gamesList.players', { count: g.player_count })}
						</p>
					</div>
					<span class="font-display text-sm font-bold text-[var(--color-brand-bright)]">
						{formatDuration(g.total_seconds)}
					</span>
				</a>
			</li>
		{/each}
	</ul>
{/if}
