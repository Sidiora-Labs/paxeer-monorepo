package distribution_test

import (
	"math/big"
	"testing"
	"time"

	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	minttypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/mint/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/distribution"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	distrtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/distribution/types"
	stakingtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

func TestDistributionGasGolden(t *testing.T) {
	const (
		planHeight  int64  = 10
		suppliedGas uint64 = 1_000_000
	)
	testApp := testkeeper.EVMTestApp
	ctx, _ := testApp.NewContext(false, tmtypes.Header{
		Height: 1,
		Time:   time.Unix(0, 0).UTC(),
	}).CacheContext()
	ctx = ctx.WithGasMeter(sdk.NewInfiniteGasMeter(1, 1))
	k := &testApp.EvmKeeper
	testApp.UpgradeKeeper.SetDone(ctx.WithBlockHeight(planHeight), pcommon.MergeUpgrade)
	require.Equal(t, planHeight, testApp.UpgradeKeeper.GetDoneHeight(ctx, pcommon.MergeUpgrade))

	key := &secp256k1.PrivKey{Key: make([]byte, 32)}
	key.Key[31] = 1
	validator := setupValidator(t, ctx, testApp, stakingtypes.Unbonded, key.PubKey())
	delegator := sdk.AccAddress(validator)
	caller := common.HexToAddress("0x1111111111111111111111111111111111111111")
	withdrawAddress := common.HexToAddress("0x2222222222222222222222222222222222222222")
	withdrawAccount := sdk.AccAddress(common.HexToAddress("0x3333333333333333333333333333333333333333").Bytes())
	k.SetAddressMapping(ctx, delegator, caller)
	k.SetAddressMapping(ctx, withdrawAccount, withdrawAddress)
	params := testApp.DistrKeeper.GetParams(ctx)
	params.WithdrawAddrEnabled = true
	testApp.DistrKeeper.SetParams(ctx, params)
	commission := sdk.NewDecCoins(sdk.NewDecCoinFromDec("uhpx", sdk.NewDecWithPrec(5, 1)))
	testApp.DistrKeeper.SetValidatorAccumulatedCommission(ctx, validator, distrtypes.ValidatorAccumulatedCommission{Commission: commission})
	testApp.DistrKeeper.SetValidatorOutstandingRewards(ctx, validator, distrtypes.ValidatorOutstandingRewards{Rewards: commission})
	backing := sdk.NewCoins(sdk.NewInt64Coin("uhpx", 1))
	require.NoError(t, testApp.BankKeeper.MintCoins(ctx, minttypes.ModuleName, backing))
	require.NoError(t, testApp.BankKeeper.SendCoinsFromModuleToModule(ctx, minttypes.ModuleName, distrtypes.ModuleName, backing))

	precompile, err := distribution.NewPrecompile(&app.PrecompileKeepers{
		DistributionKeeper: testApp.DistrKeeper,
		EVMKeeper:          k,
	})
	require.NoError(t, err)
	precompile.WithLXPReasons()

	golden := []struct {
		method string
		args   []interface{}
		gas    uint64
	}{
		{"rewards", []interface{}{caller}, 19_172},
		{"setWithdrawAddress", []interface{}{withdrawAddress}, 7_677},
		{"withdrawDelegationRewards", []interface{}{validator.String()}, 45_016},
		{"withdrawMultipleDelegationRewards", []interface{}{[]string{validator.String()}}, 45_016},
		{"withdrawValidatorCommission", nil, 10_423},
	}
	embeddedABI := pcommon.MustGetABI(f, "abi.json")
	require.Len(t, golden, len(embeddedABI.Methods))
	require.Equal(t, embeddedABI, precompile.GetABI())
	covered := make(map[string]bool, len(golden))
	for _, row := range golden {
		require.Contains(t, embeddedABI.Methods, row.method)
		require.False(t, covered[row.method], "duplicate golden method %s", row.method)
		covered[row.method] = true
	}
	for name := range embeddedABI.Methods {
		require.True(t, covered[name], "missing gas golden for %s", name)
	}

	for _, row := range golden {
		t.Run(row.method, func(t *testing.T) {
			input, err := precompile.Pack(row.method, row.args...)
			require.NoError(t, err)
			var charged [2]uint64
			var output [2][]byte
			for i, height := range []int64{planHeight - 1, planHeight} {
				callCtx, _ := ctx.CacheContext()
				callCtx = callCtx.WithBlockHeight(height).WithGasMeter(sdk.NewInfiniteGasMeter(1, 1))
				require.Equal(t, height >= planHeight, pcommon.MergePlanActive(callCtx, k))
				evm := &vm.EVM{
					StateDB:   state.NewDBImpl(callCtx, k, true),
					TxContext: vm.TxContext{Origin: caller},
				}
				ret, remaining, err := precompile.RunAndCalculateGas(
					evm, caller, caller, input, suppliedGas, big.NewInt(0), nil,
					row.method == "rewards", false,
				)
				require.NoError(t, err, "height %d", height)
				require.LessOrEqual(t, remaining, suppliedGas)
				charged[i] = suppliedGas - remaining
				output[i] = ret
				decoded, err := precompile.Methods[row.method].Outputs.Unpack(ret)
				require.NoError(t, err)
				require.Len(t, decoded, 1)
				if row.method != "rewards" {
					require.Equal(t, true, decoded[0])
				}
				t.Logf("height=%d gas=%d", height, charged[i])
				if charged[i] != row.gas {
					t.Errorf("height %d: gas = %d, frozen gas = %d", height, charged[i], row.gas)
				}
			}
			require.Equal(t, charged[0], charged[1], "gas changed at the merge plan height")
			require.Equal(t, output[0], output[1], "output changed at the merge plan height")
		})
	}
}
