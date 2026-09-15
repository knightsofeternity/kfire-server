<script lang="ts">
	import { onMount } from 'svelte';
	import { api, type PlayedGame } from '$lib/api';
	import { formatDuration } from '$lib/format';
	import { t } from '$lib/i18n';

	type SortKey = 'players' | 'hours' | 'name';

	const SORTS: Record<SortKey, (a: PlayedGame, b: PlayedGame) => number> = {
		// The default, and it stays the default: what the guild plays TOGETHER
		// is what the page is for. Pure hours puts a niche game nobody shares
		// at the top, which reads as a ranking and is not one.
		players: (a, b) => b.player_count - a.player_count || b.total_seconds - a.total_seconds,
		// Precisely for the niche games the default buries.
		hours: (a, b) => b.total_seconds - a.total_seconds || b.player_count - a.player_count,
		name: (a, b) => a.name.localeCompare(b.name)
	};

	const STORAGE_KEY = 'kfire-games-sort';

	let games = $state<PlayedGame[]>([]);
	let loading = $state(true);
	let query = $state('');
	let sort = $state<SortKey>('players');

	let filtered = $derived(
		(query.trim()
			? games.filter((g) => g.name.toLowerCase().includes(query.trim().toLowerCase()))
			: games
		)
			.slice()
			.sort(SORTS[sort])
	);

	/** Remembers the choice for this browser only; it is a convenience, not state. */
	function choose(next: SortKey) {
		sort = next;
		try {
			localStorage.setItem(STORAGE_KEY, next);
		} catch {
			/* storage blocked; the choice holds for this visit */
		}
	}

	onMount(async () => {
		try {
			const saved = localStorage.getItem(STORAGE_KEY);
			if (saved === 'players' || saved === 'hours' || saved === 'name') sort = saved;
		} catch {
			/* storage blocked; the default stands */
		}
		try {
			games = await api.getPlayedGames();
		} finally {
			loading = false;
		}
	});
</script>

<h1 class="pd-heading mb-5 text-2xl text-[var(--color-brand-bright)]">{t('gamesList.title')}</h1>

<div class="mb-6 flex flex-wrap items-center gap-3">
	<input
		type="search"
		bind:value={query}
		placeholder={t('gamesList.search')}
		class="pd-cut-sm w-full max-w-md border border-[var(--color-border)] bg-[var(--color-bg)] px-3 py-2 text-sm text-[var(--color-text)] outline-none focus:border-[var(--color-brand)]"
	/>

	<div class="flex items-center gap-2">
		<span class="text-xs uppercase tracking-wide text-[var(--color-muted)]">
			{t('gamesList.sortBy')}
		</span>
		<div class="flex" role="group" aria-label={t('gamesList.sortBy')}>
			{#each [['players', 'sortPlayers'], ['hours', 'sortHours'], ['name', 'sortName']] as const as [key, label] (key)}
				<button
					type="button"
					aria-pressed={sort === key}
					onclick={() => choose(key)}
					class="border border-[var(--color-border)] px-3 py-1.5 text-xs transition-colors first:rounded-l last:rounded-r
						{sort === key
						? 'border-[var(--color-brand)] bg-[var(--color-brand)] font-semibold text-white'
						: 'text-[var(--color-muted)] hover:text-[var(--color-text)]'}"
				>
					{t(`gamesList.${label}`)}
				</button>
			{/each}
		</div>
	</div>
</div>

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
