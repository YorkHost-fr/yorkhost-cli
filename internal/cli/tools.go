package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"

	"github.com/YorkHost-fr/yorkhost-cli/internal/api"
	"github.com/YorkHost-fr/yorkhost-cli/internal/config"
	"github.com/YorkHost-fr/yorkhost-cli/internal/output"
	"github.com/YorkHost-fr/yorkhost-cli/internal/version"
)

// ------------------------------------------------------------ config

func newConfigCmd(a *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Réglages et profils (plusieurs comptes, comme aws --profile)",
		Long: `Chaque profil a sa propre URL d'API, ses identifiants et ses réglages.
Le profil actif est utilisé par défaut ; --profile ou YORKHOST_PROFILE en choisissent un autre.

Réglages :
` + configKeysHelp(),
		Example: `  yorkhost config set output json
  yorkhost config set default_vps 1234
  yorkhost --profile pro login
  yorkhost config profile use pro`,
	}
	keyComp := func(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		var out []string
		for _, k := range config.Keys {
			out = append(out, k.Name+"\t"+k.Help)
		}
		return out, cobra.ShellCompDirectiveNoFileComp
	}
	get := &cobra.Command{
		Use: "get <réglage>", Short: "Lire un réglage du profil", Args: cobra.ExactArgs(1), ValidArgsFunction: keyComp,
		RunE: func(cmd *cobra.Command, args []string) error {
			p := a.File.Profile(a.Profile)
			if p == nil {
				p = &config.Profile{}
			}
			v, err := p.Get(args[0])
			if err != nil {
				return usageErr(err)
			}
			if v == "" && args[0] == "api_url" {
				v = config.DefaultAPIURL
			}
			fmt.Fprintln(a.Stdout, v)
			return nil
		},
	}
	set := &cobra.Command{
		Use: "set <réglage> <valeur>", Short: "Modifier un réglage du profil", Args: cobra.ExactArgs(2), ValidArgsFunction: keyComp,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := a.File.Ensure(a.Profile).Set(args[0], args[1]); err != nil {
				return usageErr(err)
			}
			if err := a.File.Save(); err != nil {
				return err
			}
			a.Printer.Success("%s = %s (profil %q)", args[0], args[1], a.Profile)
			return nil
		},
	}
	unset := &cobra.Command{
		Use: "unset <réglage>", Short: "Revenir à la valeur par défaut", Args: cobra.ExactArgs(1), ValidArgsFunction: keyComp,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := a.File.Ensure(a.Profile).Set(args[0], ""); err != nil {
				return usageErr(err)
			}
			if err := a.File.Save(); err != nil {
				return err
			}
			a.Printer.Success("%s réinitialisé (profil %q)", args[0], a.Profile)
			return nil
		},
	}
	ls := &cobra.Command{
		Use: "list", Aliases: []string{"ls", "view"}, Short: "Réglages du profil courant", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			p := a.File.Profile(a.Profile)
			if p == nil {
				p = &config.Profile{}
			}
			obj := map[string]any{"profile": a.Profile, "config_file": a.File.FilePath()}
			for _, k := range config.Keys {
				v, _ := p.Get(k.Name)
				obj[k.Name] = v
			}
			if obj["api_url"] == "" {
				obj["api_url"] = config.DefaultAPIURL + " (défaut)"
			}
			fields := []output.Field{{Header: "profile", Path: "profile"}, {Header: "config_file", Path: "config_file"}}
			for _, k := range config.Keys {
				fields = append(fields, output.Field{Header: k.Name, Path: k.Name})
			}
			return a.Printer.Object(obj, obj, fields)
		},
	}
	pathCmd := &cobra.Command{
		Use: "path", Short: "Chemin du fichier de configuration", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			fmt.Fprintln(a.Stdout, a.File.FilePath())
			return nil
		},
	}
	cmd.AddCommand(ls, get, set, unset, newProfileCmd(a), pathCmd)
	return cmd
}

func configKeysHelp() string {
	var b strings.Builder
	for _, k := range config.Keys {
		fmt.Fprintf(&b, "  %-15s %s\n", k.Name, k.Help)
	}
	return b.String()
}

func newProfileCmd(a *App) *cobra.Command {
	cmd := &cobra.Command{Use: "profile", Short: "Gérer les profils"}
	ls := &cobra.Command{
		Use: "list", Aliases: []string{"ls"}, Short: "Lister les profils", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			var rows []map[string]any
			for _, n := range a.File.ProfileNames() {
				p := a.File.Profile(n)
				u := p.APIURL
				if u == "" {
					u = config.DefaultAPIURL
				}
				rows = append(rows, map[string]any{"name": n, "active": n == a.File.Active(), "api_url": u, "logged_in": !p.Auth.Empty()})
			}
			return a.Printer.List(rows, rows, []output.Column{
				{Header: "Profil", Path: "name"},
				{Header: "Actif", Path: "active"},
				{Header: "Connecté", Path: "logged_in", Color: output.Status},
				{Header: "API", Path: "api_url"},
			})
		},
	}
	use := &cobra.Command{
		Use: "use <nom>", Short: "Choisir le profil actif (créé s'il n'existe pas)", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !config.ValidProfileName(args[0]) {
				return usageErr(fmt.Errorf("nom de profil invalide %q", args[0]))
			}
			a.File.Ensure(args[0])
			a.File.ActiveProfile = args[0]
			if err := a.File.Save(); err != nil {
				return err
			}
			a.Printer.Success("Profil actif : %s", args[0])
			return nil
		},
	}
	del := &cobra.Command{
		Use: "delete <nom>", Aliases: []string{"rm"}, Short: "Supprimer un profil (et ses identifiants)", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p := a.File.Profile(args[0])
			if p == nil {
				return &exitError{ExitNotFound, fmt.Errorf("profil %q inexistant", args[0])}
			}
			if err := a.Prompt.Confirm(fmt.Sprintf("Supprimer le profil %q ?", args[0]), a.Yes); err != nil {
				return err
			}
			a.revokeOAuth(ctx(cmd), p.Auth)
			delete(a.File.Profiles, args[0])
			if a.File.ActiveProfile == args[0] {
				a.File.ActiveProfile = ""
			}
			if err := a.File.Save(); err != nil {
				return err
			}
			a.Printer.Success("Profil %q supprimé.", args[0])
			return nil
		},
	}
	cmd.AddCommand(ls, use, del)
	return cmd
}

// ------------------------------------------------------------ api

func newAPICmd(a *App) *cobra.Command {
	var method, data string
	var fields []string
	var paginate, include bool
	cmd := &cobra.Command{
		Use:   "api <chemin>",
		Short: "Requête brute sur l'API client (comme gh api)",
		Long: `Envoie une requête authentifiée à l'API client et affiche la réponse JSON.
Le chemin est relatif à /client/v1 s'il ne commence pas par /client/.

Attention : cette commande brute n'applique pas les confirmations de sécurité
des commandes dédiées. Une requête POST, PUT ou DELETE est envoyée directement ;
vérifiez la méthode, le chemin et les données avant de l'exécuter.

Spécification : https://github.com/YorkHost-fr/yorkhost-cli/blob/main/docs/api.md`,
		Example: `  yorkhost api /services
  yorkhost api /services --paginate
  yorkhost api -X POST /vps/1234/power -F action=restart
  yorkhost api -X POST /tickets -d '{"subject":"Test","body":"Bonjour"}'`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p := args[0]
			if !strings.HasPrefix(p, "/") {
				p = "/" + p
			}
			if !strings.HasPrefix(p, "/client/") {
				p = "/client/v1" + p
			}
			u, err := url.Parse(p)
			if err != nil {
				return usageErr(err)
			}
			m := strings.ToUpper(method)
			var body any
			switch {
			case data != "":
				raw, err := a.readText(data, "données")
				if err != nil {
					return err
				}
				var v any
				if err := json.Unmarshal([]byte(raw), &v); err != nil {
					return usageErr(fmt.Errorf("-d : JSON invalide : %w", err))
				}
				body = v
			case len(fields) > 0:
				obj := map[string]any{}
				for _, f := range fields {
					k, v, ok := strings.Cut(f, "=")
					if !ok {
						return usageErr(fmt.Errorf("-F %q : attendu clé=valeur", f))
					}
					obj[k] = typed(v)
				}
				body = obj
			}
			if m == "" {
				m = http.MethodGet
				if body != nil {
					m = http.MethodPost
				}
			}
			if paginate && m == http.MethodGet {
				items, _, err := a.Client().ListAll(ctx(cmd), u.Path, u.Query(), 0)
				if err != nil {
					return err
				}
				return printJSON(a, items)
			}
			resp, err := a.Client().Do(ctx(cmd), api.Request{Method: m, Path: u.Path, Query: u.Query(), Body: body})
			if resp != nil && include {
				fmt.Fprintf(a.Stderr, "HTTP %d\n", resp.Status)
				keys := make([]string, 0, len(resp.Header))
				for k := range resp.Header {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				for _, k := range keys {
					fmt.Fprintf(a.Stderr, "%s: %s\n", k, strings.Join(resp.Header[k], ", "))
				}
				fmt.Fprintln(a.Stderr)
			}
			if resp != nil && len(resp.Body) > 0 {
				var v any
				if json.Unmarshal(resp.Body, &v) == nil {
					_ = printJSON(a, v)
				} else {
					_, _ = a.Stdout.Write(resp.Body)
				}
			}
			return err
		},
	}
	f := cmd.Flags()
	f.StringVarP(&method, "method", "X", "", "méthode HTTP (défaut GET, POST avec un corps)")
	f.StringVarP(&data, "data", "d", "", "corps JSON (texte, @fichier ou - pour stdin)")
	f.StringArrayVarP(&fields, "field", "F", nil, "champ du corps JSON clé=valeur (nombres et booléens typés)")
	f.BoolVar(&paginate, "paginate", false, "parcourir toutes les pages (listes paginées)")
	f.BoolVarP(&include, "include", "i", false, "afficher statut et en-têtes HTTP (stderr)")
	_ = cmd.RegisterFlagCompletionFunc("method", fixedCompletion("GET", "POST", "PUT", "DELETE"))
	return cmd
}

// numberRe : nombre JSON (pas de zéro initial : 0612… reste une chaîne).
var numberRe = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]+)?$`)

func typed(v string) any {
	switch v {
	case "true":
		return true
	case "false":
		return false
	case "null":
		return nil
	}
	if numberRe.MatchString(v) {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return v
}

func printJSON(a *App, v any) error {
	if a.Printer.Format == output.YAML {
		return a.Printer.Raw(v)
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return err
	}
	_, err := a.Stdout.Write(buf.Bytes())
	return err
}

// ------------------------------------------------------------ mcp

func newMCPCmd(a *App) *cobra.Command {
	var client string
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Brancher un assistant IA (Claude Code, Cursor, VS Code…) sur votre compte",
		Long: `Le serveur MCP YorkHost (` + config.DefaultAPIURL + `/client/mcp) expose les mêmes
opérations que ce CLI comme outils pour assistants IA. Cette commande affiche la
configuration à copier pour votre client. Deux méthodes :

  - OAuth (recommandé) : le client ouvre lui-même la page de connexion YorkHost ;
  - clé API : créez une clé dédiée (espace client → Clés API), scopes minimum.`,
		Example: `  yorkhost mcp
  yorkhost mcp --client cursor`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			u := a.APIURL() + "/client/mcp"
			key := "yhk_client_v1_…"
			switch client {
			case "claude", "claude-code":
				fmt.Fprintf(a.Stdout, `# Claude Code — OAuth (connexion dans le navigateur au premier usage, via /mcp)
claude mcp add --transport http yorkhost %[1]s

# Claude Code — clé API
claude mcp add --transport http yorkhost %[1]s --header "Authorization: Bearer %[2]s"

# Claude.ai / ChatGPT : ajoutez un connecteur personnalisé avec l'URL %[1]s
`, u, key)
			case "cursor", "vscode", "json":
				cfg := map[string]any{"mcpServers": map[string]any{"yorkhost": map[string]any{
					"url":     u,
					"headers": map[string]string{"Authorization": "Bearer " + key},
				}}}
				if client == "vscode" {
					cfg = map[string]any{"servers": map[string]any{"yorkhost": map[string]any{
						"type": "http", "url": u,
						"headers": map[string]string{"Authorization": "Bearer " + key},
					}}}
					fmt.Fprintln(a.Stderr, "# .vscode/mcp.json")
				} else {
					fmt.Fprintln(a.Stderr, "# ~/.cursor/mcp.json (ou .cursor/mcp.json du projet)")
				}
				return printJSON(a, cfg)
			default:
				return usageErr(fmt.Errorf("client inconnu %q (claude, cursor, vscode, json)", client))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&client, "client", "claude", "client MCP : claude, cursor, vscode, json")
	_ = cmd.RegisterFlagCompletionFunc("client", fixedCompletion("claude", "cursor", "vscode", "json"))
	return cmd
}

// ------------------------------------------------------------ version

func newVersionCmd(a *App) *cobra.Command {
	var check bool
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Version du CLI (--check : nouvelle version disponible ?)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			info := map[string]any{"version": version.Version, "commit": version.Commit, "date": version.Date}
			var latest string
			if check {
				l, err := latestRelease(ctx(cmd))
				if err != nil {
					a.Printer.Warn("vérification impossible : %v", err)
				}
				latest = l
				info["latest"] = l
			}
			if a.Printer.Machine() {
				return a.Printer.Raw(info)
			}
			fmt.Fprintln(a.Stdout, version.String())
			if latest != "" {
				if strings.TrimPrefix(latest, "v") != strings.TrimPrefix(version.Version, "v") {
					a.Printer.Info("Nouvelle version disponible : %s — https://github.com/YorkHost-fr/yorkhost-cli/releases/latest", latest)
				} else {
					a.Printer.Info("Vous avez la dernière version.")
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&check, "check", false, "comparer avec la dernière version publiée")
	return cmd
}

func latestRelease(c context.Context) (string, error) {
	c, cancel := context.WithTimeout(c, 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(c, http.MethodGet, "https://api.github.com/repos/YorkHost-fr/yorkhost-cli/releases/latest", nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", version.UserAgent())
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub HTTP %d", res.StatusCode)
	}
	var out struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return "", err
	}
	if out.TagName == "" {
		return "", errors.New("aucune version publiée")
	}
	return out.TagName, nil
}

// ------------------------------------------------------------ docs

// frenchMarkdown : intertitres de cobra/doc en français.
func frenchMarkdown(dir string) error {
	r := strings.NewReplacer(
		"### Synopsis", "### Description",
		"### Options inherited from parent commands", "### Options globales",
		"### Examples", "### Exemples",
		"### SEE ALSO", "### Voir aussi",
		"\t - parent command", "\t - commande parente",
		" [flags]", " [options]",
	)
	files, err := filepath.Glob(filepath.Join(dir, "*.md"))
	if err != nil {
		return err
	}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		if err := os.WriteFile(f, []byte(r.Replace(string(b))), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// newGenDocsCmd : référence Markdown et pages man (make docs, goreleaser).
func newGenDocsCmd(root *cobra.Command) *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:    "gen-docs <dossier>",
		Short:  "Générer la référence des commandes (markdown ou man)",
		Hidden: true,
		Args:   cobra.ExactArgs(1),
		// Pas de configuration à charger.
		PersistentPreRunE: func(*cobra.Command, []string) error { return nil },
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := os.MkdirAll(args[0], 0o755); err != nil {
				return err
			}
			root.DisableAutoGenTag = true
			switch format {
			case "man":
				return doc.GenManTree(root, &doc.GenManHeader{Title: "YORKHOST", Section: "1", Source: "YorkHost", Manual: "YorkHost CLI"}, args[0])
			case "markdown":
				if err := doc.GenMarkdownTreeCustom(root, args[0], func(string) string { return "" }, func(name string) string { return name }); err != nil {
					return err
				}
				return frenchMarkdown(args[0])
			}
			return usageErr(fmt.Errorf("format inconnu %q (markdown, man)", format))
		},
	}
	cmd.Flags().StringVar(&format, "format", "markdown", "markdown ou man")
	return cmd
}
