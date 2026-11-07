package tempfile_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/ed25519"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

const repair108ChainID = "repair-108"

func repair108Vote() *tmproto.Vote {
	return &tmproto.Vote{
		Type:      tmproto.PrecommitType,
		Height:    41,
		Round:     3,
		Timestamp: time.Unix(123456789, 0).UTC(),
		BlockID: tmproto.BlockID{
			Hash: bytes.Repeat([]byte{1}, crypto.HashSize),
			PartSetHeader: tmproto.PartSetHeader{
				Total: 1,
				Hash:  bytes.Repeat([]byte{2}, crypto.HashSize),
			},
		},
	}
}

func repair108Signer(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	keyPath, statePath := filepath.Join(dir, "key.json"), filepath.Join(dir, "state.json")
	pv := privval.NewFilePV(ed25519.TestSecretKey([]byte(repair108ChainID)), keyPath, statePath)
	require.NoError(t, pv.Save())
	vote := repair108Vote()
	vote.Height--
	require.NoError(t, pv.SignVote(t.Context(), repair108ChainID, vote))
	return keyPath, statePath
}

func repair108AssertRecovered(t *testing.T, keyPath, statePath string, signed *tmproto.Vote) {
	t.Helper()
	pv, err := privval.LoadFilePV(keyPath, statePath)
	require.NoError(t, err)
	require.Equal(t, signed.Height, pv.LastSignState.Height)
	require.Equal(t, signed.Round, pv.LastSignState.Round)
	require.EqualValues(t, 3, pv.LastSignState.Step)
	require.Equal(t, signed.Signature, pv.LastSignState.Signature)
	require.Equal(t, types.VoteSignBytes(repair108ChainID, signed), []byte(pv.LastSignState.SignBytes))
	sig, err := crypto.SigFromBytes(signed.Signature)
	require.NoError(t, err)
	require.NoError(t, pv.Key.PubKey.Verify(pv.LastSignState.SignBytes, sig))

	retry := repair108Vote()
	require.NoError(t, pv.SignVote(t.Context(), repair108ChainID, retry))
	require.Equal(t, signed.Signature, retry.Signature)
	for _, regression := range []string{"height", "round", "step", "conflicting-block"} {
		t.Run(regression, func(t *testing.T) {
			vote := repair108Vote()
			switch regression {
			case "height":
				vote.Height--
			case "round":
				vote.Round--
			case "step":
				vote.Type = tmproto.PrevoteType
			case "conflicting-block":
				vote.BlockID.Hash[0]++
			}
			require.Error(t, pv.SignVote(t.Context(), repair108ChainID, vote))
			require.Empty(t, vote.Signature)
		})
	}
}

func TestRepair_108_SignerRestartRecovery(t *testing.T) {
	if dir := os.Getenv("REPAIR_108_CRASH_CHILD"); dir != "" {
		pv, err := privval.LoadFilePV(filepath.Join(dir, "key.json"), filepath.Join(dir, "state.json"))
		require.NoError(t, err)
		vote := repair108Vote()
		require.NoError(t, pv.SignVote(t.Context(), repair108ChainID, vote))
		require.NoError(t, json.NewEncoder(os.Stdout).Encode(vote))
		var stop [1]byte
		_, err = os.Stdin.Read(stop[:])
		t.Fatalf("child must be killed after signing: %v", err)
	}
	keyPath, statePath := repair108Signer(t)
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestRepair_108_SignerRestartRecovery$")
	cmd.Env = append(os.Environ(), "REPAIR_108_CRASH_CHILD="+filepath.Dir(statePath))
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdin, err := cmd.StdinPipe()
	require.NoError(t, err)
	defer stdin.Close()
	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)
	require.NoError(t, cmd.Start())
	t.Cleanup(func() { _ = cmd.Process.Kill() })
	var signed tmproto.Vote
	err = json.NewDecoder(stdout).Decode(&signed)
	if err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatalf("read signed vote: %v; stderr: %s", err, stderr.String())
	}
	require.NoError(t, cmd.Process.Kill())
	require.Error(t, cmd.Wait())
	require.NoError(t, ctx.Err())
	repair108AssertRecovered(t, keyPath, statePath, &signed)
}
