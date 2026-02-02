package main

import "github.com/deepnoodle-ai/risor/v2/object"

// Version is the current Risor version.
const docVersion = "2.0.0"

// QuickReference provides a concise overview of Risor for quick orientation.
type QuickReference struct {
	Risor          RisorInfo         `json:"risor"`
	SyntaxQuickRef []SyntaxPattern   `json:"syntax_quick_ref"`
	Topics         map[string]string `json:"topics"`
	Next           []string          `json:"next"`
}

// RisorInfo provides basic information about Risor.
type RisorInfo struct {
	Version        string `json:"version"`
	Description    string `json:"description"`
	ExecutionModel string `json:"execution_model"`
}

// SyntaxPattern describes a syntax pattern for the quick reference.
type SyntaxPattern struct {
	Pattern     string `json:"pattern"`
	Description string `json:"description"`
}

// CategoryOverview provides an overview of a documentation category.
type CategoryOverview struct {
	Category    string `json:"category"`
	Description string `json:"description"`
	Count       int    `json:"count,omitempty"`
	Next        string `json:"next,omitempty"`
}

// BuiltinsOverview lists all builtin functions.
type BuiltinsOverview struct {
	CategoryOverview
	Functions []object.FuncSpec `json:"functions"`
}

// TypesOverview lists all types.
type TypesOverview struct {
	CategoryOverview
	Types []TypeInfo `json:"types"`
}

// TypeInfo provides summary information about a type.
type TypeInfo struct {
	Name        string   `json:"name"`
	Doc         string   `json:"doc"`
	MethodCount int      `json:"method_count,omitempty"`
	Methods     []string `json:"methods,omitempty"`
}

// ModulesOverview lists all modules.
type ModulesOverview struct {
	CategoryOverview
	Modules []ModuleInfo `json:"modules"`
}

// ModuleInfo provides summary information about a module.
type ModuleInfo struct {
	Name      string   `json:"name"`
	Doc       string   `json:"doc"`
	FuncCount int      `json:"function_count"`
	Functions []string `json:"functions,omitempty"`
}

// SyntaxOverview provides the complete syntax reference.
type SyntaxOverview struct {
	CategoryOverview
	Sections []SyntaxSection `json:"sections"`
}

// SyntaxSection groups related syntax items.
type SyntaxSection struct {
	Name  string       `json:"name"`
	Items []SyntaxItem `json:"items"`
}

// SyntaxItem describes a single syntax construct.
type SyntaxItem struct {
	Syntax string `json:"syntax"`
	Type   string `json:"type,omitempty"`
	Notes  string `json:"notes"`
}

// ErrorsOverview provides error guidance.
type ErrorsOverview struct {
	CategoryOverview
	Patterns []ErrorPattern `json:"patterns"`
}

// ErrorPattern describes a common error pattern.
type ErrorPattern struct {
	Type           string         `json:"type"`
	MessagePattern string         `json:"message_pattern"`
	Causes         []string       `json:"causes"`
	Examples       []ErrorExample `json:"examples"`
}

// ErrorExample shows a specific error case.
type ErrorExample struct {
	Error       string `json:"error"`
	BadCode     string `json:"bad_code"`
	Fix         string `json:"fix"`
	Explanation string `json:"explanation"`
}

// FunctionDetail provides detailed documentation for a function.
type FunctionDetail struct {
	Name         string            `json:"name"`
	Category     string            `json:"category"`
	Signature    string            `json:"signature"`
	Doc          string            `json:"doc"`
	Parameters   []ParameterInfo   `json:"parameters,omitempty"`
	Returns      *ReturnInfo       `json:"returns,omitempty"`
	Examples     []ExampleInfo     `json:"examples,omitempty"`
	Related      []string          `json:"related,omitempty"`
	CommonErrors []CommonErrorInfo `json:"common_errors,omitempty"`
}

// ParameterInfo describes a function parameter.
type ParameterInfo struct {
	Name string `json:"name"`
	Type string `json:"type,omitempty"`
	Doc  string `json:"doc,omitempty"`
}

// ReturnInfo describes a function's return value.
type ReturnInfo struct {
	Type string `json:"type"`
	Doc  string `json:"doc,omitempty"`
}

// ExampleInfo provides a code example.
type ExampleInfo struct {
	Code        string `json:"code"`
	Result      string `json:"result,omitempty"`
	Explanation string `json:"explanation,omitempty"`
}

// CommonErrorInfo describes a common mistake with a function.
type CommonErrorInfo struct {
	Mistake     string `json:"mistake"`
	Fix         string `json:"fix"`
	Explanation string `json:"explanation"`
}

// TypeDetail provides detailed documentation for a type.
type TypeDetail struct {
	Type    string            `json:"type"`
	Name    string            `json:"name"`
	Doc     string            `json:"doc"`
	Methods []object.AttrSpec `json:"methods,omitempty"`
}

// ModuleDetail provides detailed documentation for a module.
type ModuleDetail struct {
	Type  string            `json:"type"`
	Name  string            `json:"name"`
	Doc   string            `json:"doc"`
	Funcs []object.FuncSpec `json:"functions"`
}

// ModuleFunctionDetail provides detailed documentation for a module function.
type ModuleFunctionDetail struct {
	Type     string          `json:"type"`
	Module   string          `json:"module"`
	Function object.FuncSpec `json:"function"`
}

// BuiltinDetail provides detailed documentation for a builtin function.
type BuiltinDetail struct {
	Type     string          `json:"type"`
	Function object.FuncSpec `json:"function"`
}

// FullDocumentation contains all documentation for --all mode.
type FullDocumentation struct {
	Risor    RisorInfo             `json:"risor"`
	Builtins []object.FuncSpec     `json:"builtins"`
	Modules  map[string]ModuleInfo `json:"modules"`
	Types    map[string]TypeDetail `json:"types"`
	Syntax   []SyntaxSection       `json:"syntax"`
	Errors   []ErrorPattern        `json:"errors"`
}
