package keeper

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	rootmultiv2 "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/storev2/rootmulti"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
	ics23 "github.com/confio/ics23/go"
	"github.com/stretchr/testify/require"
)

func TestModuleCatalogueAppHashVector(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	fixture := "programs/fixtures/pay5/token-lxt20.wasm"
	blob, err := os.ReadFile(filepath.Join(root, fixture))
	require.NoError(t, err)
	require.NotEmpty(t, blob)
	codeHash := sha256.Sum256(blob)
	activityID := sha256.Sum256([]byte("module catalogue app-hash activity"))
	// moduleKVKey(9, programKey("progreplay/v1/", activityID)) and blobKey(9, codeHash).
	metadataKey := append([]byte{1, 0, 9}, []byte("progreplay/v1/")...)
	metadataKey = append(metadataKey, activityID[:]...)
	blobKey := append([]byte{5, 0, 9}, codeHash[:]...)
	configuration := config.DefaultStateCommitConfig()
	configuration.MemIAVLConfig.AsyncCommitBuffer = 0
	store := rootmultiv2.NewStore(t.TempDir(), configuration, config.StateStoreConfig{}, nil)
	hostKey := storetypes.NewKVStoreKey("lxphost")
	store.MountStoreWithDB(hostKey, storetypes.StoreTypeIAVL, nil)
	require.NoError(t, store.LoadLatestVersion())
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	require.NoError(t, store.SetInitialVersion(1200))
	var entries []any
	for i := uint64(0); i < 2; i++ {
		fields := map[string]any{}
		metadata := []byte("LXP/program-replay-native/v1\x00")
		metadata = binary.BigEndian.AppendUint16(metadata, 1)
		u16 := func(name string, value uint16) {
			fields[name] = value
			metadata = binary.BigEndian.AppendUint16(metadata, value)
		}
		u32 := func(name string, value uint32) {
			fields[name] = value
			metadata = binary.BigEndian.AppendUint32(metadata, value)
		}
		hash := func(name string, value [32]byte) {
			fields[name] = hex.EncodeToString(value[:])
			metadata = append(metadata, value[:]...)
		}
		u32("network_id", 125)
		hash("activity_id", activityID)
		fields["sequence"] = 7 + i
		metadata = binary.BigEndian.AppendUint64(metadata, 7+i)
		hash("previous_root", sha256.Sum256([]byte("previous root")))
		hash("program_id", sha256.Sum256([]byte("pay5 token program")))
		hash("code_hash", codeHash)
		hash("input_digest", sha256.Sum256([]byte("token call input")))
		u16("runtime_version", 1)
		u16("abi_version", 1)
		u32("fee_version", 1)
		u32("metering_version", 1)
		metadata = binary.BigEndian.AppendUint16(metadata, 1)
		u32("maximum_boundaries", 4096)
		u32("maximum_bytes", 1048576)
		fields["terminal_status"] = 1
		metadata = append(metadata, 1)
		u32("boundary_count", 3)
		for _, name := range []string{"authority_root", "hosts_root", "boundary_root", "witness_digest"} {
			hash(name, sha256.Sum256([]byte(name)))
		}
		fields["result_code"] = -7
		metadata = binary.BigEndian.AppendUint32(metadata, 0xfffffff9)
		hash("blob_digest", sha256.Sum256([]byte("replay blob")))
		require.Len(t, metadata, 394)
		cache := store.CacheMultiStore()
		host := cache.GetKVStore(hostKey)
		host.Set(metadataKey, metadata)
		if i == 0 {
			host.Set(blobKey, blob)
		}
		cache.Write()
		commit := store.Commit(true)
		query := func(key, value []byte) (map[string]any, []byte, []byte) {
			response := store.Query(context.Background(), abci.RequestQuery{Path: "/lxphost/key", Data: key, Height: commit.Version, Prove: true})
			require.Zero(t, response.Code, response.Log)
			require.Equal(t, value, response.Value, "keeper must retain both lxphost entries")
			require.NotNil(t, response.ProofOps)
			require.Len(t, response.ProofOps.Ops, 2)
			var leaf, outer ics23.CommitmentProof
			require.NoError(t, leaf.Unmarshal(response.ProofOps.Ops[0].Data))
			require.NoError(t, outer.Unmarshal(response.ProofOps.Ops[1].Data))
			require.NotNil(t, leaf.GetExist())
			require.NotNil(t, outer.GetExist())
			storeRoot := outer.GetExist().Value
			require.True(t, ics23.VerifyMembership(ics23.IavlSpec, storeRoot, &leaf, key, value))
			require.True(t, ics23.VerifyMembership(ics23.TendermintSpec, commit.Hash, &outer, []byte("lxphost"), storeRoot))
			proof, err := leaf.GetExist().Marshal()
			require.NoError(t, err)
			return map[string]any{"key": hex.EncodeToString(key), "value": hex.EncodeToString(value), "proof": hex.EncodeToString(proof)}, storeRoot, response.ProofOps.Ops[1].Data
		}
		metadataEntry, storeRoot, multistoreProof := query(metadataKey, metadata)
		metadataEntry["fields"] = fields
		blobEntry, blobRoot, blobOuter := query(blobKey, blob)
		require.Equal(t, storeRoot, blobRoot)
		require.Equal(t, multistoreProof, blobOuter)
		blobEntry["code_hash"] = hex.EncodeToString(codeHash[:])
		entries = append(entries, map[string]any{
			"height": commit.Version, "app_hash": hex.EncodeToString(commit.Hash),
			"store_root": hex.EncodeToString(storeRoot), "multistore_proof": hex.EncodeToString(multistoreProof),
			"metadata": metadataEntry, "blobs": []any{blobEntry},
		})
	}
	require.NotEqual(t, entries[0].(map[string]any)["app_hash"], entries[1].(map[string]any)["app_hash"])
	vector := map[string]any{"entries": entries, "source": map[string]any{
		"generator":    "modules/sequencer/keeper/ics23_module_catalogue_vector_test.go::TestModuleCatalogueAppHashVector",
		"code_fixture": fixture,
	}}
	encoded, err := json.MarshalIndent(vector, "", "  ")
	require.NoError(t, err)
	encoded = append(encoded, '\n')
	path := filepath.Join(root, "programs/tests/vectors/ics23-module-catalogue-app-hash.json")
	if os.Getenv("LXP_WRITE_VECTORS") == "1" {
		require.NoError(t, os.WriteFile(path, encoded, 0o644))
	}
	committed, err := os.ReadFile(path)
	require.NoError(t, err)
	require.True(t, bytes.Equal(committed, encoded), "regenerate with LXP_WRITE_VECTORS=1")
}
