package cmd

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"testing"

	tmcfg "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	seqkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/keeper"
	seqtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	servercmd "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/server/cmd"
	genutiltypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/genutil/types"
	"github.com/stretchr/testify/require"
)

func TestSequencerGenesisCommandsProduceValidGenesis(t *testing.T) {
	home := t.TempDir()
	run := func(args ...string) []byte {
		t.Helper()
		root, _ := NewRootCmd()
		var stdout, stderr bytes.Buffer
		root.SetOut(&stdout)
		root.SetErr(&stderr)
		root.SetArgs(append(args, "--home", home))
		require.NoError(t, servercmd.Execute(root, home), "command %v: %s", args, stderr.String())
		return stdout.Bytes()
	}

	const chainID = "sequencer-genesis-conformance"
	run("init", "sequencer-conformance", "--chain-id", chainID, "--mode", "validator")
	receiptOutput := run("sequencer", "gen-receipt-key", "--output", filepath.Join(home, "config", "receipt-key"))
	var receipt struct {
		PublicKey string `json:"public_key"`
	}
	require.NoError(t, json.Unmarshal(receiptOutput, &receipt))
	public, err := hex.DecodeString(receipt.PublicKey)
	require.NoError(t, err)
	require.Len(t, public, ed25519.PublicKeySize)
	run("add-genesis-receipt-key", "--public-key", receipt.PublicKey, "--first-height", "1", "--last-height", "1000")

	config := tmcfg.DefaultConfig().SetRoot(home)
	validator, err := privval.LoadFilePV(config.PrivValidator.KeyFile(), config.PrivValidator.StateFile())
	require.NoError(t, err)
	leader := validator.Key.PubKey.Bytes()
	require.Len(t, leader, ed25519.PublicKeySize)
	run("add-genesis-sequencer-leader", "--pubkey", hex.EncodeToString(leader))
	run("validate-genesis")

	appState, doc, err := genutiltypes.GenesisStateFromGenFile(config.GenesisFile())
	require.NoError(t, err)
	require.Equal(t, chainID, doc.ChainID)
	require.Contains(t, appState, seqtypes.ModuleName)
	var wire struct {
		Leaders          [][]byte              `json:"leaders"`
		ReceiptKeys      seqkeeper.ReceiptKeys `json:"receipt_keys"`
		ActivationHeight uint64                `json:"activation_height"`
		LastSeq          uint64                `json:"last_seq"`
		LastSeqHash      []byte                `json:"last_seq_hash"`
	}
	require.NoError(t, json.Unmarshal(appState[seqtypes.ModuleName], &wire))
	require.Equal(t, [][]byte{leader}, wire.Leaders)
	keyID := sha256.Sum256(public)
	require.Equal(t, seqkeeper.ReceiptKeys{{
		KeyId: keyID[:], PublicKey: public,
		FirstHeight: 1, LastHeight: 1000,
	}}, wire.ReceiptKeys)
	require.Zero(t, wire.LastSeq)
	require.Zero(t, wire.ActivationHeight)
	require.Empty(t, wire.LastSeqHash)

	state := seqtypes.GenesisState{
		Leaders: [][32]byte{[32]byte(wire.Leaders[0])},
		ReceiptKeys: []seqtypes.ReceiptKey{{
			KeyId:       [32]byte(wire.ReceiptKeys[0].KeyId),
			PublicKey:   [32]byte(wire.ReceiptKeys[0].PublicKey),
			FirstHeight: wire.ReceiptKeys[0].FirstHeight,
			LastHeight:  wire.ReceiptKeys[0].LastHeight,
		}},
		ActivationHeight: wire.ActivationHeight,
		LastSeq:          wire.LastSeq,
	}
	copy(state.LastSeqHash[:], wire.LastSeqHash)
	require.NoError(t, state.Validate())
}
