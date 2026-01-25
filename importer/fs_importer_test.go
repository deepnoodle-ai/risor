package importer

import (
	"context"
	"os"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"
)

func TestFSImporter_Import(t *testing.T) {
	// Create test filesystem with the bar.risor fixture
	testFS := fstest.MapFS{
		"foo/bar.risor": &fstest.MapFile{
			Data: []byte(`
function test_function() {
    return 765
}
`),
		},
		"example.rriissoorr": &fstest.MapFile{
			Data: []byte(`function hello() { return "world" }`),
		},
	}

	t.Run("successfully imports existing module", func(t *testing.T) {
		importer := NewFSImporter(FSImporterOptions{
			SourceFS: testFS,
		})

		module, err := importer.Import(context.Background(), "foo/bar")
		require.NoError(t, err)
		require.NotNil(t, module)
		require.Equal(t, "foo/bar", module.Name().Value())
		require.NotNil(t, module.Code())

		code := module.Code()
		require.Equal(t, []string{"test_function"}, code.GlobalNames())
	})

	t.Run("returns error for nonexistent module", func(t *testing.T) {
		importer := NewFSImporter(FSImporterOptions{
			SourceFS: testFS,
		})

		module, err := importer.Import(context.Background(), "nonexistent")
		require.Error(t, err)
		require.Nil(t, module)
		require.Contains(t, err.Error(), "module \"nonexistent\" not found")
	})

	t.Run("uses custom extensions", func(t *testing.T) {
		importer := NewFSImporter(FSImporterOptions{
			SourceFS:   testFS,
			Extensions: []string{".rriissoorr"},
		})

		module, err := importer.Import(context.Background(), "example")
		require.NoError(t, err)
		require.NotNil(t, module)
		require.Equal(t, "example", module.Name().Value())
	})
}

func TestFSImporter_WithRealFixtures(t *testing.T) {
	// Create FSImporter using the fixtures directory in the current directory
	importer := NewFSImporter(FSImporterOptions{
		SourceFS: os.DirFS("fixtures"),
	})

	// Import the bar module from fixtures
	module, err := importer.Import(context.Background(), "foo/bar")
	require.NoError(t, err)
	require.NotNil(t, module)
	require.Equal(t, "foo/bar", module.Name().Value())
	require.NotNil(t, module.Code())

	code := module.Code()
	require.Equal(t, []string{"bar"}, code.GlobalNames())
}
