<script lang="ts">
	import { onMount } from 'svelte';

	const REPO = 'knightsofeternity/kfire-client';

	type Asset = { name: string; browser_download_url: string; size: number };
	type Release = { tag_name: string; html_url: string; assets: Asset[] };

	let release = $state<Release | null>(null);
	let loading = $state(true);
	let noRelease = $state(false);
	let os = $state<'windows' | 'macos' | 'linux' | 'unknown'>('unknown');

	const osLabel = { windows: 'Windows', macos: 'macOS', linux: 'Linux', unknown: 'your platform' };

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
			<h1 class="pd-heading text-3xl text-[var(--color-text)]">Get the KFIRE desktop app</h1>
			<p class="mt-1 text-sm text-[var(--color-muted)]">
				Runs in your tray, detects your games and shares your presence.
			</p>
		</div>
	</div>

	{#if loading}
		<p class="font-display text-sm uppercase tracking-widest text-[var(--color-muted)]">Loading latest release...</p>
	{:else if noRelease || !release}
		<div class="pd-card p-6">
			<p class="font-display font-bold uppercase text-[var(--color-text)]">No build published yet.</p>
			<p class="mt-2 text-sm text-[var(--color-muted)]">
				The desktop app builds are on the way. Check the
				<a href="https://github.com/{REPO}/releases" target="_blank" rel="noreferrer" class="text-[var(--color-brand-bright)] hover:underline">releases page</a>
				or build it from source.
			</p>
		</div>
	{:else}
		<!-- Recommended for detected OS -->
		<div class="pd-card mb-6 p-6">
			<p class="mb-4 text-sm text-[var(--color-muted)]">
				Detected: <span class="font-semibold text-[var(--color-text)]">{osLabel[os]}</span>
				<span class="mx-1 text-[var(--color-border)]">|</span>
				version <span class="text-[var(--color-brand-bright)]">{release.tag_name}</span>
			</p>
			{#if recommended.length > 0}
				{#each recommended as a (a.name)}
					<a
						href={a.browser_download_url}
						class="btn-pd violet mb-2 w-full py-4 text-base"
					>
						<span>Download for {osLabel[os]}</span>
						<span class="ml-auto text-xs font-normal opacity-70">{a.name} - {fmtSize(a.size)}</span>
					</a>
				{/each}
			{:else}
				<p class="text-sm text-[var(--color-muted)]">
					No build for {osLabel[os]} in this release - see other platforms below.
				</p>
			{/if}
		</div>

		<!-- Other platforms -->
		{#if others.length > 0}
			<h2 class="pd-heading mb-3 text-xs text-[var(--color-muted)]">Other platforms</h2>
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
		<h2 class="pd-heading mb-3 text-sm text-[var(--color-brand-bright)]">First launch</h2>
		<ol class="ml-4 list-decimal space-y-1 text-sm text-[var(--color-muted)]">
			<li>Open the app and enter this server's address.</li>
			<li>It opens your browser to confirm - approve the device here.</li>
			<li>You're connected. The app lives in your tray.</li>
		</ol>
	</div>

	<!-- Security caution callout -->
	<div class="pd-card mt-4 border-[var(--color-gold)] p-5" style="border-color: color-mix(in srgb, var(--color-gold) 40%, transparent);">
		<h2 class="pd-heading mb-2 text-sm text-[var(--color-gold)]">Caution - Security warning on first run? It's expected.</h2>
		<p class="mb-2 text-sm text-[var(--color-muted)]">
			KFIRE is open-source and the installers aren't code-signed yet, so your OS may
			warn that the publisher is "unknown". The app is safe - here's how to proceed:
		</p>
		<ul class="ml-4 list-disc space-y-1 text-sm text-[var(--color-muted)]">
			<li>
				<span class="font-semibold text-[var(--color-text)]">Windows</span> (SmartScreen): click
				<em>More info</em> then <em>Run anyway</em>.
			</li>
			<li>
				<span class="font-semibold text-[var(--color-text)]">macOS</span> (Gatekeeper): right-click the
				app, choose <em>Open</em>, then <em>Open</em>.
			</li>
			<li><span class="font-semibold text-[var(--color-text)]">Linux</span>: no warning - just run it.</li>
		</ul>
	</div>
</div>
