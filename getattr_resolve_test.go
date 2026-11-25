package risor_test

import (
	"context"
	"errors"
	"testing"

	"github.com/deepnoodle-ai/risor/v2"
	"github.com/deepnoodle-ai/risor/v2/pkg/object"
	"github.com/deepnoodle-ai/risor/v2/pkg/op"
	"github.com/deepnoodle-ai/wonton/assert"
)

// lazyHost is a host type whose GetAttr returns a DynamicAttr.
type lazyHost struct{}

func (h *lazyHost) Type() object.Type               { return "lazy_host" }
func (h *lazyHost) Inspect() string                 { return "lazy_host()" }
func (h *lazyHost) Interface() interface{}          { return h }
func (h *lazyHost) Equals(other object.Object) bool { return h == other }
func (h *lazyHost) Attrs() []object.AttrSpec        { return nil }
func (h *lazyHost) IsTruthy() bool                  { return true }

func (h *lazyHost) SetAttr(name string, value object.Object) error {
	return errors.New("read only")
}

func (h *lazyHost) RunOperation(opType op.BinaryOpType, right object.Object) (object.Object, error) {
	return nil, errors.New("unsupported")
}

func (h *lazyHost) GetAttr(name string) (object.Object, bool) {
	switch name {
	case "name":
		return object.NewDynamicAttr(name, func(ctx context.Context, name string) (object.Object, error) {
			return object.NewString("resolved"), nil
		}), true
	case "broken":
		return object.NewDynamicAttr(name, func(ctx context.Context, name string) (object.Object, error) {
			return nil, errors.New("cannot resolve")
		}), true
	}
	return nil, false
}

// getattr resolves a DynamicAttr the same way obj.name does (issue #607).
func TestGetAttrResolvesDynamicAttr(t *testing.T) {
	ctx := context.Background()
	env := risor.WithEnv(map[string]any{
		"x":       &lazyHost{},
		"getattr": risor.Builtins()["getattr"],
	})

	for _, src := range []string{`x.name`, `getattr(x, "name")`, `getattr(x, "name", "default")`} {
		result, err := risor.Eval(ctx, src, env)
		assert.Nil(t, err, src)
		assert.Equal(t, result, "resolved", src)
	}

	for _, src := range []string{`x.broken`, `getattr(x, "broken")`} {
		_, err := risor.Eval(ctx, src, env)
		assert.NotNil(t, err, src)
		assert.Contains(t, err.Error(), "cannot resolve", src)
	}
}
