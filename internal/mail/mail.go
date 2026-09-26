// Package mail sends the few emails the server needs.
//
// Only one transport exists today, Brevo's HTTP API, behind an interface so a
// plain SMTP sender for self-hosters can be added without touching callers.
package mail

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Message is one email. To is the only recipient.
type Message struct {
	To      string
	ToName  string
	Subject string
	Text    string
	HTML    string
}

// Sender delivers a message.
type Sender interface {
	Send(ctx context.Context, m Message) error
}

// Brevo sends through Brevo's transactional API.
type Brevo struct {
	APIKey    string
	FromEmail string
	FromName  string
	// BaseURL is https://api.brevo.com; tests point it at a local server.
	BaseURL string
	Client  *http.Client
}

// NewBrevo builds a sender. The caller decides whether mail is enabled at all.
func NewBrevo(apiKey, fromEmail, fromName string) *Brevo {
	return &Brevo{
		APIKey: apiKey, FromEmail: fromEmail, FromName: fromName,
		BaseURL: "https://api.brevo.com",
		Client:  &http.Client{Timeout: 10 * time.Second},
	}
}

type brevoContact struct {
	Email string `json:"email"`
	Name  string `json:"name,omitempty"`
}

type brevoEmail struct {
	Sender      brevoContact   `json:"sender"`
	To          []brevoContact `json:"to"`
	Subject     string         `json:"subject"`
	TextContent string         `json:"textContent"`
	HTMLContent string         `json:"htmlContent"`
}

// Send posts the message. Any answer but 201 is an error carrying the HTTP
// status and Brevo's own error code, never the key nor the recipient: the
// error ends up in logs.
func (b *Brevo) Send(ctx context.Context, m Message) error {
	body, err := json.Marshal(brevoEmail{
		Sender:      brevoContact{Email: b.FromEmail, Name: b.FromName},
		To:          []brevoContact{{Email: m.To, Name: m.ToName}},
		Subject:     m.Subject,
		TextContent: m.Text,
		HTMLContent: m.HTML,
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.BaseURL+"/v3/smtp/email", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("api-key", b.APIKey)
	req.Header.Set("accept", "application/json")
	req.Header.Set("content-type", "application/json")
	res, err := b.Client.Do(req)
	if err != nil {
		return fmt.Errorf("brevo: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusCreated {
		return nil
	}
	var e struct {
		Code string `json:"code"`
	}
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
	_ = json.Unmarshal(raw, &e)
	return fmt.Errorf("brevo: status %d, code %q", res.StatusCode, e.Code)
}
