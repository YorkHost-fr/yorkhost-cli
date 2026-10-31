// Package api : client HTTP de l'API client YorkHost (/client/v1).
//
// Contrat de l'API (yorkhost-api, docs/openapi-client-v1.yaml) :
//   - Authorization: Bearer yhk_client_v1_… (clé API ou jeton OAuth) ;
//   - erreurs {"error": "…", "code": "…"} (code absent sur certaines routes) ;
//   - listes paginées {items, page, pageSize, total, hasMore} (?page=&limit=) ;
//   - 429 avec Retry-After (secondes).
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// TokenSource : fournit le jeton Bearer ; Refresh est appelé une fois sur un
// 401 (jeton OAuth expiré côté serveur avant l'échéance connue).
type TokenSource interface {
	Token(ctx context.Context) (string, error)
	Refresh(ctx context.Context) (bool, error)
}

// StaticToken : clé API fixe.
type StaticToken string

func (s StaticToken) Token(context.Context) (string, error) {
	if s == "" {
		return "", ErrNotLoggedIn
	}
	return string(s), nil
}
func (StaticToken) Refresh(context.Context) (bool, error) { return false, nil }

// ErrNotLoggedIn : aucun identifiant configuré.
var ErrNotLoggedIn = errors.New("non connecté : lancez `yorkhost login` (ou définissez YORKHOST_API_KEY)")

// Client : client de l'API.
type Client struct {
	BaseURL    string
	HTTP       *http.Client
	Tokens     TokenSource
	UserAgent  string
	Debug      io.Writer // journal des requêtes (sans secrets), nil = muet
	MaxRetries int
	// sleep : remplaçable dans les tests.
	sleep func(time.Duration)
}

// New : client avec des valeurs par défaut raisonnables.
func New(baseURL string, tokens TokenSource, userAgent string) *Client {
	return &Client{
		BaseURL:    strings.TrimRight(baseURL, "/"),
		HTTP:       &http.Client{Timeout: 90 * time.Second},
		Tokens:     tokens,
		UserAgent:  userAgent,
		MaxRetries: 2,
	}
}

// Response : réponse brute.
type Response struct {
	Status int
	Header http.Header
	Body   []byte
}

// Decode : JSON → v (corps vide toléré).
func (r *Response) Decode(v any) error {
	if len(bytes.TrimSpace(r.Body)) == 0 || v == nil {
		return nil
	}
	if err := json.Unmarshal(r.Body, v); err != nil {
		return fmt.Errorf("réponse illisible de l'API (HTTP %d) : %w", r.Status, err)
	}
	return nil
}

// Request : une requête.
type Request struct {
	Method string
	Path   string // ex. /client/v1/services
	Query  url.Values
	Body   any // sérialisé en JSON
	// NoAuth : route publique (OAuth, métadonnées).
	NoAuth bool
}

// Get : GET + décodage JSON.
func (c *Client) Get(ctx context.Context, path string, query url.Values, out any) error {
	resp, err := c.Do(ctx, Request{Method: http.MethodGet, Path: path, Query: query})
	if err != nil {
		return err
	}
	return resp.Decode(out)
}

// Post : POST JSON + décodage JSON.
func (c *Client) Post(ctx context.Context, path string, body, out any) (*Response, error) {
	resp, err := c.Do(ctx, Request{Method: http.MethodPost, Path: path, Body: body})
	if err != nil {
		return resp, err
	}
	return resp, resp.Decode(out)
}

// Put : PUT JSON + décodage JSON.
func (c *Client) Put(ctx context.Context, path string, body, out any) error {
	resp, err := c.Do(ctx, Request{Method: http.MethodPut, Path: path, Body: body})
	if err != nil {
		return err
	}
	return resp.Decode(out)
}

// Delete : DELETE + décodage JSON.
func (c *Client) Delete(ctx context.Context, path string, query url.Values, out any) error {
	resp, err := c.Do(ctx, Request{Method: http.MethodDelete, Path: path, Query: query})
	if err != nil {
		return err
	}
	return resp.Decode(out)
}

// Do : exécute la requête, avec relances bornées :
//   - 429 : toute méthode (la limite est vérifiée avant le handler, rien n'a
//     été exécuté), après Retry-After ;
//   - erreur réseau, 502/503/504 : GET seulement (idempotent) ;
//   - 401 avec un jeton OAuth : un rafraîchissement puis une nouvelle tentative.
func (c *Client) Do(ctx context.Context, r Request) (*Response, error) {
	var payload []byte
	if r.Body != nil {
		b, err := json.Marshal(r.Body)
		if err != nil {
			return nil, err
		}
		payload = b
	}
	refreshed := false
	for attempt := 0; ; attempt++ {
		resp, err := c.once(ctx, r, payload)
		if err != nil {
			if r.Method == http.MethodGet && attempt < c.MaxRetries && retryableNetErr(ctx, err) {
				c.wait(backoff(attempt))
				continue
			}
			return nil, err
		}
		if resp.Status >= 200 && resp.Status < 300 {
			return resp, nil
		}
		apiErr := parseError(resp)
		switch {
		case resp.Status == http.StatusUnauthorized && !refreshed && !r.NoAuth && c.Tokens != nil:
			refreshed = true
			ok, rerr := c.Tokens.Refresh(ctx)
			if rerr != nil {
				return resp, rerr
			}
			if ok {
				attempt--
				continue
			}
		case resp.Status == http.StatusTooManyRequests && attempt < c.MaxRetries:
			if d := apiErr.RetryAfter; d > 0 && d <= 30*time.Second {
				c.wait(d)
				continue
			}
			if apiErr.RetryAfter == 0 {
				c.wait(backoff(attempt))
				continue
			}
		case r.Method == http.MethodGet && attempt < c.MaxRetries &&
			(resp.Status == http.StatusBadGateway || resp.Status == http.StatusServiceUnavailable || resp.Status == http.StatusGatewayTimeout):
			c.wait(backoff(attempt))
			continue
		}
		return resp, apiErr
	}
}

func (c *Client) once(ctx context.Context, r Request, payload []byte) (*Response, error) {
	u := c.BaseURL + r.Path
	if len(r.Query) > 0 {
		u += "?" + r.Query.Encode()
	}
	var body io.Reader
	if payload != nil {
		body = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, r.Method, u, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}
	if !r.NoAuth {
		if c.Tokens == nil {
			return nil, ErrNotLoggedIn
		}
		tok, err := c.Tokens.Token(ctx)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	start := time.Now()
	res, err := c.HTTP.Do(req)
	if err != nil {
		c.debugf("%s %s → erreur : %v", r.Method, u, err)
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, &NetworkError{URL: c.BaseURL, Err: err}
	}
	defer res.Body.Close()
	b, err := io.ReadAll(io.LimitReader(res.Body, 64<<20))
	if err != nil {
		return nil, &NetworkError{URL: c.BaseURL, Err: err}
	}
	c.debugf("%s %s → %d (%s, %d o)", r.Method, u, res.StatusCode, time.Since(start).Round(time.Millisecond), len(b))
	return &Response{Status: res.StatusCode, Header: res.Header, Body: b}, nil
}

func (c *Client) debugf(format string, args ...any) {
	if c.Debug != nil {
		fmt.Fprintf(c.Debug, "[debug] "+format+"\n", args...)
	}
}

func (c *Client) wait(d time.Duration) {
	if c.sleep != nil {
		c.sleep(d)
		return
	}
	time.Sleep(d)
}

func backoff(attempt int) time.Duration {
	base := time.Duration(500*(1<<attempt)) * time.Millisecond
	return base + time.Duration(rand.Int63n(int64(250*time.Millisecond))) //nolint:gosec // gigue, pas de la cryptographie
}

func retryableNetErr(ctx context.Context, err error) bool {
	if ctx.Err() != nil {
		return false
	}
	var ne *NetworkError
	if !errors.As(err, &ne) {
		return false
	}
	var opErr *net.OpError
	var dnsErr *net.DNSError
	if errors.As(ne.Err, &dnsErr) && dnsErr.IsNotFound {
		return false
	}
	return errors.As(ne.Err, &opErr) || errors.Is(ne.Err, io.ErrUnexpectedEOF) || isTimeout(ne.Err)
}

func isTimeout(err error) bool {
	var t interface{ Timeout() bool }
	return errors.As(err, &t) && t.Timeout()
}

// NetworkError : API injoignable.
type NetworkError struct {
	URL string
	Err error
}

func (e *NetworkError) Error() string {
	return fmt.Sprintf("API YorkHost injoignable (%s) : %v", e.URL, e.Err)
}
func (e *NetworkError) Unwrap() error { return e.Err }

// Error : réponse d'erreur de l'API.
type Error struct {
	Status        int
	Message       string
	Code          string
	RequiredScope string
	RetryAfter    time.Duration
	Raw           map[string]any
}

func (e *Error) Error() string {
	msg := e.Message
	if msg == "" {
		msg = http.StatusText(e.Status)
	}
	s := fmt.Sprintf("%s (HTTP %d", msg, e.Status)
	if e.Code != "" {
		s += ", " + e.Code
	}
	s += ")"
	if h := e.Hint(); h != "" {
		s += "\n  → " + h
	}
	return s
}

// Hint : conseil d'action pour les erreurs connues.
func (e *Error) Hint() string {
	switch e.Code {
	case "missing_token", "invalid_token":
		return "reconnectez-vous : yorkhost login (clé révoquée, expirée ou mot de passe du compte changé)"
	case "mfa_required":
		return "activez la double authentification sur votre compte : https://clients.yorkhost.fr/mfa/"
	case "missing_scope":
		if e.RequiredScope != "" {
			return fmt.Sprintf("cette clé n'a pas la permission %q : ajoutez-la dans l'espace client (Clés API) ou relancez yorkhost login", e.RequiredScope)
		}
	case "ip_not_allowed":
		return "votre IP n'est pas dans l'allowlist de la clé (espace client → Clés API)"
	case "rate_limited", "busy":
		if e.RetryAfter > 0 {
			return fmt.Sprintf("réessayez dans %s", e.RetryAfter)
		}
		return "réessayez dans un instant"
	case "spend_cap":
		return "plafond de dépense journalier atteint : commandez depuis l'espace client"
	case "quote_invalid", "already_ordered":
		return "relancez la commande pour obtenir un nouveau devis"
	case "route_unavailable":
		return "route non ouverte aux clés API"
	case "ip_not_owned":
		return "choisissez une IP éligible (VPS ou serveur dédié) : yorkhost antiddos rules list"
	case "beta_full":
		return "règles anti-DDoS en bêta : toutes les places de test sont attribuées, réessayez plus tard"
	case "policy_unknown", "policy_unavailable":
		return "politiques disponibles : yorkhost antiddos rules list"
	case "extended_required":
		return "réservé aux comptes étendus : demandez-le au support (yorkhost ticket create)"
	case "rule_not_found":
		return "identifiants des règles : yorkhost antiddos rules list"
	case "rules_disabled":
		return "règles anti-DDoS non disponibles pour le moment"
	case "netops_unavailable":
		return "service des règles anti-DDoS indisponible : réessayez plus tard"
	}
	if e.Status == http.StatusNotFound && e.Message == "not found" && e.Code == "" {
		return "API client fermée sur ce serveur, ou URL d'API erronée (yorkhost config get api_url)"
	}
	return ""
}

func parseError(r *Response) *Error {
	e := &Error{Status: r.Status}
	if ra := r.Header.Get("Retry-After"); ra != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(ra)); err == nil && n >= 0 {
			e.RetryAfter = time.Duration(n) * time.Second
		}
	}
	var m map[string]any
	if json.Unmarshal(r.Body, &m) == nil {
		e.Raw = m
		e.Message = str(m["error"])
		if d := str(m["error_description"]); d != "" {
			// Erreurs OAuth (RFC 6749) : error = code, error_description = texte.
			e.Code, e.Message = e.Message, d
		}
		if c := str(m["code"]); c != "" {
			e.Code = c
		}
		if e.Message == "" {
			e.Message = str(m["message"])
		}
		e.RequiredScope = str(m["required_scope"])
	} else if t := strings.TrimSpace(string(r.Body)); t != "" && len(t) < 300 && !strings.HasPrefix(t, "<") {
		e.Message = t
	}
	return e
}

func str(v any) string {
	s, _ := v.(string)
	return s
}

// IsStatus : err est une erreur API de ce statut.
func IsStatus(err error, status int) bool {
	var e *Error
	return errors.As(err, &e) && e.Status == status
}
