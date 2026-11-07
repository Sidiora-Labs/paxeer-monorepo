package main

import (
	"context"
	"crypto/ecdsa"
	"flag"
	"fmt"
	"io"
	"math/big"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"

	paxbech32 "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/bech32"
)

const (
	evmRPC     = "http://127.0.0.1:8545"
	chainID    = 713714 // default EVM chain ID for "pax-chain"
	targetTPS  = 500
	numWorkers = 250
	rpcTimeout = 10 * time.Second

	// Total unique sender accounts pre-funded in genesis. Every tx sent by
	// the stress test has a distinct sender with nonce=0. At targetTPS, the
	// pool lasts totalAccounts/targetTPS seconds.
	totalAccounts = 50_000

	workloadModeTransfer        = "transfer"
	workloadModeContractStorage = "contract-storage"
)

var (
	bigChainID  = big.NewInt(chainID)
	signer      = types.NewLondonSigner(bigChainID)
	maxFee      = big.NewInt(1_000_000_000_000) // 1000 gwei
	priorityFee = big.NewInt(1_000_000_000)     // 1 gwei
	txValue     = big.NewInt(1_000_000_000_001) // 10^12+1 wei: touches both uhpx balance and wei remainder
)

// storageContractInitCode is the init code for a minimal contract whose
// constructor stores the contract's own ADDRESS at slot 0 and then
// returns an empty runtime. Used by -mode contract-storage to deposit
// per-tx EVM storage state in memiavl that the FlatKV EVM migrate then has
// to drain — the migration's per-block batch copier moves account +
// storage + code rows, so a workload that produces all three kinds in
// volume is what the cluster-level test scenario needs.
//
// Hand-assembled to avoid a Solidity compiler dependency:
//
//	30          ADDRESS
//	60 00       PUSH1 0
//	55          SSTORE         // sstore(slot=0, value=address)
//	60 00       PUSH1 0
//	60 00       PUSH1 0
//	f3          RETURN         // return runtime of length 0
//
// Total: 9 bytes; CREATE cost ~32000, SSTORE (cold) 20000, send out
// well within the per-tx Gas budget below.
var storageContractInitCode = []byte{
	0x30, 0x60, 0x00, 0x55, 0x60, 0x00, 0x60, 0x00, 0xf3,
}

// nextKey returns a unique deterministic private key for the given index.
func nextKey(idx uint64) *ecdsa.PrivateKey {
	seed := make([]byte, 32)
	// use upper 8 bytes for the index so seed is never all-zero
	seed[0] = 0x01
	for i := 0; i < 8; i++ {
		seed[1+i] = byte(idx >> (56 - 8*i))
	}
	key, err := crypto.ToECDSA(seed)
	if err != nil {
		panic(fmt.Sprintf("bad key seed %d: %v", idx, err))
	}
	return key
}

func keyAddr(key *ecdsa.PrivateKey) common.Address {
	return crypto.PubkeyToAddress(key.PublicKey)
}

func evmToPax(addr common.Address) string {
	s, err := paxbech32.ConvertAndEncode("pax", addr.Bytes())
	if err != nil {
		panic(fmt.Sprintf("bech32 encode: %v", err))
	}
	return s
}

func signTx(tx *types.Transaction, key *ecdsa.PrivateKey) *types.Transaction {
	signed, err := types.SignTx(tx, signer, key)
	if err != nil {
		panic(err)
	}
	return signed
}

func transfer(nonce uint64, to common.Address, key *ecdsa.PrivateKey) *types.Transaction {
	return signTx(types.NewTx(&types.DynamicFeeTx{
		ChainID:   bigChainID,
		Nonce:     nonce,
		GasTipCap: priorityFee,
		GasFeeCap: maxFee,
		Gas:       21_000,
		To:        &to,
		Value:     txValue,
	}), key)
}

// deployStorageContract returns a signed CREATE transaction whose init
// code is storageContractInitCode. Each sender's deploy lands at a
// distinct contract address (CREATE: address = keccak(sender || nonce))
// and emits one fresh (account, code, storage) triple into EVM state —
// which is the migration-test ammunition this mode exists to produce.
func deployStorageContract(nonce uint64, key *ecdsa.PrivateKey) *types.Transaction {
	return signTx(types.NewTx(&types.DynamicFeeTx{
		ChainID:   bigChainID,
		Nonce:     nonce,
		GasTipCap: priorityFee,
		GasFeeCap: maxFee,
		// 21000 base + ~32000 CREATE + ~20000 SSTORE (cold) + memory
		// + a small margin. 200k leaves plenty of headroom should the
		// EVM gas schedule ever shift.
		Gas:   200_000,
		To:    nil, // CREATE
		Value: big.NewInt(0),
		Data:  storageContractInitCode,
	}), key)
}

func waitForBalance(ctx context.Context, client *ethclient.Client, addr common.Address, timeout time.Duration) error {
	fmt.Printf("waiting for %s to have balance...\n", addr.Hex())
	for {
		callCtx, cancel := context.WithTimeout(ctx, timeout)
		bal, err := client.BalanceAt(callCtx, addr, nil)
		cancel()
		if err == nil && bal.Sign() > 0 {
			fmt.Printf("  %s: %s wei\n", addr.Hex(), bal.String())
			return nil
		}
		timer := time.NewTimer(300 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

type submissionCounts struct {
	attempted uint64
	accepted  uint64
	refused   uint64
}

func runStress(ctx context.Context, client *ethclient.Client, funded <-chan *ecdsa.PrivateKey,
	ticks <-chan time.Time, workers int, timeout time.Duration,
	makeTx func(*ecdsa.PrivateKey) *types.Transaction, out io.Writer) submissionCounts {
	var counts submissionCounts
	var mu sync.Mutex
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				var key *ecdsa.PrivateKey
				select {
				case <-ctx.Done():
					return
				case next, ok := <-funded:
					if !ok {
						return
					}
					key = next
				}
				select {
				case <-ctx.Done():
					return
				case <-ticks:
				}
				if ctx.Err() != nil {
					return
				}
				tx := makeTx(key)
				callCtx, cancel := context.WithTimeout(ctx, timeout)
				err := client.SendTransaction(callCtx, tx)
				cancel()
				mu.Lock()
				counts.attempted++
				if err != nil {
					counts.refused++
					fmt.Fprintf(out, "submission refused tx=%s: %v\n", tx.Hash().Hex(), err)
				} else {
					counts.accepted++
				}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	status := "completed"
	if counts.refused > 0 {
		status = "degraded"
	}
	if ctx.Err() != nil {
		status = "canceled"
	}
	fmt.Fprintf(out, "run %s: attempted=%d accepted=%d refused=%d\n",
		status, counts.attempted, counts.accepted, counts.refused)
	return counts
}

func main() {
	dumpPaxAddrs := flag.Bool("dump-pax-addrs", false, "print sender pax bech32 addresses for genesis funding and exit")
	mode := flag.String("mode", workloadModeTransfer,
		"workload mode: 'transfer' (one 21k-gas value transfer per sender, default) or "+
			"'contract-storage' (one CREATE per sender that deploys a 1-slot SSTORE constructor; "+
			"used by the FlatKV EVM migrate cluster test to deposit account+code+storage state "+
			"in memiavl before the migration)")
	flag.Parse()

	// Key 0 = recipient; keys 1..totalAccounts = one-time genesis-funded senders.
	recipient := keyAddr(nextKey(0))

	if *dumpPaxAddrs {
		for i := uint64(1); i <= totalAccounts; i++ {
			fmt.Println(evmToPax(keyAddr(nextKey(i))))
		}
		return
	}

	// Validate mode early so a typo fails before we connect to a node.
	var makeTx func(key *ecdsa.PrivateKey) *types.Transaction
	switch *mode {
	case workloadModeTransfer:
		makeTx = func(key *ecdsa.PrivateKey) *types.Transaction {
			return transfer(0, recipient, key)
		}
	case workloadModeContractStorage:
		makeTx = func(key *ecdsa.PrivateKey) *types.Transaction {
			return deployStorageContract(0, key)
		}
	default:
		panic(fmt.Sprintf("unknown -mode %q (expected %q or %q)", *mode, workloadModeTransfer, workloadModeContractStorage))
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	dialCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
	client, err := ethclient.DialContext(dialCtx, evmRPC)
	cancel()
	if err != nil {
		panic(fmt.Sprintf("dial %s: %v", evmRPC, err))
	}
	defer client.Close()

	fmt.Printf("recipient: %s mode: %s\n", recipient.Hex(), *mode)

	// Wait for genesis accounts to have balance — confirms the node is live.
	if err := waitForBalance(ctx, client, keyAddr(nextKey(1)), rpcTimeout); err != nil {
		fmt.Fprintf(os.Stderr, "balance wait stopped: %v\n", err)
		return
	}

	// Pre-fill the work queue. Each key is used for exactly one tx (nonce=0).
	funded := make(chan *ecdsa.PrivateKey, totalAccounts)
	for i := uint64(1); i <= totalAccounts; i++ {
		funded <- nextKey(i)
	}
	close(funded)

	// Shared rate limiter across all workers: one tick per tx slot.
	ticker := time.NewTicker(time.Second / time.Duration(targetTPS))
	defer ticker.Stop()

	fmt.Printf("starting %d workers, %d unique senders, target %d TPS\n",
		numWorkers, totalAccounts, targetTPS)

	runStress(ctx, client, funded, ticker.C, numWorkers, rpcTimeout, makeTx, os.Stdout)
}
