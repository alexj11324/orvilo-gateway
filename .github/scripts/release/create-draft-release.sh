#!/usr/bin/env bash
set -euo pipefail

NOTES_FILE="docs/release_notes/${VERSION}.md"
if [[ ! -f "$NOTES_FILE" ]]; then
  echo "::error::Release notes file does not exist: $NOTES_FILE"
  exit 1
fi

git fetch --force --tags origin
EXISTING_TAG_SHA="$(git rev-parse --verify --quiet "refs/tags/$VERSION^{}" || true)"
if [[ -n "$EXISTING_TAG_SHA" ]]; then
  if [[ "$EXISTING_TAG_SHA" != "$TARGET_SHA" ]]; then
    echo "::error::tag $VERSION already exists but points to $EXISTING_TAG_SHA, not target $TARGET_SHA"
    exit 1
  fi
  echo "::notice::tag $VERSION already exists and points to the target commit, reusing it"
else
  git config user.name "github-actions[bot]"
  git config user.email "41898282+github-actions[bot]@users.noreply.github.com"
  git tag -a "$VERSION" "$TARGET_SHA" -m "Release $VERSION"
  git push origin "$VERSION"
fi

if RELEASE_JSON="$(gh release view "$VERSION" --json isDraft,isPrerelease 2>/dev/null)"; then
  RELEASE_DRAFT="$(jq -r .isDraft <<< "$RELEASE_JSON")"
  RELEASE_PRERELEASE="$(jq -r .isPrerelease <<< "$RELEASE_JSON")"
  if [[ "$RELEASE_DRAFT" != "true" ]]; then
    echo "::error::Release $VERSION is already published, cannot reuse"
    exit 1
  fi
  if [[ "$RELEASE_PRERELEASE" != "$PRERELEASE" ]]; then
    echo "::error::Draft release prerelease=$RELEASE_PRERELEASE does not match input prerelease=$PRERELEASE"
    exit 1
  fi
  echo "::notice::Draft release $VERSION already exists, reusing it"
else
  RELEASE_ARGS=(
    --draft
    --verify-tag
    --target "$TARGET_SHA"
    --title "Orvilo Gateway $VERSION"
    --notes-file "$NOTES_FILE"
  )
  if [[ "$PRERELEASE" == "true" ]]; then
    RELEASE_ARGS+=(--prerelease)
  fi
  gh release create "$VERSION" "${RELEASE_ARGS[@]}"
fi
