package app_test

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types/ethtx"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"

	xevmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/engine/deps/xevm/types"
)

type gigaParamsParityOutcome struct {
	results    []*abci.ExecTxResult
	receipts   []*evmtypes.Receipt
	baseFee    string
	appHash    []byte
	err        string
	stateHash  []byte
	beforeHash []byte
}

// TestGigaParamsParity compares the ordinary and fast executors with each
// parameter key absent from the parameter store. The key list comes directly
// from the fast VM parameter set so newly added keys are covered automatically.
func TestGigaParamsParity(t *testing.T) {
	previousOCC := app.EnableOCC
	previousSkipResultsHash := tmtypes.SkipLastResultsHashValidation.Load()
	app.EnableOCC = true
	t.Cleanup(func() {
		app.EnableOCC = previousOCC
		tmtypes.SkipLastResultsHashValidation.Store(previousSkipResultsHash)
	})

	params := xevmtypes.DefaultParams()
	keys := make([][]byte, 0, len(params.ParamSetPairs()))
	for _, pair := range params.ParamSetPairs() {
		keys = append(keys, append([]byte(nil), pair.Key...))
	}
	require.NotEmpty(t, keys)

	for _, key := range keys {
		key := key
		t.Run("key_"+string(key), func(t *testing.T) {
			runGigaParamsParity(t, [][]byte{key})
		})
	}
	t.Run("all_keys", func(t *testing.T) {
		runGigaParamsParity(t, keys)
	})
}

func runGigaParamsParity(t *testing.T, removedKeys [][]byte) {
	t.Helper()
	const gasLimit uint64 = 1_000_000
	timestamp := time.Unix(1_800_000_000, 0).UTC()
	validator := utils.NewTestAccounts(1)[0]
	payer := utils.NewSigner()
	recipient := utils.NewSigner()
	genesis, err := json.Marshal(app.NewDefaultGenesisState(app.MakeEncodingConfig().Marshaler))
	require.NoError(t, err)

	newApplication := func(giga bool) *app.TestWrapper {
		a := app.SetupWithSc(t, true, true, app.TestAppOpts{
			UseSc: true, EnableGiga: true, EnableGigaOCC: true,
		}, func(ba *baseapp.BaseApp) {
			ba.SetOccEnabled(true)
			ba.SetConcurrencyWorkers(1)
		})
		t.Cleanup(func() { require.NoError(t, a.Close()) })
		_, err := a.InitChain(context.Background(), &abci.RequestInitChain{
			ChainId: chainsBlockChainID, Time: timestamp, InitialHeight: 1,
			ConsensusParams: app.DefaultConsensusParams, AppStateBytes: genesis,
		})
		require.NoError(t, err)
		a.GigaExecutorEnabled = giga
		ctx := a.NewContext(false, tmproto.Header{
			Height: 1, ChainID: chainsBlockChainID, Time: timestamp,
			ProposerAddress: validator.PublicKey.Address(),
		})
		ctx = ctx.WithConsensusParams(a.GetConsensusParams(ctx))
		a.DistrKeeper.SetPreviousProposerConsAddr(ctx, sdk.ConsAddress(validator.PublicKey.Address()))
		a.EvmKeeper.SetAddressMapping(ctx, payer.AccountAddress, payer.EvmAddress)
		a.EvmKeeper.SetAddressMapping(ctx, recipient.AccountAddress, recipient.EvmAddress)
		funding := sdk.NewCoins(sdk.NewInt64Coin("uhpx", 10_000_000_000))
		require.NoError(t, a.BankKeeper.MintCoins(ctx, "mint", funding))
		require.NoError(t, a.BankKeeper.SendCoinsFromModuleToAccount(ctx, "mint", payer.AccountAddress, funding))
		for _, key := range removedKeys {
			storeKey := append([]byte(a.EvmKeeper.Paramstore.Name()+"/"), key...)
			ctx.KVStore(a.GetKey(paramtypes.StoreKey)).Delete(storeKey)
		}
		return &app.TestWrapper{App: a, Ctx: ctx}
	}

	ordinary := newApplication(false)
	fast := newApplication(true)
	transactions, signed, contract, revertContract := gigaParamsParityTransactions(t, ordinary, &payer, &recipient, gasLimit)
	want := applyGigaParamsParityBlock(t, ordinary, transactions, signed, contract, revertContract)
	got := applyGigaParamsParityBlock(t, fast, transactions, signed, contract, revertContract)

	if want.err != "" || got.err != "" {
		// A missing value can make both executors refuse the block. The
		// refusal layers may report different text, but neither path may
		// commit state.
		require.NotEmpty(t, want.err)
		require.NotEmpty(t, got.err)
		t.Logf("ordinary refusal: %q; fast refusal: %q", want.err, got.err)
		require.Equal(t, want.beforeHash, want.stateHash)
		require.Equal(t, got.beforeHash, got.stateHash)
		require.Equal(t, want.stateHash, got.stateHash)
		return
	}
	require.Len(t, want.results, len(got.results))
	for i := range want.results {
		require.Equal(t, want.results[i].Code, got.results[i].Code)
		require.Equal(t, want.results[i].Codespace, got.results[i].Codespace)
		require.Equal(t, want.results[i].GasWanted, got.results[i].GasWanted)
		require.Equal(t, want.results[i].GasUsed, got.results[i].GasUsed)
		require.Equal(t, normalizeGigaParamsResultData(t, want.results[i].Data), normalizeGigaParamsResultData(t, got.results[i].Data))
		if want.results[i].Code == 0 {
			require.Equal(t, want.results[i].Log, got.results[i].Log)
		} else {
			require.Equal(t, want.results[i].EvmTxInfo.GetTxHash(), got.results[i].EvmTxInfo.GetTxHash())
			require.Equal(t, want.results[i].EvmTxInfo.GetNonce(), got.results[i].EvmTxInfo.GetNonce())
		}
	}
	for i := range want.receipts {
		wantReceipt := *want.receipts[i]
		gotReceipt := *got.receipts[i]
		wantReceipt.VmError = ""
		gotReceipt.VmError = ""
		require.Equal(t, &wantReceipt, &gotReceipt)
	}
	require.Equal(t, want.baseFee, got.baseFee)
	require.Equal(t, want.appHash, got.appHash)
}

func normalizeGigaParamsResultData(t *testing.T, data []byte) []byte {
	t.Helper()
	var txMsgData sdk.TxMsgData
	if err := txMsgData.Unmarshal(data); err != nil {
		return data
	}
	for _, msgData := range txMsgData.Data {
		var response evmtypes.MsgEVMTransactionResponse
		if err := response.Unmarshal(msgData.Data); err != nil {
			return data
		}
		response.VmError = ""
		encoded, err := response.Marshal()
		require.NoError(t, err)
		msgData.Data = encoded
	}
	encoded, err := txMsgData.Marshal()
	require.NoError(t, err)
	return encoded
}

func applyGigaParamsParityBlock(t *testing.T, w *app.TestWrapper, txs [][]byte, signed []*ethtypes.Transaction, contract, revertContract common.Address) gigaParamsParityOutcome {
	t.Helper()
	before := append([]byte(nil), w.App.LastCommitID().Hash...)
	response, err := w.App.FinalizeBlock(context.Background(), &abci.RequestFinalizeBlock{
		Header: &tmproto.Header{
			Height: w.Ctx.BlockHeight(), ChainID: chainsBlockChainID, Time: w.Ctx.BlockTime(),
		},
		Txs: txs,
	})
	if err != nil {
		after := append([]byte(nil), w.App.LastCommitID().Hash...)
		return gigaParamsParityOutcome{err: err.Error(), beforeHash: before, stateHash: after}
	}
	if response == nil {
		return gigaParamsParityOutcome{err: "nil finalize response", beforeHash: before, stateHash: before}
	}
	for _, result := range response.TxResults {
		if result.Code != 0 && result.Code != sdkerrors.ErrEVMVMError.ABCICode() {
			after := append([]byte(nil), w.App.LastCommitID().Hash...)
			return gigaParamsParityOutcome{err: fmt.Sprintf("%d:%s", result.Code, result.Log), beforeHash: before, stateHash: after}
		}
	}
	if _, err := w.App.Commit(context.Background()); err != nil {
		return gigaParamsParityOutcome{err: err.Error(), beforeHash: before, stateHash: before}
	}
	ctx := w.App.GetCheckCtx()
	out := gigaParamsParityOutcome{results: response.TxResults, appHash: append([]byte(nil), response.AppHash...), stateHash: append([]byte(nil), response.AppHash...), beforeHash: before}
	out.baseFee = w.App.EvmKeeper.GetNextBaseFeePerGas(ctx).String()
	for _, tx := range signed {
		receipt, err := w.App.EvmKeeper.GetReceipt(ctx, tx.Hash())
		if err != nil {
			return gigaParamsParityOutcome{err: err.Error(), beforeHash: before, stateHash: out.stateHash}
		}
		out.receipts = append(out.receipts, receipt)
	}
	for _, address := range []common.Address{contract, revertContract} {
		require.NotEmpty(t, w.App.EvmKeeper.GetCode(ctx, address), address.Hex())
	}
	return out
}

func gigaParamsParityTransactions(t *testing.T, w *app.TestWrapper, payer, recipient *utils.TestAcct, gasLimit uint64) ([][]byte, []*ethtypes.Transaction, common.Address, common.Address) {
	t.Helper()
	chainID := w.App.EvmKeeper.ChainID(w.Ctx)
	encode := func(nonce uint64, txType byte, to *common.Address, value *big.Int, data []byte) ([]byte, *ethtypes.Transaction) {
		t.Helper()
		var tx *ethtypes.Transaction
		if txType == ethtypes.DynamicFeeTxType {
			tx = ethtypes.NewTx(&ethtypes.DynamicFeeTx{
				ChainID: chainID, Nonce: nonce, GasTipCap: big.NewInt(2_000_000_000),
				GasFeeCap: big.NewInt(500_000_000_000), Gas: gasLimit, To: to, Value: value, Data: data,
			})
		} else {
			tx = ethtypes.NewTx(&ethtypes.LegacyTx{
				Nonce: nonce, GasPrice: big.NewInt(500_000_000_000), Gas: gasLimit, To: to, Value: value, Data: data,
			})
		}
		signed, err := ethtypes.SignTx(tx, ethtypes.LatestSignerForChainID(chainID), payer.EvmPrivateKey)
		require.NoError(t, err)
		txData, err := ethtx.NewTxDataFromTx(signed)
		require.NoError(t, err)
		msg, err := evmtypes.NewMsgEVMTransaction(txData)
		require.NoError(t, err)
		builder := app.MakeEncodingConfig().TxConfig.NewTxBuilder()
		require.NoError(t, builder.SetMsgs(msg))
		builder.SetGasLimit(gasLimit)
		encoded, err := app.MakeEncodingConfig().TxConfig.TxEncoder()(builder.GetTx())
		require.NoError(t, err)
		return encoded, signed
	}

	// The first creation writes three storage slots and returns code that writes
	// another slot and emits a log when called.
	initCode, err := hex.DecodeString("60016000556002600155600360025560006000a0600b6020600039600b6000f3600460035560006000a000")
	require.NoError(t, err)
	contract := crypto.CreateAddress(payer.EvmAddress, 1)
	creation, signedCreation := encode(1, ethtypes.LegacyTxType, nil, new(big.Int), initCode)
	call, signedCall := encode(2, ethtypes.LegacyTxType, &contract, new(big.Int), nil)

	// This creation returns a five-byte runtime that always reverts.
	revertInit := []byte{0x60, 0x05, 0x60, 0x0c, 0x60, 0x00, 0x39, 0x60, 0x05, 0x60, 0x00, 0xf3, 0x60, 0x00, 0x60, 0x00, 0xfd}
	revertContract := crypto.CreateAddress(payer.EvmAddress, 3)
	revertCreation, signedRevertCreation := encode(3, ethtypes.LegacyTxType, nil, new(big.Int), revertInit)
	revertCall, signedRevertCall := encode(4, ethtypes.LegacyTxType, &revertContract, new(big.Int), nil)
	transfer, signedTransfer := encode(0, ethtypes.LegacyTxType, &recipient.EvmAddress, big.NewInt(1), nil)
	priority, signedPriority := encode(5, ethtypes.DynamicFeeTxType, &recipient.EvmAddress, big.NewInt(1), nil)
	return [][]byte{transfer, creation, call, revertCreation, revertCall, priority},
		[]*ethtypes.Transaction{signedTransfer, signedCreation, signedCall, signedRevertCreation, signedRevertCall, signedPriority}, contract, revertContract
}
