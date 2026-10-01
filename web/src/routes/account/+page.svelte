<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { auth } from '$lib/stores/auth.svelte';
	import { api, getConfig, type Connection, type PsnStatus, type NintendoStatus } from '$lib/api';
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
	let bnetNeedsReconnect = $derived(
		!!battlenet && !(battlenet.scopes ?? []).includes('wow.profile')
	);
	let bnBusy = $state(false);
	let xbox = $derived(connections.find((c) => c.provider === 'xbox'));
	let xboxBusy = $state(false);
	let riot = $derived(connections.find((c) => c.provider === 'riot'));
	let riotBusy = $state(false);
	let riotRegion = $state<string | null>(null);
	let riotRegionSaving = $state(false);
	let riotIdInput = $state('');
	let riotError = $state('');
	let pubg = $derived(connections.find((c) => c.provider === 'pubg'));
	let pubgBusy = $state(false);
	let pubgPlatform = $state<string | null>(null);
	let pubgPlatformSaving = $state(false);
	let pubgNameInput = $state('');
	let pubgPlatformInput = $state('steam');
	let pubgError = $state('');
	const pubgPlatforms = ['steam', 'xbox', 'psn', 'kakao', 'stadia'];

	// Which connectors this instance has configured. A card is shown when its
	// connector is enabled OR the member already linked it (so they can still
	// unlink an account after an admin disables the connector).
	let connectors = $state({ steam: true, battlenet: true, xbox: true, riot: true, pubg: true });
	let showSteam = $derived(connectors.steam || !!steam);
	let showBattlenet = $derived(connectors.battlenet || !!battlenet);
	let showXbox = $derived(connectors.xbox || !!xbox);
	let showRiot = $derived(connectors.riot || !!riot);
	let showPubg = $derived(connectors.pubg || !!pubg);

	// PlayStation: null until loaded, and stays null on an instance without a
	// bot (the card is then hidden, like any unconfigured connector).
	let psn = $state<PsnStatus | null>(null);
	let psnIdInput = $state('');
	let psnBusy = $state(false);
	let psnError = $state('');
	let psnNotice = $state('');

	// Nintendo Switch: null until loaded, and on an instance without a bot.
	let nin = $state<NintendoStatus | null>(null);
	let ninCodeInput = $state('');
	let ninBusy = $state(false);
	let ninError = $state('');
	let ninNotice = $state('');

	// Surface the result of the OAuth redirect (?steam=… / ?battlenet=… / ?xbox=…).
	const steamResult = $derived(page.url.searchParams.get('steam'));
	const battlenetResult = $derived(page.url.searchParams.get('battlenet'));
	const xboxResult = $derived(page.url.searchParams.get('xbox'));
	const linkMessageKeys: Record<string, string> = {
		linked: 'account.linkResult.linked',
		denied: 'account.linkResult.denied',
		expired: 'account.linkResult.expired',
		conflict: 'account.linkResult.conflict',
		error: 'account.linkResult.error'
	};

	onMount(() => {
		loadConnections();
		getConfig()
			.then((cfg) => {
				connectors = cfg.connectors;
				if (cfg.connectors.psn) loadPsn();
				if (cfg.connectors.nintendo) loadNintendo();
			})
			.catch(() => {});
		// Until the bot is the member's friend, look again every 15 s so the
		// card turns to "connected" on its own once the request is accepted.
		const timer = setInterval(() => {
			if (psn?.linked && !psn.friend) loadPsn();
			if (nin?.linked && !nin.friend) loadNintendo();
		}, 15000);
		return () => clearInterval(timer);
	});

	async function loadNintendo() {
		try {
			nin = await api.getNintendo();
		} catch {
			/* non-fatal */
		}
	}

	const ninErrorKeys: Record<string, string> = {
		invalid_friend_code: 'account.nintendo.errors.invalidCode',
		nintendo_not_found: 'account.nintendo.errors.notFound',
		nintendo_unavailable: 'account.nintendo.errors.unavailable',
		nintendo_busy: 'account.nintendo.errors.busy',
		already_linked: 'account.nintendo.errors.alreadyLinked',
		connector_disabled: 'account.nintendo.errors.disabled',
		nintendo_bot_down: 'account.nintendo.errors.botDown',
		nintendo_not_friend: 'account.nintendo.errors.notFriend',
		rate_limited: 'account.nintendo.errors.rateLimited'
	};

	function ninErrorMessage(e: unknown): string {
		const code = (e as { code?: string })?.code;
		const key = code ? ninErrorKeys[code] : undefined;
		if (key) return t(key);
		return e instanceof Error ? e.message : t('common.unknownResult');
	}

	async function linkNintendo() {
		if (!ninCodeInput.trim()) return;
		ninBusy = true;
		ninError = '';
		try {
			await api.linkNintendo(ninCodeInput.trim());
			ninCodeInput = '';
			await loadNintendo();
			await loadConnections();
		} catch (e) {
			ninError = ninErrorMessage(e);
		} finally {
			ninBusy = false;
		}
	}

	async function unlinkNintendo() {
		ninBusy = true;
		ninError = '';
		try {
			await api.unlinkNintendo();
			await loadNintendo();
			await loadConnections();
		} catch (e) {
			ninError = ninErrorMessage(e);
		} finally {
			ninBusy = false;
		}
	}

	async function syncNintendo() {
		ninBusy = true;
		ninError = '';
		ninNotice = '';
		try {
			await api.syncNintendo();
			ninNotice = t('account.nintendo.synced');
		} catch (e) {
			ninError = ninErrorMessage(e);
		} finally {
			ninBusy = false;
		}
	}

	async function loadPsn() {
		try {
			psn = await api.getPsn();
		} catch {
			/* non-fatal */
		}
	}

	const psnErrorKeys: Record<string, string> = {
		invalid_psn_id: 'account.psn.errors.invalidId',
		psn_not_found: 'account.psn.errors.notFound',
		psn_unavailable: 'account.psn.errors.unavailable',
		already_linked: 'account.psn.errors.alreadyLinked',
		connector_disabled: 'account.psn.errors.disabled',
		psn_bot_down: 'account.psn.errors.botDown',
		psn_not_friend: 'account.psn.errors.notFriend',
		rate_limited: 'account.psn.errors.rateLimited'
	};

	function psnErrorMessage(e: unknown): string {
		const code = (e as { code?: string })?.code;
		const key = code ? psnErrorKeys[code] : undefined;
		if (key) return t(key);
		return e instanceof Error ? e.message : t('common.unknownResult');
	}

	async function linkPsn() {
		if (!psnIdInput.trim()) return;
		psnBusy = true;
		psnError = '';
		try {
			await api.linkPsn(psnIdInput.trim());
			psnIdInput = '';
			await loadPsn();
			await loadConnections();
		} catch (e) {
			psnError = psnErrorMessage(e);
		} finally {
			psnBusy = false;
		}
	}

	async function unlinkPsn() {
		psnBusy = true;
		psnError = '';
		try {
			await api.unlinkPsn();
			await loadPsn();
			await loadConnections();
		} catch (e) {
			psnError = psnErrorMessage(e);
		} finally {
			psnBusy = false;
		}
	}

	async function syncPsn() {
		psnBusy = true;
		psnError = '';
		psnNotice = '';
		try {
			await api.syncPsn();
			psnNotice = t('account.psn.synced');
		} catch (e) {
			psnError = psnErrorMessage(e);
		} finally {
			psnBusy = false;
		}
	}

	async function loadConnections() {
		if (!user) return;
		try {
			const profile = await api.getProfile(user.id);
			connections = profile.connections;
			if (connections.some((c) => c.provider === 'riot')) loadRiotRegion();
			if (connections.some((c) => c.provider === 'pubg')) loadPubgPlatform();
		} catch {
			/* non-fatal */
		}
	}

	async function loadRiotRegion() {
		try {
			const region = await api.getRiotRegion();
			riotRegion = region?.platform ?? null;
		} catch {
			/* non-fatal */
		}
	}

	async function loadPubgPlatform() {
		try {
			const routing = await api.getPubgPlatform();
			pubgPlatform = routing?.platform ?? null;
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

	async function linkXbox() {
		xboxBusy = true;
		try {
			window.location.href = await api.startXboxLink();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Xbox is not configured on this instance';
			xboxBusy = false;
		}
	}

	async function unlinkXbox() {
		xboxBusy = true;
		try {
			await api.unlinkXbox();
			connections = connections.filter((c) => c.provider !== 'xbox');
		} finally {
			xboxBusy = false;
		}
	}

	// The server answers in English. Translate the codes we know so a French
	// member is not told off in another language, and fall back to whatever the
	// server said for anything unforeseen.
	const riotErrorKeys: Record<string, string> = {
		invalid_riot_id: 'account.riot.errors.invalidRiotId',
		riot_id_not_found: 'account.riot.errors.notFound',
		already_linked: 'account.riot.errors.alreadyLinked',
		connector_disabled: 'account.riot.errors.disabled',
		rate_limited: 'account.riot.errors.rateLimited'
	};

	function riotErrorMessage(e: unknown): string {
		const code = (e as { code?: string })?.code;
		const key = code ? riotErrorKeys[code] : undefined;
		if (key) return t(key);
		return e instanceof Error ? e.message : t('common.unknownResult');
	}

	async function linkRiot() {
		if (!riotIdInput.trim()) return;
		riotBusy = true;
		riotError = '';
		try {
			const linked = await api.linkRiot(riotIdInput.trim());
			connections = [
				...connections.filter((c) => c.provider !== 'riot'),
				{ provider: 'riot', provider_user_id: linked.riot_id, display_name: linked.riot_id, linked_at: new Date().toISOString() }
			];
			riotRegion = linked.platform;
			riotIdInput = '';
		} catch (e) {
			riotError = riotErrorMessage(e);
		} finally {
			riotBusy = false;
		}
	}

	async function unlinkRiot() {
		riotBusy = true;
		try {
			await api.unlinkRiot();
			connections = connections.filter((c) => c.provider !== 'riot');
			riotRegion = null;
			riotError = '';
		} finally {
			riotBusy = false;
		}
	}

	// Same treatment as Riot's codes. pubg_name_not_found is the one that
	// matters: it is by far the most common failure, and it is the member's to
	// fix, so it must not read like an outage.
	const pubgErrorKeys: Record<string, string> = {
		invalid_pubg_name: 'account.pubg.errors.invalidName',
		invalid_platform: 'account.pubg.errors.invalidPlatform',
		pubg_name_not_found: 'account.pubg.errors.notFound',
		pubg_unavailable: 'account.pubg.errors.unavailable',
		already_linked: 'account.pubg.errors.alreadyLinked',
		connector_disabled: 'account.pubg.errors.disabled',
		rate_limited: 'account.pubg.errors.rateLimited'
	};

	function pubgErrorMessage(e: unknown): string {
		const code = (e as { code?: string })?.code;
		const key = code ? pubgErrorKeys[code] : undefined;
		if (key) return t(key);
		return e instanceof Error ? e.message : t('common.unknownResult');
	}

	async function linkPubg() {
		if (!pubgNameInput.trim()) return;
		pubgBusy = true;
		pubgError = '';
		try {
			const linked = await api.linkPubg(pubgNameInput.trim(), pubgPlatformInput);
			connections = [
				...connections.filter((c) => c.provider !== 'pubg'),
				{ provider: 'pubg', provider_user_id: linked.name, display_name: linked.name, linked_at: new Date().toISOString() }
			];
			pubgPlatform = linked.platform;
			pubgNameInput = '';
		} catch (e) {
			pubgError = pubgErrorMessage(e);
		} finally {
			pubgBusy = false;
		}
	}

	async function unlinkPubg() {
		pubgBusy = true;
		try {
			await api.unlinkPubg();
			connections = connections.filter((c) => c.provider !== 'pubg');
			pubgPlatform = null;
			pubgError = '';
		} finally {
			pubgBusy = false;
		}
	}

	async function changePubgPlatform(platform: string) {
		// Optimistic, but restored on failure. A refused platform matters more
		// here than for Riot: the server only moves the link when it finds the
		// name on the new shard, so leaving the picker on it would claim a link
		// that does not exist.
		const previous = pubgPlatform;
		pubgPlatform = platform;
		pubgPlatformSaving = true;
		pubgError = '';
		try {
			await api.setPubgPlatform(platform);
		} catch (e) {
			pubgPlatform = previous;
			pubgError = pubgErrorMessage(e);
		} finally {
			pubgPlatformSaving = false;
		}
	}

	async function changeRiotRegion(platform: string) {
		// Optimistic, but restored on failure: leaving the picker on a region the
		// server refused would tell the member their correction was saved when it
		// was not, and every later refresh would still use the old one.
		const previous = riotRegion;
		riotRegion = platform;
		riotRegionSaving = true;
		try {
			await api.setRiotRegion(platform);
		} catch (e) {
			riotRegion = previous;
			error = e instanceof Error ? e.message : 'failed to update region';
		} finally {
			riotRegionSaving = false;
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

		{#if showSteam}
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

		{#if showBattlenet}
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
		{#if bnetNeedsReconnect}
			<button class="mt-2 text-sm underline text-[var(--color-brand-bright)]" onclick={linkBattlenet}>
				{t('account.bnetReconnectStats')}
			</button>
		{/if}
		{/if}

		{#if xboxResult}
			<p
				class="mt-3 px-3 py-2 text-sm pd-cut-sm {xboxResult === 'linked'
					? 'bg-[var(--color-online)]/15 text-[var(--color-online)]'
					: 'bg-red-500/10 text-red-400'}"
			>
				{linkMessageKeys[xboxResult] ? t(linkMessageKeys[xboxResult]) : t('common.unknownResult')}
				<button class="ml-2 underline" onclick={() => goto('/account')}>{t('common.dismiss')}</button>
			</p>
		{/if}

		{#if showXbox}
		<!-- Xbox -->
		<div class="mt-3 flex items-center justify-between gap-4 border border-[var(--color-border)] bg-[var(--color-bg)] p-3 pd-cut-sm">
			<div class="flex items-center gap-3">
				<span class="grid h-9 w-9 place-items-center pd-cut-sm bg-[var(--color-online)]/15 text-xs font-bold text-[var(--color-online)]">X</span>
				<div>
					<p class="font-display font-semibold text-[var(--color-text)]">Xbox</p>
					{#if xbox}
						<p class="text-sm text-[var(--color-muted)]">
							{xbox.display_name ?? xbox.provider_user_id}
						</p>
					{:else}
						<p class="text-sm text-[var(--color-muted)]">{t('account.notLinked')}</p>
						<p class="mt-1 text-xs text-[var(--color-muted)]/80">{t('account.xbox.slowHint')}</p>
					{/if}
				</div>
			</div>
			{#if xbox}
				<button
					onclick={unlinkXbox}
					disabled={xboxBusy}
					class="btn-pd btn-pd-ghost px-3 py-1.5 text-sm hover:border-red-500/50 hover:text-red-400 disabled:opacity-60"
				>
					{t('account.unlink')}
				</button>
			{:else}
				<button
					onclick={linkXbox}
					disabled={xboxBusy}
					class="btn-pd disabled:opacity-60"
				>
					{xboxBusy ? t('account.xbox.linking') : t('account.xbox.link')}
				</button>
			{/if}
		</div>

		{/if}

		{#if showRiot}
		<!-- Riot Games -->
		<div class="mt-3 flex items-center justify-between gap-4 border border-[var(--color-border)] bg-[var(--color-bg)] p-3 pd-cut-sm">
			<div class="flex items-center gap-3">
				<span class="grid h-9 w-9 place-items-center pd-cut-sm bg-[#c8302633]/15 text-xs font-bold text-[#eb0029]">R</span>
				<div>
					<p class="font-display font-semibold text-[var(--color-text)]">{t('account.riot.title')}</p>
					{#if riot}
						<p class="text-sm text-[var(--color-muted)]">
							{riot.display_name ?? riot.provider_user_id}
						</p>
					{:else}
						<p class="text-sm text-[var(--color-muted)]">{t('account.notLinked')}</p>
					{/if}
				</div>
			</div>
			{#if riot}
				<button
					onclick={unlinkRiot}
					disabled={riotBusy}
					class="btn-pd btn-pd-ghost px-3 py-1.5 text-sm hover:border-red-500/50 hover:text-red-400 disabled:opacity-60"
				>
					{t('account.riot.unlink')}
				</button>
			{/if}
		</div>

		{#if !riot}
			<p class="mt-2 text-xs text-[var(--color-muted)]">{t('account.riot.blurb')}</p>
			<form
				onsubmit={(e) => {
					e.preventDefault();
					linkRiot();
				}}
				class="mt-2 flex flex-col gap-1 sm:flex-row sm:items-start sm:gap-2"
			>
				<label class="flex-1 text-xs text-[var(--color-muted)]" for="riot-id-input">
					{t('account.riot.riotIdLabel')}
					<input
						id="riot-id-input"
						type="text"
						bind:value={riotIdInput}
						placeholder={t('account.riot.riotIdPlaceholder')}
						disabled={riotBusy}
						class="mt-1 w-full border border-[var(--color-border)] bg-[var(--color-bg)] px-3 py-2 text-sm text-[var(--color-text)] outline-none focus:border-[var(--color-brand)] disabled:opacity-60"
					/>
				</label>
				<button
					type="submit"
					disabled={riotBusy || !riotIdInput.trim()}
					class="btn-pd violet mt-1 shrink-0 self-start px-3 py-2 text-sm disabled:opacity-60 sm:mt-5"
				>
					{riotBusy ? '...' : t('account.riot.submit')}
				</button>
			</form>
			{#if riotError}
				<p class="mt-1 text-sm text-red-500">{riotError}</p>
			{/if}
		{/if}

		<p class="mt-2 text-xs text-[var(--color-muted)]/80">{t('account.riot.trust')}</p>

		{#if riot}
			<label class="mt-2 flex flex-col gap-1 text-xs text-[var(--color-muted)]">
				{t('account.riot.region')}
				<select
					value={riotRegion ?? ''}
					disabled={riotRegionSaving || riotRegion === null}
					onchange={(e) => changeRiotRegion(e.currentTarget.value)}
					class="border border-[var(--color-border)] bg-[var(--color-bg)] px-3 py-2 text-sm text-[var(--color-text)] outline-none focus:border-[var(--color-brand)]"
				>
					{#each ['br1', 'eun1', 'euw1', 'jp1', 'kr', 'la1', 'la2', 'me1', 'na1', 'oc1', 'ph2', 'ru', 'sg2', 'th2', 'tr1', 'tw2', 'vn2'] as platform (platform)}
						<option value={platform}>{platform}</option>
					{/each}
				</select>
			</label>
		{/if}

		<p class="mt-2 text-xs text-[var(--color-muted)]/80">{t('common.riotDisclaimer')}</p>
		{/if}

		{#if showPubg}
		<!-- PUBG -->
		<div class="mt-3 flex items-center justify-between gap-4 border border-[var(--color-border)] bg-[var(--color-bg)] p-3 pd-cut-sm">
			<div class="flex items-center gap-3">
				<span class="grid h-9 w-9 place-items-center pd-cut-sm bg-[#f2a13733]/15 text-xs font-bold text-[#f2a137]">P</span>
				<div>
					<p class="font-display font-semibold text-[var(--color-text)]">{t('account.pubg.title')}</p>
					{#if pubg}
						<p class="text-sm text-[var(--color-muted)]">
							{pubg.display_name ?? pubg.provider_user_id}
						</p>
					{:else}
						<p class="text-sm text-[var(--color-muted)]">{t('account.notLinked')}</p>
					{/if}
				</div>
			</div>
			{#if pubg}
				<button
					onclick={unlinkPubg}
					disabled={pubgBusy}
					class="btn-pd btn-pd-ghost px-3 py-1.5 text-sm hover:border-red-500/50 hover:text-red-400 disabled:opacity-60"
				>
					{t('account.pubg.unlink')}
				</button>
			{/if}
		</div>

		{#if !pubg}
			<p class="mt-2 text-xs text-[var(--color-muted)]">{t('account.pubg.blurb')}</p>
			<form
				onsubmit={(e) => {
					e.preventDefault();
					linkPubg();
				}}
				class="mt-2 flex flex-col gap-1 sm:flex-row sm:items-start sm:gap-2"
			>
				<label class="flex-1 text-xs text-[var(--color-muted)]" for="pubg-name-input">
					{t('account.pubg.nameLabel')}
					<input
						id="pubg-name-input"
						type="text"
						bind:value={pubgNameInput}
						placeholder={t('account.pubg.namePlaceholder')}
						disabled={pubgBusy}
						class="mt-1 w-full border border-[var(--color-border)] bg-[var(--color-bg)] px-3 py-2 text-sm text-[var(--color-text)] outline-none focus:border-[var(--color-brand)] disabled:opacity-60"
					/>
				</label>
				<label class="text-xs text-[var(--color-muted)]" for="pubg-platform-input">
					{t('account.pubg.platform')}
					<select
						id="pubg-platform-input"
						bind:value={pubgPlatformInput}
						disabled={pubgBusy}
						class="mt-1 w-full border border-[var(--color-border)] bg-[var(--color-bg)] px-3 py-2 text-sm text-[var(--color-text)] outline-none focus:border-[var(--color-brand)] disabled:opacity-60"
					>
						{#each pubgPlatforms as platform (platform)}
							<option value={platform}>{platform}</option>
						{/each}
					</select>
				</label>
				<button
					type="submit"
					disabled={pubgBusy || !pubgNameInput.trim()}
					class="btn-pd violet mt-1 shrink-0 self-start px-3 py-2 text-sm disabled:opacity-60 sm:mt-5"
				>
					{pubgBusy ? '...' : t('account.pubg.submit')}
				</button>
			</form>
		{/if}

		{#if pubg}
			<label class="mt-2 flex flex-col gap-1 text-xs text-[var(--color-muted)]">
				{t('account.pubg.platform')}
				<select
					value={pubgPlatform ?? ''}
					disabled={pubgPlatformSaving || pubgPlatform === null}
					onchange={(e) => changePubgPlatform(e.currentTarget.value)}
					class="border border-[var(--color-border)] bg-[var(--color-bg)] px-3 py-2 text-sm text-[var(--color-text)] outline-none focus:border-[var(--color-brand)]"
				>
					{#each pubgPlatforms as platform (platform)}
						<option value={platform}>{platform}</option>
					{/each}
				</select>
			</label>
		{/if}

		{#if pubgError}
			<p class="mt-1 text-sm text-red-500">{pubgError}</p>
		{/if}

		<p class="mt-2 text-xs text-[var(--color-muted)]/80">{t('account.pubg.keepsHistory')}</p>
		{/if}

		{#if psn}
		<!-- PlayStation -->
		<div class="mt-3 flex items-center justify-between gap-4 border border-[var(--color-border)] bg-[var(--color-bg)] p-3 pd-cut-sm">
			<div class="flex items-center gap-3">
				<span class="grid h-9 w-9 place-items-center pd-cut-sm bg-[#0070D1]/15 text-xs font-bold text-[#3d9bff]">PS</span>
				<div>
					<p class="font-display font-semibold text-[var(--color-text)]">{t('account.psn.title')}</p>
					<p class="text-sm text-[var(--color-muted)]">
						{psn.linked ? psn.online_id : t('account.notLinked')}
					</p>
				</div>
			</div>
			{#if psn.linked}
				<div class="flex shrink-0 gap-2">
					{#if psn.friend}
						<button onclick={syncPsn} disabled={psnBusy} class="btn-pd btn-pd-ghost px-3 py-1.5 text-sm disabled:opacity-60">
							{t('account.psn.sync')}
						</button>
					{/if}
					<button
						onclick={unlinkPsn}
						disabled={psnBusy}
						class="btn-pd btn-pd-ghost px-3 py-1.5 text-sm hover:border-red-500/50 hover:text-red-400 disabled:opacity-60"
					>
						{t('account.psn.unlink')}
					</button>
				</div>
			{/if}
		</div>

		{#if !psn.linked}
			<p class="mt-2 text-xs text-[var(--color-muted)]">{t('account.psn.blurb')}</p>
			<form
				onsubmit={(e) => {
					e.preventDefault();
					linkPsn();
				}}
				class="mt-2 flex flex-col gap-1 sm:flex-row sm:items-start sm:gap-2"
			>
				<label class="flex-1 text-xs text-[var(--color-muted)]" for="psn-id-input">
					{t('account.psn.idLabel')}
					<input
						id="psn-id-input"
						type="text"
						bind:value={psnIdInput}
						placeholder={t('account.psn.idPlaceholder')}
						disabled={psnBusy}
						class="mt-1 w-full border border-[var(--color-border)] bg-[var(--color-bg)] px-3 py-2 text-sm text-[var(--color-text)] outline-none focus:border-[var(--color-brand)] disabled:opacity-60"
					/>
				</label>
				<button
					type="submit"
					disabled={psnBusy || !psnIdInput.trim()}
					class="btn-pd violet mt-1 shrink-0 self-start px-3 py-2 text-sm disabled:opacity-60 sm:mt-5"
				>
					{psnBusy ? '...' : t('account.psn.submit')}
				</button>
			</form>
		{:else if !psn.friend}
			<p class="mt-2 border-l-2 border-[#0070D1] pl-2 text-sm text-[var(--color-text)]">
				{t('account.psn.addFriend', { bot: psn.bot_online_id })}
			</p>
		{:else}
			<p class="mt-2 text-xs text-[var(--color-online)]">{t('account.psn.connected', { bot: psn.bot_online_id })}</p>
		{/if}

		{#if psnError}
			<p class="mt-1 text-sm text-red-500">{psnError}</p>
		{/if}
		{#if psnNotice}
			<p class="mt-1 text-sm text-[var(--color-muted)]">{psnNotice}</p>
		{/if}

		<p class="mt-2 text-xs text-[var(--color-muted)]/80">{t('account.psn.keepsHistory')}</p>
		{/if}

		{#if nin}
		<!-- Nintendo Switch -->
		<div class="mt-3 flex items-center justify-between gap-4 border border-[var(--color-border)] bg-[var(--color-bg)] p-3 pd-cut-sm">
			<div class="flex items-center gap-3">
				<span class="grid h-9 w-9 place-items-center pd-cut-sm bg-[#E60012]/15 text-xs font-bold text-[#ff4d5a]">NS</span>
				<div>
					<p class="font-display font-semibold text-[var(--color-text)]">{t('account.nintendo.title')}</p>
					<p class="text-sm text-[var(--color-muted)]">
						{nin.linked ? `${nin.nickname ?? ''}${nin.friend_code ? ' · SW-' + nin.friend_code : ''}` : t('account.notLinked')}
					</p>
				</div>
			</div>
			{#if nin.linked}
				<div class="flex shrink-0 gap-2">
					{#if nin.friend}
						<button onclick={syncNintendo} disabled={ninBusy} class="btn-pd btn-pd-ghost px-3 py-1.5 text-sm disabled:opacity-60">
							{t('account.nintendo.sync')}
						</button>
					{/if}
					<button
						onclick={unlinkNintendo}
						disabled={ninBusy}
						class="btn-pd btn-pd-ghost px-3 py-1.5 text-sm hover:border-red-500/50 hover:text-red-400 disabled:opacity-60"
					>
						{t('account.nintendo.unlink')}
					</button>
				</div>
			{/if}
		</div>

		{#if !nin.linked}
			<p class="mt-2 text-xs text-[var(--color-muted)]">{t('account.nintendo.blurb')}</p>
			<form
				onsubmit={(e) => {
					e.preventDefault();
					linkNintendo();
				}}
				class="mt-2 flex flex-col gap-1 sm:flex-row sm:items-start sm:gap-2"
			>
				<label class="flex-1 text-xs text-[var(--color-muted)]" for="nin-code-input">
					{t('account.nintendo.codeLabel')}
					<input
						id="nin-code-input"
						type="text"
						bind:value={ninCodeInput}
						placeholder={t('account.nintendo.codePlaceholder')}
						disabled={ninBusy}
						class="mt-1 w-full border border-[var(--color-border)] bg-[var(--color-bg)] px-3 py-2 font-mono text-sm text-[var(--color-text)] outline-none focus:border-[var(--color-brand)] disabled:opacity-60"
					/>
				</label>
				<button
					type="submit"
					disabled={ninBusy || !ninCodeInput.trim()}
					class="btn-pd violet mt-1 shrink-0 self-start px-3 py-2 text-sm disabled:opacity-60 sm:mt-5"
				>
					{ninBusy ? '...' : t('account.nintendo.submit')}
				</button>
			</form>
		{:else if !nin.friend}
			<p class="mt-2 border-l-2 border-[#E60012] pl-2 text-sm text-[var(--color-text)]">
				{t('account.nintendo.addFriend', { bot: nin.bot.nickname, code: 'SW-' + nin.bot.friend_code })}
			</p>
			<p class="mt-1 text-xs text-[var(--color-muted)]">{t('account.nintendo.settings')}</p>
		{:else}
			<p class="mt-2 text-xs text-[var(--color-online)]">{t('account.nintendo.connected', { bot: nin.bot.nickname })}</p>
		{/if}

		{#if ninError}
			<p class="mt-1 text-sm text-red-500">{ninError}</p>
		{/if}
		{#if ninNotice}
			<p class="mt-1 text-sm text-[var(--color-muted)]">{ninNotice}</p>
		{/if}

		<p class="mt-2 text-xs text-[var(--color-muted)]/80">
			{t('account.nintendo.thirdParty')}
			<a class="underline" href="https://github.com/samuelthomas2774/nxapi" target="_blank" rel="noreferrer noopener">nxapi</a>.
			{t('account.nintendo.keepsHistory')}
		</p>
		{/if}

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
