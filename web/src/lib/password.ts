// Lightweight, dependency-free password strength heuristic for UX feedback.
// The server enforces the real policy (length + common/derived rejection);
// this only guides the user while typing.

export type Strength = {
	score: 0 | 1 | 2 | 3 | 4;
	label: string;
};

const COMMON = ['password', 'azerty', 'qwerty', 'motdepasse', 'welcome', 'iloveyou', 'letmein'];

export function passwordStrength(pw: string): Strength {
	if (!pw) return { score: 0, label: '' };

	const base = pw.toLowerCase().replace(/[0-9!@#$%^&*._-]+$/, '');
	if (COMMON.includes(base)) return { score: 0, label: 'Too common' };
	if (pw.length < 12) return { score: 1, label: 'Too short' };

	let variety = 0;
	if (/[a-z]/.test(pw)) variety++;
	if (/[A-Z]/.test(pw)) variety++;
	if (/[0-9]/.test(pw)) variety++;
	if (/[^a-zA-Z0-9]/.test(pw)) variety++;

	// Length carries the most weight (passphrases are strong).
	let score = 2;
	if (pw.length >= 16 || variety >= 3) score = 3;
	if (pw.length >= 20 || (pw.length >= 16 && variety >= 3)) score = 4;

	const labels = ['Very weak', 'Weak', 'Fair', 'Good', 'Strong'];
	return { score: score as Strength['score'], label: labels[score] };
}
