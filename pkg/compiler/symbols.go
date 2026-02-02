package compiler

import (
	"fmt"
)

// Scope represents the scope of a symbol. It can be local, global, or free.
type Scope string

const (
	// Local indicates that a symbol is local to a function.
	Local Scope = "local"

	// Global indicates that a symbol is global to a module.
	Global Scope = "global"

	// Free indicates that a symbol is owned by an enclosing parent function.
	Free Scope = "free"
)

// Symbol represents an identifier in a program. It is used to store information
// about the identifier, such as its name, index, and optionally a value.
type Symbol struct {
	name       string
	index      uint16
	isConstant bool
	value      any
}

func (s *Symbol) Name() string {
	return s.name
}

func (s *Symbol) Index() uint16 {
	return s.index
}

func (s *Symbol) Value() any {
	return s.value
}

func (s *Symbol) IsConstant() bool {
	return s.isConstant
}

func (s *Symbol) String() string {
	return fmt.Sprintf("symbol(name: %s index: %d constant: %t value: %v)",
		s.name, s.index, s.isConstant, s.value)
}

// Resolution holds information about where a symbol resides, relative to
// the current scope.
//
//	func outer() {
//		x := 1
//		func inner() {
//			print(x)
//		}
//		return inner
//	}
//
// In this example, if we look up "x" while compiling "inner", we will get a
// resolution with a depth of 1 and a scope of "free". This indicates that "x"
// is defined by the immediate parent.
type Resolution struct {
	symbol    *Symbol
	scope     Scope
	depth     int
	freeIndex int
}

func (r *Resolution) String() string {
	return fmt.Sprintf("resolution(symbol: %s scope: %s depth: %d)",
		r.symbol.name, r.scope, r.depth)
}

func (r *Resolution) Symbol() *Symbol {
	return r.symbol
}

func (r *Resolution) Scope() Scope {
	return r.scope
}

func (r *Resolution) Depth() int {
	return r.depth
}

func (r *Resolution) FreeIndex() int {
	return r.freeIndex
}
