package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/YorkHost-fr/yorkhost-cli/internal/output"
)

var idRe = regexp.MustCompile(`^[1-9][0-9]{0,9}$`)

// resolveService : ID numérique, ou nom de service (exact, puis préfixe
// unique, insensible à la casse) restreint au type donné ("" = tous).
// Sans argument : réglage par défaut du profil (default_vps, default_game).
func (a *App) resolveService(c context.Context, args []string, kind string) (string, error) {
	ref := ""
	if len(args) > 0 {
		ref = strings.TrimSpace(args[0])
	}
	if ref == "" {
		if p := a.File.Profile(a.Profile); p != nil {
			switch kind {
			case "vps":
				ref = p.DefaultVPS
			case "game":
				ref = p.DefaultGame
			}
		}
	}
	if ref == "" {
		return "", usageErr(errors.New("ID ou nom du service requis (voir « yorkhost service list »)"))
	}
	if idRe.MatchString(ref) {
		return ref, nil
	}
	items, _, err := a.Client().ListAll(c, "/client/v1/services", nil, 0)
	if err != nil {
		return "", err
	}
	var exact, prefix []map[string]any
	low := strings.ToLower(ref)
	for _, it := range items {
		if kind != "" && output.Text(it["type"]) != kind {
			continue
		}
		name := strings.ToLower(output.Text(it["name"]))
		switch {
		case name == low:
			exact = append(exact, it)
		case strings.HasPrefix(name, low):
			prefix = append(prefix, it)
		}
	}
	pick := exact
	if len(pick) == 0 {
		pick = prefix
	}
	switch len(pick) {
	case 1:
		return output.Text(pick[0]["id"]), nil
	case 0:
		what := "service"
		if kind != "" {
			what = kind
		}
		return "", &exitError{ExitNotFound, fmt.Errorf("aucun %s nommé %q", what, ref)}
	}
	names := make([]string, len(pick))
	for i, it := range pick {
		names[i] = fmt.Sprintf("%s (%s)", output.Text(it["id"]), output.Text(it["name"]))
	}
	return "", usageErr(fmt.Errorf("%q est ambigu : %s — précisez l'ID", ref, strings.Join(names, ", ")))
}

// serviceCompletion : complétion des IDs de service d'un type.
func (a *App) serviceCompletion(kind string) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		if a.File == nil && a.init() != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		items, _, err := a.Client().ListAll(c, "/client/v1/services", nil, 200)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		var out []string
		for _, it := range items {
			if kind == "" || output.Text(it["type"]) == kind {
				out = append(out, output.Text(it["id"])+"\t"+output.Text(it["name"]))
			}
		}
		return out, cobra.ShellCompDirectiveNoFileComp
	}
}

// path : chemin d'API avec segments échappés.
func path(parts ...string) string {
	var b strings.Builder
	b.WriteString("/client/v1")
	for _, p := range parts {
		b.WriteByte('/')
		b.WriteString(url.PathEscape(p))
	}
	return b.String()
}

// readText : valeur directe, fichier (@chemin) ou stdin (-).
func (a *App) readText(value, label string) (string, error) {
	switch {
	case value == "-":
		b, err := io.ReadAll(io.LimitReader(a.Stdin, 4<<20))
		if err != nil {
			return "", err
		}
		return strings.TrimRight(string(b), "\n"), nil
	case strings.HasPrefix(value, "@"):
		b, err := os.ReadFile(value[1:])
		if err != nil {
			return "", fmt.Errorf("%s : %w", label, err)
		}
		return strings.TrimRight(string(b), "\n"), nil
	}
	return value, nil
}

// readMessage : corps de message (option, fichier, stdin, ou saisie
// interactive terminée par Ctrl-D).
func (a *App) readMessage(value, label string) (string, error) {
	if value != "" {
		return a.readText(value, label)
	}
	if !a.Prompt.Interactive {
		return a.readText("-", label)
	}
	fmt.Fprintf(a.Stderr, "%s (terminez par une ligne vide puis Ctrl-D, ou Ctrl-C pour annuler) :\n", label)
	b, err := io.ReadAll(io.LimitReader(a.Stdin, 4<<20))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

var errWaitTimeout = errors.New("délai d'attente dépassé")

// waitFor : interroge check jusqu'à ce qu'il renvoie true (ou timeout).
func (a *App) waitFor(c context.Context, what string, timeout time.Duration, check func() (bool, string, error)) error {
	deadline := time.Now().Add(timeout)
	last := ""
	for {
		done, state, err := check()
		if err != nil {
			return err
		}
		if done {
			return nil
		}
		if state != last && state != "" {
			a.Printer.Info("  %s : %s…", what, state)
			last = state
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("%w (%s) : état actuel %q", errWaitTimeout, timeout, state)
		}
		select {
		case <-c.Done():
			return c.Err()
		case <-time.After(3 * time.Second):
		}
	}
}

// openBrowser : ouvre une URL dans le navigateur par défaut.
func openBrowser(u string) error {
	if os.Getenv("YORKHOST_NO_BROWSER") != "" {
		return errors.New("navigateur désactivé")
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", u)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", u)
	default:
		if os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
			return errors.New("pas d'environnement graphique")
		}
		cmd = exec.Command("xdg-open", u)
	}
	cmd.Stdout, cmd.Stderr = nil, nil
	return cmd.Start()
}

// str / num : accès typés aux réponses JSON génériques.
func str(m map[string]any, k string) string {
	switch v := m[k].(type) {
	case string:
		return v
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	}
	return ""
}

func num(m map[string]any, k string) float64 {
	f, _ := m[k].(float64)
	return f
}

func list(m map[string]any, k string) []map[string]any {
	raw, _ := m[k].([]any)
	out := make([]map[string]any, 0, len(raw))
	for _, x := range raw {
		if mm, ok := x.(map[string]any); ok {
			out = append(out, mm)
		}
	}
	return out
}

// strs : tableau JSON de chaînes.
func strs(v any) []string {
	raw, _ := v.([]any)
	out := make([]string, 0, len(raw))
	for _, x := range raw {
		out = append(out, output.Text(x))
	}
	return out
}

// statusCol : colonne « Statut » colorée (champ status).
func statusCol() output.Column {
	return output.Column{Header: "Statut", Path: "status", Color: output.Status}
}

// noArgsOrOne : 0 ou 1 argument (ID facultatif grâce au défaut du profil).
var noArgsOrOne = cobra.MaximumNArgs(1)
