package receipt

import (
	"context"
	"encoding/json"
	"math"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/checked"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/ethereum/go-ethereum/common"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ReadDiscoveredMarkets restores canonical source identities without replacing
// their deployment generation or dropping their durable creation provenance.
func ReadDiscoveredMarkets(ctx context.Context, pool *pgxpool.Pool) ([]DiscoveredMarket, error) {
	if pool == nil {
		return nil, ErrQualification
	}
	rows, err := pool.Query(ctx, `SELECT token,pool,height,transaction_index,block_hash,transaction_hash,log_index,provenance FROM indexer.indexer_canonical_creations ORDER BY height,transaction_index,log_index LIMIT 2049`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	markets := []DiscoveredMarket{}
	for rows.Next() {
		var row canonicalMarketRow
		if err = rows.Scan(&row.Token, &row.Pool, &row.Height, &row.TransactionIndex, &row.BlockHash, &row.TransactionHash, &row.LogIndex, &row.Provenance); err != nil {
			return nil, err
		}
		market, err := row.decode()
		if err != nil {
			return nil, err
		}
		markets = append(markets, market)
		if len(markets) > 2048 {
			return nil, ErrQualification
		}
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return markets, nil
}

// canonicalMarketRow is the storage-neutral codec for the canonical creation
// projection. SQL transport must preserve these original source coordinates.
type canonicalMarketRow struct {
	Token, Pool                string
	Height, TransactionIndex   int64
	BlockHash, TransactionHash string
	LogIndex                   int64
	Provenance                 json.RawMessage
}

func (row canonicalMarketRow) decode() (DiscoveredMarket, error) {
	var provenance accounting.Provenance
	if !common.IsHexAddress(row.Token) || !common.IsHexAddress(row.Pool) || !rpcHash(row.BlockHash) || !rpcHash(row.TransactionHash) || row.Height < 0 || row.TransactionIndex < 0 || row.TransactionIndex > math.MaxUint32 || row.LogIndex < 0 || row.LogIndex > math.MaxUint32 || json.Unmarshal(row.Provenance, &provenance) != nil {
		return DiscoveredMarket{}, ErrQualification
	}
	rowHeightUint64, conversionErr := checked.Convert[uint64](row.Height)
	if conversionErr != nil {
		return DiscoveredMarket{}, conversionErr
	}
	rowTransactionIndexUint, conversionErr := checked.Convert[uint](row.TransactionIndex)
	if conversionErr != nil {
		return DiscoveredMarket{}, conversionErr
	}
	rowLogIndexUint, conversionErr := checked.Convert[uint](row.LogIndex)
	if conversionErr != nil {
		return DiscoveredMarket{}, conversionErr
	}
	market := DiscoveredMarket{Token: common.HexToAddress(row.Token), Pool: common.HexToAddress(row.Pool), Height: rowHeightUint64, TransactionIndex: rowTransactionIndexUint, Creation: &MarketCreationProvenance{BlockHash: common.HexToHash(row.BlockHash), TransactionHash: common.HexToHash(row.TransactionHash), LogIndex: rowLogIndexUint, Provenance: provenance}}
	if market.Token == (common.Address{}) || market.Pool == (common.Address{}) || market.Token == market.Pool || provenance.DeploymentID == "" || provenance.ABIRevision == "" || provenance.EmitterCodeEpoch == "" || provenance.PoolCodeEpoch == "" || provenance.ReceiptSHA256 == "" || provenance.HeaderSHA256 == "" {
		return DiscoveredMarket{}, ErrQualification
	}
	return market, nil
}
