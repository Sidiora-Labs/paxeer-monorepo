package app

import (
	"context"
	"encoding/json"
	"math/big"
	"net"
	"strconv"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/protocol/bindings"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/deployment"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/ethclient/simulated"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/params"
)

func authorityEVMSource(t *testing.T) (string, deployment.Manifest) {
	t.Helper()
	manifest, ok := deployment.Approved(32_226_215)
	if !ok {
		t.Fatal("approved Solidity deployment unavailable")
	}
	manifest.ActivationHeight = 1
	var artifacts struct {
		Contracts map[string]struct {
			Runtime        string                                   `json:"runtime"`
			Immutables     map[string][]struct{ Start, Length int } `json:"immutableReferences"`
			ImmutableNames map[string]string                        `json:"immutableNames"`
		} `json:"contracts"`
	}
	if err := json.Unmarshal(repoFile(t, "sdk/launchpad/shared/config/deployment/testdata/evm-artifacts.json"), &artifacts); err != nil {
		t.Fatal(err)
	}
	runtime := func(name string, address common.Address) []byte {
		t.Helper()
		artifact, ok := artifacts.Contracts[name]
		if !ok {
			t.Fatalf("contract artifact missing: %s", name)
		}
		code := common.FromHex(artifact.Runtime)
		if len(code) == 0 {
			t.Fatalf("contract runtime empty: %s", name)
		}
		for id, references := range artifact.Immutables {
			value := common.LeftPadBytes(address.Bytes(), 32)
			if name == "SidioraERC20" && artifact.ImmutableNames[id] == "decimals" {
				value = common.LeftPadBytes(big.NewInt(6).Bytes(), 32)
			}
			for _, reference := range references {
				if reference.Length != 32 || reference.Start < 0 || reference.Start+32 > len(code) {
					t.Fatal("contract immutable range invalid")
				}
				copy(code[reference.Start:reference.Start+32], value)
			}
		}
		return code
	}
	allocation := types.GenesisAlloc{}
	word := func(value int64) common.Hash { return common.BigToHash(big.NewInt(value)) }
	for name, role := range manifest.Roles {
		address := common.HexToAddress(role.Address)
		storage := map[common.Hash]common.Hash{}
		var code []byte
		switch role.Kind {
		case "erc1967":
			code = runtime("UUPSProxy", address)
			implementation := common.HexToAddress(role.ImplementationAddress)
			implementationCode := runtime(name, implementation)
			if crypto.Keccak256Hash(implementationCode).Hex() != role.ImplementationHash {
				t.Fatalf("%s implementation differs from approved artifact", name)
			}
			allocation[implementation] = types.Account{Code: implementationCode}
			storage[deployment.ImplementationSlot] = common.BytesToHash(implementation.Bytes())
		case "beacon":
			code = runtime("PoolBeacon", address)
			implementation := common.HexToAddress(role.ImplementationAddress)
			implementationCode := runtime("SidioraPool", implementation)
			if crypto.Keccak256Hash(implementationCode).Hex() != role.ImplementationHash {
				t.Fatal("pool implementation differs from approved artifact")
			}
			allocation[implementation] = types.Account{Code: implementationCode}
			storage[word(0)] = common.BytesToHash(implementation.Bytes())
			storage[word(1)] = common.BytesToHash(common.HexToAddress(manifest.Governance.ExpectedBeaconOwner).Bytes())
		case "direct":
			if name != "USDL" {
				t.Fatalf("unexpected direct contract: %s", name)
			}
			code = runtime("SidioraERC20", address)
			role.RuntimeHash = crypto.Keccak256Hash(code).Hex()
			manifest.Roles[name] = role
		default:
			t.Fatalf("Solidity deployment has unsupported contract kind: %s", role.Kind)
		}
		if crypto.Keccak256Hash(code).Hex() != role.RuntimeHash {
			t.Fatalf("%s runtime differs from approved artifact", name)
		}
		allocation[address] = types.Account{Code: code, Storage: storage}
	}
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	admin := crypto.PubkeyToAddress(key.PublicKey)
	allocation[admin] = types.Account{Balance: new(big.Int).Mul(big.NewInt(1000), big.NewInt(params.Ether))}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err = listener.Close(); err != nil {
		t.Fatal(err)
	}
	backend := simulated.NewBackend(allocation, func(n *node.Config, e *ethconfig.Config) {
		chain := *e.Genesis.Config
		chain.ChainID = big.NewInt(125)
		e.Genesis.Config = &chain
		n.HTTPHost, n.HTTPPort, n.HTTPModules = "127.0.0.1", port, []string{"eth"}
	})
	t.Cleanup(func() {
		if err := backend.Close(); err != nil {
			t.Error(err)
		}
	})
	protocol, err := bindings.NewProtocolConfig(common.HexToAddress(manifest.Roles["ProtocolConfig"].Address), backend.Client())
	if err != nil {
		t.Fatal(err)
	}
	opts, err := bind.NewKeyedTransactorWithChainID(key, big.NewInt(125))
	if err != nil {
		t.Fatal(err)
	}
	tx, err := protocol.Initialize(opts, common.HexToAddress(manifest.Roles["USDL"].Address), common.Address{}, admin)
	if err != nil {
		t.Fatal(err)
	}
	backend.Commit()
	receipt, err := backend.Client().TransactionReceipt(context.Background(), tx.Hash())
	if err != nil || receipt.Status != types.ReceiptStatusSuccessful {
		t.Fatalf("protocol initialization: %v %v", receipt, err)
	}
	return "http://127.0.0.1:" + strconv.Itoa(port), manifest
}
