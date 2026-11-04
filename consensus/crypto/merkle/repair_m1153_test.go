package merkle_test

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/ed25519"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/merkle"
	sm "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/state"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/light"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/light/provider"
	lighthttp "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/light/provider/http"
	lrpc "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/light/rpc"
	lightdb "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/light/store/db"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	tmcrypto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/crypto"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	rpcclient "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client"
	rpchttp "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client/http"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	rpcserver "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/server"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

// Produce the length-prefixed key/hash leaves consumed by ValueOp, then use
// the production Merkle tree, operation encoder and protobuf transport.
func repairM1153ValueProof(t *testing.T, key, value []byte, siblings int) ([]byte, tmcrypto.ProofOp) {
	t.Helper()
	hash := sha256.Sum256(value)
	leaf := binary.AppendUvarint(nil, uint64(len(key)))
	leaf = append(leaf, key...)
	leaf = binary.AppendUvarint(leaf, uint64(len(hash)))
	leaf = append(leaf, hash[:]...)
	leaves := [][]byte{leaf}
	for i := range siblings {
		leaves = append(leaves, []byte{byte(i)})
	}
	root, proofs := merkle.ProofsFromByteSlices(leaves)
	require.NoError(t, proofs[0].Verify(root, leaf))
	return root, merkle.NewValueOp(key, proofs[0]).ProofOp()
}

func repairM1153RoundTrip(t *testing.T, ops ...tmcrypto.ProofOp) *tmcrypto.ProofOps {
	t.Helper()
	proof := &tmcrypto.ProofOps{Ops: ops}
	wire, err := proof.Marshal()
	require.NoError(t, err)
	decoded := new(tmcrypto.ProofOps)
	require.NoError(t, decoded.Unmarshal(wire))
	return decoded
}

func TestRepairM_1153_MalformedProofs(t *testing.T) {
	runtime := merkle.DefaultProofRuntime()
	root := []byte("unproven root")
	for name, proof := range map[string]*tmcrypto.ProofOps{
		"nil": nil, "nil operations": {}, "empty operations": {Ops: []tmcrypto.ProofOp{}},
	} {
		t.Run(name, func(t *testing.T) {
			operators, err := runtime.DecodeProof(proof)
			require.Error(t, err)
			require.Nil(t, operators)
			require.Error(t, runtime.VerifyAbsence(proof, root, "/"))
			// Equal value/root and an empty key path must not bypass proof checking.
			require.Error(t, runtime.VerifyValue(proof, root, "/", root))
			for _, args := range [][][]byte{nil, {}, {root}, {root, []byte("extra")}} {
				require.Error(t, runtime.Verify(proof, root, "/", args))
			}
		})
	}
	for name, operators := range map[string]merkle.ProofOperators{
		"nil list": nil, "empty list": {}, "nil operator": {nil},
	} {
		t.Run(name, func(t *testing.T) {
			for _, args := range [][][]byte{nil, {}, {root}, {root, []byte("extra")}} {
				require.Error(t, operators.Verify(root, "/", args))
			}
			require.Error(t, operators.VerifyValue(root, "/", root))
		})
	}
}

func TestRepairM_1153_ValidProofVectors(t *testing.T) {
	runtime := merkle.DefaultProofRuntime()
	key, value := []byte("account/key"), []byte("committed value")
	for _, siblings := range []int{0, 1, 2, 3, 7} {
		root, op := repairM1153ValueProof(t, key, value, siblings)
		proof := repairM1153RoundTrip(t, op)
		path := merkle.KeyPath{}.AppendKey(key, merkle.KeyEncodingURL).String()
		require.NoError(t, runtime.VerifyValue(proof, root, path, value))
		operators, err := runtime.DecodeProof(proof)
		require.NoError(t, err)
		require.NoError(t, operators.Verify(root, path, [][]byte{value}))
		require.Error(t, runtime.VerifyValue(proof, root, path, []byte("changed")))
		require.Error(t, runtime.VerifyValue(proof, []byte("wrong root"), path, value))
		require.Error(t, runtime.VerifyValue(proof, root, "/wrong-key", value))
		require.Error(t, runtime.VerifyValue(proof, root, "/unconsumed"+path, value))
		require.Error(t, runtime.VerifyValue(proof, root, "invalid path", value))
		require.Error(t, runtime.VerifyAbsence(proof, root, path))
		require.Error(t, runtime.Verify(proof, root, path, [][]byte{value, value}))

		outerRoot, outer := repairM1153ValueProof(t, []byte("bank"), root, 2)
		chain := repairM1153RoundTrip(t, op, outer)
		require.NoError(t, runtime.VerifyValue(chain, outerRoot, "/bank"+path, value))
		require.Error(t, runtime.VerifyValue(chain, outerRoot, path, value))
		require.Error(t, runtime.VerifyValue(repairM1153RoundTrip(t, outer, op), outerRoot, "/bank"+path, value))
	}
}

// Anchor a real signed block in the real trusted store. The RPC consumer then
// obtains its AppHash through light.Client, without a LightClient test double.
func repairM1153TrustedBlock(t *testing.T, root []byte) *types.LightBlock {
	t.Helper()
	const chainID = "repair-m1153"
	key := ed25519.GenerateSecretKey()
	state, err := sm.MakeGenesisState(&types.GenesisDoc{
		ChainID: chainID, GenesisTime: time.Now().Add(-time.Minute).UTC(),
		Validators: []types.GenesisValidator{{Address: key.Public().Address(), PubKey: key.Public(), Power: 10}},
	})
	require.NoError(t, err)
	state.AppHash = root
	block := state.MakeBlockAt(2, nil, &types.Commit{}, nil, state.Validators.GetProposer().Address, state.LastBlockTime.Add(time.Second))
	parts, err := block.MakePartSet(types.BlockPartSizeBytes)
	require.NoError(t, err)
	id := types.BlockID{Hash: block.Hash(), PartSetHeader: parts.Header()}
	dir := t.TempDir()
	signer := privval.NewFilePV(key, filepath.Join(dir, "key.json"), filepath.Join(dir, "state.json"))
	votes := types.NewVoteSet(chainID, block.Height, 0, tmproto.PrecommitType, state.Validators)
	commit, err := types.MakeCommit(t.Context(), id, block.Height, 0, votes, []types.PrivValidator{signer}, block.Time)
	require.NoError(t, err)
	lb := &types.LightBlock{SignedHeader: &types.SignedHeader{Header: &block.Header, Commit: commit}, ValidatorSet: state.Validators.Copy()}
	require.NoError(t, lb.ValidateBasic(chainID))
	require.NoError(t, lb.ValidatorSet.VerifyCommitLight(chainID, id, block.Height, commit))
	return lb
}

func TestRepairM_1153_RemoteABCIResponses(t *testing.T) {
	key, value := []byte("requested"), []byte("committed value")
	innerRoot, inner := repairM1153ValueProof(t, key, value, 2)
	root, outer := repairM1153ValueProof(t, []byte("bank"), innerRoot, 1)
	valid := repairM1153RoundTrip(t, inner, outer)
	lb := repairM1153TrustedBlock(t, root)
	// Malformed transcripts are mutations of the production proof encoding.
	badData := inner
	badData.Data = []byte{0xff}
	unknown := inner
	unknown.Type = "unknown:proof"
	missingProof := merkle.NewValueOp(key, nil).ProofOp()
	for _, tc := range []struct {
		name  string
		value []byte
		proof *tmcrypto.ProofOps
		valid bool
	}{
		{"valid membership", value, valid, true},
		{"value nil proof", value, nil, false},
		{"value empty operations", value, &tmcrypto.ProofOps{Ops: []tmcrypto.ProofOp{}}, false},
		{"value malformed operation", value, repairM1153RoundTrip(t, badData, outer), false},
		{"value unknown operation", value, repairM1153RoundTrip(t, unknown, outer), false},
		{"value missing embedded proof", value, repairM1153RoundTrip(t, missingProof, outer), false},
		{"value tampered", []byte("changed"), valid, false},
		{"value incomplete path", value, repairM1153RoundTrip(t, inner), false},
		{"absence nil proof", nil, nil, false},
		{"absence empty operations", nil, &tmcrypto.ProofOps{Ops: []tmcrypto.ProofOp{}}, false},
		{"absence malformed operation", nil, repairM1153RoundTrip(t, badData, outer), false},
		{"absence unknown operation", nil, repairM1153RoundTrip(t, unknown, outer), false},
		{"absence missing embedded proof", nil, repairM1153RoundTrip(t, missingProof, outer), false},
		{"absence membership proof", nil, valid, false},
		{"valid membership after malformed responses", value, valid, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response := abci.ResponseQuery{Key: key, Value: tc.value, ProofOps: tc.proof, Height: 1}
			requests := make(chan coretypes.RequestABCIQuery, 2)
			mux := http.NewServeMux()
			rpcserver.RegisterRPCFuncs(mux, map[string]*rpcserver.RPCFunc{
				"abci_query": rpcserver.NewRPCFunc(func(_ context.Context, req *coretypes.RequestABCIQuery) (*coretypes.ResultABCIQuery, error) {
					requests <- *req
					return &coretypes.ResultABCIQuery{Response: response}, nil
				}),
			})
			server := httptest.NewServer(mux)
			t.Cleanup(server.Close)
			node, err := rpchttp.New(server.URL)
			require.NoError(t, err)
			p := lighthttp.NewWithClient(lb.ChainID, node)
			db := dbm.NewMemDB()
			t.Cleanup(func() { require.NoError(t, db.Close()) })
			trusted := lightdb.New(db)
			require.NoError(t, trusted.SaveLightBlock(lb))
			lc, err := light.NewClientFromTrustedStore(lb.ChainID, time.Hour, p, []provider.Provider{p}, trusted, time.Hour)
			require.NoError(t, err)
			consumer := lrpc.NewClient(node, lc, lrpc.KeyPathFn(lrpc.DefaultMerkleKeyPathFn()))
			options := rpcclient.ABCIQueryOptions{Height: 1, Prove: true}
			// Check the transcript actually crosses the production RPC codec.
			raw, err := node.ABCIQueryWithOptions(t.Context(), "/store/bank/key", key, options)
			require.NoError(t, err)
			require.Equal(t, key, raw.Response.Key)
			require.Equal(t, tc.value, raw.Response.Value)
			if tc.proof == nil {
				require.Nil(t, raw.Response.ProofOps)
			} else {
				require.NotNil(t, raw.Response.ProofOps)
				require.Equal(t, tc.proof.Ops, raw.Response.ProofOps.Ops)
			}
			result, err := consumer.ABCIQueryWithOptions(t.Context(), "/store/bank/key", key, options)
			if tc.valid {
				require.NoError(t, err)
				require.Equal(t, raw, result)
			} else {
				require.Error(t, err)
				require.Nil(t, result)
			}
			for range 2 {
				req := <-requests
				require.Equal(t, "/store/bank/key", req.Path)
				require.Equal(t, key, []byte(req.Data))
				require.EqualValues(t, 1, req.Height)
				require.True(t, req.Prove)
			}
		})
	}
}
