# Risor Extension for Visual Studio Code

A [Visual Studio Code](https://code.visualstudio.com/) extension for the
[Risor language](https://github.com/deepnoodle-ai/risor). It provides syntax
highlighting and connects to the Risor language server for editor features.

## Quick start

1. [Install the Risor language server](#language-server).
2. [Install the Risor extension for Visual Studio Code](https://marketplace.visualstudio.com/items?itemName=CurtisMyzie.risor-language).
3. Open or create a Risor file (with a `.risor` or `.rsr` extension) and get to work!

## Language server

The extension runs `risor-lsp`, the Risor language server. It ships with the
`risor` CLI in each Risor release.

- **Homebrew (macOS and Linux):** `brew install --cask deepnoodle-ai/risor/risor`
- **GitHub release:** download the archive for your platform from
  [Risor releases](https://github.com/deepnoodle-ai/risor/releases) and put
  `risor-lsp` on your `PATH`.
- **From source:** clone the repository and run `go install ./cmd/risor-lsp`.

The extension looks for `risor-lsp` on your `PATH`. To use a binary somewhere
else, set `risor.languageServerPath` in your settings. Reload the window after
you install the server.

## Feature details

- Syntax highlighting (works without the language server)
- Diagnostics
- Completion
- Hover
- Go to definition
- Document symbols
- Formatting

## Questions, Issues, and Feature Requests

- Ask questions in the [Risor GitHub discussions](https://github.com/deepnoodle-ai/risor/discussions).
- Open issues [in GitHub](https://github.com/deepnoodle-ai/risor/issues) or leave a
  👍 reaction if one already exists for your request.

## Contributions

All feedback is welcome 🙏 and please reach out to chat about any contributions
you have in mind.

## Data and Telemetry

No data or telemetry are collected.
