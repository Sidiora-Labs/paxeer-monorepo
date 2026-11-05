package main

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"
)

func TestVectorV6SponsoredBindVerifies(t *testing.T) {
	// Exercise the genesis editor with the production protobuf JSON encoder:
	// integer parameter strings must survive unchanged while code is appended.
	encoding := app.MakeEncodingConfig()
	evmGenesis, err := encoding.Marshaler.MarshalAsJSON(evmtypes.DefaultGenesis())
	require.NoError(t, err)
	var original map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(evmGenesis, &original))
	home := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(home, "config"), 0700))
	genesis, err := json.Marshal(map[string]any{"app_state": map[string]json.RawMessage{"evm": evmGenesis}})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(home, "config", "genesis.json"), genesis, 0600))
	prepared := &node{home: home}
	require.NoError(t, prepared.configureV6Genesis())
	var edited struct {
		AppState map[string]json.RawMessage `json:"app_state"`
	}
	require.NoError(t, readJSON(filepath.Join(home, "config", "genesis.json"), &edited))
	var patched map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(edited.AppState["evm"], &patched))
	require.JSONEq(t, string(original["params"]), string(patched["params"]))
	var parsed evmtypes.GenesisState
	require.NoError(t, encoding.Marshaler.UnmarshalAsJSON(edited.AppState["evm"], &parsed))
	require.NoError(t, parsed.Validate())
	require.Len(t, parsed.Codes, 1)
	require.Equal(t, v6Token, common.HexToAddress(parsed.Codes[0].Address))
	require.Error(t, prepared.configureV6Genesis(), "do not silently replace existing token code")
	// The qualification test starts real primary/standby sequencers and their HTTP
	// transport in this process. It needs no external node, executable or env vars.
	start := repairM1092PrepareRecorder(t, false, func(ctx sdk.Context, a *app.App, _ *ecdsa.PrivateKey) {
		c, _, _, err := v6LoadContracts()
		require.NoError(t, err)
		require.NotEmpty(t, c.Token.RuntimeCode)
		a.EvmKeeper.SetCode(ctx, v6Token, c.Token.RuntimeCode)
	})
	runtime := start()
	live, err := recordV6RPC(runtime.node, runtime.payer, runtime.pins, vectorV2Provenance{
		Revision: "0a76cd72f6c808eb67379b35bfa45c17d608d093", ChainID: "pax-test",
		Generator: "daemon/committed-fixture/vector_v6.go",
		Command:   "go test -count=1 ./daemon/committed-fixture/ -run '^TestVectorV6SponsoredBindVerifies$'",
	}, false)
	require.NoError(t, err, "record real user-signed authorization, sponsored bind and plain transfer")
	require.NoError(t, verifyVectorV6(live))
	pending, err := runtime.backup.PendingSnapshot()
	require.NoError(t, err)
	require.Equal(t, ethtypes.AddressToDelegation(live.Delegate), runtime.backup.EvmKeeper.GetCode(pending, live.User), "standby must replay the actual authorization")
	public, bound := runtime.backup.EvmKeeper.GetLayerXDid(pending, live.User)
	require.True(t, bound, "standby must replay the bind within executeSponsored")
	require.Equal(t, [32]byte(live.DIDPublicKey), public)

	data, err := os.ReadFile("vectors/" + vectorV6Name + ".json")
	require.NoError(t, err)
	var recorded v6Vector
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	require.NoError(t, decoder.Decode(&recorded))
	require.NoError(t, verifyVectorV6(&recorded))
	require.NotNil(t, vectors[vectorV6Name])
	require.Equal(t, evmrpc.RungCommitted, recorded.Positive.Receipt.Rung)
	require.Equal(t, "V6-sponsored-bind", recorded.Positive.ID)
	require.Equal(t, "V6-plain-transfer", recorded.Refusal.ID)
	require.Equal(t, sha256.Sum256(recorded.PinnedKey.PublicKey), [32]byte(recorded.PinnedKey.KeyId))
	copy, err := os.ReadFile("../../sdk/platform/conformance/fixtures/" + vectorV6Name + ".json")
	require.NoError(t, err)
	require.Equal(t, data, copy, "published V6 must be byte-identical")
	for _, mutation := range []string{"raw transaction", "signed receipt", "pin", "fee", "balance", "designator", "delegate", "bind", "DID", "sponsor", "authorization", "transfer as positive"} {
		t.Run(mutation, func(t *testing.T) {
			var changed v6Vector
			require.NoError(t, json.Unmarshal(data, &changed))
			switch mutation {
			case "raw transaction":
				changed.Positive.RawTransaction[len(changed.Positive.RawTransaction)-1] ^= 1
			case "signed receipt":
				changed.Positive.SignedReceipt[len(changed.Positive.SignedReceipt)-1] ^= 1
			case "pin":
				changed.PinnedKey.PublicKey[0] ^= 1
			case "fee":
				changed.Positive.SponsorPaidFee.ToInt().Add(changed.Positive.SponsorPaidFee.ToInt(), common.Big1)
			case "balance":
				changed.Positive.SponsorBalanceAfter.ToInt().Add(changed.Positive.SponsorBalanceAfter.ToInt(), common.Big1)
			case "designator":
				changed.Positive.DelegationDesignator[3] ^= 1
			case "delegate":
				changed.Delegate[0] ^= 1
			case "bind":
				changed.BindCalldata[len(changed.BindCalldata)-1] ^= 1
			case "DID":
				changed.BoundDID += "altered"
			case "sponsor":
				changed.Sponsor[0] ^= 1
			case "authorization":
				var tx ethtypes.Transaction
				require.NoError(t, tx.UnmarshalBinary(changed.Positive.RawTransaction))
				auth := tx.SetCodeAuthorizations()[0]
				auth.Address[0] ^= 1
				authority, err := auth.Authority()
				require.NoError(t, err)
				require.NotEqual(t, changed.User, authority, "changing signed authorization must change its recovered authority")
				changed.User = authority
			case "transfer as positive":
				changed.Positive = changed.Refusal
			}
			require.Error(t, verifyVectorV6(&changed), "reject altered %s", mutation)
		})
	}
	// The positive and the refusal are independent executed transactions. The
	// refusal keeps the established code but carries no authorization or bind.
	require.NotEqual(t, recorded.Positive.Receipt.TxHash, recorded.Refusal.Receipt.TxHash)
	require.Equal(t, recorded.Positive.DelegationDesignator, recorded.Refusal.DelegationDesignator)
	var transfer ethtypes.Transaction
	require.NoError(t, transfer.UnmarshalBinary(recorded.Refusal.RawTransaction))
	require.Equal(t, uint8(ethtypes.DynamicFeeTxType), transfer.Type())
	require.Error(t, v6SponsoredExecution(&recorded, &transfer))
	require.Equal(t, evmtypes.LayerXDid([32]byte(recorded.DIDPublicKey)), recorded.BoundDID)
}
