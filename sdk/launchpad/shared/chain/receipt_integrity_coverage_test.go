package chain

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/ethclient/simulated"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rpc"
)

func TestReceiptEvidenceRejectsChangedCommittedRPCFields(t *testing.T) {
	ctx := context.Background()
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	from := crypto.PubkeyToAddress(key.PublicKey)
	to := common.HexToAddress("0x1234")
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	cfg := *params.AllDevChainProtocolChanges
	cfg.ChainID = big.NewInt(125)
	backend := simulated.NewBackend(types.GenesisAlloc{from: {Balance: new(big.Int).Mul(big.NewInt(10), big.NewInt(params.Ether))}}, func(n *node.Config, e *ethconfig.Config) {
		n.HTTPHost = "127.0.0.1"
		n.HTTPPort = port
		n.HTTPModules = []string{"eth"}
		e.Genesis.Config = &cfg
		e.NetworkId = 125
	})
	defer func() {
		if err := backend.Close(); err != nil {
			t.Error(err)
		}
	}()
	endpoint := fmt.Sprintf("http://127.0.0.1:%d", port)
	source, err := ethclient.DialContext(ctx, endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	signed, err := types.SignNewTx(key, types.LatestSignerForChainID(big.NewInt(125)), &types.LegacyTx{To: &to, Gas: 21000, GasPrice: big.NewInt(2_000_000_000), Value: big.NewInt(1)})
	if err != nil {
		t.Fatal(err)
	}
	if err := source.SendTransaction(ctx, signed); err != nil {
		t.Fatal(err)
	}
	backend.Commit()
	for _, mode := range []string{"valid", "receipt format", "receipt status", "receipt gas", "receipt index", "receipt hash", "transaction format", "transaction absent", "transaction hash", "transaction block", "transaction height", "transaction index", "block hash", "block body", "block unavailable", "receipt absent", "both absent", "pending"} {
		t.Run(mode, func(t *testing.T) {
			proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				raw, err := io.ReadAll(r.Body)
				if err != nil {
					t.Error(err)
					return
				}
				var request struct{ Method string }
				if json.Unmarshal(raw, &request) != nil {
					t.Error("invalid request")
					return
				}
				upstream, err := http.Post(endpoint, "application/json", bytes.NewReader(raw))
				if err != nil {
					t.Error(err)
					return
				}
				defer upstream.Body.Close()
				var response map[string]json.RawMessage
				if json.NewDecoder(upstream.Body).Decode(&response) != nil {
					t.Error("invalid upstream response")
					return
				}
				result := map[string]any{}
				_ = json.Unmarshal(response["result"], &result)
				var replacement any = result
				changed := false
				if request.Method == "eth_getTransactionReceipt" {
					switch mode {
					case "receipt format":
						replacement = "invalid"
						changed = true
					case "receipt status":
						result["status"] = "0x2"
						changed = true
					case "receipt gas":
						delete(result, "gasUsed")
						changed = true
					case "receipt index":
						delete(result, "transactionIndex")
						changed = true
					case "receipt hash":
						result["transactionHash"] = common.Hash{1}.Hex()
						changed = true
					case "receipt absent", "both absent", "pending":
						replacement = nil
						changed = true
					}
				}
				if request.Method == "eth_getTransactionByHash" {
					switch mode {
					case "transaction format":
						replacement = "invalid"
						changed = true
					case "transaction absent", "both absent":
						replacement = nil
						changed = true
					case "transaction hash":
						result["hash"] = common.Hash{1}.Hex()
						changed = true
					case "transaction block":
						result["blockHash"] = common.Hash{1}.Hex()
						changed = true
					case "transaction height":
						result["blockNumber"] = "0x1234"
						changed = true
					case "transaction index":
						result["transactionIndex"] = "0x1"
						changed = true
					case "pending":
						delete(result, "blockHash")
						delete(result, "blockNumber")
						delete(result, "transactionIndex")
						changed = true
					}
				}
				if request.Method == "eth_getBlockByHash" {
					switch mode {
					case "block body":
						result["transactions"] = []string{common.Hash{1}.Hex()}
						changed = true
					case "block unavailable":
						replacement = nil
						changed = true
					}
				}
				if request.Method == "eth_getBlockByNumber" && mode == "block hash" {
					result["hash"] = common.Hash{1}.Hex()
					changed = true
				}
				if changed {
					encoded, err := json.Marshal(replacement)
					if err != nil {
						t.Error(err)
						return
					}
					response["result"] = encoded
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(response)
			}))
			defer proxy.Close()
			client, err := rpc.DialContext(ctx, proxy.URL)
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			status, err := TransactionStatus(ctx, client, signed.Hash())
			switch mode {
			case "valid":
				if err != nil || status.Status != "included_success" || status.Receipt == nil || status.Receipt.TransactionHash != signed.Hash().Hex() {
					t.Fatalf("committed receipt: %+v %v", status, err)
				}
			case "both absent":
				if err != nil || status.Status != "unknown" || status.Receipt != nil {
					t.Fatalf("absent transaction: %+v %v", status, err)
				}
			case "pending":
				if err != nil || status.Status != valueSubmitted || status.Receipt != nil {
					t.Fatalf("pending transaction: %+v %v", status, err)
				}
			default:
				if !errors.Is(err, ErrReceiptIntegrity) || status.Receipt != nil {
					t.Fatalf("changed receipt %s: %+v %v", mode, status, err)
				}
			}
		})
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := TransactionStatus(cancelled, source.Client(), signed.Hash()); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := projectReceipt(receiptEvidence{receipt: &types.Receipt{}, tx: types.NewTx(&types.LegacyTx{})}); !errors.Is(err, ErrReceiptIntegrity) {
		t.Fatal(err)
	}
	receipt, err := source.TransactionReceipt(ctx, signed.Hash())
	if err != nil {
		t.Fatal(err)
	}
	receipt.EffectiveGasPrice = nil
	if _, err := projectReceipt(receiptEvidence{receipt: receipt, tx: signed}); !errors.Is(err, ErrReceiptIntegrity) {
		t.Fatal(err)
	}
}
