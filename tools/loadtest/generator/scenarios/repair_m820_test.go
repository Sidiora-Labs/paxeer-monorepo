package scenarios_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/tracing"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/holiman/uint256"

	"github.com/paxeer-network/pax-load/config"
	"github.com/paxeer-network/pax-load/generator"
	"github.com/paxeer-network/pax-load/generator/bindings"
	"github.com/paxeer-network/pax-load/generator/scenarios"
	"github.com/paxeer-network/pax-load/stats"
	"github.com/paxeer-network/pax-load/types"
)

// Repair820EVM is a loopback JSON-RPC peer backed by geth's real state database,
// transaction admission, EVM and receipt producer. No receipts or storage writes
// are synthesized: even constructor calls execute the committed binding bytecode.
type Repair820EVM struct {
	mu           sync.Mutex
	state        *state.StateDB
	chain        *params.ChainConfig
	header       *ethtypes.Header
	receipts     map[common.Hash]*ethtypes.Receipt
	holdSend     <-chan struct{}
	holdReceipts bool
	entered      chan struct{}
}

func (p *Repair820EVM) evm(db *state.StateDB) *vm.EVM {
	return vm.NewEVM(vm.BlockContext{
		CanTransfer: core.CanTransfer, Transfer: core.Transfer,
		GetHash:     func(uint64) common.Hash { return common.Hash{} },
		BlockNumber: p.header.Number, Time: p.header.Time,
		Difficulty: new(big.Int), BaseFee: p.header.BaseFee,
		GasLimit: p.header.GasLimit, Random: new(common.Hash),
	}, db, p.chain, vm.Config{})
}

func (p *Repair820EVM) SendRawTransaction(ctx context.Context, raw hexutil.Bytes) (common.Hash, error) {
	select {
	case p.entered <- struct{}{}:
	default:
	}
	if p.holdSend != nil {
		select {
		case <-p.holdSend:
		case <-ctx.Done():
			return common.Hash{}, ctx.Err()
		}
	}
	var tx ethtypes.Transaction
	if err := tx.UnmarshalBinary(raw); err != nil {
		return common.Hash{}, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.state.SetTxContext(tx.Hash(), len(p.receipts))
	var usedGas uint64
	receipt, err := core.ApplyTransaction(p.evm(p.state), new(core.GasPool).AddGas(p.header.GasLimit), p.state, p.header, &tx, &usedGas)
	if err != nil {
		return common.Hash{}, err
	}
	// JSON-RPC encodes an empty log list as [], including constructor receipts.
	if receipt.Logs == nil {
		receipt.Logs = []*ethtypes.Log{}
	}
	p.receipts[tx.Hash()] = receipt
	return tx.Hash(), nil
}

func (p *Repair820EVM) GetTransactionReceipt(hash common.Hash) *ethtypes.Receipt {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.holdReceipts {
		return nil
	}
	return p.receipts[hash]
}

func (p *Repair820EVM) GetCode(address common.Address, block string) hexutil.Bytes {
	p.mu.Lock()
	defer p.mu.Unlock()
	return common.CopyBytes(p.state.GetCode(address))
}

type repair820CallArgs struct {
	From  common.Address
	To    common.Address
	Input hexutil.Bytes
	Data  hexutil.Bytes
}

func (p *Repair820EVM) Call(args repair820CallArgs, block string) (hexutil.Bytes, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	input := args.Input
	if input == nil {
		input = args.Data
	}
	db := p.state.Copy()
	evm := p.evm(db)
	evm.SetTxContext(vm.TxContext{Origin: args.From, GasPrice: new(uint256.Int)})
	ret, _, err := evm.StaticCall(args.From, args.To, input, 1000000)
	return ret, err
}

// Observe the client's socket EOF, including while an RPC handler is blocked.
// Geth waits for active handlers before closing its server-side socket.
type repair820Listener struct {
	net.Listener
	closed chan struct{}
}
type repair820Conn struct {
	net.Conn
	closed chan struct{}
	once   sync.Once
}

func (l repair820Listener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return &repair820Conn{Conn: conn, closed: l.closed}, nil
}
func (c *repair820Conn) Read(buffer []byte) (int, error) {
	n, err := c.Conn.Read(buffer)
	if errors.Is(err, io.EOF) {
		c.once.Do(func() {
			select {
			case c.closed <- struct{}{}:
			default:
			}
		})
	}
	return n, err
}

func repair820Peer(t *testing.T) (*Repair820EVM, string, <-chan struct{}) {
	t.Helper()
	db := state.NewDatabaseForTesting()
	t.Cleanup(func() {
		if err := db.TrieDB().Close(); err != nil {
			t.Error(err)
		}
	})
	statedb, err := state.New(ethtypes.EmptyRootHash, db)
	if err != nil {
		t.Fatal(err)
	}
	chain := *params.AllDevChainProtocolChanges
	chain.ChainID = big.NewInt(125)
	peer := &Repair820EVM{state: statedb, chain: &chain,
		header:   &ethtypes.Header{Number: big.NewInt(1), Time: 1, GasLimit: 30000000, BaseFee: new(big.Int), Difficulty: new(big.Int)},
		receipts: make(map[common.Hash]*ethtypes.Receipt), entered: make(chan struct{}, 10)}
	server := rpc.NewServer()
	if err := server.RegisterName("eth", peer); err != nil {
		t.Fatal(err)
	}
	ws := server.WebsocketHandler([]string{"*"})
	httpServer := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
			ws.ServeHTTP(w, r)
		} else {
			server.ServeHTTP(w, r)
		}
	}))
	closed := make(chan struct{}, 20)
	httpServer.Listener = repair820Listener{Listener: httpServer.Listener, closed: closed}
	httpServer.Start()
	t.Cleanup(func() { server.Stop(); httpServer.Close() })
	return peer, httpServer.URL, closed
}

func (p *Repair820EVM) fund(account *types.Account) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.state.SetBalance(account.Address, uint256.MustFromBig(new(big.Int).Lsh(big.NewInt(1), 240)), tracing.BalanceChangeUnspecified)
}

func repair820Account(t *testing.T) *types.Account {
	t.Helper()
	account, err := types.NewAccount()
	if err != nil {
		t.Fatal(err)
	}
	return account
}

func repair820Config(t *testing.T, name, selections string) *config.LoadConfig {
	t.Helper()
	var cfg config.LoadConfig
	profile := fmt.Sprintf(`{"chainId":125,"mockDeploy":true,"accounts":{"count":2},"scenarios":[{"name":%q,"weight":1%s}]}`, name, selections)
	if err := json.Unmarshal([]byte(profile), &cfg); err != nil {
		t.Fatal(err)
	}
	return &cfg
}

func repair820Deploy(t *testing.T, url, name, selections string, account *types.Account) (scenarios.ContextTxGenerator, *config.LoadConfig) {
	t.Helper()
	cfg := repair820Config(t, name, selections)
	cfg.MockDeploy, cfg.Endpoints = false, []string{url}
	scenario := scenarios.CreateScenario(cfg.Scenarios[0]).(scenarios.ContextTxGenerator)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if _, err := scenario.DeployContext(ctx, cfg, account); err != nil {
		t.Fatal(err)
	}
	return scenario, cfg
}

func repair820Generate(t *testing.T, scenario scenarios.ContextTxGenerator, sender *types.Account, receiver common.Address) *types.LoadTx {
	t.Helper()
	load, err := scenario.GenerateWithError(&types.TxScenario{Name: scenario.Name(), Sender: sender, Receiver: receiver})
	if err != nil {
		t.Fatal(err)
	}
	repair323Decode(t, load)
	return load
}

func repair820Submit(t *testing.T, url string, load *types.LoadTx) *ethtypes.Receipt {
	t.Helper()
	response, err := http.Post(url, "application/json", bytes.NewReader(load.JSONRPCPayload))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Result common.Hash
		Error  *json.RawMessage
	}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK || result.Error != nil || result.Result != load.EthTx.Hash() {
		t.Fatalf("send failed: %s", body)
	}
	client, err := ethclient.Dial(url)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	receipt, err := client.TransactionReceipt(ctx, result.Result)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.TxHash != load.EthTx.Hash() || receipt.GasUsed == 0 || receipt.GasUsed > load.EthTx.Gas() {
		t.Fatalf("invalid EVM receipt: %+v", receipt)
	}
	return receipt
}

func repair820Successful(t *testing.T, url string, load *types.LoadTx) *ethtypes.Receipt {
	t.Helper()
	receipt := repair820Submit(t, url, load)
	if receipt.Status != ethtypes.ReceiptStatusSuccessful {
		t.Fatalf("generated transaction reverted: gas %d/%d", receipt.GasUsed, load.EthTx.Gas())
	}
	return receipt
}

func TestRepairM_820_DisperseValue(t *testing.T) {
	for _, tc := range []struct {
		name, options string
		batch         int
	}{
		{"default", "", scenarios.DisperseBatchSize},
		{"bounded_override", `,"gasPicker":{"Name":"fixed","Gas":106000}`, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			peer, url, _ := repair820Peer(t)
			sender := repair820Account(t)
			peer.fund(sender)
			scenario, _ := repair820Deploy(t, url, scenarios.Disperse, tc.options, sender)
			load := repair820Generate(t, scenario, sender, common.Address{})
			abi, err := bindings.DisperseMetaData.GetAbi()
			if err != nil {
				t.Fatal(err)
			}
			args, err := abi.Methods["disperseEtherFixed"].Inputs.Unpack(load.EthTx.Data()[4:])
			if err != nil {
				t.Fatal(err)
			}
			recipients := args[0].([]common.Address)
			if len(recipients) != tc.batch || load.EthTx.Value().Cmp(big.NewInt(int64(tc.batch))) != 0 {
				t.Fatalf("batch/value = %d/%s", len(recipients), load.EthTx.Value())
			}
			for _, recipient := range recipients {
				if peer.state.GetBalance(recipient).Sign() != 0 {
					t.Fatal("recipient was not fresh")
				}
			}
			repair820Successful(t, url, load)
			for _, recipient := range recipients {
				if peer.state.GetBalance(recipient).CmpUint64(1) != 0 {
					t.Fatalf("recipient %s did not receive one wei", recipient)
				}
			}
			if peer.state.GetBalance(*load.EthTx.To()).Sign() != 0 {
				t.Fatal("batch left funds in the contract")
			}
		})
	}
	// Existing deployments can use a different fixed amount. Read the actual
	// contract setting while attaching rather than assuming the constructor default.
	t.Run("attached_amount", func(t *testing.T) {
		peer, url, _ := repair820Peer(t)
		sender := repair820Account(t)
		peer.fund(sender)
		client, err := ethclient.Dial(url)
		if err != nil {
			t.Fatal(err)
		}
		defer client.Close()
		auth, err := bind.NewKeyedTransactorWithChainID(sender.PrivKey, big.NewInt(125))
		if err != nil {
			t.Fatal(err)
		}
		auth.Nonce, auth.GasLimit = new(big.Int).SetUint64(sender.GetAndIncrementNonce()), 3000000
		auth.GasTipCap, auth.GasFeeCap = big.NewInt(2e9), big.NewInt(20e9)
		address, deployment, _, err := bindings.DeployDisperse(auth, client, big.NewInt(7), big.NewInt(1))
		if err != nil {
			t.Fatal(err)
		}
		if receipt, err := client.TransactionReceipt(context.Background(), deployment.Hash()); err != nil || receipt.Status != 1 {
			t.Fatalf("deployment: %v / %v", receipt, err)
		}
		cfg := repair820Config(t, scenarios.Disperse, "")
		cfg.MockDeploy, cfg.Endpoints = false, []string{url}
		scenario := scenarios.CreateScenario(cfg.Scenarios[0]).(scenarios.ContextTxGenerator)
		if err := scenario.AttachContext(context.Background(), cfg, address); err != nil {
			t.Fatal(err)
		}
		load := repair820Generate(t, scenario, sender, common.Address{})
		if load.EthTx.Value().Cmp(big.NewInt(7*scenarios.DisperseBatchSize)) != 0 {
			t.Fatal("attached contract's payable amount was ignored")
		}
		repair820Successful(t, url, load)
		abi, _ := bindings.DisperseMetaData.GetAbi()
		args, err := abi.Methods["disperseEtherFixed"].Inputs.Unpack(load.EthTx.Data()[4:])
		if err != nil {
			t.Fatal(err)
		}
		for _, recipient := range args[0].([]common.Address) {
			if peer.state.GetBalance(recipient).CmpUint64(7) != 0 {
				t.Fatal("attached recipient balance mismatch")
			}
		}
	})
	t.Run("insufficient_gas_refused", func(t *testing.T) {
		cfg := repair820Config(t, scenarios.Disperse, `,"gasPicker":{"Name":"fixed","Gas":21000}`)
		scenario := scenarios.CreateScenario(cfg.Scenarios[0]).(scenarios.ContextTxGenerator)
		if err := scenario.Attach(cfg, common.HexToAddress("0x820")); err != nil {
			t.Fatal(err)
		}
		load, err := scenario.GenerateWithError(&types.TxScenario{Sender: repair820Account(t)})
		if load != nil || err == nil || !strings.Contains(err.Error(), "cannot fund one recipient") {
			t.Fatalf("underfunded batch: %v / %v", load, err)
		}
	})
}

func TestRepairM_820_ConflictUnderGas(t *testing.T) {
	for _, tc := range []struct {
		name, options string
		gas           uint64
		success       bool
	}{
		{"default", "", 100000, true},
		{"override", `,"gasPicker":{"Name":"fixed","Gas":120000}`, 120000, true},
		{"small_override", `,"gasPicker":{"Name":"fixed","Gas":22460}`, 22460, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			peer, url, _ := repair820Peer(t)
			sender, receiver := repair820Account(t), repair820Account(t)
			peer.fund(sender)
			scenario, _ := repair820Deploy(t, url, scenarios.ERC20Conflict, tc.options, sender)
			client, err := ethclient.Dial(url)
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			load := repair820Generate(t, scenario, sender, receiver.Address)
			if load.EthTx.Gas() != tc.gas {
				t.Fatalf("gas = %d, want %d", load.EthTx.Gas(), tc.gas)
			}
			receipt := repair820Submit(t, url, load)
			contract, err := bindings.NewERC20Conflict(*load.EthTx.To(), client)
			if err != nil {
				t.Fatal(err)
			}
			counter, err := contract.Counter(nil)
			if err != nil {
				t.Fatal(err)
			}
			balance, err := contract.BalanceOf(nil, receiver.Address)
			if err != nil {
				t.Fatal(err)
			}
			if !tc.success {
				if receipt.Status != 0 || counter.Sign() != 0 {
					t.Fatal("small configured gas must revert without conflicting writes")
				}
				return
			}
			if receipt.Status != 1 || counter.Cmp(big.NewInt(1)) != 0 || balance.Cmp(big.NewInt(1)) != 0 {
				t.Fatalf("receipt/counter/balance = %d/%s/%s", receipt.Status, counter, balance)
			}
			load = repair820Generate(t, scenario, sender, receiver.Address)
			repair820Successful(t, url, load)
			counter, err = contract.Counter(nil)
			if err != nil {
				t.Fatal(err)
			}
			balance, err = contract.BalanceOf(nil, receiver.Address)
			if err != nil {
				t.Fatal(err)
			}
			senderBalance, err := contract.BalanceOf(nil, sender.Address)
			if err != nil {
				t.Fatal(err)
			}
			wantSender := new(big.Int).Sub(new(big.Int).Mul(big.NewInt(1000), new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)), big.NewInt(1))
			if counter.Cmp(big.NewInt(2)) != 0 || balance.Cmp(big.NewInt(2)) != 0 || senderBalance.Cmp(wantSender) != 0 {
				t.Fatalf("repeated writes: %s / %s / %s", counter, balance, senderBalance)
			}
		})
	}
}

func TestRepairM_820_NFTUnderGas(t *testing.T) {
	for _, tc := range []struct {
		name, options string
		gas           uint64
		success       bool
	}{
		{"default", "", 100000, true},
		{"override", `,"gasPicker":{"Name":"fixed","Gas":120000}`, 120000, true},
		{"small_override", `,"gasPicker":{"Name":"fixed","Gas":22460}`, 22460, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			peer, url, _ := repair820Peer(t)
			sender, receiver := repair820Account(t), repair820Account(t)
			peer.fund(sender)
			scenario, _ := repair820Deploy(t, url, scenarios.ERC721, tc.options, sender)
			client, err := ethclient.Dial(url)
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			load := repair820Generate(t, scenario, sender, receiver.Address)
			if load.EthTx.Gas() != tc.gas {
				t.Fatalf("gas = %d, want %d", load.EthTx.Gas(), tc.gas)
			}
			receipt := repair820Submit(t, url, load)
			contract, err := bindings.NewERC721(*load.EthTx.To(), client)
			if err != nil {
				t.Fatal(err)
			}
			balance, err := contract.BalanceOf(nil, receiver.Address)
			if err != nil {
				t.Fatal(err)
			}
			if !tc.success {
				if receipt.Status != 0 || balance.Sign() != 0 {
					t.Fatal("small configured gas must revert without minting")
				}
				return
			}
			owner, err := contract.OwnerOf(nil, big.NewInt(1))
			if err != nil {
				t.Fatal(err)
			}
			if receipt.Status != 1 || owner != receiver.Address || balance.Cmp(big.NewInt(1)) != 0 || len(receipt.Logs) != 1 {
				t.Fatalf("mint did not produce owner/balance/Transfer: %d/%s/%s", receipt.Status, owner, balance)
			}
			repair820Successful(t, url, repair820Generate(t, scenario, sender, receiver.Address))
			owner, err = contract.OwnerOf(nil, big.NewInt(2))
			if err != nil {
				t.Fatal(err)
			}
			balance, err = contract.BalanceOf(nil, receiver.Address)
			if err != nil {
				t.Fatal(err)
			}
			if owner != receiver.Address || balance.Cmp(big.NewInt(2)) != 0 {
				t.Fatal("successive generated token IDs did not mint")
			}
		})
	}
}

func repair820FundGenerator(t *testing.T, peer *Repair820EVM, gen generator.Generator) {
	t.Helper()
	for _, pool := range gen.GetAccountPools() {
		fixed, ok := pool.(generator.FixedAccountPool)
		if !ok {
			t.Fatal("test profile must use fixed accounts")
		}
		for _, account := range fixed.FixedAccounts() {
			peer.fund(account)
		}
	}
}

func TestRepairM_820_FeeCapSignConversion(t *testing.T) {
	for _, name := range []string{scenarios.EVMTransfer, scenarios.EVMTransferNoop} {
		t.Run(name, func(t *testing.T) {
			for _, tc := range []struct{ tip, fee uint64 }{{math.MaxInt64 + 1, math.MaxUint64}, {math.MaxUint64, math.MaxUint64}, {0, 0}} {
				peer, url, _ := repair820Peer(t)
				cfg := repair820Config(t, name, fmt.Sprintf(`,"gasTipCapPicker":{"Name":"fixed","Gas":%d},"gasFeeCapPicker":{"Name":"fixed","Gas":%d}`, tc.tip, tc.fee))
				gen, err := generator.NewConfigBasedGenerator(cfg)
				if err != nil {
					t.Fatal(err)
				}
				repair820FundGenerator(t, peer, gen)
				load, ok := gen.Generate()
				if !ok || load == nil {
					t.Fatal("generator emitted no signed transfer")
				}
				tx := repair323Decode(t, load)
				if tx.GasTipCap().Cmp(new(big.Int).SetUint64(tc.tip)) != 0 || tx.GasFeeCap().Cmp(new(big.Int).SetUint64(tc.fee)) != 0 {
					t.Fatal("unsigned fee conversion changed transaction caps")
				}
				repair820Successful(t, url, load)
			}
			for _, selection := range []string{
				`,"gasTipCapPicker":{"Name":"fixed","Gas":5},"gasFeeCapPicker":{"Name":"fixed","Gas":4}`,
				`,"gasTipCapPicker":{"Name":"fixed","Gas":18446744073709551615},"gasFeeCapPicker":{"Name":"fixed","Gas":9223372036854775808}`,
			} {
				gen, err := generator.NewConfigBasedGenerator(repair820Config(t, name, selection))
				if gen != nil || err == nil || !strings.Contains(err.Error(), "fee cap must be at least") {
					t.Fatalf("constructor lost fee admission error: %v / %v", gen, err)
				}
			}
			cfg := repair820Config(t, name, `,"gasTipCapPicker":{"Name":"fixed","Gas":2},"gasFeeCapPicker":{"Name":"fixed","Gas":3}`)
			scenario := scenarios.CreateScenario(cfg.Scenarios[0]).(scenarios.ContextTxGenerator)
			if err := scenario.Attach(cfg, common.Address{}); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(`{"Name":"fixed","Gas":1}`), cfg.Scenarios[0].GasFeeCapPicker); err != nil {
				t.Fatal(err)
			}
			sender := repair820Account(t)
			load, err := scenario.GenerateWithError(&types.TxScenario{Sender: sender})
			if load != nil || err == nil || sender.Nonce != 0 {
				t.Fatalf("signing error not returned before nonce allocation: %v / %v / %d", load, err, sender.Nonce)
			}
		})
	}
}

func TestRepairM_820_NoopScenarioName(t *testing.T) {
	peer, url, _ := repair820Peer(t)
	cfg := repair820Config(t, scenarios.EVMTransfer, "")
	cfg.Scenarios = append(cfg.Scenarios, config.Scenario{Name: scenarios.EVMTransferNoop, Weight: 1})
	gen, err := generator.NewConfigBasedGenerator(cfg)
	if err != nil {
		t.Fatal(err)
	}
	repair820FundGenerator(t, peer, gen)
	collector := stats.NewCollector()
	for _, load := range gen.GenerateN(8) {
		repair820Successful(t, url, load)
		switch load.Scenario.Name {
		case scenarios.EVMTransferNoop:
			if load.EthTx.Value().Sign() != 0 || *load.EthTx.To() != load.Scenario.Sender.Address {
				t.Fatal("noop generator produced a value transfer")
			}
		case scenarios.EVMTransfer:
			if load.EthTx.Value().Sign() <= 0 || *load.EthTx.To() != load.Scenario.Receiver {
				t.Fatal("value generator lost its transfer")
			}
		default:
			t.Fatalf("unexpected scenario name %q", load.Scenario.Name)
		}
		collector.RecordTransaction(load.Scenario.Name, url, time.Millisecond, true)
	}
	collected := collector.GetStats()
	for _, name := range []string{scenarios.EVMTransfer, scenarios.EVMTransferNoop} {
		if collected.TxCounts[name][url] != 4 {
			t.Fatalf("collector merged %s: %+v", name, collected.TxCounts)
		}
	}
	report := filepath.Join(t.TempDir(), "report.json")
	logger := stats.NewLogger(collector, time.Second, report, false)
	if _, err := logger.LogFinalStats(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(report)
	if err != nil {
		t.Fatal(err)
	}
	var final stats.FinalStats
	if err := json.Unmarshal(data, &final); err != nil {
		t.Fatal(err)
	}
	if len(final.ScenarioDistribution) != 2 || final.ScenarioDistribution[scenarios.EVMTransfer] != 4 || final.ScenarioDistribution[scenarios.EVMTransferNoop] != 4 || final.LoadTestStatistics.TotalTxs != 8 {
		t.Fatalf("final JSON report lost scenario identities: %s", data)
	}
}

func repair820Closed(t *testing.T, closed <-chan struct{}) {
	t.Helper()
	select {
	case <-closed:
	case <-time.After(2 * time.Second):
		t.Fatal("deployment RPC connection was not closed")
	}
}

func TestRepairM_820_DeploymentLifecycle(t *testing.T) {
	t.Run("successful_client_closed_and_offline_signing", func(t *testing.T) {
		peer, url, closed := repair820Peer(t)
		sender := repair820Account(t)
		peer.fund(sender)
		scenario, _ := repair820Deploy(t, "ws"+strings.TrimPrefix(url, "http"), scenarios.Disperse, "", sender)
		repair820Closed(t, closed)
		repair820Successful(t, url, repair820Generate(t, scenario, sender, common.Address{}))
	})
	for _, stage := range []string{"send", "receipt"} {
		t.Run(stage+"_deadline", func(t *testing.T) {
			peer, url, closed := repair820Peer(t)
			sender := repair820Account(t)
			peer.fund(sender)
			release := make(chan struct{})
			defer close(release)
			if stage == "send" {
				peer.holdSend = release
			} else {
				peer.holdReceipts = true
			}
			cfg := repair820Config(t, scenarios.ERC721, "")
			cfg.MockDeploy, cfg.Endpoints = false, []string{"ws" + strings.TrimPrefix(url, "http")}
			scenario := scenarios.CreateScenario(cfg.Scenarios[0]).(scenarios.ContextTxGenerator)
			ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
			defer cancel()
			started := time.Now()
			address, err := scenario.DeployContext(ctx, cfg, sender)
			if address != (common.Address{}) || !errors.Is(err, context.DeadlineExceeded) || time.Since(started) >= 2*time.Second {
				t.Fatalf("%s ignored run deadline: %s / %v", stage, address, err)
			}
			select {
			case <-peer.entered:
			default:
				t.Fatal("deadline test never reached the real deployment send")
			}
			repair820Closed(t, closed)
			if load, err := scenario.GenerateWithError(&types.TxScenario{Sender: sender}); load != nil || err == nil || !strings.Contains(err.Error(), "not deployed") {
				t.Fatal("failed startup admitted transaction generation")
			}
		})
	}
	t.Run("real_admission_error", func(t *testing.T) {
		_, url, closed := repair820Peer(t)
		cfg := repair820Config(t, scenarios.ERC721, "")
		cfg.MockDeploy, cfg.Endpoints = false, []string{"ws" + strings.TrimPrefix(url, "http")}
		scenario := scenarios.CreateScenario(cfg.Scenarios[0]).(scenarios.ContextTxGenerator)
		address, err := scenario.DeployContext(context.Background(), cfg, repair820Account(t))
		if address != (common.Address{}) || err == nil || !strings.Contains(err.Error(), "insufficient funds") {
			t.Fatalf("deployment admission error was lost: %s / %v", address, err)
		}
		repair820Closed(t, closed)
	})
	t.Run("cancel_before_dial", func(t *testing.T) {
		cfg := repair820Config(t, scenarios.ERC721, "")
		cfg.MockDeploy, cfg.Endpoints = false, []string{"unsupported://deployment"}
		scenario := scenarios.CreateScenario(cfg.Scenarios[0]).(scenarios.ContextTxGenerator)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := scenario.DeployContext(ctx, cfg, repair820Account(t)); !errors.Is(err, context.Canceled) {
			t.Fatalf("cancellation not preserved: %v", err)
		}
		if err := scenario.AttachContext(ctx, cfg, common.Address{}); !errors.Is(err, context.Canceled) {
			t.Fatalf("attach cancellation not preserved: %v", err)
		}
	})
	t.Run("attach_reads_actual_contract", func(t *testing.T) {
		_, url, closed := repair820Peer(t)
		cfg := repair820Config(t, scenarios.Disperse, "")
		cfg.MockDeploy, cfg.Endpoints = false, []string{"ws" + strings.TrimPrefix(url, "http")}
		scenario := scenarios.CreateScenario(cfg.Scenarios[0]).(scenarios.ContextTxGenerator)
		err := scenario.AttachContext(context.Background(), cfg, common.HexToAddress("0x820"))
		if !errors.Is(err, bind.ErrNoCode) {
			t.Fatalf("missing deployed contract not refused: %v", err)
		}
		repair820Closed(t, closed)
	})
}
