package localnode

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	cometjson "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/json"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	pricefeedtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/params"
	ledger "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxledger"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/std"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/bech32"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

type followerNetwork struct {
	ctx                              context.Context
	primary, follower                *ethclient.Client
	comet, followerComet, home, paxd string
	decode                           sdk.TxDecoder
	txConfig                         client.TxConfig
}

func newFollowerNetwork(t *testing.T) *followerNetwork {
	t.Helper()
	for _, name := range []string{"LOCAL_NODE_FOLLOWER_COMET", "LOCAL_NODE_FOLLOWER_RPC"} {
		if os.Getenv(name) == "" {
			t.Fatalf("%s is required; start v6120-node.sh --sequencer --follower and source its env file", name)
		}
	}
	for _, name := range []string{"LOCAL_NODE_SEQUENCER_RPC", "PERPS_DATA_TEST_COMET", "LOCAL_NODE_HOME"} {
		if os.Getenv(name) == "" {
			t.Fatalf("%s is required for the local follower network", name)
		}
	}
	n := &followerNetwork{ctx: context.Background(), comet: os.Getenv("PERPS_DATA_TEST_COMET"), followerComet: os.Getenv("LOCAL_NODE_FOLLOWER_COMET"), home: os.Getenv("LOCAL_NODE_HOME"), paxd: os.Getenv("PAXD")}
	if n.paxd == "" {
		for _, line := range strings.Split(string(followerReadFile(t, filepath.Join(n.home, "process.env"))), "\n") {
			if strings.HasPrefix(line, "PAXD=") {
				n.paxd = strings.TrimPrefix(line, "PAXD=")
			}
		}
	}
	if n.paxd == "" {
		t.Fatal("PAXD is required in the environment or node process.env")
	}
	var err error
	n.primary, err = ethclient.Dial(os.Getenv("LOCAL_NODE_SEQUENCER_RPC"))
	if err != nil {
		t.Fatalf("dial sequencer: %v", err)
	}
	t.Cleanup(n.primary.Close)
	n.follower, err = ethclient.Dial(os.Getenv("LOCAL_NODE_FOLLOWER_RPC"))
	if err != nil {
		t.Fatalf("dial follower: %v", err)
	}
	t.Cleanup(n.follower.Close)
	encoding := params.MakeEncodingConfig()
	std.RegisterInterfaces(encoding.InterfaceRegistry)
	evmtypes.RegisterInterfaces(encoding.InterfaceRegistry)
	banktypes.RegisterInterfaces(encoding.InterfaceRegistry)
	pricefeedtypes.RegisterInterfaces(encoding.InterfaceRegistry)
	n.decode = encoding.TxConfig.TxDecoder()
	n.txConfig = encoding.TxConfig
	t.Cleanup(func() {
		if !t.Failed() {
			return
		}
		alive, err := n.alive()
		t.Logf("follower process alive=%t: %v", alive, err)
		data, err := exec.Command("tail", "-n", "40", filepath.Join(n.home, "bootstrap-peer.log")).CombinedOutput()
		t.Logf("follower last forty log lines (error %v):\n%s", err, data)
	})
	return n
}

func (n *followerNetwork) alive() (bool, error) {
	data, err := os.ReadFile(filepath.Join(n.home, "bootstrap-peer.pid"))
	if err != nil {
		return false, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 {
		return false, fmt.Errorf("invalid follower pid %q", data)
	}
	if err := syscall.Kill(pid, 0); err != nil {
		return false, err
	}
	stat, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return false, err
	}
	fields := strings.Fields(string(stat)[strings.LastIndexByte(string(stat), ')')+1:])
	if len(fields) == 0 || fields[0] == "Z" {
		return false, fmt.Errorf("follower process %d is not running", pid)
	}
	return true, nil
}

func followerReadFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}

func followerCometRead(ctx context.Context, endpoint, path string, result interface{}) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+path, nil)
	if err != nil {
		return err
	}
	response, err := (&http.Client{Timeout: 3 * time.Second}).Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("Comet %s: HTTP %d", path, response.StatusCode)
	}
	var raw json.RawMessage
	if err := json.NewDecoder(response.Body).Decode(&raw); err != nil {
		return err
	}
	var rpcError struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &rpcError); err != nil {
		return err
	}
	if rpcError.Code != 0 {
		return fmt.Errorf("Comet %s: RPC %d: %s: %s", path, rpcError.Code, rpcError.Message, rpcError.Data)
	}
	return cometjson.Unmarshal(raw, result)
}

func (n *followerNetwork) command(t *testing.T, args ...string) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(n.ctx, 30*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, n.paxd, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("paxd %s: %v: %s", strings.Join(args, " "), err, output)
	}
	return output
}

func (n *followerNetwork) bankSend(t *testing.T, address common.Address) string {
	t.Helper()
	cast, err := bech32.ConvertAndEncode("pax", address.Bytes())
	if err != nil {
		t.Fatalf("encode cast address: %v", err)
	}
	output := n.command(t, "tx", "bank", "send", "validator", cast, "100000000uhpx", "--home", n.home,
		"--keyring-backend", "test", "--chain-id", "hyperpax_125-1", "--node", n.comet,
		"--gas", "200000", "--fees", "1000000uhpx", "-b", "sync", "-o", "json", "-y")
	var result struct {
		Code uint32 `json:"code"`
		Hash string `json:"txhash"`
		Log  string `json:"raw_log"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("decode bank broadcast: %v: %s", err, output)
	}
	if result.Code != 0 || len(result.Hash) != 64 {
		t.Fatalf("bank broadcast rejected: %s", output)
	}
	return result.Hash
}

func (n *followerNetwork) bankReceipt(t *testing.T, hash string) coretypes.ResultTx {
	t.Helper()
	ctx, cancel := context.WithTimeout(n.ctx, 60*time.Second)
	defer cancel()
	var result coretypes.ResultTx
	var last error
	for ctx.Err() == nil {
		last = followerCometRead(ctx, n.comet, "/tx?hash="+hash, &result)
		if last == nil && result.Height > 0 {
			if result.TxResult.Code != 0 {
				t.Fatalf("bank transaction %s failed: %+v", hash, result.TxResult)
			}
			return result
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("bank transaction %s did not commit: %v", hash, last)
	return result
}

func (n *followerNetwork) prepareBank(t *testing.T, sender string, address common.Address) []byte {
	t.Helper()
	cast, err := bech32.ConvertAndEncode("pax", address.Bytes())
	if err != nil {
		t.Fatalf("encode cast address: %v", err)
	}
	unsigned := n.command(t, "tx", "bank", "send", sender, cast, "1uhpx", "--home", n.home,
		"--keyring-backend", "test", "--chain-id", "hyperpax_125-1", "--node", n.comet,
		"--gas", "200000", "--fees", "1000000uhpx", "--generate-only", "-o", "json", "-y")
	path := filepath.Join(t.TempDir(), "bank.json")
	if err := os.WriteFile(path, unsigned, 0600); err != nil {
		t.Fatalf("write unsigned bank transaction: %v", err)
	}
	signed := n.command(t, "tx", "sign", path, "--from", sender, "--home", n.home,
		"--keyring-backend", "test", "--chain-id", "hyperpax_125-1", "--node", n.comet, "-o", "json")
	tx, err := n.txConfig.TxJSONDecoder()(signed)
	if err != nil {
		t.Fatalf("decode signed bank transaction: %v", err)
	}
	raw, err := n.txConfig.TxEncoder()(tx)
	if err != nil {
		t.Fatalf("encode bank transaction: %v", err)
	}
	return raw
}

func (n *followerNetwork) broadcast(ctx context.Context, raw []byte) error {
	var result coretypes.ResultBroadcastTx
	if err := followerCometRead(ctx, n.comet, "/broadcast_tx_sync?tx="+hexutil.Encode(raw), &result); err != nil {
		return err
	}
	if result.Code != 0 {
		return fmt.Errorf("consensus broadcast rejected: code=%d codespace=%s log=%s", result.Code, result.Codespace, result.Log)
	}
	hash := sha256.Sum256(raw)
	if !bytes.Equal(result.Hash, hash[:]) {
		return fmt.Errorf("consensus broadcast hash %X differs from signed hash %X", result.Hash, hash)
	}
	return nil
}

func (n *followerNetwork) waitBankAdmission(ctx context.Context, banks [][]byte) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	accepted := make(chan error, len(banks))
	for _, bank := range banks {
		hash := common.Hash(sha256.Sum256(bank))
		go func() {
			tick := time.NewTicker(10 * time.Millisecond)
			defer tick.Stop()
			for {
				var receipt *evmrpc.SequencerReceiptResult
				if err := n.primary.Client().CallContext(ctx, &receipt, "pax_sequencerReceipt", hash); err != nil {
					accepted <- fmt.Errorf("read bank admission %s: %w", hash, err)
					return
				}
				if receipt != nil && receipt.Rung != evmrpc.RungSubmitted {
					if receipt.Rung != evmrpc.RungReceipted && receipt.Rung != evmrpc.RungCommitted {
						accepted <- fmt.Errorf("bank admission %s has unexpected rung %q", hash, receipt.Rung)
						return
					}
					accepted <- receipt.Verify()
					return
				}
				select {
				case <-ctx.Done():
					accepted <- ctx.Err()
					return
				case <-tick.C:
				}
			}
		}()
	}
	return <-accepted
}

func (n *followerNetwork) freshKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("generate first-use EVM key: %v", err)
	}
	address := crypto.PubkeyToAddress(key.PublicKey)
	n.bankReceipt(t, n.bankSend(t, address))
	ctx, cancel := context.WithTimeout(n.ctx, 15*time.Second)
	defer cancel()
	nonce, err := n.primary.NonceAt(ctx, address, nil)
	if err != nil || nonce != 0 {
		t.Fatalf("first-use key %s nonce = %d, error %v; want zero", address, nonce, err)
	}
	balance, err := n.primary.BalanceAt(ctx, address, nil)
	if err != nil || balance == nil || balance.Sign() <= 0 {
		t.Fatalf("cast-funded key %s balance = %v, error %v", address, balance, err)
	}
	t.Logf("first-use key %s funded by bank transfer to its cast address", address)
	return key
}

func (n *followerNetwork) sign(t *testing.T, key *ecdsa.PrivateKey, to common.Address, value *big.Int, data []byte) *types.Transaction {
	t.Helper()
	ctx, cancel := context.WithTimeout(n.ctx, 15*time.Second)
	defer cancel()
	nonce, err := n.primary.NonceAt(ctx, crypto.PubkeyToAddress(key.PublicKey), nil)
	if err != nil {
		t.Fatalf("read committed nonce: %v", err)
	}
	tx, err := types.SignTx(types.NewTx(&types.LegacyTx{Nonce: nonce, GasPrice: big.NewInt(1_000_000_000_000), Gas: 7_000_000, To: &to, Value: value, Data: data}), types.LatestSignerForChainID(big.NewInt(125)), key)
	if err != nil {
		t.Fatalf("sign EVM transaction: %v", err)
	}
	return tx
}

func (n *followerNetwork) send(ctx context.Context, tx *types.Transaction) error {
	raw, err := tx.MarshalBinary()
	if err != nil {
		return err
	}
	var hash common.Hash
	err = localNodePendingCall(ctx, func() error {
		return n.primary.Client().CallContext(ctx, &hash, "eth_sendRawTransaction", hexutil.Encode(raw))
	})
	if err == nil && hash != tx.Hash() {
		return fmt.Errorf("broadcast hash %s differs from signed hash %s", hash, tx.Hash())
	}
	return err
}

func (n *followerNetwork) layout(t *testing.T, height *big.Int) (int, map[common.Hash]int, string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(n.ctx, 5*time.Second)
	defer cancel()
	var block coretypes.ResultBlock
	if err := followerCometRead(ctx, n.comet, "/block?height="+height.String(), &block); err != nil || block.Block == nil {
		t.Fatalf("read block %s layout: %v", height, err)
	}
	indexes := make(map[common.Hash]int)
	var kinds []string
	for i, raw := range block.Block.Txs {
		hash := sha256.Sum256(raw)
		if bytes.HasPrefix(raw, []byte("PAXEER/v1/sequence-batch\x00")) {
			kinds = append(kinds, "sequence-batch")
			t.Logf("block %s entry %d: sequence batch sha256=%X", height, i, hash)
			continue
		}
		tx, err := n.decode(raw)
		if err != nil {
			t.Fatalf("decode block %s entry %d (%X): %v", height, i, hash, err)
		}
		var names []string
		for _, msg := range tx.GetMsgs() {
			names = append(names, sdk.MsgTypeURL(msg))
			if evm, ok := msg.(*evmtypes.MsgEVMTransaction); ok {
				eth, _ := evm.AsTransaction()
				if eth == nil {
					t.Fatalf("block entry %d has no EVM transaction", i)
				}
				indexes[eth.Hash()] = i
				names = append(names, eth.Hash().Hex())
			}
		}
		kinds = append(kinds, strings.Join(names, "+"))
		t.Logf("block %s entry %d: %s sha256=%X", height, i, strings.Join(names, " "), hash)
	}
	return len(block.Block.Txs), indexes, strings.Join(kinds, " -> ")
}

func (n *followerNetwork) compare(t *testing.T, tx *types.Transaction, wantStatus uint64, submit bool) *types.Receipt {
	t.Helper()
	ctx, cancel := context.WithTimeout(n.ctx, 60*time.Second)
	defer cancel()
	if submit {
		if err := n.send(ctx, tx); err != nil {
			t.Fatalf("send %s: %v", tx.Hash(), err)
		}
	}
	var receipt *types.Receipt
	for ctx.Err() == nil {
		var err error
		receipt, err = n.primary.TransactionReceipt(ctx, tx.Hash())
		if err == nil {
			break
		}
		if !errors.Is(err, ethereum.NotFound) {
			t.Fatalf("sequencer receipt %s: %v", tx.Hash(), err)
		}
		time.Sleep(100 * time.Millisecond)
	}
	if receipt == nil {
		t.Fatalf("sequencer receipt %s not committed within sixty seconds", tx.Hash())
	}
	n.layout(t, receipt.BlockNumber)
	if receipt.Status != wantStatus {
		t.Fatalf("sequencer receipt %s status %d, want %d", tx.Hash(), receipt.Status, wantStatus)
	}
	followCtx, followCancel := context.WithTimeout(n.ctx, 60*time.Second)
	defer followCancel()
	var last string
	for followCtx.Err() == nil {
		if alive, err := n.alive(); !alive {
			t.Fatalf("follower process exited while following block %s for transaction %s: %v", receipt.BlockNumber, tx.Hash(), err)
		}
		var status coretypes.ResultStatus
		err := followerCometRead(followCtx, n.followerComet, "/status", &status)
		if err != nil {
			last = err.Error()
			time.Sleep(100 * time.Millisecond)
			continue
		}
		if status.SyncInfo.LatestBlockHeight < receipt.BlockNumber.Int64() {
			last = fmt.Sprintf("follower height %d, want at least %s", status.SyncInfo.LatestBlockHeight, receipt.BlockNumber)
			time.Sleep(100 * time.Millisecond)
			continue
		}
		followerReceipt, err := n.follower.TransactionReceipt(followCtx, tx.Hash())
		if err != nil {
			last = err.Error()
			time.Sleep(100 * time.Millisecond)
			continue
		}
		if followerReceipt.Status != receipt.Status || followerReceipt.BlockHash != receipt.BlockHash || followerReceipt.BlockNumber.Cmp(receipt.BlockNumber) != 0 {
			t.Fatalf("follower receipt differs at block %s: primary hash=%s status=%d follower hash=%s status=%d height=%s", receipt.BlockNumber, receipt.BlockHash, receipt.Status, followerReceipt.BlockHash, followerReceipt.Status, followerReceipt.BlockNumber)
		}
		var primaryBlock, followerBlock coretypes.ResultBlock
		path := "/block?height=" + receipt.BlockNumber.String()
		if err := followerCometRead(followCtx, n.comet, path, &primaryBlock); err != nil {
			t.Fatalf("primary block: %v", err)
		}
		if err := followerCometRead(followCtx, n.followerComet, path, &followerBlock); err != nil {
			t.Fatalf("follower block: %v", err)
		}
		if primaryBlock.Block == nil || followerBlock.Block == nil {
			t.Fatal("comparison returned an empty consensus block")
		}
		if !bytes.Equal(primaryBlock.BlockID.Hash, followerBlock.BlockID.Hash) || !bytes.Equal(primaryBlock.Block.AppHash, followerBlock.Block.AppHash) {
			t.Fatalf("consensus block %s differs: primary hash=%X app=%X follower hash=%X app=%X", receipt.BlockNumber, primaryBlock.BlockID.Hash, primaryBlock.Block.AppHash, followerBlock.BlockID.Hash, followerBlock.Block.AppHash)
		}
		// The next header commits the application state produced by this block.
		next := new(big.Int).Add(receipt.BlockNumber, big.NewInt(1))
		if status.SyncInfo.LatestBlockHeight < next.Int64() {
			last = "waiting for next header application hash"
			time.Sleep(100 * time.Millisecond)
			continue
		}
		path = "/block?height=" + next.String()
		if err := followerCometRead(followCtx, n.comet, path, &primaryBlock); err != nil {
			t.Fatalf("primary next header: %v", err)
		}
		if err := followerCometRead(followCtx, n.followerComet, path, &followerBlock); err != nil {
			t.Fatalf("follower next header: %v", err)
		}
		if primaryBlock.Block == nil || followerBlock.Block == nil {
			t.Fatal("comparison returned an empty next consensus block")
		}
		if !bytes.Equal(primaryBlock.Block.AppHash, followerBlock.Block.AppHash) {
			t.Fatalf("application hash after block %s differs: primary=%X follower=%X", receipt.BlockNumber, primaryBlock.Block.AppHash, followerBlock.Block.AppHash)
		}
		t.Logf("matching receipt at block %s: EVM hash=%s app after block=%X status=%d", receipt.BlockNumber, receipt.BlockHash, followerBlock.Block.AppHash, receipt.Status)
		return receipt
	}
	t.Fatalf("follower did not answer with matching block %s and receipt %s within sixty seconds: %s", receipt.BlockNumber, tx.Hash(), last)
	return nil
}

func TestLocalNodeV6120FollowerFirstUseTransfer(t *testing.T) {
	n := newFollowerNetwork(t)
	key := n.freshKey(t)
	recipient := common.HexToAddress("0x000000000000000000000000000000000000beef")
	n.compare(t, n.sign(t, key, recipient, big.NewInt(1), nil), types.ReceiptStatusSuccessful, true)
}

func TestLocalNodeV6120FollowerTwoFirstUseKeys(t *testing.T) {
	n := newFollowerNetwork(t)
	keys := []*ecdsa.PrivateKey{n.freshKey(t), n.freshKey(t)}
	for _, key := range keys {
		n.compare(t, n.sign(t, key, common.HexToAddress("0x000000000000000000000000000000000000beef"), big.NewInt(1), nil), types.ReceiptStatusSuccessful, true)
	}
}

func TestLocalNodeV6120FollowerBankBeforeFirstUse(t *testing.T) {
	n := newFollowerNetwork(t)
	ctx, cancel := context.WithTimeout(n.ctx, 10*time.Minute)
	defer cancel()
	n.ctx = ctx
	bankRecipient, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("generate bank recipient: %v", err)
	}
	recipient := crypto.PubkeyToAddress(bankRecipient.PublicKey)
	var senders []string
	for i := 0; i < 3; i++ {
		name := fmt.Sprintf("follower-bank-%s-%d", recipient.Hex()[2:], i)
		output := n.command(t, "keys", "add", name, "--home", n.home, "--keyring-backend", "test", "--output", "json", "--no-backup")
		var account struct {
			Address string `json:"address"`
		}
		if err := json.Unmarshal(output, &account); err != nil {
			t.Fatalf("decode bank sender: %v", err)
		}
		prefix, address, err := bech32.DecodeAndConvert(account.Address)
		if err != nil || prefix != "pax" || len(address) != common.AddressLength {
			t.Fatalf("decode bank sender address %q: %v", account.Address, err)
		}
		n.bankReceipt(t, n.bankSend(t, common.BytesToAddress(address)))
		senders = append(senders, name)
	}
	var recent []string
	for attempt := 0; attempt < 48 && ctx.Err() == nil; attempt++ {
		key := n.freshKey(t)
		tx := n.sign(t, key, common.HexToAddress("0x000000000000000000000000000000000000beef"), big.NewInt(1), nil)
		var banks [][]byte
		for _, sender := range senders {
			banks = append(banks, n.prepareBank(t, sender, recipient))
		}
		delay := time.Duration(attempt%12) * 15 * time.Millisecond
		sendCtx, sendCancel := context.WithTimeout(ctx, 60*time.Second)
		sent := make(chan error, 4)
		start := make(chan struct{})
		for _, bank := range banks {
			go func(raw []byte) { <-start; sent <- n.broadcast(sendCtx, raw) }(bank)
		}
		go func() {
			<-start
			timer := time.NewTimer(delay)
			defer timer.Stop()
			select {
			case <-timer.C:
				if err := n.waitBankAdmission(sendCtx, banks); err != nil {
					sent <- err
					return
				}
				sent <- n.send(sendCtx, tx)
			case <-sendCtx.Done():
				sent <- sendCtx.Err()
			}
		}()
		close(start)
		var sendErr error
		for i := 0; i < 4; i++ {
			sendErr = errors.Join(sendErr, <-sent)
		}
		sendCancel()
		if sendErr != nil {
			t.Fatalf("bank consensus broadcasts and EVM RPC send: %v", sendErr)
		}
		receipt := n.compare(t, tx, types.ReceiptStatusSuccessful, false)
		count, indexes, kinds := n.layout(t, receipt.BlockNumber)
		index, ok := indexes[tx.Hash()]
		if !ok {
			t.Fatalf("first-use EVM transaction %s absent from block layout", tx.Hash())
		}
		hit := false
		var bankPositions []string
		for _, bank := range banks {
			bankReceipt := n.bankReceipt(t, fmt.Sprintf("%X", sha256.Sum256(bank)))
			bankPositions = append(bankPositions, fmt.Sprintf("%d:%d", bankReceipt.Height, bankReceipt.Index))
			if bankReceipt.Height == receipt.BlockNumber.Int64() && int(bankReceipt.Index) < index {
				hit = true
			}
		}
		summary := fmt.Sprintf("try=%d height=%s delay=%s order=[%s] EVM-index=%d banks=[%s] entries=%d", attempt+1, receipt.BlockNumber, delay, kinds, index, strings.Join(bankPositions, ","), count)
		t.Log(summary)
		recent = append(recent, summary)
		if len(recent) > 3 {
			recent = recent[1:]
		}
		if hit {
			return
		}
	}
	t.Fatalf("no block placed a bank transfer ahead of the first-use EVM transfer within forty-eight tries and ten minutes: %s", strings.Join(recent, "; "))
}

func TestLocalNodeV6120FollowerFirstUseRevert(t *testing.T) {
	n := newFollowerNetwork(t)
	key := n.freshKey(t)
	usdl := common.HexToAddress("0x85FcD13735F4309833A503EE804ea32395851479")
	data := append(crypto.Keccak256([]byte("transfer(address,uint256)"))[:4], common.LeftPadBytes(common.HexToAddress("0x000000000000000000000000000000000000beef").Bytes(), 32)...)
	data = append(data, common.LeftPadBytes([]byte{1}, 32)...)
	n.compare(t, n.sign(t, key, usdl, big.NewInt(0), data), types.ReceiptStatusFailed, true)
}

func (n *followerNetwork) bind(t *testing.T, key *ecdsa.PrivateKey) {
	t.Helper()
	public, identity, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate identity: %v", err)
	}
	address := crypto.PubkeyToAddress(key.PublicKey)
	precompile := common.HexToAddress("0x0000000000000000000000000000000000001004")
	ctx, cancel := context.WithTimeout(n.ctx, 15*time.Second)
	defer cancel()
	nonceData := append(crypto.Keccak256([]byte("layerXBindNonce(address)"))[:4], common.LeftPadBytes(address.Bytes(), 32)...)
	nonceWord, err := n.primary.CallContract(ctx, ethereum.CallMsg{To: &precompile, Data: nonceData}, nil)
	if err != nil || len(nonceWord) != 32 {
		t.Fatalf("read bind nonce: %x, %v", nonceWord, err)
	}
	nonce := new(big.Int).SetBytes(nonceWord)
	if !nonce.IsUint64() || nonce.Sign() != 0 {
		t.Fatalf("first-use identity bind nonce = %s, want zero", nonce)
	}
	message := append([]byte("LX:PAXEER-BIND:v1"), common.LeftPadBytes(big.NewInt(125).Bytes(), 32)...)
	message = append(message, address.Bytes()...)
	var nonceBytes [8]byte
	binary.BigEndian.PutUint64(nonceBytes[:], nonce.Uint64())
	message = append(message, nonceBytes[:]...)
	signature := ed25519.Sign(identity, message)
	data := append(crypto.Keccak256([]byte("bindLayerX(bytes32,bytes)"))[:4], public...)
	data = append(data, common.LeftPadBytes([]byte{64}, 32)...)
	data = append(data, common.LeftPadBytes([]byte{64}, 32)...)
	data = append(data, signature...)
	receipt := n.compare(t, n.sign(t, key, precompile, big.NewInt(0), data), types.ReceiptStatusSuccessful, true)
	confirmCtx, confirmCancel := context.WithTimeout(n.ctx, 15*time.Second)
	defer confirmCancel()
	for _, client := range []*ethclient.Client{n.primary, n.follower} {
		query := append(crypto.Keccak256([]byte("getUnifiedAccount(address)"))[:4], common.LeftPadBytes(address.Bytes(), 32)...)
		answer, err := client.CallContract(confirmCtx, ethereum.CallMsg{To: &precompile, Data: query}, receipt.BlockNumber)
		if err != nil || len(answer) < 96 || !bytes.Equal(answer[64:96], public) {
			t.Fatalf("bound identity differs: %x, error %v", answer, err)
		}
	}
}

func TestLocalNodeV6120FollowerFirstUseIdentityBind(t *testing.T) {
	n := newFollowerNetwork(t)
	n.bind(t, n.freshKey(t))
}

func TestLocalNodeV6120FollowerLedgerDraw(t *testing.T) {
	n := newFollowerNetwork(t)
	key := n.freshKey(t)
	n.bind(t, key)
	contract := ledger.GetABI()
	data, err := contract.Pack("draw", [32]byte{1}, [32]byte{2}, big.NewInt(1), [32]byte{3}, uint64(0), [32]byte{4}, [32]byte{5})
	if err != nil {
		t.Fatalf("encode web search draw: %v", err)
	}
	address := common.HexToAddress("0x000000000000000000000000000000000000101a")
	ctx, cancel := context.WithTimeout(n.ctx, 15*time.Second)
	defer cancel()
	_, err = n.primary.CallContract(ctx, ethereum.CallMsg{From: crypto.PubkeyToAddress(key.PublicKey), To: &address, Data: data}, nil)
	if err == nil || !strings.Contains(err.Error(), "LXP_NO_PAYER_GRANT") {
		t.Fatalf("draw without payer grant: %v, want LXP_NO_PAYER_GRANT", err)
	}
	t.Log("ledger draw uses the web search calldata and refuses an absent payer grant")
	n.compare(t, n.sign(t, key, address, big.NewInt(0), data), types.ReceiptStatusFailed, true)
}
