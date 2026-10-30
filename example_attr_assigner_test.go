package risor_test

import (
	"context"
	"fmt"
	"reflect"

	"github.com/deepnoodle-ai/risor/v2"
	"github.com/deepnoodle-ai/risor/v2/pkg/object"
)

type Job struct {
	ID    string
	Items []int
}

// JobObject wraps a Job. Its AssignAttr receives the VM context, so field
// assignment from a script uses the limits from risor.WithLimits.
type JobObject struct {
	*object.GoStruct
}

func (j *JobObject) AssignAttr(ctx context.Context, name string, value object.Object) error {
	if name == "ID" {
		return object.TypeErrorf("field %q is read-only", name)
	}
	return j.GoStruct.SetAttrContext(ctx, name, value)
}

// A host type that embeds *object.GoStruct implements object.AttrAssigner
// to apply the limits from risor.WithLimits to field assignment.
func Example_attrAssigner() {
	job := &Job{ID: "a1"}
	obj := &JobObject{object.NewGoStruct(reflect.ValueOf(job), object.DefaultRegistry())}
	env := risor.WithEnv(map[string]any{"job": obj})
	limits := risor.WithLimits(risor.Limits{MaxListLength: 3})

	for _, src := range []string{`job.Items = [1, 2, 3, 4]`, `job.Items = [1, 2, 3]`, `job.ID = "b2"`} {
		_, err := risor.Eval(context.Background(), src, env, limits)
		fmt.Println(src, "->", err)
	}
	fmt.Println(job.ID, job.Items)
	// Output:
	// job.Items = [1, 2, 3, 4] -> value error: cannot set field "Items": value too large to convert (exceeds limit of 3 elements) (1:1)
	// job.Items = [1, 2, 3] -> <nil>
	// job.ID = "b2" -> type error: field "ID" is read-only (1:1)
	// a1 [1 2 3]
}
