<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { api } from '$lib/api';

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
			error = e instanceof Error ? e.message : 'invalid code';
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
			error = e instanceof Error ? e.message : 'approval failed';
		} finally {
			busy = false;
		}
	}
</script>

<div class="mx-auto max-w-md">
	<h1 class="mb-6 text-xl font-bold">Link a device</h1>

	{#if approved}
		<div class="rounded-xl border border-[var(--color-online)]/40 bg-[var(--color-online)]/10 p-6 text-center">
			<p class="text-lg font-semibold text-[var(--color-online)]">✓ Device linked</p>
			<p class="mt-2 text-sm text-[var(--color-muted)]">
				You can return to the KFIRE app - it will connect automatically.
			</p>
		</div>
	{:else if loading}
		<p class="text-[var(--color-muted)]">Loading…</p>
	{:else if device}
		<div class="rounded-xl border border-[var(--color-border)] bg-[var(--color-surface)] p-6">
			<p class="mb-1 text-sm text-[var(--color-muted)]">A device wants to link to your account:</p>
			<p class="mb-1 text-lg font-semibold">{device.device_name}</p>
			<p class="mb-5 text-sm text-[var(--color-muted)]">
				{platformLabel[device.platform] ?? device.platform} · code <span class="font-mono">{code}</span>
			</p>
			<p class="mb-5 text-xs text-[var(--color-muted)]">
				Only approve if you just started linking the KFIRE app on this device.
			</p>
			{#if error}<p class="mb-3 text-sm text-red-500">{error}</p>{/if}
			<button
				onclick={approve}
				disabled={busy}
				class="w-full rounded-lg bg-[var(--color-brand)] px-4 py-2 text-sm font-semibold text-[var(--color-bg)] hover:bg-[var(--color-brand-bright)] disabled:opacity-60"
			>
				{busy ? 'Linking…' : 'Approve & link this device'}
			</button>
		</div>
	{:else}
		<div class="rounded-xl border border-[var(--color-border)] bg-[var(--color-surface)] p-6">
			{#if error}<p class="mb-3 text-sm text-red-500">{error}</p>{/if}
			<label class="flex flex-col gap-1 text-xs text-[var(--color-muted)]">
				Enter the code shown in the app
				<input
					bind:value={manualCode}
					placeholder="XXXX-XXXX"
					class="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg)] px-3 py-2 text-center font-mono uppercase outline-none focus:border-[var(--color-brand)]"
				/>
			</label>
			<button
				onclick={() => lookup(manualCode)}
				disabled={!manualCode}
				class="mt-3 w-full rounded-lg bg-[var(--color-brand)] px-4 py-2 text-sm font-semibold text-[var(--color-bg)] hover:bg-[var(--color-brand-bright)] disabled:opacity-60"
			>
				Continue
			</button>
		</div>
	{/if}
</div>
