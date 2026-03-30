#!/usr/bin/env bash
set -euo pipefail

kind="${1:-}"

if [[ "$kind" != "patch" && "$kind" != "minor" ]]; then
  echo "Usage: ./scripts/release.sh [patch|minor]"
  exit 1
fi

git fetch --tags

latest="$(git tag --list 'v0.*' --sort=-version:refname | head -n1)"

if [[ -z "$latest" ]]; then
  latest="v0.1.0"
  echo "No existing v0 tag found. Starting from $latest"
fi

version="${latest#v}"
IFS='.' read -r major minor patch <<< "$version"

if [[ "$kind" == "patch" ]]; then
  patch=$((patch + 1))
else
  minor=$((minor + 1))
  patch=0
fi

next="v${major}.${minor}.${patch}"

echo "Next version: $next"
read -r -p "Create and push tag $next? [y/N] " confirm
if [[ "$confirm" != "y" && "$confirm" != "Y" ]]; then
  echo "Cancelled."
  exit 1
fi

git checkout main
git pull --ff-only
git tag "$next"
git push origin main
git push origin "$next"

echo "Released $next"
