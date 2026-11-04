package eventbus_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/require"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/eventbus"
	tmpubsub "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/pubsub"
	tmquery "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/pubsub/query"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

func TestRepairM_1115_ReservedKeys(t *testing.T) {
	data := executeEventBusApplicationTx(t,
		abci.Event{Type: "tm", Attributes: []abci.EventAttribute{
			{Key: []byte("event"), Value: []byte("NewBlock"), Index: true},
			{Key: []byte("event"), Value: []byte("NewBlockHeader")},
			{Key: []byte("application"), Value: []byte("retained")},
		}},
		abci.Event{Type: "tx", Attributes: []abci.EventAttribute{
			{Key: []byte("height"), Value: []byte("999"), Index: true},
			{Key: []byte("height"), Value: []byte("998")},
			{Key: []byte("hash"), Value: []byte("DEADBEEF"), Index: true},
			{Key: []byte("hash"), Value: []byte("BAD")},
			{Key: []byte("memo"), Value: []byte("first"), Index: true},
			{Key: []byte("memo"), Value: []byte("second")},
		}},
		abci.Event{Type: types.EventTypeKey},
		abci.Event{Type: types.TxHashKey},
		abci.Event{Type: types.TxHeightKey},
	)
	originalResult := proto.Clone(&data.Result).(*abci.ExecTxResult)
	// An application may retain spare capacity for later results. Publishing must
	// not append metadata into that application's backing array.
	backing := make([]abci.Event, len(data.Result.Events)+3)
	copy(backing, data.Result.Events)
	for i := len(data.Result.Events); i < len(backing); i++ {
		backing[i] = abci.Event{Type: "untouched"}
	}
	data.Result.Events = backing[:len(data.Result.Events)]

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	busCtx, stop := context.WithCancel(ctx)
	bus := eventbus.NewDefault()
	require.NoError(t, bus.Start(busCtx))
	t.Cleanup(func() { stop(); bus.Wait() })

	good, err := bus.SubscribeWithArgs(ctx, tmpubsub.SubscribeArgs{
		ClientID: "authoritative",
		Query: tmquery.MustCompile(fmt.Sprintf(
			"tm.event='Tx' AND tx.height=1 AND tx.hash='%X' AND app.key='sender' AND tm.application='retained' AND tx.memo='first' AND tx.memo='second'",
			types.Tx(data.Tx).Hash())),
	})
	require.NoError(t, err)
	queries := []string{
		"tm.event='NewBlock'",
		"tm.event='NewBlockHeader'",
		"tx.height=999",
		"tx.height=998",
		"tx.hash='DEADBEEF'",
		"tx.hash='BAD'",
		"tm.event=''",
		"tx.hash=''",
		"tx.height=''",
	}
	bad := make([]eventbus.Subscription, len(queries))
	for i, query := range queries {
		bad[i], err = bus.SubscribeWithArgs(ctx, tmpubsub.SubscribeArgs{
			ClientID: fmt.Sprintf("incorrect-%d", i),
			Query:    tmquery.MustCompile(query),
		})
		require.NoError(t, err)
	}

	require.NoError(t, bus.PublishEventTx(data))
	// The production pubsub sender drains its queue on context cancellation,
	// then terminates subscriptions. An empty terminated subscription proves
	// non-delivery without a sleep or a timeout standing in for synchronization.
	stop()
	message, err := good.Next(ctx)
	require.NoError(t, err)
	require.Equal(t, data, message.Data())
	require.Equal(t, originalResult, &data.Result)
	for _, event := range backing[len(data.Result.Events):] {
		require.Equal(t, abci.Event{Type: "untouched"}, event)
	}
	values := repairM1115EventValues(message.Events())
	require.Equal(t, []string{types.EventTxValue}, values[types.EventTypeKey])
	require.Equal(t, []string{fmt.Sprintf("%X", types.Tx(data.Tx).Hash())}, values[types.TxHashKey])
	require.Equal(t, []string{"1"}, values[types.TxHeightKey])
	require.Equal(t, []string{"retained"}, values["tm.application"])
	require.Equal(t, []string{"first", "second"}, values["tx.memo"])
	_, err = good.Next(ctx)
	require.ErrorIs(t, err, tmpubsub.ErrTerminated)
	for i, sub := range bad {
		_, err := sub.Next(ctx)
		require.ErrorIs(t, err, tmpubsub.ErrTerminated, "incorrect subscription: %s", queries[i])
	}
}

func TestRepairM_1115_DuplicateApplicationKeys(t *testing.T) {
	transfers := []abci.Event{
		{Type: "transfer", Attributes: []abci.EventAttribute{
			{Key: []byte("sender"), Value: []byte("foo"), Index: true},
			{Key: []byte("sender"), Value: []byte("foo")},
			{Key: []byte("recipient"), Value: []byte("bar")},
			{Key: []byte("amount"), Value: []byte("5")},
		}},
		{Type: "transfer", Attributes: []abci.EventAttribute{
			{Key: []byte("sender"), Value: []byte("baz"), Index: true},
			{Key: []byte("recipient"), Value: []byte("cat")},
			{Key: []byte("amount"), Value: []byte("13")},
		}},
		{Type: "withdraw.rewards", Attributes: []abci.EventAttribute{
			{Key: []byte("amount"), Value: []byte("33"), Index: true},
		}},
	}
	applicationEvents := append([]abci.Event{}, transfers...)
	applicationEvents = append(applicationEvents, abci.Event{Type: "tx", Attributes: []abci.EventAttribute{
		{Key: []byte("height"), Value: []byte("999")},
	}})
	data := executeEventBusApplicationTx(t, applicationEvents...)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	busCtx, stop := context.WithCancel(ctx)
	bus := eventbus.NewDefault()
	require.NoError(t, bus.Start(busCtx))
	t.Cleanup(func() { stop(); bus.Wait() })
	queries := []string{
		"tm.event='Tx' AND tx.height=1 AND transfer.sender='foo'",
		"tm.event='Tx' AND tx.height=1 AND transfer.sender='baz'",
		"tm.event='Tx' AND tx.height=1 AND transfer.sender='foo' AND transfer.sender='baz' AND transfer.amount=5 AND transfer.amount=13",
	}
	subs := make([]eventbus.Subscription, len(queries))
	for i, query := range queries {
		var err error
		subs[i], err = bus.SubscribeWithArgs(ctx, tmpubsub.SubscribeArgs{
			ClientID: fmt.Sprintf("duplicate-%d", i), Query: tmquery.MustCompile(query),
		})
		require.NoError(t, err)
	}
	missing, err := bus.SubscribeWithArgs(ctx, tmpubsub.SubscribeArgs{
		ClientID: "missing", Query: tmquery.MustCompile("transfer.sender='DoesNotExist'"),
	})
	require.NoError(t, err)
	require.NoError(t, bus.PublishEventTx(data))
	stop()
	for i, sub := range subs {
		message, err := sub.Next(ctx)
		require.NoError(t, err, queries[i])
		require.Equal(t, data, message.Data())
		require.Equal(t, transfers, message.Events()[1:1+len(transfers)])
		values := repairM1115EventValues(message.Events())
		require.Equal(t, []string{"foo", "foo", "baz"}, values["transfer.sender"])
		require.Equal(t, []string{"bar", "cat"}, values["transfer.recipient"])
		require.Equal(t, []string{"5", "13"}, values["transfer.amount"])
		require.Equal(t, []string{"33"}, values["withdraw.rewards.amount"])
		require.Equal(t, []string{"1"}, values[types.TxHeightKey])
		_, err = sub.Next(ctx)
		require.ErrorIs(t, err, tmpubsub.ErrTerminated)
	}
	_, err = missing.Next(ctx)
	require.ErrorIs(t, err, tmpubsub.ErrTerminated)
}

func repairM1115EventValues(events []abci.Event) map[string][]string {
	values := make(map[string][]string)
	for _, event := range events {
		for _, attr := range event.Attributes {
			key := event.Type + "." + string(attr.Key)
			values[key] = append(values[key], string(attr.Value))
		}
	}
	return values
}
