// Proves: sdkerrors.Register launchpad errors -> ResponseDeliverTx/ABCIError -> ContractErrorOf, Params.Validate, CalculateDynamicFee, MsgUpdateParams.ValidateBasic, Keeper.CheckRole/GetSurface, ParamsFromProtocolConfig.
//
// Repairs: params.go Params.Validate, errors.go ContractError.Revert, keeper/
// roles.go, surfaces.go and migrate_v6120_read.go used fmt wrapping that
// retained Go identity but lost registered ABCI codes (ABCIInfo follows Cause).
// math.go CalculateDynamicFee also accepted negative words or panicked on nil
// words; malformed native FeeInputs must refuse InvalidParams before saturation.
//
// Registered producer errors must retain their launchpad codes through ABCI.
// EVM custom selectors are asserted separately from the ABCI transport.
//
// The error registry has no persistent state. Every iteration calls the real
// producer and transport again. Stateful keeper producers use real application
// stores and EVM calls; no transport or keeper is replaced by a double.
package types_test

import (
	"errors"
	"math/big"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/launchpadtest"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

// proofLaunchpadTransport uses the real ABCI response encoder, protobuf wire
// format and client registry lookup. Neither the code nor the log is fabricated.
func proofLaunchpadTransport(t *testing.T, produced error, space string, code uint32) error {
	t.Helper()
	response := sdkerrors.ResponseDeliverTx(produced, 100, 25, false)
	require.Equal(t, space, response.Codespace)
	require.Equal(t, code, response.Code)
	wire, err := response.Marshal()
	require.NoError(t, err)
	var observed abci.ResponseDeliverTx
	require.NoError(t, observed.Unmarshal(wire))
	require.Equal(t, response, observed)
	if code == 0 {
		require.Empty(t, observed.Log)
		return nil
	}
	return sdkerrors.ABCIError(observed.Codespace, observed.Code, observed.Log)
}

func TestProofLaunchpadErrorsRegistry(t *testing.T) {
	caller := common.HexToAddress("0x00000000000000000000000000000000000000c1")
	role := [32]byte(crypto.Keccak256Hash([]byte("POOL_ROLE")))
	cases := []struct {
		err      *sdkerrors.Error
		code     uint32
		reason   string
		selector string
		args     []interface{}
	}{
		{types.ErrInvalidParams, 2, "LXP_LAUNCHPAD_INVALID_PARAMS", "", nil},
		{types.ErrInvalidGenesis, 24, "LXP_LAUNCHPAD_INVALID_GENESIS", "", nil},
		{types.ErrInvalidConfiguration, 42, "LXP_LAUNCHPAD_INVALID_CONFIGURATION", "0xc52a9bd3", nil},
		{types.ErrMissingRole, 47, "LXP_LAUNCHPAD_MISSING_ROLE", "0x0161a64a", []interface{}{caller, role}},
		{types.ErrInsufficientBalance, 38, "LXP_LAUNCHPAD_INSUFFICIENT_BALANCE", "0xf4d678b8", nil},
		{types.ErrInvalidBalance, 41, "LXP_LAUNCHPAD_INVALID_BALANCE", "0xc52e3eff", nil},
		{types.ErrInvalidHookResponse, 43, "LXP_LAUNCHPAD_INVALID_HOOK_RESPONSE", "0x1e048e1d", nil},
		{types.ErrTransferFailed, 59, "LXP_LAUNCHPAD_TRANSFER_FAILED", "0x90b8ec18", nil},
	}
	for _, tc := range cases {
		t.Run(tc.reason, func(t *testing.T) {
			for i := 0; i < 2; i++ {
				// These are the real package-initialisation Register results.
				require.Equal(t, tc.reason, tc.err.Error())
				observed := proofLaunchpadTransport(t, tc.err.Wrap("launchpad call"), "launchpad", tc.code)
				require.ErrorIs(t, observed, tc.err)
				contract, found := types.ContractErrorOf(observed)
				if tc.selector == "" {
					require.False(t, found)
					reason, err := abi.UnpackRevert(pcommon.RevertReason(observed))
					require.NoError(t, err)
					require.Contains(t, reason, tc.reason)
					registeredReason, err := abi.UnpackRevert(pcommon.LXPRevert(tc.reason, ""))
					require.NoError(t, err)
					wantReason := map[*sdkerrors.Error]string{
						types.ErrInvalidParams:  "LXP_LAUNCHPAD_INVALID_PARAMS: invalid params",
						types.ErrInvalidGenesis: "LXP_LAUNCHPAD_INVALID_GENESIS: invalid genesis",
					}
					require.Equal(t, wantReason[tc.err], registeredReason)
					continue
				}
				require.True(t, found)
				data, err := contract.Revert(tc.args...)
				require.NoError(t, err)
				require.Equal(t, tc.selector, hexutil.Encode(data[:4]))
				values, err := contract.Inputs.Unpack(data[4:])
				require.NoError(t, err)
				require.Len(t, values, len(tc.args))
				for n := range tc.args {
					require.Equal(t, tc.args[n], values[n])
				}
				// The actual encoder refuses malformed producer arguments.
				_, err = contract.Revert("malformed argument")
				require.ErrorIs(t, err, types.ErrInvalidParams)
				proofLaunchpadTransport(t, err, "launchpad", 2)
			}
		})
	}
	for _, refusal := range []error{
		errors.New("LXP_LAUNCHPAD_MISSING_ROLE"),
		sdkerrors.ABCIError("bank", 47, "wrong codespace"),
		sdkerrors.ABCIError("launchpad", 999, "unregistered identifier"),
	} {
		_, found := types.ContractErrorOf(refusal)
		require.False(t, found)
	}
	proofLaunchpadTransport(t, nil, "", 0)
}

func TestProofLaunchpadErrorsValidation(t *testing.T) {
	for i := 0; i < 2; i++ {
		params := types.DefaultParams()
		require.NoError(t, params.Validate())
		msg := types.MsgUpdateParams{Authority: account(1), Params: params}
		require.NoError(t, msg.ValidateBasic())
		fee, err := types.CalculateDynamicFee(params.FeeInputs(big.NewInt(3600), new(big.Int)))
		require.NoError(t, err)
		require.Equal(t, big.NewInt(280), fee)

		params.QuoteToken = "malformed"
		proofLaunchpadTransport(t, params.Validate(), "launchpad", 2)
		msg.Params = params
		proofLaunchpadTransport(t, msg.ValidateBasic(), "launchpad", 2)
		msg.Params = types.DefaultParams()
		msg.RoleGrants = []types.RoleAssignment{{Contract: gen2Factory, Account: holder, Role: "0x00"}}
		proofLaunchpadTransport(t, msg.ValidateBasic(), "launchpad", 2)
		msg.RoleGrants = nil
		msg.Authority = "wrong signer"
		proofLaunchpadTransport(t, msg.ValidateBasic(), "launchpad", 23)

		for _, corrupt := range []func(*types.FeeInputs){
			func(in *types.FeeInputs) { in.MinFee = big.NewInt(301) },
			func(in *types.FeeInputs) { in.BaseFee = nil },
			func(in *types.FeeInputs) { in.PoolAgeSeconds = big.NewInt(-1) },
			func(in *types.FeeInputs) { in.Volatility = new(big.Int).Lsh(big.NewInt(1), 256) },
			// Even saturation must not bypass malformed downstream words.
			func(in *types.FeeInputs) { in.BaseFee = big.NewInt(300); in.TopHolderBps = nil },
		} {
			in := types.DefaultParams().FeeInputs(big.NewInt(3600), new(big.Int))
			corrupt(&in)
			fee, err := types.CalculateDynamicFee(in)
			require.Nil(t, fee)
			require.ErrorIs(t, err, types.ErrInvalidParams)
			proofLaunchpadTransport(t, err, "launchpad", 2)
		}
	}
}

func TestProofLaunchpadErrorsKeeper(t *testing.T) {
	app := testkeeper.EVMTestApp
	ctx := app.GetContextForDeliverTx(nil).WithBlockHeight(1).WithBlockTime(time.Unix(1_800_000_000, 0))
	ctx = ctx.WithGasMeter(sdk.NewInfiniteGasMeterWithMultiplier(ctx))
	k, ctx := launchpadtest.NewKeeper(app, ctx)
	gs := types.DefaultGenesis()
	// Register records in this isolated launchpad store even when the parent
	// EVM already has surface code (the exported-chain genesis path).
	for role, address := range keeper.Generation2Surfaces {
		gs.Surfaces = append(gs.Surfaces, types.Surface{Address: address.Hex(), Role: role})
	}
	// A legitimate imported treasury ledger whose token has no deployed code
	// lets the real EVM produce empty return data at the decoding boundary.
	gs.Treasury = []types.TreasuryBalance{{Token: types.DefaultQuoteToken, Ledger: sdk.ZeroInt(), Held: sdk.NewInt(10)}}
	k.InitGenesis(ctx, *gs)
	require.NoError(t, k.InstallRoleSurfaces(ctx, &app.EvmKeeper))
	for _, address := range keeper.Generation2Surfaces {
		require.Equal(t, keeper.SurfaceRuntime, app.EvmKeeper.GetCode(ctx, address))
	}
	factory := k.Generation2().SidioraFactory
	who := common.HexToAddress(holder)
	other := common.HexToAddress(operator)
	role := [32]byte(crypto.Keccak256Hash([]byte("POOL_ROLE")))

	for i := 0; i < 2; i++ {
		err := k.CheckRole(ctx, factory, role, who)
		require.ErrorIs(t, err, types.ErrMissingRole)
		observed := proofLaunchpadTransport(t, err, "launchpad", 47)
		require.ErrorIs(t, observed, types.ErrMissingRole)
		var refusal *keeper.RoleError
		require.ErrorAs(t, err, &refusal)
		data, err := refusal.Revert()
		require.NoError(t, err)
		contract, found := types.ContractErrorOf(observed)
		require.True(t, found)
		args, err := contract.Inputs.Unpack(data[4:])
		require.NoError(t, err)
		require.Equal(t, []interface{}{who, role}, args)
		proofLaunchpadTransport(t, k.SetSurface(ctx, common.Address{}, keeper.Surface{}), "launchpad", 2)
		_, err = k.GovernanceGrant(ctx, k.Authority(), factory, keeper.DefaultAdminRole, who)
		proofLaunchpadTransport(t, err, "launchpad", 2)
		_, err = k.GovernanceGrant(ctx, account(2), factory, role, who)
		proofLaunchpadTransport(t, err, "launchpad", 23)
	}
	granted, err := k.GovernanceGrant(ctx, k.Authority(), factory, role, who)
	require.NoError(t, err)
	require.True(t, granted)
	for i := 0; i < 2; i++ {
		require.NoError(t, k.CheckRole(ctx, factory, role, who))
		proofLaunchpadTransport(t, k.CheckRole(ctx, factory, role, other), "launchpad", 47)
	}
	revoked, err := k.GovernanceRevoke(ctx, k.Authority(), factory, role, who)
	require.NoError(t, err)
	require.True(t, revoked)
	proofLaunchpadTransport(t, k.CheckRole(ctx, factory, role, who), "launchpad", 47)

	want := keeper.Surface{Role: types.RoleFactory, Generation: types.Generation2}
	for i := 0; i < 2; i++ {
		surfaceCtx, _ := ctx.CacheContext()
		require.NoError(t, k.SetSurface(surfaceCtx, factory, want))
		got, found, err := k.GetSurface(surfaceCtx, factory)
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, want, got)
		// Damage the real producer's encoded record, then make the consumer
		// read it again. A cached answer would incorrectly pass this read.
		store := surfaceCtx.KVStore(sdk.NewKVStoreKey(types.StoreKey))
		encoded := append([]byte(nil), store.Get(types.SurfaceKey(factory))...)
		store.Set(types.SurfaceKey(factory), encoded[:len(encoded)-1])
		_, found, err = k.GetSurface(surfaceCtx, factory)
		require.False(t, found)
		require.ErrorIs(t, err, types.ErrInvalidParams)
		proofLaunchpadTransport(t, err, "launchpad", 2)
		err = k.SetSurface(surfaceCtx, factory, want)
		require.ErrorIs(t, err, types.ErrInvalidParams)
		proofLaunchpadTransport(t, err, "launchpad", 2)
		require.Equal(t, encoded[:len(encoded)-1], store.Get(types.SurfaceKey(factory)))
	}
	require.NoError(t, k.SetSurface(ctx, factory, want))

	// Migration consumes actual EVM storage words of both ProtocolConfigs.
	params := types.DefaultParams()
	words := []*big.Int{common.HexToAddress(params.QuoteToken).Big(), params.VirtualQuoteDefault.BigInt(),
		params.VirtualTokenDefault.BigInt(), big.NewInt(10), big.NewInt(300), big.NewInt(30),
		big.NewInt(1000), big.NewInt(500), big.NewInt(100), new(big.Int), params.CreationFee.BigInt()}
	config := k.Generation2().ProtocolConfig
	for _, address := range []common.Address{config, common.HexToAddress("0xEeDF5409cFD30bd14D0399318c7d2150265575e5")} {
		for n, word := range words {
			app.EvmKeeper.SetState(ctx, address, common.BigToHash(big.NewInt(int64(n+1))), common.BigToHash(word))
		}
	}
	for i := 0; i < 2; i++ {
		got, diffs, err := keeper.ParamsFromProtocolConfig(ctx, &app.EvmKeeper)
		require.NoError(t, err)
		require.Equal(t, params, got)
		require.Empty(t, diffs)
		app.EvmKeeper.SetState(ctx, config, common.BigToHash(big.NewInt(4)), common.BigToHash(new(big.Int).Lsh(big.NewInt(1), 64)))
		_, _, err = keeper.ParamsFromProtocolConfig(ctx, &app.EvmKeeper)
		require.ErrorIs(t, err, types.ErrInvalidParams)
		proofLaunchpadTransport(t, err, "launchpad", 2)
		app.EvmKeeper.SetState(ctx, config, common.BigToHash(big.NewInt(4)), common.BigToHash(big.NewInt(10)))
	}

	proofLaunchpadProducerCodes(t, k, ctx)
	proofLaunchpadHookCode(t, k, ctx)
	proofLaunchpadUpdateParamsRefusal(t, k, ctx)
}

func proofLaunchpadProducerCodes(t *testing.T, k *keeper.Keeper, ctx sdk.Context) {
	t.Helper()
	who := common.HexToAddress(holder)
	other := common.HexToAddress(operator)
	for i := 0; i < 2; i++ {
		badGenesis := types.DefaultGenesis()
		badGenesis.AirdropEpochs = []types.AirdropEpochAmount{{}}
		_, invalidConfig := k.GetQuoteLedger(ctx, keeper.QuoteLedger{})
		_, _, invalidBalance := k.PullQuote(ctx, k.ModuleCaller(), who, keeper.TreasuryHeldLedger(), sdk.OneInt())
		cc, _ := ctx.CacheContext()
		_, transferFailed := k.PayQuote(cc, k.ModuleCaller(), keeper.TreasuryHeldLedger(), who, sdk.OneInt())
		for _, tc := range []struct {
			name     string
			produced error
			sentinel *sdkerrors.Error
			code     uint32
			selector string
		}{
			{"GenesisStateValidate", badGenesis.Validate(), types.ErrInvalidGenesis, 24, ""},
			{"GetQuoteLedger", invalidConfig, types.ErrInvalidConfiguration, 42, "0xc52a9bd3"},
			{"MoveQuote", k.MoveQuote(ctx, keeper.HeldQuoteLedger(who), keeper.HeldQuoteLedger(other), sdk.OneInt()), types.ErrInsufficientBalance, 38, "0xf4d678b8"},
			{"PullQuote", invalidBalance, types.ErrInvalidBalance, 41, "0xc52e3eff"},
			{"PayQuote", transferFailed, types.ErrTransferFailed, 59, "0x90b8ec18"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				proofLaunchpadRegisteredProducer(t, tc.produced, tc.sentinel, tc.code, tc.selector)
			})
		}
	}
}

func proofLaunchpadRegisteredProducer(t *testing.T, produced error, sentinel *sdkerrors.Error, code uint32, selector string) {
	t.Helper()
	require.ErrorIs(t, produced, sentinel)
	t.Run("EVMSelector", func(t *testing.T) {
		contract, found := types.ContractErrorOf(produced)
		require.Equal(t, selector != "", found)
		if found {
			data, err := contract.Revert()
			require.NoError(t, err)
			require.Equal(t, selector, hexutil.Encode(data))
		}
	})
	t.Run("ABCI", func(t *testing.T) {
		remote := proofLaunchpadTransport(t, produced, "launchpad", code)
		require.ErrorIs(t, remote, sentinel)
		contract, found := types.ContractErrorOf(remote)
		require.Equal(t, selector != "", found)
		if found {
			data, err := contract.Revert()
			require.NoError(t, err)
			require.Equal(t, selector, hexutil.Encode(data))
		}
	})
}

func proofLaunchpadHookCode(t *testing.T, k *keeper.Keeper, ctx sdk.Context) {
	t.Helper()
	// RecordFee decodes getFlags through ModuleCaller and the real EVM. The
	// imported pool names an EOA as optical, whose staticcall returns no word.
	who := common.HexToAddress(holder)
	hookCtx := launchpadtest.WithStore(ctx, sdk.NewKVStoreKey(types.StoreKey))
	hookGenesis := types.DefaultGenesis()
	market := poolMarket()
	market.Optical = who.Hex()
	hookGenesis.Markets = []types.Market{market}
	hookGenesis.Surfaces = []types.Surface{{Address: market.PoolAddress, Role: types.RolePool}}
	hookGenesis.PoolMetadata = []types.PoolMetadata{{Pool: market.PoolAddress, Creator: who.Hex(), Token: market.TokenAddress,
		Optical: who.Hex(), NFTID: market.FeeRightID}}
	hookGenesis.FeeHeld = sdk.NewInt(10)
	k.InitGenesis(hookCtx, *hookGenesis)
	for i := 0; i < 2; i++ {
		cc, _ := hookCtx.CacheContext()
		_, _, _, err := k.RecordFee(cc, k.ModuleCaller(), market.Denom, sdk.OneInt())
		proofLaunchpadRegisteredProducer(t, err, types.ErrInvalidHookResponse, 43, "0x1e048e1d")
	}
}

func proofLaunchpadUpdateParamsRefusal(t *testing.T, k *keeper.Keeper, ctx sdk.Context) {
	t.Helper()
	before := k.GetParams(ctx)
	params := before
	params.BaseFeeBps++
	assignment := types.RoleAssignment{Contract: k.Generation2().SidioraFactory.Hex(), Account: holder, Role: poolRole}
	msg := &types.MsgUpdateParams{Authority: account(2), Params: params, RoleGrants: []types.RoleAssignment{assignment}}
	require.NotEqual(t, k.Authority(), msg.Authority)
	require.NoError(t, msg.ValidateBasic())
	wire, err := msg.Marshal()
	require.NoError(t, err)
	var decoded types.MsgUpdateParams
	require.NoError(t, decoded.Unmarshal(wire))
	server := keeper.NewMsgServerImpl(k)
	for i := 0; i < 2; i++ {
		events := len(ctx.EventManager().Events())
		logs := len(ctx.EVMEventManager().Events())
		hadRole := k.HasRole(ctx, common.HexToAddress(assignment.Contract), common.HexToHash(assignment.Role), common.HexToAddress(assignment.Account))
		response, refusal := server.UpdateParams(sdk.WrapSDKContext(ctx), &decoded)
		require.Nil(t, response)
		require.Equal(t, before, k.GetParams(ctx))
		require.Equal(t, hadRole, k.HasRole(ctx, common.HexToAddress(assignment.Contract), common.HexToHash(assignment.Role), common.HexToAddress(assignment.Account)))
		require.Len(t, ctx.EventManager().Events(), events)
		require.Len(t, ctx.EVMEventManager().Events(), logs)
		proofLaunchpadRegisteredProducer(t, refusal, types.ErrUnauthorized, 23, "0x82b42900")
	}
}
