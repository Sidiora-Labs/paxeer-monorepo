package decode_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/decode"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

func TestNativeHolderAttributionModuleAddress(t *testing.T) {
	if decode.LaunchpadModuleAddress != common.HexToAddress("0x7b3937a5f4397ec652546ef2e72a9e9a35d5ac52") {
		t.Fatal("launchpad module address differs from the chain address")
	}
	raw, err := os.ReadFile("../receipt/testdata/producer_receipts.json")
	if err != nil {
		t.Fatal(err)
	}
	var packet struct {
		NativeSwap, TenantSwap common.Hash
		Responses              map[string]struct{ Result json.RawMessage }
	}
	if err := json.Unmarshal(raw, &packet); err != nil {
		t.Fatal(err)
	}
	for name, hash := range map[string]common.Hash{"native": packet.NativeSwap, "tenant": packet.TenantSwap} {
		t.Run(name, func(t *testing.T) {
			params, err := json.Marshal([]common.Hash{hash})
			if err != nil {
				t.Fatal(err)
			}
			var receipt types.Receipt
			if err := json.Unmarshal(packet.Responses["eth_getTransactionReceipt "+string(params)].Result, &receipt); err != nil {
				t.Fatal(err)
			}
			if len(receipt.Logs) == 0 {
				t.Fatal("recorded settlement transfer absent")
			}
			transfer := receipt.Logs[0]
			if len(transfer.Topics) != 3 || transfer.Topics[0] != crypto.Keccak256Hash([]byte("Transfer(address,address,uint256)")) || common.BytesToAddress(transfer.Topics[2].Bytes()) != decode.LaunchpadModuleAddress {
				t.Fatal("recorded settlement recipient differs from the module address")
			}
		})
	}
}
