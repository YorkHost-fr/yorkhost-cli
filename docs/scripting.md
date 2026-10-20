# Scripts et automatisation

Le CLI est pensé pour être scripté : sortie machine stable, codes de sortie documentés, aucune
question posée hors terminal.

## Formats de sortie

| `-o` | Usage |
|---|---|
| `table` (défaut) | lecture humaine, colonnes essentielles, textes longs tronqués |
| `wide` | toutes les colonnes, rien de tronqué |
| `json` | réponse de l'API telle quelle (indentée) |
| `yaml` | idem, en YAML |

Pour les listes paginées (`service`, `vps`, `game`, `invoice`, `ticket`), le CLI parcourt
toutes les pages et renvoie **un tableau JSON** des éléments (après filtres `--type`,
`--status`…). Les autres commandes renvoient l'objet de l'API.

```sh
# IDs des VPS actifs
yorkhost vps list -o json | jq -r '.[] | select(.status=="active") | .id'

# Total des factures impayées
yorkhost invoice list --status unpaid -o json | jq '[.[].total] | add'

# Adresse d'un serveur de jeu
yorkhost game get 5678 -o json | jq -r '"\(.address.ip):\(.address.port)"'

# Redémarrer tous les serveurs de jeu actifs
for id in $(yorkhost game list --status active -o json | jq -r '.[].id'); do
  yorkhost game restart "$id" --yes
done
```

**stdout** ne contient que le résultat ; progression, avertissements et erreurs vont sur
**stderr**. `--quiet` supprime les messages de progression.

## Confirmations

Les actions sensibles (`stop`, `restart`, `kill`, `sandbox delete`, `order create`, `keep`,
`claim`, `profile delete`) demandent `[o/N]`. Hors terminal, elles échouent avec le code `8`
sauf `--yes` (ou `YORKHOST_YES=1`) : un script ne reste jamais bloqué en attente d'une réponse.

## Codes de sortie

| Code | Signification |
|---|---|
| `0` | succès |
| `1` | erreur générique |
| `2` | usage : commande, option ou argument invalide (aussi les `400`/`422` de l'API) |
| `3` | authentification : non connecté, clé invalide/révoquée, A2F inactive, permission manquante, IP refusée |
| `4` | introuvable (`404`, ou nom de service inconnu) |
| `5` | conflit (`409`) : état incompatible, devis expiré ou déjà utilisé |
| `6` | limite de requêtes atteinte (`429`) après les nouvelles tentatives |
| `7` | API injoignable ou en erreur (`5xx`, réseau) |
| `8` | confirmation refusée ou impossible (sans `--yes`), ou délai `--wait` dépassé |
| `130` | interrompu (Ctrl-C) |

`yorkhost sandbox exec` renvoie le code de sortie de la commande exécutée (`124` si elle a
dépassé son délai) ; `yorkhost vps ssh` et `sandbox ssh` celui de `ssh`.

```sh
yorkhost vps get 1234 >/dev/null 2>&1
case $? in
  0) echo "ok" ;;
  3) echo "reconnectez-vous" ;;
  4) echo "VPS inconnu" ;;
esac
```

## Attendre la fin d'une action

```sh
yorkhost vps start 1234 --wait --timeout 3m
yorkhost game restart 5678 --wait
yorkhost sandbox create --wait -o json | jq -r .sandbox.id
```

## Entrées depuis un fichier ou stdin

Les messages acceptent `@fichier` ou `-` (stdin) :

```sh
yorkhost ticket create -s "Erreurs au démarrage" -b @erreurs.log --service 5678
journalctl -u app -n 100 | yorkhost ticket reply 10234 -b -
printf %s "$TOKEN" | yorkhost sandbox secret set GITHUB_TOKEN --stdin
echo 'print(2**10)' | yorkhost sandbox exec sbxabcdefghijkl -- python3
```

## Requêtes brutes : `yorkhost api`

Pour une route sans commande dédiée, ou pour explorer l'API avec vos identifiants :

> **Attention :** cette commande brute n'applique pas les confirmations de sécurité
> des commandes dédiées. Une requête `POST`, `PUT` ou `DELETE` est envoyée directement ;
> vérifiez la méthode, le chemin et les données avant de l'exécuter.

```sh
yorkhost api /account
yorkhost api /invoices --paginate
yorkhost api -X POST /game-servers/5678/power -F action=restart
yorkhost api -X POST /tickets -d @ticket.json
yorkhost api -i /services            # statut et en-têtes HTTP sur stderr
```

Le chemin est relatif à `/client/v1`. `-F clé=valeur` type les nombres et booléens ;
`-d` prend du JSON (texte, `@fichier` ou `-`).

## Limites de l'API

| Limite | Valeur |
|---|---|
| Lectures par clé | 120 / min |
| Écritures par clé | 20 / min |
| Alimentation (VPS, jeu) | 10 / min |
| Tickets | 5 créations, 10 réponses / min |
| Commandes | 5 / min, plafond de dépense journalier |
| Sandboxes | 3 créations / min, exécutions 60 / min, fichiers 120 / min |
| Par IP | 600 / min |

Le CLI attend et réessaie automatiquement sur un `429` (jusqu'à 30 s de `Retry-After`).
