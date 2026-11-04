package sequencer_test

import (
	"crypto/tls"
	"net"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	"github.com/stretchr/testify/require"
)

func listenAuthenticated(t *testing.T) (net.Listener, sequencer.ReplicationTLSConfig) {
	t.Helper()
	primary, standby := sequencer.Repair103TLSFixture(t)
	cfg, err := standby.TLSConfig(true)
	require.NoError(t, err)
	l, err := tls.Listen("tcp", "127.0.0.1:0", cfg)
	require.NoError(t, err)
	return l, primary
}
