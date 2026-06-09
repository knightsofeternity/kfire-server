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
	}
};
