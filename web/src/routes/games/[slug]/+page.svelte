<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { api, type GameDetail } from '$lib/api';
	import { auth } from '$lib/stores/auth.svelte';
	import { formatDuration } from '$lib/format';
	import Avatar from '$lib/components/Avatar.svelte';
	import { t } from '$lib/i18n';
	import {
		wowClassColor, wowClassIcon, wowRosters, wowMemberCount,
		wowVersionName, wowVersionIcon
	} from '$lib/wow';
	import { inGame, mostPlayedChampion, podium, soloRank, tierSpread, winRate,
	         crestURL, loadingArtURL, mainChampion, tierColour } from '$lib/lol';

	let detail = $state<GameDetail | null>(null);
	let loading = $state(true);
	let error = $state('');
	let toggling = $state(false);

	const slug = $derived(page.params.slug ?? '');
	const isAdmin = $derived($auth.user?.role === 'admin');

	async function toggleHidden() {
		if (!detail || toggling) return;
		toggling = true;
		try {
			const res = await api.setGameHidden(detail.game.id, !detail.game.hidden);
			detail = { ...detail, game: { ...detail.game, hidden: res.hidden } };
		} catch (e) {
			error = e instanceof Error ? e.message : t('game.loadError');
		} finally {
			toggling = false;
		}
	}
	let topSeconds = $derived(
		Math.max(1, ...(detail?.leaderboard ?? []).map((e) => e.total_seconds))
	);

	const lolPlayers = $derived(detail?.lol_players ?? []);
	const lolLive = $derived(inGame(lolPlayers));
	const lolPodium = $derived(podium(lolPlayers));
	const lolSpread = $derived(tierSpread(lolPlayers));
	const lolMostPlayed = $derived(mostPlayedChampion(lolPlayers));
	// Ticks once a minute so the live durations stay honest without a reload.
	let lolNow = $state(Date.now());
	$effect(() => {
		if (!lolLive.length) return;
		const id = setInterval(() => (lolNow = Date.now()), 60_000);
		return () => clearInterval(id);
	});
	function lolElapsed(startedAt: string): number {
		return Math.max(0, Math.floor((lolNow - new Date(startedAt).getTime()) / 1000));
	}

	const lolWeekSeconds = $derived(
		(detail?.recent_players ?? []).reduce((n, p) => n + p.total_seconds, 0)
	);

	const wowChars = $derived(detail?.wow_characters ?? []);
	const wowRosterList = $derived(wowRosters(wowChars));
	// Which member cards have their full roster expanded, by user id.
	let wowExpanded = $state(new Set<string>());
	function toggleWowRoster(userId: string) {
		const next = new Set(wowExpanded);
		if (next.has(userId)) next.delete(userId);
		else next.add(userId);
		wowExpanded = next;
	}

	onMount(load);
	async function load() {
		loading = true;
		error = '';
		try {
			detail = await api.getGame(slug);
		} catch (e) {
			error = e instanceof Error ? e.message : t('game.loadError');
		} finally {
			loading = false;
		}
	}
</script>

{#if loading}
	<p class="text-[var(--color-muted)]">{t('game.loading')}</p>
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
				<p class="text-xs text-[var(--color-muted)] uppercase tracking-wide">{t('game.totalPlayed')}</p>
			</div>
			<div>
				<p class="font-display text-xl font-bold text-[var(--color-cyan)]">{detail.player_count}</p>
				<p class="text-xs text-[var(--color-muted)] uppercase tracking-wide">{t('game.players', { count: detail.player_count })}</p>
			</div>
			{#if isAdmin}
				<div class="ml-auto flex flex-col items-end justify-center gap-1">
					<button
						type="button"
						class="pd-cut-sm px-3 py-1.5 text-sm font-display border transition-colors disabled:opacity-50
							{detail.game.hidden
								? 'border-[var(--color-brand)] text-[var(--color-brand-bright)] hover:bg-[var(--color-surface)]'
								: 'border-[var(--color-magenta)] text-[var(--color-magenta)] hover:bg-[var(--color-surface)]'}"
						disabled={toggling}
						onclick={toggleHidden}
					>
						{detail.game.hidden ? t('game.show') : t('game.hide')}
					</button>
					{#if detail.game.hidden}
						<p class="max-w-xs text-right text-xs text-[var(--color-muted)]">{t('game.hiddenNotice')}</p>
					{/if}
				</div>
			{/if}
		</div>
	</div>

	<!-- Recent players (last 7 days) -->
	<h2 class="pd-heading mb-4 flex items-center gap-2 text-sm text-[var(--color-cyan)]">
		<span class="inline-block h-4 w-1 bg-[var(--color-cyan)]"></span>
		{t('game.recentPlayers')}
	</h2>
	{#if detail.recent_players && detail.recent_players.length > 0}
		<ul class="mb-8 flex flex-col gap-2">
			{#each detail.recent_players as e, i (e.user_id)}
				<a
					href="/players/{e.user_id}"
					class="pd-card group flex items-center gap-3 p-3 transition-all duration-150 hover:border-[var(--color-cyan)]"
				>
					<span class="pd-cut-sm flex h-7 w-7 shrink-0 items-center justify-center bg-[var(--color-surface-2)] font-display text-sm font-bold text-[var(--color-muted)]">
						{i + 1}
					</span>
					<Avatar username={e.username} url={e.avatar_url} size={36} />
					<span class="flex-1 truncate font-display font-semibold text-[var(--color-text)]">
						{e.username}
					</span>
					<span class="w-20 text-right font-display text-sm text-[var(--color-cyan)]">
						{formatDuration(e.total_seconds)}
					</span>
				</a>
			{/each}
		</ul>
	{:else}
		<p class="mb-8 text-sm text-[var(--color-muted)]">{t('game.noRecentPlayers')}</p>
	{/if}

	<!-- Leaderboard -->
	<h2 class="pd-heading mb-4 flex items-center gap-2 text-sm text-[var(--color-brand-bright)]">
		<span class="inline-block h-4 w-1 bg-[var(--color-brand)]"></span>
		{t('game.allTime')}
	</h2>
	{#if detail.leaderboard.length === 0}
		<p class="text-sm text-[var(--color-muted)]">{t('game.noPlayers')}</p>
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

	<!-- Achievements -->
	<h2 class="pd-heading mt-8 mb-4 flex items-center gap-2 text-sm text-[var(--color-brand-bright)]">
		<span class="inline-block h-4 w-1 bg-[var(--color-brand)]"></span>
		{t('game.achievements')}
	</h2>
	{#if !detail.achievements || detail.achievements.length === 0}
		<p class="text-sm text-[var(--color-muted)]">{t('game.noAchievements')}</p>
	{:else}
		<div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
			{#each detail.achievements as a (a.api_name)}
				<div class="pd-card flex items-center gap-3 p-3">
					{#if a.icon_url}
						<img
							src={a.icon_url}
							alt=""
							class="pd-cut-sm h-10 w-10 shrink-0 object-cover"
						/>
					{:else}
						<span class="pd-cut-sm flex h-10 w-10 shrink-0 items-center justify-center bg-[var(--color-surface)] text-xl" aria-hidden="true">🏆</span>
					{/if}
					<div class="min-w-0 flex-1">
						<p class="font-display truncate font-bold text-[var(--color-text)]">{a.display_name ?? a.api_name}</p>
						<p class="text-xs text-[var(--color-muted)]">{t('game.unlockedBy', { count: a.unlocks })}</p>
					</div>
				</div>
			{/each}
		</div>
	{/if}

	<!-- WoW guild roster, grouped by member then by game version -->
	{#if wowChars.length}
		<section class="mt-6">
			<h2 class="pd-heading mb-3 flex items-center gap-2 text-sm text-[var(--color-brand-bright)]">
				<span class="inline-block h-4 w-1 bg-[var(--color-brand)]"></span>
				{t('game.wowRoster')}
			</h2>

			<div class="mb-3 grid grid-cols-2 gap-3 sm:grid-cols-3">
				<div class="pd-card p-3">
					<p class="text-xs uppercase tracking-wide text-[var(--color-muted)]">{t('game.wowMembers')}</p>
					<p class="font-display text-2xl font-bold text-[var(--color-text)]">{wowMemberCount(wowChars)}</p>
				</div>
				<div class="pd-card p-3">
					<p class="text-xs uppercase tracking-wide text-[var(--color-muted)]">{t('game.wowCharacterCount')}</p>
					<p class="font-display text-2xl font-bold text-[var(--color-cyan)]">{wowChars.length}</p>
				</div>
				{#if detail.wow_synced_at}
					<div class="pd-card p-3">
						<p class="text-xs uppercase tracking-wide text-[var(--color-muted)]">{t('game.wowSyncedAt')}</p>
						<p class="font-display text-lg font-bold text-[var(--color-text)]">
							{new Date(detail.wow_synced_at).toLocaleDateString()}
						</p>
					</div>
				{/if}
			</div>

			<div class="flex flex-col gap-3">
				{#each wowRosterList as r (r.userId)}
					{@const open = wowExpanded.has(r.userId)}
					{@const capped = r.groups.reduce((n, g) => n + Math.min(g.characters.length, 3), 0)}
					<div class="pd-card p-3">
						<div class="mb-2 flex items-center gap-2">
							<Avatar username={r.username} url={r.avatarUrl} size={30} />
							<a
								href="/players/{r.userId}/games/{slug}"
								class="font-display font-semibold text-[var(--color-text)] hover:underline"
							>
								{r.username}
							</a>
							<span class="text-xs text-[var(--color-muted)]">{r.total}</span>
						</div>

						{#each r.groups as g (g.version)}
							{@const shown = open ? g.characters : g.characters.slice(0, 3)}
							<div class="mt-2">
								<p class="mb-1 flex items-center gap-1.5 text-xs uppercase tracking-wide text-[var(--color-muted)]">
									{#if wowVersionIcon(g.version)}
										<img
											src={wowVersionIcon(g.version)}
											alt=""
											width="16"
											height="16"
											loading="lazy"
											class="rounded-sm"
											onerror={(e) => { (e.currentTarget as HTMLImageElement).style.display = 'none'; }}
										/>
									{/if}
									{wowVersionName(g.version) || t('game.wowOtherVersion')}
									<span class="text-[var(--color-muted)]/60">· {g.characters.length}</span>
								</p>
								<ul class="flex flex-col gap-1">
									{#each shown as ch (ch.realm + ch.name)}
										<li class="flex items-center gap-2 text-sm">
											{#if wowClassIcon(ch.class)}
												<img
													src={wowClassIcon(ch.class)}
													alt=""
													width="20"
													height="20"
													loading="lazy"
													class="shrink-0 rounded-sm"
													onerror={(e) => { (e.currentTarget as HTMLImageElement).style.display = 'none'; }}
												/>
											{/if}
											<span class="font-semibold" style="color:{wowClassColor(ch.class)}">{ch.name}</span>
											{#if ch.realm}
												<span class="truncate text-xs text-[var(--color-muted)]">{ch.realm}</span>
											{/if}
											<span class="ml-auto shrink-0 text-xs tabular-nums text-[var(--color-muted)]">
												{ch.level ?? 0}
												{#if ch.item_level > 0}
													· {t('game.wowItemLevel')} {ch.item_level}
												{/if}
												{#if ch.mythic_rating}
													· {t('game.wowMythic')} {Math.round(ch.mythic_rating)}
												{/if}
											</span>
										</li>
									{/each}
								</ul>
							</div>
						{/each}

						{#if r.total > capped}
							<button
								type="button"
								class="mt-2 font-display text-xs uppercase tracking-wide text-[var(--color-cyan)] hover:underline"
								onclick={() => toggleWowRoster(r.userId)}
							>
								{open ? t('game.wowShowLess') : t('game.wowShowMore', { count: r.total - capped })}
							</button>
						{/if}
					</div>
				{/each}
			</div>
		</section>
	{/if}

	<!-- Battle.net Profiles -->
	{#if detail.bnet_profiles?.length}
		<section class="mt-6">
			<h2 class="pd-heading mb-3 text-xs text-[var(--color-brand-bright)]">{t('game.bnetProfiles')}</h2>
			<div class="grid grid-cols-1 gap-2 sm:grid-cols-2">
				{#each detail.bnet_profiles as p}
					<div class="pd-cut-sm px-3 py-2">
						<p class="font-display font-semibold text-[var(--color-text)]">{p.username}</p>
						{#if slug === 'diablo-iii'}
							<p class="text-sm text-[var(--color-muted)]">{t('game.paragon')} {Number(p.data.paragon ?? 0)}{#if Array.isArray(p.data.heroes)} · {(p.data.heroes as unknown[]).length} {t('game.heroes')}{/if}</p>
						{:else if slug === 'starcraft-ii-battle-chest'}
							<p class="text-sm text-[var(--color-muted)]">{String(p.data.race ?? '')}{#if p.data.league} · {String(p.data.league)}{/if}</p>
						{/if}
					</div>
				{/each}
			</div>
		</section>
	{/if}

	{#if lolLive.length}
		<section class="mt-6">
			<div class="pd-card flex flex-wrap items-center gap-3 border-l-2 border-l-[var(--color-online)] p-3">
				<span class="flex items-center gap-2 font-display text-sm uppercase tracking-wide text-[var(--color-online)]">
					<span class="inline-block h-2 w-2 rounded-full bg-[var(--color-online)]"></span>
					{lolLive.length} {t('lol.nowPlayingCount')}
				</span>
				<ul class="flex flex-wrap gap-2">
					{#each lolLive as p (p.user_id)}
						<li class="pd-cut-sm flex items-center gap-2 bg-[var(--color-surface-2)] py-1 pl-1 pr-2">
							{#if p.live?.champion_icon}
								<img
									src={p.live.champion_icon}
									alt=""
									width="24"
									height="24"
									loading="lazy"
									class="shrink-0 rounded-sm"
								/>
							{/if}
							<span class="font-semibold text-[var(--color-text)]">{p.username}</span>
							{#if p.live?.champion_name}
								<span class="text-xs text-[var(--color-muted)]">{p.live.champion_name}</span>
							{/if}
							<span class="text-xs uppercase tracking-wide text-[var(--color-muted)]">
								{p.live?.mode}
							</span>
							{#if p.live?.started_at}
								<span class="text-xs tabular-nums text-[var(--color-muted)]/70">
									{formatDuration(lolElapsed(p.live.started_at))}
								</span>
							{/if}
						</li>
					{/each}
				</ul>
			</div>
		</section>
	{/if}

	{#if lolPlayers.length}
		<section class="mt-6 grid grid-cols-2 gap-3 sm:grid-cols-4">
			<div class="pd-card p-3">
				<p class="text-xs uppercase tracking-wide text-[var(--color-muted)]">{t('lol.linkedMembers')}</p>
				<p class="font-display text-2xl font-bold text-[var(--color-text)]">{lolPlayers.length}</p>
			</div>
			<div class="pd-card p-3">
				<p class="text-xs uppercase tracking-wide text-[var(--color-muted)]">{t('lol.playersThisWeek')}</p>
				<p class="font-display text-2xl font-bold text-[var(--color-cyan)]">
					{detail.recent_players?.length ?? 0}
				</p>
			</div>
			<div class="pd-card p-3">
				<p class="text-xs uppercase tracking-wide text-[var(--color-muted)]">{t('lol.playedThisWeek')}</p>
				<p class="font-display text-2xl font-bold text-[var(--color-cyan)]">{formatDuration(lolWeekSeconds)}</p>
			</div>
			<div class="pd-card p-3">
				<p class="text-xs uppercase tracking-wide text-[var(--color-muted)]">{t('lol.mostPlayed')}</p>
				<p class="font-display text-xl font-bold text-[var(--color-text)]">{lolMostPlayed ?? '-'}</p>
				<p class="text-[10px] text-[var(--color-muted)]/80">{t('lol.mostPlayedHint')}</p>
			</div>
		</section>
	{/if}

	{#if lolPodium.length}
		<section class="mt-6">
			<h2 class="pd-heading mb-3 flex items-center gap-2 text-sm text-[var(--color-brand-bright)]">
				<span class="inline-block h-4 w-1 bg-[var(--color-brand)]"></span>
				{t('lol.podium')}
			</h2>
			<div class="grid gap-3 sm:grid-cols-3">
				{#each lolPodium as p, i (p.user_id)}
					{@const r = soloRank(p)}
					{@const champ = mainChampion(p)}
					{@const art = loadingArtURL(champ?.image_id)}
					{@const crest = r ? crestURL(r.tier) : null}
					<a
						href="/players/{p.user_id}/games/{slug}"
						class="pd-card group relative overflow-hidden transition-all duration-150 hover:border-[var(--color-brand)]"
					>
						<div class="relative h-28 bg-[var(--color-surface-2)]">
							{#if art}
								<img src={art} alt="" loading="lazy"
									class="h-full w-full object-cover object-[center_18%] opacity-70" />
							{/if}
							<div class="absolute inset-0 bg-gradient-to-b from-transparent to-[var(--color-surface)]"></div>
							<span class="pd-cut-sm absolute left-2 top-2 bg-[var(--color-bg)]/80 px-2 py-0.5 font-display text-xs">
								{i + 1}
							</span>
						</div>
						<!-- relative: without it this row sits below the image gradient, which is
						     absolutely positioned, and the username disappears under it. -->
						<div class="relative -mt-5 flex items-end gap-2 px-3 pb-3">
							{#if crest}
								<img src={crest} alt="" width="40" height="40" class="shrink-0 drop-shadow" />
							{/if}
							<div class="min-w-0">
								<p class="truncate font-display font-bold text-[var(--color-text)]">{p.username}</p>
								{#if r}
									<p class="text-xs uppercase tracking-wide text-[var(--color-muted)]">
										{r.tier} {r.division}
									</p>
									<p class="text-xs text-[var(--color-muted)]">
										{r.lp} {t('lol.lp')} · {winRate(r)}%
									</p>
								{/if}
							</div>
						</div>
					</a>
				{/each}
			</div>
		</section>
	{/if}

	<!-- League of Legends standings -->
	{#if lolPlayers.length}
		<section class="mt-6">
			<h2 class="pd-heading mb-3 flex items-center gap-2 text-sm text-[var(--color-brand-bright)]">
				<span class="inline-block h-4 w-1 bg-[var(--color-brand)]"></span>
				{t('lol.leaderboard')}
			</h2>
			<div class="pd-card overflow-x-auto">
				<table class="w-full min-w-[640px] border-collapse">
					<thead>
						<tr class="border-b border-[var(--color-border)]">
							<th class="w-10 px-3 py-2"></th>
							<th class="px-3 py-2 text-left font-display text-xs uppercase tracking-wide text-[var(--color-muted)]">{t('lol.member')}</th>
							<th class="px-3 py-2 text-left font-display text-xs uppercase tracking-wide text-[var(--color-muted)]">{t('lol.soloQueue')}</th>
							<th class="px-3 py-2 text-left font-display text-xs uppercase tracking-wide text-[var(--color-muted)]">{t('lol.mainChampion')}</th>
							<th class="px-3 py-2 text-left font-display text-xs uppercase tracking-wide text-[var(--color-muted)]">{t('lol.formLabel')}</th>
							<th class="px-3 py-2 text-left font-display text-xs uppercase tracking-wide text-[var(--color-muted)]">{t('lol.winsLabel')}</th>
						</tr>
					</thead>
					<tbody>
						{#each lolPlayers as p, i (p.user_id)}
							{@const r = soloRank(p)}
							{@const champ = mainChampion(p)}
							{@const crest = r ? crestURL(r.tier) : null}
							<tr
								class="border-b border-[var(--color-border)]/50 last:border-b-0 hover:bg-[var(--color-surface-2)]
									{p.user_id === $auth.user?.id ? 'bg-[var(--color-brand)]/10' : ''}"
							>
								<td class="px-3 py-2 text-center font-display text-sm text-[var(--color-muted)]">{i + 1}</td>
								<td class="px-3 py-2">
									<a href="/players/{p.user_id}/games/{slug}" class="flex items-center gap-2 hover:underline">
										<Avatar username={p.username} url={p.avatar_url} size={30} />
										<span class="truncate font-display font-semibold text-[var(--color-text)]">{p.username}</span>
										{#if p.live}
											<span class="pd-cut-sm bg-[var(--color-online)]/15 px-1.5 py-0.5 font-display text-[10px] uppercase tracking-wide text-[var(--color-online)]">
												{t('lol.inGame')}
											</span>
										{/if}
									</a>
								</td>
								<td class="px-3 py-2">
									{#if r}
										<span class="flex items-center gap-2 whitespace-nowrap">
											{#if crest}<img src={crest} alt="" width="26" height="26" class="shrink-0" />{/if}
											<span>
												<span class="block text-sm uppercase tracking-wide text-[var(--color-text)]">{r.tier} {r.division}</span>
												<span class="block text-xs text-[var(--color-muted)]">{r.lp} {t('lol.lp')}</span>
											</span>
										</span>
									{:else}
										<span class="text-sm italic text-[var(--color-muted)]">{t('lol.unranked')}</span>
									{/if}
								</td>
								<td class="px-3 py-2">
									{#if champ}
										<span class="flex items-center gap-2 whitespace-nowrap">
											{#if champ.icon_url}
												<img src={champ.icon_url} alt="" width="26" height="26" loading="lazy" class="shrink-0 rounded-sm" />
											{/if}
											<span class="text-sm">{champ.name || `#${champ.champion_id}`}</span>
										</span>
									{/if}
								</td>
								<td class="px-3 py-2">
									<span class="flex gap-1">
										{#each p.data.recent as m (m.match_id)}
											<span
												class="inline-block h-3.5 w-3.5 rounded-sm {m.win
													? 'bg-[var(--color-online)]'
													: 'bg-[var(--color-magenta)]'}"
												title={m.champion}
											></span>
										{/each}
									</span>
								</td>
								<td class="whitespace-nowrap px-3 py-2">
									{#if r}
										<span class="font-display text-sm text-[var(--color-brand-bright)]">{winRate(r)}%</span>
										<span class="ml-1 text-xs text-[var(--color-muted)]">{r.wins}/{r.losses}</span>
									{/if}
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>

			{#if lolSpread.length}
				<div class="pd-card mt-3 p-3">
					<p class="mb-2 text-xs uppercase tracking-wide text-[var(--color-muted)]">{t('lol.spread')}</p>
					<div class="flex h-5 overflow-hidden rounded-sm bg-[var(--color-surface-2)]">
						{#each lolSpread as s (s.tier)}
							<span class="block" style="flex:{s.count};background:{tierColour(s.tier)}"></span>
						{/each}
					</div>
					<div class="mt-2 flex flex-wrap gap-3 text-xs text-[var(--color-muted)]">
						{#each lolSpread as s (s.tier)}
							<span class="flex items-center gap-1.5 uppercase tracking-wide">
								<i class="inline-block h-2 w-2 rounded-sm" style="background:{tierColour(s.tier)}"></i>
								{s.tier === 'unranked' ? t('lol.unranked') : s.tier} · {s.count}
							</span>
						{/each}
					</div>
				</div>
			{/if}

			<p class="mt-2 text-xs text-[var(--color-muted)]/80">{t('common.riotDisclaimer')}</p>
		</section>
	{/if}
{/if}
