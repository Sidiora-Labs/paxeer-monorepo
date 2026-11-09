package operations

import (
	"bytes"
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

type WasmDeploymentError struct {
	Cause                   error
	CodeIDs                 []string
	Addresses               []string
	Stage, TxHash, Recovery string
	Uncertain               bool
}

func (e *WasmDeploymentError) Error() string {
	return fmt.Sprintf("CW20 deployment stopped at %s: %v; committed code IDs=%v; committed contract addresses=%v; uncertain=%t; txhash=%q; %s", e.Stage, e.Cause, e.CodeIDs, e.Addresses, e.Uncertain, e.TxHash, e.Recovery)
}

func (e *WasmDeploymentError) Unwrap() error { return e.Cause }

func DeployWasm(ctx context.Context, commands Commands, options WasmDeployment, output io.Writer) error {
	if options.Root == "" || options.Paxd == "" || options.Key == "" || options.Chain == "" {
		return errors.New("home, binary, key and chain are required")
	}
	root, err := filepath.Abs(options.Root)
	if err != nil {
		return err
	}
	artifact := filepath.Join(root, "contracts", "wasm", "cw20_base.wasm")
	file, err := os.Open(artifact)
	if err != nil {
		return fmt.Errorf("CW20 preflight artifact %q: %w", artifact, err)
	}
	info, statErr := file.Stat()
	var header [8]byte
	_, readErr := io.ReadFull(file, header[:])
	closeErr := file.Close()
	if statErr != nil || readErr != nil || closeErr != nil || !info.Mode().IsRegular() || info.Size() <= 8 || !bytes.Equal(header[:], []byte{0, 97, 115, 109, 1, 0, 0, 0}) {
		return fmt.Errorf("CW20 preflight: %q must be a readable WebAssembly v1 artifact", artifact)
	}
	for _, command := range []struct{ name, usage string }{
		{"store", "store [wasm file]"}, {"instantiate", "instantiate [code_id_int64]"},
	} {
		raw, err := commands.Run(ctx, Command{Args: []string{options.Paxd, "tx", "wasm", command.name, "--help"}})
		if err != nil {
			return fmt.Errorf("CW20 preflight %s: %w", command.name, err)
		}
		if !strings.Contains(string(raw), command.usage) {
			return fmt.Errorf("CW20 preflight: %q does not advertise supported wasm %s", options.Paxd, command.name)
		}
	}
	state := &WasmDeploymentError{Recovery: fmt.Sprintf("Use %q on chain %q to query wasm code-info CODE_ID and list-contract-by-code CODE_ID for the committed IDs; query tx TXHASH when known, otherwise resolve the sender with keys show %q -a and query txs by message.sender=ADDRESS before retrying the uncertain step. Committed chain state is retained; do not blindly redeploy.", options.Paxd, options.Chain, options.Key)}
	fail := func(err error) error { state.Cause = err; return state }
	tx := func(stage, kind string, args ...string) (string, error) {
		state.Stage, state.TxHash, state.Uncertain = stage, "", false
		if err := ctx.Err(); err != nil {
			return "", err
		}
		argv := append([]string{options.Paxd, "tx", "wasm"}, args...)
		argv = append(argv, "-y", "--from="+options.Key, "--chain-id="+options.Chain,
			"--gas=5000000", "--fees=1000000uhpx", "--broadcast-mode=block", "--output=json")
		state.Uncertain = true
		raw, runErr := commands.Run(ctx, Command{Args: argv})
		var receipt struct {
			TxHash string          `json:"txhash"`
			Code   json.RawMessage `json:"code"`
			Height json.RawMessage `json:"height"`
		}
		if decodeJSON(raw, &receipt) == nil {
			state.TxHash = receipt.TxHash
			if code := strings.Trim(string(receipt.Code), `"`); code != "" && code != "0" {
				state.Uncertain = false
			}
		}
		if runErr != nil {
			return "", runErr
		}
		value, err := TransactionIdentifier(raw, kind)
		if err != nil {
			return "", err
		}
		height, ok := new(big.Int).SetString(strings.Trim(string(receipt.Height), `"`), 10)
		if !ok || height.Sign() <= 0 {
			return "", errors.New("transaction identifier has no committed height")
		}
		state.Uncertain = false
		return value, nil
	}
	for i := 0; i < 2; i++ {
		id, err := tx(fmt.Sprintf("store %d", i+1), "code_id", "store", artifact)
		if err != nil {
			return fail(err)
		}
		state.CodeIDs = append(state.CodeIDs, id)
	}
	for i, id := range state.CodeIDs {
		msg := `{"name":"TestToken","symbol":"TTK","decimals":6,"initial_balances":[]}`
		address, err := tx(fmt.Sprintf("instantiate %d (code ID %s)", i+1, id), "contract_address", "instantiate", id, msg, "--no-admin", "--label=cw20-loadtest")
		if err != nil {
			return fail(err)
		}
		state.Addresses = append(state.Addresses, address)
	}
	state.Stage = "publish addresses"
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	if _, err := fmt.Fprint(output, strings.Join(state.Addresses, "\n")+"\n"); err != nil {
		return fail(err)
	}
	return nil
}
