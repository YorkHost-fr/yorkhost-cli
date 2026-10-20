package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/YorkHost-fr/yorkhost-cli/internal/output"
	"github.com/YorkHost-fr/yorkhost-cli/internal/version"
)

var invoiceStatuses = []string{"paid", "unpaid", "cancelled", "refunded"}

func newInvoiceCmd(a *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "invoice",
		Aliases: []string{"invoices", "billing"},
		Short:   "Factures : liste, détail, PDF, paiement",
	}
	var status string
	var limit int
	ls := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "Lister les factures",
		Example: `  yorkhost invoice list --status unpaid`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			items, _, err := a.Client().ListAll(ctx(cmd), "/client/v1/invoices", nil, 0)
			if err != nil {
				return err
			}
			out := []map[string]any{}
			for _, it := range items {
				if status == "" || str(it, "status") == status {
					out = append(out, it)
				}
				if limit > 0 && len(out) >= limit {
					break
				}
			}
			return a.Printer.List(out, out, []output.Column{
				{Header: "ID", Path: "id"},
				{Header: "Numéro", Path: "number"},
				{Header: "Date", Path: "date"},
				{Header: "Échéance", Path: "dueDate"},
				{Header: "Total", Value: invoiceTotal},
				statusCol("Statut", "status"),
			})
		},
	}
	ls.Flags().StringVar(&status, "status", "", "filtrer : "+strings.Join(invoiceStatuses, ", "))
	ls.Flags().IntVar(&limit, "limit", 0, "nombre maximum de résultats")
	_ = ls.RegisterFlagCompletionFunc("status", fixedCompletion(invoiceStatuses...))

	get := &cobra.Command{
		Use:   "get <id>",
		Short: "Détail d'une facture et de ses lignes",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var m map[string]any
			if err := a.Client().Get(ctx(cmd), path("invoices", args[0]), nil, &m); err != nil {
				return err
			}
			if a.Printer.Machine() {
				return a.Printer.Raw(m)
			}
			if err := a.Printer.Object(m, m, []output.Field{
				{Header: "ID", Path: "id"},
				{Header: "Numéro", Path: "number"},
				{Header: "Date", Path: "date"},
				{Header: "Échéance", Path: "dueDate"},
				{Header: "Total", Value: invoiceTotal},
				{Header: "Statut", Path: "status", Color: output.Status},
			}); err != nil {
				return err
			}
			items := list(m, "items")
			if len(items) == 0 {
				return nil
			}
			fmt.Fprintln(a.Stdout)
			cur := str(m, "currency")
			return a.Printer.List(items, items, []output.Column{
				{Header: "Ligne", Path: "description", Max: 70},
				{Header: "Montant", Path: "amount", Format: output.Money(cur)},
			})
		},
	}

	var file string
	var openIt, force bool
	pdf := &cobra.Command{
		Use:   "pdf <id>",
		Short: "Télécharger le PDF d'une facture",
		Example: `  yorkhost invoice pdf 4242                 # → facture-4242.pdf
  yorkhost invoice pdf 4242 --file ~/compta/ --open`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var m map[string]any
			if err := a.Client().Get(ctx(cmd), path("invoices", args[0], "pdf"), nil, &m); err != nil {
				return err
			}
			u := str(m, "url")
			if u == "" {
				return errors.New("l'API n'a pas renvoyé de lien PDF")
			}
			dest := file
			if dest == "" {
				dest = "facture-" + args[0] + ".pdf"
			} else if st, err := os.Stat(dest); err == nil && st.IsDir() {
				dest = filepath.Join(dest, "facture-"+args[0]+".pdf")
			}
			if dest == "-" {
				return download(ctx(cmd), u, a.Stdout)
			}
			if err := downloadFile(ctx(cmd), u, dest, force); err != nil {
				return err
			}
			a.Printer.Success("Facture enregistrée : %s", dest)
			if a.Printer.Machine() {
				_ = a.Printer.Raw(map[string]any{"file": dest})
			}
			if openIt && a.OpenBrowser != nil {
				abs, _ := filepath.Abs(dest)
				_ = a.OpenBrowser("file://" + filepath.ToSlash(abs))
			}
			return nil
		},
	}
	pdf.Flags().StringVarP(&file, "file", "f", "", "fichier ou dossier de destination (- = stdout)")
	pdf.Flags().BoolVar(&force, "force", false, "écraser le fichier de destination s'il existe")
	pdf.Flags().BoolVar(&openIt, "open", false, "ouvrir le PDF après téléchargement")

	pay := &cobra.Command{
		Use:   "pay <id>",
		Short: "Ouvrir la page de paiement sécurisée d'une facture",
		Long: `Affiche le statut à jour de la facture et, si elle est impayée, ouvre la page
de paiement de l'espace client (carte avec 3-D Secure, PayPal, crédit du compte).
Aucun moyen de paiement ne transite par le CLI ni par l'API.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var m map[string]any
			if err := a.Client().Get(ctx(cmd), path("invoices", args[0], "payment-link"), nil, &m); err != nil {
				return err
			}
			if str(m, "status") != "unpaid" || str(m, "payment_url") == "" {
				if a.Printer.Machine() {
					return a.Printer.Raw(m)
				}
				a.Printer.Success("Facture %s : %s, rien à payer.", args[0], output.Status(str(m, "status")))
				return nil
			}
			a.Printer.Info("Facture %s : %s à régler.", args[0], output.Money(str(m, "currency"))(m["total"]))
			a.openURL(str(m, "payment_url"))
			return nil
		},
	}
	cmd.AddCommand(ls, get, pdf, pay)
	return cmd
}

func invoiceTotal(m map[string]any) any {
	if _, ok := m["total"].(float64); !ok {
		return nil
	}
	return output.Money(str(m, "currency"))(m["total"])
}

// download : lien signé (valable quelques minutes, sans jeton).
func download(c context.Context, u string, w io.Writer) error {
	req, err := http.NewRequestWithContext(c, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", version.UserAgent())
	res, err := (&http.Client{Timeout: 2 * time.Minute}).Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("téléchargement refusé (HTTP %d) : relancez la commande", res.StatusCode)
	}
	_, err = io.Copy(w, res.Body)
	return err
}

// downloadFile télécharge dans un fichier temporaire du même dossier, puis le
// renomme seulement quand le téléchargement et la fermeture ont réussi. Ainsi,
// une erreur réseau ne tronque jamais un fichier de destination existant.
func downloadFile(c context.Context, u, dest string, force bool) error {
	if !force {
		if _, err := os.Stat(dest); err == nil {
			return fmt.Errorf("%s existe déjà : utilisez --force pour l'écraser", dest)
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	dir := filepath.Dir(dest)
	tmp, err := os.CreateTemp(dir, ".yorkhost-invoice-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		return err
	}
	if err := download(c, u, tmp); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if !force {
		// Un lien dur publie le fichier sans fenêtre entre une vérification et
		// le renommage : l'opération échoue atomiquement si dest existe déjà.
		if err := os.Link(tmpName, dest); err != nil {
			if errors.Is(err, os.ErrExist) {
				return fmt.Errorf("%s existe déjà : utilisez --force pour l'écraser", dest)
			}
			return fmt.Errorf("finalisation de %s : %w", dest, err)
		}
		return nil
	}
	if err := os.Rename(tmpName, dest); err != nil {
		return fmt.Errorf("finalisation de %s : %w", dest, err)
	}
	return nil
}

// ------------------------------------------------------------ catalogue

var cycles = []string{"m", "q", "s", "a"}

func newCatalogCmd(a *App) *cobra.Command {
	var category string
	cmd := &cobra.Command{
		Use:     "catalog",
		Aliases: []string{"products"},
		Short:   "Produits commandables et tarifs",
		Long: `Produits commandables depuis le CLI (liste blanche), tarifs par cycle
(m = mensuel, q = trimestriel, s = semestriel, a = annuel) et plafond de dépense journalier.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			var m map[string]any
			if err := a.Client().Get(ctx(cmd), "/client/v1/catalog", nil, &m); err != nil {
				return err
			}
			products := list(m, "products")
			if category != "" {
				var keep []map[string]any
				for _, p := range products {
					if strings.Contains(strings.ToLower(str(p, "category")), strings.ToLower(category)) {
						keep = append(keep, p)
					}
				}
				products = keep
			}
			sort.SliceStable(products, func(i, j int) bool { return str(products[i], "category") < str(products[j], "category") })
			cur := str(m, "currency")
			priceCol := func(c, h string) output.Column {
				return output.Column{Header: h, Value: func(p map[string]any) any {
					prices, _ := p["prices"].(map[string]any)
					return prices[c]
				}, Format: output.Money(cur)}
			}
			if err := a.Printer.List(m, products, []output.Column{
				{Header: "ID", Path: "id"},
				{Header: "Produit", Path: "name", Max: 34},
				{Header: "Catégorie", Path: "category", Max: 24},
				priceCol("m", "Mensuel"),
				priceCol("q", "Trimestriel"),
				priceCol("s", "Semestriel"),
				priceCol("a", "Annuel"),
				{Header: "Stock", Path: "in_stock", Color: output.Status},
			}); err != nil || a.Printer.Machine() {
				return err
			}
			if capEUR, ok := m["daily_cap_eur"].(float64); ok {
				a.Printer.Info("\nPlafond de commande par API : %s par jour. Commander : yorkhost order create <id> --cycle m", output.Money(cur)(capEUR))
			}
			if n := str(m, "ordering_notice"); n != "" {
				a.Printer.Info("%s", n)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&category, "category", "", "filtrer par catégorie (contient)")
	return cmd
}

// ------------------------------------------------------------ commandes

func newOrderCmd(a *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "order",
		Short: "Commander un produit",
	}
	var cycle string
	var openPay bool
	create := &cobra.Command{
		Use:   "create <product-id>",
		Short: "Commander (devis, confirmation, facture)",
		Long: `Commande un produit du catalogue en deux temps, comme l'impose l'API :

  1. devis : prix, frais d'installation et montant dû maintenant (rien n'est commandé) ;
  2. confirmation explicite (ou --yes) : commande et facture créées.

La facture est réglée par le crédit du compte s'il suffit ; sinon le lien de
paiement sécurisé est affiché (et ouvert avec --pay).`,
		Example: `  yorkhost catalog
  yorkhost order create 42 --cycle a
  yorkhost order create 42 --yes --pay`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil || id <= 0 {
				return usageErr(fmt.Errorf("ID produit invalide %q (voir yorkhost catalog)", args[0]))
			}
			body := map[string]any{"product_id": id, "cycle": cycle}
			return a.order(cmd, "/client/v1/orders", body, openPay)
		},
	}
	create.Flags().StringVar(&cycle, "cycle", "m", "cycle de facturation : m, q, s, a")
	create.Flags().BoolVar(&openPay, "pay", false, "ouvrir la page de paiement si la facture est impayée")
	_ = create.RegisterFlagCompletionFunc("cycle", fixedCompletion(cycles...))
	cmd.AddCommand(create)
	return cmd
}

// order : préflight → confirmation → commande (orders, keep, claim).
func (a *App) order(cmd *cobra.Command, route string, body map[string]any, openPay bool) error {
	c := ctx(cmd)
	var quote map[string]any
	if _, err := a.Client().Post(c, route, body, &quote); err != nil {
		return err
	}
	token := str(quote, "confirmation_token")
	if token == "" {
		return errors.New("devis sans confirmation_token : commande impossible")
	}
	cur := str(quote, "currency")
	money := output.Money(cur)
	if !a.Printer.Machine() || !a.Yes {
		a.Printer.Info("Devis :")
		label := str(quote, "cycle_label")
		if label == "" {
			label = str(quote, "cycle")
		}
		a.Printer.Info("  Prix        %s (%s, renouvelable)", money(quote["price"]), label)
		if f := num(quote, "setup_fee"); f > 0 {
			a.Printer.Info("  Installation %s", money(f))
		}
		a.Printer.Info("  À payer     %s", money(quote["due_now"]))
		if msg := str(quote, "message"); msg != "" {
			a.Printer.Info("  %s", msg)
		}
	}
	if err := a.Prompt.Confirm(fmt.Sprintf("Confirmer la commande (%s maintenant) ?", money(quote["due_now"])), a.Yes); err != nil {
		return err
	}
	body["confirmation_token"] = token
	var res map[string]any
	if _, err := a.Client().Post(c, route, body, &res); err != nil {
		return err
	}
	if a.Printer.Machine() {
		return a.Printer.Raw(res)
	}
	a.Printer.Success("Commande %s passée — facture %s, %s (%s).", str(res, "order_id"), str(res, "invoice_id"),
		output.Money(str(res, "currency"))(res["total"]), output.Status(str(res, "invoice_status")))
	if w := str(res, "warning"); w != "" {
		a.Printer.Warn("%s", w)
	}
	if u := str(res, "contract_url"); u != "" {
		a.Printer.Info("Contrat : %s", u)
	}
	if u := str(res, "payment_url"); u != "" {
		if s := str(res, "next_step"); s != "" {
			a.Printer.Info("%s", s)
		}
		if openPay {
			a.openURL(u)
		} else {
			fmt.Fprintf(a.Stdout, "Paiement : %s\n", u)
		}
	}
	return nil
}
