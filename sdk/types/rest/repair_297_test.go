package rest_test

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/rest"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/legacy/legacytx"
	bankrest "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/client/rest"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/wasm/app"
)

func TestRepair_297_SanitizePreservesFields(t *testing.T) {
	enc := app.MakeEncodingConfig()
	from := sdk.AccAddress(bytes.Repeat([]byte{1}, 20)).String()
	for _, height := range []uint64{0, 987654, ^uint64(0)} {
		for _, simulate := range []bool{false, true} {
			t.Run(fmt.Sprintf("height_%d_simulate_%t", height, simulate), func(t *testing.T) {
				producer := rest.BaseReq{
					From: " \t" + from + "\n", Memo: " \tmemo\n ", ChainID: " chain-297 ",
					AccountNumber: ^uint64(0), Sequence: ^uint64(0) - 1, TimeoutHeight: height,
					Fees:      sdk.NewCoins(sdk.NewInt64Coin("uatom", 37), sdk.NewInt64Coin("uhpx", 91)),
					GasPrices: sdk.NewDecCoins(sdk.NewDecCoinFromDec("uhpx", sdk.MustNewDecFromStr("0.025"))),
					Gas:       " 200001 ", GasAdjustment: " 1.25 ", Simulate: simulate,
				}
				wire, err := enc.Amino.MarshalAsJSON(producer)
				require.NoError(t, err)
				var received rest.BaseReq
				require.NoError(t, enc.Amino.UnmarshalAsJSON(wire, &received))
				require.Equal(t, producer, received)
				want := producer
				want.From, want.Memo, want.ChainID = from, "memo", "chain-297"
				want.Gas, want.GasAdjustment = "200001", "1.25"
				sanitized := received.Sanitize()
				require.Equal(t, want, sanitized)
				require.Equal(t, sanitized, sanitized.Sanitize())
				require.Equal(t, producer, received)
			})
		}
	}
}

func TestRepair_297_BankSendTimeout(t *testing.T) {
	enc := app.MakeEncodingConfig()
	clientCtx := client.Context{}.WithLegacyAmino(enc.Amino).WithTxConfig(enc.TxConfig)
	router := mux.NewRouter()
	bankrest.RegisterHandlers(clientCtx, router)
	from := sdk.AccAddress(bytes.Repeat([]byte{1}, 20))
	to := sdk.AccAddress(bytes.Repeat([]byte{2}, 20))
	amount := sdk.NewCoins(sdk.NewInt64Coin("uhpx", 12345))
	fees := sdk.NewCoins(sdk.NewInt64Coin("uatom", 37), sdk.NewInt64Coin("uhpx", 91))
	prices := sdk.NewDecCoins(sdk.NewDecCoinFromDec("uhpx", sdk.MustNewDecFromStr("0.025")))
	for _, height := range []uint64{0, 987654, ^uint64(0)} {
		for _, usePrices := range []bool{false, true} {
			t.Run(fmt.Sprintf("height_%d_gas_prices_%t", height, usePrices), func(t *testing.T) {
				producer := bankrest.SendReq{
					BaseReq: rest.BaseReq{
						From: " " + from.String() + " ", Memo: " memo-297 ", ChainID: " chain-297 ",
						AccountNumber: 104, Sequence: 32, TimeoutHeight: height,
						Fees: fees, Gas: " 200001 ", GasAdjustment: " 1.25 ",
					},
					Amount: amount,
				}
				wantFees := fees
				if usePrices {
					producer.BaseReq.Fees = nil
					producer.BaseReq.GasPrices = prices
					wantFees = sdk.NewCoins(sdk.NewInt64Coin("uhpx", 5001))
				}
				wire, err := enc.Amino.MarshalAsJSON(producer)
				require.NoError(t, err)
				request := httptest.NewRequest(http.MethodPost, "/bank/accounts/"+to.String()+"/transfers", bytes.NewReader(wire))
				request.Header.Set("Content-Type", "application/json")
				response := httptest.NewRecorder()
				router.ServeHTTP(response, request)
				require.Equal(t, http.StatusOK, response.Code, response.Body.String())
				require.Equal(t, "application/json", response.Header().Get("Content-Type"))
				var unsigned legacytx.StdTx
				require.NoError(t, enc.Amino.UnmarshalAsJSON(response.Body.Bytes(), &unsigned))
				require.Equal(t, height, unsigned.GetTimeoutHeight())
				require.Equal(t, uint64(200001), unsigned.GetGas())
				require.Equal(t, wantFees, unsigned.GetFee())
				require.Equal(t, "memo-297", unsigned.GetMemo())
				require.Empty(t, unsigned.Signatures)
				require.Equal(t, []sdk.Msg{banktypes.NewMsgSend(from, to, amount)}, unsigned.GetMsgs())
				require.Equal(t, []sdk.AccAddress{from}, unsigned.GetSigners())
			})
		}
	}
}
