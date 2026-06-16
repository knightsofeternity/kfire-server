// French catalog. Mirrors the shape of en.ts.
import type { Catalog } from './en';

export const fr: Catalog = {
	common: {
		loading: 'Chargement...',
		save: 'Enregistrer',
		cancel: 'Annuler',
		dismiss: 'fermer',
		remove: 'Retirer',
		signOut: 'Se déconnecter',
		unknownResult: 'Résultat inconnu.'
	},
	nav: {
		dashboard: 'Tableau de bord',
		players: 'Joueurs',
		games: 'Jeux',
		download: "Obtenir l'app",
		admin: 'Admin',
		account: 'Compte'
	},
	footer: {
		org: 'Knights of Eternity',
		copyright: 'Knights of Eternity'
	},
	toggle: {
		light: 'Passer en clair',
		dark: 'Passer en sombre',
		language: 'Changer de langue'
	},
	status: {
		connected: 'connecté',
		connecting: 'connexion...',
		disconnected: 'déconnecté',
		notLinked: 'non lié',
		online: 'En ligne',
		offline: 'Hors ligne',
		inGame: 'En jeu'
	},
	login: {
		setupSubtitle: 'Bienvenue. Créez le premier compte ; il devient administrateur.',
		signinSubtitle: 'Connectez-vous à votre organisation',
		createSubtitle: 'Créez votre compte',
		invited: 'Vous avez été invité à rejoindre. Renseignez vos informations ci-dessous.',
		displayName: "Nom d'affichage",
		email: 'E-mail',
		password: 'Mot de passe',
		passwordHint: 'au moins 12 caractères ; une phrase de passe est idéale',
		strength: {
			tooCommon: 'Trop courant',
			tooShort: 'Trop court',
			veryWeak: 'Très faible',
			weak: 'Faible',
			fair: 'Correct',
			good: 'Bon',
			strong: 'Fort'
		},
		signin: 'Se connecter',
		create: 'Créer le compte',
		pleaseWait: 'Veuillez patienter...',
		noAccount: 'Pas encore de compte ?',
		haveAccount: 'Vous avez déjà un compte ?',
		createOne: 'En créer un',
		signinLink: 'Se connecter',
		inviteOnly: "L'inscription est sur invitation. Demandez un lien à un administrateur.",
		genericError: "une erreur s'est produite"
	},
	dashboard: {
		title: 'Tableau de bord',
		playing: '{count} en jeu',
		online: '{count} en ligne',
		liveConnection: 'Connexion en direct',
		live: 'En direct',
		connecting: 'connexion...',
		disconnected: 'déconnecté',
		loading: 'Chargement...',
		noMembers: "Aucun membre pour l'instant.",
		since: 'depuis {time}',
		onlineStatus: 'En ligne'
	},
	players: {
		heading: 'Joueurs',
		searchPlaceholder: 'Rechercher...',
		loading: 'Chargement...',
		empty: 'Aucun joueur trouvé.'
	},
	profile: {
		loading: 'Chargement...',
		loadError: 'impossible de charger le profil',
		memberSince: 'membre depuis le',
		playing: 'En train de jouer à',
		totalTracked: 'total suivi',
		achievements: 'succès',
		hoursPerGame: 'Heures par jeu',
		noSessionsYet: "Aucune session enregistrée pour l'instant.",
		recentAchievements: 'Succès récents',
		achievementsTitle: 'Succès',
		allGames: 'Tous les jeux',
		noAchievements: "Aucun succès pour l'instant.",
		recentSessions: 'Sessions récentes',
		today: "Aujourd'hui",
		yesterday: 'Hier',
		noSessions: "Aucune session pour l'instant.",
		loadMore: 'Charger plus',
		loadingMore: 'Chargement...',
		allOwnedGames: 'Voir tous les jeux',
		playedBadge: 'Joué',
		ownedFrom: 'via {source}'
	},
	game: {
		loading: 'Chargement...',
		loadError: 'impossible de charger le jeu',
		totalPlayed: "Total joué dans l'org",
		players: 'Joueurs',
		leaderboard: 'Classement',
		noPlayers: "Personne n'a encore joué à ce jeu.",
		achievements: 'Succès',
		noAchievements: "Aucun succès débloqué pour l'instant.",
		unlockedBy: '{count} membres',
		wowCharacters: 'Personnages WoW',
		ilvl: 'ilvl',
		level: 'Niv',
		achievementPoints: 'PA',
		bnetProfiles: 'Profils Battle.net',
		paragon: 'Parangon',
		heroes: 'héros',
		searchAchievements: 'Rechercher des hauts faits...',
		showMore: 'Voir plus',
		wowAchievements: 'Hauts faits'
	},
	playerGame: {
		loading: 'Chargement...',
		loadError: 'impossible de charger les données du jeu',
		backToProfile: 'Retour au profil',
		noPlaytime: 'Aucun temps de jeu enregistré.',
		playtime: 'Temps de jeu',
		sessions: 'sessions'
	},
	gamesList: {
		title: 'Jeux',
		search: 'Rechercher un jeu...',
		loading: 'Chargement...',
		empty: "Aucun jeu joué pour l'instant.",
		noMatch: 'Aucun jeu ne correspond à votre recherche.',
		players: '{count} joueurs',
		onePlayer: '1 joueur'
	},
	account: {
		title: 'Compte',
		admin: 'admin',
		memberSince: 'membre depuis {date}',
		notLinked: 'Non lié',
		unlink: 'Délier',
		connectedAccounts: 'Comptes connectés',
		comingNext: 'Riot, Epic et PlayStation arrivent bientôt.',
		privacy: {
			title: 'Confidentialité',
			toggleLabel: 'Afficher mon activité de jeu',
			toggleHint:
				"Désactivé, les autres membres vous voient en ligne mais ne voient jamais à quel jeu vous jouez. Votre historique personnel n'est pas affecté.",
			ariaLabel: "Basculer la visibilité de l'activité de jeu",
			sessionsToggleLabel: 'Afficher mes sessions récentes',
			sessionsToggleHint:
				"Désactivé, les autres membres ne voient pas l'historique de vos sessions récentes sur votre profil. Vos statistiques par jeu et vos classements ne sont pas affectés.",
			sessionsAriaLabel: 'Basculer la visibilité des sessions récentes'
		},
		linkResult: {
			linked: 'Compte lié.',
			denied: 'La connexion a été annulée ou a échoué.',
			expired: 'La demande de liaison a expiré, veuillez réessayer.',
			conflict: 'Ce compte est déjà lié à un autre membre.',
			error: "Ce connecteur n'est pas disponible sur cette instance."
		},
		steam: {
			link: 'Lier Steam',
			syncNow: 'Synchroniser',
			syncEmpty:
				'Aucun jeu importé. Steam ne partage votre bibliothèque que si vos détails de jeu sont publics.',
			syncSuccess: 'Importé {games} jeux et {achievements} succès.',
			settingsLinkText: 'Steam, Modifier le profil, Paramètres de confidentialité',
			myProfile: 'Mon profil',
			gameDetails: 'Détails du jeu',
			publicHintPre: 'Steam ne partage vos jeux et succès que si votre profil est public. Dans',
			publicHintMid: ', définissez',
			publicHintAnd: 'et',
			publicHintPost: 'sur Public.',
			privateWorkaroundPre: 'Vous souhaitez garder votre profil privé ? Passez-le en public, cliquez sur',
			privateWorkaroundPost:
				'une fois, puis remettez-le en privé : les jeux et succès importés sont conservés. Resynchronisez de la même façon pour actualiser.',
			privacyHint: {
				title: 'Rendez votre bibliothèque Steam visible',
				step1pre: 'Ouvrez',
				step1link: 'Steam, Modifier le profil, Paramètres de confidentialité',
				step1post: '.',
				step2pre: 'Définissez',
				step2post: 'sur Public.',
				step3pre: 'Définissez',
				step3post: "sur Public (c'est le plus important).",
				step4:
					'Décochez "Toujours garder mon temps de jeu total privé", enregistrez, puis cliquez à nouveau sur Synchroniser.'
			}
		},
		battlenet: {
			link: 'Lier Battle.net'
		},
		xbox: {
			link: 'Lier Xbox',
			linking: 'Liaison… (~30s)',
			slowHint: 'La liaison peut prendre ~30s après la connexion Microsoft — ne ferme pas l’onglet.'
		},
		bnetReconnectStats: 'Reconnecte Battle.net pour activer tes stats de jeu'
	},
	admin: {
		title: 'Admin',
		loading: 'Chargement...',
		brandingHeading: 'Apparence',
		logoLabel: "Logo du clan / de l'équipe",
		logoHint: "PNG ou JPEG, 2 Mo max. Affiché à côté du logo KFIRE dans l'en-tête.",
		noLogo: 'Aucun logo',
		upload: 'Téléverser',
		remove: 'Supprimer',
		dominantColor: 'Couleur dominante',
		errorSaveColor: "impossible d'enregistrer la couleur",
		errorUpload: 'échec du téléversement',
		errorRemoveLogo: 'impossible de supprimer le logo',
		inviteHeading: 'Inviter un membre',
		noteLabel: 'Note (facultatif, pour qui ?)',
		notePlaceholder: 'ex. Lancelot',
		roleLabel: 'Rôle',
		roleMember: 'Membre',
		roleAdmin: 'Admin',
		createLink: 'Créer le lien',
		inviteDefaultNote: 'Invitation',
		copied: 'Copié !',
		copyLink: 'Copier le lien',
		revoke: 'Révoquer',
		noInvites: 'Aucune invitation en attente.',
		membersHeading: 'Membres ({count})',
		you: 'vous',
		banned: 'banni',
		makeMember: 'Passer membre',
		makeAdmin: 'Passer admin',
		unban: 'Débannir',
		ban: 'Bannir',
		errorLoad: 'échec du chargement',
		errorGeneric: 'échec',
		resetPassword: 'Réinitialiser le mot de passe',
		resetLinkTitle: 'Lien de réinitialisation',
		resetLinkHint: 'Partagez ce lien à usage unique avec le membre. Il expire dans 1 heure.'
	},
	reset: {
		title: 'Réinitialisez votre mot de passe',
		subtitleFor: 'Définissez un nouveau mot de passe pour {username}.',
		invalidLink: 'Ce lien est invalide ou a expiré. Demandez-en un nouveau à un administrateur.',
		password: 'Nouveau mot de passe',
		submit: 'Définir le nouveau mot de passe',
		submitting: 'Enregistrement...',
		success: 'Mot de passe mis à jour. Vous pouvez maintenant vous connecter.',
		toLogin: 'Aller à la connexion',
		genericError: "une erreur s'est produite"
	},
	download: {
		title: "Obtenir l'application bureau KFIRE",
		subtitle:
			'Elle tourne dans votre barre des tâches, détecte vos jeux et partage votre présence.',
		loading: 'Chargement de la dernière version...',
		noRelease: "Aucune version publiée pour l'instant.",
		noReleaseHint: "Les builds de l'application bureau arrivent bientôt. Consultez la",
		releasesPage: 'page des versions',
		noReleaseOr: 'ou compilez-la depuis les sources.',
		detected: 'Détecté : {os}',
		version: 'version',
		yourPlatform: 'votre plateforme',
		downloadFor: 'Télécharger pour {os}',
		noBuildForOS: 'Aucun build pour {os} dans cette version ; voir les autres plateformes ci-dessous.',
		otherPlatforms: 'Autres plateformes',
		firstLaunch: 'Premier lancement',
		step1: "Ouvrez l'application et entrez l'adresse de ce serveur.",
		step2: 'Elle ouvre votre navigateur pour confirmer ; approuvez l\'appareil ici.',
		step3: "Vous êtes connecté. L'application vit dans votre barre des tâches.",
		securityTitle: 'Attention : un avertissement de sécurité au premier lancement ? C\'est normal.',
		securityBody:
			'KFIRE est open-source et les installeurs ne sont pas encore signés numériquement, votre OS peut donc avertir que l\'éditeur est "inconnu". L\'application est sûre, voici comment procéder :',
		securityWindows:
			'(SmartScreen) : cliquez sur <em>Plus d\'informations</em> puis <em>Exécuter quand même</em>.',
		securityMacos:
			'(Gatekeeper) : faites un clic droit sur l\'application, choisissez <em>Ouvrir</em>, puis <em>Ouvrir</em>.',
		securityLinux: 'aucun avertissement, lancez-la directement.'
	},
	link: {
		title: 'Associer un appareil',
		successTitle: 'Appareil associé',
		successBody: 'Vous pouvez retourner dans l\'application KFIRE ; elle se connectera automatiquement.',
		deviceWantsToLink: 'Un appareil souhaite se lier à votre compte',
		pairingCode: "Code d'association",
		approveWarning:
			"N'approuvez que si vous venez de démarrer l'association de l'application KFIRE sur cet appareil.",
		approve: 'Approuver et associer cet appareil',
		linking: 'Association en cours...',
		enterCode: "Entrez le code affiché dans l'application",
		continue: 'Continuer',
		errorInvalidCode: 'Code invalide',
		errorApprovalFailed: "Échec de l'approbation"
	}
};
