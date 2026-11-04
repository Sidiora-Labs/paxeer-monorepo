package pubsub_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/pubsub"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/pubsub/query"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

func repairM1161Subscriptions(t *testing.T, ctx context.Context, server *pubsub.Server) []*pubsub.Subscription {
	t.Helper()
	var subs []*pubsub.Subscription
	for _, args := range []pubsub.SubscribeArgs{
		{ClientID: "alice", Query: query.All},
		{ClientID: "alice", Query: query.MustCompile("tm.event='NewBlock'")},
		{ClientID: "bob", Query: query.All},
	} {
		sub, err := server.SubscribeWithArgs(ctx, args)
		require.NoError(t, err)
		subs = append(subs, sub)
	}
	return subs
}

func repairM1161Deliver(t *testing.T, ctx context.Context, subs []*pubsub.Subscription) {
	t.Helper()
	for _, sub := range subs {
		msg, err := sub.Next(ctx)
		require.NoError(t, err)
		require.Equal(t, types.EventDataString("lifecycle"), msg.Data())
		require.Equal(t, sub.ID(), msg.SubscriptionID())
	}
}

func repairM1161Publish(server *pubsub.Server) error {
	return server.PublishWithEvents(types.EventDataString("lifecycle"), []abci.Event{{
		Type:       "tm",
		Attributes: []abci.EventAttribute{{Key: []byte("event"), Value: []byte("NewBlock")}},
	}})
}

func TestRepairM_1161_CountsAfterShutdown(t *testing.T) {
	for _, populated := range []bool{false, true} {
		name := "empty"
		if populated {
			name = "populated"
		}
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			server := pubsub.NewServer()
			require.Zero(t, server.NumClients())
			require.Zero(t, server.NumClientSubscriptions("alice"))
			require.NoError(t, server.Start(ctx))
			t.Cleanup(func() { cancel(); server.Wait() })

			var subs []*pubsub.Subscription
			if populated {
				subs = repairM1161Subscriptions(t, ctx, server)
				require.Equal(t, 2, server.NumClients())
				require.Equal(t, 2, server.NumClientSubscriptions("alice"))
				require.Equal(t, 1, server.NumClientSubscriptions("bob"))
				require.NoError(t, repairM1161Publish(server))
				repairM1161Deliver(t, ctx, subs)
			}
			require.Zero(t, server.NumClientSubscriptions("unknown"))

			cancel()
			server.Wait()
			require.Zero(t, server.NumClients())
			for _, client := range []string{"alice", "bob", "unknown", ""} {
				require.Zero(t, server.NumClientSubscriptions(client))
			}
			for _, sub := range subs {
				_, err := sub.Next(t.Context())
				require.ErrorIs(t, err, pubsub.ErrTerminated)
			}
		})
	}
}

func TestRepairM_1161_CountsDuringShutdown(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	server := pubsub.NewServer(pubsub.BufferCapacity(1))
	require.NoError(t, server.Start(ctx))
	subs := repairM1161Subscriptions(t, ctx, server)

	observing := make(chan struct{})
	release := make(chan struct{})
	t.Cleanup(func() { close(release); cancel(); server.Wait() })
	finishDelivery := make(chan struct{})
	require.NoError(t, server.Observe(ctx, func(pubsub.Message) error {
		close(observing)
		select {
		case <-finishDelivery:
		case <-release:
		}
		return nil
	}))
	require.NoError(t, repairM1161Publish(server))
	select {
	case <-observing:
	case <-ctx.Done():
		t.Fatal("publication did not reach the observer")
	}

	cancel()
	stopped := make(chan struct{})
	go func() { server.Wait(); close(stopped) }()
	select {
	case <-stopped:
		t.Fatal("shutdown finished before the in-flight publication drained")
	default:
	}
	require.Equal(t, 2, server.NumClients())
	require.Equal(t, 2, server.NumClientSubscriptions("alice"))
	require.Equal(t, 1, server.NumClientSubscriptions("bob"))
	require.Zero(t, server.NumClientSubscriptions("unknown"))

	type counter struct {
		name string
		read func() int
		live int
	}
	counters := []counter{
		{"clients", server.NumClients, 2},
		{"alice", func() int { return server.NumClientSubscriptions("alice") }, 2},
		{"bob", func() int { return server.NumClientSubscriptions("bob") }, 1},
		{"unknown", func() int { return server.NumClientSubscriptions("unknown") }, 0},
	}
	ready := make(chan struct{}, len(counters))
	start := make(chan struct{})
	readersDone := make(chan struct{}, len(counters))
	for _, c := range counters {
		go func() {
			defer func() { readersDone <- struct{}{} }()
			if got := c.read(); got != c.live {
				t.Errorf("%s during pending shutdown = %d, want %d", c.name, got, c.live)
			}
			ready <- struct{}{}
			<-start
			if got := c.read(); got != c.live && got != 0 {
				t.Errorf("%s across shutdown = %d, want %d or 0", c.name, got, c.live)
			}
			<-stopped
			if got := c.read(); got != 0 {
				t.Errorf("%s after shutdown = %d, want 0", c.name, got)
			}
		}()
	}
	for range counters {
		<-ready
	}
	close(start)
	close(finishDelivery)
	for range counters {
		<-readersDone
	}
	repairM1161Deliver(t, t.Context(), subs)
	for _, sub := range subs {
		_, err := sub.Next(t.Context())
		require.ErrorIs(t, err, pubsub.ErrTerminated)
	}
}
