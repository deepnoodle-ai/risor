#!/usr/bin/env sh
# Sync the suite JSON tables and root LICENSE into the Rust crates so
# `cargo package` and `cargo publish` produce self-contained crate archives.
#
#   ./rust/scripts/sync-data.sh          # copy from suite/ and LICENSE
#   ./rust/scripts/sync-data.sh --check  # fail if any bundled copy differs

set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
RUST_DIR="$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)"
REPO_DIR="$(CDPATH= cd -- "$RUST_DIR/.." && pwd)"

CHECK=0
if [ "${1:-}" = "--check" ]; then
    CHECK=1
elif [ "$#" -gt 0 ]; then
    echo "usage: $0 [--check]" >&2
    exit 2
fi

FILES="
casemap.json
foreign-names.json
library.json
library-groups.json
library-math.json
library-queries.json
library-recursion.json
messages.json
messages-check.json
messages-entry-points.json
messages-feedback.json
messages-parse.json
messages-queries.json
messages-recursion.json
"

mkdir -p "$RUST_DIR/data"

drift=0
for name in $FILES; do
    src="$REPO_DIR/suite/$name"
    dst="$RUST_DIR/data/$name"
    if [ ! -f "$src" ]; then
        echo "missing source file: $src" >&2
        exit 1
    fi
    if [ "$CHECK" -eq 1 ]; then
        if [ ! -f "$dst" ] || ! cmp -s "$src" "$dst"; then
            echo "out of sync: rust/data/$name differs from suite/$name" >&2
            drift=1
        fi
    else
        cp "$src" "$dst"
    fi
done

for dst in "$RUST_DIR/LICENSE" "$RUST_DIR/derive/LICENSE"; do
    src="$REPO_DIR/LICENSE"
    if [ "$CHECK" -eq 1 ]; then
        if [ ! -f "$dst" ] || ! cmp -s "$src" "$dst"; then
            echo "out of sync: $dst differs from LICENSE" >&2
            drift=1
        fi
    else
        cp "$src" "$dst"
    fi
done

if [ "$CHECK" -eq 1 ]; then
    for path in "$RUST_DIR"/data/*; do
        [ -e "$path" ] || continue
        base="$(basename -- "$path")"
        found=0
        for name in $FILES; do
            if [ "$name" = "$base" ]; then
                found=1
                break
            fi
        done
        if [ "$found" -eq 0 ]; then
            echo "unexpected file in rust/data/: $base" >&2
            drift=1
        fi
    done
    if [ "$drift" -ne 0 ]; then
        echo "run ./rust/scripts/sync-data.sh to update bundled Rust crate data" >&2
        exit 1
    fi
fi
