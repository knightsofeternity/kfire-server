<script lang="ts">
	import { onMount } from 'svelte';
	import { api } from '$lib/api';
	import type { PluginInfo } from '$lib/api';

	let plugins = $state<PluginInfo[]>([]);
	let error = $state('');
	let busy = $state<string | null>(null);

	onMount(load);

	async function load() {
		try {
			plugins = (await api.listPlugins()).plugins;
		} catch (e) {
			error = e instanceof Error ? e.message : 'échec du chargement';
		}
	}

	async function toggle(p: PluginInfo) {
		if (!p.available) return;
		busy = p.id;
		error = '';
		try {
			await api.setPlugin(p.id, !p.enabled);
			await load();
		} catch (e) {
			error = e instanceof Error ? e.message : 'échec de la mise à jour';
			await load(); // rollback to server truth
		} finally {
			busy = null;
		}
	}
</script>

<h1 class="pd-heading mb-6 text-2xl text-[var(--color-text)]">Plugins de jeu</h1>

<p class="mb-4 text-sm text-[var(--color-muted)]">
	Activez ou désactivez les intégrations spécifiques à un jeu. Désactivé : aucune récupération de
	données et aucun affichage spécifique (le jeu reste visible en présence). Un plugin grisé nécessite
	d'abord la configuration de sa clé de connecteur.
</p>

{#if error}<p class="mb-3 text-sm text-red-500">{error}</p>{/if}

<div class="pd-card p-4">
	{#if plugins.length === 0}
		<p class="text-sm text-[var(--color-muted)]">Aucun plugin.</p>
	{:else}
		<ul class="divide-y divide-[var(--color-border)]">
			{#each plugins as p (p.id)}
				<li class="flex items-center justify-between gap-4 py-3">
					<div>
						<p class="font-display font-semibold text-[var(--color-text)]">
							{p.name}
							{#if !p.available}
								<span class="ml-2 text-xs text-[var(--color-muted)]">Indisponible</span>
							{:else if p.enabled}
								<span class="ml-2 text-xs text-[var(--color-gold)]">Activé</span>
							{:else}
								<span class="ml-2 text-xs text-[var(--color-muted)]">Désactivé</span>
							{/if}
						</p>
						<p class="text-xs text-[var(--color-muted)]">
							{p.id}
							{#if !p.available} · connecteur « {p.connector} » non configuré{/if}
						</p>
					</div>
					<button
						class="btn-pd {p.enabled ? 'violet' : 'btn-pd-ghost'} px-3 py-1.5 text-sm"
						disabled={!p.available || busy === p.id}
						onclick={() => toggle(p)}
						title={p.available ? undefined : `Configurez la clé ${p.connector} pour activer ce plugin`}
					>
						{busy === p.id ? '…' : p.enabled ? 'Désactiver' : 'Activer'}
					</button>
				</li>
			{/each}
		</ul>
	{/if}
</div>
