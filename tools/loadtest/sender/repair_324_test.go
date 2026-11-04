package sender

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	gethrpc "github.com/ethereum/go-ethereum/rpc"
	"github.com/paxeer-network/pax-load/stats"
	"github.com/paxeer-network/pax-load/types"
	"github.com/stretchr/testify/require"
)

func repair324Tx(t *testing.T, nonce uint64) *types.LoadTx {
	t.Helper()
	key, err := crypto.HexToECDSA(strings.Repeat("01", 32))
	require.NoError(t, err)
	tx, err := ethtypes.SignTx(ethtypes.NewTransaction(nonce, common.HexToAddress("0x1234"), big.NewInt(1), 21000, big.NewInt(1000000000), nil), ethtypes.NewEIP155Signer(big.NewInt(125)), key)
	require.NoError(t, err)
	return types.CreateTxFromEthTx(tx, &types.TxScenario{Name: "repair-324"})
}

func repair324Batch(t *testing.T, txs ...*types.LoadTx) []byte {
	t.Helper()
	batch := TxWriteData{TxPayloads: make([][]byte, 0, len(txs))}
	for _, tx := range txs {
		batch.TxPayloads = append(batch.TxPayloads, tx.Payload)
	}
	data, err := json.Marshal(batch)
	require.NoError(t, err)
	return data
}

func repair324ReadBatch(t *testing.T, path string, txs ...*types.LoadTx) {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, repair324Batch(t, txs...), data)
	var batch TxWriteData
	require.NoError(t, json.Unmarshal(data, &batch))
	require.Len(t, batch.TxPayloads, len(txs))
	for i, payload := range batch.TxPayloads {
		var decoded ethtypes.Transaction
		require.NoError(t, decoded.UnmarshalBinary(payload))
		require.Equal(t, txs[i].EthTx.Hash(), decoded.Hash())
	}
}

func TestRepair_324_WriterErrorDataLoss(t *testing.T) {
	for _, failure := range []string{"directory", "publication"} {
		t.Run(failure, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "batches")
			writer := NewTxsWriter(21000, dir, 41, 10)
			first, second := repair324Tx(t, 0), repair324Tx(t, 1)
			require.NoError(t, writer.Send(t.Context(), first))
			var restore func()
			if failure == "directory" {
				require.NoError(t, os.Rename(dir, dir+"-saved"))
				require.NoError(t, os.WriteFile(dir, []byte("filesystem obstruction"), 0600))
				restore = func() {
					require.NoError(t, os.Remove(dir))
					require.NoError(t, os.Rename(dir+"-saved", dir))
				}
			} else {
				require.NoError(t, os.Mkdir(writer.batchPath(41), 0755))
				restore = func() { require.NoError(t, os.Remove(writer.batchPath(41))) }
			}
			err := writer.Send(t.Context(), second)
			require.Error(t, err)
			var pathError *os.PathError
			require.ErrorAs(t, err, &pathError, "the original filesystem error must survive")
			require.Equal(t, uint64(41), writer.nextHeight)
			require.Zero(t, writer.blocksGenerated)
			require.Equal(t, uint64(21000), writer.bufferGas)
			require.Equal(t, []*types.LoadTx{first}, writer.txBuffer)
			restore()
			require.NoError(t, writer.Send(t.Context(), second))
			repair324ReadBatch(t, writer.batchPath(41), first)
			require.Equal(t, uint64(42), writer.nextHeight)
			require.Equal(t, uint64(1), writer.blocksGenerated)
			require.Equal(t, []*types.LoadTx{second}, writer.txBuffer)
			require.NoError(t, writer.Flush())
			writer = nil
			reopened := NewTxsWriter(21000, dir, 41, 10)
			require.NoError(t, reopened.resume())
			require.Equal(t, uint64(43), reopened.nextHeight)
			require.Equal(t, uint64(2), reopened.blocksGenerated)
			repair324ReadBatch(t, reopened.batchPath(41), first)
			repair324ReadBatch(t, reopened.batchPath(42), second)
		})
	}
}

func TestRepair_324_WriterRestartRecovery(t *testing.T) {
	for _, linked := range []bool{false, true} {
		t.Run(fmt.Sprintf("published_%t", linked), func(t *testing.T) {
			dir := t.TempDir()
			writer := NewTxsWriter(42000, dir, 70, 10)
			first, second := repair324Tx(t, 0), repair324Tx(t, 1)
			require.NoError(t, writer.Send(t.Context(), first))
			require.NoError(t, writer.stageBatch(repair324Batch(t, first)))
			require.NoFileExists(t, writer.batchPath(70))
			if linked {
				// Crash after exclusive publication, before pending-file cleanup.
				require.NoError(t, os.Link(writer.pendingPath(70), writer.batchPath(70)))
				require.NoError(t, syncTxsDir(dir))
			}
			// A torn temporary write has never been published or acknowledged.
			require.NoError(t, os.WriteFile(filepath.Join(dir, ".sender-tmp-interrupted"), []byte(`{"tx_payloads":[`), 0600))
			writer = nil
			reopened := NewTxsWriter(42000, dir, 70, 10)
			require.NoError(t, reopened.Send(t.Context(), second))
			require.Equal(t, uint64(71), reopened.nextHeight)
			require.Equal(t, uint64(1), reopened.blocksGenerated)
			repair324ReadBatch(t, reopened.batchPath(70), first)
			require.NoFileExists(t, reopened.pendingPath(70))
			require.NoError(t, reopened.Flush())
			repair324ReadBatch(t, reopened.batchPath(71), second)
			reopened = nil
			again := NewTxsWriter(42000, dir, 70, 10)
			require.NoError(t, again.resume())
			require.Equal(t, uint64(72), again.nextHeight)
			require.Equal(t, uint64(2), again.blocksGenerated)
		})
	}
}

func TestRepair_324_WriterDuplicateHeight(t *testing.T) {
	dir := t.TempDir()
	first, other := repair324Tx(t, 0), repair324Tx(t, 1)
	original := NewTxsWriter(42000, dir, 9, 10)
	stale := NewTxsWriter(42000, dir, 9, 10)
	require.NoError(t, original.Send(t.Context(), first))
	require.NoError(t, stale.Send(t.Context(), other))
	require.NoError(t, original.Flush())
	require.ErrorIs(t, stale.Flush(), os.ErrExist)
	require.Equal(t, uint64(9), stale.nextHeight)
	require.Equal(t, uint64(21000), stale.bufferGas)
	require.Equal(t, []*types.LoadTx{other}, stale.txBuffer)
	repair324ReadBatch(t, original.batchPath(9), first)
	// Refusal must not leave a conflicting recovery record behind.
	require.NoFileExists(t, stale.pendingPath(9))
	reopened := NewTxsWriter(42000, dir, 9, 10)
	require.NoError(t, reopened.resume())
	repair324ReadBatch(t, original.batchPath(9), first)
	wrongStart := NewTxsWriter(42000, dir, 10, 10)
	require.ErrorIs(t, wrongStart.Send(t.Context(), other), os.ErrExist)
	require.NoError(t, reopened.resume())
	require.Equal(t, uint64(10), reopened.nextHeight)
	require.NoError(t, reopened.Send(t.Context(), other))
	require.NoError(t, reopened.Flush())
	repair324ReadBatch(t, reopened.batchPath(10), other)
	// Legacy output cannot be silently adopted by a new workload.
	legacyDir := t.TempDir()
	legacyPath := filepath.Join(legacyDir, "9_txs.json")
	require.NoError(t, os.WriteFile(legacyPath, repair324Batch(t, first), 0644))
	require.ErrorIs(t, NewTxsWriter(42000, legacyDir, 9, 10).Send(t.Context(), other), os.ErrExist)
	repair324ReadBatch(t, legacyPath, first)
}

func TestRepair_324_WriterRejectsCorruptRecovery(t *testing.T) {
	for _, corruption := range []string{"manifest", "batch", "gap", "pending_conflict"} {
		t.Run(corruption, func(t *testing.T) {
			dir := t.TempDir()
			writer := NewTxsWriter(42000, dir, 2, 10)
			tx := repair324Tx(t, 0)
			require.NoError(t, writer.Send(t.Context(), tx))
			require.NoError(t, writer.Flush())
			switch corruption {
			case "manifest":
				require.NoError(t, os.WriteFile(filepath.Join(dir, txWriterManifestName), []byte("{"), 0644))
			case "batch":
				require.NoError(t, os.WriteFile(writer.batchPath(2), []byte("{"), 0644))
			case "gap":
				require.NoError(t, os.Rename(writer.batchPath(2), writer.batchPath(3)))
			case "pending_conflict":
				require.NoError(t, writer.stageBatch(repair324Batch(t, tx)))
				require.NoError(t, os.Rename(writer.pendingPath(3), writer.pendingPath(2)))
				require.NoError(t, os.WriteFile(writer.pendingPath(2), repair324Batch(t, repair324Tx(t, 1)), 0644))
			}
			writer = nil
			reopened := NewTxsWriter(42000, dir, 2, 10)
			require.Error(t, reopened.Send(t.Context(), tx))
			require.Empty(t, reopened.txBuffer)
		})
	}
}

// This committed peer fixture exposes the SendAPI hash/error contract from
// rpc/send_test.go over the production JSON-RPC server codec. It has no node or
// mempool implementation; only the exact signed input in the fixture is accepted.
type Repair324RPCTranscript struct {
	Input  hexutil.Bytes
	Hash   common.Hash
	Reject bool
}

func (f *Repair324RPCTranscript) SendRawTransaction(input hexutil.Bytes) (common.Hash, error) {
	if !bytes.Equal(input, f.Input) {
		return common.Hash{}, fmt.Errorf("request does not match the committed transaction fixture")
	}
	if f.Reject {
		return common.Hash{}, &TransactionRPCError{Code: -32000, Message: ": invalid sequence", Data: json.RawMessage(`{"reason":"nonce"}`)}
	}
	return f.Hash, nil
}

func TestRepair_324_RPCProducerTransportConsumer(t *testing.T) {
	for _, reject := range []bool{false, true} {
		t.Run(fmt.Sprintf("rejected_%t", reject), func(t *testing.T) {
			tx := repair324Tx(t, 0)
			rpcServer := gethrpc.NewServer()
			t.Cleanup(rpcServer.Stop)
			require.NoError(t, rpcServer.RegisterName("eth", &Repair324RPCTranscript{Input: tx.Payload, Hash: tx.EthTx.Hash(), Reject: reject}))
			server := httptest.NewServer(rpcServer)
			t.Cleanup(server.Close)
			worker := NewWorker(1, "125", server.URL, 4, 1, nil)
			err := worker.sendTransaction(t.Context(), server.Client(), tx)
			if reject {
				var rpcError gethrpc.Error
				require.ErrorAs(t, err, &rpcError)
				require.Equal(t, -32000, rpcError.ErrorCode())
				require.Contains(t, err.Error(), ": invalid sequence")
				var detail *TransactionRPCError
				require.ErrorAs(t, err, &detail)
				require.JSONEq(t, `{"reason":"nonce"}`, string(detail.Data))
				require.Empty(t, worker.sentTxs)
			} else {
				require.NoError(t, err)
				require.Same(t, tx, <-worker.sentTxs)
			}
			collector := stats.NewCollector()
			worker.SetStatsCollector(collector, nil)
			ctx, cancel := context.WithCancel(t.Context())
			done := make(chan error, 1)
			go func() { done <- worker.processTransactions(ctx, server.Client()) }()
			require.NoError(t, worker.Send(t.Context(), tx))
			require.Eventually(t, func() bool { return collector.GetStats().TotalTxs == 1 }, time.Second, time.Millisecond)
			cancel()
			require.ErrorIs(t, <-done, context.Canceled)
			if reject {
				require.Zero(t, collector.GetStats().EndpointStats[server.URL].SampleCount)
				require.Empty(t, worker.sentTxs)
			} else {
				require.Equal(t, 1, collector.GetStats().EndpointStats[server.URL].SampleCount)
				require.Same(t, tx, <-worker.sentTxs)
			}
		})
	}
}

func TestRepair_324_RPCInvalidResponses(t *testing.T) {
	tx := repair324Tx(t, 0)
	valid := fmt.Sprintf(`{"jsonrpc":"2.0","id":0,"result":%q}`, tx.EthTx.Hash().Hex())
	cases := map[string]string{
		"empty":            "",
		"invalid_json":     "{",
		"wrong_version":    strings.Replace(valid, `"2.0"`, `"1.0"`, 1),
		"wrong_id":         strings.Replace(valid, `"id":0`, `"id":1`, 1),
		"string_id":        strings.Replace(valid, `"id":0`, `"id":"0"`, 1),
		"missing_id":       strings.Replace(valid, `"id":0,`, "", 1),
		"null_id":          strings.Replace(valid, `"id":0`, `"id":null`, 1),
		"wrong_hash":       `{"jsonrpc":"2.0","id":0,"result":"0x` + strings.Repeat("00", 32) + `"}`,
		"short_hash":       `{"jsonrpc":"2.0","id":0,"result":"0x1234"}`,
		"nonhex_hash":      `{"jsonrpc":"2.0","id":0,"result":"0x` + strings.Repeat("zz", 32) + `"}`,
		"null_result":      `{"jsonrpc":"2.0","id":0,"result":null}`,
		"missing_result":   `{"jsonrpc":"2.0","id":0}`,
		"invalid_error":    `{"jsonrpc":"2.0","id":0,"error":{}}`,
		"null_error":       `{"jsonrpc":"2.0","id":0,"error":null}`,
		"result_and_error": strings.TrimSuffix(valid, "}") + `,"error":{"code":-32000,"message":"rejected"}}`,
		"trailing_json":    valid + `{}`,
		"oversize":         valid + strings.Repeat(" ", maxTransactionResponseBytes),
	}
	for name, response := range cases {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				request, err := io.ReadAll(r.Body)
				if err != nil || !bytes.Equal(request, tx.JSONRPCPayload) {
					http.Error(w, "unexpected transaction", http.StatusBadRequest)
					return
				}
				_, _ = io.WriteString(w, response)
			}))
			defer server.Close()
			worker := NewWorker(1, "125", server.URL, 2, 1, nil)
			require.Error(t, worker.sendTransaction(t.Context(), server.Client(), tx))
			require.Empty(t, worker.sentTxs)
		})
	}
	t.Run("real_codec_invalid_params", func(t *testing.T) {
		serverCodec := gethrpc.NewServer()
		defer serverCodec.Stop()
		require.NoError(t, serverCodec.RegisterName("eth", &Repair324RPCTranscript{Input: tx.Payload, Hash: tx.EthTx.Hash()}))
		server := httptest.NewServer(serverCodec)
		defer server.Close()
		badRequest := *tx
		badRequest.JSONRPCPayload = []byte(`{"jsonrpc":"2.0","id":0,"method":"eth_sendRawTransaction","params":[]}`)
		worker := NewWorker(1, "125", server.URL, 2, 1, nil)
		err := worker.sendTransaction(t.Context(), server.Client(), &badRequest)
		var rpcError gethrpc.Error
		require.True(t, errors.As(err, &rpcError))
		require.Equal(t, -32602, rpcError.ErrorCode())
		require.Empty(t, worker.sentTxs)
	})
}
