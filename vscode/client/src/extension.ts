import {
  workspace,
  ExtensionContext,
  window,
  env,
  Uri,
  commands,
} from "vscode";
import { exec } from "child_process";
import { promisify } from "util";

import {
  LanguageClient,
  LanguageClientOptions,
  ServerOptions,
} from "vscode-languageclient/node";

const execAsync = promisify(exec);

const installGuideUrl =
  "https://github.com/deepnoodle-ai/risor/tree/main/vscode#language-server";

let client: LanguageClient;

async function findLanguageServer(): Promise<string | undefined> {
  // Check if user has configured a custom path
  const config = workspace.getConfiguration("risor");
  const customPath = config.get<string>("languageServerPath");
  if (customPath && customPath.trim()) {
    console.log("Using custom risor-lsp path:", customPath);
    return customPath;
  }

  // Try to find risor-lsp in PATH (cross-platform)
  try {
    const command =
      process.platform === "win32" ? "where risor-lsp" : "which risor-lsp";
    const { stdout } = await execAsync(command);
    if (stdout.trim()) {
      console.log("Found risor-lsp in PATH:", stdout.trim());
      return "risor-lsp";
    }
  } catch (error) {
    console.log("risor-lsp not found in PATH");
  }

  return undefined;
}

async function showInstallHelp(): Promise<void> {
  const installGuide = "Installation Guide";
  const setPath = "Set Path";
  const choice = await window.showWarningMessage(
    "Risor Language Server (risor-lsp) was not found. Install it with Homebrew (brew install --cask deepnoodle-ai/risor/risor) or from the Risor GitHub release, then reload the window. Syntax highlighting works without it.",
    installGuide,
    setPath
  );
  if (choice === installGuide) {
    await env.openExternal(Uri.parse(installGuideUrl));
  } else if (choice === setPath) {
    await commands.executeCommand(
      "workbench.action.openSettings",
      "risor.languageServerPath"
    );
  }
}

export async function activate(context: ExtensionContext) {
  try {
    const serverCommand = await findLanguageServer();
    if (!serverCommand) {
      void showInstallHelp();
      return;
    }

    // The debug options for the server
    // --inspect=6009: runs the server in Node's Inspector mode so VS Code can attach to the server for debugging
    const debugOptions = { execArgv: ["--nolazy", "--inspect=6009"] };

    // If the extension is launched in debug mode then the debug server options are used
    // Otherwise the run options are used
    const serverOptions: ServerOptions = {
      run: {
        command: serverCommand,
        args: [],
        options: {},
      },
      debug: {
        command: serverCommand,
        args: [],
        options: {},
      },
    };

    // Options to control the language client
    const clientOptions: LanguageClientOptions = {
      // Register the server for Risor files
      documentSelector: [
        { scheme: "file", language: "risor" },
        { scheme: "file", pattern: "**/*.risor" },
        { scheme: "file", pattern: "**/*.rsr" },
      ],
      synchronize: {
        // Notify the server about file changes to Risor files
        fileEvents: workspace.createFileSystemWatcher("**/*.{risor,rsr}"),
      },
      // Enable detailed tracing to debug diagnostic issues
      traceOutputChannel: window.createOutputChannel(
        "Risor Language Server Trace",
        { log: true }
      ),
    };

    // Create the language client and start the client.
    client = new LanguageClient(
      "risorLanguageServer",
      "Risor Language Server",
      serverOptions,
      clientOptions
    );

    // Start the client. This will also launch the server
    client.start();
  } catch (error) {
    console.error("Failed to activate extension:", error);
    const message = error instanceof Error ? error.message : String(error);
    window.showErrorMessage(
      `Failed to activate Risor Language Server: ${message}`
    );
  }
}

export function deactivate(): Thenable<void> | undefined {
  if (!client) {
    return undefined;
  }
  return client.stop();
}
