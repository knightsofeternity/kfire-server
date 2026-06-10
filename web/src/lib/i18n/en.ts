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
		games: 'Games',
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
	},
	dashboard: {
		title: 'Dashboard',
		playing: '{count} playing',
		online: '{count} online',
		liveConnection: 'Live connection',
		live: 'Live',
		connecting: 'connecting...',
		disconnected: 'disconnected',
		loading: 'Loading...',
		noMembers: 'No members yet.',
		since: 'since {time}',
		onlineStatus: 'Online'
	},
	players: {
		heading: 'Players',
		searchPlaceholder: 'Search...',
		loading: 'Loading...',
		empty: 'No players found.'
	},
	profile: {
		loading: 'Loading...',
		loadError: 'failed to load profile',
		memberSince: 'member since',
		playing: 'Playing',
		totalTracked: 'total tracked',
		achievements: 'achievements',
		hoursPerGame: 'Hours per game',
		noSessionsYet: 'No sessions recorded yet.',
		recentAchievements: 'Recent achievements',
		achievementsTitle: 'Achievements',
		allGames: 'All games',
		noAchievements: 'No achievements yet.',
		recentSessions: 'Recent sessions',
		noSessions: 'No sessions yet.',
		loadMore: 'Load more',
		loadingMore: 'Loading...'
	},
	game: {
		loading: 'Loading...',
		loadError: 'failed to load game',
		totalPlayed: 'Total played in org',
		players: 'Players',
		leaderboard: 'Leaderboard',
		noPlayers: 'Nobody has played this yet.',
		achievements: 'Achievements',
		noAchievements: 'No achievements unlocked yet.',
		unlockedBy: '{count} members'
	},
	gamesList: {
		title: 'Games',
		search: 'Search games...',
		loading: 'Loading...',
		empty: 'No games played yet.',
		noMatch: 'No game matches your search.',
		players: '{count} players',
		onePlayer: '1 player'
	},
	account: {
		title: 'Account',
		admin: 'admin',
		memberSince: 'member since {date}',
		notLinked: 'Not linked',
		unlink: 'Unlink',
		connectedAccounts: 'Connected accounts',
		comingNext: 'Riot, Epic, Xbox and PlayStation are coming next.',
		privacy: {
			title: 'Privacy',
			toggleLabel: 'Show my game activity',
			toggleHint:
				"When off, other members see you as online but never see which game you're playing. Your own history is unaffected.",
			ariaLabel: 'Toggle game activity visibility'
		},
		linkResult: {
			linked: 'Account linked.',
			denied: 'Sign-in was cancelled or failed.',
			expired: 'The link request expired, please try again.',
			conflict: 'That account is already linked to another member.',
			error: 'This connector is not available on the instance.'
		},
		steam: {
			link: 'Link Steam',
			syncNow: 'Sync now',
			syncEmpty: 'No games imported. Steam only shares your library when your game details are public.',
			syncSuccess: 'Imported {games} games and {achievements} achievements.',
			settingsLinkText: 'Steam, Edit Profile, Privacy Settings',
			myProfile: 'My profile',
			gameDetails: 'Game details',
			publicHintPre:
				'Steam only shares your games and achievements when your profile is public. In',
			publicHintMid: ', set',
			publicHintAnd: 'and',
			publicHintPost: 'to Public.',
			privateWorkaroundPre: 'Want to keep your profile private? Set it public, click',
			privateWorkaroundPost:
				'once, then switch it back: the imported games and achievements stay. Re-sync the same way whenever you want to refresh.',
			privacyHint: {
				title: 'Make your Steam library visible',
				step1pre: 'Open',
				step1link: 'Steam - Edit Profile - Privacy Settings',
				step1post: '.',
				step2pre: 'Set',
				step2post: 'to Public.',
				step3pre: 'Set',
				step3post: 'to Public (this is the important one).',
				step4: 'Untick "Always keep my total playtime private", save, then click Sync now again.'
			}
		},
		battlenet: {
			link: 'Link Battle.net'
		}
	},
	admin: {
		title: 'Admin',
		loading: 'Loading...',
		brandingHeading: 'Branding',
		logoLabel: 'Clan / team logo',
		logoHint: 'PNG or JPEG, up to 2 MB. Shown next to the KFIRE logo in the header.',
		noLogo: 'No logo',
		upload: 'Upload',
		remove: 'Remove',
		dominantColor: 'Dominant color',
		errorSaveColor: 'could not save the color',
		errorUpload: 'upload failed',
		errorRemoveLogo: 'could not remove the logo',
		inviteHeading: 'Invite a member',
		noteLabel: 'Note (optional - who is it for?)',
		notePlaceholder: 'e.g. Lancelot',
		roleLabel: 'Role',
		roleMember: 'Member',
		roleAdmin: 'Admin',
		createLink: 'Create link',
		inviteDefaultNote: 'Invite',
		copied: 'Copied!',
		copyLink: 'Copy link',
		revoke: 'Revoke',
		noInvites: 'No pending invites.',
		membersHeading: 'Members ({count})',
		you: 'you',
		banned: 'banned',
		makeMember: 'Make member',
		makeAdmin: 'Make admin',
		unban: 'Unban',
		ban: 'Ban',
		errorLoad: 'failed to load',
		errorGeneric: 'failed',
		resetPassword: 'Reset password',
		resetLinkTitle: 'Password reset link',
		resetLinkHint: 'Share this single-use link with the member. It expires in 1 hour.'
	},
	reset: {
		title: 'Reset your password',
		subtitleFor: 'Set a new password for {username}.',
		invalidLink: 'This reset link is invalid or has expired. Ask an admin for a new one.',
		password: 'New password',
		submit: 'Set new password',
		submitting: 'Saving...',
		success: 'Password updated. You can now sign in.',
		toLogin: 'Go to sign in',
		genericError: 'something went wrong'
	},
	download: {
		title: 'Get the KFIRE desktop app',
		subtitle: 'Runs in your tray, detects your games and shares your presence.',
		loading: 'Loading latest release...',
		noRelease: 'No build published yet.',
		noReleaseHint: 'The desktop app builds are on the way. Check the',
		releasesPage: 'releases page',
		noReleaseOr: 'or build it from source.',
		detected: 'Detected: {os}',
		version: 'version',
		yourPlatform: 'your platform',
		downloadFor: 'Download for {os}',
		noBuildForOS: 'No build for {os} in this release - see other platforms below.',
		otherPlatforms: 'Other platforms',
		firstLaunch: 'First launch',
		step1: "Open the app and enter this server's address.",
		step2: 'It opens your browser to confirm - approve the device here.',
		step3: "You're connected. The app lives in your tray.",
		securityTitle: "Caution - Security warning on first run? It's expected.",
		securityBody:
			'KFIRE is open-source and the installers aren\'t code-signed yet, so your OS may warn that the publisher is "unknown". The app is safe - here is how to proceed:',
		securityWindows: '(SmartScreen): click <em>More info</em> then <em>Run anyway</em>.',
		securityMacos: '(Gatekeeper): right-click the app, choose <em>Open</em>, then <em>Open</em>.',
		securityLinux: 'no warning - just run it.'
	},
	link: {
		title: 'Link a device',
		successTitle: 'Device linked',
		successBody: 'You can return to the KFIRE app - it will connect automatically.',
		deviceWantsToLink: 'A device wants to link to your account',
		pairingCode: 'Pairing code',
		approveWarning: 'Only approve if you just started linking the KFIRE app on this device.',
		approve: 'Approve & link this device',
		linking: 'Linking...',
		enterCode: 'Enter the code shown in the app',
		continue: 'Continue',
		errorInvalidCode: 'Invalid code',
		errorApprovalFailed: 'Approval failed'
	}
};

export type Catalog = typeof en;
