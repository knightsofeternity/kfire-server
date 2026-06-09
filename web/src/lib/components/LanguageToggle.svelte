<script lang="ts">
	import { getLocale, setLocale, localeLabels, t } from '$lib/i18n';

	function toggle() {
		setLocale(getLocale() === 'fr' ? 'en' : 'fr');
	}
</script>

<button
	onclick={toggle}
	type="button"
	aria-label={t('toggle.language')}
	title={t('toggle.language')}
	class="font-display grid h-9 min-w-9 place-items-center px-2 text-xs font-bold tracking-wider border border-[var(--color-border)] text-[var(--color-muted)] transition-colors hover:border-[var(--color-brand)] hover:text-[var(--color-brand)]"
	style="clip-path: polygon(6px 0, 100% 0, 100% calc(100% - 6px), calc(100% - 6px) 100%, 0 100%, 0 6px);"
>
	{localeLabels[getLocale()]}
</button>
