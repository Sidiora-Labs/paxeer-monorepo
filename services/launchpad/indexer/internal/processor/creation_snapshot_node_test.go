package processor

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/decode"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/receipt"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/deployment"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

type creationSnapshotNode struct {
	ctx       context.Context
	client    *ethclient.Client
	auth      *bind.TransactOpts
	owner     common.Address
	options   receipt.PrivateFixtureRPCOptions
	endpoint  string
	creations map[common.Address]common.Hash
	roles     map[string]deployment.Role
	contracts map[string]*bind.BoundContract
}

var creationSnapshotFixture struct {
	sync.Mutex
	once     sync.Once
	node     *creationSnapshotNode
	options  receipt.PrivateFixtureRPCOptions
	cleanups []func()
}

func TestMain(m *testing.M) {
	code := m.Run()
	for i := len(creationSnapshotFixture.cleanups) - 1; i >= 0; i-- {
		creationSnapshotFixture.cleanups[i]()
	}
	os.Exit(code)
}

func sharedCreationSnapshotNode(t *testing.T) (*creationSnapshotNode, receipt.PrivateFixtureRPCOptions) {
	t.Helper()
	creationSnapshotFixture.Lock()
	t.Cleanup(creationSnapshotFixture.Unlock)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	t.Cleanup(cancel)
	creationSnapshotFixture.once.Do(func() {
		creationSnapshotFixture.node, creationSnapshotFixture.options = prepareCreationSnapshotFactory(t, func(cleanup func()) {
			creationSnapshotFixture.cleanups = append(creationSnapshotFixture.cleanups, cleanup)
		})
	})
	if creationSnapshotFixture.node == nil {
		t.Fatal("creation snapshot node preparation did not finish")
	}
	n := *creationSnapshotFixture.node
	n.ctx = ctx
	auth := *n.auth
	auth.Context = ctx
	n.auth = &auth
	var snapshot string
	if err := n.client.Client().CallContext(ctx, &snapshot, "evm_snapshot"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		var restored bool
		if err := n.client.Client().CallContext(ctx, &restored, "evm_revert", snapshot); err != nil || !restored {
			t.Errorf("restore creation snapshot node: restored=%t: %v", restored, err)
		}
		if err := n.client.Client().CallContext(ctx, nil, "evm_setAutomine", true); err != nil {
			t.Errorf("restore creation snapshot automining: %v", err)
		}
	})
	return &n, creationSnapshotFixture.options
}

func newCreationSnapshotNode(t *testing.T, cleanup func(func())) *creationSnapshotNode {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	t.Cleanup(cancel)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err = listener.Close(); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("anvil", "--host", "127.0.0.1", "--port", strconv.Itoa(port), "--threads", "2", "--chain-id", "125", "--accounts", "1", "--silent")
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	client, err := ethclient.DialContext(ctx, "http://127.0.0.1:"+strconv.Itoa(port))
	if err != nil {
		t.Fatal(err)
	}
	cleanup(client.Close)
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		id, err := client.ChainID(ctx)
		if err == nil && id.Cmp(big.NewInt(125)) == 0 {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-ticker.C:
		}
	}
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	auth, err := bind.NewKeyedTransactorWithChainID(key, big.NewInt(125))
	if err != nil {
		t.Fatal(err)
	}
	auth.Context = ctx
	if err = client.Client().CallContext(ctx, nil, "anvil_setBalance", auth.From, "0x3635c9adc5dea00000"); err != nil {
		t.Fatal(err)
	}
	return &creationSnapshotNode{endpoint: "http://127.0.0.1:" + strconv.Itoa(port), ctx: ctx, client: client, auth: auth, owner: auth.From, creations: map[common.Address]common.Hash{}, roles: map[string]deployment.Role{}, contracts: map[string]*bind.BoundContract{}}
}
func creationSnapshotArtifact(t *testing.T, name string) (abi.ABI, []byte) {
	t.Helper()
	var paths []string
	err := filepath.WalkDir("../../../../../contracts/launchpad/artifacts/contracts", func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && entry.Name() == name+".json" {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil || len(paths) != 1 {
		t.Fatalf("compiled contract %s: %v %v", name, paths, err)
	}
	body, err := os.ReadFile(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	var artifact struct {
		ABI      json.RawMessage
		Bytecode string
	}
	if err = json.Unmarshal(body, &artifact); err != nil {
		t.Fatal(err)
	}
	parsed, err := abi.JSON(bytes.NewReader(artifact.ABI))
	if err != nil {
		t.Fatal(err)
	}
	code := common.FromHex(artifact.Bytecode)
	if len(code) == 0 {
		t.Fatal("contract deployment bytecode absent")
	}
	return parsed, code
}
func (n *creationSnapshotNode) mined(t *testing.T, tx *types.Transaction) *types.Receipt {
	t.Helper()
	receipt, err := bind.WaitMined(n.ctx, n.client, tx)
	if err != nil || receipt == nil || receipt.Status != 1 {
		t.Fatalf("contract transaction %s: %v", tx.Hash(), err)
	}
	return receipt
}
func (n *creationSnapshotNode) deploy(t *testing.T, name string, args ...any) (common.Address, *bind.BoundContract, abi.ABI) {
	t.Helper()
	parsed, code := creationSnapshotArtifact(t, name)
	address, tx, contract, err := bind.DeployContract(n.auth, parsed, code, n.client, args...)
	if err != nil {
		t.Fatalf("deploy %s: %v", name, err)
	}
	r := n.mined(t, tx)
	if r.ContractAddress != address {
		t.Fatal("creation address differs from signed transaction")
	}
	n.creations[address] = tx.Hash()
	return address, contract, parsed
}
func (n *creationSnapshotNode) identity(t *testing.T, address common.Address, kind string) deployment.Role {
	t.Helper()
	code, err := n.client.CodeAt(n.ctx, address, nil)
	if err != nil || len(code) == 0 {
		t.Fatalf("deployed code %s: %v", address, err)
	}
	return deployment.Role{Address: address.Hex(), Kind: kind, RuntimeHash: crypto.Keccak256Hash(code).Hex()}
}
func (n *creationSnapshotNode) direct(t *testing.T, role, name string, args ...any) common.Address {
	t.Helper()
	address, contract, _ := n.deploy(t, name, args...)
	n.roles[role] = n.identity(t, address, "direct")
	n.contracts[role] = contract
	return address
}
func (n *creationSnapshotNode) proxy(t *testing.T, role, name string, args ...any) common.Address {
	t.Helper()
	implementation, _, parsed := n.deploy(t, name)
	input, err := parsed.Pack("initialize", args...)
	if err != nil {
		t.Fatal(err)
	}
	address, _, _ := n.deploy(t, "ERC1967Proxy", implementation, input)
	id := n.identity(t, address, "erc1967")
	id.ImplementationAddress = implementation.Hex()
	id.ImplementationHash = n.identity(t, implementation, "direct").RuntimeHash
	n.roles[role] = id
	n.contracts[role] = bind.NewBoundContract(address, parsed, n.client, n.client, n.client)
	return address
}
func (n *creationSnapshotNode) transact(t *testing.T, role, method string, args ...any) *types.Receipt {
	t.Helper()
	tx, err := n.contracts[role].Transact(n.auth, method, args...)
	if err != nil {
		t.Fatalf("%s.%s: %v", role, method, err)
	}
	return n.mined(t, tx)
}
func deployCreationSnapshotNode(t *testing.T, cleanup func(func())) (*creationSnapshotNode, receipt.PrivateFixtureRPCOptions, common.Address) {
	t.Helper()
	n := newCreationSnapshotNode(t, cleanup)
	supply := big.NewInt(1000000000000000)
	usdl := n.direct(t, "USDL", "SidioraERC20", "USD Ledger", "USDL", supply, n.owner)
	token := n.direct(t, "SidioraERC20", "SidioraERC20", "Market Token", "MKT", supply, n.owner)
	emitter := n.proxy(t, "EventEmitter", "EventEmitter", n.owner)
	n.transact(t, "EventEmitter", "reinitializeV2", n.owner)
	config := n.proxy(t, "ProtocolConfig", "ProtocolConfig", usdl, emitter, n.owner)
	treasury := n.proxy(t, "Treasury", "Treasury", common.Address{}, n.owner)
	registry := n.proxy(t, "PoolRegistry", "PoolRegistry", common.Address{}, n.owner)
	accumulator := n.proxy(t, "FeeAccumulator", "FeeAccumulator", config, treasury, registry, common.Address{}, usdl, n.owner)
	n.proxy(t, "SidioraNFT", "SidioraNFT", "Pool NFT", "NFT", common.Address{}, n.owner)
	n.proxy(t, "OpticalRegistry", "OpticalRegistry", common.Address{}, n.owner)
	implementation, _, _ := n.deploy(t, "SidioraPool")
	beacon, _, _ := n.deploy(t, "PoolBeacon", implementation, n.owner)
	id := n.identity(t, beacon, "beacon")
	id.ImplementationAddress = implementation.Hex()
	id.ImplementationHash = n.identity(t, implementation, "direct").RuntimeHash
	n.roles["PoolBeacon"] = id
	factory := n.proxy(t, "SidioraFactory", "SidioraFactory", beacon, common.HexToAddress(n.roles["SidioraNFT"].Address), registry, emitter, config, treasury, accumulator, usdl, n.owner)
	router := n.proxy(t, "Router", "Router", factory, registry, config, usdl, n.owner)
	n.direct(t, "Quoter", "Quoter")
	n.direct(t, "FeesRouter", "FeesRouter")
	pool := n.proxy(t, "SidioraPool", "SidioraPool", token, usdl, common.Address{}, accumulator, emitter, config, n.owner, big.NewInt(10000000000), big.NewInt(1000000000000))
	n.transact(t, "PoolRegistry", "grantRole", crypto.Keccak256Hash([]byte("FACTORY_ROLE")), n.owner)
	n.transact(t, "PoolRegistry", "register", pool, token, n.owner, common.Address{}, big.NewInt(1))
	n.transact(t, "EventEmitter", "setPoolRegistry", registry)
	n.transact(t, "EventEmitter", "setAuthorizedEmitter", router, true)
	parsed, _ := creationSnapshotArtifact(t, "EventEmitter")
	selector := parsed.Methods["emitRouterTrade"].ID
	var typed [4]byte
	copy(typed[:], selector)
	n.transact(t, "EventEmitter", "setTypedEmitter", router, typed, true)
	n.transact(t, "Treasury", "grantRole", crypto.Keccak256Hash([]byte("DEPOSITOR_ROLE")), accumulator)
	n.transact(t, "FeeAccumulator", "grantRole", crypto.Keccak256Hash([]byte("POOL_ROLE")), pool)
	n.transact(t, "SidioraERC20", "transfer", pool, big.NewInt(1000000000000))
	n.transact(t, "USDL", "transfer", pool, big.NewInt(1000000))
	n.transact(t, "USDL", "approve", router, new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1)))
	catalog, err := os.ReadFile("../../../../../sdk/launchpad/protocol/contracts/v1/chain/abi-catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	var c decode.Catalog
	if err = json.Unmarshal(catalog, &c); err != nil {
		t.Fatal(err)
	}
	genesis, err := n.client.HeaderByNumber(n.ctx, big.NewInt(0))
	if err != nil {
		t.Fatal(err)
	}
	f := receipt.PrivateFixtureRPCOptions{DeploymentVersion: "private-fixture-market", ActivationHeight: 1, Roles: n.roles, CatalogJSON: catalog, GenesisHash: genesis.Hash(), CreationTransactions: n.creations, Timeout: 15 * time.Second}
	for role, id := range n.roles {
		kind := role
		if kind == "USDL" {
			kind = "SidioraERC20"
		}
		for _, contract := range c.Contracts {
			if contract.Name == kind {
				f.Epochs = append(f.Epochs, receipt.RPCCodeEpoch{Deployment: decode.Deployment{Address: common.HexToAddress(id.Address), Role: kind, ABIRevision: contract.ABIRevision, From: decode.Order{Block: 1}}, Identity: id})
			}
		}
	}
	return n, f, pool
}

func prepareCreationSnapshotFactory(t *testing.T, cleanup func(func())) (*creationSnapshotNode, receipt.PrivateFixtureRPCOptions) {
	t.Helper()
	n, options, _ := deployCreationSnapshotNode(t, cleanup)
	factory := common.HexToAddress(n.roles["SidioraFactory"].Address)
	router := common.HexToAddress(n.roles["Router"].Address)
	config := common.HexToAddress(n.roles["ProtocolConfig"].Address)
	for _, b := range []struct {
		role, name string
		account    common.Address
	}{
		{"PoolRegistry", "FACTORY_ROLE", factory}, {"FeeAccumulator", "FACTORY_ROLE", factory}, {"SidioraNFT", "MINTER_ROLE", factory}, {"SidioraFactory", "ROUTER_ROLE", router}, {"SidioraFactory", "ROUTER_ROLE", n.owner},
	} {
		n.transact(t, b.role, "grantRole", crypto.Keccak256Hash([]byte(b.name)), b.account)
	}
	n.transact(t, "EventEmitter", "setSidioraFactory", factory)
	emitterABI, _ := creationSnapshotArtifact(t, "EventEmitter")
	for _, b := range []struct {
		account common.Address
		methods []string
	}{
		{factory, []string{"emitMarketCreated", "emitTokenDeployed"}}, {config, []string{"emitConfigUpdated"}},
	} {
		n.transact(t, "EventEmitter", "setAuthorizedEmitter", b.account, true)
		for _, method := range b.methods {
			var selector [4]byte
			copy(selector[:], emitterABI.Methods[method].ID)
			n.transact(t, "EventEmitter", "setTypedEmitter", b.account, selector, true)
		}
	}
	n.transact(t, "ProtocolConfig", "setFeeWeights", big.NewInt(0), big.NewInt(0), big.NewInt(0))
	return n, options
}

func (n *creationSnapshotNode) create(t *testing.T, options receipt.PrivateFixtureRPCOptions, withSwap bool) (*receipt.RPCQualification, *types.Receipt, common.Address) {
	t.Helper()
	var simulated []any
	args := []any{n.owner, "Snapshot Market", "SNP", uint8(0), common.Address{}}
	if err := n.contracts["SidioraFactory"].Call(&bind.CallOpts{Context: n.ctx, From: n.owner}, &simulated, "createMarketFor", args...); err != nil || len(simulated) != 3 {
		t.Fatalf("simulate market creation: %v", err)
	}
	token := simulated[0].(common.Address)
	pool := simulated[1].(common.Address)
	if withSwap {
		if err := n.client.Client().CallContext(n.ctx, nil, "evm_setAutomine", false); err != nil {
			t.Fatal(err)
		}
	}
	n.auth.GasLimit = 12000000
	tx, err := n.contracts["SidioraFactory"].Transact(n.auth, "createMarketFor", args...)
	if err != nil {
		t.Fatal(err)
	}
	var buy, sync *types.Transaction
	if withSwap {
		n.auth.Nonce = new(big.Int).SetUint64(tx.Nonce() + 1)
		header, err := n.client.HeaderByNumber(n.ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		buy, err = n.contracts["Router"].Transact(n.auth, "buy", pool, big.NewInt(1000000), big.NewInt(0), new(big.Int).SetUint64(header.Time+3600))
		if err != nil {
			t.Fatal(err)
		}
		poolABI, _ := creationSnapshotArtifact(t, "SidioraPool")
		n.auth.Nonce = new(big.Int).SetUint64(tx.Nonce() + 2)
		sync, err = bind.NewBoundContract(pool, poolABI, n.client, n.client, n.client).Transact(n.auth, "syncReserves")
		if err != nil {
			t.Fatal(err)
		}
		if err = n.client.Client().CallContext(n.ctx, nil, "evm_mine"); err != nil {
			t.Fatal(err)
		}
		if err = n.client.Client().CallContext(n.ctx, nil, "evm_setAutomine", true); err != nil {
			t.Fatal(err)
		}
	}
	n.auth.Nonce = nil
	n.auth.GasLimit = 0
	created := n.mined(t, tx)
	if withSwap {
		for _, next := range []*types.Transaction{buy, sync} {
			if got := n.mined(t, next); got.BlockHash != created.BlockHash {
				t.Fatal("creation, swap and reserve update must share one block")
			}
		}
	}
	options.Discovery = nil
	var catalog decode.Catalog
	if err = json.Unmarshal(options.CatalogJSON, &catalog); err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct {
		role, kind string
		address    common.Address
	}{{"SidioraERC20", "", token}, {"SidioraPool", "beacon_proxy", pool}} {
		identity := n.identity(t, item.address, "direct")
		for _, contract := range catalog.Contracts {
			if contract.Name == item.role {
				options.Discovery = append(options.Discovery, receipt.RPCDiscovery{Role: item.role, Kind: item.kind, ABIRevision: contract.ABIRevision, RuntimeHash: identity.RuntimeHash})
			}
		}
	}
	authority, err := receipt.NewPrivateFixtureRPCQualification(n.ctx, n.client, options)
	if err != nil {
		t.Fatal(err)
	}
	n.options = options
	return authority, created, pool
}
