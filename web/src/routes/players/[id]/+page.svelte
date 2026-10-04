<script lang="ts">
	import { playingGames } from '$lib/playing';
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { api, type Profile, type Session, type Achievement, type Game } from '$lib/api';
	import { formatDuration, timeAgo, formatDate } from '$lib/format';
	import { t, getLocale } from '$lib/i18n';
	import Avatar from '$lib/components/Avatar.svelte';
	import StatusBadge from '$lib/components/StatusBadge.svelte';

	let profile = $state<Profile | null>(null);
	let sessions = $state<Session[]>([]);
	let nextCursor = $state<string | undefined>(undefined);
	let loading = $state(true);
	let error = $state('');
	// A Switch friend code is copied to be typed into a Switch: show it worked.
	let copiedCode = $state('');
	async function copyFriendCode(code: string) {
		try {
			await navigator.clipboard.writeText(code);
			copiedCode = code;
			setTimeout(() => (copiedCode = ''), 1500);
		} catch {
			/* clipboard refused: the code stays readable on screen */
		}
	}
	let loadingMore = $state(false);

	let achievements = $state<Achievement[]>([]);
	let achGames = $state<{ game: Game; count: number }[]>([]);
	let achGameFilter = $state('');
	let achOffset = $state(0);
	let achHasMore = $state(false);
	let achLoading = $state(false);

	// Steam serves 404 for a fair share of achievement icons it nonetheless
	// names in its own API (measured on this very profile: 2 of 40 sampled).
	// The URL existing is therefore not a promise that the image does, and
	// without this the browser draws its broken-image glyph next to a perfectly
	// good achievement. Keyed by the same identity as the loop, so one dead
	// icon never hides another trophy.
	let brokenIcons = $state(new Set<string>());
	const ACH_LIMIT = 24;

	let libraryOpen = $state(false);
	let library = $state<{ game: Game; source: string }[]>([]);
	let libraryLoading = $state(false);

	const id = $derived(page.params.id ?? '');
	let topSeconds = $derived(Math.max(1, ...(profile?.game_stats ?? []).map((g) => g.total_seconds)));

	// Recent sessions grouped by local calendar day (the viewer's own timezone,
	// straight from the browser). Sessions arrive newest-first, so consecutive
	// runs of the same day stay contiguous and ordered.
	type SessionGroup = { key: string; iso: string; items: Session[] };
	const sessionGroups = $derived.by((): SessionGroup[] => {
		const groups: SessionGroup[] = [];
		let current: SessionGroup | null = null;
		for (const s of sessions) {
			const d = new Date(s.started_at);
			const key = `${d.getFullYear()}-${d.getMonth()}-${d.getDate()}`;
			if (!current || current.key !== key) {
				current = { key, iso: s.started_at, items: [] };
				groups.push(current);
			}
			current.items.push(s);
		}
		return groups;
	});

	const startOfLocalDay = (d: Date) => new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime();
	function dayLabel(iso: string): string {
		const d = new Date(iso);
		const diffDays = Math.round((startOfLocalDay(new Date()) - startOfLocalDay(d)) / 86400000);
		if (diffDays === 0) return t('profile.today');
		if (diffDays === 1) return t('profile.yesterday');
		return d.toLocaleDateString(getLocale(), { weekday: 'long', day: 'numeric', month: 'long' });
	}
	function timeOfDay(iso: string): string {
		return new Date(iso).toLocaleTimeString(getLocale(), { hour: '2-digit', minute: '2-digit' });
	}

	const PROVIDER_META: Record<string, { label: string; color: string; path: string }> = {
		steam: {
			label: 'Steam',
			color: '#66c0f4',
			path: 'M11.979 0C5.678 0 .511 4.86.022 11.037l6.432 2.658c.545-.371 1.203-.59 1.912-.59.063 0 .125.004.188.006l2.861-4.142V8.91c0-2.495 2.028-4.524 4.524-4.524 2.494 0 4.524 2.031 4.524 4.527s-2.03 4.525-4.524 4.525h-.105l-4.076 2.911c0 .052.004.105.004.159 0 1.875-1.515 3.396-3.39 3.396-1.635 0-3.016-1.173-3.331-2.727L.436 15.27C1.862 20.307 6.486 24 11.979 24c6.627 0 11.999-5.373 11.999-12S18.605 0 11.979 0zM7.54 18.21l-1.473-.61c.262.543.714.999 1.314 1.25 1.297.539 2.793-.076 3.332-1.375.263-.63.264-1.319.005-1.949s-.75-1.121-1.377-1.383c-.624-.26-1.29-.249-1.878-.03l1.523.63c.956.4 1.409 1.5 1.009 2.455-.397.957-1.497 1.41-2.454 1.012H7.54zm11.415-9.303c0-1.662-1.353-3.015-3.015-3.015-1.665 0-3.015 1.353-3.015 3.015 0 1.665 1.35 3.015 3.015 3.015 1.663 0 3.015-1.35 3.015-3.015zm-5.273-.005c0-1.252 1.013-2.266 2.265-2.266 1.249 0 2.266 1.014 2.266 2.266 0 1.251-1.017 2.265-2.266 2.265-1.253 0-2.265-1.014-2.265-2.265z'
		},
		battlenet: {
			label: 'Battle.net',
			color: '#148EFF',
			path: 'M18.94 8.296C15.9 6.892 11.534 6 7.426 6.332c.206-1.36.714-2.308 1.548-2.508 1.148-.275 2.4.48 3.594 1.854.782.102 1.71.28 2.355.429C12.747 2.013 9.828-.282 7.607.565c-1.688.644-2.553 2.97-2.448 6.094-2.2.468-3.915 1.3-5.013 2.495-.056.065-.181.227-.137.305.034.058.146-.008.194-.04 1.274-.89 2.904-1.373 5.027-1.676.303 3.333 1.713 7.56 4.055 10.952-1.28.502-2.356.536-2.946-.087-.812-.856-.784-2.318-.19-4.04a26.764 26.764 0 0 1-.807-2.254c-2.459 3.934-2.986 7.61-1.143 9.11 1.402 1.14 3.847.725 6.502-.926 1.505 1.672 3.083 2.74 4.667 3.094.084.015.287.043.332-.034.034-.06-.08-.124-.131-.149-1.408-.657-2.64-1.828-3.964-3.515 2.735-1.929 5.691-5.263 7.457-8.988 1.076.86 1.64 1.773 1.398 2.595-.336 1.131-1.615 1.84-3.403 2.185a27.697 27.697 0 0 1-1.548 1.826c4.634.16 8.08-1.22 8.458-3.565.286-1.786-1.295-3.696-4.053-5.17.696-2.139.832-4.04.346-5.588-.029-.08-.106-.27-.196-.27-.068 0-.067.13-.063.187.135 1.547-.263 3.2-1.062 5.19zm-8.533 9.869c-1.96-3.145-3.09-6.849-3.082-10.594 3.702-.124 7.474.748 10.714 2.627-1.743 3.269-4.385 6.1-7.633 7.966h.001z'
		},
		nintendo: {
			label: 'Nintendo Switch',
			color: '#E60012',
			path: 'M14.176 24h3.674c3.376 0 6.15-2.774 6.15-6.15V6.15C24 2.775 21.226 0 17.85 0H14.1c-.074 0-.15.074-.15.15v23.7c-.001.076.075.15.226.15zm4.574-13.199c1.351 0 2.399 1.125 2.399 2.398 0 1.352-1.125 2.4-2.399 2.4-1.35 0-2.4-1.049-2.4-2.4-.075-1.349 1.05-2.398 2.4-2.398zM11.4 0H6.15C2.775 0 0 2.775 0 6.15v11.7C0 21.226 2.775 24 6.15 24h5.25c.074 0 .15-.074.15-.149V.15c.001-.076-.075-.15-.15-.15zM9.676 22.051H6.15c-2.326 0-4.201-1.875-4.201-4.201V6.15c0-2.326 1.875-4.201 4.201-4.201H9.6l.076 20.102zM3.75 7.199c0 1.275.975 2.25 2.25 2.25s2.25-.975 2.25-2.25c0-1.273-.975-2.25-2.25-2.25s-2.25.977-2.25 2.25z'
		},
		psn: {
			label: 'PlayStation',
			color: '#0070D1',
			path: 'M8.984 2.596v17.547l3.915 1.261V6.688c0-.69.304-1.151.794-.991.636.18.76.814.76 1.505v5.875c2.441 1.193 4.362-.002 4.362-3.152 0-3.237-1.126-4.675-4.438-5.827-1.307-.448-3.728-1.186-5.39-1.502zm4.656 16.241l6.296-2.275c.715-.258.826-.625.246-.818-.586-.192-1.637-.139-2.357.123l-4.205 1.5V14.98l.24-.085s1.201-.42 2.913-.615c1.696-.18 3.785.03 5.437.661 1.848.601 2.04 1.472 1.576 2.072-.465.6-1.622 1.036-1.622 1.036l-8.544 3.107V18.86zM1.807 18.6c-1.9-.545-2.214-1.668-1.352-2.32.801-.586 2.16-1.052 2.16-1.052l5.615-2.013v2.313L4.205 17c-.705.271-.825.632-.239.826.586.195 1.637.15 2.343-.12L8.247 17v2.074c-.12.03-.256.044-.39.073-1.939.331-3.996.196-6.038-.479z'
		},
		epic: {
			label: 'Epic Games',
			color: '#c8c8c8',
			path: 'M3.537 0C2.165 0 1.66.506 1.66 1.879V18.44a4.262 4.262 0 00.02.433c.031.3.037.59.316.92.027.033.311.245.311.245.153.075.258.13.43.2l8.335 3.491c.433.199.614.276.928.27h.002c.314.006.495-.071.928-.27l8.335-3.492c.172-.07.277-.124.43-.2 0 0 .284-.211.311-.243.28-.33.285-.621.316-.92a4.261 4.261 0 00.02-.434V1.879c0-1.373-.506-1.88-1.878-1.88zm13.366 3.11h.68c1.138 0 1.688.553 1.688 1.696v1.88h-1.374v-1.8c0-.369-.17-.54-.523-.54h-.235c-.367 0-.537.17-.537.539v5.81c0 .369.17.54.537.54h.262c.353 0 .523-.171.523-.54V8.619h1.373v2.143c0 1.144-.562 1.71-1.7 1.71h-.694c-1.138 0-1.7-.566-1.7-1.71V4.82c0-1.144.562-1.709 1.7-1.709zm-12.186.08h3.114v1.274H6.117v2.603h1.648v1.275H6.117v2.774h1.74v1.275h-3.14zm3.816 0h2.198c1.138 0 1.7.564 1.7 1.708v2.445c0 1.144-.562 1.71-1.7 1.71h-.799v3.338h-1.4zm4.53 0h1.4v9.201h-1.4zm-3.13 1.235v3.392h.575c.354 0 .523-.171.523-.54V4.965c0-.368-.17-.54-.523-.54zm-3.74 10.147a1.708 1.708 0 01.591.108 1.745 1.745 0 01.49.299l-.452.546a1.247 1.247 0 00-.308-.195.91.91 0 00-.363-.068.658.658 0 00-.28.06.703.703 0 00-.224.163.783.783 0 00-.151.243.799.799 0 00-.056.299v.008a.852.852 0 00.056.31.7.7 0 00.157.245.736.736 0 00.238.16.774.774 0 00.303.058.79.79 0 00.445-.116v-.339h-.548v-.565H7.37v1.255a2.019 2.019 0 01-.524.307 1.789 1.789 0 01-.683.123 1.642 1.642 0 01-.602-.107 1.46 1.46 0 01-.478-.3 1.371 1.371 0 01-.318-.455 1.438 1.438 0 01-.115-.58v-.008a1.426 1.426 0 01.113-.57 1.449 1.449 0 01.312-.46 1.418 1.418 0 01.474-.309 1.58 1.58 0 01.598-.111 1.708 1.708 0 01.045 0zm11.963.008a2.006 2.006 0 01.612.094 1.61 1.61 0 01.507.277l-.386.546a1.562 1.562 0 00-.39-.205 1.178 1.178 0 00-.388-.07.347.347 0 00-.208.052.154.154 0 00-.07.127v.008a.158.158 0 00.022.084.198.198 0 00.076.066.831.831 0 00.147.06c.062.02.14.04.236.061a3.389 3.389 0 01.43.122 1.292 1.292 0 01.328.17.678.678 0 01.207.24.739.739 0 01.071.337v.008a.865.865 0 01-.081.382.82.82 0 01-.229.285 1.032 1.032 0 01-.353.18 1.606 1.606 0 01-.46.061 2.16 2.16 0 01-.71-.116 1.718 1.718 0 01-.593-.346l.43-.514c.277.223.578.335.9.335a.457.457 0 00.236-.05.157.157 0 00.082-.142v-.008a.15.15 0 00-.02-.077.204.204 0 00-.073-.066.753.753 0 00-.143-.062 2.45 2.45 0 00-.233-.062 5.036 5.036 0 01-.413-.113 1.26 1.26 0 01-.331-.16.72.72 0 01-.222-.243.73.73 0 01-.082-.36v-.008a.863.863 0 01.074-.359.794.794 0 01.214-.283 1.007 1.007 0 01.34-.185 1.423 1.423 0 01.448-.066 2.006 2.006 0 01.025 0zm-9.358.025h.742l1.183 2.81h-.825l-.203-.499H8.623l-.198.498h-.81zm2.197.02h.814l.663 1.08.663-1.08h.814v2.79h-.766v-1.602l-.711 1.091h-.016l-.707-1.083v1.593h-.754zm3.469 0h2.235v.658h-1.473v.422h1.334v.61h-1.334v.442h1.493v.658h-2.255zm-5.3.897l-.315.793h.624zm-1.145 5.19h8.014l-4.09 1.348z'
		},
		xbox: {
			label: 'Xbox',
			color: '#107C10',
			path: 'M4.102 21.033C6.211 22.881 8.977 24 12 24c3.026 0 5.789-1.119 7.902-2.967 1.877-1.912-4.316-8.709-7.902-11.417-3.582 2.708-9.779 9.505-7.898 11.417zm11.16-14.406c2.5 2.961 7.484 10.313 6.076 12.912C23.002 17.48 24 14.861 24 12.004c0-3.34-1.365-6.362-3.57-8.536 0 0-.027-.022-.082-.042-.063-.022-.152-.045-.281-.045-.592 0-1.985.434-4.805 3.246zM3.654 3.426c-.057.02-.082.041-.086.042C1.365 5.642 0 8.664 0 12.004c0 2.854.998 5.473 2.661 7.533-1.401-2.605 3.579-9.951 6.08-12.91-2.82-2.813-4.216-3.245-4.806-3.245-.131 0-.223.021-.281.046v-.002zM12 3.551S9.055 1.828 6.755 1.746c-.903-.033-1.454.295-1.521.339C7.379.646 9.659 0 11.984 0H12c2.334 0 4.605.646 6.766 2.085-.068-.046-.615-.372-1.52-.339C14.946 1.828 12 3.545 12 3.545v.006z'
		}
	};

	onMount(load);

	async function load() {
		loading = true;
		error = '';
		try {
			profile = await api.getProfile(id);
			const res = await api.getSessions(id);
			sessions = res.sessions;
			nextCursor = res.next_cursor;
			await loadAchievements(true);
		} catch (e) {
			error = e instanceof Error ? e.message : t('profile.loadError');
		} finally {
			loading = false;
		}
	}

	async function loadAchievements(reset: boolean) {
		achLoading = true;
		try {
			const off = reset ? 0 : achOffset;
			const r = await api.getUserAchievements(id, { gameId: achGameFilter || undefined, offset: off, limit: ACH_LIMIT });
			achievements = reset ? r.achievements : [...achievements, ...r.achievements];
			achGames = r.games;
			achHasMore = r.has_more;
			achOffset = off + r.achievements.length;
		} finally {
			achLoading = false;
		}
	}

	async function toggleLibrary() {
		if (libraryOpen) {
			libraryOpen = false;
			return;
		}
		libraryOpen = true;
		if (library.length > 0) return; // already loaded
		libraryLoading = true;
		try {
			const res = await api.userGames(id);
			library = res.games;
		} finally {
			libraryLoading = false;
		}
	}

	async function loadMore() {
		if (!nextCursor) return;
		loadingMore = true;
		try {
			const res = await api.getSessions(id, nextCursor);
			sessions = [...sessions, ...res.sessions];
			nextCursor = res.next_cursor;
		} finally {
			loadingMore = false;
		}
	}
</script>

{#if loading}
	<p class="text-[var(--color-muted)]">{t('profile.loading')}</p>
{:else if error}
	<p class="text-[var(--color-magenta)]">{error}</p>
{:else if profile}
	<!-- Profile Header -->
	<div class="pd-card pd-glow mb-8 flex items-center gap-5 p-6">
		<div class="shrink-0">
			<Avatar username={profile.user.username} url={profile.user.avatar_url} size={80} />
		</div>
		<div class="min-w-0 flex-1">
			<div class="flex items-center gap-3">
				<h1 class="pd-heading text-3xl text-[var(--color-text)]">{profile.user.username}</h1>
				{#if profile.user.role === 'admin'}
					<span class="pd-cut-sm bg-[var(--color-brand)]/20 px-2 py-0.5 font-display text-xs font-bold italic uppercase tracking-widest text-[var(--color-brand-bright)]">admin</span>
				{/if}
			</div>
			<div class="mt-1.5 flex items-center gap-3 text-sm text-[var(--color-muted)]">
				<StatusBadge status={profile.presence.status} />
				<span>- {t('profile.memberSince')} {formatDate(profile.user.created_at)}</span>
			</div>
			{#if profile.presence.status === 'in_game' && profile.presence.game}
				<p class="mt-1 font-display text-sm font-bold italic text-[var(--color-brand-bright)]">
					{t('profile.playing')} {playingGames(profile.presence).map((p) => p.game.name).join(' + ')}
				</p>
			{/if}
		</div>
		<div class="ml-auto flex shrink-0 gap-6 text-right">
			<div>
				<p class="font-display text-2xl font-bold italic text-[var(--color-brand-bright)]">{formatDuration(profile.total_seconds)}</p>
				<p class="text-xs uppercase tracking-wider text-[var(--color-muted)]">{t('profile.totalTracked')}</p>
			</div>
			{#if profile.achievement_count > 0}
				<div>
					<p class="font-display text-2xl font-bold italic text-[var(--color-gold)]">{profile.achievement_count}</p>
					<p class="text-xs uppercase tracking-wider text-[var(--color-muted)]">{t('profile.achievements', { count: profile.achievement_count })}</p>
				</div>
			{/if}
		</div>
	</div>

	{#if profile.connections.length > 0}
		<div class="mb-6 flex flex-wrap gap-2">
			{#each profile.connections as conn (conn.provider)}
				{@const meta = PROVIDER_META[conn.provider]}
				{#if conn.profile_url}
					<a
						href={conn.profile_url}
						target="_blank"
						rel="noreferrer noopener"
						class="btn-pd btn-pd-ghost pd-cut-sm inline-flex items-center gap-2 px-3 py-1.5 text-sm"
					>
						{#if meta}
							<svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true" class="h-5 w-5 shrink-0" style="color: {meta.color}"><path d={meta.path} /></svg>
						{/if}
						<span class="capitalize">{meta?.label ?? conn.provider}</span>
						{#if conn.display_name}
							<span class="text-[var(--color-muted)]">- {conn.display_name}</span>
						{/if}
					</a>
				{:else}
					<div class="pd-cut-sm inline-flex cursor-default items-center gap-2 border border-[var(--color-border)] px-3 py-1.5 text-sm">
						{#if meta}
							<svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true" class="h-5 w-5 shrink-0" style="color: {meta.color}"><path d={meta.path} /></svg>
						{/if}
						<span class="capitalize">{meta?.label ?? conn.provider}</span>
						{#if conn.display_name}
							<span class="text-[var(--color-muted)]">- {conn.display_name}</span>
						{/if}
						{#if conn.friend_code}
							<span class="font-mono text-[var(--color-text)]">· {conn.friend_code}</span>
							<button
								type="button"
								class="text-xs text-[var(--color-muted)] underline hover:text-[var(--color-text)]"
								onclick={() => copyFriendCode(conn.friend_code ?? '')}
							>
								{copiedCode === conn.friend_code ? t('profile.copied') : t('profile.copyFriendCode')}
							</button>
						{/if}
					</div>
				{/if}
			{/each}
		</div>
	{/if}

	<!-- Hours per game -->
	<section class="mb-6">
		<h2 class="pd-heading mb-4 text-sm text-[var(--color-brand-bright)]">{t('profile.hoursPerGame')}</h2>
		{#if profile.game_stats.length === 0}
			<p class="text-sm text-[var(--color-muted)]">{t('profile.noSessionsYet')}</p>
		{:else}
			<div class="pd-card flex flex-col gap-1 p-4">
				{#each profile.game_stats.slice(0, 10) as stat, i (stat.game.id)}
					<a
						href="/games/{stat.game.slug}"
						class="flex items-center gap-3 px-2 py-1.5 transition-colors hover:bg-[var(--color-surface-2)]"
					>
						<span class="w-5 shrink-0 text-center font-display text-xs font-bold italic text-[var(--color-muted)]">{i + 1}</span>
						{#if stat.game.icon_url}
							<img src={stat.game.icon_url} alt="" class="h-6 w-6 shrink-0 rounded" />
						{/if}
						<span class="min-w-0 flex-1 truncate text-sm sm:w-40 sm:flex-none">{stat.game.name}</span>
						<!-- The bar is the one thing here that can go: on a phone it
						     would be crushed to a few pixels and say nothing, while the
						     name and the figure both still have to be readable. -->
						<div class="hidden h-2 flex-1 overflow-hidden bg-[var(--color-bg)] sm:block" style="clip-path: polygon(4px 0, 100% 0, calc(100% - 4px) 100%, 0 100%)">
							<div
								class="h-full bg-[var(--color-brand)]"
								style="width:{Math.max(2, (stat.total_seconds / topSeconds) * 100)}%; clip-path: polygon(4px 0, 100% 0, calc(100% - 4px) 100%, 0 100%)"
							></div>
						</div>
						<span class="w-20 shrink-0 text-right font-display text-sm font-bold italic text-[var(--color-text)]">
							{formatDuration(stat.total_seconds)}
						</span>
					</a>
				{/each}
			</div>
		{/if}
	</section>

	<!-- Full owned game library -->
	<section class="mb-6">
		<button
			onclick={toggleLibrary}
			class="btn-pd btn-pd-ghost mb-4 flex w-full items-center justify-between px-4 py-2.5 text-left"
		>
			<span class="pd-heading text-sm text-[var(--color-brand-bright)]">{t('profile.allOwnedGames')}</span>
			<svg
				viewBox="0 0 24 24"
				fill="none"
				stroke="currentColor"
				stroke-width="2"
				class="h-4 w-4 shrink-0 text-[var(--color-muted)] transition-transform {libraryOpen ? 'rotate-180' : ''}"
				aria-hidden="true"
			>
				<path stroke-linecap="round" stroke-linejoin="round" d="M19 9l-7 7-7-7" />
			</svg>
		</button>
		{#if libraryOpen}
			{#if libraryLoading}
				<p class="text-sm text-[var(--color-muted)]">{t('common.loading')}</p>
			{:else if library.length === 0}
				<p class="text-sm text-[var(--color-muted)]">{t('profile.noSessionsYet')}</p>
			{:else}
				<div class="pd-card grid gap-2 p-4 sm:grid-cols-2 lg:grid-cols-3">
					{#each library as entry (entry.game.id)}
						{@const badge =
							entry.source === 'steam'
								? { label: 'Steam', color: '#66c0f4', bg: 'rgba(102,192,244,0.15)' }
								: entry.source === 'battlenet'
									? { label: 'Battle.net', color: '#148EFF', bg: 'rgba(20,142,255,0.15)' }
									: entry.source === 'psn'
										? { label: 'PlayStation', color: '#3d9bff', bg: 'rgba(0,112,209,0.15)' }
										: entry.source === 'nintendo'
											? { label: 'Nintendo Switch', color: '#ff4d5a', bg: 'rgba(230,0,18,0.15)' }
											: entry.source === 'epic'
												? { label: 'Epic Games', color: '#d4d4d4', bg: 'rgba(212,212,212,0.12)' }
									: { label: t('profile.playedBadge'), color: 'var(--color-muted)', bg: 'var(--color-surface-2)' }}
						<a
							href="/players/{id}/games/{entry.game.slug}"
							class="flex items-center gap-3 rounded px-2 py-1.5 transition-colors hover:bg-[var(--color-surface-2)]"
						>
							{#if entry.game.icon_url}
								<img src={entry.game.icon_url} alt="" class="h-7 w-7 shrink-0 rounded" />
							{:else}
								<span class="grid h-7 w-7 shrink-0 place-items-center rounded bg-[var(--color-bg)] text-[var(--color-muted)]">
									<svg viewBox="0 0 24 24" fill="currentColor" class="h-4 w-4" aria-hidden="true"><path d="M21 6H3a1 1 0 0 0-1 1v10a1 1 0 0 0 1 1h18a1 1 0 0 0 1-1V7a1 1 0 0 0-1-1zm-1 10H4V8h16v8zm-8-6a2 2 0 1 0 0 4 2 2 0 0 0 0-4zm-5 2a1 1 0 1 0 0 2 1 1 0 0 0 0-2zm10 0a1 1 0 1 0 0 2 1 1 0 0 0 0-2z"/></svg>
								</span>
							{/if}
							<span class="min-w-0 flex-1 truncate text-sm">{entry.game.name}</span>
							<span
								class="shrink-0 pd-cut-sm px-1.5 py-0.5 font-display text-xs font-bold italic"
								style="background: {badge.bg}; color: {badge.color}"
							>{badge.label}</span>
						</a>
					{/each}
				</div>
			{/if}
		{/if}
	</section>

	<!-- Achievements -->
	<section class="mb-6">
		<div class="mb-4 flex items-center gap-3">
			<h2 class="pd-heading text-sm text-[var(--color-gold)]">{t('profile.achievementsTitle')}</h2>
			<select
				bind:value={achGameFilter}
				onchange={() => loadAchievements(true)}
				class="pd-cut-sm ml-auto border border-[var(--color-border)] bg-[var(--color-bg)] px-2 py-1 text-xs text-[var(--color-text)]"
			>
				<option value="">{t('profile.allGames')}</option>
				{#each achGames as g (g.game.id)}
					<option value={g.game.id}>{g.game.name} ({g.count})</option>
				{/each}
			</select>
		</div>
		{#if achievements.length === 0 && !achLoading}
			<p class="text-sm text-[var(--color-muted)]">{t('profile.noAchievements')}</p>
		{:else}
			<div class="pd-card grid gap-3 p-4 sm:grid-cols-2">
				{#each achievements as a (a.game.id + a.api_name)}
					{@const key = a.game.id + a.api_name}
					<div class="flex items-center gap-3 rounded bg-[var(--color-surface-2)] px-3 py-2">
						{#if a.icon_url && !brokenIcons.has(key)}
							<img
								src={a.icon_url}
								alt=""
								class="h-9 w-9 shrink-0 pd-cut-sm object-cover"
								onerror={() => (brokenIcons = new Set(brokenIcons).add(key))}
							/>
						{:else}
							<span class="grid h-9 w-9 shrink-0 pd-cut-sm place-items-center bg-[var(--color-bg)] font-display text-base text-[var(--color-gold)]">🏆</span>
						{/if}
						<div class="min-w-0">
							<p class="truncate text-sm font-display font-semibold">{a.display_name ?? a.api_name}</p>
							<p class="truncate text-xs text-[var(--color-muted)]">
								{a.game.name} - {timeAgo(a.unlocked_at)}
							</p>
						</div>
					</div>
				{/each}
			</div>
		{/if}
		{#if achHasMore}
			<button
				onclick={() => loadAchievements(false)}
				disabled={achLoading}
				class="btn-pd btn-pd-ghost mt-3 w-full py-2 text-sm disabled:opacity-60"
			>
				{achLoading ? t('profile.loadingMore') : t('profile.loadMore')}
			</button>
		{/if}
	</section>

	<!-- Recent sessions -->
	<section>
		<h2 class="pd-heading mb-4 text-sm text-[var(--color-cyan)]">{t('profile.recentSessions')}</h2>
		{#if sessions.length === 0}
			<p class="text-sm text-[var(--color-muted)]">{t('profile.noSessions')}</p>
		{:else}
			<div class="pd-card overflow-hidden">
				{#each sessionGroups as group (group.key)}
					<div class="bg-[var(--color-surface-2)] px-4 py-1.5 font-display text-xs font-bold uppercase tracking-wide text-[var(--color-muted)]">
						{dayLabel(group.iso)}
					</div>
					<ul class="divide-y divide-[var(--color-border)]">
						{#each group.items as s (s.id)}
							<li class="flex items-center gap-3 px-4 py-2.5 transition-colors hover:bg-[var(--color-surface-2)]">
								{#if s.game.icon_url}
									<img src={s.game.icon_url} alt="" class="h-5 w-5 shrink-0 rounded" />
								{/if}
								<a href="/games/{s.game.slug}" class="flex-1 truncate text-sm hover:text-[var(--color-brand-bright)] hover:underline">{s.game.name}</a>
								{#if !s.ended_at}
									<span class="pd-cut-sm bg-[var(--color-online)]/15 px-2 py-0.5 font-display text-xs font-bold italic uppercase text-[var(--color-online)]">live</span>
								{:else}
									<span class="font-display text-sm font-bold italic text-[var(--color-text)]">{formatDuration(s.duration_seconds ?? 0)}</span>
								{/if}
								<span class="w-12 text-right text-xs text-[var(--color-muted)]">{timeOfDay(s.started_at)}</span>
							</li>
						{/each}
					</ul>
				{/each}
			</div>
			{#if nextCursor}
				<button
					onclick={loadMore}
					disabled={loadingMore}
					class="btn-pd btn-pd-ghost mt-3 w-full py-2 text-sm disabled:opacity-60"
				>
					{loadingMore ? t('profile.loadingMore') : t('profile.loadMore')}
				</button>
			{/if}
		{/if}
	</section>
{/if}
