package receipt

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/decode"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/deployment"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

type privateNodeFixture struct {
	ctx       context.Context
	client    *ethclient.Client
	auth      *bind.TransactOpts
	owner     common.Address
	creations map[common.Address]common.Hash
	roles     map[string]deployment.Role
	contracts map[string]*bind.BoundContract
}

func privateNode(t *testing.T) *privateNodeFixture {
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
	cmd := exec.CommandContext(ctx, "anvil", "--host", "127.0.0.1", "--port", strconv.Itoa(port), "--threads", "2", "--chain-id", "125", "--accounts", "1", "--silent")
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	client, err := ethclient.DialContext(ctx, "http://127.0.0.1:"+strconv.Itoa(port))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
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
	key, err := crypto.HexToECDSA("ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80")
	if err != nil {
		t.Fatal(err)
	}
	auth, err := bind.NewKeyedTransactorWithChainID(key, big.NewInt(125))
	if err != nil {
		t.Fatal(err)
	}
	auth.Context = ctx
	return &privateNodeFixture{ctx: ctx, client: client, auth: auth, owner: auth.From, creations: map[common.Address]common.Hash{}, roles: map[string]deployment.Role{}, contracts: map[string]*bind.BoundContract{}}
}
func privateArtifact(t *testing.T, name string) (abi.ABI, []byte) {
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
func (n *privateNodeFixture) mined(t *testing.T, tx *types.Transaction) *types.Receipt {
	t.Helper()
	receipt, err := bind.WaitMined(n.ctx, n.client, tx)
	if err != nil || receipt == nil || receipt.Status != 1 {
		t.Fatalf("contract transaction %s: %v", tx.Hash(), err)
	}
	return receipt
}
func (n *privateNodeFixture) deploy(t *testing.T, name string, args ...any) (common.Address, *bind.BoundContract, abi.ABI) {
	t.Helper()
	parsed, code := privateArtifact(t, name)
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
func (n *privateNodeFixture) identity(t *testing.T, address common.Address, kind string) deployment.Role {
	t.Helper()
	code, err := n.client.CodeAt(n.ctx, address, nil)
	if err != nil || len(code) == 0 {
		t.Fatalf("deployed code %s: %v", address, err)
	}
	return deployment.Role{Address: address.Hex(), Kind: kind, RuntimeHash: crypto.Keccak256Hash(code).Hex()}
}
func (n *privateNodeFixture) direct(t *testing.T, role, name string, args ...any) common.Address {
	t.Helper()
	address, contract, _ := n.deploy(t, name, args...)
	n.roles[role] = n.identity(t, address, "direct")
	n.contracts[role] = contract
	return address
}
func (n *privateNodeFixture) proxy(t *testing.T, role, name string, args ...any) common.Address {
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
func (n *privateNodeFixture) transact(t *testing.T, role, method string, args ...any) *types.Receipt {
	t.Helper()
	tx, err := n.contracts[role].Transact(n.auth, method, args...)
	if err != nil {
		t.Fatalf("%s.%s: %v", role, method, err)
	}
	return n.mined(t, tx)
}
func deployPrivateQualification(t *testing.T) (*privateNodeFixture, PrivateFixtureRPCOptions, common.Address) {
	t.Helper()
	n := privateNode(t)
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
	parsed, _ := privateArtifact(t, "EventEmitter")
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
	f := PrivateFixtureRPCOptions{DeploymentVersion: "private-fixture-market", ActivationHeight: 1, Roles: n.roles, CatalogJSON: catalog, GenesisHash: genesis.Hash(), CreationTransactions: n.creations, Timeout: 15 * time.Second}
	for role, id := range n.roles {
		kind := role
		if kind == "USDL" {
			kind = "SidioraERC20"
		}
		for _, contract := range c.Contracts {
			if contract.Name == kind {
				f.Epochs = append(f.Epochs, RPCCodeEpoch{Deployment: decode.Deployment{Address: common.HexToAddress(id.Address), Role: kind, ABIRevision: contract.ABIRevision, From: decode.Order{Block: 1}}, Identity: id})
			}
		}
	}
	return n, f, pool
}

func TestPrivateQualificationBindsActualCreationsAndPoolExecution(t *testing.T) {
	n, options, pool := deployPrivateQualification(t)
	authority, err := NewPrivateFixtureRPCQualification(n.ctx, n.client, options)
	if err != nil || authority == nil || !authority.IsPrivateFixture() {
		t.Fatalf("private contract binding: %v", err)
	}
	header, err := n.client.HeaderByNumber(n.ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := authority.StableMetadata(n.ctx, pool, header.Hash())
	if err != nil || metadata.Pool != pool || metadata.Token != common.HexToAddress(n.roles["SidioraERC20"].Address) || metadata.SettlementDecimals != 6 || metadata.TokenDecimals != 6 || metadata.DeploymentID != options.DeploymentVersion || metadata.PoolCodeEpoch == "" {
		t.Fatalf("actual pool metadata %+v %v", metadata, err)
	}
	reserves, err := authority.PoolReserves(n.ctx, pool, header.Hash())
	if err != nil || reserves.VirtualUSDL == nil || reserves.VirtualUSDL.Cmp(big.NewInt(10000000000)) != 0 || reserves.Token == nil || reserves.Token.Cmp(big.NewInt(1000000000000)) != 0 {
		t.Fatalf("actual reserves %+v %v", reserves, err)
	}
	buy := n.transact(t, "SidioraPool", "swap", big.NewInt(1000000), big.NewInt(0), true, n.owner, new(big.Int).SetUint64(header.Time+3600))
	bundle, err := authority.Bundle(n.ctx, buy.TxHash)
	if err != nil {
		t.Fatal(err)
	}
	if err = authority.ValidateReceipt(n.ctx, bundle); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, log := range buy.Logs {
		env, err := authority.Registry().Decode(*log)
		if err != nil {
			t.Fatal(err)
		}
		if env.Family == nil || !env.Family.EconomicFill {
			continue
		}
		found = true
		policy, err := authority.ExecutionPolicy(n.ctx, bundle, uint32(log.Index))
		if err != nil || policy.Quality != "unavailable" {
			t.Fatalf("actual execution policy %+v %v", policy, err)
		}
	}
	if !found {
		t.Fatal("actual pool swap has no economic fill")
	}
	for _, tc := range []struct {
		name string
		edit func(*PrivateFixtureRPCOptions)
	}{
		{"latest activation", func(o *PrivateFixtureRPCOptions) {
			o.ActivationHeight = header.Number.Uint64() + 100000
			for i := range o.Epochs {
				o.Epochs[i].Deployment.From.Block = o.ActivationHeight
			}
		}},
		{"wrong creation address", func(o *PrivateFixtureRPCOptions) {
			o.CreationTransactions = map[common.Address]common.Hash{n.owner: buy.TxHash}
		}},
		{"missing implementation creation", func(o *PrivateFixtureRPCOptions) {
			o.CreationTransactions = map[common.Address]common.Hash{}
			for a, h := range options.CreationTransactions {
				if a != common.HexToAddress(n.roles["EventEmitter"].ImplementationAddress) {
					o.CreationTransactions[a] = h
				}
			}
		}},
		{"runtime digest", func(o *PrivateFixtureRPCOptions) {
			id := o.Roles["Quoter"]
			id.RuntimeHash = common.HexToHash("0x1").Hex()
			o.Roles["Quoter"] = id
			for i := range o.Epochs {
				if o.Epochs[i].Deployment.Role == "Quoter" {
					o.Epochs[i].Identity = id
				}
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			changed, err := clone(options)
			if err != nil {
				t.Fatal(err)
			}
			tc.edit(&changed)
			if _, err = NewPrivateFixtureRPCQualification(n.ctx, n.client, changed); err == nil {
				t.Fatal("altered private contract identity accepted")
			}
		})
	}
	changed, err := clone(bundle)
	if err != nil {
		t.Fatal(err)
	}
	changed.PrepayReceipts = []*types.Receipt{nil}
	if err = authority.ValidateReceipt(n.ctx, changed); !errors.Is(err, ErrQualification) {
		t.Fatalf("missing prepayment receipt %v", err)
	}
	changed, err = clone(bundle)
	if err != nil {
		t.Fatal(err)
	}
	companion, err := authority.Bundle(n.ctx, n.creations[pool])
	if err != nil {
		t.Fatal(err)
	}
	changed.PrepayReceipts = []*types.Receipt{companion.Receipt}
	if err = authority.ValidateReceipt(n.ctx, changed); err != nil {
		t.Fatalf("committed companion receipt %v", err)
	}
	changed.PrepayReceipts[0].GasUsed++
	if err = authority.ValidateReceipt(n.ctx, changed); !errors.Is(err, ErrQualification) {
		t.Fatalf("altered companion receipt %v", err)
	}
	for _, epoch := range authority.options.Epochs {
		bad := epoch
		bad.Identity.RuntimeHash = common.HexToHash("0x1").Hex()
		if _, err = authority.validateCodeUncached(n.ctx, bad, header.Hash()); err == nil {
			t.Fatal("changed runtime identity admitted")
		}
	}
	if !strings.HasPrefix(metadata.PoolCodeEpoch, "rpc-code-epoch-v1") && len(metadata.PoolCodeEpoch) != 64 {
		t.Fatal("pool epoch identity missing")
	}
}

func TestPrivateFactoryMarketQualifiesConstantFeeBuyAndSell(t *testing.T) {
	n, options, _ := deployPrivateQualification(t)
	factory := common.HexToAddress(n.roles["SidioraFactory"].Address)
	router := common.HexToAddress(n.roles["Router"].Address)
	config := common.HexToAddress(n.roles["ProtocolConfig"].Address)
	for _, binding := range []struct {
		role, name string
		account    common.Address
	}{
		{"PoolRegistry", "FACTORY_ROLE", factory}, {"FeeAccumulator", "FACTORY_ROLE", factory}, {"SidioraNFT", "MINTER_ROLE", factory}, {"SidioraFactory", "ROUTER_ROLE", router}, {"SidioraFactory", "ROUTER_ROLE", n.owner},
	} {
		n.transact(t, binding.role, "grantRole", crypto.Keccak256Hash([]byte(binding.name)), binding.account)
	}
	n.transact(t, "EventEmitter", "setSidioraFactory", factory)
	emitterABI, _ := privateArtifact(t, "EventEmitter")
	for _, binding := range []struct {
		account common.Address
		methods []string
	}{{factory, []string{"emitMarketCreated", "emitTokenDeployed"}}, {config, []string{"emitConfigUpdated"}}} {
		n.transact(t, "EventEmitter", "setAuthorizedEmitter", binding.account, true)
		for _, method := range binding.methods {
			var selector [4]byte
			copy(selector[:], emitterABI.Methods[method].ID)
			n.transact(t, "EventEmitter", "setTypedEmitter", binding.account, selector, true)
		}
	}
	n.transact(t, "ProtocolConfig", "setFeeWeights", big.NewInt(0), big.NewInt(0), big.NewInt(0))
	created := n.transact(t, "SidioraFactory", "createMarketFor", n.owner, "Factory Market", "FCT", uint8(0), common.Address{})
	factoryABI, _ := privateArtifact(t, "SidioraFactory")
	var token, pool common.Address
	for _, log := range created.Logs {
		if log.Address == factory && len(log.Topics) > 0 && log.Topics[0] == factoryABI.Events["MarketCreated"].ID {
			token = common.BytesToAddress(log.Topics[1].Bytes())
			pool = common.BytesToAddress(log.Topics[2].Bytes())
		}
	}
	if token == (common.Address{}) || pool == (common.Address{}) || token == pool {
		t.Fatal("factory creation addresses absent")
	}
	var catalog decode.Catalog
	if err := json.Unmarshal(options.CatalogJSON, &catalog); err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct {
		role, kind string
		address    common.Address
	}{{"SidioraERC20", "direct", token}, {"SidioraPool", "beacon_proxy", pool}} {
		identity := n.identity(t, item.address, item.kind)
		for _, contract := range catalog.Contracts {
			if contract.Name == item.role {
				options.Discovery = append(options.Discovery, RPCDiscovery{Role: item.role, Kind: func() string {
					if item.role == "SidioraERC20" {
						return ""
					}
					return item.kind
				}(), ABIRevision: contract.ABIRevision, RuntimeHash: identity.RuntimeHash})
			}
		}
	}
	authority, err := NewPrivateFixtureRPCQualification(n.ctx, n.client, options)
	if err != nil {
		t.Fatal(err)
	}
	authority, err = authority.discover(n.ctx, types.Receipts{created})
	if err != nil {
		t.Fatal(err)
	}
	if len(authority.DiscoveredEpochs()) != 2 {
		t.Fatalf("factory discoveries %+v", authority.DiscoveredEpochs())
	}
	configured, err := newRPCQualification(n.ctx, n.client, authority.options, authority.manifestHash, true, nil)
	if err != nil || configured == nil {
		t.Fatalf("discovered deployment configuration %v", err)
	}
	market := DiscoveredMarket{Token: token, Pool: pool, Height: created.BlockNumber.Uint64(), TransactionIndex: created.TransactionIndex}
	for i := 0; i < 2; i++ {
		if err = authority.Rehydrate(n.ctx, []DiscoveredMarket{market}); err != nil {
			t.Fatalf("durable market restart %d: %v", i, err)
		}
		if len(authority.DiscoveredEpochs()) != 2 {
			t.Fatal("restart duplicated market epochs")
		}
	}
	for _, mode := range []string{"pool kind", "pool beacon", "token kind", "epoch coverage"} {
		t.Run(mode, func(t *testing.T) {
			changed, e := clone(configured.options)
			if e != nil {
				t.Fatal(e)
			}
			switch mode {
			case "pool kind", "pool beacon":
				for i, epoch := range changed.Epochs {
					if epoch.Deployment.Address == pool {
						if mode == "pool kind" {
							changed.Epochs[i].Identity.Kind = valueDirect
							changed.Epochs[i].Identity.BeaconAddress = ""
							changed.Epochs[i].Identity.ImplementationAddress = ""
							changed.Epochs[i].Identity.ImplementationHash = ""
						} else {
							changed.Epochs[i].Identity.BeaconAddress = token.Hex()
						}
					}
				}
			case "token kind":
				for i, epoch := range changed.Epochs {
					if epoch.Deployment.Address == token {
						changed.Epochs[i].Deployment.Role = "Router"
					}
				}
			case "epoch coverage":
				filtered := changed.Epochs[:0]
				for _, epoch := range changed.Epochs {
					if epoch.Deployment.Role != "Router" {
						filtered = append(filtered, epoch)
					}
				}
				changed.Epochs = filtered
			}
			if _, e = newRPCQualification(n.ctx, n.client, changed, configured.manifestHash, true, nil); e == nil {
				t.Fatal("inconsistent market configuration admitted")
			}
		})
	}
	tokenABI, _ := privateArtifact(t, "SidioraERC20")
	n.contracts["FactoryToken"] = bind.NewBoundContract(token, tokenABI, n.client, n.client, n.client)
	maximum := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))
	n.transact(t, "FactoryToken", "approve", router, maximum)
	for _, side := range []string{"buy", "sell"} {
		header, err := n.client.HeaderByNumber(n.ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		amount := big.NewInt(1000000)
		if side == "sell" {
			amount = big.NewInt(100000000)
		}
		trade := n.transact(t, "Router", side, pool, amount, big.NewInt(0), new(big.Int).SetUint64(header.Time+3600))
		bundle, err := authority.Bundle(n.ctx, trade.TxHash)
		if err != nil {
			t.Fatal(err)
		}
		qualified, err := Qualify(n.ctx, authority.Registry(), authority, bundle)
		if err != nil {
			t.Fatal(err)
		}
		fills, err := qualified.Fills(n.ctx)
		if err != nil || len(fills) != 1 {
			t.Fatalf("actual %s fills %+v %v", side, fills, err)
		}
		fill := fills[0]
		if fill.Side != side || fill.PoolAddress != strings.ToLower(pool.Hex()) || fill.Token.Address != strings.ToLower(token.Hex()) || fill.InputDerivationQuality != "exact" || fill.PricedInputRaw == nil || *fill.PricedInputRaw != amount.String() || fill.AMMInputAfterFeeRaw == nil || fill.Execution.Quality != "qualified_execution" || fill.Execution.FeeBPS == nil || *fill.Execution.FeeBPS != 30 || fill.Attribution.Quality != "qualified_calldata" || fill.Attribution.Payer == nil || *fill.Attribution.Payer != strings.ToLower(n.owner.Hex()) {
			t.Fatalf("actual %s accounting %+v", side, fill)
		}
		paid, ok := new(big.Int).SetString(fill.PaidInputRaw, 10)
		if !ok {
			t.Fatal("paid input is not an integer")
		}
		fee, ok := new(big.Int).SetString(fill.FeeRaw, 10)
		if !ok || new(big.Int).Sub(paid, fee).String() != *fill.AMMInputAfterFeeRaw {
			t.Fatalf("fee derivation %+v", fill)
		}
		index, err := strconv.ParseUint(fill.Position.LogIndex, 10, 32)
		if err != nil {
			t.Fatal(err)
		}
		exerciseExecutionPolicyStateFaults(t, n, authority, bundle, uint32(index))
		if _, err = authority.ExecutionPolicy(n.ctx, bundle, ^uint32(0)); !errors.Is(err, ErrQualification) {
			t.Fatalf("absent economic position %v", err)
		}
	}
}
