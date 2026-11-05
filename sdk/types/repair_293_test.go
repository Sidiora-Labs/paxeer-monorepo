package types_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/legacy"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/legacy/legacytx"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	genutiltypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/genutil/types"
	wasmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm/types"
)

func TestRepair_293_NormalizedCoinsBankGenesis(t *testing.T) {
	sdk.Repair293RegisterDenoms(t)
	cases := []struct {
		name  string
		input string
		want  sdk.Coins
	}{
		{"multiple_units", "1atom,1matom,1uatom", sdk.NewCoins(sdk.NewInt64Coin("natom", 1001001000))},
		{"fractional_units", "0.1atom,0.1matom,0.1uatom", sdk.NewCoins(sdk.NewInt64Coin("natom", 100100100))},
		{"aggregate_before_truncation", "0.0000000006atom,0.0000006matom,0.0006uatom", sdk.NewCoins(sdk.NewInt64Coin("natom", 1))},
		{"fractional_carry", "1.0000000009atom,0.0000009matom", sdk.NewCoins(sdk.NewInt64Coin("natom", 1000000001))},
		{"remove_sub_base_totals", "0.0000000001atom,0.0000001matom", sdk.Coins{}},
		{"remove_unregistered_fraction", "0.4other", sdk.Coins{}},
		{"sort_after_conversion", "1atom,2bbb,3zzz", sdk.NewCoins(sdk.NewInt64Coin("bbb", 2), sdk.NewInt64Coin("natom", 1000000000), sdk.NewInt64Coin("zzz", 3))},
		{"empty", "", nil},
	}
	cdc := codec.NewProtoCodec(codectypes.NewInterfaceRegistry())
	address := sdk.AccAddress(bytes.Repeat([]byte{1}, 20)).String()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			coins, err := sdk.ParseCoinsNormalized(tc.input)
			require.NoError(t, err)
			require.Equal(t, tc.want, coins)
			balance := banktypes.Balance{Address: address, Coins: coins}
			require.NoError(t, balance.Validate())
			genesis := banktypes.NewGenesisState(banktypes.DefaultParams(), []banktypes.Balance{balance}, coins, nil, nil)
			wire, err := cdc.MarshalAsJSON(genesis)
			require.NoError(t, err)
			wire, err = sdk.SortJSON(wire)
			require.NoError(t, err)
			recovered := banktypes.GetGenesisStateFromAppState(cdc, map[string]json.RawMessage{banktypes.ModuleName: wire})
			require.NoError(t, recovered.Validate())
			require.Len(t, recovered.Balances, 1)
			require.Equal(t, tc.want.String(), recovered.Balances[0].Coins.String())
			require.Equal(t, tc.want.String(), recovered.Supply.String())
		})
	}
	_, err := sdk.ParseCoinsNormalized("1atom,2atom")
	require.Error(t, err, "duplicate input denominations must remain invalid")
}

type repair293Numbers struct {
	Positive json.Number   `json:"positive"`
	Negative json.Number   `json:"negative"`
	Decimal  json.Number   `json:"decimal"`
	Nested   []json.Number `json:"nested"`
}

func repair293NumberPayload(t *testing.T) (repair293Numbers, []byte) {
	t.Helper()
	numbers := repair293Numbers{
		Positive: "9007199254740993",
		Negative: "-9007199254740993",
		Decimal:  "0.123456789012345678901234567890123456789",
		Nested: []json.Number{
			"18446744073709551617",
			"-18446744073709551617",
			"-12345678901234567890.12345678901234567890",
			"1.234567890123456789e+100",
			"1e1000",
			"-0.000000000000000000000000000001",
		},
	}
	payload, err := json.Marshal(numbers)
	require.NoError(t, err)
	return numbers, payload
}

func TestRepair_293_CanonicalJSONSignBytes(t *testing.T) {
	want, payload := repair293NumberPayload(t)
	key := secp256k1.GenPrivKeyFromSecret([]byte("repair-293-signing"))
	msg := wasmtypes.MsgExecuteContract{
		Sender:   sdk.AccAddress(key.PubKey().Address()).String(),
		Contract: sdk.AccAddress(bytes.Repeat([]byte{2}, 20)).String(),
		Msg:      payload,
		Funds:    sdk.NewCoins(),
	}
	require.NoError(t, msg.ValidateBasic())
	messageBytes := msg.GetSignBytes()
	var messageConsumer wasmtypes.MsgExecuteContract
	require.NoError(t, wasmtypes.ModuleCdc.UnmarshalAsJSON(messageBytes, &messageConsumer))
	var messageNumbers repair293Numbers
	require.NoError(t, json.Unmarshal(messageConsumer.Msg, &messageNumbers))
	require.Equal(t, want, messageNumbers)

	signBytes := legacytx.StdSignBytes("repair-293", 1, 2, 0, legacytx.StdFee{}, []sdk.Msg{&msg}, "")
	signature, err := key.Sign(signBytes)
	require.NoError(t, err)
	type signedTransport struct {
		SignBytes []byte `json:"sign_bytes"`
		Signature []byte `json:"signature"`
	}
	wire, err := json.Marshal(signedTransport{SignBytes: signBytes, Signature: signature})
	require.NoError(t, err)
	var received signedTransport
	require.NoError(t, json.Unmarshal(wire, &received))
	require.True(t, key.PubKey().VerifySignature(received.SignBytes, received.Signature))
	var signDoc legacytx.StdSignDoc
	require.NoError(t, legacy.Cdc.UnmarshalAsJSON(received.SignBytes, &signDoc))
	require.Len(t, signDoc.Msgs, 1)
	var consumer wasmtypes.MsgExecuteContract
	require.NoError(t, wasmtypes.ModuleCdc.UnmarshalAsJSON(signDoc.Msgs[0], &consumer))
	require.NoError(t, consumer.ValidateBasic())
	var got repair293Numbers
	require.NoError(t, json.Unmarshal(consumer.Msg, &got))
	require.Equal(t, want, got)
	require.Equal(t, msg.Sender, consumer.Sender)
	require.Equal(t, msg.Contract, consumer.Contract)
}

func TestRepair_293_CanonicalJSONGenesis(t *testing.T) {
	want, payload := repair293NumberPayload(t)
	appState, err := json.Marshal(map[string]json.RawMessage{"repair293": payload})
	require.NoError(t, err)
	producer := tmtypes.GenesisDoc{
		GenesisTime:     time.Unix(1, 0).UTC(),
		ChainID:         "repair-293",
		InitialHeight:   1,
		ConsensusParams: tmtypes.DefaultConsensusParams(),
		AppState:        appState,
	}
	inputPath := filepath.Join(t.TempDir(), "genesis.json")
	require.NoError(t, producer.SaveAs(inputPath))
	exported, err := tmtypes.GenesisDocFromFile(inputPath)
	require.NoError(t, err)
	serialized, err := json.Marshal(exported)
	require.NoError(t, err)
	canonical, err := sdk.SortJSON(serialized)
	require.NoError(t, err)
	require.Equal(t, canonical, sdk.MustSortJSON(serialized))
	require.Equal(t, canonical, sdk.MustSortJSON(canonical), "canonicalization must be idempotent")
	outputPath := filepath.Join(t.TempDir(), "canonical-genesis.json")
	require.NoError(t, os.WriteFile(outputPath, canonical, 0600))
	recovered, genDoc, err := genutiltypes.GenesisStateFromGenFile(outputPath)
	require.NoError(t, err)
	var got repair293Numbers
	require.NoError(t, json.Unmarshal(recovered["repair293"], &got))
	require.Equal(t, want, got)
	require.Equal(t, producer.ChainID, genDoc.ChainID)
	require.Equal(t, producer.InitialHeight, genDoc.InitialHeight)
}

func TestRepair_293_CanonicalJSONStrictInput(t *testing.T) {
	for _, input := range []string{
		`{"amount":9007199254740993} {"second":1}`,
		`{"amount":9007199254740993} null`,
		`1 2`,
		`{} trailing`,
		`{} {`,
		``,
		`{"amount":01}`,
	} {
		t.Run(input, func(t *testing.T) {
			got, err := sdk.SortJSON([]byte(input))
			require.Error(t, err)
			require.Nil(t, got)
			require.Panics(t, func() { sdk.MustSortJSON([]byte(input)) })
		})
	}
	input := []byte(" \n {\"z\":[9007199254740993,-9007199254740993,0.123456789012345678901],\"a\":1e1000} \t\n")
	got, err := sdk.SortJSON(input)
	require.NoError(t, err)
	require.Equal(t, `{"a":1e1000,"z":[9007199254740993,-9007199254740993,0.123456789012345678901]}`, string(got))
	require.Equal(t, got, sdk.MustSortJSON(got))
}
