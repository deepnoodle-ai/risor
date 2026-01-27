# Configurable VM Observer

## Background and Motivation

This design adds an `Observer` interface to the VM for monitoring execution events
(instruction steps, function calls, and returns). This enables debuggers, profilers,
and coverage tools without modifying Risor's core.

A naive implementation would call `OnStep` on every single instruction, creating
substantial overhead even when the observer only needs coarse-grained information.
This design avoids that by letting observers declare what events they need.

### Use Cases

1. **Profilers** primarily need call/return timing, not per-instruction callbacks
2. **Coverage tools** only need to know which source lines were executed, not every instruction
3. **Debuggers** only need callbacks at breakpoints during normal execution (not stepping)
4. **Statistical profilers** want sampled instruction callbacks

### Use Case Requirements

| Use Case     | OnStep Needs            | OnCall/Return | Key Requirement                                     |
| ------------ | ----------------------- | ------------- | --------------------------------------------------- |
| **Debugger** | Breakpoints + line-step | Yes           | Skip OnStep unless at breakpoint or single-stepping |
| **Coverage** | Line-change only        | Optional      | One callback per unique source line                 |
| **Profiler** | Sampled or none         | Yes           | Call/Return timing; optional CPU sampling           |

## Solution Overview

Introduce an `ObserverConfig` that allows observers to declare their requirements,
enabling the VM to skip unnecessary work. The key insight is that **Call/Return
events are inherently cheap** (only at function boundaries) while **OnStep is
expensive** (every instruction) - so we optimize OnStep dispatch.

### Design Goals

1. **Zero overhead when not needed**: Profilers that only want Call/Return pay nothing for OnStep
2. **Minimal overhead for common patterns**: Line-change detection is a single comparison
3. **Simple API**: Observers declare intent, VM handles optimization
4. **Keep vm.go simple**: Minimize complexity in the eval loop

## Proposed API

### Observer Modes

```go
// StepMode controls when OnStep callbacks are triggered.
type StepMode uint8

const (
    // StepAll calls OnStep for every instruction.
    // Use for: detailed tracing, instruction-level debugging.
    StepAll StepMode = iota

    // StepNone never calls OnStep.
    // Use for: profilers that only need Call/Return events.
    StepNone

    // StepSampled calls OnStep every N instructions.
    // Use for: statistical CPU profiling.
    StepSampled

    // StepOnLine calls OnStep when the source location changes.
    // Use for: coverage tools, line-level debugging, breakpoint-based debuggers.
    StepOnLine
)
```

**Note**: Breakpoint support is handled by observers using `StepOnLine` mode,
not by the VM. This keeps the VM simple and avoids the complexity of tracking
breakpoints across multiple code objects. Observers receive full source location
information in `OnStep` and can check their own breakpoint maps (keyed by file+line).

### Observer Configuration

```go
// ObserverConfig specifies what events an observer wants to receive.
// Use NewObserverConfig() to create configs with safe defaults.
type ObserverConfig struct {
    // StepMode controls OnStep callback frequency.
    StepMode StepMode

    // SampleInterval is the number of instructions between OnStep calls
    // when StepMode is StepSampled. Must be > 0; values <= 0 are treated as 1.
    // Ignored for other modes.
    SampleInterval int

    // ObserveCalls enables OnCall callbacks.
    ObserveCalls bool

    // ObserveReturns enables OnReturn callbacks.
    ObserveReturns bool
}

// NewObserverConfig creates a config with safe defaults.
// ObserveCalls and ObserveReturns default to true.
func NewObserverConfig(mode StepMode) ObserverConfig {
    return ObserverConfig{
        StepMode:       mode,
        SampleInterval: 1000, // Reasonable default for sampling
        ObserveCalls:   true,
        ObserveReturns: true,
    }
}
```

**Important**: A zero-value `ObserverConfig{}` will have `ObserveCalls` and
`ObserveReturns` as `false`, which silently disables those callbacks. Always
use `NewObserverConfig()` to avoid this footgun.

### Observer Interface

```go
// Observer monitors VM execution events.
type Observer interface {
    // Config returns the observer's configuration.
    // Called once when the observer is attached to the VM.
    Config() ObserverConfig

    // OnStep is called based on the StepMode in the observer's config.
    OnStep(event StepEvent) bool

    // OnCall is called when a function is invoked (if ObserveCalls is true).
    OnCall(event CallEvent) bool

    // OnReturn is called when a function returns (if ObserveReturns is true).
    OnReturn(event ReturnEvent) bool
}
```

### Configuration Normalization

```go
// NormalizeConfig validates and clamps config values.
// Note: Does NOT set defaults for ObserveCalls/ObserveReturns - callers
// should use NewObserverConfig() to get safe defaults.
func NormalizeConfig(cfg ObserverConfig) ObserverConfig {
    // Treat SampleInterval <= 0 as 1 (every instruction, same as StepAll)
    if cfg.StepMode == StepSampled && cfg.SampleInterval <= 0 {
        cfg.SampleInterval = 1
    }
    return cfg
}
```

### VM Configuration

```go
// In VM setup:
func (vm *VirtualMachine) configureObserver() {
    if vm.observer == nil {
        return
    }
    vm.observerConfig = NormalizeConfig(vm.observer.Config())
}
```

## Implementation Details

### VM Changes

Add configuration state to the VM:

```go
type VirtualMachine struct {
    // ... existing fields ...

    observer       Observer
    observerConfig ObserverConfig

    // Observer state
    sampleCount      int         // Counter for StepSampled mode
    lastObservedCode *loadedCode // Code object from last OnStep (changes on function call/return)
    lastObservedLine int         // Source line from last OnStep
}
```

### Optimized Eval Loop

The eval loop delegates observer handling to a separate method for readability:

```go
func (vm *VirtualMachine) eval(ctx context.Context) error {
    for vm.ip < len(vm.activeCode.Instructions) {
        // ... context cancellation check ...

        if err := vm.dispatchObserver(); err != nil {
            return err
        }

        vm.ip++
        // ... instruction dispatch ...
    }
    return nil
}
```

### Observer Dispatch

```go
// dispatchObserver handles OnStep callbacks based on the observer config.
// Returns an error if the observer halts execution.
func (vm *VirtualMachine) dispatchObserver() error {
    if vm.observer == nil {
        return nil
    }

    cfg := vm.observerConfig
    var shouldStep bool
    var loc object.SourceLocation

    switch cfg.StepMode {
    case StepAll:
        shouldStep = true
    case StepNone:
        return nil
    case StepSampled:
        vm.sampleCount++
        if vm.sampleCount >= cfg.SampleInterval {
            vm.sampleCount = 0
            shouldStep = true
        }
    case StepOnLine:
        shouldStep, loc = vm.checkLineChanged()
    }

    if !shouldStep {
        return nil
    }

    // For modes other than StepOnLine, loc is not set yet
    if loc.Line == 0 {
        loc = vm.activeCode.LocationAt(vm.ip)
    }

    event := StepEvent{
        IP:         vm.ip,
        Opcode:     vm.activeCode.Instructions[vm.ip],
        OpcodeName: op.GetInfo(vm.activeCode.Instructions[vm.ip]).Name,
        Location:   loc,
        StackDepth: vm.sp + 1,
        FrameDepth: vm.fp + 1,
    }
    if !vm.observer.OnStep(event) {
        return fmt.Errorf("execution halted by observer")
    }
    return nil
}

// checkLineChanged returns true if the source location has changed since
// the last OnStep call, along with the current location.
func (vm *VirtualMachine) checkLineChanged() (bool, object.SourceLocation) {
    loc := vm.activeCode.LocationAt(vm.ip)

    // Skip invalid locations (line 0 indicates no source info).
    // This suppresses OnStep events until a valid line is reached.
    if loc.Line == 0 {
        return false, loc
    }

    // Check if code object or line changed
    if vm.activeCode != vm.lastObservedCode || loc.Line != vm.lastObservedLine {
        vm.lastObservedCode = vm.activeCode
        vm.lastObservedLine = loc.Line
        return true, loc
    }
    return false, loc
}
```

**Note**: `LocationAt` is O(1) - it's a simple array index lookup into the
pre-populated `Locations` slice in `loadedCode`. This makes `StepOnLine` efficient.

### Call/Return Dispatch

Gate Call/Return callbacks based on config:

```go
// In callFunction:
if vm.observer != nil && vm.observerConfig.ObserveCalls {
    event := CallEvent{...}
    if !vm.observer.OnCall(event) {
        return nil, fmt.Errorf("execution halted by observer")
    }
}

// In ReturnValue handling:
if vm.observer != nil && vm.observerConfig.ObserveReturns {
    event := ReturnEvent{...}
    if !vm.observer.OnReturn(event) {
        return fmt.Errorf("execution halted by observer")
    }
}
```

### Dynamic Configuration Updates

For debuggers that need to toggle between "running" and "stepping" modes.
Configuration updates are only allowed when the VM is paused to avoid data races.

```go
// SetObserverConfig updates the observer configuration on a paused VM.
// Returns an error if the VM is currently running.
func (vm *VirtualMachine) SetObserverConfig(cfg ObserverConfig) error {
    if vm.running {
        return errors.New("cannot update observer config while VM is running")
    }
    vm.observerConfig = NormalizeConfig(cfg)
    return nil
}
```

## Performance Analysis

### Overhead by Mode

| Mode         | Per-Instruction Cost           | When to Use                  |
| ------------ | ------------------------------ | ---------------------------- |
| `StepNone`   | None                           | Profilers (Call/Return only) |
| `StepSampled`| 1 int increment + comparison   | Statistical profiling        |
| `StepOnLine` | 1 pointer + int comparison     | Coverage, debuggers          |
| `StepAll`    | Full event construction        | Detailed tracing             |

### Expected Improvements

Compared to naive "call OnStep every instruction" approach:

- **Profiler**: ~100% reduction in OnStep overhead (from every instruction to zero)
- **Coverage**: ~95% reduction (callback only on line changes, typically 5-20 instructions per line)
- **Debugger (running)**: ~95% reduction (StepOnLine + observer checks breakpoint map)
- **Debugger (stepping)**: Uses StepOnLine for line-level stepping

## Code Examples

### Profiler (Call/Return Only)

```go
type Profiler struct {
    vm.NoOpObserver
    callTimes map[string]time.Duration
    callStack []callFrame
}

func (p *Profiler) Config() vm.ObserverConfig {
    cfg := vm.NewObserverConfig(vm.StepNone)
    // Calls and returns already default to true
    return cfg
}

func (p *Profiler) OnCall(event vm.CallEvent) bool {
    p.callStack = append(p.callStack, callFrame{
        name:  event.FunctionName,
        start: time.Now(),
    })
    return true
}

func (p *Profiler) OnReturn(event vm.ReturnEvent) bool {
    if len(p.callStack) > 0 {
        frame := p.callStack[len(p.callStack)-1]
        p.callStack = p.callStack[:len(p.callStack)-1]
        p.callTimes[frame.name] += time.Since(frame.start)
    }
    return true
}
```

### Coverage Tool (Line-Based)

```go
type CoverageTool struct {
    vm.NoOpObserver
    coveredLines map[string]map[int]bool  // file -> line -> covered
}

func (c *CoverageTool) Config() vm.ObserverConfig {
    cfg := vm.NewObserverConfig(vm.StepOnLine)
    cfg.ObserveCalls = false
    cfg.ObserveReturns = false
    return cfg
}

func (c *CoverageTool) OnStep(event vm.StepEvent) bool {
    file := event.Location.Filename
    line := event.Location.Line
    if line == 0 {
        return true // Skip invalid locations
    }
    if c.coveredLines[file] == nil {
        c.coveredLines[file] = make(map[int]bool)
    }
    c.coveredLines[file][line] = true
    return true
}
```

### Debugger (Breakpoint-Based)

Debuggers handle breakpoints themselves using `StepOnLine` mode.
The observer maintains its own breakpoint map keyed by file+line.

**Note**: Observer state (breakpoints, stepping flag) should only be mutated
while the VM is paused to avoid data races. The debugger pauses by returning
`false` from `OnStep`, then can safely update its state before resuming.

**Filename normalization**: Use `event.Location.Filename` as the canonical key
for breakpoints. This is the filename set during compilation. If your debugger
accepts user input for breakpoints (e.g., relative paths), normalize user input
to match the compiled filename format before storing in the breakpoint map.

```go
type Debugger struct {
    vm.NoOpObserver
    breakpoints map[string]map[int]struct{}  // file -> line -> exists
    stepping    bool
    // Note: Only mutate breakpoints/stepping while VM is paused
}

func (d *Debugger) Config() vm.ObserverConfig {
    return vm.NewObserverConfig(vm.StepOnLine)
}

func (d *Debugger) OnStep(event vm.StepEvent) bool {
    file := event.Location.Filename
    line := event.Location.Line

    // Check if we hit a breakpoint or are stepping
    atBreakpoint := false
    if lines, ok := d.breakpoints[file]; ok {
        _, atBreakpoint = lines[line]
    }

    if atBreakpoint || d.stepping {
        d.showState(event)
        return d.waitForCommand()
    }
    return true
}

// SetBreakpoint sets a breakpoint at file:line.
func (d *Debugger) SetBreakpoint(file string, line int) {
    if d.breakpoints[file] == nil {
        d.breakpoints[file] = make(map[int]struct{})
    }
    d.breakpoints[file][line] = struct{}{}
}
```

## Testing Strategy

### Unit Tests

1. **Mode behavior tests**: Verify each StepMode triggers callbacks correctly
2. **StepOnLine cross-function**: Verify OnStep fires when calling function that starts on same line
3. **SampleInterval edge cases**: Verify <= 0 is treated as 1
4. **Line 0 handling**: Verify line 0 locations don't affect line change detection
5. **Zero-value config**: Verify `ObserverConfig{}` disables calls/returns (documents the footgun)

### Benchmarks

```go
func BenchmarkEval_NoObserver(b *testing.B)           // Baseline
func BenchmarkEval_StepAll(b *testing.B)              // Every instruction
func BenchmarkEval_StepNone(b *testing.B)             // Profiler mode
func BenchmarkEval_StepOnLine(b *testing.B)           // Coverage mode
func BenchmarkEval_StepSampled(b *testing.B)          // Sampling mode
```

### Integration Tests

1. **Profiler integration**: End-to-end profiling of real scripts
2. **Coverage integration**: Verify line coverage matches expected
3. **Debugger integration**: Breakpoint hit detection, step behavior

## Design Decisions

### Why No StepBreakpoint Mode?

The original design included a `StepBreakpoint` mode where the VM would check a
breakpoint map keyed by instruction pointer (IP). This was removed because:

1. **IP is relative to code object**: Each function has its own code object with
   IPs starting at 0. A breakpoint at IP 5 could match in multiple functions.
   Correct implementation requires a composite key like `(*loadedCode, ip)`.

2. **Exposes internal details**: `loadedCode` is an internal type. Exposing it
   in the public breakpoint API leaks implementation details.

3. **Adds VM complexity**: The VM would need to manage breakpoint lifecycle,
   handle code object identity, and provide helper methods for setting breakpoints.

4. **StepOnLine is nearly as fast**: `StepOnLine` triggers once per source line
   (typically every 5-20 instructions). The observer can check its own breakpoint
   map (keyed by file+line) with negligible overhead.

**Recommendation**: Debuggers should use `StepOnLine` mode and maintain their own
breakpoint map keyed by `(filename, line)`. This is simpler, avoids internal
details, and provides source-level breakpoints which is what users typically want.

### LocationAt Performance

`LocationAt(ip)` is O(1) - it's a simple array index into the pre-populated
`Locations` slice. The compiler generates one location entry per instruction,
so no searching is required. This makes `StepOnLine` efficient.

### Code Object Identity

`StepOnLine` needs to detect when execution moves to a different function (which
has a different code object). We use direct pointer comparison (`vm.activeCode != vm.lastObservedCode`) rather than storing a `uintptr`. This avoids the `unsafe`
package and is simpler. The `loadedCode` pointer is stable for the lifetime of
the VM execution - code objects are created when functions are loaded and not
moved or reallocated.

**One code object = one file**: Each `loadedCode` has a single filename set during
compilation. The compiler creates separate code objects for each function, and all
instructions in a code object share the same filename. This means comparing
`(code pointer, line)` is sufficient for detecting location changes - we don't
need to compare filenames within the same code object.

### Line 0 Handling

`LocationAt` may return `Line: 0` for instructions without source info (e.g.,
compiler-generated setup code). `locationChanged()` treats line 0 as "no location"
and suppresses `OnStep` events. Once a non-zero line is reached, `OnStep` fires
normally. This is intentional - there's nothing meaningful to show the user for
line 0 locations.

### Configuration Updates While Running

Configuration updates are only allowed when the VM is paused. This avoids:
- Data races between the eval loop and external configuration changes
- Complexity of atomic operations or synchronization in the hot path

Debuggers that need to toggle modes (e.g., from "running" to "stepping") should:
1. Pause execution (return false from OnStep at a breakpoint)
2. Call `SetObserverConfig()` with new configuration
3. Resume execution

### Observer State and Data Races

Observers that maintain mutable state (e.g., breakpoint maps, stepping flags)
should only mutate that state while the VM is paused. The eval loop reads
observer state during callbacks, so concurrent mutation is a data race.

The recommended pattern:
1. Return `false` from `OnStep` to pause execution
2. Update observer state (breakpoints, flags, etc.)
3. Optionally call `SetObserverConfig()` to change modes
4. Resume execution

If an observer truly needs concurrent updates (rare), it must use its own
synchronization (e.g., mutex around breakpoint map access).

## Files to Modify

- `vm/observer.go`: Add `StepMode`, `ObserverConfig`, `Observer` interface with `Config()`, `NewObserverConfig`
- `vm/vm.go`: Add `observerConfig`, `lastObservedCode`, and `lastObservedLine` fields, add `locationChanged()` helper, modify eval loop dispatch
- `vm/observer_test.go`: Add tests for new modes and configurations
