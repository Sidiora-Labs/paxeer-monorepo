package app_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	e2e "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/test/e2e/pkg"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	"github.com/stretchr/testify/require"
)

func TestRepair_122_ValidatorPowerWidth(t *testing.T) {
	root, err := filepath.Abs("../../../..")
	require.NoError(t, err)
	dir := t.TempDir()

	// Both endpoints are commands, so compile their actual source files together
	// in a temporary driver instead of duplicating their serialization logic.
	args := []string{"run"}
	for _, source := range []string{"runner/setup.go", "node/config.go"} {
		data, err := os.ReadFile(filepath.Join(root, "consensus/test/e2e", source))
		require.NoError(t, err)
		target := filepath.Join(dir, filepath.Base(source))
		require.NoError(t, os.WriteFile(target, data, 0600))
		args = append(args, target)
	}
	driver := filepath.Join(dir, "main.go")
	require.NoError(t, os.WriteFile(driver, []byte(repair122Driver), 0600))
	args = append(args, driver)

	powers := []int64{0, 255, 256, types.MaxTotalVotingPower, math.MaxInt64}
	for _, power := range powers {
		manifest := e2e.Manifest{
			KeyType: "ed25519",
			Nodes:   map[string]*e2e.ManifestNode{"validator01": {}},
			ValidatorUpdates: map[string]map[string]int64{
				"0": {"validator01": power},
				"1": {"validator01": power},
			},
		}
		file := filepath.Join(dir, fmt.Sprintf("power_%d.toml", power))
		require.NoError(t, manifest.Save(file))
		args = append(args, file)
	}

	cmd := exec.CommandContext(t.Context(), "go", args...)
	cmd.Dir = root
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	require.NoError(t, cmd.Run(), "real producer/consumer driver: %s", stderr.String())

	type update struct {
		Key   string
		Power int64
	}
	var results []struct {
		Key           string
		NodeUpdates   map[string]map[string]int64
		AppUpdates    map[string]map[string]int64
		DirectUpdates map[string]map[string]int64
		InitUpdates   []update
		BlockUpdates  []update
	}
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &results))
	require.Len(t, results, len(powers))
	for i, power := range powers {
		t.Run(fmt.Sprintf("power_%d", power), func(t *testing.T) {
			result := results[i]
			want := map[string]map[string]int64{
				"0": {result.Key: power},
				"1": {result.Key: power},
			}
			require.Equal(t, want, result.NodeUpdates)
			require.Equal(t, want, result.AppUpdates)
			require.Equal(t, want, result.DirectUpdates)
			for _, updates := range [][]update{result.InitUpdates, result.BlockUpdates} {
				require.Len(t, updates, 1)
				require.Equal(t, power, updates[0].Power)
				require.Equal(t, result.Key, updates[0].Key)
			}
		})
	}
}

const repair122Driver = `package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/test/e2e/app"
	e2e "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/test/e2e/pkg"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/paxeer-network/paxlog"
)

var logger = paxlog.NewLogger("repair122")
const builtinProtocol = "builtin"

type result struct {
	Key string
	NodeUpdates map[string]map[string]int64
	AppUpdates map[string]map[string]int64
	DirectUpdates map[string]map[string]int64
	InitUpdates []update
	BlockUpdates []update
}

type update struct {
	Key string
	Power int64
}

func summarize(updates []abci.ValidatorUpdate) []update {
	result := make([]update, len(updates))
	for i, value := range updates {
		key, err := crypto.PubKeyFromProto(value.PubKey)
		if err != nil { panic(err) }
		result[i] = update{Key: base64.StdEncoding.EncodeToString(key.Bytes()), Power: value.Power}
	}
	return result
}

func main() {
	results := []result{}
	for _, file := range os.Args[1:] {
		network, err := e2e.LoadTestnet(file)
		if err != nil { panic(err) }
		node := network.LookupNode("validator01")
		data, err := MakeAppConfig(node)
		if err != nil { panic(err) }
		configFile := file + ".app.toml"
		if err := os.WriteFile(configFile, data, 0600); err != nil { panic(err) }
		cfg, err := LoadConfig(configFile)
		if err != nil { panic(err) }
		appCfg := cfg.App()
		var direct app.Config
		if _, err := toml.Decode(string(data), &direct); err != nil { panic(err) }
		appCfg.Dir = filepath.Join(network.Dir, "state")
		application, err := app.NewApplication(appCfg)
		if err != nil { panic(err) }
		init, err := application.InitChain(context.Background(), &abci.RequestInitChain{InitialHeight: 1})
		if err != nil { panic(err) }
		block, err := application.FinalizeBlock(context.Background(), &abci.RequestFinalizeBlock{
			Header: &tmproto.Header{Height: 1},
		})
		if err != nil { panic(err) }
		results = append(results, result{
			Key: base64.StdEncoding.EncodeToString(node.PrivvalKey.Public().Bytes()),
			NodeUpdates: cfg.ValidatorUpdates,
			AppUpdates: appCfg.ValidatorUpdates,
			DirectUpdates: direct.ValidatorUpdates,
			InitUpdates: summarize(init.Validators),
			BlockUpdates: summarize(block.ValidatorUpdates),
		})
	}
	if err := json.NewEncoder(os.Stdout).Encode(results); err != nil { panic(err) }
}
`
