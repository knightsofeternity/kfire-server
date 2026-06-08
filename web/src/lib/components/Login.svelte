<script lang="ts">
	import { auth } from '$lib/stores/auth.svelte';

	let username = $state('');
	let password = $state('');
	let error = $state('');
	let busy = $state(false);

	async function submit(e: Event) {
		e.preventDefault();
		error = '';
		busy = true;
		try {
			await auth.login(username, password);
		} catch (err) {
			error = err instanceof Error ? err.message : 'login failed';
		} finally {
			busy = false;
		}
	}
</script>

<div class="grid min-h-screen place-items-center px-4">
	<div class="w-full max-w-sm">
		<h1 class="mb-1 text-center text-2xl font-bold tracking-[0.12em] text-[var(--color-brand)]">
			KFIRE
		</h1>
		<p class="mb-6 text-center text-sm text-[var(--color-muted)]">Sign in to your organization</p>

		<form
			onsubmit={submit}
			class="flex flex-col gap-4 rounded-xl border border-[var(--color-border)] bg-[var(--color-surface)] p-6"
		>
			<label class="flex flex-col gap-1 text-xs text-[var(--color-muted)]">
				Username
				<input
					type="text"
					bind:value={username}
					autocomplete="username"
					required
					class="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg)] px-3 py-2 text-sm text-[var(--color-text)] outline-none focus:border-[var(--color-brand)]"
				/>
			</label>
			<label class="flex flex-col gap-1 text-xs text-[var(--color-muted)]">
				Password
				<input
					type="password"
					bind:value={password}
					autocomplete="current-password"
					required
					class="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg)] px-3 py-2 text-sm text-[var(--color-text)] outline-none focus:border-[var(--color-brand)]"
				/>
			</label>
			{#if error}<p class="text-sm text-red-500">{error}</p>{/if}
			<button
				type="submit"
				disabled={busy}
				class="mt-1 rounded-lg bg-[var(--color-brand)] px-4 py-2 text-sm font-semibold text-[var(--color-bg)] hover:bg-[var(--color-brand-bright)] disabled:opacity-60"
			>
				{busy ? 'Signing in…' : 'Sign in'}
			</button>
		</form>
	</div>
</div>
