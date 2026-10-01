<script lang="ts">
	import PlatformBadge from '$lib/components/PlatformBadge.svelte';
	import { playingGames } from '$lib/playing';
	// Every match being played right now, one card each. The page only reads the
	// shared live store: there is no snapshot to fetch and nothing to persist,
	// so a page opened mid-match fills up with the next sample, half a second
	// later. That is the design, not a gap to paper over.
	import { liveMatches } from '$lib/stores/live.svelte';
	import { presence } from '$lib/stores/presence.svelte';
	import type { PresenceEntry } from '$lib/api';
	import Avatar from '$lib/components/Avatar.svelte';
	import { RL_SLUG, rlLiveMatch, type RlLiveMatch } from '$lib/rocketleague';
	import { LOL_SLUG, lolLiveMatch, type LolLiveMatch } from '$lib/lol';
	import { HS_SLUG, hsLiveMatch, type HsLiveMatch } from '$lib/hearthstone';
	import RocketLeagueCard from '$lib/components/live/RocketLeagueCard.svelte';
	import LeagueOfLegendsCard from '$lib/components/live/LeagueOfLegendsCard.svelte';
	import HearthstoneCard from '$lib/components/live/HearthstoneCard.svelte';
	import { t } from '$lib/i18n';

	let names = $derived(new Map(presence.list.map((m) => [m.user_id, m.username])));

	// A card's game icon comes from presence, which already carries it, rather
	// than from the live payload: that payload is broadcast twice a second and
	// must stay as small as it can be, while an icon cannot change mid-match.
	//
	// Keyed by slug, not by member, so one member being absent from presence
	// does not cost the others their icon. When nobody at all is in presence
	// the icon is simply missing, which is the degradation the username already
	// has on these cards.
	let icons = $derived(
		new Map(
			presence.list
				.filter((m) => m.game?.icon_url)
				.map((m) => [m.game!.slug, m.game!.icon_url as string])
		)
	);

	type RlCard = { userId: string; username?: string; match: RlLiveMatch };
	type LolCard = { userId: string; username?: string; match: LolLiveMatch };
	type HsCard = { userId: string; username?: string; match: HsLiveMatch };

	// Sorted on a stable key rather than on the store's iteration order, so
	// cards keep their place across the twice-a-second refresh.
	function byName<T extends { userId: string; username?: string }>(a: T, b: T): number {
		return (a.username ?? '').localeCompare(b.username ?? '') || a.userId.localeCompare(b.userId);
	}

	// Only the games with a card of their own get one. A slug we cannot render
	// yields nothing at all, and that is the normal case, not an error: a client
	// newer than this page must never produce an empty or half-drawn card.
	let rlCards = $derived(
		liveMatches.list
			.flatMap((entry): RlCard[] => {
				const match = rlLiveMatch(entry);
				return match ? [{ userId: entry.user_id, username: names.get(entry.user_id), match }] : [];
			})
			.sort(byName)
	);

	let lolCards = $derived(
		liveMatches.list
			.flatMap((entry): LolCard[] => {
				const match = lolLiveMatch(entry);
				return match ? [{ userId: entry.user_id, username: names.get(entry.user_id), match }] : [];
			})
			.sort(byName)
	);

	let hsCards = $derived(
		liveMatches.list
			.flatMap((entry): HsCard[] => {
				const match = hsLiveMatch(entry);
				return match ? [{ userId: entry.user_id, username: names.get(entry.user_id), match }] : [];
			})
			.sort(byName)
	);

	let hasCards = $derived(rlCards.length > 0 || lolCards.length > 0 || hsCards.length > 0);

	// The catalogue is far wider than the three games we draw a card for, so the
	// page would claim nobody is playing while the presence store says otherwise.
	// Members already shown above are excluded by user id: a card is the richer
	// view of the same fact, and showing both would read as two players.
	let carded = $derived(
		new Set([...rlCards, ...lolCards, ...hsCards].map((card) => card.userId))
	);

	// `in_game` without a game happens (the client knows the session started
	// before it has resolved what is running), and such an entry would render an
	// anonymous line, so it is dropped rather than shown half-filled.
	let alsoPlaying = $derived(
		presence.list
			.filter(
				(m): m is PresenceEntry & { game: NonNullable<PresenceEntry['game']> } =>
					m.status === 'in_game' && !!m.game && !carded.has(m.user_id)
			)
			.sort((a, b) => a.username.localeCompare(b.username) || a.user_id.localeCompare(b.user_id))
	);
</script>

<h1 class="pd-heading mb-5 text-xl">{t('live.heading')}</h1>

{#if !hasCards && alsoPlaying.length === 0}
	<p class="text-[var(--color-muted)]">{t('live.empty')}</p>
{/if}

{#if hasCards}
	<div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
		{#each rlCards as card (card.userId)}
			<RocketLeagueCard username={card.username} match={card.match} icon={icons.get(RL_SLUG)} />
		{/each}
		{#each lolCards as card (card.userId)}
			<LeagueOfLegendsCard
				username={card.username}
				match={card.match}
				icon={icons.get(LOL_SLUG)}
			/>
		{/each}
		{#each hsCards as card (card.userId)}
			<HearthstoneCard username={card.username} match={card.match} icon={icons.get(HS_SLUG)} />
		{/each}
	</div>
{/if}

{#if alsoPlaying.length > 0}
	<h2 class="pd-heading mb-3 text-xs text-[var(--color-muted)] {hasCards ? 'mt-8' : ''}">{t('live.alsoPlaying')}</h2>
	<ul class="pd-card overflow-hidden">
		{#each alsoPlaying as m (m.user_id)}
			<li class="border-b border-[var(--color-border)] last:border-b-0">
				<a
					href="/players/{m.user_id}"
					class="group flex items-center gap-3 px-4 py-2.5 transition-colors hover:bg-[var(--color-surface-2)]"
				>
					<Avatar username={m.username} url={m.avatar_url} size={28} />
					<span class="flex-1 truncate text-sm font-semibold group-hover:text-[var(--color-brand-bright)]"
						>{m.username}</span
					>
					{#each playingGames(m) as p (p.game.id)}
						{#if p.game.icon_url}
							<img src={p.game.icon_url} alt="" class="pd-cut-sm h-5 w-5 shrink-0 object-cover" />
						{/if}
						<span class="truncate text-sm text-[var(--color-brand)]">{p.game.name}</span>
						<PlatformBadge platform={p.platform} />
					{/each}
				</a>
			</li>
		{/each}
	</ul>
{/if}
