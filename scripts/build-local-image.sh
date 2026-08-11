#!/usr/bin/env bash
# Baut das lokale excalidraw-complete-Image (nativ arm64) aus dem Working Tree.
# Voraussetzung: excalidraw/excalidraw-app/build existiert (yarn build:app:docker).
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BUILD_DIR="$REPO_ROOT/excalidraw/excalidraw-app/build"
TAG="${1:-excalidraw-complete:local-explorer}"

[[ -f "$BUILD_DIR/index.html" ]] || {
  echo "FEHLER: $BUILD_DIR fehlt oder ist unvollständig — erst Frontend bauen." >&2
  exit 1
}

# Frontend-Artefakte in das go:embed-Verzeichnis spiegeln
rsync -a --delete --exclude=.keep "$BUILD_DIR/" "$REPO_ROOT/frontend/"

docker build -f "$REPO_ROOT/excalidraw-complete.Dockerfile" -t "$TAG" "$REPO_ROOT"
echo "Image gebaut: $TAG"
