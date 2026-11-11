package keeper

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"reflect"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxchains/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	"github.com/ethereum/go-ethereum/common"
)

type BankKeeper interface {
	SendCoinsFromAccountToModule(sdk.Context, sdk.AccAddress, string, sdk.Coins) error
	SendCoinsFromModuleToAccount(sdk.Context, string, sdk.AccAddress, sdk.Coins) error
	SendCoinsFromModuleToModule(sdk.Context, string, string, sdk.Coins) error
	GetBalance(sdk.Context, sdk.AccAddress, string) sdk.Coin
}
type Keeper struct {
	cdc             codec.BinaryCodec
	storeKey        sdk.StoreKey
	bank            BankKeeper
	baseDenom       func(sdk.Context) string
	addressResolver types.AddressResolver
}

func nilDependency(v interface{}) bool {
	if v == nil {
		return true
	}
	r := reflect.ValueOf(v)
	switch r.Kind() {
	case reflect.Ptr, reflect.Interface, reflect.Func, reflect.Map, reflect.Slice:
		return r.IsNil()
	}
	return false
}
func NewKeeper(cdc codec.BinaryCodec, storeKey sdk.StoreKey, bank BankKeeper, baseDenom func(sdk.Context) string) (*Keeper, error) {
	if nilDependency(storeKey) || nilDependency(bank) || baseDenom == nil {
		return nil, fmt.Errorf("layerxchains: missing store key, bank or base denom")
	}
	return &Keeper{cdc: cdc, storeKey: storeKey, bank: bank, baseDenom: baseDenom}, nil
}
func NewKeeperWithAddressResolver(cdc codec.BinaryCodec, storeKey sdk.StoreKey, bank BankKeeper, baseDenom func(sdk.Context) string, resolver types.AddressResolver) (*Keeper, error) {
	if nilDependency(resolver) {
		return nil, fmt.Errorf("layerxchains: missing address resolver")
	}
	k, err := NewKeeper(cdc, storeKey, bank, baseDenom)
	if err != nil {
		return nil, err
	}
	k.addressResolver = resolver
	return k, nil
}
func (k Keeper) resolveAddress(ctx sdk.Context, address common.Address) (sdk.AccAddress, error) {
	if nilDependency(k.addressResolver) {
		return nil, fmt.Errorf("layerxchains: address resolver is required for bank transfers")
	}
	return k.addressResolver.GetPaxAddressOrDefault(ctx, address), nil
}
func putJSON(ctx sdk.Context, key sdk.StoreKey, path []byte, value interface{}) {
	bz, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	ctx.KVStore(key).Set(path, bz)
}
func readJSON(bz []byte, value interface{}) {
	if err := json.Unmarshal(bz, value); err != nil {
		panic(err)
	}
}
func (k Keeper) GetParams(ctx sdk.Context) types.Params {
	bz := ctx.KVStore(k.storeKey).Get(types.ParamsKey)
	if bz == nil {
		return types.DefaultParams()
	}
	var p types.Params
	readJSON(bz, &p)
	return p
}

// BaseDenom is the denomination used for chain bonds and operator revenue.
func (k Keeper) BaseDenom(ctx sdk.Context) string { return k.baseDenom(ctx) }
func (k Keeper) SetParams(ctx sdk.Context, p types.Params) {
	if err := p.Validate(); err != nil {
		panic(err)
	}
	if k.DomainCount(ctx) > p.MaxChains {
		panic(types.ErrTooManyDomains)
	}
	k.IterateDomains(ctx, func(d types.Domain) bool {
		if err := p.ValidateAllowances(d.GasAllowance, d.ByteAllowance, d.FeeShareBps); err != nil {
			panic(err)
		}
		return false
	})
	if err := k.checkCapacity(ctx, p, 0); err != nil {
		panic(err)
	}
	putJSON(ctx, k.storeKey, types.ParamsKey, p)
}
func (k Keeper) GetDomain(ctx sdk.Context, id uint32) (types.Domain, bool) {
	bz := ctx.KVStore(k.storeKey).Get(types.DomainKey(id))
	if bz == nil {
		return types.Domain{}, false
	}
	var d types.Domain
	readJSON(bz, &d)
	return d, true
}
func (k Keeper) GetDomainByName(ctx sdk.Context, name string) (types.Domain, bool) {
	bz := ctx.KVStore(k.storeKey).Get(types.NameIndexKey(name))
	if bz == nil {
		return types.Domain{}, false
	}
	return k.GetDomain(ctx, binary.BigEndian.Uint32(bz))
}
func (k Keeper) setDomain(ctx sdk.Context, d types.Domain) {
	putJSON(ctx, k.storeKey, types.DomainKey(d.Id), d)
}
func (k Keeper) IterateDomains(ctx sdk.Context, fn func(types.Domain) bool) {
	it := sdk.KVStorePrefixIterator(ctx.KVStore(k.storeKey), types.DomainKeyPrefix)
	defer it.Close()
	for ; it.Valid(); it.Next() {
		var d types.Domain
		readJSON(it.Value(), &d)
		if fn(d) {
			break
		}
	}
}
func (k Keeper) DomainCount(ctx sdk.Context) uint32 {
	var n uint32
	k.IterateDomains(ctx, func(types.Domain) bool { n++; return false })
	return n
}
func (k Keeper) NextID(ctx sdk.Context) uint32 {
	bz := ctx.KVStore(k.storeKey).Get(types.NextIDKey)
	if bz == nil {
		return 1
	}
	return binary.BigEndian.Uint32(bz)
}
func (k Keeper) setNextID(ctx sdk.Context, id uint32) {
	bz := make([]byte, 4)
	binary.BigEndian.PutUint32(bz, id)
	ctx.KVStore(k.storeKey).Set(types.NextIDKey, bz)
}
func (k Keeper) checkCapacity(ctx sdk.Context, p types.Params, added uint64) error {
	total := added
	if total > p.MaxTotalGasAllowance {
		return types.ErrTotalAllowanceExceeded
	}
	var err error
	k.IterateDomains(ctx, func(d types.Domain) bool {
		if d.Status != types.StatusActive {
			return false
		}
		if d.GasAllowance > p.MaxTotalGasAllowance-total {
			err = types.ErrTotalAllowanceExceeded
			return true
		}
		total += d.GasAllowance
		return false
	})
	return err
}
func (k Keeper) Create(ctx sdk.Context, operator common.Address, name string, gasAllowance, byteAllowance uint64, feeShareBps uint16, value sdk.Int, initialBaseFee sdk.Dec) (types.Domain, error) {
	return k.create(ctx, nil, operator, name, gasAllowance, byteAllowance, feeShareBps, value, initialBaseFee)
}

// CreateWithPayer creates a domain whose operator and bond payer are different accounts.
func (k Keeper) CreateWithPayer(ctx sdk.Context, payer sdk.AccAddress, operator common.Address, name string, gasAllowance, byteAllowance uint64, feeShareBps uint16, value sdk.Int, initialBaseFee sdk.Dec) (types.Domain, error) {
	if len(payer) == 0 {
		return types.Domain{}, fmt.Errorf("layerxchains: missing bond payer")
	}
	return k.create(ctx, payer, operator, name, gasAllowance, byteAllowance, feeShareBps, value, initialBaseFee)
}

func (k Keeper) create(ctx sdk.Context, payer sdk.AccAddress, operator common.Address, name string, gasAllowance, byteAllowance uint64, feeShareBps uint16, value sdk.Int, initialBaseFee sdk.Dec) (types.Domain, error) {
	if err := types.ValidateName(name); err != nil {
		return types.Domain{}, err
	}
	if _, ok := k.GetDomainByName(ctx, name); ok {
		return types.Domain{}, types.ErrNameTaken
	}
	p := k.GetParams(ctx)
	if err := p.ValidateAllowances(gasAllowance, byteAllowance, feeShareBps); err != nil {
		return types.Domain{}, err
	}
	if err := k.checkCapacity(ctx, p, gasAllowance); err != nil {
		return types.Domain{}, err
	}
	id := k.NextID(ctx)
	if k.DomainCount(ctx) >= p.MaxChains || id == math.MaxUint32 {
		return types.Domain{}, types.ErrTooManyDomains
	}
	if value.IsNil() || value.LT(p.MinBond.Add(p.CreationFee)) {
		return types.Domain{}, types.ErrBondTooLow
	}
	if initialBaseFee.IsNil() || initialBaseFee.IsNegative() {
		return types.Domain{}, fmt.Errorf("invalid initial base fee")
	}
	d := types.Domain{Id: id, Name: name, Operator: operator, Status: types.StatusActive, GasAllowance: gasAllowance, ByteAllowance: byteAllowance, FeeShareBps: feeShareBps, Bond: value.Sub(p.CreationFee), TargetGasPerBlock: gasAllowance / 2, CreatedHeight: ctx.BlockHeight()}
	cache, write := ctx.CacheContext()
	denom := k.baseDenom(cache)
	if payer == nil {
		var err error
		payer, err = k.resolveAddress(cache, operator)
		if err != nil {
			return types.Domain{}, err
		}
	}
	if err := k.bank.SendCoinsFromAccountToModule(cache, payer, types.ModuleName, sdk.NewCoins(sdk.NewCoin(denom, value))); err != nil {
		return types.Domain{}, err
	}
	if p.CreationFee.IsPositive() {
		if err := k.bank.SendCoinsFromModuleToModule(cache, types.ModuleName, authtypes.FeeCollectorName, sdk.NewCoins(sdk.NewCoin(denom, p.CreationFee))); err != nil {
			return types.Domain{}, err
		}
	}
	k.setDomain(cache, d)
	index := make([]byte, 4)
	binary.BigEndian.PutUint32(index, id)
	cache.KVStore(k.storeKey).Set(types.NameIndexKey(name), index)
	k.setNextID(cache, id+1)
	k.SetNextBaseFee(cache, id, initialBaseFee)
	write()
	ctx.EventManager().EmitEvents(cache.EventManager().Events())
	return d, nil
}
func (k Keeper) Fund(ctx sdk.Context, id uint32, from common.Address, amount sdk.Int) error {
	d, ok := k.GetDomain(ctx, id)
	if !ok {
		return types.ErrUnknownDomain
	}
	if d.Status == types.StatusRetired {
		return types.ErrDomainRetired
	}
	if amount.IsNil() || !amount.IsPositive() {
		return types.ErrBondTooLow
	}
	cache, write := ctx.CacheContext()
	account, err := k.resolveAddress(cache, from)
	if err != nil {
		return err
	}
	if err := k.bank.SendCoinsFromAccountToModule(cache, account, types.ModuleName, sdk.NewCoins(sdk.NewCoin(k.baseDenom(cache), amount))); err != nil {
		return err
	}
	d.Bond = d.Bond.Add(amount)
	k.setDomain(cache, d)
	write()
	ctx.EventManager().EmitEvents(cache.EventManager().Events())
	return nil
}
func (k Keeper) operatorDomain(ctx sdk.Context, operator common.Address, id uint32) (types.Domain, error) {
	d, ok := k.GetDomain(ctx, id)
	if !ok {
		return d, types.ErrUnknownDomain
	}
	if d.Operator != operator {
		return d, types.ErrNotOperator
	}
	if d.Status == types.StatusRetired {
		return d, types.ErrDomainRetired
	}
	return d, nil
}
func (k Keeper) Pause(ctx sdk.Context, operator common.Address, id uint32) error {
	d, err := k.operatorDomain(ctx, operator, id)
	if err != nil {
		return err
	}
	if d.Status != types.StatusActive {
		return types.ErrDomainNotActive
	}
	d.Status = types.StatusPaused
	k.setDomain(ctx, d)
	return nil
}
func (k Keeper) Resume(ctx sdk.Context, operator common.Address, id uint32) error {
	d, err := k.operatorDomain(ctx, operator, id)
	if err != nil {
		return err
	}
	if d.Status == types.StatusGovernancePaused {
		return types.ErrNotAuthority
	}
	if d.Status != types.StatusPaused {
		return types.ErrDomainNotActive
	}
	if err := k.checkCapacity(ctx, k.GetParams(ctx), d.GasAllowance); err != nil {
		return err
	}
	d.Status = types.StatusActive
	k.setDomain(ctx, d)
	return nil
}
func (k Keeper) GovernancePause(ctx sdk.Context, caller common.Address, id uint32, paused bool) error {
	p := k.GetParams(ctx)
	if caller != p.Authority {
		return types.ErrNotAuthority
	}
	d, ok := k.GetDomain(ctx, id)
	if !ok {
		return types.ErrUnknownDomain
	}
	if d.Status == types.StatusRetired {
		return types.ErrDomainRetired
	}
	if paused {
		d.Status = types.StatusGovernancePaused
	} else {
		if d.Status != types.StatusGovernancePaused {
			return types.ErrDomainNotActive
		}
		if err := k.checkCapacity(ctx, p, d.GasAllowance); err != nil {
			return err
		}
		d.Status = types.StatusActive
	}
	k.setDomain(ctx, d)
	return nil
}
func (k Keeper) Retire(ctx sdk.Context, operator common.Address, id uint32) error {
	d, err := k.operatorDomain(ctx, operator, id)
	if err != nil {
		return err
	}
	cache, write := ctx.CacheContext()
	if d.Bond.IsPositive() {
		account, err := k.resolveAddress(cache, operator)
		if err != nil {
			return err
		}
		if err := k.bank.SendCoinsFromModuleToAccount(cache, types.ModuleName, account, sdk.NewCoins(sdk.NewCoin(k.baseDenom(cache), d.Bond))); err != nil {
			return err
		}
	}
	d.Bond = sdk.ZeroInt()
	d.Status = types.StatusRetired
	k.setDomain(cache, d)
	write()
	ctx.EventManager().EmitEvents(cache.EventManager().Events())
	return nil
}
func (k Keeper) SetOperator(ctx sdk.Context, operator common.Address, id uint32, next common.Address) error {
	d, err := k.operatorDomain(ctx, operator, id)
	if err != nil {
		return err
	}
	if next == (common.Address{}) {
		return fmt.Errorf("operator cannot be zero")
	}
	d.Operator = next
	k.setDomain(ctx, d)
	return nil
}
func (k Keeper) ResolveChainID(ctx sdk.Context, chainID *big.Int) (types.Domain, bool) {
	id, ok := types.DomainOfChainID(chainID)
	if !ok || id == 0 {
		return types.Domain{}, false
	}
	return k.GetDomain(ctx, id)
}
func (k Keeper) GasAllowance(ctx sdk.Context, id uint32) uint64 {
	d, ok := k.GetDomain(ctx, id)
	if !ok || d.Status != types.StatusActive {
		return 0
	}
	return d.GasAllowance
}
