package layerxprograms_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"math/big"
	"sort"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	lp "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxprograms"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/storev2/rootmulti"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

func TestProgramViewsAnswerAtExplicitHeight(t *testing.T) {
	h := newHarness(t)
	program, authority, codeHash, _ := deployRegistryViewProgram(t, h, false)
	source := h.stateDB.Ctx()

	scConfig := config.DefaultStateCommitConfig()
	scConfig.MemIAVLConfig.AsyncCommitBuffer = 0
	ssConfig := config.DefaultStateStoreConfig()
	ssConfig.AsyncWriteBuffer = 0
	commits := rootmulti.NewStore(t.TempDir(), scConfig, ssConfig, nil)
	t.Cleanup(func() { require.NoError(t, commits.Close()) })

	keys := source.MultiStore().StoreKeys()
	seen := false
	for _, key := range keys {
		seen = seen || key == h.key
	}
	if !seen {
		keys = append(keys, h.key)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].Name() < keys[j].Name() })
	for _, key := range keys {
		switch key.(type) {
		case *sdk.KVStoreKey:
			commits.MountStoreWithDB(key, sdk.StoreTypeIAVL, nil)
		case *sdk.TransientStoreKey:
			commits.MountStoreWithDB(key, sdk.StoreTypeTransient, nil)
		case *sdk.MemoryStoreKey:
			commits.MountStoreWithDB(key, sdk.StoreTypeMemory, nil)
		default:
			t.Fatalf("unsupported store key %T", key)
		}
	}
	require.NoError(t, commits.LoadLatestVersion())
	require.NoError(t, commits.SetInitialVersion(10))
	for _, key := range keys {
		iterator := source.MultiStore().GetKVStore(key).Iterator(nil, nil)
		for ; iterator.Valid(); iterator.Next() {
			commits.GetKVStore(key).Set(bytes.Clone(iterator.Key()), bytes.Clone(iterator.Value()))
		}
		require.NoError(t, iterator.Close())
	}

	headKey := append([]byte{1, 0, 9}, []byte("progstor")...)
	headKey = append(headKey, program[:]...)
	headKey = append(headKey, 1)
	writeHead := func(value []byte) []byte {
		valueDigest := sha256.Sum256(value)
		manifest := binary.BigEndian.AppendUint16(nil, 1)
		manifest = binary.BigEndian.AppendUint32(manifest, 1)
		manifest = binary.BigEndian.AppendUint16(manifest, uint16(len("counter")))
		manifest = append(manifest, []byte("counter")...)
		manifest = append(manifest, valueDigest[:]...)
		manifest = binary.BigEndian.AppendUint32(manifest, uint32(len(value)))
		manifestDigest := sha256.Sum256(manifest)
		store := commits.GetKVStore(h.key)
		store.Set(append([]byte{5, 0, 9}, valueDigest[:]...), value)
		store.Set(append([]byte{5, 0, 9}, manifestDigest[:]...), manifest)
		head := append(bytes.Clone(manifest[:6]), manifestDigest[:]...)
		store.Set(headKey, head)
		return head
	}
	head10 := writeHead([]byte{1})
	commit10 := commits.Commit(true)
	require.Equal(t, int64(10), commit10.Version)
	head11 := writeHead([]byte{2})
	commit11 := commits.Commit(true)
	require.Equal(t, int64(11), commit11.Version)
	require.NotEqual(t, head10, head11)
	require.NotEqual(t, commit10.Hash, commit11.Hash)
	require.Equal(t, int64(11), commits.GetStateStore().GetLatestVersion())

	for _, tc := range []struct {
		name   string
		height int64
		head   []byte
	}{
		{"height_11", 11, head11},
		{"height_10", 10, head10},
		{"height_11_again", 11, head11},
		{"height_10_again", 10, head10},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stores, err := commits.CacheMultiStoreWithVersion(tc.height)
			require.NoError(t, err)
			ctx := source.WithMultiStore(stores).WithBlockHeight(tc.height).
				WithBlockTime(source.BlockTime().Add(time.Duration(tc.height-10) * time.Second)).
				WithEventManager(sdk.NewEventManager())
			ctx = ctx.WithGasMeter(sdk.NewInfiniteGasMeterWithMultiplier(ctx))
			stateDB := state.NewDBImpl(ctx, h.executor.TestEVMKeeper(), true)
			block := h.evm.Context
			block.BlockNumber = big.NewInt(tc.height)
			block.Time = uint64(ctx.BlockTime().Unix())
			chain := evmtypes.DefaultChainConfig().EthereumConfig(h.executor.TestEVMKeeper().ChainID(ctx))
			evm := vm.NewEVM(block, stateDB, chain, vm.Config{}, map[common.Address]vm.PrecompiledContract{
				h.precompile.Address(): h.precompile,
			})
			require.Equal(t, common.HexToAddress("0x101d"), h.precompile.Address())
			require.Equal(t, tc.height, stateDB.Ctx().BlockHeight())
			before := h.dump(stateDB.Ctx())
			for _, view := range []struct {
				name string
				want []interface{}
			}{
				{"getStateHead", []interface{}{tc.head}},
				{"getProgram", []interface{}{uint8(1), authority, codeHash, uint16(5), uint32(1), true}},
			} {
				input, err := lp.ProgramsABI.Pack(view.name, program)
				require.NoError(t, err)
				output, _, err := evm.StaticCall(h.caller, h.precompile.Address(), input, 2000000)
				require.NoError(t, err, "%s at height %d", view.name, tc.height)
				values, err := lp.ProgramsABI.Methods[view.name].Outputs.Unpack(output)
				require.NoError(t, err)
				require.Equal(t, view.want, values, "%s at height %d", view.name, tc.height)
			}
			require.Equal(t, before, h.dump(stateDB.Ctx()))
			require.Empty(t, stateDB.Ctx().EventManager().Events())
			require.Empty(t, stateDB.GetAllLogs())
			require.NoError(t, stateDB.GetPrecompileError())
		})
	}
}
