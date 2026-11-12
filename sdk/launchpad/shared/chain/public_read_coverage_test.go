package chain

import (
	"context"
	"math/big"
	"sync/atomic"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/chain/profile"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
)

func TestPublicPinnedReadBindsCallAndRevision(t *testing.T) {
	header := testHeader(100)
	var calls atomic.Int64
	server := pinnedServer(t, header, false, &calls)
	client, err := NewVerifiedClient(context.Background(), server.URL, "", VerifiedConfig{profile.Paxeer("e1", "d1"), 100, consensusHash(header)})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	pin, err := client.Pin(context.Background(), 100)
	if err != nil {
		t.Fatal(err)
	}
	address := common.HexToAddress("0x1111111111111111111111111111111111111111")
	request := ethereum.CallMsg{To: &address, Data: []byte{1, 2, 3, 4}}
	for _, revision := range []string{"current", "next"} {
		values, err := client.ReadPublicPinned(context.Background(), pin, revision, []ethereum.CallMsg{request})
		if err != nil || len(values) != 1 || len(values[0]) != 1 || values[0][0] != 1 {
			t.Fatalf("public call at %s: %x, %v", revision, values, err)
		}
	}
	if calls.Load() != 2 {
		t.Fatalf("revision did not partition pinned results: %d upstream calls", calls.Load())
	}
	invalid := []struct {
		name string
		call ethereum.CallMsg
	}{
		{"actor", ethereum.CallMsg{From: address, To: &address, Data: request.Data}},
		{"value", ethereum.CallMsg{To: &address, Data: request.Data, Value: big.NewInt(1)}},
		{"gas price", ethereum.CallMsg{To: &address, Data: request.Data, GasPrice: big.NewInt(1)}},
		{"missing selector", ethereum.CallMsg{To: &address, Data: []byte{1}}},
		{"missing target", ethereum.CallMsg{Data: request.Data}},
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			if values, err := client.ReadPublicPinned(context.Background(), pin, "current", []ethereum.CallMsg{tc.call}); err == nil || values != nil {
				t.Fatalf("wallet dependent call admitted: %x, %v", values, err)
			}
		})
	}
	if values, err := client.ReadPublicPinned(context.Background(), pin, "", []ethereum.CallMsg{request}); err == nil || values != nil {
		t.Fatalf("unbound revision admitted: %x, %v", values, err)
	}
	unknown := pin
	unknown.Hash = common.HexToHash("0x1234")
	if values, err := client.ReadPublicPinned(context.Background(), unknown, "current", []ethereum.CallMsg{request}); err == nil || values != nil {
		t.Fatalf("unissued pin admitted: %x, %v", values, err)
	}
	client.quarantineEndpoint(client.primary)
	if values, err := client.ReadPublicPinned(context.Background(), pin, "current", []ethereum.CallMsg{request}); err == nil || values != nil {
		t.Fatalf("quarantined endpoint served public read: %x, %v", values, err)
	}
}
