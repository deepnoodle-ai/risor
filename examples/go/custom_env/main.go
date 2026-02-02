package main

import (
	"context"
	"fmt"
	"log"

	"github.com/deepnoodle-ai/risor/v2"
)

func main() {
	ctx := context.Background()

	// Start with standard builtins
	env := risor.Builtins()

	// Add custom data - maps, slices, and primitives
	env["config"] = map[string]any{
		"version":  "1.0.0",
		"debug":    true,
		"maxItems": 100,
		"apiUrl":   "https://api.example.com",
	}

	env["users"] = []map[string]any{
		{"name": "Alice", "role": "admin", "active": true},
		{"name": "Bob", "role": "user", "active": true},
		{"name": "Charlie", "role": "user", "active": false},
	}

	env["multiplier"] = 10
	env["appName"] = "MyApp"

	script := `
		// Access custom data
		let activeUsers = users.filter(u => u.active)
		let admins = activeUsers.filter(u => u.role == "admin")

		// Use custom primitives with built-in operations
		let scaled = [1, 2, 3, 4, 5].map(x => x * multiplier)

		// String operations
		let greeting = sprintf("Welcome to %s v%s", appName, config.version)

		{
			greeting: greeting,
			scaled: scaled,
			activeCount: len(activeUsers),
			adminNames: admins.map(u => u.name),
			apiUrl: config.apiUrl,
			debug: config.debug
		}
	`

	result, err := risor.Eval(ctx, script, risor.WithEnv(env))
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Result:", result)
}
