package http

import (
	"testing"

	"github.com/risor-io/risor/object"
	"github.com/stretchr/testify/require"
)

func TestModule(t *testing.T) {
	m := Module()
	require.NotNil(t, m)
	reqObj, ok := m.GetAttr("request")
	require.True(t, ok)
	req, ok := reqObj.(*object.Builtin)
	require.True(t, ok)
	require.Equal(t, "http.request", req.Name())

	// Web server functionality removed in v2
	_, ok = m.GetAttr("listen_and_serve")
	require.False(t, ok)
}
