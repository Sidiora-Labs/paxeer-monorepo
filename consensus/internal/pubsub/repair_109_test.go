package pubsub_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/eventbus"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/pubsub"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/pubsub/query"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/rpc/core"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	rpcserver "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/server"
	rpctypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

func TestRepair_109_WebsocketUnsubscribeID(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	bus := eventbus.NewDefault()
	require.NoError(t, bus.Start(ctx))
	t.Cleanup(bus.Wait)
	env := &core.Environment{EventBus: bus}
	manager := rpcserver.NewWebsocketManager(map[string]*rpcserver.RPCFunc{
		"unsubscribe":     rpcserver.NewWSRPCFunc(env.Unsubscribe),
		"unsubscribe_all": rpcserver.NewWSRPCFunc(env.UnsubscribeAll),
	})
	server := httptest.NewServer(http.HandlerFunc(manager.WebsocketHandler))
	t.Cleanup(server.Close)
	conn, response, err := websocket.DefaultDialer.DialContext(ctx, "ws"+strings.TrimPrefix(server.URL, "http"), nil)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
	t.Cleanup(func() { _ = conn.Close() })
	deadline, ok := ctx.Deadline()
	require.True(t, ok)
	require.NoError(t, conn.SetReadDeadline(deadline))
	require.NoError(t, conn.SetWriteDeadline(deadline))
	client := conn.LocalAddr().String()
	selected, err := bus.SubscribeWithArgs(ctx, pubsub.SubscribeArgs{
		ClientID: client, Query: query.MustCompile("tm.event='NewBlock'"),
	})
	require.NoError(t, err)
	survivor, err := bus.SubscribeWithArgs(ctx, pubsub.SubscribeArgs{
		ClientID: client, Query: query.MustCompile("tm.event EXISTS"),
	})
	require.NoError(t, err)
	foreign, err := bus.SubscribeWithArgs(ctx, pubsub.SubscribeArgs{
		ClientID: "another-client", Query: query.All,
	})
	require.NoError(t, err)

	readEvent := func(sub eventbus.Subscription, data types.EventDataString) pubsub.Message {
		t.Helper()
		msg, err := sub.Next(ctx)
		require.NoError(t, err)
		require.Equal(t, data, msg.Data())
		require.Equal(t, sub.ID(), msg.SubscriptionID())
		return msg
	}
	require.NoError(t, bus.Publish(types.EventNewBlockValue, types.EventDataString("before")))
	selectedID := readEvent(selected, "before").SubscriptionID()
	readEvent(survivor, "before")
	foreignID := readEvent(foreign, "before").SubscriptionID()

	requestID := 0
	call := func(method string, params any) rpctypes.RPCResponse {
		t.Helper()
		requestID++
		req := rpctypes.NewRequest(requestID)
		require.NoError(t, req.SetMethodAndParams(method, params))
		require.NoError(t, conn.WriteJSON(req))
		var resp rpctypes.RPCResponse
		require.NoError(t, conn.ReadJSON(&resp))
		require.Equal(t, req.ID(), resp.ID())
		return resp
	}
	unsubscribe := func(id string) rpctypes.RPCResponse {
		return call("unsubscribe", &coretypes.RequestUnsubscribe{Query: id})
	}
	assertLive := func(data types.EventDataString, subs ...eventbus.Subscription) {
		t.Helper()
		require.NoError(t, bus.Publish(types.EventNewBlockValue, data))
		for _, sub := range subs {
			readEvent(sub, data)
		}
	}

	require.NotNil(t, unsubscribe("unknown-subscription-id").Error)
	require.Equal(t, 2, bus.NumClientSubscriptions(client))
	assertLive("after unknown ID", selected, survivor, foreign)
	require.NotNil(t, unsubscribe(foreignID).Error)
	require.Equal(t, 2, bus.NumClientSubscriptions(client))
	require.Equal(t, 1, bus.NumClientSubscriptions("another-client"))
	assertLive("after foreign ID", selected, survivor, foreign)

	require.Nil(t, unsubscribe(selectedID).Error)
	require.Equal(t, 1, bus.NumClientSubscriptions(client))
	_, err = selected.Next(ctx)
	require.ErrorIs(t, err, pubsub.ErrUnsubscribed)
	assertLive("after selected ID", survivor, foreign)
	require.NotNil(t, unsubscribe(selectedID).Error)
	require.Equal(t, 1, bus.NumClientSubscriptions(client))
	assertLive("after stale ID", survivor, foreign)

	require.Nil(t, call("unsubscribe_all", nil).Error)
	require.Zero(t, bus.NumClientSubscriptions(client))
	_, err = survivor.Next(ctx)
	require.ErrorIs(t, err, pubsub.ErrUnsubscribed)
	assertLive("after unsubscribe all", foreign)
}

func TestRepair_109_UnsubscribeSelectors(t *testing.T) {
	for _, tc := range []struct {
		name       string
		id         string
		query      string
		subscriber string
		removed    bool
		notFound   bool
	}{
		{name: "ID and matching query", id: "selected", query: "selected", subscriber: "owner", removed: true},
		{name: "ID and different query", id: "selected", query: "other", subscriber: "owner", notFound: true},
		{name: "unknown ID and matching query", id: "unknown", query: "selected", subscriber: "owner", notFound: true},
		{name: "foreign ID and matching query", id: "foreign", query: "selected", subscriber: "owner", notFound: true},
		{name: "query only", query: "selected", subscriber: "owner", removed: true},
		{name: "subscriber only", subscriber: "owner"},
		{name: "missing subscriber", id: "selected", query: "selected"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			s := newTestServer(ctx, t)
			selectedQuery := query.MustCompile("tm.event='NewBlock'")
			otherQuery := query.MustCompile("tm.event EXISTS")
			subscribe := func(client string, q *query.Query) *pubsub.Subscription {
				sub, err := s.SubscribeWithArgs(ctx, pubsub.SubscribeArgs{ClientID: client, Query: q})
				require.NoError(t, err)
				return sub
			}
			selected := subscribe("owner", selectedQuery)
			other := subscribe("owner", otherQuery)
			foreign := subscribe("another-owner", selectedQuery)
			args := pubsub.UnsubscribeArgs{
				Subscriber: tc.subscriber,
				ID:         map[string]string{"selected": selected.ID(), "foreign": foreign.ID(), "unknown": "unknown-id"}[tc.id],
				Query:      map[string]*query.Query{"selected": selectedQuery, "other": otherQuery}[tc.query],
			}
			err := s.Unsubscribe(ctx, args)
			if tc.removed {
				require.NoError(t, err)
				require.Equal(t, 1, s.NumClientSubscriptions("owner"))
				_, err = selected.Next(ctx)
				require.ErrorIs(t, err, pubsub.ErrUnsubscribed)
			} else {
				require.Error(t, err)
				if tc.notFound {
					require.ErrorIs(t, err, pubsub.ErrSubscriptionNotFound)
				}
				require.Equal(t, 2, s.NumClientSubscriptions("owner"))
			}
			require.Equal(t, 1, s.NumClientSubscriptions("another-owner"))
			events := []abci.Event{{Type: "tm", Attributes: []abci.EventAttribute{{Key: []byte("event"), Value: []byte("NewBlock")}}}}
			data := types.EventDataString("after unsubscribe")
			require.NoError(t, s.PublishWithEvents(data, events))
			live := []*pubsub.Subscription{other, foreign}
			if !tc.removed {
				live = append(live, selected)
			}
			for _, sub := range live {
				msg, err := sub.Next(ctx)
				require.NoError(t, err)
				require.Equal(t, data, msg.Data())
				require.Equal(t, sub.ID(), msg.SubscriptionID())
			}
			if tc.removed {
				replacement := subscribe("owner", selectedQuery)
				require.NotEqual(t, selected.ID(), replacement.ID())
				require.ErrorIs(t, s.Unsubscribe(ctx, pubsub.UnsubscribeArgs{Subscriber: "owner", ID: selected.ID()}), pubsub.ErrSubscriptionNotFound)
				require.Equal(t, 2, s.NumClientSubscriptions("owner"))
				require.NoError(t, s.Unsubscribe(ctx, pubsub.UnsubscribeArgs{Subscriber: "owner", ID: replacement.ID()}))
				_, err = replacement.Next(ctx)
				require.ErrorIs(t, err, pubsub.ErrUnsubscribed)
			}
		})
	}
}

func TestRepair_109_UnsubscribeAllQuery(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	s := newTestServer(ctx, t)
	all, err := s.SubscribeWithArgs(ctx, pubsub.SubscribeArgs{ClientID: "owner", Query: query.All})
	require.NoError(t, err)
	filtered, err := s.SubscribeWithArgs(ctx, pubsub.SubscribeArgs{
		ClientID: "owner", Query: query.MustCompile("tm.event EXISTS"),
	})
	require.NoError(t, err)
	foreign, err := s.SubscribeWithArgs(ctx, pubsub.SubscribeArgs{ClientID: "another-owner", Query: query.All})
	require.NoError(t, err)
	require.NoError(t, s.Unsubscribe(ctx, pubsub.UnsubscribeArgs{Subscriber: "owner", Query: query.All}))
	require.Equal(t, 1, s.NumClientSubscriptions("owner"))
	require.Equal(t, 1, s.NumClientSubscriptions("another-owner"))
	_, err = all.Next(ctx)
	require.ErrorIs(t, err, pubsub.ErrUnsubscribed)
	require.ErrorIs(t, s.Unsubscribe(ctx, pubsub.UnsubscribeArgs{Subscriber: "owner", Query: query.All}), pubsub.ErrSubscriptionNotFound)
	data := types.EventDataString("after removing all-events query")
	events := []abci.Event{{Type: "tm", Attributes: []abci.EventAttribute{{Key: []byte("event"), Value: []byte("NewBlock")}}}}
	require.NoError(t, s.PublishWithEvents(data, events))
	for _, sub := range []*pubsub.Subscription{filtered, foreign} {
		msg, err := sub.Next(ctx)
		require.NoError(t, err)
		require.Equal(t, data, msg.Data())
		require.Equal(t, sub.ID(), msg.SubscriptionID())
	}
	require.NoError(t, s.UnsubscribeAll(ctx, "owner"))
	_, err = filtered.Next(ctx)
	require.ErrorIs(t, err, pubsub.ErrUnsubscribed)
}
