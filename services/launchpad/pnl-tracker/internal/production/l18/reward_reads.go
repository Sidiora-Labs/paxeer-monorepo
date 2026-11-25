package l18

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rpc"
)

type rewardProgressKey struct{}
type rewardProgress struct {
	markets int
	group   string
}

func rewardStage(ctx context.Context, markets int, group string) {
	if p, ok := ctx.Value(rewardProgressKey{}).(*rewardProgress); ok {
		p.markets, p.group = markets, group
	}
}

func logRewardTimeout(ctx context.Context, p *rewardProgress, err error) {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
		slog.ErrorContext(ctx, "reward request timed out", "markets", p.markets, "group", p.group)
	}
}

func nodeCall(ctx context.Context, r RPC, result any, method string, args ...any) error {
	bounded, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return r.CallContext(bounded, result, method, args...)
}

func nodeBatch(ctx context.Context, r batchRPC, batch []rpc.BatchElem) error {
	bounded, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return r.BatchCallContext(bounded, batch)
}

type rewardRead struct {
	target string
	abi    abi.ABI
	method string
	arg    any
}

func (c *Creator) readGroup(ctx context.Context, b Checkpoint, reads []rewardRead) ([]any, error) {
	values := make([]any, len(reads))
	if len(reads) == 0 {
		return values, nil
	}
	if batcher, ok := c.rpc.(batchRPC); ok {
		for start := 0; start < len(reads); start += 100 {
			end := min(start+100, len(reads))
			results := make([]hexutil.Bytes, end-start)
			batch := make([]rpc.BatchElem, end-start)
			for i, read := range reads[start:end] {
				data, err := read.abi.Pack(read.method, read.arg)
				if err != nil {
					return nil, err
				}
				batch[i] = rpc.BatchElem{Method: "eth_call", Args: []any{
					map[string]any{"to": read.target, fieldData: hexutil.Encode(data)},
					map[string]any{fieldBlockHash: b.Hash, fieldRequireCanonical: true},
				}, Result: &results[i]}
			}
			if err := nodeBatch(ctx, batcher, batch); err != nil {
				return nil, err
			}
			for i, read := range reads[start:end] {
				if batch[i].Error != nil {
					return nil, batch[i].Error
				}
				decoded, err := read.abi.Unpack(read.method, results[i])
				if err != nil || len(decoded) != 1 {
					return nil, ErrUnavailable
				}
				values[start+i] = decoded[0]
			}
		}
		return values, nil
	}
	if len(reads) <= 3 {
		for i, read := range reads {
			value, err := c.read(ctx, b, read.target, read.abi, read.method, read.arg)
			if err != nil {
				return nil, err
			}
			values[i] = value
		}
		return values, nil
	}
	group, cancel := context.WithCancel(ctx)
	defer cancel()
	var wg sync.WaitGroup
	var once sync.Once
	var first error
	jobs := make(chan int)
	for worker := 0; worker < min(8, len(reads)); worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				if group.Err() != nil {
					return
				}
				read := reads[i]
				value, err := c.read(group, b, read.target, read.abi, read.method, read.arg)
				if err != nil {
					once.Do(func() { first = err; cancel() })
					return
				}
				values[i] = value
			}
		}()
	}
send:
	for i := range reads {
		select {
		case jobs <- i:
		case <-group.Done():
			break send
		}
	}
	close(jobs)
	wg.Wait()
	if first != nil {
		return nil, first
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return values, nil
}

func (c *Creator) previewPage(ctx context.Context, b Checkpoint, markets []marketSource) ([]FeePreview, error) {
	rewardStage(ctx, len(markets), "nft")
	first := make([]rewardRead, 0, 3*len(markets))
	for _, market := range markets {
		id, err := amount(market.NFTID)
		if err != nil {
			return nil, err
		}
		for _, method := range []string{"ownerOf", "getPoolAddress", "getFeeStrategy"} {
			first = append(first, rewardRead{NFT, nft, method, id})
		}
	}
	tokens, err := c.readGroup(ctx, b, first)
	if err != nil {
		return nil, err
	}
	rewardStage(ctx, len(markets), "fees")
	second := make([]rewardRead, 0, 2*len(markets))
	for i := range markets {
		pool := tokens[3*i+1].(common.Address)
		for _, method := range []string{"supportsAirdrop", "getAccumulatedFees"} {
			second = append(second, rewardRead{Accumulator, accumulator, method, pool})
		}
	}
	fees, err := c.readGroup(ctx, b, second)
	if err != nil {
		return nil, err
	}
	out := make([]FeePreview, len(markets))
	for i, market := range markets {
		out[i] = FeePreview{
			NFTID: market.NFTID, Owner: strings.ToLower(tokens[3*i].(common.Address).Hex()),
			Pool: strings.ToLower(tokens[3*i+1].(common.Address).Hex()), HistoricalCreator: market.Creator,
			ClaimableRaw: fees[2*i+1].(*big.Int).String(), Asset: USDL,
			Capability: StrategyCapability(tokens[3*i+2].(uint8), fees[2*i].(bool)), Block: b, Version: RuleVersion,
		}
	}
	rewardStage(ctx, len(markets), "response")
	return out, nil
}

func validateRewardTimeout(timeout time.Duration) error {
	if timeout < time.Second || timeout > 15*time.Second {
		return fmt.Errorf("rewards: request timeout must be between one and fifteen seconds")
	}
	return nil
}

// Request runtime checks share the page's concurrency bound on transports that
// expose only single calls. Results retain their original validation order.
type rewardRuntimeRPC struct{ RPC }

func (r rewardRuntimeRPC) BatchCallContext(ctx context.Context, batch []rpc.BatchElem) error {
	group, cancel := context.WithCancel(ctx)
	defer cancel()
	var wg sync.WaitGroup
	jobs := make(chan int)
	for worker := 0; worker < min(8, len(batch)); worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				if group.Err() != nil {
					return
				}
				batch[i].Error = nodeCall(group, r.RPC, batch[i].Result, batch[i].Method, batch[i].Args...)
			}
		}()
	}
send:
	for i := range batch {
		select {
		case jobs <- i:
		case <-group.Done():
			break send
		}
	}
	close(jobs)
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return err
	}
	return nil
}
