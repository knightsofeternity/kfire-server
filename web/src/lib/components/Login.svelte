<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { auth } from '$lib/stores/auth.svelte';
	import { getConfig } from '$lib/api';
	import { passwordStrength } from '$lib/password';

	let mode = $state<'login' | 'register'>('login');
	let displayName = $state('');
	let email = $state('');
	let password = $state('');
	let error = $state('');
	let busy = $state(false);

	let inviteCode = $state<string | null>(null);
	let openRegistration = $state(true);
	let needsSetup = $state(false);
	let serverHasLogo = $state(false);
	let orgName = $state('');

	let strength = $derived(passwordStrength(password));
	const barColors = ['bg-red-500', 'bg-red-500', 'bg-yellow-500', 'bg-lime-500', 'bg-[var(--color-online)]'];

	// Self-registration is offered on a fresh instance (first = admin), when the
	// instance is open, or when the visitor arrived with an invite link.
	let canRegister = $derived(needsSetup || openRegistration || !!inviteCode);

	onMount(async () => {
		inviteCode = page.url.searchParams.get('invite');
		const cfg = await getConfig();
		openRegistration = cfg.open_registration;
		needsSetup = cfg.needs_setup;
		serverHasLogo = cfg.has_logo;
		orgName = cfg.org_name;
		if (inviteCode || needsSetup) mode = 'register';
	});

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
				await auth.register(displayName, email, password, inviteCode ?? undefined);
			} else {
				await auth.login(email, password);
			}
		} catch (err) {
			error = err instanceof Error ? err.message : 'something went wrong';
		} finally {
			busy = false;
		}
	}
</script>

<div class="pd-grid-bg grid flex-1 place-items-center px-4 py-10">
	<div class="w-full max-w-sm">
		<img src="/kfire-logo-640.png" alt="KFIRE" class="pd-logo-glow mx-auto mb-3 h-32 w-auto" />
		<h1 class="font-display text-center text-3xl font-extrabold tracking-[0.08em] italic">
			K<span class="text-[var(--color-brand)]">FIRE</span>
		</h1>
		{#if serverHasLogo}
			<img src="/img/org/logo" alt={orgName} title={orgName} class="mx-auto mt-3 h-12 w-auto" />
		{/if}
		<p class="mt-1 mb-6 text-center text-sm text-[var(--color-muted)]">
			{#if needsSetup}
				Welcome. Create the first account; it becomes the admin.
			{:else if mode === 'login'}
				Sign in to your organization
			{:else}
				Create your account
			{/if}
		</p>

		<form onsubmit={submit} class="pd-card flex flex-col gap-4 p-6">
			{#if mode === 'register' && inviteCode}
				<p class="rounded-lg bg-[var(--color-brand)]/10 px-3 py-2 text-xs text-[var(--color-brand)]">
					You were invited to join. Set your details below.
				</p>
			{/if}

			{#if mode === 'register'}
				<label class="flex flex-col gap-1 text-xs text-[var(--color-muted)]">
					Display name
					<input
						type="text"
						bind:value={displayName}
						autocomplete="nickname"
						required
						minlength={3}
						maxlength={32}
						class="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg)] px-3 py-2 text-sm text-[var(--color-text)] outline-none focus:border-[var(--color-brand)]"
					/>
				</label>
			{/if}

			<label class="flex flex-col gap-1 text-xs text-[var(--color-muted)]">
				Email
				<input
					type="email"
					bind:value={email}
					autocomplete={mode === 'register' ? 'email' : 'username'}
					required
					class="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg)] px-3 py-2 text-sm text-[var(--color-text)] outline-none focus:border-[var(--color-brand)]"
				/>
			</label>

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
			</label>

			{#if mode === 'register' && password}
				<div class="-mt-2 flex flex-col gap-1">
					<div class="flex gap-1">
						{#each [0, 1, 2, 3] as i (i)}
							<span
								class="h-1 flex-1 rounded-full {strength.score > i
									? barColors[strength.score]
									: 'bg-[var(--color-border)]'}"
							></span>
						{/each}
					</div>
					<span class="text-xs text-[var(--color-muted)]">
						{strength.label} · at least 12 characters; a passphrase works great
					</span>
				</div>
			{/if}

			{#if error}<p class="text-sm text-red-500">{error}</p>{/if}

			<button type="submit" disabled={busy} class="btn-pd violet mt-1 w-full">
				{busy ? 'Please wait…' : mode === 'login' ? 'Sign in' : 'Create account'}
			</button>
		</form>

		{#if canRegister}
			<p class="mt-4 text-center text-sm text-[var(--color-muted)]">
				{mode === 'login' ? 'No account yet?' : 'Already have an account?'}
				<button type="button" onclick={toggle} class="text-[var(--color-brand)] hover:underline">
					{mode === 'login' ? 'Create one' : 'Sign in'}
				</button>
			</p>
		{:else if mode === 'login'}
			<p class="mt-4 text-center text-xs text-[var(--color-muted)]">
				Registration is invite-only. Ask an admin for an invite link.
			</p>
		{/if}
	</div>
</div>
