package keeper_test

import (
	"math/big"
	"testing"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/ante"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types/ethtx"
	chainstypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxchains/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

type repair180Block struct {
	app     *app.App
	ctx     sdk.Context
	msgs    []*types.MsgEVMTransaction
	results []*abci.ExecTxResult
}

func repair180CreateDomain(t *testing.T, a *app.App, ctx sdk.Context, name string, gas uint64, bps uint16) chainstypes.Domain {
	t.Helper()
	params := a.LayerXChainsKeeper.GetParams(ctx)
	params.MaxFeeShareBps = 10000
	a.LayerXChainsKeeper.SetParams(ctx, params)
	key, err := crypto.GenerateKey()
	require.NoError(t, err)
	operator := crypto.PubkeyToAddress(key.PublicKey)
	fundDomainAccount(t, a, ctx, sdk.AccAddress(operator.Bytes()), sdk.NewInt(1_000_000))
	domain, err := a.LayerXChainsKeeper.Create(ctx, operator, name, gas, 16384, bps, sdk.OneInt(), sdk.MustNewDecFromStr("10.001"))
	require.NoError(t, err)
	return domain
}

func (b *repair180Block) execute(t *testing.T, id uint32, tip int64, feeDenom string, revert bool) sdk.Int {
	t.Helper()
	k := &b.app.EvmKeeper
	collector := b.app.AccountKeeper.GetModuleAddress(authtypes.FeeCollectorName)
	collectorCoins := b.app.BankKeeper.GetAllBalances(b.ctx, collector)
	collectorWei := b.app.BankKeeper.GetWeiBalance(b.ctx, collector)
	key, err := crypto.GenerateKey()
	require.NoError(t, err)
	sender := crypto.PubkeyToAddress(key.PublicKey)
	payer := sdk.AccAddress(sender.Bytes())
	fundDomainAccount(t, b.app, b.ctx, payer, sdk.NewInt(1_000_000))
	if feeDenom != k.GetBaseDenom(b.ctx) {
		coins := sdk.NewCoins(sdk.NewInt64Coin(feeDenom, 1_000_000))
		require.NoError(t, b.app.BankKeeper.MintCoins(b.ctx, types.ModuleName, coins))
		require.NoError(t, b.app.BankKeeper.SendCoinsFromModuleToAccount(b.ctx, types.ModuleName, payer, coins))
		require.NoError(t, k.SetAccountFeeDenom(b.ctx, sender, feeDenom))
	}
	chainID := k.ChainID(b.ctx)
	baseFee := k.GetNextBaseFeePerGas(b.ctx).TruncateInt()
	if id != 0 {
		chainID = chainstypes.ChainIDOf(id)
		baseFee = b.app.LayerXChainsKeeper.NextBaseFee(b.ctx, id).TruncateInt()
	}
	target := common.HexToAddress("0x180180")
	if revert {
		k.SetCode(b.ctx, target, []byte{0x60, 0, 0x60, 0, 0xfd})
	}
	tx, err := ethtypes.SignTx(ethtypes.NewTx(&ethtypes.DynamicFeeTx{
		ChainID: chainID, Gas: 30000, GasFeeCap: baseFee.AddRaw(tip + 100).BigInt(), GasTipCap: big.NewInt(tip), To: &target,
	}), ethtypes.LatestSignerForChainID(chainID), key)
	require.NoError(t, err)
	data, err := ethtx.NewDynamicFeeTx(tx)
	require.NoError(t, err)
	msg, err := types.NewMsgEVMTransaction(data)
	require.NoError(t, err)
	idx := len(b.msgs)
	ctx := b.ctx.WithTxIndex(idx).WithIsEVM(true).WithEVMNonce(0).WithEVMSenderAddress(sender).WithEVMTxHash(tx.Hash())
	if id != 0 {
		ctx = chainstypes.WithDomain(ctx, id)
	}
	require.NoError(t, ante.Preprocess(ctx, msg, chainID, false))
	builder := b.app.GetTxConfig().NewTxBuilder()
	require.NoError(t, builder.SetMsgs(msg))
	feeCheck := ante.NewEVMFeeCheckDecorator(k, &b.app.UpgradeKeeper)
	feeCheck.ChainsFeeReader = b.app.LayerXChainsKeeper
	executionCtx, err := feeCheck.AnteHandle(ctx, builder.GetTx(), false, func(next sdk.Context, _ sdk.Tx, _ bool) (sdk.Context, error) { return next, nil })
	require.NoError(t, err)
	payerBefore := sdk.NewIntFromBigInt(k.GetBalance(b.ctx, payer))
	response, err := keeper.NewMsgServerImpl(k).EVMTransaction(sdk.WrapSDKContext(executionCtx), msg)
	require.NoError(t, err)
	receipt, err := k.GetTransientReceipt(b.ctx, tx.Hash(), uint64(idx))
	require.NoError(t, err)
	require.Equal(t, response.GasUsed, receipt.GasUsed)
	require.Less(t, receipt.GasUsed, uint64(30000))
	require.Equal(t, collectorCoins, b.app.BankKeeper.GetAllBalances(b.ctx, collector))
	require.Equal(t, collectorWei, b.app.BankKeeper.GetWeiBalance(b.ctx, collector))
	if revert {
		require.NotEmpty(t, response.VmError)
		require.Equal(t, uint32(ethtypes.ReceiptStatusFailed), receipt.Status)
	} else {
		require.Empty(t, response.VmError)
		require.Equal(t, uint32(ethtypes.ReceiptStatusSuccessful), receipt.Status)
	}
	gas := sdk.NewIntFromUint64(receipt.GasUsed)
	reward := baseFee.AddRaw(tip).Mul(gas)
	coinbase := state.GetCoinbaseAddress(idx)
	if feeDenom == k.GetBaseDenom(b.ctx) {
		require.Equal(t, reward.String(), sdk.NewIntFromBigInt(k.GetBalance(b.ctx, coinbase)).String())
		refund := baseFee.AddRaw(tip).Mul(sdk.NewInt(30000).Sub(gas))
		require.Equal(t, payerBefore.Add(refund).String(), sdk.NewIntFromBigInt(k.GetBalance(b.ctx, payer)).String())
	} else {
		charge, err := k.GetAnteFeeTokenCharge(b.ctx, tx.Hash())
		require.NoError(t, err)
		require.NotNil(t, charge)
		expectedReward, err := k.ConvertFeeToDenom(reward, charge.Rate, false)
		require.NoError(t, err)
		require.Equal(t, expectedReward, b.app.BankKeeper.GetBalance(b.ctx, coinbase, feeDenom).Amount)
	}
	b.msgs = append(b.msgs, msg)
	b.results = append(b.results, &abci.ExecTxResult{GasUsed: int64(receipt.GasUsed)})
	k.SetMsgs(b.msgs)
	k.SetTxResults(b.results)
	return gas
}

func TestRepair_180_011_ExactCoinAndWeiShare(t *testing.T) {
	for _, tc := range []struct {
		name   string
		base   string
		bps    uint16
		funded bool
		revert bool
	}{
		{"sub-unit-empty", "10.001", 3333, false, false},
		{"sub-unit-funded", "10.001", 3333, true, false},
		{"coin-and-wei", "1000000003.999", 3333, true, false},
		{"full-share", "1000000003.999", 10000, false, false},
		{"zero-share", "1000000003.999", 0, false, false},
		{"executed-revert", "1000000003.999", 3333, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, ctx := domainFeeContext(t)
			domain := repair180CreateDomain(t, a, ctx, "exact-share", 40000, tc.bps)
			fee := sdk.MustNewDecFromStr(tc.base)
			params := a.EvmKeeper.GetParams(ctx)
			params.MaximumFeePerGas = sdk.NewDec(1_000_000_000_000)
			a.EvmKeeper.SetParams(ctx, params)
			a.LayerXChainsKeeper.SetNextBaseFee(ctx, domain.Id, fee)
			collector := a.AccountKeeper.GetModuleAddress(authtypes.FeeCollectorName)
			denom := a.EvmKeeper.GetBaseDenom(ctx)
			require.Zero(t, a.EvmKeeper.GetBalance(ctx, collector).Sign())
			if tc.funded {
				coins := sdk.NewCoins(sdk.NewInt64Coin(denom, 1_000_000))
				require.NoError(t, a.BankKeeper.MintCoins(ctx, types.ModuleName, coins))
				require.NoError(t, a.BankKeeper.SendCoinsFromModuleToModule(ctx, types.ModuleName, authtypes.FeeCollectorName, coins))
			}
			operator := a.EvmKeeper.GetPaxAddressOrDefault(ctx, domain.Operator)
			before := sdk.NewIntFromBigInt(a.EvmKeeper.GetBalance(ctx, operator))
			collectorBefore := sdk.NewIntFromBigInt(a.EvmKeeper.GetBalance(ctx, collector))
			block := repair180Block{app: a, ctx: ctx}
			gas := block.execute(t, domain.Id, 7, denom, tc.revert)
			require.Equal(t, before.String(), sdk.NewIntFromBigInt(a.EvmKeeper.GetBalance(ctx, operator)).String())
			baseFees := fee.TruncateInt().Mul(gas)
			share := baseFees.MulRaw(int64(tc.bps)).QuoRaw(10000)
			a.EvmKeeper.EndBlock(ctx, ctx.BlockHeight(), 0)
			require.Equal(t, before.Add(share).String(), sdk.NewIntFromBigInt(a.EvmKeeper.GetBalance(ctx, operator)).String())
			coins, wei := state.SplitUhpxWeiAmount(before.Add(share).BigInt())
			require.Equal(t, coins, a.BankKeeper.GetBalance(ctx, operator, denom).Amount)
			require.Equal(t, wei, a.BankKeeper.GetWeiBalance(ctx, operator))
			require.Equal(t, collectorBefore.Add(baseFees).Add(gas.MulRaw(7)).Sub(share).String(), sdk.NewIntFromBigInt(a.EvmKeeper.GetBalance(ctx, collector)).String())
			require.Zero(t, a.EvmKeeper.GetBalance(ctx, state.GetCoinbaseAddress(0)).Sign())
			require.True(t, a.LayerXChainsKeeper.NextBaseFee(ctx, domain.Id).GT(fee))
		})
	}
}

func TestRepair_180_012_EmptyCollectorSettlesPaidDenominations(t *testing.T) {
	a, ctx := domainFeeContext(t)
	k := &a.EvmKeeper
	domain := repair180CreateDomain(t, a, ctx, "mixed-fees", 60000, 3333)
	other := repair180CreateDomain(t, a, ctx, "other-fees", 40000, 10000)
	fee := sdk.MustNewDecFromStr("1000000003.999")
	a.LayerXChainsKeeper.SetNextBaseFee(ctx, domain.Id, fee)
	a.LayerXChainsKeeper.SetNextBaseFee(ctx, other.Id, sdk.NewDec(10))
	params := k.GetParams(ctx)
	params.FeeTokenEnabled = true
	params.AllowedFeeDenoms = []types.AllowedFeeDenom{{Denom: "usid", Rate: sdk.NewDec(types.InitialSidioraBaseUnitsPerPax), RateUpdateHeight: ctx.BlockHeight()}}
	k.SetParams(ctx, params)
	collector := a.AccountKeeper.GetModuleAddress(authtypes.FeeCollectorName)
	require.Empty(t, a.BankKeeper.GetAllBalances(ctx, collector))
	require.True(t, a.BankKeeper.GetWeiBalance(ctx, collector).IsZero())
	operator := k.GetPaxAddressOrDefault(ctx, domain.Operator)
	otherOperator := k.GetPaxAddressOrDefault(ctx, other.Operator)
	nativeBefore := sdk.NewIntFromBigInt(k.GetBalance(ctx, operator))
	otherBefore := sdk.NewIntFromBigInt(k.GetBalance(ctx, otherOperator))
	tokenBefore := a.BankKeeper.GetBalance(ctx, operator, "usid").Amount
	block := repair180Block{app: a, ctx: ctx}
	tokenGas := block.execute(t, domain.Id, 3_000_000_000, "usid", false)
	nativeGas := block.execute(t, domain.Id, 7, k.GetBaseDenom(ctx), false)
	rootGas := block.execute(t, 0, 7, k.GetBaseDenom(ctx), false)
	otherGas := block.execute(t, other.Id, 0, k.GetBaseDenom(ctx), false)
	paidTokens, err := k.ConvertFeeToDenom(fee.TruncateInt().AddRaw(3_000_000_000).Mul(tokenGas), params.AllowedFeeDenoms[0].Rate, false)
	require.NoError(t, err)
	baseTokens, err := k.ConvertFeeToDenom(fee.TruncateInt().Mul(tokenGas), params.AllowedFeeDenoms[0].Rate, false)
	require.NoError(t, err)
	tokenShare := baseTokens.MulRaw(3333).QuoRaw(10000)
	nativeFees := fee.TruncateInt().Mul(nativeGas)
	nativeShare := nativeFees.MulRaw(3333).QuoRaw(10000)
	require.True(t, tokenShare.IsPositive())
	require.True(t, nativeShare.IsPositive())
	params.AllowedFeeDenoms[0].Rate = sdk.NewDec(1_000_000)
	k.SetParams(ctx, params)
	k.EndBlock(ctx, ctx.BlockHeight(), int64(rootGas.Uint64()))
	require.Equal(t, tokenBefore.Add(tokenShare), a.BankKeeper.GetBalance(ctx, operator, "usid").Amount)
	require.Equal(t, paidTokens.Sub(tokenShare), a.BankKeeper.GetBalance(ctx, collector, "usid").Amount)
	require.Equal(t, nativeBefore.Add(nativeShare).String(), sdk.NewIntFromBigInt(k.GetBalance(ctx, operator)).String())
	require.Equal(t, otherBefore.Add(otherGas.MulRaw(10)).String(), sdk.NewIntFromBigInt(k.GetBalance(ctx, otherOperator)).String())
	require.Equal(t, nativeFees.Add(nativeGas.MulRaw(7)).Sub(nativeShare).Add(rootGas.MulRaw(17)).String(), sdk.NewIntFromBigInt(k.GetBalance(ctx, collector)).String())
	for i := range block.msgs {
		require.Empty(t, a.BankKeeper.GetAllBalances(ctx, state.GetCoinbaseAddress(i)))
		require.True(t, a.BankKeeper.GetWeiBalance(ctx, state.GetCoinbaseAddress(i)).IsZero())
	}
}
