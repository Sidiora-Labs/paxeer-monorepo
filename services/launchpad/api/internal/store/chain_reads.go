package store

import (
	"context"
	"strconv"

	"github.com/jackc/pgx/v5"
)

// LaunchedChainRow is a chain registry row with the fee right's NFT number.
type LaunchedChainRow struct {
	Chain       string `json:"chain"`
	ChainID     string `json:"chainId"`
	Pool        string `json:"pool"`
	Token       string `json:"token"`
	NFTID       string `json:"nftId"`
	Creator     string `json:"creator"`
	BlockNumber string `json:"blockNumber"`
	Status      string `json:"status"`
}

type ChainRevenueEpochRow struct {
	Epoch          string `json:"epoch"`
	EpochAmount    string `json:"epochAmount"`
	SnapshotBlock  string `json:"snapshotBlock"`
	EligibleSupply string `json:"eligibleSupply"`
}

type ChainRevenueClaimRow struct {
	Epoch       string `json:"epoch"`
	Holder      string `json:"holder"`
	Amount      string `json:"amount"`
	BlockNumber string `json:"blockNumber"`
}

type LaunchedChainsPage struct {
	Items      []LaunchedChainRow
	SnapshotID string
	Total      int
}

type ChainRevenuePage struct {
	Epochs     []ChainRevenueEpochRow
	Claims     []ChainRevenueClaimRow
	SnapshotID string
	Total      int
}

func validChainRow(r LaunchedChainRow) bool {
	chain, e := strconv.ParseUint(r.Chain, 10, 32)
	if e != nil || chain == 0 || strconv.FormatUint(chain, 10) != r.Chain {
		return false
	}
	chainID, e := strconv.ParseUint(r.ChainID, 10, 64)
	if e != nil || chainID == 0 || strconv.FormatUint(chainID, 10) != r.ChainID {
		return false
	}
	block, e := strconv.ParseUint(r.BlockNumber, 10, 64)
	if e != nil || strconv.FormatUint(block, 10) != r.BlockNumber {
		return false
	}
	if _, e = ParseUInt256(r.NFTID); e != nil || r.Status != "active" && r.Status != "retired" {
		return false
	}
	for _, address := range []string{r.Pool, r.Token, r.Creator} {
		if normalized, e := Address(address); e != nil || normalized != address {
			return false
		}
	}
	return true
}

func scanChainRows(rows pgx.Rows) ([]LaunchedChainRow, error) {
	out := []LaunchedChainRow{}
	for rows.Next() {
		var r LaunchedChainRow
		if err := rows.Scan(&r.Chain, &r.ChainID, &r.Pool, &r.Token, &r.NFTID, &r.Creator, &r.BlockNumber, &r.Status); err != nil {
			return nil, err
		}
		if !validChainRow(r) {
			return nil, ErrDataIntegrity
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

const chainColumns = `chain::text,chain_id::text,pool,token,fee_right,creator,block_number::text,status`

func (s *Store) LaunchedChains(ctx context.Context, limit, offset int) (LaunchedChainsPage, error) {
	if limit < 1 || limit > 100 || offset < 0 {
		return LaunchedChainsPage{}, ErrDataIntegrity
	}
	ctx, cancel, tx, err := s.beginLegacyRead(ctx)
	if err != nil {
		return LaunchedChainsPage{}, err
	}
	defer cancel()
	defer rollbackRead(tx)
	var total int
	var maxChain string
	if err = tx.QueryRow(ctx, `SELECT count(*)::int,COALESCE(max(chain),0)::text FROM indexer.launched_chains`).Scan(&total, &maxChain); err != nil {
		return LaunchedChainsPage{}, err
	}
	if offset > total {
		if err = tx.Commit(ctx); err != nil {
			return LaunchedChainsPage{}, err
		}
		return LaunchedChainsPage{Items: []LaunchedChainRow{}, Total: total, SnapshotID: maxChain + ":" + strconv.Itoa(total)}, nil
	}
	rows, err := tx.Query(ctx, `SELECT `+chainColumns+` FROM indexer.launched_chains ORDER BY block_number DESC,chain DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return LaunchedChainsPage{}, err
	}
	items, err := scanChainRows(rows)
	rows.Close()
	if err != nil {
		return LaunchedChainsPage{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return LaunchedChainsPage{}, err
	}
	return LaunchedChainsPage{Items: items, Total: total, SnapshotID: maxChain + ":" + strconv.Itoa(total)}, nil
}

func (s *Store) LaunchedChain(ctx context.Context, chain uint32) (*LaunchedChainRow, error) {
	ctx, cancel, tx, err := s.beginLegacyRead(ctx)
	if err != nil {
		return nil, err
	}
	defer cancel()
	defer rollbackRead(tx)
	rows, err := tx.Query(ctx, `SELECT `+chainColumns+` FROM indexer.launched_chains WHERE chain=$1`, int64(chain))
	if err != nil {
		return nil, err
	}
	items, err := scanChainRows(rows)
	rows.Close()
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, nil
	}
	return &items[0], nil
}

func (s *Store) ChainRevenue(ctx context.Context, chain uint32, holder string, limit, offset int) (ChainRevenuePage, error) {
	if limit < 1 || limit > 100 || offset < 0 {
		return ChainRevenuePage{}, ErrDataIntegrity
	}
	ctx, cancel, tx, err := s.beginLegacyRead(ctx)
	if err != nil {
		return ChainRevenuePage{}, err
	}
	defer cancel()
	defer rollbackRead(tx)
	var total int
	var maxEpoch string
	if err = tx.QueryRow(ctx, `SELECT count(*)::int,COALESCE(max(epoch),0)::text FROM indexer.chain_revenue_epochs WHERE chain=$1`, int64(chain)).Scan(&total, &maxEpoch); err != nil {
		return ChainRevenuePage{}, err
	}
	if offset > total {
		if err = tx.Commit(ctx); err != nil {
			return ChainRevenuePage{}, err
		}
		return ChainRevenuePage{Epochs: []ChainRevenueEpochRow{}, Claims: []ChainRevenueClaimRow{}, Total: total, SnapshotID: maxEpoch + ":" + strconv.Itoa(total)}, nil
	}
	rows, err := tx.Query(ctx, `SELECT epoch::text,amount,snapshot_block::text,eligible_supply FROM indexer.chain_revenue_epochs WHERE chain=$1 ORDER BY epoch DESC LIMIT $2 OFFSET $3`, int64(chain), limit, offset)
	if err != nil {
		return ChainRevenuePage{}, err
	}
	page := ChainRevenuePage{Epochs: []ChainRevenueEpochRow{}, Claims: []ChainRevenueClaimRow{}, Total: total, SnapshotID: maxEpoch + ":" + strconv.Itoa(total)}
	for rows.Next() {
		var e ChainRevenueEpochRow
		if err = rows.Scan(&e.Epoch, &e.EpochAmount, &e.SnapshotBlock, &e.EligibleSupply); err != nil {
			break
		}
		for _, number := range []string{e.Epoch, e.SnapshotBlock} {
			value, parseErr := strconv.ParseUint(number, 10, 64)
			if parseErr != nil || strconv.FormatUint(value, 10) != number {
				err = ErrDataIntegrity
				break
			}
		}
		if err != nil {
			break
		}
		if _, err = ParseUInt256(e.EpochAmount); err != nil {
			break
		}
		if _, err = ParseUInt256(e.EligibleSupply); err != nil {
			break
		}
		page.Epochs = append(page.Epochs, e)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return ChainRevenuePage{}, err
	}
	if holder != "" && len(page.Epochs) != 0 {
		first, last := page.Epochs[0].Epoch, page.Epochs[len(page.Epochs)-1].Epoch
		claims, queryErr := tx.Query(ctx, `SELECT epoch::text,holder,amount,block_number::text FROM indexer.chain_revenue_claims WHERE chain=$1 AND holder=$2 AND epoch BETWEEN $3::numeric AND $4::numeric ORDER BY epoch DESC`, int64(chain), holder, last, first)
		if queryErr != nil {
			return ChainRevenuePage{}, queryErr
		}
		for claims.Next() {
			var c ChainRevenueClaimRow
			if err = claims.Scan(&c.Epoch, &c.Holder, &c.Amount, &c.BlockNumber); err != nil {
				break
			}
			for _, number := range []string{c.Epoch, c.BlockNumber} {
				value, parseErr := strconv.ParseUint(number, 10, 64)
				if parseErr != nil || strconv.FormatUint(value, 10) != number {
					err = ErrDataIntegrity
					break
				}
			}
			if err != nil {
				break
			}
			if _, err = ParseUInt256(c.Amount); err != nil {
				break
			}
			if c.Holder != holder {
				err = ErrDataIntegrity
				break
			}
			page.Claims = append(page.Claims, c)
		}
		if err == nil {
			err = claims.Err()
		}
		claims.Close()
		if err != nil {
			return ChainRevenuePage{}, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return ChainRevenuePage{}, err
	}
	return page, nil
}

func (s *Store) PoolHomeChains(ctx context.Context, pools []string) (map[string]string, error) {
	if len(pools) == 0 || len(pools) > 100 {
		return nil, ErrDataIntegrity
	}
	ctx, cancel, tx, err := s.beginLegacyRead(ctx)
	if err != nil {
		return nil, err
	}
	defer cancel()
	defer rollbackRead(tx)
	rows, err := tx.Query(ctx, `SELECT lower(pool_address),home_chain::text FROM indexer.pools WHERE lower(pool_address)=ANY($1)`, pools)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(pools))
	for rows.Next() {
		var pool, home string
		if err = rows.Scan(&pool, &home); err != nil {
			break
		}
		value, parseErr := strconv.ParseUint(home, 10, 32)
		if parseErr != nil || strconv.FormatUint(value, 10) != home {
			err = ErrDataIntegrity
			break
		}
		out[pool] = home
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return nil, err
	}
	for _, pool := range pools {
		if _, ok := out[pool]; !ok {
			return nil, ErrDataIntegrity
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return out, nil
}
