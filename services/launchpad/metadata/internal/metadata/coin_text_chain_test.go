package metadata

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/ethclient/simulated"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/params"
)

type coinTextContract struct {
	address common.Address
	abi     abi.ABI
	bound   *bind.BoundContract
}

type coinTextChain struct {
	backend *simulated.Backend
	auth    *bind.TransactOpts
	factory coinTextContract
	emitter coinTextContract
}

func coinTextArtifact(t *testing.T, path string) (abi.ABI, []byte) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var artifact struct {
		ABI      json.RawMessage `json:"abi"`
		Bytecode string          `json:"bytecode"`
	}
	if err = json.Unmarshal(raw, &artifact); err != nil {
		t.Fatal(err)
	}
	parsed, err := abi.JSON(bytes.NewReader(artifact.ABI))
	if err != nil {
		t.Fatal(err)
	}
	return parsed, common.FromHex(artifact.Bytecode)
}

func (c *coinTextChain) mine(t *testing.T, tx *types.Transaction, err error) *types.Receipt {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
	c.backend.Commit()
	receipt, err := c.backend.Client().TransactionReceipt(context.Background(), tx.Hash())
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Status != types.ReceiptStatusSuccessful {
		t.Fatalf("contract transaction reverted: %s", tx.Hash())
	}
	return receipt
}

func (c *coinTextChain) send(t *testing.T, contract coinTextContract, method string, args ...any) *types.Receipt {
	t.Helper()
	tx, err := contract.bound.Transact(c.auth, method, args...)
	return c.mine(t, tx, err)
}

func (c *coinTextChain) deploy(t *testing.T, path string, args ...any) coinTextContract {
	t.Helper()
	parsed, code := coinTextArtifact(t, path)
	address, tx, bound, err := bind.DeployContract(c.auth, parsed, code, c.backend.Client(), args...)
	c.mine(t, tx, err)
	return coinTextContract{address: address, abi: parsed, bound: bound}
}

func newCoinTextChain(t *testing.T) *coinTextChain {
	t.Helper()
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	auth, err := bind.NewKeyedTransactorWithChainID(key, big.NewInt(125))
	if err != nil {
		t.Fatal(err)
	}
	chainConfig := *params.AllDevChainProtocolChanges
	chainConfig.ChainID = big.NewInt(125)
	backend := simulated.NewBackend(types.GenesisAlloc{auth.From: {Balance: new(big.Int).Exp(big.NewInt(10), big.NewInt(24), nil)}}, func(_ *node.Config, config *ethconfig.Config) {
		config.Genesis.Config = &chainConfig
		config.NetworkId = 125
	})
	t.Cleanup(func() {
		if err := backend.Close(); err != nil {
			t.Error(err)
		}
	})
	backend.Commit()
	c := &coinTextChain{backend: backend, auth: auth}
	artifactPath := func(name string) string { return filepath.Join("testdata", "coin_text", name+".json") }
	proxyPath := "../../../../../sdk/launchpad/protocol/bindings/testdata/contract_events/ERC1967Proxy.json"
	proxy := func(name string, args ...any) coinTextContract {
		implementation := c.deploy(t, artifactPath(name))
		init, err := implementation.abi.Pack("initialize", args...)
		if err != nil {
			t.Fatal(err)
		}
		deployed := c.deploy(t, proxyPath, implementation.address, init)
		deployed.abi = implementation.abi
		deployed.bound = bind.NewBoundContract(deployed.address, deployed.abi, backend.Client(), backend.Client(), backend.Client())
		return deployed
	}
	collateral := c.deploy(t, artifactPath("SidioraERC20"), "USD Ledger", "USDL", big.NewInt(1_000_000_000_000), auth.From)
	c.emitter = proxy("EventEmitter", auth.From)
	c.send(t, c.emitter, "reinitializeV2", auth.From)
	config := proxy("ProtocolConfig", collateral.address, c.emitter.address, auth.From)
	treasury := proxy("Treasury", c.emitter.address, auth.From)
	registry := proxy("PoolRegistry", c.emitter.address, auth.From)
	accumulator := proxy("FeeAccumulator", config.address, treasury.address, registry.address, c.emitter.address, collateral.address, auth.From)
	nft := proxy("SidioraNFT", "Sidiora Pool NFT", "SIDNFT", c.emitter.address, auth.From)
	pool := c.deploy(t, artifactPath("SidioraPool"))
	beacon := c.deploy(t, artifactPath("PoolBeacon"), pool.address, auth.From)
	c.factory = proxy("SidioraFactory", beacon.address, nft.address, registry.address, c.emitter.address, config.address, treasury.address, accumulator.address, collateral.address, auth.From)
	for _, grant := range []struct {
		contract coinTextContract
		role     string
	}{
		{registry, "FACTORY_ROLE"}, {accumulator, "FACTORY_ROLE"}, {nft, "MINTER_ROLE"},
	} {
		c.send(t, grant.contract, "grantRole", crypto.Keccak256Hash([]byte(grant.role)), c.factory.address)
	}
	c.send(t, c.emitter, "setPoolRegistry", registry.address)
	c.send(t, c.emitter, "setSidioraFactory", c.factory.address)
	for _, producer := range []coinTextContract{config, treasury, registry, accumulator, nft} {
		c.send(t, c.emitter, "setAuthorizedEmitter", producer.address, true)
	}
	for _, pair := range []struct {
		producer coinTextContract
		method   string
	}{
		{config, "emitConfigUpdated"}, {registry, "emitPoolRegistered"},
		{accumulator, "emitFeeFlow"}, {accumulator, "emitFeeRecorded"}, {accumulator, "emitFeeDistributed"},
		{nft, "emitFeeStrategyChanged"}, {nft, "emitNftMint"},
	} {
		var selector [4]byte
		copy(selector[:], c.emitter.abi.Methods[pair.method].ID)
		c.send(t, c.emitter, "setTypedEmitter", pair.producer.address, selector, true)
	}
	c.send(t, config, "setCreationFee", big.NewInt(0))
	return c
}

func (c *coinTextChain) creation(t *testing.T, name, symbol string) creationFixture {
	t.Helper()
	receipt := c.send(t, c.factory, "createMarket", name, symbol, uint8(0), common.Address{})
	header, err := c.backend.Client().HeaderByNumber(context.Background(), receipt.BlockNumber)
	if err != nil {
		t.Fatal(err)
	}
	f := creationFixture{Height: receipt.BlockNumber.Uint64(), TxIndex: uint64(receipt.TransactionIndex), Timestamp: header.Time, BlockHash: receipt.BlockHash.Hex(), TxHash: receipt.TxHash.Hex()}
	parsed, err := abi.JSON(strings.NewReader(canonicalEventsABI))
	if err != nil {
		t.Fatal(err)
	}
	for _, log := range receipt.Logs {
		if len(log.Topics) == 0 {
			continue
		}
		var target *fixtureLog
		switch log.Topics[0] {
		case parsed.Events["MarketCreated"].ID:
			target = &f.Market
		case parsed.Events["TokenDeployed"].ID:
			target = &f.Deployed
		default:
			continue
		}
		topics := make([]string, len(log.Topics))
		for i, topic := range log.Topics {
			topics[i] = topic.Hex()
		}
		*target = fixtureLog{Address: strings.ToLower(log.Address.Hex()), Topics: topics, Data: "0x" + common.Bytes2Hex(log.Data), LogIndex: strconv.FormatUint(uint64(log.Index), 16)}
	}
	if f.Market.Address == "" || f.Deployed.Address == "" {
		t.Fatal("creation receipt lacks factory and token events")
	}
	f.Factory, f.Emitter = f.Market.Address, f.Deployed.Address
	f.Token = strings.ToLower(common.HexToAddress(f.Market.Topics[1]).Hex())
	f.Pool = strings.ToLower(common.HexToAddress(f.Market.Topics[2]).Hex())
	f.Creator = strings.ToLower(c.auth.From.Hex())
	market, err := parsed.Events["MarketCreated"].Inputs.NonIndexed().Unpack(common.FromHex(f.Market.Data))
	if err != nil {
		t.Fatal(err)
	}
	f.NFTID, f.Optical = market[0].(*big.Int).String(), strings.ToLower(market[1].(common.Address).Hex())
	deployed, err := parsed.Events["TokenDeployed"].Inputs.NonIndexed().Unpack(common.FromHex(f.Deployed.Data))
	if err != nil {
		t.Fatal(err)
	}
	f.Name, f.Symbol = chainCoinText(deployed[1].(string)), chainCoinText(deployed[2].(string))
	f.Decimals, f.Supply = deployed[3].(uint8), deployed[4].(*big.Int).String()
	return f
}
