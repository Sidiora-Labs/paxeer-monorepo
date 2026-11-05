package evmrpc_test

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/ed25519"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	"github.com/ethereum/go-ethereum/common/hexutil"
	gethrpc "github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
)

func TestNetPeerCountReportsPeers(t *testing.T) {
	f := newRepair1291Fixture(t, "rpc-test-network", false)
	_, httpClient := f.server(t, false)
	_, wsClient := f.server(t, true)
	observe := func(want hexutil.Uint) {
		t.Helper()
		info, err := f.consensus.NetInfo(t.Context())
		require.NoError(t, err)
		require.Equal(t, int(want), info.NPeers)
		for _, consumer := range []*gethrpc.Client{httpClient, wsClient} {
			var result hexutil.Uint
			require.NoError(t, consumer.CallContext(t.Context(), &result, "net_peerCount"))
			require.Equal(t, want, result, "consumer must decode a JSON hex quantity")
		}
	}
	observe(0)
	// Get real NodeAddress values via the public node construction path: the
	// consensus Router is internal to its module and cannot be imported here.
	first := tmtypes.NodeIDFromPubKey(ed25519.GenerateSecretKey().Public())
	second := tmtypes.NodeIDFromPubKey(ed25519.GenerateSecretKey().Public())
	inventory := newRepair1291Consensus(t, first.AddressString("127.0.0.1:1")+","+second.AddressString("127.0.0.1:2"))
	addresses := inventory.Environment.Router.AllAddrs()
	require.Len(t, addresses, 2)
	router := f.consensus.Environment.Router
	require.NoError(t, router.AddAddrs(first, addresses[:1]))
	observe(1)
	require.NoError(t, router.AddAddrs(first, addresses))
	observe(2)
	// Unconnected PEX senders occupy the real ten-entry FIFO cache. Fresh empty
	// inventories evict stale entries, exercising removal through the public API.
	for i := 0; i < 10; i++ {
		require.NoError(t, router.AddAddrs(first, addresses[:0]))
	}
	observe(0)
	f.consensus.Environment.Router = nil
	defer func() { f.consensus.Environment.Router = router }()
	for _, consumer := range []*gethrpc.Client{httpClient, wsClient} {
		var result hexutil.Uint
		repair1291RPCError(t, consumer.CallContext(t.Context(), &result, "net_peerCount"), -32603)
	}
}
