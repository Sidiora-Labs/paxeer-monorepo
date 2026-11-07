package app

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	sequencermodule "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer"
	seqtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	"github.com/stretchr/testify/require"
)

func repair588Example(t *testing.T) ([]byte, mergePlanState, int64) {
	t.Helper()
	bz, err := os.ReadFile("../tools/release/upgradeinfo/merge-plan-info.example.json")
	require.NoError(t, err)
	var state mergePlanState
	require.NoError(t, json.Unmarshal(bz, &state))
	var metadata struct {
		PlanHeight int64 `json:"plan_height"`
	}
	require.NoError(t, json.Unmarshal(bz, &metadata))
	require.Positive(t, metadata.PlanHeight)
	return bz, state, metadata.PlanHeight
}

func repair588Tool(t *testing.T, success bool, args ...string) []byte {
	t.Helper()
	cmd := exec.Command("python3", append([]string{"../tools/release/upgrade_info.py"}, args...)...)
	output, err := cmd.CombinedOutput()
	if success {
		require.NoError(t, err, "%s", output)
	} else {
		require.Error(t, err, "%s", output)
		require.Contains(t, string(output), "upgrade_info:")
	}
	return output
}

func TestRepair_22_31_588_PlanInfoDecodes(t *testing.T) {
	bz, state, height := repair588Example(t)
	require.NotEmpty(t, state.Leaders)
	leaders := make([][32]byte, len(state.Leaders))
	for i, leader := range state.Leaders {
		require.Len(t, leader, 32)
		copy(leaders[i][:], leader)
	}
	require.NoError(t, seqtypes.ValidateLeaders(leaders))
	require.NotEmpty(t, state.ReceiptKeys)
	for _, entry := range state.ReceiptKeys {
		require.NoError(t, entry.Validate())
	}
	require.NoError(t, state.ReceiptKeys.Validate())
	require.Zero(t, state.LastSeq)
	require.Equal(t, make([]byte, 32), state.LastSeqHash)
	require.Equal(t, uint64(height+1), state.ActivationHeight)
	require.NoError(t, (sequencermodule.AppModuleBasic{}).ValidateGenesis(nil, nil, bz))

	// Reproduce the checked-in example through the real producer.
	args := []string{"plan-info", "--plan-height", strconv.FormatInt(height, 10),
		"--activation-height", strconv.FormatUint(state.ActivationHeight, 10),
		"--last-seq", "0", "--last-seq-hash", base64.StdEncoding.EncodeToString(state.LastSeqHash)}
	for _, leader := range state.Leaders {
		args = append(args, "--leader", base64.StdEncoding.EncodeToString(leader))
	}
	for _, entry := range state.ReceiptKeys {
		encoded, err := json.Marshal(entry)
		require.NoError(t, err)
		args = append(args, "--receipt-key", string(encoded))
	}
	require.JSONEq(t, string(bz), string(repair588Tool(t, true, args...)))
}

func TestRepair_22_31_588_PlanInfoCoversFirstHeight(t *testing.T) {
	_, state, planHeight := repair588Example(t)
	firstHeight := uint64(planHeight) + 1
	covered := false
	for _, entry := range state.ReceiptKeys {
		if entry.FirstHeight <= firstHeight && firstHeight <= entry.LastHeight {
			pinned, err := state.ReceiptKeys.KeyForHeight([32]byte(entry.KeyId), firstHeight)
			require.NoError(t, err)
			require.Equal(t, entry, pinned)
			covered = true
		}
	}
	require.True(t, covered, "example must pin a receipt key for plan height plus one")
}

func TestRepair_22_31_588_OperatorInfo(t *testing.T) {
	_, _, height := repair588Example(t)
	dir := t.TempDir()
	sums := filepath.Join(dir, "SHA256SUMS")
	identityBytes, err := os.ReadFile("../tools/release/version.json")
	require.NoError(t, err)
	var identity struct {
		Version string `json:"version"`
	}
	require.NoError(t, json.Unmarshal(identityBytes, &identity))
	checksum := strings.Repeat("a", 64)
	require.NoError(t, os.WriteFile(sums, []byte(fmt.Sprintf("%s  paxd-%s-linux-amd64\n", checksum, identity.Version)), 0o600))
	document := filepath.Join(dir, "release.json")
	example := "../tools/release/upgradeinfo/merge-plan-info.example.json"
	repair588Tool(t, true, "release", "--sums", sums, "--plan-info", example, "--out", document)
	proposalBytes, err := os.ReadFile(document)
	require.NoError(t, err)
	var proposal struct {
		Name string `json:"name"`
		Info string `json:"info"`
	}
	require.NoError(t, json.Unmarshal(proposalBytes, &proposal))
	require.Equal(t, MergeUpgrade, proposal.Name)
	var info map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(proposal.Info), &info))
	require.NotEmpty(t, info["binaries"])
	regenerated := repair588Tool(t, true, "plan-info", "--check", document, "--plan-height", strconv.FormatInt(height, 10))
	require.Empty(t, regenerated)

	wrapper := NewTestWrapper(t, time.Now().UTC(), secp256k1.GenPrivKey().PubKey(), true)
	a, ctx := wrapper.App, wrapper.Ctx
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	a.UpgradeKeeper.ClearUpgradePlan(ctx)
	path, err := a.UpgradeKeeper.GetUpgradeInfoPath()
	require.NoError(t, err)
	home := filepath.Dir(filepath.Dir(path))
	for _, args := range [][]string{
		{"--release-doc", document, "--sha256", checksum},
		{"--plan-info", example},
	} {
		repair588Tool(t, true, append([]string{"node", "--home", home, "--height", strconv.FormatInt(height, 10)}, args...)...)
		plan, err := a.resolveNamedUpgradePlan(ctx, MergeUpgrade, height)
		require.NoError(t, err)
		var state mergePlanState
		require.NoError(t, json.Unmarshal([]byte(plan.Info), &state))
		_, expected, _ := repair588Example(t)
		require.Equal(t, expected, state)
		if args[0] == "--release-doc" {
			require.Equal(t, proposal.Info, plan.Info)
		}
		stat, err := os.Stat(path)
		require.NoError(t, err)
		require.Equal(t, os.FileMode(0o600), stat.Mode().Perm())
	}
	// A node cannot silently discard genesis or accept a plan at another height.
	repair588Tool(t, false, "node", "--home", t.TempDir(), "--height", strconv.FormatInt(height, 10))
	repair588Tool(t, false, "node", "--home", t.TempDir(), "--height", strconv.FormatInt(height+1, 10), "--release-doc", document, "--sha256", checksum)

	// The other supported ordering starts with a binary-only release document.
	binaryDoc := filepath.Join(dir, "binaries.json")
	repair588Tool(t, true, "release", "--sums", sums, "--out", binaryDoc)
	_, state, _ := repair588Example(t)
	entry, err := json.Marshal(state.ReceiptKeys[0])
	require.NoError(t, err)
	merged := repair588Tool(t, true, "plan-info", "--leader", base64.StdEncoding.EncodeToString(state.Leaders[0]),
		"--receipt-key", string(entry), "--activation-height", strconv.FormatUint(state.ActivationHeight, 10),
		"--plan-height", strconv.FormatInt(height, 10), "--release-doc", binaryDoc)
	var mergedInfo map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(merged, &mergedInfo))
	require.JSONEq(t, string(info["binaries"]), string(mergedInfo["binaries"]))
}

func TestRepair_22_31_588_InvalidPlanInfo(t *testing.T) {
	bz, _, height := repair588Example(t)
	b64 := func(b []byte) string { return base64.StdEncoding.EncodeToString(b) }
	cases := []struct {
		name   string
		change func(map[string]any)
	}{
		{"empty leaders", func(p map[string]any) { p["leaders"] = []any{} }},
		{"zero leader", func(p map[string]any) { p["leaders"] = []any{b64(make([]byte, 32))} }},
		{"duplicate leader", func(p map[string]any) { v := p["leaders"].([]any); p["leaders"] = append(v, v[0]) }},
		{"empty receipts", func(p map[string]any) { p["receipt_keys"] = []any{} }},
		{"duplicate receipt", func(p map[string]any) { v := p["receipt_keys"].([]any); p["receipt_keys"] = append(v, v[0]) }},
		{"last sequence", func(p map[string]any) { p["last_seq"] = 1 }},
		{"last hash", func(p map[string]any) { p["last_seq_hash"] = b64(bytes.Repeat([]byte{1}, 32)) }},
		{"short hash", func(p map[string]any) { p["last_seq_hash"] = b64([]byte{1}) }},
		{"activation uncovered", func(p map[string]any) { p["activation_height"] = 1 }},
		{"activation overflow", func(p map[string]any) { p["activation_height"] = uint64(math.MaxInt64) + 1 }},
		{"plan overflow", func(p map[string]any) { p["plan_height"] = uint64(math.MaxInt64) }},
	}
	for _, tc := range []struct {
		name, field string
		value       any
	}{
		{"zero id", "key_id", b64(make([]byte, 32))},
		{"short id", "key_id", b64([]byte{1})},
		{"short public key", "public_key", b64([]byte{1})},
		{"noncanonical public key", "public_key", b64(bytes.Repeat([]byte{255}, 32))},
		{"identity public key", "public_key", b64(append([]byte{1}, make([]byte, 31)...))},
		{"invalid base64", "public_key", "!"},
		{"zero first", "first_height", 0},
		{"inverted range", "last_height", 1},
		{"negative range", "first_height", -1},
		{"fractional range", "first_height", 1.5},
		{"boolean range", "first_height", true},
		{"overflow range", "last_height", json.RawMessage("18446744073709551616")},
	} {
		cases = append(cases, struct {
			name   string
			change func(map[string]any)
		}{tc.name, func(p map[string]any) {
			for _, entry := range p["receipt_keys"].([]any) {
				entry.(map[string]any)[tc.field] = tc.value
			}
		}})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var info map[string]any
			decoder := json.NewDecoder(bytes.NewReader(bz))
			decoder.UseNumber()
			require.NoError(t, decoder.Decode(&info))
			tc.change(info)
			encoded, err := json.Marshal(info)
			require.NoError(t, err)
			path := filepath.Join(t.TempDir(), "invalid.json")
			require.NoError(t, os.WriteFile(path, encoded, 0o600))
			repair588Tool(t, false, "plan-info", "--check", path)
		})
	}
	// Coverage at activation does not substitute for coverage of H+1.
	var info map[string]any
	decoder := json.NewDecoder(bytes.NewReader(bz))
	decoder.UseNumber()
	require.NoError(t, decoder.Decode(&info))
	info["activation_height"] = height
	for _, entry := range info["receipt_keys"].([]any) {
		entry.(map[string]any)["first_height"] = height
		entry.(map[string]any)["last_height"] = height
	}
	encoded, err := json.Marshal(info)
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "uncovered.json")
	require.NoError(t, os.WriteFile(path, encoded, 0o600))
	repair588Tool(t, false, "plan-info", "--check", path, "--plan-height", strconv.FormatInt(height, 10))
}
