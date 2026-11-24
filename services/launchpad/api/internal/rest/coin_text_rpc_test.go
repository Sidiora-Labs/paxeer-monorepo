package rest

import (
	"encoding/json"
	"math/big"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/store"
	interfaces "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

func TestCoinTextDevelopmentRPC(t *testing.T) {
	raw, err := os.ReadFile("../../../../../contracts/launchpad/artifacts/contracts/core/SidioraERC20.sol/SidioraERC20.json")
	if err != nil {
		t.Fatal(err)
	}
	var artifact struct {
		ABI      json.RawMessage `json:"abi"`
		Bytecode string          `json:"bytecode"`
	}
	if err = json.Unmarshal(raw, &artifact); err != nil {
		t.Fatal(err)
	}
	contract, err := abi.JSON(strings.NewReader(string(artifact.ABI)))
	if err != nil {
		t.Fatal(err)
	}
	chain := newReceiptRecoveryChain(t)
	reader, err := newDevelopmentChain(t.Context(), chain.origin)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.client.Close()
	for _, tc := range []struct{ label, name, symbol, wantName, wantSymbol string }{
		{"multibyte", strings.Repeat("界", 100) + "x", strings.Repeat("🪙", 16) + "x", strings.Repeat("界", 100), strings.Repeat("🪙", 16)},
		{"empty", "", "", "", ""},
	} {
		t.Run(tc.label, func(t *testing.T) {
			opts, authErr := bind.NewKeyedTransactorWithChainID(chain.key, big.NewInt(125))
			if authErr != nil {
				t.Fatal(authErr)
			}
			opts.Context = t.Context()
			address, tx, _, deployErr := bind.DeployContract(opts, contract, common.FromHex(artifact.Bytecode), chain.backend.Client(), tc.name, tc.symbol, big.NewInt(1000000), crypto.PubkeyToAddress(chain.key.PublicKey))
			if deployErr != nil {
				t.Fatal(deployErr)
			}
			chain.backend.Commit()
			receipt, receiptErr := chain.backend.Client().TransactionReceipt(t.Context(), tx.Hash())
			if receiptErr != nil || receipt.Status != types.ReceiptStatusSuccessful {
				t.Fatalf("token deployment: %v %+v", receiptErr, receipt)
			}
			wm := store.ChainWatermark{ChainID: "125", Height: strconv.FormatUint(receipt.BlockNumber.Uint64(), 10), BlockHash: receipt.BlockHash.Hex()}
			doc, readErr := reader.tokenDocument(t.Context(), strings.ToLower(address.Hex()), wm)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if doc["name"] != tc.wantName || doc["symbol"] != tc.wantSymbol || doc["decimals"] != 6 || doc["totalSupplyRaw"] != "1000000" || doc["decimalsQuality"] != "verified" || doc["supplyQuality"] != "verified" || doc["sourcePosition"] == nil {
				t.Fatalf("RPC token document: %v", doc)
			}
			body, marshalErr := json.Marshal(doc)
			if marshalErr != nil {
				t.Fatal(marshalErr)
			}
			if err = interfaces.ValidateJSON("Token", body); err != nil {
				t.Fatal(err)
			}
		})
	}
}
