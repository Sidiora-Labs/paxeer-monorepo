package types

import (
	"fmt"
	"math"
	"math/rand"
	"sync"
)

// AccountPool returns a next account for load generation.
type AccountPool interface {
	NextAccount() *Account
}

// AccountConfig stores the configuration for account generation.
type AccountConfig struct {
	Accounts       []*Account
	NewAccountRate float64
}

type accountPool struct {
	Accounts []*Account
	cfg      *AccountConfig

	mx  sync.Mutex
	idx int
}

func (a *accountPool) nextIndex() int {
	a.mx.Lock()
	defer a.mx.Unlock()
	a.idx++
	a.idx %= len(a.Accounts)
	return a.idx
}

// NextAccount returns the next account.
func (a *accountPool) NextAccount() *Account {
	if a.cfg.NewAccountRate > 0 {
		randomNumber := rand.Float64()
		if randomNumber <= a.cfg.NewAccountRate {
			return GenerateAccounts(1)[0]
		}
	}
	return a.Accounts[a.nextIndex()]
}

// NewAccountPool creates an account generator or returns a configuration error.
func NewAccountPool(cfg *AccountConfig) (AccountPool, error) {
	if cfg == nil {
		return nil, fmt.Errorf("account pool: configuration must not be nil")
	}
	if math.IsNaN(cfg.NewAccountRate) || math.IsInf(cfg.NewAccountRate, 0) ||
		cfg.NewAccountRate < 0 || cfg.NewAccountRate > 1 {
		return nil, fmt.Errorf("account pool: new account rate must be finite and between 0 and 1, got %v", cfg.NewAccountRate)
	}
	if len(cfg.Accounts) == 0 && cfg.NewAccountRate != 1 {
		return nil, fmt.Errorf("account pool: fixed accounts must be nonempty unless new account rate is exactly 1")
	}
	for i, account := range cfg.Accounts {
		if account == nil {
			return nil, fmt.Errorf("account pool: fixed account at index %d must not be nil", i)
		}
	}
	admitted := *cfg
	admitted.Accounts = append([]*Account(nil), cfg.Accounts...)
	return &accountPool{
		Accounts: admitted.Accounts,
		cfg:      &admitted,
	}, nil
}
