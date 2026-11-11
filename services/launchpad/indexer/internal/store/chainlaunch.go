package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

type LaunchedChain struct {
	Chain       int64
	ChainID     string
	Pool        string
	Token       string
	FeeRight    string
	Creator     string
	BlockNumber int64
	TxHash      string
}

type ChainRevenueEpoch struct {
	Chain          int64
	Epoch          string
	Amount         string
	SnapshotBlock  string
	EligibleSupply string
	OpenedBlock    int64
}

type ChainRevenueClaim struct {
	Chain       int64
	Epoch       string
	Holder      string
	Amount      string
	BlockNumber int64
	TxHash      string
}

func (s *Store) InsertLaunchedChain(ctx context.Context, row LaunchedChain) error {
	tag, err := s.db.Exec(ctx, `INSERT INTO indexer.launched_chains(chain,chain_id,pool,token,fee_right,creator,block_number,tx_hash,status) VALUES($1,$2,$3,$4,$5,$6,$7,$8,'active') ON CONFLICT DO NOTHING`, row.Chain, row.ChainID, row.Pool, row.Token, row.FeeRight, row.Creator, row.BlockNumber, row.TxHash)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		var same bool
		err = s.db.QueryRow(ctx, `SELECT chain_id=$2::numeric AND pool=$3 AND token=$4 AND fee_right=$5 AND creator=$6 AND block_number=$7 AND tx_hash=$8 FROM indexer.launched_chains WHERE chain=$1`, row.Chain, row.ChainID, row.Pool, row.Token, row.FeeRight, row.Creator, row.BlockNumber, row.TxHash).Scan(&same)
		if errors.Is(err, pgx.ErrNoRows) || err == nil && !same {
			return ErrIntegrity
		}
		if err != nil {
			return err
		}
	}
	_, err = s.db.Exec(ctx, `UPDATE indexer.pools SET home_chain=$1 WHERE lower(pool_address)=lower($2) AND lower(token_address)=lower($3) AND (home_chain=0 OR home_chain=$1)`, row.Chain, row.Pool, row.Token)
	return err
}

func (s *Store) InsertChainRevenueEpoch(ctx context.Context, row ChainRevenueEpoch) error {
	tag, err := s.db.Exec(ctx, `INSERT INTO indexer.chain_revenue_epochs(chain,epoch,amount,snapshot_block,eligible_supply,opened_block) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT DO NOTHING`, row.Chain, row.Epoch, row.Amount, row.SnapshotBlock, row.EligibleSupply, row.OpenedBlock)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		var same bool
		err = s.db.QueryRow(ctx, `SELECT amount=$3 AND snapshot_block=$4::numeric AND eligible_supply=$5 AND opened_block=$6 FROM indexer.chain_revenue_epochs WHERE chain=$1 AND epoch=$2::numeric`, row.Chain, row.Epoch, row.Amount, row.SnapshotBlock, row.EligibleSupply, row.OpenedBlock).Scan(&same)
		if errors.Is(err, pgx.ErrNoRows) || err == nil && !same {
			return ErrIntegrity
		}
		return err
	}
	return nil
}

func (s *Store) InsertChainRevenueClaim(ctx context.Context, row ChainRevenueClaim) error {
	tag, err := s.db.Exec(ctx, `INSERT INTO indexer.chain_revenue_claims(chain,epoch,holder,amount,block_number,tx_hash) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT DO NOTHING`, row.Chain, row.Epoch, row.Holder, row.Amount, row.BlockNumber, row.TxHash)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		var same bool
		err = s.db.QueryRow(ctx, `SELECT amount=$4 AND block_number=$5 AND tx_hash=$6 FROM indexer.chain_revenue_claims WHERE chain=$1 AND epoch=$2::numeric AND holder=$3`, row.Chain, row.Epoch, row.Holder, row.Amount, row.BlockNumber, row.TxHash).Scan(&same)
		if errors.Is(err, pgx.ErrNoRows) || err == nil && !same {
			return ErrIntegrity
		}
		return err
	}
	return nil
}

func (s *Store) RetireLaunchedChain(ctx context.Context, chain, block int64) error {
	tag, err := s.db.Exec(ctx, `UPDATE indexer.launched_chains SET status='retired',retired_block=$2 WHERE chain=$1 AND status='active'`, chain, block)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 1 {
		return nil
	}
	var same bool
	err = s.db.QueryRow(ctx, `SELECT status='retired' AND retired_block=$2 FROM indexer.launched_chains WHERE chain=$1`, chain, block).Scan(&same)
	if errors.Is(err, pgx.ErrNoRows) || err == nil && !same {
		return ErrIntegrity
	}
	return err
}
