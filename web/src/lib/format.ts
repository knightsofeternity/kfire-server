/** "12h 34m", "5m", "-" for durations in seconds. */
export function formatDuration(seconds: number): string {
	if (!seconds || seconds < 60) return seconds > 0 ? '<1m' : '-';
	const h = Math.floor(seconds / 3600);
	const m = Math.floor((seconds % 3600) / 60);
	if (h === 0) return `${m}m`;
	return m === 0 ? `${h}h` : `${h}h ${m}m`;
}

/** Relative time like "3m ago", "2h ago", "5d ago". */
export function timeAgo(iso: string | undefined): string {
	if (!iso) return '';
	const diff = Date.now() - new Date(iso).getTime();
	const s = Math.floor(diff / 1000);
	if (s < 60) return 'just now';
	const m = Math.floor(s / 60);
	if (m < 60) return `${m}m ago`;
	const h = Math.floor(m / 60);
	if (h < 24) return `${h}h ago`;
	return `${Math.floor(h / 24)}d ago`;
}

export function formatDate(iso: string): string {
	return new Date(iso).toLocaleDateString(undefined, {
		year: 'numeric',
		month: 'short',
		day: 'numeric'
	});
}

/** Deterministic accent color from a string (avatar fallback). */
export function colorFor(seed: string): string {
	let hash = 0;
	for (let i = 0; i < seed.length; i++) hash = seed.charCodeAt(i) + ((hash << 5) - hash);
	const hue = Math.abs(hash) % 360;
	return `hsl(${hue} 45% 35%)`;
}
