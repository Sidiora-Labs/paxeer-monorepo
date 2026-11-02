package keeper_test

import (
	"crypto/ecdsa"
	"math/big"
	"testing"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/ante"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types/ethtx"
	chainstypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxchains/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

func domainFeeContext(t *testing.T) (*app.App, sdk.Context) {
	t.Helper()
	a := testkeeper.EVMTestApp
	ctx, _ := a.GetContextForDeliverTx(nil).WithBlockHeight(1).CacheContext()
	ctx = ctx.WithConsensusParams(&tmproto.ConsensusParams{Block: &tmproto.BlockParams{MaxGas: 100000}})
	k := &a.EvmKeeper
	params := k.GetParams(ctx)
	params.BaseFeePerGas = sdk.NewDec(10)
	params.MinimumFeePerGas = sdk.NewDec(10)
	params.MaximumFeePerGas = sdk.NewDec(1000)
	params.MaxDynamicBaseFeeUpwardAdjustment = sdk.MustNewDecFromStr("0.5")
	params.MaxDynamicBaseFeeDownwardAdjustment = sdk.MustNewDecFromStr("0.5")
	params.TargetGasUsedPerBlock = 50000
	k.SetParams(ctx, params)
	k.SetNextBaseFeePerGas(ctx, sdk.NewDec(10))
	chainsParams := a.LayerXChainsKeeper.GetParams(ctx)
	chainsParams.MinBond = sdk.OneInt()
	chainsParams.CreationFee = sdk.ZeroInt()
	chainsParams.MinGasAllowance = 21000
	a.LayerXChainsKeeper.SetParams(ctx, chainsParams)
	return a, ctx
}

func fundDomainAccount(t *testing.T, a *app.App, ctx sdk.Context, address sdk.AccAddress, amount sdk.Int) {
	t.Helper()
	coins := sdk.NewCoins(sdk.NewCoin(a.EvmKeeper.GetBaseDenom(ctx), amount))
	require.NoError(t, a.BankKeeper.MintCoins(ctx, types.ModuleName, coins))
	require.NoError(t, a.BankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, address, coins))
}

func createFeeDomain(t *testing.T, a *app.App, ctx sdk.Context, name string, share uint16) chainstypes.Domain {
	t.Helper()
	key, err := crypto.GenerateKey()
	require.NoError(t, err)
	operator := crypto.PubkeyToAddress(key.PublicKey)
	fundDomainAccount(t, a, ctx, sdk.AccAddress(operator.Bytes()), sdk.NewInt(1000000))
	domain, err := a.LayerXChainsKeeper.Create(ctx, operator, name, 42000, 16384, share, sdk.OneInt(), sdk.MustNewDecFromStr("10.001"))
	require.NoError(t, err)
	return domain
}

func signedDomainMessage(t *testing.T, key *ecdsa.PrivateKey, chainID *big.Int, nonce uint64, fee int64) (*types.MsgEVMTransaction, common.Hash) {
	t.Helper()
	to := common.HexToAddress("0x123456")
	tx, err := ethtypes.SignTx(ethtypes.NewTx(&ethtypes.DynamicFeeTx{
		ChainID: chainID, Nonce: nonce, Gas: 21000, GasFeeCap: big.NewInt(fee), GasTipCap: big.NewInt(0), To: &to, Value: big.NewInt(0),
	}), ethtypes.LatestSignerForChainID(chainID), key)
	require.NoError(t, err)
	data, err := ethtx.NewDynamicFeeTx(tx)
	require.NoError(t, err)
	msg, err := types.NewMsgEVMTransaction(data)
	require.NoError(t, err)
	return msg, tx.Hash()
}

func executeDomainTransfer(t *testing.T, a *app.App, ctx sdk.Context, key *ecdsa.PrivateKey, chainID *big.Int, nonce uint64) (common.Hash, uint64) {
	t.Helper()
	msg, hash := signedDomainMessage(t, key, chainID, nonce, 100)
	ctx = ctx.WithIsEVM(true).WithEVMNonce(nonce).WithEVMSenderAddress(crypto.PubkeyToAddress(key.PublicKey)).WithEVMTxHash(hash)
	require.NoError(t, ante.Preprocess(ctx, msg, chainID, false))
	builder := a.GetTxConfig().NewTxBuilder()
	require.NoError(t, builder.SetMsgs(msg))
	executionCtx, err := ante.NewEVMFeeCheckDecorator(&a.EvmKeeper, &a.UpgradeKeeper).AnteHandle(ctx, builder.GetTx(), false,
		func(nextCtx sdk.Context, _ sdk.Tx, _ bool) (sdk.Context, error) { return nextCtx, nil })
	require.NoError(t, err)
	response, err := keeper.NewMsgServerImpl(&a.EvmKeeper).EVMTransaction(sdk.WrapSDKContext(executionCtx), msg)
	require.NoError(t, err)
	require.Empty(t, response.VmError)
	require.Equal(t, uint64(21000), response.GasUsed)
	receipt, err := a.EvmKeeper.GetTransientReceipt(ctx, hash, uint64(ctx.TxIndex()))
	require.NoError(t, err)
	require.Equal(t, uint32(ethtypes.ReceiptStatusSuccessful), receipt.Status)
	return hash, response.GasUsed
}

func TestChainTransactionIsIndexedAndRaisesItsOwnBaseFee(t *testing.T) {
	a, ctx := domainFeeContext(t)
	busy := createFeeDomain(t, a, ctx, "busy-chain", 0)
	idle := createFeeDomain(t, a, ctx, "idle-chain", 0)
	key, err := crypto.GenerateKey()
	require.NoError(t, err)
	fundDomainAccount(t, a, ctx, sdk.AccAddress(crypto.PubkeyToAddress(key.PublicKey).Bytes()), sdk.NewInt(1000000))
	first, firstGas := executeDomainTransfer(t, a, ctx.WithTxIndex(0), key, chainstypes.ChainIDOf(busy.Id), 0)
	second, secondGas := executeDomainTransfer(t, a, ctx.WithTxIndex(1), key, chainstypes.ChainIDOf(busy.Id), 1)
	rootHash, rootGas := executeDomainTransfer(t, a, ctx.WithTxIndex(2), key, a.EvmKeeper.ChainID(ctx), 2)
	require.Equal(t, busy.GasAllowance, firstGas+secondGas)
	require.Equal(t, []common.Hash{first, second}, a.LayerXChainsKeeper.DomainTxs(ctx, busy.Id, 1))
	require.NotContains(t, a.LayerXChainsKeeper.DomainTxs(ctx, busy.Id, 1), rootHash)
	require.Equal(t, busy.GasAllowance, a.LayerXChainsKeeper.DomainGasUsed(ctx, busy.Id, 1))
	require.Zero(t, a.LayerXChainsKeeper.DomainGasUsed(ctx, idle.Id, 1))
	require.Empty(t, a.LayerXChainsKeeper.DomainTxs(ctx, idle.Id, 1))

	before := a.LayerXChainsKeeper.NextBaseFee(ctx, busy.Id)
	a.EvmKeeper.SetNextBaseFeePerGas(ctx, sdk.NewDec(100))
	a.EvmKeeper.EndBlock(ctx, ctx.BlockHeight(), int64(rootGas))
	require.True(t, a.LayerXChainsKeeper.NextBaseFee(ctx, busy.Id).Equal(before.Mul(sdk.MustNewDecFromStr("1.5"))))
	require.True(t, a.LayerXChainsKeeper.NextBaseFee(ctx, busy.Id).GT(before))
	require.True(t, a.LayerXChainsKeeper.NextBaseFee(ctx, idle.Id).Equal(a.EvmKeeper.GetMinimumFeePerGas(ctx)))
	require.True(t, a.EvmKeeper.GetNextBaseFeePerGas(ctx).Equal(sdk.NewDec(71)))
	require.Equal(t, []common.Hash{first, second}, a.LayerXChainsKeeper.DomainTxs(ctx, busy.Id, 1))

	lowFee, _ := signedDomainMessage(t, key, chainstypes.ChainIDOf(busy.Id), 3, 14)
	require.NoError(t, ante.Preprocess(ctx, lowFee, chainstypes.ChainIDOf(busy.Id), false))
	_, err = keeper.NewMsgServerImpl(&a.EvmKeeper).EVMTransaction(sdk.WrapSDKContext(ctx.WithTxIndex(3)), lowFee)
	require.ErrorIs(t, err, sdkerrors.ErrInsufficientFee)
	require.Equal(t, busy.GasAllowance, a.LayerXChainsKeeper.DomainGasUsed(ctx, busy.Id, 1))
}

func TestOperatorReceivesTheFeeShareAtEndBlock(t *testing.T) {
	for _, scenario := range []struct {
		name   string
		share  uint16
		funded bool
	}{
		{"funded", 3333, true}, {"zero-share", 0, true}, {"insufficient-collector", 3333, false},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			a, ctx := domainFeeContext(t)
			domain := createFeeDomain(t, a, ctx, "operator-chain", scenario.share)
			key, err := crypto.GenerateKey()
			require.NoError(t, err)
			fundDomainAccount(t, a, ctx, sdk.AccAddress(crypto.PubkeyToAddress(key.PublicKey).Bytes()), sdk.NewInt(1000000))
			hash, gas := executeDomainTransfer(t, a, ctx.WithTxIndex(0), key, chainstypes.ChainIDOf(domain.Id), 0)
			require.Equal(t, []common.Hash{hash}, a.LayerXChainsKeeper.DomainTxs(ctx, domain.Id, 1))
			require.Equal(t, gas, a.LayerXChainsKeeper.DomainGasUsed(ctx, domain.Id, 1))
			denom := a.EvmKeeper.GetBaseDenom(ctx)
			collector := a.AccountKeeper.GetModuleAddress(authtypes.FeeCollectorName)
			if scenario.funded {
				coins := sdk.NewCoins(sdk.NewCoin(denom, sdk.NewInt(1000000)))
				require.NoError(t, a.BankKeeper.MintCoins(ctx, types.ModuleName, coins))
				require.NoError(t, a.BankKeeper.SendCoinsFromModuleToModule(ctx, types.ModuleName, authtypes.FeeCollectorName, coins))
			}
			operator := a.EvmKeeper.GetPaxAddressOrDefault(ctx, domain.Operator)
			before := a.BankKeeper.GetBalance(ctx, operator, denom).Amount
			collectorBefore := a.BankKeeper.GetBalance(ctx, collector, denom).Amount
			a.EvmKeeper.EndBlock(ctx, ctx.BlockHeight(), 0)
			expected := sdk.ZeroInt()
			if scenario.funded && scenario.share != 0 {
				expected = sdk.NewInt(69999)
			}
			require.Equal(t, expected.String(), a.BankKeeper.GetBalance(ctx, operator, denom).Amount.Sub(before).String())
			require.Equal(t, collectorBefore.Sub(expected).String(), a.BankKeeper.GetBalance(ctx, collector, denom).Amount.String())
		})
	}
}
