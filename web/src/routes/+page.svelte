<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { get } from 'svelte/store';
	import { api, type PresenceEntry } from '$lib/api';
	import { auth } from '$lib/stores/auth.svelte';
	import { connectPresence, type PresenceSocket } from '$lib/ws';
	import { timeAgo } from '$lib/format';
	import Avatar from '$lib/components/Avatar.svelte';
	import StatusBadge from '$lib/components/StatusBadge.svelte';

	let entries = $state<Map<string, PresenceEntry>>(new Map());
	let wsStatus = $state<'connecting' | 'connected' | 'disconnected'>('connecting');
	let loading = $state(true);
	let socket: PresenceSocket | null = null;

	const rank = { in_game: 0, online: 1, offline: 2 };
	let sorted = $derived(
		[...entries.values()].sort(
			(a, b) => rank[a.status] - rank[b.status] || a.username.localeCompare(b.username)
		)
	);
	let playing = $derived(sorted.filter((e) => e.status === 'in_game').length);
	let online = $derived(sorted.filter((e) => e.status !== 'offline').length);

	onMount(async () => {
		try {
			const snapshot = await api.getPresence();
			const map = new Map<string, PresenceEntry>();
			for (const e of snapshot) map.set(e.user_id, e);
			entries = map;
		} finally {
			loading = false;
		}

		socket = connectPresence(
			() => get(auth).accessToken,
			(entry) => {
				const next = new Map(entries);
				next.set(entry.user_id, entry);
				entries = next;
			},
			(status) => (wsStatus = status)
		);
	});

	onDestroy(() => socket?.close());
</script>

<div class="mb-5 flex items-center justify-between">
	<div>
		<h1 class="pd-heading text-2xl">Dashboard</h1>
		<p class="text-sm text-[var(--color-muted)]">
			<span style="color: var(--color-in-game);">{playing} playing</span>
			&middot;
			<span style="color: var(--color-online);">{online} online</span>
		</p>
	</div>
	<span class="inline-flex items-center gap-1.5 text-xs text-[var(--color-muted)]" title="Live connection">
		<span
			class="h-2 w-2 rounded-full {wsStatus === 'connected'
				? 'bg-[var(--color-online)]'
				: wsStatus === 'connecting'
					? 'bg-yellow-500'
					: 'bg-[var(--color-muted)]'}"
		></span>
		{wsStatus === 'connected' ? 'Live' : wsStatus}
	</span>
</div>

{#if loading}
	<p class="text-[var(--color-muted)]">Loading...</p>
{:else if sorted.length === 0}
	<p class="text-[var(--color-muted)]">No members yet.</p>
{:else}
	<div class="grid gap-3 sm:grid-cols-2">
		{#each sorted as entry (entry.user_id)}
			<a
				href="/players/{entry.user_id}"
				class="pd-card flex items-center gap-3 p-4 transition-colors hover:border-[var(--color-brand)] {entry.status === 'in_game' ? 'pd-glow' : ''}"
				class:opacity-60={entry.status === 'offline'}
			>
				<Avatar username={entry.username} url={entry.avatar_url} size={44} />
				<div class="min-w-0 flex-1">
					<div class="flex items-center gap-2">
						<span
							class="truncate font-semibold {entry.status === 'in_game' ? 'text-[var(--color-brand-bright)]' : ''}"
						>{entry.username}</span>
						<StatusBadge status={entry.status} />
					</div>
					{#if entry.status === 'in_game' && entry.game}
						<div class="mt-1 flex items-center gap-2">
							{#if entry.game.icon_url}
								<img src={entry.game.icon_url} alt="" class="h-5 w-5 pd-cut-sm" />
							{/if}
							<span class="truncate text-sm font-semibold text-[var(--color-brand)]">{entry.game.name}</span>
						</div>
						{#if entry.since}
							<p class="mt-0.5 text-xs text-[var(--color-muted)]">since {timeAgo(entry.since)}</p>
						{/if}
					{:else if entry.status === 'online'}
						<p class="mt-1 text-sm" style="color: var(--color-online);">Online</p>
					{/if}
				</div>
			</a>
		{/each}
	</div>
{/if}
