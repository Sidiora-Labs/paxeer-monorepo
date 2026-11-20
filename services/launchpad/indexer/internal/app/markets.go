package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/checked"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/receipt"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/source"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/jackc/pgx/v5/pgxpool"
)

const marketRecoveryPageSize = 128
const marketHintLimit = 65536

type marketInventory interface {
	Page(context.Context, []common.Address, uint64, string) ([]receipt.DiscoveredMarket, error)
}

type postgresMarketInventory struct{ pool *pgxpool.Pool }

// Pages are selected by addresses referenced by the current operation, never
// by lifetime count. The token cursor preserves each durable creation identity.
func marketPageQuery(addresses []common.Address, through uint64, after string) (string, []any) {
	args := make([]any, 0, 2+len(addresses))
	args = append(args, int64(through), after) //nolint:gosec // Page bounds the height to the signed database range
	parameters := make([]string, len(addresses))
	for i, address := range addresses {
		args = append(args, strings.ToLower(address.Hex()))
		parameters[i] = fmt.Sprintf("$%d", i+3)
	}
	in := strings.Join(parameters, ",")
	return `SELECT token, pool, height, transaction_index FROM indexer.indexer_canonical_creations WHERE height <= $1 AND token > $2 AND (token IN (` + in + `) OR pool IN (` + in + `)) ORDER BY token LIMIT 128`, args
}

func persistedMarket(token, pool string, height, index int64) (receipt.DiscoveredMarket, error) {
	if !common.IsHexAddress(token) || !common.IsHexAddress(pool) || height < 0 || index < 0 || index > math.MaxUint32 {
		return receipt.DiscoveredMarket{}, errors.New("indexer: persisted canonical creation invalid")
	}
	heightUint64, conversionErr := checked.Convert[uint64](height)
	if conversionErr != nil {
		return receipt.DiscoveredMarket{}, conversionErr
	}
	indexUint, conversionErr := checked.Convert[uint](index)
	if conversionErr != nil {
		return receipt.DiscoveredMarket{}, conversionErr
	}
	m := receipt.DiscoveredMarket{Token: common.HexToAddress(token), Pool: common.HexToAddress(pool), Height: heightUint64, TransactionIndex: indexUint}
	if m.Token == (common.Address{}) || m.Pool == (common.Address{}) || m.Token == m.Pool || token != strings.ToLower(m.Token.Hex()) || pool != strings.ToLower(m.Pool.Hex()) {
		return receipt.DiscoveredMarket{}, errors.New("indexer: persisted canonical market identity invalid")
	}
	return m, nil
}

func (s postgresMarketInventory) Page(ctx context.Context, addresses []common.Address, through uint64, after string) ([]receipt.DiscoveredMarket, error) {
	if s.pool == nil || len(addresses) == 0 || len(addresses) > marketRecoveryPageSize || through > math.MaxInt64 {
		return nil, receipt.ErrQualification
	}
	query, args := marketPageQuery(addresses, through, after)
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	markets := make([]receipt.DiscoveredMarket, 0, marketRecoveryPageSize)
	for rows.Next() {
		var token, pool string
		var height, index int64
		if err = rows.Scan(&token, &pool, &height, &index); err != nil {
			return nil, err
		}
		market, err := persistedMarket(token, pool, height, index)
		if err != nil {
			return nil, err
		}
		markets = append(markets, market)
	}
	return markets, rows.Err()
}

func recoverBatchMarkets(ctx context.Context, inventory marketInventory, addresses []common.Address, through uint64) ([]receipt.DiscoveredMarket, error) {
	if inventory == nil || len(addresses) > marketHintLimit || through > math.MaxInt64 {
		return nil, receipt.ErrQualification
	}
	markets := []receipt.DiscoveredMarket{}
	known := map[common.Address]receipt.DiscoveredMarket{}
	for start := 0; start < len(addresses); start += marketRecoveryPageSize {
		chunk := addresses[start:min(start+marketRecoveryPageSize, len(addresses))]
		after := ""
		for {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			page, err := inventory.Page(ctx, chunk, through, after)
			if err != nil {
				return nil, err
			}
			if len(page) > marketRecoveryPageSize {
				return nil, receipt.ErrQualification
			}
			for _, market := range page {
				token := strings.ToLower(market.Token.Hex())
				if token <= after || market.Height > through {
					return nil, receipt.ErrQualification
				}
				after = token
				if previous, ok := known[market.Token]; ok {
					if previous != market {
						return nil, receipt.ErrQualification
					}
					continue
				}
				// This is Rehydrate's existing per-call admission limit. A dense
				// range can be bisected; old, inactive markets consume no slots.
				if len(markets) == 2048 {
					return nil, receipt.ErrQualification
				}
				known[market.Token] = market
				markets = append(markets, market)
			}
			if len(page) < marketRecoveryPageSize {
				break
			}
		}
	}
	return markets, nil
}

type recoveringQualifiedSource struct {
	inventory     marketInventory
	client        *ethclient.Client
	newAuthority  func(context.Context) (*receipt.RPCQualification, error)
	history       *source.HistoricalSource
	durableHeight func() uint64
	priorHash     func(context.Context, uint64) (common.Hash, error)
	blockIdentity string
	historical    bool
	reserves      *recoveringReserveReader
}

func (s *recoveringQualifiedSource) FetchQualifiedBatch(ctx context.Context, from, to uint64) (store.CanonicalBatch, []*receipt.QualifiedReceipt, error) {
	if s == nil || s.history == nil || s.history.RPC == nil || s.client == nil || s.newAuthority == nil || s.durableHeight == nil || from > to || to > math.MaxInt64 || to-from >= 64 {
		return store.CanonicalBatch{}, nil, receipt.ErrQualification
	}
	durable := s.durableHeight()
	if !s.historical && (durable == math.MaxUint64 || from != durable+1) {
		return store.CanonicalBatch{}, nil, receipt.ErrBinding
	}
	authority, err := s.newAuthority(ctx)
	if err != nil {
		return store.CanonicalBatch{}, nil, err
	}
	addresses, pinned, err := s.marketAddressesWithAuthority(ctx, authority, from, to)
	if err != nil {
		return store.CanonicalBatch{}, nil, err
	}
	markets, err := recoverBatchMarkets(ctx, s.inventory, addresses, to)
	if err != nil {
		return store.CanonicalBatch{}, nil, err
	}
	if err = authority.Rehydrate(ctx, markets); err != nil {
		return store.CanonicalBatch{}, nil, fmt.Errorf("indexer: recover batch markets: %w", err)
	}
	var batch store.CanonicalBatch
	var qualified []*receipt.QualifiedReceipt
	if s.historical {
		batch, qualified, err = (&source.HistoricalQualifiedSource{History: s.history, Authority: authority, DurableHeight: s.durableHeight}).FetchQualifiedBatch(ctx, from, to)
	} else {
		batch, qualified, err = (&source.LiveQualifiedSource{RPC: s.history.RPC, Authority: authority, DurableHeight: s.durableHeight, PriorHash: s.priorHash}).FetchQualifiedBatch(ctx, from, to)
	}
	if err != nil {
		return store.CanonicalBatch{}, nil, err
	}
	if len(batch.Blocks) != len(pinned) {
		return store.CanonicalBatch{}, nil, receipt.ErrBinding
	}
	for _, block := range batch.Blocks {
		blockHeightUint64, conversionErr := checked.Convert[uint64](block.Height)
		if conversionErr != nil {
			return store.CanonicalBatch{}, nil, conversionErr
		}
		if block.Height < 0 || strings.ToLower(pinned[blockHeightUint64].Hash.Hex()) != block.Hash {
			return store.CanonicalBatch{}, nil, receipt.ErrBinding
		}
	}
	if s.reserves != nil {
		s.reserves.mu.Lock()
		s.reserves.active = authority
		s.reserves.mu.Unlock()
	}
	return batch, qualified, nil
}

// marketHeightRead contains the complete network response for one height.
// Requests are collected concurrently, while admission checks below consume
// these entries in height order.
type marketHeightRead struct {
	header       types.Header
	hash         common.Hash
	transactions types.Transactions
	receipts     types.Receipts
	rawTxs       []json.RawMessage
}

// Read every receipt, including opaque emitters, before choosing the bounded
// authority. Filtering by an authority's previous discoveries would omit old
// pools after restart. Network reads are bounded by the historical source
// concurrency and are cancelled as soon as one read fails.
func (s *recoveringQualifiedSource) marketAddresses(ctx context.Context, from, to uint64) ([]common.Address, map[uint64]source.BlockRecord, error) {
	return s.marketAddressesWithAuthority(ctx, nil, from, to)
}

func (s *recoveringQualifiedSource) marketAddressesWithAuthority(ctx context.Context, authority *receipt.RPCQualification, from, to uint64) ([]common.Address, map[uint64]source.BlockRecord, error) {
	if authority != nil {
		reads, err := authority.ReadBatch(ctx, from, to)
		if err != nil {
			if errors.Is(err, receipt.ErrQualification) {
				return nil, nil, receipt.ErrBinding
			}
			return nil, nil, err
		}
		return s.validateMarketReads(ctx, from, to, readsToMarketReads(reads))
	}
	if s == nil || s.client == nil || from > to || to-from >= 64 {
		return nil, nil, receipt.ErrQualification
	}
	concurrency := 1
	if s.history != nil && s.history.RPC != nil {
		concurrency = s.history.RPC.HistoricalConcurrency()
	}
	reads, err := s.readMarketHeights(ctx, from, to, concurrency)
	if err != nil {
		return nil, nil, err
	}
	return s.validateMarketReads(ctx, from, to, reads)
}

func readsToMarketReads(reads []receipt.BatchRead) []marketHeightRead {
	out := make([]marketHeightRead, len(reads))
	for i, read := range reads {
		header := types.Header{}
		if read.Header != nil {
			header = *read.Header
		}
		out[i] = marketHeightRead{header: header, hash: read.Hash, transactions: read.Transactions, receipts: read.Receipts}
	}
	return out
}

func (s *recoveringQualifiedSource) readMarketHeights(ctx context.Context, from, to uint64, concurrency int) ([]marketHeightRead, error) {
	if from > to || to-from >= 64 {
		return nil, receipt.ErrQualification
	}
	count, err := checked.Convert[int](to - from)
	if err != nil {
		return nil, err
	}
	count++
	return source.ParallelBatchRead(ctx, count, concurrency, func(readCtx context.Context, index int) (marketHeightRead, error) {
		offset, err := checked.Convert[uint64](index)
		if err != nil {
			return marketHeightRead{}, err
		}
		return s.readMarketHeight(readCtx, from+offset)
	})
}

func (s *recoveringQualifiedSource) readMarketHeight(ctx context.Context, height uint64) (marketHeightRead, error) {
	var raw json.RawMessage
	if err := s.client.Client().CallContext(ctx, &raw, "eth_getBlockByNumber", hexutil.EncodeUint64(height), true); err != nil {
		return marketHeightRead{}, err
	}
	var header types.Header
	var body struct {
		Hash         common.Hash
		Transactions []json.RawMessage
	}
	if json.Unmarshal(raw, &header) != nil || json.Unmarshal(raw, &body) != nil || header.Number == nil || !header.Number.IsUint64() || header.Number.Uint64() != height || body.Hash == (common.Hash{}) || header.Time == 0 || header.Time > math.MaxInt64/1000 || len(body.Transactions) > 4096 {
		return marketHeightRead{}, receipt.ErrBinding
	}
	transactions := make(types.Transactions, len(body.Transactions))
	for i, rawTx := range body.Transactions {
		var tx types.Transaction
		if tx.UnmarshalJSON(rawTx) != nil {
			return marketHeightRead{}, receipt.ErrBinding
		}
		transactions[i] = &tx
	}
	receipts, err := s.client.BlockReceipts(ctx, rpc.BlockNumberOrHashWithHash(body.Hash, true))
	if err != nil {
		var rpcErr rpc.Error
		if !errors.As(err, &rpcErr) || rpcErr.ErrorCode() != -32601 {
			return marketHeightRead{}, err
		}
		receipts = make(types.Receipts, len(transactions))
		for i, tx := range transactions {
			receipts[i], err = s.client.TransactionReceipt(ctx, tx.Hash())
			if err != nil {
				return marketHeightRead{}, err
			}
		}
	}
	return marketHeightRead{header: header, hash: body.Hash, transactions: transactions, receipts: types.Receipts(receipts), rawTxs: body.Transactions}, nil
}

func (s *recoveringQualifiedSource) validateMarketReads(ctx context.Context, from, to uint64, reads []marketHeightRead) ([]common.Address, map[uint64]source.BlockRecord, error) {
	blocks := map[uint64]source.BlockRecord{}
	addresses := map[common.Address]bool{}
	add := func(word []byte) error {
		if len(word) == 32 {
			if !bytes.Equal(word[:12], make([]byte, 12)) {
				return nil
			}
			word = word[12:]
		}
		address := common.BytesToAddress(word)
		if address == (common.Address{}) {
			return nil
		}
		if !addresses[address] && len(addresses) == marketHintLimit {
			return receipt.ErrQualification
		}
		addresses[address] = true
		return nil
	}
	if from > to || to-from >= 64 {
		return nil, nil, receipt.ErrBinding
	}
	count, err := checked.Convert[int](to - from)
	if err != nil || len(reads) != count+1 {
		return nil, nil, receipt.ErrBinding
	}
	totalBytes, totalLogs, totalTransactions := 0, 0, 0
	for index, read := range reads {
		offset, err := checked.Convert[uint64](index)
		if err != nil {
			return nil, nil, err
		}
		height := from + offset
		header := read.header
		block := source.BlockRecord{ChainID: 125, Height: height, Hash: read.hash, ParentHash: header.ParentHash, TimestampSeconds: 0, TransactionCount: len(read.transactions)}
		headerTimeInt64, conversionErr := checked.Convert[int64](header.Time)
		if conversionErr != nil || header.Number == nil || !header.Number.IsUint64() || header.Number.Uint64() != height || read.hash == (common.Hash{}) || header.Time == 0 || header.Time > math.MaxInt64/1000 || len(read.transactions) > 4096 {
			return nil, nil, receipt.ErrBinding
		}
		block.TimestampSeconds = headerTimeInt64
		if height > from && block.ParentHash != blocks[height-1].Hash {
			return nil, nil, receipt.ErrBinding
		}
		blocks[height] = block
		if s.blockIdentity != receipt.BlockIdentityReported && header.Hash() != block.Hash {
			return nil, nil, receipt.ErrBinding
		}
		totalTransactions += len(read.transactions)
		if totalTransactions > 4096 {
			return nil, nil, receipt.ErrQualification
		}
		if len(read.rawTxs) > 0 {
			for i, rawTx := range read.rawTxs {
				var meta struct {
					Hash             *common.Hash
					BlockHash        *common.Hash
					TransactionIndex *hexutil.Uint64
				}
				if json.Unmarshal(rawTx, &meta) != nil || meta.Hash == nil || *meta.Hash != read.transactions[i].Hash() || meta.BlockHash != nil && *meta.BlockHash != block.Hash {
					return nil, nil, receipt.ErrBinding
				}
				if meta.TransactionIndex != nil {
					transactionIndex, conversionErr := checked.Convert[uint64](*meta.TransactionIndex)
					expectedIndex, indexErr := checked.Convert[uint64](i)
					if conversionErr != nil || indexErr != nil || transactionIndex != expectedIndex {
						return nil, nil, receipt.ErrBinding
					}
				}
			}
		}
		if s.blockIdentity != receipt.BlockIdentityReported && types.DeriveSha(read.transactions, trie.NewStackTrie(nil)) != header.TxHash {
			return nil, nil, receipt.ErrBinding
		}
		regularReceipts, shellReceipts, err := receipt.SeparateShellReceipts(read.transactions, read.receipts, block.Hash, header.Number, header.Bloom, header.ReceiptHash, s.blockIdentity == receipt.BlockIdentityReported)
		if err != nil {
			return nil, nil, receipt.ErrBinding
		}
		allReceipts := append(append(types.Receipts(nil), regularReceipts...), shellReceipts...)
		for _, r := range allReceipts {
			if r == nil {
				return nil, nil, receipt.ErrBinding
			}
			totalLogs += len(r.Logs)
			for _, log := range r.Logs {
				if log == nil {
					return nil, nil, receipt.ErrBinding
				}
				totalBytes += 256 + len(log.Data) + 32*len(log.Topics)
			}
		}
		if totalLogs > 65536 || totalBytes > 32<<20 {
			return nil, nil, receipt.ErrQualification
		}
		for _, r := range regularReceipts {
			for _, log := range r.Logs {
				if err = add(log.Address.Bytes()); err != nil {
					return nil, nil, err
				}
				for _, topic := range log.Topics {
					if err = add(topic.Bytes()); err != nil {
						return nil, nil, err
					}
				}
				for offset := 0; offset+32 <= len(log.Data); offset += 32 {
					if err = add(log.Data[offset : offset+32]); err != nil {
						return nil, nil, err
					}
				}
			}
		}
	}
	out := make([]common.Address, 0, len(addresses))
	for address := range addresses {
		out = append(out, address)
	}
	sort.Slice(out, func(i, j int) bool { return bytes.Compare(out[i][:], out[j][:]) < 0 })
	return out, blocks, nil
}

type recoveringReserveReader struct {
	mu           sync.RWMutex
	active       *receipt.RPCQualification
	inventory    marketInventory
	newAuthority func(context.Context) (*receipt.RPCQualification, error)
}

func (s *recoveringReserveReader) DiscoveredEpochs() []receipt.RPCCodeEpoch {
	s.mu.RLock()
	active := s.active
	s.mu.RUnlock()
	return active.DiscoveredEpochs()
}

func (s *recoveringReserveReader) PoolReserves(ctx context.Context, pool common.Address, block common.Hash) (receipt.PoolReserves, error) {
	s.mu.RLock()
	active := s.active
	s.mu.RUnlock()
	if active != nil {
		for _, epoch := range active.DiscoveredEpochs() {
			if epoch.Deployment.Address == pool && epoch.Deployment.Role == "SidioraPool" {
				return active.PoolReserves(ctx, pool, block)
			}
		}
	}
	markets, err := recoverBatchMarkets(ctx, s.inventory, []common.Address{pool}, math.MaxInt64)
	if err != nil {
		return receipt.PoolReserves{}, err
	}
	if len(markets) != 1 || markets[0].Pool != pool {
		return receipt.PoolReserves{}, receipt.ErrQualification
	}
	authority, err := s.newAuthority(ctx)
	if err != nil {
		return receipt.PoolReserves{}, err
	}
	if err = authority.Rehydrate(ctx, markets); err != nil {
		return receipt.PoolReserves{}, err
	}
	return authority.PoolReserves(ctx, pool, block)
}
