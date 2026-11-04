package p2p

import (
	"net/netip"
	"testing"
	"testing/synctest"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils/require"
)

func TestRepairM_1122_IdleHistoryExpires(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const window = time.Minute
		tracker := newConnTracker(2, window)
		for batch := range 8 {
			for i := range 512 {
				n := batch*512 + i
				addr := netip.AddrPortFrom(netip.AddrFrom4([4]byte{127, 1, byte(n >> 8), byte(n)}), 1234)
				require.NoError(t, tracker.AddConn(addr))
				tracker.RemoveConn(addr)
				require.Error(t, tracker.AddConn(addr))
			}
			require.Zero(t, tracker.Len())
			tracker.mutex.RLock()
			require.Len(t, tracker.lastConnect, 512)
			require.Equal(t, 512, tracker.expiry.Len())
			tracker.mutex.RUnlock()

			// Virtual time advances without another tracker call to trigger pruning.
			time.Sleep(window)
			synctest.Wait()
			tracker.mutex.RLock()
			require.Empty(t, tracker.lastConnect)
			require.Zero(t, tracker.expiry.Len())
			require.Nil(t, tracker.expiryTimer)
			tracker.mutex.RUnlock()
		}
	})
}

func TestRepairM_1122_RefreshPreservesWindowAndLimit(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const window = time.Minute
		tracker := newConnTracker(2, window)
		a := netip.MustParseAddrPort("127.0.0.1:1000")
		b := netip.MustParseAddrPort("127.0.0.2:1000")
		require.NoError(t, tracker.AddConn(a))
		time.Sleep(window / 4)
		require.NoError(t, tracker.AddConn(b))
		tracker.RemoveConn(b)
		time.Sleep(window / 4)
		for range 1000 {
			require.NoError(t, tracker.AddConn(a))
			require.Error(t, tracker.AddConn(a))
			tracker.RemoveConn(a)
		}
		tracker.RemoveConn(a)
		require.Zero(t, tracker.Len())
		tracker.mutex.RLock()
		require.Len(t, tracker.lastConnect, 2)
		require.Equal(t, 2, tracker.expiry.Len())
		tracker.mutex.RUnlock()

		time.Sleep(window / 2)
		synctest.Wait()
		require.Error(t, tracker.AddConn(a))
		require.Error(t, tracker.AddConn(b))
		time.Sleep(window / 4)
		synctest.Wait()
		tracker.mutex.RLock()
		require.Len(t, tracker.lastConnect, 1)
		require.NotNil(t, tracker.lastConnect[a.Addr()])
		tracker.mutex.RUnlock()
		require.Error(t, tracker.AddConn(a))
		require.NoError(t, tracker.AddConn(b))
		tracker.RemoveConn(b)
		time.Sleep(window / 4)
		synctest.Wait()
		require.NoError(t, tracker.AddConn(a))
		require.NoError(t, tracker.AddConn(a))
		require.Error(t, tracker.AddConn(a))
		time.Sleep(window)
		synctest.Wait()
		// Expiring history must not remove live connection accounting.
		require.Equal(t, 1, tracker.Len())
		require.Error(t, tracker.AddConn(a))
		tracker.RemoveConn(a)
		tracker.RemoveConn(a)
		require.NoError(t, tracker.AddConn(a))
		tracker.RemoveConn(a)
		time.Sleep(window)
		synctest.Wait()
	})
}

func TestRepairM_1122_DisabledWindowKeepsNoHistory(t *testing.T) {
	tracker := newConnTracker(1, 0)
	addr := netip.MustParseAddrPort("127.0.0.1:1000")
	for range 1000 {
		require.NoError(t, tracker.AddConn(addr))
		require.Error(t, tracker.AddConn(addr))
		tracker.RemoveConn(addr)
	}
	require.Zero(t, tracker.Len())
	require.Empty(t, tracker.lastConnect)
	require.Zero(t, tracker.expiry.Len())
	require.Nil(t, tracker.expiryTimer)
}

func TestRepairM_1122_OrderedPeerUpdates(t *testing.T) {
	network := MakeTestNetwork(t, TestNetworkOptions{})
	observer := network.MakeNode(t, TestNodeOptions{})
	first := network.MakeNode(t, TestNodeOptions{})
	second := network.MakeNode(t, TestNodeOptions{})
	third := network.MakeNode(t, TestNodeOptions{})
	sub := observer.Router.Subscribe()

	require.NoError(t, observer.Connect(t.Context(), first))
	RequireUpdates(t, sub, []PeerUpdate{{NodeID: first.NodeID, Status: PeerStatusUp}})

	// Down events precede up events in the real subscription, even when both
	// transitions have completed before the consumer reads its next batch.
	network.Remove(t, first.NodeID)
	require.NoError(t, observer.Connect(t.Context(), second))
	expected := []PeerUpdate{
		{NodeID: first.NodeID, Status: PeerStatusDown},
		{NodeID: second.NodeID, Status: PeerStatusUp},
	}
	RequireUpdates(t, sub, expected)
	require.Len(t, expected, 2)

	network.Remove(t, second.NodeID)
	require.NoError(t, observer.Connect(t.Context(), third))
	RequireUpdates(t, sub, []PeerUpdate{
		{NodeID: second.NodeID, Status: PeerStatusDown},
		{NodeID: third.NodeID, Status: PeerStatusUp},
	})
	network.Remove(t, third.NodeID)
	RequireUpdates(t, sub, []PeerUpdate{{NodeID: third.NodeID, Status: PeerStatusDown}})
}
