package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"

	rpcserver "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/server"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxbridge/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/std"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	typesrest "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/rest"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/legacy/legacytx"
	govrest "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/client/rest"
	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/types"
)

func repairM1268Request(t *testing.T, proposal []byte) (client.Context, []byte) {
	t.Helper()
	cdc, txConfig := testCodec()
	amino := codec.NewLegacyAmino()
	std.RegisterLegacyAminoCodec(amino)
	legacytx.RegisterLegacyAminoCodec(amino)
	govtypes.RegisterLegacyAminoCodec(amino)
	types.RegisterCodec(amino)
	clientCtx := client.Context{}.
		WithCodec(cdc).
		WithInterfaceRegistry(cdc.InterfaceRegistry()).
		WithTxConfig(txConfig).
		WithLegacyAmino(amino)
	body, err := json.Marshal(BridgeProposalRequest{
		BaseReq: typesrest.BaseReq{
			From:          testProposer.String(),
			ChainID:       "paxeer-x-test",
			Memo:          "repair 22.33.1268",
			Gas:           "200000",
			Fees:          sdk.NewCoins(sdk.NewInt64Coin("uhpx", 500)),
			TimeoutHeight: 123,
		},
		Deposit:  testDeposit,
		Proposal: proposal,
	})
	require.NoError(t, err)
	return clientCtx, body
}

func repairM1268Server(t *testing.T, clientCtx client.Context, limit int64) *httptest.Server {
	t.Helper()
	router := mux.NewRouter()
	govrest.RegisterHandlers(clientCtx, router, []govrest.ProposalRESTHandler{
		BridgeProposalHandler.RESTHandler(clientCtx),
	})
	server := httptest.NewServer(rpcserver.MaxBytesHandler(router, limit))
	server.Client().Timeout = 5 * time.Second
	t.Cleanup(server.Close)
	return server
}

func repairM1268Post(t *testing.T, server *httptest.Server, body string, status int) []byte {
	t.Helper()
	response, err := server.Client().Post(server.URL+"/gov/proposals/"+ProposalRESTSubRoute,
		"application/json", strings.NewReader(body))
	require.NoError(t, err)
	defer response.Body.Close()
	contents, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.Equal(t, status, response.StatusCode, "%s", contents)
	require.Contains(t, response.Header.Get("Content-Type"), "application/json")
	return contents
}

func repairM1268Unsigned(t *testing.T, clientCtx client.Context, response, proposal []byte) {
	t.Helper()
	var unsigned legacytx.StdTx
	require.NoError(t, clientCtx.LegacyAmino.UnmarshalAsJSON(response, &unsigned))
	require.Empty(t, unsigned.Signatures)
	require.Equal(t, "repair 22.33.1268", unsigned.Memo)
	require.Equal(t, uint64(200000), unsigned.Fee.Gas)
	require.Equal(t, sdk.NewCoins(sdk.NewInt64Coin("uhpx", 500)), unsigned.Fee.Amount)
	require.Equal(t, uint64(123), unsigned.TimeoutHeight)
	require.Len(t, unsigned.Msgs, 1)
	submit, ok := unsigned.Msgs[0].(*govtypes.MsgSubmitProposal)
	require.True(t, ok, "unexpected unsigned message %T", unsigned.Msgs[0])
	require.NoError(t, submit.ValidateBasic())
	require.Equal(t, testProposer.String(), submit.Proposer)
	require.Equal(t, sdk.NewCoins(sdk.NewInt64Coin("uhpx", 10_000_000)), submit.InitialDeposit)
	content, ok := submit.GetContent().(*types.BridgeProposal)
	require.True(t, ok, "unexpected content %T", submit.GetContent())
	require.NoError(t, content.ValidateBasic())
	encoded, err := clientCtx.Codec.MarshalInterfaceJSON(content)
	require.NoError(t, err)
	require.JSONEq(t, string(proposal), string(encoded))
}

func TestRepairM_1268_WhitespacePreservesUnsignedTransaction(t *testing.T) {
	for _, chain := range []string{"ethereum", "solana"} {
		_, files := generatedProposals(t, chain)
		for _, file := range files {
			t.Run(chain+"/"+file.Name, func(t *testing.T) {
				clientCtx, body := repairM1268Request(t, file.Body)
				server := repairM1268Server(t, clientCtx, rpcserver.DefaultConfig().MaxBodyBytes)
				plain := repairM1268Post(t, server, string(body), http.StatusOK)
				repairM1268Unsigned(t, clientCtx, plain, file.Body)
				padded := repairM1268Post(t, server, " \r\n\t"+string(body)+"\t\r\n ", http.StatusOK)
				require.JSONEq(t, string(plain), string(padded))
				repairM1268Unsigned(t, clientCtx, padded, file.Body)
			})
		}
	}
}

func TestRepairM_1268_RejectsSecondJSONValue(t *testing.T) {
	_, files := generatedProposals(t, "ethereum")
	clientCtx, body := repairM1268Request(t, files[0].Body)
	server := repairM1268Server(t, clientCtx, rpcserver.DefaultConfig().MaxBodyBytes)
	for _, suffix := range []string{`{}`, string(body), `[]`, `null`, `true`, `42`, `"extra"`} {
		t.Run(suffix, func(t *testing.T) {
			response := repairM1268Post(t, server, string(body)+" \n"+suffix, http.StatusBadRequest)
			require.Contains(t, string(response), "exactly one JSON value")
		})
	}
}

func TestRepairM_1268_RejectsTrailingInvalidBytes(t *testing.T) {
	_, files := generatedProposals(t, "ethereum")
	clientCtx, body := repairM1268Request(t, files[0].Body)
	server := repairM1268Server(t, clientCtx, rpcserver.DefaultConfig().MaxBodyBytes)
	for _, suffix := range []string{"garbage", "}", "{", "\x00", "\xff", "/* comment */"} {
		t.Run(suffix, func(t *testing.T) {
			response := repairM1268Post(t, server, string(body)+" \n"+suffix, http.StatusBadRequest)
			require.Contains(t, string(response), "exactly one JSON value")
		})
	}
}

func TestRepairM_1268_PreservesAPIBodyLimit(t *testing.T) {
	_, files := generatedProposals(t, "ethereum")
	clientCtx, body := repairM1268Request(t, files[0].Body)
	limit := int64(len(body) + 8)
	server := repairM1268Server(t, clientCtx, limit)
	atLimit := string(body) + strings.Repeat(" ", 8)
	response := repairM1268Post(t, server, atLimit, http.StatusOK)
	repairM1268Unsigned(t, clientCtx, response, files[0].Body)
	for _, suffix := range []string{" ", "{}", "garbage"} {
		t.Run(suffix, func(t *testing.T) {
			response := repairM1268Post(t, server, atLimit+suffix, http.StatusBadRequest)
			require.Contains(t, string(response), "http: request body too large")
			require.NotContains(t, string(response), "exactly one JSON value")
		})
	}
	defaultLimit := rpcserver.DefaultConfig().MaxBodyBytes
	defaultServer := repairM1268Server(t, clientCtx, defaultLimit)
	oversized := append(bytes.Clone(body), bytes.Repeat([]byte(" "), int(defaultLimit)-len(body)+1)...)
	response = repairM1268Post(t, defaultServer, string(oversized), http.StatusBadRequest)
	require.Contains(t, string(response), "http: request body too large")
}
