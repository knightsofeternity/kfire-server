<script lang="ts">
	// The console a game runs on, next to its name. Nothing for the desktop client.
	let { platform }: { platform?: 'playstation' | 'xbox' | 'nintendo' } = $props();
</script>

{#if platform === 'playstation'}
	<span class="shrink-0 border border-[#0070D1]/50 px-1 text-[10px] font-bold text-[#3d9bff]" title="PlayStation">PS</span>
{:else if platform === 'nintendo'}
	<span class="shrink-0 border border-[#E60012]/50 px-1 text-[10px] font-bold text-[#ff4d5a]" title="Nintendo Switch">NS</span>
{:else if platform === 'xbox'}
	<span class="shrink-0 border border-[#107C10]/50 px-1 text-[10px] font-bold text-[#3fbf3f]" title="Xbox">XB</span>
{/if}
