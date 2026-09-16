-- 0035: résultats de matchs Rocket League, rapportés par le client de bureau.
--
-- Rocket League n'a pas d'API joueur publique : la seule source est une socket
-- TCP que le jeu ouvre sur la machine du joueur. Le client de bureau la lit,
-- agrège en mémoire et envoie UN résumé par match, donc cette table est nourrie
-- par le plan de contrôle WebSocket, pas par un crawler.
--
-- La garantie de confidentialité est ici STRUCTURELLE, et non une règle qu'il
-- faudrait se rappeler d'appliquer : la table n'a AUCUNE colonne de texte libre.
-- Chaque colonne est un entier, un booléen ou une énumération sous CHECK, donc
-- elle est incapable de porter le pseudonyme d'un coéquipier ou d'un adversaire,
-- quoi qu'un client envoie. Le flux du jeu, lui, les porte tous : ils restent
-- sur la machine du membre, qui n'en émet que des faits sur lui-même.
--
-- Il n'y a délibérément pas de colonne arena : un identifiant de carte serait
-- inoffensif, mais une colonne de texte libre rouvrirait la porte que cette
-- table ferme, pour un confort marginal.
--
-- playlist est l'identifiant numérique brut de Psyonix, jamais un libellé :
-- l'identifiant est le fait, le libellé est de la présentation et il est
-- localisé. Même raisonnement que hero_card_id en 0034.
--
-- duration_seconds est la durée réelle mesurée par le client entre l'ouverture
-- et la fermeture du match. Le plafond à 7200 refuse une horloge folle sans
-- interdire une prolongation à rallonge.

BEGIN;

CREATE TABLE rocket_league_matches (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id           uuid        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    game_id           uuid        NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    playlist          int         NOT NULL CHECK (playlist >= 0),
    team_size         int         NOT NULL CHECK (team_size BETWEEN 1 AND 4),
    player_team       int         NOT NULL CHECK (player_team IN (0, 1)),
    team_blue_score   int         NOT NULL CHECK (team_blue_score >= 0),
    team_orange_score int         NOT NULL CHECK (team_orange_score >= 0),
    result            text        NOT NULL CHECK (result IN ('win', 'loss', 'draw')),
    goals             int         NOT NULL CHECK (goals >= 0),
    assists           int         NOT NULL CHECK (assists >= 0),
    saves             int         NOT NULL CHECK (saves >= 0),
    shots             int         NOT NULL CHECK (shots >= 0),
    score             int         NOT NULL CHECK (score >= 0),
    demos             int         NOT NULL CHECK (demos >= 0),
    mvp               boolean     NOT NULL DEFAULT false,
    duration_seconds  int         NOT NULL CHECK (duration_seconds BETWEEN 0 AND 7200),
    played_at         timestamptz NOT NULL,
    created_at        timestamptz NOT NULL DEFAULT now(),
    -- Le client peut réémettre un match resté en file après une reconnexion ;
    -- le même membre ne peut pas avoir deux matchs se terminant au même instant.
    UNIQUE (user_id, played_at)
);

CREATE INDEX rocket_league_matches_game_idx ON rocket_league_matches (game_id, played_at DESC);

COMMIT;
