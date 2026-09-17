<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { get } from 'svelte/store';
	import { api, type PresenceEntry } from '$lib/api';
	import { auth } from '$lib/stores/auth.svelte';
	import { connectPresence, type PresenceSocket, type LiveMatch } from '$lib/ws';
	import { formatClock } from '$lib/format';
	import Avatar from '$lib/components/Avatar.svelte';
	import StatusBadge from '$lib/components/StatusBadge.svelte';
	import { t } from '$lib/i18n';

	// Keyed by user_id so live presence_update events overwrite the right row.
	// Without the live subscription the list froze at page-load state: a member
	// who went offline still showed online until a manual reload.
	let entries = $state<Map<string, PresenceEntry>>(new Map());
	// Keyed by user_id, same reasoning: a live_match event overwrites this
	// member's score, and a `null` match removes him from the map so the row
	// shows nothing again. Never seeded from the initial snapshot: there is no
	// REST endpoint for it, by design, so a page opened mid-match simply picks
	// up the next sample within about half a second.
	let liveMatches = $state<Map<string, LiveMatch>>(new Map());
	let query = $state('');
	let loading = $state(true);
	let socket: PresenceSocket | null = null;

	let filtered = $derived(
		[...entries.values()]
			.filter((m) => m.username.toLowerCase().includes(query.toLowerCase()))
			.sort((a, b) => a.username.localeCompare(b.username))
	);

	onMount(async () => {
		try {
			const snapshot = await api.getPresence();
			const map = new Map<string, PresenceEntry>();
			for (const e of snapshot) map.set(e.user_id, e);
			entries = map;
		} finally {
			loading = false;
		}

		socket = connectPresence(
			() => get(auth).accessToken,
			(entry) => {
				const next = new Map(entries);
				next.set(entry.user_id, entry);
				entries = next;
			},
			() => {},
			(update) => {
				const next = new Map(liveMatches);
				if (update.match && update.match.game_slug === 'rocket-league') {
					next.set(update.user_id, update.match);
				} else {
					next.delete(update.user_id);
				}
				liveMatches = next;
			}
		);
	});

	onDestroy(() => socket?.close());
</script>

<div class="mb-5 flex items-center justify-between gap-4">
	<h1 class="pd-heading text-xl">{t('players.heading')}</h1>
	<input
		type="search"
		placeholder={t('players.searchPlaceholder')}
		bind:value={query}
		class="pd-cut-sm w-48 border border-[var(--color-border)] bg-[var(--color-surface)] px-3 py-1.5 text-sm outline-none focus:border-[var(--color-brand)]"
	/>
</div>

{#if loading}
	<p class="text-[var(--color-muted)]">{t('players.loading')}</p>
{:else if filtered.length === 0}
	<p class="text-[var(--color-muted)]">{t('players.empty')}</p>
{:else}
	<ul class="pd-card overflow-hidden">
		{#each filtered as m (m.user_id)}
			<li class="border-b border-[var(--color-border)] last:border-b-0">
				<a
					href="/players/{m.user_id}"
					class="group flex items-center gap-3 px-4 py-3 transition-colors hover:bg-[var(--color-surface-2)]"
				>
					<Avatar username={m.username} url={m.avatar_url} size={36} />
					<span class="font-display flex-1 font-semibold group-hover:text-[var(--color-brand-bright)]"
						>{m.username}</span
					>
					{#if m.status === 'in_game' && m.game}
						<span class="truncate text-sm font-medium text-[var(--color-brand)]"
							>{m.game.name}</span
						>
					{/if}
					{#if liveMatches.has(m.user_id)}
						{@const lm = liveMatches.get(m.user_id)!}
						<span
							class="pd-cut-sm flex shrink-0 items-center gap-1.5 bg-[var(--color-online)]/15 px-2 py-0.5 font-display text-xs font-bold italic"
						>
							<span class="uppercase text-[var(--color-online)]">{t('players.rlLive')}</span>
							<span class="flex items-center gap-1 tabular-nums">
								<span
									class="inline-block h-2 w-2 shrink-0 rounded-full bg-[var(--color-blue)]"
									title={t('game.rlBlue')}
								></span>
								<span class="inline-block w-4 text-right text-[var(--color-text)]"
									>{lm.team_blue_score}</span
								>
								<span class="text-[var(--color-muted)]">-</span>
								<span class="inline-block w-4 text-left text-[var(--color-text)]"
									>{lm.team_orange_score}</span
								>
								<span
									class="inline-block h-2 w-2 shrink-0 rounded-full bg-[var(--color-brand-bright)]"
									title={t('game.rlOrange')}
								></span>
							</span>
							<span class="inline-block w-10 text-right tabular-nums text-[var(--color-muted)]"
								>{formatClock(lm.seconds_remaining)}</span
							>
							{#if lm.overtime}
								<span class="text-[var(--color-magenta)]">{t('players.rlOvertime')}</span>
							{/if}
						</span>
					{/if}
					<StatusBadge status={m.status} />
				</a>
			</li>
		{/each}
	</ul>
{/if}
