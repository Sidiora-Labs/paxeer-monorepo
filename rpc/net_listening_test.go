package evmrpc_test

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNetListeningReportsNetInfo(t *testing.T) {
	response := sendRequestGoodWithNamespace(t, "net", "listening")
	require.Nil(t, response["error"])
	require.Equal(t, true, response["result"])
}
