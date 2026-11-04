package operations

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

const NativeOrdersAddress = "0x0000000000000000000000000000000000001022"

var addressPattern = regexp.MustCompile(`^0x[0-9a-fA-F]{40}$`)
var transactionPattern = regexp.MustCompile(`(?m)^Transaction hash: (0x[0-9a-fA-F]{64})\s*$`)

type ExchangeScenario struct {
	Stablecoin  string `json:"stablecoin"`
	Token       string `json:"token"`
	Amount      string `json:"amount"`
	TargetPrice string `json:"target_price"`
	ExpiresAt   string `json:"expires_at"`
}

type ExchangeOptions struct {
	Paxd, From, ChainID, RPC, Config string
	Scenario                         ExchangeScenario
}

type ExchangeSetup struct {
	Commands Commands
	Client   *http.Client
	Poll     time.Duration
}

func positiveInteger(value string) bool {
	n, ok := new(big.Int).SetString(value, 10)
	return ok && n.Sign() > 0 && n.BitLen() <= 256
}

func (s ExchangeSetup) committed(ctx context.Context, options ExchangeOptions, args []string) error {
	argv := []string{options.Paxd, "tx", "evm"}
	argv = append(argv, args...)
	argv = append(argv, "--from", options.From, "--chain-id", options.ChainID, "--evm-rpc", options.RPC, "--value", "0", "-y")
	raw, err := s.Commands.Run(ctx, Command{Args: argv})
	if err != nil {
		return err
	}
	match := transactionPattern.FindSubmatch(raw)
	if len(match) != 2 {
		return errors.New("CLI did not report a transaction hash")
	}
	// A successful submission is not a committed EVM execution.
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	client := s.Client
	if client == nil {
		client = &http.Client{Timeout: RequestTimeout}
	}
	poll := s.Poll
	if poll <= 0 {
		poll = 200 * time.Millisecond
	}
	for {
		body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "eth_getTransactionReceipt", "params": []string{string(match[1])}})
		requestCtx, cancelRequest := context.WithTimeout(ctx, RequestTimeout)
		req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, options.RPC, strings.NewReader(string(body)))
		if err != nil {
			cancelRequest()
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		response, err := client.Do(req)
		if err != nil {
			cancelRequest()
			return err
		}
		var envelope struct {
			Version string          `json:"jsonrpc"`
			ID      int             `json:"id"`
			Error   json.RawMessage `json:"error"`
			Result  *struct {
				Hash   string `json:"transactionHash"`
				Status string `json:"status"`
				Block  string `json:"blockNumber"`
			} `json:"result"`
		}
		err = json.NewDecoder(response.Body).Decode(&envelope)
		response.Body.Close()
		cancelRequest()
		if err != nil {
			return err
		}
		if response.StatusCode != http.StatusOK || envelope.Version != "2.0" || envelope.ID != 1 || (len(envelope.Error) > 0 && string(envelope.Error) != "null") {
			return errors.New("receipt RPC failed")
		}
		if envelope.Result != nil {
			if !strings.EqualFold(envelope.Result.Hash, string(match[1])) {
				return errors.New("receipt transaction hash mismatch")
			}
			height, ok := new(big.Int).SetString(strings.TrimPrefix(envelope.Result.Block, "0x"), 16)
			if envelope.Result.Status != "0x1" || !ok || height.Sign() <= 0 {
				return errors.New("exchange transaction did not commit successfully")
			}
			return nil
		}
		if err = Sleep(ctx, poll); err != nil {
			return err
		}
	}
}

func (s ExchangeSetup) Deploy(ctx context.Context, options ExchangeOptions) error {
	scenario := options.Scenario
	if !addressPattern.MatchString(scenario.Token) || !addressPattern.MatchString(scenario.Stablecoin) || strings.EqualFold(scenario.Token, scenario.Stablecoin) || strings.EqualFold(scenario.Token, "0x"+strings.Repeat("0", 40)) || strings.EqualFold(scenario.Stablecoin, "0x"+strings.Repeat("0", 40)) {
		return errors.New("two distinct nonzero token addresses are required")
	}
	for _, value := range []string{scenario.Amount, scenario.TargetPrice, scenario.ExpiresAt} {
		if !positiveInteger(value) {
			return errors.New("amount, target price and expiry must be positive uint256 values")
		}
	}
	expiry, _ := new(big.Int).SetString(scenario.ExpiresAt, 10)
	if !expiry.IsInt64() || expiry.Int64() <= time.Now().Unix() {
		return errors.New("exchange scenario expiry must be in the future")
	}
	if options.From == "" || options.ChainID == "" || options.Paxd == "" {
		return errors.New("paxd, sender and chain id are required")
	}
	amount, _ := new(big.Int).SetString(scenario.Amount, 10)
	amount.Mul(amount, big.NewInt(10))
	if amount.BitLen() > 256 {
		return errors.New("ten-order allowance exceeds uint256")
	}
	raw, err := os.ReadFile(options.Config)
	if err != nil {
		return err
	}
	var config map[string]any
	if err = decodeJSON(raw, &config); err != nil {
		return err
	}
	for _, asset := range []struct{ address, stable string }{{scenario.Stablecoin, "true"}, {scenario.Token, "false"}} {
		if err = s.committed(ctx, options, []string{"call-precompile", "spot_vault", "registerToken", asset.address, asset.stable}); err != nil {
			return err
		}
	}
	// placeLimitBuy pulls the stablecoin into the native order rail. Approve the
	// exact cover for all ten orders, rather than granting an unlimited allowance.
	payload := "095ea7b3" + strings.Repeat("0", 24) + strings.TrimPrefix(NativeOrdersAddress, "0x") + fmt.Sprintf("%064x", amount)
	if err = s.committed(ctx, options, []string{"call-contract", scenario.Stablecoin, payload}); err != nil {
		return err
	}
	distribution := make([]map[string]string, 0, 10)
	for index := 0; index < 10; index++ {
		if err = s.committed(ctx, options, []string{"call-precompile", "spot_orders", "placeLimitBuy", scenario.Stablecoin, scenario.Token, scenario.Amount, scenario.TargetPrice, scenario.ExpiresAt}); err != nil {
			return err
		}
		distribution = append(distribution, map[string]string{"contract_address": NativeOrdersAddress, "percentage": "0.1"})
	}
	current, err := os.ReadFile(options.Config)
	if err != nil {
		return err
	}
	if string(current) != string(raw) {
		return errors.New("loadtest configuration changed during exchange setup")
	}
	config["contract_distribution"] = distribution
	config["native_exchange_scenarios"] = []ExchangeScenario{scenario}
	info, err := os.Stat(options.Config)
	if err != nil {
		return err
	}
	return writeJSONAtomic(options.Config, config, info.Mode().Perm())
}
