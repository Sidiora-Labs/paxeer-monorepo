package keeper

import (
	"encoding/binary"
	"encoding/json"
	"fmt"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/types"
	"github.com/ethereum/go-ethereum/common"
)

// Keeper owns the native launchpad: every market's token supply, real quote
// balance and undistributed fees sit in the launchpad module account, and
// only swaps and fee-strategy executions move them.
type Keeper struct {
	storeKey sdk.StoreKey

	bankKeeper   types.BankKeeper
	tokenFactory types.TokenFactory
	evmKeeper    types.EVMKeeper

	authority string

	generation2 RoleAddresses
}

// RoleAddresses are the role contract addresses of one Launch-x generation.
type RoleAddresses struct {
	SidioraFactory  common.Address
	Router          common.Address
	Quoter          common.Address
	FeesRouter      common.Address
	FeeAccumulator  common.Address
	PoolRegistry    common.Address
	ProtocolConfig  common.Address
	SidioraNFT      common.Address
	Treasury        common.Address
	OpticalRegistry common.Address
	EventEmitter    common.Address
	PoolBeacon      common.Address
}

// generation2Roles are the generation 2 role addresses served by 0x1017 for
// every pool; the Treasury is the address SidioraFactory slot 6 held.
var generation2Roles = RoleAddresses{
	SidioraFactory:  common.HexToAddress("0x3993D004166738d1069B5C4e0a923f7B9D5D8830"),
	Router:          common.HexToAddress("0x78FB06f525cD964388f2305A489035263E31C6e4"),
	Quoter:          common.HexToAddress("0x793006f4F321b8863F26DBD75b11bAa01DA181dc"),
	FeesRouter:      common.HexToAddress("0x05681dA6f611C2dfFD3E92611F6dE2d247647927"),
	FeeAccumulator:  common.HexToAddress("0x7b14808b72f61664cDb79D25E6d1587655571a66"),
	PoolRegistry:    common.HexToAddress("0x1dC357d2B7B850332a91F06A9E66634F46fC4Ba7"),
	ProtocolConfig:  common.HexToAddress("0xF3669eDc475Cb6E1F0d7635240ABD8e8FB7E20C6"),
	SidioraNFT:      common.HexToAddress("0xe8DcC844a8006dbf64f5Bb851bCad90292a6409B"),
	Treasury:        common.HexToAddress("0xBb82d74E071e03d8E68F1d78Fae9EFe8b6b7A723"),
	OpticalRegistry: common.HexToAddress("0x0FB5E510372Bd3EddFED16A5Dd766d4473CffcF0"),
	EventEmitter:    common.HexToAddress("0x622E1bb90E0E2B6E6e5e9e2DbE1555389E8A3EF6"),
	PoolBeacon:      common.HexToAddress("0x0c60Ea33B1DD602A17E1EF1875eE912F3fad71BE"),
}

// NewKeeper wires the launchpad keeper. Params change only through
// UpdateParams signed by the governance module account.
func NewKeeper(storeKey sdk.StoreKey, bankKeeper types.BankKeeper, tokenFactory types.TokenFactory,
	evmKeeper types.EVMKeeper) *Keeper {
	return &Keeper{storeKey: storeKey, bankKeeper: bankKeeper, tokenFactory: tokenFactory, evmKeeper: evmKeeper,
		authority: authtypes.NewModuleAddress(govtypes.ModuleName).String(), generation2: generation2Roles}
}

func (k *Keeper) store(ctx sdk.Context) sdk.KVStore { return ctx.KVStore(k.storeKey) }

// ModuleAddress is the escrow account of every market.
func (k *Keeper) ModuleAddress() sdk.AccAddress { return authtypes.NewModuleAddress(types.ModuleName) }

// Generation2 returns the generation 2 role addresses.
func (k *Keeper) Generation2() RoleAddresses { return k.generation2 }

// TreasuryContract is the generation 2 Treasury address that serves the
// Treasury of every generation.
func (k *Keeper) TreasuryContract() common.Address { return k.generation2.Treasury }

// Authority is the bech32 account allowed to change params.
func (k *Keeper) Authority() string { return k.authority }

func (k *Keeper) GetParams(ctx sdk.Context) types.Params {
	bz := k.store(ctx).Get(types.ParamsKey)
	if bz == nil {
		return types.DefaultParams()
	}
	var params types.Params
	if err := json.Unmarshal(bz, &params); err != nil {
		panic(fmt.Errorf("launchpad: corrupt params: %w", err))
	}
	return params
}

func (k *Keeper) setParams(ctx sdk.Context, params types.Params) error {
	if err := params.Validate(); err != nil {
		return err
	}
	bz, err := json.Marshal(params)
	if err != nil {
		return err
	}
	k.store(ctx).Set(types.ParamsKey, bz)
	return nil
}

// UpdateParams replaces the params when signed by the governance module
// account, which is how ProtocolConfig's admin setters are reached natively.
func (k *Keeper) UpdateParams(ctx sdk.Context, authority string, params types.Params) error {
	if authority != k.authority {
		return fmt.Errorf("%w: %s", types.ErrUnauthorized, authority)
	}
	if err := k.setParams(ctx, params); err != nil {
		return err
	}
	ctx.EventManager().EmitEvent(sdk.NewEvent(types.EventTypeParamsUpdated))
	return nil
}

func (k *Keeper) getUint64(ctx sdk.Context, key []byte) uint64 {
	bz := k.store(ctx).Get(key)
	if bz == nil {
		return 0
	}
	return binary.BigEndian.Uint64(bz)
}

func (k *Keeper) setUint64(ctx sdk.Context, key []byte, value uint64) {
	k.store(ctx).Set(key, binary.BigEndian.AppendUint64(nil, value))
}

// GetMarketCount is the number of markets ever created.
func (k *Keeper) GetMarketCount(ctx sdk.Context) uint64 {
	return k.getUint64(ctx, types.MarketCountKey)
}

func (k *Keeper) GetMarket(ctx sdk.Context, denom string) (types.Market, bool) {
	bz := k.store(ctx).Get(types.MarketKey(denom))
	if bz == nil {
		return types.Market{}, false
	}
	var market types.Market
	if err := json.Unmarshal(bz, &market); err != nil {
		panic(fmt.Errorf("launchpad: corrupt market %s: %w", denom, err))
	}
	return market, true
}

func (k *Keeper) mustMarket(ctx sdk.Context, denom string) (types.Market, error) {
	market, found := k.GetMarket(ctx, denom)
	if !found {
		return types.Market{}, fmt.Errorf("%w: %s", types.ErrUnknownMarket, denom)
	}
	return market, nil
}

func (k *Keeper) setMarket(ctx sdk.Context, market types.Market) {
	bz, err := json.Marshal(market)
	if err != nil {
		panic(err)
	}
	k.store(ctx).Set(types.MarketKey(market.Denom), bz)
}

// indexMarket writes a market's position, pointer and creator indexes.
func (k *Keeper) indexMarket(ctx sdk.Context, market types.Market) {
	store := k.store(ctx)
	store.Set(types.MarketIndexKey(market.Index), []byte(market.Denom))
	if common.IsHexAddress(market.Pointer) {
		store.Set(types.MarketPointerKey(common.HexToAddress(market.Pointer)), []byte(market.Denom))
	}
	creator := sdk.MustAccAddressFromBech32(market.Creator)
	store.Set(types.CreatorMarketKey(creator, market.Index), []byte(market.Denom))
}

// GetMarketByIndex resolves the index-th market (1-based, creation order).
func (k *Keeper) GetMarketByIndex(ctx sdk.Context, index uint64) (types.Market, bool) {
	denom := k.store(ctx).Get(types.MarketIndexKey(index))
	if denom == nil {
		return types.Market{}, false
	}
	return k.GetMarket(ctx, string(denom))
}

// GetMarketByPointer resolves a market from its ERC20 pointer.
func (k *Keeper) GetMarketByPointer(ctx sdk.Context, pointer common.Address) (types.Market, bool) {
	denom := k.store(ctx).Get(types.MarketPointerKey(pointer))
	if denom == nil {
		return types.Market{}, false
	}
	return k.GetMarket(ctx, string(denom))
}

// GetMarkets returns up to limit markets starting at the offset-th (0-based)
// in creation order.
func (k *Keeper) GetMarkets(ctx sdk.Context, offset, limit uint64) []types.Market {
	count := k.GetMarketCount(ctx)
	out := []types.Market{}
	for index := offset + 1; index <= count && uint64(len(out)) < limit && index > offset; index++ {
		if market, found := k.GetMarketByIndex(ctx, index); found {
			out = append(out, market)
		}
	}
	return out
}

// GetMarketsByCreator returns up to limit of creator's markets after skipping
// offset of them, in creation order.
func (k *Keeper) GetMarketsByCreator(ctx sdk.Context, creator sdk.AccAddress, offset, limit uint64) []types.Market {
	out := []types.Market{}
	iterator := sdk.KVStorePrefixIterator(k.store(ctx), types.CreatorMarketPrefixFor(creator))
	defer iterator.Close()
	skipped := uint64(0)
	for ; iterator.Valid() && uint64(len(out)) < limit; iterator.Next() {
		if skipped < offset {
			skipped++
			continue
		}
		if market, found := k.GetMarket(ctx, string(iterator.Value())); found {
			out = append(out, market)
		}
	}
	return out
}

// IterateMarkets visits every market in creation order until cb returns true.
func (k *Keeper) IterateMarkets(ctx sdk.Context, cb func(types.Market) bool) {
	count := k.GetMarketCount(ctx)
	for index := uint64(1); index <= count; index++ {
		market, found := k.GetMarketByIndex(ctx, index)
		if found && cb(market) {
			return
		}
	}
}
