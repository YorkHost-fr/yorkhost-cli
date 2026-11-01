package cli

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/YorkHost-fr/yorkhost-cli/internal/output"
)

func accountFields() []output.Field {
	return []output.Field{
		{Header: "Client", Path: "clientId"},
		{Header: "Nom", Value: func(m map[string]any) any {
			return strings.TrimSpace(str(m, "firstName") + " " + str(m, "lastName"))
		}},
		{Header: "Société", Path: "company"},
		{Header: "E-mail", Path: "email"},
		{Header: "Crédit", Path: "credit", Format: output.Money("EUR")},
		{Header: "Factures impayées", Path: "unpaidInvoices", Color: func(s string) string {
			if s != "0" {
				return output.Yellow(s)
			}
			return s
		}},
	}
}

func newAccountCmd(a *App) *cobra.Command {
	cmd := &cobra.Command{Use: "account", Short: "Profil du compte (e-mail, crédit, impayés)"}
	get := &cobra.Command{
		Use:   "get",
		Short: "Afficher le profil du compte",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			var m map[string]any
			if err := a.Client().Get(ctx(cmd), "/client/v1/account", nil, &m); err != nil {
				return err
			}
			return a.Printer.Object(m, m, accountFields())
		},
	}
	cmd.AddCommand(get)
	cmd.RunE = get.RunE
	return cmd
}

var serviceTypes = []string{"vps", "dedicated", "game", "other"}
var serviceStatuses = []string{"active", "suspended", "pending", "expired"}

func serviceColumns() []output.Column {
	return []output.Column{
		{Header: "ID", Path: "id"},
		{Header: "Nom", Path: "name", Max: 36},
		{Header: "Type", Path: "type"},
		statusCol(),
		{Header: "Produit", Path: "productName", Max: 30},
		{Header: "Échéance", Path: "nextDueDate"},
		{Header: "Prix", Value: price, Wide: true},
		{Header: "Panel", Path: "panel", Wide: true},
	}
}

func price(m map[string]any) any {
	amount, ok := m["recurringAmount"].(float64)
	if !ok {
		return nil
	}
	s := output.Money("EUR")(amount)
	if c := str(m, "billingCycle"); c != "" {
		s += " / " + c
	}
	return s
}

// listServices : services filtrés (type, statut) — filtrage local, l'API ne
// filtre pas.
func (a *App) listServices(cmd *cobra.Command, kind, status string, limit int) error {
	items, _, err := a.Client().ListAll(ctx(cmd), "/client/v1/services", nil, 0)
	if err != nil {
		return err
	}
	var out []map[string]any
	for _, it := range items {
		if kind != "" && str(it, "type") != kind {
			continue
		}
		if status != "" && str(it, "status") != status {
			continue
		}
		out = append(out, it)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	if out == nil {
		out = []map[string]any{}
	}
	return a.Printer.List(out, out, serviceColumns())
}

func newServiceCmd(a *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "service",
		Aliases: []string{"services", "svc"},
		Short:   "Tous vos services (VPS, dédiés, jeux, web…)",
	}
	var kind, status string
	var limit int
	ls := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "Lister les services",
		Example: `  yorkhost service list
  yorkhost service list --type game --status active
  yorkhost service list -o json | jq -r '.[] | select(.status=="suspended") | .id'`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error { return a.listServices(cmd, kind, status, limit) },
	}
	ls.Flags().StringVar(&kind, "type", "", "filtrer par type : "+strings.Join(serviceTypes, ", "))
	ls.Flags().StringVar(&status, "status", "", "filtrer par statut : "+strings.Join(serviceStatuses, ", "))
	ls.Flags().IntVar(&limit, "limit", 0, "nombre maximum de résultats")
	_ = ls.RegisterFlagCompletionFunc("type", fixedCompletion(serviceTypes...))
	_ = ls.RegisterFlagCompletionFunc("status", fixedCompletion(serviceStatuses...))

	get := &cobra.Command{
		Use:               "get <id|nom>",
		Short:             "Détail d'un service",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: a.serviceCompletion(""),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := a.resolveService(ctx(cmd), args, "")
			if err != nil {
				return err
			}
			var m map[string]any
			if err := a.Client().Get(ctx(cmd), path("services", id), nil, &m); err != nil {
				return err
			}
			return a.Printer.Object(m, m, []output.Field{
				{Header: "ID", Path: "id"},
				{Header: "Nom", Path: "name"},
				{Header: "Type", Path: "type"},
				{Header: "Statut", Path: "status", Color: output.Status},
				{Header: "Produit", Path: "productName"},
				{Header: "Panel", Path: "panel"},
				{Header: "Prix", Value: price},
				{Header: "Échéance", Path: "nextDueDate"},
				{Header: "Prochaine facture", Path: "nextInvoiceDate"},
			})
		},
	}
	cmd.AddCommand(ls, get)
	return cmd
}
