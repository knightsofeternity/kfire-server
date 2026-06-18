<script lang="ts">
	import { onMount } from 'svelte';
	import { api, type WeeklyLeaderboards } from '$lib/api';
	import { formatDuration } from '$lib/format';
	import Avatar from '$lib/components/Avatar.svelte';
	import { t } from '$lib/i18n';

	let data = $state<WeeklyLeaderboards | null>(null);
	let loading = $state(true);

	onMount(async () => {
		try {
			data = await api.getWeeklyLeaderboards();
		} finally {
			loading = false;
		}
	});
</script>

<h1 class="pd-heading mb-1 text-2xl text-[var(--color-brand-bright)]">{t('leaderboards.title')}</h1>
<p class="mb-6 text-sm text-[var(--color-muted)]">{t('leaderboards.subtitle')}</p>

{#if loading}
	<p class="text-[var(--color-muted)]">{t('leaderboards.loading')}</p>
{:else if data}
	<div class="grid gap-6 lg:grid-cols-2">
		<!-- Top players -->
		<section>
			<h2 class="font-display mb-3 text-lg font-bold text-[var(--color-text)]">
				{t('leaderboards.topPlayers')}
			</h2>
			{#if data.top_players.length === 0}
				<p class="text-[var(--color-muted)]">{t('leaderboards.empty')}</p>
			{:else}
				<ul class="flex flex-col gap-2">
					{#each data.top_players as p, i (p.user_id)}
						<li>
							<a
								href={`/players/${p.user_id}`}
								class="pd-card group flex items-center gap-3 p-3 transition-colors hover:border-[var(--color-brand)]"
							>
								<span class="w-6 text-center font-display font-bold text-[var(--color-muted)]">
									{i + 1}
								</span>
								<Avatar username={p.username} url={p.avatar_url} size={36} />
								<span
									class="min-w-0 flex-1 truncate font-display font-bold text-[var(--color-text)] group-hover:text-[var(--color-brand-bright)]"
								>
									{p.username}
								</span>
								<span class="font-display text-sm font-bold text-[var(--color-brand-bright)]">
									{formatDuration(p.total_seconds)}
								</span>
							</a>
						</li>
					{/each}
				</ul>
			{/if}
		</section>

		<!-- Top games -->
		<section>
			<h2 class="font-display mb-3 text-lg font-bold text-[var(--color-text)]">
				{t('leaderboards.topGames')}
			</h2>
			{#if data.top_games.length === 0}
				<p class="text-[var(--color-muted)]">{t('leaderboards.empty')}</p>
			{:else}
				<ul class="flex flex-col gap-2">
					{#each data.top_games as row, i (row.game.id)}
						<li>
							<a
								href={`/games/${row.game.slug}`}
								class="pd-card group flex items-center gap-3 p-3 transition-colors hover:border-[var(--color-brand)]"
							>
								<span class="w-6 text-center font-display font-bold text-[var(--color-muted)]">
									{i + 1}
								</span>
								{#if row.game.icon_url}
									<img src={row.game.icon_url} alt="" class="pd-cut-sm h-10 w-10 shrink-0 object-cover" />
								{:else}
									<span
										class="pd-cut-sm grid h-10 w-10 shrink-0 place-items-center bg-[var(--color-surface-2)]"
										>🎮</span
									>
								{/if}
								<div class="min-w-0 flex-1">
									<p
										class="truncate font-display font-bold text-[var(--color-text)] group-hover:text-[var(--color-brand-bright)]"
									>
										{row.game.name}
									</p>
									<p class="text-xs text-[var(--color-muted)]">
										{row.player_count === 1
											? t('leaderboards.onePlayer')
											: t('leaderboards.players', { count: row.player_count })}
									</p>
								</div>
								<span class="font-display text-sm font-bold text-[var(--color-brand-bright)]">
									{formatDuration(row.total_seconds)}
								</span>
							</a>
						</li>
					{/each}
				</ul>
			{/if}
		</section>
	</div>
{/if}
