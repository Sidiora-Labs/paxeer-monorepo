package evmrpc_test

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNetPeerCountReportsPeers(t *testing.T) {
	response := sendRequestGoodWithNamespace(t, "net", "peerCount")

	require.Nil(t, response["error"])
	require.Equal(t, "0x3", response["result"])
}
