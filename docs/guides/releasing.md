# Releasing Risor

## Prerequisites

- `goreleaser` installed (`brew install goreleaser`)
- `GITHUB_TOKEN` env var with repo access (used by GoReleaser to create the
  GitHub release and push the Homebrew cask)
- Docker with `buildx` (only needed for Docker image publishing)

## Release Process

### 1. Prepare and verify the release

Choose the next semantic version, then prepare a pull request that:

- Moves the accumulated `CHANGELOG.md` entries from `Unreleased` into a dated
  version section.
- Updates `pkg/docs.Version` and any nested `go.mod` references to the new
  version.

Run the full release gate from a clean worktree:

```bash
make test
make generate
make format
git diff --exit-code
make release-snapshot
git diff --exit-code
```

`release-snapshot` exercises the GoReleaser configuration and builds the same
archives locally without publishing them. Confirm the snapshot binary reports a
generated version rather than `dev`:

```bash
./dist/standard_darwin_arm64_v8.0/risor version
```

The exact snapshot path varies by operating system and architecture. Merge the
release-preparation pull request and wait for `main` CI to pass before tagging.

### 2. Tag the exact release commit

Synchronize a clean local `main`, then use the checked-in helper:

```bash
git switch main
git pull --ff-only origin main
scripts/tag.sh v2.x.x
```

The helper refuses dirty worktrees, non-`main` branches, commits that differ
from `origin/main`, and existing tags. Risor v2 publishes one repository tag;
the CLI archives are built from `cmd/risor` by GoReleaser.

### 3. Run GoReleaser

```bash
make release
```

This runs `goreleaser release --clean -p 2`, which:

- Builds cross-platform binaries (linux/darwin amd64+arm64, windows amd64)
- Creates tar.gz archives (zip for windows) with checksums
- Creates the GitHub release at `deepnoodle-ai/risor`
- Updates the Homebrew cask at `deepnoodle-ai/homebrew-risor`

Configuration: `.goreleaser.yaml`

Verify the published GitHub release is neither a draft nor a prerelease, its
archives match `checksums.txt`, an extracted binary reports the release version,
and the Homebrew cask points to the new tag and checksums.

### 4. Publish Docker images (optional)

```bash
make docker-build
```

This pushes multi-arch images (amd64, arm64) to Docker Hub:

- `deepnoodle/risor:latest`
- `deepnoodle/risor:<version>` (derived from the git tag)
- `deepnoodle/risor:<git-revision>`

The version is derived automatically from `git describe --tags`.

After publishing, inspect the `latest`, version, and git-revision manifests on
Docker Hub and confirm that both amd64 and arm64 images are present.

## Homebrew

The Homebrew tap is at
[deepnoodle-ai/homebrew-risor](https://github.com/deepnoodle-ai/homebrew-risor).

GoReleaser auto-updates `Casks/risor.rb` on each release. Users install with:

```bash
brew install --cask deepnoodle-ai/risor/risor
```

The v2.2.0 release is the first cask-based release. After its cask is published
and verified, remove the obsolete unversioned `Formula/risor.rb` from the tap so
plain `brew install` cannot select the stale v2.1.0 formula. Preserve
`Formula/risor@1.rb` for v1 users.

### Preserving a previous major version

Before a future breaking major release, copy the current `Casks/risor.rb` to a
versioned cask such as `Casks/risor@2.rb`, change its token to `risor@2`, and
commit it to the tap before GoReleaser replaces the unversioned cask.

Users can pin to the old version:

```bash
brew install deepnoodle-ai/risor/risor@1
```

The `risor@1` formula was created for the v1-to-v2 transition. Its download URLs
point to the `risor-io/risor` GitHub org (the original home of the project). The
stable `risor` cask points to `deepnoodle-ai/risor`.

## VS Code extension

The VS Code extension has its own version and is not published by `make
release`. To release it, bump the version in `vscode/package.json`, compile and
package it, then publish it separately with `make extension-publish`. Do not run
that target as part of a Risor library release unless an extension release is
intended.
