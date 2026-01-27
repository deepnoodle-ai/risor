package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/deepnoodle-ai/wonton/cli"
	"github.com/deepnoodle-ai/wonton/tui"
	"github.com/risor-io/risor"
	"github.com/risor-io/risor/object"
)

// Example represents a code example
type Example struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Code        string `json:"code"`
	Category    string `json:"category"`
}

var examples = []Example{
	// Basics
	{
		Name:        "hello",
		Description: "Hello World",
		Category:    "basics",
		Code:        `print("Hello, World!")`,
	},
	{
		Name:        "variables",
		Description: "Variable declarations",
		Category:    "basics",
		Code: `// let declares a mutable variable
let x = 42
x = x + 1
print(x)

// const declares an immutable constant
const PI = 3.14159
print(PI)`,
	},
	{
		Name:        "types",
		Description: "Basic data types",
		Category:    "basics",
		Code: `let str = "hello"        // string
let num = 42             // int
let pi = 3.14            // float
let yes = true           // bool
let nothing = nil        // nil

print(type(str), type(num), type(pi), type(yes), type(nothing))`,
	},

	// Collections
	{
		Name:        "lists",
		Description: "Working with lists",
		Category:    "collections",
		Code: `let nums = [1, 2, 3, 4, 5]

// Access elements
print(nums[0])      // 1
print(nums[-1])     // 5 (last element)

// Slice
print(nums[1:3])    // [2, 3]

// Methods
nums.append(6)
print(nums)
print(nums.map(x => x * 2))
print(nums.filter(x => x > 3))`,
	},
	{
		Name:        "maps",
		Description: "Working with maps",
		Category:    "collections",
		Code: `let person = {
    name: "Alice",
    age: 30,
    city: "NYC"
}

// Access values
print(person.name)
print(person["age"])

// Check keys
print("name" in person)  // true

// Get keys and values
print(keys(person))    // ["name", "age", "city"]`,
	},
	{
		Name:        "spread",
		Description: "Spread operator",
		Category:    "collections",
		Code: `// Spread in lists
let a = [1, 2, 3]
let b = [...a, 4, 5]
print(b)  // [1, 2, 3, 4, 5]

// Spread in maps
let defaults = {theme: "dark", lang: "en"}
let settings = {...defaults, lang: "es"}
print(settings)  // {theme: "dark", lang: "es"}`,
	},

	// Functions
	{
		Name:        "functions",
		Description: "Function definitions",
		Category:    "functions",
		Code: `// Named function
function greet(name) {
    return "Hello, " + name + "!"
}
print(greet("World"))

// With default parameter
function power(base, exp = 2) {
    return math.pow(base, exp)
}
print(power(3))     // 9
print(power(2, 10)) // 1024`,
	},
	{
		Name:        "arrows",
		Description: "Arrow functions",
		Category:    "functions",
		Code: `// Single parameter (no parens needed)
let double = x => x * 2

// Multiple parameters
let add = (a, b) => a + b

// With block body
let factorial = n => {
    if (n <= 1) { return 1 }
    return n * factorial(n - 1)
}

print(double(21))
print(add(10, 32))
print(factorial(5))`,
	},
	{
		Name:        "closures",
		Description: "Closures and state",
		Category:    "functions",
		Code: `function makeCounter() {
    let count = 0
    return function() {
        count = count + 1
        return count
    }
}

let counter = makeCounter()
print(counter())  // 1
print(counter())  // 2
print(counter())  // 3`,
	},

	// Control Flow
	{
		Name:        "conditionals",
		Description: "If/else expressions",
		Category:    "control",
		Code: `let x = 42

// If as expression (returns value)
let size = if (x > 100) {
    "large"
} else if (x > 10) {
    "medium"
} else {
    "small"
}
print(size)`,
	},
	{
		Name:        "iteration",
		Description: "Functional iteration",
		Category:    "control",
		Code: `// Use map, filter, and other methods for iteration
let nums = [1, 2, 3, 4, 5]

// Transform each element
let doubled = nums.map(x => x * 2)
print(doubled)  // [2, 4, 6, 8, 10]

// Filter elements
let evens = nums.filter(x => x % 2 == 0)
print(evens)  // [2, 4]

// Use range for sequences
let squares = list(range(5)).map(x => x * x)
print(squares)  // [0, 1, 4, 9, 16]`,
	},
	{
		Name:        "switch",
		Description: "Switch expressions",
		Category:    "control",
		Code: `let day = 3

let name = switch (day) {
    case 1: "Monday"
    case 2: "Tuesday"
    case 3: "Wednesday"
    case 4: "Thursday"
    case 5: "Friday"
    case 6, 7: "Weekend"
    default: "Unknown"
}
print(name)`,
	},

	// Error Handling
	{
		Name:        "errors",
		Description: "Error handling with try/catch",
		Category:    "errors",
		Code: `function divide(a, b) {
    if (b == 0) {
        throw error("division by zero")
    }
    return a / b
}

// Handle errors with try/catch
try {
    print(divide(10, 2))
    print(divide(10, 0))
} catch (e) {
    print("Caught:", e)
}`,
	},

	// Strings
	{
		Name:        "strings",
		Description: "String operations",
		Category:    "strings",
		Code: `let s = "hello world"

print(s.upper())           // HELLO WORLD
print(s.split(" "))        // ["hello", "world"]
print(s.replace("o", "0")) // hell0 w0rld
print(s.contains("world")) // true

// String interpolation
let name = "Alice"
let age = 30
print("Name: {name}, Age: {age}")`,
	},

	// Functional
	{
		Name:        "pipeline",
		Description: "Functional pipelines",
		Category:    "functional",
		Code: `let numbers = [1, 2, 3, 4, 5, 6, 7, 8, 9, 10]

// Chain operations
let result = numbers
    .filter(x => x % 2 == 0)  // even numbers
    .map(x => x * x)           // square them
    .filter(x => x > 10)       // greater than 10

print(result)  // [16, 36, 64, 100]`,
	},

	// Destructuring
	{
		Name:        "destructure",
		Description: "Destructuring assignment",
		Category:    "advanced",
		Code: `// Object destructuring
let {name, age} = {name: "Alice", age: 30, city: "NYC"}
print(name, age)

// With alias
let {name: n, age: a} = {name: "Bob", age: 25}
print(n, a)

// Array destructuring
let [first, second] = [1, 2, 3]
print(first, second)

// With defaults
let {x = 10} = {}
print(x)  // 10`,
	},

	// Modules
	{
		Name:        "math",
		Description: "Math module",
		Category:    "modules",
		Code: `print(math.PI)
print(math.sqrt(16))
print(math.pow(2, 10))
print(math.abs(-42))
print(math.floor(3.7))
print(math.ceil(3.2))
print(math.round(3.5))`,
	},
	{
		Name:        "time",
		Description: "Time module",
		Category:    "modules",
		Code: `let now = time.now()
print(now)
print(now.format("2006-01-02 15:04:05"))
print(now.year())
print(now.month())
print(now.day())

// Measure duration
let start = time.now()
time.sleep(0.1)
print("Elapsed:", time.since(start), "seconds")`,
	},
	{
		Name:        "regexp",
		Description: "Regular expressions",
		Category:    "modules",
		Code: `// Match pattern
print(regexp.match("\\d+", "abc123def"))  // true

// Find matches
print(regexp.find("\\d+", "abc123def"))      // "123"
print(regexp.find_all("\\d+", "a1b2c3"))     // ["1", "2", "3"]

// Replace
print(regexp.replace_all("\\d", "a1b2c3", "X"))  // aXbXcX

// Split
print(regexp.split("\\s+", "hello   world"))  // ["hello", "world"]`,
	},
	{
		Name:        "rand",
		Description: "Random numbers",
		Category:    "modules",
		Code: `// Random float in [0, 1)
print(rand.float())

// Random int in [0, n)
print(rand.int(100))

// Shuffle a list
let cards = ["A", "K", "Q", "J"]
rand.shuffle(cards)
print(cards)`,
	},
}

func examplesHandler(ctx *cli.Context) error {
	topic := ctx.Arg(0)
	run := ctx.Bool("run")
	outputFormat := ctx.String("output")

	// JSON output
	if outputFormat == "json" {
		return examplesHandlerJSON(topic)
	}

	// Styles
	titleStyle := tui.NewStyle().WithFgRGB(tui.RGB{R: 255, G: 200, B: 80}).WithBold()
	categoryStyle := tui.NewStyle().WithFgRGB(tui.RGB{R: 180, G: 140, B: 220}).WithBold()
	nameStyle := tui.NewStyle().WithFgRGB(tui.RGB{R: 100, G: 200, B: 255})
	descStyle := tui.NewStyle().WithFgRGB(tui.RGB{R: 180, G: 180, B: 190})
	outputStyle := tui.NewStyle().WithFgRGB(tui.RGB{R: 255, G: 180, B: 100})
	mutedStyle := tui.NewStyle().WithFgRGB(tui.RGB{R: 120, G: 120, B: 130})

	if topic == "" {
		// Show all examples grouped by category
		fmt.Println(tui.Sprint(tui.Text("Risor Examples").Style(titleStyle)))
		fmt.Println()

		// Group by category
		categories := make(map[string][]Example)
		var categoryOrder []string
		for _, ex := range examples {
			if _, ok := categories[ex.Category]; !ok {
				categoryOrder = append(categoryOrder, ex.Category)
			}
			categories[ex.Category] = append(categories[ex.Category], ex)
		}

		for _, cat := range categoryOrder {
			fmt.Println(tui.Sprint(tui.Text("%s", strings.ToUpper(cat)).Style(categoryStyle)))
			for _, ex := range categories[cat] {
				fmt.Println(tui.Sprint(tui.Group(
					tui.Text("  %-12s", ex.Name).Style(nameStyle),
					tui.Text("  %s", ex.Description).Style(descStyle),
				)))
			}
			fmt.Println()
		}

		fmt.Println(tui.Sprint(tui.Text("Run 'risor examples <name>' to see code").Style(mutedStyle)))
		fmt.Println(tui.Sprint(tui.Text("Run 'risor examples <name> --run' to execute").Style(mutedStyle)))
		return nil
	}

	// Find the example
	var found *Example
	for i := range examples {
		if examples[i].Name == topic {
			found = &examples[i]
			break
		}
	}

	if found == nil {
		// Try partial match
		var matches []Example
		for _, ex := range examples {
			if strings.Contains(ex.Name, topic) || strings.Contains(ex.Category, topic) {
				matches = append(matches, ex)
			}
		}
		if len(matches) == 0 {
			return fmt.Errorf("example %q not found", topic)
		}
		if len(matches) == 1 {
			found = &matches[0]
		} else {
			fmt.Println(tui.Sprint(tui.Text("Multiple matches:").Style(titleStyle)))
			for _, ex := range matches {
				fmt.Println(tui.Sprint(tui.Group(
					tui.Text("  %-12s", ex.Name).Style(nameStyle),
					tui.Text("  %s", ex.Description).Style(descStyle),
				)))
			}
			return nil
		}
	}

	// Show the example
	fmt.Println(tui.Sprint(tui.Text("%s", found.Description).Style(titleStyle)))
	fmt.Println(tui.Sprint(tui.Text("Category: %s", found.Category).Style(mutedStyle)))
	fmt.Println()

	// Print code with syntax highlighting (use "javascript" as closest match for Risor syntax)
	tui.Print(tui.Code(found.Code, "javascript").Theme("monokai"))
	fmt.Println()

	if run {
		fmt.Println()
		fmt.Println(tui.Sprint(tui.Text("OUTPUT").Style(categoryStyle)))
		fmt.Println(tui.Sprint(tui.Text("%s", strings.Repeat("-", 40)).Style(mutedStyle)))

		// Execute the code with print function
		env := risor.Builtins()
		env["print"] = object.NewBuiltin("print", func(ctx context.Context, args ...object.Object) (object.Object, error) {
			parts := make([]string, len(args))
			for i, arg := range args {
				parts[i] = arg.Inspect()
			}
			fmt.Println(strings.Join(parts, " "))
			return object.Nil, nil
		})
		result, err := risor.Eval(context.Background(), found.Code, risor.WithEnv(env))
		if err != nil {
			fmt.Println(tui.Sprint(tui.Text("Error: %v", err).Style(tui.NewStyle().WithFgRGB(tui.RGB{R: 255, G: 100, B: 100}))))
		} else if result != nil {
			fmt.Println(tui.Sprint(tui.Text("=> %v", result).Style(outputStyle)))
		}
	} else {
		fmt.Println()
		fmt.Println(tui.Sprint(tui.Text("Run with --run to execute").Style(mutedStyle)))
	}

	return nil
}

func listExampleNames() []string {
	names := make([]string, 0, len(examples))
	for _, ex := range examples {
		names = append(names, ex.Name)
	}
	sort.Strings(names)
	return names
}

func examplesHandlerJSON(topic string) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")

	if topic == "" {
		// Return all examples grouped by category
		result := struct {
			Examples   []Example           `json:"examples"`
			Categories map[string][]string `json:"categories"`
		}{
			Examples:   examples,
			Categories: make(map[string][]string),
		}

		for _, ex := range examples {
			result.Categories[ex.Category] = append(result.Categories[ex.Category], ex.Name)
		}

		return enc.Encode(result)
	}

	// Find specific example
	for _, ex := range examples {
		if ex.Name == topic {
			return enc.Encode(ex)
		}
	}

	// Try partial match
	var matches []Example
	for _, ex := range examples {
		if strings.Contains(ex.Name, topic) || strings.Contains(ex.Category, topic) {
			matches = append(matches, ex)
		}
	}

	if len(matches) == 0 {
		return fmt.Errorf("example %q not found", topic)
	}
	if len(matches) == 1 {
		return enc.Encode(matches[0])
	}

	return enc.Encode(struct {
		Matches []Example `json:"matches"`
	}{matches})
}
