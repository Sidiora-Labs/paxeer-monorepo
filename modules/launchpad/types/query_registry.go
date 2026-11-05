package types

func FeeRightInfoOf(right FeeRight) FeeRightInfo {
	return FeeRightInfo{
		ID:       right.ID,
		Owner:    right.Owner,
		Approved: right.Approved,
		Strategy: uint32(right.Strategy),
		Pool:     right.Pool,
	}
}

func AirdropEpochInfoOf(epoch PoolAirdropEpoch) AirdropEpochInfo {
	return AirdropEpochInfo{
		Pool:            epoch.Pool,
		Epoch:           epoch.Epoch,
		Amount:          epoch.Amount,
		SnapshotBlock:   epoch.SnapshotBlock,
		EligibleSupply:  epoch.EligibleSupply,
		Remaining:       epoch.Remaining,
		ClaimedSupply:   epoch.ClaimedSupply,
		LegacyRoot:      epoch.LegacyRoot,
		LegacyRemaining: epoch.LegacyRemaining,
	}
}

func RetiredInfoOf(retired RetiredAddress) RetiredInfo {
	return RetiredInfo{
		Address:     retired.Address,
		Role:        retired.Role,
		Replacement: retired.Replacement,
	}
}
