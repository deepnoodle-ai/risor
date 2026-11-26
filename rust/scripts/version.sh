#!/usr/bin/env sh
# Set one SemVer version on the Rust workspace crates (`risor` and
# `risor-derive`) and keep the exact `risor-derive` dependency pin in sync.
#
#   ./rust/scripts/version.sh 3.0.0-alpha.2
#
# The release workflow publishes on the tag `rust-v<version>`, and checks
# that both crates and the `risor-derive` pin carry that version.

set -eu

VERSION="${1:-}"
SEMVER_RE='^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-((0|[1-9][0-9]*|[0-9]*[A-Za-z-][0-9A-Za-z-]*)(\.(0|[1-9][0-9]*|[0-9]*[A-Za-z-][0-9A-Za-z-]*))*))?(\+[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$'
if [ -z "$VERSION" ] || ! printf '%s\n' "$VERSION" | grep -Eq "$SEMVER_RE"; then
    echo "usage: $0 <semver>, for example 3.0.0-alpha.2" >&2
    exit 2
fi

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
RUST_DIR="$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)"
MANIFEST="$RUST_DIR/Cargo.toml"

tmp="$(mktemp "$MANIFEST.XXXXXX")"
trap 'rm -f "$tmp"' EXIT

awk -v ver="$VERSION" '
BEGIN { section = ""; updated_ws = 0; updated_dep = 0 }
/^\[/ { section = $0 }
{
    if (section == "[workspace.package]" && $0 ~ /^version = "/) {
        print "version = \"" ver "\""
        updated_ws++
        next
    }
    if (section == "[dependencies]" && $0 ~ /^risor-derive = \{/) {
        sub(/version = "=[0-9A-Za-z.+-]+"/, "version = \"=" ver "\"")
        print
        updated_dep++
        next
    }
    print
}
END {
    if (updated_ws != 1) {
        print "error: expected 1 [workspace.package] version line, updated " updated_ws > "/dev/stderr"
        exit 1
    }
    if (updated_dep != 1) {
        print "error: expected 1 risor-derive dependency pin, updated " updated_dep > "/dev/stderr"
        exit 1
    }
}
' "$MANIFEST" > "$tmp"

mv "$tmp" "$MANIFEST"

# Refresh Cargo.lock if present locally, and verify the workspace manifests.
if [ -f "$RUST_DIR/Cargo.lock" ]; then
    cargo update --manifest-path "$MANIFEST" --workspace --offline >/dev/null 2>&1 || \
        cargo generate-lockfile --manifest-path "$MANIFEST" >/dev/null
fi
cargo metadata --manifest-path "$MANIFEST" --no-deps --format-version 1 >/dev/null

echo "risor $VERSION"
echo "risor-derive $VERSION"
