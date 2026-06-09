// Tiny reactive i18n. Locale is runes state so any {t('key')} in markup
// re-renders when the locale changes. Choice is persisted in localStorage and
// defaults to the browser language (French for `fr*`, English otherwise).
import { en } from './en';
import { fr } from './fr';

export type Locale = 'en' | 'fr';
export const locales: Locale[] = ['en', 'fr'];
export const localeLabels: Record<Locale, string> = { en: 'EN', fr: 'FR' };

const catalogs = { en, fr };
const STORAGE_KEY = 'kfire-locale';

let current = $state<Locale>('en');

function detect(): Locale {
	if (typeof navigator === 'undefined') return 'en';
	return navigator.language?.toLowerCase().startsWith('fr') ? 'fr' : 'en';
}

/** Resolve the initial locale (stored choice, else browser) and apply it. */
export function initLocale(): void {
	let next: Locale = detect();
	try {
		const saved = localStorage.getItem(STORAGE_KEY);
		if (saved === 'en' || saved === 'fr') next = saved;
	} catch {
		/* storage blocked */
	}
	current = next;
	if (typeof document !== 'undefined') document.documentElement.lang = next;
}

export function getLocale(): Locale {
	return current;
}

export function setLocale(next: Locale): void {
	current = next;
	try {
		localStorage.setItem(STORAGE_KEY, next);
	} catch {
		/* storage blocked; applies for this session */
	}
	if (typeof document !== 'undefined') document.documentElement.lang = next;
}

function lookup(cat: object, path: string): string | undefined {
	let node: unknown = cat;
	for (const part of path.split('.')) {
		if (node && typeof node === 'object' && part in node) {
			node = (node as Record<string, unknown>)[part];
		} else {
			return undefined;
		}
	}
	return typeof node === 'string' ? node : undefined;
}

/**
 * Translate a dot-path key, optionally interpolating {name} placeholders.
 * Falls back to English, then to the key. Reading `current` here makes every
 * call reactive to locale changes.
 */
export function t(key: string, params?: Record<string, string | number>): string {
	const msg = lookup(catalogs[current], key) ?? lookup(en, key) ?? key;
	if (!params) return msg;
	return msg.replace(/\{(\w+)\}/g, (_, k) => String(params[k] ?? `{${k}}`));
}
