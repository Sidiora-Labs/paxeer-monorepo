package light_test

import (
	"bytes"
	"context"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/ed25519"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/rpc/core"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/light"
	dbs "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/light/store/db"
	rpcserver "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/server"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

// Keep this compiled, executed example identical to the package documentation.
func repairM1111TrustedLightBlock(ctx context.Context, chainID, dbDir, primary string,
	witnesses []string, header *types.Header) (*types.LightBlock, error) {
	db, err := dbm.NewGoLevelDB("light-client-db", dbDir)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	blacklistTTL := 5 * time.Minute
	c, err := light.NewHTTPClient(
		ctx,
		chainID,
		light.TrustOptions{
			Period: 504 * time.Hour, // 21 days
			Height: header.Height,
			Hash:   header.Hash(),
		},
		primary,
		witnesses,
		dbs.New(db),
		blacklistTTL,
	)
	if err != nil {
		return nil, err
	}

	return c.TrustedLightBlock(header.Height)
}

func TestRepairM_1111_PublishedExampleMatchesCompiledPath(t *testing.T) {
	source, err := os.ReadFile("doc.go")
	require.NoError(t, err)
	// Extract the actual Go doc code block, including imports, rather than a
	// second textual assertion of which constructor signatures should work.
	doc := string(source)
	start := strings.Index(doc, "\timport (")
	require.NotEqual(t, -1, start)
	end := strings.Index(doc[start:], "\nCheck out other examples")
	require.NotEqual(t, -1, end)
	example := "package example\n" + doc[start:start+end]
	fset := token.NewFileSet()
	published, err := parser.ParseFile(fset, "published.go", example, 0)
	require.NoError(t, err)
	compiled, err := parser.ParseFile(fset, "repair_m1111_test.go", nil, 0)
	require.NoError(t, err)

	function := func(file *ast.File, name string) *ast.FuncDecl {
		for _, decl := range file.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == name {
				return fn
			}
		}
		t.Fatalf("missing function %s", name)
		return nil
	}
	written := function(published, "trustedLightBlock")
	executed := function(compiled, "repairM1111TrustedLightBlock")
	executed.Name.Name = written.Name.Name
	var writtenCode, executedCode bytes.Buffer
	require.NoError(t, format.Node(&writtenCode, fset, written))
	require.NoError(t, format.Node(&executedCode, fset, executed))
	require.Equal(t, executedCode.String(), writtenCode.String(),
		"the published example must match the compiled function exercised through HTTP")

	// The import aliases are part of the published example's compilation contract.
	for _, want := range published.Imports {
		matched := false
		for _, got := range compiled.Imports {
			if got.Path.Value == want.Path.Value {
				if want.Name == nil {
					require.Nil(t, got.Name)
				} else {
					require.NotNil(t, got.Name)
					require.Equal(t, want.Name.Name, got.Name.Name)
				}
				matched = true
			}
		}
		require.True(t, matched, "uncompiled example import %s", want.Path.Value)
	}
	require.Contains(t, doc, "Tendermint/CometBFT signed")
	require.Contains(t, doc, "validators RPC")
	require.Contains(t, doc, "ErrCommitNotAvailable")
	require.Contains(t, doc, "it does not verify Autobahn finality")
}

func repairM1111Endpoint(t *testing.T, env *core.Environment) (string, *atomic.Int64) {
	t.Helper()
	mux := http.NewServeMux()
	rpcserver.RegisterRPCFuncs(mux, map[string]*rpcserver.RPCFunc{
		"commit":     rpcserver.NewRPCFunc(env.Commit),
		"validators": rpcserver.NewRPCFunc(env.Validators),
	})
	requests := new(atomic.Int64)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		mux.ServeHTTP(w, r)
	}))
	t.Cleanup(server.Close)
	return server.URL, requests
}

func TestRepairM_1111_DocumentedHTTPClientRoundTrip(t *testing.T) {
	key := ed25519.TestSecretKey([]byte("repair-m1111-signed-commit-source"))
	blocks := repair83Produce(t, key, hash("repair-m1111-canonical"), 100)
	root := blocks[0]
	primary, primaryRequests := repairM1111Endpoint(t, repair83Environment(t, root))
	witness, witnessRequests := repairM1111Endpoint(t, repair83Environment(t, root))
	dir := t.TempDir()

	got, err := repairM1111TrustedLightBlock(t.Context(), chainID, dir,
		primary, []string{witness}, root.Header)
	require.NoError(t, err)
	require.Equal(t, root.Height, got.Height)
	require.Equal(t, root.Hash(), got.Hash())
	require.Equal(t, root.ValidatorSet.Hash(), got.ValidatorSet.Hash())
	require.NoError(t, got.ValidateBasic(chainID))
	require.NoError(t, got.ValidatorSet.VerifyCommitLight(chainID, got.Commit.BlockID, got.Height, got.Commit))
	require.GreaterOrEqual(t, primaryRequests.Load(), int64(2), "fetch both the signed commit and validator set")
	require.GreaterOrEqual(t, witnessRequests.Load(), int64(2), "cross-check the independent witness")

	// The example closes the actual LevelDB; reopening proves that retrieval
	// used the production trusted store and preserved its protobuf payload.
	db, err := dbm.NewGoLevelDB("light-client-db", dir)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	store := dbs.New(db)
	require.EqualValues(t, 1, store.Size())
	saved, err := store.LightBlock(root.Height)
	require.NoError(t, err)
	require.Equal(t, got, saved)

	// A separately produced signed header cannot replace the caller's
	// independently trusted hash, even when every HTTP peer agrees.
	otherRoot := repair83Produce(t, key, hash("repair-m1111-other-commitment"), 100)[0]
	require.NotEqual(t, root.Hash(), otherRoot.Hash())
	rejectedDir := t.TempDir()
	rejected, err := repairM1111TrustedLightBlock(t.Context(), chainID, rejectedDir,
		primary, []string{witness}, otherRoot.Header)
	require.Error(t, err)
	require.Nil(t, rejected)
	rejectedDB, err := dbm.NewGoLevelDB("light-client-db", rejectedDir)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, rejectedDB.Close()) })
	rejectedStore := dbs.New(rejectedDB)
	require.Zero(t, rejectedStore.Size())
	lastHeight, err := rejectedStore.LastLightBlockHeight()
	require.NoError(t, err)
	require.EqualValues(t, -1, lastHeight)
}
