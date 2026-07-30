<script lang="ts">
	import { onMount } from 'svelte';
	import { api, type CatalogStatus } from '$lib/api';
	import { formatDate } from '$lib/format';
	import { t } from '$lib/i18n';

	let status = $state<CatalogStatus | null>(null);
	let error = $state('');
	let syncing = $state(false);
	let synced = $state<number | null>(null);

	onMount(load);

	async function load() {
		try {
			status = await api.getCatalogStatus();
		} catch (e) {
			error = e instanceof Error ? e.message : t('admin.errorLoad');
		}
	}

	async function sync() {
		syncing = true;
		error = '';
		synced = null;
		try {
			synced = (await api.syncCatalog()).upserted;
			await load();
		} catch (e) {
			error = e instanceof Error ? e.message : t('admin.errorGeneric');
		} finally {
			syncing = false;
		}
	}
</script>

<h1 class="pd-heading mb-6 text-2xl text-[var(--color-text)]">{t('admin.catalog.title')}</h1>

<p class="mb-4 text-sm text-[var(--color-muted)]">{t('admin.catalog.intro')}</p>

{#if error}<p class="mb-3 text-sm text-red-500">{error}</p>{/if}

<div class="pd-card p-4">
	{#if status === null}
		<p class="text-sm text-[var(--color-muted)]">{t('admin.loading')}</p>
	{:else}
		<p class="font-display font-semibold text-[var(--color-text)]">
			{t('admin.catalog.gamesCount', { count: status.games })}
		</p>
		<p class="mt-1 text-xs text-[var(--color-muted)]">
			{#if status.synced_at}
				{t('admin.catalog.lastSync', { date: formatDate(status.synced_at) })}
			{:else}
				{t('admin.catalog.neverSynced')}
			{/if}
		</p>

		<div class="mt-4 flex items-center gap-3">
			<button class="btn-pd px-3 py-1.5 text-sm" disabled={syncing} onclick={sync}>
				{syncing ? t('admin.catalog.syncing') : t('admin.catalog.sync')}
			</button>
			{#if synced !== null}
				<span class="text-xs text-[var(--color-gold)]">
					{t('admin.catalog.syncDone', { count: synced })}
				</span>
			{/if}
		</div>

		<p class="mt-3 text-xs text-[var(--color-muted)]">{t('admin.catalog.syncHint')}</p>
	{/if}
</div>
