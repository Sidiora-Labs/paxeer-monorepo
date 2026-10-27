package cmd

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	sequencertypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	genutiltypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/genutil/types"
	"github.com/stretchr/testify/require"
)

func receiptGenesisPublicKey(seedByte byte) [32]byte {
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{seedByte}, ed25519.SeedSize))
	var public [32]byte
	copy(public[:], private.Public().(ed25519.PublicKey))
	return public
}

func writeReceiptGenesis(t *testing.T, home string, state *sequencertypes.GenesisState) string {
	t.Helper()
	appState := map[string]json.RawMessage{"bank": json.RawMessage(`{"balances":[],"supply":[]}`)}
	if state != nil {
		raw, err := json.Marshal(state)
		require.NoError(t, err)
		appState[sequencertypes.ModuleName] = raw
	}
	raw, err := json.Marshal(appState)
	require.NoError(t, err)
	doc := &tmtypes.GenesisDoc{
		ChainID:       "receipt-key-test",
		GenesisTime:   time.Unix(1700000000, 0).UTC(),
		InitialHeight: 1,
		AppState:      raw,
	}
	require.NoError(t, doc.ValidateAndComplete())
	path := filepath.Join(home, "config", "genesis.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
	require.NoError(t, doc.SaveAs(path))
	return path
}

func TestAddGenesisReceiptKeyPinsKeyInSequencerGenesis(t *testing.T) {
	public := receiptGenesisPublicKey(1)
	previousPublic := receiptGenesisPublicKey(2)
	previous := ReceiptKey{KeyId: sha256.Sum256(previousPublic[:]), PublicKey: previousPublic, FirstHeight: 1, LastHeight: 500}
	for _, name := range []string{"empty sequencer", "existing key", "missing sequencer"} {
		t.Run(name, func(t *testing.T) {
			home := t.TempDir()
			state := sequencertypes.GenesisState{}
			if name == "existing key" {
				state = sequencertypes.GenesisState{
					Leaders:          [][32]byte{previousPublic},
					ReceiptKeys:      []ReceiptKey{previous},
					ActivationHeight: 1,
					LastSeq:          3,
					LastSeqHash:      [32]byte{4},
				}
			}
			initial := &state
			if name == "missing sequencer" {
				initial = nil
			}
			path := writeReceiptGenesis(t, home, initial)
			beforeApp, beforeDoc, err := genutiltypes.GenesisStateFromGenFile(path)
			require.NoError(t, err)
			cmd := AddGenesisReceiptKeyCmd(home)
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			args := []string{"--public-key", hex.EncodeToString(public[:]), "--first-height", "1", "--last-height", "1000"}
			if name == "existing key" {
				cmd = AddGenesisReceiptKeyCmd(filepath.Join(t.TempDir(), "unused"))
				args = append(args, "--home", home)
			}
			cmd.SetArgs(args)
			require.NoError(t, cmd.Execute())

			afterApp, afterDoc, err := genutiltypes.GenesisStateFromGenFile(path)
			require.NoError(t, err)
			var got sequencertypes.GenesisState
			require.NoError(t, json.Unmarshal(afterApp[sequencertypes.ModuleName], &got))
			key := ReceiptKey{KeyId: sha256.Sum256(public[:]), PublicKey: public, FirstHeight: 1, LastHeight: 1000}
			require.NotEqual(t, [32]byte{}, key.KeyId)
			state.ReceiptKeys = append(state.ReceiptKeys, key)
			require.Equal(t, state, got)
			require.True(t, key.Covers(1))
			require.True(t, key.Covers(1000))
			require.JSONEq(t, string(beforeApp["bank"]), string(afterApp["bank"]))
			require.Len(t, afterApp, 2)
			afterDoc.AppState = beforeDoc.AppState
			require.Equal(t, beforeDoc, afterDoc)
			if name == "existing key" {
				require.NoError(t, got.Validate())
			}
		})
	}
}

func TestAddGenesisReceiptKeyRefusesInvertedRange(t *testing.T) {
	public := receiptGenesisPublicKey(1)
	pinnedPublic := receiptGenesisPublicKey(2)
	for _, tc := range []struct {
		name, publicKey, first, last, wantError string
	}{
		{"inverted range", hex.EncodeToString(public[:]), "10", "5", "above last-height"},
		{"zero first height", hex.EncodeToString(public[:]), "0", "1000", "greater than zero"},
		{"zero last height", hex.EncodeToString(public[:]), "1", "0", "above last-height"},
		{"noncanonical zero", strings.Repeat("00", 32), "1", "1000", "not a canonical"},
		{"noncanonical identity", "01" + strings.Repeat("00", 31), "1", "1000", "not a canonical"},
		{"noncanonical field", "ed" + strings.Repeat("ff", 30) + "7f", "1", "1000", "not a canonical"},
		{"duplicate id", hex.EncodeToString(pinnedPublic[:]), "501", "1000", "already pinned"},
		{"invalid hex", "zz" + strings.Repeat("00", 31), "1", "1000", "hex-encoded 32-byte"},
		{"short public key", strings.Repeat("02", 31), "1", "1000", "hex-encoded 32-byte"},
		{"long public key", strings.Repeat("02", 33), "1", "1000", "hex-encoded 32-byte"},
		{"negative height", hex.EncodeToString(public[:]), "-1", "1000", "invalid argument"},
		{"height overflow", hex.EncodeToString(public[:]), "1", "18446744073709551616", "value out of range"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			state := sequencertypes.GenesisState{
				Leaders: [][32]byte{pinnedPublic},
				ReceiptKeys: []ReceiptKey{{
					KeyId: sha256.Sum256(pinnedPublic[:]), PublicKey: pinnedPublic, FirstHeight: 1, LastHeight: 500,
				}},
			}
			require.NoError(t, state.Validate())
			path := writeReceiptGenesis(t, home, &state)
			before, err := os.ReadFile(path)
			require.NoError(t, err)
			cmd := AddGenesisReceiptKeyCmd(home)
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			cmd.SetArgs([]string{"--public-key", tc.publicKey, "--first-height", tc.first, "--last-height", tc.last})
			require.ErrorContains(t, cmd.Execute(), tc.wantError)
			after, err := os.ReadFile(path)
			require.NoError(t, err)
			require.Equal(t, before, after, "rejected input must leave genesis.json byte-identical")
		})
	}
}
