// Proves: sdkerrors.Register launchpad errors -> ResponseDeliverTx/ABCIError -> ContractErrorOf, Params.Validate, CalculateDynamicFee, MsgUpdateParams.ValidateBasic, Keeper.CheckRole/GetSurface, ParamsFromProtocolConfig.
//
// Repairs: params.go Params.Validate, errors.go ContractError.Revert, keeper/
// roles.go, surfaces.go and migrate_v6120_read.go used fmt wrapping that
// retained Go identity but lost registered ABCI codes (ABCIInfo follows Cause).
// math.go CalculateDynamicFee also accepted negative words or panicked on nil
// words; malformed native FeeInputs must refuse InvalidParams before saturation.
//
// Findings outside touches: types/genesis.go GenesisState.Validate and keeper/
// custody.go QuoteLedger.key, MoveQuote, quoteBalance and callQuote, and keeper/
// fees.go opticalFlags use fmt wrapping. Their local sentinel/contract identity
// survives, but ABCIInfo currently emits undefined/1 rather than launchpad's
// registered code. The proof asserts that current behaviour explicitly; the
// expected eventual behaviour is preservation of the registered code.
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
		require.NoError(t, k.SetSurface(ctx, factory, want))
		got, found, err := k.GetSurface(ctx, factory)
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, want, got)
		// Damage the real producer's encoded record, then make the consumer
		// read it again. A cached answer would incorrectly pass this read.
		store := ctx.KVStore(sdk.NewKVStoreKey(types.StoreKey))
		encoded := append([]byte(nil), store.Get(types.SurfaceKey(factory))...)
		store.Set(types.SurfaceKey(factory), encoded[:len(encoded)-1])
		_, found, err = k.GetSurface(ctx, factory)
		require.False(t, found)
		require.ErrorIs(t, err, types.ErrInvalidParams)
		proofLaunchpadTransport(t, err, "launchpad", 2)
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

	for i := 0; i < 2; i++ {
		// These runtime producers are outside touches: keep their observed
		// ABCI limitation explicit while proving local contract matching.
		badGenesis := types.DefaultGenesis()
		badGenesis.AirdropEpochs = []types.AirdropEpochAmount{{}}
		_, invalidConfig := k.GetQuoteLedger(ctx, keeper.QuoteLedger{})
		_, _, invalidBalance := k.PullQuote(ctx, k.ModuleCaller(), who, keeper.TreasuryHeldLedger(), sdk.OneInt())
		cc, _ := ctx.CacheContext()
		_, transferFailed := k.PayQuote(cc, k.ModuleCaller(), keeper.TreasuryHeldLedger(), who, sdk.OneInt())
		for _, tc := range []struct {
			produced error
			sentinel *sdkerrors.Error
		}{
			{badGenesis.Validate(), types.ErrInvalidGenesis},
			{invalidConfig, types.ErrInvalidConfiguration},
			{k.MoveQuote(ctx, keeper.HeldQuoteLedger(who), keeper.HeldQuoteLedger(other), sdk.OneInt()), types.ErrInsufficientBalance},
			{invalidBalance, types.ErrInvalidBalance},
			{transferFailed, types.ErrTransferFailed},
		} {
			require.ErrorIs(t, tc.produced, tc.sentinel)
			remote := proofLaunchpadTransport(t, tc.produced, "undefined", 1)
			require.NotErrorIs(t, remote, tc.sentinel)
			_, found := types.ContractErrorOf(tc.produced)
			require.Equal(t, tc.sentinel != types.ErrInvalidGenesis, found)
		}
	}

	// RecordFee decodes getFlags through ModuleCaller and the real EVM. The
	// imported pool names an EOA as optical, whose staticcall returns no word.
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
		require.ErrorIs(t, err, types.ErrInvalidHookResponse)
		proofLaunchpadTransport(t, err, "undefined", 1)
		contract, found := types.ContractErrorOf(err)
		require.True(t, found)
		data, packErr := contract.Revert()
		require.NoError(t, packErr)
		require.Equal(t, "0x1e048e1d", hexutil.Encode(data))
	}
}
