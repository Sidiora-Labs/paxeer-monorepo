package rootmulti_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/merkle"
	tmcrypto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/crypto"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/rootmulti"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	rootmultiv2 "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/storev2/rootmulti"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
	ics23 "github.com/confio/ics23/go"
	"github.com/stretchr/testify/require"
)

func TestChainProofVectorV1(t *testing.T) {
	scConfig := config.DefaultStateCommitConfig()
	scConfig.MemIAVLConfig.AsyncCommitBuffer = 0
	store := rootmultiv2.NewStore(t.TempDir(), scConfig, config.StateStoreConfig{}, nil)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	moduleKey := storetypes.NewKVStoreKey("bank")
	store.MountStoreWithDB(moduleKey, storetypes.StoreTypeIAVL, nil)
	require.NoError(t, store.LoadLatestVersion())

	presentKey, value := []byte("key"), []byte("value")
	cache := store.CacheMultiStore()
	cache.GetKVStore(moduleKey).Set(presentKey, value)
	cache.Write()
	commit := store.Commit(true)
	require.Equal(t, int64(1), commit.Version)
	require.Len(t, commit.Hash, 32)
	require.Equal(t, commit, store.LastCommitID())

	require.True(t, rootmulti.RequireProof("/key"))
	runtime := rootmulti.DefaultProofRuntime()
	var vector strings.Builder
	fmt.Fprintf(&vector, "version=1\nheight=%d\nmodule_key=%x\napp_hash=%x\n", commit.Version, moduleKey.Name(), commit.Hash)
	for _, tc := range []struct {
		name   string
		key    []byte
		value  []byte
		exists bool
	}{
		{name: "present", key: presentKey, value: value, exists: true},
		{name: "absent", key: []byte("missing"), exists: false},
	} {
		response := store.Query(context.Background(), abci.RequestQuery{
			Path: "/" + moduleKey.Name() + "/key", Data: tc.key, Height: commit.Version, Prove: true,
		})
		require.Zero(t, response.Code, "%s: %s", tc.name, response.Log)
		require.Equal(t, commit.Version, response.Height)
		require.Equal(t, tc.key, response.Key)
		require.Equal(t, tc.value, response.Value)
		require.NotNil(t, response.ProofOps)
		require.Len(t, response.ProofOps.Ops, 2)
		require.Equal(t, storetypes.ProofOpIAVLCommitment, response.ProofOps.Ops[0].Type)
		require.Equal(t, tc.key, response.ProofOps.Ops[0].Key)
		require.Equal(t, storetypes.ProofOpSimpleMerkleCommitment, response.ProofOps.Ops[1].Type)
		require.Equal(t, []byte(moduleKey.Name()), response.ProofOps.Ops[1].Key)

		var leaf, outer ics23.CommitmentProof
		require.NoError(t, leaf.Unmarshal(response.ProofOps.Ops[0].Data))
		require.NoError(t, outer.Unmarshal(response.ProofOps.Ops[1].Data))
		require.NotNil(t, outer.GetExist())
		if tc.exists {
			require.NotNil(t, leaf.GetExist())
		} else {
			require.NotNil(t, leaf.GetNonexist())
		}

		keypath := merkle.KeyPath{}.
			AppendKey([]byte(moduleKey.Name()), merkle.KeyEncodingHex).
			AppendKey(tc.key, merkle.KeyEncodingHex).String()
		if tc.exists {
			require.NoError(t, runtime.VerifyValue(response.ProofOps, commit.Hash, keypath, tc.value))
		} else {
			require.NoError(t, runtime.VerifyAbsence(response.ProofOps, commit.Hash, keypath))
		}

		encoded, err := response.ProofOps.Marshal()
		require.NoError(t, err)
		var decoded tmcrypto.ProofOps
		require.NoError(t, decoded.Unmarshal(encoded))
		wrongHash := append([]byte(nil), commit.Hash...)
		wrongHash[0] ^= 1
		if tc.exists {
			require.NoError(t, runtime.VerifyValue(&decoded, commit.Hash, keypath, tc.value))
			require.Error(t, runtime.VerifyValue(&decoded, wrongHash, keypath, tc.value))
		} else {
			require.NoError(t, runtime.VerifyAbsence(&decoded, commit.Hash, keypath))
			require.Error(t, runtime.VerifyAbsence(&decoded, wrongHash, keypath))
		}
		fmt.Fprintf(&vector, "%s_key=%x\n%s_value=%x\n%s_proof_ops=%x\n", tc.name, tc.key, tc.name, tc.value, tc.name, encoded)
	}

	computed := []byte(vector.String())
	path := filepath.Join("testdata", "chain_proof_v1.hex")
	if os.Getenv("LXP_WRITE_VECTORS") == "1" {
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, computed, 0o644))
	}
	committed, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, committed, computed, "chain proof vector changed; regenerate explicitly with LXP_WRITE_VECTORS=1")
}
