// Package impl runs the outcome command of each implementation.
package impl

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Impl is one implementation and how to run its outcome command.
type Impl struct {
	Name  string
	Dir   string   // the directory the commands run in
	Build []string // run once before the first case; may be empty
	Cmd   []string // the command; the case file's path is appended
	Env   []string // added to the environment of both commands
}

// Defaults gives the three implementations of ruling AJ, under the
// repository root. Go and Rust are built once into bin, and the built
// program is run for each file.
func Defaults(root, bin string) map[string]*Impl {
	return map[string]*Impl{
		"go": {
			Name:  "go",
			Dir:   filepath.Join(root, "go"),
			Build: []string{"go", "build", "-o", filepath.Join(bin, "outcome-go"), "./cmd/outcome"},
			Cmd:   []string{filepath.Join(bin, "outcome-go")},
			Env:   []string{"GOWORK=off"},
		},
		"ts": {
			Name: "ts",
			Dir:  filepath.Join(root, "ts"),
			Cmd:  []string{"node", "scripts/outcome.ts"},
		},
		"rust": {
			Name:  "rust",
			Dir:   filepath.Join(root, "rust"),
			Build: []string{"cargo", "build", "--quiet", "--release", "--bin", "outcome"},
			Cmd:   []string{filepath.Join(root, "rust", "target", "release", "outcome")},
		},
	}
}

// Prepare runs the build command, if there is one.
func (i *Impl) Prepare(timeout time.Duration) error {
	if len(i.Build) == 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, i.Build[0], i.Build[1:]...)
	cmd.Dir = i.Dir
	cmd.Env = append(os.Environ(), i.Env...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: build %s: %v\n%s", i.Name, strings.Join(i.Build, " "), err, out)
	}
	return nil
}

// Result is what one run of the command gave.
type Result struct {
	Stdout  []byte
	Stderr  []byte
	Status  int
	Timeout bool
}

// Failed says whether the run is itself a finding: a crash, a timeout or a
// status other than 0.
func (r *Result) Failed() bool { return r.Timeout || r.Status != 0 }

// Describe says in a few words how the run failed.
func (r *Result) Describe() string {
	if r.Timeout {
		return "timeout"
	}
	msg := strings.TrimSpace(string(r.Stderr))
	if len(msg) > 300 {
		msg = msg[:300] + "..."
	}
	return fmt.Sprintf("status %d: %s", r.Status, msg)
}

// Run runs the command on one case file.
func (i *Impl) Run(file string, timeout time.Duration) (*Result, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	args := append(append([]string(nil), i.Cmd[1:]...), file)
	cmd := exec.CommandContext(ctx, i.Cmd[0], args...)
	cmd.Dir = i.Dir
	cmd.Env = append(os.Environ(), i.Env...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	cmd.WaitDelay = time.Second
	err := cmd.Run()
	res := &Result{Stdout: stdout.Bytes(), Stderr: stderr.Bytes()}
	if ctx.Err() != nil {
		res.Timeout = true
		return res, nil
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		res.Status = ee.ExitCode()
		if res.Status < 0 {
			res.Status = 128 // killed by a signal
		}
		return res, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", i.Name, err)
	}
	return res, nil
}
