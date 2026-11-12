package chainreader

import (
	"context"
	"encoding/json"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/internaltest/chainfixture"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

func TestCreationEvidenceRejectsMalformedRecordedLogs(t *testing.T) {
	resolver, hash, _ := recordedCreationResolver(t, nil)
	original, err := resolver.rpc.TransactionReceipt(context.Background(), hash)
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"nil log", "removed log", "coordinates", "conflicting duplicate", "malformed event", "ambiguous creations", "zero token", "zero NFT", "missing native"} {
		t.Run(mode, func(t *testing.T) {
			body, err := json.Marshal(original)
			if err != nil {
				t.Fatal(err)
			}
			var receipt types.Receipt
			if err = json.Unmarshal(body, &receipt); err != nil {
				t.Fatal(err)
			}
			index := -1
			for i, l := range receipt.Logs {
				if len(l.Topics) > 0 && l.Topics[0] == marketCreatedFactoryTopic {
					index = i
					break
				}
			}
			if index < 0 {
				t.Fatal("native creation absent")
			}
			switch mode {
			case "nil log":
				receipt.Logs[0] = nil
			case "removed log":
				receipt.Logs[0].Removed = true
			case "coordinates":
				receipt.Logs[0].TxHash = common.HexToHash("0x1")
			case "conflicting duplicate":
				extra := *receipt.Logs[index]
				extra.Data = append(append([]byte(nil), extra.Data...), 1)
				receipt.Logs = append(receipt.Logs, &extra)
			case "malformed event":
				receipt.Logs[index].Data = []byte{1}
			case "ambiguous creations":
				extra := *receipt.Logs[index]
				extra.Index++
				receipt.Logs = append(receipt.Logs, &extra)
			case "zero token":
				receipt.Logs[index].Topics[1] = common.Hash{}
			case "zero NFT":
				for i := 0; i < 32; i++ {
					receipt.Logs[index].Data[i] = 0
				}
			case "missing native":
				receipt.Logs = append(receipt.Logs[:index], receipt.Logs[index+1:]...)
			}
			if got, err := CreationFromReceipt(resolver.registry, &receipt, hash); err == nil || got != nil {
				t.Fatalf("invalid creation %s admitted: %+v %v", mode, got, err)
			}
		})
	}
}

func TestCreationResolverDistinguishesPendingAndUnavailableRecordedEvidence(t *testing.T) {
	baseline, hash, _ := recordedCreationResolver(t, nil)
	fixture := chainfixture.New(t)
	var mode atomic.Value
	mode.Store("")
	fixture.Mutate = func(key string, raw json.RawMessage) json.RawMessage {
		switch mode.Load().(string) {
		case "chain":
			if strings.HasPrefix(key, "eth_chainId ") {
				return json.RawMessage(`"0x1"`)
			}
		case "pending":
			if strings.HasPrefix(key, "eth_getTransactionReceipt ") {
				return json.RawMessage(`null`)
			}
		case "header":
			if strings.HasPrefix(key, "eth_getBlockByNumber ") {
				return json.RawMessage(`{"number":false}`)
			}
		case "native":
			if strings.HasPrefix(key, "eth_getTransactionReceipt ") {
				var fields map[string]json.RawMessage
				if err := json.Unmarshal(raw, &fields); err != nil {
					t.Error(err)
					return raw
				}
				fields["logs"] = json.RawMessage(`[]`)
				body, err := json.Marshal(fields)
				if err != nil {
					t.Error(err)
					return raw
				}
				return body
			}
		}
		return raw
	}
	client, err := ethclient.DialContext(context.Background(), recordedHeaderEndpoint(t, fixture))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	resolver, err := NewCreationResolver(context.Background(), client, baseline.registry)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		mode, status, failure string
		valid                 bool
	}{{"pending", "pending", "", true}, {"header", "unavailable", "historical_header_rpc", false}, {"native", "integrity_error", "native_creation_evidence", false}} {
		mode.Store(tc.mode)
		out, err := resolver.Resolve(context.Background(), 125, hash.Hex(), nil)
		if (err == nil) != tc.valid || out.Status != tc.status || out.Failure != tc.failure || out.Token != nil {
			t.Fatalf("creation state %s: %+v %v", tc.mode, out, err)
		}
	}
	mode.Store("chain")
	if _, err := NewCreationResolver(context.Background(), client, baseline.registry); err == nil {
		t.Fatal("foreign creation chain admitted")
	}
	if _, err := NewHistoricalReader(context.Background(), client); err == nil {
		t.Fatal("foreign historical chain admitted")
	}
	client.Close()
	if _, err := NewCreationResolver(context.Background(), client, baseline.registry); err == nil {
		t.Fatal("closed creation transport admitted")
	}
	if _, err := NewHistoricalReader(context.Background(), client); err == nil {
		t.Fatal("closed historical transport admitted")
	}
}
