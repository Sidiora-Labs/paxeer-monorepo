package http

import (
	"errors"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/light/provider"
	rpchttp "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client/http"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	rpctypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/types"
)

func TestParseRPCErrorCommitNotAvailableIsUnreliable(t *testing.T) {
	client, err := rpchttp.New("http://127.0.0.1:26657")
	if err != nil {
		t.Fatal(err)
	}
	p := NewWithClient("chain-test", client).(*http)

	for _, data := range []string{
		coretypes.ErrCommitNotAvailable.Error(),
		"commit request failed: " + coretypes.ErrCommitNotAvailable.Error() + " (height 42)",
	} {
		rpcErr := &rpctypes.RPCError{Code: -32603, Message: "Internal error", Data: data}
		err := p.parseRPCError(rpcErr)
		var unreliable provider.ErrUnreliableProvider
		if !errors.As(err, &unreliable) {
			t.Fatalf("parseRPCError(%q) returned %T, want provider.ErrUnreliableProvider", data, err)
		}
		if unreliable.Reason != rpcErr {
			t.Fatalf("parseRPCError(%q) did not preserve the original RPC error as its reason", data)
		}
		if !errors.Is(err, rpcErr) {
			t.Fatalf("parseRPCError(%q) did not unwrap to the original RPC error", data)
		}
	}
}
