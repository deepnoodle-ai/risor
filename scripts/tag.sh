#!/usr/bin/env bash

set -euo pipefail

VERSION=${1:-}

if [[ $# -ne 1 || ! "$VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+([.-][0-9A-Za-z.-]+)?$ ]]; then
    echo "Usage: scripts/tag.sh v<major>.<minor>.<patch>" >&2
    exit 2
fi

if [[ -n "$(git status --porcelain)" ]]; then
    echo "Refusing to tag a dirty worktree." >&2
    exit 1
fi

git fetch origin main --prune --tags

if [[ "$(git branch --show-current)" != "main" ]]; then
    echo "Refusing to tag outside the main branch." >&2
    exit 1
fi

if [[ "$(git rev-parse HEAD)" != "$(git rev-parse origin/main)" ]]; then
    echo "Refusing to tag: local main is not exactly origin/main." >&2
    exit 1
fi

RELEASE_VERSION=${VERSION#v}

if ! git grep --quiet --fixed-strings "## [$RELEASE_VERSION]" -- CHANGELOG.md; then
    echo "Refusing to tag: CHANGELOG.md has no $RELEASE_VERSION section." >&2
    exit 1
fi

if ! git grep --quiet --fixed-strings "const Version = \"$RELEASE_VERSION\"" -- pkg/docs/docs.go; then
    echo "Refusing to tag: pkg/docs.Version is not $RELEASE_VERSION." >&2
    exit 1
fi

MODULE_MISMATCHES=$(git grep -n \
    'github.com/deepnoodle-ai/risor/v2 v' -- '**/go.mod' \
    | grep -v " v$RELEASE_VERSION$" || true)
if [[ -n "$MODULE_MISMATCHES" ]]; then
    echo "Refusing to tag: nested modules do not all require v$RELEASE_VERSION:" >&2
    echo "$MODULE_MISMATCHES" >&2
    exit 1
fi

if git rev-parse --verify --quiet "refs/tags/$VERSION" >/dev/null; then
    echo "Refusing to overwrite existing tag $VERSION." >&2
    exit 1
fi

git tag "$VERSION"
git push origin "refs/tags/$VERSION"
