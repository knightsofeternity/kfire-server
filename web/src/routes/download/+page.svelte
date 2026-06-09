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
	<div class="mb-6 flex items-center gap-3">
		<img src="/favicon-96.png" alt="" class="h-12 w-12" />
		<div>
			<h1 class="text-xl font-bold">Get the KFIRE desktop app</h1>
			<p class="text-sm text-[var(--color-muted)]">
				Runs in your tray, detects your games and shares your presence.
			</p>
		</div>
	</div>

	{#if loading}
		<p class="text-[var(--color-muted)]">Loading latest release…</p>
	{:else if noRelease || !release}
		<div class="rounded-xl border border-[var(--color-border)] bg-[var(--color-surface)] p-6">
			<p class="font-medium">No build published yet.</p>
			<p class="mt-1 text-sm text-[var(--color-muted)]">
				The desktop app builds are on the way. Check the
				<a href="https://github.com/{REPO}/releases" target="_blank" rel="noreferrer" class="text-[var(--color-brand)] hover:underline">releases page</a>
				or build it from source.
			</p>
		</div>
	{:else}
		<!-- Recommended for detected OS -->
		<div class="mb-6 rounded-xl border border-[var(--color-border)] bg-[var(--color-surface)] p-6">
			<p class="mb-3 text-sm text-[var(--color-muted)]">
				Detected: <span class="text-[var(--color-text)]">{osLabel[os]}</span> · version {release.tag_name}
			</p>
			{#if recommended.length > 0}
				{#each recommended as a (a.name)}
					<a
						href={a.browser_download_url}
						class="mb-2 flex items-center justify-between rounded-lg bg-[var(--color-brand)] px-4 py-3 font-semibold text-[var(--color-bg)] hover:bg-[var(--color-brand-bright)]"
					>
						<span>Download for {osLabel[os]}</span>
						<span class="text-sm opacity-80">{a.name} · {fmtSize(a.size)}</span>
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
			<h2 class="mb-2 text-sm font-semibold tracking-wide text-[var(--color-muted)] uppercase">
				Other platforms
			</h2>
			<ul class="divide-y divide-[var(--color-border)] overflow-hidden rounded-xl border border-[var(--color-border)] bg-[var(--color-surface)]">
				{#each others as a (a.name)}
					<li>
						<a href={a.browser_download_url} class="flex items-center justify-between px-4 py-3 hover:bg-[var(--color-surface-2)]">
							<span class="text-sm">{a.name}</span>
							<span class="text-xs text-[var(--color-muted)]">{fmtSize(a.size)}</span>
						</a>
					</li>
				{/each}
			</ul>
		{/if}
	{/if}

	<div class="mt-6 rounded-xl border border-[var(--color-border)] bg-[var(--color-surface)] p-5">
		<h2 class="mb-2 text-sm font-semibold">First launch</h2>
		<ol class="ml-4 list-decimal space-y-1 text-sm text-[var(--color-muted)]">
			<li>Open the app and enter this server's address.</li>
			<li>It opens your browser to confirm - approve the device here.</li>
			<li>You're connected. The app lives in your tray.</li>
		</ol>
	</div>

	<!-- Security warning note -->
	<div class="mt-4 rounded-xl border border-yellow-500/30 bg-yellow-500/5 p-5">
		<h2 class="mb-2 text-sm font-semibold text-yellow-500">⚠️ A security warning on first run? It's expected.</h2>
		<p class="mb-2 text-sm text-[var(--color-muted)]">
			KFIRE is open-source and the installers aren't code-signed yet, so your OS may
			warn that the publisher is "unknown". The app is safe - here's how to proceed:
		</p>
		<ul class="ml-4 list-disc space-y-1 text-sm text-[var(--color-muted)]">
			<li>
				<span class="text-[var(--color-text)]">Windows</span> (SmartScreen): click
				<em>More info</em> → <em>Run anyway</em>.
			</li>
			<li>
				<span class="text-[var(--color-text)]">macOS</span> (Gatekeeper): right-click the
				app → <em>Open</em> → <em>Open</em>.
			</li>
			<li><span class="text-[var(--color-text)]">Linux</span>: no warning - just run it.</li>
		</ul>
	</div>
</div>
