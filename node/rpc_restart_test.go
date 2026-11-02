package app_test

import (
	"context"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
)

func freeTCPPort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := l.Addr().(*net.TCPAddr).Port
	require.NoError(t, l.Close())
	return port
}

func newRestartTestHTTPServer(t *testing.T, port int) *evmrpc.HTTPServer {
	t.Helper()
	srv := evmrpc.NewHTTPServer(rpc.HTTPTimeouts{})
	require.NoError(t, srv.SetListenAddr("127.0.0.1", port))
	require.NoError(t, srv.EnableRPC([]rpc.API{{Namespace: "echo", Service: evmrpc.NewEchoAPI()}}, evmrpc.HTTPConfig{Vhosts: []string{"*"}}))
	return srv
}

func newRestartTestWSServer(t *testing.T, port int) *evmrpc.HTTPServer {
	t.Helper()
	srv := evmrpc.NewHTTPServer(rpc.HTTPTimeouts{})
	require.NoError(t, srv.SetListenAddr("127.0.0.1", port))
	require.NoError(t, srv.EnableWS([]rpc.API{{Namespace: "echo", Service: evmrpc.NewEchoAPI()}}, evmrpc.WsConfig{Origins: []string{"*"}}))
	return srv
}

func portAccepts(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 100*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func portFree(port int) bool {
	l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return false
	}
	_ = l.Close()
	return true
}

func TestAppCloseStopsEVMRPCServers(t *testing.T) {
	httpPort := freeTCPPort(t)
	wsPort := freeTCPPort(t)

	first := app.Setup(t, false, false, false)
	first.ServeEVMServers(newRestartTestHTTPServer(t, httpPort), newRestartTestWSServer(t, wsPort))
	first.SendEVMServerStartSignals()
	require.Eventually(t, func() bool { return portAccepts(httpPort) }, 5*time.Second, 20*time.Millisecond)
	require.Eventually(t, func() bool { return portAccepts(wsPort) }, 5*time.Second, 20*time.Millisecond)

	require.NoError(t, first.Close())
	require.True(t, portFree(httpPort))
	require.True(t, portFree(wsPort))

	second := app.Setup(t, false, false, false)
	second.ServeEVMServers(newRestartTestHTTPServer(t, httpPort), newRestartTestWSServer(t, wsPort))
	second.SendEVMServerStartSignals()
	require.Eventually(t, func() bool { return portAccepts(httpPort) }, 5*time.Second, 20*time.Millisecond)
	require.Eventually(t, func() bool { return portAccepts(wsPort) }, 5*time.Second, 20*time.Millisecond)
	require.NoError(t, second.Close())
	require.True(t, portFree(httpPort))
	require.True(t, portFree(wsPort))

	third := app.Setup(t, false, false, false)
	third.ServeEVMServers(newRestartTestHTTPServer(t, httpPort), newRestartTestWSServer(t, wsPort))
	require.NoError(t, third.Close())
	third.SendEVMServerStartSignals()
	require.Never(t, func() bool { return portAccepts(httpPort) || portAccepts(wsPort) }, 500*time.Millisecond, 20*time.Millisecond)
	require.True(t, portFree(httpPort))
	require.True(t, portFree(wsPort))
}

func TestSealedFinalizeStartsEVMRPCServers(t *testing.T) {
	at := time.Now().UTC().Truncate(time.Millisecond)
	proposer := secp256k1.GenPrivKey().PubKey()
	w := newSequencerWrapper(t, at, proposer, false)
	a := w.App
	closed := false
	t.Cleanup(func() {
		if !closed {
			require.NoError(t, a.Close())
		}
	})
	require.NoError(t, a.SequencerKeeper.SetActivationHeight(w.Ctx, 2))
	activate(t, w)
	require.NoError(t, a.SetSealedStateToCommit(w.Ctx, w.Ctx.MultiStore().(sdk.CacheMultiStore)))
	_, err := a.Commit(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(1), a.LastBlockHeight())

	exec, err := sequencer.NewExecutor(a, sequencer.Options{ChainID: sequencedChainID, ProposerAddress: proposer.Address()})
	require.NoError(t, err)
	dir := t.TempDir()
	consensusKey := filepath.Join(dir, "consensus.json")
	pv, err := privval.GenFilePV(consensusKey, filepath.Join(dir, "consensus-state.json"), "")
	require.NoError(t, err)
	require.NoError(t, pv.Save())
	receiptKey := filepath.Join(dir, "receipt-key")
	require.NoError(t, os.WriteFile(receiptKey, []byte(hex.EncodeToString(sequencerKeySeed)), 0o600))
	pins, err := a.SequencerKeeper.ReceiptKeyPins(a.GetCheckCtx())
	require.NoError(t, err)
	key, err := sequencer.LoadReceiptKey(receiptKey, consensusKey, pins, 2)
	require.NoError(t, err)
	source, err := sequencer.NewConsensusSource(a, exec, key)
	require.NoError(t, err)
	a.SetSequencerSource(source)
	at = at.Add(time.Second)
	require.NoError(t, exec.Open(2, at.UnixMilli()))
	_, err = source.Seal(context.Background(), 2)
	require.NoError(t, err)
	sealed := source.Sealed()
	require.NotNil(t, sealed)

	httpPort := freeTCPPort(t)
	wsPort := freeTCPPort(t)
	a.ServeEVMServers(newRestartTestHTTPServer(t, httpPort), newRestartTestWSServer(t, wsPort))
	require.Never(t, func() bool { return portAccepts(httpPort) || portAccepts(wsPort) }, 100*time.Millisecond, 20*time.Millisecond)
	resp, err := a.FinalizeBlocker(a.GetCheckCtx(), &abci.RequestFinalizeBlock{
		Hash: []byte(t.Name()), Txs: sealed.Txs,
		Header: &tmproto.Header{ChainID: sequencedChainID, Height: 2, Time: at, ProposerAddress: proposer.Address()},
	})
	require.NoError(t, err)
	require.Len(t, resp.TxResults, len(sealed.Results))
	require.Same(t, sealed.Results[0], resp.TxResults[0])
	require.Eventually(t, func() bool { return portAccepts(httpPort) }, 5*time.Second, 20*time.Millisecond)
	require.Eventually(t, func() bool { return portAccepts(wsPort) }, 5*time.Second, 20*time.Millisecond)
	require.NoError(t, a.Close())
	closed = true
	require.True(t, portFree(httpPort))
	require.True(t, portFree(wsPort))
	t.Run("CloseStopsEVMRPCServers", TestAppCloseStopsEVMRPCServers)
}
