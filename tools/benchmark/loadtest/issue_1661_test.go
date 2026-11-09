package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	rpchttp "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client/http"
	paxcmd "github.com/Sidiora-Labs/Paxeer-X-Network/daemon/paxd/cmd"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/network"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tools/benchmark/loadtest/operations"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

// Execute the production command tree with the disposable validator's client
// context. Every transaction still signs, broadcasts over RPC and commits.
type issue1661Commands struct {
	client           client.Context
	missing          string
	rejectLast       bool
	txs              int
	responses        [][]byte
	cancelAfterThird context.CancelFunc
	sequence         []string
}

func (r *issue1661Commands) Run(ctx context.Context, command operations.Command) ([]byte, error) {
	root, _ := paxcmd.NewRootCmd()
	if r.missing != "" {
		leaf, _, err := root.Find([]string{"tx", "wasm", r.missing})
		if err != nil {
			return nil, err
		}
		leaf.Parent().RemoveCommand(leaf)
	}
	args := append([]string(nil), command.Args[1:]...)
	isTx := len(args) > 3 && args[0] == "tx" && args[len(args)-1] != "--help"
	if isTx {
		r.txs++
		r.sequence = append(r.sequence, strings.Join(args[:3], " "))
		if r.rejectLast && r.txs == 4 {
			for i, arg := range args {
				if strings.HasPrefix(arg, "--fees=") {
					args[i] = "--fees=999999999999999999999999999999uhpx"
				}
			}
		}
	}
	var output bytes.Buffer
	cli := r.client.WithOutput(&output)
	root.PersistentPreRunE = func(cmd *cobra.Command, _ []string) error { return client.SetCmdClientContext(cmd, cli) }
	root.SetOut(&output)
	root.SetErr(io.Discard)
	root.SetArgs(args)
	ctx = context.WithValue(ctx, client.ClientContextKey, &cli)
	err := root.ExecuteContext(ctx)
	raw := append([]byte(nil), output.Bytes()...)
	if isTx {
		r.responses = append(r.responses, raw)
		if r.txs == 3 && r.cancelAfterThird != nil {
			r.cancelAfterThird()
		}
	}
	return raw, err
}

func TestIssue1661_CW20Deployment(t *testing.T) {
	cfg := network.DefaultConfig(t)
	cfg.TimeoutCommit = 100 * time.Millisecond
	cfg.BondDenom = "uhpx"
	n := network.New(t, cfg)
	t.Cleanup(n.Cleanup)
	_, err := n.WaitForHeightWithTimeout(2, time.Minute)
	require.NoError(t, err)
	val := n.Validators[0]
	cli := val.ClientCtx.WithClient(val.RPCClient).WithNodeURI(val.RPCAddress)
	options := operations.WasmDeployment{Root: "../../..", Paxd: "paxd", Key: val.Moniker, Chain: cfg.ChainID}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	query := func(args ...string) []byte {
		t.Helper()
		runner := &issue1661Commands{client: cli}
		raw, err := runner.Run(ctx, operations.Command{Args: append([]string{"paxd", "query"}, append(args, "--output=json")...)})
		require.NoError(t, err)
		return raw
	}
	codes := func() int {
		t.Helper()
		var response struct {
			Codes []json.RawMessage `json:"code_infos"`
		}
		require.NoError(t, json.Unmarshal(query("wasm", "list-code"), &response))
		return len(response.Codes)
	}
	staking := func() []byte {
		t.Helper()
		raw := query("staking", "delegations", val.Address.String())
		var response struct {
			Delegations []json.RawMessage `json:"delegation_responses"`
		}
		require.NoError(t, json.Unmarshal(raw, &response))
		result, err := json.Marshal(response.Delegations)
		require.NoError(t, err)
		return result
	}
	contract := func(address string) {
		t.Helper()
		_, err := sdk.AccAddressFromBech32(address)
		require.NoError(t, err)
		var response struct {
			Data struct {
				Name, Symbol string
				TotalSupply  string `json:"total_supply"`
				Decimals     int
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(query("wasm", "contract-state", "smart", address, `{"token_info":{}}`), &response))
		require.Equal(t, "TestToken", response.Data.Name)
		require.Equal(t, "TTK", response.Data.Symbol)
		require.Equal(t, 6, response.Data.Decimals)
		require.Equal(t, "0", response.Data.TotalSupply)
	}
	beforeStaking := staking()
	beforeUnbonding := query("staking", "unbonding-delegations", val.Address.String())
	initialCodes := codes()
	t.Run("production command registration", func(t *testing.T) {
		root, _ := paxcmd.NewRootCmd()
		for _, name := range []string{"store", "instantiate"} {
			cmd, remaining, err := root.Find([]string{"tx", "wasm", name})
			require.NoError(t, err)
			require.Empty(t, remaining)
			require.Equal(t, name, cmd.Name())
		}
		cmd, _, err := root.Find([]string{"tx", "dex"})
		require.True(t, err != nil || cmd.Name() != "dex", "retired DEX command registered")
	})
	t.Run("preflight commits nothing", func(t *testing.T) {
		for _, missing := range []string{"store", "instantiate"} {
			runner := &issue1661Commands{client: cli, missing: missing}
			var output bytes.Buffer
			require.Error(t, operations.DeployWasm(ctx, runner, options, &output))
			require.Zero(t, runner.txs)
			require.Empty(t, output.String())
			require.Equal(t, initialCodes, codes())
		}
		for _, contents := range [][]byte{nil, []byte("invalid wasm")} {
			root := t.TempDir()
			if contents != nil {
				path := filepath.Join(root, "contracts", "wasm", "cw20_base.wasm")
				require.NoError(t, os.MkdirAll(filepath.Dir(path), 0700))
				require.NoError(t, os.WriteFile(path, contents, 0600))
			}
			bad := options
			bad.Root = root
			runner := &issue1661Commands{client: cli}
			var output bytes.Buffer
			require.Error(t, operations.DeployWasm(ctx, runner, bad, &output))
			require.Zero(t, runner.txs)
			require.Empty(t, output.String())
			require.Equal(t, initialCodes, codes())
		}
	})
	t.Run("two queryable CW20 contracts", func(t *testing.T) {
		runner := &issue1661Commands{client: cli}
		var output bytes.Buffer
		require.NoError(t, operations.DeployWasm(ctx, runner, options, &output))
		addresses := strings.Split(strings.TrimSuffix(output.String(), "\n"), "\n")
		require.Len(t, addresses, 2)
		require.NotEqual(t, addresses[0], addresses[1])
		require.Equal(t, 4, runner.txs)
		require.Equal(t, []string{"tx wasm store", "tx wasm store", "tx wasm instantiate", "tx wasm instantiate"}, runner.sequence)
		require.Equal(t, initialCodes+2, codes())
		for _, address := range addresses {
			contract(address)
		}
		require.Equal(t, beforeStaking, staking())
		require.JSONEq(t, string(beforeUnbonding), string(query("staking", "unbonding-delegations", val.Address.String())))
	})
	t.Run("actual transaction rejection retains committed state", func(t *testing.T) {
		runner := &issue1661Commands{client: cli, rejectLast: true}
		var output bytes.Buffer
		err := operations.DeployWasm(ctx, runner, options, &output)
		var partial *operations.WasmDeploymentError
		require.ErrorAs(t, err, &partial)
		require.Empty(t, output.String())
		require.Len(t, partial.CodeIDs, 2)
		require.Len(t, partial.Addresses, 1)
		require.False(t, partial.Uncertain)
		require.Contains(t, partial.Recovery, "list-contract-by-code")
		require.Contains(t, err.Error(), partial.Addresses[0])
		require.NotEmpty(t, partial.TxHash)
		query("wasm", "code-info", partial.CodeIDs[0])
		query("wasm", "code-info", partial.CodeIDs[1])
		contract(partial.Addresses[0])
		var rejected struct {
			Code uint32 `json:"code"`
		}
		require.NoError(t, json.Unmarshal(runner.responses[3], &rejected))
		require.NotZero(t, rejected.Code)
		require.Equal(t, beforeStaking, staking())
		require.JSONEq(t, string(beforeUnbonding), string(query("staking", "unbonding-delegations", val.Address.String())))
	})
	t.Run("cancellation after committed contract", func(t *testing.T) {
		cancelled, cancel := context.WithCancel(ctx)
		defer cancel()
		runner := &issue1661Commands{client: cli, cancelAfterThird: cancel}
		var output bytes.Buffer
		err := operations.DeployWasm(cancelled, runner, options, &output)
		require.True(t, errors.Is(err, context.Canceled))
		var partial *operations.WasmDeploymentError
		require.ErrorAs(t, err, &partial)
		require.Len(t, partial.CodeIDs, 2)
		require.Len(t, partial.Addresses, 1)
		require.False(t, partial.Uncertain, "no new transaction attempted after cancellation")
		require.Equal(t, 3, runner.txs)
		require.Empty(t, output.String())
		contract(partial.Addresses[0])
		require.Equal(t, beforeStaking, staking())
	})
	t.Run("lost committed response retains recovery", func(t *testing.T) {
		var broadcasts atomic.Int32
		committed := make(chan []byte, 1)
		upstream := strings.Replace(val.RPCAddress, "tcp://", "http://", 1)
		transport := &http.Client{Timeout: 20 * time.Second}
		proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			body, err := io.ReadAll(req.Body)
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			forward, err := http.NewRequestWithContext(req.Context(), req.Method, upstream+req.URL.RequestURI(), bytes.NewReader(body))
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			forward.Header = req.Header.Clone()
			response, err := transport.Do(forward)
			if err != nil {
				http.Error(w, err.Error(), 502)
				return
			}
			defer response.Body.Close()
			raw, err := io.ReadAll(response.Body)
			if err != nil {
				http.Error(w, err.Error(), 502)
				return
			}
			var rpcRequest struct {
				Method string `json:"method"`
			}
			_ = json.Unmarshal(body, &rpcRequest)
			if rpcRequest.Method == "broadcast_tx_commit" && broadcasts.Add(1) == 4 {
				committed <- raw
				conn, _, err := w.(http.Hijacker).Hijack()
				if err == nil {
					_ = conn.Close()
				}
				return
			}
			for name, values := range response.Header {
				for _, value := range values {
					w.Header().Add(name, value)
				}
			}
			w.WriteHeader(response.StatusCode)
			_, _ = w.Write(raw)
		}))
		defer proxy.Close()
		remote, err := rpchttp.New(proxy.URL)
		require.NoError(t, err)
		runner := &issue1661Commands{client: cli.WithClient(remote).WithNodeURI(proxy.URL)}
		var output bytes.Buffer
		err = operations.DeployWasm(ctx, runner, options, &output)
		var partial *operations.WasmDeploymentError
		require.ErrorAs(t, err, &partial)
		require.Empty(t, output.String())
		require.Len(t, partial.CodeIDs, 2)
		require.Len(t, partial.Addresses, 1)
		require.True(t, partial.Uncertain)
		require.Contains(t, partial.Stage, "instantiate 2")
		require.Contains(t, partial.Recovery, "before retrying")
		select {
		case raw := <-committed:
			var envelope struct {
				Error  json.RawMessage `json:"error"`
				Result struct {
					Hash   string `json:"hash"`
					Height string `json:"height"`
				} `json:"result"`
			}
			require.NoError(t, json.Unmarshal(raw, &envelope))
			require.Empty(t, envelope.Error)
			require.NotEmpty(t, envelope.Result.Hash)
			require.NotEmpty(t, envelope.Result.Height)
			receipt := query("tx", envelope.Result.Hash)
			address, err := operations.TransactionIdentifier(receipt, "contract_address")
			require.NoError(t, err, "the interrupted response must belong to a committed CW20 instance")
			contract(address)
		default:
			t.Fatal("transport did not interrupt a real committed transaction")
		}
		// Recovery opens a fresh command tree and queries the retained code IDs.
		for _, id := range partial.CodeIDs {
			var response struct {
				Contracts []string `json:"contracts"`
			}
			require.NoError(t, json.Unmarshal(query("wasm", "list-contract-by-code", id), &response))
			require.Len(t, response.Contracts, 1)
			contract(response.Contracts[0])
		}
		require.Equal(t, beforeStaking, staking())
		require.JSONEq(t, string(beforeUnbonding), string(query("staking", "unbonding-delegations", val.Address.String())))
	})
}
