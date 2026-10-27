package xweb_test

import (
	"bytes"
	"encoding/binary"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/keeper"
	xwebtestutil "github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/testutil"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

type mergePlanSuite struct {
	t         *testing.T
	app       *app.App
	k         keeper.Keeper
	requester common.Address
	attestors []xwebtestutil.Attestor
}

func TestMergePlanKeepsOriginOneStateAndOpensOriginTwo(t *testing.T) {
	const planHeight = int64(100)
	testApp := app.Setup(t, false, false, false)
	ctx := testApp.GetContextForDeliverTx([]byte{}).WithBlockHeight(planHeight - 1).
		WithBlockTime(time.Unix(1_800_000_000, 0))
	k, ctx := xwebtestutil.NewKeeper(testApp, ctx)
	attestors := xwebtestutil.Attestors(3)
	genesis := types.DefaultGenesis()
	genesis.Params.Fee = sdk.NewInt(1_000_003)
	genesis.Params.TimeoutBlocks = 1
	genesis.Params.KernelNetworkId = 7
	genesis.Params.ProgramFeeDenoms = []string{types.ProgramFeeDenomPAX}
	genesis.Attestors = xwebtestutil.Set(attestors, 2)
	genesis.Paused = false
	require.NoError(t, genesis.Validate())
	k.InitGenesis(ctx, *genesis)
	s := mergePlanSuite{t: t, app: testApp, k: k,
		requester: common.HexToAddress("0x00000000000000000000000000000000000a11ce"),
		attestors: attestors}
	payer := testApp.EvmKeeper.GetPaxAddressOrDefault(ctx, s.requester)
	coins := sdk.NewCoins(sdk.NewCoin(sdk.MustGetBaseDenom(), genesis.Params.Fee.MulRaw(10)))
	require.NoError(t, testApp.BankKeeper.MintCoins(ctx, "evm", coins))
	require.NoError(t, testApp.BankKeeper.SendCoinsFromModuleToAccount(ctx, "evm", payer, coins))
	principal := sdk.AccAddress(bytes.Repeat([]byte{0xa1}, 20))
	programCoins := sdk.NewCoins(sdk.NewCoin(types.ProgramFeeDenomPAX, sdk.NewInt(150)))
	require.NoError(t, testApp.BankKeeper.MintCoins(ctx, "evm", programCoins))
	require.NoError(t, testApp.BankKeeper.SendCoinsFromModuleToAccount(ctx, "evm", principal, programCoins))

	s.ProgramOriginActive(ctx, false)
	fulfilled := s.Request(ctx)
	fulfilled, result := s.fulfil(ctx, fulfilled)
	expiring := s.Request(ctx)
	require.Equal(t, planHeight, expiring.TimeoutHeight)
	_, err := k.Refund(ctx, expiring.ID)
	require.ErrorIs(t, err, types.ErrNotExpired)
	s.RequestFromProgram(ctx, false)

	before := k.ExportGenesis(ctx)
	balances := func(at sdk.Context) []sdk.Coins {
		accounts := []sdk.AccAddress{payer, k.ModuleAddress(), principal, keeper.ProgramFeeAccount(types.Hash32{9})}
		for _, attestor := range attestors {
			accounts = append(accounts, attestor.Payout)
		}
		out := make([]sdk.Coins, 0, len(accounts))
		for _, account := range accounts {
			out = append(out, testApp.BankKeeper.GetAllBalances(at, account))
		}
		return out
	}
	bankBefore := balances(ctx)
	atPlan := ctx.WithBlockHeight(planHeight)
	require.Equal(t, "v7.0", keeper.ProgramOriginPlan)
	testApp.UpgradeKeeper.SetDone(atPlan, keeper.ProgramOriginPlan)
	require.Equal(t, planHeight, testApp.UpgradeKeeper.GetDoneHeight(atPlan, keeper.ProgramOriginPlan))
	require.Equal(t, before, k.ExportGenesis(atPlan), "activation preserves the entire xweb store")
	require.Equal(t, bankBefore, balances(atPlan), "activation moves no origin-1 fees or payouts")

	s.ProgramOriginActive(ctx, false)
	s.RequestFromProgram(ctx, false)
	s.ProgramOriginActive(atPlan, true)
	storedRequest, found := k.GetRequest(atPlan, fulfilled.ID)
	require.True(t, found)
	require.Equal(t, fulfilled, storedRequest)
	storedResult, found := k.GetResult(atPlan, fulfilled.ID)
	require.True(t, found)
	require.Equal(t, result, storedResult)
	storedPending, found := k.GetRequest(atPlan, expiring.ID)
	require.True(t, found)
	require.Equal(t, expiring, storedPending)
	_, err = k.Refund(atPlan, fulfilled.ID)
	require.ErrorIs(t, err, types.ErrAlreadyFulfilled)
	s.Refund(atPlan, expiring)
	s.RequestFromProgram(atPlan, true)

	newRequest := s.Request(atPlan)
	require.Equal(t, expiring.ID+1, newRequest.ID, "origin 2 consumes no origin-1 nonce")
	s.fulfil(atPlan, newRequest)
	storedRequest, found = k.GetRequest(atPlan, fulfilled.ID)
	require.True(t, found)
	require.Equal(t, fulfilled, storedRequest)
	storedResult, found = k.GetResult(atPlan, fulfilled.ID)
	require.True(t, found)
	require.Equal(t, result, storedResult)
	require.Equal(t, genesis.Params, k.GetParams(atPlan))
	require.Equal(t, genesis.Attestors, k.GetAttestorSet(atPlan))
	require.False(t, k.IsPaused(atPlan))

	after := k.ExportGenesis(atPlan)
	later := atPlan.WithBlockHeight(planHeight + 1)
	s.ProgramOriginActive(later, true)
	require.Equal(t, after, k.ExportGenesis(later))
}

func (s mergePlanSuite) ProgramOriginActive(ctx sdk.Context, active bool) {
	s.t.Helper()
	require.Equal(s.t, active, s.k.ProgramOriginActive(ctx), "origin-2 activation at height %d", ctx.BlockHeight())
}

func (s mergePlanSuite) Request(ctx sdk.Context) types.Request {
	s.t.Helper()
	params := s.k.GetParams(ctx)
	payload := []byte("https://paxeer.app/")
	const callbackGas = uint64(200_000)
	payer := s.app.EvmKeeper.GetPaxAddressOrDefault(ctx, s.requester)
	balance := s.app.BankKeeper.GetBalance(ctx, payer, sdk.MustGetBaseDenom()).Amount
	escrow := s.app.BankKeeper.GetBalance(ctx, s.k.ModuleAddress(), sdk.MustGetBaseDenom()).Amount
	nonce := s.k.Nonce(ctx)
	id, err := s.k.Request(ctx, s.requester, types.KindFetch, payload, callbackGas, params.Fee)
	require.NoError(s.t, err)
	require.Equal(s.t, nonce+1, id)
	require.Equal(s.t, id, s.k.Nonce(ctx))
	request, found := s.k.GetRequest(ctx, id)
	require.True(s.t, found)
	require.Equal(s.t, types.Request{
		ID: id, Requester: types.Address20(s.requester), Kind: types.KindFetch,
		PayloadHash: types.Keccak(payload), CallbackGas: callbackGas, Fee: params.Fee,
		Height: ctx.BlockHeight(), TimeoutHeight: ctx.BlockHeight() + int64(params.TimeoutBlocks),
		Status: types.StatusPending, Level: types.LevelMajority,
	}, request)
	require.Equal(s.t, balance.Sub(params.Fee), s.app.BankKeeper.GetBalance(ctx, payer, sdk.MustGetBaseDenom()).Amount)
	require.Equal(s.t, escrow.Add(params.Fee), s.app.BankKeeper.GetBalance(ctx, s.k.ModuleAddress(), sdk.MustGetBaseDenom()).Amount)
	return request
}

func (s mergePlanSuite) fulfil(ctx sdk.Context, request types.Request) (types.Request, types.Result) {
	s.t.Helper()
	response := []byte("Paxeer X Network")
	content := types.Keccak([]byte("full origin-1 content"))
	const fullLength = uint32(9000)
	attestation := types.Attestation{
		Origin: types.OriginEVM, NetworkID: s.app.EvmKeeper.ChainID(ctx),
		Requester: types.EVMRequester(request.Requester), RequestID: request.ID,
		Kind: request.Kind, PayloadHash: request.PayloadHash, ContentDigest: content,
		ResponseHash: types.Keccak(response), FullLength: fullLength,
	}
	payouts := make([]sdk.Int, 2)
	for i, attestor := range s.attestors[:2] {
		payouts[i] = s.app.BankKeeper.GetBalance(ctx, attestor.Payout, sdk.MustGetBaseDenom()).Amount
	}
	fulfilled, result, err := s.k.Fulfil(ctx, request.ID, response, content, fullLength,
		xwebtestutil.Sign(types.Digest(attestation), s.attestors[:2]...))
	require.NoError(s.t, err)
	request.Status = types.StatusFulfilled
	require.Equal(s.t, request, fulfilled)
	require.Equal(s.t, types.Result{
		RequestID: request.ID, Response: response, ContentDigest: content, FullLength: fullLength,
		Signers: []types.Address20{s.attestors[0].Signer, s.attestors[1].Signer},
		Height:  ctx.BlockHeight(), Callback: types.CallbackPending, Level: types.LevelMajority,
	}, result)
	for i, attestor := range s.attestors[:2] {
		share := request.Fee.QuoRaw(2)
		if i == 0 {
			share = share.Add(request.Fee.ModRaw(2))
		}
		require.Equal(s.t, payouts[i].Add(share), s.app.BankKeeper.GetBalance(ctx, attestor.Payout, sdk.MustGetBaseDenom()).Amount)
	}
	require.NoError(s.t, s.k.RecordCallback(ctx, request.ID, types.CallbackDelivered, 21_000))
	result.Callback = types.CallbackDelivered
	result.CallbackGasUsed = 21_000
	stored, found := s.k.GetResult(ctx, request.ID)
	require.True(s.t, found)
	require.Equal(s.t, result, stored)
	return fulfilled, result
}

func (s mergePlanSuite) Refund(ctx sdk.Context, pending types.Request) {
	s.t.Helper()
	payer := s.app.EvmKeeper.GetPaxAddressOrDefault(ctx, s.requester)
	balance := s.app.BankKeeper.GetBalance(ctx, payer, sdk.MustGetBaseDenom()).Amount
	escrow := s.app.BankKeeper.GetBalance(ctx, s.k.ModuleAddress(), sdk.MustGetBaseDenom()).Amount
	refunded, err := s.k.Refund(ctx, pending.ID)
	require.NoError(s.t, err)
	pending.Status = types.StatusRefunded
	require.Equal(s.t, pending, refunded)
	stored, found := s.k.GetRequest(ctx, pending.ID)
	require.True(s.t, found)
	require.Equal(s.t, pending, stored)
	require.Equal(s.t, balance.Add(pending.Fee), s.app.BankKeeper.GetBalance(ctx, payer, sdk.MustGetBaseDenom()).Amount)
	require.True(s.t, escrow.Sub(pending.Fee).Equal(s.app.BankKeeper.GetBalance(ctx, s.k.ModuleAddress(), sdk.MustGetBaseDenom()).Amount))
	before := s.k.ExportGenesis(ctx)
	_, err = s.k.Refund(ctx, pending.ID)
	require.ErrorIs(s.t, err, types.ErrRefunded)
	require.Equal(s.t, before, s.k.ExportGenesis(ctx))
	require.Equal(s.t, balance.Add(pending.Fee), s.app.BankKeeper.GetBalance(ctx, payer, sdk.MustGetBaseDenom()).Amount)
	_, found = s.k.GetResult(ctx, pending.ID)
	require.False(s.t, found)
}

func (s mergePlanSuite) RequestFromProgram(ctx sdk.Context, active bool) {
	s.t.Helper()
	const requestID = uint64(1)
	principal := sdk.AccAddress(bytes.Repeat([]byte{0xa1}, 20))
	programID, asset := types.Hash32{0x41}, types.Hash32{9}
	feeAccount := keeper.ProgramFeeAccount(asset)
	paid := sdk.NewInt(75)
	payload := []byte("origin-2 merge request")
	data := binary.BigEndian.AppendUint64(nil, requestID)
	data = append(data, types.KindSearch)
	data = binary.BigEndian.AppendUint32(data, uint32(len(payload)))
	data = append(data, payload...)
	call := &keeper.ProgramCall{
		ProgramID: programID, Principal: principal, Sequence: 42,
		Legs: []keeper.ProgramLeg{{From: principal, To: feeAccount, Asset: asset,
			Denom: types.ProgramFeeDenomPAX, Amount: paid}},
	}
	before := s.k.ExportGenesis(ctx)
	balance := s.app.BankKeeper.GetBalance(ctx, principal, types.ProgramFeeDenomPAX).Amount
	escrow := s.app.BankKeeper.GetBalance(ctx, feeAccount, types.ProgramFeeDenomPAX).Amount
	cached, write := ctx.CacheContext()
	require.NoError(s.t, s.app.BankKeeper.SendCoins(cached, principal, feeAccount,
		sdk.NewCoins(sdk.NewCoin(types.ProgramFeeDenomPAX, paid))))
	err := s.k.RequestFromProgram(cached, call, []byte(keeper.ProgramRequestTopic), data)
	if !active {
		require.EqualError(s.t, err, "LXP_MODULE_DISABLED")
		require.ErrorIs(s.t, err, keeper.ErrProgramModuleDisabled)
		require.Equal(s.t, before, s.k.ExportGenesis(cached))
		_, found, readErr := s.k.GetProgramPendingRecord(ctx, programID, requestID)
		require.NoError(s.t, readErr)
		require.False(s.t, found)
		require.Zero(s.t, s.k.ProgramPendingCount(ctx))
		require.Equal(s.t, balance, s.app.BankKeeper.GetBalance(ctx, principal, types.ProgramFeeDenomPAX).Amount)
		require.Equal(s.t, escrow, s.app.BankKeeper.GetBalance(ctx, feeAccount, types.ProgramFeeDenomPAX).Amount)
		return
	}
	require.NoError(s.t, err)
	write()
	record, found, err := s.k.GetProgramPendingRecord(ctx, programID, requestID)
	require.NoError(s.t, err)
	require.True(s.t, found)
	require.Equal(s.t, keeper.ProgramPendingRecord{
		Kind: types.KindSearch, PayloadHash: types.Keccak(payload), FeeAsset: asset,
		FeeAccount: keeper.ProgramFeeAccountID(asset), Amount: paid, Sequence: 42,
	}, record)
	require.Equal(s.t, uint64(1), s.k.ProgramPendingCount(ctx))
	require.Zero(s.t, s.k.ProgramCommittedCount(ctx))
	denom, found := s.k.ProgramFeeDenom(ctx, asset)
	require.True(s.t, found)
	require.Equal(s.t, types.ProgramFeeDenomPAX, denom)
	require.Equal(s.t, balance.Sub(paid), s.app.BankKeeper.GetBalance(ctx, principal, types.ProgramFeeDenomPAX).Amount)
	require.Equal(s.t, escrow.Add(paid), s.app.BankKeeper.GetBalance(ctx, feeAccount, types.ProgramFeeDenomPAX).Amount)
	after := s.k.ExportGenesis(ctx)
	require.Equal(s.t, before.Requests, after.Requests)
	require.Equal(s.t, before.Results, after.Results)
	require.Equal(s.t, before.Nonce, after.Nonce)
}
