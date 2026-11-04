package generator

import (
	"context"
	"fmt"
	"math/rand"
	"reflect"
	"sort"
	"sync"

	"github.com/paxeer-network/pax-load/types"
)

// WeightedCfg is a configuration for a weighted scenarioGenerator.
type WeightedCfg struct {
	Weight    int
	Generator Generator
}

// WeightedConfig creates a configuration for a weighted scenarioGenerator.
func WeightedConfig(weight int, generator Generator) *WeightedCfg {
	return &WeightedCfg{
		Weight:    weight,
		Generator: generator,
	}
}

type weightedGenerator struct {
	generators []Generator
	cumulative []int64
	total      int64
	mx         sync.RWMutex
	counter    int64
}

// GenerateInfinite generates transactions indefinitely.
func (w *weightedGenerator) GenerateInfinite(ctx context.Context) <-chan *types.LoadTx {
	output := make(chan *types.LoadTx, 10000)
	go func() {
		defer close(output)
		for ctx.Err() == nil {
			select {
			case <-ctx.Done():
				return
			default:
				select {
				case <-ctx.Done():
					return
				case output <- func() *types.LoadTx {
					tx, _ := w.nextGenerator().Generate()
					return tx
				}():
				}
			}
		}
	}()
	return output
}

func (w *weightedGenerator) nextGenerator() Generator {
	w.mx.Lock()
	defer w.mx.Unlock()
	slot := w.counter
	w.counter = (w.counter + 1) % w.total
	index := sort.Search(len(w.cumulative), func(i int) bool { return slot < w.cumulative[i] })
	return w.generators[index]
}

// GenerateN generates n transactions.
func (w *weightedGenerator) GenerateN(n int) []*types.LoadTx {
	txs := make([]*types.LoadTx, 0, n)
	for range n {
		if tx, ok := w.Generate(); ok {
			txs = append(txs, tx)
		} else {
			break // Generator is done
		}
	}
	return txs
}

// Generate generates 1 transaction.
func (w *weightedGenerator) Generate() (*types.LoadTx, bool) {
	return w.nextGenerator().Generate()
}

// GetAccountPools returns all account pools from underlying generators
func (w *weightedGenerator) GetAccountPools() []types.AccountPool {
	w.mx.RLock()
	defer w.mx.RUnlock()

	var allPools []types.AccountPool
	seen := make(map[types.AccountPool]bool)
	for _, gen := range w.generators {
		for _, pool := range gen.GetAccountPools() {
			if !seen[pool] {
				seen[pool] = true
				allPools = append(allPools, pool)
			}
		}
	}
	return allPools
}

// MaxTotalWeight bounds arithmetic and the length of a weighted selection cycle.
const MaxTotalWeight int64 = 1<<31 - 1

func addWeight(total int64, weight int) (int64, error) {
	if weight < 0 || int64(weight) > MaxTotalWeight-total {
		return 0, fmt.Errorf("weight %d is negative or total exceeds %d", weight, MaxTotalWeight)
	}
	return total + int64(weight), nil
}

// NewWeightedGenerator admits nonnegative weights with a bounded positive total.
// Zero weights disable a generator, including in a single-entry configuration.
func NewWeightedGenerator(cfgs ...*WeightedCfg) (Generator, error) {
	var total int64
	for i, cfg := range cfgs {
		if cfg == nil || cfg.Generator == nil {
			return nil, fmt.Errorf("weighted configuration %d has no generator", i)
		}
		value := reflect.ValueOf(cfg.Generator)
		if value.Kind() == reflect.Pointer && value.IsNil() {
			return nil, fmt.Errorf("weighted configuration %d has a nil generator", i)
		}
		var err error
		total, err = addWeight(total, cfg.Weight)
		if err != nil {
			return nil, fmt.Errorf("weighted configuration %d: %w", i, err)
		}
	}
	if total == 0 {
		return nil, fmt.Errorf("weights must have a positive total")
	}
	w := &weightedGenerator{total: total, counter: rand.Int63n(total)}
	var cumulative int64
	for _, cfg := range cfgs {
		if cfg.Weight == 0 {
			continue
		}
		cumulative += int64(cfg.Weight)
		w.generators = append(w.generators, cfg.Generator)
		w.cumulative = append(w.cumulative, cumulative)
	}
	return w, nil
}
