<script lang="ts">
	import { auth } from '$lib/stores/auth.svelte';

	let mode = $state<'login' | 'register'>('login');
	let username = $state('');
	let email = $state('');
	let password = $state('');
	let error = $state('');
	let busy = $state(false);

	function toggle() {
		mode = mode === 'login' ? 'register' : 'login';
		error = '';
	}

	async function submit(e: Event) {
		e.preventDefault();
		error = '';
		busy = true;
		try {
			if (mode === 'register') {
				await auth.register(username, email, password);
			} else {
				await auth.login(username, password);
			}
		} catch (err) {
			error = err instanceof Error ? err.message : 'something went wrong';
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
		<p class="mb-6 text-center text-sm text-[var(--color-muted)]">
			{mode === 'login' ? 'Sign in to your organization' : 'Create your account'}
		</p>

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

			{#if mode === 'register'}
				<label class="flex flex-col gap-1 text-xs text-[var(--color-muted)]">
					Email
					<input
						type="email"
						bind:value={email}
						autocomplete="email"
						required
						class="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg)] px-3 py-2 text-sm text-[var(--color-text)] outline-none focus:border-[var(--color-brand)]"
					/>
				</label>
			{/if}

			<label class="flex flex-col gap-1 text-xs text-[var(--color-muted)]">
				Password
				<input
					type="password"
					bind:value={password}
					autocomplete={mode === 'register' ? 'new-password' : 'current-password'}
					required
					minlength={mode === 'register' ? 12 : undefined}
					class="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg)] px-3 py-2 text-sm text-[var(--color-text)] outline-none focus:border-[var(--color-brand)]"
				/>
				{#if mode === 'register'}
					<span class="text-[var(--color-muted)]">At least 12 characters.</span>
				{/if}
			</label>

			{#if error}<p class="text-sm text-red-500">{error}</p>{/if}

			<button
				type="submit"
				disabled={busy}
				class="mt-1 rounded-lg bg-[var(--color-brand)] px-4 py-2 text-sm font-semibold text-[var(--color-bg)] hover:bg-[var(--color-brand-bright)] disabled:opacity-60"
			>
				{busy ? 'Please wait…' : mode === 'login' ? 'Sign in' : 'Create account'}
			</button>
		</form>

		<p class="mt-4 text-center text-sm text-[var(--color-muted)]">
			{mode === 'login' ? 'No account yet?' : 'Already have an account?'}
			<button type="button" onclick={toggle} class="text-[var(--color-brand)] hover:underline">
				{mode === 'login' ? 'Create one' : 'Sign in'}
			</button>
		</p>
	</div>
</div>
