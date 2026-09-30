<script lang="ts">
	import { onMount } from 'svelte';
	import { api, ApiError } from '$lib/api';
	import type { PsnBotStatus } from '$lib/api';

	let bot = $state<PsnBotStatus | null>(null);
	let npsso = $state('');
	let busy = $state(false);
	let error = $state('');
	let saved = $state(false);

	onMount(load);

	async function load() {
		try {
			bot = await api.getPsnBot();
		} catch (e) {
			error = e instanceof Error ? e.message : 'échec du chargement';
		}
	}

	const messages: Record<string, string> = {
		invalid_npsso: 'Ce n’est pas un NPSSO : la valeur fait 64 caractères. Vous pouvez coller toute la page de Sony.',
		npsso_refused: 'Sony a refusé ce jeton. Reconnectez-vous sur playstation.com avec le compte du bot et copiez-en un nouveau.',
		psn_unavailable: 'PlayStation n’a pas répondu. Réessayez dans un instant.',
		rate_limited: 'Trop de tentatives. Attendez une minute.'
	};

	async function save() {
		if (!npsso.trim()) return;
		busy = true;
		error = '';
		saved = false;
		try {
			bot = await api.setPsnNPSSO(npsso.trim());
			npsso = '';
			saved = true;
		} catch (e) {
			error = e instanceof ApiError && messages[e.code] ? messages[e.code] : e instanceof Error ? e.message : 'échec';
		} finally {
			busy = false;
		}
	}

	function when(iso?: string | null): string {
		if (!iso) return 'jamais';
		const d = new Date(iso);
		const days = Math.floor((Date.now() - d.getTime()) / 86400000);
		const ago = days <= 0 ? "aujourd'hui" : days === 1 ? 'hier' : `il y a ${days} jours`;
		return `${d.toLocaleString('fr-FR')} (${ago})`;
	}
</script>

<h1 class="pd-heading mb-6 text-2xl text-[var(--color-text)]">Bot PlayStation</h1>

<p class="mb-4 text-sm text-[var(--color-muted)]">
	KFIRE lit la présence, le temps de jeu et les trophées PS4/PS5 des membres à travers un compte PSN dédié
	à l'instance, dont chaque membre devient l'ami. Son jeton (NPSSO) est le seul secret : il est chiffré en
	base et n'est jamais réaffiché.
</p>

{#if error}<p class="mb-3 text-sm text-red-500">{error}</p>{/if}

<div class="pd-card mb-4 p-4">
	{#if !bot}
		<p class="text-sm text-[var(--color-muted)]">Chargement…</p>
	{:else if !bot.configured}
		<p class="font-display font-semibold text-[var(--color-text)]">
			Non configuré <span class="ml-2 text-xs text-[var(--color-muted)]">la carte PlayStation est masquée aux membres</span>
		</p>
	{:else}
		<p class="font-display font-semibold text-[var(--color-text)]">
			{bot.online_id ?? 'Bot'}
			{#if bot.status === 'ok'}
				<span class="ml-2 text-xs" style="color: var(--color-online);">Actif</span>
			{:else}
				<span class="ml-2 text-xs text-red-400">Jeton refusé par Sony : collez-en un nouveau</span>
			{/if}
		</p>
		<ul class="mt-2 space-y-1 text-xs text-[var(--color-muted)]">
			<li>Jeton posé le {when(bot.npsso_set_at)}</li>
			<li>Dernier succès le {when(bot.last_ok_at)}</li>
			{#if bot.last_error}<li class="text-red-400">Dernière erreur : {bot.last_error}</li>{/if}
		</ul>
	{/if}
</div>

<div class="pd-card p-4">
	<form
		onsubmit={(e) => {
			e.preventDefault();
			save();
		}}
		class="flex flex-col gap-2"
	>
		<label class="text-xs text-[var(--color-muted)]" for="npsso-input">
			{bot?.configured ? 'Remplacer le NPSSO' : 'Coller le NPSSO'}
			<input
				id="npsso-input"
				type="password"
				autocomplete="off"
				bind:value={npsso}
				disabled={busy}
				placeholder={'{"npsso":"…"}'}
				class="mt-1 w-full border border-[var(--color-border)] bg-[var(--color-bg)] px-3 py-2 text-sm text-[var(--color-text)] outline-none focus:border-[var(--color-brand)] disabled:opacity-60"
			/>
		</label>
		<button type="submit" disabled={busy || !npsso.trim()} class="btn-pd violet self-start px-3 py-2 text-sm disabled:opacity-60">
			{busy ? '…' : 'Enregistrer'}
		</button>
		{#if saved}<p class="text-sm" style="color: var(--color-online);">Jeton accepté par Sony et enregistré.</p>{/if}
	</form>
	<ol class="mt-4 list-decimal space-y-1 pl-5 text-xs text-[var(--color-muted)]">
		<li>Connectez-vous sur playstation.com avec le compte du bot.</li>
		<li>
			Dans le même navigateur, ouvrez
			<a class="underline" href="https://ca.account.sony.com/api/v1/ssocookie" target="_blank" rel="noreferrer noopener"
				>ca.account.sony.com/api/v1/ssocookie</a
			>.
		</li>
		<li>Copiez toute la page (ou seulement la valeur npsso) et collez-la ci-dessus.</li>
		<li>Ensuite, ne vous reconnectez plus à ce compte ailleurs : une nouvelle connexion peut invalider le jeton.</li>
	</ol>
</div>
