package keeper_test

import (
	"math/big"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxgov"
	layerxgovtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxgov/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/spot/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/spot/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

func NewProposalHandler(t *testing.T) (govtypes.Handler, sdk.Context, *keeper.Keeper) {
	t.Helper()
	app := testkeeper.EVMTestApp
	ctx, _ := app.GetContextForDeliverTx(nil).CacheContext()
	ctx = ctx.WithBlockHeight(10).WithEvmEventManager(sdk.NewEVMEventManager())
	ctx = ctx.WithGasMeter(sdk.NewInfiniteGasMeterWithMultiplier(ctx))
	k := app.SpotKeeper
	gs := types.DefaultGenesis()
	gs.Params.Tokens = nil
	k.InitGenesis(ctx, *gs)
	require.Equal(t, layerxgovtypes.GovernanceAuthority(), k.Authority())
	for _, msg := range []sdk.Msg{&types.MsgUpdateParams{}, &types.MsgSyncReserves{}} {
		require.NotNil(t, app.MsgServiceRouter().Handler(msg))
	}
	return layerxgov.NewProposalHandler(app.MsgServiceRouter()), ctx, k
}

func TestLayerXProposalRoutesSpotUpdateParams(t *testing.T) {
	handler, ctx, k := NewProposalHandler(t)
	before, err := k.GetParams(ctx)
	require.NoError(t, err)
	params := before
	params.SwapFeeBps = 9
	require.NotEqual(t, before.SwapFeeBps, params.SwapFeeBps)
	proposal, err := layerxgovtypes.NewLayerXProposal("Update spot fees", "Set the spot swap fee through governance.",
		&types.MsgUpdateParams{Authority: layerxgovtypes.GovernanceAuthority(), Params: params})
	require.NoError(t, err)
	require.NoError(t, proposal.ValidateBasic())
	require.NoError(t, handler(ctx, proposal))
	after, err := k.GetParams(ctx)
	require.NoError(t, err)
	require.Equal(t, uint64(9), after.SwapFeeBps)
	require.Equal(t, params, after)
}

func TestLayerXProposalRoutesSpotSyncReserves(t *testing.T) {
	handler, ctx, k := NewProposalHandler(t)
	app := testkeeper.EVMTestApp
	usdl := common.HexToAddress(types.USDLAddress)
	holder := common.BytesToAddress(crypto.Keccak256([]byte("spot governance USDL holder"))[12:])
	deployer := common.BytesToAddress(crypto.Keccak256([]byte("spot governance USDL deployer"))[12:])
	stringType, err := abi.NewType("string", "", nil)
	require.NoError(t, err)
	uintType, err := abi.NewType("uint256", "", nil)
	require.NoError(t, err)
	addressType, err := abi.NewType("address", "", nil)
	require.NoError(t, err)
	args, err := abi.Arguments{{Type: stringType}, {Type: stringType}, {Type: uintType}, {Type: addressType}}.
		Pack("USD Ledger", "USDL", big.NewInt(1_300), holder)
	require.NoError(t, err)
	nonce := app.EvmKeeper.GetNonce(ctx, deployer)
	_, err = app.EvmKeeper.CallEVM(ctx, deployer, nil, nil, append(common.FromHex(limitTokenCreation), args...))
	require.NoError(t, err)
	deployed := crypto.CreateAddress(deployer, nonce)
	code := app.EvmKeeper.GetCode(ctx, deployed)
	require.NotEmpty(t, code)
	var slots [][2]common.Hash
	app.EvmKeeper.IterateState(ctx, func(address common.Address, key, value common.Hash) bool {
		if address == deployed {
			slots = append(slots, [2]common.Hash{key, value})
		}
		return false
	})
	require.NotEmpty(t, slots)
	app.EvmKeeper.SetCode(ctx, usdl, code)
	for _, slot := range slots {
		app.EvmKeeper.SetState(ctx, usdl, slot[0], slot[1])
	}

	gs := types.DefaultGenesis()
	gs.Params.Tokens = []types.TokenParams{{Address: usdl, Kind: types.TokenKindERC20, IsStablecoin: true,
		Decimals: types.OpeningTokenDecimals, MaxUtilizationBps: types.OpeningTokenMaxUtilizationBps}}
	gs.Books = []types.TokenBook{{Token: usdl, Reserves: sdk.NewInt(1_000), TotalDeposited: sdk.NewInt(1_000),
		TotalWithdrawn: sdk.ZeroInt(), Decimals: types.OpeningTokenDecimals, ShareDenom: k.ShareDenom(usdl)}}
	k.InitGenesis(ctx, *gs)
	custodyAddress := k.CustodyEVMAddress(ctx)
	require.Equal(t, app.EvmKeeper.GetEVMAddressOrDefault(ctx, keeper.CustodyAddress()), custodyAddress)
	input := append(append(crypto.Keccak256([]byte("transfer(address,uint256)"))[:4],
		common.LeftPadBytes(custodyAddress.Bytes(), 32)...), word(1_300)...)
	ret, err := app.EvmKeeper.CallEVM(ctx, holder, &usdl, nil, input)
	require.NoError(t, err)
	require.Equal(t, word(1), ret)
	_, custody, err := k.CustodyBalance(ctx, usdl)
	require.NoError(t, err)
	require.Equal(t, sdk.NewInt(1_300), custody)
	before, err := k.GetBook(ctx, usdl)
	require.NoError(t, err)
	require.Equal(t, sdk.NewInt(1_000), before.Reserves)

	proposal, err := layerxgovtypes.NewLayerXProposal("Sync spot reserves", "Recover the direct USDL transfer into spot reserves.",
		&types.MsgSyncReserves{Authority: layerxgovtypes.GovernanceAuthority(), Tokens: []string{usdl.Hex()}})
	require.NoError(t, err)
	require.NoError(t, proposal.ValidateBasic())
	logStart := len(ctx.EVMEventManager().Events())
	require.NoError(t, handler(ctx, proposal))
	after, err := k.GetBook(ctx, usdl)
	require.NoError(t, err)
	want := before
	want.Reserves = custody
	want.TotalDeposited = before.TotalDeposited.AddRaw(300)
	require.Equal(t, want, after)
	_, afterCustody, err := k.CustodyBalance(ctx, usdl)
	require.NoError(t, err)
	require.Equal(t, custody, afterCustody)
	logs := logsSince(ctx, logStart)
	require.Len(t, logs, 2)
	tokenTopic := common.BytesToHash(usdl.Bytes())
	requireLog(t, logs[0], pecorVault, []common.Hash{keeper.ReservesSyncTopic, tokenTopic},
		word(1_000), word(1_300), word(300))
	requireLog(t, logs[1], pecorVault, []common.Hash{keeper.ReservesUpdatedTopic, tokenTopic}, word(1_000), word(1_300))
}
