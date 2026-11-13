// This test producer runs the repository's Solidity contracts on a local geth
// node. Coverage talks to the node's real eth and debug HTTP APIs.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/eth"
	"github.com/ethereum/go-ethereum/eth/catalyst"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/eth/tracers"
	_ "github.com/ethereum/go-ethereum/eth/tracers/native"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/trie"
)

type compiled struct {
	ABI json.RawMessage `json:"abi"`
	EVM struct {
		Bytecode struct {
			Object string `json:"object"`
		} `json:"bytecode"`
		DeployedBytecode struct {
			Object string `json:"object"`
		} `json:"deployedBytecode"`
	} `json:"evm"`
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func compile(root string) map[string]compiled {
	sources := map[string]map[string]string{}
	imports := regexp.MustCompile(`import\s+"([^"]+)"`)
	var load func(string)
	load = func(path string) {
		path = filepath.ToSlash(filepath.Clean(path))
		if _, exists := sources[path]; exists {
			return
		}
		raw, err := os.ReadFile(filepath.Join(root, path))
		must(err)
		sources[path] = map[string]string{"content": string(raw)}
		for _, match := range imports.FindAllSubmatch(raw, -1) {
			load(filepath.Join(filepath.Dir(path), string(match[1])))
		}
	}
	for _, path := range []string{"core/SidioraPool.sol", "core/SidioraFactory.sol", "core/SidioraNFT.sol", "core/SidioraERC20.sol", "core/PoolBeacon.sol", "data/EventEmitter.sol", "data/PoolRegistry.sol", "data/FeeAccumulator.sol", "protocol/ProtocolConfig.sol", "protocol/Treasury.sol", "periphery/Router.sol"} {
		load(path)
	}
	input, err := json.Marshal(map[string]any{"language": "Solidity", "sources": sources, "settings": map[string]any{
		"optimizer": map[string]any{"enabled": true, "runs": 200}, "viaIR": true, "evmVersion": "cancun",
		"outputSelection": map[string]any{"*": map[string]any{"*": []string{"abi", "evm.bytecode.object", "evm.deployedBytecode.object"}}},
	}})
	must(err)
	dir, err := os.MkdirTemp("", "repair28-solc-")
	must(err)
	defer os.RemoveAll(dir)
	in, err := os.Create(filepath.Join(dir, "input.json"))
	must(err)
	_, err = in.Write(input)
	must(err)
	_, err = in.Seek(0, 0)
	must(err)
	defer in.Close()
	out, err := os.Create(filepath.Join(dir, "output.json"))
	must(err)
	defer out.Close()
	cmd := exec.Command("npx", "--yes", "--package=solc@0.8.24", "solcjs", "--standard-json")
	cmd.Stdin, cmd.Stdout, cmd.Stderr = in, out, os.Stderr
	must(cmd.Run())
	_, err = out.Seek(0, 0)
	must(err)
	raw, err := io.ReadAll(out)
	must(err)
	start := bytes.IndexByte(raw, '{')
	if start < 0 {
		panic(string(raw))
	}
	var result struct {
		Contracts map[string]map[string]compiled                `json:"contracts"`
		Errors    []struct{ Severity, FormattedMessage string } `json:"errors"`
	}
	must(json.Unmarshal(raw[start:], &result))
	for _, problem := range result.Errors {
		if problem.Severity == "error" {
			panic(problem.FormattedMessage)
		}
	}
	contracts := map[string]compiled{}
	for _, file := range result.Contracts {
		for name, contract := range file {
			contracts[name] = contract
		}
	}
	return contracts
}

func main() {
	contracts := compile(os.Args[1])
	key, err := crypto.GenerateKey()
	must(err)
	account := crypto.PubkeyToAddress(key.PublicKey)
	names := []string{"EventEmitter", "ProtocolConfig", "PoolRegistry", "Treasury", "FeeAccumulator", "SidioraNFT", "SidioraFactory", "Router", "SidioraPool"}
	addresses := map[string]common.Address{}
	abis := map[string]abi.ABI{}
	alloc := types.GenesisAlloc{account: {Balance: new(big.Int).Exp(big.NewInt(10), big.NewInt(24), nil)}}
	for i, name := range names {
		address := common.BigToAddress(big.NewInt(0x10000 + int64(i)))
		addresses[name] = address
		contract := contracts[name]
		a, err := abi.JSON(bytes.NewReader(contract.ABI))
		must(err)
		abis[name] = a
		alloc[address] = types.Account{Code: common.FromHex(contract.EVM.DeployedBytecode.Object), Balance: new(big.Int)}
	}
	// Use a non-precompile address for every Solidity contract.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	must(err)
	port := listener.Addr().(*net.TCPAddr).Port
	must(listener.Close())
	nodeConfig := node.DefaultConfig
	nodeConfig.DataDir = ""
	nodeConfig.P2P = p2p.Config{NoDiscovery: true}
	nodeConfig.HTTPHost, nodeConfig.HTTPPort, nodeConfig.HTTPModules = "127.0.0.1", port, []string{"eth", "debug"}
	stack, err := node.New(&nodeConfig)
	must(err)
	defer stack.Close()
	chainConfig := *params.AllDevChainProtocolChanges
	chainConfig.ChainID = big.NewInt(125)
	ethConfig := ethconfig.Defaults
	ethConfig.Genesis = &core.Genesis{Config: &chainConfig, GasLimit: 60000000, Alloc: alloc}
	ethConfig.SyncMode = ethconfig.FullSync
	ethConfig.TxPool.NoLocals = true
	ethConfig.LogNoHistory = true
	ethConfig.Miner.GasCeil = 60000000
	backend, err := eth.New(stack, &ethConfig)
	must(err)
	// The simulated backend omits the debug tracing service. Register geth's
	// real tracer before starting the node so coverage uses the production API.
	stack.RegisterAPIs(tracers.APIs(backend.APIBackend))
	must(stack.Start())
	chainBeacon, err := catalyst.NewSimulatedBeacon(0, common.Address{}, backend)
	must(err)
	defer chainBeacon.Stop()
	must(chainBeacon.Fork(backend.BlockChain().GetCanonicalHash(0)))
	client := ethclient.NewClient(stack.Attach())
	defer client.Close()
	ctx := context.Background()
	nonce := uint64(0)
	send := func(to *common.Address, data []byte) *types.Receipt {
		tx, err := types.SignTx(types.NewTx(&types.LegacyTx{Nonce: nonce, To: to, Data: data, Gas: 16000000, GasPrice: big.NewInt(2000000000)}), types.LatestSignerForChainID(big.NewInt(125)), key)
		must(err)
		nonce++
		must(client.SendTransaction(ctx, tx))
		chainBeacon.Commit()
		receipt, err := client.TransactionReceipt(ctx, tx.Hash())
		must(err)
		if receipt.Status != 1 {
			panic(fmt.Sprintf("producer transaction failed: to=%v input=%x hash=%s", to, data, tx.Hash()))
		}
		return receipt
	}
	call := func(name, method string, args ...any) *types.Receipt {
		input, err := abis[name].Pack(method, args...)
		must(err)
		target := addresses[name]
		return send(&target, input)
	}
	deploy := func(name string, args ...any) common.Address {
		contract := contracts[name]
		a, err := abi.JSON(bytes.NewReader(contract.ABI))
		must(err)
		input, err := a.Pack("", args...)
		must(err)
		receipt := send(nil, append(common.FromHex(contract.EVM.Bytecode.Object), input...))
		return receipt.ContractAddress
	}
	zero := common.Address{}
	usdl := deploy("SidioraERC20", "Settlement", "USDL", new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil), account)
	addresses["USDL"] = usdl
	abis["USDL"], err = abi.JSON(bytes.NewReader(contracts["SidioraERC20"].ABI))
	must(err)
	beacon := deploy("PoolBeacon", addresses["SidioraPool"], account)
	call("EventEmitter", "initialize", account)
	call("ProtocolConfig", "initialize", usdl, addresses["EventEmitter"], account)
	call("PoolRegistry", "initialize", addresses["EventEmitter"], account)
	call("Treasury", "initialize", addresses["EventEmitter"], account)
	call("FeeAccumulator", "initialize", addresses["ProtocolConfig"], addresses["Treasury"], addresses["PoolRegistry"], addresses["EventEmitter"], usdl, account)
	call("SidioraNFT", "initialize", "Fee Rights", "FEE", addresses["EventEmitter"], account)
	call("SidioraFactory", "initialize", beacon, addresses["SidioraNFT"], addresses["PoolRegistry"], addresses["EventEmitter"], addresses["ProtocolConfig"], addresses["Treasury"], addresses["FeeAccumulator"], usdl, account)
	call("Router", "initialize", addresses["SidioraFactory"], addresses["PoolRegistry"], addresses["ProtocolConfig"], usdl, account)
	grant := func(name, role string, address common.Address) {
		call(name, "grantRole", crypto.Keccak256Hash([]byte(role)), address)
	}
	grant("PoolRegistry", "FACTORY_ROLE", addresses["SidioraFactory"])
	grant("FeeAccumulator", "FACTORY_ROLE", addresses["SidioraFactory"])
	grant("SidioraNFT", "MINTER_ROLE", addresses["SidioraFactory"])
	grant("SidioraFactory", "ROUTER_ROLE", addresses["Router"])
	grant("Treasury", "DEPOSITOR_ROLE", addresses["FeeAccumulator"])
	grant("Treasury", "DEPOSITOR_ROLE", addresses["SidioraFactory"])
	call("EventEmitter", "setPoolRegistry", addresses["PoolRegistry"])
	call("EventEmitter", "setSidioraFactory", addresses["SidioraFactory"])
	for _, item := range []struct{ Name, Method string }{{"SidioraNFT", "emitNftMint"}, {"Router", "emitRouterTrade"}, {"FeeAccumulator", "emitFeeFlow"}} {
		var selector [4]byte
		copy(selector[:], abis["EventEmitter"].Methods[item.Method].ID)
		call("EventEmitter", "setTypedEmitter", addresses[item.Name], selector, true)
		call("EventEmitter", "setAuthorizedEmitter", addresses[item.Name], true)
	}
	call("USDL", "approve", addresses["SidioraFactory"], big.NewInt(1000000000))
	creation := call("SidioraFactory", "createMarket", "Coverage", "COV", uint8(0), zero)
	var pool, token common.Address
	event := abis["SidioraFactory"].Events["MarketCreated"]
	for _, l := range creation.Logs {
		if l.Address != addresses["SidioraFactory"] || len(l.Topics) == 0 || l.Topics[0] != event.ID {
			continue
		}
		fields := map[string]any{}
		must(event.Inputs.NonIndexed().UnpackIntoMap(fields, l.Data))
		var indexed abi.Arguments
		for _, arg := range event.Inputs {
			if arg.Indexed {
				indexed = append(indexed, arg)
			}
		}
		must(abi.ParseTopicsIntoMap(fields, indexed, l.Topics[1:]))
		pool, token = fields["pool"].(common.Address), fields["token"].(common.Address)
	}
	if pool == zero || token == zero {
		panic("real factory creation missing")
	}
	addresses["SidioraPool"], addresses["SidioraERC20"] = pool, token
	abis["SidioraERC20"], err = abi.JSON(bytes.NewReader(contracts["SidioraERC20"].ABI))
	must(err)
	funded := call("USDL", "transfer", pool, big.NewInt(100000000))
	direct := call("SidioraPool", "swap", big.NewInt(100000000), big.NewInt(1), true, account, big.NewInt(1<<62))
	call("USDL", "approve", addresses["Router"], big.NewInt(1000000000))
	routed := call("Router", "buy", pool, big.NewInt(100000000), big.NewInt(1), big.NewInt(1<<62))
	var selector [4]byte
	copy(selector[:], abis["EventEmitter"].Methods["emitSwap"].ID)
	call("EventEmitter", "setTypedEmitter", account, selector, true)
	call("EventEmitter", "setAuthorizedEmitter", account, true)
	forged := call("EventEmitter", "emitSwap", common.BytesToHash(pool.Bytes()), account, true, big.NewInt(100000000), big.NewInt(1), big.NewInt(0), big.NewInt(1))
	copy(selector[:], abis["EventEmitter"].Methods["emitFeeFlow"].ID)
	call("EventEmitter", "setTypedEmitter", account, selector, true)
	orphan := call("EventEmitter", "emitFeeFlow", uint8(0), pool, account, big.NewInt(1), big.NewInt(0), big.NewInt(1), big.NewInt(0))
	balanceInput, err := abis["SidioraERC20"].Pack("balanceOf", account)
	must(err)
	balanceOutput, err := client.CallContract(ctx, ethereum.CallMsg{To: &token, Data: balanceInput}, nil)
	must(err)
	balanceValues, err := abis["SidioraERC20"].Unpack("balanceOf", balanceOutput)
	must(err)
	sellAmount := new(big.Int).Div(balanceValues[0].(*big.Int), big.NewInt(4))
	if sellAmount.Sign() == 0 {
		panic("bought token balance missing")
	}
	call("SidioraERC20", "approve", addresses["Router"], sellAmount)
	sold := call("Router", "sell", pool, sellAmount, big.NewInt(1), big.NewInt(1<<62))
	// A block crossing RLP index 0x7f supplies independent geth trie roots for
	// the coverage receipt verifier, including both legacy and typed values.
	var lists []map[string]any
	for _, count := range []int{0, 1, 2, 16, 128, 129, 300} {
		txs := make(types.Transactions, count)
		rs := make(types.Receipts, count)
		for i := range txs {
			txs[i] = types.NewTx(&types.LegacyTx{Nonce: uint64(i), Gas: 21000, GasPrice: big.NewInt(1)})
			rs[i] = &types.Receipt{Type: uint8(i % 3), Status: 1, CumulativeGasUsed: uint64(i+1) * 21000, Logs: []*types.Log{}}
		}
		var rawTxs [][]byte
		for _, tx := range txs {
			raw, err := tx.MarshalBinary()
			must(err)
			rawTxs = append(rawTxs, raw)
		}
		lists = append(lists, map[string]any{"Transactions": rawTxs, "Receipts": rs, "TransactionRoot": types.DeriveSha(txs, trie.NewStackTrie(nil)), "ReceiptRoot": types.DeriveSha(rs, trie.NewStackTrie(nil))})
	}
	must(json.NewEncoder(os.Stdout).Encode(map[string]any{"Endpoint": fmt.Sprintf("http://127.0.0.1:%d", port), "Addresses": addresses, "Account": account, "Created": creation.TxHash, "Funded": funded.TxHash, "Direct": direct.TxHash, "Routed": routed.TxHash, "Forged": forged.TxHash, "Orphan": orphan.TxHash, "Sold": sold.TxHash, "Lists": lists}))
	_, err = io.Copy(io.Discard, os.Stdin)
	if err != nil && !strings.Contains(err.Error(), "closed") {
		must(err)
	}
}
