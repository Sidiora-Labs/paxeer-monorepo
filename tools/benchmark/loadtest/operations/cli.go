package operations

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// RunCLI is shared by the installed helper and the script entry points.
func RunCLI(ctx context.Context, args []string, input io.Reader, output io.Writer, commands Commands) error {
	if len(args) == 0 {
		return errors.New("expected generate-bindings, populate-genesis, validator-faults, deploy-ten or metric")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	flags := flag.NewFlagSet(args[0], flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	switch args[0] {
	case "generate-bindings":
		root := flags.String("bindings", "", "bindings directory")
		abigen := flags.String("abigen", "abigen", "generator executable")
		if err = flags.Parse(args[1:]); err != nil {
			return err
		}
		if *root == "" || flags.NArg() != 0 {
			return errors.New("--bindings is required")
		}
		return GenerateBindings(ctx, commands, *root, *abigen)
	case "populate-genesis":
		count := flags.Int("count", -1, "exact account count")
		paxHome := flags.String("home", filepath.Join(home, ".pax"), "paxd home")
		accounts := flags.String("accounts", filepath.Join(home, "test_accounts"), "private account directory")
		paxd := flags.String("paxd", filepath.Join(home, "go", "bin", "paxd"), "paxd executable")
		local := flags.Bool("local", false, "use test keyring")
		if err = flags.Parse(args[1:]); err != nil {
			return err
		}
		if flags.NArg() != 0 {
			return errors.New("unexpected genesis arguments")
		}
		return GenerateGenesis(ctx, commands, GenesisOptions{Count: *count, Home: *paxHome, Accounts: *accounts, Paxd: *paxd, Local: *local})
	case "validator-faults":
		kind := flags.String("failure", "", "restart, delegation-change or slow-network")
		validators := flags.String("validators", "", "comma separated hosts")
		concurrent := flags.Int("concurrent", 2, "batch size")
		key := flags.String("ssh-key", "", "SSH identity path")
		chain := flags.String("chain-id", "", "chain id")
		paxd := flags.String("paxd", "paxd", "paxd executable")
		recovery := flags.String("recovery", filepath.Join(home, ".pax", "benchmark_faults", "recovery.json"), "private recovery record")
		recoverPath := flags.String("recover", "", "recover a stopped controller")
		hold := flags.Duration("hold", 10*time.Minute, "fault hold duration")
		if err = flags.Parse(args[1:]); err != nil {
			return err
		}
		if flags.NArg() != 0 || *hold < 0 {
			return errors.New("invalid fault arguments")
		}
		if *recoverPath != "" {
			controller, e := OpenRecovery(commands, *recoverPath)
			if e != nil {
				return e
			}
			return controller.Cleanup()
		}
		hosts := []string{}
		if *validators != "" {
			hosts = strings.Split(*validators, ",")
		}
		controller := NewFaultController(commands, FaultOptions{Paxd: *paxd, SSHKey: *key, ChainID: *chain, Validators: hosts, Concurrent: *concurrent, Recovery: *recovery, Hold: *hold})
		switch *kind {
		case "restart":
			if len(hosts) == 0 {
				return errors.New("validators are required")
			}
			return controller.Restarts(ctx)
		case "slow-network":
			if len(hosts) == 0 {
				return errors.New("validators are required")
			}
			return controller.SlowNetwork(ctx)
		case "delegation-change":
			if *chain == "" {
				return errors.New("chain-id is required")
			}
			return controller.DelegationChange(ctx)
		default:
			return errors.New("a supported --failure is required")
		}
	case "deploy-ten":
		scenarioPath := flags.String("scenario", "", "native asset/order scenario JSON")
		config := flags.String("config", "", "loadtest configuration to publish")
		paxd := flags.String("paxd", "paxd", "paxd executable")
		from := flags.String("from", "", "funded vault owner key")
		chain := flags.String("chain-id", "", "chain id")
		rpc := flags.String("evm-rpc", "http://localhost:8545", "EVM RPC endpoint")
		if err = flags.Parse(args[1:]); err != nil {
			return err
		}
		if *scenarioPath == "" || *config == "" || flags.NArg() != 0 {
			return errors.New("--scenario and --config are required")
		}
		raw, e := os.ReadFile(*scenarioPath)
		if e != nil {
			return e
		}
		var scenario ExchangeScenario
		if err = decodeJSON(raw, &scenario); err != nil {
			return err
		}
		return (ExchangeSetup{Commands: commands}).Deploy(ctx, ExchangeOptions{Paxd: *paxd, From: *from, ChainID: *chain, RPC: *rpc, Config: *config, Scenario: scenario})
	case "metric":
		if len(args) != 1 {
			return errors.New("metric expects its request on stdin")
		}
		var request struct {
			Operation string      `json:"operation"`
			RPC       string      `json:"rpc"`
			REST      string      `json:"rest"`
			Height    int         `json:"height"`
			Hash      string      `json:"hash"`
			Path      string      `json:"path"`
			Blocks    []BlockInfo `json:"blocks"`
		}
		if err = json.NewDecoder(io.LimitReader(input, 16<<20)).Decode(&request); err != nil {
			return err
		}
		client := MetricsClient{RPC: request.RPC, REST: request.REST}
		var result any
		switch request.Operation {
		case "block":
			result, err = client.Block(ctx, request.Height)
		case "info":
			result, err = client.BlockInfo(ctx, request.Height)
		case "breakdown":
			result, err = client.Breakdown(ctx, request.Height)
		case "height":
			result, err = client.Height(ctx, request.Hash)
		case "best":
			result, err = client.BestStats(ctx, request.Blocks)
		case "metrics":
			result, err = client.Metrics(ctx, request.Path)
		default:
			err = errors.New("unsupported metric operation")
		}
		envelope := map[string]any{"result": result}
		if err != nil {
			failure := map[string]any{"message": err.Error(), "kind": "value"}
			var httpError *HTTPError
			if errors.As(err, &httpError) {
				failure["kind"] = "http"
				failure["status"] = httpError.Status
				failure["url"] = httpError.URL
			}
			if strings.HasPrefix(err.Error(), "Block JSON-RPC error") {
				failure["kind"] = "rpc"
			}
			envelope["error"] = failure
		}
		if encodeErr := json.NewEncoder(output).Encode(envelope); encodeErr != nil {
			return encodeErr
		}
		return err
	default:
		return fmt.Errorf("unknown operation %q", args[0])
	}
}
