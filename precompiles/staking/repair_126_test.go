package staking_test

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/staking"
	stakingv65 "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/staking/legacy/v65"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/ed25519"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/query"
	stakingkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/teststaking"
	stakingtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/types"
	upgradetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

const repair126PlanHeight int64 = 100

// Produce the entry through BeginRedelegation, after a real slash has changed
// the destination's exchange rate. No query response or ABI output is supplied
// by the fixture: both are produced by the real keepers and precompile.
func repair126RedelegationState(t *testing.T) (*app.App, sdk.Context, *stakingtypes.QueryRedelegationsRequest) {
	t.Helper()
	a := testkeeper.EVMTestApp
	ctx, _ := a.GetContextForDeliverTx(nil).WithBlockHeight(20).
		WithBlockTime(time.Unix(1_800_000_000, 0).UTC()).
		WithGasMeter(sdk.NewInfiniteGasMeter(1, 1)).CacheContext()
	k := a.StakingKeeper
	validators := make([]sdk.ValAddress, 2)
	for i := range validators {
		pubkey := ed25519.GenPrivKeyFromSecret([]byte(fmt.Sprintf("repair-126-validator-%d", i))).PubKey()
		address := sdk.ValAddress(pubkey.Address())
		stake := k.TokensFromConsensusPower(ctx, 3)
		funds := sdk.NewCoins(sdk.NewCoin(k.BondDenom(ctx), stake))
		require.NoError(t, a.BankKeeper.MintCoins(ctx, "mint", funds))
		require.NoError(t, a.BankKeeper.SendCoinsFromModuleToAccount(ctx, "mint", sdk.AccAddress(address), funds))
		h := teststaking.NewHelper(t, ctx, k)
		h.Denom = k.BondDenom(ctx)
		h.Handle(h.CreateValidatorMsg(address, pubkey, stake), true)
		validator, found := k.GetValidator(ctx, address)
		require.True(t, found)
		// Unbonding validators and newly created validators both use the
		// not-bonded pool; the future completion keeps the redelegation live.
		validator = validator.UpdateStatus(stakingtypes.Unbonding)
		validator.UnbondingTime = ctx.BlockTime().Add(24 * time.Hour)
		validator.UnbondingHeight = ctx.BlockHeight()
		k.SetValidator(ctx, validator)
		validators[i] = address
	}
	dst, found := k.GetValidator(ctx, validators[1])
	require.True(t, found)
	consensusAddress, err := dst.GetConsAddr()
	require.NoError(t, err)
	k.Slash(ctx, consensusAddress, ctx.BlockHeight(), 3, sdk.MustNewDecFromStr("0.4"))
	dst, found = k.GetValidator(ctx, validators[1])
	require.True(t, found)
	require.Equal(t, "0.600000000000000000", dst.TokensFromShares(sdk.OneDec()).String())

	delegator := sdk.AccAddress(validators[0])
	_, err = k.BeginRedelegation(ctx, delegator, validators[0], validators[1], sdk.NewDec(7))
	require.NoError(t, err)
	return a, ctx, &stakingtypes.QueryRedelegationsRequest{
		DelegatorAddr: delegator.String(), SrcValidatorAddr: validators[0].String(),
		DstValidatorAddr: validators[1].String(), Pagination: &query.PageRequest{},
	}
}

func repair126Query(t *testing.T, a *app.App, ctx sdk.Context, req *stakingtypes.QueryRedelegationsRequest) *stakingtypes.QueryRedelegationsResponse {
	t.Helper()
	response, err := (stakingkeeper.Querier{Keeper: a.StakingKeeper}).Redelegations(sdk.WrapSDKContext(ctx), req)
	require.NoError(t, err)
	require.Len(t, response.RedelegationResponses, 1)
	require.Len(t, response.RedelegationResponses[0].Entries, 1)
	return response
}

func repair126Call(t *testing.T, ctx sdk.Context, k *evmkeeper.Keeper, runner vm.DynamicGasPrecompiledContract, contractABI abi.ABI, req *stakingtypes.QueryRedelegationsRequest) ([]byte, staking.RedelegationsResponse) {
	t.Helper()
	input, err := contractABI.Pack(staking.RedelegationsMethod, req.DelegatorAddr, req.SrcValidatorAddr, req.DstValidatorAddr, req.Pagination.Key)
	require.NoError(t, err)
	require.Equal(t, "7ac3ee49", common.Bytes2Hex(input[:4]), "frozen redelegations selector")
	evm := &vm.EVM{StateDB: state.NewDBImpl(ctx, k, true)}
	ret, _, err := runner.RunAndCalculateGas(evm, common.Address{}, common.Address{}, input, 20_000_000, nil, nil, true, false)
	require.NoError(t, err, "redelegations reverted: %x", ret)
	values, err := contractABI.Methods[staking.RedelegationsMethod].Outputs.Unpack(ret)
	require.NoError(t, err)
	require.Len(t, values, 1)
	decoded := abi.ConvertType(values[0], new(staking.RedelegationsResponse)).(*staking.RedelegationsResponse)
	require.Len(t, decoded.Redelegations, 1)
	require.Len(t, decoded.Redelegations[0].Entries, 1)
	return ret, *decoded
}

func TestRepair_126_RedelegationShares(t *testing.T) {
	a, base, request := repair126RedelegationState(t)
	a.UpgradeKeeper.SetDone(base.WithBlockHeight(repair126PlanHeight), pcommon.MergeUpgrade)
	precompile, err := staking.NewPrecompile(a.GetPrecompileKeepers())
	require.NoError(t, err)
	for _, sourceIndex := range []bool{false, true} {
		t.Run(fmt.Sprintf("source_index_%t", sourceIndex), func(t *testing.T) {
			req := *request
			if sourceIndex {
				req.DelegatorAddr, req.DstValidatorAddr = "", ""
			}
			ctx, _ := base.WithBlockHeight(repair126PlanHeight).CacheContext()
			producer := repair126Query(t, a, ctx, &req)
			entry := producer.RedelegationResponses[0].Entries[0]
			// SharesFromTokens uses Dec.QuoInt, which truncates to 18 places.
			require.Equal(t, "11.666666666666666666", entry.RedelegationEntry.SharesDst.String())
			require.NotEqual(t, entry.Balance.ToDec(), entry.RedelegationEntry.SharesDst)
			_, consumer := repair126Call(t, ctx, &a.EvmKeeper, precompile, precompile.GetABI(), &req)
			redel := consumer.Redelegations[0]
			require.Equal(t, request.DelegatorAddr, redel.DelegatorAddress)
			require.Equal(t, request.SrcValidatorAddr, redel.ValidatorSrcAddress)
			require.Equal(t, request.DstValidatorAddr, redel.ValidatorDstAddress)
			require.Equal(t, entry.RedelegationEntry.SharesDst.String(), redel.Entries[0].SharesDst)
			require.NotEqual(t, entry.Balance.String(), redel.Entries[0].SharesDst)
			require.Equal(t, entry.RedelegationEntry.InitialBalance.String(), redel.Entries[0].InitialBalance)
			require.Equal(t, entry.RedelegationEntry.CreationHeight, redel.Entries[0].CreationHeight)
			require.Equal(t, entry.RedelegationEntry.CompletionTime.Unix(), redel.Entries[0].CompletionTime)
			require.True(t, bytes.Equal(producer.Pagination.NextKey, consumer.NextKey), "pagination key must survive ABI encoding")
		})
	}
}

func TestRepair_126_UpgradeReplay(t *testing.T) {
	a, base, req := repair126RedelegationState(t)
	k := a.EvmKeeper
	address := common.HexToAddress(staking.StakingAddress)
	versions := staking.GetVersioned(pcommon.MergeUpgrade, a.GetPrecompileKeepers())
	k.SetCustomPrecompiles(map[common.Address]utils.VersionedPrecompiles{address: versions}, pcommon.MergeUpgrade)
	a.UpgradeKeeper.SetDone(base.WithBlockHeight(1), "v6.4.0")
	require.Zero(t, a.UpgradeKeeper.GetDoneHeight(base, pcommon.MergeUpgrade))
	legacy, err := stakingv65.NewPrecompile(a.GetPrecompileKeepers())
	require.NoError(t, err)
	current := versions[pcommon.MergeUpgrade].(*pcommon.DynamicGasPrecompile)
	contractABI := current.GetABI()
	producer := repair126Query(t, a, base, req)
	entry := producer.RedelegationResponses[0].Entries[0]

	for _, phase := range []string{"unscheduled", "scheduled", "applied"} {
		switch phase {
		case "scheduled":
			require.NoError(t, a.UpgradeKeeper.ScheduleUpgrade(base, upgradetypes.Plan{
				Name: pcommon.MergeUpgrade, Height: repair126PlanHeight,
			}))
		case "applied":
			a.UpgradeKeeper.SetDone(base.WithBlockHeight(repair126PlanHeight), pcommon.MergeUpgrade)
		}
		for _, height := range []int64{89, repair126PlanHeight - 1, repair126PlanHeight, repair126PlanHeight + 1} {
			for _, tracing := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/height_%d/tracing_%t", phase, height, tracing), func(t *testing.T) {
					ctx, _ := base.WithBlockHeight(height).WithIsTracing(tracing).CacheContext()
					served, found := k.CustomPrecompiles(ctx)[address]
					require.True(t, found)
					runner, ok := served.(vm.DynamicGasPrecompiledContract)
					require.True(t, ok)
					wantVersion := "v6.4.0"
					if phase == "applied" && height >= repair126PlanHeight {
						wantVersion = pcommon.MergeUpgrade
					}
					require.Equal(t, wantVersion, k.GetCustomPrecompilesVersions(ctx)[address])
					actual, decoded := repair126Call(t, ctx, &k, runner, contractABI, req)
					legacyCtx, _ := ctx.CacheContext()
					historical, historicalDecoded := repair126Call(t, legacyCtx, &k, legacy, contractABI, req)
					require.Equal(t, entry.Balance.String(), historicalDecoded.Redelegations[0].Entries[0].SharesDst,
						"archived implementation must preserve historical bytes even after activation")
					if phase == "applied" && height >= repair126PlanHeight {
						require.Equal(t, entry.RedelegationEntry.SharesDst.String(), decoded.Redelegations[0].Entries[0].SharesDst)
						expected := historicalDecoded
						expected.Redelegations[0].Entries[0].SharesDst = entry.RedelegationEntry.SharesDst.String()
						expectedBytes, err := contractABI.Methods[staking.RedelegationsMethod].Outputs.Pack(expected)
						require.NoError(t, err)
						require.Equal(t, expectedBytes, actual, "only the sharesDst projection changes")
					} else {
						require.Equal(t, entry.Balance.String(), decoded.Redelegations[0].Entries[0].SharesDst)
						require.Equal(t, historical, actual, "pre-activation replay must be byte identical")
					}
				})
			}
		}
	}
}
