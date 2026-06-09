// English catalog. Keys are namespaced by area; French (fr.ts) mirrors this
// shape. Missing keys fall back to English, then to the key itself.
export const en = {
	common: {
		loading: 'Loading...',
		save: 'Save',
		cancel: 'Cancel',
		dismiss: 'dismiss',
		remove: 'Remove',
		signOut: 'Sign out',
		unknownResult: 'Unknown result.'
	},
	nav: {
		dashboard: 'Dashboard',
		players: 'Players',
		download: 'Get the app',
		admin: 'Admin',
		account: 'Account'
	},
	footer: {
		org: 'Knights of Eternity',
		copyright: 'Knights of Eternity'
	},
	toggle: {
		light: 'Switch to light',
		dark: 'Switch to dark',
		language: 'Switch language'
	},
	status: {
		connected: 'connected',
		connecting: 'connecting...',
		disconnected: 'disconnected',
		notLinked: 'not linked',
		online: 'Online',
		offline: 'Offline',
		inGame: 'In game'
	},
	login: {
		setupSubtitle: 'Welcome. Create the first account; it becomes the admin.',
		signinSubtitle: 'Sign in to your organization',
		createSubtitle: 'Create your account',
		invited: 'You were invited to join. Set your details below.',
		displayName: 'Display name',
		email: 'Email',
		password: 'Password',
		passwordHint: 'at least 12 characters; a passphrase works great',
		strength: {
			tooCommon: 'Too common',
			tooShort: 'Too short',
			veryWeak: 'Very weak',
			weak: 'Weak',
			fair: 'Fair',
			good: 'Good',
			strong: 'Strong'
		},
		signin: 'Sign in',
		create: 'Create account',
		pleaseWait: 'Please wait...',
		noAccount: 'No account yet?',
		haveAccount: 'Already have an account?',
		createOne: 'Create one',
		signinLink: 'Sign in',
		inviteOnly: 'Registration is invite-only. Ask an admin for an invite link.',
		genericError: 'something went wrong'
	}
};

export type Catalog = typeof en;
