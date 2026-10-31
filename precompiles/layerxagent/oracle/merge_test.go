package oracle_test

import (
	"math/big"
	"testing"

	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	agentcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/oracle"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/utils"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

func TestOracleMergeConstructorCaps(t *testing.T) {
	require.Equal(t, 32, oracle.MergeMaxFeeds)
	cap := big.NewInt(1_000_000_000_000_000_000)
	require.Equal(t, cap, oracle.MergeMaxConfidence)

	testApp := app.Setup(t, false, true, false)
	keepers := app.NewPrecompileKeepers(testApp)
	missingEVM := *keepers
	missingEVM.EVMKeeper = nil
	nilEVM := *keepers
	nilEVM.EVMKeeper = (*evmkeeper.Keeper)(nil)
	for _, tc := range []struct {
		name    string
		keepers utils.Keepers
	}{
		{"nil keepers", nil},
		{"empty keepers", &utils.EmptyKeepers{}},
		{"missing exchange", &app.PrecompileKeepers{EVMKeeper: &testApp.EvmKeeper}},
		{"missing evm", &missingEVM},
		{"nil evm", &nilEVM},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, err := oracle.NewMergePrecompile(tc.keepers, precompiles.MergeUpgrade)
			require.Nil(t, p)
			require.ErrorIs(t, err, agentcommon.ErrNoContext)
		})
	}
	missingUpgrade := *keepers
	missingUpgrade.EVMKeeper = new(evmkeeper.Keeper)
	p, err := oracle.NewMergePrecompile(&missingUpgrade, precompiles.MergeUpgrade)
	require.Nil(t, p)
	require.ErrorIs(t, err, agentcommon.ErrNotActive)
	p, err = oracle.NewMergePrecompile(keepers, "")
	require.Nil(t, p)
	require.ErrorIs(t, err, agentcommon.ErrNotActive)
	p, err = oracle.NewMergePrecompile(keepers, precompiles.MergeUpgrade)
	require.NoError(t, err)
	require.NotNil(t, p)

	address := common.HexToAddress("0x0000000000000000000000000000000000000903")
	set, err := precompiles.GetCustomPrecompilesE(app.LatestUpgrade, keepers)
	require.NoError(t, err)
	versions, found := set[address]
	require.True(t, found)
	served, ok := versions[precompiles.MergeUpgrade].(*agentcommon.Precompile)
	require.True(t, ok)
	require.Same(t, served, versions[app.LatestUpgrade])
	require.Equal(t, address, served.Address())
	require.Equal(t, "oracle aggregator", served.GetName())
	require.EqualError(t, precompiles.RegisterMergeConstructor(address, oracle.NewMergePrecompile),
		"merge constructor for "+address.Hex()+" already registered")

	k := &testApp.EvmKeeper
	k.SetCustomPrecompileActivation([]evmkeeper.CustomPrecompileActivation{{
		Upgrade: precompiles.MergeUpgrade,
		Modules: map[common.Address][]string{address: {"layerxagent"}},
	}})
	k.SetCustomPrecompiles(set, app.LatestUpgrade)
	ctx := testApp.GetContextForDeliverTx([]byte{}).WithBlockHeight(20)
	ctx = ctx.WithGasMeter(sdk.NewInfiniteGasMeterWithMultiplier(ctx))
	k.UpgradeKeeper().SetDone(ctx, precompiles.MergeUpgrade)
	require.NotContains(t, k.CustomPrecompiles(ctx.WithBlockHeight(19)), address)
	require.Same(t, served, k.CustomPrecompiles(ctx)[address])

	for _, tc := range []struct {
		name       string
		count      int
		confidence *big.Int
		height     int64
		refusal    agentcommon.ErrorCode
	}{
		{"before activation", 1, cap, 19, agentcommon.ErrNotActive},
		{"one feed", 1, big.NewInt(1), 20, ""},
		{"at both caps", 32, cap, 20, ""},
		{"above feed cap", 33, cap, 20, agentcommon.ErrBatchTooLarge},
		{"above confidence cap", 1, new(big.Int).Add(cap, big.NewInt(1)), 20, agentcommon.ErrOracleConfidence},
	} {
		t.Run(tc.name, func(t *testing.T) {
			feeds := make([]oracle.Feed, tc.count)
			for i := range feeds {
				feeds[i] = oracle.Feed{Price: big.NewInt(100), Confidence: tc.confidence, Timestamp: big.NewInt(1)}
			}
			input, err := served.GetABI().Pack(oracle.AggregateMethod, feeds)
			require.NoError(t, err)
			db := state.NewDBImpl(ctx.WithBlockHeight(tc.height), k, false)
			data, err := served.Run(&vm.EVM{StateDB: db}, common.Address{}, address, input, nil, true, false, nil)
			if tc.refusal != "" {
				require.ErrorIs(t, err, vm.ErrExecutionReverted)
				require.ErrorIs(t, db.GetPrecompileError(), tc.refusal)
				require.Contains(t, revertText(t, data), string(tc.refusal))
				return
			}
			require.NoError(t, err)
			values, err := served.GetABI().Unpack(oracle.AggregateMethod, data)
			require.NoError(t, err)
			require.Equal(t, []interface{}{big.NewInt(100), tc.confidence}, values)
		})
	}
}
