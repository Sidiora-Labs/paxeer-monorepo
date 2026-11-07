package keeper

import (
	"fmt"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/artifacts"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
)

func (k *Keeper) QueryERCSingleOutput(ctx sdk.Context, typ string, addr common.Address, query string) (interface{}, error) {
	moduleAddr := k.AccountKeeper().GetModuleAddress(types.ModuleName)
	parsedABI := artifacts.GetParsedABI(typ)
	q, err := parsedABI.Pack(query)
	if err != nil {
		return nil, fmt.Errorf("pack %s query %q: %w", typ, query, err)
	}
	r, err := k.StaticCallEVM(ctx, moduleAddr, &addr, q)
	if err != nil {
		logger.Error("Error calling address for query, skipping", "address", addr, "query", query, "err", err)
		return nil, err
	}
	o, err := parsedABI.Unpack(query, r)
	if err != nil {
		return nil, fmt.Errorf("unpack %s query %q at %s: %w", typ, query, addr, err)
	}
	if len(o) != 1 {
		logger.Error("Not getting exactly one outputs when querying address, skipping", "outputs", len(o), "address", addr, "query", query)
		return nil, fmt.Errorf("%s query %q at %s: expected exactly one output, got %d", typ, query, addr, len(o))
	}
	return o[0], nil
}
