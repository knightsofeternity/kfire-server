<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { peekReset, submitReset } from '$lib/api';
	import { passwordStrength } from '$lib/password';
	import { t } from '$lib/i18n';

	const token = $derived(page.params.token ?? '');
	let loading = $state(true);
	let valid = $state(false);
	let username = $state('');
	let password = $state('');
	let busy = $state(false);
	let done = $state(false);
	let error = $state('');

	let strength = $derived(passwordStrength(password));
	const barColors = [
		'bg-red-500',
		'bg-red-500',
		'bg-yellow-500',
		'bg-lime-500',
		'bg-[var(--color-online)]'
	];

	onMount(async () => {
		try {
			const r = await peekReset(token);
			username = r.username;
			valid = true;
		} catch {
			valid = false;
		} finally {
			loading = false;
		}
	});

	async function submit(e: Event) {
		e.preventDefault();
		error = '';
		busy = true;
		try {
			await submitReset(token, password);
			done = true;
		} catch (err) {
			error = err instanceof Error ? err.message : t('reset.genericError');
		} finally {
			busy = false;
		}
	}
</script>

<div class="pd-grid-bg grid flex-1 place-items-center px-4 py-10">
	<div class="w-full max-w-sm">
		<img src="/kfire-logo-640.png" alt="KFIRE" class="pd-logo-glow mx-auto mb-3 h-28 w-auto" />
		<h1 class="font-display text-center text-2xl font-extrabold tracking-[0.06em] italic">
			{t('reset.title')}
		</h1>

		{#if loading}
			<p class="mt-6 text-center text-sm text-[var(--color-muted)]">{t('common.loading')}</p>
		{:else if !valid}
			<p class="mt-6 text-center text-sm text-[var(--color-magenta)]">{t('reset.invalidLink')}</p>
			<p class="mt-4 text-center">
				<a href="/" class="text-[var(--color-brand)] hover:underline">{t('reset.toLogin')}</a>
			</p>
		{:else if done}
			<p class="mt-6 text-center text-sm text-[var(--color-online)]">{t('reset.success')}</p>
			<p class="mt-4 text-center">
				<a href="/" class="text-[var(--color-brand)] hover:underline">{t('reset.toLogin')}</a>
			</p>
		{:else}
			<p class="mt-1 mb-6 text-center text-sm text-[var(--color-muted)]">
				{t('reset.subtitleFor', { username })}
			</p>
			<form onsubmit={submit} class="pd-card flex flex-col gap-4 p-6">
				<label class="flex flex-col gap-1 text-xs text-[var(--color-muted)]">
					{t('reset.password')}
					<input
						type="password"
						bind:value={password}
						autocomplete="new-password"
						required
						minlength={12}
						class="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg)] px-3 py-2 text-sm text-[var(--color-text)] outline-none focus:border-[var(--color-brand)]"
					/>
				</label>
				{#if password}
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
							{strength.key ? t('login.strength.' + strength.key) : ''} · {t('login.passwordHint')}
						</span>
					</div>
				{/if}
				{#if error}<p class="text-sm text-red-500">{error}</p>{/if}
				<button type="submit" disabled={busy} class="btn-pd violet mt-1 w-full">
					{busy ? t('reset.submitting') : t('reset.submit')}
				</button>
			</form>
		{/if}
	</div>
</div>
