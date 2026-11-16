package app_test

import (
	"context"
	"encoding/hex"
	"encoding/json"
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
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

func TestGigaPurgeParity(t *testing.T) {
	previousOCC := app.EnableOCC
	previousSkipResultsHash := tmtypes.SkipLastResultsHashValidation.Load()
	app.EnableOCC = true
	t.Cleanup(func() {
		app.EnableOCC = previousOCC
		tmtypes.SkipLastResultsHashValidation.Store(previousSkipResultsHash)
	})

	const calls = 64
	const gasLimit uint64 = 10_000_000
	timestamp := time.Unix(1_800_000_000, 0).UTC()
	validator := utils.NewTestAccounts(1)[0]
	payer := utils.NewSigner()
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
		funding := sdk.NewCoins(sdk.NewInt64Coin("uhpx", 10_000_000_000))
		require.NoError(t, a.BankKeeper.MintCoins(ctx, "mint", funding))
		require.NoError(t, a.BankKeeper.SendCoinsFromModuleToAccount(ctx, "mint", payer.AccountAddress, funding))
		require.False(t, a.Sequenced(ctx, ctx.BlockHeight()))
		return &app.TestWrapper{App: a, Ctx: ctx}
	}
	ordinary, giga := newApplication(false), newApplication(true)
	require.NotNil(t, giga.App.GigaEvmKeeper.EvmoneVM)
	require.True(t, giga.App.GigaOCCEnabled)
	require.Equal(t, gigaStorageGasState(t, ordinary), gigaStorageGasState(t, giga))

	initCode := gigaPurgeCreationCode(t, calls)
	contract := crypto.CreateAddress(payer.EvmAddress, 0)
	child := crypto.CreateAddress(contract, 1)
	chainID := ordinary.App.EvmKeeper.ChainID(ordinary.Ctx)
	signed, err := ethtypes.SignTx(ethtypes.NewTx(&ethtypes.LegacyTx{
		GasPrice: big.NewInt(500_000_000_000), Gas: gasLimit,
		Value: new(big.Int), Data: initCode,
	}), ethtypes.LatestSignerForChainID(chainID), payer.EvmPrivateKey)
	require.NoError(t, err)
	txData, err := ethtx.NewTxDataFromTx(signed)
	require.NoError(t, err)
	msg, err := evmtypes.NewMsgEVMTransaction(txData)
	require.NoError(t, err)
	txConfig := app.MakeEncodingConfig().TxConfig
	builder := txConfig.NewTxBuilder()
	require.NoError(t, builder.SetMsgs(msg))
	builder.SetGasLimit(gasLimit)
	tx, err := txConfig.TxEncoder()(builder.GetTx())
	require.NoError(t, err)

	before := mergeFallbackCounter(t)
	apply := func(w *app.TestWrapper) (*abci.ExecTxResult, *evmtypes.Receipt, []byte) {
		t.Helper()
		response, err := w.App.FinalizeBlock(context.Background(), &abci.RequestFinalizeBlock{
			Header: &tmproto.Header{
				Height: 1, ChainID: chainsBlockChainID, Time: timestamp,
				ProposerAddress: validator.PublicKey.Address(),
			},
			Txs: [][]byte{tx},
		})
		require.NoError(t, err)
		require.Len(t, response.TxResults, 1)
		require.NotEmpty(t, response.AppHash)
		_, err = w.App.Commit(context.Background())
		require.NoError(t, err)
		require.Equal(t, response.AppHash, w.App.LastCommitID().Hash)
		result := response.TxResults[0]
		require.Zero(t, result.Code, result.Log)
		ctx := w.App.GetCheckCtx()
		receipt, err := w.App.EvmKeeper.GetReceipt(ctx, signed.Hash())
		require.NoError(t, err)
		require.Equal(t, uint32(1), receipt.Status, receipt.VmError)
		require.Empty(t, receipt.VmError)
		require.Positive(t, receipt.GasUsed)
		require.Equal(t, contract.Hex(), receipt.ContractAddress)
		require.Len(t, receipt.Logs, calls)
		for i, log := range receipt.Logs {
			require.Equal(t, child.Hex(), log.Address)
			require.Empty(t, log.Topics)
			require.Equal(t, common.LeftPadBytes(big.NewInt(int64(i)).Bytes(), 32), log.Data)
		}
		store := ctx.KVStore(w.App.EvmKeeper.GetStoreKey())
		for _, pref := range [][]byte{evmtypes.CodeKeyPrefix, evmtypes.CodeHashKeyPrefix, evmtypes.CodeSizeKeyPrefix, evmtypes.NonceKeyPrefix} {
			require.Nil(t, store.Get(append(append([]byte{}, pref...), child[:]...)))
		}
		statePrefix := evmtypes.StateKey(child)
		require.Empty(t, store.GetAllKeyStrsInRange(statePrefix, sdk.PrefixEndBytes(statePrefix)))
		require.Empty(t, w.App.EvmKeeper.GetCode(ctx, child))
		require.Zero(t, w.App.EvmKeeper.GetNonce(ctx, child))
		require.Zero(t, w.App.EvmKeeper.GetBalance(ctx, w.App.EvmKeeper.GetPaxAddressOrDefault(ctx, child)).Sign())
		for i := 0; i < calls; i++ {
			require.Equal(t, common.Hash{}, w.App.EvmKeeper.GetState(ctx, child, common.BigToHash(big.NewInt(int64(i)))))
		}
		require.Equal(t, []byte{0}, w.App.EvmKeeper.GetCode(ctx, contract))
		require.Equal(t, uint64(1), w.App.EvmKeeper.GetNonce(ctx, payer.EvmAddress))
		return result, receipt, response.AppHash
	}
	wantResult, wantReceipt, wantHash := apply(ordinary)
	gotResult, gotReceipt, gotHash := apply(giga)
	require.Equal(t, before+1, mergeFallbackCounter(t))
	require.Equal(t, wantResult.Code, gotResult.Code)
	require.Equal(t, wantResult.GasUsed, gotResult.GasUsed)
	require.Equal(t, wantResult.Log, gotResult.Log)
	require.Equal(t, wantResult.Data, gotResult.Data)
	require.Equal(t, wantReceipt.Status, gotReceipt.Status)
	require.Equal(t, wantReceipt.GasUsed, gotReceipt.GasUsed)
	require.Equal(t, wantReceipt.Logs, gotReceipt.Logs)
	require.Equal(t, wantReceipt.ContractAddress, gotReceipt.ContractAddress)
	require.Equal(t, wantHash, gotHash)
}

func gigaPurgeCreationCode(t *testing.T, calls int) []byte {
	t.Helper()
	decode := func(code string) []byte {
		data, err := hex.DecodeString(code)
		require.NoError(t, err)
		return data
	}
	runtime := decode("361560005760003580600101905560003560005260206000a000")
	runtime[3] = byte(len(runtime))
	runtime = append(runtime, decode("5b33ff")...)
	childInit := append([]byte{0x60, byte(len(runtime)), 0x60, 12, 0x60, 0, 0x39, 0x60, byte(len(runtime)), 0x60, 0, 0xf3}, runtime...)
	code := []byte{0x61, byte(len(childInit) >> 8), byte(len(childInit)), 0x61, 0, 0, 0x60, 0x80, 0x39,
		0x61, byte(len(childInit) >> 8), byte(len(childInit)), 0x60, 0x80, 0x60, 0, 0xf0, 0x80, 0x15}
	var jumps []int
	check := func() {
		code = append(code, 0x61, 0, 0, 0x57)
		jumps = append(jumps, len(code)-3)
	}
	check()
	code = append(code, decode("604052")...)
	for i := 0; i < calls; i++ {
		code = append(code, 0x60, byte(i))
		code = append(code, decode("600052600060006020600060006040515af115")...)
		check()
	}
	code = append(code, decode("600060006000600060006040515af115")...)
	check()
	code = append(code, decode("600060005360016000f3")...)
	revert := len(code)
	code = append(code, decode("5b60006000fd")...)
	for _, jump := range jumps {
		code[jump], code[jump+1] = byte(revert>>8), byte(revert)
	}
	code[4], code[5] = byte(len(code)>>8), byte(len(code))
	return append(code, childInit...)
}
