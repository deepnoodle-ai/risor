package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/deepnoodle-ai/risor/v2"
)

type State struct {
	Name    string
	Running bool
}

type Service struct {
	name       string
	running    bool
	startCount int
	stopCount  int
}

func (s *State) IsRunning() bool {
	return s.Running
}

func (s *Service) Start() error {
	if s.running {
		return fmt.Errorf("service %s already running", s.name)
	}
	s.running = true
	s.startCount++
	return nil
}

func (s *Service) Stop() error {
	if !s.running {
		return fmt.Errorf("service %s not running", s.name)
	}
	s.running = false
	s.stopCount++
	return nil
}

func (s *Service) SetName(name string) {
	s.name = name
}

func (s *Service) GetName() string {
	return s.name
}

func (s *Service) PrintState() {
	fmt.Printf("printing state... name: %s running %t\n", s.name, s.running)
}

func (s *Service) GetState() *State {
	return &State{
		Name:    s.name,
		Running: s.running,
	}
}

const defaultExample = `
svc.SetName("My Service")
svc.Start()
state := svc.GetState()
// Return a map with state info (print not available in sandboxed mode)
{"state": state, "type": type(state), "running": state.IsRunning()}
`

var red = color.New(color.FgRed).SprintfFunc()

func main() {
	var code string
	flag.StringVar(&code, "code", defaultExample, "Code to evaluate")
	flag.Parse()

	ctx := context.Background()

	// Initialize the service
	svc := &Service{}

	// Run the Risor code which can access the service as `svc`
	result, err := risor.Eval(ctx, code, risor.WithEnv(map[string]any{"svc": svc}))
	if err != nil {
		fmt.Println(red(err.Error()))
		os.Exit(1)
	}
	fmt.Println("RESULT:", result)
}
