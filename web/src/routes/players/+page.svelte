<script lang="ts">
	import { onMount } from 'svelte';
	import { api, type PresenceEntry } from '$lib/api';
	import Avatar from '$lib/components/Avatar.svelte';
	import StatusBadge from '$lib/components/StatusBadge.svelte';

	let members = $state<PresenceEntry[]>([]);
	let query = $state('');
	let loading = $state(true);

	let filtered = $derived(
		members
			.filter((m) => m.username.toLowerCase().includes(query.toLowerCase()))
			.sort((a, b) => a.username.localeCompare(b.username))
	);

	onMount(async () => {
		try {
			members = await api.getPresence();
		} finally {
			loading = false;
		}
	});
</script>

<div class="mb-5 flex items-center justify-between gap-4">
	<h1 class="text-xl font-bold">Players</h1>
	<input
		type="search"
		placeholder="Search…"
		bind:value={query}
		class="w-48 rounded-lg border border-[var(--color-border)] bg-[var(--color-surface)] px-3 py-1.5 text-sm outline-none focus:border-[var(--color-brand)]"
	/>
</div>

{#if loading}
	<p class="text-[var(--color-muted)]">Loading…</p>
{:else if filtered.length === 0}
	<p class="text-[var(--color-muted)]">No players found.</p>
{:else}
	<ul class="divide-y divide-[var(--color-border)] overflow-hidden rounded-xl border border-[var(--color-border)] bg-[var(--color-surface)]">
		{#each filtered as m (m.user_id)}
			<li>
				<a href="/players/{m.user_id}" class="flex items-center gap-3 px-4 py-3 hover:bg-[var(--color-surface-2)]">
					<Avatar username={m.username} url={m.avatar_url} size={36} />
					<span class="flex-1 font-medium">{m.username}</span>
					{#if m.status === 'in_game' && m.game}
						<span class="truncate text-sm text-[var(--color-brand)]">{m.game.name}</span>
					{/if}
					<StatusBadge status={m.status} />
				</a>
			</li>
		{/each}
	</ul>
{/if}
