package rest_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/std"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/rest"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/legacy/legacytx"
	authtx "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/tx"
	bankrest "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/client/rest"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
)

func repairM650Handler() (*codec.LegacyAmino, *mux.Router, bankrest.SendReq, string) {
	amino := codec.NewLegacyAmino()
	registry := codectypes.NewInterfaceRegistry()
	std.RegisterLegacyAminoCodec(amino)
	std.RegisterInterfaces(registry)
	legacytx.RegisterLegacyAminoCodec(amino)
	banktypes.RegisterLegacyAminoCodec(amino)
	banktypes.RegisterInterfaces(registry)
	ctx := client.Context{}.WithLegacyAmino(amino).WithTxConfig(authtx.NewTxConfig(codec.NewProtoCodec(registry), authtx.DefaultSignModes))
	router := mux.NewRouter()
	bankrest.RegisterHandlers(ctx, router)
	from := sdk.AccAddress(bytes.Repeat([]byte{1}, 20))
	to := sdk.AccAddress(bytes.Repeat([]byte{2}, 20))
	producer := bankrest.SendReq{
		BaseReq: rest.BaseReq{
			From: from.String(), ChainID: "repair-m650", Memo: "preserved-m650",
			AccountNumber: 104, Sequence: 32, TimeoutHeight: 987654,
			Fees: sdk.NewCoins(sdk.NewInt64Coin("uhpx", 37)), Gas: "200000", GasAdjustment: "1.25",
		},
		Amount: sdk.NewCoins(sdk.NewInt64Coin("uhpx", 12345)),
	}
	return amino, router, producer, "/bank/accounts/" + to.String() + "/transfers"
}

func repairM650AssertTx(t *testing.T, amino *codec.LegacyAmino, body []byte, producer bankrest.SendReq, fees sdk.Coins) {
	t.Helper()
	var unsigned legacytx.StdTx
	require.NoError(t, amino.UnmarshalAsJSON(body, &unsigned))
	to := sdk.AccAddress(bytes.Repeat([]byte{2}, 20))
	from, err := sdk.AccAddressFromBech32(producer.BaseReq.From)
	require.NoError(t, err)
	require.Equal(t, []sdk.Msg{banktypes.NewMsgSend(from, to, producer.Amount)}, unsigned.GetMsgs())
	require.Equal(t, []sdk.AccAddress{from}, unsigned.GetSigners())
	require.Equal(t, producer.BaseReq.TimeoutHeight, unsigned.GetTimeoutHeight())
	require.Equal(t, uint64(200000), unsigned.GetGas())
	require.Equal(t, fees, unsigned.GetFee())
	require.Equal(t, producer.BaseReq.Memo, unsigned.GetMemo())
	require.Empty(t, unsigned.Signatures)
}

func TestRepairM_650_InvalidFees(t *testing.T) {
	amino, router, original, path := repairM650Handler()
	for _, representation := range []string{"fees", "gas_prices"} {
		for _, simulate := range []bool{false, true} {
			for _, defect := range []string{"invalid_denom", "negative", "zero", "unsorted", "duplicate", "missing_amount", "missing_second_amount", "null_amount", "invalid_amount_encoding"} {
				t.Run(fmt.Sprintf("%s/%s/simulate_%t", representation, defect, simulate), func(t *testing.T) {
					producer := original
					producer.BaseReq.Simulate = simulate
					producer.BaseReq.Fees = nil
					producer.BaseReq.GasPrices = nil
					denoms := []string{"uhpx"}
					amount := int64(1)
					switch defect {
					case "invalid_denom":
						denoms = []string{"!"}
					case "negative":
						amount = -1
					case "zero":
						amount = 0
					case "unsorted":
						denoms = []string{"uhpx", "uatom"}
					case "duplicate":
						denoms = []string{"uhpx", "uhpx"}
					case "missing_second_amount":
						denoms = []string{"uatom", "uhpx"}
					}
					for _, denom := range denoms {
						if representation == "fees" {
							producer.BaseReq.Fees = append(producer.BaseReq.Fees, sdk.Coin{Denom: denom, Amount: sdk.NewInt(amount)})
						} else {
							producer.BaseReq.GasPrices = append(producer.BaseReq.GasPrices, sdk.DecCoin{Denom: denom, Amount: sdk.NewDec(amount)})
						}
					}
					wire, err := amino.MarshalAsJSON(producer)
					require.NoError(t, err)
					wantStatus := http.StatusPaymentRequired
					if strings.Contains(defect, "amount") {
						var document map[string]json.RawMessage
						require.NoError(t, json.Unmarshal(wire, &document))
						var base map[string]json.RawMessage
						require.NoError(t, json.Unmarshal(document["base_req"], &base))
						var coins []map[string]json.RawMessage
						require.NoError(t, json.Unmarshal(base[representation], &coins))
						i := len(coins) - 1
						if defect == "null_amount" {
							coins[i]["amount"] = json.RawMessage("null")
						} else if defect == "invalid_amount_encoding" {
							coins[i]["amount"] = json.RawMessage(`{"invalid":"amount must be a string"}`)
							wantStatus = http.StatusBadRequest
						} else {
							delete(coins[i], "amount")
						}
						base[representation], err = json.Marshal(coins)
						require.NoError(t, err)
						document["base_req"], err = json.Marshal(base)
						require.NoError(t, err)
						wire, err = json.Marshal(document)
						require.NoError(t, err)
					}
					request := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(wire))
					response := httptest.NewRecorder()
					require.NotPanics(t, func() { router.ServeHTTP(response, request) })
					require.Equal(t, wantStatus, response.Code, response.Body.String())
					require.Equal(t, "true", response.Header().Get("Deprecation"))
					require.Equal(t, "application/json", response.Header().Get("Content-Type"))
					var failure rest.ErrorResponse
					require.NoError(t, json.Unmarshal(response.Body.Bytes(), &failure))
					if wantStatus == http.StatusPaymentRequired {
						require.Equal(t, "invalid fees or gas prices provided", failure.Error)
						var decoded bankrest.SendReq
						require.NoError(t, amino.UnmarshalAsJSON(wire, &decoded))
						validation := httptest.NewRecorder()
						require.NotPanics(t, func() { require.False(t, decoded.BaseReq.Sanitize().ValidateBasic(validation)) })
						require.Equal(t, http.StatusPaymentRequired, validation.Code)
					} else {
						require.Contains(t, failure.Error, "failed to decode JSON payload:")
					}
				})
			}
		}
	}
}

func TestRepairM_650_FeeRepresentations(t *testing.T) {
	amino, router, original, path := repairM650Handler()
	for _, representation := range []string{"fees", "gas_prices", "neither", "both"} {
		t.Run(representation, func(t *testing.T) {
			producer := original
			prices := sdk.NewDecCoins(sdk.NewDecCoinFromDec("uhpx", sdk.MustNewDecFromStr("0.025")))
			wantFees := original.BaseReq.Fees
			switch representation {
			case "gas_prices":
				producer.BaseReq.Fees = nil
				producer.BaseReq.GasPrices = prices
				wantFees = sdk.NewCoins(sdk.NewInt64Coin("uhpx", 5000))
			case "neither":
				producer.BaseReq.Fees = nil
				wantFees = nil
			case "both":
				producer.BaseReq.GasPrices = prices
			}
			wire, err := amino.MarshalAsJSON(producer)
			require.NoError(t, err)
			request := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(wire))
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if representation == "both" {
				require.Equal(t, http.StatusBadRequest, response.Code)
				var failure rest.ErrorResponse
				require.NoError(t, json.Unmarshal(response.Body.Bytes(), &failure))
				require.Equal(t, "cannot provide both fees and gas prices", failure.Error)
				return
			}
			require.Equal(t, http.StatusOK, response.Code, response.Body.String())
			repairM650AssertTx(t, amino, response.Body.Bytes(), producer, wantFees)
		})
	}
}

func TestRepairM_650_StreamedBodyLimit(t *testing.T) {
	amino, router, producer, path := repairM650Handler()
	wire, err := amino.MarshalAsJSON(producer)
	require.NoError(t, err)
	require.Equal(t, 1_000_000, rest.MaxRequestBodyBytes)
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)
	for _, size := range []int{len(wire), rest.MaxRequestBodyBytes - 1, rest.MaxRequestBodyBytes, rest.MaxRequestBodyBytes + 1} {
		t.Run(fmt.Sprintf("bytes_%d", size), func(t *testing.T) {
			stream := io.MultiReader(bytes.NewReader(wire), strings.NewReader(strings.Repeat(" ", size-len(wire))))
			request, err := http.NewRequest(http.MethodPost, server.URL+path, stream)
			require.NoError(t, err)
			require.Zero(t, request.ContentLength)
			request.Header.Set("Content-Type", "application/json")
			response, err := server.Client().Do(request)
			require.NoError(t, err)
			defer response.Body.Close()
			body, err := io.ReadAll(response.Body)
			require.NoError(t, err)
			require.Equal(t, "true", response.Header.Get("Deprecation"))
			require.Equal(t, "application/json", response.Header.Get("Content-Type"))
			if size > rest.MaxRequestBodyBytes {
				require.Equal(t, http.StatusRequestEntityTooLarge, response.StatusCode, string(body))
				require.JSONEq(t, `{"error":"request body exceeds maximum size of 1000000 bytes"}`, string(body))
				return
			}
			require.Equal(t, http.StatusOK, response.StatusCode, string(body))
			repairM650AssertTx(t, amino, body, producer, producer.BaseReq.Fees)
		})
	}
}

func TestRepairM_650_BoundedRead(t *testing.T) {
	amino, router, producer, path := repairM650Handler()
	wire, err := amino.MarshalAsJSON(producer)
	require.NoError(t, err)
	for _, knownLength := range []bool{false, true} {
		t.Run(fmt.Sprintf("known_length_%t", knownLength), func(t *testing.T) {
			size := int64(rest.MaxRequestBodyBytes * 4)
			stream := &io.LimitedReader{
				R: io.MultiReader(bytes.NewReader(wire), strings.NewReader(strings.Repeat(" ", int(size)-len(wire)))), N: size,
			}
			request := httptest.NewRequest(http.MethodPost, path, stream)
			if knownLength {
				request.ContentLength = size
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			require.Equal(t, http.StatusRequestEntityTooLarge, response.Code)
			require.JSONEq(t, `{"error":"request body exceeds maximum size of 1000000 bytes"}`, response.Body.String())
			require.Equal(t, int64(rest.MaxRequestBodyBytes+1), size-stream.N, "must stop reading after the single overflow probe byte")
		})
	}
	request := httptest.NewRequest(http.MethodPost, path, strings.NewReader("{"))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	require.Equal(t, http.StatusBadRequest, response.Code)
	var failure rest.ErrorResponse
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &failure))
	require.Contains(t, failure.Error, "failed to decode JSON payload:")
}
