<script lang="ts">
	import { onMount } from 'svelte';
	import { api } from '$lib/api';
	import { formatDate } from '$lib/format';

	type KeyRow = {
		id: string;
		label: string;
		key_prefix: string;
		created_at: string;
		last_used_at?: string;
		revoked: boolean;
	};

	let keys = $state<KeyRow[]>([]);
	let label = $state('');
	let creating = $state(false);
	let error = $state('');
	let freshKey = $state(''); // the full secret, shown once after creation

	onMount(load);

	async function load() {
		try {
			keys = (await api.listApiKeys()).keys;
		} catch (e) {
			error = e instanceof Error ? e.message : 'failed to load';
		}
	}

	async function create() {
		if (!label.trim()) return;
		creating = true;
		error = '';
		try {
			const r = await api.createApiKey(label.trim());
			freshKey = r.key;
			label = '';
			await load();
		} catch (e) {
			error = e instanceof Error ? e.message : 'failed to create';
		} finally {
			creating = false;
		}
	}

	async function revoke(id: string) {
		if (!confirm('Révoquer cette clé ? Les consommateurs qui l\'utilisent cesseront de fonctionner.')) return;
		await api.revokeApiKey(id);
		await load();
	}
</script>

<h1 class="pd-heading mb-6 text-2xl text-[var(--color-text)]">Clés API</h1>

{#if freshKey}
	<div class="pd-card mb-4 border border-[var(--color-gold)]/40 p-4">
		<p class="mb-2 font-display font-bold text-[var(--color-gold)]">
			Copiez cette clé maintenant — elle ne sera plus affichée.
		</p>
		<code class="block break-all rounded bg-[var(--color-bg)] p-2 text-sm">{freshKey}</code>
		<div class="mt-2 flex gap-2">
			<button class="btn-pd btn-pd-ghost px-3 py-1.5 text-sm" onclick={() => navigator.clipboard.writeText(freshKey)}>Copier</button>
			<button class="btn-pd btn-pd-ghost px-3 py-1.5 text-sm" onclick={() => (freshKey = '')}>Fermer</button>
		</div>
	</div>
{/if}

<div class="pd-card mb-4 p-4">
	<div class="flex gap-2">
		<input
			class="flex-1 rounded bg-[var(--color-bg)] px-3 py-2 text-sm"
			placeholder="Nom de la clé (ex. site knights-of-eternity)"
			bind:value={label}
		/>
		<button class="btn-pd violet" disabled={creating || !label.trim()} onclick={create}>
			{creating ? '...' : 'Créer'}
		</button>
	</div>
	{#if error}<p class="mt-2 text-sm text-red-500">{error}</p>{/if}
</div>

<div class="pd-card p-4">
	{#if keys.length === 0}
		<p class="text-sm text-[var(--color-muted)]">Aucune clé.</p>
	{:else}
		<ul class="divide-y divide-[var(--color-border)]">
			{#each keys as k (k.id)}
				<li class="flex items-center justify-between gap-4 py-2">
					<div>
						<p class="font-display font-semibold text-[var(--color-text)]">
							{k.label}
							{#if k.revoked}<span class="ml-2 text-xs text-red-400">révoquée</span>{/if}
						</p>
						<p class="text-xs text-[var(--color-muted)]">
							{k.key_prefix}… · créée le {formatDate(k.created_at)}
							{#if k.last_used_at} · vue {formatDate(k.last_used_at)}{/if}
						</p>
					</div>
					{#if !k.revoked}
						<button class="btn-pd btn-pd-ghost px-3 py-1.5 text-sm hover:text-red-400" onclick={() => revoke(k.id)}>
							Révoquer
						</button>
					{/if}
				</li>
			{/each}
		</ul>
	{/if}
</div>
