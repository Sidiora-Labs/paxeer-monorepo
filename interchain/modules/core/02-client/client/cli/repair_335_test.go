package cli_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	clientcli "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/02-client/client/cli"
	clienttypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/02-client/types"
	commitmenttypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/23-commitment/types"
	host "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/24-host"
	solotypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/light-clients/06-solomachine/types"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/light-clients/07-tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	cryptocodec "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	rootmultiv2 "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/storev2/rootmulti"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/tx/signing"
	authtx "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/tx"
	upgradekeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/keeper"
	upgradetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func repair335Context() (client.Context, *codec.ProtoCodec) {
	registry := codectypes.NewInterfaceRegistry()
	sdk.RegisterInterfaces(registry)
	cryptocodec.RegisterInterfaces(registry)
	clienttypes.RegisterInterfaces(registry)
	solotypes.RegisterInterfaces(registry)
	tmtypes.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	address := sdk.AccAddress(bytes.Repeat([]byte{1}, 20))
	ctx := client.Context{}.WithCodec(cdc).WithInterfaceRegistry(registry).
		WithTxConfig(authtx.NewTxConfig(cdc, authtx.DefaultSignModes)).
		WithChainID("receiver-0").WithFrom(address.String()).WithFromAddress(address).
		WithGenerateOnly(true).WithOffline(true)
	return ctx, cdc
}

func repair335Execute(ctx client.Context, cmd *cobra.Command, args ...string) ([]byte, error) {
	var output bytes.Buffer
	ctx = ctx.WithOutput(&output)
	cmd.SetContext(context.WithValue(context.Background(), client.ClientContextKey, &ctx))
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetArgs(args)
	err := cmd.Execute()
	return output.Bytes(), err
}

func repair335Message(t *testing.T, ctx client.Context, output []byte) sdk.Msg {
	t.Helper()
	tx, err := ctx.TxConfig.TxJSONDecoder()(output)
	require.NoError(t, err, "%s", output)
	require.Len(t, tx.GetMsgs(), 1)
	// Exercise protobuf transaction transport as well as the CLI's JSON output.
	encoded, err := ctx.TxConfig.TxEncoder()(tx)
	require.NoError(t, err)
	decoded, err := ctx.TxConfig.TxDecoder()(encoded)
	require.NoError(t, err)
	require.Len(t, decoded.GetMsgs(), 1)
	return decoded.GetMsgs()[0]
}

func TestRepair_335_MisbehaviourFileDecode(t *testing.T) {
	ctx, cdc := repair335Context()
	key := secp256k1.GenPrivKey()
	publicKey, err := codectypes.NewAnyWithValue(key.PubKey())
	require.NoError(t, err)
	consensus := &solotypes.ConsensusState{PublicKey: publicKey, Diversifier: "repair335", Timestamp: 10}
	state := solotypes.NewClientState(1, consensus, false)
	dataOne, err := solotypes.ClientStateDataBytes(cdc, commitmenttypes.NewMerklePath(host.FullClientStatePath("06-solomachine-1")), state)
	require.NoError(t, err)
	dataTwo, err := solotypes.ConsensusStateDataBytes(cdc, commitmenttypes.NewMerklePath(host.FullConsensusStatePath("06-solomachine-1", clienttypes.NewHeight(0, 1))), consensus)
	require.NoError(t, err)
	sign := func(dataType solotypes.DataType, data []byte, timestamp uint64) *solotypes.SignatureAndData {
		signBytes, err := solotypes.MisbehaviourSignBytes(cdc, 1, timestamp, consensus.Diversifier, dataType, data)
		require.NoError(t, err)
		signature, err := key.Sign(signBytes)
		require.NoError(t, err)
		encoded, err := cdc.Marshal(signing.SignatureDataToProto(&signing.SingleSignatureData{Signature: signature}))
		require.NoError(t, err)
		return &solotypes.SignatureAndData{Signature: encoded, DataType: dataType, Data: data, Timestamp: timestamp}
	}
	evidence := &solotypes.Misbehaviour{ClientId: "06-solomachine-0", Sequence: 1,
		SignatureOne: sign(solotypes.CLIENT, dataOne, 10), SignatureTwo: sign(solotypes.CONSENSUS, dataTwo, 11)}
	require.NoError(t, evidence.ValidateBasic())
	content, err := cdc.MarshalInterfaceJSON(evidence)
	require.NoError(t, err)
	filename := filepath.Join(t.TempDir(), "misbehaviour.json")
	require.NoError(t, os.WriteFile(filename, content, 0600))
	inline, err := repair335Execute(ctx, clientcli.NewSubmitMisbehaviourCmd(), string(content))
	require.NoError(t, err)
	fromFile, err := repair335Execute(ctx, clientcli.NewSubmitMisbehaviourCmd(), filename)
	require.NoError(t, err)
	require.JSONEq(t, string(inline), string(fromFile))
	msg, ok := repair335Message(t, ctx, fromFile).(*clienttypes.MsgSubmitMisbehaviour)
	require.True(t, ok)
	require.Equal(t, evidence.ClientId, msg.ClientId)
	require.Equal(t, ctx.GetFromAddress().String(), msg.Signer)
	require.NoError(t, msg.ValidateBasic())
	decoded, err := clienttypes.UnpackMisbehaviour(msg.Misbehaviour)
	require.NoError(t, err)
	require.IsType(t, evidence, decoded)
	require.Equal(t, evidence, decoded)
	storeKey := sdk.NewKVStoreKey("client")
	sdkCtx := testutil.DefaultContext(storeKey, sdk.NewTransientStoreKey("transient"))
	frozen, err := state.CheckMisbehaviourAndUpdateState(sdkCtx, cdc, sdkCtx.KVStore(storeKey), decoded)
	require.NoError(t, err)
	require.True(t, frozen.(*solotypes.ClientState).IsFrozen)
	for _, malformed := range []string{`{"@type":`, `{"@type":"/unregistered.Misbehaviour"}`, `{"@type":"/ibc.lightclients.solomachine.v2.Misbehaviour","sequence":"not-a-number"}`} {
		require.NoError(t, os.WriteFile(filename, []byte(malformed), 0600))
		output, err := repair335Execute(ctx, clientcli.NewSubmitMisbehaviourCmd(), filename)
		require.ErrorContains(t, err, "error unmarshalling misbehaviour file")
		require.Empty(t, output)
	}
}

func TestRepair_335_UpgradeProofEncoding(t *testing.T) {
	ctx, cdc := repair335Context()
	scConfig := config.DefaultStateCommitConfig()
	scConfig.MemIAVLConfig.AsyncCommitBuffer = 0
	producer := rootmultiv2.NewStore(t.TempDir(), scConfig, config.StateStoreConfig{}, nil)
	t.Cleanup(func() { require.NoError(t, producer.Close()) })
	upgradeKey := sdk.NewKVStoreKey(upgradetypes.StoreKey)
	producer.MountStoreWithDB(upgradeKey, sdk.StoreTypeIAVL, nil)
	require.NoError(t, producer.LoadLatestVersion())
	now := time.Unix(1700000000, 0).UTC()
	upgradePath := []string{upgradetypes.StoreKey, upgradetypes.KeyUpgradedIBCState}
	lastHeight := clienttypes.NewHeight(0, 2)
	nextHeight := clienttypes.NewHeight(1, 1)
	oldClient := tmtypes.NewClientState("counterparty-0", tmtypes.DefaultTrustLevel, time.Hour, 24*time.Hour,
		time.Minute, lastHeight, commitmenttypes.GetSDKSpecs(), upgradePath, false, false)
	upgraded := tmtypes.NewClientState("counterparty-1", tmtypes.DefaultTrustLevel, time.Hour, 48*time.Hour,
		time.Minute, nextHeight, commitmenttypes.GetSDKSpecs(), upgradePath, false, false).ZeroCustomFields()
	upgradedConsensus := &tmtypes.ConsensusState{Timestamp: now, NextValidatorsHash: bytes.Repeat([]byte{2}, 32)}
	clientBytes, err := cdc.MarshalInterface(upgraded)
	require.NoError(t, err)
	consensusBytes, err := cdc.MarshalInterface(upgradedConsensus)
	require.NoError(t, err)
	cache := producer.CacheMultiStore()
	producerCtx := sdk.NewContext(cache, tmproto.Header{ChainID: "counterparty-0", Height: 1, Time: now}, false)
	keeper := upgradekeeper.NewKeeper(nil, upgradeKey, cdc, t.TempDir(), nil)
	require.NoError(t, keeper.SetUpgradedClient(producerCtx, 2, clientBytes))
	require.NoError(t, keeper.SetUpgradedConsensusState(producerCtx, 2, consensusBytes))
	cache.Write()
	commit := producer.Commit(true)
	require.Equal(t, int64(1), commit.Version)
	require.Len(t, commit.Hash, 32)
	queryProof := func(key, value []byte) []byte {
		response := producer.Query(context.Background(), abci.RequestQuery{
			Path: "/upgrade/key", Data: key, Height: commit.Version, Prove: true,
		})
		require.Zero(t, response.Code, response.Log)
		require.Equal(t, commit.Version, response.Height)
		require.Equal(t, value, response.Value)
		require.NotNil(t, response.ProofOps)
		require.Len(t, response.ProofOps.Ops, 2)
		// The same proof conversion and protobuf encoding used by QueryTendermintProof.
		proof, err := commitmenttypes.ConvertProofs(response.ProofOps)
		require.NoError(t, err)
		encoded, err := cdc.Marshal(&proof)
		require.NoError(t, err)
		require.GreaterOrEqual(t, bytes.IndexByte(encoded, 0), 0, "proof includes binary bytes unsuitable for argv")
		return encoded
	}
	clientProof := queryProof(upgradetypes.UpgradedClientKey(2), clientBytes)
	consensusProof := queryProof(upgradetypes.UpgradedConsStateKey(2), consensusBytes)
	clientJSON, err := cdc.MarshalInterfaceJSON(upgraded)
	require.NoError(t, err)
	consensusJSON, err := cdc.MarshalInterfaceJSON(upgradedConsensus)
	require.NoError(t, err)
	// Transfer the proof's JSON byte-field value directly into argv.
	proofArgument := func(proof []byte) string {
		encoded, err := json.Marshal(proof)
		require.NoError(t, err)
		var argument string
		require.NoError(t, json.Unmarshal(encoded, &argument))
		require.Equal(t, base64.StdEncoding.EncodeToString(proof), argument)
		return argument
	}
	args := []string{"07-tendermint-0", string(clientJSON), string(consensusJSON), proofArgument(clientProof), proofArgument(consensusProof)}
	output, err := repair335Execute(ctx, clientcli.NewUpgradeClientCmd(), args...)
	require.NoError(t, err)
	msg, ok := repair335Message(t, ctx, output).(*clienttypes.MsgUpgradeClient)
	require.True(t, ok)
	require.NoError(t, msg.ValidateBasic())
	require.Equal(t, args[0], msg.ClientId)
	require.Equal(t, ctx.GetFromAddress().String(), msg.Signer)
	require.Equal(t, clientProof, msg.ProofUpgradeClient)
	require.Equal(t, consensusProof, msg.ProofUpgradeConsensusState)
	decodedClient, err := clienttypes.UnpackClientState(msg.ClientState)
	require.NoError(t, err)
	decodedConsensus, err := clienttypes.UnpackConsensusState(msg.ConsensusState)
	require.NoError(t, err)
	require.Equal(t, upgraded, decodedClient)
	require.Equal(t, upgradedConsensus, decodedConsensus)
	receiverKey := sdk.NewKVStoreKey("client")
	receiverCtx := testutil.DefaultContext(receiverKey, sdk.NewTransientStoreKey("transient")).
		WithBlockHeader(tmproto.Header{ChainID: "receiver-0", Height: 10, Time: now.Add(time.Minute)})
	receiverStore := receiverCtx.KVStore(receiverKey)
	tmtypes.SetConsensusState(receiverStore, cdc, tmtypes.NewConsensusState(now, commitmenttypes.NewMerkleRoot(commit.Hash), bytes.Repeat([]byte{3}, 32)), lastHeight)
	// Valid protobuf proof bytes for the wrong key must still fail membership verification.
	_, _, err = oldClient.VerifyUpgradeAndUpdateState(receiverCtx, cdc, receiverStore, decodedClient, decodedConsensus,
		msg.ProofUpgradeConsensusState, msg.ProofUpgradeClient)
	require.Error(t, err)
	_, processed := tmtypes.GetProcessedTime(receiverStore, nextHeight)
	require.False(t, processed)
	newClient, newConsensus, err := oldClient.VerifyUpgradeAndUpdateState(receiverCtx, cdc, receiverStore, decodedClient, decodedConsensus,
		msg.ProofUpgradeClient, msg.ProofUpgradeConsensusState)
	require.NoError(t, err)
	require.Equal(t, nextHeight, newClient.GetLatestHeight())
	require.Equal(t, "counterparty-1", newClient.(*tmtypes.ClientState).ChainId)
	require.Equal(t, oldClient.TrustingPeriod, newClient.(*tmtypes.ClientState).TrustingPeriod)
	require.Equal(t, upgradedConsensus.Timestamp, newConsensus.(*tmtypes.ConsensusState).Timestamp)
	require.Equal(t, upgradedConsensus.NextValidatorsHash, newConsensus.(*tmtypes.ConsensusState).NextValidatorsHash)
	require.Equal(t, []byte(tmtypes.SentinelRoot), newConsensus.GetRoot().GetHash())
	processedTime, found := tmtypes.GetProcessedTime(receiverStore, nextHeight)
	require.True(t, found)
	require.Equal(t, uint64(receiverCtx.BlockTime().UnixNano()), processedTime)
	processedHeight, found := tmtypes.GetProcessedHeight(receiverStore, nextHeight)
	require.True(t, found)
	require.Equal(t, clienttypes.NewHeight(0, 10), processedHeight)
	require.Contains(t, clientcli.NewUpgradeClientCmd().Long, "canonical padded standard base64")
	for _, malformed := range []string{"", "not-base64!", "Zg", "Zg=", "Zh==", "_w==", " Zg==", "Zg==\n", "Zg==\r\n", string(clientProof)} {
		for _, index := range []int{3, 4} {
			invalidArgs := append([]string(nil), args...)
			invalidArgs[index] = malformed
			output, err := repair335Execute(ctx, clientcli.NewUpgradeClientCmd(), invalidArgs...)
			expected := "invalid upgrade client proof"
			if index == 4 {
				expected = "invalid upgrade consensus state proof"
			}
			require.ErrorContains(t, err, expected)
			require.Empty(t, output)
		}
	}
}
