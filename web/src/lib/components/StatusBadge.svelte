<script lang="ts">
	import { t } from '$lib/i18n';
	let { status }: { status: 'offline' | 'online' | 'in_game' } = $props();

	const labelKey = { offline: 'status.offline', online: 'status.online', in_game: 'status.inGame' };
	const dot = {
		offline: 'bg-[var(--color-muted)]',
		online: 'bg-[var(--color-online)] shadow-[0_0_8px_var(--color-online)]',
		in_game: 'bg-[var(--color-in-game)] shadow-[0_0_8px_var(--color-in-game)]'
	};
	const text = {
		offline: 'text-[var(--color-muted)]',
		online: 'text-[var(--color-online)]',
		in_game: 'text-[var(--color-brand-bright)]'
	};
</script>

<span
	class="font-display inline-flex items-center gap-1.5 text-xs font-bold tracking-wider uppercase {text[
		status
	]}"
>
	<span class="h-2 w-2 rounded-full {dot[status]}"></span>
	{t(labelKey[status])}
</span>
