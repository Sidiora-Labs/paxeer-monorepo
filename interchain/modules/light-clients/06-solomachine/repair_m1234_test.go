package solomachine_test

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	clienttypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/02-client/types"
	commitmenttypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/23-commitment/types"
	host "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/24-host"
	"github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/exported"
	solomachinetypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/light-clients/06-solomachine/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	cryptocodec "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/ed25519"
	cryptotypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/dbadapter"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/tx/signing"
)

func TestRepairM_1234_DocumentedProofRoundTrip(t *testing.T) {
	// Keep the compiled producer identical to the guide, so later doc drift fails.
	guide, err := os.ReadFile("spec/01_concepts.md")
	require.NoError(t, err)
	_, example, found := strings.Cut(string(guide), "```go\n")
	require.True(t, found)
	example, _, found = strings.Cut(example, "```")
	require.True(t, found)
	start := strings.Index(example, "func clientStateProof(")
	require.NotEqual(t, -1, start)
	source, err := os.ReadFile("repair_m1234_test.go")
	require.NoError(t, err)
	require.Contains(t, string(source), example[start:])

	cdc, key, client, attested, prefix, path := repairM1234Fixture(t)
	const timestamp uint64 = 200
	sequence := client.Sequence
	proof, err := clientStateProof(cdc, key, path, attested, sequence, timestamp, client.ConsensusState.Diversifier)
	require.NoError(t, err)

	var envelope solomachinetypes.TimestampedSignatureData
	require.NoError(t, cdc.Unmarshal(proof, &envelope))
	require.Equal(t, timestamp, envelope.Timestamp)
	decoded, err := solomachinetypes.UnmarshalSignatureData(cdc, envelope.SignatureData)
	require.NoError(t, err)
	single, ok := decoded.(*signing.SingleSignatureData)
	require.True(t, ok)
	signBz, err := solomachinetypes.ClientStateSignBytes(cdc, sequence, timestamp, client.ConsensusState.Diversifier, path, attested)
	require.NoError(t, err)
	require.NoError(t, solomachinetypes.VerifySignature(key.PubKey(), signBz, single))
	descriptorBz, err := cdc.Marshal(signing.SignatureDataToProto(single))
	require.NoError(t, err)
	require.Equal(t, descriptorBz, envelope.SignatureData)
	roundTrip, err := cdc.Marshal(&envelope)
	require.NoError(t, err)
	require.Equal(t, proof, roundTrip)

	db := dbm.NewMemDB()
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	store := dbadapter.Store{DB: db}
	require.NoError(t, client.VerifyClientState(store, cdc, clienttypes.NewHeight(0, sequence-1), prefix, "06-solomachine-1", proof, attested))
	require.Equal(t, sequence+1, client.Sequence)
	require.Equal(t, timestamp, client.ConsensusState.Timestamp)
	stored := store.Get([]byte(host.KeyClientState))
	require.NotEmpty(t, stored)
	recovered := clienttypes.MustUnmarshalClientState(cdc, stored)
	require.Equal(t, client, recovered)
	recoveredSolo, ok := recovered.(*solomachinetypes.ClientState)
	require.True(t, ok)
	recoveredKey, err := recoveredSolo.ConsensusState.GetPubKey()
	require.NoError(t, err)
	require.Equal(t, key.PubKey().Bytes(), recoveredKey.Bytes())

	// Consumption advances the sequence; the same proof cannot be replayed.
	require.Error(t, recoveredSolo.VerifyClientState(store, cdc, clienttypes.NewHeight(0, sequence-1), prefix, "06-solomachine-1", proof, attested))
	require.Equal(t, stored, store.Get([]byte(host.KeyClientState)))
}

func TestRepairM_1234_RejectInvalidProofEncoding(t *testing.T) {
	for _, invalid := range []string{"changed timestamp", "raw signature bytes"} {
		t.Run(invalid, func(t *testing.T) {
			cdc, key, client, attested, prefix, path := repairM1234Fixture(t)
			proof, err := clientStateProof(cdc, key, path, attested, client.Sequence, 200, client.ConsensusState.Diversifier)
			require.NoError(t, err)
			var envelope solomachinetypes.TimestampedSignatureData
			require.NoError(t, cdc.Unmarshal(proof, &envelope))
			switch invalid {
			case "changed timestamp":
				envelope.Timestamp++
			case "raw signature bytes":
				decoded, err := solomachinetypes.UnmarshalSignatureData(cdc, envelope.SignatureData)
				require.NoError(t, err)
				single, ok := decoded.(*signing.SingleSignatureData)
				require.True(t, ok)
				envelope.SignatureData = single.Signature
			}
			invalidProof, err := cdc.Marshal(&envelope)
			require.NoError(t, err)
			db := dbm.NewMemDB()
			t.Cleanup(func() { require.NoError(t, db.Close()) })
			store := dbadapter.Store{DB: db}
			before := clienttypes.MustMarshalClientState(cdc, client)
			store.Set([]byte(host.KeyClientState), before)
			require.Error(t, client.VerifyClientState(store, cdc, clienttypes.NewHeight(0, client.Sequence-1), prefix, "06-solomachine-1", invalidProof, attested))
			require.Equal(t, before, clienttypes.MustMarshalClientState(cdc, client))
			require.Equal(t, before, store.Get([]byte(host.KeyClientState)))
		})
	}
}

func repairM1234Fixture(t *testing.T) (*codec.ProtoCodec, cryptotypes.PrivKey, *solomachinetypes.ClientState, *solomachinetypes.ClientState, *commitmenttypes.MerklePrefix, commitmenttypes.MerklePath) {
	t.Helper()
	registry := codectypes.NewInterfaceRegistry()
	cryptocodec.RegisterInterfaces(registry)
	solomachinetypes.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	key := ed25519.GenPrivKeyFromSecret([]byte("repair-m1234-solo-machine-fixture"))
	publicKey, err := codectypes.NewAnyWithValue(key.PubKey())
	require.NoError(t, err)
	client := solomachinetypes.NewClientState(7, &solomachinetypes.ConsensusState{
		PublicKey: publicKey, Diversifier: "repair-m1234", Timestamp: 100,
	}, false)
	attested := solomachinetypes.NewClientState(3, &solomachinetypes.ConsensusState{
		PublicKey: publicKey, Diversifier: "counterparty", Timestamp: 50,
	}, false)
	require.NoError(t, client.Validate())
	require.NoError(t, attested.Validate())
	prefix := commitmenttypes.NewMerklePrefix([]byte("ibc"))
	path, err := commitmenttypes.ApplyPrefix(prefix, commitmenttypes.NewMerklePath(host.FullClientStatePath("06-solomachine-1")))
	require.NoError(t, err)
	return cdc, key, client, attested, &prefix, path
}

func clientStateProof(
	cdc codec.BinaryCodec,
	key cryptotypes.PrivKey,
	path commitmenttypes.MerklePath,
	clientState exported.ClientState,
	sequence, timestamp uint64,
	diversifier string,
) ([]byte, error) {
	signBz, err := solomachinetypes.ClientStateSignBytes(
		cdc, sequence, timestamp, diversifier, path, clientState,
	)
	if err != nil {
		return nil, err
	}

	sig, err := key.Sign(signBz)
	if err != nil {
		return nil, err
	}
	sigData := &signing.SingleSignatureData{Signature: sig}
	protoSigData := signing.SignatureDataToProto(sigData)
	bz, err := cdc.Marshal(protoSigData)
	if err != nil {
		return nil, err
	}

	return cdc.Marshal(&solomachinetypes.TimestampedSignatureData{
		SignatureData: bz,
		Timestamp:     timestamp,
	})
}
