package rpc_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/ed25519"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/inspect/rpc"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/state"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/store"
	tmjson "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/json"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	rpctypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

func TestRepairM_1097_ConnectReadDisconnect(t *testing.T) {
	repairM1097Disconnect(t, true)
}

func TestRepairM_1097_AbruptDisconnect(t *testing.T) {
	repairM1097Disconnect(t, false)
}

func repairM1097Disconnect(t *testing.T, graceful bool) {
	t.Helper()
	states := state.NewStore(dbm.NewMemDB())
	t.Cleanup(func() { require.NoError(t, states.Close()) })
	blocks := store.NewBlockStore(dbm.NewMemDB())
	t.Cleanup(func() { require.NoError(t, blocks.Close()) })
	genesis := &types.GenesisDoc{
		ChainID:         "repair-m1097",
		GenesisTime:     time.Unix(1, 0).UTC(),
		ConsensusParams: types.DefaultConsensusParams(),
		Validators: []types.GenesisValidator{{
			PubKey: ed25519.TestSecretKey([]byte("repair-m1097-validator")).Public(),
			Power:  10,
		}},
	}
	genesis.ConsensusParams.Block.MaxBytes = 1234567
	initial, err := state.MakeGenesisState(genesis)
	require.NoError(t, err)
	require.NoError(t, states.Save(initial))
	persisted, err := states.Load()
	require.NoError(t, err)
	want := initial.ConsensusParams
	want.Synchrony = want.Synchrony.SynchronyParamsOrDefaults()
	want.Timeout = want.Timeout.TimeoutParamsOrDefaults()

	cfg := config.DefaultRPCConfig()
	handler := rpc.Handler(cfg, rpc.Routes(*cfg, states, blocks, nil))
	finished := make(chan any, 2)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/websocket" {
			// Observe the real handler's return, including its final Stop call.
			defer func() { finished <- recover() }()
		}
		handler.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	client := srv.Client()
	client.Timeout = 5 * time.Second
	dialer := websocket.Dialer{HandshakeTimeout: 5 * time.Second}

	checkResult := func(payload json.RawMessage) {
		t.Helper()
		var result coretypes.ResultConsensusParams
		require.NoError(t, tmjson.Unmarshal(payload, &result))
		require.Equal(t, int64(1), result.BlockHeight)
		require.Equal(t, want, result.ConsensusParams)
	}

	// Reconnect to the same inspector after the first disconnect.
	for round := range 2 {
		conn, response, err := dialer.DialContext(t.Context(), "ws"+strings.TrimPrefix(srv.URL, "http")+"/websocket", nil)
		require.NoError(t, err)
		t.Cleanup(func() { _ = conn.Close() })
		require.Equal(t, http.StatusSwitchingProtocols, response.StatusCode)
		require.NoError(t, response.Body.Close())
		require.NoError(t, conn.SetReadDeadline(time.Now().Add(5*time.Second)))
		require.NoError(t, conn.SetWriteDeadline(time.Now().Add(5*time.Second)))
		request := rpctypes.NewRequest(round + 1)
		require.NoError(t, request.SetMethodAndParams("consensus_params", &coretypes.RequestConsensusParams{}))
		require.NoError(t, conn.WriteJSON(request))
		var result rpctypes.RPCResponse
		require.NoError(t, conn.ReadJSON(&result))
		require.Equal(t, request.ID(), result.ID())
		require.Nil(t, result.Error)
		checkResult(result.Result)

		if graceful {
			require.NoError(t, conn.WriteControl(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Now().Add(5*time.Second)))
			_, _, err = conn.ReadMessage()
			require.True(t, websocket.IsCloseError(err, websocket.CloseNormalClosure), "close handshake: %v", err)
		}
		require.NoError(t, conn.Close())
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		select {
		case panicValue := <-finished:
			require.Nil(t, panicValue, "inspector panicked on disconnect")
		case <-ctx.Done():
			cancel()
			t.Fatal("inspector websocket handler did not stop after disconnect")
		}
		cancel()

		httpRequest, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/consensus_params", nil)
		require.NoError(t, err)
		httpResponse, err := client.Do(httpRequest)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, httpResponse.StatusCode)
		require.Equal(t, "application/json", httpResponse.Header.Get("Content-Type"))
		var httpResult json.RawMessage
		err = json.NewDecoder(httpResponse.Body).Decode(&httpResult)
		require.NoError(t, httpResponse.Body.Close())
		require.NoError(t, err)
		checkResult(httpResult)
	}
	loaded, err := states.Load()
	require.NoError(t, err)
	require.Equal(t, persisted, loaded)
	require.Zero(t, blocks.Height())
}
