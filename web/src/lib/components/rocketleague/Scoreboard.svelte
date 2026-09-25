<script module lang="ts">
	import type { RlScoreboard } from '$lib/api';

	// One request per match per page load: folding and unfolding a line does not
	// fetch again. A failed request is forgotten so the next unfold retries.
	const cache = new Map<string, Promise<RlScoreboard>>();
</script>

<script lang="ts">
	import { api, type RlBoardRow } from '$lib/api';
	import Avatar from '$lib/components/Avatar.svelte';
	import { t } from '$lib/i18n';

	let { matchId }: { matchId: string } = $props();

	const board = $derived.by(() => {
		let p = cache.get(matchId);
		if (!p) {
			p = api.getRlScoreboard(matchId);
			cache.set(matchId, p);
			p.catch(() => cache.delete(matchId));
		}
		return p;
	});

	/** "Teammate 2" on the reference member's side, "Opponent 2" on the other. */
	function anonLabel(row: RlBoardRow, team: number, reference: number): string {
		return t(team === reference ? 'rlBoard.teammate' : 'rlBoard.opponent', {
			n: row.anon_index ?? 0
		});
	}

	const cell = 'px-2 py-1.5 text-right tabular-nums';
	const head = 'px-2 py-1 text-right text-[10px] uppercase tracking-wide text-[var(--color-muted)]';
</script>

{#await board}
	<p class="px-3 py-2 text-xs text-[var(--color-muted)]">{t('rlBoard.loading')}</p>
{:then b}
	<div class="grid gap-3 p-3 md:grid-cols-2">
		{#each b.teams as team (team.team)}
			{@const color = team.team === 0 ? 'var(--color-blue)' : 'var(--color-brand-bright)'}
			<section class="pd-card overflow-hidden border-t-2" style="border-color: {color}">
				<header class="flex items-center justify-between px-3 py-2">
					<div>
						{#if team.winner}
							<p class="font-display text-[10px] uppercase tracking-wide text-[var(--color-gold)]">
								{t('rlBoard.winner')}
							</p>
						{/if}
						<p class="font-display text-sm font-bold" style="color: {color}">
							{team.team === 0 ? t('rlBoard.blue') : t('rlBoard.orange')}
						</p>
					</div>
					<span class="font-display text-2xl font-bold tabular-nums">{team.score}</span>
				</header>
				<div class="overflow-x-auto">
					<table class="w-full min-w-[360px] border-collapse text-sm">
						<thead>
							<tr class="border-b border-[var(--color-border)]">
								<th class="{head} text-left">{t('rlBoard.player')}</th>
								<th class={head}>{t('rlBoard.score')}</th>
								<th class={head}>{t('rlBoard.goals')}</th>
								<th class={head}>{t('rlBoard.assists')}</th>
								<th class={head}>{t('rlBoard.saves')}</th>
								<th class={head}>{t('rlBoard.shots')}</th>
							</tr>
						</thead>
						<tbody>
							{#each team.rows as row, i (i)}
								<tr class="border-b border-[var(--color-border)]/50 last:border-b-0">
									<td class="px-2 py-1.5">
										<span class="flex items-center gap-2">
											{#if row.user_id}
												<a href="/players/{row.user_id}" class="flex items-center gap-2 hover:underline">
													<Avatar username={row.username ?? ''} url={row.avatar_url} size={20} />
													<span class="font-semibold text-[var(--color-text)]">{row.username}</span>
												</a>
											{:else}
												<span class="text-[var(--color-muted)] italic">
													{anonLabel(row, team.team, b.reference_team)}
												</span>
											{/if}
											{#if row.mvp}
												<span class="font-display text-[10px] font-bold uppercase text-[var(--color-gold)]">
													{t('rlBoard.mvp')}
												</span>
											{/if}
											{#if row.left}
												<span class="text-[10px] text-[var(--color-muted)]">{t('rlBoard.left')}</span>
											{/if}
										</span>
									</td>
									<td class="{cell} font-semibold">{row.score}</td>
									<td class={cell}>{row.goals}</td>
									<td class={cell}>{row.assists}</td>
									<td class={cell}>{row.saves}</td>
									<td class={cell}>{row.shots}</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>
			</section>
		{/each}
	</div>
{:catch}
	<p class="px-3 py-2 text-xs text-[var(--color-magenta)]">{t('rlBoard.error')}</p>
{/await}
