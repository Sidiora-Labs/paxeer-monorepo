package chainreader

import (
	"encoding/json"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/decode"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"math/big"
	"os"
	"testing"
)

func creationReceiptFixture(t *testing.T) (*decode.Registry, *ethtypes.Receipt, map[string]string) {
	t.Helper()
	root := "../../../../../sdk/launchpad/protocol/contracts/v1/chain/"
	catalog, err := os.ReadFile(root + "abi-catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(root + "golden-fixtures.json")
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		Addresses map[string]string
		Receipts  []struct {
			Label       string
			Hash        string
			Status      uint64
			BlockHash   string
			BlockNumber uint64
			Index       uint
			Logs        []struct {
				Address          string
				BlockHash        string
				BlockNumber      uint64
				Data             string
				Index            uint
				Topics           []string
				TransactionHash  string
				TransactionIndex uint
			}
		}
	}
	if err = json.Unmarshal(b, &f); err != nil {
		t.Fatal(err)
	}
	var c decode.Catalog
	if err = json.Unmarshal(catalog, &c); err != nil {
		t.Fatal(err)
	}
	var ds []decode.Deployment
	roles := map[string]string{"factory": "SidioraFactory", "eventEmitter": "EventEmitter", "registry": "PoolRegistry", "accumulator": "FeeAccumulator", "nft": "SidioraNFT", "router": "Router", "token": "SidioraERC20", "usdl": "SidioraERC20"}
	for key, role := range roles {
		for _, cc := range c.Contracts {
			if cc.Name == role {
				ds = append(ds, decode.Deployment{Address: common.HexToAddress(f.Addresses[key]), Role: role, ABIRevision: cc.ABIRevision})
			}
		}
	}
	r, err := decode.NewRegistry(catalog, ds)
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range f.Receipts {
		if g.Label != "router-create" {
			continue
		}
		receipt := &ethtypes.Receipt{TxHash: common.HexToHash(g.Hash), BlockHash: common.HexToHash(g.BlockHash), BlockNumber: new(big.Int).SetUint64(g.BlockNumber), Status: g.Status, TransactionIndex: g.Index}
		for _, raw := range g.Logs {
			l := &ethtypes.Log{Address: common.HexToAddress(raw.Address), BlockHash: common.HexToHash(raw.BlockHash), BlockNumber: raw.BlockNumber, Data: common.FromHex(raw.Data), Index: raw.Index, TxHash: common.HexToHash(raw.TransactionHash), TxIndex: raw.TransactionIndex}
			for _, topic := range raw.Topics {
				l.Topics = append(l.Topics, common.HexToHash(topic))
			}
			receipt.Logs = append(receipt.Logs, l)
		}
		return r, receipt, f.Addresses
	}
	t.Fatal("authentic creation receipt absent")
	return nil, nil, nil
}

func TestCreationFromAuthenticFrozenReceipt(t *testing.T) {
	r, receipt, addresses := creationReceiptFixture(t)
	result, err := CreationFromReceipt(r, receipt, receipt.TxHash)
	if err != nil {
		t.Fatal(err)
	}
	if result.Token != common.HexToAddress(addresses["token"]) || result.Pool != common.HexToAddress(addresses["pool"]) || result.NFTID.Sign() <= 0 {
		t.Fatalf("wrong creation %#v", result)
	}
	receipt.Logs = append(receipt.Logs, receipt.Logs...)
	if _, err = CreationFromReceipt(r, receipt, receipt.TxHash); err != nil {
		t.Fatal("duplicate raw delivery", err)
	}
	if _, err = CreationFromReceipt(r, receipt, common.HexToHash("0x123")); err == nil {
		t.Fatal("wrong exact receipt")
	}
	receipt.Status = 0
	if _, err = CreationFromReceipt(r, receipt, receipt.TxHash); err == nil {
		t.Fatal("reverted receipt")
	}
	receipt.Status = 1
	receipt.BlockNumber = nil
	if _, err = CreationFromReceipt(r, receipt, receipt.TxHash); err == nil {
		t.Fatal("missing height")
	}
}

func TestCreationReceiptAdmissionCaps(t *testing.T) {
	r, receipt, _ := creationReceiptFixture(t)
	original := append([]*ethtypes.Log(nil), receipt.Logs...)
	foreign := &ethtypes.Log{Address: common.HexToAddress("0x111"), TxHash: receipt.TxHash, BlockHash: receipt.BlockHash, BlockNumber: receipt.BlockNumber.Uint64(), TxIndex: receipt.TransactionIndex}
	for _, mode := range []string{"receipt logs", "log data", "bytes"} {
		t.Run(mode, func(t *testing.T) {
			receipt.Logs = append([]*ethtypes.Log(nil), original...)
			count, size := decode.MaxRawBatchLogs+1, 0
			if mode == "log data" {
				count, size = 1, decode.MaxEnvelopeBytes+1
			} else if mode == "bytes" {
				count, size = decode.MaxBlockLogBytes/(256+(1<<20))+1, 1<<20
			}
			data := make([]byte, size)
			for i := 0; i < count; i++ {
				log := *foreign
				log.Index, log.Data = uint(len(original)+i), data
				receipt.Logs = append(receipt.Logs, &log)
			}
			got, err := CreationFromReceipt(r, receipt, receipt.TxHash)
			if mode == "bytes" {
				if err == nil || got != nil {
					t.Fatal("receipt byte bound")
				}
			} else if err != nil || got == nil {
				t.Fatalf("creation receipt content: %v", err)
			}
		})
	}
}
