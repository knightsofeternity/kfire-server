<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { auth } from '$lib/stores/auth.svelte';
	import { api, type Connection } from '$lib/api';
	import { formatDate } from '$lib/format';
	import Avatar from '$lib/components/Avatar.svelte';
	import { t } from '$lib/i18n';

	let saving = $state(false);
	let error = $state('');
	let connections = $state<Connection[]>([]);
	let steamBusy = $state(false);
	let syncMessage = $state('');
	let syncPrivacyHint = $state(false);

	let user = $derived($auth.user);
	let steam = $derived(connections.find((c) => c.provider === 'steam'));
	let battlenet = $derived(connections.find((c) => c.provider === 'battlenet'));
	let bnBusy = $state(false);

	// Surface the result of the OAuth redirect (?steam=… / ?battlenet=…).
	const steamResult = $derived(page.url.searchParams.get('steam'));
	const battlenetResult = $derived(page.url.searchParams.get('battlenet'));
	const linkMessageKeys: Record<string, string> = {
		linked: 'account.linkResult.linked',
		denied: 'account.linkResult.denied',
		expired: 'account.linkResult.expired',
		conflict: 'account.linkResult.conflict',
		error: 'account.linkResult.error'
	};

	onMount(loadConnections);

	async function loadConnections() {
		if (!user) return;
		try {
			const profile = await api.getProfile(user.id);
			connections = profile.connections;
		} catch {
			/* non-fatal */
		}
	}

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

	async function toggleSessions() {
		if (!user) return;
		saving = true;
		error = '';
		try {
			const updated = await api.updateSessionsVisible(!user.sessions_visible);
			auth.setUser(updated);
		} catch (e) {
			error = e instanceof Error ? e.message : 'failed to update';
		} finally {
			saving = false;
		}
	}

	async function linkSteam() {
		steamBusy = true;
		try {
			window.location.href = await api.startSteamLink();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Steam is not configured on this instance';
			steamBusy = false;
		}
	}

	async function unlinkSteam() {
		steamBusy = true;
		try {
			await api.unlinkSteam();
			connections = connections.filter((c) => c.provider !== 'steam');
			syncMessage = '';
		} finally {
			steamBusy = false;
		}
	}

	async function syncSteam() {
		steamBusy = true;
		syncMessage = '';
		syncPrivacyHint = false;
		try {
			const r = await api.syncSteam();
			if (r.games_imported === 0) {
				// The sync succeeded but Steam returned an empty library - almost always
				// because the profile's "Game details" are private (Steam's default).
				syncMessage = t('account.steam.syncEmpty');
				syncPrivacyHint = true;
			} else {
				syncMessage = t('account.steam.syncSuccess', { games: r.games_imported, achievements: r.achievements_imported });
			}
		} catch (e) {
			syncMessage = e instanceof Error ? e.message : 'sync failed';
		} finally {
			steamBusy = false;
		}
	}

	async function linkBattlenet() {
		bnBusy = true;
		try {
			window.location.href = await api.startBattlenetLink();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Battle.net is not configured on this instance';
			bnBusy = false;
		}
	}

	async function unlinkBattlenet() {
		bnBusy = true;
		try {
			await api.unlinkBattlenet();
			connections = connections.filter((c) => c.provider !== 'battlenet');
		} finally {
			bnBusy = false;
		}
	}
</script>

{#if user}
	<h1 class="pd-heading mb-6 text-2xl text-[var(--color-text)]">{t('account.title')}</h1>

	<!-- Profile card -->
	<div class="pd-card mb-4 flex items-center gap-4 p-5">
		<Avatar username={user.username} url={user.avatar_url} size={56} />
		<div>
			<div class="flex items-center gap-2">
				<span class="font-display text-lg font-semibold text-[var(--color-text)]">{user.username}</span>
				{#if user.role === 'admin'}
					<span class="pd-cut-sm bg-[var(--color-brand)]/15 px-2 py-0.5 font-display text-xs font-bold uppercase tracking-widest text-[var(--color-brand-bright)]">{t('account.admin')}</span>
				{/if}
			</div>
			{#if user.email}<p class="text-sm text-[var(--color-muted)]">{user.email}</p>{/if}
			<p class="text-xs text-[var(--color-muted)]">{t('account.memberSince', { date: formatDate(user.created_at) })}</p>
		</div>
	</div>

	<!-- Privacy -->
	<div class="pd-card mb-4 p-5">
		<h2 class="pd-heading mb-3 text-xs text-[var(--color-brand-bright)]">{t('account.privacy.title')}</h2>
		<div class="flex items-center justify-between gap-4">
			<div>
				<p class="font-display font-semibold text-[var(--color-text)]">{t('account.privacy.toggleLabel')}</p>
				<p class="text-sm text-[var(--color-muted)]">
					{t('account.privacy.toggleHint')}
				</p>
			</div>
			<button
				onclick={toggleActivity}
				disabled={saving}
				role="switch"
				aria-checked={user.activity_visible}
				aria-label={t('account.privacy.ariaLabel')}
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
		<div class="mt-4 flex items-center justify-between gap-4 border-t border-[var(--color-border)] pt-4">
			<div>
				<p class="font-display font-semibold text-[var(--color-text)]">{t('account.privacy.sessionsToggleLabel')}</p>
				<p class="text-sm text-[var(--color-muted)]">{t('account.privacy.sessionsToggleHint')}</p>
			</div>
			<button
				onclick={toggleSessions}
				disabled={saving}
				role="switch"
				aria-checked={user.sessions_visible}
				aria-label={t('account.privacy.sessionsAriaLabel')}
				class="relative h-6 w-11 shrink-0 rounded-full transition-colors disabled:opacity-60 {user.sessions_visible
					? 'bg-[var(--color-brand)]'
					: 'bg-[var(--color-border)]'}"
			>
				<span
					class="absolute top-0.5 h-5 w-5 rounded-full bg-white transition-all {user.sessions_visible
						? 'left-[22px]'
						: 'left-0.5'}"
				></span>
			</button>
		</div>
		{#if error}<p class="mt-2 text-sm text-red-500">{error}</p>{/if}
	</div>

	<!-- Connected accounts -->
	<div class="pd-card mb-4 p-5">
		<h2 class="pd-heading mb-3 text-xs text-[var(--color-brand-bright)]">{t('account.connectedAccounts')}</h2>

		{#if steamResult}
			<p
				class="mb-3 px-3 py-2 text-sm pd-cut-sm {steamResult === 'linked'
					? 'bg-[var(--color-online)]/15 text-[var(--color-online)]'
					: 'bg-red-500/10 text-red-400'}"
			>
				{linkMessageKeys[steamResult] ? t(linkMessageKeys[steamResult]) : t('common.unknownResult')}
				<button class="ml-2 underline" onclick={() => goto('/account')}>{t('common.dismiss')}</button>
			</p>
		{/if}

		<!-- Steam -->
		<div class="flex items-center justify-between gap-4 border border-[var(--color-border)] bg-[var(--color-bg)] p-3 pd-cut-sm">
			<div class="flex items-center gap-3">
				{#if steam?.avatar_url}
					<img src={steam.avatar_url} alt="" class="h-9 w-9 pd-cut-sm" />
				{:else}
					<span class="grid h-9 w-9 place-items-center pd-cut-sm bg-[#1b2838] text-xs font-bold text-[#66c0f4]">St</span>
				{/if}
				<div>
					<p class="font-display font-semibold text-[var(--color-text)]">Steam</p>
					{#if steam}
						<p class="text-sm text-[var(--color-muted)]">
							{steam.display_name ?? steam.provider_user_id}
						</p>
					{:else}
						<p class="text-sm text-[var(--color-muted)]">{t('account.notLinked')}</p>
					{/if}
				</div>
			</div>
			{#if steam}
				<div class="flex gap-2">
					<button
						onclick={syncSteam}
						disabled={steamBusy}
						class="btn-pd btn-pd-ghost px-3 py-1.5 text-sm disabled:opacity-60"
					>
						{steamBusy ? '...' : t('account.steam.syncNow')}
					</button>
					<button
						onclick={unlinkSteam}
						disabled={steamBusy}
						class="btn-pd btn-pd-ghost px-3 py-1.5 text-sm hover:border-red-500/50 hover:text-red-400 disabled:opacity-60"
					>
						{t('account.unlink')}
					</button>
				</div>
			{:else}
				<button
					onclick={linkSteam}
					disabled={steamBusy}
					class="btn-pd violet disabled:opacity-60"
				>
					{steamBusy ? '...' : t('account.steam.link')}
				</button>
			{/if}
		</div>

		<!-- How to make a Steam profile importable (shown before and after linking). -->
		<p class="mt-2 text-xs text-[var(--color-muted)]">
			{t('account.steam.publicHintPre')}
			<a
				href="https://steamcommunity.com/my/edit/settings"
				target="_blank"
				rel="noreferrer"
				class="text-[var(--color-brand-bright)] hover:underline">{t('account.steam.settingsLinkText')}</a
			>{t('account.steam.publicHintMid')} <span class="text-[var(--color-text)]">{t('account.steam.myProfile')}</span> {t('account.steam.publicHintAnd')}
			<span class="text-[var(--color-text)]">{t('account.steam.gameDetails')}</span> {t('account.steam.publicHintPost')}
		</p>
		<p class="mt-1 text-xs text-[var(--color-muted)]">
			{t('account.steam.privateWorkaroundPre')} <span class="text-[var(--color-text)]">{t('account.steam.syncNow')}</span> {t('account.steam.privateWorkaroundPost')}
		</p>

		{#if syncMessage}
			<p class="mt-2 text-sm text-[var(--color-muted)]">{syncMessage}</p>
		{/if}
		{#if syncPrivacyHint}
			<div class="mt-2 border border-[var(--color-gold)]/30 bg-[var(--color-gold)]/5 p-3 text-sm text-[var(--color-muted)] pd-cut-sm">
				<p class="mb-1 font-display font-bold uppercase tracking-wide text-[var(--color-gold)]">{t('account.steam.privacyHint.title')}</p>
				<ol class="ml-4 list-decimal space-y-0.5">
					<li>
						{t('account.steam.privacyHint.step1pre')}
						<a
							href="https://steamcommunity.com/my/edit/settings"
							target="_blank"
							rel="noreferrer"
							class="text-[var(--color-brand-bright)] hover:underline">{t('account.steam.privacyHint.step1link')}</a
						>{t('account.steam.privacyHint.step1post')}
					</li>
					<li>{t('account.steam.privacyHint.step2pre')} <span class="text-[var(--color-text)]">{t('account.steam.myProfile')}</span> {t('account.steam.privacyHint.step2post')}</li>
					<li>{t('account.steam.privacyHint.step3pre')} <span class="text-[var(--color-text)]">{t('account.steam.gameDetails')}</span> {t('account.steam.privacyHint.step3post')}</li>
					<li>{t('account.steam.privacyHint.step4')}</li>
				</ol>
			</div>
		{/if}

		{#if battlenetResult}
			<p
				class="mt-3 px-3 py-2 text-sm pd-cut-sm {battlenetResult === 'linked'
					? 'bg-[var(--color-online)]/15 text-[var(--color-online)]'
					: 'bg-red-500/10 text-red-400'}"
			>
				{linkMessageKeys[battlenetResult] ? t(linkMessageKeys[battlenetResult]) : t('common.unknownResult')}
				<button class="ml-2 underline" onclick={() => goto('/account')}>{t('common.dismiss')}</button>
			</p>
		{/if}

		<!-- Battle.net -->
		<div class="mt-3 flex items-center justify-between gap-4 border border-[var(--color-border)] bg-[var(--color-bg)] p-3 pd-cut-sm">
			<div class="flex items-center gap-3">
				<span class="grid h-9 w-9 place-items-center pd-cut-sm bg-[var(--color-blue)]/15 text-xs font-bold text-[var(--color-blue)]">B</span>
				<div>
					<p class="font-display font-semibold text-[var(--color-text)]">Battle.net</p>
					{#if battlenet}
						<p class="text-sm text-[var(--color-muted)]">
							{battlenet.display_name ?? battlenet.provider_user_id}
						</p>
					{:else}
						<p class="text-sm text-[var(--color-muted)]">{t('account.notLinked')}</p>
					{/if}
				</div>
			</div>
			{#if battlenet}
				<button
					onclick={unlinkBattlenet}
					disabled={bnBusy}
					class="btn-pd btn-pd-ghost px-3 py-1.5 text-sm hover:border-red-500/50 hover:text-red-400 disabled:opacity-60"
				>
					{t('account.unlink')}
				</button>
			{:else}
				<button
					onclick={linkBattlenet}
					disabled={bnBusy}
					class="btn-pd violet disabled:opacity-60"
				>
					{bnBusy ? '...' : t('account.battlenet.link')}
				</button>
			{/if}
		</div>

		<p class="mt-3 text-xs text-[var(--color-muted)]">
			{t('account.comingNext')}
		</p>
	</div>

	<button
		onclick={() => auth.logout()}
		class="btn-pd btn-pd-ghost px-4 py-2 text-sm"
	>
		{t('common.signOut')}
	</button>
{/if}
