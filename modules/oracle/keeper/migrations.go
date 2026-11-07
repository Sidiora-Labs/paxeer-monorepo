package keeper

import (
	"fmt"

	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	gogotypes "github.com/gogo/protobuf/types"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/oracle/types"
)

// Migrator is a struct for handling in-place store migrations.
type Migrator struct {
	keeper            Keeper
	votePeriodHistory []VotePeriodHistory
}

// VotePeriodHistory specifies the parameters actually used by MidBlocker and
// EndBlocker over [StartHeight, EndHeight). The upgrade block is excluded.
// A migration policy must cover every block from genesis height 1 through the
// block preceding the upgrade, including every period or window change.
type VotePeriodHistory struct {
	StartHeight uint64
	EndHeight   uint64
	VotePeriod  uint64
	SlashWindow uint64
}

// NewMigrator returns a new Migrator.
// Legacy counters cannot establish their observation count without an explicit
// historical policy; current parameters alone do not establish past periods.
func NewMigrator(keeper Keeper, history ...VotePeriodHistory) Migrator {
	return Migrator{keeper: keeper, votePeriodHistory: append([]VotePeriodHistory(nil), history...)}
}

// Migrate2to3 migrates from version 2 to 3.
func (m Migrator) Migrate2to3(ctx sdk.Context) error {
	store := ctx.KVStore(m.keeper.storeKey)

	iter := sdk.KVStorePrefixIterator(store, types.ExchangeRateKey)
	defer func() { _ = iter.Close() }()
	for ; iter.Valid(); iter.Next() {
		dp := sdk.DecProto{}
		m.keeper.cdc.MustUnmarshal(iter.Value(), &dp)
		// create proto for new data value
		// because we don't have a lastUpdate, we set it to 0
		rate := types.OracleExchangeRate{ExchangeRate: dp.Dec, LastUpdate: sdk.ZeroInt()}
		bz := m.keeper.cdc.MustMarshal(&rate)
		store.Set(iter.Key(), bz)
	}

	return nil
}

// Migrate3to4 migrates from version 3 to 4
func (m Migrator) Migrate3to4(ctx sdk.Context) error {
	// we need to migrate the miss counters to be stored as VotePenaltyCounter to introduce abstain count logic
	store := ctx.KVStore(m.keeper.storeKey)

	// previously the data was stored as uint64, now it is VotePenaltyCounter proto
	iter := sdk.KVStorePrefixIterator(store, types.VotePenaltyCounterKey)
	defer func() { _ = iter.Close() }()
	for ; iter.Valid(); iter.Next() {
		var missCounter gogotypes.UInt64Value
		m.keeper.cdc.MustUnmarshal(iter.Value(), &missCounter)
		// create proto for new data value
		// because we don't have a lastUpdate, we set it to 0
		votePenaltyCounter := types.VotePenaltyCounter{MissCount: missCounter.Value, AbstainCount: 0}
		bz := m.keeper.cdc.MustMarshal(&votePenaltyCounter)
		store.Set(iter.Key(), bz)
	}

	return nil
}

// Migrate3to4 migrates from version 4 to 5
func (m Migrator) Migrate4to5(ctx sdk.Context) error {
	// we remove the prevotes from store in this migration
	store := ctx.KVStore(m.keeper.storeKey)

	oldPrevoteKey := []byte{0x04}
	iter := sdk.KVStorePrefixIterator(store, oldPrevoteKey)
	defer func() { _ = iter.Close() }()
	for ; iter.Valid(); iter.Next() {
		store.Delete(iter.Key())
	}
	return nil
}

func (m Migrator) Migrate5To6(ctx sdk.Context) error {
	// Do a one time backfill for success count in the vote penalty counter
	params := m.keeper.GetParams(ctx)
	height := ctx.BlockHeight()
	if params.SlashWindow == 0 || params.VotePeriod == 0 || height < 0 {
		return fmt.Errorf("invalid oracle migration height, vote period or slash window")
	}
	cacheCtx, commit := ctx.CacheContext()
	store := cacheCtx.KVStore(m.keeper.storeKey)

	// previously the data was stored as uint64, now it is VotePenaltyCounter proto
	iter := sdk.KVStorePrefixIterator(store, types.VotePenaltyCounterKey)
	closed := false
	defer func() {
		if !closed {
			_ = iter.Close()
		}
	}()
	var observations uint64
	if iter.Valid() {
		var err error
		observations, err = m.completedVotePeriods(uint64(height))
		if err != nil {
			return err
		}
	}
	for ; iter.Valid(); iter.Next() {
		var votePenaltyCounter types.VotePenaltyCounter
		if err := m.keeper.cdc.Unmarshal(iter.Value(), &votePenaltyCounter); err != nil {
			return fmt.Errorf("invalid oracle vote penalty counter: %w", err)
		}
		if votePenaltyCounter.MissCount > observations ||
			votePenaltyCounter.AbstainCount > observations-votePenaltyCounter.MissCount {
			return fmt.Errorf("oracle vote penalties exceed completed vote periods")
		}
		successCount := observations - votePenaltyCounter.MissCount - votePenaltyCounter.AbstainCount
		newVotePenaltyCounter := types.VotePenaltyCounter{
			MissCount:    votePenaltyCounter.MissCount,
			AbstainCount: votePenaltyCounter.AbstainCount,
			SuccessCount: successCount,
		}
		bz := m.keeper.cdc.MustMarshal(&newVotePenaltyCounter)
		store.Set(iter.Key(), bz)
	}
	err := iter.Close()
	closed = true
	if err != nil {
		return err
	}
	commit()
	return nil
}

func (m Migrator) completedVotePeriods(height uint64) (uint64, error) {
	if height <= 1 && len(m.votePeriodHistory) == 0 {
		return 0, nil
	}
	var observations uint64
	nextHeight := uint64(1)
	for _, period := range m.votePeriodHistory {
		if period.StartHeight != nextHeight || period.EndHeight <= period.StartHeight ||
			period.EndHeight > height || period.VotePeriod == 0 || period.SlashWindow == 0 {
			return 0, fmt.Errorf("unreconstructable oracle voting history at height %d", nextHeight)
		}
		start := period.StartHeight
		// EndBlocker resets after the tally at heights (h+1) % window == 0.
		// A boundary at start belongs to the previous policy, so only resets
		// strictly inside this interval can discard earlier observations.
		lastReset := period.EndHeight / period.SlashWindow * period.SlashWindow
		if lastReset > start {
			observations = 0
			start = lastReset
		}
		// MidBlocker increments once at (h+1) % period == 0. The migration
		// runs in BeginBlock, before the tally at height itself.
		observations += period.EndHeight/period.VotePeriod - start/period.VotePeriod
		nextHeight = period.EndHeight
	}
	if nextHeight != height {
		return 0, fmt.Errorf("unreconstructable oracle voting history: missing heights [%d, %d)", nextHeight, height)
	}
	return observations, nil
}
