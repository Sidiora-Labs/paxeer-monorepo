package evmrpc_test

import (
	"testing"

	gethrpc "github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
)

func TestNetListeningReportsNetInfo(t *testing.T) {
	f := newRepair1291Fixture(t, "rpc-test-network", false)
	_, httpClient := f.server(t, false)
	_, wsClient := f.server(t, true)
	for _, listening := range []bool{false, true, false} {
		f.consensus.Environment.IsListening = listening
		for _, consumer := range []*gethrpc.Client{httpClient, wsClient} {
			var result bool
			require.NoError(t, consumer.CallContext(t.Context(), &result, "net_listening"))
			require.Equal(t, listening, result)
		}
	}
	// A genuinely unavailable consensus router fails inside the production
	// Environment; the transport must surface the failure, never invent false.
	router := f.consensus.Environment.Router
	f.consensus.Environment.Router = nil
	defer func() { f.consensus.Environment.Router = router }()
	for _, consumer := range []*gethrpc.Client{httpClient, wsClient} {
		var result bool
		repair1291RPCError(t, consumer.CallContext(t.Context(), &result, "net_listening"), -32603)
	}
}
