package genesis

import (
	"go/build"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEmbeddedGenesisRetiredChains(t *testing.T) {
	pkg, err := build.Default.ImportDir(".", 0)
	require.NoError(t, err)
	require.Empty(t, pkg.EmbedPatterns, "genesis must not embed any documents")
	require.NotContains(t, pkg.Imports, "embed")

	for _, chainID := range []string{"arctic-1", "atlantic-2", "pacific-1"} {
		t.Run(chainID, func(t *testing.T) {
			_, err := os.Stat(filepath.Join("chains", chainID+".json"))
			require.ErrorIs(t, err, os.ErrNotExist, "retired genesis document must not ship with the node")
		})
	}
}
