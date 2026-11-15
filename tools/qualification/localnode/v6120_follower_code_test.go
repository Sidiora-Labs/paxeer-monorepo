package localnode

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/binary"
	"encoding/json"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	coretypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/holiman/uint256"
)

type followerCodeContract struct {
	ABI      json.RawMessage `json:"abi"`
	Creation hexutil.Bytes   `json:"creationCode"`
	Runtime  hexutil.Bytes   `json:"runtimeCode"`
}

type followerCodeContracts struct {
	Delegate followerCodeContract `json:"delegate"`
	Token    followerCodeContract `json:"token"`
}

type followerCodeCall struct {
	To    common.Address
	Value *big.Int
	Data  []byte
}

type followerCodeQuote struct {
	Sponsor        common.Address
	Token          common.Address
	MaxTokenAmount *big.Int
	TokenAmount    *big.Int
	Deadline       *big.Int
	QuoteNonce     *big.Int
	GasCost        *big.Int
}

var followerSidioraToken = common.HexToAddress("0x21f7b20a555199fa73A238B1a91FD0f549068fEe")

const (
	followerBridgeAuthority = "pax10d07y265gmmuvt4z0w9aw880jnsr700jxdwa9m"
	followerSidioraChain    = uint64(0x0000534f4c414e41)
)

func followerCodeRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not locate the module root")
		}
		dir = parent
	}
}

func followerSidioraNetwork(t *testing.T) *followerNetwork {
	t.Helper()
	original := newFollowerNetwork(t)
	ctx, cancel := context.WithTimeout(original.ctx, 15*time.Second)
	for _, client := range []*ethclient.Client{original.primary, original.follower} {
		code, err := client.CodeAt(ctx, followerSidioraToken, nil)
		if err != nil {
			cancel()
			t.Fatalf("read Sidiora code before setup: %v", err)
		}
		t.Logf("Sidiora runtime before setup: %d bytes", len(code))
	}
	cancel()
	dir, err := os.MkdirTemp("", "lx-follower-code-")
	if err != nil {
		t.Fatal(err)
	}
	log, err := os.OpenFile(filepath.Join(dir, "setup.log"), os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	script := filepath.Join(followerCodeRoot(t), "tools", "qualification", "localnode", "v6120-node.sh")
	envFile := filepath.Join(dir, "network.env")
	ctx, cancel = context.WithTimeout(original.ctx, 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "bash", script, "--sequencer", "--follower", "--sidiora", "--env-out", envFile)
	cmd.Stdout, cmd.Stderr = log, log
	if err := cmd.Run(); err != nil {
		t.Fatalf("start Sidiora network: %v; evidence %s", err, log.Name())
	}
	output, err := exec.Command("bash", "-c", `set -a; . "$1"; env -0`, "follower-code-env", envFile).Output()
	if err != nil {
		t.Fatalf("read Sidiora network environment: %v", err)
	}
	for _, entry := range bytes.Split(output, []byte{0}) {
		key, value, ok := strings.Cut(string(entry), "=")
		if ok && (strings.HasPrefix(key, "LOCAL_NODE_") || strings.HasPrefix(key, "PERPS_DATA_TEST_")) {
			t.Setenv(key, value)
		}
	}
	home := os.Getenv("LOCAL_NODE_HOME")
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		output, err := exec.CommandContext(ctx, "bash", script, "--stop", home).CombinedOutput()
		if err != nil {
			t.Errorf("stop Sidiora network: %v: %s", err, output)
		}
	})
	n := newFollowerNetwork(t)
	ctx, cancel = context.WithTimeout(n.ctx, 15*time.Second)
	defer cancel()
	height, err := n.primary.BlockNumber(ctx)
	if err != nil {
		t.Fatalf("read Sidiora network height: %v", err)
	}
	block := new(big.Int).SetUint64(height)
	deadline := time.Now().Add(60 * time.Second)
	for {
		followerHeight, err := n.follower.BlockNumber(ctx)
		if err != nil {
			t.Fatalf("read Sidiora follower height: %v", err)
		}
		if followerHeight >= height {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("Sidiora follower height %d, want %d", followerHeight, height)
		}
		time.Sleep(100 * time.Millisecond)
	}
	n.assertCodeNonceStorage(t, followerSidioraToken, block, common.Hash{})
	for _, client := range []*ethclient.Client{n.primary, n.follower} {
		code, err := client.CodeAt(ctx, followerSidioraToken, block)
		if err != nil || len(code) == 0 {
			t.Fatalf("Sidiora code at block %s: %x, %v", block, code, err)
		}
		data := crypto.Keccak256([]byte("denom()"))[:4]
		result, err := client.CallContract(ctx, ethereum.CallMsg{To: &followerSidioraToken, Data: data}, block)
		stringType, typeErr := abi.NewType("string", "", nil)
		if err != nil || typeErr != nil {
			t.Fatalf("read Sidiora denom: %v/%v", err, typeErr)
		}
		values, err := (abi.Arguments{{Type: stringType}}).Unpack(result)
		if err != nil || len(values) != 1 || values[0] != "factory/pax1dzfx9mk4fl9kl2mysjmtvk2xp75ljumk6nynhf/usid" {
			t.Fatalf("Sidiora denom=%v, error=%v", values, err)
		}
	}
	return n
}

func followerCodeFixture(t *testing.T) (followerCodeContracts, abi.ABI, abi.ABI) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(followerCodeRoot(t), "daemon", "committed-fixture", "v6_contracts.json"))
	if err != nil {
		t.Fatalf("read V6 contract fixture: %v", err)
	}
	var contracts followerCodeContracts
	if err := json.Unmarshal(data, &contracts); err != nil {
		t.Fatalf("decode V6 contract fixture: %v", err)
	}
	delegateABI, err := abi.JSON(bytes.NewReader(contracts.Delegate.ABI))
	if err != nil {
		t.Fatalf("decode delegate ABI: %v", err)
	}
	tokenABI, err := abi.JSON(bytes.NewReader(contracts.Token.ABI))
	if err != nil {
		t.Fatalf("decode token ABI: %v", err)
	}
	return contracts, delegateABI, tokenABI
}

func followerBridgeABI(t *testing.T) abi.ABI {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(followerCodeRoot(t), "precompiles", "layerxbridge", "abi.json"))
	if err != nil {
		t.Fatalf("read bridge ABI: %v", err)
	}
	bridgeABI, err := abi.JSON(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode bridge ABI: %v", err)
	}
	return bridgeABI
}

func followerSubmitBridgeProposal(t *testing.T, n *followerNetwork, sponsor common.Address) {
	t.Helper()
	signer := strings.TrimPrefix(strings.ToLower(sponsor.Hex()), "0x")
	proposal := map[string]interface{}{
		"@type":       "/paxprotocol.paxchain.layerxbridge.BridgeProposal",
		"title":       "Fund the local sponsored batch account",
		"description": "Registers the local Sidiora pair and its attestor for the sponsored batch comparison.",
		"messages": []interface{}{
			map[string]interface{}{
				"@type":     "/paxprotocol.paxchain.layerxbridge.MsgRegisterChain",
				"authority": followerBridgeAuthority,
				"chain": map[string]interface{}{
					"chain_id":       followerSidioraChain,
					"vault":          "1000000000000000000000000000000000000001",
					"finality_depth": 1,
					"enabled":        true,
				},
			},
			map[string]interface{}{
				"@type":     "/paxprotocol.paxchain.layerxbridge.MsgSetAttestors",
				"authority": followerBridgeAuthority,
				"set": map[string]interface{}{
					"attestors": []interface{}{map[string]interface{}{"signer": signer, "bond": "0"}},
					"threshold": 1,
				},
			},
			map[string]interface{}{
				"@type":     "/paxprotocol.paxchain.layerxbridge.MsgRegisterSidioraPair",
				"authority": followerBridgeAuthority,
				"chain_id":  followerSidioraChain,
			},
			map[string]interface{}{
				"@type":         "/paxprotocol.paxchain.layerxbridge.MsgSetCap",
				"authority":     followerBridgeAuthority,
				"chain_id":      followerSidioraChain,
				"asset":         strings.TrimPrefix(strings.ToLower(followerSidioraToken.Hex()), "0x"),
				"max_in_flight": "1000000000000",
				"max_per_tx":    "1000000000000",
			},
		},
	}
	body, err := json.Marshal(proposal)
	if err != nil {
		t.Fatalf("encode bridge proposal: %v", err)
	}
	path := filepath.Join(t.TempDir(), "bridge-proposal.json")
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatalf("write bridge proposal: %v", err)
	}
	output := n.command(t, "tx", "gov", "submit-proposal", "layerxbridge-proposal", path,
		"--from", "validator", "--home", n.home, "--keyring-backend", "test",
		"--chain-id", "hyperpax_125-1", "--node", n.comet, "--gas", "2000000",
		"--fees", "1000000uhpx", "--deposit", "10000000uhpx", "-b", "sync", "-o", "json", "-y")
	var submitted struct {
		Code uint32 `json:"code"`
		Hash string `json:"txhash"`
	}
	if err := json.Unmarshal(output, &submitted); err != nil || submitted.Code != 0 || submitted.Hash == "" {
		t.Fatalf("submit bridge proposal: %s", output)
	}
	proposalID := ""
	var proposalOutput []byte
	proposalDeadline := time.Now().Add(30 * time.Second)
	for proposalID == "" && time.Now().Before(proposalDeadline) {
		proposalOutput = n.command(t, "q", "gov", "proposals", "--node", n.comet, "-o", "json")
		var proposals struct {
			Proposals []struct {
				ID      string `json:"proposal_id"`
				Content struct {
					Title string `json:"title"`
				} `json:"content"`
			} `json:"proposals"`
		}
		if json.Unmarshal(proposalOutput, &proposals) == nil {
			for _, item := range proposals.Proposals {
				if item.Content.Title == "Fund the local sponsored batch account" {
					proposalID = item.ID
				}
			}
		}
		if proposalID == "" {
			time.Sleep(200 * time.Millisecond)
		}
	}
	if proposalID == "" {
		t.Fatalf("bridge proposal was not listed: %s", proposalOutput)
	}
	n.command(t, "tx", "gov", "vote", proposalID, "yes", "--from", "validator", "--home", n.home,
		"--keyring-backend", "test", "--chain-id", "hyperpax_125-1", "--node", n.comet,
		"--gas", "200000", "--fees", "1000000uhpx", "-b", "sync", "-o", "json", "-y")
	deadline := time.Now().Add(45 * time.Second)
	for time.Now().Before(deadline) {
		query := n.command(t, "q", "gov", "proposal", proposalID, "--node", n.comet, "-o", "json")
		var state struct {
			Status string `json:"status"`
		}
		if json.Unmarshal(query, &state) == nil && state.Status == "PROPOSAL_STATUS_PASSED" {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("bridge proposal did not pass")
}

func followerFundSidiora(t *testing.T, n *followerNetwork, sponsor, user *ecdsa.PrivateKey) {
	t.Helper()
	followerSubmitBridgeProposal(t, n, crypto.PubkeyToAddress(sponsor.PublicKey))
	bridgeABI := followerBridgeABI(t)
	var txHash [32]byte
	copy(txHash[:], crypto.Keccak256(crypto.PubkeyToAddress(user.PublicKey).Bytes(), []byte("sponsored")))
	var recipient [32]byte
	copy(recipient[12:], crypto.PubkeyToAddress(user.PublicKey).Bytes())
	digestInput := make([]byte, 0, 196)
	word := func(value *big.Int) []byte { return common.BigToHash(value).Bytes() }
	digestInput = append(digestInput, []byte("PAXEERX_BRIDGE_IN_V1")...)
	digestInput = append(digestInput, word(new(big.Int).SetUint64(followerSidioraChain))...)
	digestInput = append(digestInput, common.HexToAddress("0x1000000000000000000000000000000000000001").Bytes()...)
	digestInput = append(digestInput, txHash[:]...)
	var logIndex [8]byte
	binary.BigEndian.PutUint64(logIndex[:], 1)
	digestInput = append(digestInput, logIndex[:]...)
	digestInput = append(digestInput, recipient[:]...)
	digestInput = append(digestInput, followerSidioraToken.Bytes()...)
	digestInput = append(digestInput, word(big.NewInt(100_000_000))...)
	signature, err := crypto.Sign(crypto.Keccak256(digestInput), sponsor)
	if err != nil {
		t.Fatalf("sign bridge attestation: %v", err)
	}
	signature[64] += 27
	input, err := bridgeABI.Pack("bridgeIn", followerSidioraChain,
		common.HexToAddress("0x1000000000000000000000000000000000000001"), txHash, uint64(1), recipient,
		followerSidioraToken, big.NewInt(100_000_000), [][]byte{signature})
	if err != nil {
		t.Fatalf("pack bridge funding: %v", err)
	}
	tx := n.sign(t, sponsor, common.HexToAddress("0x0000000000000000000000000000000000001016"), new(big.Int), input)
	receipt := n.compareCodeReceipt(t, tx, types.ReceiptStatusSuccessful, true)
	_, _, tokenABI := followerCodeFixture(t)
	n.assertCodeNonceStorage(t, followerSidioraToken, receipt.BlockNumber, common.Hash{})
	ctx, cancel := context.WithTimeout(n.ctx, 15*time.Second)
	defer cancel()
	for _, client := range []*ethclient.Client{n.primary, n.follower} {
		balance, err := client.CallContract(ctx, ethereum.CallMsg{To: &followerSidioraToken, Data: mustPackBalanceOf(t, tokenABI, crypto.PubkeyToAddress(user.PublicKey))}, receipt.BlockNumber)
		if err != nil || len(balance) != 32 || new(big.Int).SetBytes(balance).Cmp(big.NewInt(100_000_000)) != 0 {
			t.Fatalf("funded Sidiora balance at block %s=%x, error=%v", receipt.BlockNumber, balance, err)
		}
	}
}

func (n *followerNetwork) signCreate(t *testing.T, key *ecdsa.PrivateKey, data []byte) *types.Transaction {
	t.Helper()
	ctx, cancel := context.WithTimeout(n.ctx, 15*time.Second)
	defer cancel()
	address := crypto.PubkeyToAddress(key.PublicKey)
	nonce, err := n.primary.NonceAt(ctx, address, nil)
	if err != nil {
		t.Fatalf("read creation nonce: %v", err)
	}
	price, err := n.primary.SuggestGasPrice(ctx)
	if err != nil {
		t.Fatalf("read creation gas price: %v", err)
	}
	tx, err := types.SignTx(types.NewTx(&types.LegacyTx{Nonce: nonce, GasPrice: price, Gas: 7_000_000, Data: data}), types.LatestSignerForChainID(big.NewInt(125)), key)
	if err != nil {
		t.Fatalf("sign creation: %v", err)
	}
	return tx
}

func (n *followerNetwork) signSetCode(t *testing.T, sender *ecdsa.PrivateKey, to common.Address, data []byte, auth []types.SetCodeAuthorization) *types.Transaction {
	t.Helper()
	ctx, cancel := context.WithTimeout(n.ctx, 15*time.Second)
	defer cancel()
	nonce, err := n.primary.NonceAt(ctx, crypto.PubkeyToAddress(sender.PublicKey), nil)
	if err != nil {
		t.Fatalf("read set-code nonce: %v", err)
	}
	price, err := n.primary.SuggestGasPrice(ctx)
	if err != nil {
		t.Fatalf("read set-code gas price: %v", err)
	}
	tx, err := types.SignNewTx(sender, types.LatestSignerForChainID(big.NewInt(125)), &types.SetCodeTx{
		ChainID: uint256.NewInt(125), Nonce: nonce, GasTipCap: new(uint256.Int), GasFeeCap: uint256.MustFromBig(price), Gas: 7_000_000,
		To: to, Value: new(uint256.Int), Data: data, AuthList: auth,
	})
	if err != nil {
		t.Fatalf("sign set-code transaction: %v", err)
	}
	return tx
}

func followerCodeAuthorization(t *testing.T, key *ecdsa.PrivateKey, address common.Address, nonce uint64) types.SetCodeAuthorization {
	t.Helper()
	authorization, err := types.SignSetCode(key, types.SetCodeAuthorization{ChainID: *uint256.NewInt(125), Address: address, Nonce: nonce})
	if err != nil {
		t.Fatalf("sign set-code authorization: %v", err)
	}
	return authorization
}

func (n *followerNetwork) compareCodeReceipt(t *testing.T, tx *types.Transaction, status uint64, submit bool) *types.Receipt {
	t.Helper()
	t.Cleanup(func() {
		if !t.Failed() {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		receipt, err := n.primary.TransactionReceipt(ctx, tx.Hash())
		t.Logf("transaction %s receipt=%+v error=%v call to=%v data=%x", tx.Hash(), receipt, err, tx.To(), tx.Data())
		if receipt != nil && receipt.Status == types.ReceiptStatusFailed && status == types.ReceiptStatusSuccessful {
			var trace json.RawMessage
			err := n.primary.Client().CallContext(ctx, &trace, "debug_traceTransaction", tx.Hash(), map[string]interface{}{"tracer": "callTracer"})
			t.Logf("transaction %s trace=%s error=%v", tx.Hash(), trace, err)
		}
	})
	primary := n.compare(t, tx, status, submit)
	ctx, cancel := context.WithTimeout(n.ctx, 15*time.Second)
	defer cancel()
	follower, err := n.follower.TransactionReceipt(ctx, tx.Hash())
	if err != nil {
		t.Fatalf("read follower receipt %s: %v", tx.Hash(), err)
	}
	if primary.TransactionIndex != follower.TransactionIndex {
		t.Fatalf("receipt %s at block %s transaction index differs: primary=%d follower=%d", tx.Hash(), primary.BlockNumber, primary.TransactionIndex, follower.TransactionIndex)
	}
	if primary.ContractAddress != follower.ContractAddress {
		t.Fatalf("receipt %s contract address differs: primary=%s follower=%s", tx.Hash(), primary.ContractAddress, follower.ContractAddress)
	}
	if primary.GasUsed != follower.GasUsed {
		t.Fatalf("receipt %s gas used differs: primary=%d follower=%d", tx.Hash(), primary.GasUsed, follower.GasUsed)
	}
	if !reflect.DeepEqual(primary.Logs, follower.Logs) {
		t.Fatalf("receipt %s logs differ: primary=%v follower=%v", tx.Hash(), primary.Logs, follower.Logs)
	}
	if primary.Bloom != follower.Bloom {
		t.Fatalf("receipt %s bloom differs: primary=%x follower=%x", tx.Hash(), primary.Bloom, follower.Bloom)
	}
	return primary
}

func (n *followerNetwork) assertCodeNonceStorage(t *testing.T, address common.Address, block *big.Int, slot common.Hash) {
	t.Helper()
	ctx, cancel := context.WithTimeout(n.ctx, 15*time.Second)
	defer cancel()
	primaryCode, err := n.primary.CodeAt(ctx, address, block)
	if err != nil {
		t.Fatalf("read primary code %s at %s: %v", address, block, err)
	}
	followerCode, err := n.follower.CodeAt(ctx, address, block)
	if err != nil {
		t.Fatalf("read follower code %s at %s: %v", address, block, err)
	}
	if !bytes.Equal(primaryCode, followerCode) {
		t.Fatalf("code %s at block %s differs: primary=%x follower=%x", address, block, primaryCode, followerCode)
	}
	primaryNonce, err := n.primary.NonceAt(ctx, address, block)
	if err != nil {
		t.Fatalf("read primary nonce %s at %s: %v", address, block, err)
	}
	followerNonce, err := n.follower.NonceAt(ctx, address, block)
	if err != nil {
		t.Fatalf("read follower nonce %s at %s: %v", address, block, err)
	}
	if primaryNonce != followerNonce {
		t.Fatalf("nonce %s at block %s differs: primary=%d follower=%d", address, block, primaryNonce, followerNonce)
	}
	primaryStorage, err := n.primary.StorageAt(ctx, address, slot, block)
	if err != nil {
		t.Fatalf("read primary storage %s at %s: %v", address, block, err)
	}
	followerStorage, err := n.follower.StorageAt(ctx, address, slot, block)
	if err != nil {
		t.Fatalf("read follower storage %s at %s: %v", address, block, err)
	}
	if !bytes.Equal(primaryStorage, followerStorage) {
		t.Fatalf("storage %s slot %s at block %s differs: primary=%x follower=%x", address, slot, block, primaryStorage, followerStorage)
	}
}

func followerTokenCreation(t *testing.T, contracts followerCodeContracts, tokenABI abi.ABI) []byte {
	t.Helper()
	constructor, err := tokenABI.Pack("", "LayerX Test Token", "LXT")
	if err != nil {
		t.Fatalf("pack token constructor: %v", err)
	}
	return append(append([]byte(nil), contracts.Token.Creation...), constructor...)
}

func followerDelegateCreation(t *testing.T, contracts followerCodeContracts, delegateABI abi.ABI, owner common.Address) []byte {
	t.Helper()
	constructor, err := delegateABI.Pack("", owner, big.NewInt(3_114_000), big.NewInt(3600))
	if err != nil {
		t.Fatalf("pack delegate constructor: %v", err)
	}
	return append(append([]byte(nil), contracts.Delegate.Creation...), constructor...)
}

func followerTokenTransfer(t *testing.T, tokenABI abi.ABI, to common.Address, amount *big.Int) []byte {
	t.Helper()
	data, err := tokenABI.Pack("transfer", to, amount)
	if err != nil {
		t.Fatalf("pack token transfer: %v", err)
	}
	return data
}

func followerABICalls(t *testing.T, delegateABI abi.ABI, calls []followerCodeCall) interface{} {
	t.Helper()
	callType := delegateABI.Methods["executeSponsored"].Inputs[0].Type
	encoded := reflect.MakeSlice(callType.GetType(), len(calls), len(calls))
	for i, call := range calls {
		item := encoded.Index(i)
		item.FieldByName("To").Set(reflect.ValueOf(call.To))
		value := call.Value
		if value == nil {
			value = new(big.Int)
		}
		item.FieldByName("Value").Set(reflect.ValueOf(value))
		item.FieldByName("Data").Set(reflect.ValueOf(call.Data))
	}
	return encoded.Interface()
}

func followerQuoteDigest(account common.Address, quote followerCodeQuote) []byte {
	word := func(value *big.Int) []byte { return common.BigToHash(value).Bytes() }
	addressWord := func(value common.Address) []byte { return common.BytesToHash(value.Bytes()).Bytes() }
	return accounts.TextHash(crypto.Keccak256(crypto.Keccak256([]byte("Quote(uint256 chainId,address account,address sponsor,address token,uint256 maxTokenAmount,uint256 tokenAmount,uint256 deadline,uint256 quoteNonce,uint256 gasCost)")), word(big.NewInt(125)), addressWord(account), addressWord(quote.Sponsor), addressWord(quote.Token), word(quote.MaxTokenAmount), word(quote.TokenAmount), word(quote.Deadline), word(quote.QuoteNonce), word(quote.GasCost)))
}

func followerSponsoredInput(t *testing.T, delegateABI abi.ABI, user *ecdsa.PrivateKey, sponsor *ecdsa.PrivateKey, calls []followerCodeCall, quote followerCodeQuote) []byte {
	return followerSponsoredInputAtNonce(t, delegateABI, user, sponsor, calls, quote, new(big.Int))
}

func followerSponsoredInputAtNonce(t *testing.T, delegateABI abi.ABI, user *ecdsa.PrivateKey, sponsor *ecdsa.PrivateKey, calls []followerCodeCall, quote followerCodeQuote, batchNonce *big.Int) []byte {
	t.Helper()
	callType := delegateABI.Methods["executeSponsored"].Inputs[0].Type
	typedCalls := followerABICalls(t, delegateABI, calls)
	encodedCalls, err := abi.Arguments{{Type: callType}}.Pack(typedCalls)
	if err != nil {
		t.Fatalf("pack sponsored calls: %v", err)
	}
	quoteDigest := followerQuoteDigest(crypto.PubkeyToAddress(user.PublicKey), quote)
	batchDigest := accounts.TextHash(crypto.Keccak256(crypto.Keccak256([]byte("SponsoredBatch(uint256 nonce,bytes32 callsHash,bytes32 quoteDigest)")), common.BigToHash(batchNonce).Bytes(), crypto.Keccak256(encodedCalls), quoteDigest))
	accountSignature, err := crypto.Sign(batchDigest, user)
	if err != nil {
		t.Fatalf("sign sponsored account digest: %v", err)
	}
	accountSignature[64] += 27
	relayerSignature, err := crypto.Sign(quoteDigest, sponsor)
	if err != nil {
		t.Fatalf("sign sponsored quote digest: %v", err)
	}
	relayerSignature[64] += 27
	input, err := delegateABI.Pack("executeSponsored", typedCalls, quote, accountSignature, relayerSignature)
	if err != nil {
		t.Fatalf("pack sponsored input: %v", err)
	}
	return input
}

func followerFactoryCreation() []byte {
	child := []byte{0x60, 0x01, 0x60, 0x0c, 0x60, 0x00, 0x39, 0x60, 0x01, 0x60, 0x00, 0xf3, 0x00}
	runtime := []byte{0x60, byte(len(child)), 0x60, 0x0e, 0x60, 0x00, 0x39, 0x60, byte(len(child)), 0x60, 0x00, 0x60, 0x00, 0xf0}
	runtime = append(runtime, child...)
	init := []byte{0x60, byte(len(runtime)), 0x60, 0x0c, 0x60, 0x00, 0x39, 0x60, byte(len(runtime)), 0x60, 0x00, 0xf3}
	return append(init, runtime...)
}

func TestLocalNodeV6120FollowerCodeCreation(t *testing.T) {
	n := newFollowerNetwork(t)
	contracts, _, tokenABI := followerCodeFixture(t)
	key := n.freshKey(t)
	creation := followerTokenCreation(t, contracts, tokenABI)
	address := crypto.CreateAddress(crypto.PubkeyToAddress(key.PublicKey), 0)
	receipt := n.compareCodeReceipt(t, n.signCreate(t, key, creation), types.ReceiptStatusSuccessful, true)
	if receipt.ContractAddress != address {
		t.Fatalf("created token address=%s, want nonce-derived %s", receipt.ContractAddress, address)
	}
	n.assertCodeNonceStorage(t, address, receipt.BlockNumber, common.Hash{})
	ctx, cancel := context.WithTimeout(n.ctx, 15*time.Second)
	defer cancel()
	code, err := n.primary.CodeAt(ctx, address, receipt.BlockNumber)
	if err != nil || !bytes.Equal(code, contracts.Token.Runtime) {
		t.Fatalf("created token runtime code differs: len=%d error=%v", len(code), err)
	}
	mint, err := tokenABI.Pack("mint", crypto.PubkeyToAddress(key.PublicKey), big.NewInt(1))
	if err != nil {
		t.Fatalf("pack token mint: %v", err)
	}
	n.compareCodeReceipt(t, n.sign(t, key, address, new(big.Int), mint), types.ReceiptStatusSuccessful, true)
	n.compareCodeReceipt(t, n.sign(t, key, address, new(big.Int), followerTokenTransfer(t, tokenABI, crypto.PubkeyToAddress(key.PublicKey), big.NewInt(1))), types.ReceiptStatusSuccessful, true)
}

func TestLocalNodeV6120FollowerCodeCreationAfterTransaction(t *testing.T) {
	n := newFollowerNetwork(t)
	contracts, _, tokenABI := followerCodeFixture(t)
	key := n.freshKey(t)
	n.compareCodeReceipt(t, n.sign(t, key, common.HexToAddress("0x000000000000000000000000000000000000beef"), big.NewInt(1), nil), types.ReceiptStatusSuccessful, true)
	creation := followerTokenCreation(t, contracts, tokenABI)
	address := crypto.CreateAddress(crypto.PubkeyToAddress(key.PublicKey), 1)
	receipt := n.compareCodeReceipt(t, n.signCreate(t, key, creation), types.ReceiptStatusSuccessful, true)
	if receipt.ContractAddress != address {
		t.Fatalf("created token address=%s, want nonce-derived %s", receipt.ContractAddress, address)
	}
	n.assertCodeNonceStorage(t, address, receipt.BlockNumber, common.Hash{})
	ctx, cancel := context.WithTimeout(n.ctx, 15*time.Second)
	defer cancel()
	for _, client := range []*ethclient.Client{n.primary, n.follower} {
		code, err := client.CodeAt(ctx, address, receipt.BlockNumber)
		if err != nil || !bytes.Equal(code, contracts.Token.Runtime) {
			t.Fatalf("created token runtime=%x, error=%v", code, err)
		}
	}
}

func TestLocalNodeV6120FollowerCodeFailedCreation(t *testing.T) {
	n := newFollowerNetwork(t)
	key := n.freshKey(t)
	address := crypto.CreateAddress(crypto.PubkeyToAddress(key.PublicKey), 0)
	receipt := n.compareCodeReceipt(t, n.signCreate(t, key, []byte{0x60, 0x00, 0x60, 0x00, 0xfd}), types.ReceiptStatusFailed, true)
	if receipt.ContractAddress != address {
		t.Fatalf("failed creation address=%s, want nonce-derived %s", receipt.ContractAddress, address)
	}
	n.assertCodeNonceStorage(t, address, receipt.BlockNumber, common.Hash{})
	n.assertCodeNonceStorage(t, crypto.PubkeyToAddress(key.PublicKey), receipt.BlockNumber, common.Hash{})
	ctx, cancel := context.WithTimeout(n.ctx, 15*time.Second)
	defer cancel()
	for _, client := range []*ethclient.Client{n.primary, n.follower} {
		code, err := client.CodeAt(ctx, address, receipt.BlockNumber)
		if err != nil || len(code) != 0 {
			t.Fatalf("failed creation left code on %s: %x, %v", address, code, err)
		}
	}
}

func TestLocalNodeV6120FollowerCodeContractCreation(t *testing.T) {
	n := newFollowerNetwork(t)
	key := n.freshKey(t)
	factory := crypto.CreateAddress(crypto.PubkeyToAddress(key.PublicKey), 0)
	created := n.compareCodeReceipt(t, n.signCreate(t, key, followerFactoryCreation()), types.ReceiptStatusSuccessful, true)
	if created.ContractAddress != factory {
		t.Fatalf("factory address=%s, want %s", created.ContractAddress, factory)
	}
	child := crypto.CreateAddress(factory, 1)
	receipt := n.compareCodeReceipt(t, n.sign(t, key, factory, new(big.Int), nil), types.ReceiptStatusSuccessful, true)
	n.assertCodeNonceStorage(t, factory, receipt.BlockNumber, common.Hash{})
	n.assertCodeNonceStorage(t, child, receipt.BlockNumber, common.Hash{})
	ctx, cancel := context.WithTimeout(n.ctx, 15*time.Second)
	defer cancel()
	for _, client := range []*ethclient.Client{n.primary, n.follower} {
		factoryCode, err := client.CodeAt(ctx, factory, receipt.BlockNumber)
		if err != nil || !bytes.Equal(factoryCode, followerFactoryCreation()[12:]) {
			t.Fatalf("factory runtime=%x, error=%v", factoryCode, err)
		}
		code, err := client.CodeAt(ctx, child, receipt.BlockNumber)
		if err != nil || !bytes.Equal(code, []byte{0}) {
			t.Fatalf("child contract has no code: %x, %v", code, err)
		}
	}
}

func TestLocalNodeV6120FollowerCodeSponsoredBatch(t *testing.T) {
	n := followerSidioraNetwork(t)
	contracts, delegateABI, tokenABI := followerCodeFixture(t)
	sponsor := n.freshKey(t)
	user := n.freshKey(t)
	tokenReceipt := n.compareCodeReceipt(t, n.signCreate(t, sponsor, followerTokenCreation(t, contracts, tokenABI)), types.ReceiptStatusSuccessful, true)
	n.assertCodeNonceStorage(t, tokenReceipt.ContractAddress, tokenReceipt.BlockNumber, common.Hash{})
	token := followerSidioraToken
	delegateReceipt := n.compareCodeReceipt(t, n.signCreate(t, sponsor, followerDelegateCreation(t, contracts, delegateABI, crypto.PubkeyToAddress(sponsor.PublicKey))), types.ReceiptStatusSuccessful, true)
	delegate := delegateReceipt.ContractAddress
	followerFundSidiora(t, n, sponsor, user)
	amount := big.NewInt(3_114_000)
	quote := followerCodeQuote{Sponsor: crypto.PubkeyToAddress(sponsor.PublicKey), Token: token, MaxTokenAmount: amount, TokenAmount: amount, Deadline: big.NewInt(time.Now().Add(10 * time.Minute).Unix()), QuoteNonce: big.NewInt(7), GasCost: big.NewInt(1_000_000_000_000_000_000)}
	calls := []followerCodeCall{{To: token, Value: new(big.Int), Data: followerTokenTransfer(t, tokenABI, crypto.PubkeyToAddress(sponsor.PublicKey), big.NewInt(1))}}
	input := followerSponsoredInput(t, delegateABI, user, sponsor, calls, quote)
	ctx, cancel := context.WithTimeout(n.ctx, 15*time.Second)
	userNonce, err := n.primary.NonceAt(ctx, crypto.PubkeyToAddress(user.PublicKey), nil)
	cancel()
	if err != nil {
		t.Fatalf("read user authorization nonce: %v", err)
	}
	authorization := followerCodeAuthorization(t, user, delegate, userNonce)
	batch := n.signSetCode(t, sponsor, crypto.PubkeyToAddress(user.PublicKey), input, []types.SetCodeAuthorization{authorization})
	receipt := n.compareCodeReceipt(t, batch, types.ReceiptStatusSuccessful, true)
	n.assertUsedQuote(t, crypto.PubkeyToAddress(user.PublicKey), quote, receipt.BlockNumber, 1)
	n.assertCodeNonceStorage(t, crypto.PubkeyToAddress(user.PublicKey), receipt.BlockNumber, common.Hash{})
	n.assertCodeNonceStorage(t, crypto.PubkeyToAddress(sponsor.PublicKey), receipt.BlockNumber, common.Hash{})
	ctx, cancel = context.WithTimeout(n.ctx, 15*time.Second)
	for _, client := range []*ethclient.Client{n.primary, n.follower} {
		code, err := client.CodeAt(ctx, crypto.PubkeyToAddress(user.PublicKey), receipt.BlockNumber)
		if err != nil || !bytes.Equal(code, types.AddressToDelegation(delegate)) {
			t.Fatalf("user delegation differs: %x, %v", code, err)
		}
	}
	cancel()
	for _, address := range []common.Address{crypto.PubkeyToAddress(user.PublicKey), crypto.PubkeyToAddress(sponsor.PublicKey)} {
		ctx, cancel = context.WithTimeout(n.ctx, 15*time.Second)
		primaryBalance, err := n.primary.BalanceAt(ctx, address, receipt.BlockNumber)
		followerBalance, followerErr := n.follower.BalanceAt(ctx, address, receipt.BlockNumber)
		cancel()
		if err != nil || followerErr != nil || primaryBalance.Cmp(followerBalance) != 0 {
			t.Fatalf("native balance %s differs after sponsored batch: %v/%v", address, primaryBalance, followerBalance)
		}
	}
	for _, address := range []common.Address{crypto.PubkeyToAddress(user.PublicKey), crypto.PubkeyToAddress(sponsor.PublicKey)} {
		ctx, cancel = context.WithTimeout(n.ctx, 15*time.Second)
		primaryBalance, err := n.primary.CallContract(ctx, ethereum.CallMsg{To: &token, Data: mustPackBalanceOf(t, tokenABI, address)}, receipt.BlockNumber)
		followerBalance, followerErr := n.follower.CallContract(ctx, ethereum.CallMsg{To: &token, Data: mustPackBalanceOf(t, tokenABI, address)}, receipt.BlockNumber)
		cancel()
		if err != nil || followerErr != nil || len(primaryBalance) != 32 || len(followerBalance) != 32 || !bytes.Equal(primaryBalance, followerBalance) {
			t.Fatalf("token balance %s differs after sponsored batch: %x/%x", address, primaryBalance, followerBalance)
		}
	}
}

func mustPackBalanceOf(t *testing.T, tokenABI abi.ABI, address common.Address) []byte {
	t.Helper()
	data, err := tokenABI.Pack("balanceOf", address)
	if err != nil {
		t.Fatalf("pack balanceOf: %v", err)
	}
	return data
}

func TestLocalNodeV6120FollowerCodeSponsoredBatchLaterAndAuthorization(t *testing.T) {
	n := followerSidioraNetwork(t)
	contracts, delegateABI, tokenABI := followerCodeFixture(t)
	sponsor, user := n.freshKey(t), n.freshKey(t)
	tokenReceipt := n.compareCodeReceipt(t, n.signCreate(t, sponsor, followerTokenCreation(t, contracts, tokenABI)), types.ReceiptStatusSuccessful, true)
	n.assertCodeNonceStorage(t, tokenReceipt.ContractAddress, tokenReceipt.BlockNumber, common.Hash{})
	token := followerSidioraToken
	delegate := n.compareCodeReceipt(t, n.signCreate(t, sponsor, followerDelegateCreation(t, contracts, delegateABI, crypto.PubkeyToAddress(sponsor.PublicKey))), types.ReceiptStatusSuccessful, true).ContractAddress
	followerFundSidiora(t, n, sponsor, user)
	amount := big.NewInt(3_114_000)
	firstQuote := followerCodeQuote{Sponsor: crypto.PubkeyToAddress(sponsor.PublicKey), Token: token, MaxTokenAmount: amount, TokenAmount: amount, Deadline: big.NewInt(time.Now().Add(10 * time.Minute).Unix()), QuoteNonce: big.NewInt(7), GasCost: big.NewInt(1_000_000_000_000_000_000)}
	firstInput := followerSponsoredInput(t, delegateABI, user, sponsor, []followerCodeCall{{To: token, Data: followerTokenTransfer(t, tokenABI, crypto.PubkeyToAddress(sponsor.PublicKey), big.NewInt(1))}}, firstQuote)
	ctx, cancel := context.WithTimeout(n.ctx, 15*time.Second)
	userNonce, err := n.primary.NonceAt(ctx, crypto.PubkeyToAddress(user.PublicKey), nil)
	cancel()
	if err != nil {
		t.Fatalf("read initial user nonce: %v", err)
	}
	first := n.signSetCode(t, sponsor, crypto.PubkeyToAddress(user.PublicKey), firstInput, []types.SetCodeAuthorization{followerCodeAuthorization(t, user, delegate, userNonce)})
	firstReceipt := n.compareCodeReceipt(t, first, types.ReceiptStatusSuccessful, true)
	n.assertUsedQuote(t, crypto.PubkeyToAddress(user.PublicKey), firstQuote, firstReceipt.BlockNumber, 1)
	secondQuote := firstQuote
	secondQuote.QuoteNonce = big.NewInt(8)
	secondInput := followerSponsoredInputAtNonce(t, delegateABI, user, sponsor, nil, secondQuote, big.NewInt(1))
	second := n.sign(t, sponsor, crypto.PubkeyToAddress(user.PublicKey), new(big.Int), secondInput)
	secondReceipt := n.compareCodeReceipt(t, second, types.ReceiptStatusSuccessful, true)
	n.assertUsedQuote(t, crypto.PubkeyToAddress(user.PublicKey), secondQuote, secondReceipt.BlockNumber, 2)
	repeat := n.sign(t, sponsor, crypto.PubkeyToAddress(user.PublicKey), new(big.Int), secondInput)
	repeatReceipt := n.compareCodeReceipt(t, repeat, types.ReceiptStatusFailed, true)
	n.assertUsedQuote(t, crypto.PubkeyToAddress(user.PublicKey), secondQuote, repeatReceipt.BlockNumber, 2)
	unauthorized := n.freshKey(t)
	badAuth := followerCodeAuthorization(t, unauthorized, delegate, 1)
	badAuthTx := n.signSetCode(t, sponsor, crypto.PubkeyToAddress(unauthorized.PublicKey), nil, []types.SetCodeAuthorization{badAuth})
	badReceipt := n.compareCodeReceipt(t, badAuthTx, types.ReceiptStatusSuccessful, true)
	n.assertCodeNonceStorage(t, crypto.PubkeyToAddress(unauthorized.PublicKey), badReceipt.BlockNumber, common.Hash{})
	ctx, cancel = context.WithTimeout(n.ctx, 15*time.Second)
	code, err := n.primary.CodeAt(ctx, crypto.PubkeyToAddress(unauthorized.PublicKey), badReceipt.BlockNumber)
	followerCode, followerErr := n.follower.CodeAt(ctx, crypto.PubkeyToAddress(unauthorized.PublicKey), badReceipt.BlockNumber)
	cancel()
	if err != nil || followerErr != nil || len(code) != 0 || len(followerCode) != 0 {
		t.Fatalf("non-applying authorization installed code: primary=%x follower=%x errors=%v/%v", code, followerCode, err, followerErr)
	}
	ctx, cancel = context.WithTimeout(n.ctx, 15*time.Second)
	userNonce, err = n.primary.NonceAt(ctx, crypto.PubkeyToAddress(user.PublicKey), nil)
	cancel()
	if err != nil {
		t.Fatalf("read delegation removal nonce: %v", err)
	}
	remove := n.signSetCode(t, sponsor, crypto.PubkeyToAddress(user.PublicKey), nil, []types.SetCodeAuthorization{followerCodeAuthorization(t, user, common.Address{}, userNonce)})
	removeReceipt := n.compareCodeReceipt(t, remove, types.ReceiptStatusSuccessful, true)
	n.assertCodeNonceStorage(t, crypto.PubkeyToAddress(user.PublicKey), removeReceipt.BlockNumber, common.Hash{})
	ctx, cancel = context.WithTimeout(n.ctx, 15*time.Second)
	for _, client := range []*ethclient.Client{n.primary, n.follower} {
		code, err := client.CodeAt(ctx, crypto.PubkeyToAddress(user.PublicKey), removeReceipt.BlockNumber)
		if err != nil || len(code) != 0 {
			t.Fatalf("delegation removal left code: %x, %v", code, err)
		}
	}
	cancel()
}

func TestLocalNodeV6120FollowerCodeMixedBlock(t *testing.T) {
	n := followerSidioraNetwork(t)
	contracts, delegateABI, tokenABI := followerCodeFixture(t)
	sponsor, user := n.freshKey(t), n.freshKey(t)
	tokenReceipt := n.compareCodeReceipt(t, n.signCreate(t, sponsor, followerTokenCreation(t, contracts, tokenABI)), types.ReceiptStatusSuccessful, true)
	n.assertCodeNonceStorage(t, tokenReceipt.ContractAddress, tokenReceipt.BlockNumber, common.Hash{})
	token := followerSidioraToken
	delegate := n.compareCodeReceipt(t, n.signCreate(t, sponsor, followerDelegateCreation(t, contracts, delegateABI, crypto.PubkeyToAddress(sponsor.PublicKey))), types.ReceiptStatusSuccessful, true).ContractAddress
	followerFundSidiora(t, n, sponsor, user)
	userNonce := uint64(0)
	input := followerSponsoredInputAtNonce(t, delegateABI, user, sponsor, nil, followerCodeQuote{Sponsor: crypto.PubkeyToAddress(sponsor.PublicKey), Token: token, MaxTokenAmount: big.NewInt(3_114_000), TokenAmount: big.NewInt(3_114_000), Deadline: big.NewInt(time.Now().Add(10 * time.Minute).Unix()), QuoteNonce: big.NewInt(21), GasCost: big.NewInt(1_000_000_000_000_000_000)}, big.NewInt(0))
	setup := n.signSetCode(t, sponsor, crypto.PubkeyToAddress(user.PublicKey), input, []types.SetCodeAuthorization{followerCodeAuthorization(t, user, delegate, userNonce)})
	n.compareCodeReceipt(t, setup, types.ReceiptStatusSuccessful, true)
	keyA, keyB := n.freshKey(t), n.freshKey(t)
	txA := n.signCreate(t, keyA, followerTokenCreation(t, contracts, tokenABI))
	txB := n.signCreate(t, keyB, followerTokenCreation(t, contracts, tokenABI))
	// The three transactions are submitted together; the batch is a plain call through the existing delegation.
	batchInput := followerSponsoredInputAtNonce(t, delegateABI, user, sponsor, nil, followerCodeQuote{Sponsor: crypto.PubkeyToAddress(sponsor.PublicKey), Token: token, MaxTokenAmount: big.NewInt(3_114_000), TokenAmount: big.NewInt(3_114_000), Deadline: big.NewInt(time.Now().Add(10 * time.Minute).Unix()), QuoteNonce: big.NewInt(22), GasCost: big.NewInt(1_000_000_000_000_000_000)}, big.NewInt(1))
	batch := n.sign(t, user, crypto.PubkeyToAddress(user.PublicKey), new(big.Int), batchInput)
	ctx, cancel := context.WithTimeout(n.ctx, 60*time.Second)
	errCh := make(chan error, 3)
	for _, tx := range []*types.Transaction{txA, txB, batch} {
		go func(tx *types.Transaction) { errCh <- n.send(ctx, tx) }(tx)
	}
	for range 3 {
		if err := <-errCh; err != nil {
			cancel()
			t.Fatalf("mixed-block submission: %v", err)
		}
	}
	cancel()
	receipts := make([]*types.Receipt, 0, 3)
	for _, tx := range []*types.Transaction{txA, txB, batch} {
		receipts = append(receipts, n.compareCodeReceipt(t, tx, types.ReceiptStatusSuccessful, false))
	}
	if receipts[0].BlockNumber.Cmp(receipts[1].BlockNumber) != 0 || receipts[0].BlockNumber.Cmp(receipts[2].BlockNumber) != 0 {
		t.Fatalf("mixed transactions committed in different blocks: %s, %s, %s", receipts[0].BlockNumber, receipts[1].BlockNumber, receipts[2].BlockNumber)
	}
	ctx, cancel = context.WithTimeout(n.ctx, 15*time.Second)
	primaryBlock, err := n.primary.BlockByNumber(ctx, receipts[0].BlockNumber)
	if err != nil {
		cancel()
		t.Fatalf("read primary mixed block: %v", err)
	}
	followerBlock, err := n.follower.BlockByNumber(ctx, receipts[0].BlockNumber)
	cancel()
	if err != nil {
		t.Fatalf("read follower mixed block: %v", err)
	}
	if len(primaryBlock.Transactions()) != len(followerBlock.Transactions()) {
		t.Fatalf("mixed block %s transaction count differs: primary=%d follower=%d", receipts[0].BlockNumber, len(primaryBlock.Transactions()), len(followerBlock.Transactions()))
	}
	for index, transaction := range primaryBlock.Transactions() {
		if transaction.Hash() != followerBlock.Transactions()[index].Hash() {
			t.Fatalf("mixed block %s order at %d differs: primary=%s follower=%s", receipts[0].BlockNumber, index, transaction.Hash(), followerBlock.Transactions()[index].Hash())
		}
	}
	_, indexes, _ := n.layout(t, receipts[0].BlockNumber)
	for _, tx := range []*types.Transaction{txA, txB, batch} {
		if _, ok := indexes[tx.Hash()]; !ok {
			t.Fatalf("mixed transaction %s missing from block layout", tx.Hash())
		}
	}
	statusCtx, statusCancel := context.WithTimeout(n.ctx, 15*time.Second)
	defer statusCancel()
	var height hexutil.Big
	if err := n.primary.Client().CallContext(statusCtx, &height, "eth_blockNumber"); err != nil {
		t.Fatalf("sequencer height after mixed block: %v", err)
	}
	for {
		var status coretypes.ResultStatus
		if alive, err := n.alive(); !alive {
			t.Fatalf("follower exited after mixed block: %v", err)
		}
		if err := followerCometRead(statusCtx, n.followerComet, "/status", &status); err != nil {
			t.Fatalf("follower status after mixed block: %v", err)
		}
		if status.SyncInfo.LatestBlockHeight >= height.ToInt().Int64() {
			break
		}
		if statusCtx.Err() != nil {
			t.Fatalf("follower height %d is behind sequencer height %d", status.SyncInfo.LatestBlockHeight, height.ToInt().Int64())
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func (n *followerNetwork) assertUsedQuote(t *testing.T, user common.Address, quote followerCodeQuote, block *big.Int, nonce uint64) {
	t.Helper()
	sponsorSlot := crypto.Keccak256Hash(common.BytesToHash(quote.Sponsor.Bytes()).Bytes(), common.BigToHash(big.NewInt(1)).Bytes())
	slot := crypto.Keccak256Hash(common.BigToHash(quote.QuoteNonce).Bytes(), sponsorSlot.Bytes())
	n.assertCodeNonceStorage(t, user, block, slot)
	ctx, cancel := context.WithTimeout(n.ctx, 15*time.Second)
	defer cancel()
	for _, client := range []*ethclient.Client{n.primary, n.follower} {
		used, err := client.StorageAt(ctx, user, slot, block)
		if err != nil || new(big.Int).SetBytes(used).Cmp(big.NewInt(1)) != 0 {
			t.Fatalf("quote %s not used at block %s: %x, %v", quote.QuoteNonce, block, used, err)
		}
		actual, err := client.StorageAt(ctx, user, common.Hash{}, block)
		if err != nil || new(big.Int).SetBytes(actual).Uint64() != nonce {
			t.Fatalf("batch nonce at block %s=%x, want %d, error=%v", block, actual, nonce, err)
		}
	}
}
