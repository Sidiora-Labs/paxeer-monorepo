package keeper_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm/types"

	snapshottypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/snapshots/types"
	storev2 "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/storev2/rootmulti"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
	wasmvm "github.com/Sidiora-Labs/Paxeer-X-Network/wasm-runtime"
	protoio "github.com/gogo/protobuf/io"
	"github.com/stretchr/testify/assert"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	cryptocodec "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/ed25519"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/wasm/app"
	"github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm/keeper"
)

func TestSnapshotter(t *testing.T) {
	specs := map[string]struct {
		wasmFiles []string
	}{
		"single contract": {
			wasmFiles: []string{"./testdata/reflect.wasm"},
		},
		"multiple contract": {
			wasmFiles: []string{"./testdata/reflect.wasm", "./testdata/burner.wasm", "./testdata/reflect.wasm"},
		},
		"duplicate contracts": {
			wasmFiles: []string{"./testdata/reflect.wasm", "./testdata/reflect.wasm"},
		},
	}
	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			// setup source app
			srcWasmApp, genesisAddr := newWasmExampleApp(t)

			// store wasm codes on chain (use cached deliver-state context so
			// writes are visible to subsequent reads within the same block)
			ctx := srcWasmApp.NewContext(false, tmproto.Header{
				ChainID: "foo",
				Height:  srcWasmApp.LastBlockHeight() + 1,
				Time:    time.Now(),
			})
			wasmKeeper := app.NewTestSupport(t, srcWasmApp).WasmKeeper()
			contractKeeper := keeper.NewDefaultPermissionKeeper(&wasmKeeper)

			srcCodeIDToChecksum := make(map[uint64][]byte, len(spec.wasmFiles))
			for i, v := range spec.wasmFiles {
				wasmCode, err := os.ReadFile(v)
				require.NoError(t, err)
				codeID, err := contractKeeper.Create(ctx, genesisAddr, wasmCode, nil)
				require.NoError(t, err)
				require.Equal(t, uint64(i+1), codeID)
				hash := sha256.Sum256(wasmCode)
				srcCodeIDToChecksum[codeID] = hash[:]
			}
			// create snapshot
			srcWasmApp.SetDeliverStateToCommit()
			srcWasmApp.Commit(context.Background())
			snapshotHeight := uint64(srcWasmApp.LastBlockHeight())
			snapshot, err := srcWasmApp.SnapshotManager().Create(snapshotHeight)
			require.NoError(t, err)
			assert.NotNil(t, snapshot)

			// when snapshot imported into dest app instance
			destWasmApp := app.SetupWithEmptyStore(t)
			require.NoError(t, destWasmApp.SnapshotManager().Restore(*snapshot))
			for i := uint32(0); i < snapshot.Chunks; i++ {
				chunkBz, err := srcWasmApp.SnapshotManager().LoadChunk(snapshot.Height, snapshot.Format, i)
				require.NoError(t, err)
				end, err := destWasmApp.SnapshotManager().RestoreChunk(chunkBz)
				require.NoError(t, err)
				if end {
					break
				}
			}

			// then all wasm contracts are imported
			wasmKeeper = app.NewTestSupport(t, destWasmApp).WasmKeeper()
			ctx = destWasmApp.NewUncachedContext(false, tmproto.Header{
				ChainID: "foo",
				Height:  destWasmApp.LastBlockHeight() + 1,
				Time:    time.Now(),
			})

			destCodeIDToChecksum := make(map[uint64][]byte, len(spec.wasmFiles))
			wasmKeeper.IterateCodeInfos(ctx, func(id uint64, info types.CodeInfo) bool {
				bz, err := wasmKeeper.GetByteCode(ctx, id)
				require.NoError(t, err)
				hash := sha256.Sum256(bz)
				destCodeIDToChecksum[id] = hash[:]
				assert.Equal(t, hash[:], info.CodeHash)
				return false
			})
			assert.Equal(t, srcCodeIDToChecksum, destCodeIDToChecksum)
		})
	}
}

func newWasmExampleApp(t *testing.T) (*app.WasmApp, sdk.AccAddress) {
	senderPrivKey := ed25519.GenPrivKey()
	pubKey, err := cryptocodec.ToTmPubKeyInterface(senderPrivKey.PubKey())
	require.NoError(t, err)

	senderAddr := senderPrivKey.PubKey().Address().Bytes()
	acc := authtypes.NewBaseAccount(senderAddr, senderPrivKey.PubKey(), 0, 0)
	amount, ok := sdk.NewIntFromString("10000000000000000000")
	require.True(t, ok)

	balance := banktypes.Balance{
		Address: acc.GetAddress().String(),
		Coins:   sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, amount)),
	}
	validator := tmtypes.NewValidator(pubKey, 1)
	valSet := tmtypes.NewValidatorSet([]*tmtypes.Validator{validator})
	wasmApp := app.SetupWithGenesisValSet(t, "testing", valSet, []authtypes.GenesisAccount{acc}, nil, balance)

	return wasmApp, senderAddr
}

func repairM1288SnapshotOpen(t *testing.T, dir string) (sdk.Context, keeper.TestKeepers, *wasmvm.VM, func()) {
	t.Helper()
	sc := config.DefaultStateCommitConfig()
	sc.WriteMode = config.MemiavlOnly
	sc.MemIAVLConfig.AsyncCommitBuffer = 0
	sc.MemIAVLConfig.SnapshotInterval = 1
	sc.MemIAVLConfig.SnapshotMinTimeInterval = 0
	ms := storev2.NewStore(dir, sc, config.StateStoreConfig{}, nil)
	vm, err := wasmvm.NewVM(filepath.Join(dir, "wasm"), keeper.SupportedFeatures, 32, false, 32)
	require.NoError(t, err)
	ctx, keepers := keeper.CreateTestInputWithMultiStore(t, keeper.SupportedFeatures, ms, keeper.WithWasmEngine(keeper.NewVMWrapper(vm)))
	closed := false
	closeStore := func() {
		if closed {
			return
		}
		closed = true
		vm.Cleanup()
		require.NoError(t, keepers.MultiStore.Close())
	}
	t.Cleanup(closeStore)
	return ctx, keepers, vm, closeStore
}

func repairM1288SnapshotRecovery(t *testing.T) {
	t.Helper()
	srcCtx, src, _, closeSource := repairM1288SnapshotOpen(t, t.TempDir())
	defer closeSource()
	admin, owner := keeper.RandomAccountAddress(t), keeper.RandomAccountAddress(t)
	id := keeper.StoreReflectContract(t, srcCtx, src)
	addr, _, err := src.ContractKeeper.Instantiate(srcCtx, id, admin, admin, []byte(`{}`), "snapshot state", nil)
	require.NoError(t, err)
	_, err = src.ContractKeeper.Execute(srcCtx, addr, admin, repair184JSON(t, map[string]interface{}{"change_owner": map[string]string{"owner": owner.String()}}), nil)
	require.NoError(t, err)
	require.NoError(t, keeper.NewGovPermissionKeeper(src.WasmKeeper).PinCode(srcCtx, id))
	srcCtx.MultiStore().(sdk.CacheMultiStore).Write()
	commit := src.MultiStore.Commit(true)
	srcCtx = srcCtx.WithMultiStore(src.MultiStore)
	height := uint64(commit.Version)
	var extension, state bytes.Buffer
	snapshotter := keeper.NewWasmSnapshotter(src.MultiStore, src.WasmKeeper)
	require.NoError(t, snapshotter.Snapshot(height, protoio.NewDelimitedWriter(&extension)), "SDK must supply the committed export consumed by the real Wasm extension")
	require.NoError(t, src.MultiStore.Snapshot(height, protoio.NewDelimitedWriter(&state)))

	destDir := t.TempDir()
	dstCtx, dst, _, closeDestination := repairM1288SnapshotOpen(t, destDir)
	_, err = dst.MultiStore.Restore(height, snapshottypes.CurrentFormat, protoio.NewDelimitedReader(bytes.NewReader(state.Bytes()), 64<<20))
	require.NoError(t, err)
	dstCtx = dstCtx.WithMultiStore(dst.MultiStore.CacheMultiStore())
	_, err = keeper.NewWasmSnapshotter(dst.MultiStore, dst.WasmKeeper).Restore(height, keeper.SnapshotFormat, protoio.NewDelimitedReader(bytes.NewReader(extension.Bytes()), 64<<20))
	require.NoError(t, err)
	require.Equal(t, src.WasmKeeper.GetContractInfo(srcCtx, addr), dst.WasmKeeper.GetContractInfo(dstCtx, addr))
	before, err := dst.WasmKeeper.QuerySmart(dstCtx, addr, []byte(`{"owner":{}}`))
	require.NoError(t, err)
	require.JSONEq(t, fmt.Sprintf(`{"owner":%q}`, owner.String()), string(before))
	dst.MultiStore.Commit(true)
	closeDestination()

	dstCtx, dst, vm, closeDestination := repairM1288SnapshotOpen(t, destDir)
	defer closeDestination()
	metrics, err := vm.GetMetrics()
	require.NoError(t, err)
	require.Zero(t, metrics.ElementsPinnedMemoryCache, "reopening creates a fresh native cache")
	require.True(t, dst.WasmKeeper.IsPinnedCode(dstCtx, id))
	require.NoError(t, dst.WasmKeeper.InitializePinnedCodes(dstCtx))
	metrics, err = vm.GetMetrics()
	require.NoError(t, err)
	require.Equal(t, uint64(1), metrics.ElementsPinnedMemoryCache)
	require.Positive(t, metrics.SizePinnedMemoryCache)
	before, err = dst.WasmKeeper.QuerySmart(dstCtx, addr, []byte(`{"owner":{}}`))
	require.NoError(t, err)
	require.JSONEq(t, fmt.Sprintf(`{"owner":%q}`, owner.String()), string(before))
	_, err = dst.ContractKeeper.Execute(dstCtx, addr, owner, repair184JSON(t, map[string]interface{}{"change_owner": map[string]string{"owner": admin.String()}}), nil)
	require.NoError(t, err)
	after, err := dst.WasmKeeper.QuerySmart(dstCtx, addr, []byte(`{"owner":{}}`))
	require.NoError(t, err)
	require.JSONEq(t, fmt.Sprintf(`{"owner":%q}`, admin.String()), string(after))
	metrics, err = vm.GetMetrics()
	require.NoError(t, err)
	require.Positive(t, metrics.HitsPinnedMemoryCache)
}
