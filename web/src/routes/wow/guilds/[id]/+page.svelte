<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { api, type WowGuildPage } from '$lib/api';
	import { t, getLocale } from '$lib/i18n';
	import { wowClassColor, wowClassIcon } from '$lib/wow';
	import {
		heat,
		shortName,
		sortRoster,
		whoCrafts,
		wowClassName,
		wowheadURL,
		type RosterSort
	} from '$lib/wowGuild';

	let data = $state<WowGuildPage | null>(null);
	let loading = $state(true);
	let error = $state('');

	const id = $derived(page.params.id ?? '');

	onMount(async () => {
		try {
			data = await api.wowGuild(id);
		} catch (e) {
			error = e instanceof Error ? e.message : t('wowGuild.loadError');
		} finally {
			loading = false;
		}
	});

	// Roster: sort and filter.
	let sortBy = $state<RosterSort>('rank');
	let sortDesc = $state(false);
	let filter = $state('');
	function sortOn(by: RosterSort) {
		if (sortBy === by) sortDesc = !sortDesc;
		else {
			sortBy = by;
			// Levels and last seen read best highest / newest first.
			sortDesc = by === 'level' || by === 'seen';
		}
	}
	const roster = $derived.by(() => {
		const q = filter.trim().toLowerCase();
		const rows = (data?.roster ?? []).filter(
			(r) =>
				!q ||
				r.name.toLowerCase().includes(q) ||
				(r.pseudo ?? '').toLowerCase().includes(q) ||
				(r.username ?? '').toLowerCase().includes(q) ||
				(r.rank ?? '').toLowerCase().includes(q)
		);
		return sortRoster(rows, sortBy, sortDesc);
	});
	const columns: { key: RosterSort | null; label: string }[] = [
		{ key: 'name', label: 'wowGuild.colName' },
		{ key: 'level', label: 'wowGuild.colLevel' },
		{ key: 'class', label: 'wowGuild.colClass' },
		{ key: 'rank', label: 'wowGuild.colRank' },
		{ key: 'pseudo', label: 'wowGuild.colPseudo' },
		{ key: null, label: 'wowGuild.colMain' },
		{ key: null, label: 'wowGuild.colRole' },
		{ key: null, label: 'wowGuild.colAddon' },
		{ key: 'seen', label: 'wowGuild.colSeen' }
	];

	/** Seen within five minutes of the snapshot: online when it was taken. */
	function online(seen?: string): boolean {
		const snap = data?.guild.snapshot_at;
		if (!seen || !snap) return false;
		return new Date(snap).getTime() - new Date(seen).getTime() < 5 * 60_000;
	}
	function seenLabel(seen?: string): string {
		if (!seen) return '–';
		const days = Math.round((Date.now() - new Date(seen).getTime()) / 86_400_000);
		const rtf = new Intl.RelativeTimeFormat(getLocale(), { numeric: 'auto' });
		if (days < 1) {
			const hours = Math.max(0, Math.round((Date.now() - new Date(seen).getTime()) / 3_600_000));
			return rtf.format(-hours, 'hour');
		}
		return rtf.format(-days, 'day');
	}
	function roleLabel(role?: string): string {
		return role ? t(`wowGuild.role${role}`) : '–';
	}

	// Professions.
	let query = $state('');
	const hits = $derived(data ? whoCrafts(data.professions, query) : []);
	let openProf = $state<string | null>(null);

	// Presence.
	const week = $derived(data?.presence.week ?? []);
	const weekMax = $derived(Math.max(0, ...week.flat().map((v) => v ?? 0)));
	const hasPresence = $derived(week.some((d) => d.some((v) => v != null)));
	function day(n: number): string {
		return t(`wowGuild.weekdays.${n}`);
	}
	function fmt(v: number): string {
		return v.toLocaleString(getLocale(), { maximumFractionDigits: 1 });
	}
</script>

<a href="/wow/guilds" class="mb-4 inline-block text-sm text-[var(--color-muted)] hover:text-[var(--color-text)]">
	&larr; {t('wowGuild.title')}
</a>

{#if loading}
	<p class="text-[var(--color-muted)]">{t('wowGuild.loading')}</p>
{:else if error}
	<p class="text-[var(--color-magenta)]">{error}</p>
{:else if data}
	<div class="pd-card pd-glow mb-6 p-5">
		<h1 class="pd-heading text-3xl text-white" style="text-shadow: 0 0 24px rgba(154,108,255,0.7);">
			{data.guild.name}
		</h1>
		<p class="mt-1 text-sm text-[var(--color-muted)]">
			{data.guild.realm}
			{#if data.guild.snapshot_at}
				&middot; {t('wowGuild.snapshot', { date: new Date(data.guild.snapshot_at).toLocaleString(getLocale()) })}
			{/if}
		</p>
		<div class="mt-3 flex gap-6 text-sm">
			<span class="text-[var(--color-cyan)]">{t('wowGuild.rosterCount', { count: data.guild.roster_count })}</span>
			<span class="text-[var(--color-brand-bright)]">{t('wowGuild.addonCount', { count: data.guild.addon_count })}</span>
		</div>
	</div>

	<!-- Roster -->
	<section class="mb-8">
		<div class="mb-3 flex flex-wrap items-center gap-3">
			<h2 class="pd-heading flex items-center gap-2 text-sm text-[var(--color-brand-bright)]">
				<span class="inline-block h-4 w-1 bg-[var(--color-brand)]"></span>
				{t('wowGuild.roster')}
			</h2>
			<input
				type="search"
				bind:value={filter}
				placeholder={t('wowGuild.filter')}
				class="ml-auto w-full max-w-xs rounded border border-[var(--color-border)] bg-[var(--color-surface)] px-3 py-1.5 text-sm text-[var(--color-text)] sm:w-64"
			/>
		</div>
		<div class="pd-card overflow-x-auto">
			<table class="w-full min-w-[820px] border-collapse text-sm">
				<thead>
					<tr class="border-b border-[var(--color-border)]">
						{#each columns as col (col.label)}
							<th class="px-3 py-2 text-left font-display text-xs uppercase tracking-wide text-[var(--color-muted)]"
								aria-sort={col.key && sortBy === col.key ? (sortDesc ? 'descending' : 'ascending') : undefined}>
								{#if col.key}
									{@const key = col.key}
									<button type="button" class="uppercase hover:text-[var(--color-text)]" onclick={() => sortOn(key)}>
										{t(col.label)}{#if sortBy === key}{sortDesc ? ' ▼' : ' ▲'}{/if}
									</button>
								{:else}
									{t(col.label)}
								{/if}
							</th>
						{/each}
					</tr>
				</thead>
				<tbody>
					{#each roster as r (r.name)}
						{@const cls = wowClassName(r.class)}
						<tr class="border-b border-[var(--color-border)]/50 last:border-b-0 hover:bg-[var(--color-surface-2)]">
							<td class="px-3 py-2 whitespace-nowrap">
								<span class="font-semibold" style="color:{wowClassColor(cls)}" title={r.name}>{shortName(r.name)}</span>
							</td>
							<td class="px-3 py-2 tabular-nums">{r.level ?? '–'}</td>
							<td class="px-3 py-2 whitespace-nowrap">
								<span class="inline-flex items-center gap-1.5">
									{#if wowClassIcon(cls)}
										<img src={wowClassIcon(cls)} alt="" class="h-4 w-4 rounded-sm" loading="lazy" />
									{/if}
									{cls ?? '–'}
								</span>
							</td>
							<td class="px-3 py-2 whitespace-nowrap text-[var(--color-muted)]">{r.rank ?? '–'}</td>
							<td class="px-3 py-2 whitespace-nowrap">
								{#if r.user_id}
									<a href="/players/{r.user_id}" class="text-[var(--color-cyan)] hover:underline" title={r.username}>
										{r.pseudo ?? r.username}
									</a>
								{:else}
									{r.pseudo ?? '–'}
								{/if}
							</td>
							<td class="px-3 py-2 whitespace-nowrap text-[var(--color-muted)]" title={r.main}>
								{r.main ? (r.main === r.name ? '★' : shortName(r.main)) : '–'}
							</td>
							<td class="px-3 py-2 whitespace-nowrap">{roleLabel(r.role)}</td>
							<td class="px-3 py-2 whitespace-nowrap text-xs text-[var(--color-muted)]">{r.addon_version ?? '–'}</td>
							<td class="px-3 py-2 whitespace-nowrap text-xs">
								{#if online(r.last_seen_at)}
									<span style="color: var(--color-online);">● {t('wowGuild.online')}</span>
								{:else}
									<span class="text-[var(--color-muted)]" title={r.last_seen_at ? new Date(r.last_seen_at).toLocaleString(getLocale()) : undefined}>
										{seenLabel(r.last_seen_at)}
									</span>
								{/if}
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	</section>

	<!-- Professions -->
	<section class="mb-8">
		<h2 class="pd-heading mb-3 flex items-center gap-2 text-sm text-[var(--color-cyan)]">
			<span class="inline-block h-4 w-1 bg-[var(--color-cyan)]"></span>
			{t('wowGuild.professions')}
		</h2>
		{#if data.professions.length === 0}
			<p class="text-sm text-[var(--color-muted)]">{t('wowGuild.noProfessions')}</p>
		{:else}
			<input
				type="search"
				bind:value={query}
				placeholder={t('wowGuild.search')}
				class="mb-3 w-full max-w-md rounded border border-[var(--color-border)] bg-[var(--color-surface)] px-3 py-1.5 text-sm text-[var(--color-text)]"
			/>
			{#if query.trim().length >= 2}
				{#if hits.length === 0}
					<p class="mb-4 text-sm text-[var(--color-muted)]">{t('wowGuild.noHit', { q: query.trim() })}</p>
				{:else}
					<ul class="pd-card mb-4 divide-y divide-[var(--color-border)]/50">
						{#each hits as h (h.profession + h.id)}
							<li class="flex flex-wrap items-baseline gap-x-3 gap-y-1 px-3 py-2 text-sm">
								<a href={wowheadURL(h.id)} target="_blank" rel="noopener noreferrer" class="font-semibold text-[var(--color-text)] hover:underline">
									{h.name ?? t('wowGuild.unnamed', { id: h.id })}
								</a>
								<span class="text-xs text-[var(--color-muted)]">{h.profession}</span>
								<span class="ml-auto text-[var(--color-cyan)]">
									{h.crafters.map((c) => shortName(c.name) + (c.username ? ` (${c.username})` : '')).join(', ')}
								</span>
							</li>
						{/each}
					</ul>
				{/if}
			{:else if query.trim().length > 0}
				<p class="mb-4 text-xs text-[var(--color-muted)]">{t('wowGuild.searchHint')}</p>
			{/if}
			<div class="grid gap-3 sm:grid-cols-2">
				{#each data.professions as p (p.profession)}
					<div class="pd-card p-4">
						<div class="mb-2 flex items-baseline gap-2">
							<h3 class="font-display text-base font-semibold text-[var(--color-text)]">{p.profession}</h3>
							<span class="text-xs text-[var(--color-muted)]">{t('wowGuild.recipes', { count: p.recipes.length })}</span>
						</div>
						<ul class="mb-2 flex flex-col gap-1 text-sm">
							{#each p.crafters as c (c.name)}
								<li class="flex items-center gap-2">
									<span title={c.name}>{shortName(c.name)}</span>
									{#if c.user_id}
										<a href="/players/{c.user_id}" class="text-xs text-[var(--color-cyan)] hover:underline">{c.username}</a>
									{/if}
									<span class="ml-auto tabular-nums text-[var(--color-muted)]">
										{c.level ?? '?'}{#if c.max} / {c.max}{/if}
									</span>
								</li>
							{/each}
						</ul>
						{#if p.recipes.length}
							<button type="button" class="text-xs text-[var(--color-brand-bright)] hover:underline"
								onclick={() => (openProf = openProf === p.profession ? null : p.profession)}>
								{openProf === p.profession ? t('wowGuild.hideRecipes') : t('wowGuild.showRecipes')}
							</button>
							{#if openProf === p.profession}
								<ul class="mt-2 max-h-72 overflow-y-auto text-xs">
									{#each p.recipes as r (r.id)}
										<li class="flex gap-2 py-0.5">
											<a href={wowheadURL(r.id)} target="_blank" rel="noopener noreferrer" class="hover:underline">
												{r.name ?? t('wowGuild.unnamed', { id: r.id })}
											</a>
											<span class="ml-auto text-[var(--color-muted)]">{r.crafters.map(shortName).join(', ')}</span>
										</li>
									{/each}
								</ul>
							{/if}
						{/if}
					</div>
				{/each}
			</div>
		{/if}
	</section>

	<!-- Presence -->
	<section class="mb-8">
		<h2 class="pd-heading mb-3 flex items-center gap-2 text-sm text-[var(--color-gold)]">
			<span class="inline-block h-4 w-1 bg-[var(--color-gold)]"></span>
			{t('wowGuild.presence')}
		</h2>
		{#if !hasPresence}
			<p class="text-sm text-[var(--color-muted)]">{t('wowGuild.noPresence')}</p>
		{:else}
			<p class="mb-3 text-xs text-[var(--color-muted)]">
				{t('wowGuild.presenceHint', { weeks: data.presence.weeks, days: data.presence.days })}
			</p>
			{#if data.presence.best.length}
				<div class="mb-4 flex flex-wrap items-center gap-2 text-sm">
					<span class="text-xs uppercase tracking-wide text-[var(--color-muted)]">{t('wowGuild.bestSlots')}</span>
					{#each data.presence.best as s (s.weekday * 24 + s.hour)}
						<span class="pd-cut-sm bg-[var(--color-surface-2)] px-2 py-1">
							{day(s.weekday)} {s.hour}h &middot;
							<span class="text-[var(--color-gold)]">{fmt(s.average)}</span>
							<span class="text-xs text-[var(--color-muted)]">{t('wowGuild.members')}</span>
						</span>
					{/each}
				</div>
			{/if}
			<div class="pd-card overflow-x-auto p-3">
				<table class="border-collapse text-[10px]">
					<thead>
						<tr>
							<th></th>
							{#each Array.from({ length: 24 }, (_, h) => h) as h (h)}
								<th class="w-6 text-center font-normal text-[var(--color-muted)]">{h}</th>
							{/each}
						</tr>
					</thead>
					<tbody>
						{#each week as hours, wd (wd)}
							<tr>
								<th class="pr-2 text-left font-normal text-[var(--color-muted)]">{day(wd)}</th>
								{#each hours as v, h (h)}
									<td class="h-6 w-6 border border-[var(--color-bg)] p-0"
										style="background-color: {v == null
											? 'transparent'
											: `color-mix(in srgb, var(--color-gold) ${Math.round(10 + heat(v, weekMax) * 90)}%, var(--color-surface-2))`};"
										title="{day(wd)} {h}h: {v == null ? '–' : fmt(v)}"
									></td>
								{/each}
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		{/if}
	</section>
{/if}
