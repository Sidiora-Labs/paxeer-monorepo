package generator

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/paxeer-network/pax-load/config"
	"github.com/paxeer-network/pax-load/generator/scenarios"
	"github.com/paxeer-network/pax-load/types"
)

// FixedAccountPool exposes the finite population without selecting or creating accounts.
type FixedAccountPool interface {
	types.AccountPool
	FixedAccounts() []*types.Account
}

type fixedAccountPool struct {
	types.AccountPool
	accounts []*types.Account
}

func newFixedAccountPool(cfg *types.AccountConfig) types.AccountPool {
	var fixed []*types.Account
	// Fresh-only workloads have no reusable accounts to prewarm.
	if cfg.NewAccountRate != 1 {
		fixed = append(fixed, cfg.Accounts...)
	}
	return &fixedAccountPool{AccountPool: types.NewAccountPool(cfg), accounts: fixed}
}

func (p *fixedAccountPool) FixedAccounts() []*types.Account {
	return append([]*types.Account(nil), p.accounts...)
}

// PrewarmGenerator visits each fixed account once, independent of selection and nonce.
type PrewarmGenerator struct {
	accountPools []types.AccountPool
	accounts     []*types.Account
	evmScenario  scenarios.TxGenerator
	next         int
	hashes       []common.Hash
	mu           sync.RWMutex
}

// NewPrewarmGenerator requires pools that explicitly expose their finite population.
func NewPrewarmGenerator(cfg *config.LoadConfig, mainGenerator Generator) *PrewarmGenerator {
	pg := &PrewarmGenerator{evmScenario: scenarios.NewEVMTransferScenario(config.Scenario{})}
	seen := make(map[common.Address]bool)
	pools := make(map[types.AccountPool]bool)
	for _, pool := range mainGenerator.GetAccountPools() {
		if pools[pool] {
			continue
		}
		pools[pool] = true
		fixed, ok := pool.(FixedAccountPool)
		if !ok {
			panic("prewarm requires a FixedAccountPool with an explicit account inventory")
		}
		pg.accountPools = append(pg.accountPools, pool)
		for _, account := range fixed.FixedAccounts() {
			if !seen[account.Address] {
				seen[account.Address] = true
				pg.accounts = append(pg.accounts, account)
			}
		}
	}
	pg.evmScenario.Deploy(cfg, nil)
	return pg
}

func (pg *PrewarmGenerator) Generate() (*types.LoadTx, bool) {
	pg.mu.Lock()
	defer pg.mu.Unlock()
	if pg.next == len(pg.accounts) {
		return nil, false
	}
	account := pg.accounts[pg.next]
	tx := pg.evmScenario.Generate(&types.TxScenario{
		Name: "EVMTransfer", Sender: account, Receiver: account.Address,
	})
	pg.hashes = append(pg.hashes, tx.EthTx.Hash())
	pg.next++
	return tx, true
}

func (pg *PrewarmGenerator) GenerateN(n int) []*types.LoadTx {
	result := make([]*types.LoadTx, 0, n)
	for i := 0; i < n; i++ {
		tx, ok := pg.Generate()
		if !ok {
			break
		}
		result = append(result, tx)
	}
	return result
}

func (pg *PrewarmGenerator) GetAccountPools() []types.AccountPool {
	pg.mu.RLock()
	defer pg.mu.RUnlock()
	return append([]types.AccountPool(nil), pg.accountPools...)
}

// PrewarmReceiptReader is implemented by ethclient.Client.
type PrewarmReceiptReader interface {
	TransactionReceipt(context.Context, common.Hash) (*ethtypes.Receipt, error)
}

// Wait must succeed after submission and before resetting counters or measuring traffic.
// Exhausting Generate alone does not establish that prewarm transactions were mined.
func (pg *PrewarmGenerator) Wait(ctx context.Context, reader PrewarmReceiptReader) error {
	pg.mu.RLock()
	if pg.next != len(pg.accounts) {
		pg.mu.RUnlock()
		return fmt.Errorf("prewarm generation is incomplete")
	}
	hashes := append([]common.Hash(nil), pg.hashes...)
	pg.mu.RUnlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(hashes) == 0 {
		return nil
	}
	if reader == nil {
		return fmt.Errorf("prewarm receipt reader is required")
	}
	for _, hash := range hashes {
		for {
			if err := ctx.Err(); err != nil {
				return err
			}
			receipt, err := reader.TransactionReceipt(ctx, hash)
			if err == nil {
				if receipt == nil || receipt.TxHash != hash || receipt.BlockNumber == nil {
					return fmt.Errorf("invalid prewarm receipt for %s", hash)
				}
				if receipt.Status != ethtypes.ReceiptStatusSuccessful {
					return fmt.Errorf("prewarm transaction %s failed with status %d", hash, receipt.Status)
				}
				break
			}
			if !errors.Is(err, ethereum.NotFound) {
				return fmt.Errorf("prewarm receipt %s: %w", hash, err)
			}
			timer := time.NewTimer(100 * time.Millisecond)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
		}
	}
	return nil
}
