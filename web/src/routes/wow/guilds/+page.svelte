<script lang="ts">
	import { onMount } from 'svelte';
	import { api, type WowGuild } from '$lib/api';
	import { t, getLocale } from '$lib/i18n';

	let guilds = $state<WowGuild[]>([]);
	let loading = $state(true);
	let error = $state('');

	onMount(async () => {
		try {
			guilds = await api.wowGuilds();
		} catch (e) {
			error = e instanceof Error ? e.message : t('wowGuild.loadError');
		} finally {
			loading = false;
		}
	});
</script>

<a href="/games/world-of-warcraft-forever" class="mb-4 inline-block text-sm text-[var(--color-muted)] hover:text-[var(--color-text)]">
	&larr; {t('wowGuild.back')}
</a>
<h1 class="pd-heading mb-6 text-2xl text-[var(--color-text)]">{t('wowGuild.title')}</h1>

{#if loading}
	<p class="text-[var(--color-muted)]">{t('wowGuild.loading')}</p>
{:else if error}
	<p class="text-[var(--color-magenta)]">{error}</p>
{:else if guilds.length === 0}
	<p class="text-sm text-[var(--color-muted)]">{t('wowGuild.none')}</p>
{:else}
	<ul class="flex flex-col gap-2">
		{#each guilds as g (g.id)}
			<a href="/wow/guilds/{g.id}" class="pd-card flex flex-wrap items-center gap-x-4 gap-y-1 p-4 transition-colors hover:border-[var(--color-brand)]">
				<span class="font-display text-lg font-semibold text-[var(--color-text)]">{g.name}</span>
				<span class="text-sm text-[var(--color-muted)]">{g.realm}</span>
				<span class="ml-auto text-sm text-[var(--color-cyan)]">{t('wowGuild.rosterCount', { count: g.roster_count })}</span>
				<span class="text-sm text-[var(--color-brand-bright)]">{t('wowGuild.addonCount', { count: g.addon_count })}</span>
				{#if g.snapshot_at}
					<span class="w-full text-xs text-[var(--color-muted)]">
						{t('wowGuild.snapshot', { date: new Date(g.snapshot_at).toLocaleString(getLocale()) })}
					</span>
				{/if}
			</a>
		{/each}
	</ul>
{/if}
