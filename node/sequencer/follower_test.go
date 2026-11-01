package sequencer_test

import (
	"net/url"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	"github.com/stretchr/testify/require"
)

func TestFollowerReceiptOptionForwards(t *testing.T) {
	for _, endpoint := range []string{
		"http://127.0.0.1:8545",
		"https://sequencer.example/receipts?network=paxeer",
	} {
		t.Run(endpoint, func(t *testing.T) {
			cfg := sequencer.DefaultConfig()
			cfg.ReceiptForwardURL = endpoint
			option, ok := sequencer.NewFollowerReceiptOption(cfg)
			require.True(t, ok)
			expected, err := url.Parse(endpoint)
			require.NoError(t, err)
			require.Equal(t, evmrpc.SequencerReceiptOption{ForwardURL: expected}, option)
			require.Equal(t, endpoint, option.ForwardURL.String())
		})
	}

	for _, tc := range []struct {
		name    string
		enabled bool
		url     string
	}{
		{name: "unconfigured follower"},
		{name: "sequencer", enabled: true},
		{name: "sequencer with forward URL", enabled: true, url: "https://sequencer.example"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := sequencer.DefaultConfig()
			cfg.Enable = tc.enabled
			cfg.ReceiptForwardURL = tc.url
			option, ok := sequencer.NewFollowerReceiptOption(cfg)
			require.False(t, ok)
			require.Equal(t, evmrpc.SequencerReceiptOption{}, option)
		})
	}
}

func TestFollowerReceiptOptionRefusesBadScheme(t *testing.T) {
	for _, endpoint := range []string{
		"ftp://sequencer.example/receipts",
		"ws://sequencer.example/receipts",
		"wss://sequencer.example/receipts",
		"file:///receipts",
		"sequencer.example/receipts",
		"//sequencer.example/receipts",
		"/receipts",
		"https:///receipts",
		"http:receipts",
		"http://[::1",
		"https://sequencer.example/%zz",
	} {
		t.Run(endpoint, func(t *testing.T) {
			cfg := sequencer.DefaultConfig()
			cfg.ReceiptForwardURL = endpoint
			defer func() {
				failure := recover()
				require.NotNil(t, failure, "invalid forward URL must panic")
				err, ok := failure.(error)
				require.True(t, ok, "panic must carry an error, got %T", failure)
				require.ErrorIs(t, err, sequencer.ErrConfigValue)
			}()
			sequencer.NewFollowerReceiptOption(cfg)
		})
	}
}
