package cmd

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/verify"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	"github.com/stretchr/testify/require"
)

func LoadReceiptKey(t *testing.T, path string, public [32]byte) *sequencer.ReceiptKey {
	t.Helper()
	dir := t.TempDir()
	consensusPath := filepath.Join(dir, "priv_validator_key.json")
	validator, err := privval.GenFilePV(consensusPath, filepath.Join(dir, "priv_validator_state.json"), "")
	require.NoError(t, err)
	require.NoError(t, validator.Key.Save())
	key, err := sequencer.LoadReceiptKey(path, consensusPath, []verify.SequencerAuthorization{{
		SequencerID:      [32]byte{1},
		PublicKey:        public,
		FirstBatchNumber: 1,
		LastBatchNumber:  10,
	}}, 1)
	require.NoError(t, err)
	return key
}

func TestGenReceiptKeyWritesLoadableSeed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "receipt_key")
	var stdout, stderr bytes.Buffer
	cmd := SequencerCmd()
	require.Equal(t, "sequencer", cmd.Use)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"gen-receipt-key", "--output", path})
	require.NoError(t, cmd.Execute())
	require.Empty(t, stderr.String())

	info, err := os.Stat(path)
	require.NoError(t, err)
	require.True(t, info.Mode().IsRegular())
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	seedHex := strings.TrimSuffix(string(raw), "\n")
	require.Len(t, seedHex, hex.EncodedLen(ed25519.SeedSize))
	seed, err := hex.DecodeString(seedHex)
	require.NoError(t, err)
	require.Len(t, seed, ed25519.SeedSize)
	public := [32]byte(ed25519.NewKeyFromSeed(seed).Public().(ed25519.PublicKey))

	var result map[string]string
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &result))
	require.Equal(t, map[string]string{"public_key": hex.EncodeToString(public[:])}, result)
	require.NotContains(t, stdout.String(), seedHex)
	require.NotContains(t, stderr.String(), seedHex)
	key := LoadReceiptKey(t, path, public)
	require.Equal(t, public, key.PublicKey())
	keyID, err := key.KeyID(1)
	require.NoError(t, err)
	require.Equal(t, [32]byte{1}, keyID)
}

func TestGenReceiptKeyRefusesExistingFile(t *testing.T) {
	for _, symlink := range []bool{false, true} {
		name := "regular file"
		if symlink {
			name = "symlink"
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			existingPath := filepath.Join(dir, "existing_key")
			original := []byte(strings.Repeat("ab", ed25519.SeedSize) + "\n")
			require.NoError(t, os.WriteFile(existingPath, original, 0o600))
			before, err := os.Stat(existingPath)
			require.NoError(t, err)
			path := existingPath
			if symlink {
				path = filepath.Join(dir, "receipt_key")
				require.NoError(t, os.Symlink(existingPath, path))
			}
			var stdout, stderr bytes.Buffer
			cmd := SequencerCmd()
			cmd.SilenceErrors = true
			cmd.SilenceUsage = true
			cmd.SetOut(&stdout)
			cmd.SetErr(&stderr)
			cmd.SetArgs([]string{"gen-receipt-key", "--output", path})
			err = cmd.Execute()
			require.ErrorIs(t, err, os.ErrExist)
			require.Empty(t, stdout.String())
			require.NotContains(t, stderr.String(), strings.TrimSpace(string(original)))
			after, err := os.Stat(existingPath)
			require.NoError(t, err)
			require.True(t, os.SameFile(before, after))
			require.Equal(t, before.Mode(), after.Mode())
			require.Equal(t, before.ModTime(), after.ModTime())
			contents, err := os.ReadFile(existingPath)
			require.NoError(t, err)
			require.Equal(t, original, contents)
			if symlink {
				target, err := os.Readlink(path)
				require.NoError(t, err)
				require.Equal(t, existingPath, target)
			}
		})
	}
}
