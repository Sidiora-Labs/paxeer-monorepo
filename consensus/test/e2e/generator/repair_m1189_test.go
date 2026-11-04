package main

import (
	"context"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"testing"
	"time"

	e2e "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/test/e2e/pkg"
	"github.com/stretchr/testify/require"
)

func TestRepairM_1189_Cardinalities(t *testing.T) {
	assertUniformSetChoiceCardinalities(t)
}

func TestRepairM_1189_SeededChoices(t *testing.T) {
	probabilities := probSetChoice{"alpha": 0.1, "beta": 0.4, "delta": 0.7, "gamma": 1}
	weights := weightedChoice{"alpha": 1, "beta": 1, "delta": 1, "gamma": 1}
	for attempt := 0; attempt < 32; attempt++ {
		r := rand.New(rand.NewSource(randomSeed))
		replay := rand.New(rand.NewSource(randomSeed))
		for draw := 0; draw < 64; draw++ {
			expected := []string{}
			for _, item := range []string{"alpha", "beta", "delta", "gamma"} {
				if replay.Float64() <= probabilities[item] {
					expected = append(expected, item)
				}
			}
			require.Equal(t, expected, probabilities.Choose(r), "probability draw %d", draw)
		}

		r = rand.New(rand.NewSource(randomSeed))
		replay = rand.New(rand.NewSource(randomSeed))
		for draw := 0; draw < 64; draw++ {
			expected := []string{"alpha", "beta", "delta", "gamma"}[replay.Intn(4)]
			require.Equal(t, expected, weights.Choose(r), "equal-weight draw %d", draw)
		}
	}
}

func TestRepairM_1189_ReproducibleManifests(t *testing.T) {
	executable, err := os.Executable()
	require.NoError(t, err)
	var reference map[string][]byte
	for process := 0; process < 8; process++ {
		directory := t.TempDir()
		ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
		cmd := exec.CommandContext(ctx, executable,
			"-test.run=^TestRepairM_1189_ManifestProcess$", "--", "repair-m1189-manifests", directory)
		output, err := cmd.CombinedOutput()
		cancel()
		require.NoError(t, err, "process %d: %s", process, output)

		entries, err := os.ReadDir(directory)
		require.NoError(t, err)
		require.Len(t, entries, 48, "all testnet option combinations must be written")
		manifests := make(map[string][]byte, len(entries))
		for _, entry := range entries {
			require.False(t, entry.IsDir())
			data, err := os.ReadFile(filepath.Join(directory, entry.Name()))
			require.NoError(t, err)
			require.NotEmpty(t, data)
			manifests[entry.Name()] = data
			if process > 0 {
				require.Equal(t, reference[entry.Name()], data,
					"seed %d changed complete manifest %s in process %d", randomSeed, entry.Name(), process)
			}
		}
		if process == 0 {
			reference = manifests
		} else {
			require.Equal(t, reference, manifests)
		}
	}
}

func TestRepairM_1189_ManifestProcess(t *testing.T) {
	directory := t.TempDir()
	for i, arg := range os.Args {
		if arg == "repair-m1189-manifests" {
			require.Less(t, i+1, len(os.Args))
			directory = os.Args[i+1]
			break
		}
	}
	cli, err := NewCLI()
	require.NoError(t, err)
	cli.root.SetArgs([]string{"--dir", directory})
	require.NoError(t, cli.root.ExecuteContext(t.Context()))
	entries, err := os.ReadDir(directory)
	require.NoError(t, err)
	require.Len(t, entries, 48)
	lightClients := 0
	for _, entry := range entries {
		manifest, err := e2e.LoadManifest(filepath.Join(directory, entry.Name()))
		require.NoError(t, err)
		require.NotEmpty(t, manifest.Nodes)
		for _, node := range manifest.Nodes {
			for _, peer := range append(append([]string{}, node.Seeds...), node.PersistentPeers...) {
				require.Contains(t, manifest.Nodes, peer)
			}
			require.True(t, sort.StringsAreSorted(node.Perturb))
			if node.Mode == string(e2e.ModeLight) {
				lightClients++
				require.GreaterOrEqual(t, len(node.PersistentPeers), 2)
				require.True(t, sort.StringsAreSorted(node.PersistentPeers))
			}
		}
	}
	require.Positive(t, lightClients, "fixture seed must exercise light-provider ordering")
}
