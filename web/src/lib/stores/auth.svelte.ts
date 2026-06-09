// Browser auth: device-bound session, access token in memory, refresh token in
// localStorage (acceptable for a self-hosted internal tool; documented caveat:
// vulnerable to XSS - a future iteration could move to an httpOnly cookie).

import { writable, type Readable } from 'svelte/store';
import type { User } from '../api';

const REFRESH_KEY = 'kfire_refresh';
const DEVICE_KEY = 'kfire_device';

type AuthState = {
	accessToken: string | null;
	user: User | null;
	ready: boolean; // initial refresh attempt completed
};

function deviceId(): string {
	let id = localStorage.getItem(DEVICE_KEY);
	if (!id) {
		id = crypto.randomUUID();
		localStorage.setItem(DEVICE_KEY, id);
	}
	return id;
}

function createAuth() {
	const { subscribe, set, update } = writable<AuthState>({
		accessToken: null,
		user: null,
		ready: false
	});

	let accessToken: string | null = null;

	async function fetchUser(token: string): Promise<User | null> {
		const res = await fetch('/api/v1/users/me', {
			headers: { Authorization: `Bearer ${token}` }
		});
		return res.ok ? res.json() : null;
	}

	async function register(
		username: string,
		email: string,
		password: string,
		inviteCode?: string
	): Promise<void> {
		const res = await fetch('/api/v1/auth/register', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ username, email, password, invite_code: inviteCode })
		});
		if (!res.ok) {
			const err = await res.json().catch(() => ({ message: 'registration failed' }));
			throw new Error(err.message ?? 'registration failed');
		}
		// Registration doesn't return tokens; sign in right away.
		await login(username, password);
	}

	async function login(username: string, password: string): Promise<void> {
		const res = await fetch('/api/v1/auth/login', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({
				username,
				password,
				device: { device_id: deviceId(), name: 'Web', platform: 'web' }
			})
		});
		if (!res.ok) {
			const err = await res.json().catch(() => ({ message: 'login failed' }));
			throw new Error(err.message ?? 'login failed');
		}
		const tokens = await res.json();
		accessToken = tokens.access_token;
		localStorage.setItem(REFRESH_KEY, tokens.refresh_token);
		const user = await fetchUser(accessToken!);
		set({ accessToken, user, ready: true });
	}

	/** Exchanges the stored refresh token for a new access token. */
	async function refresh(): Promise<boolean> {
		const refreshToken = localStorage.getItem(REFRESH_KEY);
		if (!refreshToken) {
			update((s) => ({ ...s, ready: true }));
			return false;
		}
		const res = await fetch('/api/v1/auth/refresh', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ refresh_token: refreshToken, device_id: deviceId() })
		});
		if (!res.ok) {
			localStorage.removeItem(REFRESH_KEY);
			accessToken = null;
			set({ accessToken: null, user: null, ready: true });
			return false;
		}
		const tokens = await res.json();
		accessToken = tokens.access_token;
		localStorage.setItem(REFRESH_KEY, tokens.refresh_token);
		const user = await fetchUser(accessToken!);
		set({ accessToken, user, ready: true });
		return true;
	}

	async function logout(): Promise<void> {
		if (accessToken) {
			await fetch('/api/v1/auth/logout', {
				method: 'POST',
				headers: { Authorization: `Bearer ${accessToken}` }
			}).catch(() => {});
		}
		localStorage.removeItem(REFRESH_KEY);
		accessToken = null;
		set({ accessToken: null, user: null, ready: true });
	}

	/** Called once on app start to restore a session. */
	async function init(): Promise<void> {
		await refresh();
	}

	function setUser(user: User) {
		update((s) => ({ ...s, user }));
	}

	return { subscribe, login, register, logout, refresh, init, setUser };
}

export const auth: Readable<AuthState> & {
	login(username: string, password: string): Promise<void>;
	register(username: string, email: string, password: string, inviteCode?: string): Promise<void>;
	logout(): Promise<void>;
	refresh(): Promise<boolean>;
	init(): Promise<void>;
	setUser(user: User): void;
} = createAuth();
