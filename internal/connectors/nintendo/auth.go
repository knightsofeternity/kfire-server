package nintendo

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	appClientID   = "71b963c1b7b6d119"
	appRedirect   = "npf71b963c1b7b6d119://auth"
	appScope      = "openid user user.birthday user.mii user.screenName"
	accountsAgent = "OnlineLounge/3.5.0 NASDKAPI Android"
)

// Accounts is accounts.nintendo.com, overridable for tests.
type Accounts struct {
	Base string // https://accounts.nintendo.com
	HTTP *http.Client
}

func NewAccounts() *Accounts {
	return &Accounts{Base: "https://accounts.nintendo.com", HTTP: &http.Client{Timeout: 30 * time.Second}}
}

// Login is one pending bot login: the admin opens URL, signs in with the bot's
// account and pastes back the npf link; the verifier then proves it was us.
type Login struct {
	State    string
	Verifier string
	URL      string
}

func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func random(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err) // crypto/rand never fails on supported platforms
	}
	return b64(b)
}

// Challenge is the S256 PKCE challenge of a verifier.
func Challenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return b64(sum[:])
}

// NewLogin prepares the Nintendo Switch Online app's own login flow.
func (a *Accounts) NewLogin() Login {
	l := Login{State: random(36), Verifier: random(32)}
	q := url.Values{
		"state": {l.State}, "redirect_uri": {appRedirect}, "client_id": {appClientID},
		"scope": {appScope}, "response_type": {"session_token_code"},
		"session_token_code_challenge": {Challenge(l.Verifier)}, "session_token_code_challenge_method": {"S256"},
		"theme": {"login_form"},
	}
	l.URL = a.Base + "/connect/1.0.0/authorize?" + q.Encode()
	return l
}

// ParseRedirect reads the link copied from the "Select this account" button.
func ParseRedirect(link string) (code, state string, err error) {
	link = strings.TrimSpace(link)
	if !strings.HasPrefix(link, appRedirect) {
		return "", "", errors.New("nintendo: not an npf71b963c1b7b6d119://auth link")
	}
	i := strings.Index(link, "#")
	if i < 0 {
		return "", "", errors.New("nintendo: the link has no session code")
	}
	v, err := url.ParseQuery(link[i+1:])
	if err != nil {
		return "", "", err
	}
	code, state = v.Get("session_token_code"), v.Get("state")
	if code == "" || state == "" {
		return "", "", errors.New("nintendo: the link has no session code")
	}
	return code, state, nil
}

// Exchange turns the session code into the bot's long-lived session token.
func (a *Accounts) Exchange(ctx context.Context, code, verifier string) (string, error) {
	form := url.Values{"client_id": {appClientID}, "session_token_code": {code}, "session_token_code_verifier": {verifier}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.Base+"/connect/1.0.0/api/session_token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", accountsAgent)
	resp, err := a.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var body struct {
		SessionToken string `json:"session_token"`
		Error        string `json:"error"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if resp.StatusCode != http.StatusOK || body.SessionToken == "" {
		return "", fmt.Errorf("nintendo: session token exchange: HTTP %d %s", resp.StatusCode, body.Error)
	}
	return body.SessionToken, nil
}
