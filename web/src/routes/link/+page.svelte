<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { api } from '$lib/api';
	import { t } from '$lib/i18n';

	let code = $state('');
	let manualCode = $state('');
	let device = $state<{ device_name: string; platform: string } | null>(null);
	let loading = $state(true);
	let error = $state('');
	let approved = $state(false);
	let busy = $state(false);

	const platformLabel: Record<string, string> = {
		windows: 'Windows',
		macos: 'macOS',
		linux: 'Linux'
	};

	onMount(() => {
		code = page.url.searchParams.get('code') ?? '';
		if (code) lookup(code);
		else loading = false;
	});

	async function lookup(c: string) {
		loading = true;
		error = '';
		device = null;
		try {
			device = await api.getPairInfo(c.trim().toUpperCase());
			code = c.trim().toUpperCase();
		} catch (e) {
			error = e instanceof Error ? e.message : t('link.errorInvalidCode');
		} finally {
			loading = false;
		}
	}

	async function approve() {
		busy = true;
		error = '';
		try {
			await api.approvePair(code);
			approved = true;
		} catch (e) {
			error = e instanceof Error ? e.message : t('link.errorApprovalFailed');
		} finally {
			busy = false;
		}
	}
</script>

<div class="mx-auto max-w-md px-4 py-10">
	<h1 class="pd-heading mb-8 text-2xl text-[var(--color-brand-bright)]">{t('link.title')}</h1>

	{#if approved}
		<div class="pd-card p-6 text-center" style="border-color: color-mix(in srgb, var(--color-online) 40%, transparent);">
			<p class="pd-heading text-lg text-[var(--color-online)]">{t('link.successTitle')}</p>
			<p class="mt-3 text-sm text-[var(--color-muted)]">
				{t('link.successBody')}
			</p>
		</div>
	{:else if loading}
		<p class="text-[var(--color-muted)]">{t('common.loading')}</p>
	{:else if device}
		<div class="pd-card pd-glow p-6">
			<p class="mb-2 text-xs uppercase tracking-widest text-[var(--color-muted)]">
				{t('link.deviceWantsToLink')}
			</p>
			<p class="font-display mb-1 text-xl font-bold text-[var(--color-text)]">{device.device_name}</p>
			<p class="mb-1 text-sm text-[var(--color-muted)]">
				{platformLabel[device.platform] ?? device.platform}
			</p>

			<div class="my-5 flex flex-col items-center gap-1">
				<span class="text-xs uppercase tracking-widest text-[var(--color-muted)]">{t('link.pairingCode')}</span>
				<span
					class="pd-cut-sm font-display inline-block bg-[var(--color-surface-2)] px-6 py-3 text-3xl font-extrabold italic tracking-[0.25em] text-[var(--color-brand-bright)]"
				>
					{code}
				</span>
			</div>

			<p class="mb-5 text-xs text-[var(--color-muted)]">
				{t('link.approveWarning')}
			</p>

			{#if error}<p class="mb-3 text-sm text-[var(--color-magenta)]">{error}</p>{/if}

			<button onclick={approve} disabled={busy} class="btn-pd violet w-full">
				{busy ? t('link.linking') : t('link.approve')}
			</button>
		</div>
	{:else}
		<div class="pd-card p-6">
			{#if error}<p class="mb-3 text-sm text-[var(--color-magenta)]">{error}</p>{/if}

			<label class="flex flex-col gap-2 text-xs uppercase tracking-widest text-[var(--color-muted)]">
				{t('link.enterCode')}
				<input
					bind:value={manualCode}
					placeholder="XXXX-XXXX"
					class="pd-cut-sm border border-[var(--color-border)] bg-[var(--color-bg)] px-3 py-3 text-center font-mono text-lg uppercase tracking-[0.2em] text-[var(--color-brand-bright)] outline-none focus:border-[var(--color-brand)] focus:shadow-[0_0_10px_-2px_var(--color-brand)]"
				/>
			</label>

			<button
				onclick={() => lookup(manualCode)}
				disabled={!manualCode}
				class="btn-pd violet mt-4 w-full"
			>
				{t('link.continue')}
			</button>
		</div>
	{/if}
</div>
