package processor

import (
	"encoding/json"
	"math/big"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/decode"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func TestEmitterOutsideEpochsTransferFactsRequireDecodedEnvelope(t *testing.T) {
	body, err := os.ReadFile("../../../../../sdk/launchpad/protocol/contracts/v1/chain/abi-catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog decode.Catalog
	if err = json.Unmarshal(body, &catalog); err != nil {
		t.Fatal(err)
	}
	token := common.HexToAddress("0x20")
	foreign := common.HexToAddress("0x21")
	var d decode.Deployment
	var transfer abi.Event
	for _, contract := range catalog.Contracts {
		if contract.Name != "SidioraERC20" {
			continue
		}
		a, err := abi.JSON(strings.NewReader(string(contract.ABI)))
		if err != nil {
			t.Fatal(err)
		}
		transfer = a.Events["Transfer"]
		d = decode.Deployment{Address: token, Role: contract.Name, ABIRevision: contract.ABIRevision}
	}
	if transfer.Name != "Transfer" {
		t.Fatal("catalog Transfer required")
	}
	r, err := decode.NewRegistry(body, []decode.Deployment{d})
	if err != nil {
		t.Fatal(err)
	}
	data, err := transfer.Inputs.NonIndexed().Pack(big.NewInt(7))
	if err != nil {
		t.Fatal(err)
	}
	log := types.Log{Address: token, BlockNumber: 70, BlockHash: common.HexToHash("0x777"), TxHash: common.HexToHash("0x888"), Topics: []common.Hash{transfer.ID, common.HexToHash("0x1"), common.HexToHash("0x2")}, Data: data}
	unbound := log
	unbound.Address = foreign
	unbound.Index = 1
	logs := []types.Log{log, unbound}
	var envelopes []*decode.DecodedEnvelope
	for _, l := range logs {
		env, err := r.Decode(l)
		if err != nil {
			t.Fatal(err)
		}
		envelopes = append(envelopes, env)
	}
	if envelopes[0].Status != "decoded" || envelopes[0].Event == nil || envelopes[1].Status != "opaque" || envelopes[1].Event != nil || envelopes[1].FailureReason != "untrusted_emitter" {
		t.Fatalf("transfer envelopes: %+v", envelopes)
	}
	if got := decodedSourceLogs(logs, envelopes); !reflect.DeepEqual(got, []types.Log{log}) {
		t.Fatalf("decoded source logs: %+v", got)
	}
	for _, status := range []string{"opaque", "quarantined", ""} {
		changed := *envelopes[0]
		changed.Status = status
		if got := decodedSourceLogs(logs, []*decode.DecodedEnvelope{nil, &changed, envelopes[1]}); len(got) != 0 {
			t.Fatalf("status %q supplies transfer facts: %+v", status, got)
		}
	}
	if got := decodedSourceLogs(logs, nil); len(got) != 0 {
		t.Fatalf("raw topics supply transfer facts: %+v", got)
	}
}
