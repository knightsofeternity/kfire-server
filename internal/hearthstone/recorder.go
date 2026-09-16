package hearthstone

import (
	"context"
	"encoding/json"
	"regexp"
	"time"

	"github.com/knightsofeternity/kfire-server/internal/matchrecord"
	"github.com/knightsofeternity/kfire-server/internal/store"
)

// clockSkewTolerance est l'avance maximale tolérée sur l'horloge du serveur
// avant qu'un résultat de match soit refusé.
const clockSkewTolerance = 5 * time.Minute

// payload est un match terminé, déjà résumé par le client de bureau. Le client
// lit les logs du jeu et n'envoie que ceci : jamais le nom de l'adversaire,
// jamais une carte.
//
// Turns et Placement sont des pointeurs parce qu'un match mérite d'être
// enregistré même quand un champ secondaire était illisible.
type payload struct {
	Mode      string `json:"mode"`
	Result    string `json:"result"`
	Turns     *int   `json:"turns"`
	Placement *int   `json:"placement"`
	// HeroCardID est le héros de Bataille de Gueux, sous la forme de
	// l'identifiant de carte que le jeu écrit dans son propre log. Jamais le
	// nom du héros : le log est localisé, donc deux membres jouant le même
	// héros rapporteraient deux chaînes différentes.
	HeroCardID *string   `json:"hero_card_id"`
	PlayedAt   time.Time `json:"played_at"`
}

// heroCardID est la forme d'un identifiant de carte. Refuser autre chose garde
// la colonne incapable de porter un nom, ce qui est tout l'objet.
var heroCardID = regexp.MustCompile(`^[A-Za-z0-9_]{1,64}$`)

// valid dit si la charge utile mérite d'être écrite. La base applique les mêmes
// règles, mais refuser ici donne au client une erreur claire plutôt qu'un échec
// d'écriture opaque.
//
// Le slug n'est PAS vérifié ici : il appartient à l'aiguillage, pas au jeu.
func (p payload) valid() bool {
	if p.PlayedAt.IsZero() {
		return false
	}
	if p.Mode != "battlegrounds" && p.Mode != "constructed" {
		return false
	}
	if p.Result != "win" && p.Result != "loss" && p.Result != "draw" {
		return false
	}
	if p.Placement != nil && (*p.Placement < 1 || *p.Placement > 8) {
		return false
	}
	if p.Turns != nil && *p.Turns < 0 {
		return false
	}
	if p.HeroCardID != nil && !heroCardID.MatchString(*p.HeroCardID) {
		return false
	}
	// Un match en file d'attente peut être vieux, jamais futur. La tolérance
	// absorbe une horloge de bureau qui dérive un peu sans laisser une horloge
	// mal réglée empoisonner la date de dernière partie de tout le roster.
	if p.PlayedAt.After(time.Now().Add(clockSkewTolerance)) {
		return false
	}
	return true
}

// Recorder écrit les matchs Hearthstone rapportés par le client de bureau.
type Recorder struct {
	st *store.Store
}

// NewRecorder construit l'enregistreur.
func NewRecorder(st *store.Store) *Recorder { return &Recorder{st: st} }

// Slug rend le slug de catalogue revendiqué.
func (r *Recorder) Slug() string { return "hearthstone" }

// Record valide puis écrit un match.
func (r *Recorder) Record(ctx context.Context, userID, gameID string, raw json.RawMessage) error {
	var p payload
	if err := json.Unmarshal(raw, &p); err != nil || !p.valid() {
		return matchrecord.ErrInvalidPayload
	}
	return r.st.InsertHearthstoneMatch(ctx, store.HearthstoneMatch{
		UserID: userID, GameID: gameID,
		Mode: p.Mode, Result: p.Result,
		Turns: p.Turns, Placement: p.Placement, HeroCardID: p.HeroCardID,
		PlayedAt: p.PlayedAt,
	})
}
