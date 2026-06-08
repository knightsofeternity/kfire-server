// Browser WebSocket client for live presence. Read-only: the dashboard never
// sends game events, only authenticates and consumes presence_update.

import type { PresenceEntry } from './api';

type Status = 'connecting' | 'connected' | 'disconnected';

export type PresenceSocket = { close: () => void };

const HEARTBEAT_MS = 30_000;

function wsUrl(): string {
	const proto = location.protocol === 'https:' ? 'wss' : 'ws';
	return `${proto}://${location.host}/ws`;
}

/**
 * Opens an authenticated presence socket. Calls `onUpdate` for every
 * presence_update and `onStatus` on connection state changes. Reconnects with
 * exponential backoff + jitter. `getToken` returns a currently-valid access
 * token (refreshed by the auth store).
 */
export function connectPresence(
	getToken: () => string | null,
	onUpdate: (entry: PresenceEntry) => void,
	onStatus: (status: Status) => void
): PresenceSocket {
	let socket: WebSocket | null = null;
	let heartbeat: ReturnType<typeof setInterval> | null = null;
	let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
	let attempt = 0;
	let closed = false;

	const envelope = (type: string, payload: unknown) =>
		JSON.stringify({ type, ts: new Date().toISOString(), payload });

	function cleanup() {
		if (heartbeat) clearInterval(heartbeat);
		heartbeat = null;
		socket = null;
	}

	function scheduleReconnect() {
		if (closed) return;
		attempt++;
		const delay = Math.min(2 ** attempt * 1000 + Math.random() * 1000, 60_000);
		onStatus('disconnected');
		reconnectTimer = setTimeout(open, delay);
	}

	function open() {
		const token = getToken();
		if (!token || closed) return;
		onStatus('connecting');

		const ws = new WebSocket(wsUrl());
		socket = ws;

		ws.onopen = () => {
			ws.send(
				envelope('hello', {
					protocol_version: 1,
					access_token: token,
					device_id: 'web-dashboard',
					client: 'kfire-web'
				})
			);
		};

		ws.onmessage = (e) => {
			let msg: { type: string; payload: PresenceEntry };
			try {
				msg = JSON.parse(e.data);
			} catch {
				return;
			}
			if (msg.type === 'hello_ack') {
				attempt = 0;
				onStatus('connected');
				heartbeat = setInterval(() => {
					if (ws.readyState === WebSocket.OPEN) ws.send(envelope('heartbeat', {}));
				}, HEARTBEAT_MS);
			} else if (msg.type === 'presence_update') {
				onUpdate(msg.payload);
			}
		};

		ws.onclose = () => {
			cleanup();
			scheduleReconnect();
		};
		ws.onerror = () => ws.close();
	}

	open();

	return {
		close() {
			closed = true;
			if (reconnectTimer) clearTimeout(reconnectTimer);
			if (heartbeat) clearInterval(heartbeat);
			socket?.close();
		}
	};
}
