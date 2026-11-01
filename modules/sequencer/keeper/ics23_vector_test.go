package keeper

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	rootmultiv2 "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/storev2/rootmulti"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
	ics23 "github.com/confio/ics23/go"
	"github.com/stretchr/testify/require"
)

func TestReceiptKeysAppHashVector(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	corpus, err := os.ReadFile(filepath.Join(root, "sdk/platform/conformance/fixtures/receipt-sequencer-v1.json"))
	require.NoError(t, err)
	var receipts struct {
		Vectors []struct {
			ID      string `json:"id"`
			Height  uint64 `json:"height"`
			Receipt string `json:"receipt"`
			Keys    []struct {
				ID     string `json:"keyId"`
				Public string `json:"publicKey"`
				First  uint64 `json:"firstHeight"`
				Last   uint64 `json:"lastHeight"`
			} `json:"receiptKeys"`
		} `json:"vectors"`
	}
	require.NoError(t, json.Unmarshal(corpus, &receipts))
	decode := func(s string) []byte {
		t.Helper()
		decoded, err := hex.DecodeString(s[2:])
		require.NoError(t, err)
		return decoded
	}
	var keys []types.ReceiptKey
	var receipt []byte
	var height uint64
	for _, candidate := range receipts.Vectors {
		if candidate.ID != "receipted-primary-key" {
			continue
		}
		receipt, height = decode(candidate.Receipt), candidate.Height
		for _, pin := range candidate.Keys {
			key := types.ReceiptKey{FirstHeight: pin.First, LastHeight: pin.Last}
			copy(key.KeyId[:], decode(pin.ID))
			copy(key.PublicKey[:], decode(pin.Public))
			keys = append(keys, key)
		}
	}
	require.NotEmpty(t, receipt)
	require.NotEmpty(t, keys)
	configuration := config.DefaultStateCommitConfig()
	configuration.MemIAVLConfig.AsyncCommitBuffer = 0
	store := rootmultiv2.NewStore(t.TempDir(), configuration, config.StateStoreConfig{}, nil)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	key := storetypes.NewKVStoreKey(types.StoreKey)
	store.MountStoreWithDB(key, storetypes.StoreTypeIAVL, nil)
	require.NoError(t, store.LoadLatestVersion())
	require.NoError(t, store.SetInitialVersion(int64(height)))
	cache := store.CacheMultiStore()
	value := encodeReceiptKeys(keys)
	cache.GetKVStore(key).Set(types.ReceiptKeysKey, value)
	cache.Write()
	commit := store.Commit(true)
	response := store.Query(context.Background(), abci.RequestQuery{
		Path: "/sequencer/key", Data: types.ReceiptKeysKey, Height: commit.Version, Prove: true,
	})
	require.Zero(t, response.Code, response.Log)
	require.NotNil(t, response.ProofOps)
	require.Len(t, response.ProofOps.Ops, 2)
	var leaf, outer ics23.CommitmentProof
	require.NoError(t, leaf.Unmarshal(response.ProofOps.Ops[0].Data))
	require.NoError(t, outer.Unmarshal(response.ProofOps.Ops[1].Data))
	require.NotNil(t, leaf.GetExist())
	require.NotNil(t, outer.GetExist())
	storeRoot := outer.GetExist().Value
	require.True(t, ics23.VerifyMembership(ics23.IavlSpec, storeRoot, &leaf, types.ReceiptKeysKey, value))
	require.True(t, ics23.VerifyMembership(ics23.TendermintSpec, commit.Hash, &outer, []byte(types.StoreKey), storeRoot))
	proof, err := leaf.GetExist().Marshal()
	require.NoError(t, err)
	entry := map[string]any{
		"height": height, "key": hex.EncodeToString(types.ReceiptKeysKey),
		"value": hex.EncodeToString(value), "store_root": hex.EncodeToString(storeRoot),
		"app_hash": hex.EncodeToString(commit.Hash), "proof": hex.EncodeToString(proof),
		"multistore_proof": hex.EncodeToString(response.ProofOps.Ops[1].Data),
		"receipt":          hex.EncodeToString(receipt),
	}
	vector := map[string]any{
		"entries": []any{entry},
		"source": map[string]any{
			"generator":       "modules/sequencer/keeper/ics23_vector_test.go::TestReceiptKeysAppHashVector",
			"version":         commit.Version,
			"receipt_fixture": "sdk/platform/conformance/fixtures/receipt-sequencer-v1.json::receipted-primary-key",
		},
	}
	encoded, err := json.MarshalIndent(vector, "", "  ")
	require.NoError(t, err)
	encoded = append(encoded, '\n')
	path := filepath.Join(root, "programs/tests/vectors/ics23-receipt-keys-app-hash.json")
	if os.Getenv("LXP_WRITE_VECTORS") == "1" {
		require.NoError(t, os.WriteFile(path, encoded, 0o644))
	}
	committed, err := os.ReadFile(path)
	require.NoError(t, err)
	require.True(t, bytes.Equal(committed, encoded), "regenerate the committed host vector with LXP_WRITE_VECTORS=1")
}
