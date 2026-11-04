package app

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	chainstypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxchains/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/prefix"
	upgradetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/types"
	scconfig "github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm"
	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

// Exported only in the test binary, since the executor imports node itself.
func RepairM1141MigrationConfiguration(t *testing.T) {
	t.Helper()
	for _, tc := range []struct {
		name, value string
		want        int
		refused     bool
	}{
		{name: "absent", want: 1024},
		{name: "malformed", value: `"nonsense"`, refused: true},
		{name: "zero", value: "0", refused: true},
		{name: "negative", value: "-3", refused: true},
		{name: "positive", value: "17", want: 17},
		{name: "numeric string", value: `"23"`, want: 23},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := viper.New()
			opts.SetConfigType("toml")
			input := "[state-commit]\nsc-enable = true\nsc-write-mode = \"migrate_evm\"\n"
			if tc.value != "" {
				input += "sc-keys-to-migrate-per-block = " + tc.value + "\n"
			}
			require.NoError(t, opts.ReadConfig(strings.NewReader(input)))
			if tc.refused {
				var refusal any
				func() { defer func() { refusal = recover() }(); parseSCConfigs(opts) }()
				require.NotNil(t, refusal)
				err, ok := refusal.(error)
				require.True(t, ok)
				require.ErrorContains(t, err, FlagSCKeysToMigratePerBlock)
				require.Panics(t, func() { SetupPaxDB(t.TempDir(), opts, nil) })
				return
			}
			parsed := parseSCConfigs(opts)
			require.Equal(t, tc.want, parsed.KeysToMigratePerBlock)
			require.Equal(t, scconfig.MigrateEVM, parsed.WriteMode)
			require.NoError(t, parsed.Validate())
		})
	}
}

func RepairM1141UpgradeLifecycle(t *testing.T) {
	t.Helper()
	TestActivationUpgradeIsRegisteredBesideTheTagList(t)
	TestMergePlanIsRegisteredBesideTheTagList(t)
	a := NewTestWrapper(t, time.Now().UTC(), secp256k1.GenPrivKey().PubKey(), false).App
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	for _, plan := range knownUpgradePlans() {
		require.True(t, a.UpgradeKeeper.HasHandler(plan), plan)
	}
}

func RepairM1141ChainsActivation(t *testing.T) {
	t.Helper()
	w := NewTestWrapper(t, time.Now().UTC(), secp256k1.GenPrivKey().PubKey(), true)
	a, ctx := w.App, w.Ctx
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	addr := common.HexToAddress("0x101E")
	require.Equal(t, []string{chainstypes.ModuleName}, mergePrecompileGate().Modules[addr])
	require.Contains(t, a.EvmKeeper.CustomPrecompiles(ctx), addr)
	versions := a.UpgradeKeeper.GetModuleVersionMap(ctx)
	old := versions[chainstypes.ModuleName]
	require.NotZero(t, old)
	prefix.NewStore(ctx.KVStore(a.GetKey(upgradetypes.StoreKey)), []byte{upgradetypes.VersionMapByte}).Delete([]byte(chainstypes.ModuleName))
	require.NotContains(t, a.EvmKeeper.CustomPrecompiles(ctx), addr)
	versions[chainstypes.ModuleName] = old
	a.UpgradeKeeper.SetModuleVersionMap(ctx, versions)
	require.Contains(t, a.EvmKeeper.CustomPrecompiles(ctx), addr)
	a.UpgradeKeeper.SetDone(ctx.WithBlockHeight(10), MergeUpgrade)
	for _, tracing := range []bool{false, true} {
		require.NotContains(t, a.EvmKeeper.CustomPrecompiles(ctx.WithBlockHeight(9).WithIsTracing(tracing)), addr)
		for _, h := range []int64{10, 11} {
			require.Contains(t, a.EvmKeeper.CustomPrecompiles(ctx.WithBlockHeight(h).WithIsTracing(tracing)), addr)
		}
	}
}

func RepairM1141MergedStores(t *testing.T) {
	t.Helper()
	stores, ok := layerxStoreUpgrades(MergeUpgrade)
	require.True(t, ok)
	require.Equal(t, expectedMergeStores(), stores.Added)
	require.Empty(t, stores.Deleted)
	require.Empty(t, stores.Renamed)
	w := NewTestWrapper(t, time.Now().UTC(), secp256k1.GenPrivKey().PubKey(), false)
	a := w.App
	for _, name := range stores.Added {
		require.Contains(t, kvStoreKeyNames, name)
		require.NotNil(t, a.GetKey(name))
		require.NotNil(t, a.CommitMultiStore().GetCommitKVStore(a.GetKey(name)))
	}
	key, value := []byte("repair-m1141-preserved"), []byte("preexisting-state")
	w.Ctx.KVStore(a.GetKey("bank")).Set(key, value)
	for _, name := range stores.Added {
		w.Ctx.KVStore(a.GetKey(name)).Set(key, []byte(name))
	}
	info, err := a.UpgradeKeeper.GetUpgradeInfoPath()
	require.NoError(t, err)
	_, err = a.FinalizeBlock(context.Background(), &abci.RequestFinalizeBlock{Header: &tmproto.Header{ChainID: "pax-test", Height: 1, Time: w.Ctx.BlockTime()}})
	require.NoError(t, err)
	a.SetDeliverStateToCommit()
	_, err = a.Commit(context.Background())
	require.NoError(t, err)
	committed := a.LastCommitID()
	require.NoError(t, a.Close())
	home := filepath.Dir(filepath.Dir(info))
	reopened := New(dbm.NewMemDB(), nil, true, map[int64]bool{}, home, 1, false, config.TestConfig(), MakeEncodingConfig(), wasm.EnableAllProposals, TestAppOpts{}, EmptyWasmOpts, nil)
	t.Cleanup(func() { require.NoError(t, reopened.Close()) })
	require.Equal(t, committed, reopened.LastCommitID())
	ctx := reopened.GetCheckCtx()
	require.Equal(t, value, ctx.KVStore(reopened.GetKey("bank")).Get(key))
	for _, name := range stores.Added {
		require.Equal(t, []byte(name), ctx.KVStore(reopened.GetKey(name)).Get(key), name)
	}
}

func RepairM1141InitializationFailure(t *testing.T) {
	t.Helper()
	a := Setup(t, true, false, false)
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	// The real InitChainer panics on corrupt serialized genesis; the helper must expose it.
	require.Panics(t, func() { _ = initializeTestChain(a, []byte("{")) })
}
