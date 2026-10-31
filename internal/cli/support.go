package cli

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/YorkHost-fr/yorkhost-cli/internal/output"
)

var ticketTypes = []string{"HELP", "BILLING", "CANCELLATION", "OTHER"}

func newTicketCmd(a *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "ticket",
		Aliases: []string{"tickets", "support"},
		Short:   "Support : tickets, réponses",
	}
	var openOnly bool
	ls := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "Lister vos tickets",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			items, _, err := a.Client().ListAll(ctx(cmd), "/client/v1/tickets", nil, 0)
			if err != nil {
				return err
			}
			out := []map[string]any{}
			for _, it := range items {
				if !openOnly || it["open"] == true {
					out = append(out, it)
				}
			}
			return a.Printer.List(out, out, []output.Column{
				{Header: "Numéro", Path: "number"},
				{Header: "Sujet", Path: "subject", Max: 50},
				{Header: "Statut", Path: "statusLabel"},
				{Header: "Ouvert", Path: "open", Color: output.Status},
				{Header: "Mis à jour", Path: "lastUpdate", Format: output.Date},
				{Header: "ID", Path: "id", Wide: true},
			})
		},
	}
	ls.Flags().BoolVar(&openOnly, "open", false, "tickets ouverts seulement")

	get := &cobra.Command{
		Use:   "get <numéro|id>",
		Short: "Afficher un ticket et ses messages",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var m map[string]any
			if err := a.Client().Get(ctx(cmd), path("tickets", args[0]), nil, &m); err != nil {
				return err
			}
			if a.Printer.Machine() {
				return a.Printer.Raw(m)
			}
			if err := a.Printer.Object(m, m, []output.Field{
				{Header: "Ticket", Path: "number"},
				{Header: "Sujet", Path: "subject"},
				{Header: "Statut", Path: "statusLabel"},
				{Header: "Service", Path: "department"},
				{Header: "Mis à jour", Path: "lastUpdate", Format: output.Date},
			}); err != nil {
				return err
			}
			for _, msg := range list(m, "messages") {
				who := "Vous"
				if str(msg, "author") == "staff" {
					who = "Support YorkHost"
				}
				head := fmt.Sprintf("── %s · %s ", who, output.Date(msg["date"]))
				if a.Printer.Color {
					head = output.Bold(head)
				}
				fmt.Fprintf(a.Stdout, "\n%s\n%s\n", head, strings.TrimSpace(str(msg, "body")))
			}
			return nil
		},
	}

	var subject, body, kind, service string
	create := &cobra.Command{
		Use:   "create",
		Short: "Ouvrir un ticket",
		Example: `  yorkhost ticket create --subject "VPS injoignable" --body "Depuis 10h…" --service 1234
  yorkhost ticket create --subject "Question facture" --type BILLING --body @message.txt
  journalctl -n 50 | yorkhost ticket create --subject "Erreurs au démarrage" --body -`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			var err error
			if subject == "" {
				if subject, err = a.Prompt.Input("Sujet", ""); err != nil {
					return usageErr(errors.New("--subject requis"))
				}
			}
			text, err := a.readMessage(body, "Message")
			if err != nil {
				return err
			}
			if strings.TrimSpace(subject) == "" || strings.TrimSpace(text) == "" {
				return usageErr(errors.New("sujet et message requis"))
			}
			req := map[string]any{"subject": subject, "body": text, "requestType": strings.ToUpper(kind)}
			if service != "" {
				id, err := a.resolveService(ctx(cmd), []string{service}, "")
				if err != nil {
					return err
				}
				req["serviceId"] = id
			}
			var res map[string]any
			if _, err := a.Client().Post(ctx(cmd), "/client/v1/tickets", req, &res); err != nil {
				return err
			}
			if a.Printer.Machine() {
				return a.Printer.Raw(res)
			}
			a.Printer.Success("Ticket %s ouvert. Suivi : yorkhost ticket get %s", str(res, "number"), str(res, "number"))
			return nil
		},
	}
	f := create.Flags()
	f.StringVarP(&subject, "subject", "s", "", "sujet")
	f.StringVarP(&body, "body", "b", "", "message (texte, @fichier ou - pour stdin)")
	f.StringVarP(&kind, "type", "t", "HELP", "type de demande : "+strings.Join(ticketTypes, ", "))
	f.StringVar(&service, "service", "", "service concerné (ID ou nom)")
	_ = create.RegisterFlagCompletionFunc("type", fixedCompletion(ticketTypes...))

	var replyBody string
	reply := &cobra.Command{
		Use:     "reply <numéro|id>",
		Short:   "Répondre à un ticket",
		Example: `  yorkhost ticket reply 10234 --body "C'est réglé, merci !"`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			text, err := a.readMessage(replyBody, "Réponse")
			if err != nil {
				return err
			}
			if strings.TrimSpace(text) == "" {
				return usageErr(errors.New("message vide"))
			}
			if _, err := a.Client().Post(ctx(cmd), path("tickets", args[0], "reply"), map[string]string{"body": text}, nil); err != nil {
				return err
			}
			a.Printer.Success("Réponse envoyée sur le ticket %s.", args[0])
			if a.Printer.Machine() {
				return a.Printer.Raw(map[string]any{"ticket": args[0], "replied": true})
			}
			return nil
		},
	}
	reply.Flags().StringVarP(&replyBody, "body", "b", "", "message (texte, @fichier ou - pour stdin)")

	deps := &cobra.Command{
		Use:   "types",
		Short: "Types de demande disponibles",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			var arr []map[string]any
			if err := a.Client().Get(ctx(cmd), "/client/v1/ticket-departments", nil, &arr); err != nil {
				return err
			}
			return a.Printer.List(arr, arr, []output.Column{{Header: "Type", Path: "id"}, {Header: "Nom", Path: "name"}})
		},
	}
	cmd.AddCommand(ls, get, create, reply, deps)
	return cmd
}

// ------------------------------------------------------------ anti-DDoS

func newAntiDDoSCmd(a *App) *cobra.Command {
	var days int
	var ip string
	fetch := func(cmd *cobra.Command) (map[string]any, error) {
		var m map[string]any
		q := url.Values{}
		if days > 0 {
			q.Set("days", strconv.Itoa(days))
		}
		if ip != "" {
			q.Set("ip", ip)
		}
		err := a.Client().Get(ctx(cmd), "/client/v1/antiddos", q, &m)
		return m, err
	}
	cmd := &cobra.Command{
		Use:     "antiddos",
		Aliases: []string{"ddos"},
		Short:   "Anti-DDoS : attaques détectées, IP protégées et règles",
	}
	// --days : propre aux commandes d'attaques ; gardé (masqué) sur antiddos
	// pour « yorkhost antiddos --days 7 attacks ».
	cmd.PersistentFlags().IntVar(&days, "days", 90, "période en jours (1 à 365)")
	_ = cmd.PersistentFlags().MarkHidden("days")

	attacks := &cobra.Command{
		Use:     "attacks",
		Aliases: []string{"list", "ls"},
		Short:   "Attaques détectées sur vos IP",
		Example: `  yorkhost antiddos attacks --days 7
  yorkhost antiddos attacks --ip 203.0.113.10 -o json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			m, err := fetch(cmd)
			if err != nil {
				return err
			}
			rows := list(m, "attacks")
			return a.Printer.List(m, rows, []output.Column{
				{Header: "Début", Path: "start_time", Format: output.Date},
				{Header: "Type", Path: "alert_type"},
				{Header: "Cible", Value: func(r map[string]any) any {
					var t []string
					for _, mt := range list(r, "matches") {
						s := str(mt, "ip")
						if p := str(mt, "port"); p != "" {
							s += ":" + p
						}
						if n := str(mt, "service_name"); n != "" {
							s += " (" + n + ")"
						}
						t = append(t, s)
					}
					return strings.Join(t, ", ")
				}, Max: 50},
				{Header: "Débit", Path: "power_bps", Format: bps},
				{Header: "Paquets", Path: "power_pps", Format: pps},
				{Header: "Ports", Path: "dst_ports", Wide: true},
				{Header: "Source", Path: "source", Wide: true},
			})
		},
	}
	targets := &cobra.Command{
		Use:   "targets",
		Short: "IP et serveurs de jeu protégés",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			m, err := fetch(cmd)
			if err != nil {
				return err
			}
			var rows []map[string]any
			for _, ip := range list(m, "ipam_ips") {
				rows = append(rows, map[string]any{"kind": "IP", "addr": ip["ip"], "service_id": ip["service_id"], "service_name": ip["service_name"], "since": ip["active_since"]})
			}
			for _, g := range list(m, "game_servers") {
				addr := str(g, "ip") + ":" + str(g, "port")
				rows = append(rows, map[string]any{"kind": "jeu", "addr": addr, "service_id": g["service_id"], "service_name": g["service_name"], "since": g["active_since"]})
			}
			raw := map[string]any{"ipam_ips": m["ipam_ips"], "game_servers": m["game_servers"]}
			return a.Printer.List(raw, rows, []output.Column{
				{Header: "Type", Path: "kind"},
				{Header: "Adresse", Path: "addr"},
				{Header: "Service", Path: "service_id"},
				{Header: "Nom", Path: "service_name", Max: 36},
				{Header: "Depuis", Path: "since", Format: output.Date},
			})
		},
	}
	status := &cobra.Command{
		Use:   "status",
		Short: "État de la protection",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			m, err := fetch(cmd)
			if err != nil {
				return err
			}
			m["attack_count"] = float64(len(list(m, "attacks")))
			return a.Printer.Object(m, m, []output.Field{
				{Header: "Protection active", Path: "protection_active", Color: output.Status},
				{Header: "Vérifiée", Path: "protection_checked"},
				{Header: "Règles activées", Path: "rules_enabled"},
				{Header: "Attaques", Path: "attack_count"},
				{Header: "Période (jours)", Path: "days"},
				{Header: "Généré", Path: "generated_at", Format: output.Date},
			})
		},
	}
	for _, c := range []*cobra.Command{attacks, targets, status} {
		c.Flags().IntVar(&days, "days", 90, "période en jours (1 à 365)")
		c.Flags().StringVar(&ip, "ip", "", "une seule adresse IP (cibles et attaques de cette IP)")
	}
	cmd.AddCommand(attacks, targets, status, newAntiDDoSRulesCmd(a))
	return cmd
}

// antiddosRuleColumns : colonnes d'une règle (liste et résultat d'écriture).
var antiddosRuleColumns = []output.Column{
	{Header: "ID", Path: "id"},
	{Header: "IP", Path: "ip"},
	{Header: "Pos.", Path: "position"},
	{Header: "Politique", Path: "policy", Max: 28},
	{Header: "Protocoles", Value: func(r map[string]any) any { return strings.Join(strs(r["proto_list"]), ",") }},
	{Header: "Ports dest.", Value: func(r map[string]any) any { return strings.Join(strs(r["dport_list"]), ",") }, Max: 24},
	statusCol("Statut", "status"),
	{Header: "Ports source", Value: func(r map[string]any) any { return strings.Join(strs(r["sport_list"]), ",") }, Wide: true},
	{Header: "Sources", Value: func(r map[string]any) any { return strings.Join(strs(r["sip_list"]), ",") }, Wide: true, Max: 40},
	{Header: "Description", Path: "description", Wide: true, Max: 40},
	{Header: "Mise à jour", Path: "updated_at", Format: output.Date, Wide: true},
}

// splitList : valeurs de drapeaux répétables ou séparées par des virgules.
func splitList(in []string) []string {
	out := []string{}
	for _, v := range in {
		for _, p := range strings.Split(v, ",") {
			if p = strings.TrimSpace(p); p != "" {
				out = append(out, p)
			}
		}
	}
	return out
}

func newAntiDDoSRulesCmd(a *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "rules",
		Aliases: []string{"rule"},
		Short:   "Règles anti-DDoS de vos IP (créées en attente, appliquées par le NOC)",
		Long: `Règles anti-DDoS de vos IP : politique Gcore appliquée au trafic qui correspond
(protocoles, ports, sources). Une nouvelle règle est créée avec le statut « pending » puis
appliquée par le NOC YorkHost. Les règles d'une IP sont évaluées dans l'ordre (position 1 en
premier).`,
	}

	// --- list ---------------------------------------------------------------
	var listIP string
	ls := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "Lister vos règles, politiques disponibles et IP éligibles",
		Example: `  yorkhost antiddos rules list
  yorkhost antiddos rules list --ip 203.0.113.10 -o wide
  yorkhost antiddos rules list -o json   # état complet : politiques, plafonds, IP éligibles`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			q := url.Values{}
			if listIP != "" {
				q.Set("ip", listIP)
			}
			var m map[string]any
			if err := a.Client().Get(ctx(cmd), "/client/v1/antiddos/rules", q, &m); err != nil {
				return err
			}
			if err := a.Printer.List(m, list(m, "rules"), antiddosRuleColumns); err != nil || a.Printer.Machine() {
				return err
			}
			var pols []string
			for _, p := range list(m, "policies") {
				pols = append(pols, str(p, "name"))
			}
			a.Printer.Info("Compte %s · IP éligibles : %s", str(m, "account_level"), orNone(strs(m["eligible_ips"])))
			a.Printer.Info("Politiques disponibles : %s", orNone(pols))
			return nil
		},
	}
	ls.Flags().StringVar(&listIP, "ip", "", "règles d'une seule adresse IP")

	// --- create -------------------------------------------------------------
	var (
		ip, policy, description string
		protos, dports, sports  []string
		sources                 []string
		position                int
	)
	create := &cobra.Command{
		Use:   "create",
		Short: "Créer une règle (statut « pending » jusqu'à son application)",
		Example: `  yorkhost antiddos rules create --ip 203.0.113.10 --policy "FiveM UDP" --proto udp --dport 30120
  yorkhost antiddos rules create --ip 203.0.113.10 --policy "Rate Limiter Low" --proto udp --dport 27015-27030 --position 1
  yorkhost antiddos rules create --ip 203.0.113.10 --policy "Allow List" --proto tcp --dport 22 --sip 198.51.100.7 --description "bastion"`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(ip) == "" || strings.TrimSpace(policy) == "" {
				return usageErr(errors.New("--ip et --policy requis (voir yorkhost antiddos rules list)"))
			}
			if cmd.Flags().Changed("position") && position < 1 {
				return usageErr(errors.New("--position doit être ≥ 1"))
			}
			req := map[string]any{"ip": strings.TrimSpace(ip), "policy": strings.TrimSpace(policy)}
			for k, v := range map[string][]string{"proto_list": protos, "dport_list": dports, "sport_list": sports, "sip_list": sources} {
				if l := splitList(v); len(l) > 0 {
					req[k] = l
				}
			}
			if description != "" {
				req["description"] = description
			}
			if position > 0 {
				req["position"] = position
			}
			var res map[string]any
			if _, err := a.Client().Post(ctx(cmd), "/client/v1/antiddos/rules", req, &res); err != nil {
				return err
			}
			if a.Printer.Machine() {
				return a.Printer.Raw(res)
			}
			rule, _ := res["rule"].(map[string]any)
			a.Printer.Success("Règle %s demandée (%s) : elle sera appliquée par le NOC. Suivi : yorkhost antiddos rules list --ip %s",
				str(rule, "id"), str(rule, "status"), str(req, "ip"))
			return nil
		},
	}
	f := create.Flags()
	f.StringVar(&ip, "ip", "", "IPv4 protégée (une des IP éligibles)")
	f.StringVar(&policy, "policy", "", "politique (voir rules list), ex. \"FiveM UDP\", Drop")
	f.StringSliceVar(&protos, "proto", nil, "protocoles : tcp, udp, icmp, gre (répétable ou séparés par des virgules)")
	f.StringSliceVar(&dports, "dport", nil, "ports de destination, ex. 30120 ou 27015-27030")
	f.StringSliceVar(&sports, "sport", nil, "ports source")
	f.StringSliceVar(&sources, "sip", nil, "adresses ou réseaux source (obligatoires pour Allow List)")
	f.StringVar(&description, "description", "", "note libre")
	f.IntVar(&position, "position", 0, "position parmi les règles de l'IP (1 = en premier ; défaut : en dernier)")
	_ = create.RegisterFlagCompletionFunc("proto", fixedCompletion("tcp", "udp", "icmp", "gre"))

	// --- move ---------------------------------------------------------------
	var movePos int
	move := &cobra.Command{
		Use:     "move <id>",
		Short:   "Changer l'ordre d'une règle parmi celles de son IP",
		Example: `  yorkhost antiddos rules move r_123 --position 1`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if movePos < 1 {
				return usageErr(errors.New("--position requis (≥ 1, 1 = en premier)"))
			}
			var res map[string]any
			if _, err := a.Client().Post(ctx(cmd), path("antiddos", "rules", args[0], "move"), map[string]any{"position": movePos}, &res); err != nil {
				return err
			}
			if a.Printer.Machine() {
				return a.Printer.Raw(res)
			}
			rule, _ := res["rule"].(map[string]any)
			a.Printer.Success("Règle %s en position %s.", args[0], str(rule, "position"))
			return nil
		},
	}
	move.Flags().IntVar(&movePos, "position", 0, "nouvelle position (1 = évaluée en premier)")

	// --- delete -------------------------------------------------------------
	del := &cobra.Command{
		Use:     "delete <id>",
		Aliases: []string{"rm"},
		Short:   "Supprimer une règle",
		Example: `  yorkhost antiddos rules delete r_123
  yorkhost antiddos rules delete r_123 --yes`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			if err := a.Prompt.Confirm(fmt.Sprintf("Supprimer la règle anti-DDoS %s ? La protection qu'elle apporte sera retirée.", id), a.Yes); err != nil {
				return err
			}
			var res map[string]any
			if err := a.Client().Delete(ctx(cmd), path("antiddos", "rules", id), nil, &res); err != nil {
				return err
			}
			if a.Printer.Machine() {
				return a.Printer.Raw(res)
			}
			a.Printer.Success("Suppression de la règle %s demandée.", id)
			return nil
		},
	}

	cmd.AddCommand(ls, create, move, del)
	return cmd
}

// orNone : liste lisible, « aucune » si vide.
func orNone(l []string) string {
	if len(l) == 0 {
		return "aucune"
	}
	return strings.Join(l, ", ")
}

func bps(v any) string {
	f, ok := v.(float64)
	if !ok {
		return output.Text(v)
	}
	units := []string{"bit/s", "Kbit/s", "Mbit/s", "Gbit/s", "Tbit/s"}
	i := 0
	for f >= 1000 && i < len(units)-1 {
		f /= 1000
		i++
	}
	return strconv.FormatFloat(f, 'f', 1, 64) + " " + units[i]
}

func pps(v any) string {
	f, ok := v.(float64)
	if !ok {
		return output.Text(v)
	}
	switch {
	case f >= 1e6:
		return strconv.FormatFloat(f/1e6, 'f', 1, 64) + " Mpps"
	case f >= 1e3:
		return strconv.FormatFloat(f/1e3, 'f', 1, 64) + " Kpps"
	}
	return output.Number(f) + " pps"
}
