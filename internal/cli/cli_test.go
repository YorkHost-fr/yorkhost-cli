package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/YorkHost-fr/yorkhost-cli/internal/config"
)

const testKey = "yhk_client_v1_AAAAAAAAAAAAAAAA_BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB"

type call struct {
	Method, Path string
	Body         map[string]any
	Query        string
}

// fakeAPI : routes /client/v1 avec les formats de yorkhost-api.
type fakeAPI struct {
	mu    sync.Mutex
	calls []call
	srv   *httptest.Server
}

func (f *fakeAPI) record(r *http.Request) map[string]any {
	var body map[string]any
	b, _ := io.ReadAll(r.Body)
	_ = json.Unmarshal(b, &body)
	f.mu.Lock()
	f.calls = append(f.calls, call{r.Method, r.URL.Path, body, r.URL.RawQuery})
	f.mu.Unlock()
	return body
}

func (f *fakeAPI) find(method, path string) []call {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []call
	for _, c := range f.calls {
		if c.Method == method && c.Path == path {
			out = append(out, c)
		}
	}
	return out
}

func newFakeAPI(t *testing.T) *fakeAPI {
	f := &fakeAPI{}
	js := func(w http.ResponseWriter, status int, v any) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(v)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		f.record(r)
		js(w, 404, map[string]any{"error": "not found"})
	})
	auth := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "Bearer "+testKey {
				f.record(r)
				js(w, 401, map[string]any{"error": "invalid, expired or revoked API key", "code": "invalid_token"})
				return
			}
			h(w, r)
		}
	}
	mux.HandleFunc("GET /client/v1/account", auth(func(w http.ResponseWriter, r *http.Request) {
		f.record(r)
		js(w, 200, map[string]any{"clientId": "77", "email": "ada@example.test", "firstName": "Ada", "lastName": "L", "credit": 12.5, "unpaidInvoices": 1})
	}))
	mux.HandleFunc("GET /client/v1/services", auth(func(w http.ResponseWriter, r *http.Request) {
		f.record(r)
		js(w, 200, map[string]any{"page": 1, "pageSize": 100, "total": 2, "hasMore": false, "items": []any{
			map[string]any{"id": "101", "name": "web-prod", "type": "vps", "status": "active", "productName": "VPS-2", "nextDueDate": "2026-11-01", "recurringAmount": 9.99, "billingCycle": "Monthly"},
			map[string]any{"id": "202", "name": "fivem-rp", "type": "game", "status": "suspended", "productName": "FiveM", "nextDueDate": nil},
		}})
	}))
	mux.HandleFunc("GET /client/v1/vps/{id}", auth(func(w http.ResponseWriter, r *http.Request) {
		f.record(r)
		if r.PathValue("id") != "101" {
			js(w, 404, map[string]any{"error": "service not found", "code": "not_found"})
			return
		}
		js(w, 200, map[string]any{"id": "101", "state": "running", "hostname": "web-prod", "ips": []any{"203.0.113.10"}, "ramMb": 1024.0, "ramMaxMb": 4096.0, "ramPct": 25.0, "username": "debian"})
	}))
	mux.HandleFunc("POST /client/v1/vps/{id}/power", auth(func(w http.ResponseWriter, r *http.Request) {
		f.record(r)
		w.WriteHeader(204)
	}))
	mux.HandleFunc("GET /client/v1/invoices/{id}", auth(func(w http.ResponseWriter, r *http.Request) {
		f.record(r)
		js(w, 404, map[string]any{"error": "invoice not found"})
	}))
	mux.HandleFunc("POST /client/v1/orders", auth(func(w http.ResponseWriter, r *http.Request) {
		body := f.record(r)
		if body["confirmation_token"] == nil {
			js(w, 200, map[string]any{"preflight": true, "product_id": body["product_id"], "cycle": "m", "cycle_label": "Monthly", "price": 9.99, "setup_fee": 0, "due_now": 9.99, "currency": "EUR", "confirmation_token": "qt_1", "expires_in": 600})
			return
		}
		if body["confirmation_token"] != "qt_1" {
			js(w, 409, map[string]any{"error": "quote expired", "code": "quote_invalid"})
			return
		}
		js(w, 201, map[string]any{"order_id": 5, "invoice_id": 6, "total": 9.99, "currency": "EUR", "invoice_status": "unpaid", "payment_url": "https://clients.example.test/pay/6"})
	}))
	mux.HandleFunc("POST /client/v1/sandboxes/{id}/exec", auth(func(w http.ResponseWriter, r *http.Request) {
		body := f.record(r)
		js(w, 200, map[string]any{"exit_code": 3, "stdout": "out:" + fmt.Sprint(body["command"]) + "\n", "stderr": "warn\n", "timed_out": false, "duration_ms": 12, "cwd": "/home/agent"})
	}))
	mux.HandleFunc("POST /client/v1/tickets", auth(func(w http.ResponseWriter, r *http.Request) {
		f.record(r)
		js(w, 200, map[string]any{"id": "t1", "number": "10234"})
	}))
	// Anti-DDoS : formats de yorkhost-api (internal/handlers/v1/client/antiddos.go).
	mux.HandleFunc("GET /client/v1/antiddos", auth(func(w http.ResponseWriter, r *http.Request) {
		f.record(r)
		js(w, 200, map[string]any{"attacks": []any{map[string]any{"id": "a1", "alert_type": "udp_flood", "start_time": "2026-10-30T10:00:00Z", "power_bps": 2.5e9, "power_pps": 1.2e6,
			"matches": []any{map[string]any{"type": "ipam", "ip": "203.0.113.10", "service_id": 101, "service_name": "web-prod"}}}},
			"ipam_ips": []any{map[string]any{"ip": "203.0.113.10", "service_id": 101, "service_name": "web-prod"}}, "game_servers": []any{},
			"protection_active": true, "protection_checked": true, "days": 90, "ip": r.URL.Query().Get("ip")})
	}))
	mux.HandleFunc("GET /client/v1/antiddos/rules", auth(func(w http.ResponseWriter, r *http.Request) {
		f.record(r)
		js(w, 200, map[string]any{"backend": "netops", "account_level": "standard",
			"policies":     []any{map[string]any{"name": "FiveM UDP", "protocols": []any{"udp"}}, map[string]any{"name": "Drop"}},
			"limits":       map[string]any{"max_rules_per_ip": 10},
			"eligible_ips": []any{"203.0.113.10"},
			"rules": []any{map[string]any{"id": "r_1", "ip": "203.0.113.10", "policy": "FiveM UDP", "proto_list": []any{"udp"}, "dport_list": []any{"30120"},
				"sport_list": []any{}, "sip_list": []any{}, "status": "active", "position": 1, "description": "jeu"}}})
	}))
	mux.HandleFunc("POST /client/v1/antiddos/rules", auth(func(w http.ResponseWriter, r *http.Request) {
		body := f.record(r)
		if body["ip"] != "203.0.113.10" {
			js(w, 403, map[string]any{"error": "Cette adresse n'est pas allouée à votre compte", "code": "ip_not_owned", "field": "ip"})
			return
		}
		js(w, 201, map[string]any{"backend": "netops", "rule": map[string]any{"id": "r_2", "ip": body["ip"], "policy": body["policy"], "status": "pending", "position": 1}})
	}))
	mux.HandleFunc("POST /client/v1/antiddos/rules/{id}/move", auth(func(w http.ResponseWriter, r *http.Request) {
		body := f.record(r)
		js(w, 200, map[string]any{"backend": "netops", "rule": map[string]any{"id": r.PathValue("id"), "position": body["position"], "status": "active"}})
	}))
	mux.HandleFunc("DELETE /client/v1/antiddos/rules/{id}", auth(func(w http.ResponseWriter, r *http.Request) {
		f.record(r)
		if r.PathValue("id") != "r_1" {
			js(w, 404, map[string]any{"error": "Unknown rule for this client", "code": "rule_not_found"})
			return
		}
		js(w, 200, map[string]any{"backend": "netops", "rule_id": r.PathValue("id")})
	}))
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

type result struct {
	stdout, stderr string
	code           int
}

// run : exécute le CLI contre le faux serveur, dans une configuration isolée.
func run(t *testing.T, f *fakeAPI, stdin string, args ...string) result {
	t.Helper()
	var out, errb bytes.Buffer
	no := false
	a := &App{Stdin: strings.NewReader(stdin), Stdout: &out, Stderr: &errb, Interactive: &no}
	code := Execute(context.Background(), a, args)
	return result{out.String(), errb.String(), code}
}

func setup(t *testing.T, withKey bool) *fakeAPI {
	f := newFakeAPI(t)
	t.Setenv("YORKHOST_CONFIG_FILE", filepath.Join(t.TempDir(), "config.yaml"))
	t.Setenv("YORKHOST_API_URL", f.srv.URL)
	t.Setenv("YORKHOST_PROFILE", "")
	t.Setenv("YORKHOST_OUTPUT", "")
	t.Setenv("YORKHOST_YES", "")
	t.Setenv("YORKHOST_TOKEN", "")
	t.Setenv("NO_COLOR", "1")
	if withKey {
		t.Setenv("YORKHOST_API_KEY", testKey)
	} else {
		t.Setenv("YORKHOST_API_KEY", "")
	}
	return f
}

func TestServiceListTable(t *testing.T) {
	f := setup(t, true)
	r := run(t, f, "", "service", "list")
	if r.code != 0 {
		t.Fatalf("code %d : %s", r.code, r.stderr)
	}
	for _, want := range []string{"ID", "NOM", "web-prod", "fivem-rp", "suspended"} {
		if !strings.Contains(r.stdout, want) {
			t.Errorf("%q absent de :\n%s", want, r.stdout)
		}
	}
	r = run(t, f, "", "service", "list", "--type", "game", "-o", "json")
	var arr []map[string]any
	if err := json.Unmarshal([]byte(r.stdout), &arr); err != nil || len(arr) != 1 || arr[0]["id"] != "202" {
		t.Fatalf("json = %s (%v)", r.stdout, err)
	}
}

func TestPowerNeedsConfirmation(t *testing.T) {
	f := setup(t, true)
	r := run(t, f, "", "vps", "restart", "web-prod")
	if r.code != ExitAborted || !strings.Contains(r.stderr, "--yes") {
		t.Fatalf("code %d, stderr %s", r.code, r.stderr)
	}
	if len(f.find("POST", "/client/v1/vps/101/power")) != 0 {
		t.Fatal("action envoyée sans confirmation")
	}
	r = run(t, f, "", "vps", "restart", "web-prod", "--yes")
	if r.code != 0 {
		t.Fatalf("code %d : %s", r.code, r.stderr)
	}
	calls := f.find("POST", "/client/v1/vps/101/power")
	if len(calls) != 1 || calls[0].Body["action"] != "restart" {
		t.Fatalf("calls = %+v", calls)
	}
	// start n'est pas destructif : pas de confirmation.
	if r := run(t, f, "", "vps", "start", "101"); r.code != 0 {
		t.Fatalf("start : code %d %s", r.code, r.stderr)
	}
}

func TestVPSGet(t *testing.T) {
	f := setup(t, true)
	r := run(t, f, "", "vps", "get", "101")
	if r.code != 0 || !strings.Contains(r.stdout, "203.0.113.10") || !strings.Contains(r.stdout, "1.0 Go / 4.0 Go (25.0 %)") {
		t.Fatalf("code %d\n%s%s", r.code, r.stdout, r.stderr)
	}
	r = run(t, f, "", "vps", "get", "999")
	if r.code != ExitNotFound {
		t.Fatalf("404 : code %d", r.code)
	}
	r = run(t, f, "", "vps", "get", "nope")
	if r.code != ExitNotFound {
		t.Fatalf("nom inconnu : code %d %s", r.code, r.stderr)
	}
}

func TestExitCodes(t *testing.T) {
	f := setup(t, true)
	if r := run(t, f, "", "invoice", "get", "9"); r.code != ExitNotFound || !strings.Contains(r.stderr, "invoice not found") {
		t.Fatalf("invoice 404 : %d %s", r.code, r.stderr)
	}
	if r := run(t, f, "", "nope"); r.code != ExitUsage {
		t.Fatalf("commande inconnue : %d", r.code)
	}
	t.Setenv("YORKHOST_API_KEY", "yhk_client_v1_ZZZZZZZZZZZZZZZZ_ZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZ")
	if r := run(t, f, "", "account"); r.code != ExitAuth || !strings.Contains(r.stderr, "yorkhost login") {
		t.Fatalf("401 : %d %s", r.code, r.stderr)
	}
}

func TestOrderFlow(t *testing.T) {
	f := setup(t, true)
	r := run(t, f, "", "order", "create", "42")
	if r.code != ExitAborted {
		t.Fatalf("sans --yes : code %d", r.code)
	}
	r = run(t, f, "", "order", "create", "42", "--cycle", "m", "--yes")
	if r.code != 0 {
		t.Fatalf("code %d : %s", r.code, r.stderr)
	}
	calls := f.find("POST", "/client/v1/orders")
	last := calls[len(calls)-1]
	if last.Body["confirmation_token"] != "qt_1" || last.Body["product_id"] != 42.0 {
		t.Fatalf("confirmation = %+v", last)
	}
	if !strings.Contains(r.stdout, "https://clients.example.test/pay/6") || !strings.Contains(r.stderr, "9,99 €") {
		t.Fatalf("stdout %s\nstderr %s", r.stdout, r.stderr)
	}
}

func TestSandboxExecExitCode(t *testing.T) {
	f := setup(t, true)
	r := run(t, f, "", "sandbox", "exec", "sbxabcdefghijkl", "--", "ls", "-la")
	if r.code != 3 || r.stdout != "out:ls -la\n" || !strings.Contains(r.stderr, "warn") {
		t.Fatalf("code %d stdout %q stderr %q", r.code, r.stdout, r.stderr)
	}
	if r := run(t, f, "", "sandbox", "exec", "pas-un-id", "--", "ls"); r.code != ExitUsage {
		t.Fatalf("ID invalide : code %d", r.code)
	}
}

func TestTicketFromStdin(t *testing.T) {
	f := setup(t, true)
	r := run(t, f, "ligne 1\nligne 2\n", "ticket", "create", "--subject", "Panne", "--body", "-", "--service", "fivem-rp", "--type", "help")
	if r.code != 0 {
		t.Fatalf("code %d : %s", r.code, r.stderr)
	}
	c := f.find("POST", "/client/v1/tickets")
	if len(c) != 1 || c[0].Body["body"] != "ligne 1\nligne 2" || c[0].Body["serviceId"] != "202" || c[0].Body["requestType"] != "HELP" {
		t.Fatalf("calls = %+v", c)
	}
}

func TestLoginWithToken(t *testing.T) {
	f := setup(t, false)
	if r := run(t, f, "pas-une-cle\n", "login", "--with-token"); r.code != ExitUsage {
		t.Fatalf("clé invalide : code %d", r.code)
	}
	r := run(t, f, testKey+"\n", "login", "--with-token", "--profile", "ci")
	if r.code != 0 || !strings.Contains(r.stdout, "Ada L") {
		t.Fatalf("code %d\n%s%s", r.code, r.stdout, r.stderr)
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if p := cfg.Profile("ci"); p == nil || p.Auth.APIKey != testKey || cfg.Active() != "ci" {
		t.Fatalf("config = %+v", cfg)
	}
	r = run(t, f, "", "whoami")
	if r.code != 0 || !strings.Contains(r.stdout, "ada@example.test") || strings.Contains(r.stdout, "BBBBBBBB") {
		t.Fatalf("whoami : %d\n%s%s", r.code, r.stdout, r.stderr)
	}
	r = run(t, f, "", "logout")
	if r.code != 0 {
		t.Fatalf("logout : %d %s", r.code, r.stderr)
	}
	if r := run(t, f, "", "account"); r.code != ExitAuth {
		t.Fatalf("après logout : %d", r.code)
	}
}

func TestRawAPI(t *testing.T) {
	f := setup(t, true)
	r := run(t, f, "", "api", "/services", "--paginate")
	var arr []any
	if r.code != 0 || json.Unmarshal([]byte(r.stdout), &arr) != nil || len(arr) != 2 {
		t.Fatalf("code %d : %s %s", r.code, r.stdout, r.stderr)
	}
	r = run(t, f, "", "api", "-X", "POST", "vps/101/power", "-F", "action=stop")
	if r.code != 0 {
		t.Fatalf("code %d : %s", r.code, r.stderr)
	}
	c := f.find("POST", "/client/v1/vps/101/power")
	if len(c) != 1 || c[0].Body["action"] != "stop" {
		t.Fatalf("calls = %+v", c)
	}
}

func TestConfigCommands(t *testing.T) {
	f := setup(t, true)
	if r := run(t, f, "", "config", "set", "output", "yaml"); r.code != 0 {
		t.Fatalf("set : %d %s", r.code, r.stderr)
	}
	if r := run(t, f, "", "config", "get", "output"); r.stdout != "yaml\n" {
		t.Fatalf("get = %q", r.stdout)
	}
	// Le profil impose yaml ; -o prime.
	if r := run(t, f, "", "account"); !strings.Contains(r.stdout, "email: ada@example.test") {
		t.Fatalf("yaml attendu : %s", r.stdout)
	}
	if r := run(t, f, "", "config", "set", "output", "xml"); r.code != ExitUsage {
		t.Fatalf("valeur invalide : %d", r.code)
	}
	if r := run(t, f, "", "config", "set", "default_vps", "101"); r.code != 0 {
		t.Fatal(r.stderr)
	}
	if r := run(t, f, "", "vps", "get", "-o", "json"); r.code != 0 || !strings.Contains(r.stdout, `"hostname": "web-prod"`) {
		t.Fatalf("default_vps : %d %s %s", r.code, r.stdout, r.stderr)
	}
}

func TestTypedFields(t *testing.T) {
	for in, want := range map[string]any{"42": 42.0, "-1.5": -1.5, "true": true, "0612": "0612", "abc": "abc", "null": nil} {
		if got := typed(in); got != want {
			t.Errorf("typed(%q) = %#v, want %#v", in, got, want)
		}
	}
}

func TestSparkline(t *testing.T) {
	if s := Sparkline([]float64{0, 1, 2, 3}); s != "▁▃▅█" {
		t.Fatalf("sparkline = %q", s)
	}
	if s := Sparkline(nil); s != "-" {
		t.Fatalf("vide = %q", s)
	}
}

func TestAntiDDoSRules(t *testing.T) {
	f := setup(t, true)

	// Liste : table des règles sur stdout, politiques et IP éligibles sur stderr.
	r := run(t, f, "", "antiddos", "rules", "list", "--ip", "203.0.113.10")
	if r.code != 0 {
		t.Fatalf("list : code %d %s", r.code, r.stderr)
	}
	for _, want := range []string{"r_1", "FiveM UDP", "30120", "active"} {
		if !strings.Contains(r.stdout, want) {
			t.Errorf("%q absent de :\n%s", want, r.stdout)
		}
	}
	if !strings.Contains(r.stderr, "FiveM UDP, Drop") || !strings.Contains(r.stderr, "203.0.113.10") {
		t.Errorf("politiques / IP éligibles absentes : %s", r.stderr)
	}
	if c := f.find("GET", "/client/v1/antiddos/rules"); len(c) != 1 || c[0].Query != "ip=203.0.113.10" {
		t.Errorf("liste : %+v", c)
	}
	// -o json : réponse de l'API telle quelle.
	r = run(t, f, "", "antiddos", "rules", "list", "-o", "json")
	var st map[string]any
	if err := json.Unmarshal([]byte(r.stdout), &st); err != nil || st["account_level"] != "standard" || st["eligible_ips"] == nil {
		t.Fatalf("json = %s (%v)", r.stdout, err)
	}

	// Création : listes répétables ou séparées par des virgules.
	r = run(t, f, "", "antiddos", "rules", "create", "--ip", "203.0.113.10", "--policy", "FiveM UDP",
		"--proto", "udp", "--dport", "30120,30121", "--dport", "40000-40010", "--description", "jeu", "--position", "1")
	if r.code != 0 || !strings.Contains(r.stdout, "r_2") {
		t.Fatalf("create : code %d %s", r.code, r.stderr)
	}
	c := f.find("POST", "/client/v1/antiddos/rules")
	if len(c) != 1 {
		t.Fatalf("create : %+v", c)
	}
	b, _ := json.Marshal(c[0].Body)
	if string(b) != `{"description":"jeu","dport_list":["30120","30121","40000-40010"],"ip":"203.0.113.10","policy":"FiveM UDP","position":1,"proto_list":["udp"]}` {
		t.Errorf("corps envoyé : %s", b)
	}
	// Champs requis : erreur d'usage, rien n'est envoyé.
	if r := run(t, f, "", "antiddos", "rules", "create", "--policy", "Drop"); r.code != ExitUsage {
		t.Errorf("sans --ip : code %d", r.code)
	}
	if r := run(t, f, "", "antiddos", "rules", "create", "--ip", "203.0.113.10", "--policy", "Drop", "--position", "0"); r.code != ExitUsage {
		t.Errorf("--position 0 : code %d", r.code)
	}
	if n := len(f.find("POST", "/client/v1/antiddos/rules")); n != 1 {
		t.Errorf("%d créations envoyées", n)
	}
	// Refus de l'API : message, code et conseil.
	r = run(t, f, "", "antiddos", "rules", "create", "--ip", "198.51.100.1", "--policy", "Drop")
	if r.code == 0 || !strings.Contains(r.stderr, "ip_not_owned") || !strings.Contains(r.stderr, "rules list") {
		t.Errorf("refus : code %d %s", r.code, r.stderr)
	}

	// Déplacement.
	if r := run(t, f, "", "antiddos", "rules", "move", "r_1"); r.code != ExitUsage {
		t.Errorf("move sans --position : code %d", r.code)
	}
	r = run(t, f, "", "antiddos", "rules", "move", "r_1", "--position", "2")
	if mv := f.find("POST", "/client/v1/antiddos/rules/r_1/move"); r.code != 0 || len(mv) != 1 || mv[0].Body["position"] != 2.0 {
		t.Errorf("move : code %d %s %+v", r.code, r.stderr, mv)
	}

	// Suppression : confirmation obligatoire (non interactif sans --yes).
	r = run(t, f, "", "antiddos", "rules", "delete", "r_1")
	if r.code != ExitAborted || !strings.Contains(r.stderr, "--yes") {
		t.Fatalf("delete sans --yes : code %d %s", r.code, r.stderr)
	}
	if len(f.find("DELETE", "/client/v1/antiddos/rules/r_1")) != 0 {
		t.Fatal("suppression envoyée sans confirmation")
	}
	r = run(t, f, "", "antiddos", "rules", "delete", "r_1", "--yes", "-o", "json")
	if r.code != 0 || !strings.Contains(r.stdout, `"rule_id": "r_1"`) && !strings.Contains(r.stdout, `"rule_id":"r_1"`) {
		t.Fatalf("delete --yes : code %d %q %s", r.code, r.stdout, r.stderr)
	}
	if r := run(t, f, "", "antiddos", "rules", "delete", "r_9", "--yes"); r.code == 0 || !strings.Contains(r.stderr, "rule_not_found") {
		t.Errorf("règle inconnue : code %d %s", r.code, r.stderr)
	}
}

func TestAntiDDoSAttacksByIP(t *testing.T) {
	f := setup(t, true)
	r := run(t, f, "", "antiddos", "attacks", "--ip", "203.0.113.10", "--days", "7")
	if r.code != 0 || !strings.Contains(r.stdout, "203.0.113.10 (web-prod)") {
		t.Fatalf("attacks : code %d %s %s", r.code, r.stdout, r.stderr)
	}
	c := f.find("GET", "/client/v1/antiddos")
	if len(c) != 1 || c[0].Query != "days=7&ip=203.0.113.10" {
		t.Errorf("requête : %+v", c)
	}
}
