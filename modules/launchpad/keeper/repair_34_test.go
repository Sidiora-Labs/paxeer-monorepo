package keeper_test

import (
	"encoding/json"
	"math"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/launchpadtest"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

// Persist and close the real producer's export, discard the keeper, and
// reopen the document into a fresh launchpad store over the preserved EVM.
func repair34Restart(t *testing.T, e *env) {
	t.Helper()
	raw, err := json.Marshal(e.k.ExportGenesis(e.ctx))
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "launchpad-genesis.json")
	file, err := os.Create(path)
	require.NoError(t, err)
	n, err := file.Write(raw)
	require.NoError(t, err)
	require.Equal(t, len(raw), n)
	require.NoError(t, file.Sync())
	require.NoError(t, file.Close())
	e.k = nil
	raw = nil
	file, err = os.Open(path)
	require.NoError(t, err)
	var restored types.GenesisState
	require.NoError(t, json.NewDecoder(file).Decode(&restored))
	require.NoError(t, file.Close())
	require.NoError(t, restored.Validate())
	e.k, e.ctx = launchpadtest.NewKeeper(testkeeper.EVMTestApp, e.ctx)
	require.Zero(t, e.k.GetMarketCount(e.ctx))
	e.k.InitGenesis(e.ctx, restored)
	sameJSON(t, restored, e.k.ExportGenesis(e.ctx))
}

func TestRepair_34_f01_FactoryNonceRecovery(t *testing.T) {
	e := newEnv(t)
	creators := []sdk.AccAddress{e.account(10_000_000), e.account(10_000_000)}
	var markets []types.Market
	for _, creator := range creators {
		for n := 0; n < 3; n++ {
			markets = append(markets, e.create(creator, types.FeeStrategyClaim))
		}
	}
	factoryABI := proofModuleABI(t, types.RoleFactory)
	check := func() {
		evm, db := proofModuleEVM(t, e.k, e.ctx)
		for _, creator := range creators {
			owner := e.evmAddress(creator)
			require.Equal(t, uint64(3), e.k.FactoryNonce(e.ctx, owner))
			require.Equal(t, big.NewInt(3), proofModuleView(t, evm, owner, e.k.Generation2().SidioraFactory,
				factoryABI, "getNonce", owner))
		}
		for _, market := range markets {
			requireSameMarket(t, market, e.market(market.Denom))
			require.NotEmpty(t, db.GetCode(common.HexToAddress(market.TokenAddress)))
			require.Equal(t, keeper.SurfaceRuntime, db.GetCode(common.HexToAddress(market.PoolAddress)))
		}
	}
	check()
	repair34Restart(t, e)
	check()
	for _, creator := range creators {
		owner := e.evmAddress(creator)
		want := tokenAddress(t, e.k.Generation2().SidioraFactory, tokenSalt(owner, "Kindle Token", "KNDL", 3),
			"Kindle Token", "KNDL")
		market := e.create(creator, types.FeeStrategyClaim)
		require.Equal(t, want.Hex(), market.TokenAddress)
		require.Equal(t, uint64(4), e.k.FactoryNonce(e.ctx, owner))
		for _, old := range markets {
			require.NotEqual(t, old.TokenAddress, market.TokenAddress)
		}
	}
	// Nonces are exported independently of native market counts, including
	// entries imported by migration and an exhausted uint64 nonce.
	gs := types.DefaultGenesis()
	gs.Params = e.k.GetParams(e.ctx)
	gs.FactoryNonces = []types.FactoryNonce{{Creator: address("migrated creator").Hex(), Nonce: 17},
		{Creator: address("exhausted creator").Hex(), Nonce: math.MaxUint64}}
	fresh, ctx := launchpadtest.NewKeeper(testkeeper.EVMTestApp, e.ctx)
	fresh.InitGenesis(ctx, *gs)
	e.k, e.ctx = fresh, ctx
	repair34Restart(t, e)
	for _, entry := range gs.FactoryNonces {
		require.Equal(t, entry.Nonce, e.k.FactoryNonce(e.ctx, common.HexToAddress(entry.Creator)))
	}
}

func TestRepair_34_f02_OwnerEnumerationRecovery(t *testing.T) {
	e := newEnv(t)
	a, b := e.account(10_000_000), e.account(10_000_000)
	ownerA, ownerB := e.evmAddress(a), e.evmAddress(b)
	for n := 0; n < 3; n++ {
		e.create(a, types.FeeStrategyClaim)
	}
	e.create(b, types.FeeStrategyBurn)
	collection := e.k.Generation2().SidioraNFT
	nftABI := proofModuleABI(t, types.RoleFeeRights)
	operator, spender := address("repair 34 operator"), address("repair 34 spender")
	evm, db := proofModuleEVM(t, e.k, e.ctx)
	for _, transfer := range []struct {
		from, to common.Address
		id       int64
	}{
		{ownerA, ownerB, 1}, {ownerB, ownerA, 4},
	} {
		input, err := nftABI.Pack("transferFrom", transfer.from, transfer.to, big.NewInt(transfer.id))
		require.NoError(t, err)
		_, _, err = evm.Call(transfer.from, collection, input, 30_000_000, uint256.NewInt(0))
		require.NoError(t, err)
	}
	e.ctx = db.Ctx()
	require.NoError(t, e.k.ApproveFeeRight(e.ctx, collection, ownerA, spender, 2))
	e.k.SetFeeRightApprovalForAll(e.ctx, collection, ownerB, operator, true)
	check := func() {
		evm, _ := proofModuleEVM(t, e.k, e.ctx)
		require.Equal(t, big.NewInt(4), proofModuleView(t, evm, ownerA, collection, nftABI, "totalSupply"))
		for _, entry := range []struct {
			owner common.Address
			ids   []uint64
		}{
			{ownerA, []uint64{3, 2, 4}}, {ownerB, []uint64{1}},
		} {
			require.Equal(t, big.NewInt(int64(len(entry.ids))), proofModuleView(t, evm, entry.owner, collection,
				nftABI, "balanceOf", entry.owner))
			for index, id := range entry.ids {
				got, err := e.k.FeeRightOfOwnerByIndex(e.ctx, collection, entry.owner, uint64(index))
				require.NoError(t, err)
				require.Equal(t, id, got)
				require.Equal(t, new(big.Int).SetUint64(id), proofModuleView(t, evm, entry.owner, collection,
					nftABI, "tokenOfOwnerByIndex", entry.owner, big.NewInt(int64(index))))
				require.Equal(t, entry.owner, proofModuleView(t, evm, entry.owner, collection, nftABI, "ownerOf",
					new(big.Int).SetUint64(id)))
			}
			_, err := e.k.FeeRightOfOwnerByIndex(e.ctx, collection, entry.owner, uint64(len(entry.ids)))
			require.ErrorIs(t, err, types.ErrIndexOutOfBounds)
		}
		for index := int64(0); index < 4; index++ {
			require.Equal(t, big.NewInt(index+1), proofModuleView(t, evm, ownerA, collection, nftABI,
				"tokenByIndex", big.NewInt(index)))
		}
		require.Equal(t, spender, proofModuleView(t, evm, ownerA, collection, nftABI, "getApproved", big.NewInt(2)))
		require.Equal(t, common.Address{}, proofModuleView(t, evm, ownerB, collection, nftABI, "getApproved", big.NewInt(1)))
		require.Equal(t, true, proofModuleView(t, evm, ownerB, collection, nftABI, "isApprovedForAll", ownerB, operator))
	}
	check()
	repair34Restart(t, e)
	check()
	// A later removal exercises the recovered reverse owner indexes too.
	require.NoError(t, e.k.TransferFeeRight(e.ctx, collection, ownerA, ownerA, ownerB, 3))
	for index, id := range []uint64{4, 2} {
		got, err := e.k.FeeRightOfOwnerByIndex(e.ctx, collection, ownerA, uint64(index))
		require.NoError(t, err)
		require.Equal(t, id, got)
	}
}

func TestRepair_34_GenesisValidation(t *testing.T) {
	e := newEnv(t)
	a, b := e.account(10_000_000), e.account(10_000_000)
	for n := 0; n < 2; n++ {
		e.create(a, types.FeeStrategyClaim)
	}
	e.create(b, types.FeeStrategyClaim)
	raw, err := json.Marshal(e.k.ExportGenesis(e.ctx))
	require.NoError(t, err)
	for _, tc := range []struct {
		name    string
		corrupt func(*types.GenesisState)
	}{
		{"duplicate nonce", func(gs *types.GenesisState) { gs.FactoryNonces = append(gs.FactoryNonces, gs.FactoryNonces[0]) }},
		{"zero creator", func(gs *types.GenesisState) { gs.FactoryNonces[0].Creator = common.Address{}.Hex() }},
		{"malformed creator", func(gs *types.GenesisState) { gs.FactoryNonces[0].Creator = "not-an-address" }},
		{"missing owner order", func(gs *types.GenesisState) { gs.FeeRights.OwnerTokens = nil }},
		{"missing token", func(gs *types.GenesisState) { gs.FeeRights.OwnerTokens[0].TokenIDs = nil }},
		{"duplicate owner", func(gs *types.GenesisState) {
			gs.FeeRights.OwnerTokens = append(gs.FeeRights.OwnerTokens, gs.FeeRights.OwnerTokens[0])
		}},
		{"unknown token", func(gs *types.GenesisState) { gs.FeeRights.OwnerTokens[0].TokenIDs[0] = 999 }},
		{"wrong owner", func(gs *types.GenesisState) { gs.FeeRights.OwnerTokens[0].Owner = address("wrong owner").Hex() }},
		{"duplicate token", func(gs *types.GenesisState) {
			gs.FeeRights.OwnerTokens[0].TokenIDs = append(gs.FeeRights.OwnerTokens[0].TokenIDs, gs.FeeRights.OwnerTokens[0].TokenIDs[0])
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var gs types.GenesisState
			require.NoError(t, json.Unmarshal(raw, &gs))
			tc.corrupt(&gs)
			require.ErrorIs(t, gs.Validate(), types.ErrInvalidGenesis)
			fresh, ctx := launchpadtest.NewKeeper(testkeeper.EVMTestApp, e.ctx)
			require.Panics(t, func() { fresh.InitGenesis(ctx, gs) })
			require.Zero(t, fresh.GetMarketCount(ctx))
		})
	}
}

func repair34QuoteToken(t *testing.T, e *env) common.Address {
	t.Helper()
	deployer := address("repair 34 replacement quote deployer")
	args := packArguments(t, []string{"string", "string", "uint256", "address"},
		"Replacement Quote", "RQ", big.NewInt(1_000_000_000_000), reserve)
	evm := &testkeeper.EVMTestApp.EvmKeeper
	nonce := evm.GetNonce(e.ctx, deployer)
	_, err := evm.CallEVM(e.ctx, deployer, nil, nil, append(common.FromHex(sidioraERC20Creation), args...))
	require.NoError(t, err)
	token := crypto.CreateAddress(deployer, nonce)
	require.NotEmpty(t, evm.GetCode(e.ctx, token))
	return token
}

func TestRepair_34_f04_QuoteTokenGovernance(t *testing.T) {
	e := newEnv(t)
	creator, trader := e.account(10_000_000), e.account(100_000_000)
	market := e.create(creator, types.FeeStrategyClaim)
	e.at(mature)
	e.swap(trader, market.Denom, true, 2_000_000)
	require.True(t, e.market(market.Denom).RealQuoteBalance.IsPositive())
	require.True(t, e.market(market.Denom).AccumulatedFees.IsPositive())
	require.True(t, e.feeHeld().IsPositive())
	replacement := repair34QuoteToken(t, e)
	custody := e.k.ModuleEVMAddress(e.ctx)
	e.sendToken(replacement, reserve, custody, big.NewInt(250_000))
	params := e.k.GetParams(e.ctx)
	changed := params
	changed.QuoteToken = replacement.Hex()
	changed.BaseFeeBps++
	before := e.k.ExportGenesis(e.ctx)
	oldBalance, newBalance := e.tokenBalance(e.usdl, custody), e.tokenBalance(replacement, custody)
	oldOwner, newOwner := e.usdlOf(creator), e.tokenBalance(replacement, e.evmAddress(creator))
	events, logs := len(e.ctx.EventManager().Events()), len(e.ctx.EVMEventManager().Events())
	server := keeper.NewMsgServerImpl(e.k)
	response, err := server.UpdateParams(sdk.WrapSDKContext(e.ctx), &types.MsgUpdateParams{
		Authority: e.k.Authority(), Params: changed,
		RoleGrants: []types.RoleAssignment{{Contract: e.k.Generation2().SidioraFactory.Hex(),
			Role: keeper.FeeRightMinterRole.Hex(), Account: e.evmAddress(trader).Hex()}},
	})
	require.Nil(t, response)
	require.ErrorIs(t, err, types.ErrInvalidParams)
	sameJSON(t, params, e.k.GetParams(e.ctx))
	sameJSON(t, before, e.k.ExportGenesis(e.ctx))
	require.Equal(t, oldBalance, e.tokenBalance(e.usdl, custody))
	require.Equal(t, newBalance, e.tokenBalance(replacement, custody))
	require.Equal(t, oldOwner, e.usdlOf(creator))
	require.Equal(t, newOwner, e.tokenBalance(replacement, e.evmAddress(creator)))
	require.Len(t, e.ctx.EventManager().Events(), events)
	require.Len(t, e.ctx.EVMEventManager().Events(), logs)
	claim, err := e.k.ClaimFees(e.ctx, creator, market.Denom, creator)
	require.NoError(t, err)
	require.True(t, claim.IsPositive())
	require.Equal(t, oldOwner.Add(claim), e.usdlOf(creator))
	require.Equal(t, oldBalance.Sub(claim), e.tokenBalance(e.usdl, custody))
	require.Equal(t, newBalance, e.tokenBalance(replacement, custody))
	// An update retaining the denomination still works for a funded market.
	params.BaseFeeBps++
	params.QuoteToken = strings.ToLower(params.QuoteToken)
	_, err = server.UpdateParams(sdk.WrapSDKContext(e.ctx), &types.MsgUpdateParams{Authority: e.k.Authority(), Params: params})
	require.NoError(t, err)
}

func TestRepair_34_f04_QuoteLiabilityWithoutMarkets(t *testing.T) {
	for _, kind := range []string{"empty", "held quote", "fee held", "fee liabilities", "empty market"} {
		t.Run(kind, func(t *testing.T) {
			e := newEnv(t)
			user := e.account(1_000_000)
			switch kind {
			case "held quote", "fee held":
				ledger := keeper.HeldQuoteLedger(address("orphan pool"))
				if kind == "fee held" {
					ledger = keeper.FeeHeldLedger(e.k.Generation2().FeeAccumulator)
				}
				var err error
				e.ctx, _, err = e.k.PullQuote(e.ctx, e.k.ModuleCaller(), e.evmAddress(user), ledger, i(100))
				require.NoError(t, err)
			case "fee liabilities":
				// Load a validated genesis carrying fee liabilities even with no markets.
				gs := types.DefaultGenesis()
				gs.Params = e.k.GetParams(e.ctx)
				gs.FeeHeld, gs.FeeAccounting.TotalLiabilities = i(100), i(100)
				e.k, e.ctx = launchpadtest.NewKeeper(testkeeper.EVMTestApp, e.ctx)
				e.k.InitGenesis(e.ctx, *gs)
			case "empty market":
				market := e.create(user, types.FeeStrategyClaim)
				require.True(t, market.RealQuoteBalance.IsZero())
			}
			params := e.k.GetParams(e.ctx)
			changed := params
			changed.QuoteToken = repair34QuoteToken(t, e).Hex()
			_, err := keeper.NewMsgServerImpl(e.k).UpdateParams(sdk.WrapSDKContext(e.ctx),
				&types.MsgUpdateParams{Authority: e.k.Authority(), Params: changed})
			if kind == "empty" {
				require.NoError(t, err)
				sameJSON(t, changed, e.k.GetParams(e.ctx))
			} else {
				require.ErrorIs(t, err, types.ErrInvalidParams)
				sameJSON(t, params, e.k.GetParams(e.ctx))
			}
		})
	}
}

func repair34RequireUnauthorized(t *testing.T, produced error) {
	t.Helper()
	require.ErrorIs(t, produced, types.ErrUnauthorized)
	response := sdkerrors.ResponseDeliverTx(produced, 100, 25, false)
	require.Equal(t, "launchpad", response.Codespace)
	require.Equal(t, uint32(23), response.Code)
	wire, err := response.Marshal()
	require.NoError(t, err)
	var observed abci.ResponseDeliverTx
	require.NoError(t, observed.Unmarshal(wire))
	require.Equal(t, response, observed)
	require.ErrorIs(t, sdkerrors.ABCIError(observed.Codespace, observed.Code, observed.Log), types.ErrUnauthorized)
}

func TestRepair_34_f05_UnauthorizedABCI(t *testing.T) {
	e := newEnv(t)
	creator := e.account(10_000_000)
	e.create(creator, types.FeeStrategyClaim)
	outsider := e.account(0)
	params := e.k.GetParams(e.ctx)
	changed := params
	changed.BaseFeeBps++
	before := e.k.ExportGenesis(e.ctx)
	events, logs := len(e.ctx.EventManager().Events()), len(e.ctx.EVMEventManager().Events())
	repair34RequireUnauthorized(t, e.k.UpdateParams(e.ctx, outsider.String(), changed))
	message := &types.MsgUpdateParams{
		Authority: outsider.String(), Params: changed,
		RoleGrants: []types.RoleAssignment{{Contract: e.k.Generation2().SidioraFactory.Hex(),
			Role: keeper.FeeRightMinterRole.Hex(), Account: e.evmAddress(outsider).Hex()}},
	}
	response, err := keeper.NewMsgServerImpl(e.k).UpdateParams(sdk.WrapSDKContext(e.ctx), message)
	require.Nil(t, response)
	repair34RequireUnauthorized(t, err)
	// Decode the real message's protobuf and dispatch through the registered
	// native service router before encoding its refusal for the ABCI client.
	wire, err := message.Marshal()
	require.NoError(t, err)
	var decoded types.MsgUpdateParams
	require.NoError(t, decoded.Unmarshal(wire))
	router := baseapp.NewMsgServiceRouter()
	router.SetInterfaceRegistry(testkeeper.EVMTestApp.InterfaceRegistry())
	types.RegisterMsgServer(router, keeper.NewMsgServerImpl(e.k))
	handler := router.Handler(&decoded)
	require.NotNil(t, handler)
	result, err := handler(e.ctx, &decoded)
	require.Nil(t, result)
	repair34RequireUnauthorized(t, err)
	// The audit's other active unauthorized producer is the native Treasury
	// payout guard; it must preserve the same registered client error.
	_, err = e.k.TreasuryWithdraw(e.ctx, e.k.ModuleCaller(), outsider.String(), e.usdl, e.evmAddress(outsider), i(1))
	repair34RequireUnauthorized(t, err)
	sameJSON(t, before, e.k.ExportGenesis(e.ctx))
	sameJSON(t, params, e.k.GetParams(e.ctx))
	require.Len(t, e.ctx.EventManager().Events(), events)
	require.Len(t, e.ctx.EVMEventManager().Events(), logs)
}
