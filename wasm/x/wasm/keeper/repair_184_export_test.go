package keeper

import (
	"testing"

	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	wasmvm "github.com/Sidiora-Labs/Paxeer-X-Network/wasm-runtime"
	"github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm/types"
	dbm "github.com/tendermint/tm-db"
)

func Repair184PersistentInput(t testing.TB, db dbm.DB, opts ...Option) (sdk.Context, TestKeepers) {
	return createTestInput(t, false, SupportedFeatures, types.DefaultWasmConfig(), db, opts...)
}

func Repair184ImportState(k *Keeper, ctx sdk.Context, addr sdk.AccAddress, models []types.Model) error {
	return k.importContractState(ctx, addr, models)
}

func Repair184Store(k *Keeper, ctx sdk.Context) sdk.KVStore {
	return ctx.KVStore(k.storeKey)
}

func Repair184AppendHistory(k *Keeper, ctx sdk.Context, addr sdk.AccAddress, entries ...types.ContractCodeHistoryEntry) {
	k.appendToContractHistory(ctx, addr, entries...)
}

func Repair184NativeQuery(k *Keeper, ctx sdk.Context, addr sdk.AccAddress, msg []byte, q wasmvm.Querier, limit uint64) ([]byte, uint64, error) {
	_, code, store, err := k.contractInstance(ctx, addr)
	if err != nil {
		return nil, 0, err
	}
	return k.getWasmer(ctx).Query(code.CodeHash, types.NewEnv(ctx, addr), msg, store, cosmwasmAPI, q, k.gasMeter(ctx), limit, costJSONDeserialization)
}
