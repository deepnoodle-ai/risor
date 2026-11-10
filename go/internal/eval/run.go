// Package eval compiles a checked syntax tree to Go closures and runs it, by
// spec/core/02-evaluation.md. There is no bytecode and no virtual machine.
//
// Every name has one fixed slot for the whole run (02-evaluation.md 11.1), so
// a run is a slice of slots and a set of counters. A failure is raised as a
// Go panic of *failure and recovered by a try or by Run.
package eval

import (
	"context"
	"fmt"
	"strconv"
	"time"
	"unsafe"

	"github.com/deepnoodle-ai/risor/v3/internal/diag"
	"github.com/deepnoodle-ai/risor/v3/internal/messages"
	"github.com/deepnoodle-ai/risor/v3/internal/syntax"
	"github.com/deepnoodle-ai/risor/v3/internal/value"
)

// NoLimit turns a step or allocation limit off.
const NoLimit = -1

// Limits are the limits of a run (03-host.md 3.6).
type Limits struct {
	Steps   int64 // NoLimit for none
	Alloc   int64 // NoLimit for none
	Depth   int
	Timeout time.Duration // 0 for none
}

// DefaultLimits are the limits of 03-host.md 3.6.1.
var DefaultLimits = Limits{Steps: 1_000_000, Alloc: 1_000_000, Depth: 200}

// Usage is what a run charged.
type Usage struct {
	Steps int64
	Alloc int64
}

// Vars supplies the values of variables to a run.
type Vars interface {
	Var(name string) (any, bool)
}

// HostFunc is the implementation of a host function or of a method of an
// opaque type. For a method, args[0] is the receiver.
type HostFunc func(c *Call, args []any) (any, error)

// Call is the call handle a host function receives (03-host.md 3.5.2).
type Call struct {
	r *run
}

// Context returns the run's context, which carries its cancellation.
func (c *Call) Context() context.Context { return c.r.ctx }

// LimitError is returned by Charge and Allocate when the budget cannot cover
// the charge. The run then ends with the limit failure.
type LimitError struct{ Code string }

func (e *LimitError) Error() string { return e.Code }

// Charge charges steps to the run.
func (c *Call) Charge(steps int64) error {
	if steps <= 0 {
		return nil
	}
	if f := c.r.tryStep(steps); f != nil {
		c.r.pending = f
		return &LimitError{Code: f.code}
	}
	return nil
}

// Allocate charges allocation units to the run.
func (c *Call) Allocate(units int64) error {
	if units <= 0 {
		return nil
	}
	if f := c.r.tryUnit(units); f != nil {
		c.r.pending = f
		return &LimitError{Code: f.code}
	}
	return nil
}

// failure is a run failure on its way to a try or to the host.
type failure struct {
	code     string
	template string
	params   []messages.Param
	span     syntax.Span
	hasSpan  bool
	msg      string // the message of failed and function-error
	ownMsg   bool
	data     any
	hasData  bool
	host     bool // number-range from a host number: class environment
	cause    error
}

func (f *failure) message() string {
	if f.ownMsg {
		return f.msg
	}
	return messages.Build(f.code, f.template, f.params...)
}

// class is the class of a run diagnostic in a program compiled without a
// check (03-host.md 3.7.6).
func (f *failure) class() string {
	switch f.code {
	case "null-operand", "wrong-kind", "missing-field", "no-match":
		return "dynamic"
	case "number-range":
		if f.host {
			return "environment"
		}
		return "range"
	case "missing-variable":
		return "environment"
	case "function-error", "function-panic":
		return "host"
	case "index-range":
		return "index"
	case "divide-by-zero":
		return "divide"
	case "invalid-argument", "failed", "not-conforming":
		return "value"
	case "internal-error":
		return ""
	}
	if diag.LimitCodes[f.code] {
		return "limit"
	}
	return ""
}

// run is the state of one run.
type run struct {
	prog    *Program
	ctx     context.Context
	vars    Vars
	slots   []any
	varVals []any
	varSeen []bool

	steps, units       int64
	maxSteps, maxUnits int64
	depth, walk        int
	maxDepth           int
	deadline           time.Time
	pending            *failure
	strs               map[strKey]string
}

func (r *run) raise(f *failure) { panic(f) }

func fail(code, template string, sp syntax.Span, params ...messages.Param) *failure {
	return &failure{code: code, template: template, span: sp, hasSpan: true, params: params}
}

func (r *run) limitFailure(code string, limit int64) *failure {
	f := &failure{code: code}
	if code != "deadline-exceeded" && code != "cancelled" {
		f.params = []messages.Param{messages.Word("limit", strconv.FormatInt(limit, 10))}
	}
	return f
}

func (r *run) tryStep(n int64) *failure {
	if r.maxSteps >= 0 && r.steps+n > r.maxSteps {
		return r.limitFailure("steps-exceeded", r.maxSteps)
	}
	old := r.steps
	r.steps += n
	if old>>10 != r.steps>>10 {
		if f := r.checkDeadline(); f != nil {
			return f
		}
	}
	return nil
}

func (r *run) tryUnit(n int64) *failure {
	if r.maxUnits >= 0 && r.units+n > r.maxUnits {
		return r.limitFailure("allocation-exceeded", r.maxUnits)
	}
	r.units += n
	return nil
}

// step charges n steps (02-evaluation.md 8.2).
func (r *run) step(n int64) {
	if n == 0 {
		return
	}
	if f := r.tryStep(n); f != nil {
		r.raise(f)
	}
}

// unit charges n allocation units.
func (r *run) unit(n int64) {
	if n == 0 {
		return
	}
	if f := r.tryUnit(n); f != nil {
		r.raise(f)
	}
}

// checkDeadline is the check of 02-evaluation.md 8.3, made at least once in
// every 1,024 steps.
func (r *run) checkDeadline() *failure {
	if err := r.ctx.Err(); err != nil {
		if err == context.DeadlineExceeded {
			return r.limitFailure("deadline-exceeded", 0)
		}
		return r.limitFailure("cancelled", 0)
	}
	if !r.deadline.IsZero() && time.Now().After(r.deadline) {
		return r.limitFailure("deadline-exceeded", 0)
	}
	return nil
}

// enterCall tests the depth limit for a call that adds measure, and adds it
// (02-evaluation.md 8.4).
func (r *run) enterCall(measure int) {
	if r.depth+r.walk+measure > r.maxDepth {
		r.raise(r.limitFailure("depth-exceeded", int64(r.maxDepth)))
	}
	r.depth += measure
}

// enterWalk enters one level of a walk.
func (r *run) enterWalk() {
	if r.depth+r.walk+1 > r.maxDepth {
		r.raise(r.limitFailure("depth-exceeded", int64(r.maxDepth)))
	}
	r.walk++
}

// read reads a host value at the node that yields it (ruling AC16).
func (r *run) read(v any, sp syntax.Span) any {
	switch v.(type) {
	case nil, bool, float64, []any, map[string]any:
		return v
	case string:
		return r.readString(v.(string))
	}
	x, e := value.Read(v)
	if e == value.ReadRange {
		f := fail("number-range", "host", sp)
		f.host = true
		r.raise(f)
	}
	return x
}

// readString gives the valid form of host text (ruling AC18). A long string
// is checked once in a run, so a read costs constant work whatever its size.
func (r *run) readString(s string) string {
	if len(s) <= 64 {
		for i := 0; i < len(s); i++ {
			if s[i] >= 0x80 {
				x, _ := value.Read(s)
				return x.(string)
			}
		}
		return s
	}
	key := strKey{unsafe.StringData(s), len(s)}
	if x, ok := r.strs[key]; ok {
		return x
	}
	x, _ := value.Read(s)
	if r.strs == nil {
		r.strs = map[strKey]string{}
	}
	r.strs[key] = x.(string)
	return x.(string)
}

type strKey struct {
	p *byte
	n int
}

// protect runs c and returns its value, or the failure it raised.
func (r *run) protect(c code) (v any, f *failure) {
	defer func() {
		if x := recover(); x != nil {
			if ff, ok := x.(*failure); ok {
				f = ff
				return
			}
			panic(x)
		}
	}()
	return c(r), nil
}

// Diagnostic turns a failure into a run diagnostic.
func (f *failure) diagnostic(file *syntax.File) diag.Diagnostic {
	d := diag.Diagnostic{Phase: diag.PhaseRun, Code: f.code, Class: f.class(), Message: f.message()}
	if f.hasSpan {
		sp := file.Span(f.span.Start, f.span.End)
		d.Span = &sp
	}
	if f.code == "failed" {
		d.Data, d.HasData = f.data, true
	}
	return d
}

func internalError(x any) *failure {
	return &failure{code: "internal-error", msg: fmt.Sprintf("the run stopped on a fault inside the implementation: %v", x)}
}
