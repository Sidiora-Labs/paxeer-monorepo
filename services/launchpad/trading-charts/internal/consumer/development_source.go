package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"strings"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/engine"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DevelopmentSource recovers committed development indexer rows after startup
// and missed Redis publications. Receipts supply the indexes absent from those
// legacy rows; only the resulting complete source frames reach the builder.
type DevelopmentSource struct {
	pool    *pgxpool.Pool
	rpc     *rpc.Client
	builder *engine.Builder
	logger  *slog.Logger
}

func NewDevelopmentSource(pool *pgxpool.Pool, client *rpc.Client, builder *engine.Builder, logger *slog.Logger) *DevelopmentSource {
	return &DevelopmentSource{pool: pool, rpc: client, builder: builder, logger: logger}
}

func (sc *SwapConsumer) SetDevelopmentSource(source *DevelopmentSource) { sc.source = source }

type developmentSourceRow struct {
	poolID, pool, sender, amountIn, amountOut, fee, price, hash string
	buy                                                         bool
	height, timestamp                                           int64
	logIndex                                                    int64
}

func (s *DevelopmentSource) frame(ctx context.Context, row developmentSourceRow) (DevelopmentFrame, error) {
	var receipt *struct {
		Status           string          `json:"status"`
		BlockNumber      hexutil.Uint64  `json:"blockNumber"`
		BlockHash        string          `json:"blockHash"`
		TransactionHash  string          `json:"transactionHash"`
		TransactionIndex *hexutil.Uint64 `json:"transactionIndex"`
		Logs             []struct {
			Topics   []string        `json:"topics"`
			Data     hexutil.Bytes   `json:"data"`
			LogIndex *hexutil.Uint64 `json:"logIndex"`
			Removed  bool            `json:"removed"`
		} `json:"logs"`
	}
	if err := s.rpc.CallContext(ctx, &receipt, "eth_getTransactionReceipt", row.hash); err != nil {
		return DevelopmentFrame{}, err
	}
	if receipt == nil || receipt.Status != "0x1" || receipt.TransactionIndex == nil || uint64(receipt.BlockNumber) != uint64(row.height) || !strings.EqualFold(receipt.TransactionHash, row.hash) || len(receipt.BlockHash) != 66 {
		return DevelopmentFrame{}, errors.New("consumer: development receipt position unavailable")
	}
	var block *struct {
		Hash      string         `json:"hash"`
		Number    hexutil.Uint64 `json:"number"`
		Timestamp hexutil.Uint64 `json:"timestamp"`
	}
	if err := s.rpc.CallContext(ctx, &block, "eth_getBlockByNumber", hexutil.EncodeUint64(uint64(row.height)), false); err != nil {
		return DevelopmentFrame{}, err
	}
	if block == nil || block.Number != receipt.BlockNumber || !strings.EqualFold(block.Hash, receipt.BlockHash) || uint64(block.Timestamp) != uint64(row.timestamp) {
		return DevelopmentFrame{}, errors.New("consumer: development block authority mismatch")
	}
	for _, log := range receipt.Logs {
		if log.LogIndex == nil || uint64(*log.LogIndex) != uint64(row.logIndex) {
			continue
		}
		if log.Removed || len(log.Topics) != 3 || !strings.EqualFold(log.Topics[0], swapEventTopic.Hex()) || !strings.EqualFold(log.Topics[1], row.poolID) || len(log.Topics[2]) != 66 || !strings.EqualFold("0x"+log.Topics[2][26:], row.sender) || len(log.Data) != 7*32 {
			return DevelopmentFrame{}, errors.New("consumer: development source is not the indexed Swap")
		}
		values := []string{"0", row.amountIn, row.amountOut, row.fee, row.price, fmt.Sprint(row.timestamp), fmt.Sprint(row.height)}
		if row.buy {
			values[0] = "1"
		}
		for i, want := range values {
			if new(big.Int).SetBytes(log.Data[i*32:(i+1)*32]).String() != want {
				return DevelopmentFrame{}, errors.New("consumer: development Swap differs from receipt")
			}
		}
		height, txIndex, logIndex := uint64(receipt.BlockNumber), uint64(*receipt.TransactionIndex), uint64(*log.LogIndex)
		return DevelopmentFrame{EventName: "Swap", BlockNumber: &height, TransactionIndex: &txIndex, LogIndex: &logIndex, BlockTimestamp: row.timestamp, TxHash: strings.ToLower(row.hash), Args: map[string]any{
			"poolId": strings.ToLower(row.poolID), "poolAddress": strings.ToLower(row.pool), "sender": strings.ToLower(row.sender), "isBuy": row.buy,
			"amountIn": row.amountIn, "amountOut": row.amountOut, "fee": row.fee, "price": row.price,
		}}, nil
	}
	return DevelopmentFrame{}, errors.New("consumer: development Swap absent from receipt")
}

// Project completes and consumes one bounded batch, in source block/log order.
// Durable claims are the recovery cursor; they commit with every candle fold.
func (s *DevelopmentSource) Project(ctx context.Context) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if s.rpc == nil {
		return 0, errors.New("consumer: development source RPC required")
	}
	rows, err := s.pool.Query(ctx, `SELECT w.pool_id,w.pool_address,w.sender,w.is_buy,w.amount_in,w.amount_out,w.fee,w.price,w.block_number,w.block_timestamp,w.tx_hash,w.log_index
        FROM indexer.swaps w WHERE w.block_number <= (SELECT last_processed_block FROM indexer.cursors WHERE chain_id=125)
        AND NOT EXISTS (SELECT 1 FROM candles.processed_swaps p WHERE p.tx_hash=w.tx_hash AND p.log_index=w.log_index)
        ORDER BY w.block_number,w.log_index LIMIT 100`)
	if err != nil {
		return 0, err
	}
	var batch []developmentSourceRow
	for rows.Next() {
		var row developmentSourceRow
		if err = rows.Scan(&row.poolID, &row.pool, &row.sender, &row.buy, &row.amountIn, &row.amountOut, &row.fee, &row.price, &row.height, &row.timestamp, &row.hash, &row.logIndex); err != nil {
			rows.Close()
			return 0, err
		}
		batch = append(batch, row)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return 0, err
	}
	for i, row := range batch {
		frame, err := s.frame(ctx, row)
		if err != nil {
			return i, err
		}
		wire, err := json.Marshal(frame)
		if err != nil {
			return i, err
		}
		swap, err := DecodeDevelopmentSwap(wire)
		if err != nil {
			return i, err
		}
		if err = s.builder.ProcessSwap(ctx, swap); err != nil {
			return i, err
		}
	}
	// Recovery also drains a commit whose Redis acknowledgement was lost.
	pending, err := s.pool.Query(ctx, `SELECT key FROM candles.development_state WHERE starts_with(key,'outbox/') AND payload<>convert_to('[]','UTF8') LIMIT 100`)
	if err != nil {
		return len(batch), err
	}
	var pools []string
	for pending.Next() {
		var key string
		if err = pending.Scan(&key); err != nil {
			pending.Close()
			return len(batch), err
		}
		var parts []string
		if err = json.Unmarshal([]byte(strings.TrimPrefix(key, "outbox/")), &parts); err != nil || len(parts) != 1 {
			pending.Close()
			return len(batch), errors.New("consumer: invalid development outbox key")
		}
		pools = append(pools, parts[0])
	}
	pending.Close()
	if err = pending.Err(); err != nil {
		return len(batch), err
	}
	for _, pool := range pools {
		if err = s.builder.RelayCommitted(ctx, pool); err != nil {
			return len(batch), err
		}
	}
	return len(batch), nil
}

func (s *DevelopmentSource) Run(ctx context.Context) {
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	last := ""
	for {
		_, err := s.Project(ctx)
		message := ""
		if err != nil {
			message = err.Error()
		}
		if message != "" && message != last && ctx.Err() == nil {
			s.logger.Warn("development chart source degraded", slog.String("err", message))
		}
		last = message
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}
