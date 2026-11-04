package keeper_test

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"math/big"
	"path/filepath"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	"github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm"
	"github.com/tendermint/tm-db"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
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
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

func domainFeeContext(t *testing.T) (*app.App, sdk.Context) {
	t.Helper()
	return domainFeeContextWithDB(t, db.NewMemDB(), t.TempDir())
}

func openDomainFeeApp(t *testing.T, database db.DB, home string) *app.App {
	t.Helper()
	return app.New(database, nil, true, map[int64]bool{}, home, 1, false,
		config.TestConfig(), app.MakeEncodingConfig(), wasm.EnableAllProposals,
		app.TestAppOpts{}, app.EmptyWasmOpts, nil)
}

func domainFeeContextWithDB(t *testing.T, database db.DB, home string) (*app.App, sdk.Context) {
	t.Helper()
	a := openDomainFeeApp(t, database, home)
	genesis, err := json.Marshal(app.NewDefaultGenesisState(app.MakeEncodingConfig().Marshaler))
	require.NoError(t, err)
	_, err = a.InitChain(context.Background(), &abci.RequestInitChain{
		Time: time.Now(), ConsensusParams: app.DefaultConsensusParams,
		ChainId: "pax-test", AppStateBytes: genesis,
	})
	require.NoError(t, err)
	ctx := a.GetContextForDeliverTx(nil).WithBlockHeight(1)
	ctx = ctx.WithConsensusParams(&tmproto.ConsensusParams{Block: &tmproto.BlockParams{MaxGas: 100000}})
	k := &a.EvmKeeper
	k.BeginBlock(ctx)
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
	return createFeeDomainWithBaseFee(t, a, ctx, name, share, sdk.MustNewDecFromStr("10.001"))
}

func createFeeDomainWithBaseFee(t *testing.T, a *app.App, ctx sdk.Context, name string, share uint16, baseFee sdk.Dec) chainstypes.Domain {
	t.Helper()
	key, err := crypto.GenerateKey()
	require.NoError(t, err)
	operator := crypto.PubkeyToAddress(key.PublicKey)
	fundDomainAccount(t, a, ctx, sdk.AccAddress(operator.Bytes()), sdk.NewInt(1000000))
	domain, err := a.LayerXChainsKeeper.Create(ctx, operator, name, 42000, 16384, share, sdk.OneInt(), baseFee)
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

	firstMsg, _ := signedDomainMessage(t, key, chainstypes.ChainIDOf(busy.Id), 0, 100)
	secondMsg, _ := signedDomainMessage(t, key, chainstypes.ChainIDOf(busy.Id), 1, 100)
	rootMsg, _ := signedDomainMessage(t, key, a.EvmKeeper.ChainID(ctx), 2, 100)
	a.EvmKeeper.SetMsgs([]*types.MsgEVMTransaction{firstMsg, secondMsg, rootMsg})
	a.EvmKeeper.SetTxResults([]*abci.ExecTxResult{{}, {}, {}})
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
	proveOperatorFeeSettlement(t)
}

func proveOperatorFeeSettlement(t *testing.T) {
	t.Helper()
	for _, share := range []uint16{3333, 0, 5000, 10000} {
		t.Run(sdk.NewInt(int64(share)).String(), func(t *testing.T) {
			directory := filepath.Join(t.TempDir(), "committed")
			database, err := db.NewGoLevelDB("application", directory)
			require.NoError(t, err)
			home := filepath.Join(directory, "node")
			a, ctx := domainFeeContextWithDB(t, database, home)
			t.Cleanup(func() { require.NoError(t, a.Close()) })
			if share > a.LayerXChainsKeeper.GetParams(ctx).MaxFeeShareBps {
				_, err := a.LayerXChainsKeeper.Create(ctx, common.HexToAddress("0x124213"), "operator-chain", 42000, 16384, share, sdk.OneInt(), sdk.MustNewDecFromStr("10000000001.001"))
				require.ErrorIs(t, err, chainstypes.ErrAllowanceOutOfRange)
				require.Zero(t, a.LayerXChainsKeeper.DomainCount(ctx))
				return
			}
			k := &a.EvmKeeper
			params := k.GetParams(ctx)
			params.MinimumFeePerGas = types.DefaultMinFeePerGas
			params.MaximumFeePerGas = types.DefaultMaxFeePerGas
			require.NoError(t, params.Validate())
			k.SetParams(ctx, params)
			domain := createFeeDomainWithBaseFee(t, a, ctx, "operator-chain", share, sdk.MustNewDecFromStr("10000000001.001"))
			key, err := crypto.GenerateKey()
			require.NoError(t, err)
			msg, hash := signedDomainMessage(t, key, chainstypes.ChainIDOf(domain.Id), 0, 30_000_000_000)
			require.NoError(t, ante.Preprocess(ctx, msg, chainstypes.ChainIDOf(domain.Id), false))
			sender := msg.Derived.SenderPaxAddr
			fundDomainAccount(t, a, ctx, sender, sdk.NewInt(1_000_000))
			operator := k.GetPaxAddressOrDefault(ctx, domain.Operator)
			collector := a.AccountKeeper.GetModuleAddress(authtypes.FeeCollectorName)
			balance := func(c sdk.Context, addr sdk.AccAddress) sdk.Int { return sdk.NewIntFromBigInt(k.GetBalance(c, addr)) }
			senderBefore, operatorBefore := balance(ctx, sender), balance(ctx, operator)
			require.True(t, balance(ctx, collector).IsZero(), "collector must be funded only by execution")
			baseFee := a.LayerXChainsKeeper.NextBaseFee(ctx, domain.Id).TruncateInt()
			require.True(t, baseFee.GTE(params.MinimumFeePerGas.TruncateInt()))
			msg.Derived = nil
			builder := a.GetTxConfig().NewTxBuilder()
			require.NoError(t, builder.SetMsgs(msg))
			wire, err := a.GetTxConfig().TxEncoder()(builder.GetTx())
			require.NoError(t, err)
			decodedTx, err := a.GetTxConfig().TxDecoder()(wire)
			require.NoError(t, err)
			msg = decodedTx.GetMsgs()[0].(*types.MsgEVMTransaction)
			require.Nil(t, msg.Derived)
			executionCtx, err := a.AnteHandler(ctx.WithTxIndex(0), decodedTx, false)
			require.NoError(t, err)
			response, err := keeper.NewMsgServerImpl(k).EVMTransaction(sdk.WrapSDKContext(executionCtx), msg)
			require.NoError(t, err)
			require.Empty(t, response.VmError)
			require.Equal(t, uint64(21000), response.GasUsed)
			receipt, err := k.GetTransientReceipt(ctx, hash, 0)
			require.NoError(t, err)
			require.Equal(t, uint32(ethtypes.ReceiptStatusSuccessful), receipt.Status)
			k.SetMsgs([]*types.MsgEVMTransaction{msg})
			k.SetTxResults([]*abci.ExecTxResult{{}})
			k.EndBlock(ctx, ctx.BlockHeight(), 0)
			fee := baseFee.Mul(sdk.NewIntFromUint64(response.GasUsed))
			payout := fee.MulRaw(int64(share)).QuoRaw(10000)
			assertState := func(c sdk.Context) {
				require.Equal(t, fee.String(), senderBefore.Sub(balance(c, sender)).String())
				require.Equal(t, payout.String(), balance(c, operator).Sub(operatorBefore).String())
				require.Equal(t, fee.Sub(payout).String(), balance(c, collector).String())
				require.Equal(t, senderBefore.Add(operatorBefore).String(), balance(c, sender).Add(balance(c, operator)).Add(balance(c, collector)).String())
				require.True(t, balance(c, state.GetCoinbaseAddress(0)).IsZero())
				require.Equal(t, []common.Hash{hash}, a.LayerXChainsKeeper.DomainTxs(c, domain.Id, 1))
				require.Equal(t, response.GasUsed, a.LayerXChainsKeeper.DomainGasUsed(c, domain.Id, 1))
			}
			assertState(ctx)
			if share == 3333 {
				_, remainder := state.SplitUhpxWeiAmount(payout.BigInt())
				require.True(t, remainder.IsPositive(), "exercise sub-native-unit settlement")
			}
			nextFee := a.LayerXChainsKeeper.NextBaseFee(ctx, domain.Id)
			require.NoError(t, a.HandlePreCommit(ctx))
			a.SetDeliverStateToCommit()
			committed := a.WriteState().Commit(true)
			require.Positive(t, committed.Version)
			require.NoError(t, a.Close())
			reopenedDB, err := db.NewGoLevelDB("application", directory)
			require.NoError(t, err)
			a = openDomainFeeApp(t, reopenedDB, home)
			k = &a.EvmKeeper
			require.Equal(t, committed.Version, a.CommitMultiStore().LastCommitID().Version)
			recoveredCtx := a.NewUncachedContext(false, ctx.BlockHeader())
			assertState(recoveredCtx)
			require.True(t, nextFee.Equal(a.LayerXChainsKeeper.NextBaseFee(recoveredCtx, domain.Id)))
		})
	}
}
