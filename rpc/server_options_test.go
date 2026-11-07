package evmrpc_test

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	"github.com/stretchr/testify/require"
)

var (
	_ evmrpc.ServerOption = evmrpc.PendingOption{}
	_ evmrpc.ServerOption = evmrpc.SequencerForwardOption{}
)

func TestNewPendingOptionRejectsNil(t *testing.T) {
	option, err := evmrpc.NewPendingOption(nil)
	require.Error(t, err)
	require.Equal(t, evmrpc.PendingOption{}, option)

	s := newPendingSequencer(t, 1)
	typedNil := reflect.Zero(reflect.TypeOf(s.exec)).Interface().(evmrpc.PendingSnapshot)
	option, err = evmrpc.NewPendingOption(typedNil)
	require.Error(t, err)
	require.Equal(t, evmrpc.PendingOption{}, option)

	option, err = evmrpc.NewPendingOption(s.exec)
	require.NoError(t, err)
	require.NotEqual(t, evmrpc.PendingOption{}, option)
}

func TestNewSequencerForwardOptionRejectsNonHTTPURL(t *testing.T) {
	for _, raw := range []string{
		"", "localhost:8545", "/rpc", "//localhost:8545/rpc",
		"ftp://localhost/rpc", "ws://localhost/rpc", "wss://localhost/rpc",
		"http:", "http:///rpc", "https:///rpc", "http:localhost/rpc",
		"http://:8545/rpc", "http://[::1", "http://localhost:bad/rpc",
		"http://localhost/%zz",
	} {
		t.Run(raw, func(t *testing.T) {
			option, err := evmrpc.NewSequencerForwardOption(raw)
			require.Error(t, err)
			require.Equal(t, evmrpc.SequencerForwardOption{}, option)
		})
	}

	for _, tls := range []bool{false, true} {
		t.Run(map[bool]string{false: "http", true: "https"}[tls], func(t *testing.T) {
			endpoint := httptest.NewUnstartedServer(http.NewServeMux())
			if tls {
				endpoint.StartTLS()
			} else {
				endpoint.Start()
			}
			t.Cleanup(endpoint.Close)
			option, err := evmrpc.NewSequencerForwardOption(endpoint.URL + "/rpc?network=pax")
			require.NoError(t, err)
			require.NotEqual(t, evmrpc.SequencerForwardOption{}, option)
		})
	}
}
