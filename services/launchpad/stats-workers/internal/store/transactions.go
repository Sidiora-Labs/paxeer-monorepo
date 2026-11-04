package store

import (
	"context"
	"encoding/json"
	"fmt"
)

// TransactionRow is a stats.pool_transactions row. JSON tags match the TS drizzle
// property names (camelCase) for response parity.
type TransactionRow struct {
	ID             string `json:"id"`
	PoolAddress    string `json:"poolAddress"`
	Sender         string `json:"sender"`
	IsBuy          bool   `json:"isBuy"`
	AmountIn       string `json:"amountIn"`
	AmountOut      string `json:"amountOut"`
	Price          string `json:"price"`
	Fee            string `json:"fee"`
	BlockTimestamp int64  `json:"blockTimestamp"`
	TxHash         string `json:"txHash"`
}

// CrossTokenSwapRow is a stats.cross_token_swaps row (camelCase JSON parity).
type CrossTokenSwapRow struct {
	ID               string `json:"id"`
	Sender           string `json:"sender"`
	TokenIn          string `json:"tokenIn"`
	TokenOut         string `json:"tokenOut"`
	PoolIn           string `json:"poolIn"`
	PoolOut          string `json:"poolOut"`
	AmountIn         string `json:"amountIn"`
	IntermediateUsdl string `json:"intermediateUsdl"`
	AmountOut        string `json:"amountOut"`
	FeeIn            string `json:"feeIn"`
	FeeOut           string `json:"feeOut"`
	BlockTimestamp   int64  `json:"blockTimestamp"`
	TxHash           string `json:"txHash"`
}

// InsertTransaction records a swap in pool_transactions, idempotently keyed on id
// (txHash-logIndex), so webhook redelivery is safe (invariant i9). Ports step 1
// of StatsSwapConsumer.processEvent.
func (s *Store) InsertTransaction(ctx context.Context, t TransactionRow) error {
	_, err := s.RecordTransaction(ctx, t)
	return err
}

// RecordTransaction is InsertTransaction reporting whether the row was newly
// inserted (false when the id already existed). On a WithTx store the row is
// visible to other sessions only once the caller commits.
func (s *Store) RecordTransaction(ctx context.Context, t TransactionRow) (bool, error) {
	tag, err := s.db.Exec(ctx, `
		INSERT INTO stats.pool_transactions
			(id, pool_address, sender, is_buy, amount_in, amount_out, price, fee, block_timestamp, tx_hash)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		ON CONFLICT (id) DO NOTHING`,
		t.ID, t.PoolAddress, t.Sender, t.IsBuy, t.AmountIn, t.AmountOut, t.Price, t.Fee, t.BlockTimestamp, t.TxHash)
	if err != nil {
		return false, fmt.Errorf("store: insert transaction: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// ListTransactions returns a page of a pool's transactions, newest first,
// optionally filtered by side ("buy"/"sell"; anything else means all). Ports the
// GET /stats/:pool/transactions query.
func (s *Store) ListTransactions(ctx context.Context, poolAddress string, limit, offset int, txType string) ([]TransactionRow, error) {
	query := `
		SELECT id, pool_address, sender, is_buy, amount_in, amount_out, price, fee, block_timestamp, tx_hash
		FROM stats.pool_transactions
		WHERE pool_address = $1`
	args := []any{poolAddress}
	switch txType {
	case "buy":
		query += ` AND is_buy = true`
	case "sell":
		query += ` AND is_buy = false`
	}
	query += ` ORDER BY block_timestamp DESC, id DESC LIMIT $2 OFFSET $3`
	args = append(args, limit, offset)

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("store: list transactions: %w", err)
	}
	defer rows.Close()
	return scanTransactions(rows)
}

// CreatorTransactions retains the bounded first-page API for internal callers.
func (s *Store) CreatorTransactions(ctx context.Context, pool, creator string) ([]TransactionRow, error) {
	page, err := s.CreatorTransactionPage(ctx, pool, creator, 100, 0, nil, nil)
	return page.Transactions, err
}

// InsertCrossTokenSwap records a native multihop (token-for-token) swap,
// idempotently. Ports MultihopConsumer.processEvent.
func (s *Store) InsertCrossTokenSwap(ctx context.Context, c CrossTokenSwapRow) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO stats.cross_token_swaps
			(id, sender, token_in, token_out, pool_in, pool_out, amount_in, intermediate_usdl, amount_out, fee_in, fee_out, block_timestamp, tx_hash)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		ON CONFLICT (id) DO NOTHING`,
		c.ID, c.Sender, c.TokenIn, c.TokenOut, c.PoolIn, c.PoolOut, c.AmountIn,
		c.IntermediateUsdl, c.AmountOut, c.FeeIn, c.FeeOut, c.BlockTimestamp, c.TxHash)
	if err != nil {
		return fmt.Errorf("store: insert cross-token swap: %w", err)
	}
	return nil
}

// ListCrossTokenSwapsByWallet returns a wallet's multihop swap history, newest
// first (wallet lower-cased by the caller).
func (s *Store) ListCrossTokenSwapsByWallet(ctx context.Context, walletLower string, limit, offset int) ([]CrossTokenSwapRow, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, sender, token_in, token_out, pool_in, pool_out, amount_in, intermediate_usdl, amount_out, fee_in, fee_out, block_timestamp, tx_hash
		FROM stats.cross_token_swaps
		WHERE sender = $1
		ORDER BY block_timestamp DESC
		LIMIT $2 OFFSET $3`, walletLower, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("store: cross-token swaps by wallet: %w", err)
	}
	defer rows.Close()
	return scanCrossTokenSwaps(rows)
}

// ListCrossTokenSwapsByToken returns all multihop swaps involving a token as
// input or output, newest first (token lower-cased by the caller).
func (s *Store) ListCrossTokenSwapsByToken(ctx context.Context, tokenLower string, limit, offset int) ([]CrossTokenSwapRow, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, sender, token_in, token_out, pool_in, pool_out, amount_in, intermediate_usdl, amount_out, fee_in, fee_out, block_timestamp, tx_hash
		FROM stats.cross_token_swaps
		WHERE token_in = $1 OR token_out = $1
		ORDER BY block_timestamp DESC
		LIMIT $2 OFFSET $3`, tokenLower, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("store: cross-token swaps by token: %w", err)
	}
	defer rows.Close()
	return scanCrossTokenSwaps(rows)
}

func scanTransactions(rows interface {
	Next() bool
	Scan(...any) error
	Err() error
}) ([]TransactionRow, error) {
	out := []TransactionRow{}
	for rows.Next() {
		var t TransactionRow
		if err := rows.Scan(&t.ID, &t.PoolAddress, &t.Sender, &t.IsBuy, &t.AmountIn, &t.AmountOut,
			&t.Price, &t.Fee, &t.BlockTimestamp, &t.TxHash); err != nil {
			return nil, fmt.Errorf("store: scan transaction: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func scanCrossTokenSwaps(rows interface {
	Next() bool
	Scan(...any) error
	Err() error
}) ([]CrossTokenSwapRow, error) {
	out := []CrossTokenSwapRow{}
	for rows.Next() {
		var c CrossTokenSwapRow
		if err := rows.Scan(&c.ID, &c.Sender, &c.TokenIn, &c.TokenOut, &c.PoolIn, &c.PoolOut,
			&c.AmountIn, &c.IntermediateUsdl, &c.AmountOut, &c.FeeIn, &c.FeeOut, &c.BlockTimestamp, &c.TxHash); err != nil {
			return nil, fmt.Errorf("store: scan cross-token swap: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// HistoryPosition is the immutable timestamp/id ordering key of stored history.
type HistoryPosition struct {
	Timestamp int64  `json:"timestamp"`
	ID        string `json:"id"`
}

type CreatorSummary struct {
	BuyCount          int64  `json:"buyCount"`
	SellCount         int64  `json:"sellCount"`
	HasSold           bool   `json:"hasSold"`
	TotalBoughtTokens string `json:"totalBoughtTokens"`
	TotalSoldTokens   string `json:"totalSoldTokens"`
	NetTokenBalance   string `json:"netTokenBalance"`
}
type CreatorPage struct {
	Transactions []TransactionRow
	Summary      CreatorSummary
	Head         *HistoryPosition
}

// CreatorTransactionPage computes totals and the bounded page in one SQL
// statement, so a concurrent append cannot split the summary from its page.
// Subsequent pages retain the first page's upper ordering boundary.
func (s *Store) CreatorTransactionPage(ctx context.Context, pool, creator string, limit, offset int, after, head *HistoryPosition) (out CreatorPage, err error) {
	if limit < 1 || limit > 101 || offset < 0 || offset > 10000 {
		return out, fmt.Errorf("store: invalid creator page")
	}
	var afterTime, headTime int64
	var afterID, headID string
	if after != nil {
		afterTime, afterID = after.Timestamp, after.ID
	}
	if head != nil {
		headTime, headID = head.Timestamp, head.ID
	}
	var body []byte
	var topTime *int64
	var topID *string
	err = s.db.QueryRow(ctx, `
 WITH summary AS (
  SELECT count(*) FILTER (WHERE is_buy) AS buys, count(*) FILTER (WHERE NOT is_buy) AS sells,
   COALESCE(sum(amount_out::numeric) FILTER (WHERE is_buy),0) AS bought,
   COALESCE(sum(amount_in::numeric) FILTER (WHERE NOT is_buy),0) AS sold
  FROM stats.pool_transactions WHERE pool_address=$1 AND sender=$2
   AND ($7='' OR (block_timestamp,id)<=($6,$7))
 ), head AS (
  SELECT block_timestamp,id FROM stats.pool_transactions WHERE pool_address=$1 AND sender=$2
   AND ($7='' OR (block_timestamp,id)<=($6,$7)) ORDER BY block_timestamp DESC,id DESC LIMIT 1
 ), page AS (
  SELECT id, pool_address AS "poolAddress", sender, is_buy AS "isBuy", amount_in AS "amountIn", amount_out AS "amountOut", price, fee, block_timestamp AS "blockTimestamp", tx_hash AS "txHash"
  FROM stats.pool_transactions WHERE pool_address=$1 AND sender=$2
   AND ($5='' OR (block_timestamp,id)<($4,$5)) AND ($7='' OR (block_timestamp,id)<=($6,$7))
  ORDER BY block_timestamp DESC,id DESC LIMIT $8 OFFSET $3
 )
 SELECT buys, sells, bought::text, sold::text, (bought-sold)::text,
  (SELECT block_timestamp FROM head), (SELECT id FROM head),
  COALESCE((SELECT jsonb_agg(page ORDER BY "blockTimestamp" DESC,id DESC) FROM page),'[]'::jsonb)
 FROM summary`, pool, creator, offset, afterTime, afterID, headTime, headID, limit).Scan(
		&out.Summary.BuyCount, &out.Summary.SellCount, &out.Summary.TotalBoughtTokens, &out.Summary.TotalSoldTokens, &out.Summary.NetTokenBalance, &topTime, &topID, &body)
	if err != nil {
		return out, fmt.Errorf("store: creator page: %w", err)
	}
	out.Summary.HasSold = out.Summary.SellCount > 0
	if topTime != nil && topID != nil {
		out.Head = &HistoryPosition{Timestamp: *topTime, ID: *topID}
	}
	if err = json.Unmarshal(body, &out.Transactions); err != nil {
		return out, fmt.Errorf("store: creator page decode: %w", err)
	}
	return out, nil
}

func (s *Store) CrossTokenSwapPage(ctx context.Context, subject string, token bool, limit, offset int, after *HistoryPosition) ([]CrossTokenSwapRow, error) {
	if limit < 1 || limit > 501 || offset < 0 || offset > 10000 {
		return nil, fmt.Errorf("store: invalid cross swap page")
	}
	var timestamp int64
	var id string
	if after != nil {
		timestamp, id = after.Timestamp, after.ID
	}
	predicate := "sender=$1"
	if token {
		predicate = "(token_in=$1 OR token_out=$1)"
	}
	rows, err := s.db.Query(ctx, `SELECT id, sender, token_in, token_out, pool_in, pool_out, amount_in, intermediate_usdl, amount_out, fee_in, fee_out, block_timestamp, tx_hash
 FROM stats.cross_token_swaps WHERE `+predicate+` AND ($5='' OR (block_timestamp,id)<($4,$5))
 ORDER BY block_timestamp DESC,id DESC LIMIT $2 OFFSET $3`, subject, limit, offset, timestamp, id)
	if err != nil {
		return nil, fmt.Errorf("store: cross swap page: %w", err)
	}
	defer rows.Close()
	return scanCrossTokenSwaps(rows)
}
