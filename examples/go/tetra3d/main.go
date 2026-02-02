package main

import (
	"context"
	"log"

	"github.com/deepnoodle-ai/risor/v2"
	"github.com/solarlune/tetra3d"
)

type Engine struct{}

func (e *Engine) NewVector(x, y, z float32) tetra3d.Vector3 {
	return tetra3d.NewVector3(x, y, z)
}

func main() {
	src := `
	a := Engine.NewVector(4,5,6)
	b := Engine.NewVector(1,2,3)
	c := a.Add(b) // This works now, which is great!
	c.X = 15 // Set field X
	// Return the result (print not available in sandboxed mode)
	{"x": c.X, "vector": c}
	`
	_, err := risor.Eval(context.Background(), src, risor.WithEnv(map[string]any{"Engine": &Engine{}}))
	if err != nil {
		log.Fatal(err)
	}
}
