<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { api, type PlayerGameDetail, type WowAchievementEntry } from '$lib/api';
	import { formatDuration, timeAgo } from '$lib/format';
	import { t } from '$lib/i18n';
	import { wowClassColor, wowClassIcon } from '$lib/wow';

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

	<!-- WoW Characters -->
	{#if detail.wow_characters?.length}
		<section class="mb-6">
			<h2 class="pd-heading mb-3 flex items-center gap-2 text-sm text-[var(--color-brand-bright)]">
				<span class="inline-block h-4 w-1 bg-[var(--color-brand)]"></span>
				{t('game.wowCharacters')}
			</h2>
			<div class="grid grid-cols-1 gap-2 sm:grid-cols-2">
				{#each detail.wow_characters as ch}
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
									{#if ch.level}{t('game.level')} {ch.level} · {/if}{ch.race ?? ''}{ch.race && ch.class ? ' ' : ''}<span style="color: {wowClassColor(ch.class)}">{ch.class ?? ''}</span>{#if ch.race || ch.class} · {/if}{t('game.ilvl')} {ch.item_level}{#if ch.mythic_rating} · M+ {Math.round(ch.mythic_rating)}{/if}{#if ch.achievement_points} · {t('game.achievementPoints')} {ch.achievement_points}{/if}
								</p>
							</div>
							{#if rs}
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
