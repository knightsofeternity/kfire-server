<script lang="ts">
	let { status }: { status: 'offline' | 'online' | 'in_game' } = $props();

	const label = { offline: 'Offline', online: 'Online', in_game: 'In game' };
	const dot = {
		offline: 'bg-[var(--color-muted)]',
		online: 'bg-[var(--color-online)]',
		in_game: 'bg-[var(--color-in-game)]'
	};
</script>

<span class="inline-flex items-center gap-1.5 text-xs text-[var(--color-muted)]">
	<span class="h-2 w-2 rounded-full {dot[status]}"></span>
	{label[status]}
</span>
