package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func newTestClient(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	c := New(srv.URL, StaticToken("tok"), "test-agent")
	c.sleep = func(time.Duration) {}
	return c
}

func TestBearerAndUserAgent(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer tok" {
			t.Errorf("Authorization = %q", got)
		}
		if got := r.Header.Get("User-Agent"); got != "test-agent" {
			t.Errorf("User-Agent = %q", got)
		}
		fmt.Fprint(w, `{"ok":true}`)
	})
	var out map[string]any
	if err := c.Get(context.Background(), "/x", nil, &out); err != nil {
		t.Fatal(err)
	}
	if out["ok"] != true {
		t.Fatalf("out = %v", out)
	}
}

func TestErrorParsingAndHints(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, `{"error":"this API key lacks the required permission","code":"missing_scope","required_scope":"vps:power"}`)
	})
	err := c.Get(context.Background(), "/x", nil, nil)
	var ae *Error
	if !errors.As(err, &ae) {
		t.Fatalf("err = %T %v", err, err)
	}
	if ae.Code != "missing_scope" || ae.RequiredScope != "vps:power" || ae.Status != 403 {
		t.Fatalf("ae = %+v", ae)
	}
	if !strings.Contains(err.Error(), `"vps:power"`) {
		t.Fatalf("hint absent : %s", err)
	}
}

func TestBetaFullHint(t *testing.T) {
	e := parseError(&Response{Status: 409, Header: http.Header{}, Body: []byte(`{"error":"Les règles de filtrage personnalisées sont en bêta : toutes les places de test sont déjà attribuées. Réessayez plus tard.","code":"beta_full"}`)})
	if e.Code != "beta_full" || !strings.Contains(e.Error(), "places de test") || !strings.Contains(e.Error(), "réessayez plus tard") {
		t.Fatalf("beta_full : %s", e.Error())
	}
}

func TestOAuthStyleError(t *testing.T) {
	e := parseError(&Response{Status: 400, Header: http.Header{}, Body: []byte(`{"error":"invalid_grant","error_description":"refresh token expired"}`)})
	if e.Code != "invalid_grant" || e.Message != "refresh token expired" {
		t.Fatalf("e = %+v", e)
	}
}

func TestRetry429ThenOK(t *testing.T) {
	var n int32
	var slept []time.Duration
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&n, 1) == 1 {
			w.Header().Set("Retry-After", "2")
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprint(w, `{"error":"too many","code":"rate_limited"}`)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	c.sleep = func(d time.Duration) { slept = append(slept, d) }
	if _, err := c.Post(context.Background(), "/power", map[string]string{"action": "restart"}, nil); err != nil {
		t.Fatal(err)
	}
	if n != 2 || len(slept) != 1 || slept[0] != 2*time.Second {
		t.Fatalf("n=%d slept=%v", n, slept)
	}
}

func TestNoRetryOnPost5xx(t *testing.T) {
	var n int32
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&n, 1)
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	if _, err := c.Post(context.Background(), "/orders", map[string]any{}, nil); !IsStatus(err, 503) {
		t.Fatalf("err = %v", err)
	}
	if n != 1 {
		t.Fatalf("POST relancé %d fois", n)
	}
}

func TestRetryOnGet5xx(t *testing.T) {
	var n int32
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&n, 1) < 3 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		fmt.Fprint(w, `{}`)
	})
	if err := c.Get(context.Background(), "/x", nil, nil); err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Fatalf("n = %d", n)
	}
}

type refreshing struct {
	tok       string
	refreshed int
}

func (r *refreshing) Token(context.Context) (string, error) { return r.tok, nil }
func (r *refreshing) Refresh(context.Context) (bool, error) {
	r.refreshed++
	r.tok = "new"
	return true, nil
}

func TestRefreshOn401(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer new" {
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, `{"error":"invalid","code":"invalid_token"}`)
			return
		}
		fmt.Fprint(w, `{}`)
	})
	ts := &refreshing{tok: "old"}
	c.Tokens = ts
	if err := c.Get(context.Background(), "/x", nil, nil); err != nil {
		t.Fatal(err)
	}
	if ts.refreshed != 1 {
		t.Fatalf("refreshed = %d", ts.refreshed)
	}
}

func TestListAll(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("page")
		if l := r.URL.Query().Get("limit"); l != "100" && l != "3" {
			t.Errorf("limit = %s", r.URL.Query().Get("limit"))
		}
		items := []map[string]any{{"id": page + "a"}, {"id": page + "b"}}
		json.NewEncoder(w).Encode(map[string]any{"items": items, "page": 1, "total": 6, "hasMore": page != "3"})
	})
	all, total, err := c.ListAll(context.Background(), "/services", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 6 || total != 6 || all[5]["id"] != "3b" {
		t.Fatalf("all = %v total = %d", all, total)
	}
	some, _, err := c.ListAll(context.Background(), "/services", nil, 3)
	if err != nil || len(some) != 3 {
		t.Fatalf("some = %v err = %v", some, err)
	}
}

func TestNotLoggedIn(t *testing.T) {
	c := New("http://127.0.0.1:1", StaticToken(""), "")
	if err := c.Get(context.Background(), "/x", nil, nil); !errors.Is(err, ErrNotLoggedIn) {
		t.Fatalf("err = %v", err)
	}
}
