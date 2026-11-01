package cli

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/spf13/cobra"

	"github.com/YorkHost-fr/yorkhost-cli/internal/output"
)

var (
	sandboxIDRe  = regexp.MustCompile(`^sbx[a-z2-7]{12}$`)
	secretNameRe = regexp.MustCompile(`^[A-Z_][A-Z0-9_]{0,63}$`)
	sandboxKinds = []string{"linux", "desktop", "windows"}
)

// maxFile : limite de lecture/écriture de fichier de l'API (1 Mio).
const maxFile = 1 << 20

func sandboxArg(args []string) (string, error) {
	if len(args) == 0 || !sandboxIDRe.MatchString(args[0]) {
		return "", usageErr(errors.New("ID de sandbox attendu (sbx…, voir « yorkhost sandbox list »)"))
	}
	return args[0], nil
}

func (a *App) sandboxCompletion(cmd *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	if a.File == nil && a.init() != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	var m map[string]any
	if err := a.Client().Get(ctx(cmd), "/client/v1/sandboxes", nil, &m); err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	var out []string
	for _, s := range list(m, "sandboxes") {
		out = append(out, str(s, "id")+"\t"+str(s, "kind")+" "+str(s, "status"))
	}
	return out, cobra.ShellCompDirectiveNoFileComp
}

func sandboxColumns() []output.Column {
	return []output.Column{
		{Header: "ID", Path: "id"},
		{Header: "Type", Path: "kind"},
		statusCol(),
		{Header: "SSH", Path: "ssh"},
		{Header: "Arrêt", Path: "run_until", Format: output.Relative},
		{Header: "Preview", Path: "preview_url", Wide: true},
		{Header: "Suppression", Path: "claim_until", Format: output.Relative, Wide: true},
	}
}

func sandboxFields() []output.Field {
	return []output.Field{
		{Header: "ID", Path: "id"},
		{Header: "Type", Path: "kind"},
		{Header: "Statut", Path: "status", Color: output.Status},
		{Header: "SSH", Path: "ssh"},
		{Header: "Preview", Path: "preview_url"},
		{Header: "Bureau", Path: "desktop_url"},
		{Header: "Créée", Path: "created_at", Format: output.Date},
		{Header: "Arrêt", Path: "run_until", Format: output.Relative},
		{Header: "Suppression", Path: "claim_until", Format: output.Relative},
		{Header: "Facture VPS", Path: "invoice_id"},
		{Header: "VPS", Path: "vps_service_id"},
		{Header: "IP du VPS", Path: "vps_ip"},
		{Header: "Transfert", Path: "transfer_state"},
	}
}

func newSandboxCmd(a *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "sandbox",
		Aliases: []string{"sandboxes", "sbx"},
		Short:   "Sandboxes agents : VM d'essai jetables (SSH via yorkhost.sh)",
		Long: `VM d'essai pour agents IA (Linux, bureau Linux, bureau Windows), sans IP publique :
SSH par la passerelle (ssh <id>@yorkhost.sh), applications web sur leur URL de preview.
Gratuites pendant leur fenêtre de fonctionnement, puis arrêtées ; supprimées avec leurs
fichiers au bout de 24 h, sauf passage en VPS (yorkhost sandbox keep).`,
		Example: `  yorkhost sandbox create --ssh            # crée un essai Linux et s'y connecte
  yorkhost sandbox exec sbxabcdefghijkl -- uname -a
  yorkhost sandbox upload sbxabcdefghijkl ./app.py
  yorkhost sandbox keep sbxabcdefghijkl    # le garder : passage en VPS-2`,
	}

	ls := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "Lister vos sandboxes",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			var m map[string]any
			if err := a.Client().Get(ctx(cmd), "/client/v1/sandboxes", nil, &m); err != nil {
				return err
			}
			return a.Printer.List(m, list(m, "sandboxes"), sandboxColumns())
		},
	}

	get := &cobra.Command{
		Use:               "get <id>",
		Short:             "Détail d'une sandbox",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: a.sandboxCompletion,
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := sandboxArg(args)
			if err != nil {
				return err
			}
			var m map[string]any
			if err := a.Client().Get(ctx(cmd), path("sandboxes", id), nil, &m); err != nil {
				return err
			}
			return a.Printer.Object(m, m, sandboxFields())
		},
	}

	cmd.AddCommand(ls, get, newSandboxCreateCmd(a), newSandboxDeleteCmd(a), newSandboxSSHCmd(a),
		newSandboxExecCmd(a), newSandboxLsCmd(a), newSandboxCatCmd(a), newSandboxUploadCmd(a),
		newSandboxDownloadCmd(a), newSandboxLinksCmd(a), newSandboxKeepCmd(a), newSandboxClaimCmd(a),
		newSandboxOffersCmd(a), newSandboxKeyCmd(a), newSandboxSecretCmd(a))
	return cmd
}

// defaultPublicKey : clé publique SSH locale (réglage du profil, sinon les
// emplacements habituels).
func (a *App) defaultPublicKey() (string, string, error) {
	var candidates []string
	if p := a.File.Profile(a.Profile); p != nil && p.SSHPublicKey != "" {
		candidates = append(candidates, expandHome(p.SSHPublicKey))
	} else if home, err := os.UserHomeDir(); err == nil {
		for _, n := range []string{"id_ed25519.pub", "id_ecdsa.pub", "id_rsa.pub"} {
			candidates = append(candidates, filepath.Join(home, ".ssh", n))
		}
	}
	for _, c := range candidates {
		if b, err := os.ReadFile(c); err == nil {
			return strings.TrimSpace(string(b)), c, nil
		}
	}
	return "", "", errors.New("aucune clé SSH publique trouvée : créez-en une (ssh-keygen -t ed25519) ou passez --ssh-key <fichier.pub>")
}

func readPublicKey(file string) (string, error) {
	b, err := os.ReadFile(expandHome(file))
	if err != nil {
		return "", err
	}
	k := strings.TrimSpace(string(b))
	if strings.Contains(k, "PRIVATE KEY") {
		return "", errors.New("c'est une clé privée : donnez le fichier .pub")
	}
	return k, nil
}

func expandHome(p string) string {
	if strings.HasPrefix(p, "~/") || p == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(p, "~"))
		}
	}
	return p
}

func newSandboxCreateCmd(a *App) *cobra.Command {
	var kind, keyFile string
	var noSecrets, connect, wait bool
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Créer une sandbox (gratuite pendant sa fenêtre de fonctionnement)",
		Example: `  yorkhost sandbox create
  yorkhost sandbox create --kind desktop --wait
  yorkhost sandbox create --ssh-key ~/.ssh/work.pub --no-secrets --ssh`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			c := ctx(cmd)
			var key, from string
			var err error
			if keyFile != "" {
				key, err = readPublicKey(keyFile)
				from = keyFile
			} else {
				key, from, err = a.defaultPublicKey()
			}
			if err != nil {
				return err
			}
			a.Printer.Info("Clé SSH : %s", from)
			body := map[string]any{"kind": kind, "ssh_public_key": key, "inject_secrets": !noSecrets}
			var res map[string]any
			if _, err := a.Client().Post(c, "/client/v1/sandboxes", body, &res); err != nil {
				return err
			}
			sb, _ := res["sandbox"].(map[string]any)
			if sb == nil {
				return errors.New("réponse inattendue de l'API (sandbox absente)")
			}
			id := str(sb, "id")
			for _, w := range strs(res["warnings"]) {
				a.Printer.Warn("%s", w)
			}
			if wait || connect {
				a.Printer.Info("Démarrage de %s…", id)
				err := a.waitFor(c, id, 5*time.Minute, func() (bool, string, error) {
					var cur map[string]any
					if err := a.Client().Get(c, path("sandboxes", id), nil, &cur); err != nil {
						return false, "", err
					}
					st := str(cur, "status")
					if st == "failed" || st == "destroyed" {
						return false, st, fmt.Errorf("la sandbox %s est %s", id, st)
					}
					if st == "running" {
						for k, v := range cur {
							if _, ok := sb[k]; !ok || k == "status" {
								sb[k] = v
							}
						}
					}
					return st == "running", st, nil
				})
				if err != nil {
					return err
				}
			}
			if a.Printer.Machine() {
				return a.Printer.Raw(res)
			}
			a.Printer.Success("Sandbox %s créée.", id)
			if err := a.Printer.Object(sb, sb, sandboxFields()); err != nil {
				return err
			}
			if msg := str(res, "message"); msg != "" {
				a.Printer.Info("\n%s", msg)
			}
			if n := num(res, "secrets_injected"); n > 0 {
				a.Printer.Info("%s secret(s) du compte exporté(s) dans l'essai.", output.Number(n))
			}
			if str(sb, "desktop_url") != "" {
				a.Printer.Info("Le lien bureau n'est montré qu'une fois ; nouveau lien : yorkhost sandbox links %s", id)
			}
			if connect {
				return a.runSSH([]string{sshTarget(sb)})
			}
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&kind, "kind", "linux", "type : "+strings.Join(sandboxKinds, ", "))
	f.StringVar(&keyFile, "ssh-key", "", "clé publique SSH (défaut : ~/.ssh/id_ed25519.pub, id_ecdsa.pub ou id_rsa.pub)")
	f.BoolVar(&noSecrets, "no-secrets", false, "ne pas exporter les secrets du compte dans l'essai")
	f.BoolVar(&wait, "wait", false, "attendre que la sandbox soit démarrée")
	f.BoolVar(&connect, "ssh", false, "se connecter en SSH une fois démarrée")
	_ = cmd.RegisterFlagCompletionFunc("kind", fixedCompletion(sandboxKinds...))
	return cmd
}

// sshTarget : « ssh sbx…@yorkhost.sh » → « sbx…@yorkhost.sh ».
func sshTarget(sb map[string]any) string {
	s := strings.TrimSpace(str(sb, "ssh"))
	s = strings.TrimSpace(strings.TrimPrefix(s, "ssh "))
	if s == "" {
		s = str(sb, "id") + "@yorkhost.sh"
	}
	return s
}

func newSandboxDeleteCmd(a *App) *cobra.Command {
	return &cobra.Command{
		Use:               "delete <id>",
		Aliases:           []string{"rm"},
		Short:             "Supprimer une sandbox et ses fichiers",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: a.sandboxCompletion,
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := sandboxArg(args)
			if err != nil {
				return err
			}
			if err := a.Prompt.Confirm(fmt.Sprintf("Supprimer %s et tous ses fichiers ? C'est définitif.", id), a.Yes); err != nil {
				return err
			}
			var m map[string]any
			if err := a.Client().Delete(ctx(cmd), path("sandboxes", id), nil, &m); err != nil {
				return err
			}
			if a.Printer.Machine() {
				return a.Printer.Raw(m)
			}
			a.Printer.Success("Sandbox %s supprimée.", id)
			return nil
		},
	}
}

func newSandboxSSHCmd(a *App) *cobra.Command {
	return &cobra.Command{
		Use:   "ssh <id> [-- commande…]",
		Short: "Se connecter en SSH (passerelle yorkhost.sh)",
		Example: `  yorkhost sandbox ssh sbxabcdefghijkl
  yorkhost sandbox ssh sbxabcdefghijkl -- -L 3000:localhost:3000`,
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: a.sandboxCompletion,
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := sandboxArg(args)
			if err != nil {
				return err
			}
			var sb map[string]any
			if err := a.Client().Get(ctx(cmd), path("sandboxes", id), nil, &sb); err != nil {
				return err
			}
			if st := str(sb, "status"); st != "running" {
				a.Printer.Warn("La sandbox est %s : la connexion peut échouer.", st)
			}
			extra := args[1:]
			// Options ssh (-L, -A…) avant la cible, commande après.
			var opts, rest []string
			for i := 0; i < len(extra); i++ {
				if strings.HasPrefix(extra[i], "-") && len(rest) == 0 {
					opts = append(opts, extra[i])
					if len(extra[i]) == 2 && strings.ContainsRune("LRDJiopFlbcEmOQSWw", rune(extra[i][1])) && i+1 < len(extra) {
						opts = append(opts, extra[i+1])
						i++
					}
					continue
				}
				rest = append(rest, extra[i])
			}
			return a.runSSH(append(append(opts, sshTarget(sb)), rest...))
		},
	}
}

func newSandboxExecCmd(a *App) *cobra.Command {
	var cwd, stdinFile string
	var timeout int
	cmd := &cobra.Command{
		Use:   "exec <id> -- <commande…>",
		Short: "Exécuter une commande (bash -lc, utilisateur agent) sans SSH",
		Long: `Exécute une commande dans la sandbox (Linux et bureau Linux en marche) sous
l'utilisateur agent, via bash -lc, dans /home/agent ou --cwd. stdout et stderr sont
relayés ; le code de sortie du CLI est celui de la commande. 60 s par défaut, 300 s au plus.`,
		Example: `  yorkhost sandbox exec sbxabcdefghijkl -- 'cd app && npm test'
  echo 'print(1+1)' | yorkhost sandbox exec sbxabcdefghijkl -- python3
  yorkhost sandbox exec sbxabcdefghijkl --timeout 300 -- ./build.sh`,
		Args:              cobra.MinimumNArgs(2),
		ValidArgsFunction: a.sandboxCompletion,
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := sandboxArg(args)
			if err != nil {
				return err
			}
			body := map[string]any{"command": strings.Join(args[1:], " ")}
			if cwd != "" {
				body["cwd"] = cwd
			}
			if timeout > 0 {
				body["timeout_seconds"] = timeout
			}
			switch {
			case stdinFile != "":
				b, err := os.ReadFile(stdinFile)
				if err != nil {
					return err
				}
				body["stdin"] = string(b)
			case !a.Prompt.Interactive && a.Stdin != nil && isPiped(a.Stdin):
				s, err := a.readText("-", "stdin")
				if err != nil {
					return err
				}
				if s != "" {
					body["stdin"] = s + "\n"
				}
			}
			var res map[string]any
			if _, err := a.Client().Post(ctx(cmd), path("sandboxes", id, "exec"), body, &res); err != nil {
				return err
			}
			if a.Printer.Machine() {
				if err := a.Printer.Raw(res); err != nil {
					return err
				}
			} else {
				fmt.Fprint(a.Stdout, str(res, "stdout"))
				fmt.Fprint(a.Stderr, str(res, "stderr"))
				if res["stdout_truncated"] == true || res["stderr_truncated"] == true {
					a.Printer.Warn("sortie tronquée (256 Kio au plus par flux)")
				}
				if res["timed_out"] == true {
					a.Printer.Warn("délai dépassé : la commande a été tuée")
				}
				if n := str(res, "note"); n != "" {
					a.Printer.Info("%s", n)
				}
			}
			if code := int(num(res, "exit_code")); code != 0 {
				return silentExit(code)
			}
			if res["timed_out"] == true {
				return silentExit(124)
			}
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&cwd, "cwd", "", "dossier de travail (sous /home/agent)")
	f.IntVar(&timeout, "timeout", 0, "délai en secondes (1 à 300, défaut 60)")
	f.StringVar(&stdinFile, "stdin-file", "", "fichier passé sur l'entrée standard (32 Kio au plus)")
	return cmd
}

func isPiped(r any) bool {
	f, ok := r.(*os.File)
	if !ok {
		return true
	}
	st, err := f.Stat()
	return err == nil && st.Mode()&os.ModeCharDevice == 0
}

func newSandboxLsCmd(a *App) *cobra.Command {
	var depth int
	cmd := &cobra.Command{
		Use:               "ls <id> [dossier]",
		Short:             "Lister un dossier (défaut /home/agent)",
		Args:              cobra.RangeArgs(1, 2),
		ValidArgsFunction: a.sandboxCompletion,
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := sandboxArg(args)
			if err != nil {
				return err
			}
			q := url.Values{}
			if len(args) > 1 {
				q.Set("path", args[1])
			}
			if depth > 1 {
				q.Set("depth", strconv.Itoa(depth))
			}
			var m map[string]any
			if err := a.Client().Get(ctx(cmd), path("sandboxes", id, "files"), q, &m); err != nil {
				return err
			}
			for _, w := range strs(m["warnings"]) {
				a.Printer.Warn("%s", w)
			}
			if err := a.Printer.List(m, list(m, "entries"), []output.Column{
				{Header: "Droits", Path: "mode"},
				{Header: "Taille", Path: "size", Format: output.Bytes},
				{Header: "Modifié", Path: "mtime", Format: output.Date},
				{Header: "Type", Path: "type", Wide: true},
				{Header: "Nom", Value: func(e map[string]any) any {
					if str(e, "type") == "dir" {
						return str(e, "name") + "/"
					}
					return str(e, "name")
				}},
			}); err != nil {
				return err
			}
			if m["truncated"] == true {
				a.Printer.Warn("liste tronquée à 1 000 entrées")
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&depth, "depth", 1, "profondeur (1 à 3)")
	return cmd
}

// readRemote : contenu d'un fichier distant (décodé).
func (a *App) readRemote(cmd *cobra.Command, id, remote string) ([]byte, map[string]any, error) {
	var m map[string]any
	if err := a.Client().Get(ctx(cmd), path("sandboxes", id, "files", "content"), url.Values{"path": {remote}}, &m); err != nil {
		return nil, nil, err
	}
	content := str(m, "content")
	if str(m, "encoding") == "base64" {
		b, err := base64.StdEncoding.DecodeString(content)
		if err != nil {
			return nil, nil, fmt.Errorf("contenu base64 illisible : %w", err)
		}
		return b, m, nil
	}
	return []byte(content), m, nil
}

func newSandboxCatCmd(a *App) *cobra.Command {
	return &cobra.Command{
		Use:               "cat <id> <fichier>",
		Short:             "Afficher un fichier (1 Mio au plus)",
		Args:              cobra.ExactArgs(2),
		ValidArgsFunction: a.sandboxCompletion,
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := sandboxArg(args)
			if err != nil {
				return err
			}
			b, m, err := a.readRemote(cmd, id, args[1])
			if err != nil {
				return err
			}
			if a.Printer.Machine() {
				return a.Printer.Raw(m)
			}
			if _, err := a.Stdout.Write(b); err != nil {
				return err
			}
			if m["truncated"] == true {
				a.Printer.Warn("fichier tronqué au premier Mio (%s au total)", output.Bytes(m["size"]))
			}
			return nil
		},
	}
}

func newSandboxDownloadCmd(a *App) *cobra.Command {
	return &cobra.Command{
		Use:               "download <id> <fichier distant> [destination]",
		Short:             "Télécharger un fichier (1 Mio au plus)",
		Args:              cobra.RangeArgs(2, 3),
		ValidArgsFunction: a.sandboxCompletion,
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := sandboxArg(args)
			if err != nil {
				return err
			}
			b, m, err := a.readRemote(cmd, id, args[1])
			if err != nil {
				return err
			}
			if m["truncated"] == true {
				return fmt.Errorf("fichier trop gros (%s) : l'API en lit 1 Mio au plus — passez par yorkhost sandbox ssh / scp", output.Bytes(m["size"]))
			}
			dest := filepath.Base(args[1])
			if len(args) > 2 {
				dest = args[2]
				if st, err := os.Stat(dest); err == nil && st.IsDir() {
					dest = filepath.Join(dest, filepath.Base(args[1]))
				}
			}
			if err := os.WriteFile(dest, b, 0o644); err != nil {
				return err
			}
			a.Printer.Success("%s → %s (%s)", args[1], dest, output.Bytes(float64(len(b))))
			if a.Printer.Machine() {
				return a.Printer.Raw(map[string]any{"path": args[1], "file": dest, "size": len(b)})
			}
			return nil
		},
	}
}

func newSandboxUploadCmd(a *App) *cobra.Command {
	var mode string
	cmd := &cobra.Command{
		Use:   "upload <id> <fichier local> [chemin distant]",
		Short: "Envoyer un fichier (1 Mio au plus, défaut /home/agent/<nom>)",
		Example: `  yorkhost sandbox upload sbxabcdefghijkl ./setup.sh --mode 755
  yorkhost sandbox upload sbxabcdefghijkl .env app/.env`,
		Args:              cobra.RangeArgs(2, 3),
		ValidArgsFunction: a.sandboxCompletion,
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := sandboxArg(args)
			if err != nil {
				return err
			}
			b, err := os.ReadFile(args[1])
			if err != nil {
				return err
			}
			if len(b) > maxFile {
				return fmt.Errorf("fichier trop gros (%s, 1 Mio au plus) : passez par yorkhost sandbox ssh / scp", output.Bytes(float64(len(b))))
			}
			remote := "/home/agent/" + filepath.Base(args[1])
			if len(args) > 2 {
				remote = args[2]
				if strings.HasSuffix(remote, "/") {
					remote += filepath.Base(args[1])
				}
			}
			body := map[string]any{"path": remote}
			if utf8.Valid(b) {
				body["content"], body["encoding"] = string(b), "utf-8"
			} else {
				body["content"], body["encoding"] = base64.StdEncoding.EncodeToString(b), "base64"
			}
			if mode == "" {
				if st, err := os.Stat(args[1]); err == nil && st.Mode().Perm()&0o111 != 0 {
					mode = "755"
				}
			}
			if mode != "" {
				body["mode"] = mode
			}
			var m map[string]any
			if err := a.Client().Put(ctx(cmd), path("sandboxes", id, "files", "content"), body, &m); err != nil {
				return err
			}
			if a.Printer.Machine() {
				return a.Printer.Raw(m)
			}
			a.Printer.Success("%s → %s:%s (%s, %s)", args[1], id, str(m, "path"), output.Bytes(m["size"]), str(m, "mode"))
			return nil
		},
	}
	cmd.Flags().StringVar(&mode, "mode", "", "droits en octal (ex. 644, 755) ; défaut : 755 si exécutable localement")
	return cmd
}

func newSandboxLinksCmd(a *App) *cobra.Command {
	return &cobra.Command{
		Use:               "links <id>",
		Short:             "Nouveau lien bureau (l'ancien cesse de fonctionner)",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: a.sandboxCompletion,
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := sandboxArg(args)
			if err != nil {
				return err
			}
			var m map[string]any
			if _, err := a.Client().Post(ctx(cmd), path("sandboxes", id, "links"), nil, &m); err != nil {
				return err
			}
			u := str(m, "desktop_url")
			if u == "" {
				if a.Printer.Machine() {
					return a.Printer.Raw(m)
				}
				return errors.New("pas de bureau sur cette sandbox (type linux)")
			}
			a.openURL(u)
			return nil
		},
	}
}

func newSandboxKeepCmd(a *App) *cobra.Command {
	var openPay bool
	cmd := &cobra.Command{
		Use:   "keep <id>",
		Short: "Garder la sandbox : commander un VPS-2 et y copier /home/agent",
		Long: `Commande un VPS classique (VPS-2, IP publique, géré dans l'espace client).
Une fois la facture payée, le dossier /home/agent de l'essai est copié et vérifié
dans le nouveau VPS, puis l'essai est supprimé. Devis puis confirmation, comme
« yorkhost order create ».`,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: a.sandboxCompletion,
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := sandboxArg(args)
			if err != nil {
				return err
			}
			return a.order(cmd, path("sandboxes", id, "keep"), map[string]any{}, openPay)
		},
	}
	cmd.Flags().BoolVar(&openPay, "pay", false, "ouvrir la page de paiement si la facture est impayée")
	return cmd
}

func newSandboxClaimCmd(a *App) *cobra.Command {
	var openPay bool
	cmd := &cobra.Command{
		Use:   "claim <jeton|lien>",
		Short: "Rattacher un essai anonyme (ssh yorkhost.sh) et le passer en VPS",
		Long: `Rattache à votre compte un essai créé anonymement avec « ssh yorkhost.sh », à partir
du jeton yhsc_… ou du lien « Keep it » affiché dans l'essai, et commande le VPS-2 qui
le recevra (devis puis confirmation).`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.order(cmd, "/client/v1/sandboxes/claim", map[string]any{"claim_token": strings.TrimSpace(args[0])}, openPay)
		},
	}
	cmd.Flags().BoolVar(&openPay, "pay", false, "ouvrir la page de paiement si la facture est impayée")
	return cmd
}

func newSandboxOffersCmd(a *App) *cobra.Command {
	return &cobra.Command{
		Use:   "offers",
		Short: "Types de sandbox proposés et leurs limites",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			var m map[string]any
			if err := a.Client().Get(ctx(cmd), "/client/v1/sandboxes", nil, &m); err != nil {
				return err
			}
			offers := list(m, "offers")
			return a.Printer.List(offers, offers, []output.Column{
				{Header: "Type", Path: "kind"},
				{Header: "Bureau", Path: "desktop"},
				{Header: "Gratuit (min)", Path: "free_minutes"},
				{Header: "vCPU", Path: "cores"},
				{Header: "RAM", Path: "memory_mb", Format: output.MB},
				{Header: "Disque (Go)", Path: "disk_gb"},
				{Header: "Réseau (Mbit/s)", Path: "network_mbit"},
				{Header: "Max actifs", Path: "max_active_per_account"},
				{Header: "→ VPS", Path: "can_become_vps"},
			})
		},
	}
}

// ------------------------------------------------------------ clés SSH du compte

func newSandboxKeyCmd(a *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "key",
		Aliases: []string{"keys", "ssh-key"},
		Short:   "Clés SSH du compte acceptées sur toutes vos sandboxes",
	}
	ls := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "Lister les clés",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			var m map[string]any
			if err := a.Client().Get(ctx(cmd), "/client/v1/sandbox-keys", nil, &m); err != nil {
				return err
			}
			return a.Printer.List(m, list(m, "keys"), []output.Column{
				{Header: "Nom", Path: "name"},
				{Header: "Empreinte", Path: "fingerprint"},
				{Header: "Ajoutée", Path: "created_at", Format: output.Date},
				{Header: "Clé", Path: "public_key", Wide: true},
			})
		},
	}
	var name string
	add := &cobra.Command{
		Use:     "add [fichier.pub]",
		Short:   "Enregistrer une clé publique (défaut : ~/.ssh/id_ed25519.pub…)",
		Example: `  yorkhost sandbox key add ~/.ssh/id_ed25519.pub --name laptop`,
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var key string
			var err error
			if len(args) == 1 {
				key, err = readPublicKey(args[0])
			} else {
				key, _, err = a.defaultPublicKey()
			}
			if err != nil {
				return err
			}
			body := map[string]any{"public_key": key}
			if name != "" {
				body["name"] = name
			}
			var m map[string]any
			if _, err := a.Client().Post(ctx(cmd), "/client/v1/sandbox-keys", body, &m); err != nil {
				return err
			}
			if a.Printer.Machine() {
				return a.Printer.Raw(m)
			}
			k, _ := m["key"].(map[string]any)
			a.Printer.Success("Clé %s enregistrée.", str(k, "fingerprint"))
			return nil
		},
	}
	add.Flags().StringVar(&name, "name", "", "libellé (défaut : commentaire de la clé)")
	rm := &cobra.Command{
		Use:     "remove <empreinte|fichier.pub>",
		Aliases: []string{"rm", "delete"},
		Short:   "Retirer une clé",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fp := args[0]
			if !strings.HasPrefix(fp, "SHA256:") {
				if k, err := readPublicKey(fp); err == nil {
					fp = k
				}
			}
			var m map[string]any
			if err := a.Client().Delete(ctx(cmd), "/client/v1/sandbox-keys", url.Values{"fingerprint": {fp}}, &m); err != nil {
				return err
			}
			if a.Printer.Machine() {
				return a.Printer.Raw(m)
			}
			a.Printer.Success("Clé %s retirée.", str(m, "fingerprint"))
			return nil
		},
	}
	cmd.AddCommand(ls, add, rm)
	return cmd
}

// ------------------------------------------------------------ secrets du compte

func newSandboxSecretCmd(a *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "secret",
		Aliases: []string{"secrets", "env"},
		Short:   "Secrets exportés dans vos sandboxes Linux (ANTHROPIC_API_KEY…)",
		Long: `Variables d'environnement chiffrées (AES-256-GCM) exportées dans les sandboxes Linux
créées ensuite (/home/agent/.config/yorkhost/env). Les valeurs ne sont jamais relues.`,
	}
	ls := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "Lister les secrets (noms seulement)",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			var m map[string]any
			if err := a.Client().Get(ctx(cmd), "/client/v1/sandbox-secrets", nil, &m); err != nil {
				return err
			}
			return a.Printer.List(m, list(m, "secrets"), []output.Column{
				{Header: "Nom", Path: "name"},
				{Header: "Mis à jour", Path: "updated_at", Format: output.Date},
			})
		},
	}
	var fromStdin bool
	set := &cobra.Command{
		Use:   "set <NOM> [valeur]",
		Short: "Créer ou remplacer un secret (saisie masquée si la valeur est omise)",
		Example: `  yorkhost sandbox secret set ANTHROPIC_API_KEY
  printf %s "$TOKEN" | yorkhost sandbox secret set GITHUB_TOKEN --stdin`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if !secretNameRe.MatchString(name) {
				return usageErr(fmt.Errorf("nom invalide %q : majuscules, chiffres et _, 64 caractères au plus", name))
			}
			var value string
			var err error
			switch {
			case len(args) == 2:
				value = args[1]
				a.Printer.Warn("valeur passée en argument : elle peut rester dans l'historique du shell (préférez la saisie masquée ou --stdin)")
			case fromStdin:
				value, err = a.readText("-", "valeur")
			default:
				value, err = a.Prompt.Secret("Valeur de " + name)
			}
			if err != nil {
				return err
			}
			if value == "" {
				return usageErr(errors.New("valeur vide"))
			}
			var m map[string]any
			if err := a.Client().Put(ctx(cmd), path("sandbox-secrets", name), map[string]string{"value": value}, &m); err != nil {
				return err
			}
			if a.Printer.Machine() {
				return a.Printer.Raw(m)
			}
			a.Printer.Success("Secret %s enregistré (sandboxes créées à partir de maintenant).", name)
			return nil
		},
	}
	set.Flags().BoolVar(&fromStdin, "stdin", false, "lire la valeur sur stdin")
	del := &cobra.Command{
		Use:     "delete <NOM>",
		Aliases: []string{"rm", "unset"},
		Short:   "Supprimer un secret",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var m map[string]any
			if err := a.Client().Delete(ctx(cmd), path("sandbox-secrets", args[0]), nil, &m); err != nil {
				return err
			}
			if a.Printer.Machine() {
				return a.Printer.Raw(m)
			}
			a.Printer.Success("Secret %s supprimé.", args[0])
			return nil
		},
	}
	cmd.AddCommand(ls, set, del)
	return cmd
}
