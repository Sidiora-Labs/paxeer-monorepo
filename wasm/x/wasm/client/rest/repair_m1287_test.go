package rest

import (
	"bytes"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"

	rpchttp "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client/http"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/std"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkrest "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/rest"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/legacy/legacytx"
	authtx "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/tx"
	"github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm/types"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"
)

func repairM1287Route(t *testing.T) (*mux.Router, client.Context, *atomic.Int64) {
	t.Helper()
	requests := new(atomic.Int64)
	node := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		http.Error(w, "unexpected node request", http.StatusInternalServerError)
	}))
	t.Cleanup(node.Close)
	rpc, err := rpchttp.NewWithClient(node.URL, node.Client())
	require.NoError(t, err)
	amino := codec.NewLegacyAmino()
	std.RegisterLegacyAminoCodec(amino)
	legacytx.RegisterLegacyAminoCodec(amino)
	types.RegisterLegacyAminoCodec(amino)
	registry := codectypes.NewInterfaceRegistry()
	std.RegisterInterfaces(registry)
	types.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	ctx := client.Context{}.
		WithLegacyAmino(amino).
		WithCodec(cdc).
		WithInterfaceRegistry(registry).
		WithTxConfig(authtx.NewTxConfig(cdc, authtx.DefaultSignModes)).
		WithClient(rpc)
	router := mux.NewRouter()
	RegisterRoutes(ctx, router)
	return router, ctx, requests
}

func repairM1287Request() instantiateContractReq {
	return instantiateContractReq{
		BaseReq: sdkrest.BaseReq{
			From:    sdk.AccAddress(bytes.Repeat([]byte{1}, 20)).String(),
			ChainID: "repair-m1287",
			Gas:     "200000",
			Fees:    sdk.NewCoins(),
		},
		Label:   "repair-m1287",
		Deposit: sdk.NewCoins(),
		Msg:     []byte(`{"count":0}`),
	}
}

func repairM1287Post(t *testing.T, router *mux.Router, ctx client.Context, codeID string, req instantiateContractReq) *httptest.ResponseRecorder {
	t.Helper()
	body, err := ctx.LegacyAmino.MarshalAsJSON(req)
	require.NoError(t, err)
	r := httptest.NewRequest(http.MethodPost, "/wasm/code/"+codeID, bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)
	return w
}

func TestRepairM_1287_InvalidCodeIDs(t *testing.T) {
	for _, codeID := range []string{"not-a-number", "18446744073709551616", "-1"} {
		for _, simulate := range []bool{false, true} {
			t.Run(codeID+"/simulate="+strconv.FormatBool(simulate), func(t *testing.T) {
				router, ctx, requests := repairM1287Route(t)
				req := repairM1287Request()
				req.BaseReq.Simulate = simulate
				w := repairM1287Post(t, router, ctx, codeID, req)
				require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
				require.Equal(t, "application/json", w.Header().Get("Content-Type"))
				_, parseErr := strconv.ParseUint(codeID, 10, 64)
				require.Error(t, parseErr)
				var response sdkrest.ErrorResponse
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
				require.Equal(t, parseErr.Error(), response.Error)
				var fields map[string]json.RawMessage
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &fields))
				require.Len(t, fields, 1, "response must contain only the error, with no generated transaction or simulation result")
				require.Contains(t, fields, "error")
				require.Zero(t, requests.Load(), "invalid IDs must not simulate or broadcast to the node")
			})
		}
	}
}

func TestRepairM_1287_ValidCodeIDsGenerateTransaction(t *testing.T) {
	for _, codeID := range []uint64{1, math.MaxUint64} {
		t.Run(strconv.FormatUint(codeID, 10), func(t *testing.T) {
			router, ctx, requests := repairM1287Route(t)
			req := repairM1287Request()
			w := repairM1287Post(t, router, ctx, strconv.FormatUint(codeID, 10), req)
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			require.Equal(t, "application/json", w.Header().Get("Content-Type"))
			var generated legacytx.StdTx
			require.NoError(t, ctx.LegacyAmino.UnmarshalAsJSON(w.Body.Bytes(), &generated))
			require.Len(t, generated.GetMsgs(), 1)
			msg, ok := generated.GetMsgs()[0].(*types.MsgInstantiateContract)
			require.True(t, ok, "generated message must be an instantiate transaction")
			require.Equal(t, codeID, msg.CodeID)
			require.Equal(t, req.BaseReq.From, msg.Sender)
			require.Equal(t, req.Label, msg.Label)
			require.Equal(t, req.Msg, []byte(msg.Msg))
			require.NoError(t, msg.ValidateBasic())
			require.Zero(t, requests.Load(), "generating an unsigned transaction must not broadcast")
		})
	}
}
