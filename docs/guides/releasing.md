# Releasing Risor

## Prerequisites

- `goreleaser` installed (`brew install goreleaser`)
- `GITHUB_TOKEN` env var with repo access (used by GoReleaser to create the
  GitHub release and push the Homebrew formula)
- Docker with `buildx` (only needed for Docker image publishing)

## Release Process

### 1. Tag the release

```bash
git tag v2.x.x
git push origin v2.x.x
```

### 2. Run GoReleaser

```bash
make release
```

This runs `goreleaser release --clean -p 2`, which:

- Builds cross-platform binaries (linux/darwin amd64+arm64, windows amd64)
- Creates tar.gz archives (zip for windows) with checksums
- Creates the GitHub release at `deepnoodle-ai/risor`
- Updates the Homebrew formula at `deepnoodle-ai/homebrew-risor`

Configuration: `.goreleaser.yaml`

### 3. Publish Docker images (optional)

```bash
make docker-build
```

This pushes multi-arch images (amd64, arm64) to Docker Hub:

- `deepnoodle/risor:latest`
- `deepnoodle/risor:<version>` (derived from the git tag)
- `deepnoodle/risor:<git-revision>`

The version is derived automatically from `git describe --tags`.

## Homebrew

The Homebrew tap is at
[deepnoodle-ai/homebrew-risor](https://github.com/deepnoodle-ai/homebrew-risor).

GoReleaser auto-updates `Formula/risor.rb` on each release. Users install with:

```bash
brew install deepnoodle-ai/risor/risor
```

### Versioned formulas

When a major version introduces breaking changes, preserve the previous version
as a versioned formula before releasing:

1. Clone the tap: `git clone https://github.com/deepnoodle-ai/homebrew-risor.git`
2. Copy `Formula/risor.rb` to `Formula/risor@<major>.rb`
3. Rename the class from `Risor` to `RisorAT<major>` (Homebrew convention)
4. Commit and push the versioned formula
5. Return to the main risor repo, tag the new release, and run `make release`
   (GoReleaser will overwrite `risor.rb` with the new version)

Users can pin to the old version:

```bash
brew install deepnoodle-ai/risor/risor@1
```

The `risor@1` formula was created for the v1-to-v2 transition. Its download URLs
point to the `risor-io/risor` GitHub org (the original home of the project). The
main `risor` formula points to `deepnoodle-ai/risor`.
