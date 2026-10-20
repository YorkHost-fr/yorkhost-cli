#!/bin/sh
# Prochaine version à publier, d'après le dernier tag vX.Y.Z et les messages
# de commit depuis (Conventional Commits) :
#
#   feat!: / BREAKING CHANGE   → majeure (mineure tant qu'on est en 0.x)
#   feat:                      → mineure
#   tout le reste              → corrective
#
# Choisir explicitement « major » force la majeure (ex. passage en v1.0.0).
#
# Usage : scripts/next-version.sh [auto|patch|minor|major]   (défaut : auto)
# Affiche la version (ex. v0.1.1) sur stdout, le raisonnement sur stderr.
set -eu

bump=${1:-auto}
requested=$bump

last=$(git tag -l 'v[0-9]*.[0-9]*.[0-9]*' --sort=-v:refname | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' | head -n1 || true)
if [ -z "$last" ]; then
	last=v0.0.0
	range=HEAD
else
	range="$last..HEAD"
fi

if [ "$last" != v0.0.0 ] && [ -z "$(git rev-list "$range")" ]; then
	echo "Aucun commit depuis $last : rien à publier." >&2
	exit 1
fi

if [ "$bump" = auto ]; then
	log=$(git log --format='%s%n%b' "$range")
	if printf '%s\n' "$log" | grep -Eq '^[a-z]+(\([^)]*\))?!:|^BREAKING[ -]CHANGE'; then
		bump=major
	elif printf '%s\n' "$log" | grep -Eq '^feat(\([^)]*\))?:'; then
		bump=minor
	else
		bump=patch
	fi
	echo "Changements depuis $last : $bump (déduit des commits)" >&2
fi

ver=${last#v}
major=${ver%%.*}
rest=${ver#*.}
minor=${rest%%.*}
patch=${rest#*.}

# En 0.x, l'API n'est pas stable (SemVer §4) : un changement cassant déduit
# des commits monte la mineure ; seul un choix explicite passe en 1.0.0.
if [ "$major" -eq 0 ] && [ "$bump" = major ] && [ "$requested" = auto ]; then
	bump=minor
fi

case $bump in
	major) major=$((major + 1)); minor=0; patch=0 ;;
	minor) minor=$((minor + 1)); patch=0 ;;
	patch) patch=$((patch + 1)) ;;
	*) echo "Type de version inconnu : $bump (auto, patch, minor, major)" >&2; exit 2 ;;
esac

echo "v$major.$minor.$patch"
