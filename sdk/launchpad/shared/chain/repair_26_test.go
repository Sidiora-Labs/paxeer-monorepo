package chain

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/chain/profile"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/chain/readpolicy"
	contractv2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/ethclient/simulated"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/params"
)

func TestRepair_26_Transfer(t *testing.T) {
	repair26ReceiptRoundTrip(t, 2, types.ReceiptStatusSuccessful, 0)
}

func TestRepair_26_Reversion(t *testing.T) {
	repair26ReceiptRoundTrip(t, 3, types.ReceiptStatusFailed, 0)
}

func TestRepair_26_Events(t *testing.T) {
	repair26ReceiptRoundTrip(t, 1, types.ReceiptStatusSuccessful, 1)
}

func TestRepair_26_ContractCreation(t *testing.T) {
	repair26ReceiptRoundTrip(t, 4, types.ReceiptStatusSuccessful, 0)
}

func repair26ReceiptRoundTrip(t *testing.T, selected int, expectedStatus uint64, expectedLogs int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	from := crypto.PubkeyToAddress(key.PublicKey)
	emitter := common.HexToAddress("0x10000")
	reverter := common.HexToAddress("0x20000")
	receiver := common.HexToAddress("0x30000")
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := probe.Addr().(*net.TCPAddr).Port
	if err := probe.Close(); err != nil {
		t.Fatal(err)
	}
	chainConfig := *params.AllDevChainProtocolChanges
	chainConfig.ChainID = big.NewInt(125)
	backend := simulated.NewBackend(types.GenesisAlloc{
		from:     {Balance: new(big.Int).Mul(big.NewInt(100), big.NewInt(params.Ether))},
		emitter:  {Code: common.FromHex("0x60006000a000"), Balance: new(big.Int)},
		reverter: {Code: common.FromHex("0x60006000fd"), Balance: new(big.Int)},
	}, func(nodeConfig *node.Config, ethConfig *ethconfig.Config) {
		nodeConfig.HTTPHost = "127.0.0.1"
		nodeConfig.HTTPPort = port
		nodeConfig.HTTPModules = []string{"eth"}
		ethConfig.Genesis.Config = &chainConfig
		ethConfig.NetworkId = 125
	})
	t.Cleanup(func() {
		if err := backend.Close(); err != nil {
			t.Error(err)
		}
	})
	backend.Commit()
	endpoint := fmt.Sprintf("http://127.0.0.1:%d", port)
	raw, err := ethclient.DialContext(ctx, endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	anchor, err := reportedBlockByNumber(ctx, raw, nil)
	if err != nil {
		t.Fatal(err)
	}
	cfg := VerifiedConfig{Profile: profile.Paxeer("repair-26", "repair-26"), AnchorHeight: anchor.Header.Number.Uint64(), AnchorHash: anchor.Hash}
	client, err := NewVerifiedClient(ctx, endpoint, "", cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	var transactions []*types.Transaction
	// Two event-emitting calls make the second receipt's first block-global
	// log index nonzero. All cases also have a nonzero transaction index.
	for nonce, to := range []*common.Address{&emitter, &emitter, &receiver, &reverter, nil} {
		value := new(big.Int)
		var data []byte
		if nonce == 2 {
			value.SetInt64(12345)
		}
		if to == nil {
			// Successful creation of an empty-runtime contract, without logs.
			data = common.FromHex("0x60006000f3")
		}
		tx, err := types.SignNewTx(key, types.LatestSignerForChainID(big.NewInt(125)), &types.LegacyTx{Nonce: uint64(nonce), GasPrice: big.NewInt(2_000_000_000), Gas: 100000, To: to, Value: value, Data: data})
		if err != nil {
			t.Fatal(err)
		}
		if err := raw.SendTransaction(ctx, tx); err != nil {
			t.Fatal(err)
		}
		transactions = append(transactions, tx)
	}
	backend.Commit()
	tx := transactions[selected]
	receipt, err := raw.TransactionReceipt(ctx, tx.Hash())
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Status != expectedStatus || len(receipt.Logs) != expectedLogs || receipt.TransactionIndex != uint(selected) {
		t.Fatalf("real node did not execute expected transaction: %+v", receipt)
	}
	if selected == 1 && receipt.Logs[0].Index != 1 {
		t.Fatalf("event test requires a nonzero first log index: %+v", receipt.Logs)
	}
	if selected == 2 {
		balance, err := raw.BalanceAt(ctx, receiver, receipt.BlockNumber)
		if err != nil || balance.Cmp(tx.Value()) != 0 {
			t.Fatalf("native transfer not executed: balance=%v error=%v", balance, err)
		}
	}
	state, err := readpolicy.Submitted(tx.Hash(), cfg.Profile.DeploymentVersion)
	if err != nil {
		t.Fatal(err)
	}
	next, sdk, err := client.ReceiptState(ctx, state)
	if err != nil {
		t.Fatalf("SDK rejected committed receipt: %v", err)
	}
	expectedState, expectedPublicStatus := "included_success", "included_success"
	if expectedStatus == types.ReceiptStatusFailed {
		expectedState, expectedPublicStatus = "included_failure", "included_reverted"
	}
	if next.State != expectedState || next.BlockHash == nil || *next.BlockHash != receipt.BlockHash || next.BlockNumber != receipt.BlockNumber.String() || sdk.Status != expectedPublicStatus || sdk.Receipt == nil {
		t.Fatalf("committed state/receipt lost: %+v %+v", next, sdk)
	}
	// Drive the API's current production adapter, preserving its committed-block
	// and signed-transaction checks as well as the SDK's verified-client path.
	api, err := TransactionStatus(ctx, raw.Client(), tx.Hash())
	if err != nil || !reflect.DeepEqual(api, sdk) {
		t.Fatalf("API/SDK receipt mismatch: api=%+v sdk=%+v error=%v", api, sdk, err)
	}
	// Also retain coverage of the RPC receipt projection from the first repair.
	// The node produces every receipt message passed to either projection.
	var apiReceipt *RPCReceipt
	if err := raw.Client().CallContext(ctx, &apiReceipt, "eth_getTransactionReceipt", tx.Hash()); err != nil || apiReceipt == nil {
		t.Fatalf("API receipt transport: %v", err)
	}
	rpcView, err := apiReceipt.TxStatus(apiReceipt.EffectiveGasPrice)
	if err != nil || !reflect.DeepEqual(rpcView, sdk) {
		t.Fatalf("RPC/SDK receipt mismatch: rpc=%+v sdk=%+v error=%v", rpcView, sdk, err)
	}
	for _, status := range []contractv2.TxStatusData{sdk, api, rpcView} {
		body, err := json.Marshal(status)
		if err != nil {
			t.Fatal(err)
		}
		if err := contractv2.ValidateJSON("TxStatusData", body); err != nil {
			t.Fatalf("public receipt contract: %v\n%s", err, body)
		}
		var consumer contractv2.TxStatusData
		if err := json.Unmarshal(body, &consumer); err != nil || !reflect.DeepEqual(consumer, status) {
			t.Fatalf("consumer round-trip: %+v error=%v", consumer, err)
		}
		view := consumer.Receipt
		logIndex := "0"
		if len(receipt.Logs) != 0 {
			logIndex = strconv.FormatUint(uint64(receipt.Logs[0].Index), 10)
		}
		if view.Position.ChainId != "125" || string(view.Position.Height) != receipt.BlockNumber.String() || view.Position.BlockHash != receipt.BlockHash.Hex() || view.Position.TransactionHash != tx.Hash().Hex() || string(view.Position.TransactionIndex) != strconv.Itoa(selected) || string(view.Position.LogIndex) != logIndex || view.TransactionHash != tx.Hash().Hex() || view.BlockHash != receipt.BlockHash.Hex() || view.Height != view.Position.Height || view.Success != (expectedStatus == types.ReceiptStatusSuccessful) || view.From != from.Hex() || string(view.GasUsedRaw) != strconv.FormatUint(receipt.GasUsed, 10) || string(view.EffectiveGasPriceRaw) != receipt.EffectiveGasPrice.String() {
			t.Fatalf("consumer lost real receipt evidence: %+v", view)
		}
		if tx.To() == nil {
			if view.To != nil {
				t.Fatal("contract creation acquired a destination")
			}
		} else if view.To == nil || *view.To != tx.To().Hex() {
			t.Fatal("transaction destination lost", view.To)
		}
	}
}
