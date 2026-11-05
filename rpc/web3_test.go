package evmrpc_test

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClientVersion(t *testing.T) {
	f := newRepair1291Fixture(t, "rpc-test-network", false)
	expected := "Geth/" + runtime.GOOS + "-" + runtime.GOARCH + "/" + runtime.Version()
	for _, ws := range []bool{false, true} {
		_, consumer := f.server(t, ws)
		var version string
		require.NoError(t, consumer.CallContext(t.Context(), &version, "web3_clientVersion"))
		require.Equal(t, expected, version)
	}
}
