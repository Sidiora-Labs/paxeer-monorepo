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

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/decode"
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
	args := []any{int64(through), after}
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
	m := receipt.DiscoveredMarket{Token: common.HexToAddress(token), Pool: common.HexToAddress(pool), Height: uint64(height), TransactionIndex: uint(index)}
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
	addresses, pinned, err := s.marketAddresses(ctx, from, to)
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
		if block.Height < 0 || strings.ToLower(pinned[uint64(block.Height)].Hash.Hex()) != block.Hash {
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

// Read every receipt, including opaque emitters, before choosing the bounded
// authority. Filtering by an authority's previous discoveries would omit old
// pools after restart. The source pins the consensus block; receipts and signed
// transactions are checked by the same identity rules as receipt qualification.
func (s *recoveringQualifiedSource) marketAddresses(ctx context.Context, from, to uint64) ([]common.Address, map[uint64]source.BlockRecord, error) {
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
	totalBytes, totalLogs, totalTransactions := 0, 0, 0
	for height := from; height <= to; height++ {
		var raw json.RawMessage
		if err := s.client.Client().CallContext(ctx, &raw, "eth_getBlockByNumber", hexutil.EncodeUint64(height), true); err != nil {
			return nil, nil, err
		}
		var header types.Header
		var body struct {
			Hash         common.Hash
			Transactions []json.RawMessage
		}
		if json.Unmarshal(raw, &header) != nil || json.Unmarshal(raw, &body) != nil || header.Number == nil || !header.Number.IsUint64() || header.Number.Uint64() != height || body.Hash == (common.Hash{}) || header.Time == 0 || header.Time > math.MaxInt64/1000 || len(body.Transactions) > 4096 {
			return nil, nil, receipt.ErrBinding
		}
		block := source.BlockRecord{ChainID: 125, Height: height, Hash: body.Hash, ParentHash: header.ParentHash, TimestampSeconds: int64(header.Time), TransactionCount: len(body.Transactions)}
		if height > from && block.ParentHash != blocks[height-1].Hash {
			return nil, nil, receipt.ErrBinding
		}
		blocks[height] = block
		if s.blockIdentity != receipt.BlockIdentityReported && header.Hash() != block.Hash {
			return nil, nil, receipt.ErrBinding
		}
		totalTransactions += len(body.Transactions)
		if totalTransactions > 4096 {
			return nil, nil, receipt.ErrQualification
		}
		transactions := make(types.Transactions, len(body.Transactions))
		for i, rawTx := range body.Transactions {
			var tx types.Transaction
			var meta struct {
				Hash             *common.Hash
				BlockHash        *common.Hash
				TransactionIndex *hexutil.Uint64
			}
			if tx.UnmarshalJSON(rawTx) != nil || json.Unmarshal(rawTx, &meta) != nil || meta.Hash == nil || *meta.Hash != tx.Hash() || meta.BlockHash != nil && *meta.BlockHash != block.Hash || meta.TransactionIndex != nil && uint64(*meta.TransactionIndex) != uint64(i) {
				return nil, nil, receipt.ErrBinding
			}
			transactions[i] = &tx
		}
		if s.blockIdentity != receipt.BlockIdentityReported && types.DeriveSha(transactions, trie.NewStackTrie(nil)) != header.TxHash {
			return nil, nil, receipt.ErrBinding
		}
		receipts, err := s.client.BlockReceipts(ctx, rpc.BlockNumberOrHashWithHash(block.Hash, true))
		if err != nil {
			var rpcErr rpc.Error
			if !errors.As(err, &rpcErr) || rpcErr.ErrorCode() != -32601 {
				return nil, nil, err
			}
			receipts = make(types.Receipts, len(transactions))
			for i, tx := range transactions {
				receipts[i], err = s.client.TransactionReceipt(ctx, tx.Hash())
				if err != nil {
					return nil, nil, err
				}
			}
		}
		if len(receipts) != len(transactions) {
			return nil, nil, receipt.ErrBinding
		}
		logIndex := uint(0)
		for i, r := range receipts {
			if r == nil || r.BlockNumber == nil || r.BlockNumber.Cmp(header.Number) != 0 || r.BlockHash != block.Hash || r.TxHash != transactions[i].Hash() || r.TransactionIndex != uint(i) || r.Type != transactions[i].Type() || r.Status > 1 || len(r.Logs) > 4096 {
				return nil, nil, receipt.ErrBinding
			}
			for _, log := range r.Logs {
				if log == nil || log.Removed || log.BlockHash != block.Hash || log.BlockNumber != height || log.TxHash != r.TxHash || log.TxIndex != uint(i) || log.Index != logIndex || len(log.Topics) > 4 || len(log.Data) > decode.MaxFieldBytes {
					return nil, nil, receipt.ErrBinding
				}
				logIndex++
				totalLogs++
				totalBytes += 256 + len(log.Data) + 32*len(log.Topics)
				if totalLogs > 65536 || totalBytes > 32<<20 {
					return nil, nil, receipt.ErrQualification
				}
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
			if r.Bloom != types.CreateBloom(r) {
				return nil, nil, receipt.ErrBinding
			}
		}
		if types.MergeBloom(receipts) != header.Bloom || s.blockIdentity != receipt.BlockIdentityReported && types.DeriveSha(types.Receipts(receipts), trie.NewStackTrie(nil)) != header.ReceiptHash {
			return nil, nil, receipt.ErrBinding
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
