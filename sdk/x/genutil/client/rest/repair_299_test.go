package rest_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"

	rpchttp "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client/http"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client/local"
	rpcserver "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/server"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	clienttx "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/tx"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	cryptocodec "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/ed25519"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	txtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/tx"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/tx/signing"
	authrest "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/client/rest"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/legacy/legacytx"
	authsigning "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/signing"
	authtx "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/tx"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/genutil"
	genutiltypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/genutil/types"
	stakingtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/types"
)

const repair299ChainID = "gentx-round-trip"

func repair299Context() client.Context {
	registry := codectypes.NewInterfaceRegistry()
	sdk.RegisterInterfaces(registry)
	cryptocodec.RegisterInterfaces(registry)
	txtypes.RegisterInterfaces(registry)
	stakingtypes.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	amino := codec.NewLegacyAmino()
	sdk.RegisterLegacyAminoCodec(amino)
	cryptocodec.RegisterCrypto(amino)
	legacytx.RegisterLegacyAminoCodec(amino)
	stakingtypes.RegisterLegacyAminoCodec(amino)
	return client.Context{}.
		WithCodec(cdc).
		WithLegacyAmino(amino).
		WithInterfaceRegistry(registry).
		WithTxConfig(authtx.NewTxConfig(cdc, authtx.DefaultSignModes))
}

func repair299SignedGenTx(t *testing.T, ctx client.Context, mode signing.SignMode, memo string) authsigning.Tx {
	t.Helper()
	key := secp256k1.GenPrivKey()
	msg, err := stakingtypes.NewMsgCreateValidator(
		sdk.ValAddress(key.PubKey().Address()), ed25519.GenPrivKey().PubKey(),
		sdk.NewInt64Coin(sdk.DefaultBondDenom, 1000000),
		stakingtypes.NewDescription("round-trip-validator", "", "", "", ""),
		stakingtypes.NewCommissionRates(sdk.NewDecWithPrec(1, 1), sdk.NewDecWithPrec(2, 1), sdk.NewDecWithPrec(1, 2)),
		sdk.OneInt(),
	)
	require.NoError(t, err)
	require.NoError(t, msg.ValidateBasic())
	txf := clienttx.Factory{}.
		WithTxConfig(ctx.TxConfig).
		WithChainID(repair299ChainID).
		WithGas(200000).
		WithFees("7stake").
		WithMemo(memo).
		WithTimeoutHeight(123)
	builder, err := clienttx.BuildUnsignedTx(txf, msg)
	require.NoError(t, err)
	require.NoError(t, builder.SetSignatures(signing.SignatureV2{
		PubKey: key.PubKey(),
		Data:   &signing.SingleSignatureData{SignMode: mode},
	}))
	sig, err := clienttx.SignWithPrivKey(mode, authsigning.SignerData{ChainID: repair299ChainID}, builder, key, ctx.TxConfig, 0)
	require.NoError(t, err)
	require.NoError(t, builder.SetSignatures(sig))
	require.NoError(t, builder.GetTx().ValidateBasic())
	return builder.GetTx()
}

// Type inference lets the test initialize the real RPC environment exposed by
// Local without importing the consensus internal package or replacing its methods.
func repair299New[T any](_ *T) *T { return new(T) }

func repair299GenesisRoute(t *testing.T, ctx client.Context, state map[string]json.RawMessage) (*mux.Router, *tmtypes.GenesisDoc) {
	t.Helper()
	appState, err := json.Marshal(state)
	require.NoError(t, err)
	doc := &tmtypes.GenesisDoc{ChainID: repair299ChainID, AppState: appState}
	path := filepath.Join(t.TempDir(), "genesis.json")
	require.NoError(t, doc.SaveAs(path))
	loaded, err := tmtypes.GenesisDocFromFile(path)
	require.NoError(t, err)
	node := &local.Local{}
	node.Environment = repair299New(node.Environment)
	node.GenDoc = loaded
	require.NoError(t, node.InitGenesisChunks())
	rpcMux := http.NewServeMux()
	rpcserver.RegisterRPCFuncs(rpcMux, map[string]*rpcserver.RPCFunc{
		"genesis": rpcserver.NewRPCFunc(node.Environment.Genesis),
	})
	server := httptest.NewServer(rpcMux)
	t.Cleanup(server.Close)
	rpc, err := rpchttp.NewWithClient(server.URL, server.Client())
	require.NoError(t, err)
	result, err := rpc.Genesis(context.Background())
	require.NoError(t, err)
	require.JSONEq(t, string(loaded.AppState), string(result.Genesis.AppState))
	router := mux.NewRouter()
	authrest.RegisterTxRoutes(ctx.WithClient(rpc), router)
	return router, loaded
}

func repair299Query(router *mux.Router) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/txs?height=0", nil))
	return w
}

func TestRepair_299_GenesisTxRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mode   signing.SignMode
		legacy bool
	}{
		{"protobuf_direct", signing.SignMode_SIGN_MODE_DIRECT, false},
		{"protobuf_amino_signature", signing.SignMode_SIGN_MODE_LEGACY_AMINO_JSON, false},
		{"legacy_amino", signing.SignMode_SIGN_MODE_LEGACY_AMINO_JSON, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := repair299Context()
			if tc.legacy {
				ctx = ctx.WithTxConfig(legacytx.StdTxConfig{Cdc: ctx.LegacyAmino})
			}
			originals := []sdk.Tx{
				repair299SignedGenTx(t, ctx, tc.mode, "first-validator"),
				repair299SignedGenTx(t, ctx, tc.mode, "second-validator"),
			}
			state, err := genutil.SetGenTxsInAppGenesisState(ctx.Codec, ctx.TxConfig.TxJSONEncoder(), map[string]json.RawMessage{}, originals)
			require.NoError(t, err)
			router, loaded := repair299GenesisRoute(t, ctx, state)
			appState, err := genutiltypes.GenesisStateFromGenDoc(*loaded)
			require.NoError(t, err)
			stored := genutiltypes.GetGenesisStateFromAppState(ctx.Codec, appState)
			require.NoError(t, genutiltypes.ValidateGenesis(stored, ctx.TxConfig.TxJSONDecoder()))
			w := repair299Query(router)
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			require.Equal(t, "application/json", w.Header().Get("Content-Type"))
			var response []json.RawMessage
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
			require.Len(t, response, len(originals))
			for i, raw := range response {
				require.JSONEq(t, string(stored.GenTxs[i]), string(raw))
				decoded, err := ctx.TxConfig.TxJSONDecoder()(raw)
				require.NoError(t, err)
				want, err := ctx.TxConfig.TxEncoder()(originals[i])
				require.NoError(t, err)
				got, err := ctx.TxConfig.TxEncoder()(decoded)
				require.NoError(t, err)
				require.Equal(t, want, got)
				signed, ok := decoded.(authsigning.Tx)
				require.True(t, ok)
				require.NoError(t, signed.ValidateBasic())
				sigs, err := signed.GetSignaturesV2()
				require.NoError(t, err)
				require.Len(t, sigs, 1)
				data, ok := sigs[0].Data.(*signing.SingleSignatureData)
				require.True(t, ok)
				require.Equal(t, tc.mode, data.SignMode)
				signBytes, err := ctx.TxConfig.SignModeHandler().GetSignBytes(tc.mode, authsigning.SignerData{ChainID: repair299ChainID}, decoded)
				require.NoError(t, err)
				require.True(t, sigs[0].PubKey.VerifySignature(signBytes, data.Signature))
			}
		})
	}
}

func TestRepair_299_EmptyGenesisTxs(t *testing.T) {
	ctx := repair299Context()
	state, err := genutil.SetGenTxsInAppGenesisState(ctx.Codec, ctx.TxConfig.TxJSONEncoder(), map[string]json.RawMessage{}, nil)
	require.NoError(t, err)
	router, _ := repair299GenesisRoute(t, ctx, state)
	w := repair299Query(router)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Equal(t, "application/json", w.Header().Get("Content-Type"))
	require.JSONEq(t, "[]", w.Body.String())
}

func TestRepair_299_InvalidGenesisTx(t *testing.T) {
	ctx := repair299Context()
	tx := repair299SignedGenTx(t, ctx, signing.SignMode_SIGN_MODE_DIRECT, "valid-before-invalid")
	state, err := genutil.SetGenTxsInAppGenesisState(ctx.Codec, ctx.TxConfig.TxJSONEncoder(), map[string]json.RawMessage{}, []sdk.Tx{tx, tx})
	require.NoError(t, err)
	genState := genutiltypes.GetGenesisStateFromAppState(ctx.Codec, state)
	bad := bytes.Replace(genState.GenTxs[1], []byte("/cosmos.staking.v1beta1.MsgCreateValidator"), []byte("/unknown.MsgCreateValidator"), 1)
	require.NotEqual(t, genState.GenTxs[1], json.RawMessage(bad))
	genState.GenTxs[1] = bad
	state = genutiltypes.SetGenesisStateInAppState(ctx.Codec, state, genState)
	router, _ := repair299GenesisRoute(t, ctx, state)
	w := repair299Query(router)
	require.Equal(t, http.StatusInternalServerError, w.Code)
	require.Equal(t, "application/json", w.Header().Get("Content-Type"))
	var response struct {
		Error string `json:"error"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.Contains(t, response.Error, "failed to decode genesis transaction")
	require.Contains(t, response.Error, "/unknown.MsgCreateValidator")
}
