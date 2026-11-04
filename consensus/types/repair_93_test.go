package types_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/example/kvstore"
	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/eventbus"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/pubsub"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/rpc/core"
	tmjson "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/json"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	rpcclient "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/client"
	rpcserver "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/server"
	rpctypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

func TestRepair_93_FailedTransactionSubscription(t *testing.T) {
	app := kvstore.NewApplication()
	t.Cleanup(func() { require.NoError(t, app.Close()) })
	tx := types.Tx("val:invalid-validator-update")
	finalized, err := app.FinalizeBlock(t.Context(), &abci.RequestFinalizeBlock{
		Txs: [][]byte{tx},
	})
	require.NoError(t, err)
	require.Len(t, finalized.TxResults, 1)
	require.True(t, finalized.TxResults[0].IsErr())
	_, err = app.Commit(t.Context())
	require.NoError(t, err)

	event := types.EventDataTx{TxResultV2: abci.TxResultV2{
		Height: 1, Tx: tx, Result: *finalized.TxResults[0],
	}}
	decoded, _ := repair93SubscriptionRoundTrip(t, event)
	repair93AssertResult(t, event, decoded)
	require.NotZero(t, decoded.TxResult.Result.Code)
}

func TestRepair_93_LegacyTxResultSubscription(t *testing.T) {
	for _, code := range []uint32{37, math.MaxUint32, 0} {
		t.Run(strconv.FormatUint(uint64(code), 10), func(t *testing.T) {
			result := abci.ExecTxResult{
				Code:      code,
				Data:      []byte{0, 0xff, 0x80, 'r', 'e', 's', 'u', 'l', 't'},
				Log:       "execution result",
				Info:      "application diagnostics",
				GasWanted: math.MaxInt64,
				GasUsed:   -1,
				Events: []abci.Event{{
					Type:       "execution",
					Attributes: []abci.EventAttribute{{Key: []byte("outcome"), Value: []byte("committed"), Index: true}},
				}},
				Codespace: "evm",
				EvmTxInfo: &abci.EvmTxInfo{
					SenderAddress: "0x1234", Nonce: math.MaxUint64,
					TxHash: "0xabcd", VmError: "execution reverted",
				},
			}
			producer := abci.ResponseFinalizeBlock{TxResults: []*abci.ExecTxResult{&result}}
			encoded, err := producer.Marshal()
			require.NoError(t, err)
			var transported abci.ResponseFinalizeBlock
			require.NoError(t, transported.Unmarshal(encoded))
			require.Equal(t, producer, transported)
			event := types.EventDataTx{TxResultV2: abci.TxResultV2{
				Height: math.MaxInt64, Index: 3, Tx: []byte("transaction-result"),
				Result: *transported.TxResults[0],
			}}
			decoded, wire := repair93SubscriptionRoundTrip(t, event)
			repair93AssertResult(t, event, decoded)

			var envelope struct {
				Data struct {
					Type  string `json:"type"`
					Value struct {
						TxResult struct {
							Height string                     `json:"height"`
							Result map[string]json.RawMessage `json:"result"`
						} `json:"TxResult"`
					} `json:"value"`
				} `json:"data"`
			}
			require.NoError(t, json.Unmarshal(wire, &envelope))
			require.Equal(t, "tendermint/event/Tx", envelope.Data.Type)
			require.Equal(t, strconv.FormatInt(event.Height, 10), envelope.Data.Value.TxResult.Height)
			fields := envelope.Data.Value.TxResult.Result
			if code != 0 {
				require.JSONEq(t, strconv.FormatUint(uint64(code), 10), string(fields["code"]))
			}
			for name, want := range map[string]string{
				"gas_wanted": strconv.FormatInt(result.GasWanted, 10),
				"gas_used":   strconv.FormatInt(result.GasUsed, 10),
				"data":       base64.StdEncoding.EncodeToString(result.Data),
			} {
				var got string
				require.NoError(t, json.Unmarshal(fields[name], &got))
				require.Equal(t, want, got, name)
			}
		})
	}
}

func TestRepair_93_EmptyTransactionResultSubscription(t *testing.T) {
	event := types.EventDataTx{TxResultV2: abci.TxResultV2{Height: 1, Tx: []byte("empty-result")}}
	decoded, _ := repair93SubscriptionRoundTrip(t, event)
	repair93AssertResult(t, event, decoded)
}

func repair93SubscriptionRoundTrip(t *testing.T, event types.EventDataTx) (types.LegacyEventDataTx, json.RawMessage) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	t.Cleanup(cancel)
	bus := eventbus.NewDefault()
	require.NoError(t, bus.Start(ctx))
	t.Cleanup(bus.Stop)
	query := types.EventQueryTxFor(types.Tx(event.Tx))
	sub, err := bus.SubscribeWithArgs(ctx, pubsub.SubscribeArgs{ClientID: "repair93", Query: query, Limit: 1})
	require.NoError(t, err)
	env := &core.Environment{EventBus: bus, Config: *config.DefaultRPCConfig()}
	manager := rpcserver.NewWebsocketManager(map[string]*rpcserver.RPCFunc{
		"subscribe":       rpcserver.NewWSRPCFunc(env.Subscribe),
		"unsubscribe_all": rpcserver.NewWSRPCFunc(env.UnsubscribeAll),
	})
	mux := http.NewServeMux()
	mux.HandleFunc("/websocket", manager.WebsocketHandler)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	client, err := rpcclient.NewWS(server.URL, "/websocket")
	require.NoError(t, err)
	require.NoError(t, client.Start(ctx))
	t.Cleanup(func() { require.NoError(t, client.Stop()) })
	require.NoError(t, client.Subscribe(ctx, query.String()))
	ack := repair93Response(t, ctx, client)
	require.Nil(t, ack.Error)
	require.JSONEq(t, "{}", string(ack.Result))

	require.NoError(t, bus.PublishEventTx(event))
	message, err := sub.Next(ctx)
	require.NoError(t, err)
	require.Equal(t, event, message.Data())
	response := repair93Response(t, ctx, client)
	require.Nil(t, response.Error)
	var decoded coretypes.ResultEvent
	require.NoError(t, tmjson.Unmarshal(response.Result, &decoded))
	require.Equal(t, query.String(), decoded.Query)
	legacy, ok := decoded.Data.(types.LegacyEventDataTx)
	require.True(t, ok, "unexpected event type %T", decoded.Data)
	require.Equal(t, message.LegacyData(), legacy)

	require.NoError(t, client.UnsubscribeAll(ctx))
	unsubscribed := repair93Response(t, ctx, client)
	require.Nil(t, unsubscribed.Error)
	require.NoError(t, bus.UnsubscribeAll(ctx, "repair93"))
	return legacy, response.Result
}

func repair93Response(t *testing.T, ctx context.Context, client *rpcclient.WSClient) rpctypes.RPCResponse {
	t.Helper()
	select {
	case response, ok := <-client.ResponsesCh:
		require.True(t, ok)
		return response
	case <-ctx.Done():
		t.Fatal(ctx.Err())
		return rpctypes.RPCResponse{}
	}
}

func repair93AssertResult(t *testing.T, want types.EventDataTx, got types.LegacyEventDataTx) {
	t.Helper()
	require.Equal(t, strconv.FormatInt(want.Height, 10), got.TxResult.Height)
	require.Equal(t, want.Index, got.TxResult.Index)
	require.Equal(t, want.Tx, got.TxResult.Tx)
	result := got.TxResult.Result
	require.Equal(t, want.Result.Code, result.Code)
	require.Equal(t, want.Result.Data, result.Data)
	require.Equal(t, want.Result.Log, result.Log)
	require.Equal(t, want.Result.Info, result.Info)
	require.Equal(t, strconv.FormatInt(want.Result.GasWanted, 10), result.GasWanted)
	require.Equal(t, strconv.FormatInt(want.Result.GasUsed, 10), result.GasUsed)
	require.Equal(t, want.Result.Events, result.Events)
	require.Equal(t, want.Result.Codespace, result.Codespace)
	require.Equal(t, want.Result.EvmTxInfo, result.EvmTxInfo)
}
