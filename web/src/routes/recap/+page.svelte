<script lang="ts">
	// What the guild played over a hand-picked range: one summary per game, then
	// the single all-games timeline the evening actually happened in.
	//
	// Nothing here says who played WITH whom, and nothing may be added that
	// would. The match tables carry no column able to hold another player, so a
	// teammate's name never leaves a member's machine. Two entries sharing a
	// minute is a coincidence, not a shared match: pairing them would turn a
	// guess into a fact the reader could not tell apart from a measurement.
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import {
		api,
		type Recap,
		type RecapEntry,
		type RecapGameBlock,
		type RecapHsEntry,
		type RecapHsMember,
		type RecapMember,
		type RecapRlEntry,
		type RecapRlMember
	} from '$lib/api';
	import { formatClock, formatDuration } from '$lib/format';
	import { HS_SLUG, heroName, hsModeLabel } from '$lib/hearthstone';
	import { RL_SLUG, rlModeLabel, rlSideScore } from '$lib/rocketleague';
	import Avatar from '$lib/components/Avatar.svelte';
	import HsResult from '$lib/components/HsResult.svelte';
	import GameIcon from '$lib/components/GameIcon.svelte';
	import { t } from '$lib/i18n';

	type Range = { from: string; to: string };

	/** How far back an unparameterised recap looks: one evening, generously. */
	const DEFAULT_WINDOW_MS = 12 * 60 * 60 * 1000;

	function pad(n: number): string {
		return n.toString().padStart(2, '0');
	}

	/**
	 * An instant as the `YYYY-MM-DDTHH:mm` a datetime-local input expects, read
	 * off the LOCAL calendar.
	 *
	 * Built component by component on purpose. Slicing `toISOString()` is the
	 * one-liner everybody writes here, and it is wrong: that string is UTC, so
	 * the fields would come back shifted by the reader's offset and the whole
	 * recap would quietly cover the wrong hours.
	 */
	function toLocalInput(d: Date): string {
		return (
			`${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}` +
			`T${pad(d.getHours())}:${pad(d.getMinutes())}`
		);
	}

	/**
	 * The exact reverse: the wall-clock stamp a member typed, back to the
	 * instant it names in their own zone.
	 *
	 * Parsed by hand and rebuilt through the local `Date` constructor rather
	 * than fed to `new Date(value)`, so the missing offset is supplied by an
	 * explicit rule instead of by whatever the engine decides. Returns null for
	 * anything short of a complete stamp, which is what a half-typed field
	 * gives.
	 */
	function fromLocalInput(value: string): Date | null {
		const m = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})$/.exec(value.trim());
		if (!m) return null;
		const d = new Date(Number(m[1]), Number(m[2]) - 1, Number(m[3]), Number(m[4]), Number(m[5]));
		return Number.isNaN(d.getTime()) ? null : d;
	}

	/**
	 * The last twelve hours, as RFC 3339 instants.
	 *
	 * Evaluated once, at page setup, and never inside a derivation: a default
	 * recomputed from the clock would produce a new range on every pass and
	 * refetch forever.
	 */
	const fallbackRange: Range = (() => {
		const to = new Date();
		to.setSeconds(0, 0);
		return { from: new Date(to.getTime() - DEFAULT_WINDOW_MS).toISOString(), to: to.toISOString() };
	})();

	// The URL owns the window, not the fields: that is what makes a recap a link
	// somebody can paste in Discord and reopen unchanged. The bounds travel as
	// RFC 3339 instants so the link means the same evening to a reader in
	// another time zone.
	const urlRange = $derived.by((): Range => {
		const from = page.url.searchParams.get('from');
		const to = page.url.searchParams.get('to');
		// Both or neither: half a window is not a window, and silently keeping
		// one typed bound next to a clock-derived one would be a third range
		// nobody asked for.
		return from && to ? { from, to } : fallbackRange;
	});

	// The bounds are read as two plain strings rather than through the object
	// above. A derivation that yields a fresh object re-fires everything that
	// depends on it even when the window has not moved, which would fetch the
	// same recap twice on the way in; two string derivations settle instead.
	const rangeFrom = $derived(urlRange.from);
	const rangeTo = $derived(urlRange.to);

	let fromInput = $state('');
	let toInput = $state('');

	// The fields follow the URL, including through back and forward. They are
	// only an editor for the window; the URL is where it lives.
	$effect(() => {
		fromInput = toLocalInput(new Date(rangeFrom));
		toInput = toLocalInput(new Date(rangeTo));
	});

	// The two filters live in the URL for the same reason the bounds do: a link
	// to "Wednesday evening, Rocket League only" has to reopen on exactly that.
	// They hold ids rather than names, which survive a member renaming himself.
	const gameFilter = $derived(page.url.searchParams.get('game') ?? '');
	const memberFilter = $derived(page.url.searchParams.get('member') ?? '');
	const filterActive = $derived(gameFilter !== '' || memberFilter !== '');

	// Pin the default into the URL so that even an untouched recap is a link
	// worth sharing. Replaces rather than pushes: it is the same view, not a
	// step the reader took.
	$effect(() => {
		if (!page.url.searchParams.get('from') || !page.url.searchParams.get('to')) {
			goto(viewHref(fallbackRange, gameFilter, memberFilter), {
				replaceState: true,
				keepFocus: true,
				noScroll: true
			});
		}
	});

	/**
	 * The whole view as one address: window plus filters.
	 *
	 * Every navigation goes through here so that moving one of the three never
	 * drops the other two, which is what would make a shared link reopen on a
	 * view nobody meant.
	 */
	function viewHref(r: Range, game: string, member: string): string {
		const params = new URLSearchParams({ from: r.from, to: r.to });
		if (game) params.set('game', game);
		if (member) params.set('member', member);
		return `/recap?${params.toString()}`;
	}

	function applyRange(event: SubmitEvent) {
		event.preventDefault();
		const start = fromLocalInput(fromInput);
		const end = fromLocalInput(toInput);
		// An incomplete field is already refused by the input itself; there is
		// nothing useful to say that the browser has not said.
		if (!start || !end) return;
		const r = { from: start.toISOString(), to: end.toISOString() };
		goto(viewHref(r, gameFilter, memberFilter), { keepFocus: true, noScroll: true });
	}

	// Filters take effect as they are picked, without going through the range
	// form's button: nothing is fetched, so there is nothing to confirm.
	function applyFilters(game: string, member: string) {
		goto(viewHref({ from: rangeFrom, to: rangeTo }, game, member), {
			keepFocus: true,
			noScroll: true
		});
	}

	let recap = $state<Recap | null>(null);
	let loading = $state(true);
	let errorMessage = $state('');
	// Guards against a slow request landing after a newer one: the reader would
	// otherwise be shown a recap for a range they have already left.
	let requestSeq = 0;

	$effect(() => {
		const from = rangeFrom;
		const to = rangeTo;
		const seq = ++requestSeq;
		loading = true;
		errorMessage = '';
		api
			.getRecap(from, to)
			.then((result) => {
				if (seq !== requestSeq) return;
				recap = result;
			})
			.catch((err: unknown) => {
				if (seq !== requestSeq) return;
				recap = null;
				// The server's sentence says precisely what is wrong with the range;
				// replacing it with a generic message would throw that away.
				errorMessage = err instanceof Error ? err.message : String(err);
			})
			.finally(() => {
				if (seq === requestSeq) loading = false;
			});
	});

	// Both lists are read off the answer that came back, never off a fixed
	// catalogue: offering a game nobody touched that evening would be a dead
	// end, and the reader would blame the page rather than the choice.
	const gameOptions = $derived(recap?.games ?? []);

	const memberOptions = $derived.by((): RecapMember[] => {
		const seen = new Map<string, RecapMember>();
		for (const block of recap?.games ?? []) {
			for (const m of block.members) if (!seen.has(m.user_id)) seen.set(m.user_id, m);
		}
		return [...seen.values()].sort((a, b) => a.username.localeCompare(b.username));
	});

	// A filter kept across a range change can point at a game or a member absent
	// from the new range. The filter is NOT cleared on its own, because that
	// would silently rewrite a link somebody shared. But the chip must not fall
	// back to the raw id: a uuid on screen tells the reader nothing and looks
	// broken. It says the filter no longer matches, which is what happened.
	const gameFilterName = $derived(
		gameOptions.find((b) => b.game_id === gameFilter)?.game_name ?? t('recap.filterStale')
	);
	const memberFilterName = $derived(
		memberOptions.find((m) => m.user_id === memberFilter)?.username ?? t('recap.filterStale')
	);

	// The server sends the evening oldest first; the page shows the latest
	// match on top, so the one just played is read without scrolling.
	const timeline = $derived(
		(recap?.timeline ?? [])
			.filter(
				(e) =>
					(gameFilter === '' || e.game_id === gameFilter) &&
					(memberFilter === '' || e.user_id === memberFilter)
			)
			.reverse()
	);

	/**
	 * The summary, cut down to what the timeline still shows.
	 *
	 * The per-game count is recounted off the filtered timeline instead of
	 * being summed from the member rows: a game this portal has no columns for
	 * carries rows without any count at all, while the timeline holds one line
	 * per match whatever the game. Both halves of the page then rest on the
	 * same measure and cannot contradict each other.
	 */
	const gameBlocks = $derived.by((): RecapGameBlock[] => {
		if (!recap) return [];
		if (!filterActive) return recap.games;
		const counts = new Map<string, number>();
		for (const e of timeline) counts.set(e.game_id, (counts.get(e.game_id) ?? 0) + 1);
		return recap.games
			.filter((b) => counts.has(b.game_id))
			.map((b) => ({
				...b,
				matches: counts.get(b.game_id) ?? 0,
				// A member row already counts that member alone over the window, so
				// only the rows themselves are dropped, never their figures.
				members:
					memberFilter === '' ? b.members : b.members.filter((m) => m.user_id === memberFilter)
			}));
	});

	// Unfiltered, the server's own total is kept rather than the timeline
	// length: they agree today, and the figure the server computed is the one
	// to trust if they ever stop agreeing.
	const shownMatches = $derived(filterActive ? timeline.length : (recap?.total_matches ?? 0));

	// The summary rows and the timeline entries carry a shape that depends on
	// the game, and the payload says which only through `game_slug`. These are
	// the single place an assertion happens, and only once the slug has been
	// checked at runtime, so the markup never casts.
	function rlMembers(block: RecapGameBlock): RecapRlMember[] | null {
		return block.game_slug === RL_SLUG ? (block.members as RecapRlMember[]) : null;
	}

	function hsMembers(block: RecapGameBlock): RecapHsMember[] | null {
		return block.game_slug === HS_SLUG ? (block.members as RecapHsMember[]) : null;
	}

	function rlEntry(entry: RecapEntry): RecapRlEntry | null {
		return entry.game_slug === RL_SLUG ? (entry as RecapRlEntry) : null;
	}

	function hsEntry(entry: RecapEntry): RecapHsEntry | null {
		return entry.game_slug === HS_SLUG ? (entry as RecapHsEntry) : null;
	}

	/** The result's label and accent, the text carrying the meaning first. */
	function resultInfo(result: string): { label: string; colorClass: string } {
		if (result === 'win') return { label: t('game.rlWin'), colorClass: 'text-[var(--color-online)]' };
		if (result === 'loss')
			return { label: t('game.rlLoss'), colorClass: 'text-[var(--color-magenta)]' };
		return { label: t('game.rlDraw'), colorClass: 'text-[var(--color-muted)]' };
	}

	// A range that stays inside one local day needs no date on every line; one
	// that crosses midnight does, and an evening usually does.
	const spansDays = $derived(
		new Date(rangeFrom).toDateString() !== new Date(rangeTo).toDateString()
	);

	function stamp(iso: string): string {
		const d = new Date(iso);
		const time = d.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' });
		if (!spansDays) return time;
		return `${d.toLocaleDateString(undefined, { month: 'short', day: 'numeric' })} ${time}`;
	}

	const headerClass =
		'px-3 py-2 text-left font-display text-xs uppercase tracking-wide text-[var(--color-muted)]';
	const numberClass = 'px-3 py-2 whitespace-nowrap text-sm tabular-nums';
	const inputClass =
		'mt-1 w-full border border-[var(--color-border)] bg-[var(--color-bg)] px-3 py-2 text-sm text-[var(--color-text)] outline-none focus:border-[var(--color-brand)]';
	const selectClass = `${inputClass} pd-cut-sm`;
</script>

<h1 class="pd-heading mb-1 text-2xl text-[var(--color-brand-bright)]">{t('recap.title')}</h1>
<p class="mb-5 text-sm text-[var(--color-muted)]">{t('recap.subtitle')}</p>

<form class="pd-card mb-6 flex flex-col gap-3 p-4 sm:flex-row sm:items-end" onsubmit={applyRange}>
	<label class="flex-1 text-xs text-[var(--color-muted)]" for="recap-from">
		{t('recap.from')}
		<input id="recap-from" type="datetime-local" bind:value={fromInput} class={inputClass} />
	</label>
	<label class="flex-1 text-xs text-[var(--color-muted)]" for="recap-to">
		{t('recap.to')}
		<input id="recap-to" type="datetime-local" bind:value={toInput} class={inputClass} />
	</label>
	<button type="submit" class="btn-pd shrink-0 px-4 py-2 text-sm">{t('recap.apply')}</button>
</form>

<!-- Two lists rather than rows of chips: the page is already dense and the
     date fields own the top of it, so the filters have to cost one line on a
     phone. They sit below the range because they narrow its answer. -->
<div class="mb-4 flex flex-col gap-3 sm:flex-row sm:items-end">
	<label class="flex-1 text-xs text-[var(--color-muted)]" for="recap-game">
		{t('recap.filterGame')}
		<select
			id="recap-game"
			value={gameFilter}
			onchange={(e) => applyFilters(e.currentTarget.value, memberFilter)}
			class={selectClass}
		>
			<option value="">{t('recap.allGames')}</option>
			{#each gameOptions as block (block.game_id)}
				<option value={block.game_id}>{block.game_name}</option>
			{/each}
		</select>
	</label>
	<label class="flex-1 text-xs text-[var(--color-muted)]" for="recap-member">
		{t('recap.filterMember')}
		<select
			id="recap-member"
			value={memberFilter}
			onchange={(e) => applyFilters(gameFilter, e.currentTarget.value)}
			class={selectClass}
		>
			<option value="">{t('recap.allMembers')}</option>
			{#each memberOptions as m (m.user_id)}
				<option value={m.user_id}>{m.username}</option>
			{/each}
		</select>
	</label>
</div>

{#if filterActive && recap}
	<!-- Somebody opening a shared link has to see at a glance that the page is
	     holding something back, without reading the two lists, and has to find
	     the way out in the same place. Held back until the answer is in, since
	     until then the names behind the ids are not known. -->
	<div class="mb-5 flex flex-wrap items-center gap-2 text-xs">
		<span class="text-[var(--color-muted)]">{t('recap.filtering')}</span>
		{#if gameFilter}
			<span
				class="pd-cut-sm border border-[var(--color-brand)] px-2 py-1 text-[var(--color-brand-bright)]"
			>
				{gameFilterName}
			</span>
		{/if}
		{#if memberFilter}
			<span
				class="pd-cut-sm border border-[var(--color-brand)] px-2 py-1 text-[var(--color-brand-bright)]"
			>
				{memberFilterName}
			</span>
		{/if}
		<button
			type="button"
			onclick={() => applyFilters('', '')}
			class="pd-cut-sm border border-[var(--color-border)] px-2 py-1 text-[var(--color-muted)] hover:border-[var(--color-brand)] hover:text-[var(--color-text)]"
		>
			{t('recap.clearFilters')}
		</button>
	</div>
{/if}

{#if errorMessage}
	<p class="text-sm text-[var(--color-magenta)]">{errorMessage}</p>
{:else if loading && !recap}
	<p class="text-[var(--color-muted)]">{t('recap.loading')}</p>
{:else if recap}
	{#if recap.total_matches === 0}
		<p class="text-[var(--color-muted)]">{t('recap.empty')}</p>
	{:else if timeline.length === 0}
		<!-- A different sentence from the one above, because the two say opposite
		     things: nobody played, versus the filter hides everybody who did.
		     Reading the second as the first would make a busy evening look dead
		     and nobody would think of clearing the filter. -->
		<p class="text-[var(--color-muted)]">{t('recap.emptyFiltered')}</p>
	{:else}
		<p class="mb-5 text-sm text-[var(--color-muted)] tabular-nums">
			{t('recap.totalMatches', { count: shownMatches })}
		</p>

		<!-- Summary: one block per game, then one line per member inside it. No
		     column is shared between games that have nothing in common. -->
		<h2 class="pd-heading mb-3 text-sm text-[var(--color-brand-bright)]">{t('recap.summary')}</h2>
		{#each gameBlocks as block (block.game_id)}
			{@const rl = rlMembers(block)}
			{@const hs = hsMembers(block)}
			<section class="mb-5">
				<h3 class="font-display mb-2 flex items-center gap-2 text-lg font-bold">
					<GameIcon url={block.icon_url} size={22} />
					<span class="text-[var(--color-text)]">{block.game_name}</span>
					<span class="text-xs text-[var(--color-muted)] tabular-nums">
						{t('recap.totalMatches', { count: block.matches })}
					</span>
				</h3>
				<div class="pd-card overflow-x-auto">
					{#if rl}
						<table class="w-full min-w-[640px] border-collapse">
							<thead>
								<tr class="border-b border-[var(--color-border)]">
									<th class={headerClass}>{t('recap.member')}</th>
									<th class={headerClass}>{t('recap.matches')}</th>
									<th class={headerClass}>{t('recap.record')}</th>
									<th class={headerClass}>{t('recap.rl.goals')}</th>
									<th class={headerClass}>{t('recap.rl.assists')}</th>
									<th class={headerClass}>{t('recap.rl.saves')}</th>
									<th class={headerClass}>{t('recap.rl.mvps')}</th>
									<th class={headerClass}>{t('recap.rl.playTime')}</th>
								</tr>
							</thead>
							<tbody>
								{#each rl as m (m.user_id)}
									<tr
										class="border-b border-[var(--color-border)]/50 last:border-b-0 hover:bg-[var(--color-surface-2)]"
									>
										<td class="px-3 py-2">
											<a href="/players/{m.user_id}" class="flex items-center gap-2 hover:underline">
												<Avatar username={m.username} url={m.avatar_url} size={28} />
												<span class="font-display truncate font-semibold text-[var(--color-text)]">
													{m.username}
												</span>
											</a>
										</td>
										<td class={numberClass}>{m.matches}</td>
										<td class={numberClass}>{m.wins}-{m.losses}-{m.draws}</td>
										<td class="{numberClass} text-[var(--color-brand-bright)]">{m.goals}</td>
										<td class={numberClass}>{m.assists}</td>
										<td class={numberClass}>{m.saves}</td>
										<td class={numberClass}>{m.mvps}</td>
										<td class="{numberClass} text-[var(--color-muted)]">
											{formatDuration(m.play_time_seconds)}
										</td>
									</tr>
								{/each}
							</tbody>
						</table>
					{:else if hs}
						<table class="w-full min-w-[560px] border-collapse">
							<thead>
								<tr class="border-b border-[var(--color-border)]">
									<th class={headerClass}>{t('recap.member')}</th>
									<th class={headerClass}>{t('recap.matches')}</th>
									<th class={headerClass}>{t('recap.record')}</th>
									<th class={headerClass}>{t('recap.hs.ranked')}</th>
									<th class={headerClass}>{t('recap.hs.avgPlacement')}</th>
									<th class={headerClass}>{t('recap.hs.top4')}</th>
								</tr>
							</thead>
							<tbody>
								{#each hs as m (m.user_id)}
									<tr
										class="border-b border-[var(--color-border)]/50 last:border-b-0 hover:bg-[var(--color-surface-2)]"
									>
										<td class="px-3 py-2">
											<a href="/players/{m.user_id}" class="flex items-center gap-2 hover:underline">
												<Avatar username={m.username} url={m.avatar_url} size={28} />
												<span class="font-display truncate font-semibold text-[var(--color-text)]">
													{m.username}
												</span>
											</a>
										</td>
										<td class={numberClass}>{m.matches}</td>
										<td class={numberClass}>{m.wins}-{m.losses}-{m.draws}</td>
										<td class={numberClass}>{m.ranked}</td>
										<td class="{numberClass} text-[var(--color-brand-bright)]">
											<!-- Null, not zero, when nothing carried a placement: an average
											     of nothing is not a first place. -->
											{m.avg_placement === null ? t('recap.none') : m.avg_placement.toFixed(2)}
										</td>
										<td class={numberClass}>{m.top4}</td>
									</tr>
								{/each}
							</tbody>
						</table>
					{:else}
						<!-- A game this page has no columns for: a client newer than the
						     portal, or a game whose block is still to be written. Its
						     members are still named, because only the per-game figures
						     are unknown. -->
						<ul class="divide-y divide-[var(--color-border)]/50">
							{#each block.members as m (m.user_id)}
								<li class="flex items-center gap-2 px-3 py-2">
									<Avatar username={m.username} url={m.avatar_url} size={28} />
									<a
										href="/players/{m.user_id}"
										class="font-display truncate font-semibold text-[var(--color-text)] hover:underline"
									>
										{m.username}
									</a>
								</li>
							{/each}
						</ul>
						<p class="px-3 py-2 text-xs text-[var(--color-muted)]">{t('recap.noDetail')}</p>
					{/if}
				</div>
			</section>
		{/each}

		<!-- Timeline: every game merged, latest first (the server sends it oldest
		     first). Each line names the member the match came from, and only them. -->
		<h2 class="pd-heading mt-6 mb-3 text-sm text-[var(--color-brand-bright)]">
			{t('recap.timeline')}
		</h2>
		<div class="pd-card overflow-x-auto">
			<table class="w-full min-w-[680px] border-collapse">
				<thead>
					<tr class="border-b border-[var(--color-border)]">
						<th class={headerClass}>{t('recap.time')}</th>
						<th class={headerClass}>{t('recap.member')}</th>
						<th class={headerClass}>{t('recap.game')}</th>
						<th class={headerClass}>{t('recap.detail')}</th>
					</tr>
				</thead>
				<tbody>
					{#each timeline as entry, i (i)}
						{@const rl = rlEntry(entry)}
						{@const hs = hsEntry(entry)}
						<tr
							class="border-b border-[var(--color-border)]/50 last:border-b-0 hover:bg-[var(--color-surface-2)]"
						>
							<td class="px-3 py-2 text-sm whitespace-nowrap text-[var(--color-muted)] tabular-nums">
								{stamp(entry.played_at)}
							</td>
							<td class="px-3 py-2">
								<a href="/players/{entry.user_id}" class="flex items-center gap-2 hover:underline">
									<Avatar username={entry.username} url={entry.avatar_url} size={24} />
									<span class="font-display truncate text-sm font-semibold text-[var(--color-text)]">
										{entry.username}
									</span>
								</a>
							</td>
							<td class="px-3 py-2 text-sm whitespace-nowrap text-[var(--color-muted)]">
								<span class="flex items-center gap-2">
									<GameIcon url={entry.icon_url} size={18} />
									{entry.game_name}
								</span>
							</td>
							<td class="px-3 py-2 text-sm">
								{#if rl}
									{@const side = rlSideScore(rl)}
									{@const info = resultInfo(rl.result)}
									<div class="flex flex-wrap items-center gap-x-3 gap-y-1">
										<span class="font-display font-bold {info.colorClass}">{info.label}</span>
										<span class="font-display font-bold tabular-nums">
											{side.own}-{side.opponent}
										</span>
										<span class="text-xs text-[var(--color-muted)]">{rlModeLabel(rl)}</span>
										<span class="text-xs tabular-nums">
											{t('recap.rl.goals')}
											{rl.goals} · {t('recap.rl.assists')}
											{rl.assists} · {t('recap.rl.saves')}
											{rl.saves}
										</span>
										{#if rl.mvp}
											<span class="font-display text-xs font-bold text-[var(--color-gold)]">
												{t('recap.rl.mvps')}
											</span>
										{/if}
										<span class="text-xs text-[var(--color-muted)] tabular-nums">
											{formatClock(rl.duration_seconds)}
										</span>
									</div>
								{:else if hs}
									<div class="flex flex-wrap items-center gap-x-3 gap-y-1">
										<!-- No separate placement chip: in Battlegrounds the result IS
										     the placement, so the chip would repeat it word for word. -->
										<HsResult mode={hs.mode} result={hs.result} placement={hs.placement} />
										<span class="text-xs text-[var(--color-muted)]">{hsModeLabel(hs.mode)}</span>
										{#if hs.hero_card_id}
											<span class="text-xs">{heroName(hs.hero_card_id)}</span>
										{/if}
										{#if hs.turns !== null}
											<span class="text-xs text-[var(--color-muted)] tabular-nums">
												{t('recap.hs.turns')}
												{hs.turns}
											</span>
										{/if}
									</div>
								{:else}
									<!-- An unknown game still gets its line: the member, the time and
									     the game are true whatever the portal can render of the rest. -->
									<span class="text-xs text-[var(--color-muted)] italic">{t('recap.noDetail')}</span>
								{/if}
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}
{/if}
