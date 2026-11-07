package coretypes_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/eventbus"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/rpc/core"
	tmjson "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/json"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	rpcclient "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/client"
	rpcserver "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/server"
	rpctypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

// Finding 35: use the real bus, Subscribe handler, JSON-RPC WebSocket transport,
// and the same ResultEvent decoder used by the HTTP client's event listener.
func TestRepair_90_SubscriptionDottedNames(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	bus := eventbus.NewDefault()
	require.NoError(t, bus.Start(ctx))
	env := &core.Environment{EventBus: bus, Config: *config.DefaultRPCConfig()}
	manager := rpcserver.NewWebsocketManager(core.NewRoutesMap(env, nil),
		rpcserver.OnDisconnect(func(addr string) { _ = bus.UnsubscribeAll(ctx, addr) }))
	mux := http.NewServeMux()
	mux.HandleFunc("/websocket", manager.WebsocketHandler)
	server := httptest.NewServer(mux)
	defer server.Close()
	client, err := rpcclient.NewWS(server.URL, "/websocket")
	require.NoError(t, err)
	require.NoError(t, client.Start(ctx))
	defer func() {
		cancel()
		require.NoError(t, client.Stop())
	}()

	const query = "tm.event = 'NewBlockHeader'"
	req := rpctypes.NewRequest(90)
	require.NoError(t, req.SetMethodAndParams("subscribe", &coretypes.RequestSubscribe{Query: query}))
	require.NoError(t, client.Send(ctx, req))
	select {
	case ack := <-client.ResponsesCh:
		require.Nil(t, ack.Error)
		require.Equal(t, req.ID(), ack.ID())
		var subscribed coretypes.ResultSubscribe
		require.NoError(t, tmjson.Unmarshal(ack.Result, &subscribed))
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	require.Equal(t, 1, bus.NumClients())

	events := []abci.Event{
		{Type: "transfer.v2", Attributes: []abci.EventAttribute{
			{Key: []byte("account.owner"), Value: []byte("alice.東京"), Index: true},
			{Key: []byte("amount"), Value: []byte("7")},
			{Key: []byte("amount"), Value: []byte("9"), Index: true},
		}},
		{Type: "transfer.v2", Attributes: []abci.EventAttribute{
			{Key: []byte("other.key"), Value: []byte{0xff, 0x00, 0x80}},
		}},
		{Type: "transfer", Attributes: []abci.EventAttribute{
			{Key: []byte("v2.account.owner"), Value: []byte("different grouping")},
		}},
		{Type: "empty.event"},
	}
	want := append(append([]abci.Event{}, events...), types.EventNewBlockHeader)
	hash := crypto.Checksum([]byte("repair-90-header")).Bytes()
	header := types.Header{
		ChainID: "repair-90", Height: 90, Time: time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC),
		LastBlockID:    types.BlockID{Hash: hash, PartSetHeader: types.PartSetHeader{Total: 1, Hash: hash}},
		LastCommitHash: hash, DataHash: hash, ValidatorsHash: hash, NextValidatorsHash: hash,
		ConsensusHash: hash, AppHash: hash, LastResultsHash: hash, EvidenceHash: hash,
		ProposerAddress: hash[:20],
	}
	for range 2 {
		data := types.EventDataNewBlockHeader{Header: header, ResultFinalizeBlock: abci.ResponseFinalizeBlock{Events: events}}
		require.NoError(t, bus.PublishEventNewBlockHeader(data))
		select {
		case response := <-client.ResponsesCh:
			require.Nil(t, response.Error)
			require.Equal(t, req.ID(), response.ID())
			var got coretypes.ResultEvent
			require.NoError(t, tmjson.Unmarshal(response.Result, &got))
			require.Equal(t, query, got.Query)
			require.Equal(t, want, got.Events)
			require.Equal(t, data.ToLegacy(), got.Data)
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}

	unsubscribe := rpctypes.NewRequest(91)
	require.NoError(t, unsubscribe.SetMethodAndParams("unsubscribe", &coretypes.RequestUnsubscribe{Query: query}))
	require.NoError(t, client.Send(ctx, unsubscribe))
	select {
	case ack := <-client.ResponsesCh:
		require.Nil(t, ack.Error)
		require.Equal(t, unsubscribe.ID(), ack.ID())
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	require.Zero(t, bus.NumClients())
}

func TestRepair_90_EventEncodingCompatibility(t *testing.T) {
	want := coretypes.ResultEvent{
		SubscriptionID: "subscription-90", Query: "transfer.amount = '7'", Data: types.EventDataString("payload"),
		Events: []abci.Event{{Type: "transfer", Attributes: []abci.EventAttribute{
			{Key: []byte("amount"), Value: []byte("7")},
		}}},
	}
	raw, err := json.Marshal(want)
	require.NoError(t, err)
	var legacy struct {
		Events map[string][]string `json:"events"`
	}
	require.NoError(t, json.Unmarshal(raw, &legacy))
	require.Equal(t, map[string][]string{"transfer.amount": {"7"}}, legacy.Events)
	var got coretypes.ResultEvent
	require.NoError(t, tmjson.Unmarshal(raw, &got))
	require.Equal(t, want, got)

	// Removing the extension models a response from an older producer, retaining
	// the actual legacy encoding emitted by MarshalJSON.
	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &fields))
	delete(fields, "event_groups")
	oldWire, err := json.Marshal(fields)
	require.NoError(t, err)
	require.NoError(t, tmjson.Unmarshal(oldWire, &got))
	require.Equal(t, want, got)

	for _, events := range [][]abci.Event{nil, {}, {{Type: "no.attributes"}}} {
		want.Events = events
		wire, err := json.Marshal(want)
		require.NoError(t, err)
		require.NoError(t, tmjson.Unmarshal(wire, &got))
		require.Equal(t, want, got)
	}
}

func TestRepair_90_RejectUnsupportedEventFormat(t *testing.T) {
	raw, err := json.Marshal(coretypes.ResultEvent{
		Data: types.EventDataString("payload"),
		Events: []abci.Event{{Type: "transfer", Attributes: []abci.EventAttribute{
			{Key: []byte("amount"), Value: []byte("7")},
		}}},
	})
	require.NoError(t, err)
	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &fields))
	for _, groups := range []string{
		`{"version":2,"events":[]}`,
		`{"version":0,"events":[]}`,
		`{"version":1,"events":{}}`,
	} {
		fields["event_groups"] = json.RawMessage(groups)
		wire, err := json.Marshal(fields)
		require.NoError(t, err)
		var got coretypes.ResultEvent
		require.Error(t, tmjson.Unmarshal(wire, &got))
		require.Nil(t, got.Events, "invalid extensions must not fall back to legacy events")
	}
}
