<script lang="ts">
	import { onMount } from 'svelte';
	import { api, ApiError } from '$lib/api';
	import type { NintendoBotStatus } from '$lib/api';

	let bot = $state<NintendoBotStatus | null>(null);
	let loginUrl = $state('');
	let link = $state('');
	let busy = $state(false);
	let error = $state('');
	let saved = $state(false);

	onMount(load);

	async function load() {
		try {
			bot = await api.getNintendoBot();
		} catch (e) {
			error =
				e instanceof ApiError && e.code === 'connector_disabled'
					? 'Le conteneur nxapi n’est pas configuré sur cette instance (KFIRE_NXAPI_URL, KFIRE_NXAPI_CLIENT_ID, KFIRE_NXAPI_SHARED_SECRET).'
					: e instanceof Error
						? e.message
						: 'échec du chargement';
		}
	}

	async function generate() {
		busy = true;
		error = '';
		saved = false;
		try {
			loginUrl = (await api.startNintendoLogin()).url;
		} catch (e) {
			error = e instanceof Error ? e.message : 'échec';
		} finally {
			busy = false;
		}
	}

	const messages: Record<string, string> = {
		invalid_link: 'Ce n’est pas le bon lien : il doit commencer par npf71b963c1b7b6d119://auth.',
		login_expired: 'Ce lien correspond à une autre connexion ou a expiré : générez un nouveau lien.',
		nintendo_unavailable: 'Nintendo a refusé ce lien ou n’a pas répondu : générez un nouveau lien.',
		rate_limited: 'Trop de tentatives. Attendez une minute.'
	};

	async function finish() {
		if (!link.trim()) return;
		busy = true;
		error = '';
		try {
			bot = await api.finishNintendoLogin(link.trim());
			link = '';
			loginUrl = '';
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

<h1 class="pd-heading mb-6 text-2xl text-[var(--color-text)]">Bot Nintendo</h1>

<p class="mb-4 text-sm text-[var(--color-muted)]">
	KFIRE lit la présence et l'historique de jeu Switch des membres à travers un compte Nintendo dédié à
	l'instance, dont chaque membre devient l'ami. Nintendo n'ayant pas d'API publique, les échanges passent
	par le conteneur nxapi. La session du bot est chiffrée en base et n'est jamais réaffichée.
</p>

{#if error}<p class="mb-3 text-sm text-red-500">{error}</p>{/if}

{#if bot}
	<div class="pd-card mb-4 p-4">
		{#if !bot.configured}
			<p class="font-display font-semibold text-[var(--color-text)]">
				Non connecté <span class="ml-2 text-xs text-[var(--color-muted)]">la carte Nintendo est masquée aux membres</span>
			</p>
		{:else}
			<p class="font-display font-semibold text-[var(--color-text)]">
				{bot.nickname ?? 'Bot'}
				{#if bot.friend_code}<span class="ml-2 font-mono text-xs text-[var(--color-muted)]">SW-{bot.friend_code}</span>{/if}
				{#if bot.status === 'ok'}
					<span class="ml-2 text-xs" style="color: var(--color-online);">Actif</span>
				{:else}
					<span class="ml-2 text-xs text-red-400">Session refusée par Nintendo : reconnectez le bot</span>
				{/if}
			</p>
			<ul class="mt-2 space-y-1 text-xs text-[var(--color-muted)]">
				<li>Session posée le {when(bot.session_set_at)}</li>
				<li>Dernier succès le {when(bot.last_ok_at)}</li>
				{#if bot.last_error}<li class="text-red-400">Dernière erreur : {bot.last_error}</li>{/if}
			</ul>
		{/if}
	</div>

	<div class="pd-card p-4">
		<ol class="list-decimal space-y-2 pl-5 text-sm text-[var(--color-muted)]">
			<li>
				<button type="button" onclick={generate} disabled={busy} class="btn-pd btn-pd-ghost px-3 py-1.5 text-sm disabled:opacity-60">
					{bot.configured ? 'Générer un lien de reconnexion' : 'Générer le lien de connexion'}
				</button>
				{#if loginUrl}
					<a class="ml-2 underline" href={loginUrl} target="_blank" rel="noreferrer noopener">Ouvrir la connexion Nintendo</a>
					<span class="ml-1 text-xs">(valable 15 minutes)</span>
				{/if}
			</li>
			<li>Connectez-vous avec le compte Nintendo du bot, de préférence dans une fenêtre privée.</li>
			<li>
				Sur la page « Choisir ce compte », faites un clic droit sur le bouton rouge et choisissez « Copier
				l'adresse du lien ». Cliquer sur le bouton ne fait rien sur un ordinateur, c'est normal.
			</li>
			<li>
				<form
					onsubmit={(e) => {
						e.preventDefault();
						finish();
					}}
					class="flex flex-col gap-2 sm:flex-row"
				>
					<input
						type="password"
						autocomplete="off"
						bind:value={link}
						disabled={busy || !loginUrl}
						placeholder="npf71b963c1b7b6d119://auth#…"
						class="w-full border border-[var(--color-border)] bg-[var(--color-bg)] px-3 py-2 text-sm text-[var(--color-text)] outline-none focus:border-[var(--color-brand)] disabled:opacity-60"
					/>
					<button type="submit" disabled={busy || !link.trim()} class="btn-pd violet shrink-0 px-3 py-2 text-sm disabled:opacity-60">
						{busy ? '…' : 'Enregistrer'}
					</button>
				</form>
			</li>
		</ol>
		{#if saved}<p class="mt-3 text-sm" style="color: var(--color-online);">Bot connecté et vérifié.</p>{/if}
	</div>
{/if}
