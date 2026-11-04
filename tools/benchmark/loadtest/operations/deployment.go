package operations

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

type deploymentEvent struct {
	Type       string `json:"type"`
	Attributes []struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	} `json:"attributes"`
}
type deploymentLog struct {
	Events []deploymentEvent `json:"events"`
}

// TransactionIdentifier consumes the complete CLI response, including JSON inside raw_log.
func TransactionIdentifier(raw []byte, kind string) (string, error) {
	var response struct {
		Code   json.RawMessage `json:"code"`
		RawLog string          `json:"raw_log"`
		Logs   []deploymentLog `json:"logs"`
	}
	if err := decodeJSON(raw, &response); err != nil {
		return "", err
	}
	code := strings.Trim(string(response.Code), `"`)
	if code != "0" {
		return "", errors.New("transaction refused or missing transaction code")
	}
	if kind == "success" {
		return "", nil
	}
	eventType, key := "", ""
	switch kind {
	case "code_id":
		eventType, key = "store_code", "code_id"
	case "contract_address":
		eventType, key = "instantiate", "_contract_address"
	case "proposal_id":
		eventType, key = "submit_proposal", "proposal_id"
	default:
		return "", errors.New("unknown transaction identifier")
	}
	logs := response.Logs
	if response.RawLog != "" {
		if err := decodeJSON([]byte(response.RawLog), &logs); err != nil {
			return "", fmt.Errorf("raw_log: %w", err)
		}
	}
	value := ""
	for _, log := range logs {
		for _, event := range log.Events {
			if event.Type == eventType {
				for _, attr := range event.Attributes {
					if attr.Key == key {
						if value != "" && value != attr.Value {
							return "", errors.New("ambiguous transaction identifier")
						}
						value = attr.Value
					}
				}
			}
		}
	}
	if kind == "contract_address" {
		if _, err := sdk.AccAddressFromBech32(value); err != nil {
			return "", fmt.Errorf("invalid contract address: %w", err)
		}
	} else {
		if !regexp.MustCompile(`^[1-9][0-9]*$`).MatchString(value) {
			return "", errors.New("missing or malformed identifier")
		}
		n, ok := new(big.Int).SetString(value, 10)
		if !ok || !n.IsUint64() {
			return "", errors.New("identifier exceeds uint64")
		}
	}
	return value, nil
}

func validAddress(value string) bool {
	return addressPattern.MatchString(value) && !strings.EqualFold(value, "0x"+strings.Repeat("0", 40))
}

func DeploymentAddress(raw []byte) (string, error) {
	var response struct {
		DeployedTo string `json:"deployedTo"`
	}
	if err := decodeJSON(raw, &response); err != nil {
		return "", err
	}
	if !validAddress(response.DeployedTo) {
		return "", errors.New("deployment did not return a nonzero address")
	}
	return response.DeployedTo, nil
}

// CanonicalABI preserves every entry and parameter field, tuple component and overload.
func CanonicalABI(raw []byte) ([]string, error) {
	var entries []map[string]any
	if err := decodeJSON(raw, &entries); err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, errors.New("empty ABI")
	}
	result := make([]string, 0, len(entries))
	for _, entry := range entries {
		if _, ok := entry["type"].(string); !ok {
			return nil, errors.New("ABI entry has no type")
		}
		encoded, err := json.Marshal(entry)
		if err != nil {
			return nil, err
		}
		result = append(result, string(encoded))
	}
	sort.Strings(result)
	return result, nil
}

type EVMDeployment struct{ Root, Endpoint, Key, Contract, Paxd string }

func DeployEVM(ctx context.Context, commands Commands, options EVMDeployment) (string, error) {
	if options.Key == "" || options.Endpoint == "" {
		return "", errors.New("deployer key and endpoint are required")
	}
	source := ""
	switch options.Contract {
	case "erc20":
		source = "src/NoopToken.sol:NoopToken"
	case "erc721":
		source = "src/ERC721.sol:MyNFT"
	default:
		return "", errors.New("unsupported contract")
	}
	// Setup verifies the committed dependency trees. A failed setup must prevent forge.
	if _, err := commands.Run(ctx, Command{Args: []string{filepath.Join(options.Root, "setup.sh")}}); err != nil {
		return "", err
	}
	accountRaw, err := commands.Run(ctx, Command{Args: []string{"cast", "wallet", "address", "--private-key", options.Key}})
	if err != nil {
		return "", err
	}
	account := strings.TrimSpace(string(accountRaw))
	if !validAddress(account) {
		return "", errors.New("invalid deployer address")
	}
	balanceRaw, err := commands.Run(ctx, Command{Args: []string{"cast", "balance", account, "--rpc-url", options.Endpoint}})
	if err != nil {
		return "", err
	}
	balance, ok := new(big.Int).SetString(strings.TrimSpace(string(balanceRaw)), 10)
	if !ok || balance.Sign() < 0 {
		return "", errors.New("invalid deployer balance")
	}
	threshold, _ := new(big.Int).SetString("100000000000000000000", 10)
	if balance.Cmp(threshold) < 0 {
		raw, err := commands.Run(ctx, Command{Args: []string{options.Paxd, "tx", "evm", "send", account, threshold.String(), "--from", "admin", "--evm-rpc", options.Endpoint, "-y"}})
		if err != nil {
			return "", err
		}
		match := transactionPattern.FindSubmatch(raw)
		if len(match) != 2 {
			return "", errors.New("funding did not return a transaction hash")
		}
		if err = confirmFunding(ctx, options.Endpoint, common.HexToHash(string(match[1]))); err != nil {
			return "", err
		}

	}
	args := []string{"forge", "create", "--root", options.Root, "--broadcast", "-r", options.Endpoint, "--private-key", options.Key, source, "--json"}
	if options.Contract == "erc20" {
		args = append(args, "--constructor-args", "NoopToken", "NT")
	}
	raw, err := commands.Run(ctx, Command{Args: args})
	if err != nil {
		return "", err
	}
	return DeploymentAddress(raw)
}

func confirmFunding(ctx context.Context, endpoint string, hash common.Hash) error {
	ctx, cancel := context.WithTimeout(ctx, CommandTimeout)
	defer cancel()
	client, err := ethclient.DialContext(ctx, endpoint)
	if err != nil {
		return err
	}
	defer client.Close()
	for {
		callCtx, callCancel := context.WithTimeout(ctx, RequestTimeout)
		receipt, err := client.TransactionReceipt(callCtx, hash)
		callCancel()
		if err == nil {
			if receipt.TxHash != hash || receipt.BlockNumber == nil || receipt.BlockNumber.Sign() <= 0 || receipt.Status != 1 {
				return errors.New("funding transaction failed or is uncommitted")
			}
			return nil
		}
		if !errors.Is(err, ethereum.NotFound) {
			return err
		}
		if err = Sleep(ctx, 100*time.Millisecond); err != nil {
			return err
		}
	}
}

type WasmDeployment struct {
	Root, Paxd, Key, Chain string
	Settle                 time.Duration
}

func DeployWasm(ctx context.Context, commands Commands, options WasmDeployment, output io.Writer) error {
	if options.Root == "" || options.Paxd == "" || options.Key == "" || options.Chain == "" {
		return errors.New("home, binary, key and chain are required")
	}
	root, err := filepath.Abs(options.Root)
	if err != nil {
		return err
	}
	options.Root = root
	names := []string{"jupiter", "mars", "saturn", "venus"}
	for _, name := range names {
		root := filepath.Join(options.Root, name)
		if _, err := commands.Run(ctx, Command{Args: []string{"cargo", "build", "--manifest-path", filepath.Join(root, "Cargo.toml")}}); err != nil {
			return err
		}
		if _, err := commands.Run(ctx, Command{Args: []string{"docker", "run", "--rm", "-v", root + ":/code", "--mount", "type=volume,source=" + name + "_cache,target=/code/target", "--mount", "type=volume,source=registry_cache,target=/usr/local/cargo/registry", "cosmwasm/rust-optimizer:0.12.5"}}); err != nil {
			return err
		}
	}
	tx := func(kind string, args ...string) (string, error) {
		argv := append([]string{options.Paxd, "tx"}, args...)
		argv = append(argv, "-y", "--from="+options.Key, "--chain-id="+options.Chain)
		if args[0] == "wasm" {
			argv = append(argv, "--gas=5000000", "--fees=1000000uhpx")
		}
		if args[0] == "dex" {
			argv = append(argv, "--gas=500000", "--fees=10000000uhpx")
		}
		argv = append(argv, "--broadcast-mode=block", "--output=json")
		raw, err := commands.Run(ctx, Command{Args: argv})
		if err != nil {
			return "", err
		}
		return TransactionIdentifier(raw, kind)
	}
	ids, addresses := map[string]string{}, map[string]string{}
	for _, name := range names {
		id, err := tx("code_id", "wasm", "store", filepath.Join(options.Root, name, "artifacts", name+".wasm"))
		if err != nil {
			return err
		}
		ids[name] = id
	}
	raw, err := commands.Run(ctx, Command{Args: []string{options.Paxd, "keys", "show", "node_admin", "--bech=val", "--output=json"}})
	if err != nil {
		return err
	}
	var validator struct {
		Address string `json:"address"`
	}
	if err = decodeJSON(raw, &validator); err != nil {
		return err
	}
	if _, err = sdk.ValAddressFromBech32(validator.Address); err != nil {
		return err
	}
	if _, err = tx("success", "staking", "delegate", validator.Address, "1000000000uhpx"); err != nil {
		return err
	}
	for _, name := range []string{"mars", "saturn", "venus", "jupiter"} {
		msg := "{}"
		if name == "jupiter" {
			encoded, _ := json.Marshal(map[string]string{"mars_address": addresses["mars"]})
			msg = string(encoded)
		}
		address, err := tx("contract_address", "wasm", "instantiate", ids[name], msg, "--no-admin", "--label=dex")
		if err != nil {
			return err
		}
		addresses[name] = address
	}
	for _, name := range []string{"mars", "saturn", "venus", "jupiter"} {
		args := []string{"dex", "register-contract", addresses[name], ids[name], "false", "true"}
		if name == "saturn" || name == "jupiter" {
			args = append(args, addresses["mars"])
		}
		if _, err := tx("success", args...); err != nil {
			return err
		}
	}
	for _, name := range names {
		pair := map[string]any{"batch_contract_pair": []any{map[string]any{"contract_addr": addresses[name], "pairs": []any{map[string]string{"price_denom": "PAX", "asset_denom": "ATOM", "tick_size": "0.0000001"}}}}}
		file, err := os.CreateTemp(options.Root, ".pairs-*.json")
		if err != nil {
			return err
		}
		path := file.Name()
		err = json.NewEncoder(file).Encode(pair)
		closeErr := file.Close()
		if err == nil {
			err = closeErr
		}
		if err == nil {
			_, err = tx("success", "dex", "register-pairs", path)
		}
		os.Remove(path)
		if err != nil {
			return err
		}
	}
	if err := Sleep(ctx, options.Settle); err != nil {
		return err
	}
	if _, err := tx("success", "staking", "unbond", validator.Address, "1000000000uhpx"); err != nil {
		return err
	}
	for _, name := range names {
		if _, err := fmt.Fprintln(output, addresses[name]); err != nil {
			return err
		}
	}
	return nil
}
