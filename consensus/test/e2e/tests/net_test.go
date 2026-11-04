package e2e_test

import (
	"context"
	"fmt"
	"net"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/p2p"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	e2e "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/test/e2e/pkg"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

// Tests the configured topology, including connectivity obtained through discovery.
func TestNet_Peers(t *testing.T) {
	testNode(t, func(ctx context.Context, t *testing.T, node e2e.Node) {
		client, err := node.Client()
		require.NoError(t, err)
		netInfo, err := client.NetInfo(ctx)
		require.NoError(t, err)
		require.NoError(t, checkPeerTopology(node, netInfo))
	})
}

func checkPeerTopology(node e2e.Node, info *coretypes.ResultNetInfo) error {
	nodes := make(map[types.NodeID]*e2e.Node, len(node.Testnet.Nodes))
	for _, peer := range node.Testnet.Nodes {
		id := types.NodeIDFromPubKey(peer.NodeKey.Public())
		if _, exists := nodes[id]; exists {
			return fmt.Errorf("duplicate manifest peer ID %s", id)
		}
		nodes[id] = peer
	}
	self := types.NodeIDFromPubKey(node.NodeKey.Public())
	known := make(map[types.NodeID]bool, len(info.Peers))
	for _, peer := range info.Peers {
		n, ok := nodes[peer.ID]
		if !ok || peer.ID == self {
			return fmt.Errorf("unknown or self peer %s", peer.ID)
		}
		if known[peer.ID] {
			return fmt.Errorf("duplicate advertised peer %s", peer.ID)
		}
		addr, err := p2p.ParseNodeAddress(peer.URL)
		if err != nil {
			return err
		}
		if addr.NodeID != peer.ID || !net.ParseIP(addr.Hostname).Equal(n.IP) {
			return fmt.Errorf("unexpected address for peer %s (%s): %s", n.Name, peer.ID, peer.URL)
		}
		known[peer.ID] = true
	}
	if info.NPeers != len(known) {
		return fmt.Errorf("peer count %d differs from advertised peers %d", info.NPeers, len(known))
	}
	connected := make(map[types.NodeID]bool, len(info.PeerConnections))
	for _, connection := range info.PeerConnections {
		if _, ok := nodes[connection.ID]; !ok || connection.ID == self {
			return fmt.Errorf("unknown or self connection %s", connection.ID)
		}
		states := strings.Split(connection.State, ",")
		ready, up := false, false
		for _, state := range states {
			ready = ready || state == "ready"
			up = up || state == "connected"
		}
		if !ready || !up {
			return fmt.Errorf("peer %s is not ready and connected: %s", connection.ID, connection.State)
		}
		connected[connection.ID] = true
	}
	for _, peer := range node.PersistentPeers {
		if !connected[types.NodeIDFromPubKey(peer.NodeKey.Public())] {
			return fmt.Errorf("node %s missing persistent connection to %s", node.Name, peer.Name)
		}
	}
	// Seeds bootstrap discovery and may disconnect after exchanging addresses.
	// Require a stateful connection, not a permanent connection to each seed.
	if len(node.Seeds) > 0 {
		for id := range connected {
			if !nodes[id].Stateless() {
				return nil
			}
		}
		return fmt.Errorf("node %s has no stateful connection after discovery", node.Name)
	}
	return nil
}
