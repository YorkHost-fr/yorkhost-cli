# Publier une version

Les versions suivent le [versionnage sémantique](https://semver.org/lang/fr/) : `vMAJEUR.MINEUR.CORRECTIF`.
Une commande ou une option retirée, un format de sortie ou un code de sortie modifié = version majeure.

## Étapes

1. Facultatif : compléter `CHANGELOG.md` (les notes de la release GitHub sont, elles, générées automatiquement à partir des commits).
2. Vérifier en local :

   ```sh
   make lint test docs-check
   make snapshot          # goreleaser à blanc → dist/
   ```

3. Publier : *Actions → Release → Run workflow*, sans rien remplir.

   La prochaine version est calculée d'après le dernier tag et les messages de commit
   ([`scripts/next-version.sh`](../scripts/next-version.sh)) :

   | Commits depuis la dernière version | Version |
   |---|---|
   | seulement `fix:`, `docs:`, `chore:`… | corrective (`v0.1.0` → `v0.1.1`) |
   | au moins un `feat:` | mineure (`v0.1.1` → `v0.2.0`) |
   | un `feat!:` ou `BREAKING CHANGE` | majeure (`v1.2.3` → `v2.0.0`) ; en 0.x, mineure |

   Le champ *Type de version* force `patch`, `minor` ou `major` (ex. passage en `v1.0.0`),
   et *Version exacte* impose un numéro (pré-release comprise, `v1.1.0-rc.1`).
   Le workflow crée le tag sur `main` puis publie. Sans aucun commit depuis la dernière
   version, il s'arrête sans rien publier.

   On peut aussi taguer à la main : `git tag -a v1.0.0 -m v1.0.0 && git push origin v1.0.0`.

Le workflow [`release.yml`](../.github/workflows/release.yml) lance les tests puis GoReleaser,
qui publie sur la release GitHub :

- les binaires Linux, macOS, Windows, FreeBSD (amd64, arm64, armv7) en `.tar.gz` / `.zip`,
  avec README, licence, autocomplétions et pages `man` ;
- les paquets `.deb`, `.rpm` et `.apk` ;
- `checksums.txt` (SHA-256), utilisé par les scripts d'installation ;
- un changelog groupé à partir des messages de commit (`feat:`, `fix:`…).

Un tag contenant un suffixe (`v1.1.0-rc.1`) est publié comme *pre-release* et n'est pas pris
par `install.sh` (qui suit la dernière release stable).

## Version dans le binaire

GoReleaser injecte version, commit et date (`-ldflags -X …/internal/version.*`).
`go install …@v1.0.0` sans ldflags lit la version du module. `yorkhost version` affiche le tout.
