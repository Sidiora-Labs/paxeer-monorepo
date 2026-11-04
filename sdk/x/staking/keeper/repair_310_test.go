package keeper_test

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	paxapp "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	pstaking "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/staking"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	sdkclient "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/server/api"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/rootmulti"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/query"
	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/keeper"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	bankkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/keeper"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	paramskeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/teststaking"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

type repair310Store struct {
	store   *rootmulti.Store
	ctx     sdk.Context
	staking keeper.Keeper
	bank    bankkeeper.BaseKeeper
}

func repair310Open(t *testing.T, directory string) repair310Store {
	t.Helper()
	db, err := dbm.NewGoLevelDB("staking", directory)
	require.NoError(t, err)
	ms := rootmulti.NewStore(db)
	keys := sdk.NewKVStoreKeys("auth", "bank", "staking", "params")
	for _, key := range keys {
		ms.MountStoreWithDB(key, storetypes.StoreTypeIAVL, nil)
	}
	transient := sdk.NewTransientStoreKey("transient_params")
	ms.MountStoreWithDB(transient, storetypes.StoreTypeTransient, nil)
	deferred := storetypes.NewMemoryStoreKey("deferred_bank")
	ms.MountStoreWithDB(deferred, storetypes.StoreTypeMemory, nil)
	require.NoError(t, ms.LoadLatestVersion())
	ctx := sdk.NewContext(ms, tmproto.Header{Height: 0, Time: time.Unix(100, 0).UTC()}, false)
	encoding := paxapp.MakeEncodingConfig()
	pk := paramskeeper.NewKeeper(encoding.Marshaler, encoding.Amino, keys["params"], transient)
	ak := authkeeper.NewAccountKeeper(encoding.Marshaler, keys["auth"], pk.Subspace("auth"), authtypes.ProtoBaseAccount,
		map[string][]string{"mint": {authtypes.Minter}, types.BondedPoolName: {authtypes.Staking, authtypes.Burner}, types.NotBondedPoolName: {authtypes.Staking, authtypes.Burner}})
	bk := bankkeeper.NewBaseKeeperWithDeferredCache(encoding.Marshaler, keys["bank"], ak, pk.Subspace("bank"), map[string]bool{}, deferred)
	sk := keeper.NewKeeper(encoding.Marshaler, keys["staking"], ak, bk, pk.Subspace("staking"))
	if ms.LastCommitID().Version == 0 {
		ak.SetParams(ctx, authtypes.DefaultParams())
		bk.SetParams(ctx, banktypes.DefaultParams())
		sk.SetParams(ctx, types.DefaultParams())
		for _, name := range []string{"mint", types.BondedPoolName, types.NotBondedPoolName} {
			ak.GetModuleAccount(ctx, name)
		}
	}
	return repair310Store{ms, ctx, sk, bk}
}

func (f repair310Store) fund(t *testing.T, address sdk.AccAddress, amount sdk.Int) {
	t.Helper()
	coins := sdk.NewCoins(sdk.NewCoin(f.staking.BondDenom(f.ctx), amount))
	require.NoError(t, f.bank.MintCoins(f.ctx, "mint", coins))
	require.NoError(t, f.bank.SendCoinsFromModuleToAccount(f.ctx, "mint", address, coins))
}

func repair310Validator(t *testing.T, k keeper.Keeper, ctx sdk.Context, i int) types.Validator {
	t.Helper()
	validator := teststaking.NewValidator(t, sdk.ValAddress(PKs[i].Address()), PKs[i])
	k.SetValidator(ctx, validator)
	require.NoError(t, k.SetValidatorByConsAddr(ctx, validator))
	k.SetValidatorByPowerIndex(ctx, validator)
	return validator
}

func TestRepair_310_UnbondingRetryRestart(t *testing.T) {
	directory := t.TempDir()
	f := repair310Open(t, directory)
	del := sdk.AccAddress(PKs[200].Address())
	validator := repair310Validator(t, f.staking, f.ctx, 201)
	tokens := f.staking.TokensFromConsensusPower(f.ctx, 10)
	f.fund(t, del, tokens)
	shares, err := f.staking.Delegate(f.ctx, del, tokens, types.Unbonded, validator, true)
	require.NoError(t, err)
	otherDel := sdk.AccAddress(PKs[206].Address())
	f.fund(t, otherDel, tokens)
	validator, _ = f.staking.GetValidator(f.ctx, validator.GetOperator())
	otherShares, err := f.staking.Delegate(f.ctx, otherDel, tokens, types.Unbonded, validator, true)
	require.NoError(t, err)
	f.staking.BlockValidatorUpdates(f.ctx)
	f.ctx = f.ctx.WithBlockHeight(1)
	completion, err := f.staking.Undelegate(f.ctx, del, validator.GetOperator(), shares.QuoInt64(2))
	require.NoError(t, err)
	_, err = f.staking.Undelegate(f.ctx, otherDel, validator.GetOperator(), otherShares.QuoInt64(2))
	require.NoError(t, err)
	f.ctx = f.ctx.WithBlockHeight(2).WithBlockTime(completion)
	before, found := f.staking.GetUnbondingDelegation(f.ctx, del, validator.GetOperator())
	require.True(t, found)
	pool := f.staking.GetNotBondedPool(f.ctx).GetAddress()
	denom := f.staking.BondDenom(f.ctx)
	otherBefore := f.bank.GetBalance(f.ctx, otherDel, denom)
	otherPayout := sdk.NewCoin(denom, tokens.QuoRaw(2))
	poolBefore := f.bank.GetBalance(f.ctx, pool, denom)
	delBefore := f.bank.GetBalance(f.ctx, del, denom)
	supply := f.bank.GetSupply(f.ctx, denom)
	f.bank.RegisterRecipientChecker(func(_ sdk.Context, recipient sdk.AccAddress) bool { return !recipient.Equals(del) })
	_, err = f.staking.CompleteUnbonding(f.ctx, del, validator.GetOperator())
	require.ErrorIs(t, err, sdkerrors.ErrInvalidRecipient)
	f.staking.BlockValidatorUpdates(f.ctx)
	require.True(t, otherBefore.Add(otherPayout).IsEqual(f.bank.GetBalance(f.ctx, otherDel, denom)), "blocked work must not prevent another payout")
	poolBefore = poolBefore.Sub(otherPayout)
	require.True(t, poolBefore.IsEqual(f.bank.GetBalance(f.ctx, pool, denom)))
	require.True(t, delBefore.IsEqual(f.bank.GetBalance(f.ctx, del, denom)))
	require.True(t, supply.IsEqual(f.bank.GetSupply(f.ctx, denom)))
	require.Equal(t, []types.DVPair{{DelegatorAddress: del.String(), ValidatorAddress: validator.OperatorAddress}}, f.staking.GetUBDQueueTimeSlice(f.ctx, completion))
	after, found := f.staking.GetUnbondingDelegation(f.ctx, del, validator.GetOperator())
	require.True(t, found)
	require.Equal(t, before, after)
	committed := f.store.Commit(true)
	require.NoError(t, f.store.Close())

	f = repair310Open(t, directory)
	defer func() { require.NoError(t, f.store.Close()) }()
	require.Equal(t, committed, f.store.LastCommitID())
	f.ctx = f.ctx.WithBlockHeight(3).WithBlockTime(completion.Add(time.Second))
	recovered, found := f.staking.GetUnbondingDelegation(f.ctx, del, validator.GetOperator())
	require.True(t, found)
	require.Equal(t, before, recovered)
	require.Len(t, f.staking.GetUBDQueueTimeSlice(f.ctx, completion), 1)
	f.staking.BlockValidatorUpdates(f.ctx)
	payout := sdk.NewCoin(denom, tokens.QuoRaw(2))
	require.True(t, delBefore.Add(payout).IsEqual(f.bank.GetBalance(f.ctx, del, denom)))
	require.True(t, poolBefore.Sub(payout).IsEqual(f.bank.GetBalance(f.ctx, pool, denom)))
	require.True(t, supply.IsEqual(f.bank.GetSupply(f.ctx, denom)))
	_, found = f.staking.GetUnbondingDelegation(f.ctx, del, validator.GetOperator())
	require.False(t, found)
	f.ctx = f.ctx.WithBlockHeight(4).WithBlockTime(completion.Add(2 * time.Second))
	f.staking.BlockValidatorUpdates(f.ctx)
	require.True(t, delBefore.Add(payout).IsEqual(f.bank.GetBalance(f.ctx, del, denom)))
	message, broken := bankkeeper.TotalSupply(f.bank)(f.ctx)
	require.False(t, broken, message)
}

func TestRepair_310_UnbondingAtomicFailure(t *testing.T) {
	f := repair310Open(t, t.TempDir())
	defer func() { require.NoError(t, f.store.Close()) }()
	del := sdk.AccAddress(PKs[202].Address())
	validator := repair310Validator(t, f.staking, f.ctx, 203)
	f.fund(t, del, sdk.NewInt(20))
	shares, err := f.staking.Delegate(f.ctx, del, sdk.NewInt(20), types.Unbonded, validator, true)
	require.NoError(t, err)
	// The real unbonded source keeps funds in the not-bonded pool.
	_, err = f.staking.Undelegate(f.ctx.WithBlockHeight(1), del, validator.GetOperator(), shares.QuoInt64(4))
	require.NoError(t, err)
	completion, err := f.staking.Undelegate(f.ctx.WithBlockHeight(2), del, validator.GetOperator(), shares.QuoInt64(4))
	require.NoError(t, err)
	f.ctx = f.ctx.WithBlockHeight(3).WithBlockTime(completion)
	before, found := f.staking.GetUnbondingDelegation(f.ctx, del, validator.GetOperator())
	require.True(t, found)
	require.Len(t, before.Entries, 2)
	// Real bank insufficient-funds failure on the second payment, after the
	// first payment would have succeeded without the completion cache.
	require.NoError(t, f.bank.BurnCoins(f.ctx, types.NotBondedPoolName, sdk.NewCoins(sdk.NewInt64Coin(f.staking.BondDenom(f.ctx), 15))))
	pool := f.staking.GetNotBondedPool(f.ctx).GetAddress()
	poolBefore := f.bank.GetAllBalances(f.ctx, pool)
	delBefore := f.bank.GetAllBalances(f.ctx, del)
	events := len(f.ctx.EventManager().Events())
	_, err = f.staking.CompleteUnbonding(f.ctx, del, validator.GetOperator())
	require.Error(t, err)
	require.True(t, poolBefore.IsEqual(f.bank.GetAllBalances(f.ctx, pool)))
	require.True(t, delBefore.IsEqual(f.bank.GetAllBalances(f.ctx, del)))
	require.Len(t, f.ctx.EventManager().Events(), events)
	after, found := f.staking.GetUnbondingDelegation(f.ctx, del, validator.GetOperator())
	require.True(t, found)
	require.Equal(t, before, after)
}

func TestRepair_310_RedelegationPowerCap(t *testing.T) {
	f := repair310Open(t, t.TempDir())
	defer func() { require.NoError(t, f.store.Close()) }()
	del := sdk.AccAddress(PKs[204].Address())
	f.fund(t, del, f.staking.TokensFromConsensusPower(f.ctx, 2_000_000))
	params := f.staking.GetParams(f.ctx)
	params.MaxValidators = 6
	f.staking.SetParams(f.ctx, params)
	validators := make([]types.Validator, 6)
	for i, power := range []int64{190_000, 172_000, 172_000, 172_000, 172_000, 172_000} {
		validators[i] = repair310Validator(t, f.staking, f.ctx, 210+i)
		_, err := f.staking.Delegate(f.ctx, del, f.staking.TokensFromConsensusPower(f.ctx, power), types.Unbonded, validators[i], true)
		require.NoError(t, err)
	}
	f.staking.BlockValidatorUpdates(f.ctx)
	f.ctx = f.ctx.WithBlockHeight(1)
	require.Equal(t, sdk.NewInt(1_050_000), f.staking.GetLastTotalPower(f.ctx))
	transfer := func(source int, power int64) error {
		_, err := f.staking.BeginRedelegation(f.ctx, del, validators[source].GetOperator(), validators[0].GetOperator(), f.staking.TokensFromConsensusPower(f.ctx, power).ToDec())
		return err
	}
	sourceBefore, _ := f.staking.GetValidator(f.ctx, validators[1].GetOperator())
	require.ErrorIs(t, transfer(1, 21_000), types.ErrExceedMaxVotingPowerRatio)
	sourceAfter, _ := f.staking.GetValidator(f.ctx, validators[1].GetOperator())
	require.Equal(t, sourceBefore, sourceAfter, "rejected redelegation must preserve its source")
	require.NoError(t, transfer(1, 19_000))
	require.ErrorIs(t, transfer(2, 2_000), types.ErrExceedMaxVotingPowerRatio)
	// An earlier ordinary delegation contributes real new bonded power to the
	// same block. Its extra 10,000 permits exactly another 3,000 at the target.
	other, _ := f.staking.GetValidator(f.ctx, validators[3].GetOperator())
	_, err := f.staking.Delegate(f.ctx, del, f.staking.TokensFromConsensusPower(f.ctx, 10_000), types.Unbonded, other, true)
	require.NoError(t, err)
	require.NoError(t, transfer(2, 3_000))
	require.ErrorIs(t, transfer(4, 1), types.ErrExceedMaxVotingPowerRatio)
	f.staking.BlockValidatorUpdates(f.ctx)
	total := f.staking.GetLastTotalPower(f.ctx)
	require.Equal(t, sdk.NewInt(1_060_000), total)
	require.Equal(t, int64(212_000), f.staking.GetLastValidatorPower(f.ctx, validators[0].GetOperator()))
	for _, validator := range validators {
		power := sdk.NewInt(f.staking.GetLastValidatorPower(f.ctx, validator.GetOperator()))
		require.True(t, power.ToDec().LTE(total.ToDec().Mul(params.MaxVotingPowerRatio)))
	}
}

func TestRepair_310_RedelegationFiltersTransports(t *testing.T) {
	_, app, ctx := createTestInput(t)
	ctx = ctx.WithBlockTime(time.Unix(100, 0).UTC()).WithGasMeter(sdk.NewInfiniteGasMeter(1, 1))
	k := app.StakingKeeper
	delegators := paxapp.AddTestAddrsIncremental(app, ctx, 2, sdk.NewInt(100_000))
	validators := make([]types.Validator, 105)
	for i := range validators {
		validator := repair310Validator(t, k, ctx, i)
		validator = validator.UpdateStatus(types.Unbonding)
		validator.UnbondingTime = ctx.BlockTime().Add(24 * time.Hour)
		k.SetValidator(ctx, validator)
		validators[i] = validator
	}
	for _, del := range delegators {
		for _, source := range []int{0, 1} {
			_, err := k.Delegate(ctx, del, sdk.NewInt(10_000), types.Unbonded, validators[source], true)
			require.NoError(t, err)
			validators[source], _ = k.GetValidator(ctx, validators[source].GetOperator())
		}
	}
	// Produce 102 matching records and source, destination and delegator
	// distractors through the real redelegation state transition.
	for i := 3; i < len(validators); i++ {
		_, err := k.BeginRedelegation(ctx, delegators[0], validators[0].GetOperator(), validators[i].GetOperator(), sdk.OneDec())
		require.NoError(t, err)
		if i < 7 {
			_, err = k.BeginRedelegation(ctx, delegators[0], validators[1].GetOperator(), validators[i].GetOperator(), sdk.OneDec())
			require.NoError(t, err)
		}
	}
	_, err := k.BeginRedelegation(ctx, delegators[1], validators[0].GetOperator(), validators[3].GetOperator(), sdk.OneDec())
	require.NoError(t, err)
	queryServer := keeper.Querier{Keeper: k}
	helper := baseapp.NewQueryServerTestHelper(ctx, app.InterfaceRegistry())
	types.RegisterQueryServer(helper, queryServer)
	client := types.NewQueryClient(helper)
	mux := api.New(sdkclient.Context{InterfaceRegistry: app.InterfaceRegistry()}).GRPCGatewayRouter
	require.NoError(t, types.RegisterQueryHandlerClient(context.Background(), mux, client))
	precompile, err := pstaking.NewPrecompile(app.GetPrecompileKeepers())
	require.NoError(t, err)
	contractABI := precompile.GetABI()
	del, src, dst := delegators[0].String(), validators[0].OperatorAddress, validators[3].OperatorAddress
	cases := []types.QueryRedelegationsRequest{
		{DelegatorAddr: del, SrcValidatorAddr: src},
		{DelegatorAddr: del, DstValidatorAddr: dst},
		{DelegatorAddr: del, SrcValidatorAddr: src, DstValidatorAddr: dst},
		{SrcValidatorAddr: src, DstValidatorAddr: dst},
		{DstValidatorAddr: dst},
		{SrcValidatorAddr: src},
	}
	for index, request := range cases {
		var expected []types.Redelegation
		for _, delegator := range delegators {
			if request.DelegatorAddr != "" && request.DelegatorAddr != delegator.String() {
				continue
			}
			expected = append(expected, k.GetAllRedelegations(ctx, delegator, mustRepair310Val(request.SrcValidatorAddr), mustRepair310Val(request.DstValidatorAddr))...)
		}
		require.NotEmpty(t, expected)
		transports := []string{"grpc", "precompile"}
		if request.DelegatorAddr != "" {
			transports = append(transports, "http")
		}
		for _, transport := range transports {
			t.Run(fmt.Sprintf("%d_%s", index, transport), func(t *testing.T) {
				req := request
				req.Pagination = &query.PageRequest{Limit: 1, CountTotal: true}
				seen := map[string]bool{}
				pages := 0
				for {
					pages++
					require.LessOrEqual(t, pages, len(expected)+1, "pagination must make progress")
					var rows []types.Redelegation
					var next []byte
					switch transport {
					case "grpc":
						response, err := client.Redelegations(context.Background(), &req)
						require.NoError(t, err)
						if pages == 1 {
							require.Equal(t, uint64(len(expected)), response.Pagination.Total)
						}
						for _, response := range response.RedelegationResponses {
							rows = append(rows, response.Redelegation)
						}
						next = response.Pagination.NextKey
					case "http":
						values := url.Values{"src_validator_addr": {req.SrcValidatorAddr}, "dst_validator_addr": {req.DstValidatorAddr}, "pagination.limit": {"1"}, "pagination.count_total": {"true"}}
						if len(req.Pagination.Key) > 0 {
							values.Set("pagination.key", base64.StdEncoding.EncodeToString(req.Pagination.Key))
						}
						recorder := httptest.NewRecorder()
						mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/cosmos/staking/v1beta1/delegators/"+del+"/redelegations?"+values.Encode(), nil))
						require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
						var response types.QueryRedelegationsResponse
						require.NoError(t, app.AppCodec().UnmarshalAsJSON(recorder.Body.Bytes(), &response))
						if pages == 1 {
							require.Equal(t, uint64(len(expected)), response.Pagination.Total)
						}
						for _, response := range response.RedelegationResponses {
							rows = append(rows, response.Redelegation)
						}
						next = response.Pagination.NextKey
					case "precompile":
						input, err := contractABI.Pack(pstaking.RedelegationsMethod, req.DelegatorAddr, req.SrcValidatorAddr, req.DstValidatorAddr, req.Pagination.Key)
						require.NoError(t, err)
						evm := &vm.EVM{StateDB: state.NewDBImpl(ctx, &app.EvmKeeper, true)}
						output, _, err := precompile.RunAndCalculateGas(evm, common.Address{}, common.Address{}, input, 100_000_000, nil, nil, true, false)
						require.NoError(t, err, "ABI result %x", output)
						decoded, err := contractABI.Methods[pstaking.RedelegationsMethod].Outputs.Unpack(output)
						require.NoError(t, err)
						response := abi.ConvertType(decoded[0], new(pstaking.RedelegationsResponse)).(*pstaking.RedelegationsResponse)
						for _, red := range response.Redelegations {
							rows = append(rows, types.Redelegation{DelegatorAddress: red.DelegatorAddress, ValidatorSrcAddress: red.ValidatorSrcAddress, ValidatorDstAddress: red.ValidatorDstAddress})
						}
						next = response.NextKey
					}
					require.NotEmpty(t, rows)
					for _, red := range rows {
						if req.DelegatorAddr != "" {
							require.Equal(t, req.DelegatorAddr, red.DelegatorAddress)
						}
						if req.SrcValidatorAddr != "" {
							require.Equal(t, req.SrcValidatorAddr, red.ValidatorSrcAddress)
						}
						if req.DstValidatorAddr != "" {
							require.Equal(t, req.DstValidatorAddr, red.ValidatorDstAddress)
						}
						key := red.DelegatorAddress + red.ValidatorSrcAddress + red.ValidatorDstAddress
						require.False(t, seen[key], "record repeated across pages")
						seen[key] = true
					}
					if len(next) == 0 {
						break
					}
					req.Pagination = &query.PageRequest{Key: next, Limit: 1}
				}
				require.Len(t, seen, len(expected))
				if index == 0 && transport == "precompile" {
					require.Equal(t, 2, pages)
				}
			})
		}
		for _, reverse := range []bool{false, true} {
			req := request
			req.Pagination = &query.PageRequest{Offset: 1, Limit: 1, CountTotal: true, Reverse: reverse}
			if len(expected) < 2 {
				continue
			}
			response, err := client.Redelegations(context.Background(), &req)
			require.NoError(t, err)
			require.Equal(t, uint64(len(expected)), response.Pagination.Total)
			require.Len(t, response.RedelegationResponses, 1)
		}
	}
	empty, err := client.Redelegations(context.Background(), &types.QueryRedelegationsRequest{
		DelegatorAddr: del, DstValidatorAddr: validators[2].OperatorAddress,
		Pagination: &query.PageRequest{Limit: 1, CountTotal: true},
	})
	require.NoError(t, err)
	require.Empty(t, empty.RedelegationResponses)
	require.Zero(t, empty.Pagination.Total)
	require.Empty(t, empty.Pagination.NextKey)
	for _, req := range []types.QueryRedelegationsRequest{{DelegatorAddr: del, SrcValidatorAddr: "bad"}, {DelegatorAddr: del, DstValidatorAddr: "bad"}} {
		_, err := client.Redelegations(context.Background(), &req)
		require.Error(t, err)
	}
}

func mustRepair310Val(address string) sdk.ValAddress {
	if address == "" {
		return nil
	}
	addr, err := sdk.ValAddressFromBech32(address)
	if err != nil {
		panic(err)
	}
	return addr
}
