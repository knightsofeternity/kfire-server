<script lang="ts">
	import { onMount } from 'svelte';
	import { t } from '$lib/i18n';

	const REPO = 'knightsofeternity/kfire-client';

	type Asset = { name: string; browser_download_url: string; size: number };
	type Release = { tag_name: string; html_url: string; assets: Asset[] };

	let release = $state<Release | null>(null);
	let loading = $state(true);
	let noRelease = $state(false);
	let os = $state<'windows' | 'macos' | 'linux' | 'unknown'>('unknown');

	const osLabel = { windows: 'Windows', macos: 'macOS', linux: 'Linux', unknown: '' };

	function detectOS(): typeof os {
		const ua = navigator.userAgent;
		if (/Win/.test(ua)) return 'windows';
		if (/Mac/.test(ua) && !/iPhone|iPad/.test(ua)) return 'macos';
		if (/Linux/.test(ua) && !/Android/.test(ua)) return 'linux';
		return 'unknown';
	}

	function assetOS(name: string): typeof os | null {
		const n = name.toLowerCase();
		if (n.endsWith('.msi') || n.endsWith('.exe')) return 'windows';
		if (n.endsWith('.dmg') || n.endsWith('.app.tar.gz')) return 'macos';
		if (n.endsWith('.appimage') || n.endsWith('.deb') || n.endsWith('.rpm')) return 'linux';
		return null;
	}

	let recommended = $derived(release?.assets.filter((a) => assetOS(a.name) === os) ?? []);
	let others = $derived(release?.assets.filter((a) => assetOS(a.name) && assetOS(a.name) !== os) ?? []);

	function fmtSize(b: number) {
		return b > 1e6 ? `${(b / 1e6).toFixed(1)} MB` : `${Math.round(b / 1e3)} KB`;
	}

	onMount(async () => {
		os = detectOS();
		try {
			const res = await fetch(`https://api.github.com/repos/${REPO}/releases/latest`);
			if (res.ok) release = await res.json();
			else noRelease = true;
		} catch {
			noRelease = true;
		} finally {
			loading = false;
		}
	});
</script>

<div class="mx-auto max-w-2xl">
	<!-- Page header -->
	<div class="mb-8 flex items-center gap-4">
		<div class="pd-glow rounded-sm">
			<img src="/favicon-96.png" alt="" class="h-16 w-16" />
		</div>
		<div>
			<h1 class="pd-heading text-3xl text-[var(--color-text)]">{t('download.title')}</h1>
			<p class="mt-1 text-sm text-[var(--color-muted)]">
				{t('download.subtitle')}
			</p>
		</div>
	</div>

	{#if loading}
		<p class="font-display text-sm uppercase tracking-widest text-[var(--color-muted)]">{t('download.loading')}</p>
	{:else if noRelease || !release}
		<div class="pd-card p-6">
			<p class="font-display font-bold uppercase text-[var(--color-text)]">{t('download.noRelease')}</p>
			<p class="mt-2 text-sm text-[var(--color-muted)]">
				{t('download.noReleaseHint')}
				<a href="https://github.com/{REPO}/releases" target="_blank" rel="noreferrer" class="text-[var(--color-brand-bright)] hover:underline">{t('download.releasesPage')}</a>
				{t('download.noReleaseOr')}
			</p>
		</div>
	{:else}
		<!-- Recommended for detected OS -->
		<div class="pd-card mb-6 p-6">
			<p class="mb-4 text-sm text-[var(--color-muted)]">
				{t('download.detected', { os: os === 'unknown' ? t('download.yourPlatform') : osLabel[os] })}
				<span class="mx-1 text-[var(--color-border)]">|</span>
				{t('download.version')} <span class="text-[var(--color-brand-bright)]">{release.tag_name}</span>
			</p>
			{#if recommended.length > 0}
				{#each recommended as a (a.name)}
					<a
						href={a.browser_download_url}
						class="btn-pd violet mb-2 w-full py-4 text-base"
					>
						<span>{t('download.downloadFor', { os: os === 'unknown' ? t('download.yourPlatform') : osLabel[os] })}</span>
						<span class="ml-auto text-xs font-normal opacity-70">{a.name} - {fmtSize(a.size)}</span>
					</a>
				{/each}
			{:else}
				<p class="text-sm text-[var(--color-muted)]">
					{t('download.noBuildForOS', { os: os === 'unknown' ? t('download.yourPlatform') : osLabel[os] })}
				</p>
			{/if}
		</div>

		<!-- Other platforms -->
		{#if others.length > 0}
			<h2 class="pd-heading mb-3 text-xs text-[var(--color-muted)]">{t('download.otherPlatforms')}</h2>
			<ul class="pd-card mb-6 divide-y divide-[var(--color-border)] overflow-hidden">
				{#each others as a (a.name)}
					<li>
						<a
							href={a.browser_download_url}
							class="flex items-center justify-between px-4 py-3 transition-colors hover:bg-[var(--color-surface-2)]"
						>
							<span class="text-sm text-[var(--color-text)]">{a.name}</span>
							<span class="text-xs text-[var(--color-muted)]">{fmtSize(a.size)}</span>
						</a>
					</li>
				{/each}
			</ul>
		{/if}
	{/if}

	<!-- First launch steps -->
	<div class="pd-card mt-6 p-5">
		<h2 class="pd-heading mb-3 text-sm text-[var(--color-brand-bright)]">{t('download.firstLaunch')}</h2>
		<ol class="ml-4 list-decimal space-y-1 text-sm text-[var(--color-muted)]">
			<li>{t('download.step1')}</li>
			<li>{t('download.step2')}</li>
			<li>{t('download.step3')}</li>
		</ol>
	</div>

	<!-- Security caution callout -->
	<div class="pd-card mt-4 border-[var(--color-gold)] p-5" style="border-color: color-mix(in srgb, var(--color-gold) 40%, transparent);">
		<h2 class="pd-heading mb-2 text-sm text-[var(--color-gold)]">{t('download.securityTitle')}</h2>
		<p class="mb-2 text-sm text-[var(--color-muted)]">
			{t('download.securityBody')}
		</p>
		<ul class="ml-4 list-disc space-y-1 text-sm text-[var(--color-muted)]">
			<li>
				<span class="font-semibold text-[var(--color-text)]">Windows</span> {@html t('download.securityWindows')}
			</li>
			<li>
				<span class="font-semibold text-[var(--color-text)]">macOS</span> {@html t('download.securityMacos')}
			</li>
			<li><span class="font-semibold text-[var(--color-text)]">Linux</span>: {t('download.securityLinux')}</li>
		</ul>
	</div>
</div>
