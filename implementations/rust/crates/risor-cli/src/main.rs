//! Risor CLI - command-line interface for the Risor scripting language.

use std::collections::HashMap;
use std::env;
use std::fs;
use std::path::Path;

use risor::{eval_with_globals, create_builtins, Object};
use rustyline::error::ReadlineError;
use rustyline::DefaultEditor;

const VERSION: &str = "2.0.0";

fn main() {
    let args: Vec<String> = env::args().skip(1).collect();

    if let Err(e) = run(args) {
        eprintln!("Error: {}", e);
        std::process::exit(1);
    }
}

fn run(args: Vec<String>) -> Result<(), String> {
    let mut eval_code: Option<String> = None;
    let mut interactive = false;
    let mut file: Option<String> = None;

    let mut i = 0;
    while i < args.len() {
        let arg = &args[i];

        match arg.as_str() {
            "-h" | "--help" => {
                print_usage();
                return Ok(());
            }
            "-v" | "--version" => {
                print_version();
                return Ok(());
            }
            "-e" | "--eval" => {
                i += 1;
                if i >= args.len() {
                    return Err("-e requires an argument".to_string());
                }
                eval_code = Some(args[i].clone());
            }
            "-i" | "--interactive" => {
                interactive = true;
            }
            arg if arg.starts_with('-') => {
                return Err(format!("Unknown option: {}", arg));
            }
            _ => {
                file = Some(arg.clone());
                break;
            }
        }
        i += 1;
    }

    let builtins = create_builtins();

    if let Some(code) = eval_code {
        let result = eval_with_globals(&code, builtins.clone())
            .map_err(|e| e.to_string())?;

        if !matches!(result, Object::Nil) {
            println!("{}", result);
        }

        if interactive {
            start_repl(builtins)?;
        }
    } else if let Some(filepath) = file {
        let result = run_file(&filepath, builtins.clone())?;

        if !matches!(result, Object::Nil) {
            println!("{}", result);
        }

        if interactive {
            start_repl(builtins)?;
        }
    } else {
        start_repl(builtins)?;
    }

    Ok(())
}

fn print_usage() {
    println!(
        r#"
Risor v{} - Fast, embedded scripting language

Usage:
  risor [options] [file] [args...]

Options:
  -h, --help      Show this help message
  -v, --version   Show version
  -e, --eval      Evaluate code from command line
  -i, --interactive  Start REPL after running file

Examples:
  risor                    Start interactive REPL
  risor script.risor       Run a script file
  risor -e "print(1 + 2)"  Evaluate code
  risor -i script.risor    Run script then start REPL
"#,
        VERSION
    );
}

fn print_version() {
    println!("Risor {}", VERSION);
}

fn run_file(filepath: &str, builtins: HashMap<String, Object>) -> Result<Object, String> {
    let path = Path::new(filepath);

    if !path.exists() {
        return Err(format!("File not found: {}", filepath));
    }

    let code = fs::read_to_string(path)
        .map_err(|e| format!("Failed to read file: {}", e))?;

    eval_with_globals(&code, builtins)
        .map_err(|e| format!("Error in {}: {}", filepath, e))
}

fn start_repl(builtins: HashMap<String, Object>) -> Result<(), String> {
    println!("Risor v{} - Type 'exit' or Ctrl+D to quit", VERSION);
    println!();

    let mut rl = DefaultEditor::new()
        .map_err(|e| format!("Failed to create editor: {}", e))?;

    let mut buffer = String::new();

    loop {
        let prompt = if buffer.is_empty() { ">>> " } else { "... " };

        match rl.readline(prompt) {
            Ok(line) => {
                let trimmed = line.trim();

                // Handle exit commands
                if buffer.is_empty() && (trimmed == "exit" || trimmed == "quit") {
                    println!("Goodbye!");
                    break;
                }

                // Handle special commands
                if buffer.is_empty() && trimmed.starts_with('/') {
                    handle_command(trimmed);
                    continue;
                }

                // Accumulate input
                if !buffer.is_empty() {
                    buffer.push('\n');
                }
                buffer.push_str(&line);

                // Check if input is complete
                if is_complete(&buffer) {
                    if !buffer.trim().is_empty() {
                        rl.add_history_entry(buffer.trim())
                            .ok(); // Ignore history errors

                        match eval_with_globals(&buffer, builtins.clone()) {
                            Ok(result) => {
                                if !matches!(result, Object::Nil) {
                                    println!("{}", result);
                                }
                            }
                            Err(e) => {
                                eprintln!("Error: {}", e);
                            }
                        }
                    }
                    buffer.clear();
                }
            }
            Err(ReadlineError::Interrupted) => {
                buffer.clear();
                println!("^C");
            }
            Err(ReadlineError::Eof) => {
                println!("\nGoodbye!");
                break;
            }
            Err(e) => {
                return Err(format!("Readline error: {}", e));
            }
        }
    }

    Ok(())
}

/// Check if the input is syntactically complete.
fn is_complete(input: &str) -> bool {
    let mut braces = 0;
    let mut brackets = 0;
    let mut parens = 0;
    let mut in_string = false;
    let mut string_char = '\0';
    let mut prev_char = '\0';

    for c in input.chars() {
        if in_string {
            if c == string_char && prev_char != '\\' {
                in_string = false;
            }
            prev_char = c;
            continue;
        }

        match c {
            '"' | '\'' | '`' => {
                in_string = true;
                string_char = c;
            }
            '{' => braces += 1,
            '}' => braces -= 1,
            '[' => brackets += 1,
            ']' => brackets -= 1,
            '(' => parens += 1,
            ')' => parens -= 1,
            _ => {}
        }
        prev_char = c;
    }

    braces == 0 && brackets == 0 && parens == 0 && !in_string
}

fn handle_command(cmd: &str) {
    let parts: Vec<&str> = cmd[1..].split_whitespace().collect();
    let command = parts.first().map(|s| s.to_lowercase()).unwrap_or_default();

    match command.as_str() {
        "help" => {
            println!(
                r#"
REPL Commands:
  /help     Show this help
  /clear    Clear the screen
  /exit     Exit the REPL
"#
            );
        }
        "clear" => {
            // ANSI escape code to clear screen
            print!("\x1B[2J\x1B[1;1H");
        }
        "exit" | "quit" => {
            std::process::exit(0);
        }
        _ => {
            println!("Unknown command: /{}. Type /help for available commands.", command);
        }
    }
}
