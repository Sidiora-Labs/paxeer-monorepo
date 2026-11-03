package keeper_test

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	rootmultiv2 "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/storev2/rootmulti"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	ics23 "github.com/confio/ics23/go"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestEvmStorageAppHashVector(t *testing.T) {
	configuration := config.DefaultStateCommitConfig()
	configuration.MemIAVLConfig.AsyncCommitBuffer = 0
	store := rootmultiv2.NewStore(t.TempDir(), configuration, config.StateStoreConfig{}, nil)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	app := testkeeper.EVMTestApp
	keeper := &app.EvmKeeper
	store.MountStoreWithDB(keeper.GetStoreKey(), storetypes.StoreTypeIAVL, nil)
	require.NoError(t, store.LoadLatestVersion())
	require.NoError(t, store.SetInitialVersion(1200))
	address := common.HexToAddress("0x1122334455667788990011223344556677889900")
	slot := common.HexToHash("0x0123456789abcdef")
	value := common.HexToHash("0x80ab")
	cache := store.CacheMultiStore()
	ctx := app.GetContextForDeliverTx(nil).WithMultiStore(cache).WithBlockHeight(1200)
	keeper.SetState(ctx, address, slot, value)
	require.Equal(t, value, keeper.GetState(ctx, address, slot))
	cache.Write()
	commit := store.Commit(true)
	key := append(evmtypes.StateKey(address), slot[:]...)
	response := store.Query(context.Background(), abci.RequestQuery{
		Path: "/" + keeper.GetStoreKey().Name() + "/key", Data: key, Height: commit.Version, Prove: true,
	})
	require.Zero(t, response.Code, response.Log)
	require.Equal(t, value[:], response.Value)
	require.NotNil(t, response.ProofOps)
	require.Len(t, response.ProofOps.Ops, 2)
	require.Equal(t, "ics23:iavl", response.ProofOps.Ops[0].Type)
	require.Equal(t, key, response.ProofOps.Ops[0].Key)
	require.Equal(t, "ics23:simple", response.ProofOps.Ops[1].Type)
	var leaf, outer ics23.CommitmentProof
	require.NoError(t, leaf.Unmarshal(response.ProofOps.Ops[0].Data))
	require.NoError(t, outer.Unmarshal(response.ProofOps.Ops[1].Data))
	require.NotNil(t, leaf.GetExist())
	require.NotNil(t, outer.GetExist())
	storeRoot := outer.GetExist().Value
	require.True(t, ics23.VerifyMembership(ics23.IavlSpec, storeRoot, &leaf, key, value[:]))
	require.True(t, ics23.VerifyMembership(ics23.TendermintSpec, commit.Hash, &outer, []byte("evm"), storeRoot))
	proof, err := leaf.GetExist().Marshal()
	require.NoError(t, err)
	vector := map[string]any{
		"source": map[string]any{
			"generator": "modules/sequencer/keeper/ics23_evm_storage_vector_test.go::TestEvmStorageAppHashVector",
			"version":   commit.Version,
		},
		"entries": []any{map[string]any{
			"address": hex.EncodeToString(address[:]), "slot": hex.EncodeToString(slot[:]),
			"value": hex.EncodeToString(value[:]), "key": hex.EncodeToString(key),
			"height": commit.Version, "app_hash": hex.EncodeToString(commit.Hash),
			"store_root": hex.EncodeToString(storeRoot), "proof": hex.EncodeToString(proof),
			"multistore_proof": hex.EncodeToString(response.ProofOps.Ops[1].Data),
			"response": map[string]any{
				"address": address, "hexValues": []string{hex.EncodeToString(response.Value)},
				"storageProof": []any{response.ProofOps},
			},
		}},
	}
	encoded, err := json.MarshalIndent(vector, "", "  ")
	require.NoError(t, err)
	encoded = append(encoded, '\n')
	path := filepath.Join("..", "..", "..", "programs/tests/vectors/ics23-evm-storage-app-hash.json")
	if os.Getenv("LXP_WRITE_VECTORS") == "1" {
		require.NoError(t, os.WriteFile(path, encoded, 0o644))
	}
	committed, err := os.ReadFile(path)
	require.NoError(t, err)
	require.True(t, bytes.Equal(committed, encoded), "regenerate with LXP_WRITE_VECTORS=1")
}
