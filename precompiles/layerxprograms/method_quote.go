package layerxprograms

import (
	"fmt"
	"math/big"

	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

func init() {
	register("quoteCallFee", quoteHandler)
}

func quoteHandler(ctx sdk.Context, p *PrecompileExecutor, call callContext, args []interface{}) ([]byte, error) {
	decoded, err := decodeArgs(call.method, args)
	if err != nil {
		return nil, err
	}
	if p == nil || p.host == nil {
		return nil, fmt.Errorf("LXP_PRECOMPILE_NO_CONTEXT: programs fee reader is unavailable")
	}
	record, err := p.host.ReadFeeSchedule(ctx)
	if err != nil {
		return nil, err
	}
	schedule, err := decodeFeeScheduleRecord(record)
	if err != nil {
		return nil, fmt.Errorf("LXP_NON_CANONICAL: invalid programs fee schedule record: %w", err)
	}
	budget := decoded[0].([7]uint64)
	ceiling := new(big.Int)
	// The first six components cover execution; the seventh is available for
	// occupancy responsibility when the call binds its signed fee limit.
	for i, word := range budget {
		price := schedule.Prices[i]
		component := new(big.Int).Mul(new(big.Int).SetUint64(word), new(big.Int).SetUint64(price))
		ceiling.Add(ceiling, component)
		if ceiling.BitLen() > 128 {
			return nil, fmt.Errorf("LXP_OVERFLOW: programs fee ceiling exceeds uint128")
		}
	}
	return programsABI.Methods["quoteCallFee"].Outputs.Pack(schedule.Version, ceiling, schedule.Asset)
}
