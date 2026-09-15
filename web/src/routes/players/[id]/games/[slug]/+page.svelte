<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { api, type PlayerGameDetail, type WowAchievementEntry } from '$lib/api';
	import { formatDate, formatDuration, timeAgo } from '$lib/format';
	import { t } from '$lib/i18n';
	import { wowClassColor, wowClassIcon, wowVersionGroups, wowVersionIcon, wowVersionName } from '$lib/wow';
	import { heroArt, heroName, hsHeroRate, hsPlacementBars, hsTrend } from '$lib/hearthstone';

	let detail = $state<PlayerGameDetail | null>(null);
	let loading = $state(true);
	let error = $state('');

	const id = $derived(page.params.id ?? '');
	const slug = $derived(page.params.slug ?? '');

	// Per-character achievements state, keyed by realm_slug+"|"+name
	type AchievementState = {
		open: boolean;
		loading: boolean;
		list: WowAchievementEntry[];
		search: string;
		shown: number; // how many items to show (pagination step 50)
	};
	let achStates = $state<Map<string, AchievementState>>(new Map());

	function achKey(realmSlug: string, name: string): string {
		return `${realmSlug}|${name}`;
	}

	function getAchState(realmSlug: string, name: string): AchievementState {
		const k = achKey(realmSlug, name);
		let s = achStates.get(k);
		if (!s) {
			s = { open: false, loading: false, list: [], search: '', shown: 50 };
			achStates.set(k, s);
		}
		return s;
	}

	async function toggleAchievements(realmSlug: string, name: string) {
		const k = achKey(realmSlug, name);
		let s = achStates.get(k);
		if (!s) {
			s = { open: false, loading: false, list: [], search: '', shown: 50 };
		}
		if (s.open) {
			// close
			achStates.set(k, { ...s, open: false });
			achStates = new Map(achStates);
			return;
		}
		// open: load if not loaded
		if (s.list.length === 0 && !s.loading) {
			achStates.set(k, { ...s, open: true, loading: true });
			achStates = new Map(achStates);
			try {
				const data = await api.wowAchievements(id, realmSlug, name);
				const cur = achStates.get(k)!;
				achStates.set(k, { ...cur, loading: false, list: data.achievements });
				achStates = new Map(achStates);
			} catch {
				const cur = achStates.get(k)!;
				achStates.set(k, { ...cur, loading: false });
				achStates = new Map(achStates);
			}
		} else {
			achStates.set(k, { ...s, open: true });
			achStates = new Map(achStates);
		}
	}

	function setSearch(realmSlug: string, name: string, value: string) {
		const k = achKey(realmSlug, name);
		const s = achStates.get(k)!;
		achStates.set(k, { ...s, search: value, shown: 50 });
		achStates = new Map(achStates);
	}

	function showMore(realmSlug: string, name: string) {
		const k = achKey(realmSlug, name);
		const s = achStates.get(k)!;
		achStates.set(k, { ...s, shown: s.shown + 50 });
		achStates = new Map(achStates);
	}

	function filteredAchs(s: AchievementState): WowAchievementEntry[] {
		if (!s.search.trim()) return s.list;
		const q = s.search.toLowerCase();
		return s.list.filter((a) => a.name.toLowerCase().includes(q));
	}

	/** Format epoch ms as a short date string. */
	function achDate(ms: number): string {
		return new Date(ms).toLocaleDateString(undefined, {
			year: 'numeric',
			month: 'short',
			day: 'numeric'
		});
	}

	/** Falls back to the numeric champion id when the icon service failed to
	 * resolve a display name during sync (name comes back empty, not absent). */
	function lolChampName(name: string | undefined, id?: number): string {
		if (name && name.trim()) return name;
		return id !== undefined ? `#${id}` : '?';
	}

	/** Mode labels the catalog knows; anything else shows the raw mode name. */
	const HS_MODE_KEYS: Record<string, string> = {
		battlegrounds: 'game.hsModeBattlegrounds',
		constructed: 'game.hsModeConstructed',
		arena: 'game.hsModeArena'
	};

	function hsModeLabel(mode: string): string {
		const key = HS_MODE_KEYS[mode];
		return key ? t(key) : mode;
	}

	/** Short day-and-month stamp for a match, in the reader's locale. */
	function hsMatchDate(iso: string): string {
		return new Date(iso).toLocaleDateString(undefined, { month: 'short', day: 'numeric' });
	}

	/**
	 * The trend curve inside a 100x40 viewBox, with the bounds it was drawn
	 * against so the axis can be labelled truthfully.
	 *
	 * The axis fits the data rather than the full 1 to 8 range. A rolling
	 * average moves by tenths of a place, so on a full-range axis every curve
	 * is a flat line and the block says nothing. The window never shrinks
	 * below one whole placement, which stops a steady run from being magnified
	 * into noise.
	 *
	 * Placement 1 sits at the top, so a member whose placements improve draws a
	 * line that climbs, which is the way progress is read.
	 */
	function hsTrendChart(series: number[]): { points: string; best: number; worst: number } {
		const W = 100;
		const H = 40;
		const lo = Math.min(...series);
		const hi = Math.max(...series);
		const span = Math.max(hi - lo, 1);
		const best = Math.min(Math.max((lo + hi) / 2 - span / 2, 1), 8 - span);
		const worst = best + span;
		const points = series
			.map((v, i) => {
				const x = series.length > 1 ? (i * W) / (series.length - 1) : W / 2;
				const y = ((v - best) / span) * H;
				return `${x.toFixed(2)},${y.toFixed(2)}`;
			})
			.join(' ');
		return { points, best, worst };
	}

	onMount(load);
	async function load() {
		loading = true;
		error = '';
		try {
			detail = await api.userGameDetail(id, slug);
		} catch (e) {
			error = e instanceof Error ? e.message : t('playerGame.loadError');
		} finally {
			loading = false;
		}
	}
</script>

{#if loading}
	<p class="text-[var(--color-muted)]">{t('playerGame.loading')}</p>
{:else if error}
	<p class="text-[var(--color-magenta)]">{error}</p>
{:else if detail}
	<!-- Back link -->
	<a
		href="/players/{id}"
		class="mb-5 inline-flex items-center gap-1.5 text-sm text-[var(--color-muted)] transition-colors hover:text-[var(--color-brand-bright)]"
	>
		<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="h-4 w-4" aria-hidden="true">
			<path stroke-linecap="round" stroke-linejoin="round" d="M15 19l-7-7 7-7" />
		</svg>
		{t('playerGame.backToProfile')}
	</a>

	<!-- Game header -->
	<div class="pd-card pd-glow mb-6 overflow-hidden">
		{#if detail.game.cover_url}
			<div class="relative h-40 sm:h-56">
				<img src={detail.game.cover_url} alt="" class="h-full w-full object-cover" />
				<div class="absolute inset-0 bg-gradient-to-t from-[var(--color-bg)] via-[rgba(18,19,25,0.55)] to-transparent"></div>
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
				<h1 class="pd-heading text-3xl text-white" style="text-shadow: 0 0 24px rgba(154,108,255,0.7);">
					{detail.game.name}
				</h1>
			</div>
		{/if}

		<!-- Playtime stats row -->
		<div class="flex gap-8 border-t border-[var(--color-border)] bg-[var(--color-surface-2)] px-5 py-4">
			{#if detail.total_seconds}
				<div>
					<p class="font-display text-xl font-bold text-[var(--color-brand-bright)]">{formatDuration(detail.total_seconds)}</p>
					<p class="text-xs text-[var(--color-muted)] uppercase tracking-wide">{t('playerGame.playtime')}</p>
				</div>
				{#if detail.session_count}
					<div>
						<p class="font-display text-xl font-bold text-[var(--color-cyan)]">{detail.session_count}</p>
						<p class="text-xs text-[var(--color-muted)] uppercase tracking-wide">{t('playerGame.sessions')}</p>
					</div>
				{/if}
				{#if detail.last_played_at}
					<div>
						<p class="font-display text-xl font-bold text-[var(--color-text)]">{timeAgo(detail.last_played_at)}</p>
						<p class="text-xs text-[var(--color-muted)] uppercase tracking-wide">last played</p>
					</div>
				{/if}
			{:else}
				<p class="text-sm text-[var(--color-muted)]">{t('playerGame.noPlaytime')}</p>
			{/if}
		</div>
	</div>

	<!-- WoW Characters, grouped by game version like the game page does -->
	{#if detail.wow_characters?.length}
		{@const wowGroups = wowVersionGroups(detail.wow_characters)}
		{@const someMissing = detail.wow_characters.some((c) => !c.has_achievements)}
		<section class="mb-6">
			<h2 class="pd-heading mb-3 flex items-center gap-2 text-sm text-[var(--color-brand-bright)]">
				<span class="inline-block h-4 w-1 bg-[var(--color-brand)]"></span>
				{t('game.wowCharacters')}
			</h2>

			<!-- A Blizzard token lasts a day and cannot be renewed, so the list below
			     stopped moving. That is worth saying plainly, not worth alarming over. -->
			{#if detail.wow_link_expired}
				<p class="pd-cut-sm mb-3 border border-[var(--color-border)] bg-[var(--color-surface-2)] px-3 py-2 text-sm text-[var(--color-muted)]">
					{#if detail.wow_synced_at}{t('game.wowSyncedOn', { date: formatDate(detail.wow_synced_at) })}{' '}{/if}{t(
						'game.wowLinkExpired'
					)}{' '}
					<a href="/account" class="text-[var(--color-brand-bright)] hover:underline">{t('game.wowRelink')}</a>
				</p>
			{/if}

			{#each wowGroups as group (group.version)}
				<p class="mt-3 mb-1 flex items-center gap-1.5 text-xs uppercase tracking-wide text-[var(--color-muted)]">
					{#if wowVersionIcon(group.version)}
						<img
							src={wowVersionIcon(group.version)}
							alt=""
							width="16"
							height="16"
							loading="lazy"
							class="rounded-sm"
							onerror={(e) => { (e.currentTarget as HTMLImageElement).style.display = 'none'; }}
						/>
					{/if}
					{wowVersionName(group.version) || t('game.wowOtherVersion')}
					<span class="text-[var(--color-muted)]/60">&middot; {group.characters.length}</span>
				</p>
				<div class="grid grid-cols-1 gap-2 sm:grid-cols-2">
					{#each group.characters as ch (ch.realm_slug + '|' + ch.name)}
						{@const rs = ch.realm_slug ?? ''}
						{@const s = getAchState(rs, ch.name)}
						<div class="pd-cut-sm border border-[var(--color-border)] bg-[var(--color-surface-2)]">
							<!-- Character row -->
							<div class="flex items-center gap-3 px-3 py-2">
								{#if wowClassIcon(ch.class)}
									<img
										src={wowClassIcon(ch.class)}
										alt={ch.class ?? ''}
										class="h-8 w-8 shrink-0 rounded"
										loading="lazy"
										onerror={(e) => { (e.currentTarget as HTMLImageElement).style.display = 'none'; }}
									/>
								{/if}
								<div class="min-w-0 flex-1">
									<p class="font-display font-semibold text-[var(--color-text)]">
										{ch.name}{#if ch.realm}<span class="text-[var(--color-muted)]"> - {ch.realm}</span>{/if}
									</p>
									<p class="text-sm text-[var(--color-muted)]">
										{#if ch.level}{t('game.level')} {ch.level} &middot; {/if}{ch.race ?? ''}{ch.race && ch.class ? ' ' : ''}<span style="color: {wowClassColor(ch.class)}">{ch.class ?? ''}</span>{#if ch.race || ch.class} &middot; {/if}{t('game.ilvl')} {ch.item_level}{#if ch.mythic_rating} &middot; M+ {Math.round(ch.mythic_rating)}{/if}{#if ch.achievement_points} &middot; {t('game.achievementPoints')} {ch.achievement_points}{/if}
									</p>
								</div>
								<!-- No list on file means Blizzard never gave one, which an empty
								     panel would pass off as a character without achievements. -->
								{#if !ch.has_achievements}
									<span class="ml-auto shrink-0 px-2 py-1 text-xs text-[var(--color-muted)]/70">
										{t('game.wowNoAchievements')}
									</span>
								{:else if rs}
									<button
										onclick={() => toggleAchievements(rs, ch.name)}
										class="ml-auto shrink-0 rounded px-2 py-1 text-xs text-[var(--color-muted)] transition-colors hover:bg-[var(--color-surface)] hover:text-[var(--color-brand-bright)]"
										aria-expanded={s.open}
									>
										{t('game.wowAchievements')}
										{s.open ? '▾' : '▸'}
									</button>
								{/if}
							</div>

							<!-- Expandable achievements panel -->
							{#if s.open}
								<div class="border-t border-[var(--color-border)] px-3 py-2">
									{#if s.loading}
										<p class="py-2 text-xs text-[var(--color-muted)]">{t('common.loading')}</p>
									{:else if s.list.length === 0}
										<p class="py-2 text-xs text-[var(--color-muted)]">{t('game.noAchievements')}</p>
									{:else}
										<!-- Search -->
										<input
											type="search"
											placeholder={t('game.searchAchievements')}
											value={s.search}
											oninput={(e) => setSearch(rs, ch.name, (e.currentTarget as HTMLInputElement).value)}
											class="mb-2 w-full rounded border border-[var(--color-border)] bg-[var(--color-surface)] px-2 py-1 text-xs text-[var(--color-text)] placeholder:text-[var(--color-muted)] focus:outline-none focus:ring-1 focus:ring-[var(--color-brand)]"
										/>
										{@const filtered = filteredAchs(s)}
										{@const visible = filtered.slice(0, s.shown)}
										{#if visible.length === 0}
											<p class="py-2 text-xs text-[var(--color-muted)]">{t('game.noAchievements')}</p>
										{:else}
											<ul class="divide-y divide-[var(--color-border)]">
												{#each visible as a (a.id)}
													<li class="flex items-center justify-between gap-2 py-1.5">
														<span class="min-w-0 truncate text-xs text-[var(--color-text)]">{a.name}</span>
														<span class="shrink-0 text-xs text-[var(--color-muted)]">{achDate(a.completed_at)}</span>
													</li>
												{/each}
											</ul>
											{#if filtered.length > s.shown}
												<button
													onclick={() => showMore(rs, ch.name)}
													class="mt-2 w-full rounded border border-[var(--color-border)] py-1 text-xs text-[var(--color-muted)] transition-colors hover:text-[var(--color-brand-bright)]"
												>
													{t('game.showMore')} ({filtered.length - s.shown} {t('game.wowAchievements').toLowerCase()})
												</button>
											{/if}
										{/if}
									{/if}
								</div>
							{/if}
						</div>
					{/each}
				</div>
			{/each}

			{#if someMissing}
				<p class="mt-3 text-xs text-[var(--color-muted)]/80">{t('game.wowAchievementsGoneNote')}</p>
			{/if}
		</section>
	{/if}

	<!-- Battle.net Profile (Diablo III / StarCraft II) -->
	{#if detail.bnet_profile}
		<section class="mb-6">
			<h2 class="pd-heading mb-3 flex items-center gap-2 text-sm text-[var(--color-brand-bright)]">
				<span class="inline-block h-4 w-1 bg-[var(--color-brand)]"></span>
				{t('game.bnetProfiles')}
			</h2>
			<div class="pd-cut-sm px-3 py-2 border border-[var(--color-border)] bg-[var(--color-surface-2)]">
				{#if slug === 'diablo-iii'}
					<p class="text-sm text-[var(--color-muted)]">
						{t('game.paragon')} {Number(detail.bnet_profile.paragon ?? 0)}{#if Array.isArray(detail.bnet_profile.heroes)} · {(detail.bnet_profile.heroes as unknown[]).length} {t('game.heroes')}{/if}
					</p>
				{:else if slug === 'starcraft-ii-battle-chest'}
					<p class="text-sm text-[var(--color-muted)]">
						{String(detail.bnet_profile.race ?? '')}{#if detail.bnet_profile.league} · {String(detail.bnet_profile.league)}{/if}
					</p>
				{:else}
					<p class="text-sm text-[var(--color-muted)] font-mono text-xs">{JSON.stringify(detail.bnet_profile)}</p>
				{/if}
			</div>
		</section>
	{/if}

	<!-- League of Legends profile -->
	{#if detail.lol_profile}
		{@const profile = detail.lol_profile}
		{@const solo = profile.ranks.find((r) => r.queue === 'RANKED_SOLO_5x5')}
		{@const flex = profile.ranks.find((r) => r.queue === 'RANKED_FLEX_SR')}
		<section class="mb-6">
			<h2 class="pd-heading mb-3 flex items-center gap-2 text-sm text-[var(--color-brand-bright)]">
				<span class="inline-block h-4 w-1 bg-[var(--color-brand)]"></span>
				League of Legends
				{#if detail.lol_live}
					<span class="pd-cut-sm bg-[var(--color-online)]/15 px-2 py-0.5 font-display text-xs uppercase tracking-wide text-[var(--color-online)]">
						{t('lol.inGame')}
					</span>
				{/if}
			</h2>
			<div class="pd-cut-sm border border-[var(--color-border)] bg-[var(--color-surface-2)] p-3">
				<p class="font-display font-semibold text-[var(--color-text)]">
					{profile.riot_id}<span class="text-[var(--color-muted)]"> - {profile.platform}</span>
				</p>

				<!-- Ranked queues -->
				<div class="mt-3 grid grid-cols-1 gap-3 sm:grid-cols-2">
					<div>
						<p class="text-xs uppercase tracking-wide text-[var(--color-muted)]">{t('lol.solo')}</p>
						{#if solo}
							<p class="text-sm text-[var(--color-text)]">
								{solo.tier} {solo.division} · {solo.lp} LP
								<span class="text-[var(--color-muted)]">({solo.wins}W {solo.losses}L)</span>
								{#if solo.hot_streak}<span class="text-[var(--color-gold)]"> · {t('lol.hotStreak')}</span>{/if}
							</p>
						{:else}
							<p class="text-sm text-[var(--color-muted)]">{t('lol.unranked')}</p>
						{/if}
					</div>
					<div>
						<p class="text-xs uppercase tracking-wide text-[var(--color-muted)]">{t('lol.flex')}</p>
						{#if flex}
							<p class="text-sm text-[var(--color-text)]">
								{flex.tier} {flex.division} · {flex.lp} LP
								<span class="text-[var(--color-muted)]">({flex.wins}W {flex.losses}L)</span>
								{#if flex.hot_streak}<span class="text-[var(--color-gold)]"> · {t('lol.hotStreak')}</span>{/if}
							</p>
						{:else}
							<p class="text-sm text-[var(--color-muted)]">{t('lol.unranked')}</p>
						{/if}
					</div>
				</div>

				<!-- Top champions -->
				{#if profile.top_champions?.length}
					<div class="mt-3 border-t border-[var(--color-border)] pt-3">
						<p class="mb-2 text-xs uppercase tracking-wide text-[var(--color-muted)]">{t('lol.champions')}</p>
						<div class="flex flex-wrap gap-3">
							{#each profile.top_champions as c (c.champion_id)}
								<div class="flex items-center gap-2">
									{#if c.icon_url}
										<img
											src={c.icon_url}
											alt=""
											class="h-8 w-8 shrink-0 rounded"
											loading="lazy"
											onerror={(e) => { (e.currentTarget as HTMLImageElement).style.display = 'none'; }}
										/>
									{/if}
									<div class="min-w-0">
										<p class="truncate text-sm text-[var(--color-text)]">{lolChampName(c.name, c.champion_id)}</p>
										<p class="text-xs text-[var(--color-muted)]">{c.points.toLocaleString()} pts</p>
									</div>
								</div>
							{/each}
						</div>
					</div>
				{/if}

				<!-- Recent games -->
				{#if profile.recent?.length}
					<div class="mt-3 border-t border-[var(--color-border)] pt-3">
						<p class="mb-2 text-xs uppercase tracking-wide text-[var(--color-muted)]">{t('lol.recent')}</p>
						<ul class="flex flex-col gap-1">
							{#each profile.recent as m (m.match_id)}
								<li class="flex items-center justify-between gap-2 text-sm">
									<span class="font-display {m.win ? 'text-[var(--color-online)]' : 'text-[var(--color-magenta)]'}">
										{m.win ? t('lol.win') : t('lol.loss')}
									</span>
									<span class="flex-1 truncate text-[var(--color-text)]">{lolChampName(m.champion)}</span>
									<span class="shrink-0 text-[var(--color-muted)]">{m.kills}/{m.deaths}/{m.assists}</span>
								</li>
							{/each}
						</ul>
					</div>
				{/if}
			</div>
			<p class="mt-2 text-xs text-[var(--color-muted)]/80">{t('common.riotDisclaimer')}</p>
		</section>
	{/if}

	<!-- Hearthstone: the member's own record -->
	{#if detail.hs_profile}
		{@const hs = detail.hs_profile}
		{@const bars = hsPlacementBars(hs)}
		{@const trend = hsTrend(hs.recent ?? [])}
		{@const heroes = (hs.heroes ?? []).slice(0, 6)}
		{@const recent = (hs.recent ?? []).slice(0, 10)}
		<section class="mb-6">
			<h2 class="pd-heading mb-3 flex items-center gap-2 text-sm text-[var(--color-brand-bright)]">
				<span class="inline-block h-4 w-1 bg-[var(--color-brand)]"></span>
				{t('game.hsRecord')}
			</h2>

			<!-- Counters -->
			<div class="mb-3 grid grid-cols-2 gap-3 sm:grid-cols-4">
				<div class="pd-card p-3">
					<p class="text-xs uppercase tracking-wide text-[var(--color-muted)]">{t('game.hsMatches')}</p>
					<p class="font-display text-2xl font-bold text-[var(--color-text)]">{hs.matches}</p>
				</div>
				{#if hs.avg_placement !== undefined}
					<div class="pd-card p-3">
						<p class="text-xs uppercase tracking-wide text-[var(--color-muted)]">{t('game.hsAvgPlacement')}</p>
						<p class="font-display text-2xl font-bold tabular-nums text-[var(--color-brand-bright)]">
							{hs.avg_placement.toFixed(1)}
						</p>
					</div>
				{/if}
				<div class="pd-card p-3">
					<p class="text-xs uppercase tracking-wide text-[var(--color-muted)]">{t('game.hsTop4')}</p>
					<p class="font-display text-2xl font-bold tabular-nums text-[var(--color-cyan)]">
						{hs.ranked > 0 ? Math.round((hs.top4 * 100) / hs.ranked) : 0}%
					</p>
				</div>
				<div class="pd-card p-3">
					<p class="text-xs uppercase tracking-wide text-[var(--color-muted)]">{t('game.hsWins')}</p>
					<p class="font-display text-2xl font-bold tabular-nums text-[var(--color-gold)]">{hs.wins}</p>
				</div>
			</div>

			<div class="grid grid-cols-1 gap-3 {trend.length ? 'lg:grid-cols-2' : ''}">
				<!-- Placement spread: the shape of the bars is what tells two players apart -->
				<div class="pd-card p-3">
					<p class="mb-2 text-xs uppercase tracking-wide text-[var(--color-muted)]">{t('game.hsSpread')}</p>
					<ul class="flex flex-col gap-1">
						{#each bars as b (b.placement)}
							<li class="flex items-center gap-2 {b.placement === 4 ? 'mb-1 border-b border-dashed border-[var(--color-border)] pb-2' : ''}">
								<span
									class="w-4 shrink-0 text-right font-display text-xs tabular-nums {b.placement <= 4
										? 'text-[var(--color-brand-bright)]'
										: 'text-[var(--color-muted)]'}"
								>
									{b.placement}
								</span>
								<span class="h-3 flex-1 bg-[var(--color-surface-2)]">
									<span
										class="block h-3 {b.placement <= 4
											? 'bg-[var(--color-brand)]'
											: 'bg-[var(--color-muted)]/40'}"
										style="width: {(b.share * 100).toFixed(1)}%"
									></span>
								</span>
								<span class="w-8 shrink-0 text-right text-xs tabular-nums text-[var(--color-muted)]">{b.matches}</span>
							</li>
						{/each}
					</ul>
				</div>

				<!-- Trend: drawn with the best placement at the top so progress climbs -->
				{#if trend.length}
					{@const chart = hsTrendChart(trend)}
					<div class="pd-card p-3">
						<p class="mb-2 text-xs uppercase tracking-wide text-[var(--color-muted)]">{t('game.hsTrend')}</p>
						<div class="flex items-stretch gap-2">
							<div class="flex shrink-0 flex-col justify-between py-0.5 text-[10px] tabular-nums text-[var(--color-muted)]">
								<span>{chart.best.toFixed(1)}</span>
								<span>{chart.worst.toFixed(1)}</span>
							</div>
							<svg
								viewBox="0 0 100 40"
								preserveAspectRatio="none"
								class="h-24 w-full"
								role="img"
								aria-label={t('game.hsTrendCaption')}
							>
								<!-- The top-four line, drawn only when the axis reaches it. -->
								{#if chart.best <= 4 && chart.worst >= 4}
									{@const y = ((4 - chart.best) / (chart.worst - chart.best)) * 40}
									<line x1="0" y1={y} x2="100" y2={y}
										stroke="var(--color-border)" stroke-width="1" stroke-dasharray="3 3"
										vector-effect="non-scaling-stroke" />
								{/if}
								<polyline
									points={chart.points}
									fill="none"
									stroke="var(--color-brand-bright)"
									stroke-width="2"
									stroke-linejoin="round"
									stroke-linecap="round"
									vector-effect="non-scaling-stroke"
								/>
							</svg>
						</div>
						<p class="mt-2 text-xs text-[var(--color-muted)]/80">{t('game.hsTrendCaption')}</p>
					</div>
				{/if}
			</div>

			<!-- Heroes -->
			{#if heroes.length}
				<h3 class="mt-5 mb-2 font-display text-xs uppercase tracking-wide text-[var(--color-muted)]">
					{t('game.hsHeroes')}
				</h3>
				<div class="grid grid-cols-2 gap-3 sm:grid-cols-3">
					{#each heroes as h (h.hero_card_id)}
						<div class="pd-card flex items-center gap-3 p-2">
							<img
								src={heroArt(h.hero_card_id)}
								alt=""
								loading="lazy"
								class="h-12 w-12 shrink-0 rounded-full object-cover"
							/>
							<div class="min-w-0">
								<p class="truncate font-display text-sm font-semibold text-[var(--color-text)]">
									{heroName(h.hero_card_id)}
								</p>
								<p class="text-xs text-[var(--color-muted)]">
									{h.matches > 1 ? t('game.hsHeroMatches', { n: h.matches }) : t('game.hsHeroMatch')}
								</p>
								<p class="text-xs tabular-nums">
									<span class="text-[var(--color-brand-bright)]">{h.avg_placement.toFixed(1)}</span>
									<span class="text-[var(--color-muted)]">&middot; {hsHeroRate(h)}% {t('game.hsTop4')}</span>
								</p>
							</div>
						</div>
					{/each}
				</div>
			{/if}

			<!-- Last ten matches -->
			{#if recent.length}
				<h3 class="mt-5 mb-2 font-display text-xs uppercase tracking-wide text-[var(--color-muted)]">
					{t('game.hsRecentMatches')}
				</h3>
				<div class="pd-card overflow-x-auto">
					<table class="w-full min-w-[420px] border-collapse">
						<thead>
							<tr class="border-b border-[var(--color-border)]">
								<th class="px-3 py-2 text-left font-display text-xs uppercase tracking-wide text-[var(--color-muted)]">{t('game.hsDate')}</th>
								<th class="px-3 py-2 text-left font-display text-xs uppercase tracking-wide text-[var(--color-muted)]">{t('game.hsHero')}</th>
								<th class="px-3 py-2 text-left font-display text-xs uppercase tracking-wide text-[var(--color-muted)]">{t('game.hsPlacement')}</th>
								<th class="px-3 py-2 text-left font-display text-xs uppercase tracking-wide text-[var(--color-muted)]">{t('game.hsTurns')}</th>
							</tr>
						</thead>
						<tbody>
							{#each recent as m (m.played_at)}
								<tr class="border-b border-[var(--color-border)]/50 last:border-b-0 hover:bg-[var(--color-surface-2)]">
									<td class="px-3 py-2 whitespace-nowrap text-sm text-[var(--color-muted)]">{hsMatchDate(m.played_at)}</td>
									<td class="px-3 py-2 text-sm text-[var(--color-text)]">
										{#if m.hero_card_id}{heroName(m.hero_card_id)}{:else}&mdash;{/if}
									</td>
									<td class="px-3 py-2 whitespace-nowrap text-sm tabular-nums">
										{#if m.placement !== undefined}
											<span class={m.placement <= 4 ? 'text-[var(--color-brand-bright)]' : 'text-[var(--color-muted)]'}>
												{m.placement}
											</span>
										{:else}
											<!-- A constructed game has no placement; naming the mode beats an empty cell -->
											<span class="text-xs italic text-[var(--color-muted)]">{hsModeLabel(m.mode)}</span>
										{/if}
									</td>
									<td class="px-3 py-2 whitespace-nowrap text-sm tabular-nums text-[var(--color-muted)]">
										{#if m.turns !== undefined}{m.turns}{/if}
									</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>
			{/if}

			<p class="mt-2 text-xs text-[var(--color-muted)]/80">{t('game.hsRatingNote')}</p>
		</section>
	{/if}

	<!-- Achievements -->
	{#if detail.achievements?.length}
		<section class="mb-6">
			<h2 class="pd-heading mb-4 flex items-center gap-2 text-sm text-[var(--color-gold)]">
				<span class="inline-block h-4 w-1 bg-[var(--color-gold)]"></span>
				{t('game.achievements')}
			</h2>
			<div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
				{#each detail.achievements as a (a.api_name)}
					<div class="pd-card flex items-center gap-3 p-3">
						{#if a.icon_url}
							<img src={a.icon_url} alt="" class="pd-cut-sm h-10 w-10 shrink-0 object-cover" />
						{:else}
							<span class="pd-cut-sm flex h-10 w-10 shrink-0 items-center justify-center bg-[var(--color-surface)] text-xl" aria-hidden="true">🏆</span>
						{/if}
						<div class="min-w-0 flex-1">
							<p class="font-display truncate font-bold text-[var(--color-text)]">{a.display_name ?? a.api_name}</p>
							<p class="text-xs text-[var(--color-muted)]">{timeAgo(a.unlocked_at)}</p>
						</div>
					</div>
				{/each}
			</div>
		</section>
	{/if}
{/if}
