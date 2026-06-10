// Apply the saved theme before paint to avoid a flash of the wrong theme.
// External (not inline) so the CSP can keep script-src 'self'.
try {
	if (localStorage.getItem('kfire-theme') === 'light') {
		document.documentElement.dataset.theme = 'light';
	}
} catch (e) {}
