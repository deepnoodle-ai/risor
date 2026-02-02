import { workspace, ExtensionContext, window } from "vscode";
import { exec } from "child_process";
import { promisify } from "util";

import {
  LanguageClient,
  LanguageClientOptions,
  ServerOptions,
} from "vscode-languageclient/node";

const execAsync = promisify(exec);

let client: LanguageClient;

async function findOrInstallLanguageServer(
  context: ExtensionContext
): Promise<string> {
  // Check if user has configured a custom path
  const config = workspace.getConfiguration("risor");
  const customPath = config.get<string>("languageServerPath");
  if (customPath && customPath.trim()) {
    console.log("Using custom risor-lsp path:", customPath);
    return customPath;
  }

  // First, try to find risor-lsp in PATH (cross-platform)
  try {
    const command =
      process.platform === "win32" ? "where risor-lsp" : "which risor-lsp";
    const { stdout } = await execAsync(command);
    if (stdout.trim()) {
      console.log("Found risor-lsp in PATH:", stdout.trim());
      return "risor-lsp";
    }
  } catch (error) {
    console.log("risor-lsp not found in PATH, attempting installation...");
  }

  // Check if go is available
  try {
    await execAsync("go version");
  } catch (error) {
    window.showErrorMessage(
      "Risor Language Server requires Go to be installed. Please install Go and try again, or install risor-lsp manually."
    );
    throw new Error("Go not found");
  }

  // Try to install via go install
  try {
    window.showInformationMessage("Installing Risor Language Server...");
    await execAsync(
      "go install github.com/deepnoodle-ai/risor/v2/cmd/risor-lsp@v1.8.1"
    );

    // Verify installation
    const verifyCommand =
      process.platform === "win32" ? "where risor-lsp" : "which risor-lsp";
    const { stdout } = await execAsync(verifyCommand);
    if (stdout.trim()) {
      window.showInformationMessage(
        "Risor Language Server installed successfully!"
      );
      return "risor-lsp";
    }
  } catch (error) {
    console.error("Failed to install risor-lsp via go install:", error);
    window.showErrorMessage(
      "Failed to install Risor Language Server automatically. Please run: go install github.com/deepnoodle-ai/risor/v2/cmd/risor-lsp@v1.8.1"
    );
    throw error;
  }

  throw new Error("Could not find or install risor-lsp");
}

export async function activate(context: ExtensionContext) {
  try {
    const serverCommand = await findOrInstallLanguageServer(context);

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
      traceOutputChannel: {
        name: "Risor Language Server Trace",
        // Implementation of OutputChannel interface for console logging
        append: (value: string) => console.log("[LSP TRACE]", value),
        appendLine: (value: string) => console.log("[LSP TRACE]", value),
        clear: () => console.log("[LSP TRACE] CLEARED"),
        show: () => { },
        hide: () => { },
        dispose: () => { },
        replace: (value: string) => console.log("[LSP TRACE REPLACE]", value),
      },
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
    window.showErrorMessage(
      `Failed to activate Risor Language Server: ${error.message}`
    );
  }
}

export function deactivate(): Thenable<void> | undefined {
  if (!client) {
    return undefined;
  }
  return client.stop();
}
