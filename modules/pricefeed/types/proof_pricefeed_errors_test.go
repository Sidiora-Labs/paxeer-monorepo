// Proves: pricefeed Keeper and AppModule -> SDK ResponseDeliverTx/ABCIError and Precompile.Run -> LXPName, LXPReason and abi.UnpackRevert.
//
// Repairs modules/pricefeed/module.go AppModule.InitGenesis: malformed JSON must
// panic with ErrInvalidGenesis, just as AppModuleBasic.ValidateGenesis refuses it.
// Repairs modules/pricefeed/keeper/genesis.go Keeper.ExportGenesis: syntactically
// valid stored JSON must pass ValidateGenesis before the module exports it.
// The registry is stateless: every producer and consumer is called twice, and
// malformed error identities and truncated ABI results are refused at decoding.
package types_test

import (
	"errors"
	"maps"
	"math"
	"math/big"
	"strings"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	pricefeed "github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/keeper"
	pricefeedtestutil "github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/testutil"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/types"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	pfprecompile "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/pricefeed"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/ed25519"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	stakingtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

const proofPricefeedNow = int64(1_800_000_000_000)

func proofPricefeedKeeper(t *testing.T) (keeper.Keeper, sdk.Context, sdk.StoreKey) {
	t.Helper()
	app := testkeeper.EVMTestApp
	ctx := app.GetContextForDeliverTx(nil).WithBlockHeight(8).
		WithBlockTime(time.UnixMilli(proofPricefeedNow)).WithEventManager(sdk.NewEventManager())
	ctx = ctx.WithGasMeter(sdk.NewInfiniteGasMeterWithMultiplier(ctx))
	key := sdk.NewKVStoreKey(types.StoreKey)
	ctx = pricefeedtestutil.WithPriceFeedStore(ctx, key)
	k := keeper.NewKeeper(key, app.StakingKeeper, app.AccountKeeper, &app.EvmKeeper)
	require.NoError(t, keeper.InitV6120(ctx, k))
	return k, ctx, key
}

func proofPricefeedValidator(t *testing.T, ctx sdk.Context, seed string) sdk.ValAddress {
	t.Helper()
	operator := sdk.ValAddress(secp256k1.GenPrivKeyFromSecret([]byte(seed)).PubKey().Address())
	validator, err := stakingtypes.NewValidator(operator,
		ed25519.GenPrivKeyFromSecret([]byte(seed)).PubKey(), stakingtypes.Description{})
	require.NoError(t, err)
	testkeeper.EVMTestApp.StakingKeeper.SetValidator(ctx, validator.UpdateStatus(stakingtypes.Bonded))
	return operator
}

// Observe the same serialized fields the SDK client uses, then let the SDK's
// registry reconstruct identity. No test-side mapping substitutes for ABCIError.
func proofPricefeedObserve(t *testing.T, produced error, want *sdkerrors.Error, code uint32, name string) {
	t.Helper()
	require.ErrorIs(t, produced, want)
	response := sdkerrors.ResponseDeliverTx(sdkerrors.Wrap(produced, "deliver pricefeed"), 100, 25, false)
	wire, err := response.Marshal()
	require.NoError(t, err)
	var received abci.ResponseDeliverTx
	require.NoError(t, received.Unmarshal(wire))
	require.Equal(t, "pricefeed", received.Codespace)
	require.Equal(t, code, received.Code)
	require.Equal(t, int64(100), received.GasWanted)
	require.Equal(t, int64(25), received.GasUsed)
	observed := sdkerrors.ABCIError(received.Codespace, received.Code, received.Log)
	require.ErrorIs(t, observed, want)
	got, found := types.LXPName(observed)
	require.True(t, found)
	require.Equal(t, name, got)
	// The common precompile registry consumes the reconstructed Go identity too.
	data := pcommon.LXPReason(observed)
	reason, err := abi.UnpackRevert(data)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(reason, name+": "), reason)
	require.Contains(t, reason, want.Error())
	canonical, err := abi.UnpackRevert(pcommon.LXPRevert(got, ""))
	require.NoError(t, err)
	require.Equal(t, name+": "+want.Error(), canonical)
	_, err = abi.UnpackRevert(data[:len(data)/2])
	require.Error(t, err, "a truncated producer result must not decode as a refusal")
}

func TestProofPricefeedErrorsRegisteredProducers(t *testing.T) {
	k, ctx, key := proofPricefeedKeeper(t)
	operator := proofPricefeedValidator(t, ctx, "proof-pricefeed-operator")
	other := proofPricefeedValidator(t, ctx, "proof-pricefeed-other")
	feedKey := secp256k1.GenPrivKeyFromSecret([]byte("proof-pricefeed-feed")).PubKey().Bytes()
	feeder, _, err := k.RegisterFeedKey(ctx, operator, feedKey)
	require.NoError(t, err)
	msg := types.MsgSubmitPrices{Feeder: feeder.String(), Observations: []types.Observation{{
		MarketId: 0, Sequence: 1, IndexPrice: "64250000000000000000000",
		MarkPrice: "64250000000000000000000", SourceTimestampMs: uint64(proofPricefeedNow),
	}}}
	server := keeper.NewMsgServerImpl(k)
	accepted, err := server.SubmitPrices(sdk.WrapSDKContext(ctx), &msg)
	require.NoError(t, err)
	require.Equal(t, uint32(1), accepted.Accepted)
	require.Zero(t, accepted.Refused)
	for i := 0; i < 2; i++ {
		record, found := k.GetObservation(ctx, 0, operator)
		require.True(t, found)
		require.Equal(t, uint64(1), record.Sequence)
		require.Equal(t, msg.Observations[0].IndexPrice, record.IndexPrice.String())
		success := sdkerrors.ResponseDeliverTx(nil, 0, 0, false)
		require.Zero(t, success.Code)
		require.Empty(t, success.Codespace)
	}
	paramsBefore := append([]byte(nil), ctx.KVStore(key).Get(types.ParamsKey)...)
	cases := []struct {
		name    string
		want    *sdkerrors.Error
		code    uint32
		produce func() error
	}{
		{"UNAUTHORIZED", types.ErrUnauthorized, 2, func() error {
			return k.UpdateParams(ctx, sdk.AccAddress(other).String(), k.GetParams(ctx))
		}},
		{"INVALID_PARAMS", types.ErrInvalidParams, 3, func() error {
			params := k.GetParams(ctx)
			params.MinSources = 0
			return k.SetParams(ctx, params)
		}},
		{"INVALID_GENESIS", types.ErrInvalidGenesis, 4, func() error {
			return (pricefeed.AppModuleBasic{}).ValidateGenesis(nil, nil, []byte(`{"params":`))
		}},
		{"NOT_VALIDATOR", types.ErrNotValidator, 5, func() error {
			return k.RequireBonded(ctx, sdk.ValAddress([]byte("unknown-validator---")))
		}},
		{"INVALID_KEY", types.ErrInvalidKey, 6, func() error {
			_, _, err := k.RegisterFeedKey(ctx, operator, []byte{2})
			return err
		}},
		{"KEY_IN_USE", types.ErrKeyInUse, 7, func() error {
			_, _, err := k.RegisterFeedKey(ctx, other, feedKey)
			return err
		}},
		{"NO_KEY", types.ErrNoKey, 8, func() error {
			_, err := k.RevokeFeedKey(ctx, other)
			return err
		}},
		{"UNKNOWN_MARKET", types.ErrUnknownMarket, 9, func() error {
			_, err := k.GetPrice(ctx, uint64(len(k.GetParams(ctx).Markets)))
			return err
		}},
		{"UNREGISTERED_FEEDER", types.ErrUnregisteredFeeder, 10, func() error {
			wrong := msg
			wrong.Feeder = sdk.AccAddress(other).String()
			_, err := server.SubmitPrices(sdk.WrapSDKContext(ctx), &wrong)
			return err
		}},
		{"RATE_LIMITED", types.ErrRateLimited, 11, func() error {
			// This is also the real fee consumer that node/app.go calls before ante.
			gasless, err := pricefeed.SubmissionIsGasless(ctx, &msg, k)
			require.False(t, gasless)
			_, submitErr := server.SubmitPrices(sdk.WrapSDKContext(ctx), &msg)
			require.ErrorIs(t, submitErr, types.ErrRateLimited)
			return err
		}},
		{"TOO_MANY_OBSERVATIONS", types.ErrTooManyObservations, 12, func() error {
			many := msg
			many.Observations = make([]types.Observation, types.DefaultMaxObservationsPerSubmission+1)
			for i := range many.Observations {
				many.Observations[i] = msg.Observations[0]
				many.Observations[i].MarketId = uint64(i)
			}
			_, err := server.SubmitPrices(sdk.WrapSDKContext(ctx), &many)
			return err
		}},
		{"SUBMISSION_TOO_LARGE", types.ErrSubmissionTooLarge, 13, func() error {
			oversized := ctx.WithTxBytes(make([]byte, types.DefaultMaxSubmissionBytes+1))
			_, err := server.SubmitPrices(sdk.WrapSDKContext(oversized), &msg)
			return err
		}},
		{"DUPLICATE_MARKET", types.ErrDuplicateMarket, 14, func() error {
			duplicate := msg
			duplicate.Observations = append(append([]types.Observation(nil), msg.Observations...), msg.Observations[0])
			_, err := server.SubmitPrices(sdk.WrapSDKContext(ctx), &duplicate)
			return err
		}},
		{"SUBMISSION_ALONE", types.ErrSubmissionAlone, 15, func() error {
			submission, err := types.CheckSubmissionAlone([]sdk.Msg{&msg, &msg})
			require.True(t, submission)
			return err
		}},
		{"INVALID_OBSERVATION", types.ErrInvalidObservation, 16, func() error {
			malformed := msg
			malformed.Observations = append([]types.Observation(nil), msg.Observations...)
			malformed.Observations[0].IndexPrice = "not-a-price"
			_, err := server.SubmitPrices(sdk.WrapSDKContext(ctx), &malformed)
			return err
		}},
	}
	require.Len(t, cases, 15)
	require.Len(t, types.LXPCodes(), len(cases))
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for attempt := 0; attempt < 2; attempt++ {
				proofPricefeedObserve(t, tc.produce(), tc.want, tc.code, "LXP_PRICEFEED_"+tc.name)
			}
		})
	}
	require.Equal(t, paramsBefore, ctx.KVStore(key).Get(types.ParamsKey))
	record, found := k.GetObservation(ctx, 0, operator)
	require.True(t, found)
	require.Equal(t, uint64(1), record.Sequence)
}

func TestProofPricefeedErrorsGenesisConsumers(t *testing.T) {
	k, ctx, key := proofPricefeedKeeper(t)
	basic := pricefeed.AppModuleBasic{}
	am := pricefeed.NewAppModule(k)
	for attempt := 0; attempt < 2; attempt++ {
		encoded := am.ExportGenesis(ctx, nil)
		require.NoError(t, basic.ValidateGenesis(nil, nil, encoded))
		require.Empty(t, am.InitGenesis(ctx, nil, encoded))
		require.Equal(t, types.DefaultParams(types.DefaultAuthority()), k.GetParams(ctx))
		// Assert the actual panic value, not merely its printable description.
		func() {
			defer func() {
				panicErr, ok := recover().(error)
				require.True(t, ok, "malformed genesis must panic with an error")
				proofPricefeedObserve(t, panicErr, types.ErrInvalidGenesis, 4, "LXP_PRICEFEED_INVALID_GENESIS")
			}()
			am.InitGenesis(ctx, nil, []byte(`{"params":`))
		}()
	}
	// Corrupt the real store with valid JSON: syntax-only export used to accept it.
	ctx.KVStore(key).Set(types.AggregateKey(0), []byte(`{"index_price":"0","mark_price":"1","basis":"0","sources":1}`))
	for attempt := 0; attempt < 2; attempt++ {
		exported, err := k.ExportGenesis(ctx)
		require.Equal(t, types.GenesisState{}, exported)
		proofPricefeedObserve(t, err, types.ErrInvalidGenesis, 4, "LXP_PRICEFEED_INVALID_GENESIS")
		func() {
			defer func() {
				panicErr, ok := recover().(error)
				require.True(t, ok)
				require.ErrorIs(t, panicErr, types.ErrInvalidGenesis)
			}()
			am.ExportGenesis(ctx, nil)
		}()
	}
}

func TestProofPricefeedErrorsRejectMalformedIdentity(t *testing.T) {
	for attempt := 0; attempt < 2; attempt++ {
		for _, err := range []error{nil, errors.New("invalid params"), errors.New("LXP_PRICEFEED_INVALID_PARAMS"),
			sdkerrors.ABCIError("wrong-pricefeed", 3, "invalid params"),
			sdkerrors.ABCIError("pricefeed", 999, "invalid params"), sdkerrors.ErrInvalidSequence} {
			name, found := types.LXPName(err)
			require.False(t, found)
			require.Empty(t, name)
		}
		// A caller may edit its returned registry view without changing resolution.
		codes := types.LXPCodes()
		codes[0] = types.LXPCode{Err: types.ErrInvalidParams, Name: "broken"}
		name, found := types.LXPName(types.ErrUnauthorized)
		require.True(t, found)
		require.Equal(t, "LXP_PRICEFEED_UNAUTHORIZED", name)
	}
}

func TestProofPricefeedErrorsPrecompileTransport(t *testing.T) {
	k, ctx, _ := proofPricefeedKeeper(t)
	app := testkeeper.EVMTestApp
	p, err := pfprecompile.NewPrecompileWithKeeper(&k, &app.EvmKeeper)
	require.NoError(t, err)
	db := state.NewDBImpl(ctx, &app.EvmKeeper, false)
	block, err := app.EvmKeeper.GetVMBlockContext(ctx, core.GasPool(math.MaxUint64))
	require.NoError(t, err)
	contracts := maps.Clone(app.EvmKeeper.CustomPrecompiles(ctx))
	if contracts == nil {
		contracts = make(map[common.Address]vm.PrecompiledContract)
	}
	address := common.HexToAddress(pfprecompile.PriceFeedAddress)
	contracts[address] = p
	config := evmtypes.DefaultChainConfig().EthereumConfig(app.EvmKeeper.ChainID(ctx))
	evm := vm.NewEVM(*block, db, config, vm.Config{}, contracts)
	caller := common.HexToAddress("0x9001")
	operator := proofPricefeedValidator(t, db.Ctx(), "proof-pricefeed-precompile")
	app.EvmKeeper.SetAddressMapping(db.Ctx(), sdk.AccAddress(operator), caller)
	pack := func(method string, args ...interface{}) []byte {
		input, err := p.GetABI().Pack(method, args...)
		require.NoError(t, err)
		return input
	}
	unwired, err := pfprecompile.NewPrecompileWithKeeper(nil, &app.EvmKeeper)
	require.NoError(t, err)
	batch := make([]*big.Int, 65)
	for i := range batch {
		batch[i] = big.NewInt(0)
	}
	cases := []struct {
		name     string
		p        *pfprecompile.Precompile
		input    []byte
		readOnly bool
		delegate bool
	}{
		{"LXP_PRICEFEED_UNKNOWN_MARKET", p, pack("getPrice", big.NewInt(999)), true, false},
		{"LXP_PRICEFEED_INVALID_KEY", p, pack("registerFeedKey", []byte{2}), false, false},
		{pfprecompile.LXP_PRICEFEED_BATCH_TOO_LARGE, p, pack("getPrices", batch), true, false},
		{pfprecompile.LXP_PRICEFEED_STATICCALL, p, pack("revokeFeedKey"), true, false},
		{pfprecompile.LXP_PRICEFEED_DELEGATECALL, p, pack("getMarketCount"), false, true},
		{pfprecompile.LXP_PRICEFEED_KEEPER_MISSING, unwired, pack("getMarketCount"), true, false},
		{"LXP_PRECOMPILE_ARG_COUNT", p, p.GetABI().Methods["getPrice"].ID, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for attempt := 0; attempt < 2; attempt++ {
				data, err := tc.p.Run(evm, caller, caller, tc.input, big.NewInt(0), tc.readOnly, tc.delegate, nil)
				require.ErrorIs(t, err, vm.ErrExecutionReverted)
				reason, decodeErr := abi.UnpackRevert(data)
				require.NoError(t, decodeErr)
				code, _, found := strings.Cut(reason, ":")
				require.True(t, found)
				require.Equal(t, tc.name, code)
				_, decodeErr = abi.UnpackRevert(data[:len(data)/2])
				require.Error(t, decodeErr)
			}
		})
	}
	for attempt := 0; attempt < 2; attempt++ {
		data, _, err := evm.StaticCall(caller, address, pack("getMarketCount"), 2_000_000)
		require.NoError(t, err)
		values, err := p.GetABI().Methods["getMarketCount"].Outputs.Unpack(data)
		require.NoError(t, err)
		require.Equal(t, big.NewInt(18), values[0])
	}
	_, registered := k.GetFeedKey(db.Ctx(), operator)
	require.False(t, registered, "refusals must not register a malformed key")
	require.Empty(t, db.GetAllLogs())
}
