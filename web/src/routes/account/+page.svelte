<script lang="ts">
	import { auth } from '$lib/stores/auth.svelte';
	import { api } from '$lib/api';
	import { formatDate } from '$lib/format';
	import Avatar from '$lib/components/Avatar.svelte';

	let saving = $state(false);
	let error = $state('');

	let user = $derived($auth.user);

	async function toggleActivity() {
		if (!user) return;
		saving = true;
		error = '';
		try {
			const updated = await api.updateActivityVisible(!user.activity_visible);
			auth.setUser(updated);
		} catch (e) {
			error = e instanceof Error ? e.message : 'failed to update';
		} finally {
			saving = false;
		}
	}
</script>

{#if user}
	<h1 class="mb-6 text-xl font-bold">Account</h1>

	<div class="mb-4 flex items-center gap-4 rounded-xl border border-[var(--color-border)] bg-[var(--color-surface)] p-5">
		<Avatar username={user.username} url={user.avatar_url} size={56} />
		<div>
			<div class="flex items-center gap-2">
				<span class="text-lg font-semibold">{user.username}</span>
				{#if user.role === 'admin'}
					<span class="rounded bg-[var(--color-brand)]/15 px-2 py-0.5 text-xs text-[var(--color-brand)]">admin</span>
				{/if}
			</div>
			{#if user.email}<p class="text-sm text-[var(--color-muted)]">{user.email}</p>{/if}
			<p class="text-xs text-[var(--color-muted)]">member since {formatDate(user.created_at)}</p>
		</div>
	</div>

	<!-- Privacy -->
	<div class="mb-4 rounded-xl border border-[var(--color-border)] bg-[var(--color-surface)] p-5">
		<h2 class="mb-3 text-sm font-semibold tracking-wide text-[var(--color-muted)] uppercase">Privacy</h2>
		<div class="flex items-center justify-between gap-4">
			<div>
				<p class="font-medium">Show my game activity</p>
				<p class="text-sm text-[var(--color-muted)]">
					When off, other members see you as online but never see which game you're playing.
					Your own history is unaffected.
				</p>
			</div>
			<button
				onclick={toggleActivity}
				disabled={saving}
				role="switch"
				aria-checked={user.activity_visible}
				aria-label="Toggle game activity visibility"
				class="relative h-6 w-11 shrink-0 rounded-full transition-colors disabled:opacity-60 {user.activity_visible
					? 'bg-[var(--color-brand)]'
					: 'bg-[var(--color-border)]'}"
			>
				<span
					class="absolute top-0.5 h-5 w-5 rounded-full bg-white transition-all {user.activity_visible
						? 'left-[22px]'
						: 'left-0.5'}"
				></span>
			</button>
		</div>
		{#if error}<p class="mt-2 text-sm text-red-500">{error}</p>{/if}
	</div>

	<!-- Connected accounts (placeholder for the connectors milestone) -->
	<div class="mb-4 rounded-xl border border-[var(--color-border)] bg-[var(--color-surface)] p-5">
		<h2 class="mb-3 text-sm font-semibold tracking-wide text-[var(--color-muted)] uppercase">
			Connected accounts
		</h2>
		<p class="text-sm text-[var(--color-muted)]">
			Linking Steam, Battle.net, Riot and others (for console activity and achievements) is coming
			soon.
		</p>
	</div>

	<button
		onclick={() => auth.logout()}
		class="rounded-lg border border-[var(--color-border)] px-4 py-2 text-sm text-[var(--color-muted)] hover:border-red-500/50 hover:text-red-400"
	>
		Sign out
	</button>
{/if}
