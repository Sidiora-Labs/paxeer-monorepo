package evmrpc_test

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"math/big"
	"strings"
	"sync"
	"testing"
	"time"

	wasmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm/types"

	tmbytes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/bytes"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types/ethtx"
	seqtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	"github.com/Sidiora-Labs/Paxeer-X-Network/rpc/rpcutils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/export"
	gethrpc "github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
)

type sequenceBatchBlockClient struct {
	*MockClient
	block *coretypes.ResultBlock
}

func (c *sequenceBatchBlockClient) Block(ctx context.Context, height *int64) (*coretypes.ResultBlock, error) {
	if height != nil && *height == c.block.Block.Height {
		return c.block, nil
	}
	return c.MockClient.Block(ctx, height)
}

func (c *sequenceBatchBlockClient) BlockByHash(ctx context.Context, hash tmbytes.HexBytes) (*coretypes.ResultBlock, error) {
	if hash.String() == c.block.BlockID.Hash.String() {
		return c.block, nil
	}
	return c.MockClient.BlockByHash(ctx, hash)
}

func TestBlockViewsSkipSequenceBatch(t *testing.T) {
	application := app.Setup(t, false, true, false)
	k := &application.EvmKeeper
	header := mockBlockHeader(MockHeight8)
	sdkCtx := application.GetContextForDeliverTx(nil).WithBlockHeight(MockHeight8).
		WithBlockTime(header.Time).WithClosestUpgradeName(LatestCtxUpgradeName)
	ctxProvider := func(height int64) sdk.Context {
		if height == evmrpc.LatestCtxHeight {
			return sdkCtx
		}
		return sdkCtx.WithBlockHeight(height)
	}
	txBytes, err := Encoder(Tx1)
	require.NoError(t, err)
	transaction, _ := Tx1.GetMsgs()[0].(*types.MsgEVMTransaction).AsTransaction()
	require.NotNil(t, transaction)
	sender, err := rpcutils.RecoverEVMSender(transaction, MockHeight8, header.Time.Unix())
	require.NoError(t, err)
	testkeeper.MustMockReceipt(t, k, sdkCtx, transaction.Hash(), &types.Receipt{
		BlockNumber: MockHeight8, TransactionIndex: 0, TxHashHex: transaction.Hash().Hex(),
		From: sender.Hex(), Status: 1, GasUsed: 21000,
	})
	require.Equal(t, transaction.Hash().Hex(), testkeeper.WaitForReceipt(t, k, sdkCtx, transaction.Hash()).TxHashHex)

	privateKey := ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize))
	publicKey := privateKey.Public().(ed25519.PublicKey)
	keyID := sha256.Sum256(publicKey)
	txRoot := sha256.Sum256(txBytes)
	batch := seqtypes.MsgSequenceBatch{
		Height: MockHeight8, FromSeq: 1, ToSeq: 1,
		PrevSeqHash: make([]byte, seqtypes.HashLength), LastSeqHash: txRoot[:], TxRoot: txRoot[:], KeyId: keyID[:],
	}
	digest, err := batch.SignBytes(header.ChainID)
	require.NoError(t, err)
	batch.Sig = ed25519.Sign(privateKey, digest[:])
	require.NoError(t, batch.VerifySignature(header.ChainID, [32]byte(publicKey)))
	batchBytes, err := seqtypes.EncodeSequenceBatchTx(batch)
	require.NoError(t, err)
	require.Len(t, batchBytes, 241)
	wrongDomain := append([]byte(nil), batchBytes...)
	wrongDomain[0] = 'X'
	_, err = seqtypes.DecodeSequenceBatchTx(wrongDomain)
	require.Error(t, err)
	_, err = TxConfig.TxDecoder()(wrongDomain)
	require.Error(t, err)

	for _, tc := range []struct {
		name        string
		txs         []tmtypes.Tx
		decodeError string
	}{
		{name: "sequence batch first", txs: []tmtypes.Tx{batchBytes, txBytes}},
		{name: "different domain first", txs: []tmtypes.Tx{wrongDomain, txBytes}, decodeError: "decode consensus transaction 0"},
		{name: "sequence batch after transaction", txs: []tmtypes.Tx{txBytes, batchBytes}, decodeError: "decode consensus transaction 1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tmClient := &sequenceBatchBlockClient{
				MockClient: &MockClient{},
				block: &coretypes.ResultBlock{
					BlockID: MockBlockID,
					Block: &tmtypes.Block{
						Header: header, Data: tmtypes.Data{Txs: tc.txs},
						LastCommit: &tmtypes.Commit{Height: MockHeight8 - 1},
					},
				},
			}
			watermarks := evmrpc.NewWatermarkManager(tmClient, nil, nil, nil)
			api := evmrpc.NewBlockAPI(tmClient, k, ctxProvider, func(int64) client.TxConfig { return TxConfig },
				evmrpc.ConnectionTypeHTTP, watermarks, evmrpc.NewBlockCache(3000), &sync.Mutex{})
			server := gethrpc.NewServer()
			t.Cleanup(server.Stop)
			require.NoError(t, server.RegisterName("eth", api))
			rpcClient := gethrpc.DialInProc(server)
			t.Cleanup(rpcClient.Close)
			blockHash := common.BytesToHash(MockBlockID.Hash).Hex()
			for _, fullTx := range []bool{false, true} {
				for _, call := range []struct {
					method string
					block  string
				}{
					{method: "eth_getBlockByNumber", block: "0x8"},
					{method: "eth_getBlockByHash", block: blockHash},
				} {
					var result map[string]interface{}
					err := rpcClient.CallContext(context.Background(), &result, call.method, call.block, fullTx)
					if tc.decodeError != "" {
						require.ErrorContains(t, err, tc.decodeError, call.method)
						continue
					}
					require.NoError(t, err, call.method)
					require.Equal(t, "0x8", result["number"])
					require.Equal(t, blockHash, result["hash"])
					transactions := result["transactions"].([]interface{})
					require.Len(t, transactions, 1, call.method)
					if fullTx {
						served := transactions[0].(map[string]interface{})
						require.Equal(t, transaction.Hash().Hex(), served["hash"])
						require.Equal(t, "0x0", served["transactionIndex"])
					} else {
						require.Equal(t, transaction.Hash().Hex(), transactions[0])
					}
				}
			}
			var count hexutil.Uint
			err := rpcClient.CallContext(context.Background(), &count, "eth_getBlockTransactionCountByNumber", "0x8")
			if tc.decodeError != "" {
				require.ErrorContains(t, err, tc.decodeError)
				return
			}
			require.NoError(t, err)
			require.Equal(t, hexutil.Uint(1), count)
		})
	}
}

func TestEncodeTmBlock_EmptyTransactions(t *testing.T) {
	k := &testkeeper.EVMTestApp.EvmKeeper
	ctx := testkeeper.EVMTestApp.GetContextForDeliverTx([]byte{}).WithBlockTime(time.Now())
	block := &coretypes.ResultBlock{
		BlockID: MockBlockID,
		Block: &tmtypes.Block{
			Header: mockBlockHeader(MockHeight8),
			Data:   tmtypes.Data{},
			LastCommit: &tmtypes.Commit{
				Height: MockHeight8 - 1,
			},
		},
	}

	// Call EncodeTmBlock with empty transactions
	result, err := evmrpc.EncodeTmBlock(func(i int64) sdk.Context { return ctx }, func(i int64) client.TxConfig { return TxConfig }, block, k, true, false, false, false, evmrpc.NewBlockCache(3000), &sync.Mutex{})
	require.Nil(t, err)

	// Assert txHash is equal to ethtypes.EmptyTxsHash
	require.Equal(t, ethtypes.EmptyTxsHash, result["transactionsRoot"])
}

func TestEncodeBankMsg(t *testing.T) {
	k := &testkeeper.EVMTestApp.EvmKeeper
	ctx := testkeeper.EVMTestApp.GetContextForDeliverTx([]byte{}).WithBlockTime(time.Now())
	fromPaxAddr, _ := testkeeper.MockAddressPair()
	toPaxAddr, _ := testkeeper.MockAddressPair()
	b := TxConfig.NewTxBuilder()
	b.SetMsgs(banktypes.NewMsgSend(fromPaxAddr, toPaxAddr, sdk.NewCoins(sdk.NewCoin("uhpx", sdk.NewInt(10)))))
	tx := b.GetTx()
	resBlock := coretypes.ResultBlock{
		BlockID: MockBlockID,
		Block: &tmtypes.Block{
			Header: mockBlockHeader(MockHeight8),
			Data: tmtypes.Data{
				Txs: []tmtypes.Tx{func() []byte {
					bz, _ := Encoder(tx)
					return bz
				}()},
			},
			LastCommit: &tmtypes.Commit{
				Height: MockHeight8 - 1,
			},
		},
	}
	res, err := evmrpc.EncodeTmBlock(func(i int64) sdk.Context { return ctx }, func(i int64) client.TxConfig { return TxConfig }, &resBlock, k, true, false, false, false, evmrpc.NewBlockCache(3000), &sync.Mutex{})
	require.Nil(t, err)
	txs := res["transactions"].([]interface{})
	require.Equal(t, 0, len(txs))
}

func TestEncodeWasmExecuteMsg(t *testing.T) {
	k := &testkeeper.EVMTestApp.EvmKeeper
	ctx := testkeeper.EVMTestApp.GetContextForDeliverTx(nil).WithBlockHeight(MockHeight8)
	fromPaxAddr, fromEvmAddr := testkeeper.MockAddressPair()
	toPaxAddr, _ := testkeeper.MockAddressPair()
	b := TxConfig.NewTxBuilder()
	b.SetMsgs(&wasmtypes.MsgExecuteContract{
		Sender:   fromPaxAddr.String(),
		Contract: toPaxAddr.String(),
		Msg:      []byte{1, 2, 3},
	})
	tx := b.GetTx()
	bz, _ := Encoder(tx)
	txHash := sha256.Sum256(bz)
	hash := common.BytesToHash(txHash[:])
	testkeeper.MustMockReceipt(t, k, ctx, hash, &types.Receipt{
		TransactionIndex: 1,
		BlockNumber:      MockHeight8,
		From:             fromEvmAddr.Hex(),
		TxHashHex:        hash.Hex(),
	})
	receipt := testkeeper.WaitForReceipt(t, k, ctx, hash)
	require.Equal(t, hash.Hex(), receipt.TxHashHex)
	resBlock := coretypes.ResultBlock{
		BlockID: MockBlockID,
		Block: &tmtypes.Block{
			Header: mockBlockHeader(MockHeight8),
			Data: tmtypes.Data{
				Txs: []tmtypes.Tx{bz},
			},
			LastCommit: &tmtypes.Commit{
				Height: MockHeight8 - 1,
			},
		},
	}
	res, err := evmrpc.EncodeTmBlock(func(i int64) sdk.Context { return ctx }, func(i int64) client.TxConfig { return TxConfig }, &resBlock, k, true, false, true, false, evmrpc.NewBlockCache(3000), &sync.Mutex{})
	require.Nil(t, err)
	txs := res["transactions"].([]interface{})
	require.Equal(t, 1, len(txs))
	ti := uint64(0)
	bh := common.HexToHash(MockBlockID.Hash.String())
	to := common.Address(toPaxAddr)
	require.Equal(t, &export.RPCTransaction{
		BlockHash:        &bh,
		BlockNumber:      (*hexutil.Big)(big.NewInt(MockHeight8)),
		From:             fromEvmAddr,
		To:               &to,
		Input:            []byte{1, 2, 3},
		Hash:             common.Hash(sha256.Sum256(bz)),
		TransactionIndex: (*hexutil.Uint64)(&ti),
		V:                nil,
		R:                nil,
		S:                nil,
	}, txs[0].(*export.RPCTransaction))
}

func TestEncodeWasmExecuteMsgRequiresCanonicalReceipt(t *testing.T) {
	k := &testkeeper.EVMTestApp.EvmKeeper
	ctx := testkeeper.EVMTestApp.GetContextForDeliverTx(nil).WithBlockHeight(MockHeight8)
	fromPaxAddr, _ := testkeeper.MockAddressPair()
	toPaxAddr, _ := testkeeper.MockAddressPair()
	builder := TxConfig.NewTxBuilder()
	require.NoError(t, builder.SetMsgs(&wasmtypes.MsgExecuteContract{
		Sender:   fromPaxAddr.String(),
		Contract: toPaxAddr.String(),
		Msg:      []byte{9, 8, 7, 6},
	}))
	txBytes, err := Encoder(builder.GetTx())
	require.NoError(t, err)
	block := &coretypes.ResultBlock{
		BlockID: MockBlockID,
		Block: &tmtypes.Block{
			Header: mockBlockHeader(MockHeight8),
			Data:   tmtypes.Data{Txs: []tmtypes.Tx{txBytes}},
			LastCommit: &tmtypes.Commit{
				Height: MockHeight8 - 1,
			},
		},
	}
	result, err := evmrpc.EncodeTmBlock(
		func(int64) sdk.Context { return ctx },
		func(int64) client.TxConfig { return TxConfig },
		block, k, true, false, true, false, evmrpc.NewBlockCache(3000), &sync.Mutex{},
	)
	require.Nil(t, result)
	require.ErrorContains(t, err, "has no canonical receipt")
}

func TestEncodeBankTransferMsg(t *testing.T) {
	k := &testkeeper.EVMTestApp.EvmKeeper
	ctx := testkeeper.EVMTestApp.GetContextForDeliverTx(nil)
	fromPaxAddr, fromEvmAddr := testkeeper.MockAddressPair()
	k.SetAddressMapping(ctx, fromPaxAddr, fromEvmAddr)
	toPaxAddr, _ := testkeeper.MockAddressPair()
	b := TxConfig.NewTxBuilder()
	b.SetMsgs(&banktypes.MsgSend{
		FromAddress: fromPaxAddr.String(),
		ToAddress:   toPaxAddr.String(),
		Amount:      sdk.NewCoins(sdk.NewCoin("uhpx", sdk.OneInt())),
	})
	ti := uint64(0)
	tx := b.GetTx()
	bz, _ := Encoder(tx)
	resBlock := coretypes.ResultBlock{
		BlockID: MockBlockID,
		Block: &tmtypes.Block{
			Header: mockBlockHeader(MockHeight8),
			Data: tmtypes.Data{
				Txs: []tmtypes.Tx{bz},
			},
			LastCommit: &tmtypes.Commit{
				Height: MockHeight8 - 1,
			},
		},
	}
	res, err := evmrpc.EncodeTmBlock(func(i int64) sdk.Context { return ctx }, func(i int64) client.TxConfig { return TxConfig }, &resBlock, k, true, true, false, false, evmrpc.NewBlockCache(3000), &sync.Mutex{})
	require.Nil(t, err)
	txs := res["transactions"].([]interface{})
	require.Equal(t, 1, len(txs))
	bh := common.HexToHash(MockBlockID.Hash.String())
	to := common.Address(toPaxAddr)
	require.Equal(t, &export.RPCTransaction{
		BlockHash:        &bh,
		BlockNumber:      (*hexutil.Big)(big.NewInt(MockHeight8)),
		From:             fromEvmAddr,
		To:               &to,
		Value:            (*hexutil.Big)(big.NewInt(1_000_000_000_000)),
		Hash:             common.Hash(sha256.Sum256(bz)),
		TransactionIndex: (*hexutil.Uint64)(&ti),
		V:                nil,
		R:                nil,
		S:                nil,
	}, txs[0].(*export.RPCTransaction))
}

// Wasm-execute txs contribute receipt.GasUsed to the block's gasUsed total.
func TestEncodeWasmExecuteMsg_GasUsedFromReceipt(t *testing.T) {
	k := &testkeeper.EVMTestApp.EvmKeeper
	ctx := testkeeper.EVMTestApp.GetContextForDeliverTx(nil).WithBlockHeight(MockHeight8)
	fromPaxAddr, fromEvmAddr := testkeeper.MockAddressPair()
	toPaxAddr, _ := testkeeper.MockAddressPair()
	b := TxConfig.NewTxBuilder()
	b.SetMsgs(&wasmtypes.MsgExecuteContract{
		Sender:   fromPaxAddr.String(),
		Contract: toPaxAddr.String(),
		Msg:      []byte{1, 2, 3},
	})
	tx := b.GetTx()
	bz, _ := Encoder(tx)
	txHash := sha256.Sum256(bz)
	hash := common.BytesToHash(txHash[:])
	testkeeper.MustMockReceipt(t, k, ctx, hash, &types.Receipt{
		TransactionIndex: 1,
		BlockNumber:      MockHeight8,
		From:             fromEvmAddr.Hex(),
		TxHashHex:        hash.Hex(),
		GasUsed:          54321,
	})
	resBlock := coretypes.ResultBlock{
		BlockID: MockBlockID,
		Block: &tmtypes.Block{
			Header: mockBlockHeader(MockHeight8),
			Data: tmtypes.Data{
				Txs: []tmtypes.Tx{bz},
			},
			LastCommit: &tmtypes.Commit{
				Height: MockHeight8 - 1,
			},
		},
	}
	res, err := evmrpc.EncodeTmBlock(func(i int64) sdk.Context { return ctx }, func(i int64) client.TxConfig { return TxConfig }, &resBlock, k, true, false, true, false, evmrpc.NewBlockCache(3000), &sync.Mutex{})
	require.Nil(t, err)
	require.Equal(t, hexutil.Uint64(54321), res["gasUsed"])
	txs := res["transactions"].([]interface{})
	require.Equal(t, 1, len(txs))
}

// Bank-send txs without a matching EVM receipt contribute 0 to the block's
// gasUsed total. (This is the Autobahn case for plain MsgSend txs; under
// legacy, bank sends that emit EVM-relevant events would have a synthetic
// receipt and contribute receipt.GasUsed.)
func TestEncodeBankTransferMsg_NoReceiptGasUsedZero(t *testing.T) {
	k := &testkeeper.EVMTestApp.EvmKeeper
	ctx := testkeeper.EVMTestApp.GetContextForDeliverTx(nil)
	fromPaxAddr, fromEvmAddr := testkeeper.MockAddressPair()
	k.SetAddressMapping(ctx, fromPaxAddr, fromEvmAddr)
	toPaxAddr, _ := testkeeper.MockAddressPair()
	b := TxConfig.NewTxBuilder()
	b.SetMsgs(&banktypes.MsgSend{
		FromAddress: fromPaxAddr.String(),
		ToAddress:   toPaxAddr.String(),
		Amount:      sdk.NewCoins(sdk.NewCoin("uhpx", sdk.OneInt())),
	})
	tx := b.GetTx()
	bz, _ := Encoder(tx)
	resBlock := coretypes.ResultBlock{
		BlockID: MockBlockID,
		Block: &tmtypes.Block{
			Header: mockBlockHeader(MockHeight8),
			Data: tmtypes.Data{
				Txs: []tmtypes.Tx{bz},
			},
			LastCommit: &tmtypes.Commit{
				Height: MockHeight8 - 1,
			},
		},
	}
	res, err := evmrpc.EncodeTmBlock(func(i int64) sdk.Context { return ctx }, func(i int64) client.TxConfig { return TxConfig }, &resBlock, k, true, true, false, false, evmrpc.NewBlockCache(3000), &sync.Mutex{})
	require.Nil(t, err)
	require.Equal(t, hexutil.Uint64(0), res["gasUsed"])
	txs := res["transactions"].([]interface{})
	require.Equal(t, 1, len(txs))
}

// TestEncodeTmBlock_ExcludeUntraceable pins the block-side counterpart of the
// tx-side discriminator: a correct-nonce ante failure (e.g. insufficient
// funds) lands a stub receipt in the receipt store. filterTransactions's
// isReceiptFromAnteError post-v5.8.0 only matches nonce-error VmError
// content, so the stub flows through to EncodeTmBlock — where
// excludeUntraceable=true catches it via EffectiveGasPrice==0 && GasUsed==0.
//
// We use the global EVMKeeper (where setup_test.go's fixtures live) and a
// post-v5.8.0 ctx so filterTransactions takes the production branch.
// DebugTracePanicTx isn't suitable here (Nonce=100, gets filtered by
// filterTransactions's nonce-bumping check before reaching EncodeTmBlock);
// we register a correct-nonce ante-stub fixture instead.
func TestEncodeTmBlock_ExcludeUntraceable(t *testing.T) {
	k := EVMKeeper

	// Build a correct-nonce EVM tx whose receipt mimics an ante-deferred
	// stub: EffectiveGasPrice=0, GasUsed=0, VmError set. We need a fresh
	// sender so startOfBlockNonce equals the tx's nonce — otherwise
	// filterTransactions's nonce-bumping branch filters it out at
	// utils.go:253 before EncodeTmBlock sees it.
	chainID := big.NewInt(config.DefaultChainID)
	privKey, _ := crypto.GenerateKey()
	signer := ethtypes.MakeSigner(types.DefaultChainConfig().EthereumConfig(chainID), big.NewInt(Ctx.BlockHeight()), uint64(Ctx.BlockTime().Unix()))
	to := common.HexToAddress("0x1234567890123456789012345678901234567890")
	rawTx, err := ethtypes.SignTx(ethtypes.NewTx(&ethtypes.DynamicFeeTx{
		Nonce: 0, GasFeeCap: big.NewInt(10), Gas: 22000, To: &to, Value: big.NewInt(0), ChainID: chainID,
	}), signer, privKey)
	require.NoError(t, err)
	td, err := ethtx.NewDynamicFeeTx(rawTx)
	require.NoError(t, err)
	msg, err := types.NewMsgEVMTransaction(td)
	require.NoError(t, err)
	builder := TxConfig.NewTxBuilder()
	require.NoError(t, builder.SetMsgs(msg))
	anteStubTx := builder.GetTx()
	anteStubBz, _ := Encoder(anteStubTx)
	anteStubHash := rawTx.Hash()

	// Mock the stub receipt on EVMKeeper so getOrSetCachedReceipt finds it.
	stubHeight := int64(MockHeight103)
	stubCtx := Ctx.WithBlockHeight(stubHeight)
	require.NoError(t, k.MockReceipt(stubCtx, anteStubHash, &types.Receipt{
		BlockNumber:      uint64(stubHeight),
		TransactionIndex: 0,
		TxHashHex:        anteStubHash.Hex(),
		VmError:          "insufficient funds",
	}))

	nonPanicBz, _ := Encoder(DebugTraceNonPanicTx)

	block := &coretypes.ResultBlock{
		BlockID: MockBlockID,
		Block: &tmtypes.Block{
			Header: mockBlockHeader(stubHeight),
			Data: tmtypes.Data{
				Txs: []tmtypes.Tx{anteStubBz, nonPanicBz},
			},
			LastCommit: &tmtypes.Commit{Height: stubHeight - 1},
		},
	}

	// Post-v5.8.0 ctx so filterTransactions's isReceiptFromAnteError stays
	// narrow (nonce-only VmError) — the production path where the stub
	// flows through and EncodeTmBlock has to apply its own check.
	ctx := Ctx.WithBlockHeight(stubHeight).WithClosestUpgradeName(LatestCtxUpgradeName)
	ctxProvider := func(int64) sdk.Context { return ctx }
	txConfigProvider := func(int64) client.TxConfig { return TxConfig }

	// excludeUntraceable=true: ante stub dropped, revert kept.
	res, err := evmrpc.EncodeTmBlock(ctxProvider, txConfigProvider, block, k,
		false /*fullTx*/, false /*includeBankTransfers*/, false /*includeSyntheticTxs*/, true, /*excludeUntraceable*/
		evmrpc.NewBlockCache(3000), &sync.Mutex{})
	require.NoError(t, err)
	txs := res["transactions"].([]interface{})
	require.Len(t, txs, 1, "expected only the revert to survive, got %v", txs)
	require.Equal(t, strings.ToLower(TestNonPanicTxHash), strings.ToLower(txs[0].(string)))

	// excludeUntraceable=false: ante stub flows through (matches regular
	// eth_getBlockBy* behavior per PR #2343's TestAnteFailureOthers).
	res, err = evmrpc.EncodeTmBlock(ctxProvider, txConfigProvider, block, k,
		false /*fullTx*/, false /*includeBankTransfers*/, false /*includeSyntheticTxs*/, false, /*excludeUntraceable*/
		evmrpc.NewBlockCache(3000), &sync.Mutex{})
	require.NoError(t, err)
	txs = res["transactions"].([]interface{})
	require.Len(t, txs, 2, "expected revert + ante stub to flow through, got %v", txs)
}

func TestMissingEVMReceiptRequiresUnconsumedHistoricalNonce(t *testing.T) {
	application := app.Setup(t, false, true, false)
	k := &application.EvmKeeper
	base := application.GetContextForDeliverTx(nil).WithBlockHeight(MockHeight8)
	before, _ := base.WithBlockHeight(MockHeight8 - 1).CacheContext()
	after, _ := base.CacheContext()
	encoded, err := Encoder(Tx1)
	require.NoError(t, err)
	transaction, _ := Tx1.GetMsgs()[0].(*types.MsgEVMTransaction).AsTransaction()
	require.NotNil(t, transaction)
	header := mockBlockHeader(MockHeight8)
	sender, err := rpcutils.RecoverEVMSender(transaction, MockHeight8, header.Time.Unix())
	require.NoError(t, err)
	k.SetNonce(before, sender, transaction.Nonce())
	k.SetNonce(after, sender, transaction.Nonce()+1)
	block := &coretypes.ResultBlock{
		BlockID: MockBlockID,
		Block: &tmtypes.Block{Header: header, Data: tmtypes.Data{Txs: []tmtypes.Tx{encoded}},
			LastCommit: &tmtypes.Commit{Height: MockHeight8 - 1}},
	}
	contexts := func(height int64) sdk.Context {
		if height == MockHeight8-1 {
			return before
		}
		return after
	}
	encode := func() (map[string]interface{}, error) {
		return evmrpc.EncodeTmBlock(contexts, func(int64) client.TxConfig { return TxConfig },
			block, k, false, false, false, false, evmrpc.NewBlockCache(3000), &sync.Mutex{})
	}
	result, err := encode()
	require.Nil(t, result)
	require.ErrorContains(t, err, "has no canonical receipt")
	k.SetNonce(after, sender, transaction.Nonce())
	result, err = encode()
	require.NoError(t, err)
	require.Empty(t, result["transactions"])
	k.SetNonce(before, sender, transaction.Nonce()+1)
	result, err = encode()
	require.Nil(t, result)
	require.ErrorContains(t, err, "nonce regressed")
}
