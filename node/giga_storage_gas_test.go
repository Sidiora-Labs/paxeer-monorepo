package app_test

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"math/big"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types/ethtx"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

func TestGigaStorageGasWithoutValue(t *testing.T) {
	runGigaStorageGas(t, nil, evmkeeper.LegacySstoreSetGasEIP2200)
}

func TestGigaStorageGasStoredValue(t *testing.T) {
	value := uint64(72000)
	runGigaStorageGas(t, &value, value)
}

func TestGigaStorageGasZeroValue(t *testing.T) {
	value := uint64(0)
	runGigaStorageGas(t, &value, evmkeeper.LegacySstoreSetGasEIP2200)
}

func runGigaStorageGas(t *testing.T, stored *uint64, storageGas uint64) {
	t.Helper()
	previousOCC := app.EnableOCC
	previousSkipResultsHash := tmtypes.SkipLastResultsHashValidation.Load()
	app.EnableOCC = true
	t.Cleanup(func() {
		app.EnableOCC = previousOCC
		tmtypes.SkipLastResultsHashValidation.Store(previousSkipResultsHash)
	})

	const gasLimit uint64 = 1_000_000
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
		if stored == nil {
			key := append([]byte(a.EvmKeeper.Paramstore.Name()+"/"), evmtypes.KeyPaxSstoreSetGasEIP2200...)
			ctx.KVStore(a.GetKey(paramtypes.StoreKey)).Delete(key)
		} else {
			a.EvmKeeper.Paramstore.Set(ctx, evmtypes.KeyPaxSstoreSetGasEIP2200, *stored)
		}
		require.Equal(t, stored != nil, a.EvmKeeper.Paramstore.Has(ctx, evmtypes.KeyPaxSstoreSetGasEIP2200))
		require.Equal(t, stored != nil, a.GigaEvmKeeper.Paramstore.Has(ctx, evmtypes.KeyPaxSstoreSetGasEIP2200))
		require.Equal(t, storageGas, a.EvmKeeper.GetSstoreSetGasEIP2200(ctx))
		require.Equal(t, storageGas, a.GigaEvmKeeper.GetSstoreSetGasEIP2200(ctx))
		require.False(t, a.Sequenced(ctx, ctx.BlockHeight()))
		return &app.TestWrapper{App: a, Ctx: ctx}
	}
	ordinary, giga := newApplication(false), newApplication(true)
	require.NotNil(t, giga.App.GigaEvmKeeper.EvmoneVM)
	require.True(t, giga.App.GigaOCCEnabled)
	require.Equal(t, gigaStorageGasState(t, ordinary), gigaStorageGasState(t, giga))

	// The constructor sets slots 0, 1 and 2, emits LOG0, and returns runtime
	// code that sets slot 3 and emits LOG0 when called.
	initCode, err := hex.DecodeString("60016000556002600155600360025560006000a0600b6020600039600b6000f3600460035560006000a000")
	require.NoError(t, err)
	contract := crypto.CreateAddress(payer.EvmAddress, 0)
	encode := func(nonce uint64, to *common.Address, data []byte) ([]byte, *ethtypes.Transaction) {
		t.Helper()
		chainID := ordinary.App.EvmKeeper.ChainID(ordinary.Ctx)
		signed, err := ethtypes.SignTx(ethtypes.NewTx(&ethtypes.LegacyTx{
			Nonce: nonce, GasPrice: big.NewInt(500_000_000_000), Gas: gasLimit,
			To: to, Value: new(big.Int), Data: data,
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
		return tx, signed
	}
	creation, signedCreation := encode(0, nil, initCode)
	call, signedCall := encode(1, &contract, nil)
	intrinsicCreation, err := core.IntrinsicGas(initCode, nil, nil, true, true, true, true)
	require.NoError(t, err)
	// Execution adds three cold writes, LOG0, code copy/return and eleven
	// bytes of code deposit; the call adds one cold write and LOG0.
	wantGas := []uint64{intrinsicCreation + 3*storageGas + 8923, 21000 + storageGas + 2487}

	before := mergeFallbackCounter(t)
	apply := func(w *app.TestWrapper) ([]*abci.ExecTxResult, []*evmtypes.Receipt, []byte) {
		t.Helper()
		response, err := w.App.FinalizeBlock(context.Background(), &abci.RequestFinalizeBlock{
			Header: &tmproto.Header{
				Height: 1, ChainID: chainsBlockChainID, Time: timestamp,
				ProposerAddress: validator.PublicKey.Address(),
			},
			Txs: [][]byte{creation, call},
		})
		require.NoError(t, err)
		require.Len(t, response.TxResults, 2)
		require.NotEmpty(t, response.AppHash)
		_, err = w.App.Commit(context.Background())
		require.NoError(t, err)
		require.Equal(t, response.AppHash, w.App.LastCommitID().Hash)
		app.WaitForTestReceiptHeight(t, w.App, 1)
		ctx := w.App.GetCheckCtx()
		var receipts []*evmtypes.Receipt
		for i, signed := range []*ethtypes.Transaction{signedCreation, signedCall} {
			result := response.TxResults[i]
			require.Zero(t, result.Code, result.Log)
			receipt, err := w.App.EvmKeeper.GetReceipt(ctx, signed.Hash())
			require.NoError(t, err)
			require.Equal(t, uint32(1), receipt.Status, receipt.VmError)
			require.Empty(t, receipt.VmError)
			require.Equal(t, wantGas[i], receipt.GasUsed)
			require.Len(t, receipt.Logs, 1)
			receipts = append(receipts, receipt)
		}
		require.Equal(t, contract.Hex(), receipts[0].ContractAddress)
		for slot := int64(0); slot < 4; slot++ {
			require.Equal(t, common.BigToHash(big.NewInt(slot+1)),
				w.App.EvmKeeper.GetState(ctx, contract, common.BigToHash(big.NewInt(slot))))
		}
		require.Equal(t, uint64(2), w.App.EvmKeeper.GetNonce(ctx, payer.EvmAddress))
		return response.TxResults, receipts, response.AppHash
	}
	wantResults, wantReceipts, wantHash := apply(ordinary)
	gotResults, gotReceipts, gotHash := apply(giga)
	require.Equal(t, before, mergeFallbackCounter(t))
	for i := range wantResults {
		require.Equal(t, wantResults[i].GasUsed, gotResults[i].GasUsed)
		require.Equal(t, wantResults[i].Code, gotResults[i].Code)
		require.Equal(t, wantResults[i].Log, gotResults[i].Log)
		require.Equal(t, wantResults[i].Data, gotResults[i].Data)
		require.Equal(t, wantReceipts[i].GasUsed, gotReceipts[i].GasUsed)
		require.Equal(t, wantReceipts[i].Status, gotReceipts[i].Status)
		require.Equal(t, wantReceipts[i].Logs, gotReceipts[i].Logs)
		require.Equal(t, wantReceipts[i].ContractAddress, gotReceipts[i].ContractAddress)
	}
	require.Equal(t, wantHash, gotHash)
}

func gigaStorageGasState(t *testing.T, w *app.TestWrapper) map[string]map[string]string {
	t.Helper()
	state := make(map[string]map[string]string)
	for _, key := range w.Ctx.MultiStore().StoreKeys() {
		if _, persistent := key.(*sdk.KVStoreKey); !persistent {
			continue
		}
		values := make(map[string]string)
		iterator := w.Ctx.KVStore(key).Iterator(nil, nil)
		for ; iterator.Valid(); iterator.Next() {
			values[string(iterator.Key())] = string(iterator.Value())
		}
		require.NoError(t, iterator.Close())
		state[key.Name()] = values
	}
	return state
}

func TestGigaStorageGasSources(t *testing.T) {
	_, source, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Dir(filepath.Dir(source))
	readers := map[string]int{
		"engine/deps/xevm/keeper/params.go": 0,
		"modules/evm/keeper/params.go":      0,
	}
	for _, directory := range []string{"node", "rpc", "modules/evm/keeper", "modules/layerxagent/keeper", "engine/deps/xevm/keeper"} {
		err := filepath.WalkDir(filepath.Join(root, directory), func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			positions := token.NewFileSet()
			file, err := parser.ParseFile(positions, path, nil, parser.ParseComments)
			if err != nil {
				return err
			}
			if ast.IsGenerated(file) {
				return nil
			}
			relative, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			relative = filepath.ToSlash(relative)
			var reader *ast.FuncDecl
			if _, allowed := readers[relative]; allowed {
				for _, decl := range file.Decls {
					if function, ok := decl.(*ast.FuncDecl); ok && function.Name.Name == "GetSstoreSetGasEIP2200" && function.Recv != nil {
						reader = function
					}
				}
				require.NotNil(t, reader, relative)
			}
			ast.Inspect(file, func(node ast.Node) bool {
				selector, ok := node.(*ast.SelectorExpr)
				if !ok || selector.Sel.Name != "PaxSstoreSetGasEip2200" {
					return true
				}
				if reader == nil || selector.Pos() < reader.Body.Pos() || selector.End() > reader.Body.End() {
					t.Errorf("%s: storage gas must be read through the keeper reader", positions.Position(selector.Pos()))
				} else {
					readers[relative]++
				}
				return true
			})
			return nil
		})
		require.NoError(t, err)
	}
	for path, reads := range readers {
		require.Equal(t, 1, reads, path)
	}
}
