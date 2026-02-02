package main

import (
	"context"
	"fmt"
	"log"

	"github.com/deepnoodle-ai/risor/v2"
)

// Product represents an item with dynamic pricing rules
type Product struct {
	Name      string
	BasePrice float64
	Category  string
}

func main() {
	ctx := context.Background()

	// Products to evaluate
	products := []Product{
		{Name: "Widget", BasePrice: 10.0, Category: "tools"},
		{Name: "Gadget", BasePrice: 25.0, Category: "electronics"},
		{Name: "Doohickey", BasePrice: 5.0, Category: "misc"},
	}

	// User-defined pricing rules (could come from config/database)
	rules := map[string]string{
		"discount":   `price > 20 ? 0.15 : 0.05`,
		"taxRate":    `category == "electronics" ? 0.08 : 0.06`,
		"finalPrice": `price * (1 - discount) * (1 + taxRate)`,
	}

	fmt.Println("Dynamic Expression Evaluation")
	fmt.Println("=============================")

	for _, product := range products {
		// Build environment with product data
		env := risor.Builtins()
		env["price"] = product.BasePrice
		env["category"] = product.Category
		env["name"] = product.Name

		// Evaluate discount rule
		discount, err := risor.Eval(ctx, rules["discount"], risor.WithEnv(env))
		if err != nil {
			log.Fatal(err)
		}
		env["discount"] = discount

		// Evaluate tax rate
		taxRate, err := risor.Eval(ctx, rules["taxRate"], risor.WithEnv(env))
		if err != nil {
			log.Fatal(err)
		}
		env["taxRate"] = taxRate

		// Evaluate final price
		finalPrice, err := risor.Eval(ctx, rules["finalPrice"], risor.WithEnv(env))
		if err != nil {
			log.Fatal(err)
		}

		fmt.Printf("\n%s:\n", product.Name)
		fmt.Printf("  Base Price: $%.2f\n", product.BasePrice)
		fmt.Printf("  Discount:   %.0f%%\n", discount.(float64)*100)
		fmt.Printf("  Tax Rate:   %.0f%%\n", taxRate.(float64)*100)
		fmt.Printf("  Final:      $%.2f\n", finalPrice)
	}

	// Example: User-defined filter expression
	fmt.Println("\n\nFiltering with Custom Expression")
	fmt.Println("=================================")

	filterExpr := `items.filter(item => item.price > threshold)`

	env := risor.Builtins()
	env["items"] = []map[string]any{
		{"name": "A", "price": 10},
		{"name": "B", "price": 25},
		{"name": "C", "price": 5},
		{"name": "D", "price": 50},
	}
	env["threshold"] = 15

	filtered, err := risor.Eval(ctx, filterExpr, risor.WithEnv(env))
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Items with price > 15: %v\n", filtered)
}
