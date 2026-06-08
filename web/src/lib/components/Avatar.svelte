<script lang="ts">
	import { colorFor } from '../format';

	let {
		username,
		url,
		size = 40
	}: { username: string; url?: string | null; size?: number } = $props();
</script>

{#if url}
	<img
		src={url}
		alt={username}
		class="rounded-full object-cover"
		style="width:{size}px;height:{size}px"
	/>
{:else}
	<span
		class="inline-flex items-center justify-center rounded-full font-semibold text-white uppercase"
		style="width:{size}px;height:{size}px;background:{colorFor(username)};font-size:{size *
			0.4}px"
	>
		{username.slice(0, 2)}
	</span>
{/if}
