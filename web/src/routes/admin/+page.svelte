<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { auth } from '$lib/stores/auth.svelte';
	import { api, type Member, type Invite } from '$lib/api';
	import { formatDate } from '$lib/format';
	import Avatar from '$lib/components/Avatar.svelte';

	let members = $state<Member[]>([]);
	let invites = $state<Invite[]>([]);
	let loading = $state(true);
	let error = $state('');

	// invite creation
	let newNote = $state('');
	let newRole = $state<'member' | 'admin'>('member');
	let creating = $state(false);
	let copied = $state<string | null>(null);

	const myId = $derived($auth.user?.id);

	onMount(() => {
		if ($auth.user?.role !== 'admin') {
			goto('/');
			return;
		}
		load();
		loadBranding();
	});

	async function load() {
		loading = true;
		try {
			[members, invites] = await Promise.all([api.getMembers(), api.getInvites()]);
		} catch (e) {
			error = e instanceof Error ? e.message : 'failed to load';
		} finally {
			loading = false;
		}
	}

	async function setRole(m: Member, role: 'admin' | 'member') {
		try {
			await api.patchMember(m.id, { role });
			await load();
		} catch (e) {
			error = e instanceof Error ? e.message : 'failed';
		}
	}

	async function setBanned(m: Member, banned: boolean) {
		try {
			await api.patchMember(m.id, { banned });
			await load();
		} catch (e) {
			error = e instanceof Error ? e.message : 'failed';
		}
	}

	async function createInvite() {
		creating = true;
		error = '';
		try {
			await api.createInvite({ note: newNote || undefined, role: newRole });
			newNote = '';
			newRole = 'member';
			invites = await api.getInvites();
		} catch (e) {
			error = e instanceof Error ? e.message : 'failed';
		} finally {
			creating = false;
		}
	}

	async function revoke(code: string) {
		await api.deleteInvite(code);
		invites = invites.filter((i) => i.code !== code);
	}

	async function copy(url: string, code: string) {
		await navigator.clipboard.writeText(url);
		copied = code;
		setTimeout(() => (copied = copied === code ? null : copied), 1500);
	}

	// --- branding -----------------------------------------------------------
	const accents = ['orange', 'violet', 'blue', 'red', 'green', 'yellow'];
	const accentHex: Record<string, string> = {
		orange: '#f24405',
		violet: '#7442ce',
		blue: '#1c7ff3',
		red: '#e11d2a',
		green: '#1f9d4d',
		yellow: '#e6a700'
	};
	let accent = $state('orange');
	let hasLogo = $state(false);
	let brandingMsg = $state('');
	let logoVersion = $state(0); // cache-busts the preview after upload/remove

	async function loadBranding() {
		try {
			const b = await api.getBranding();
			accent = b.accent;
			hasLogo = b.has_logo;
		} catch (e) {
			/* non-fatal */
		}
	}

	async function chooseAccent(a: string) {
		accent = a;
		brandingMsg = '';
		try {
			await api.setAccent(a);
			if (a !== 'orange') document.documentElement.dataset.accent = a;
			else delete document.documentElement.dataset.accent;
		} catch (e) {
			brandingMsg = e instanceof Error ? e.message : 'could not save the color';
		}
	}

	async function onLogoFile(e: Event) {
		const input = e.target as HTMLInputElement;
		const file = input.files?.[0];
		if (!file) return;
		brandingMsg = '';
		try {
			await api.uploadLogo(file);
			hasLogo = true;
			logoVersion++;
		} catch (err) {
			brandingMsg = err instanceof Error ? err.message : 'upload failed';
		} finally {
			input.value = '';
		}
	}

	async function removeLogo() {
		brandingMsg = '';
		try {
			await api.deleteLogo();
			hasLogo = false;
			logoVersion++;
		} catch (e) {
			brandingMsg = e instanceof Error ? e.message : 'could not remove the logo';
		}
	}
</script>

<h1 class="pd-heading mb-6 text-2xl text-[var(--color-brand-bright)]">Admin</h1>

{#if loading}
	<p class="text-[var(--color-muted)]">Loading...</p>
{:else}
	{#if error}<p class="mb-4 text-sm text-[var(--color-magenta)]">{error}</p>{/if}

	<!-- Branding -->
	<section class="mb-8">
		<h2 class="pd-heading mb-3 text-xs text-[var(--color-muted)]">Branding</h2>
		<div class="pd-card flex flex-col gap-6 p-4">
			<div class="flex flex-wrap items-center gap-4">
				<div
					class="grid h-16 w-16 place-items-center border border-[var(--color-border)] bg-[var(--color-bg)]"
				>
					{#if hasLogo}
						<img
							src={`/img/org/logo?v=${logoVersion}`}
							alt="Clan logo"
							class="h-14 w-14 object-contain"
						/>
					{:else}
						<span class="text-[10px] text-[var(--color-muted)]">No logo</span>
					{/if}
				</div>
				<div class="flex flex-col gap-2">
					<p class="text-sm text-[var(--color-text)]">Clan / team logo</p>
					<p class="text-xs text-[var(--color-muted)]">
						PNG or JPEG, up to 2 MB. Shown next to the KFIRE logo in the header.
					</p>
					<div class="flex gap-2">
						<label class="btn-pd violet cursor-pointer">
							Upload
							<input
								type="file"
								accept="image/png,image/jpeg"
								class="hidden"
								onchange={onLogoFile}
							/>
						</label>
						{#if hasLogo}
							<button class="btn-pd btn-pd-ghost" onclick={removeLogo}>Remove</button>
						{/if}
					</div>
				</div>
			</div>

			<div class="flex flex-col gap-2">
				<p class="text-sm text-[var(--color-text)]">Dominant color</p>
				<div class="flex flex-wrap gap-2">
					{#each accents as a (a)}
						<button
							onclick={() => chooseAccent(a)}
							title={a}
							aria-label={a}
							class="h-9 w-9 border-2 transition-transform hover:scale-110 {accent === a
								? 'border-[var(--color-text)]'
								: 'border-transparent'}"
							style={`background:${accentHex[a]}; clip-path: polygon(6px 0,100% 0,100% calc(100% - 6px),calc(100% - 6px) 100%,0 100%,0 6px);`}
							aria-pressed={accent === a}
						></button>
					{/each}
				</div>
			</div>

			{#if brandingMsg}<p class="text-sm text-[var(--color-magenta)]">{brandingMsg}</p>{/if}
		</div>
	</section>

	<!-- Invites -->
	<section class="mb-8">
		<h2 class="pd-heading mb-3 text-xs text-[var(--color-muted)]">
			Invite a member
		</h2>
		<div class="pd-card mb-4 flex flex-wrap items-end gap-3 p-4">
			<label class="flex flex-1 flex-col gap-1 text-xs text-[var(--color-muted)]">
				Note (optional - who's it for?)
				<input
					type="text"
					bind:value={newNote}
					placeholder="e.g. Lancelot"
					class="border border-[var(--color-border)] bg-[var(--color-bg)] px-3 py-2 text-sm text-[var(--color-text)] outline-none focus:border-[var(--color-brand)]"
				/>
			</label>
			<label class="flex flex-col gap-1 text-xs text-[var(--color-muted)]">
				Role
				<select
					bind:value={newRole}
					class="border border-[var(--color-border)] bg-[var(--color-bg)] px-3 py-2 text-sm text-[var(--color-text)] outline-none focus:border-[var(--color-brand)]"
				>
					<option value="member">Member</option>
					<option value="admin">Admin</option>
				</select>
			</label>
			<button
				onclick={createInvite}
				disabled={creating}
				class="btn-pd violet"
			>
				{creating ? '...' : 'Create link'}
			</button>
		</div>

		{#if invites.length > 0}
			<ul class="pd-card divide-y divide-[var(--color-border)] overflow-hidden">
				{#each invites as inv (inv.code)}
					<li class="flex items-center gap-3 px-4 py-3">
						<div class="min-w-0 flex-1">
							<p class="truncate text-sm">
								{inv.note ?? 'Invite'}
								<span class="text-[var(--color-muted)]">- {inv.role}</span>
							</p>
							<p class="truncate text-xs text-[var(--color-muted)]">
								{inv.url} - expires {formatDate(inv.expires_at)}
							</p>
						</div>
						<button
							onclick={() => copy(inv.url, inv.code)}
							class="btn-pd btn-pd-ghost px-3 py-1.5 text-xs"
						>
							{copied === inv.code ? 'Copied!' : 'Copy link'}
						</button>
						<button
							onclick={() => revoke(inv.code)}
							class="btn-pd magenta px-3 py-1.5 text-xs"
						>
							Revoke
						</button>
					</li>
				{/each}
			</ul>
		{:else}
			<p class="text-sm text-[var(--color-muted)]">No pending invites.</p>
		{/if}
	</section>

	<!-- Members -->
	<section>
		<h2 class="pd-heading mb-3 text-xs text-[var(--color-muted)]">
			Members ({members.length})
		</h2>
		<ul class="pd-card divide-y divide-[var(--color-border)] overflow-hidden">
			{#each members as m (m.id)}
				<li class="flex items-center gap-3 px-4 py-3" class:opacity-60={m.banned}>
					<Avatar username={m.username} url={m.avatar_url} size={36} />
					<div class="min-w-0 flex-1">
						<p class="truncate text-sm font-medium">
							{m.username}
							{#if m.id === myId}<span class="text-xs text-[var(--color-muted)]">(you)</span>{/if}
						</p>
						<p class="truncate text-xs text-[var(--color-muted)]">{m.email}</p>
					</div>

					<span
						class="pd-cut-sm px-2 py-0.5 text-xs font-display {m.role === 'admin'
							? 'bg-[var(--color-brand)]/15 text-[var(--color-brand-bright)]'
							: 'text-[var(--color-muted)]'}"
					>
						{m.role}
					</span>

					{#if m.banned}
						<span class="pd-cut-sm bg-[var(--color-magenta)]/15 px-2 py-0.5 text-xs text-[var(--color-magenta)]">banned</span>
					{/if}

					{#if m.id !== myId}
						<div class="flex gap-2">
							{#if m.role === 'admin'}
								<button onclick={() => setRole(m, 'member')} class="btn-pd-ghost btn-pd px-3 py-1 text-xs">
									Make member
								</button>
							{:else}
								<button onclick={() => setRole(m, 'admin')} class="btn-pd-ghost btn-pd px-3 py-1 text-xs">
									Make admin
								</button>
							{/if}
							{#if m.banned}
								<button onclick={() => setBanned(m, false)} class="btn-pd cyan px-3 py-1 text-xs">
									Unban
								</button>
							{:else}
								<button onclick={() => setBanned(m, true)} class="btn-pd magenta px-3 py-1 text-xs">
									Ban
								</button>
							{/if}
						</div>
					{/if}
				</li>
			{/each}
		</ul>
	</section>
{/if}
