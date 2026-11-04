package keeper_test

import (
	"encoding/json"
	"math"
	"path/filepath"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
)

func TestRepairM_1248_ProgramResultProof(t *testing.T) {
	TestProofGoApiProducerTransportConsumer(t)
}

func TestRepairM_1248_TimeoutParameters(t *testing.T) {
	s := newSuite(t, true)
	s.request()
	server := keeper.NewMsgServerImpl(s.k)
	msg := types.MsgSetParams{Authority: authority, Fee: sdk.NewInt(fee), MaxPayloadBytes: payloadCap,
		MaxCallbackGas: callbackCap, TimeoutBlocks: uint64(math.MaxInt64),
		KernelNetworkId: kernelNetworkID, ProgramFeeDenoms: programFeeDenoms}
	require.Equal(t, uint64(math.MaxInt64), types.TimeoutBlocksLimit)
	require.NoError(t, msg.ValidateBasic())
	_, err := server.SetParams(sdk.WrapSDKContext(s.ctx), &msg)
	require.NoError(t, err)
	require.Equal(t, uint64(math.MaxInt64), s.k.GetParams(s.ctx).TimeoutBlocks)
	before := s.k.ExportGenesis(s.ctx)
	require.NoError(t, before.Validate())
	payerBefore, escrowBefore := s.balance(s.account(requester)), s.balance(s.k.ModuleAddress())
	eventsBefore := len(s.ctx.EventManager().Events())
	for _, timeoutBlocks := range []uint64{0, uint64(math.MaxInt64) + 1, math.MaxUint64} {
		bad := msg
		bad.TimeoutBlocks = timeoutBlocks
		require.ErrorIs(t, bad.ValidateBasic(), types.ErrInvalidParams)
		params := s.k.GetParams(s.ctx)
		params.TimeoutBlocks = timeoutBlocks
		require.ErrorIs(t, params.Validate(), types.ErrInvalidParams)
		require.ErrorIs(t, s.k.SetParams(s.ctx, params), types.ErrInvalidParams)
		_, err := server.SetParams(sdk.WrapSDKContext(s.ctx), &bad)
		require.ErrorIs(t, err, types.ErrInvalidParams)
		require.Equal(t, before, s.k.ExportGenesis(s.ctx))
		require.Equal(t, payerBefore, s.balance(s.account(requester)))
		require.Equal(t, escrowBefore, s.balance(s.k.ModuleAddress()))
		require.Len(t, s.ctx.EventManager().Events(), eventsBefore)
	}
}

func TestRepairM_1248_TimeoutHeightBoundaries(t *testing.T) {
	s := newSuite(t, true)
	s.request()
	for _, tc := range []struct {
		name    string
		timeout uint64
		height  int64
		accept  bool
	}{
		{"maximum timeout at zero", math.MaxInt64, 0, true},
		{"maximum timeout overflow", math.MaxInt64, 1, false},
		{"maximum timeout at positive height", math.MaxInt64 - 1, 1, true},
		{"maximum height minus one", 1, math.MaxInt64 - 1, true},
		{"maximum height", 1, math.MaxInt64, false},
		{"configured timeout at last safe height", timeout, math.MaxInt64 - int64(timeout), true},
		{"configured timeout past last safe height", timeout, math.MaxInt64 - int64(timeout) + 1, false},
		{"negative height", 1, -1, false},
		{"minimum height", 1, math.MinInt64, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, _ := s.ctx.CacheContext()
			ctx = ctx.WithBlockHeight(tc.height)
			params := s.k.GetParams(ctx)
			params.TimeoutBlocks = tc.timeout
			require.NoError(t, s.k.SetParams(ctx, params))
			before := s.k.ExportGenesis(ctx)
			payer := s.account(requester)
			payerBefore := s.app.BankKeeper.GetBalance(ctx, payer, sdk.MustGetBaseDenom())
			escrowBefore := s.app.BankKeeper.GetBalance(ctx, s.k.ModuleAddress(), sdk.MustGetBaseDenom())
			eventsBefore := len(ctx.EventManager().Events())
			id, err := s.k.Request(ctx, requester, types.KindFetch, payload, callbackGas, sdk.NewInt(fee))
			if !tc.accept {
				require.ErrorIs(t, err, types.ErrInvalidRequest)
				require.Zero(t, id)
				require.Equal(t, before, s.k.ExportGenesis(ctx))
				require.Equal(t, payerBefore, s.app.BankKeeper.GetBalance(ctx, payer, sdk.MustGetBaseDenom()))
				require.Equal(t, escrowBefore, s.app.BankKeeper.GetBalance(ctx, s.k.ModuleAddress(), sdk.MustGetBaseDenom()))
				require.Len(t, ctx.EventManager().Events(), eventsBefore)
				_, found := s.k.GetRequest(ctx, before.Nonce+1)
				require.False(t, found)
				return
			}
			require.NoError(t, err)
			require.Equal(t, before.Nonce+1, id)
			require.Equal(t, id, s.k.Nonce(ctx))
			request, found := s.k.GetRequest(ctx, id)
			require.True(t, found)
			require.Equal(t, tc.height, request.Height)
			require.Equal(t, int64(math.MaxInt64), request.TimeoutHeight)
			require.NoError(t, request.Validate())
			require.NoError(t, s.k.ExportGenesis(ctx).Validate())
			require.Equal(t, payerBefore.Amount.SubRaw(fee), s.app.BankKeeper.GetBalance(ctx, payer, sdk.MustGetBaseDenom()).Amount)
			require.Equal(t, escrowBefore.Amount.AddRaw(fee), s.app.BankKeeper.GetBalance(ctx, s.k.ModuleAddress(), sdk.MustGetBaseDenom()).Amount)
			_, err = s.k.Refund(ctx.WithBlockHeight(math.MaxInt64-1), id)
			require.ErrorIs(t, err, types.ErrNotExpired)
			refunded, err := s.k.Refund(ctx.WithBlockHeight(math.MaxInt64), id)
			require.NoError(t, err)
			require.Equal(t, types.StatusRefunded, refunded.Status)
			require.Equal(t, payerBefore, s.app.BankKeeper.GetBalance(ctx, payer, sdk.MustGetBaseDenom()))
			require.Equal(t, escrowBefore, s.app.BankKeeper.GetBalance(ctx, s.k.ModuleAddress(), sdk.MustGetBaseDenom()))
		})
	}
}

func TestRepairM_1248_PersistedInvalidTimeout(t *testing.T) {
	a, ctx := proofGoAPIApp(t, filepath.Join(t.TempDir(), "node"), true)
	defer func() { require.NoError(t, a.Close()) }()
	genesis := types.DefaultGenesis()
	genesis.Paused = false
	a.XWebKeeper.InitGenesis(ctx, *genesis)
	payer := a.EvmKeeper.GetPaxAddressOrDefault(ctx, requester)
	coins := sdk.NewCoins(sdk.NewCoin(sdk.MustGetBaseDenom(), genesis.Params.Fee.MulRaw(3)))
	require.NoError(t, a.BankKeeper.MintCoins(ctx, "mint", coins))
	require.NoError(t, a.BankKeeper.SendCoinsFromModuleToAccount(ctx, "mint", payer, coins))
	_, err := a.XWebKeeper.Request(ctx, requester, types.KindFetch, payload, callbackGas, genesis.Params.Fee)
	require.NoError(t, err)
	for _, timeoutBlocks := range []uint64{0, uint64(math.MaxInt64) + 1, math.MaxUint64} {
		branch, _ := ctx.CacheContext()
		params := a.XWebKeeper.GetParams(branch)
		params.TimeoutBlocks = timeoutBlocks
		encoded, err := json.Marshal(params)
		require.NoError(t, err)
		// Exercise persisted configuration independently of governance validation.
		branch.KVStore(a.GetKey(types.StoreKey)).Set(types.ParamsKey, encoded)
		before := a.XWebKeeper.ExportGenesis(branch)
		payerBefore := a.BankKeeper.GetBalance(branch, payer, sdk.MustGetBaseDenom())
		escrowBefore := a.BankKeeper.GetBalance(branch, a.XWebKeeper.ModuleAddress(), sdk.MustGetBaseDenom())
		eventsBefore := len(branch.EventManager().Events())
		id, err := a.XWebKeeper.Request(branch, requester, types.KindFetch, payload, callbackGas, params.Fee)
		require.ErrorIs(t, err, types.ErrInvalidParams)
		require.Zero(t, id)
		require.Equal(t, before, a.XWebKeeper.ExportGenesis(branch))
		require.Equal(t, payerBefore, a.BankKeeper.GetBalance(branch, payer, sdk.MustGetBaseDenom()))
		require.Equal(t, escrowBefore, a.BankKeeper.GetBalance(branch, a.XWebKeeper.ModuleAddress(), sdk.MustGetBaseDenom()))
		require.Len(t, branch.EventManager().Events(), eventsBefore)
	}
}
