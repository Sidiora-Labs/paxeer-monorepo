package sequencer_test

import (
	"crypto/ed25519"
	"math/big"
	"testing"

	seqkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/keeper"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/ethereum/go-ethereum/common"
	gethrpc "github.com/ethereum/go-ethereum/rpc"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestFollowerStartReceiptRPC(t *testing.T) {
	admissionChainID(t, 125)
	h := newRPCE2EHarness(t)
	tx, receipt := h.send(t, 0, utils.NewSigner().EvmAddress, big.NewInt(1))
	rpcE2EVerifyReceipt(t, h.node.App, receipt)

	for _, tc := range []struct {
		name        string
		forward     bool
		otherSigner bool
		wantError   string
	}{
		{name: "forwarded receipt", forward: true},
		{name: "signer differs from local pin", forward: true, otherSigner: true,
			wantError: "sequencer receipt: JSON view differs from receipt bytes: signingKey differs from the pinned receipt key"},
		{name: "no forward URL", wantError: "pax_sequencerReceipt served by sequencer"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			port := rpcE2EPort(t)
			follower := newRPCE2EApp(t, h.sender, h.genesis, port)
			if tc.otherSigner {
				public, _, err := ed25519.GenerateKey(nil)
				require.NoError(t, err)
				require.NotEqual(t, []byte(receipt.SigningKey), []byte(public))
				require.NoError(t, follower.SequencerKeeper.SetReceiptKeys(follower.GetCheckCtx(), seqkeeper.ReceiptKeys{{
					KeyId: testKeyID[:], PublicKey: public, FirstHeight: 1, LastHeight: 1000,
				}}))
			}
			opts := viper.New()
			opts.Set(sequencer.FlagEnable, false)
			if tc.forward {
				opts.Set("sequencer.receipt-forward-url", h.url)
			}
			cfg, err := sequencer.ReadConfig(opts)
			require.NoError(t, err)
			n, err := sequencer.NewNode(cfg, follower)
			require.NoError(t, err)
			require.Nil(t, n)
			require.Nil(t, follower.SequencerSource())
			_, err = follower.PendingSnapshot()
			require.ErrorIs(t, err, app.ErrPendingServedBySequencer)
			client, _ := rpcE2EClient(t, h.ctx, follower, h.consensus, port)
			for repeat := 0; repeat < 2; repeat++ {
				var forwarded *evmrpc.SequencerReceiptResult
				err = client.CallContext(h.ctx, &forwarded, evmrpc.SequencerReceiptMethod, tx.Hash())
				if tc.wantError != "" {
					require.EqualError(t, err, tc.wantError)
					var rpcError gethrpc.Error
					require.ErrorAs(t, err, &rpcError)
					require.Equal(t, -32000, rpcError.ErrorCode())
					require.Nil(t, forwarded)
					continue
				}
				require.NoError(t, err)
				require.Equal(t, receipt, forwarded)
				rpcE2EVerifyReceipt(t, follower, forwarded)
			}
			if tc.forward {
				var unknown *evmrpc.SequencerReceiptResult
				require.NoError(t, client.CallContext(h.ctx, &unknown, evmrpc.SequencerReceiptMethod, common.HexToHash("0xdeadbeef")))
				require.Nil(t, unknown)
			}
			require.Equal(t, committedTip, follower.LastBlockHeight())
			require.Zero(t, follower.EvmKeeper.GetNonce(follower.GetCheckCtx(), h.sender.EvmAddress))
		})
	}
}

func TestFollowerStartRejectsForwardURLWhenSequencingEnabled(t *testing.T) {
	cfg := sequencer.DefaultConfig()
	cfg.Enable = true
	cfg.ReceiptForwardURL = "https://sequencer.example/receipts"
	n, err := sequencer.NewNode(cfg, nil)
	require.Nil(t, n)
	require.ErrorIs(t, err, sequencer.ErrConfigValue)
	require.EqualError(t, err,
		"sequencer config: invalid value: sequencer.receipt-forward-url cannot be set when sequencer.enable is true")
}

func TestFollowerStartRejectsInvalidForwardURL(t *testing.T) {
	for _, endpoint := range []string{"ftp://sequencer.example/receipts", "/receipts", "https:///receipts"} {
		t.Run(endpoint, func(t *testing.T) {
			cfg := sequencer.DefaultConfig()
			cfg.ReceiptForwardURL = endpoint
			require.PanicsWithError(t,
				"sequencer config: invalid value: sequencer.receipt-forward-url must be an http or https URL with a host",
				func() { _, _ = sequencer.NewNode(cfg, nil) })
		})
	}
	cfg := sequencer.DefaultConfig()
	cfg.ReceiptForwardURL = "http://[::1"
	require.PanicsWithError(t,
		`sequencer config: invalid value: sequencer.receipt-forward-url: parse "http://[::1": missing ']' in host`,
		func() { _, _ = sequencer.NewNode(cfg, nil) })
}

func TestFollowerStartWithoutForwardURLNeedsNoApp(t *testing.T) {
	n, err := sequencer.NewNode(sequencer.DefaultConfig(), nil)
	require.NoError(t, err)
	require.Nil(t, n)
}

func TestFollowerStartWithForwardURLNeedsApp(t *testing.T) {
	cfg := sequencer.DefaultConfig()
	cfg.ReceiptForwardURL = "https://sequencer.example/receipts"
	n, err := sequencer.NewNode(cfg, nil)
	require.Nil(t, n)
	require.ErrorIs(t, err, sequencer.ErrConfigValue)
	require.EqualError(t, err, "sequencer config: invalid value: sequencer needs an app")
}
