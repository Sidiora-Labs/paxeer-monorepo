# Markets contract: perps, spot, pricefeed, oracle, epoch, mint and layerxexchange

The market modules provide perpetuals, spot trading, price feeds, epochs and minting. The perps construct uses `0x1015` from `v6.12.0`; pricefeed uses `0x1020`; spot uses `0x1021` through `0x1025`. The retained oracle precompile at `0x1008` refuses both queries with `ErrOraclePrecompileRetired`. Address selection and activation are defined by [precompiles/setup.go](../../../precompiles/setup.go) and [node/precompiles_v6120.go](../../../node/precompiles_v6120.go).

The tables identify code declarations and configuration names. Activation, authentication and transport registration are defined by the linked implementations.

<a id="scope-and-reading-guide"></a>

<a id="surface-summary"></a>

<a id="perps"></a>

## perps-0x1015

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `func ValidateGenesis(gs types.GenesisState) error` | [modules/perps/keeper/genesis.go](../../../modules/perps/keeper/genesis.go) |
| `go_exported` | `func RegisterInvariants(registry sdk.InvariantRegistry, k *Keeper)` | [modules/perps/keeper/invariants.go](../../../modules/perps/keeper/invariants.go) |
| `go_exported` | `func NewKeeper(storeKey sdk.StoreKey, evm types.EVMKeeper, pricefeed types.PricefeedKeeper) *Keeper` | [modules/perps/keeper/keeper.go](../../../modules/perps/keeper/keeper.go) |
| `go_exported` | `func (k *Keeper) CheckPause(ctx sdk.Context, method string) error` | [modules/perps/keeper/pause.go](../../../modules/perps/keeper/pause.go) |
| `go_exported` | `func SeedV6120(ctx sdk.Context, k Keeper) error` | [modules/perps/keeper/seed_v6120.go](../../../modules/perps/keeper/seed_v6120.go) |
| `go_exported` | `func (AppModuleBasic) ValidateGenesis(_ codec.JSONCodec, _ client.TxEncodingConfig, bz json.RawMessage) error` | [modules/perps/module.go](../../../modules/perps/module.go) |
| `go_exported` | `type PricefeedKeeper interface {` | [modules/perps/types/expected_keepers.go](../../../modules/perps/types/expected_keepers.go) |
| `genesis_param` | `type GenesisState struct` | [modules/perps/types/genesis.go](../../../modules/perps/types/genesis.go) |
| `storage_key` | `ParamsKey = []byte{0x01}` | [modules/perps/types/keys.go](../../../modules/perps/types/keys.go) |
| `storage_key` | `MarketPrefix = []byte{0x02}` | [modules/perps/types/keys.go](../../../modules/perps/types/keys.go) |
| `storage_key` | `MarketStatePrefix = []byte{0x03}` | [modules/perps/types/keys.go](../../../modules/perps/types/keys.go) |
| `storage_key` | `AccountPrefix = []byte{0x04}` | [modules/perps/types/keys.go](../../../modules/perps/types/keys.go) |
| `storage_key` | `OrderPrefix = []byte{0x05}` | [modules/perps/types/keys.go](../../../modules/perps/types/keys.go) |
| `storage_key` | `BookPrefix = []byte{0x06}` | [modules/perps/types/keys.go](../../../modules/perps/types/keys.go) |
| `storage_key` | `CloidPrefix = []byte{0x07}` | [modules/perps/types/keys.go](../../../modules/perps/types/keys.go) |
| `storage_key` | `AccountOrderPrefix = []byte{0x08}` | [modules/perps/types/keys.go](../../../modules/perps/types/keys.go) |
| `storage_key` | `PositionPrefix = []byte{0x09}` | [modules/perps/types/keys.go](../../../modules/perps/types/keys.go) |
| `storage_key` | `AccountMarketPositionPrefix = []byte{0x0A}` | [modules/perps/types/keys.go](../../../modules/perps/types/keys.go) |
| `storage_key` | `TriggerPrefix = []byte{0x0B}` | [modules/perps/types/keys.go](../../../modules/perps/types/keys.go) |
| `storage_key` | `ExpiryPrefix = []byte{0x0C}` | [modules/perps/types/keys.go](../../../modules/perps/types/keys.go) |
| `storage_key` | `VolumePrefix = []byte{0x0D}` | [modules/perps/types/keys.go](../../../modules/perps/types/keys.go) |
| `storage_key` | `VenueVolumePrefix = []byte{0x0E}` | [modules/perps/types/keys.go](../../../modules/perps/types/keys.go) |
| `storage_key` | `LiquidationCooldownPrefix = []byte{0x0F}` | [modules/perps/types/keys.go](../../../modules/perps/types/keys.go) |
| `storage_key` | `SequencePrefix = []byte{0x10}` | [modules/perps/types/keys.go](../../../modules/perps/types/keys.go) |
| `storage_key` | `AttachedTpSlPrefix = []byte{0x11}` | [modules/perps/types/keys.go](../../../modules/perps/types/keys.go) |
| `storage_key` | `MarginRatioIndexPrefix = []byte{0x12}` | [modules/perps/types/keys.go](../../../modules/perps/types/keys.go) |
| `storage_key` | `AdlRankIndexPrefix = []byte{0x13}` | [modules/perps/types/keys.go](../../../modules/perps/types/keys.go) |
| `storage_key` | `SweepCursorKey = []byte{0x14}` | [modules/perps/types/keys.go](../../../modules/perps/types/keys.go) |
| `storage_key` | `VenueBooksKey = []byte{0x15}` | [modules/perps/types/keys.go](../../../modules/perps/types/keys.go) |
| `genesis_param` | `type Market struct` | [modules/perps/types/market.go](../../../modules/perps/types/market.go) |
| `genesis_param` | `type Params struct` | [modules/perps/types/params.go](../../../modules/perps/types/params.go) |
| `precompile_method` | `function addCollateral(uint256,uint256) nonpayable returns ()` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_method` | `function addSize(uint256,uint256,uint256) nonpayable returns ()` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_method` | `function cancelByCloid(uint256,bytes16) nonpayable returns ()` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_method` | `function cancelOrder(uint256) nonpayable returns ()` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_method` | `function checkLiquidatable(uint256) view returns (bool,uint256)` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_method` | `function closePosition(uint256) nonpayable returns ()` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_method` | `function closePositionWithParams(uint256,uint256,uint256,uint64) nonpayable returns ()` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_method` | `function createVault() nonpayable returns (address)` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_method` | `function deposit(address,uint256) nonpayable returns ()` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_method` | `function emergencyWithdraw(address) nonpayable returns ()` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_error` | `error PerpsCloseSizeExceedsPosition()` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_error` | `error PerpsCrossMarginOnly()` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_error` | `error PerpsDeadlineExpired()` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_error` | `error PerpsDelegatecall()` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_error` | `error PerpsDuplicateCloid()` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_error` | `error PerpsFokNotFilled()` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_error` | `error PerpsInsufficientCollateral()` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_error` | `error PerpsInsufficientMargin()` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_error` | `error PerpsInvalidGenesis()` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_error` | `error PerpsInvalidOrderType()` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_error` | `error PerpsInvalidParams()` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_error` | `error PerpsInvalidTimeInForce()` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_error` | `error PerpsInvalidTriggerPrice()` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_error` | `error PerpsLeverageTooHigh()` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_error` | `error PerpsLiquidationCooldown()` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_error` | `error PerpsLotSize()` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_error` | `error PerpsMarketDisabled()` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_error` | `error PerpsMaxOrderValue()` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_error` | `error PerpsMinOrderValue()` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_error` | `error PerpsNoAccount()` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_error` | `error PerpsNoLiquidity()` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_error` | `error PerpsNotLiquidatable()` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_error` | `error PerpsNotOrderHolder()` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_error` | `error PerpsNotPositionHolder()` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_error` | `error PerpsOpenInterestCap()` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_error` | `error PerpsOverflow()` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_error` | `error PerpsPaused()` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_error` | `error PerpsPostOnlyWouldCross()` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_error` | `error PerpsPriceStale()` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_error` | `error PerpsReduceOnlyViolation()` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_error` | `error PerpsSelfTransfer()` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_error` | `error PerpsSlippageExceeded()` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_error` | `error PerpsStaticCall()` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_error` | `error PerpsTickSize()` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_error` | `error PerpsTokenTransferFailed()` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_error` | `error PerpsTooManyOpenOrders()` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_error` | `error PerpsTriggerNotReached()` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_error` | `error PerpsUnauthorized()` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_error` | `error PerpsUnknownMarket()` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_error` | `error PerpsUnknownOrder()` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_error` | `error PerpsUnknownPosition()` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_error` | `error PerpsUnsupportedCollateral()` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_error` | `error PerpsZeroAddress()` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_error` | `error PerpsZeroAmount()` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_event` | `event ADLExecuted(uint256,uint256)` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_event` | `event ADLMatched(uint256,uint256,address,uint256,uint256,uint256)` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_event` | `event BackstopLiquidation(address,uint256,uint256,uint256,uint256)` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_event` | `event CollateralDeposited(address,uint256,uint256)` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_event` | `event CollateralTransferred(address,address,uint256)` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_event` | `event CollateralWithdrawn(address,uint256,uint256)` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_event` | `event FeeCollected(uint256,address,uint256,uint8,uint256,uint256,address)` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_event` | `event Fill(uint256,uint256,uint256,address,address,bool,uint256,uint256,int256,uint256)` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_event` | `event FundingRateUpdated(uint256,int256,int256)` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_event` | `event FundingSettled(uint256,int256,int256,int256)` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_event` | `event InsuranceContribution(address,uint256,uint8)` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_event` | `event InsuranceDraw(uint256,uint256,uint256)` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_event` | `event Liquidation(uint256,address,uint256,uint256,uint256,address)` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_event` | `event MarkPriceUpdated(uint256,uint256,uint256,uint256)` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_event` | `event OpenInterestChanged(uint256,uint256,uint256,uint256,bool)` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_event` | `event OrderCancelled(uint256,address)` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_event` | `event OrderExecuted(uint256,uint256,uint256)` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_event` | `event OrderExpired(uint256,address,uint256)` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_event` | `event OrderOptions(uint256,uint8,bool,uint256,uint256,uint64,bytes16)` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_event` | `event OrderPlaced(uint256,address,uint256,uint8,bool,uint256,uint256)` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_event` | `event OrderTriggered(uint256,uint256,uint256)` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_event` | `event PositionClosed(uint256,address,uint256,uint256,uint256,int256,bool)` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_event` | `event PositionEntryPriceChanged(uint256,uint256,uint256)` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_event` | `event PositionFundingApplied(uint256,address,uint256,int256,uint256,uint256)` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_event` | `event PositionModified(uint256,uint256,uint256,uint256)` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_event` | `event PositionOpened(uint256,address,uint256,bool,uint256,uint256,uint256,address,uint256)` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_event` | `event PositionTpSlSet(uint256,uint256,uint256)` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_event` | `event TradeSettled(uint256,address,uint256,uint8,uint256,uint256,int256,uint256,int256,uint256)` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_event` | `event VaultCreated(address,address)` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_event` | `event VaultDeficit(address,uint256)` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_method` | `function executeOrder(uint256) nonpayable returns (uint256)` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_method` | `function getAccount(address) view returns ((uint256,int256,uint256,uint256,uint256,uint8,uint256,uint256,uint256))` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_method` | `function getAccountSummary(address) view returns ((uint256,uint256,uint256))` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_method` | `function getActiveMarketIds() view returns (uint256[])` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_method` | `function getADLRank(uint256) view returns (uint256,bool)` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_method` | `function getBalance(address,address) view returns (uint256)` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_method` | `function getCurrentFundingRate(uint256) view returns (int256)` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_method` | `function getFees() view returns (uint256,uint256,uint256,uint256)` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_method` | `function getFeeTier(address) view returns (uint8,uint256,int256,uint256)` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_method` | `function getFundingDetails(uint256) view returns (int256,uint256,int256,int256,uint256)` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_method` | `function getFundingState(uint256) view returns (int256,int256,uint256,int256)` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_method` | `function getInsuranceBalance(address) view returns (uint256)` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_method` | `function getLiquidationPrice(uint256) view returns (uint256)` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_method` | `function getLockedBalance(address) view returns (uint256)` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_method` | `function getMarginMode() view returns (uint8)` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_method` | `function getMarginTiers(uint256) view returns ((uint256,uint256,uint256,uint256)[])` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_method` | `function getMarket(uint256) view returns (string,string,uint256,uint256,uint256,bool)` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_method` | `function getMarketConfig(uint256) view returns ((uint256,string,string,string,uint8,uint8,uint256,uint256,uint256,uint8,uint256,uint256,uint256,bool,uint64))` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_method` | `function getMarkPrice(uint256) view returns (uint256)` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_method` | `function getMarkPriceDetails(uint256) view returns (uint256,uint256,uint256,bool)` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_method` | `function getOpenInterest(uint256) view returns (uint256,uint256)` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_method` | `function getOrder(uint256) view returns (address,uint256,bool,uint8,uint256,uint256,uint256,uint256,address,uint256,bool)` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_method` | `function getOrderDetails(uint256) view returns ((uint256,address,uint256,bool,uint8,uint8,uint8,bool,uint256,uint256,uint256,uint256,uint256,uint256,uint256,uint64,bytes16,uint64))` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_method` | `function getPosition(uint256) view returns (address,uint256,bool,uint256,uint256,address,uint256,uint256,uint256,bool)` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_method` | `function getPositionDetails(uint256) view returns ((uint256,address,uint256,bool,uint256,uint256,uint256,uint256,int256,int256,uint256,uint256,uint256,uint256))` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_method` | `function getPositionFunding(uint256) view returns (int256)` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_method` | `function getPositionMargin(uint256) view returns ((address,uint256,uint256,bool))` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_method` | `function getProtocolAccounts() view returns (address,address)` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_method` | `function getUserMarketPosition(address,uint256) view returns (uint256)` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_method` | `function getUserOrderIds(address) view returns (uint256[])` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_method` | `function getUserPositionIds(address) view returns (uint256[])` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_method` | `function getVault(address) view returns (address)` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_method` | `function isMarketActive(uint256) view returns (bool)` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_method` | `function liquidate(uint256) nonpayable returns ()` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_method` | `function openPosition(uint256,address,uint256,uint256,bool) nonpayable returns (uint256)` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_method` | `function partialClose(uint256,uint256) nonpayable returns ()` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_method` | `function placeLimitOrder(uint256,bool,uint256,uint256,uint256,address,uint256) nonpayable returns (uint256)` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_method` | `function placeOrder((uint256,bool,uint8,uint8,uint256,uint256,uint256,uint256,uint256,uint256,bool,uint256,uint256,uint64,bytes16)) nonpayable returns (uint256)` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_method` | `function placeStopLimitOrder(uint256,bool,uint256,uint256,uint256,uint256,address,uint256) nonpayable returns (uint256)` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_method` | `function placeStopLossOrder(uint256,uint256) nonpayable returns (uint256)` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_method` | `function placeTakeProfitOrder(uint256,uint256) nonpayable returns (uint256)` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_method` | `function quoteClosePosition(uint256) view returns ((uint256,int256,uint256,uint256,int256,int256,uint256))` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_method` | `function quoteMarket(uint256) view returns ((uint256,uint256,uint256,int256,int256,uint256,uint256,uint256,uint256,bool,bool))` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_method` | `function quoteOpenPosition(uint256,address,uint256,uint256,bool) view returns ((uint256,uint256,uint256,uint256,uint256,uint256,uint256,uint256,int256,uint256))` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_method` | `function quotePartialClose(uint256,uint256) view returns (uint256,int256,uint256,uint256)` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_method` | `function removeCollateral(uint256,uint256) nonpayable returns ()` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_method` | `function setMarginMode(uint8) nonpayable returns ()` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_method` | `function setPositionTpSl(uint256,uint256,uint256) nonpayable returns ()` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_method` | `function topUpPosition(uint256,uint256) nonpayable returns ()` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_method` | `function totalMarkets() view returns (uint256)` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_method` | `function transferCollateral(address,uint256) nonpayable returns ()` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_method` | `function transferMargin(uint256,uint256,uint256) nonpayable returns ()` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `precompile_method` | `function withdraw(address,uint256) nonpayable returns ()` | [precompiles/perps/abi/abi.json](../../../precompiles/perps/abi/abi.json) |
| `go_exported` | `type PerpsKeepers interface { PerpsK() *perpskeeper.Keeper }` | [precompiles/perps/perps.go](../../../precompiles/perps/perps.go) |
| `go_exported` | `var GasTable = pcommon.GasTable{` | [precompiles/perps/perps.go](../../../precompiles/perps/perps.go) |
| `go_exported` | `func NewPrecompile(keepers utils.Keepers) (*pcommon.Precompile, error)` | [precompiles/perps/perps.go](../../../precompiles/perps/perps.go) |

## perps-errors

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `func RegisteredErrors() []*sdkerrors.Error` | [modules/perps/types/errors.go](../../../modules/perps/types/errors.go) |

## perps-events

| Kind | Declaration | Source |
|---|---|---|
| `event` | `EventVaultCreated = "VaultCreated"` | [modules/perps/types/events.go](../../../modules/perps/types/events.go) |
| `event` | `EventCollateralDeposited = "CollateralDeposited"` | [modules/perps/types/events.go](../../../modules/perps/types/events.go) |
| `event` | `EventCollateralWithdrawn = "CollateralWithdrawn"` | [modules/perps/types/events.go](../../../modules/perps/types/events.go) |
| `event` | `EventCollateralTransferred = "CollateralTransferred"` | [modules/perps/types/events.go](../../../modules/perps/types/events.go) |
| `event` | `EventOrderPlaced = "OrderPlaced"` | [modules/perps/types/events.go](../../../modules/perps/types/events.go) |
| `event` | `EventOrderOptions = "OrderOptions"` | [modules/perps/types/events.go](../../../modules/perps/types/events.go) |
| `event` | `EventOrderCancelled = "OrderCancelled"` | [modules/perps/types/events.go](../../../modules/perps/types/events.go) |
| `event` | `EventOrderExpired = "OrderExpired"` | [modules/perps/types/events.go](../../../modules/perps/types/events.go) |
| `event` | `EventOrderTriggered = "OrderTriggered"` | [modules/perps/types/events.go](../../../modules/perps/types/events.go) |
| `event` | `EventOrderExecuted = "OrderExecuted"` | [modules/perps/types/events.go](../../../modules/perps/types/events.go) |
| `event` | `EventFill = "Fill"` | [modules/perps/types/events.go](../../../modules/perps/types/events.go) |
| `event` | `EventPositionOpened = "PositionOpened"` | [modules/perps/types/events.go](../../../modules/perps/types/events.go) |
| `event` | `EventPositionModified = "PositionModified"` | [modules/perps/types/events.go](../../../modules/perps/types/events.go) |
| `event` | `EventPositionEntryPriceChanged = "PositionEntryPriceChanged"` | [modules/perps/types/events.go](../../../modules/perps/types/events.go) |
| `event` | `EventPositionClosed = "PositionClosed"` | [modules/perps/types/events.go](../../../modules/perps/types/events.go) |
| `event` | `EventPositionTpSlSet = "PositionTpSlSet"` | [modules/perps/types/events.go](../../../modules/perps/types/events.go) |
| `event` | `EventTradeSettled = "TradeSettled"` | [modules/perps/types/events.go](../../../modules/perps/types/events.go) |
| `event` | `EventFeeCollected = "FeeCollected"` | [modules/perps/types/events.go](../../../modules/perps/types/events.go) |
| `event` | `EventMarkPriceUpdated = "MarkPriceUpdated"` | [modules/perps/types/events.go](../../../modules/perps/types/events.go) |
| `event` | `EventFundingRateUpdated = "FundingRateUpdated"` | [modules/perps/types/events.go](../../../modules/perps/types/events.go) |
| `event` | `EventFundingSettled = "FundingSettled"` | [modules/perps/types/events.go](../../../modules/perps/types/events.go) |
| `event` | `EventPositionFundingApplied = "PositionFundingApplied"` | [modules/perps/types/events.go](../../../modules/perps/types/events.go) |
| `event` | `EventOpenInterestChanged = "OpenInterestChanged"` | [modules/perps/types/events.go](../../../modules/perps/types/events.go) |
| `event` | `EventLiquidation = "Liquidation"` | [modules/perps/types/events.go](../../../modules/perps/types/events.go) |
| `event` | `EventBackstopLiquidation = "BackstopLiquidation"` | [modules/perps/types/events.go](../../../modules/perps/types/events.go) |
| `event` | `EventInsuranceContribution = "InsuranceContribution"` | [modules/perps/types/events.go](../../../modules/perps/types/events.go) |
| `event` | `EventInsuranceDraw = "InsuranceDraw"` | [modules/perps/types/events.go](../../../modules/perps/types/events.go) |
| `event` | `EventVaultDeficit = "VaultDeficit"` | [modules/perps/types/events.go](../../../modules/perps/types/events.go) |
| `event` | `EventADLExecuted = "ADLExecuted"` | [modules/perps/types/events.go](../../../modules/perps/types/events.go) |
| `event` | `EventADLMatched = "ADLMatched"` | [modules/perps/types/events.go](../../../modules/perps/types/events.go) |
| source | — | [modules/perps/keeper/endblock.go](../../../modules/perps/keeper/endblock.go) |

## perps-governance

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message MsgSetMarket { string authority = 1; bytes market = 2 [(gogoproto.customtype) = "Market", (gogoproto.nullable) = false]; }` | [api/perps/tx.proto](../../../api/perps/tx.proto) |
| `proto_message` | `message MsgSetMarketResponse {}` | [api/perps/tx.proto](../../../api/perps/tx.proto) |
| `proto_message` | `message MsgSetPaused { string authority = 1; bool paused = 2; }` | [api/perps/tx.proto](../../../api/perps/tx.proto) |
| `proto_message` | `message MsgSetPausedResponse {}` | [api/perps/tx.proto](../../../api/perps/tx.proto) |
| `proto_message` | `message MsgUpdateParams { string authority = 1; bytes params = 2 [(gogoproto.customtype) = "Params", (gogoproto.nullable) = false]; }` | [api/perps/tx.proto](../../../api/perps/tx.proto) |
| `proto_message` | `message MsgUpdateParamsResponse {}` | [api/perps/tx.proto](../../../api/perps/tx.proto) |
| `go_exported` | `func NewMsgServerImpl(k *Keeper) types.MsgServer` | [modules/perps/keeper/msg_server.go](../../../modules/perps/keeper/msg_server.go) |
| `go_exported` | `func (k *Keeper) SetPaused(ctx sdk.Context, authority string, paused bool) error` | [modules/perps/keeper/pause.go](../../../modules/perps/keeper/pause.go) |
| `go_exported` | `func (AppModuleBasic) RegisterInterfaces(registry cdctypes.InterfaceRegistry) {` | [modules/perps/module.go](../../../modules/perps/module.go) |
| `go_exported` | `func (am AppModule) RegisterServices(cfg module.Configurator) {` | [modules/perps/module.go](../../../modules/perps/module.go) |
| `go_exported` | `func RegisterInterfaces(registry cdctypes.InterfaceRegistry)` | [modules/perps/types/codec.go](../../../modules/perps/types/codec.go) |
| `go_exported` | `func RegisterLegacyAminoCodec(cdc *codec.LegacyAmino)` | [modules/perps/types/codec.go](../../../modules/perps/types/codec.go) |

## go-exported-modules-perps

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `func EndBlocker(ctx sdk.Context, k keeper.Keeper)` | [modules/perps/abci.go](../../../modules/perps/abci.go) |
| `go_exported` | `func NewAppModule(k *keeper.Keeper) AppModule` | [modules/perps/module.go](../../../modules/perps/module.go) |

## go-exported-modules-perps-keeper

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `func (k *Keeper) GetAccount(ctx sdk.Context, address common.Address) (types.Account, error)` | [modules/perps/keeper/account.go](../../../modules/perps/keeper/account.go) |
| `go_exported` | `func (k *Keeper) EnsureAccount(ctx sdk.Context, address common.Address) (account types.Account, created bool, err error)` | [modules/perps/keeper/account.go](../../../modules/perps/keeper/account.go) |
| `go_exported` | `func (k *Keeper) Deposit(ctx sdk.Context, address common.Address, amount uint256.Int) ([]types.EventRecord, error)` | [modules/perps/keeper/account.go](../../../modules/perps/keeper/account.go) |
| `go_exported` | `func (k *Keeper) Withdraw(ctx sdk.Context, address common.Address, amount uint256.Int) ([]types.EventRecord, error)` | [modules/perps/keeper/account.go](../../../modules/perps/keeper/account.go) |
| `go_exported` | `func (k *Keeper) TransferCollateral(ctx sdk.Context, from, to common.Address, amount uint256.Int) ([]types.EventRecord, error)` | [modules/perps/keeper/account.go](../../../modules/perps/keeper/account.go) |
| `go_exported` | `func (k *Keeper) EmergencyWithdraw(ctx sdk.Context, address common.Address) (uint256.Int, []types.EventRecord, error)` | [modules/perps/keeper/account.go](../../../modules/perps/keeper/account.go) |
| `go_exported` | `func (k *Keeper) Withdrawable(ctx sdk.Context, address common.Address) (uint256.Int, error)` | [modules/perps/keeper/account.go](../../../modules/perps/keeper/account.go) |
| `go_exported` | `func (k *Keeper) GetPosition(ctx sdk.Context, id uint256.Int) (types.Position, error)` | [modules/perps/keeper/account.go](../../../modules/perps/keeper/account.go) |
| `go_exported` | `func (k *Keeper) AccountPositions(ctx sdk.Context, address common.Address) ([]types.Position, error)` | [modules/perps/keeper/account.go](../../../modules/perps/keeper/account.go) |
| `go_exported` | `func (k *Keeper) AccountOpenOrders(ctx sdk.Context, address common.Address) ([]uint256.Int, error)` | [modules/perps/keeper/account.go](../../../modules/perps/keeper/account.go) |
| `go_exported` | `func (k *Keeper) GetOrder(ctx sdk.Context, orderID uint256.Int) (types.Order, error)` | [modules/perps/keeper/book.go](../../../modules/perps/keeper/book.go) |
| `go_exported` | `func (k *Keeper) BestBid(ctx sdk.Context, marketID uint64) (uint256.Int, bool, error)` | [modules/perps/keeper/book.go](../../../modules/perps/keeper/book.go) |
| `go_exported` | `func (k *Keeper) MidPrice(ctx sdk.Context, marketID uint64) (uint256.Int, bool, error)` | [modules/perps/keeper/book.go](../../../modules/perps/keeper/book.go) |
| `go_exported` | `func (k *Keeper) EndBlock(ctx sdk.Context) []types.EventRecord` | [modules/perps/keeper/endblock.go](../../../modules/perps/keeper/endblock.go) |
| `go_exported` | `func (k *Keeper) FeeTier(ctx sdk.Context, account common.Address) (AccountFeeTier, error)` | [modules/perps/keeper/fees.go](../../../modules/perps/keeper/fees.go) |
| `go_exported` | `func FundingOwed(position types.Position, fundingIndex uint256.Int) (FundingPayment, error)` | [modules/perps/keeper/funding.go](../../../modules/perps/keeper/funding.go) |
| `go_exported` | `func (k *Keeper) InitGenesis(ctx sdk.Context, gs types.GenesisState)` | [modules/perps/keeper/genesis.go](../../../modules/perps/keeper/genesis.go) |
| `go_exported` | `func (k *Keeper) ExportGenesis(ctx sdk.Context) *types.GenesisState` | [modules/perps/keeper/genesis.go](../../../modules/perps/keeper/genesis.go) |
| `go_exported` | `func (k *Keeper) ModuleAddress() sdk.AccAddress { return authtypes.NewModuleAddress(types.ModuleName) }` | [modules/perps/keeper/keeper.go](../../../modules/perps/keeper/keeper.go) |
| `go_exported` | `func (k *Keeper) ModuleEVMAddress() common.Address { return common.BytesToAddress(k.ModuleAddress()) }` | [modules/perps/keeper/keeper.go](../../../modules/perps/keeper/keeper.go) |
| `go_exported` | `func (k *Keeper) FeesAccountAddress() common.Address` | [modules/perps/keeper/keeper.go](../../../modules/perps/keeper/keeper.go) |
| `go_exported` | `func (k *Keeper) InsuranceAccountAddress() common.Address` | [modules/perps/keeper/keeper.go](../../../modules/perps/keeper/keeper.go) |
| `go_exported` | `func (k *Keeper) GetParams(ctx sdk.Context) (types.Params, error)` | [modules/perps/keeper/keeper.go](../../../modules/perps/keeper/keeper.go) |
| `go_exported` | `func (k *Keeper) GetMarket(ctx sdk.Context, marketID uint64) (types.Market, error)` | [modules/perps/keeper/keeper.go](../../../modules/perps/keeper/keeper.go) |
| `go_exported` | `func (k *Keeper) IterateMarkets(ctx sdk.Context, visit func(types.Market) (stop bool)) error` | [modules/perps/keeper/keeper.go](../../../modules/perps/keeper/keeper.go) |
| `go_exported` | `func (k *Keeper) Liquidate(ctx sdk.Context, caller common.Address, positionID uint256.Int) ([]types.EventRecord, error)` | [modules/perps/keeper/liquidation.go](../../../modules/perps/keeper/liquidation.go) |
| `go_exported` | `func (k *Keeper) LiquidationPrice(ctx sdk.Context, positionID uint256.Int) (uint256.Int, error)` | [modules/perps/keeper/liquidation.go](../../../modules/perps/keeper/liquidation.go) |
| `go_exported` | `const MarginModeCross uint8 = 0` | [modules/perps/keeper/margin.go](../../../modules/perps/keeper/margin.go) |
| `go_exported` | `func (m MarginSummary) Liquidatable() bool` | [modules/perps/keeper/margin.go](../../../modules/perps/keeper/margin.go) |
| `go_exported` | `func InitialMargin(notionalUsd sdk.Int, leverage uint256.Int) (sdk.Int, error)` | [modules/perps/keeper/margin.go](../../../modules/perps/keeper/margin.go) |
| `go_exported` | `func CompatibilityNotional(collateralUsd sdk.Int, leverage uint256.Int, takerFeePpm uint64) (sdk.Int, error)` | [modules/perps/keeper/margin.go](../../../modules/perps/keeper/margin.go) |
| `go_exported` | `func (k *Keeper) AccountMargin(ctx sdk.Context, account common.Address) (MarginSummary, error)` | [modules/perps/keeper/margin.go](../../../modules/perps/keeper/margin.go) |
| `go_exported` | `func (k *Keeper) SetMarginMode(ctx sdk.Context, account common.Address, mode uint8) error` | [modules/perps/keeper/margin.go](../../../modules/perps/keeper/margin.go) |
| `go_exported` | `func (k *Keeper) GetMarketState(ctx sdk.Context, marketID uint64) (types.MarketState, error)` | [modules/perps/keeper/mark.go](../../../modules/perps/keeper/mark.go) |
| `go_exported` | `func (k *Keeper) OpenInterest(ctx sdk.Context, marketID uint64) (MarketOpenInterest, error)` | [modules/perps/keeper/openinterest.go](../../../modules/perps/keeper/openinterest.go) |
| `go_exported` | `func (k *Keeper) OpenInterestCap(ctx sdk.Context, marketID uint64) (sdk.Int, error)` | [modules/perps/keeper/openinterest.go](../../../modules/perps/keeper/openinterest.go) |
| `go_exported` | `type OrderParams struct` | [modules/perps/keeper/orders.go](../../../modules/perps/keeper/orders.go) |
| `go_exported` | `type PlaceResult struct` | [modules/perps/keeper/orders.go](../../../modules/perps/keeper/orders.go) |
| `go_exported` | `func (k *Keeper) PlaceOrder(ctx sdk.Context, account common.Address, p OrderParams) (PlaceResult, error)` | [modules/perps/keeper/orders.go](../../../modules/perps/keeper/orders.go) |
| `go_exported` | `func (k *Keeper) CancelOrder(ctx sdk.Context, account common.Address, orderID uint256.Int) (types.EventRecord, error)` | [modules/perps/keeper/orders.go](../../../modules/perps/keeper/orders.go) |
| `go_exported` | `func (k *Keeper) CancelByCloid(ctx sdk.Context, account common.Address, marketID uint64, cloid [16]byte) (types.EventRecord, error)` | [modules/perps/keeper/orders.go](../../../modules/perps/keeper/orders.go) |
| `go_exported` | `func (k *Keeper) ClosePosition(ctx sdk.Context, caller common.Address, positionID uint256.Int) (types.Order, error)` | [modules/perps/keeper/positions.go](../../../modules/perps/keeper/positions.go) |
| `go_exported` | `func (k *Keeper) PartialClose(ctx sdk.Context, caller common.Address, positionID, closeSizeUsd uint256.Int) (types.Order, error)` | [modules/perps/keeper/positions.go](../../../modules/perps/keeper/positions.go) |
| `go_exported` | `func (k *Keeper) ExecuteOrder(ctx sdk.Context, orderID uint256.Int) (uint256.Int, []types.EventRecord, error)` | [modules/perps/keeper/triggers.go](../../../modules/perps/keeper/triggers.go) |
| `go_exported` | `func (k *Keeper) SetPositionTpSl(ctx sdk.Context, caller common.Address, positionID, takeProfitPrice,` | [modules/perps/keeper/triggers.go](../../../modules/perps/keeper/triggers.go) |
| `go_exported` | `func (k *Keeper) GetAttachedTpSl(ctx sdk.Context, positionID uint256.Int) (AttachedTpSl, error)` | [modules/perps/keeper/triggers.go](../../../modules/perps/keeper/triggers.go) |

## go-exported-modules-perps-types

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `const ModuleName = "perps"` | [modules/perps/types/errors.go](../../../modules/perps/types/errors.go) |
| `go_exported` | `type EventRecord struct` | [modules/perps/types/events.go](../../../modules/perps/types/events.go) |
| `go_exported` | `type EVMKeeper interface` | [modules/perps/types/expected_keepers.go](../../../modules/perps/types/expected_keepers.go) |
| `go_exported` | `type BookEntry struct` | [modules/perps/types/genesis.go](../../../modules/perps/types/genesis.go) |
| `go_exported` | `type CloidEntry struct` | [modules/perps/types/genesis.go](../../../modules/perps/types/genesis.go) |
| `go_exported` | `type AccountOrderEntry struct` | [modules/perps/types/genesis.go](../../../modules/perps/types/genesis.go) |
| `go_exported` | `type AccountMarketPositionEntry struct` | [modules/perps/types/genesis.go](../../../modules/perps/types/genesis.go) |
| `go_exported` | `type TriggerEntry struct` | [modules/perps/types/genesis.go](../../../modules/perps/types/genesis.go) |
| `go_exported` | `type ExpiryEntry struct` | [modules/perps/types/genesis.go](../../../modules/perps/types/genesis.go) |
| `go_exported` | `type VolumeBucket struct` | [modules/perps/types/genesis.go](../../../modules/perps/types/genesis.go) |
| `go_exported` | `type VenueVolumeBucket struct` | [modules/perps/types/genesis.go](../../../modules/perps/types/genesis.go) |
| `go_exported` | `type LiquidationCooldown struct` | [modules/perps/types/genesis.go](../../../modules/perps/types/genesis.go) |
| `go_exported` | `type AttachedTpSl struct` | [modules/perps/types/genesis.go](../../../modules/perps/types/genesis.go) |
| `go_exported` | `type MarginRatioEntry struct` | [modules/perps/types/genesis.go](../../../modules/perps/types/genesis.go) |
| `go_exported` | `type AdlRankEntry struct` | [modules/perps/types/genesis.go](../../../modules/perps/types/genesis.go) |
| `go_exported` | `func DefaultGenesis() *GenesisState` | [modules/perps/types/genesis.go](../../../modules/perps/types/genesis.go) |
| `go_exported` | `func ValidateGenesis(gs GenesisState) error` | [modules/perps/types/genesis.go](../../../modules/perps/types/genesis.go) |
| `go_exported` | `StoreKey = ModuleName` | [modules/perps/types/keys.go](../../../modules/perps/types/keys.go) |
| `go_exported` | `FeesAccountName = "perps_fees"` | [modules/perps/types/keys.go](../../../modules/perps/types/keys.go) |
| `go_exported` | `InsuranceAccountName = "perps_insurance"` | [modules/perps/types/keys.go](../../../modules/perps/types/keys.go) |
| `go_exported` | `SequenceOrderID byte = 0x01` | [modules/perps/types/keys.go](../../../modules/perps/types/keys.go) |
| `go_exported` | `SequencePositionID byte = 0x02` | [modules/perps/types/keys.go](../../../modules/perps/types/keys.go) |
| `go_exported` | `SequencePlacement byte = 0x03` | [modules/perps/types/keys.go](../../../modules/perps/types/keys.go) |
| `go_exported` | `func BookPriceKey(isBid bool, price uint256.Int) []byte` | [modules/perps/types/keys.go](../../../modules/perps/types/keys.go) |
| `go_exported` | `func MarketKey(marketID uint64) []byte { return join(MarketPrefix, uint64Bytes(marketID)) }` | [modules/perps/types/keys.go](../../../modules/perps/types/keys.go) |
| `go_exported` | `func MarketStateKey(marketID uint64) []byte { return join(MarketStatePrefix, uint64Bytes(marketID)) }` | [modules/perps/types/keys.go](../../../modules/perps/types/keys.go) |
| `go_exported` | `func AccountKey(account common.Address) []byte` | [modules/perps/types/keys.go](../../../modules/perps/types/keys.go) |
| `go_exported` | `func OrderKey(orderID uint256.Int) []byte { return join(OrderPrefix, uint256Bytes(orderID)) }` | [modules/perps/types/keys.go](../../../modules/perps/types/keys.go) |
| `go_exported` | `func BookSidePrefix(marketID uint64, side byte) []byte` | [modules/perps/types/keys.go](../../../modules/perps/types/keys.go) |
| `go_exported` | `func BookKey(marketID uint64, side byte, priceKey []byte, sequence uint64) []byte` | [modules/perps/types/keys.go](../../../modules/perps/types/keys.go) |
| `go_exported` | `func CloidKey(account common.Address, marketID uint64, cloid [16]byte) []byte` | [modules/perps/types/keys.go](../../../modules/perps/types/keys.go) |
| `go_exported` | `func AccountOrderPrefixFor(account common.Address) []byte` | [modules/perps/types/keys.go](../../../modules/perps/types/keys.go) |
| `go_exported` | `func AccountOrderKey(account common.Address, orderID uint256.Int) []byte` | [modules/perps/types/keys.go](../../../modules/perps/types/keys.go) |
| `go_exported` | `func PositionKey(positionID uint256.Int) []byte` | [modules/perps/types/keys.go](../../../modules/perps/types/keys.go) |
| `go_exported` | `func AccountMarketPositionKey(account common.Address, marketID uint64) []byte` | [modules/perps/types/keys.go](../../../modules/perps/types/keys.go) |
| `go_exported` | `func TriggerDirectionPrefix(marketID uint64, direction byte) []byte` | [modules/perps/types/keys.go](../../../modules/perps/types/keys.go) |
| `go_exported` | `func TriggerKey(marketID uint64, direction byte, priceKey []byte, sequence uint64) []byte` | [modules/perps/types/keys.go](../../../modules/perps/types/keys.go) |
| `go_exported` | `func ExpiryKey(deadline uint64, orderID uint256.Int) []byte` | [modules/perps/types/keys.go](../../../modules/perps/types/keys.go) |
| `go_exported` | `func VolumeKey(account common.Address, day uint64) []byte` | [modules/perps/types/keys.go](../../../modules/perps/types/keys.go) |
| `go_exported` | `func VenueVolumeKey(day uint64) []byte { return join(VenueVolumePrefix, uint64Bytes(day)) }` | [modules/perps/types/keys.go](../../../modules/perps/types/keys.go) |
| `go_exported` | `func LiquidationCooldownKey(positionID uint256.Int) []byte` | [modules/perps/types/keys.go](../../../modules/perps/types/keys.go) |
| `go_exported` | `const LiquidationCooldownValueLength = 8 + 32` | [modules/perps/types/keys.go](../../../modules/perps/types/keys.go) |
| `go_exported` | `func LiquidationCooldownValue(lastSliceTime uint64, sliceBasis uint256.Int) []byte` | [modules/perps/types/keys.go](../../../modules/perps/types/keys.go) |
| `go_exported` | `func SequenceKey(name byte) []byte { return join(SequencePrefix, []byte{name}) }` | [modules/perps/types/keys.go](../../../modules/perps/types/keys.go) |
| `go_exported` | `func AttachedTpSlKey(positionID uint256.Int) []byte` | [modules/perps/types/keys.go](../../../modules/perps/types/keys.go) |
| `go_exported` | `func MarginRatioIndexKey(bucket uint16, account common.Address) []byte` | [modules/perps/types/keys.go](../../../modules/perps/types/keys.go) |
| `go_exported` | `func AdlRankSidePrefix(marketID uint64, side byte) []byte` | [modules/perps/types/keys.go](../../../modules/perps/types/keys.go) |
| `go_exported` | `func AdlRankIndexKey(marketID uint64, side byte, rankScore, positionID uint256.Int) []byte` | [modules/perps/types/keys.go](../../../modules/perps/types/keys.go) |
| `go_exported` | `VenueMaxLeverage = 1000` | [modules/perps/types/market.go](../../../modules/perps/types/market.go) |
| `go_exported` | `CollateralDecimals = 6` | [modules/perps/types/market.go](../../../modules/perps/types/market.go) |
| `go_exported` | `func (m Market) Validate() error` | [modules/perps/types/market.go](../../../modules/perps/types/market.go) |
| `go_exported` | `func UsdToUsdlDown(usd sdk.Int) (usdl sdk.Int, residueUsd sdk.Int, err error)` | [modules/perps/types/math.go](../../../modules/perps/types/math.go) |
| `go_exported` | `func UsdToUsdlUp(usd sdk.Int) (usdl sdk.Int, residueUsd sdk.Int, err error)` | [modules/perps/types/math.go](../../../modules/perps/types/math.go) |
| `go_exported` | `PpmDenominator = 1_000_000` | [modules/perps/types/params.go](../../../modules/perps/types/params.go) |
| `go_exported` | `USDDecimals = 18` | [modules/perps/types/params.go](../../../modules/perps/types/params.go) |
| `go_exported` | `func DefaultParams() Params` | [modules/perps/types/params.go](../../../modules/perps/types/params.go) |
| `go_exported` | `func (p Params) Validate() error` | [modules/perps/types/params.go](../../../modules/perps/types/params.go) |
| `go_exported` | `SideBuy byte = 1` | [modules/perps/types/state.go](../../../modules/perps/types/state.go) |
| `go_exported` | `SideSell byte = 2` | [modules/perps/types/state.go](../../../modules/perps/types/state.go) |
| `go_exported` | `TriggerRising byte = 1` | [modules/perps/types/state.go](../../../modules/perps/types/state.go) |
| `go_exported` | `TriggerFalling byte = 2` | [modules/perps/types/state.go](../../../modules/perps/types/state.go) |
| `go_exported` | `func SideOf(isLong bool) byte` | [modules/perps/types/state.go](../../../modules/perps/types/state.go) |
| `go_exported` | `type OrderType uint8` | [modules/perps/types/state.go](../../../modules/perps/types/state.go) |
| `go_exported` | `OrderTypeLimit OrderType = iota` | [modules/perps/types/state.go](../../../modules/perps/types/state.go) |
| `go_exported` | `OrderTypeStopLimit` | [modules/perps/types/state.go](../../../modules/perps/types/state.go) |
| `go_exported` | `OrderTypeTakeProfit` | [modules/perps/types/state.go](../../../modules/perps/types/state.go) |
| `go_exported` | `OrderTypeStopLoss` | [modules/perps/types/state.go](../../../modules/perps/types/state.go) |
| `go_exported` | `OrderTypeMarket` | [modules/perps/types/state.go](../../../modules/perps/types/state.go) |
| `go_exported` | `OrderTypeStopMarket` | [modules/perps/types/state.go](../../../modules/perps/types/state.go) |
| `go_exported` | `type TimeInForce uint8` | [modules/perps/types/state.go](../../../modules/perps/types/state.go) |
| `go_exported` | `TimeInForceGTC TimeInForce = iota` | [modules/perps/types/state.go](../../../modules/perps/types/state.go) |
| `go_exported` | `TimeInForceIOC` | [modules/perps/types/state.go](../../../modules/perps/types/state.go) |
| `go_exported` | `TimeInForceFOK` | [modules/perps/types/state.go](../../../modules/perps/types/state.go) |
| `go_exported` | `TimeInForcePostOnly` | [modules/perps/types/state.go](../../../modules/perps/types/state.go) |
| `go_exported` | `type OrderStatus uint8` | [modules/perps/types/state.go](../../../modules/perps/types/state.go) |
| `go_exported` | `OrderStatusUntriggered OrderStatus = iota` | [modules/perps/types/state.go](../../../modules/perps/types/state.go) |
| `go_exported` | `OrderStatusResting` | [modules/perps/types/state.go](../../../modules/perps/types/state.go) |
| `go_exported` | `OrderStatusFilled` | [modules/perps/types/state.go](../../../modules/perps/types/state.go) |
| `go_exported` | `OrderStatusCancelled` | [modules/perps/types/state.go](../../../modules/perps/types/state.go) |
| `go_exported` | `OrderStatusExpired` | [modules/perps/types/state.go](../../../modules/perps/types/state.go) |
| `go_exported` | `type Order struct` | [modules/perps/types/state.go](../../../modules/perps/types/state.go) |
| `go_exported` | `type Position struct` | [modules/perps/types/state.go](../../../modules/perps/types/state.go) |
| `go_exported` | `type Account struct` | [modules/perps/types/state.go](../../../modules/perps/types/state.go) |
| `go_exported` | `type MarketState struct` | [modules/perps/types/state.go](../../../modules/perps/types/state.go) |
| `go_exported` | `type VenueBooks struct` | [modules/perps/types/state.go](../../../modules/perps/types/state.go) |
| `go_exported` | `func (o Order) Marshal() []byte` | [modules/perps/types/state.go](../../../modules/perps/types/state.go) |
| `go_exported` | `func (o *Order) Unmarshal(bz []byte) error` | [modules/perps/types/state.go](../../../modules/perps/types/state.go) |
| `go_exported` | `func (p Position) Marshal() []byte` | [modules/perps/types/state.go](../../../modules/perps/types/state.go) |
| `go_exported` | `func (p *Position) Unmarshal(bz []byte) error` | [modules/perps/types/state.go](../../../modules/perps/types/state.go) |
| `go_exported` | `func (a Account) Marshal() []byte` | [modules/perps/types/state.go](../../../modules/perps/types/state.go) |
| `go_exported` | `func (a *Account) Unmarshal(bz []byte) error` | [modules/perps/types/state.go](../../../modules/perps/types/state.go) |
| `go_exported` | `func (m MarketState) Marshal() []byte` | [modules/perps/types/state.go](../../../modules/perps/types/state.go) |
| `go_exported` | `func (m *MarketState) Unmarshal(bz []byte) error` | [modules/perps/types/state.go](../../../modules/perps/types/state.go) |
| `go_exported` | `func (v VenueBooks) Marshal() []byte` | [modules/perps/types/state.go](../../../modules/perps/types/state.go) |
| `go_exported` | `func (v *VenueBooks) Unmarshal(bz []byte) error` | [modules/perps/types/state.go](../../../modules/perps/types/state.go) |
| `go_exported` | `type TierClass uint8` | [modules/perps/types/tiers.go](../../../modules/perps/types/tiers.go) |
| `go_exported` | `TierClassA TierClass = iota // BTC and ETH` | [modules/perps/types/tiers.go](../../../modules/perps/types/tiers.go) |
| `go_exported` | `TierClassB // major alternative assets` | [modules/perps/types/tiers.go](../../../modules/perps/types/tiers.go) |
| `go_exported` | `TierClassC // indices and gold` | [modules/perps/types/tiers.go](../../../modules/perps/types/tiers.go) |
| `go_exported` | `TierClassD // single equities` | [modules/perps/types/tiers.go](../../../modules/perps/types/tiers.go) |
| `go_exported` | `TierClassE // native and new listings` | [modules/perps/types/tiers.go](../../../modules/perps/types/tiers.go) |
| `go_exported` | `TierClassF // LINK` | [modules/perps/types/tiers.go](../../../modules/perps/types/tiers.go) |
| `go_exported` | `BpsDenominator = 10_000` | [modules/perps/types/tiers.go](../../../modules/perps/types/tiers.go) |
| `go_exported` | `SecondsPerDay = 86_400` | [modules/perps/types/tiers.go](../../../modules/perps/types/tiers.go) |
| `go_exported` | `type MarginTier struct` | [modules/perps/types/tiers.go](../../../modules/perps/types/tiers.go) |
| `go_exported` | `func TiersFor(market Market) ([]MarginTier, error)` | [modules/perps/types/tiers.go](../../../modules/perps/types/tiers.go) |
| `go_exported` | `func OiCapInForce(market Market, params Params, blockTime time.Time) (sdk.Int, error)` | [modules/perps/types/tiers.go](../../../modules/perps/types/tiers.go) |

## go-exported-precompiles-perps-abi

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `func ABI() (abi.ABI, error)` | [precompiles/perps/abi/abi.go](../../../precompiles/perps/abi/abi.go) |
| `go_exported` | `func PackEvent(name string, args ...interface{}) ([]common.Hash, []byte, error)` | [precompiles/perps/abi/pack.go](../../../precompiles/perps/abi/pack.go) |

<a id="pricefeed"></a>

## pricefeed-0x1020

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `func SubmissionIsGasless(ctx sdk.Context, msg *types.MsgSubmitPrices, k keeper.Keeper) (bool, error)` | [modules/pricefeed/ante.go](../../../modules/pricefeed/ante.go) |
| `go_exported` | `func CheckSubmission(ctx sdk.Context, tx sdk.Tx, k keeper.Keeper) error` | [modules/pricefeed/ante.go](../../../modules/pricefeed/ante.go) |
| `go_exported` | `func MarkSubmission(ctx sdk.Context, tx sdk.Tx, k keeper.Keeper) error` | [modules/pricefeed/ante.go](../../../modules/pricefeed/ante.go) |
| `go_exported` | `func InitV6120(ctx sdk.Context, k Keeper) error` | [modules/pricefeed/keeper/init_v6120.go](../../../modules/pricefeed/keeper/init_v6120.go) |
| `go_exported` | `func NewKeeper(storeKey sdk.StoreKey, stakingKeeper types.StakingKeeper, accountKeeper types.AccountKeeper,` | [modules/pricefeed/keeper/keeper.go](../../../modules/pricefeed/keeper/keeper.go) |
| `go_exported` | `func (k Keeper) MarketSymbol(ctx sdk.Context, marketId uint64) (string, bool, error)` | [modules/pricefeed/keeper/price.go](../../../modules/pricefeed/keeper/price.go) |
| `go_exported` | `func (k Keeper) GetPrice(ctx sdk.Context, marketId uint64) (Price, error)` | [modules/pricefeed/keeper/price.go](../../../modules/pricefeed/keeper/price.go) |
| `go_exported` | `func (k Keeper) GetPriceBySymbol(ctx sdk.Context, symbol string) (Price, error)` | [modules/pricefeed/keeper/price.go](../../../modules/pricefeed/keeper/price.go) |
| `go_exported` | `func (AppModuleBasic) ValidateGenesis(_ codec.JSONCodec, _ client.TxEncodingConfig, bz json.RawMessage) error` | [modules/pricefeed/module.go](../../../modules/pricefeed/module.go) |
| `genesis_param` | `type Market struct` | [modules/pricefeed/types/market.go](../../../modules/pricefeed/types/market.go) |
| `genesis_param` | `type Params struct` | [modules/pricefeed/types/params.go](../../../modules/pricefeed/types/params.go) |
| `storage_key` | `ParamsKey = []byte{0x01}` | [modules/pricefeed/types/state.go](../../../modules/pricefeed/types/state.go) |
| `storage_key` | `FeedKeyPrefix = []byte{0x02}` | [modules/pricefeed/types/state.go](../../../modules/pricefeed/types/state.go) |
| `storage_key` | `FeedAccountPrefix = []byte{0x03}` | [modules/pricefeed/types/state.go](../../../modules/pricefeed/types/state.go) |
| `storage_key` | `ObservationPrefix = []byte{0x04}` | [modules/pricefeed/types/state.go](../../../modules/pricefeed/types/state.go) |
| `storage_key` | `AggregatePrefix = []byte{0x05}` | [modules/pricefeed/types/state.go](../../../modules/pricefeed/types/state.go) |
| `storage_key` | `LastSubmitPrefix = []byte{0x06}` | [modules/pricefeed/types/state.go](../../../modules/pricefeed/types/state.go) |
| `genesis_param` | `type GenesisState struct` | [modules/pricefeed/types/state.go](../../../modules/pricefeed/types/state.go) |
| `precompile_method` | `function registerFeedKey(bytes) nonpayable returns (address)` | [precompiles/pricefeed/abi/abi.json](../../../precompiles/pricefeed/abi/abi.json) |
| `precompile_method` | `function revokeFeedKey() nonpayable returns ()` | [precompiles/pricefeed/abi/abi.json](../../../precompiles/pricefeed/abi/abi.json) |
| `precompile_method` | `function getPrice(uint256) view returns ((uint256,uint256,uint64,int64,uint32,bool))` | [precompiles/pricefeed/abi/abi.json](../../../precompiles/pricefeed/abi/abi.json) |
| `precompile_method` | `function getPriceBySymbol(string) view returns ((uint256,uint256,uint64,int64,uint32,bool))` | [precompiles/pricefeed/abi/abi.json](../../../precompiles/pricefeed/abi/abi.json) |
| `precompile_method` | `function getPrices(uint256[]) view returns ((uint256,uint256,uint64,int64,uint32,bool)[])` | [precompiles/pricefeed/abi/abi.json](../../../precompiles/pricefeed/abi/abi.json) |
| `precompile_method` | `function getMarket(uint256) view returns ((uint64,string,string,uint256,uint256,bool))` | [precompiles/pricefeed/abi/abi.json](../../../precompiles/pricefeed/abi/abi.json) |
| `precompile_method` | `function getMarketCount() view returns (uint256)` | [precompiles/pricefeed/abi/abi.json](../../../precompiles/pricefeed/abi/abi.json) |
| `precompile_method` | `function marketIdOf(string) view returns (uint256)` | [precompiles/pricefeed/abi/abi.json](../../../precompiles/pricefeed/abi/abi.json) |
| `precompile_method` | `function getObservation(uint256,address) view returns ((uint64,uint256,uint256,uint64,int64))` | [precompiles/pricefeed/abi/abi.json](../../../precompiles/pricefeed/abi/abi.json) |
| `precompile_method` | `function getFeedKey(address) view returns (bytes,address)` | [precompiles/pricefeed/abi/abi.json](../../../precompiles/pricefeed/abi/abi.json) |
| `precompile_method` | `function getValidatorByFeedAccount(address) view returns (address)` | [precompiles/pricefeed/abi/abi.json](../../../precompiles/pricefeed/abi/abi.json) |
| `precompile_method` | `function getParams() view returns ((uint64,uint64,uint32,uint32,uint32,uint32,uint32,uint64,uint32,uint64))` | [precompiles/pricefeed/abi/abi.json](../../../precompiles/pricefeed/abi/abi.json) |
| `precompile_event` | `event FeedKeyRegistered(address,address,bytes,address)` | [precompiles/pricefeed/abi/abi.json](../../../precompiles/pricefeed/abi/abi.json) |
| `precompile_event` | `event FeedKeyRevoked(address,address)` | [precompiles/pricefeed/abi/abi.json](../../../precompiles/pricefeed/abi/abi.json) |
| `go_exported` | `PriceFeedAddress = "0x0000000000000000000000000000000000001020"` | [precompiles/pricefeed/pricefeed.go](../../../precompiles/pricefeed/pricefeed.go) |
| `go_exported` | `type PriceFeedKeepers interface` | [precompiles/pricefeed/pricefeed.go](../../../precompiles/pricefeed/pricefeed.go) |
| `go_exported` | `func NewPrecompile(keepers utils.Keepers) (*Precompile, error)` | [precompiles/pricefeed/pricefeed.go](../../../precompiles/pricefeed/pricefeed.go) |

## pricefeed-errors

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `func LXPCodes() []LXPCode` | [modules/pricefeed/types/errors.go](../../../modules/pricefeed/types/errors.go) |
| `go_exported` | `func LXPName(err error) (string, bool)` | [modules/pricefeed/types/errors.go](../../../modules/pricefeed/types/errors.go) |
| source | — | [precompiles/pricefeed/pricefeed.go](../../../precompiles/pricefeed/pricefeed.go) |
| source | — | [precompiles/pricefeed/views.go](../../../precompiles/pricefeed/views.go) |

## pricefeed-events

| Kind | Declaration | Source |
|---|---|---|
| source | — | [modules/pricefeed/types/events.go](../../../modules/pricefeed/types/events.go) |

## pricefeed-query

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message QueryFeedKeyRequest` | [api/pricefeed/query.proto](../../../api/pricefeed/query.proto) |
| `proto_message` | `message QueryFeedKeyResponse` | [api/pricefeed/query.proto](../../../api/pricefeed/query.proto) |
| `proto_message` | `message QueryObservationRequest` | [api/pricefeed/query.proto](../../../api/pricefeed/query.proto) |
| `proto_message` | `message QueryObservationResponse` | [api/pricefeed/query.proto](../../../api/pricefeed/query.proto) |
| `proto_message` | `message QueryParamsRequest` | [api/pricefeed/query.proto](../../../api/pricefeed/query.proto) |
| `proto_message` | `message QueryParamsResponse` | [api/pricefeed/query.proto](../../../api/pricefeed/query.proto) |
| `proto_message` | `message QueryPriceRequest` | [api/pricefeed/query.proto](../../../api/pricefeed/query.proto) |
| `proto_message` | `message QueryPriceResponse` | [api/pricefeed/query.proto](../../../api/pricefeed/query.proto) |
| `proto_message` | `message QueryPricesRequest` | [api/pricefeed/query.proto](../../../api/pricefeed/query.proto) |
| `proto_message` | `message QueryPricesResponse` | [api/pricefeed/query.proto](../../../api/pricefeed/query.proto) |
| `proto_message` | `message QueryValidatorByFeedAccountRequest` | [api/pricefeed/query.proto](../../../api/pricefeed/query.proto) |
| `proto_message` | `message QueryValidatorByFeedAccountResponse` | [api/pricefeed/query.proto](../../../api/pricefeed/query.proto) |
| `cli_command` | `pricefeed` | [modules/pricefeed/client/cli/query.go](../../../modules/pricefeed/client/cli/query.go) |
| `go_exported` | `func NewQueryServerImpl(k Keeper) types.QueryServer` | [modules/pricefeed/keeper/grpc_query.go](../../../modules/pricefeed/keeper/grpc_query.go) |
| `go_exported` | `GetQueryCmd` | [modules/pricefeed/module.go](../../../modules/pricefeed/module.go) |
| `go_exported` | `func (am AppModule) RegisterServices(cfg module.Configurator)` | [modules/pricefeed/module.go](../../../modules/pricefeed/module.go) |
| source | — | [modules/pricefeed/client/cli/query_feeds.go](../../../modules/pricefeed/client/cli/query_feeds.go) |
| source | — | [modules/pricefeed/client/cli/query_prices.go](../../../modules/pricefeed/client/cli/query_prices.go) |

## msg-pricefeed-setparams

| Kind | Declaration | Source |
|---|---|---|
| source | — | [api/pricefeed/tx.proto](../../../api/pricefeed/tx.proto) |

## msg-pricefeed-submitprices

| Kind | Declaration | Source |
|---|---|---|
| source | — | [api/pricefeed/tx.proto](../../../api/pricefeed/tx.proto) |

## proto-pricefeed-market

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message Market {` | [api/pricefeed/tx.proto](../../../api/pricefeed/tx.proto) |

## proto-pricefeed-msgsetparams

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message MsgSetParams {` | [api/pricefeed/tx.proto](../../../api/pricefeed/tx.proto) |

## proto-pricefeed-msgsetparamsresponse

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `MsgSetParamsResponse` | [api/pricefeed/tx.proto](../../../api/pricefeed/tx.proto) |

## proto-pricefeed-msgsubmitprices

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message MsgSubmitPrices {` | [api/pricefeed/tx.proto](../../../api/pricefeed/tx.proto) |

## proto-pricefeed-msgsubmitpricesresponse

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message MsgSubmitPricesResponse {` | [api/pricefeed/tx.proto](../../../api/pricefeed/tx.proto) |

## proto-pricefeed-observation

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message Observation {` | [api/pricefeed/tx.proto](../../../api/pricefeed/tx.proto) |

## go-exported-modules-pricefeed

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `func NewAppModule(k keeper.Keeper) AppModule { return AppModule{keeper: k} }` | [modules/pricefeed/module.go](../../../modules/pricefeed/module.go) |

## go-exported-modules-pricefeed-keeper

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `func (k Keeper) Aggregate(ctx sdk.Context)` | [modules/pricefeed/keeper/aggregate.go](../../../modules/pricefeed/keeper/aggregate.go) |
| `go_exported` | `func (k Keeper) InitGenesis(ctx sdk.Context, genesis types.GenesisState) error` | [modules/pricefeed/keeper/genesis.go](../../../modules/pricefeed/keeper/genesis.go) |
| `go_exported` | `func (k Keeper) ExportGenesis(ctx sdk.Context) (types.GenesisState, error)` | [modules/pricefeed/keeper/genesis.go](../../../modules/pricefeed/keeper/genesis.go) |
| `go_exported` | `func NewMsgServerImpl(k Keeper) types.MsgServer { return msgServer{keeper: k} }` | [modules/pricefeed/keeper/msg_server.go](../../../modules/pricefeed/keeper/msg_server.go) |
| `go_exported` | `func (k Keeper) GetLastSubmissionHeight(ctx sdk.Context, operator sdk.ValAddress) (int64, bool)` | [modules/pricefeed/keeper/msg_server.go](../../../modules/pricefeed/keeper/msg_server.go) |
| `go_exported` | `func (k Keeper) SetLastSubmissionHeight(ctx sdk.Context, operator sdk.ValAddress, height int64)` | [modules/pricefeed/keeper/msg_server.go](../../../modules/pricefeed/keeper/msg_server.go) |
| `go_exported` | `func (k Keeper) GetObservation(ctx sdk.Context, marketID uint64, operator sdk.ValAddress) (types.ObservationRecord, bool)` | [modules/pricefeed/keeper/observation.go](../../../modules/pricefeed/keeper/observation.go) |
| `go_exported` | `func (k Keeper) GetParams(ctx sdk.Context) types.Params` | [modules/pricefeed/keeper/params.go](../../../modules/pricefeed/keeper/params.go) |
| `go_exported` | `func (k Keeper) RequireBonded(ctx sdk.Context, operator sdk.ValAddress) error` | [modules/pricefeed/keeper/registry.go](../../../modules/pricefeed/keeper/registry.go) |
| `go_exported` | `func (k Keeper) GetFeedKey(ctx sdk.Context, operator sdk.ValAddress) (types.FeedKey, bool)` | [modules/pricefeed/keeper/registry.go](../../../modules/pricefeed/keeper/registry.go) |
| `go_exported` | `func (k Keeper) GetValidatorByFeedAccount(ctx sdk.Context, feedAccount sdk.AccAddress) (sdk.ValAddress, bool)` | [modules/pricefeed/keeper/registry.go](../../../modules/pricefeed/keeper/registry.go) |

## go-exported-modules-pricefeed-types

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `func TargetHeight(ctx sdk.Context) int64` | [modules/pricefeed/types/ante.go](../../../modules/pricefeed/types/ante.go) |
| `go_exported` | `func IsSubmissionTx(tx sdk.Tx) bool` | [modules/pricefeed/types/ante.go](../../../modules/pricefeed/types/ante.go) |
| `go_exported` | `func CheckSubmissionAlone(msgs []sdk.Msg) (bool, error)` | [modules/pricefeed/types/ante.go](../../../modules/pricefeed/types/ante.go) |
| `go_exported` | `func RegisterCodec(cdc *codec.LegacyAmino)` | [modules/pricefeed/types/codec.go](../../../modules/pricefeed/types/codec.go) |
| `go_exported` | `func RegisterInterfaces(registry cdctypes.InterfaceRegistry)` | [modules/pricefeed/types/codec.go](../../../modules/pricefeed/types/codec.go) |
| `go_exported` | `const ModuleName = "pricefeed"` | [modules/pricefeed/types/errors.go](../../../modules/pricefeed/types/errors.go) |
| `go_exported` | `AttributeValidator = "validator"` | [modules/pricefeed/types/events.go](../../../modules/pricefeed/types/events.go) |
| `go_exported` | `AttributeMarketID = "market_id"` | [modules/pricefeed/types/events.go](../../../modules/pricefeed/types/events.go) |
| `go_exported` | `AttributeSequence = "sequence"` | [modules/pricefeed/types/events.go](../../../modules/pricefeed/types/events.go) |
| `go_exported` | `AttributeIndexPrice = "index_price"` | [modules/pricefeed/types/events.go](../../../modules/pricefeed/types/events.go) |
| `go_exported` | `AttributeMarkPrice = "mark_price"` | [modules/pricefeed/types/events.go](../../../modules/pricefeed/types/events.go) |
| `go_exported` | `AttributeSourceTimestampMs = "source_timestamp_ms"` | [modules/pricefeed/types/events.go](../../../modules/pricefeed/types/events.go) |
| `go_exported` | `AttributeReason = "reason"` | [modules/pricefeed/types/events.go](../../../modules/pricefeed/types/events.go) |
| `go_exported` | `AttributeBasis = "basis"` | [modules/pricefeed/types/events.go](../../../modules/pricefeed/types/events.go) |
| `go_exported` | `AttributeTimestampMs = "timestamp_ms"` | [modules/pricefeed/types/events.go](../../../modules/pricefeed/types/events.go) |
| `go_exported` | `AttributeSources = "sources"` | [modules/pricefeed/types/events.go](../../../modules/pricefeed/types/events.go) |
| `go_exported` | `AttributeRequired = "required"` | [modules/pricefeed/types/events.go](../../../modules/pricefeed/types/events.go) |
| `go_exported` | `AttributeHeight = "height"` | [modules/pricefeed/types/events.go](../../../modules/pricefeed/types/events.go) |
| `go_exported` | `AttributeFeedAccount = "feed_account"` | [modules/pricefeed/types/events.go](../../../modules/pricefeed/types/events.go) |
| `go_exported` | `AttributePreviousFeedAccount = "previous_feed_account"` | [modules/pricefeed/types/events.go](../../../modules/pricefeed/types/events.go) |
| `go_exported` | `AttributeAuthority = "authority"` | [modules/pricefeed/types/events.go](../../../modules/pricefeed/types/events.go) |
| `go_exported` | `RefusalUnknownMarket = "unknown_market"` | [modules/pricefeed/types/events.go](../../../modules/pricefeed/types/events.go) |
| `go_exported` | `RefusalMarketDisabled = "market_disabled"` | [modules/pricefeed/types/events.go](../../../modules/pricefeed/types/events.go) |
| `go_exported` | `RefusalSequence = "sequence"` | [modules/pricefeed/types/events.go](../../../modules/pricefeed/types/events.go) |
| `go_exported` | `RefusalFuture = "future"` | [modules/pricefeed/types/events.go](../../../modules/pricefeed/types/events.go) |
| `go_exported` | `RefusalStale = "stale"` | [modules/pricefeed/types/events.go](../../../modules/pricefeed/types/events.go) |
| `go_exported` | `RefusalBounds = "bounds"` | [modules/pricefeed/types/events.go](../../../modules/pricefeed/types/events.go) |
| `go_exported` | `RefusalDeviation = "deviation"` | [modules/pricefeed/types/events.go](../../../modules/pricefeed/types/events.go) |
| `go_exported` | `type StakingKeeper interface` | [modules/pricefeed/types/expected_keepers.go](../../../modules/pricefeed/types/expected_keepers.go) |
| `go_exported` | `type AccountKeeper interface` | [modules/pricefeed/types/expected_keepers.go](../../../modules/pricefeed/types/expected_keepers.go) |
| `go_exported` | `type EVMKeeper interface` | [modules/pricefeed/types/expected_keepers.go](../../../modules/pricefeed/types/expected_keepers.go) |
| `go_exported` | `func (m MsgSubmitPrices) GetSigners() []sdk.AccAddress { return singleSigner(m.Feeder) }` | [modules/pricefeed/types/msgs.go](../../../modules/pricefeed/types/msgs.go) |
| `go_exported` | `func (m MsgSetParams) GetSigners() []sdk.AccAddress { return singleSigner(m.Authority) }` | [modules/pricefeed/types/msgs.go](../../../modules/pricefeed/types/msgs.go) |
| `go_exported` | `func (m MsgSubmitPrices) ValidateBasic() error` | [modules/pricefeed/types/msgs.go](../../../modules/pricefeed/types/msgs.go) |
| `go_exported` | `func (m MsgSetParams) ValidateBasic() error` | [modules/pricefeed/types/msgs.go](../../../modules/pricefeed/types/msgs.go) |
| `go_exported` | `BpsDenominator uint32 = 10000` | [modules/pricefeed/types/params.go](../../../modules/pricefeed/types/params.go) |
| `go_exported` | `func DefaultAuthority() string` | [modules/pricefeed/types/params.go](../../../modules/pricefeed/types/params.go) |
| `go_exported` | `func DefaultParams(authority string) Params` | [modules/pricefeed/types/params.go](../../../modules/pricefeed/types/params.go) |
| `go_exported` | `func (p Params) Validate() error` | [modules/pricefeed/types/params.go](../../../modules/pricefeed/types/params.go) |
| `go_exported` | `const StoreKey = ModuleName` | [modules/pricefeed/types/state.go](../../../modules/pricefeed/types/state.go) |
| `go_exported` | `func FeedKeyKey(operator sdk.ValAddress) []byte { return join(FeedKeyPrefix, operator) }` | [modules/pricefeed/types/state.go](../../../modules/pricefeed/types/state.go) |
| `go_exported` | `func FeedAccountKey(feedAccount sdk.AccAddress) []byte { return join(FeedAccountPrefix, feedAccount) }` | [modules/pricefeed/types/state.go](../../../modules/pricefeed/types/state.go) |
| `go_exported` | `func ObservationKey(marketID uint64, operator sdk.ValAddress) []byte` | [modules/pricefeed/types/state.go](../../../modules/pricefeed/types/state.go) |
| `go_exported` | `func AggregateKey(marketID uint64) []byte { return join(AggregatePrefix, u64(marketID)) }` | [modules/pricefeed/types/state.go](../../../modules/pricefeed/types/state.go) |
| `go_exported` | `func LastSubmitKey(operator sdk.ValAddress) []byte { return join(LastSubmitPrefix, operator) }` | [modules/pricefeed/types/state.go](../../../modules/pricefeed/types/state.go) |
| `go_exported` | `type FeedKey struct` | [modules/pricefeed/types/state.go](../../../modules/pricefeed/types/state.go) |
| `go_exported` | `type ObservationRecord struct` | [modules/pricefeed/types/state.go](../../../modules/pricefeed/types/state.go) |
| `go_exported` | `type Aggregate struct` | [modules/pricefeed/types/state.go](../../../modules/pricefeed/types/state.go) |
| `go_exported` | `type GenesisFeedKey struct` | [modules/pricefeed/types/state.go](../../../modules/pricefeed/types/state.go) |
| `go_exported` | `type GenesisObservation struct` | [modules/pricefeed/types/state.go](../../../modules/pricefeed/types/state.go) |
| `go_exported` | `type GenesisAggregate struct` | [modules/pricefeed/types/state.go](../../../modules/pricefeed/types/state.go) |
| `go_exported` | `func DefaultGenesis() *GenesisState` | [modules/pricefeed/types/state.go](../../../modules/pricefeed/types/state.go) |

## go-exported-precompiles-pricefeed-abi

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `var JSON string` | [precompiles/pricefeed/abi/abi.go](../../../precompiles/pricefeed/abi/abi.go) |

<a id="oracle"></a>

## oracle-0x1008

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `func NewKeeper(cdc codec.BinaryCodec, storeKey sdk.StoreKey, memKey sdk.StoreKey,` | [modules/oracle/keeper/keeper.go](../../../modules/oracle/keeper/keeper.go) |
| `storage_key` | `ExchangeRateKey = []byte{0x01}` | [modules/oracle/types/keys.go](../../../modules/oracle/types/keys.go) |
| `storage_key` | `FeederDelegationKey = []byte{0x02}` | [modules/oracle/types/keys.go](../../../modules/oracle/types/keys.go) |
| `storage_key` | `VotePenaltyCounterKey = []byte{0x03}` | [modules/oracle/types/keys.go](../../../modules/oracle/types/keys.go) |
| `storage_key` | `AggregateExchangeRateVoteKey = []byte{0x05}` | [modules/oracle/types/keys.go](../../../modules/oracle/types/keys.go) |
| `storage_key` | `VoteTargetKey = []byte{0x06}` | [modules/oracle/types/keys.go](../../../modules/oracle/types/keys.go) |
| `storage_key` | `PriceSnapshotKey = []byte{0x07}` | [modules/oracle/types/keys.go](../../../modules/oracle/types/keys.go) |
| `storage_key` | `SpamPreventionCounter = []byte{0x08}` | [modules/oracle/types/keys.go](../../../modules/oracle/types/keys.go) |
| `precompile_method` | `function getExchangeRates() view returns ((string,(string,string,int64))[])` | [precompiles/oracle/abi.json](../../../precompiles/oracle/abi.json) |
| `precompile_method` | `function getOracleTwaps(uint64) view returns ((string,string,int64)[])` | [precompiles/oracle/abi.json](../../../precompiles/oracle/abi.json) |
| `go_exported` | `OracleAddress = "0x0000000000000000000000000000000000001008"` | [precompiles/oracle/oracle.go](../../../precompiles/oracle/oracle.go) |
| `go_exported` | `var ErrOraclePrecompileRetired = errors.New("oracle precompile is retired; oracle data queries are disabled")` | [precompiles/oracle/oracle.go](../../../precompiles/oracle/oracle.go) |
| `go_exported` | `func GetVersioned(latestUpgrade string, keepers utils.Keepers) utils.VersionedPrecompiles` | [precompiles/oracle/setup.go](../../../precompiles/oracle/setup.go) |

## oracle-errors

| Kind | Declaration | Source |
|---|---|---|
| source | — | [modules/oracle/types/errors.go](../../../modules/oracle/types/errors.go) |

## oracle-events

| Kind | Declaration | Source |
|---|---|---|
| source | — | [modules/oracle/keeper/msg_server.go](../../../modules/oracle/keeper/msg_server.go) |
| source | — | [modules/oracle/types/events.go](../../../modules/oracle/types/events.go) |

## msg-oracle-aggregateexchangeratevote

| Kind | Declaration | Source |
|---|---|---|
| source | — | [api/oracle/tx.proto](../../../api/oracle/tx.proto) |

## msg-oracle-delegatefeedconsent

| Kind | Declaration | Source |
|---|---|---|
| source | — | [api/oracle/tx.proto](../../../api/oracle/tx.proto) |

## proto-oracle-aggregateexchangeratevote

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message AggregateExchangeRateVote {` | [api/oracle/oracle.proto](../../../api/oracle/oracle.proto) |

## proto-oracle-denom

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message Denom {` | [api/oracle/oracle.proto](../../../api/oracle/oracle.proto) |

## proto-oracle-denomoracleexchangeratepair

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message DenomOracleExchangeRatePair {` | [api/oracle/query.proto](../../../api/oracle/query.proto) |

## proto-oracle-exchangeratetuple

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message ExchangeRateTuple {` | [api/oracle/oracle.proto](../../../api/oracle/oracle.proto) |

## proto-oracle-feederdelegation

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message FeederDelegation {` | [api/oracle/genesis.proto](../../../api/oracle/genesis.proto) |

## proto-oracle-genesisstate

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message GenesisState {` | [api/oracle/genesis.proto](../../../api/oracle/genesis.proto) |

## proto-oracle-msgaggregateexchangeratevote

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message MsgAggregateExchangeRateVote {` | [api/oracle/tx.proto](../../../api/oracle/tx.proto) |

## proto-oracle-msgaggregateexchangeratevoteresponse

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `MsgAggregateExchangeRateVoteResponse` | [api/oracle/tx.proto](../../../api/oracle/tx.proto) |

## proto-oracle-msgdelegatefeedconsent

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message MsgDelegateFeedConsent {` | [api/oracle/tx.proto](../../../api/oracle/tx.proto) |

## proto-oracle-msgdelegatefeedconsentresponse

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `MsgDelegateFeedConsentResponse` | [api/oracle/tx.proto](../../../api/oracle/tx.proto) |

## proto-oracle-oracleexchangerate

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message OracleExchangeRate {` | [api/oracle/oracle.proto](../../../api/oracle/oracle.proto) |

## proto-oracle-oracletwap

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message OracleTwap {` | [api/oracle/oracle.proto](../../../api/oracle/oracle.proto) |

## proto-oracle-params

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message Params {` | [api/oracle/oracle.proto](../../../api/oracle/oracle.proto) |

## proto-oracle-penaltycounter

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message PenaltyCounter {` | [api/oracle/genesis.proto](../../../api/oracle/genesis.proto) |

## proto-oracle-pricesnapshot

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message PriceSnapshot {` | [api/oracle/oracle.proto](../../../api/oracle/oracle.proto) |

## proto-oracle-pricesnapshotitem

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message PriceSnapshotItem {` | [api/oracle/oracle.proto](../../../api/oracle/oracle.proto) |

## proto-oracle-queryactivesrequest

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `QueryActivesRequest` | [api/oracle/query.proto](../../../api/oracle/query.proto) |

## proto-oracle-queryactivesresponse

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message QueryActivesResponse {` | [api/oracle/query.proto](../../../api/oracle/query.proto) |

## proto-oracle-queryexchangeraterequest

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message QueryExchangeRateRequest {` | [api/oracle/query.proto](../../../api/oracle/query.proto) |

## proto-oracle-queryexchangerateresponse

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message QueryExchangeRateResponse {` | [api/oracle/query.proto](../../../api/oracle/query.proto) |

## proto-oracle-queryexchangeratesrequest

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `QueryExchangeRatesRequest` | [api/oracle/query.proto](../../../api/oracle/query.proto) |

## proto-oracle-queryexchangeratesresponse

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message QueryExchangeRatesResponse {` | [api/oracle/query.proto](../../../api/oracle/query.proto) |

## proto-oracle-queryfeederdelegationrequest

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message QueryFeederDelegationRequest {` | [api/oracle/query.proto](../../../api/oracle/query.proto) |

## proto-oracle-queryfeederdelegationresponse

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message QueryFeederDelegationResponse {` | [api/oracle/query.proto](../../../api/oracle/query.proto) |

## proto-oracle-queryparamsrequest

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `QueryParamsRequest` | [api/oracle/query.proto](../../../api/oracle/query.proto) |

## proto-oracle-queryparamsresponse

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message QueryParamsResponse {` | [api/oracle/query.proto](../../../api/oracle/query.proto) |

## proto-oracle-querypricesnapshothistoryrequest

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `QueryPriceSnapshotHistoryRequest` | [api/oracle/query.proto](../../../api/oracle/query.proto) |

## proto-oracle-querypricesnapshothistoryresponse

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message QueryPriceSnapshotHistoryResponse {` | [api/oracle/query.proto](../../../api/oracle/query.proto) |

## proto-oracle-queryslashwindowrequest

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `QuerySlashWindowRequest` | [api/oracle/query.proto](../../../api/oracle/query.proto) |

## proto-oracle-queryslashwindowresponse

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message QuerySlashWindowResponse {` | [api/oracle/query.proto](../../../api/oracle/query.proto) |

## proto-oracle-querytwapsrequest

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message QueryTwapsRequest {` | [api/oracle/query.proto](../../../api/oracle/query.proto) |

## proto-oracle-querytwapsresponse

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message QueryTwapsResponse {` | [api/oracle/query.proto](../../../api/oracle/query.proto) |

## proto-oracle-queryvotepenaltycounterrequest

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message QueryVotePenaltyCounterRequest {` | [api/oracle/query.proto](../../../api/oracle/query.proto) |

## proto-oracle-queryvotepenaltycounterresponse

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message QueryVotePenaltyCounterResponse {` | [api/oracle/query.proto](../../../api/oracle/query.proto) |

## proto-oracle-queryvotetargetsrequest

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `QueryVoteTargetsRequest` | [api/oracle/query.proto](../../../api/oracle/query.proto) |

## proto-oracle-queryvotetargetsresponse

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message QueryVoteTargetsResponse {` | [api/oracle/query.proto](../../../api/oracle/query.proto) |

## proto-oracle-votepenaltycounter

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message VotePenaltyCounter {` | [api/oracle/oracle.proto](../../../api/oracle/oracle.proto) |

## cli-oracle-query-actives

| Kind | Declaration | Source |
|---|---|---|
| source | — | [modules/oracle/client/cli/query.go](../../../modules/oracle/client/cli/query.go) |

## cli-oracle-query-exchange-rates

| Kind | Declaration | Source |
|---|---|---|
| source | — | [modules/oracle/client/cli/query.go](../../../modules/oracle/client/cli/query.go) |

## cli-oracle-query-feeder

| Kind | Declaration | Source |
|---|---|---|
| source | — | [modules/oracle/client/cli/query.go](../../../modules/oracle/client/cli/query.go) |

## cli-oracle-query-params

| Kind | Declaration | Source |
|---|---|---|
| source | — | [modules/oracle/client/cli/query.go](../../../modules/oracle/client/cli/query.go) |

## cli-oracle-query-price-snapshot-history

| Kind | Declaration | Source |
|---|---|---|
| source | — | [modules/oracle/client/cli/query.go](../../../modules/oracle/client/cli/query.go) |

## cli-oracle-query-root

| Kind | Declaration | Source |
|---|---|---|
| `cli_command` | `paxd query oracle` | [modules/oracle/client/cli/query.go](../../../modules/oracle/client/cli/query.go) |

## cli-oracle-query-twaps

| Kind | Declaration | Source |
|---|---|---|
| source | — | [modules/oracle/client/cli/query.go](../../../modules/oracle/client/cli/query.go) |

## cli-oracle-query-vote-penalty-counter

| Kind | Declaration | Source |
|---|---|---|
| source | — | [modules/oracle/client/cli/query.go](../../../modules/oracle/client/cli/query.go) |

## cli-oracle-query-vote-targets

| Kind | Declaration | Source |
|---|---|---|
| source | — | [modules/oracle/client/cli/query.go](../../../modules/oracle/client/cli/query.go) |

## cli-oracle-tx-aggregate-vote

| Kind | Declaration | Source |
|---|---|---|
| source | — | [modules/oracle/client/cli/tx.go](../../../modules/oracle/client/cli/tx.go) |

## cli-oracle-tx-root

| Kind | Declaration | Source |
|---|---|---|
| `cli_command` | `paxd tx oracle` | [modules/oracle/client/cli/tx.go](../../../modules/oracle/client/cli/tx.go) |

## cli-oracle-tx-set-feeder

| Kind | Declaration | Source |
|---|---|---|
| source | — | [modules/oracle/client/cli/tx.go](../../../modules/oracle/client/cli/tx.go) |

## go-exported-modules-oracle

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `func EndBlocker(ctx sdk.Context, k keeper.Keeper)` | [modules/oracle/abci.go](../../../modules/oracle/abci.go) |
| `go_exported` | `func NewSpammingPreventionDecorator(oracleKeeper keeper.Keeper) SpammingPreventionDecorator` | [modules/oracle/ante.go](../../../modules/oracle/ante.go) |
| `go_exported` | `func NewOracleVoteAloneDecorator() VoteAloneDecorator` | [modules/oracle/ante.go](../../../modules/oracle/ante.go) |
| `go_exported` | `type AppModuleBasic struct` | [modules/oracle/module.go](../../../modules/oracle/module.go) |
| `go_exported` | `func NewAppModule(` | [modules/oracle/module.go](../../../modules/oracle/module.go) |

## go-exported-modules-oracle-client-rest

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `func RegisterRoutes(clientCtx client.Context, rtr *mux.Router)` | [modules/oracle/client/rest/rest.go](../../../modules/oracle/client/rest/rest.go) |

## go-exported-modules-oracle-client-wasm

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `type OracleWasmQueryHandler struct` | [modules/oracle/client/wasm/query.go](../../../modules/oracle/client/wasm/query.go) |
| `go_exported` | `func NewOracleWasmQueryHandler(keeper *oraclekeeper.Keeper) *OracleWasmQueryHandler` | [modules/oracle/client/wasm/query.go](../../../modules/oracle/client/wasm/query.go) |
| `go_exported` | `func (handler OracleWasmQueryHandler) GetExchangeRates(ctx sdk.Context) (*types.QueryExchangeRatesResponse, error)` | [modules/oracle/client/wasm/query.go](../../../modules/oracle/client/wasm/query.go) |
| `go_exported` | `func (handler OracleWasmQueryHandler) GetOracleTwaps(ctx sdk.Context, req *types.QueryTwapsRequest) (*types.QueryTwapsResponse, error)` | [modules/oracle/client/wasm/query.go](../../../modules/oracle/client/wasm/query.go) |

## go-exported-modules-oracle-keeper

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `func (k Keeper) GetOracleAccount(ctx sdk.Context) authtypes.ModuleAccountI` | [modules/oracle/keeper/alias_functions.go](../../../modules/oracle/keeper/alias_functions.go) |
| `go_exported` | `func (k Keeper) OrganizeBallotByDenom(ctx sdk.Context, validatorClaimMap map[string]types.Claim) (votes map[string]types.ExchangeRateBallot)` | [modules/oracle/keeper/ballot.go](../../../modules/oracle/keeper/ballot.go) |
| `go_exported` | `func (k Keeper) ClearBallots(ctx sdk.Context, _ uint64)` | [modules/oracle/keeper/ballot.go](../../../modules/oracle/keeper/ballot.go) |
| `go_exported` | `func (k Keeper) ApplyWhitelist(ctx sdk.Context, whitelist types.DenomList, voteTargets map[string]types.Denom)` | [modules/oracle/keeper/ballot.go](../../../modules/oracle/keeper/ballot.go) |
| `go_exported` | `func (k Keeper) SetBaseExchangeRate(ctx sdk.Context, denom string, exchangeRate sdk.Dec)` | [modules/oracle/keeper/keeper.go](../../../modules/oracle/keeper/keeper.go) |
| `go_exported` | `func (k Keeper) SetBaseExchangeRateWithEvent(ctx sdk.Context, denom string, exchangeRate sdk.Dec)` | [modules/oracle/keeper/keeper.go](../../../modules/oracle/keeper/keeper.go) |
| `go_exported` | `func (k Keeper) IterateBaseExchangeRates(ctx sdk.Context, handler func(denom string, exchangeRate types.OracleExchangeRate) (stop bool))` | [modules/oracle/keeper/keeper.go](../../../modules/oracle/keeper/keeper.go) |
| `go_exported` | `func (k Keeper) RemoveExcessFeeds(ctx sdk.Context)` | [modules/oracle/keeper/keeper.go](../../../modules/oracle/keeper/keeper.go) |
| `go_exported` | `func (k Keeper) GetFeederDelegation(ctx sdk.Context, operator sdk.ValAddress) sdk.AccAddress` | [modules/oracle/keeper/keeper.go](../../../modules/oracle/keeper/keeper.go) |
| `go_exported` | `func (k Keeper) SetFeederDelegation(ctx sdk.Context, operator sdk.ValAddress, delegatedFeeder sdk.AccAddress)` | [modules/oracle/keeper/keeper.go](../../../modules/oracle/keeper/keeper.go) |
| `go_exported` | `func (k Keeper) IterateFeederDelegations(ctx sdk.Context,` | [modules/oracle/keeper/keeper.go](../../../modules/oracle/keeper/keeper.go) |
| `go_exported` | `func (k Keeper) SetVotePenaltyCounter(ctx sdk.Context, operator sdk.ValAddress, missCount, abstainCount, successCount uint64)` | [modules/oracle/keeper/keeper.go](../../../modules/oracle/keeper/keeper.go) |
| `go_exported` | `func (k Keeper) IncrementMissCount(ctx sdk.Context, operator sdk.ValAddress)` | [modules/oracle/keeper/keeper.go](../../../modules/oracle/keeper/keeper.go) |
| `go_exported` | `func (k Keeper) IncrementAbstainCount(ctx sdk.Context, operator sdk.ValAddress)` | [modules/oracle/keeper/keeper.go](../../../modules/oracle/keeper/keeper.go) |
| `go_exported` | `func (k Keeper) IncrementSuccessCount(ctx sdk.Context, operator sdk.ValAddress)` | [modules/oracle/keeper/keeper.go](../../../modules/oracle/keeper/keeper.go) |
| `go_exported` | `func (k Keeper) IterateVotePenaltyCounters(ctx sdk.Context,` | [modules/oracle/keeper/keeper.go](../../../modules/oracle/keeper/keeper.go) |
| `go_exported` | `func (k Keeper) GetAggregateExchangeRateVote(ctx sdk.Context, voter sdk.ValAddress) (aggregateVote types.AggregateExchangeRateVote, err error)` | [modules/oracle/keeper/keeper.go](../../../modules/oracle/keeper/keeper.go) |
| `go_exported` | `func (k Keeper) SetAggregateExchangeRateVote(ctx sdk.Context, voter sdk.ValAddress, vote types.AggregateExchangeRateVote)` | [modules/oracle/keeper/keeper.go](../../../modules/oracle/keeper/keeper.go) |
| `go_exported` | `func (k Keeper) IterateAggregateExchangeRateVotes(ctx sdk.Context, handler func(voterAddr sdk.ValAddress, aggregateVote types.AggregateExchangeRateVote) (stop bool))` | [modules/oracle/keeper/keeper.go](../../../modules/oracle/keeper/keeper.go) |
| `go_exported` | `func (k Keeper) SetVoteTarget(ctx sdk.Context, denom string)` | [modules/oracle/keeper/keeper.go](../../../modules/oracle/keeper/keeper.go) |
| `go_exported` | `func (k Keeper) IterateVoteTargets(ctx sdk.Context, handler func(denom string, denomInfo types.Denom) (stop bool))` | [modules/oracle/keeper/keeper.go](../../../modules/oracle/keeper/keeper.go) |
| `go_exported` | `func (k Keeper) ValidateFeeder(ctx sdk.Context, feederAddr sdk.AccAddress, validatorAddr sdk.ValAddress) error` | [modules/oracle/keeper/keeper.go](../../../modules/oracle/keeper/keeper.go) |
| `go_exported` | `func (k Keeper) AddPriceSnapshot(ctx sdk.Context, snapshot types.PriceSnapshot)` | [modules/oracle/keeper/keeper.go](../../../modules/oracle/keeper/keeper.go) |
| `go_exported` | `func (k Keeper) IteratePriceSnapshots(ctx sdk.Context, handler func(snapshot types.PriceSnapshot) (stop bool))` | [modules/oracle/keeper/keeper.go](../../../modules/oracle/keeper/keeper.go) |
| `go_exported` | `func (k Keeper) CheckAndSetSpamPreventionCounter(ctx sdk.Context, validatorAddr sdk.ValAddress) error` | [modules/oracle/keeper/keeper.go](../../../modules/oracle/keeper/keeper.go) |
| `go_exported` | `func NewMigrator(keeper Keeper, history ...VotePeriodHistory) Migrator {` | [modules/oracle/keeper/migrations.go](../../../modules/oracle/keeper/migrations.go) |
| `go_exported` | `func NewMsgServerImpl(keeper Keeper) types.MsgServer` | [modules/oracle/keeper/msg_server.go](../../../modules/oracle/keeper/msg_server.go) |
| `go_exported` | `func (k Keeper) VoteThreshold(ctx sdk.Context) (res sdk.Dec)` | [modules/oracle/keeper/params.go](../../../modules/oracle/keeper/params.go) |
| `go_exported` | `func (k Keeper) GetParams(ctx sdk.Context) (params types.Params)` | [modules/oracle/keeper/params.go](../../../modules/oracle/keeper/params.go) |
| `go_exported` | `func (k Keeper) SetParams(ctx sdk.Context, params types.Params)` | [modules/oracle/keeper/params.go](../../../modules/oracle/keeper/params.go) |
| `go_exported` | `func NewQuerier(keeper Keeper) types.QueryServer` | [modules/oracle/keeper/querier.go](../../../modules/oracle/keeper/querier.go) |
| `go_exported` | `func (k Keeper) SlashAndResetCounters(ctx sdk.Context)` | [modules/oracle/keeper/slash.go](../../../modules/oracle/keeper/slash.go) |

## go-exported-modules-oracle-simulation

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `func NewDecodeStore(cdc codec.Codec) func(kvA, kvB kv.Pair) string` | [modules/oracle/simulation/decoder.go](../../../modules/oracle/simulation/decoder.go) |
| `go_exported` | `func RandomizedGenState(simState *module.SimulationState)` | [modules/oracle/simulation/genesis.go](../../../modules/oracle/simulation/genesis.go) |
| `go_exported` | `func WeightedOperations(` | [modules/oracle/simulation/operations.go](../../../modules/oracle/simulation/operations.go) |
| `go_exported` | `func ParamChanges(_ *rand.Rand) []simtypes.ParamChange` | [modules/oracle/simulation/params.go](../../../modules/oracle/simulation/params.go) |

## go-exported-modules-oracle-types

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `func NewVoteForTally(rate sdk.Dec, denom string, voter sdk.ValAddress, power int64) VoteForTally` | [modules/oracle/types/ballot.go](../../../modules/oracle/types/ballot.go) |
| `go_exported` | `type ExchangeRateBallot []VoteForTally` | [modules/oracle/types/ballot.go](../../../modules/oracle/types/ballot.go) |
| `go_exported` | `func (pb ExchangeRateBallot) ToMap() map[string]sdk.Dec` | [modules/oracle/types/ballot.go](../../../modules/oracle/types/ballot.go) |
| `go_exported` | `func (pb ExchangeRateBallot) ToCrossRateWithSort(bases map[string]sdk.Dec) (cb ExchangeRateBallot)` | [modules/oracle/types/ballot.go](../../../modules/oracle/types/ballot.go) |
| `go_exported` | `func (pb ExchangeRateBallot) Power() int64` | [modules/oracle/types/ballot.go](../../../modules/oracle/types/ballot.go) |
| `go_exported` | `func (pb ExchangeRateBallot) WeightedMedianWithAssertion() sdk.Dec` | [modules/oracle/types/ballot.go](../../../modules/oracle/types/ballot.go) |
| `go_exported` | `func (pb ExchangeRateBallot) StandardDeviation(median sdk.Dec) sdk.Dec {` | [modules/oracle/types/ballot.go](../../../modules/oracle/types/ballot.go) |
| `go_exported` | `type Claim struct` | [modules/oracle/types/ballot.go](../../../modules/oracle/types/ballot.go) |
| `go_exported` | `func NewClaim(power, weight, winCount int64, recipient sdk.ValAddress, didVote bool) Claim` | [modules/oracle/types/ballot.go](../../../modules/oracle/types/ballot.go) |
| `go_exported` | `func RegisterCodec(cdc *codec.LegacyAmino)` | [modules/oracle/types/codec.go](../../../modules/oracle/types/codec.go) |
| `go_exported` | `func RegisterInterfaces(registry codectypes.InterfaceRegistry)` | [modules/oracle/types/codec.go](../../../modules/oracle/types/codec.go) |
| `go_exported` | `func (d Denom) String() string` | [modules/oracle/types/denom.go](../../../modules/oracle/types/denom.go) |
| `go_exported` | `func (d Denom) Equal(d1 *Denom) bool` | [modules/oracle/types/denom.go](../../../modules/oracle/types/denom.go) |
| `go_exported` | `type DenomList []Denom` | [modules/oracle/types/denom.go](../../../modules/oracle/types/denom.go) |
| `go_exported` | `func (dl DenomList) Contains(denom string) bool` | [modules/oracle/types/denom.go](../../../modules/oracle/types/denom.go) |
| `go_exported` | `func (dl DenomList) String() (out string)` | [modules/oracle/types/denom.go](../../../modules/oracle/types/denom.go) |
| `go_exported` | `AttributeKeyDenom = "denom"` | [modules/oracle/types/events.go](../../../modules/oracle/types/events.go) |
| `go_exported` | `AttributeKeyVoter = "voter"` | [modules/oracle/types/events.go](../../../modules/oracle/types/events.go) |
| `go_exported` | `AttributeKeyExchangeRate = "exchange_rate"` | [modules/oracle/types/events.go](../../../modules/oracle/types/events.go) |
| `go_exported` | `AttributeKeyExchangeRates = "exchange_rates"` | [modules/oracle/types/events.go](../../../modules/oracle/types/events.go) |
| `go_exported` | `AttributeKeyOperator = "operator"` | [modules/oracle/types/events.go](../../../modules/oracle/types/events.go) |
| `go_exported` | `AttributeKeyFeeder = "feeder"` | [modules/oracle/types/events.go](../../../modules/oracle/types/events.go) |
| `go_exported` | `AttributeKeyMissCount = "miss_count"` | [modules/oracle/types/events.go](../../../modules/oracle/types/events.go) |
| `go_exported` | `AttributeKeyAbstainCount = "abstain_count"` | [modules/oracle/types/events.go](../../../modules/oracle/types/events.go) |
| `go_exported` | `AttributeKeySuccessCount = "success_count"` | [modules/oracle/types/events.go](../../../modules/oracle/types/events.go) |
| `go_exported` | `AttributeValueCategory = ModuleName` | [modules/oracle/types/events.go](../../../modules/oracle/types/events.go) |
| `go_exported` | `type StakingKeeper interface` | [modules/oracle/types/expected_keeper.go](../../../modules/oracle/types/expected_keeper.go) |
| `go_exported` | `type DistributionKeeper interface` | [modules/oracle/types/expected_keeper.go](../../../modules/oracle/types/expected_keeper.go) |
| `go_exported` | `type AccountKeeper interface` | [modules/oracle/types/expected_keeper.go](../../../modules/oracle/types/expected_keeper.go) |
| `go_exported` | `type BankKeeper interface` | [modules/oracle/types/expected_keeper.go](../../../modules/oracle/types/expected_keeper.go) |
| `go_exported` | `func NewGenesisState(` | [modules/oracle/types/genesis.go](../../../modules/oracle/types/genesis.go) |
| `go_exported` | `func DefaultGenesisState() *GenesisState` | [modules/oracle/types/genesis.go](../../../modules/oracle/types/genesis.go) |
| `go_exported` | `func ValidateGenesis(data *GenesisState) error` | [modules/oracle/types/genesis.go](../../../modules/oracle/types/genesis.go) |
| `go_exported` | `ModuleName = "oracle"` | [modules/oracle/types/keys.go](../../../modules/oracle/types/keys.go) |
| `go_exported` | `StoreKey = ModuleName` | [modules/oracle/types/keys.go](../../../modules/oracle/types/keys.go) |
| `go_exported` | `MemStoreKey = "oracle_mem"` | [modules/oracle/types/keys.go](../../../modules/oracle/types/keys.go) |
| `go_exported` | `RouterKey = ModuleName` | [modules/oracle/types/keys.go](../../../modules/oracle/types/keys.go) |
| `go_exported` | `QuerierRoute = ModuleName` | [modules/oracle/types/keys.go](../../../modules/oracle/types/keys.go) |
| `go_exported` | `func GetExchangeRateKey(denom string) []byte` | [modules/oracle/types/keys.go](../../../modules/oracle/types/keys.go) |
| `go_exported` | `func GetFeederDelegationKey(v sdk.ValAddress) []byte` | [modules/oracle/types/keys.go](../../../modules/oracle/types/keys.go) |
| `go_exported` | `func GetVotePenaltyCounterKey(v sdk.ValAddress) []byte` | [modules/oracle/types/keys.go](../../../modules/oracle/types/keys.go) |
| `go_exported` | `func GetAggregateExchangeRateVoteKey(v sdk.ValAddress) []byte` | [modules/oracle/types/keys.go](../../../modules/oracle/types/keys.go) |
| `go_exported` | `func GetSpamPreventionCounterKey(v sdk.ValAddress) []byte` | [modules/oracle/types/keys.go](../../../modules/oracle/types/keys.go) |
| `go_exported` | `func GetVoteTargetKey(d string) []byte` | [modules/oracle/types/keys.go](../../../modules/oracle/types/keys.go) |
| `go_exported` | `func ExtractDenomFromVoteTargetKey(key []byte) (denom string)` | [modules/oracle/types/keys.go](../../../modules/oracle/types/keys.go) |
| `go_exported` | `func GetPriceSnapshotKey(timestamp uint64) []byte` | [modules/oracle/types/keys.go](../../../modules/oracle/types/keys.go) |
| `go_exported` | `func GetPriceSnapshotKeyForIteration(timestampA uint64, timestampB uint64) []byte` | [modules/oracle/types/keys.go](../../../modules/oracle/types/keys.go) |
| `go_exported` | `TypeMsgDelegateFeedConsent = "delegate_feeder"` | [modules/oracle/types/msgs.go](../../../modules/oracle/types/msgs.go) |
| `go_exported` | `TypeMsgAggregateExchangeRateVote = "aggregate_exchange_rate_vote"` | [modules/oracle/types/msgs.go](../../../modules/oracle/types/msgs.go) |
| `go_exported` | `func NewMsgAggregateExchangeRateVote(exchangeRates string, feeder sdk.AccAddress, validator sdk.ValAddress) *MsgAggregateExchangeRateVote` | [modules/oracle/types/msgs.go](../../../modules/oracle/types/msgs.go) |
| `go_exported` | `func (msg MsgAggregateExchangeRateVote) Type() string { return TypeMsgAggregateExchangeRateVote }` | [modules/oracle/types/msgs.go](../../../modules/oracle/types/msgs.go) |
| `go_exported` | `func (msg MsgAggregateExchangeRateVote) GetSigners() []sdk.AccAddress` | [modules/oracle/types/msgs.go](../../../modules/oracle/types/msgs.go) |
| `go_exported` | `func (msg MsgAggregateExchangeRateVote) ValidateBasic() error` | [modules/oracle/types/msgs.go](../../../modules/oracle/types/msgs.go) |
| `go_exported` | `func NewMsgDelegateFeedConsent(operatorAddress sdk.ValAddress, feederAddress sdk.AccAddress) *MsgDelegateFeedConsent` | [modules/oracle/types/msgs.go](../../../modules/oracle/types/msgs.go) |
| `go_exported` | `func (msg MsgDelegateFeedConsent) Type() string { return TypeMsgDelegateFeedConsent }` | [modules/oracle/types/msgs.go](../../../modules/oracle/types/msgs.go) |
| `go_exported` | `func (msg MsgDelegateFeedConsent) GetSigners() []sdk.AccAddress` | [modules/oracle/types/msgs.go](../../../modules/oracle/types/msgs.go) |
| `go_exported` | `func (msg MsgDelegateFeedConsent) ValidateBasic() error` | [modules/oracle/types/msgs.go](../../../modules/oracle/types/msgs.go) |
| `go_exported` | `KeyVotePeriod = []byte("VotePeriod")` | [modules/oracle/types/params.go](../../../modules/oracle/types/params.go) |
| `go_exported` | `KeyVoteThreshold = []byte("VoteThreshold")` | [modules/oracle/types/params.go](../../../modules/oracle/types/params.go) |
| `go_exported` | `KeyRewardBand = []byte("RewardBand")` | [modules/oracle/types/params.go](../../../modules/oracle/types/params.go) |
| `go_exported` | `KeyWhitelist = []byte("Whitelist")` | [modules/oracle/types/params.go](../../../modules/oracle/types/params.go) |
| `go_exported` | `KeySlashFraction = []byte("SlashFraction")` | [modules/oracle/types/params.go](../../../modules/oracle/types/params.go) |
| `go_exported` | `KeySlashWindow = []byte("SlashWindow")` | [modules/oracle/types/params.go](../../../modules/oracle/types/params.go) |
| `go_exported` | `KeyMinValidPerWindow = []byte("MinValidPerWindow")` | [modules/oracle/types/params.go](../../../modules/oracle/types/params.go) |
| `go_exported` | `KeyLookbackDuration = []byte("LookbackDuration")` | [modules/oracle/types/params.go](../../../modules/oracle/types/params.go) |
| `go_exported` | `func DefaultParams() Params` | [modules/oracle/types/params.go](../../../modules/oracle/types/params.go) |
| `go_exported` | `func ParamKeyTable() paramstypes.KeyTable` | [modules/oracle/types/params.go](../../../modules/oracle/types/params.go) |
| `go_exported` | `func (p Params) String() string` | [modules/oracle/types/params.go](../../../modules/oracle/types/params.go) |
| `go_exported` | `QueryParameters = "parameters"` | [modules/oracle/types/querier.go](../../../modules/oracle/types/querier.go) |
| `go_exported` | `QueryExchangeRate = "exchangeRate"` | [modules/oracle/types/querier.go](../../../modules/oracle/types/querier.go) |
| `go_exported` | `QueryExchangeRates = "exchangeRates"` | [modules/oracle/types/querier.go](../../../modules/oracle/types/querier.go) |
| `go_exported` | `QueryPriceSnapshotHistory = "priceSnapshotHistory"` | [modules/oracle/types/querier.go](../../../modules/oracle/types/querier.go) |
| `go_exported` | `QueryTwaps = "twaps"` | [modules/oracle/types/querier.go](../../../modules/oracle/types/querier.go) |
| `go_exported` | `QueryActives = "actives"` | [modules/oracle/types/querier.go](../../../modules/oracle/types/querier.go) |
| `go_exported` | `QueryFeederDelegation = "feederDelegation"` | [modules/oracle/types/querier.go](../../../modules/oracle/types/querier.go) |
| `go_exported` | `QueryVotePenaltyCounter = "votePenaltyCounter"` | [modules/oracle/types/querier.go](../../../modules/oracle/types/querier.go) |
| `go_exported` | `QueryVoteTargets = "voteTargets"` | [modules/oracle/types/querier.go](../../../modules/oracle/types/querier.go) |
| `go_exported` | `func NewQueryExchangeRateParams(denom string) QueryExchangeRateParams` | [modules/oracle/types/querier.go](../../../modules/oracle/types/querier.go) |
| `go_exported` | `func NewQueryTwapsParams(lookbackSeconds int64) QueryTwapsParams` | [modules/oracle/types/querier.go](../../../modules/oracle/types/querier.go) |
| `go_exported` | `func NewQueryFeederDelegationParams(validator sdk.ValAddress) QueryFeederDelegationParams` | [modules/oracle/types/querier.go](../../../modules/oracle/types/querier.go) |
| `go_exported` | `func NewQueryVotePenaltyCounterParams(validator sdk.ValAddress) QueryVotePenaltyCounterParams` | [modules/oracle/types/querier.go](../../../modules/oracle/types/querier.go) |
| `go_exported` | `type PriceSnapshots []PriceSnapshot` | [modules/oracle/types/snapshots.go](../../../modules/oracle/types/snapshots.go) |
| `go_exported` | `OracleTwaps []OracleTwap` | [modules/oracle/types/snapshots.go](../../../modules/oracle/types/snapshots.go) |
| `go_exported` | `func (snapshots PriceSnapshots) String() string` | [modules/oracle/types/snapshots.go](../../../modules/oracle/types/snapshots.go) |
| `go_exported` | `func (items PriceSnapshotItems) String() string` | [modules/oracle/types/snapshots.go](../../../modules/oracle/types/snapshots.go) |
| `go_exported` | `func NewAggregateExchangeRateVote(exchangeRateTuples ExchangeRateTuples, voter sdk.ValAddress) AggregateExchangeRateVote` | [modules/oracle/types/vote.go](../../../modules/oracle/types/vote.go) |
| `go_exported` | `func (v AggregateExchangeRateVote) String() string` | [modules/oracle/types/vote.go](../../../modules/oracle/types/vote.go) |
| `go_exported` | `func (v ExchangeRateTuple) String() string` | [modules/oracle/types/vote.go](../../../modules/oracle/types/vote.go) |
| `go_exported` | `func (tuples ExchangeRateTuples) String() string` | [modules/oracle/types/vote.go](../../../modules/oracle/types/vote.go) |
| `go_exported` | `func ParseExchangeRateTuples(tuplesStr string) (ExchangeRateTuples, error)` | [modules/oracle/types/vote.go](../../../modules/oracle/types/vote.go) |
| `go_exported` | `func (ex OracleExchangeRate) String() string` | [modules/oracle/types/vote.go](../../../modules/oracle/types/vote.go) |
| `go_exported` | `func (rates DenomOracleExchangeRatePairs) String() string` | [modules/oracle/types/vote.go](../../../modules/oracle/types/vote.go) |

## go-exported-modules-oracle-utils

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `MicroPaxDenom = "uhpx"` | [modules/oracle/utils/assets.go](../../../modules/oracle/utils/assets.go) |
| `go_exported` | `MicroAtomDenom = "uatom"` | [modules/oracle/utils/assets.go](../../../modules/oracle/utils/assets.go) |
| `go_exported` | `MicroEthDenom = "ueth"` | [modules/oracle/utils/assets.go](../../../modules/oracle/utils/assets.go) |
| `go_exported` | `BlocksPerDay = BlocksPerHour * 24` | [modules/oracle/utils/period.go](../../../modules/oracle/utils/period.go) |
| `go_exported` | `func IsPeriodLastBlock(ctx sdk.Context, blocksPerPeriod uint64) bool` | [modules/oracle/utils/period.go](../../../modules/oracle/utils/period.go) |

## go-exported-precompiles-oracle

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `func NewPrecompile(keepers utils.Keepers) (*pcommon.DynamicGasPrecompile, error)` | [precompiles/oracle/oracle.go](../../../precompiles/oracle/oracle.go) |

<a id="spot"></a>

## spot-0x1021

| Kind | Declaration | Source |
|---|---|---|
| `precompile_method` | `function BPS_DENOMINATOR() view returns (uint256)` | [precompiles/spot/abi/pecor.json](../../../precompiles/spot/abi/pecor.json) |
| `precompile_method` | `function MAX_FEE_BPS() view returns (uint256)` | [precompiles/spot/abi/pecor.json](../../../precompiles/spot/abi/pecor.json) |
| `precompile_method` | `function MAX_IMPACT_BPS() view returns (uint256)` | [precompiles/spot/abi/pecor.json](../../../precompiles/spot/abi/pecor.json) |
| `precompile_method` | `function PRECISION() view returns (uint256)` | [precompiles/spot/abi/pecor.json](../../../precompiles/spot/abi/pecor.json) |
| `precompile_method` | `function TIER1_THRESHOLD() view returns (uint256)` | [precompiles/spot/abi/pecor.json](../../../precompiles/spot/abi/pecor.json) |
| `precompile_method` | `function TIER2_THRESHOLD() view returns (uint256)` | [precompiles/spot/abi/pecor.json](../../../precompiles/spot/abi/pecor.json) |
| `precompile_method` | `function WETH() view returns (address)` | [precompiles/spot/abi/pecor.json](../../../precompiles/spot/abi/pecor.json) |
| `precompile_method` | `function accumulatedFees(address) view returns (uint256)` | [precompiles/spot/abi/pecor.json](../../../precompiles/spot/abi/pecor.json) |
| `precompile_method` | `function collectFees(address) nonpayable returns ()` | [precompiles/spot/abi/pecor.json](../../../precompiles/spot/abi/pecor.json) |
| `precompile_method` | `function feeCollector() view returns (address)` | [precompiles/spot/abi/pecor.json](../../../precompiles/spot/abi/pecor.json) |
| `precompile_method` | `function getAccumulatedFees(address) view returns (uint256)` | [precompiles/spot/abi/pecor.json](../../../precompiles/spot/abi/pecor.json) |
| `precompile_method` | `function getDetailedQuote(address,address,uint256) view returns (uint256,uint256,uint256,uint256,uint256)` | [precompiles/spot/abi/pecor.json](../../../precompiles/spot/abi/pecor.json) |
| `precompile_method` | `function getQuoteExactIn(address,address,uint256) view returns (uint256)` | [precompiles/spot/abi/pecor.json](../../../precompiles/spot/abi/pecor.json) |
| `precompile_method` | `function getQuoteExactOut(address,address,uint256) view returns (uint256)` | [precompiles/spot/abi/pecor.json](../../../precompiles/spot/abi/pecor.json) |
| `precompile_method` | `function impactScalarBps() view returns (uint256)` | [precompiles/spot/abi/pecor.json](../../../precompiles/spot/abi/pecor.json) |
| `precompile_method` | `function marketBuy(address,address,uint256,uint256,uint256) nonpayable returns (uint256)` | [precompiles/spot/abi/pecor.json](../../../precompiles/spot/abi/pecor.json) |
| `precompile_method` | `function marketSell(address,address,uint256,uint256,uint256) nonpayable returns (uint256)` | [precompiles/spot/abi/pecor.json](../../../precompiles/spot/abi/pecor.json) |
| `precompile_method` | `function maxTwapDeviationBps() view returns (uint256)` | [precompiles/spot/abi/pecor.json](../../../precompiles/spot/abi/pecor.json) |
| `precompile_method` | `function multicall(bytes[]) nonpayable returns (bytes[])` | [precompiles/spot/abi/pecor.json](../../../precompiles/spot/abi/pecor.json) |
| `precompile_method` | `function owner() view returns (address)` | [precompiles/spot/abi/pecor.json](../../../precompiles/spot/abi/pecor.json) |
| `precompile_method` | `function pause() nonpayable returns ()` | [precompiles/spot/abi/pecor.json](../../../precompiles/spot/abi/pecor.json) |
| `precompile_method` | `function paused() view returns (bool)` | [precompiles/spot/abi/pecor.json](../../../precompiles/spot/abi/pecor.json) |
| `precompile_method` | `function priceImpactEnabled() view returns (bool)` | [precompiles/spot/abi/pecor.json](../../../precompiles/spot/abi/pecor.json) |
| `precompile_method` | `function priceOracle() view returns (address)` | [precompiles/spot/abi/pecor.json](../../../precompiles/spot/abi/pecor.json) |
| `precompile_method` | `function renounceOwnership() nonpayable returns ()` | [precompiles/spot/abi/pecor.json](../../../precompiles/spot/abi/pecor.json) |
| `precompile_method` | `function setFeeCollector(address) nonpayable returns ()` | [precompiles/spot/abi/pecor.json](../../../precompiles/spot/abi/pecor.json) |
| `precompile_method` | `function setPriceImpact(bool,uint256) nonpayable returns ()` | [precompiles/spot/abi/pecor.json](../../../precompiles/spot/abi/pecor.json) |
| `precompile_method` | `function setPriceOracle(address) nonpayable returns ()` | [precompiles/spot/abi/pecor.json](../../../precompiles/spot/abi/pecor.json) |
| `precompile_method` | `function setSwapFee(uint256) nonpayable returns ()` | [precompiles/spot/abi/pecor.json](../../../precompiles/spot/abi/pecor.json) |
| `precompile_method` | `function setTieredFees(uint256,uint256) nonpayable returns ()` | [precompiles/spot/abi/pecor.json](../../../precompiles/spot/abi/pecor.json) |
| `precompile_method` | `function setTransactionTracker(address) nonpayable returns ()` | [precompiles/spot/abi/pecor.json](../../../precompiles/spot/abi/pecor.json) |
| `precompile_method` | `function setTwapSanity(bool,uint256,uint256) nonpayable returns ()` | [precompiles/spot/abi/pecor.json](../../../precompiles/spot/abi/pecor.json) |
| `precompile_method` | `function swapExactIn(address,address,uint256,uint256,uint256) nonpayable returns (uint256)` | [precompiles/spot/abi/pecor.json](../../../precompiles/spot/abi/pecor.json) |
| `precompile_method` | `function swapExactInNative(address,uint256,uint256) payable returns (uint256)` | [precompiles/spot/abi/pecor.json](../../../precompiles/spot/abi/pecor.json) |
| `precompile_method` | `function swapExactInToNative(address,uint256,uint256,uint256) nonpayable returns (uint256)` | [precompiles/spot/abi/pecor.json](../../../precompiles/spot/abi/pecor.json) |
| `precompile_method` | `function swapExactOut(address,address,uint256,uint256,uint256) nonpayable returns (uint256)` | [precompiles/spot/abi/pecor.json](../../../precompiles/spot/abi/pecor.json) |
| `precompile_method` | `function swapFeeBps() view returns (uint256)` | [precompiles/spot/abi/pecor.json](../../../precompiles/spot/abi/pecor.json) |
| `precompile_method` | `function tier1FeeBps() view returns (uint256)` | [precompiles/spot/abi/pecor.json](../../../precompiles/spot/abi/pecor.json) |
| `precompile_method` | `function tier2FeeBps() view returns (uint256)` | [precompiles/spot/abi/pecor.json](../../../precompiles/spot/abi/pecor.json) |
| `precompile_method` | `function transactionTracker() view returns (address)` | [precompiles/spot/abi/pecor.json](../../../precompiles/spot/abi/pecor.json) |
| `precompile_method` | `function transferOwnership(address) nonpayable returns ()` | [precompiles/spot/abi/pecor.json](../../../precompiles/spot/abi/pecor.json) |
| `precompile_method` | `function twapSanityEnabled() view returns (bool)` | [precompiles/spot/abi/pecor.json](../../../precompiles/spot/abi/pecor.json) |
| `precompile_method` | `function twapSanityPeriod() view returns (uint256)` | [precompiles/spot/abi/pecor.json](../../../precompiles/spot/abi/pecor.json) |
| `precompile_method` | `function unpause() nonpayable returns ()` | [precompiles/spot/abi/pecor.json](../../../precompiles/spot/abi/pecor.json) |
| `precompile_method` | `function vault() view returns (address)` | [precompiles/spot/abi/pecor.json](../../../precompiles/spot/abi/pecor.json) |
| `precompile_event` | `event MarketOrderExecuted(address,address,address,uint256,uint256,bool)` | [precompiles/spot/abi/pecor.json](../../../precompiles/spot/abi/pecor.json) |
| `precompile_event` | `event NativeSwap(address,address,uint256,uint256,bool)` | [precompiles/spot/abi/pecor.json](../../../precompiles/spot/abi/pecor.json) |
| `precompile_event` | `event OwnershipTransferred(address,address)` | [precompiles/spot/abi/pecor.json](../../../precompiles/spot/abi/pecor.json) |
| `precompile_event` | `event Paused(address)` | [precompiles/spot/abi/pecor.json](../../../precompiles/spot/abi/pecor.json) |
| `precompile_event` | `event PriceImpactApplied(address,address,uint256,uint256)` | [precompiles/spot/abi/pecor.json](../../../precompiles/spot/abi/pecor.json) |
| `precompile_event` | `event SimpleSwap(address,address,address,uint256,uint256,uint256)` | [precompiles/spot/abi/pecor.json](../../../precompiles/spot/abi/pecor.json) |
| `precompile_event` | `event TieredFeeApplied(address,address,uint256,uint256,uint256)` | [precompiles/spot/abi/pecor.json](../../../precompiles/spot/abi/pecor.json) |
| `precompile_event` | `event TwapSanityUpdated(bool,uint256,uint256)` | [precompiles/spot/abi/pecor.json](../../../precompiles/spot/abi/pecor.json) |
| `precompile_event` | `event Unpaused(address)` | [precompiles/spot/abi/pecor.json](../../../precompiles/spot/abi/pecor.json) |
| `precompile_error` | `error EnforcedPause()` | [precompiles/spot/abi/pecor.json](../../../precompiles/spot/abi/pecor.json) |
| `precompile_error` | `error ExpectedPause()` | [precompiles/spot/abi/pecor.json](../../../precompiles/spot/abi/pecor.json) |
| `precompile_error` | `error OwnableInvalidOwner(address)` | [precompiles/spot/abi/pecor.json](../../../precompiles/spot/abi/pecor.json) |
| `precompile_error` | `error OwnableUnauthorizedAccount(address)` | [precompiles/spot/abi/pecor.json](../../../precompiles/spot/abi/pecor.json) |
| `precompile_error` | `error ReentrancyGuardReentrantCall()` | [precompiles/spot/abi/pecor.json](../../../precompiles/spot/abi/pecor.json) |
| `precompile_error` | `error SafeERC20FailedOperation(address)` | [precompiles/spot/abi/pecor.json](../../../precompiles/spot/abi/pecor.json) |
| `precompile_error` | `error ZeroAmount()` | [precompiles/spot/abi/pecor.json](../../../precompiles/spot/abi/pecor.json) |
| `precompile_error` | `error SameToken()` | [precompiles/spot/abi/pecor.json](../../../precompiles/spot/abi/pecor.json) |
| `precompile_error` | `error Expired()` | [precompiles/spot/abi/pecor.json](../../../precompiles/spot/abi/pecor.json) |
| `precompile_error` | `error InsufficientOutput()` | [precompiles/spot/abi/pecor.json](../../../precompiles/spot/abi/pecor.json) |
| `precompile_error` | `error ExcessiveInput()` | [precompiles/spot/abi/pecor.json](../../../precompiles/spot/abi/pecor.json) |
| `precompile_error` | `error InsufficientLiquidity()` | [precompiles/spot/abi/pecor.json](../../../precompiles/spot/abi/pecor.json) |
| `precompile_error` | `error NotAStablecoin()` | [precompiles/spot/abi/pecor.json](../../../precompiles/spot/abi/pecor.json) |
| `precompile_error` | `error TokenIsStablecoin()` | [precompiles/spot/abi/pecor.json](../../../precompiles/spot/abi/pecor.json) |
| `precompile_error` | `error UseWethDeposit()` | [precompiles/spot/abi/pecor.json](../../../precompiles/spot/abi/pecor.json) |
| `precompile_error` | `error UseWethWithdraw()` | [precompiles/spot/abi/pecor.json](../../../precompiles/spot/abi/pecor.json) |
| `precompile_error` | `error NoFeesToCollect()` | [precompiles/spot/abi/pecor.json](../../../precompiles/spot/abi/pecor.json) |
| `precompile_error` | `error NativeTransferFailed()` | [precompiles/spot/abi/pecor.json](../../../precompiles/spot/abi/pecor.json) |
| `precompile_error` | `error MulticallFailed(uint256)` | [precompiles/spot/abi/pecor.json](../../../precompiles/spot/abi/pecor.json) |
| `precompile_error` | `error UnsupportedInputTransfer()` | [precompiles/spot/abi/pecor.json](../../../precompiles/spot/abi/pecor.json) |
| `precompile_error` | `error StalePrice()` | [precompiles/spot/abi/pecor.json](../../../precompiles/spot/abi/pecor.json) |
| `precompile_error` | `error TokenPaused()` | [precompiles/spot/abi/pecor.json](../../../precompiles/spot/abi/pecor.json) |
| `precompile_error` | `error InvalidVault()` | [precompiles/spot/abi/pecor.json](../../../precompiles/spot/abi/pecor.json) |
| `precompile_error` | `error UnsupportedTransfer()` | [precompiles/spot/abi/pecor.json](../../../precompiles/spot/abi/pecor.json) |

## spot-0x1022

| Kind | Declaration | Source |
|---|---|---|
| `precompile_method` | `function BPS_DENOMINATOR() view returns (uint256)` | [precompiles/spot/abi/orders.json](../../../precompiles/spot/abi/orders.json) |
| `precompile_method` | `function MAX_ORDER_FEE_BPS() view returns (uint256)` | [precompiles/spot/abi/orders.json](../../../precompiles/spot/abi/orders.json) |
| `precompile_method` | `function accumulatedFees(address) view returns (uint256)` | [precompiles/spot/abi/orders.json](../../../precompiles/spot/abi/orders.json) |
| `precompile_method` | `function activeLimitOrderIds(uint256) view returns (uint256)` | [precompiles/spot/abi/orders.json](../../../precompiles/spot/abi/orders.json) |
| `precompile_method` | `function batchExecuteLimitOrders(uint256[]) nonpayable returns (uint256)` | [precompiles/spot/abi/orders.json](../../../precompiles/spot/abi/orders.json) |
| `precompile_method` | `function batchExpireOrders(uint256[]) nonpayable returns (uint256)` | [precompiles/spot/abi/orders.json](../../../precompiles/spot/abi/orders.json) |
| `precompile_method` | `function canExecuteLimitOrder(uint256) view returns (bool,string)` | [precompiles/spot/abi/orders.json](../../../precompiles/spot/abi/orders.json) |
| `precompile_method` | `function cancelLimitOrder(uint256) nonpayable returns ()` | [precompiles/spot/abi/orders.json](../../../precompiles/spot/abi/orders.json) |
| `precompile_method` | `function collectFees(address) nonpayable returns ()` | [precompiles/spot/abi/orders.json](../../../precompiles/spot/abi/orders.json) |
| `precompile_method` | `function executeLimitOrder(uint256) nonpayable returns ()` | [precompiles/spot/abi/orders.json](../../../precompiles/spot/abi/orders.json) |
| `precompile_method` | `function feeCollector() view returns (address)` | [precompiles/spot/abi/orders.json](../../../precompiles/spot/abi/orders.json) |
| `precompile_method` | `function fillQualityTwapPeriod() view returns (uint256)` | [precompiles/spot/abi/orders.json](../../../precompiles/spot/abi/orders.json) |
| `precompile_method` | `function getAccumulatedFees(address) view returns (uint256)` | [precompiles/spot/abi/orders.json](../../../precompiles/spot/abi/orders.json) |
| `precompile_method` | `function getActiveLimitOrderCount() view returns (uint256)` | [precompiles/spot/abi/orders.json](../../../precompiles/spot/abi/orders.json) |
| `precompile_method` | `function getExecutableLimitOrders(uint256) view returns (uint256[])` | [precompiles/spot/abi/orders.json](../../../precompiles/spot/abi/orders.json) |
| `precompile_method` | `function getLimitOrder(uint256) view returns ((uint256,address,address,address,uint256,uint256,uint8,uint8,uint256,uint256))` | [precompiles/spot/abi/orders.json](../../../precompiles/spot/abi/orders.json) |
| `precompile_method` | `function getUserLimitOrders(address) view returns (uint256[])` | [precompiles/spot/abi/orders.json](../../../precompiles/spot/abi/orders.json) |
| `precompile_method` | `function keepers(address) view returns (bool)` | [precompiles/spot/abi/orders.json](../../../precompiles/spot/abi/orders.json) |
| `precompile_method` | `function limitOrders(uint256) view returns (uint256,address,address,address,uint256,uint256,uint8,uint8,uint256,uint256)` | [precompiles/spot/abi/orders.json](../../../precompiles/spot/abi/orders.json) |
| `precompile_method` | `function modifyLimitOrder(uint256,uint256,uint256) nonpayable returns ()` | [precompiles/spot/abi/orders.json](../../../precompiles/spot/abi/orders.json) |
| `precompile_method` | `function nextOrderId() view returns (uint256)` | [precompiles/spot/abi/orders.json](../../../precompiles/spot/abi/orders.json) |
| `precompile_method` | `function orderFeeBps() view returns (uint256)` | [precompiles/spot/abi/orders.json](../../../precompiles/spot/abi/orders.json) |
| `precompile_method` | `function owner() view returns (address)` | [precompiles/spot/abi/orders.json](../../../precompiles/spot/abi/orders.json) |
| `precompile_method` | `function pause() nonpayable returns ()` | [precompiles/spot/abi/orders.json](../../../precompiles/spot/abi/orders.json) |
| `precompile_method` | `function paused() view returns (bool)` | [precompiles/spot/abi/orders.json](../../../precompiles/spot/abi/orders.json) |
| `precompile_method` | `function placeLimitBuy(address,address,uint256,uint256,uint256) nonpayable returns (uint256)` | [precompiles/spot/abi/orders.json](../../../precompiles/spot/abi/orders.json) |
| `precompile_method` | `function placeLimitSell(address,address,uint256,uint256,uint256) nonpayable returns (uint256)` | [precompiles/spot/abi/orders.json](../../../precompiles/spot/abi/orders.json) |
| `precompile_method` | `function priceOracle() view returns (address)` | [precompiles/spot/abi/orders.json](../../../precompiles/spot/abi/orders.json) |
| `precompile_method` | `function renounceOwnership() nonpayable returns ()` | [precompiles/spot/abi/orders.json](../../../precompiles/spot/abi/orders.json) |
| `precompile_method` | `function setFeeCollector(address) nonpayable returns ()` | [precompiles/spot/abi/orders.json](../../../precompiles/spot/abi/orders.json) |
| `precompile_method` | `function setFillQualityTwapPeriod(uint256) nonpayable returns ()` | [precompiles/spot/abi/orders.json](../../../precompiles/spot/abi/orders.json) |
| `precompile_method` | `function setKeeper(address,bool) nonpayable returns ()` | [precompiles/spot/abi/orders.json](../../../precompiles/spot/abi/orders.json) |
| `precompile_method` | `function setOrderFee(uint256) nonpayable returns ()` | [precompiles/spot/abi/orders.json](../../../precompiles/spot/abi/orders.json) |
| `precompile_method` | `function setPriceOracle(address) nonpayable returns ()` | [precompiles/spot/abi/orders.json](../../../precompiles/spot/abi/orders.json) |
| `precompile_method` | `function setTransactionTracker(address) nonpayable returns ()` | [precompiles/spot/abi/orders.json](../../../precompiles/spot/abi/orders.json) |
| `precompile_method` | `function transactionTracker() view returns (address)` | [precompiles/spot/abi/orders.json](../../../precompiles/spot/abi/orders.json) |
| `precompile_method` | `function transferOwnership(address) nonpayable returns ()` | [precompiles/spot/abi/orders.json](../../../precompiles/spot/abi/orders.json) |
| `precompile_method` | `function unpause() nonpayable returns ()` | [precompiles/spot/abi/orders.json](../../../precompiles/spot/abi/orders.json) |
| `precompile_method` | `function userLimitOrders(address,uint256) view returns (uint256)` | [precompiles/spot/abi/orders.json](../../../precompiles/spot/abi/orders.json) |
| `precompile_method` | `function vault() view returns (address)` | [precompiles/spot/abi/orders.json](../../../precompiles/spot/abi/orders.json) |
| `precompile_event` | `event LimitOrderCancelled(uint256)` | [precompiles/spot/abi/orders.json](../../../precompiles/spot/abi/orders.json) |
| `precompile_event` | `event LimitOrderCreated(uint256,address,uint8,uint256,uint256)` | [precompiles/spot/abi/orders.json](../../../precompiles/spot/abi/orders.json) |
| `precompile_event` | `event LimitOrderExecuted(uint256,uint256)` | [precompiles/spot/abi/orders.json](../../../precompiles/spot/abi/orders.json) |
| `precompile_event` | `event LimitOrderExpired(uint256)` | [precompiles/spot/abi/orders.json](../../../precompiles/spot/abi/orders.json) |
| `precompile_event` | `event LimitOrderModified(uint256,uint256,uint256)` | [precompiles/spot/abi/orders.json](../../../precompiles/spot/abi/orders.json) |
| `precompile_event` | `event OwnershipTransferred(address,address)` | [precompiles/spot/abi/orders.json](../../../precompiles/spot/abi/orders.json) |
| `precompile_event` | `event Paused(address)` | [precompiles/spot/abi/orders.json](../../../precompiles/spot/abi/orders.json) |
| `precompile_event` | `event Unpaused(address)` | [precompiles/spot/abi/orders.json](../../../precompiles/spot/abi/orders.json) |
| `precompile_error` | `error CannotCancel()` | [precompiles/spot/abi/orders.json](../../../precompiles/spot/abi/orders.json) |
| `precompile_error` | `error EnforcedPause()` | [precompiles/spot/abi/orders.json](../../../precompiles/spot/abi/orders.json) |
| `precompile_error` | `error ExpectedPause()` | [precompiles/spot/abi/orders.json](../../../precompiles/spot/abi/orders.json) |
| `precompile_error` | `error FeeExceedsMax()` | [precompiles/spot/abi/orders.json](../../../precompiles/spot/abi/orders.json) |
| `precompile_error` | `error InsufficientLiquidity()` | [precompiles/spot/abi/orders.json](../../../precompiles/spot/abi/orders.json) |
| `precompile_error` | `error InvalidExpiry()` | [precompiles/spot/abi/orders.json](../../../precompiles/spot/abi/orders.json) |
| `precompile_error` | `error InvalidOracle()` | [precompiles/spot/abi/orders.json](../../../precompiles/spot/abi/orders.json) |
| `precompile_error` | `error InvalidPrice()` | [precompiles/spot/abi/orders.json](../../../precompiles/spot/abi/orders.json) |
| `precompile_error` | `error InvalidTWAPPeriod()` | [precompiles/spot/abi/orders.json](../../../precompiles/spot/abi/orders.json) |
| `precompile_error` | `error InvalidVault()` | [precompiles/spot/abi/orders.json](../../../precompiles/spot/abi/orders.json) |
| `precompile_error` | `error NoFeesToCollect()` | [precompiles/spot/abi/orders.json](../../../precompiles/spot/abi/orders.json) |
| `precompile_error` | `error NotAStablecoin()` | [precompiles/spot/abi/orders.json](../../../precompiles/spot/abi/orders.json) |
| `precompile_error` | `error NotAuthorized()` | [precompiles/spot/abi/orders.json](../../../precompiles/spot/abi/orders.json) |
| `precompile_error` | `error NotFeeCollector()` | [precompiles/spot/abi/orders.json](../../../precompiles/spot/abi/orders.json) |
| `precompile_error` | `error NotOrderOwner()` | [precompiles/spot/abi/orders.json](../../../precompiles/spot/abi/orders.json) |
| `precompile_error` | `error OrderExpired()` | [precompiles/spot/abi/orders.json](../../../precompiles/spot/abi/orders.json) |
| `precompile_error` | `error OrderNotPending()` | [precompiles/spot/abi/orders.json](../../../precompiles/spot/abi/orders.json) |
| `precompile_error` | `error OwnableInvalidOwner(address)` | [precompiles/spot/abi/orders.json](../../../precompiles/spot/abi/orders.json) |
| `precompile_error` | `error OwnableUnauthorizedAccount(address)` | [precompiles/spot/abi/orders.json](../../../precompiles/spot/abi/orders.json) |
| `precompile_error` | `error PriceAboveTarget()` | [precompiles/spot/abi/orders.json](../../../precompiles/spot/abi/orders.json) |
| `precompile_error` | `error PriceBelowTarget()` | [precompiles/spot/abi/orders.json](../../../precompiles/spot/abi/orders.json) |
| `precompile_error` | `error ReentrancyGuardReentrantCall()` | [precompiles/spot/abi/orders.json](../../../precompiles/spot/abi/orders.json) |
| `precompile_error` | `error SameToken()` | [precompiles/spot/abi/orders.json](../../../precompiles/spot/abi/orders.json) |
| `precompile_error` | `error TokenIsStablecoin()` | [precompiles/spot/abi/orders.json](../../../precompiles/spot/abi/orders.json) |
| `precompile_error` | `error TokenPaused()` | [precompiles/spot/abi/orders.json](../../../precompiles/spot/abi/orders.json) |
| `precompile_error` | `error ZeroAmount()` | [precompiles/spot/abi/orders.json](../../../precompiles/spot/abi/orders.json) |
| `precompile_error` | `error InsufficientOutput()` | [precompiles/spot/abi/orders.json](../../../precompiles/spot/abi/orders.json) |
| `precompile_error` | `error StalePrice()` | [precompiles/spot/abi/orders.json](../../../precompiles/spot/abi/orders.json) |
| `precompile_error` | `error UnsupportedTransfer()` | [precompiles/spot/abi/orders.json](../../../precompiles/spot/abi/orders.json) |
| `precompile_error` | `error UnsupportedInputTransfer()` | [precompiles/spot/abi/orders.json](../../../precompiles/spot/abi/orders.json) |
| `precompile_error` | `error SafeERC20FailedOperation(address)` | [precompiles/spot/abi/orders.json](../../../precompiles/spot/abi/orders.json) |
| `precompile_error` | `error OrderBelowMinimumNotional()` | [precompiles/spot/abi/orders.json](../../../precompiles/spot/abi/orders.json) |
| `precompile_error` | `error TooManyActiveOrders()` | [precompiles/spot/abi/orders.json](../../../precompiles/spot/abi/orders.json) |

## spot-0x1023

| Kind | Declaration | Source |
|---|---|---|
| `precompile_method` | `function BPS_DENOMINATOR() view returns (uint256)` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_method` | `function MAX_ORDER_FEE_BPS() view returns (uint256)` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_method` | `function accumulatedFees(address) view returns (uint256)` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_method` | `function activeStopLimitOrderIds(uint256) view returns (uint256)` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_method` | `function activeStopLossOrderIds(uint256) view returns (uint256)` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_method` | `function batchCheckAndExecuteStopLimits(uint256[]) nonpayable returns (uint256,uint256)` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_method` | `function batchExecuteStopLossOrders(uint256[]) nonpayable returns (uint256)` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_method` | `function cancelStopLimitOrder(uint256) nonpayable returns ()` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_method` | `function cancelStopLossOrder(uint256) nonpayable returns ()` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_method` | `function checkStopLimitActivation(uint256) nonpayable returns ()` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_method` | `function collectFees(address) nonpayable returns ()` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_method` | `function executeStopLimitOrder(uint256) nonpayable returns ()` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_method` | `function executeStopLossOrder(uint256) nonpayable returns ()` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_method` | `function feeCollector() view returns (address)` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_method` | `function fillQualityTwapPeriod() view returns (uint256)` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_method` | `function getAccumulatedFees(address) view returns (uint256)` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_method` | `function getActivatableStopLimits(uint256) view returns (uint256[])` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_method` | `function getActiveStopLimitOrderCount() view returns (uint256)` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_method` | `function getActiveStopLossOrderCount() view returns (uint256)` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_method` | `function getExecutableStopLimits(uint256) view returns (uint256[])` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_method` | `function getStopLimitOrder(uint256) view returns ((uint256,address,address,address,uint256,uint256,uint256,uint8,uint8,uint256,uint256))` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_method` | `function getStopLossOrder(uint256) view returns ((uint256,address,address,address,uint256,uint256,uint8,uint256,uint256))` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_method` | `function getUserStopLimitOrders(address) view returns (uint256[])` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_method` | `function getUserStopLossOrders(address) view returns (uint256[])` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_method` | `function keepers(address) view returns (bool)` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_method` | `function nextOrderId() view returns (uint256)` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_method` | `function orderFeeBps() view returns (uint256)` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_method` | `function owner() view returns (address)` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_method` | `function pause() nonpayable returns ()` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_method` | `function paused() view returns (bool)` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_method` | `function placeStopLimitBuy(address,address,uint256,uint256,uint256,uint256) nonpayable returns (uint256)` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_method` | `function placeStopLimitSell(address,address,uint256,uint256,uint256,uint256) nonpayable returns (uint256)` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_method` | `function placeStopLoss(address,address,uint256,uint256,uint256) nonpayable returns (uint256)` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_method` | `function priceOracle() view returns (address)` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_method` | `function renounceOwnership() nonpayable returns ()` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_method` | `function setFeeCollector(address) nonpayable returns ()` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_method` | `function setFillQualityTwapPeriod(uint256) nonpayable returns ()` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_method` | `function setKeeper(address,bool) nonpayable returns ()` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_method` | `function setOrderFee(uint256) nonpayable returns ()` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_method` | `function setPriceOracle(address) nonpayable returns ()` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_method` | `function setTransactionTracker(address) nonpayable returns ()` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_method` | `function stopLimitOrders(uint256) view returns (uint256,address,address,address,uint256,uint256,uint256,uint8,uint8,uint256,uint256)` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_method` | `function stopLossOrders(uint256) view returns (uint256,address,address,address,uint256,uint256,uint8,uint256,uint256)` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_method` | `function transactionTracker() view returns (address)` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_method` | `function transferOwnership(address) nonpayable returns ()` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_method` | `function unpause() nonpayable returns ()` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_method` | `function userStopLimitOrders(address,uint256) view returns (uint256)` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_method` | `function userStopLossOrders(address,uint256) view returns (uint256)` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_method` | `function vault() view returns (address)` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_event` | `event OwnershipTransferred(address,address)` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_event` | `event Paused(address)` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_event` | `event StopLimitActivated(uint256,uint256)` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_event` | `event StopLimitCancelled(uint256)` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_event` | `event StopLimitExecuted(uint256,uint256)` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_event` | `event StopLimitOrderCreated(uint256,address,uint8,uint256,uint256,uint256)` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_event` | `event StopLossOrderCancelled(uint256)` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_event` | `event StopLossOrderCreated(uint256,address,uint256,uint256)` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_event` | `event StopLossOrderExecuted(uint256,uint256)` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_event` | `event Unpaused(address)` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_error` | `error CannotCancel()` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_error` | `error EnforcedPause()` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_error` | `error ExpectedPause()` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_error` | `error FeeExceedsMax()` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_error` | `error InsufficientLiquidity()` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_error` | `error InvalidExpiry()` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_error` | `error InvalidOracle()` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_error` | `error InvalidPrice()` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_error` | `error InvalidPrices()` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_error` | `error InvalidTWAPPeriod()` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_error` | `error InvalidVault()` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_error` | `error LimitMustBeAboveStopBuy()` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_error` | `error LimitMustBeBelowStopSell()` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_error` | `error NoFeesToCollect()` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_error` | `error NotAStablecoin()` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_error` | `error NotActivated()` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_error` | `error NotAuthorized()` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_error` | `error NotFeeCollector()` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_error` | `error NotOrderOwner()` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_error` | `error NotPending()` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_error` | `error OrderExpired()` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_error` | `error OrderNotPending()` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_error` | `error OwnableInvalidOwner(address)` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_error` | `error OwnableUnauthorizedAccount(address)` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_error` | `error PriceAboveLimit()` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_error` | `error PriceAboveTrigger()` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_error` | `error PriceBelowLimit()` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_error` | `error ReentrancyGuardReentrantCall()` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_error` | `error TokenIsStablecoin()` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_error` | `error TokenPaused()` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_error` | `error ZeroAmount()` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_error` | `error InsufficientOutput()` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_error` | `error StalePrice()` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_error` | `error UnsupportedTransfer()` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_error` | `error UnsupportedInputTransfer()` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_error` | `error SafeERC20FailedOperation(address)` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_error` | `error OrderBelowMinimumNotional()` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |
| `precompile_error` | `error TooManyActiveOrders()` | [precompiles/spot/abi/stoporders.json](../../../precompiles/spot/abi/stoporders.json) |

## spot-0x1024

| Kind | Declaration | Source |
|---|---|---|
| `precompile_method` | `function WETH() view returns (address)` | [precompiles/spot/abi/quoter.json](../../../precompiles/spot/abi/quoter.json) |
| `precompile_method` | `function batchQuote((address,address,uint256,bool)[]) view returns ((uint256,uint256,uint256,uint256,uint256,uint256,uint256,uint256,bool,uint256,uint256,uint256,bool,bool,uint256,uint256,bool,uint256,uint256,uint256,uint256,uint256,uint256)[])` | [precompiles/spot/abi/quoter.json](../../../precompiles/spot/abi/quoter.json) |
| `precompile_method` | `function getAllLiquidityInfo() view returns (address[],uint256[],uint256[],bool[])` | [precompiles/spot/abi/quoter.json](../../../precompiles/spot/abi/quoter.json) |
| `precompile_method` | `function getLiquidityInfo(address) view returns (uint256,uint256,bool)` | [precompiles/spot/abi/quoter.json](../../../precompiles/spot/abi/quoter.json) |
| `precompile_method` | `function getTWAP(address,uint256) view returns (uint256)` | [precompiles/spot/abi/quoter.json](../../../precompiles/spot/abi/quoter.json) |
| `precompile_method` | `function getTokenPrice(address) view returns (uint256,uint256,bool)` | [precompiles/spot/abi/quoter.json](../../../precompiles/spot/abi/quoter.json) |
| `precompile_method` | `function getTokenPrices(address[]) view returns (uint256[],uint256[],bool[])` | [precompiles/spot/abi/quoter.json](../../../precompiles/spot/abi/quoter.json) |
| `precompile_method` | `function pecor() view returns (address)` | [precompiles/spot/abi/quoter.json](../../../precompiles/spot/abi/quoter.json) |
| `precompile_method` | `function priceOracle() view returns (address)` | [precompiles/spot/abi/quoter.json](../../../precompiles/spot/abi/quoter.json) |
| `precompile_method` | `function quoteExactIn(address,address,uint256) view returns ((uint256,uint256,uint256,uint256,uint256,uint256,uint256,uint256,bool,uint256,uint256,uint256,bool,bool,uint256,uint256,bool,uint256,uint256,uint256,uint256,uint256,uint256))` | [precompiles/spot/abi/quoter.json](../../../precompiles/spot/abi/quoter.json) |
| `precompile_method` | `function quoteExactInNative(address,uint256) view returns ((uint256,uint256,uint256,uint256,uint256,uint256,uint256,uint256,bool,uint256,uint256,uint256,bool,bool,uint256,uint256,bool,uint256,uint256,uint256,uint256,uint256,uint256))` | [precompiles/spot/abi/quoter.json](../../../precompiles/spot/abi/quoter.json) |
| `precompile_method` | `function quoteExactInToNative(address,uint256) view returns ((uint256,uint256,uint256,uint256,uint256,uint256,uint256,uint256,bool,uint256,uint256,uint256,bool,bool,uint256,uint256,bool,uint256,uint256,uint256,uint256,uint256,uint256))` | [precompiles/spot/abi/quoter.json](../../../precompiles/spot/abi/quoter.json) |
| `precompile_method` | `function quoteExactOut(address,address,uint256) view returns ((uint256,uint256,uint256,uint256,uint256,uint256,uint256,uint256,bool,uint256,uint256,uint256,bool,bool,uint256,uint256,bool,uint256,uint256,uint256,uint256,uint256,uint256))` | [precompiles/spot/abi/quoter.json](../../../precompiles/spot/abi/quoter.json) |
| `precompile_method` | `function quoteMarketBuy(address,address,uint256) view returns ((uint256,uint256,uint256,uint256,uint256,uint256,uint256,uint256,bool,uint256,uint256,uint256,bool,bool,uint256,uint256,bool,uint256,uint256,uint256,uint256,uint256,uint256))` | [precompiles/spot/abi/quoter.json](../../../precompiles/spot/abi/quoter.json) |
| `precompile_method` | `function quoteMarketSell(address,address,uint256) view returns ((uint256,uint256,uint256,uint256,uint256,uint256,uint256,uint256,bool,uint256,uint256,uint256,bool,bool,uint256,uint256,bool,uint256,uint256,uint256,uint256,uint256,uint256))` | [precompiles/spot/abi/quoter.json](../../../precompiles/spot/abi/quoter.json) |
| `precompile_method` | `function setPecor(address) nonpayable returns ()` | [precompiles/spot/abi/quoter.json](../../../precompiles/spot/abi/quoter.json) |
| `precompile_method` | `function vault() view returns (address)` | [precompiles/spot/abi/quoter.json](../../../precompiles/spot/abi/quoter.json) |
| `precompile_error` | `error TwapWindowInvalid()` | [precompiles/spot/abi/quoter.json](../../../precompiles/spot/abi/quoter.json) |
| `precompile_error` | `error StalePrice()` | [precompiles/spot/abi/quoter.json](../../../precompiles/spot/abi/quoter.json) |
| `precompile_error` | `error OwnableUnauthorizedAccount(address)` | [precompiles/spot/abi/quoter.json](../../../precompiles/spot/abi/quoter.json) |

## spot-0x1025

| Kind | Declaration | Source |
|---|---|---|
| `precompile_method` | `function BPS_DENOMINATOR() view returns (uint256)` | [precompiles/spot/abi/vault.json](../../../precompiles/spot/abi/vault.json) |
| `precompile_method` | `function DEFAULT_MAX_UTILIZATION_BPS() view returns (uint256)` | [precompiles/spot/abi/vault.json](../../../precompiles/spot/abi/vault.json) |
| `precompile_method` | `function WETH() view returns (address)` | [precompiles/spot/abi/vault.json](../../../precompiles/spot/abi/vault.json) |
| `precompile_method` | `function authorizedOperators(address) view returns (bool)` | [precompiles/spot/abi/vault.json](../../../precompiles/spot/abi/vault.json) |
| `precompile_method` | `function deposit(address,uint256) nonpayable returns ()` | [precompiles/spot/abi/vault.json](../../../precompiles/spot/abi/vault.json) |
| `precompile_method` | `function depositBatch(address[],uint256[]) nonpayable returns ()` | [precompiles/spot/abi/vault.json](../../../precompiles/spot/abi/vault.json) |
| `precompile_method` | `function depositNative() payable returns ()` | [precompiles/spot/abi/vault.json](../../../precompiles/spot/abi/vault.json) |
| `precompile_method` | `function emergencyWithdraw(address,uint256,address) nonpayable returns ()` | [precompiles/spot/abi/vault.json](../../../precompiles/spot/abi/vault.json) |
| `precompile_method` | `function getAllReserves() view returns (address[],uint256[])` | [precompiles/spot/abi/vault.json](../../../precompiles/spot/abi/vault.json) |
| `precompile_method` | `function getRegisteredStablecoins() view returns (address[])` | [precompiles/spot/abi/vault.json](../../../precompiles/spot/abi/vault.json) |
| `precompile_method` | `function getRegisteredTokenCount() view returns (uint256)` | [precompiles/spot/abi/vault.json](../../../precompiles/spot/abi/vault.json) |
| `precompile_method` | `function getRegisteredTokens() view returns (address[])` | [precompiles/spot/abi/vault.json](../../../precompiles/spot/abi/vault.json) |
| `precompile_method` | `function getReserves(address) view returns (uint256)` | [precompiles/spot/abi/vault.json](../../../precompiles/spot/abi/vault.json) |
| `precompile_method` | `function getTokenDecimals(address) view returns (uint8)` | [precompiles/spot/abi/vault.json](../../../precompiles/spot/abi/vault.json) |
| `precompile_method` | `function getTokenInfo(address) view returns (bool,bool,uint8,uint256,uint256,uint256)` | [precompiles/spot/abi/vault.json](../../../precompiles/spot/abi/vault.json) |
| `precompile_method` | `function getUntrackedFunds(address) view returns (uint256)` | [precompiles/spot/abi/vault.json](../../../precompiles/spot/abi/vault.json) |
| `precompile_method` | `function hasLiquidity(address,uint256) view returns (bool)` | [precompiles/spot/abi/vault.json](../../../precompiles/spot/abi/vault.json) |
| `precompile_method` | `function isStablecoin(address) view returns (bool)` | [precompiles/spot/abi/vault.json](../../../precompiles/spot/abi/vault.json) |
| `precompile_method` | `function isTokenPaused(address) view returns (bool)` | [precompiles/spot/abi/vault.json](../../../precompiles/spot/abi/vault.json) |
| `precompile_method` | `function maxUtilizationBps(address) view returns (uint256)` | [precompiles/spot/abi/vault.json](../../../precompiles/spot/abi/vault.json) |
| `precompile_method` | `function owner() view returns (address)` | [precompiles/spot/abi/vault.json](../../../precompiles/spot/abi/vault.json) |
| `precompile_method` | `function pausedTokens(address) view returns (bool)` | [precompiles/spot/abi/vault.json](../../../precompiles/spot/abi/vault.json) |
| `precompile_method` | `function pullTokens(address,address,uint256) nonpayable returns (uint256)` | [precompiles/spot/abi/vault.json](../../../precompiles/spot/abi/vault.json) |
| `precompile_method` | `function pushTokens(address,address,uint256) nonpayable returns ()` | [precompiles/spot/abi/vault.json](../../../precompiles/spot/abi/vault.json) |
| `precompile_method` | `function registerToken(address,bool) nonpayable returns ()` | [precompiles/spot/abi/vault.json](../../../precompiles/spot/abi/vault.json) |
| `precompile_method` | `function registeredStablecoins(uint256) view returns (address)` | [precompiles/spot/abi/vault.json](../../../precompiles/spot/abi/vault.json) |
| `precompile_method` | `function registeredTokens(uint256) view returns (address)` | [precompiles/spot/abi/vault.json](../../../precompiles/spot/abi/vault.json) |
| `precompile_method` | `function renounceOwnership() nonpayable returns ()` | [precompiles/spot/abi/vault.json](../../../precompiles/spot/abi/vault.json) |
| `precompile_method` | `function setMaxUtilizationBps(address,uint256) nonpayable returns ()` | [precompiles/spot/abi/vault.json](../../../precompiles/spot/abi/vault.json) |
| `precompile_method` | `function setOperator(address,bool) nonpayable returns ()` | [precompiles/spot/abi/vault.json](../../../precompiles/spot/abi/vault.json) |
| `precompile_method` | `function setStablecoinStatus(address,bool) nonpayable returns ()` | [precompiles/spot/abi/vault.json](../../../precompiles/spot/abi/vault.json) |
| `precompile_method` | `function setTokenPaused(address,bool) nonpayable returns ()` | [precompiles/spot/abi/vault.json](../../../precompiles/spot/abi/vault.json) |
| `precompile_method` | `function setTransactionTracker(address) nonpayable returns ()` | [precompiles/spot/abi/vault.json](../../../precompiles/spot/abi/vault.json) |
| `precompile_method` | `function shareTokens(address) view returns (address)` | [precompiles/spot/abi/vault.json](../../../precompiles/spot/abi/vault.json) |
| `precompile_method` | `function syncAllReserves() nonpayable returns ()` | [precompiles/spot/abi/vault.json](../../../precompiles/spot/abi/vault.json) |
| `precompile_method` | `function syncReserves(address) nonpayable returns ()` | [precompiles/spot/abi/vault.json](../../../precompiles/spot/abi/vault.json) |
| `precompile_method` | `function tokens(address) view returns (bool,bool,uint8,uint256,uint256,uint256)` | [precompiles/spot/abi/vault.json](../../../precompiles/spot/abi/vault.json) |
| `precompile_method` | `function transactionTracker() view returns (address)` | [precompiles/spot/abi/vault.json](../../../precompiles/spot/abi/vault.json) |
| `precompile_method` | `function transferOwnership(address) nonpayable returns ()` | [precompiles/spot/abi/vault.json](../../../precompiles/spot/abi/vault.json) |
| `precompile_method` | `function updateReserves(address,uint256,address,uint256) nonpayable returns ()` | [precompiles/spot/abi/vault.json](../../../precompiles/spot/abi/vault.json) |
| `precompile_method` | `function withdraw(address,uint256) nonpayable returns (uint256)` | [precompiles/spot/abi/vault.json](../../../precompiles/spot/abi/vault.json) |
| `precompile_method` | `function withdrawNative(uint256,address) nonpayable returns ()` | [precompiles/spot/abi/vault.json](../../../precompiles/spot/abi/vault.json) |
| `precompile_event` | `event Deposit(address,address,uint256,uint256)` | [precompiles/spot/abi/vault.json](../../../precompiles/spot/abi/vault.json) |
| `precompile_event` | `event LPWithdrawal(address,address,uint256,uint256)` | [precompiles/spot/abi/vault.json](../../../precompiles/spot/abi/vault.json) |
| `precompile_event` | `event NativeDeposit(address,uint256,uint256)` | [precompiles/spot/abi/vault.json](../../../precompiles/spot/abi/vault.json) |
| `precompile_event` | `event NativeWithdrawal(address,uint256,uint256)` | [precompiles/spot/abi/vault.json](../../../precompiles/spot/abi/vault.json) |
| `precompile_event` | `event OperatorUpdated(address,bool)` | [precompiles/spot/abi/vault.json](../../../precompiles/spot/abi/vault.json) |
| `precompile_event` | `event OwnershipTransferred(address,address)` | [precompiles/spot/abi/vault.json](../../../precompiles/spot/abi/vault.json) |
| `precompile_event` | `event ReservesSync(address,uint256,uint256,uint256)` | [precompiles/spot/abi/vault.json](../../../precompiles/spot/abi/vault.json) |
| `precompile_event` | `event ReservesUpdated(address,uint256,uint256)` | [precompiles/spot/abi/vault.json](../../../precompiles/spot/abi/vault.json) |
| `precompile_event` | `event ShareTokenCreated(address,address)` | [precompiles/spot/abi/vault.json](../../../precompiles/spot/abi/vault.json) |
| `precompile_event` | `event TokenPauseUpdated(address,bool)` | [precompiles/spot/abi/vault.json](../../../precompiles/spot/abi/vault.json) |
| `precompile_event` | `event TokenRegistered(address,uint8,bool)` | [precompiles/spot/abi/vault.json](../../../precompiles/spot/abi/vault.json) |
| `precompile_event` | `event TransactionTrackerUpdated(address)` | [precompiles/spot/abi/vault.json](../../../precompiles/spot/abi/vault.json) |
| `precompile_event` | `event UtilizationCapUpdated(address,uint256)` | [precompiles/spot/abi/vault.json](../../../precompiles/spot/abi/vault.json) |
| `precompile_event` | `event Withdrawal(address,address,uint256,uint256)` | [precompiles/spot/abi/vault.json](../../../precompiles/spot/abi/vault.json) |
| `precompile_error` | `error OwnableInvalidOwner(address)` | [precompiles/spot/abi/vault.json](../../../precompiles/spot/abi/vault.json) |
| `precompile_error` | `error OwnableUnauthorizedAccount(address)` | [precompiles/spot/abi/vault.json](../../../precompiles/spot/abi/vault.json) |
| `precompile_error` | `error ReentrancyGuardReentrantCall()` | [precompiles/spot/abi/vault.json](../../../precompiles/spot/abi/vault.json) |
| `precompile_error` | `error SafeERC20FailedOperation(address)` | [precompiles/spot/abi/vault.json](../../../precompiles/spot/abi/vault.json) |
| `precompile_error` | `error ZeroAmount()` | [precompiles/spot/abi/vault.json](../../../precompiles/spot/abi/vault.json) |
| `precompile_error` | `error InvalidArrayLength()` | [precompiles/spot/abi/vault.json](../../../precompiles/spot/abi/vault.json) |
| `precompile_error` | `error WethOnly()` | [precompiles/spot/abi/vault.json](../../../precompiles/spot/abi/vault.json) |
| `precompile_error` | `error TokenNotRegistered()` | [precompiles/spot/abi/vault.json](../../../precompiles/spot/abi/vault.json) |
| `precompile_error` | `error TokenPaused()` | [precompiles/spot/abi/vault.json](../../../precompiles/spot/abi/vault.json) |
| `precompile_error` | `error InsufficientLiquidity()` | [precompiles/spot/abi/vault.json](../../../precompiles/spot/abi/vault.json) |
| `precompile_error` | `error UnsupportedTransfer()` | [precompiles/spot/abi/vault.json](../../../precompiles/spot/abi/vault.json) |
| `precompile_error` | `error UnsupportedInputTransfer()` | [precompiles/spot/abi/vault.json](../../../precompiles/spot/abi/vault.json) |

<a id="spot-active-index"></a>

## spot-errors

| Kind | Declaration | Source |
|---|---|---|
| `error_code` | `ErrInvalidPriceOrDecimals = errors.New("InvalidPriceOrDecimals()")` | [modules/spot/keeper/pricing.go](../../../modules/spot/keeper/pricing.go) |
| `error_code` | `ErrQuoteOverflow = errors.New("QuoteOverflow()")` | [modules/spot/keeper/pricing.go](../../../modules/spot/keeper/pricing.go) |
| `error_code` | `ErrCannotCancel = newAbiError(3, "CannotCancel()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `error_code` | `ErrERC20InsufficientAllowance = newAbiError(4, "ERC20InsufficientAllowance(address,uint256,uint256)")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `error_code` | `ErrERC20InsufficientBalance = newAbiError(5, "ERC20InsufficientBalance(address,uint256,uint256)")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `error_code` | `ErrERC20InvalidApprover = newAbiError(6, "ERC20InvalidApprover(address)")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `error_code` | `ErrERC20InvalidReceiver = newAbiError(7, "ERC20InvalidReceiver(address)")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `error_code` | `ErrERC20InvalidSender = newAbiError(8, "ERC20InvalidSender(address)")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `error_code` | `ErrERC20InvalidSpender = newAbiError(9, "ERC20InvalidSpender(address)")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `error_code` | `ErrEnforcedPause = newAbiError(10, "EnforcedPause()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `error_code` | `ErrExcessiveInput = newAbiError(11, "ExcessiveInput()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `error_code` | `ErrExpectedPause = newAbiError(12, "ExpectedPause()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `error_code` | `ErrExpired = newAbiError(13, "Expired()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `error_code` | `ErrFeeExceedsMax = newAbiError(14, "FeeExceedsMax()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `error_code` | `ErrFeeTooHigh = newAbiError(15, "FeeTooHigh()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `error_code` | `ErrInsufficientLiquidity = newAbiError(16, "InsufficientLiquidity()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `error_code` | `ErrInsufficientOutput = newAbiError(17, "InsufficientOutput()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `error_code` | `ErrInvalidArrayLength = newAbiError(18, "InvalidArrayLength()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `error_code` | `ErrInvalidExpiry = newAbiError(19, "InvalidExpiry()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `error_code` | `ErrInvalidOracle = newAbiError(20, "InvalidOracle()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `error_code` | `ErrInvalidPrice = newAbiError(21, "InvalidPrice()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `error_code` | `ErrInvalidPrices = newAbiError(22, "InvalidPrices()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `error_code` | `ErrInvalidTWAPPeriod = newAbiError(23, "InvalidTWAPPeriod()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `error_code` | `ErrInvalidVault = newAbiError(24, "InvalidVault()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `error_code` | `ErrLimitMustBeAboveStopBuy = newAbiError(25, "LimitMustBeAboveStopBuy()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `error_code` | `ErrLimitMustBeBelowStopSell = newAbiError(26, "LimitMustBeBelowStopSell()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `error_code` | `ErrMulticallFailed = newAbiError(27, "MulticallFailed(uint256)")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `error_code` | `ErrNativeTransferFailed = newAbiError(28, "NativeTransferFailed()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `error_code` | `ErrNoFeesToCollect = newAbiError(29, "NoFeesToCollect()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `error_code` | `ErrNotAStablecoin = newAbiError(30, "NotAStablecoin()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `error_code` | `ErrNotActivated = newAbiError(31, "NotActivated()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `error_code` | `ErrNotAuthorized = newAbiError(32, "NotAuthorized()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `error_code` | `ErrNotFeeCollector = newAbiError(33, "NotFeeCollector()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `error_code` | `ErrNotOrderOwner = newAbiError(34, "NotOrderOwner()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `error_code` | `ErrNotPending = newAbiError(35, "NotPending()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `error_code` | `ErrOrderBelowMinimumNotional = newAbiError(36, "OrderBelowMinimumNotional()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `error_code` | `ErrOrderExpired = newAbiError(37, "OrderExpired()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `error_code` | `ErrOrderNotPending = newAbiError(38, "OrderNotPending()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `error_code` | `ErrOwnableInvalidOwner = newAbiError(39, "OwnableInvalidOwner(address)")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `error_code` | `ErrOwnableUnauthorizedAccount = newAbiError(40, "OwnableUnauthorizedAccount(address)")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `error_code` | `ErrPriceAboveLimit = newAbiError(41, "PriceAboveLimit()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `error_code` | `ErrPriceAboveTarget = newAbiError(42, "PriceAboveTarget()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `error_code` | `ErrPriceAboveTrigger = newAbiError(43, "PriceAboveTrigger()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `error_code` | `ErrPriceBelowLimit = newAbiError(44, "PriceBelowLimit()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `error_code` | `ErrPriceBelowTarget = newAbiError(45, "PriceBelowTarget()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `error_code` | `ErrReentrancyGuardReentrantCall = newAbiError(46, "ReentrancyGuardReentrantCall()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `error_code` | `ErrSafeERC20FailedOperation = newAbiError(47, "SafeERC20FailedOperation(address)")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `error_code` | `ErrSameToken = newAbiError(48, "SameToken()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `error_code` | `ErrScalarTooHigh = newAbiError(49, "ScalarTooHigh()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `error_code` | `ErrStalePrice = newAbiError(50, "StalePrice()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `error_code` | `ErrTier2BelowTier1 = newAbiError(51, "Tier2BelowTier1()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `error_code` | `ErrTokenAlreadyRegistered = newAbiError(52, "TokenAlreadyRegistered()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `error_code` | `ErrTokenIsStablecoin = newAbiError(53, "TokenIsStablecoin()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `error_code` | `ErrTokenNotRegistered = newAbiError(54, "TokenNotRegistered()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `error_code` | `ErrTokenPaused = newAbiError(55, "TokenPaused()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `error_code` | `ErrTooManyActiveOrders = newAbiError(56, "TooManyActiveOrders()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `error_code` | `ErrTwapWindowInvalid = newAbiError(57, "TwapWindowInvalid()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `error_code` | `ErrUnsupportedInputTransfer = newAbiError(58, "UnsupportedInputTransfer()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `error_code` | `ErrUnsupportedToken = newAbiError(59, "UnsupportedToken()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `error_code` | `ErrUnsupportedTransfer = newAbiError(60, "UnsupportedTransfer()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `error_code` | `ErrUseWethDeposit = newAbiError(61, "UseWethDeposit()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `error_code` | `ErrUseWethWithdraw = newAbiError(62, "UseWethWithdraw()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `error_code` | `ErrWethOnly = newAbiError(63, "WethOnly()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `error_code` | `ErrZeroAddress = newAbiError(64, "ZeroAddress()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `error_code` | `ErrZeroAmount = newAbiError(65, "ZeroAmount()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `error_code` | `ErrParamsFeeTooHigh ParamsError = "FeeTooHigh()"` | [modules/spot/types/params.go](../../../modules/spot/types/params.go) |
| `error_code` | `ErrParamsTier2BelowTier1 ParamsError = "Tier2BelowTier1()"` | [modules/spot/types/params.go](../../../modules/spot/types/params.go) |
| `error_code` | `ErrParamsScalarTooHigh ParamsError = "ScalarTooHigh()"` | [modules/spot/types/params.go](../../../modules/spot/types/params.go) |
| `error_code` | `ErrParamsFeeExceedsMax ParamsError = "FeeExceedsMax()"` | [modules/spot/types/params.go](../../../modules/spot/types/params.go) |
| `error_code` | `ErrParamsInvalidTWAPPeriod ParamsError = "InvalidTWAPPeriod()"` | [modules/spot/types/params.go](../../../modules/spot/types/params.go) |
| `error_code` | `ErrParamsZeroAddress ParamsError = "ZeroAddress()"` | [modules/spot/types/params.go](../../../modules/spot/types/params.go) |
| `error_code` | `ErrParamsTokenAlreadyRegistered ParamsError = "TokenAlreadyRegistered()"` | [modules/spot/types/params.go](../../../modules/spot/types/params.go) |
| `error_code` | `ErrParamsUnsupportedToken ParamsError = "UnsupportedToken()"` | [modules/spot/types/params.go](../../../modules/spot/types/params.go) |
| `error_code` | `ErrParamsInvalid ParamsError = "invalid spot params"` | [modules/spot/types/params.go](../../../modules/spot/types/params.go) |

## spot-events

| Kind | Declaration | Source |
|---|---|---|
| `event` | `TieredFeeApplied(address,address,uint256,uint256,uint256)` | [modules/spot/keeper/fees.go](../../../modules/spot/keeper/fees.go) |
| `event` | `PriceImpactApplied(address,address,uint256,uint256)` | [modules/spot/keeper/fees.go](../../../modules/spot/keeper/fees.go) |
| `event` | `OwnershipTransferred(address,address)` | [modules/spot/keeper/init_v6120.go](../../../modules/spot/keeper/init_v6120.go) |
| `event` | `OperatorUpdated(address,bool)` | [modules/spot/keeper/init_v6120.go](../../../modules/spot/keeper/init_v6120.go) |
| `event` | `ShareTokenCreated(address,address)` | [modules/spot/keeper/init_v6120.go](../../../modules/spot/keeper/init_v6120.go) |
| `event` | `TransactionTrackerUpdated(address)` | [modules/spot/keeper/init_v6120.go](../../../modules/spot/keeper/init_v6120.go) |
| `event` | `Paused(address)` | [modules/spot/keeper/msg_server.go](../../../modules/spot/keeper/msg_server.go) |
| `event` | `Unpaused(address)` | [modules/spot/keeper/msg_server.go](../../../modules/spot/keeper/msg_server.go) |
| `event` | `TwapSanityUpdated(bool,uint256,uint256)` | [modules/spot/keeper/msg_server.go](../../../modules/spot/keeper/msg_server.go) |
| `event` | `TokenPauseUpdated(address,bool)` | [modules/spot/keeper/msg_server.go](../../../modules/spot/keeper/msg_server.go) |
| `event` | `UtilizationCapUpdated(address,uint256)` | [modules/spot/keeper/msg_server.go](../../../modules/spot/keeper/msg_server.go) |
| `event` | `ReservesSync(address,uint256,uint256,uint256)` | [modules/spot/keeper/msg_server.go](../../../modules/spot/keeper/msg_server.go) |
| `event` | `ReservesUpdated(address,uint256,uint256)` | [modules/spot/keeper/msg_server.go](../../../modules/spot/keeper/msg_server.go) |
| `event` | `Transfer(address,address,uint256)` | [modules/spot/keeper/shares.go](../../../modules/spot/keeper/shares.go) |
| `event` | `MarketOrderExecuted(address,address,address,uint256,uint256,bool)` | [modules/spot/keeper/swap.go](../../../modules/spot/keeper/swap.go) |
| `event` | `SimpleSwap(address,address,address,uint256,uint256,uint256)` | [modules/spot/keeper/swap.go](../../../modules/spot/keeper/swap.go) |
| `event` | `NativeSwap(address,address,uint256,uint256,bool)` | [modules/spot/keeper/swap.go](../../../modules/spot/keeper/swap.go) |
| `event` | `Deposit(address,address,uint256,uint256)` | [modules/spot/keeper/vault.go](../../../modules/spot/keeper/vault.go) |
| `event` | `LPWithdrawal(address,address,uint256,uint256)` | [modules/spot/keeper/vault.go](../../../modules/spot/keeper/vault.go) |
| `event` | `NativeDeposit(address,uint256,uint256)` | [modules/spot/keeper/vault.go](../../../modules/spot/keeper/vault.go) |
| `event` | `NativeWithdrawal(address,uint256,uint256)` | [modules/spot/keeper/vault.go](../../../modules/spot/keeper/vault.go) |
| `event` | `Withdrawal(address,address,uint256,uint256)` | [modules/spot/keeper/vault.go](../../../modules/spot/keeper/vault.go) |
| `event` | `TokenRegistered(address,uint8,bool)` | [modules/spot/keeper/vault.go](../../../modules/spot/keeper/vault.go) |
| source | — | [modules/spot/keeper/execute.go](../../../modules/spot/keeper/execute.go) |
| source | — | [modules/spot/keeper/stoporders.go](../../../modules/spot/keeper/stoporders.go) |

## spot-params

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `func (k *Keeper) ValidatePriceMarket(ctx sdk.Context, token types.TokenParams) error` | [modules/spot/keeper/pricing.go](../../../modules/spot/keeper/pricing.go) |
| `go_exported` | `func (k *Keeper) ListTokens(ctx sdk.Context, before, after []types.TokenParams) ([]types.TokenParams, []*ethtypes.Log, error)` | [modules/spot/keeper/vault.go](../../../modules/spot/keeper/vault.go) |
| source | — | [modules/spot/types/params.go](../../../modules/spot/types/params.go) |

<a id="spot-params-governance"></a>

## spot-resting

| Kind | Declaration | Source |
|---|---|---|
| source | — | [modules/spot/abci.go](../../../modules/spot/abci.go) |

## spot-served-set

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `func NewServedSet(keepers utils.Keepers) (map[common.Address]*Precompile, error)` | [precompiles/spot/setup.go](../../../precompiles/spot/setup.go) |

## spot-share-token

| Kind | Declaration | Source |
|---|---|---|
| `precompile_method` | `function allowance(address,address) view returns (uint256)` | [precompiles/spot/abi/share.json](../../../precompiles/spot/abi/share.json) |
| `precompile_method` | `function approve(address,uint256) nonpayable returns (bool)` | [precompiles/spot/abi/share.json](../../../precompiles/spot/abi/share.json) |
| `precompile_method` | `function balanceOf(address) view returns (uint256)` | [precompiles/spot/abi/share.json](../../../precompiles/spot/abi/share.json) |
| `precompile_method` | `function decimals() view returns (uint8)` | [precompiles/spot/abi/share.json](../../../precompiles/spot/abi/share.json) |
| `precompile_method` | `function name() view returns (string)` | [precompiles/spot/abi/share.json](../../../precompiles/spot/abi/share.json) |
| `precompile_method` | `function symbol() view returns (string)` | [precompiles/spot/abi/share.json](../../../precompiles/spot/abi/share.json) |
| `precompile_method` | `function totalSupply() view returns (uint256)` | [precompiles/spot/abi/share.json](../../../precompiles/spot/abi/share.json) |
| `precompile_method` | `function transfer(address,uint256) nonpayable returns (bool)` | [precompiles/spot/abi/share.json](../../../precompiles/spot/abi/share.json) |
| `precompile_method` | `function transferFrom(address,address,uint256) nonpayable returns (bool)` | [precompiles/spot/abi/share.json](../../../precompiles/spot/abi/share.json) |
| `precompile_event` | `event Approval(address,address,uint256)` | [precompiles/spot/abi/share.json](../../../precompiles/spot/abi/share.json) |
| `precompile_event` | `event Transfer(address,address,uint256)` | [precompiles/spot/abi/share.json](../../../precompiles/spot/abi/share.json) |
| `precompile_error` | `error ERC20InsufficientAllowance(address,uint256,uint256)` | [precompiles/spot/abi/share.json](../../../precompiles/spot/abi/share.json) |
| `precompile_error` | `error ERC20InsufficientBalance(address,uint256,uint256)` | [precompiles/spot/abi/share.json](../../../precompiles/spot/abi/share.json) |
| `precompile_error` | `error ERC20InvalidApprover(address)` | [precompiles/spot/abi/share.json](../../../precompiles/spot/abi/share.json) |
| `precompile_error` | `error ERC20InvalidReceiver(address)` | [precompiles/spot/abi/share.json](../../../precompiles/spot/abi/share.json) |
| `precompile_error` | `error ERC20InvalidSender(address)` | [precompiles/spot/abi/share.json](../../../precompiles/spot/abi/share.json) |
| `precompile_error` | `error ERC20InvalidSpender(address)` | [precompiles/spot/abi/share.json](../../../precompiles/spot/abi/share.json) |

## spot-storage

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `func ActiveIndexInvariant(k *Keeper) sdk.Invariant` | [modules/spot/keeper/invariants.go](../../../modules/spot/keeper/invariants.go) |
| `go_exported` | `func (m Migrator) Migrate1to2(ctx sdk.Context) error` | [modules/spot/keeper/migrations.go](../../../modules/spot/keeper/migrations.go) |
| `storage_key` | `ActiveLengthPrefix = []byte{0x16}` | [modules/spot/types/keys.go](../../../modules/spot/types/keys.go) |
| `storage_key` | `ActivePositionPrefix = []byte{0x15}` | [modules/spot/types/keys.go](../../../modules/spot/types/keys.go) |
| `storage_key` | `ParamsKey = []byte{0x01}` | [modules/spot/types/keys.go](../../../modules/spot/types/keys.go) |
| `storage_key` | `BookPrefix = []byte{0x02}` | [modules/spot/types/keys.go](../../../modules/spot/types/keys.go) |
| `storage_key` | `EscrowPrefix = []byte{0x03}` | [modules/spot/types/keys.go](../../../modules/spot/types/keys.go) |
| `storage_key` | `FeeAccrualPrefix = []byte{0x04}` | [modules/spot/types/keys.go](../../../modules/spot/types/keys.go) |
| `storage_key` | `NextOrderIDKey = []byte{0x05}` | [modules/spot/types/keys.go](../../../modules/spot/types/keys.go) |
| `storage_key` | `LimitOrderPrefix = []byte{0x06}` | [modules/spot/types/keys.go](../../../modules/spot/types/keys.go) |
| `storage_key` | `StopLimitOrderPrefix = []byte{0x07}` | [modules/spot/types/keys.go](../../../modules/spot/types/keys.go) |
| `storage_key` | `UserOrderPrefix = []byte{0x08}` | [modules/spot/types/keys.go](../../../modules/spot/types/keys.go) |
| `storage_key` | `ActiveLimitPrefix = []byte{0x09}` | [modules/spot/types/keys.go](../../../modules/spot/types/keys.go) |
| `storage_key` | `ActiveStopLimitPrefix = []byte{0x0A}` | [modules/spot/types/keys.go](../../../modules/spot/types/keys.go) |
| `storage_key` | `ActiveStopLossPrefix = []byte{0x0B}` | [modules/spot/types/keys.go](../../../modules/spot/types/keys.go) |
| `storage_key` | `TwapSamplePrefix = []byte{0x0C}` | [modules/spot/types/keys.go](../../../modules/spot/types/keys.go) |
| `storage_key` | `TwapRingPrefix = []byte{0x0D}` | [modules/spot/types/keys.go](../../../modules/spot/types/keys.go) |
| `storage_key` | `VolumePrefix = []byte{0x0E}` | [modules/spot/types/keys.go](../../../modules/spot/types/keys.go) |
| `storage_key` | `StopLossOrderPrefix = []byte{0x0F}` | [modules/spot/types/keys.go](../../../modules/spot/types/keys.go) |
| `storage_key` | `OrderMinOutputPrefix = []byte{0x10}` | [modules/spot/types/keys.go](../../../modules/spot/types/keys.go) |
| `storage_key` | `ExpiryIndexPrefix = []byte{0x11}` | [modules/spot/types/keys.go](../../../modules/spot/types/keys.go) |
| `storage_key` | `CursorPrefix = []byte{0x12}` | [modules/spot/types/keys.go](../../../modules/spot/types/keys.go) |
| `storage_key` | `FillBackoffPrefix = []byte{0x13}` | [modules/spot/types/keys.go](../../../modules/spot/types/keys.go) |
| `storage_key` | `ActiveCountPrefix = []byte{0x14}` | [modules/spot/types/keys.go](../../../modules/spot/types/keys.go) |

## msg-spot-syncreserves

| Kind | Declaration | Source |
|---|---|---|
| source | — | [api/spot/tx.proto](../../../api/spot/tx.proto) |

## msg-spot-updateparams

| Kind | Declaration | Source |
|---|---|---|
| source | — | [api/spot/tx.proto](../../../api/spot/tx.proto) |

## proto-spot-msgsyncreserves

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message MsgSyncReserves {` | [api/spot/tx.proto](../../../api/spot/tx.proto) |

## proto-spot-msgsyncreservesresponse

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `MsgSyncReservesResponse` | [api/spot/tx.proto](../../../api/spot/tx.proto) |

## proto-spot-msgupdateparams

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message MsgUpdateParams {` | [api/spot/tx.proto](../../../api/spot/tx.proto) |

## proto-spot-msgupdateparamsresponse

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `MsgUpdateParamsResponse` | [api/spot/tx.proto](../../../api/spot/tx.proto) |

## go-exported-modules-spot

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `func EndBlocker(ctx sdk.Context, k *keeper.Keeper) error` | [modules/spot/abci.go](../../../modules/spot/abci.go) |
| `go_exported` | `type AppModuleBasic struct{}` | [modules/spot/module.go](../../../modules/spot/module.go) |
| `go_exported` | `func NewAppModule(k *keeper.Keeper) AppModule` | [modules/spot/module.go](../../../modules/spot/module.go) |

## go-exported-modules-spot-keeper

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `type Run struct` | [modules/spot/keeper/custody.go](../../../modules/spot/keeper/custody.go) |
| `go_exported` | `func WithRun(ctx sdk.Context, run *Run) sdk.Context { return ctx.WithValue(runKey{}, run) }` | [modules/spot/keeper/custody.go](../../../modules/spot/keeper/custody.go) |
| `go_exported` | `func CustodyAddress() sdk.AccAddress { return authtypes.NewModuleAddress(types.ModuleName) }` | [modules/spot/keeper/custody.go](../../../modules/spot/keeper/custody.go) |
| `go_exported` | `func (k *Keeper) NonReentrant(ctx sdk.Context, fn func(sdk.Context) (sdk.Context, error)) (sdk.Context, error)` | [modules/spot/keeper/custody.go](../../../modules/spot/keeper/custody.go) |
| `go_exported` | `func (k *Keeper) ExecuteResting(ctx sdk.Context) (RestingExecution, error)` | [modules/spot/keeper/execute.go](../../../modules/spot/keeper/execute.go) |
| `go_exported` | `func Tier1Threshold() sdk.Int { return sdk.NewIntWithDecimal(10_000, 18) }` | [modules/spot/keeper/fees.go](../../../modules/spot/keeper/fees.go) |
| `go_exported` | `func Tier2Threshold() sdk.Int { return sdk.NewIntWithDecimal(100_000, 18) }` | [modules/spot/keeper/fees.go](../../../modules/spot/keeper/fees.go) |
| `go_exported` | `func (k *Keeper) Price(ctx sdk.Context, rail, input, output common.Address, amount sdk.Int) (PriceResult, error)` | [modules/spot/keeper/fees.go](../../../modules/spot/keeper/fees.go) |
| `go_exported` | `func (k *Keeper) QuoteExactOut(ctx sdk.Context, rail, input, output common.Address, target sdk.Int) (sdk.Int, error)` | [modules/spot/keeper/fees.go](../../../modules/spot/keeper/fees.go) |
| `go_exported` | `func (k *Keeper) AccumulatedFees(ctx sdk.Context, rail, token common.Address) (sdk.Int, error)` | [modules/spot/keeper/fees.go](../../../modules/spot/keeper/fees.go) |
| `go_exported` | `func (k *Keeper) CollectFees(ctx sdk.Context, rail, collector, token common.Address) ([]*ethtypes.Log, error)` | [modules/spot/keeper/fees.go](../../../modules/spot/keeper/fees.go) |
| `go_exported` | `func (k *Keeper) InitGenesis(ctx sdk.Context, gs types.GenesisState)` | [modules/spot/keeper/genesis.go](../../../modules/spot/keeper/genesis.go) |
| `go_exported` | `func (k *Keeper) EnsureModuleAccount(ctx sdk.Context) error` | [modules/spot/keeper/genesis.go](../../../modules/spot/keeper/genesis.go) |
| `go_exported` | `func (k *Keeper) ExportGenesis(ctx sdk.Context) *types.GenesisState` | [modules/spot/keeper/genesis.go](../../../modules/spot/keeper/genesis.go) |
| `go_exported` | `var ErrAlreadyInitialised = errors.New("spot: already initialised")` | [modules/spot/keeper/init_v6120.go](../../../modules/spot/keeper/init_v6120.go) |
| `go_exported` | `func InitV6120(ctx sdk.Context, k Keeper) error` | [modules/spot/keeper/init_v6120.go](../../../modules/spot/keeper/init_v6120.go) |
| `go_exported` | `SolvencyInvariantName     = "spot-solvency"` | [modules/spot/keeper/invariants.go](../../../modules/spot/keeper/invariants.go) |
| `go_exported` | `ShareBackingInvariantName = "spot-share-backing"` | [modules/spot/keeper/invariants.go](../../../modules/spot/keeper/invariants.go) |
| `go_exported` | `EscrowInvariantName       = "spot-escrow"` | [modules/spot/keeper/invariants.go](../../../modules/spot/keeper/invariants.go) |
| `go_exported` | `func RegisterInvariants(registry sdk.InvariantRegistry, k *Keeper)` | [modules/spot/keeper/invariants.go](../../../modules/spot/keeper/invariants.go) |
| `go_exported` | `type Keeper struct` | [modules/spot/keeper/keeper.go](../../../modules/spot/keeper/keeper.go) |
| `go_exported` | `func NewKeeper(storeKey sdk.StoreKey, bankKeeper BankKeeper, evmKeeper EVMKeeper,` | [modules/spot/keeper/keeper.go](../../../modules/spot/keeper/keeper.go) |
| `go_exported` | `func (k *Keeper) Authority() string { return k.authority }` | [modules/spot/keeper/keeper.go](../../../modules/spot/keeper/keeper.go) |
| `go_exported` | `func (k *Keeper) GetParams(ctx sdk.Context) (types.Params, error)` | [modules/spot/keeper/keeper.go](../../../modules/spot/keeper/keeper.go) |
| `go_exported` | `ReservesUpdatedTopic       = crypto.Keccak256Hash([]byte("ReservesUpdated(address,uint256,uint256)"))` | [modules/spot/keeper/msg_server.go](../../../modules/spot/keeper/msg_server.go) |
| `go_exported` | `func NewMsgServerImpl(k *Keeper) types.MsgServer { return msgServer{keeper: k} }` | [modules/spot/keeper/msg_server.go](../../../modules/spot/keeper/msg_server.go) |
| `go_exported` | `type LimitOrderEvent struct` | [modules/spot/keeper/orders.go](../../../modules/spot/keeper/orders.go) |
| `go_exported` | `func (k *Keeper) NextOrderID(ctx sdk.Context) (uint64, error)` | [modules/spot/keeper/orders.go](../../../modules/spot/keeper/orders.go) |
| `go_exported` | `func (k *Keeper) GetLimitOrder(ctx sdk.Context, id uint64) (types.LimitOrder, error)` | [modules/spot/keeper/orders.go](../../../modules/spot/keeper/orders.go) |
| `go_exported` | `func (k *Keeper) GetUserLimitOrders(ctx sdk.Context, user common.Address) ([]uint64, error)` | [modules/spot/keeper/orders.go](../../../modules/spot/keeper/orders.go) |
| `go_exported` | `func (k *Keeper) GetActiveLimitOrderCount(ctx sdk.Context) (uint64, error)` | [modules/spot/keeper/orders.go](../../../modules/spot/keeper/orders.go) |
| `go_exported` | `func (k *Keeper) ActiveLimitOrderID(ctx sdk.Context, position uint64) (uint64, error)` | [modules/spot/keeper/orders.go](../../../modules/spot/keeper/orders.go) |
| `go_exported` | `func (k *Keeper) PlaceLimit(ctx sdk.Context, user, stablecoin, token common.Address, amount, target sdk.Int, expiry uint64, kind types.OrderType) (uint64, []LimitOrderEvent, error)` | [modules/spot/keeper/orders.go](../../../modules/spot/keeper/orders.go) |
| `go_exported` | `func (k *Keeper) ModifyLimit(ctx sdk.Context, user common.Address, id uint64, target sdk.Int, expiry uint64) ([]LimitOrderEvent, error)` | [modules/spot/keeper/orders.go](../../../modules/spot/keeper/orders.go) |
| `go_exported` | `func (k *Keeper) CancelLimit(ctx sdk.Context, user common.Address, id uint64) ([]LimitOrderEvent, error)` | [modules/spot/keeper/orders.go](../../../modules/spot/keeper/orders.go) |
| `go_exported` | `func (k *Keeper) ExpireLimit(ctx sdk.Context, id uint64) ([]LimitOrderEvent, error)` | [modules/spot/keeper/orders.go](../../../modules/spot/keeper/orders.go) |
| `go_exported` | `func (k *Keeper) CanExecuteLimitOrder(ctx sdk.Context, id uint64) (bool, string, error)` | [modules/spot/keeper/orders.go](../../../modules/spot/keeper/orders.go) |
| `go_exported` | `func (k *Keeper) GetExecutableLimitOrders(ctx sdk.Context, maxCount uint64) ([]uint64, error)` | [modules/spot/keeper/orders.go](../../../modules/spot/keeper/orders.go) |
| `go_exported` | `func (k *Keeper) ExecuteLimit(ctx sdk.Context, id uint64) ([]LimitOrderEvent, error)` | [modules/spot/keeper/orders.go](../../../modules/spot/keeper/orders.go) |
| `go_exported` | `func (k *Keeper) IndexPrice(ctx sdk.Context, token common.Address) (sdk.Int, error)` | [modules/spot/keeper/pricing.go](../../../modules/spot/keeper/pricing.go) |
| `go_exported` | `type QuoteResult struct` | [modules/spot/keeper/quote.go](../../../modules/spot/keeper/quote.go) |
| `go_exported` | `func (k *Keeper) Quote(ctx sdk.Context, tokenIn, tokenOut common.Address, amount sdk.Int, exactIn bool) QuoteResult` | [modules/spot/keeper/quote.go](../../../modules/spot/keeper/quote.go) |
| `go_exported` | `func (k *Keeper) LiquidityInfo(ctx sdk.Context, token common.Address) (available, price *big.Int, stale bool,` | [modules/spot/keeper/quote.go](../../../modules/spot/keeper/quote.go) |
| `go_exported` | `func (k *Keeper) TokenPrice(ctx sdk.Context, token common.Address) (price, timestamp *big.Int, stale bool)` | [modules/spot/keeper/quote.go](../../../modules/spot/keeper/quote.go) |
| `go_exported` | `const ShareDenomPrefix = "spot/share/"` | [modules/spot/keeper/shares.go](../../../modules/spot/keeper/shares.go) |
| `go_exported` | `var ShareTransferTopic = crypto.Keccak256Hash([]byte("Transfer(address,address,uint256)"))` | [modules/spot/keeper/shares.go](../../../modules/spot/keeper/shares.go) |
| `go_exported` | `func (k *Keeper) ShareDenom(token common.Address) string` | [modules/spot/keeper/shares.go](../../../modules/spot/keeper/shares.go) |
| `go_exported` | `func (k *Keeper) GetStopLimitOrder(ctx sdk.Context, id uint64) (types.StopLimitOrder, error)` | [modules/spot/keeper/stoporders.go](../../../modules/spot/keeper/stoporders.go) |
| `go_exported` | `func (k *Keeper) GetStopLossOrder(ctx sdk.Context, id uint64) (types.StopLossOrder, error)` | [modules/spot/keeper/stoporders.go](../../../modules/spot/keeper/stoporders.go) |
| `go_exported` | `func (k *Keeper) PlaceStopLimitOrder(ctx sdk.Context, user, stablecoin, token common.Address, amount, stopPrice, limitPrice sdk.Int, expiresAt uint64, kind types.OrderType) (types.StopLimitOrder, error)` | [modules/spot/keeper/stoporders.go](../../../modules/spot/keeper/stoporders.go) |
| `go_exported` | `func (k *Keeper) PlaceStopLossOrder(ctx sdk.Context, user, token, stablecoin common.Address, amount, triggerPrice sdk.Int, expiresAt uint64) (types.StopLossOrder, error)` | [modules/spot/keeper/stoporders.go](../../../modules/spot/keeper/stoporders.go) |
| `go_exported` | `func (k *Keeper) ActivateStopLimit(ctx sdk.Context, id uint64) (bool, error)` | [modules/spot/keeper/stoporders.go](../../../modules/spot/keeper/stoporders.go) |
| `go_exported` | `func (k *Keeper) ExecuteStopLimit(ctx sdk.Context, id uint64) error` | [modules/spot/keeper/stoporders.go](../../../modules/spot/keeper/stoporders.go) |
| `go_exported` | `func (k *Keeper) ExecuteStopLoss(ctx sdk.Context, id uint64) error` | [modules/spot/keeper/stoporders.go](../../../modules/spot/keeper/stoporders.go) |
| `go_exported` | `func (k *Keeper) CancelStopLimit(ctx sdk.Context, caller common.Address, id uint64) error` | [modules/spot/keeper/stoporders.go](../../../modules/spot/keeper/stoporders.go) |
| `go_exported` | `func (k *Keeper) CancelStopLoss(ctx sdk.Context, caller common.Address, id uint64) error` | [modules/spot/keeper/stoporders.go](../../../modules/spot/keeper/stoporders.go) |
| `go_exported` | `func (k *Keeper) BatchCheckAndExecuteStopLimits(ctx sdk.Context, caller common.Address, ids []uint64) (uint64, uint64, error)` | [modules/spot/keeper/stoporders.go](../../../modules/spot/keeper/stoporders.go) |
| `go_exported` | `func (k *Keeper) BatchExecuteStopLossOrders(ctx sdk.Context, caller common.Address, ids []uint64) (uint64, error)` | [modules/spot/keeper/stoporders.go](../../../modules/spot/keeper/stoporders.go) |
| `go_exported` | `func (k *Keeper) GetActiveStopLimitOrderCount(ctx sdk.Context) (uint64, error)` | [modules/spot/keeper/stoporders.go](../../../modules/spot/keeper/stoporders.go) |
| `go_exported` | `func (k *Keeper) GetActiveStopLossOrderCount(ctx sdk.Context) (uint64, error)` | [modules/spot/keeper/stoporders.go](../../../modules/spot/keeper/stoporders.go) |
| `go_exported` | `func (k *Keeper) ActiveStopLimitOrderIDs(ctx sdk.Context, index uint64) (uint64, error)` | [modules/spot/keeper/stoporders.go](../../../modules/spot/keeper/stoporders.go) |
| `go_exported` | `func (k *Keeper) ActiveStopLossOrderIDs(ctx sdk.Context, index uint64) (uint64, error)` | [modules/spot/keeper/stoporders.go](../../../modules/spot/keeper/stoporders.go) |
| `go_exported` | `func (k *Keeper) GetUserStopLimitOrders(ctx sdk.Context, user common.Address) ([]uint64, error)` | [modules/spot/keeper/stoporders.go](../../../modules/spot/keeper/stoporders.go) |
| `go_exported` | `func (k *Keeper) GetUserStopLossOrders(ctx sdk.Context, user common.Address) ([]uint64, error)` | [modules/spot/keeper/stoporders.go](../../../modules/spot/keeper/stoporders.go) |
| `go_exported` | `func (k *Keeper) GetActivatableStopLimits(ctx sdk.Context, maxCount uint64) ([]uint64, error)` | [modules/spot/keeper/stoporders.go](../../../modules/spot/keeper/stoporders.go) |
| `go_exported` | `func (k *Keeper) GetExecutableStopLimits(ctx sdk.Context, maxCount uint64) ([]uint64, error)` | [modules/spot/keeper/stoporders.go](../../../modules/spot/keeper/stoporders.go) |
| `go_exported` | `type SwapResult struct` | [modules/spot/keeper/swap.go](../../../modules/spot/keeper/swap.go) |
| `go_exported` | `func (k *Keeper) MarketBuy(ctx sdk.Context, user, stablecoin, token common.Address, amount, minimum, deadline *big.Int) (SwapResult, error)` | [modules/spot/keeper/swap.go](../../../modules/spot/keeper/swap.go) |
| `go_exported` | `func (k *Keeper) MarketSell(ctx sdk.Context, user, token, stablecoin common.Address, amount, minimum, deadline *big.Int) (SwapResult, error)` | [modules/spot/keeper/swap.go](../../../modules/spot/keeper/swap.go) |
| `go_exported` | `func (k *Keeper) SwapExactIn(ctx sdk.Context, user, tokenIn, tokenOut common.Address, amount, minimum, deadline *big.Int) (SwapResult, error)` | [modules/spot/keeper/swap.go](../../../modules/spot/keeper/swap.go) |
| `go_exported` | `func (k *Keeper) SwapExactOut(ctx sdk.Context, user, tokenIn, tokenOut common.Address, amountOut, maximum, deadline *big.Int) (SwapResult, error)` | [modules/spot/keeper/swap.go](../../../modules/spot/keeper/swap.go) |
| `go_exported` | `func (k *Keeper) SwapNative(ctx sdk.Context, user, tokenOther common.Address, amount, minimum, deadline *big.Int, nativeIsInput bool) (SwapResult, error)` | [modules/spot/keeper/swap.go](../../../modules/spot/keeper/swap.go) |
| `go_exported` | `func (k *Keeper) TWAP(ctx sdk.Context, token common.Address, period uint64) (sdk.Int, error)` | [modules/spot/keeper/twap.go](../../../modules/spot/keeper/twap.go) |
| `go_exported` | `DepositTopic          = crypto.Keccak256Hash([]byte("Deposit(address,address,uint256,uint256)"))` | [modules/spot/keeper/vault.go](../../../modules/spot/keeper/vault.go) |
| `go_exported` | `LPWithdrawalTopic     = crypto.Keccak256Hash([]byte("LPWithdrawal(address,address,uint256,uint256)"))` | [modules/spot/keeper/vault.go](../../../modules/spot/keeper/vault.go) |
| `go_exported` | `NativeDepositTopic    = crypto.Keccak256Hash([]byte("NativeDeposit(address,uint256,uint256)"))` | [modules/spot/keeper/vault.go](../../../modules/spot/keeper/vault.go) |
| `go_exported` | `NativeWithdrawalTopic = crypto.Keccak256Hash([]byte("NativeWithdrawal(address,uint256,uint256)"))` | [modules/spot/keeper/vault.go](../../../modules/spot/keeper/vault.go) |
| `go_exported` | `WithdrawalTopic       = crypto.Keccak256Hash([]byte("Withdrawal(address,address,uint256,uint256)"))` | [modules/spot/keeper/vault.go](../../../modules/spot/keeper/vault.go) |
| `go_exported` | `func (k *Keeper) GetBook(ctx sdk.Context, token common.Address) (types.TokenBook, error)` | [modules/spot/keeper/vault.go](../../../modules/spot/keeper/vault.go) |
| `go_exported` | `func (k *Keeper) UntrackedFunds(ctx sdk.Context, token common.Address) (sdk.Int, error)` | [modules/spot/keeper/vault.go](../../../modules/spot/keeper/vault.go) |
| `go_exported` | `func MaxUtilizationBps(params types.TokenParams) uint64` | [modules/spot/keeper/vault.go](../../../modules/spot/keeper/vault.go) |
| `go_exported` | `func (k *Keeper) HasLiquidity(ctx sdk.Context, token common.Address, amount sdk.Int) (bool, error)` | [modules/spot/keeper/vault.go](../../../modules/spot/keeper/vault.go) |
| `go_exported` | `func (k *Keeper) DepositBatch(ctx sdk.Context, depositor common.Address, tokens []common.Address,` | [modules/spot/keeper/vault.go](../../../modules/spot/keeper/vault.go) |
| `go_exported` | `func (k *Keeper) DepositNative(ctx sdk.Context, depositor common.Address, value *big.Int) (sdk.Int, []*ethtypes.Log, error)` | [modules/spot/keeper/vault.go](../../../modules/spot/keeper/vault.go) |
| `go_exported` | `func (k *Keeper) Withdraw(ctx sdk.Context, provider, token common.Address, shares sdk.Int) (sdk.Int, []*ethtypes.Log, error)` | [modules/spot/keeper/vault.go](../../../modules/spot/keeper/vault.go) |
| `go_exported` | `func (k *Keeper) WithdrawNative(ctx sdk.Context, provider common.Address, amount sdk.Int,` | [modules/spot/keeper/vault.go](../../../modules/spot/keeper/vault.go) |

## go-exported-modules-spot-types

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `type AbiError struct` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `go_exported` | `func (e *AbiError) Error() string { return e.code.Error() }` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `go_exported` | `func (e *AbiError) Is(target error) bool` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `go_exported` | `func (e *AbiError) Data() []byte` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `go_exported` | `var ErrInvalidGenesis = sdkerrors.Register(ModuleName, 2, "invalid genesis")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `go_exported` | `ErrCannotCancel                 = newAbiError(3, "CannotCancel()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `go_exported` | `ErrERC20InsufficientBalance     = newAbiError(5, "ERC20InsufficientBalance(address,uint256,uint256)")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `go_exported` | `ErrERC20InvalidReceiver         = newAbiError(7, "ERC20InvalidReceiver(address)")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `go_exported` | `ErrERC20InvalidSender           = newAbiError(8, "ERC20InvalidSender(address)")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `go_exported` | `ErrEnforcedPause                = newAbiError(10, "EnforcedPause()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `go_exported` | `ErrExcessiveInput               = newAbiError(11, "ExcessiveInput()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `go_exported` | `ErrExpectedPause                = newAbiError(12, "ExpectedPause()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `go_exported` | `ErrExpired                      = newAbiError(13, "Expired()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `go_exported` | `ErrFeeExceedsMax                = newAbiError(14, "FeeExceedsMax()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `go_exported` | `ErrFeeTooHigh                   = newAbiError(15, "FeeTooHigh()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `go_exported` | `ErrInsufficientLiquidity        = newAbiError(16, "InsufficientLiquidity()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `go_exported` | `ErrInsufficientOutput           = newAbiError(17, "InsufficientOutput()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `go_exported` | `ErrInvalidArrayLength           = newAbiError(18, "InvalidArrayLength()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `go_exported` | `ErrInvalidExpiry                = newAbiError(19, "InvalidExpiry()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `go_exported` | `ErrInvalidOracle                = newAbiError(20, "InvalidOracle()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `go_exported` | `ErrInvalidPrice                 = newAbiError(21, "InvalidPrice()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `go_exported` | `ErrInvalidPrices                = newAbiError(22, "InvalidPrices()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `go_exported` | `ErrInvalidTWAPPeriod            = newAbiError(23, "InvalidTWAPPeriod()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `go_exported` | `ErrInvalidVault                 = newAbiError(24, "InvalidVault()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `go_exported` | `ErrLimitMustBeAboveStopBuy      = newAbiError(25, "LimitMustBeAboveStopBuy()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `go_exported` | `ErrLimitMustBeBelowStopSell     = newAbiError(26, "LimitMustBeBelowStopSell()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `go_exported` | `ErrNativeTransferFailed         = newAbiError(28, "NativeTransferFailed()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `go_exported` | `ErrNoFeesToCollect              = newAbiError(29, "NoFeesToCollect()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `go_exported` | `ErrNotAStablecoin               = newAbiError(30, "NotAStablecoin()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `go_exported` | `ErrNotActivated                 = newAbiError(31, "NotActivated()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `go_exported` | `ErrNotAuthorized                = newAbiError(32, "NotAuthorized()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `go_exported` | `ErrNotFeeCollector              = newAbiError(33, "NotFeeCollector()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `go_exported` | `ErrNotOrderOwner                = newAbiError(34, "NotOrderOwner()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `go_exported` | `ErrNotPending                   = newAbiError(35, "NotPending()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `go_exported` | `ErrOrderBelowMinimumNotional    = newAbiError(36, "OrderBelowMinimumNotional()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `go_exported` | `ErrOrderExpired                 = newAbiError(37, "OrderExpired()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `go_exported` | `ErrOrderNotPending              = newAbiError(38, "OrderNotPending()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `go_exported` | `ErrOwnableUnauthorizedAccount   = newAbiError(40, "OwnableUnauthorizedAccount(address)")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `go_exported` | `ErrPriceAboveLimit              = newAbiError(41, "PriceAboveLimit()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `go_exported` | `ErrPriceAboveTarget             = newAbiError(42, "PriceAboveTarget()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `go_exported` | `ErrPriceAboveTrigger            = newAbiError(43, "PriceAboveTrigger()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `go_exported` | `ErrPriceBelowLimit              = newAbiError(44, "PriceBelowLimit()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `go_exported` | `ErrPriceBelowTarget             = newAbiError(45, "PriceBelowTarget()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `go_exported` | `ErrReentrancyGuardReentrantCall = newAbiError(46, "ReentrancyGuardReentrantCall()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `go_exported` | `ErrSameToken                    = newAbiError(48, "SameToken()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `go_exported` | `ErrScalarTooHigh                = newAbiError(49, "ScalarTooHigh()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `go_exported` | `ErrStalePrice                   = newAbiError(50, "StalePrice()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `go_exported` | `ErrTier2BelowTier1              = newAbiError(51, "Tier2BelowTier1()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `go_exported` | `ErrTokenAlreadyRegistered       = newAbiError(52, "TokenAlreadyRegistered()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `go_exported` | `ErrTokenIsStablecoin            = newAbiError(53, "TokenIsStablecoin()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `go_exported` | `ErrTokenNotRegistered           = newAbiError(54, "TokenNotRegistered()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `go_exported` | `ErrTokenPaused                  = newAbiError(55, "TokenPaused()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `go_exported` | `ErrTooManyActiveOrders          = newAbiError(56, "TooManyActiveOrders()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `go_exported` | `ErrTwapWindowInvalid            = newAbiError(57, "TwapWindowInvalid()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `go_exported` | `ErrUnsupportedInputTransfer     = newAbiError(58, "UnsupportedInputTransfer()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `go_exported` | `ErrUnsupportedToken             = newAbiError(59, "UnsupportedToken()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `go_exported` | `ErrUnsupportedTransfer          = newAbiError(60, "UnsupportedTransfer()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `go_exported` | `ErrUseWethDeposit               = newAbiError(61, "UseWethDeposit()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `go_exported` | `ErrUseWethWithdraw              = newAbiError(62, "UseWethWithdraw()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `go_exported` | `ErrWethOnly                     = newAbiError(63, "WethOnly()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `go_exported` | `ErrZeroAddress                  = newAbiError(64, "ZeroAddress()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `go_exported` | `ErrZeroAmount                   = newAbiError(65, "ZeroAmount()")` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `go_exported` | `func ERC20InsufficientAllowance(spender common.Address, allowance, needed *uint256.Int) *AbiError` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `go_exported` | `func ERC20InsufficientBalance(sender common.Address, balance, needed *uint256.Int) *AbiError` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `go_exported` | `func ERC20InvalidApprover(approver common.Address) *AbiError` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `go_exported` | `func ERC20InvalidReceiver(receiver common.Address) *AbiError` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `go_exported` | `func ERC20InvalidSender(sender common.Address) *AbiError` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `go_exported` | `func ERC20InvalidSpender(spender common.Address) *AbiError` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `go_exported` | `func MulticallFailed(index *uint256.Int) *AbiError` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `go_exported` | `func OwnableInvalidOwner(owner common.Address) *AbiError` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `go_exported` | `func OwnableUnauthorizedAccount(account common.Address) *AbiError` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `go_exported` | `func SafeERC20FailedOperation(token common.Address) *AbiError` | [modules/spot/types/errors.go](../../../modules/spot/types/errors.go) |
| `go_exported` | `const TwapHistoryDepth = 1000` | [modules/spot/types/genesis.go](../../../modules/spot/types/genesis.go) |
| `go_exported` | `type TokenBook struct` | [modules/spot/types/genesis.go](../../../modules/spot/types/genesis.go) |
| `go_exported` | `type TokenEscrow struct` | [modules/spot/types/genesis.go](../../../modules/spot/types/genesis.go) |
| `go_exported` | `type FeeRail uint8` | [modules/spot/types/genesis.go](../../../modules/spot/types/genesis.go) |
| `go_exported` | `FeeRailSwap FeeRail = iota` | [modules/spot/types/genesis.go](../../../modules/spot/types/genesis.go) |
| `go_exported` | `FeeRailOrder` | [modules/spot/types/genesis.go](../../../modules/spot/types/genesis.go) |
| `go_exported` | `type FeeAccrual struct` | [modules/spot/types/genesis.go](../../../modules/spot/types/genesis.go) |
| `go_exported` | `type UserOrder struct` | [modules/spot/types/genesis.go](../../../modules/spot/types/genesis.go) |
| `go_exported` | `type TwapSample struct` | [modules/spot/types/genesis.go](../../../modules/spot/types/genesis.go) |
| `go_exported` | `type TwapRing struct` | [modules/spot/types/genesis.go](../../../modules/spot/types/genesis.go) |
| `go_exported` | `type TokenVolume struct` | [modules/spot/types/genesis.go](../../../modules/spot/types/genesis.go) |
| `go_exported` | `type OrderExpiry struct` | [modules/spot/types/genesis.go](../../../modules/spot/types/genesis.go) |
| `go_exported` | `type FillIndex uint8` | [modules/spot/types/genesis.go](../../../modules/spot/types/genesis.go) |
| `go_exported` | `FillIndexStopLimit FillIndex = iota` | [modules/spot/types/genesis.go](../../../modules/spot/types/genesis.go) |
| `go_exported` | `FillIndexLimit` | [modules/spot/types/genesis.go](../../../modules/spot/types/genesis.go) |
| `go_exported` | `FillIndexStopLoss` | [modules/spot/types/genesis.go](../../../modules/spot/types/genesis.go) |
| `go_exported` | `type EndBlockCursors struct` | [modules/spot/types/genesis.go](../../../modules/spot/types/genesis.go) |
| `go_exported` | `type FillBackoff struct` | [modules/spot/types/genesis.go](../../../modules/spot/types/genesis.go) |
| `go_exported` | `type ActiveOrderCount struct` | [modules/spot/types/genesis.go](../../../modules/spot/types/genesis.go) |
| `go_exported` | `type GenesisState struct` | [modules/spot/types/genesis.go](../../../modules/spot/types/genesis.go) |
| `go_exported` | `func DefaultGenesis() *GenesisState` | [modules/spot/types/genesis.go](../../../modules/spot/types/genesis.go) |
| `go_exported` | `func (gs GenesisState) Validate() error` | [modules/spot/types/genesis.go](../../../modules/spot/types/genesis.go) |
| `go_exported` | `ModuleName = "spot"` | [modules/spot/types/keys.go](../../../modules/spot/types/keys.go) |
| `go_exported` | `StoreKey = ModuleName` | [modules/spot/types/keys.go](../../../modules/spot/types/keys.go) |
| `go_exported` | `PECORAddress = "0x0000000000000000000000000000000000001021"` | [modules/spot/types/keys.go](../../../modules/spot/types/keys.go) |
| `go_exported` | `PECOROrdersAddress = "0x0000000000000000000000000000000000001022"` | [modules/spot/types/keys.go](../../../modules/spot/types/keys.go) |
| `go_exported` | `PECORStopOrdersAddress = "0x0000000000000000000000000000000000001023"` | [modules/spot/types/keys.go](../../../modules/spot/types/keys.go) |
| `go_exported` | `PECORQuoterAddress = "0x0000000000000000000000000000000000001024"` | [modules/spot/types/keys.go](../../../modules/spot/types/keys.go) |
| `go_exported` | `PECORVaultAddress = "0x0000000000000000000000000000000000001025"` | [modules/spot/types/keys.go](../../../modules/spot/types/keys.go) |
| `go_exported` | `ParamsKey             = []byte{0x01}` | [modules/spot/types/keys.go](../../../modules/spot/types/keys.go) |
| `go_exported` | `BookPrefix            = []byte{0x02}` | [modules/spot/types/keys.go](../../../modules/spot/types/keys.go) |
| `go_exported` | `EscrowPrefix          = []byte{0x03}` | [modules/spot/types/keys.go](../../../modules/spot/types/keys.go) |
| `go_exported` | `FeeAccrualPrefix      = []byte{0x04}` | [modules/spot/types/keys.go](../../../modules/spot/types/keys.go) |
| `go_exported` | `NextOrderIDKey        = []byte{0x05}` | [modules/spot/types/keys.go](../../../modules/spot/types/keys.go) |
| `go_exported` | `LimitOrderPrefix      = []byte{0x06}` | [modules/spot/types/keys.go](../../../modules/spot/types/keys.go) |
| `go_exported` | `StopLimitOrderPrefix  = []byte{0x07}` | [modules/spot/types/keys.go](../../../modules/spot/types/keys.go) |
| `go_exported` | `UserOrderPrefix       = []byte{0x08}` | [modules/spot/types/keys.go](../../../modules/spot/types/keys.go) |
| `go_exported` | `ActiveLimitPrefix     = []byte{0x09}` | [modules/spot/types/keys.go](../../../modules/spot/types/keys.go) |
| `go_exported` | `ActiveStopLimitPrefix = []byte{0x0A}` | [modules/spot/types/keys.go](../../../modules/spot/types/keys.go) |
| `go_exported` | `ActiveStopLossPrefix  = []byte{0x0B}` | [modules/spot/types/keys.go](../../../modules/spot/types/keys.go) |
| `go_exported` | `TwapSamplePrefix      = []byte{0x0C}` | [modules/spot/types/keys.go](../../../modules/spot/types/keys.go) |
| `go_exported` | `TwapRingPrefix        = []byte{0x0D}` | [modules/spot/types/keys.go](../../../modules/spot/types/keys.go) |
| `go_exported` | `VolumePrefix          = []byte{0x0E}` | [modules/spot/types/keys.go](../../../modules/spot/types/keys.go) |
| `go_exported` | `StopLossOrderPrefix   = []byte{0x0F}` | [modules/spot/types/keys.go](../../../modules/spot/types/keys.go) |
| `go_exported` | `OrderMinOutputPrefix  = []byte{0x10}` | [modules/spot/types/keys.go](../../../modules/spot/types/keys.go) |
| `go_exported` | `ExpiryIndexPrefix     = []byte{0x11}` | [modules/spot/types/keys.go](../../../modules/spot/types/keys.go) |
| `go_exported` | `CursorPrefix          = []byte{0x12}` | [modules/spot/types/keys.go](../../../modules/spot/types/keys.go) |
| `go_exported` | `FillBackoffPrefix     = []byte{0x13}` | [modules/spot/types/keys.go](../../../modules/spot/types/keys.go) |
| `go_exported` | `ActiveCountPrefix     = []byte{0x14}` | [modules/spot/types/keys.go](../../../modules/spot/types/keys.go) |
| `go_exported` | `func RegisterCodec(cdc *codec.LegacyAmino)` | [modules/spot/types/msgs.go](../../../modules/spot/types/msgs.go) |
| `go_exported` | `func RegisterInterfaces(registry cdctypes.InterfaceRegistry)` | [modules/spot/types/msgs.go](../../../modules/spot/types/msgs.go) |
| `go_exported` | `func (p Params) Marshal() ([]byte, error) { return p.MarshalJSON() }` | [modules/spot/types/msgs.go](../../../modules/spot/types/msgs.go) |
| `go_exported` | `func (p *Params) Unmarshal(data []byte) error` | [modules/spot/types/msgs.go](../../../modules/spot/types/msgs.go) |
| `go_exported` | `func (m MsgUpdateParams) Type() string { return TypeMsgUpdateParams }` | [modules/spot/types/msgs.go](../../../modules/spot/types/msgs.go) |
| `go_exported` | `func (m MsgUpdateParams) String() string { return jsonString(m) }` | [modules/spot/types/msgs.go](../../../modules/spot/types/msgs.go) |
| `go_exported` | `func (m MsgUpdateParams) ValidateBasic() error` | [modules/spot/types/msgs.go](../../../modules/spot/types/msgs.go) |
| `go_exported` | `func (m MsgSyncReserves) Type() string { return TypeMsgSyncReserves }` | [modules/spot/types/msgs.go](../../../modules/spot/types/msgs.go) |
| `go_exported` | `func (m MsgSyncReserves) String() string { return jsonString(m) }` | [modules/spot/types/msgs.go](../../../modules/spot/types/msgs.go) |
| `go_exported` | `func (m MsgSyncReserves) ValidateBasic() error` | [modules/spot/types/msgs.go](../../../modules/spot/types/msgs.go) |
| `go_exported` | `type OrderType uint8` | [modules/spot/types/orders.go](../../../modules/spot/types/orders.go) |
| `go_exported` | `OrderTypeLimitBuy OrderType = iota` | [modules/spot/types/orders.go](../../../modules/spot/types/orders.go) |
| `go_exported` | `OrderTypeLimitSell` | [modules/spot/types/orders.go](../../../modules/spot/types/orders.go) |
| `go_exported` | `OrderTypeStopLoss` | [modules/spot/types/orders.go](../../../modules/spot/types/orders.go) |
| `go_exported` | `OrderTypeStopLimitBuy` | [modules/spot/types/orders.go](../../../modules/spot/types/orders.go) |
| `go_exported` | `OrderTypeStopLimitSell` | [modules/spot/types/orders.go](../../../modules/spot/types/orders.go) |
| `go_exported` | `type OrderStatus uint8` | [modules/spot/types/orders.go](../../../modules/spot/types/orders.go) |
| `go_exported` | `OrderStatusPending OrderStatus = iota` | [modules/spot/types/orders.go](../../../modules/spot/types/orders.go) |
| `go_exported` | `OrderStatusActivated` | [modules/spot/types/orders.go](../../../modules/spot/types/orders.go) |
| `go_exported` | `OrderStatusExecuted` | [modules/spot/types/orders.go](../../../modules/spot/types/orders.go) |
| `go_exported` | `OrderStatusCancelled` | [modules/spot/types/orders.go](../../../modules/spot/types/orders.go) |
| `go_exported` | `OrderStatusExpired` | [modules/spot/types/orders.go](../../../modules/spot/types/orders.go) |
| `go_exported` | `type LimitOrder struct` | [modules/spot/types/orders.go](../../../modules/spot/types/orders.go) |
| `go_exported` | `type StopLimitOrder struct` | [modules/spot/types/orders.go](../../../modules/spot/types/orders.go) |
| `go_exported` | `type StopLossOrder struct` | [modules/spot/types/orders.go](../../../modules/spot/types/orders.go) |
| `go_exported` | `type OrderMinOutput struct` | [modules/spot/types/orders.go](../../../modules/spot/types/orders.go) |
| `go_exported` | `func (t OrderType) IsBuy() bool { return t == OrderTypeLimitBuy \|\| t == OrderTypeStopLimitBuy }` | [modules/spot/types/orders.go](../../../modules/spot/types/orders.go) |
| `go_exported` | `func (o LimitOrder) EscrowToken() common.Address` | [modules/spot/types/orders.go](../../../modules/spot/types/orders.go) |
| `go_exported` | `func (o StopLimitOrder) EscrowToken() common.Address` | [modules/spot/types/orders.go](../../../modules/spot/types/orders.go) |
| `go_exported` | `func (o StopLossOrder) EscrowToken() common.Address { return o.Token }` | [modules/spot/types/orders.go](../../../modules/spot/types/orders.go) |
| `go_exported` | `func (o LimitOrder) IsActive() bool { return o.Status == OrderStatusPending }` | [modules/spot/types/orders.go](../../../modules/spot/types/orders.go) |
| `go_exported` | `func (o StopLimitOrder) IsActive() bool` | [modules/spot/types/orders.go](../../../modules/spot/types/orders.go) |
| `go_exported` | `func (o StopLossOrder) IsActive() bool { return o.Status == OrderStatusPending }` | [modules/spot/types/orders.go](../../../modules/spot/types/orders.go) |
| `go_exported` | `func (o LimitOrder) Validate() error` | [modules/spot/types/orders.go](../../../modules/spot/types/orders.go) |
| `go_exported` | `func (o StopLimitOrder) Validate() error` | [modules/spot/types/orders.go](../../../modules/spot/types/orders.go) |
| `go_exported` | `func (o StopLossOrder) Validate() error` | [modules/spot/types/orders.go](../../../modules/spot/types/orders.go) |
| `go_exported` | `BpsDenominator = 10_000` | [modules/spot/types/params.go](../../../modules/spot/types/params.go) |
| `go_exported` | `MaxFeeBps = 200` | [modules/spot/types/params.go](../../../modules/spot/types/params.go) |
| `go_exported` | `MaxOrderFeeBps = 200` | [modules/spot/types/params.go](../../../modules/spot/types/params.go) |
| `go_exported` | `MaxImpactBps = 500` | [modules/spot/types/params.go](../../../modules/spot/types/params.go) |
| `go_exported` | `DefaultMaxUtilizationBps = 10_000` | [modules/spot/types/params.go](../../../modules/spot/types/params.go) |
| `go_exported` | `OpeningTokenMaxUtilizationBps = 9_000` | [modules/spot/types/params.go](../../../modules/spot/types/params.go) |
| `go_exported` | `MaxTokenDecimals = 36` | [modules/spot/types/params.go](../../../modules/spot/types/params.go) |
| `go_exported` | `USDLAddress = "0x85FcD13735F4309833A503EE804ea32395851479"` | [modules/spot/types/params.go](../../../modules/spot/types/params.go) |
| `go_exported` | `OpeningTokenDecimals = 6` | [modules/spot/types/params.go](../../../modules/spot/types/params.go) |
| `go_exported` | `PAXMarketID = 11` | [modules/spot/types/params.go](../../../modules/spot/types/params.go) |
| `go_exported` | `SIDMarketID = 12` | [modules/spot/types/params.go](../../../modules/spot/types/params.go) |
| `go_exported` | `type ParamsError string` | [modules/spot/types/params.go](../../../modules/spot/types/params.go) |
| `go_exported` | `func (e ParamsError) Error() string { return string(e) }` | [modules/spot/types/params.go](../../../modules/spot/types/params.go) |
| `go_exported` | `ErrParamsFeeTooHigh             ParamsError = "FeeTooHigh()"` | [modules/spot/types/params.go](../../../modules/spot/types/params.go) |
| `go_exported` | `ErrParamsTier2BelowTier1        ParamsError = "Tier2BelowTier1()"` | [modules/spot/types/params.go](../../../modules/spot/types/params.go) |
| `go_exported` | `ErrParamsScalarTooHigh          ParamsError = "ScalarTooHigh()"` | [modules/spot/types/params.go](../../../modules/spot/types/params.go) |
| `go_exported` | `ErrParamsFeeExceedsMax          ParamsError = "FeeExceedsMax()"` | [modules/spot/types/params.go](../../../modules/spot/types/params.go) |
| `go_exported` | `ErrParamsInvalidTWAPPeriod      ParamsError = "InvalidTWAPPeriod()"` | [modules/spot/types/params.go](../../../modules/spot/types/params.go) |
| `go_exported` | `ErrParamsZeroAddress            ParamsError = "ZeroAddress()"` | [modules/spot/types/params.go](../../../modules/spot/types/params.go) |
| `go_exported` | `ErrParamsTokenAlreadyRegistered ParamsError = "TokenAlreadyRegistered()"` | [modules/spot/types/params.go](../../../modules/spot/types/params.go) |
| `go_exported` | `ErrParamsUnsupportedToken       ParamsError = "UnsupportedToken()"` | [modules/spot/types/params.go](../../../modules/spot/types/params.go) |
| `go_exported` | `TokenKindERC20` | [modules/spot/types/params.go](../../../modules/spot/types/params.go) |
| `go_exported` | `TokenKindNative` | [modules/spot/types/params.go](../../../modules/spot/types/params.go) |
| `go_exported` | `type TokenParams struct` | [modules/spot/types/params.go](../../../modules/spot/types/params.go) |
| `go_exported` | `type Params struct` | [modules/spot/types/params.go](../../../modules/spot/types/params.go) |
| `go_exported` | `func GovernanceEVMAddress() common.Address` | [modules/spot/types/params.go](../../../modules/spot/types/params.go) |
| `go_exported` | `func DefaultParams() Params` | [modules/spot/types/params.go](../../../modules/spot/types/params.go) |
| `go_exported` | `func OpeningTokens(paxPointer, sidPointer common.Address) []TokenParams` | [modules/spot/types/params.go](../../../modules/spot/types/params.go) |
| `go_exported` | `func (p Params) Validate() error` | [modules/spot/types/params.go](../../../modules/spot/types/params.go) |

## go-exported-precompiles-spot

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `type SpotKeepers interface` | [precompiles/spot/spot.go](../../../precompiles/spot/spot.go) |

## go-exported-precompiles-spot-abi

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `func PECOR() (abi.ABI, error) { return parse(pecorJSON) }` | [precompiles/spot/abi/abi.go](../../../precompiles/spot/abi/abi.go) |
| `go_exported` | `func Orders() (abi.ABI, error) { return parse(ordersJSON) }` | [precompiles/spot/abi/abi.go](../../../precompiles/spot/abi/abi.go) |
| `go_exported` | `func StopOrders() (abi.ABI, error) { return parse(stopOrdersJSON) }` | [precompiles/spot/abi/abi.go](../../../precompiles/spot/abi/abi.go) |
| `go_exported` | `func Quoter() (abi.ABI, error) { return parse(quoterJSON) }` | [precompiles/spot/abi/abi.go](../../../precompiles/spot/abi/abi.go) |
| `go_exported` | `func Vault() (abi.ABI, error) { return parse(vaultJSON) }` | [precompiles/spot/abi/abi.go](../../../precompiles/spot/abi/abi.go) |
| `go_exported` | `func Share() (abi.ABI, error) { return parse(shareJSON) }` | [precompiles/spot/abi/abi.go](../../../precompiles/spot/abi/abi.go) |

<a id="epoch"></a>

## epoch-errors

| Kind | Declaration | Source |
|---|---|---|
| source | — | [modules/epoch/types/errors.go](../../../modules/epoch/types/errors.go) |

## epoch-events

| Kind | Declaration | Source |
|---|---|---|
| source | — | [modules/epoch/types/events.go](../../../modules/epoch/types/events.go) |

## epoch-genesis

| Kind | Declaration | Source |
|---|---|---|
| source | — | [api/epoch/genesis.proto](../../../api/epoch/genesis.proto) |

<a id="epoch-hooks"></a>

## epoch-storage

| Kind | Declaration | Source |
|---|---|---|
| `storage_key` | `EpochKey = "epoch"` | [modules/epoch/keeper/epoch.go](../../../modules/epoch/keeper/epoch.go) |

## proto-epoch-epoch

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message Epoch {` | [api/epoch/epoch.proto](../../../api/epoch/epoch.proto) |

## proto-epoch-genesisstate

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message GenesisState {` | [api/epoch/genesis.proto](../../../api/epoch/genesis.proto) |

## proto-epoch-params

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message Params {` | [api/epoch/params.proto](../../../api/epoch/params.proto) |

## proto-epoch-queryepochrequest

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `QueryEpochRequest` | [api/epoch/query.proto](../../../api/epoch/query.proto) |

## proto-epoch-queryepochresponse

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message QueryEpochResponse {` | [api/epoch/query.proto](../../../api/epoch/query.proto) |

## proto-epoch-queryparamsrequest

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `QueryParamsRequest` | [api/epoch/query.proto](../../../api/epoch/query.proto) |

## proto-epoch-queryparamsresponse

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message QueryParamsResponse {` | [api/epoch/query.proto](../../../api/epoch/query.proto) |

## cli-epoch-query-epoch

| Kind | Declaration | Source |
|---|---|---|
| source | — | [modules/epoch/client/cli/query_epoch.go](../../../modules/epoch/client/cli/query_epoch.go) |

## cli-epoch-query-params

| Kind | Declaration | Source |
|---|---|---|
| source | — | [modules/epoch/client/cli/query_params.go](../../../modules/epoch/client/cli/query_params.go) |

## cli-epoch-query-root

| Kind | Declaration | Source |
|---|---|---|
| source | — | [modules/epoch/client/cli/query.go](../../../modules/epoch/client/cli/query.go) |

## cli-epoch-tx-root

| Kind | Declaration | Source |
|---|---|---|
| source | — | [modules/epoch/client/cli/tx.go](../../../modules/epoch/client/cli/tx.go) |

## go-exported-modules-epoch

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `type AppModuleBasic struct` | [modules/epoch/module.go](../../../modules/epoch/module.go) |
| `go_exported` | `func NewAppModule(` | [modules/epoch/module.go](../../../modules/epoch/module.go) |

## go-exported-modules-epoch-client-cli

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `func GetQueryCmd(_ string) *cobra.Command` | [modules/epoch/client/cli/query.go](../../../modules/epoch/client/cli/query.go) |
| `go_exported` | `func GetTxCmd() *cobra.Command` | [modules/epoch/client/cli/tx.go](../../../modules/epoch/client/cli/tx.go) |

## go-exported-modules-epoch-client-wasm

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `type EpochWasmQueryHandler struct` | [modules/epoch/client/wasm/query.go](../../../modules/epoch/client/wasm/query.go) |
| `go_exported` | `func NewEpochWasmQueryHandler(keeper *keeper.Keeper) *EpochWasmQueryHandler` | [modules/epoch/client/wasm/query.go](../../../modules/epoch/client/wasm/query.go) |
| `go_exported` | `func (handler EpochWasmQueryHandler) GetEpoch(ctx sdk.Context, req *types.QueryEpochRequest) (*types.QueryEpochResponse, error)` | [modules/epoch/client/wasm/query.go](../../../modules/epoch/client/wasm/query.go) |

## go-exported-modules-epoch-client-wasm-bindings

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `type PaxEpochQuery struct` | [modules/epoch/client/wasm/bindings/queries.go](../../../modules/epoch/client/wasm/bindings/queries.go) |

## go-exported-modules-epoch-keeper

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `func (k Keeper) BeginBlock(ctx sdk.Context)` | [modules/epoch/keeper/abci.go](../../../modules/epoch/keeper/abci.go) |
| `go_exported` | `func (k Keeper) SetEpoch(ctx sdk.Context, epoch types.Epoch)` | [modules/epoch/keeper/epoch.go](../../../modules/epoch/keeper/epoch.go) |
| `go_exported` | `func (k Keeper) GetEpoch(ctx sdk.Context) (epoch types.Epoch)` | [modules/epoch/keeper/epoch.go](../../../modules/epoch/keeper/epoch.go) |
| `go_exported` | `func (k Keeper) Epoch(c context.Context, _ *types.QueryEpochRequest) (*types.QueryEpochResponse, error)` | [modules/epoch/keeper/grpc_query_epoch.go](../../../modules/epoch/keeper/grpc_query_epoch.go) |
| `go_exported` | `Keeper struct {` | [modules/epoch/keeper/keeper.go](../../../modules/epoch/keeper/keeper.go) |
| `go_exported` | `func NewKeeper(` | [modules/epoch/keeper/keeper.go](../../../modules/epoch/keeper/keeper.go) |
| `go_exported` | `func (k *Keeper) SetHooks(eh types.EpochHooks) *Keeper` | [modules/epoch/keeper/keeper.go](../../../modules/epoch/keeper/keeper.go) |
| `go_exported` | `func (k Keeper) GetParams(_ sdk.Context) types.Params` | [modules/epoch/keeper/params.go](../../../modules/epoch/keeper/params.go) |
| `go_exported` | `func (k Keeper) SetParams(ctx sdk.Context, params types.Params)` | [modules/epoch/keeper/params.go](../../../modules/epoch/keeper/params.go) |

## go-exported-modules-epoch-simulation

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `func FindAccount(accs []simtypes.Account, address string) (simtypes.Account, bool)` | [modules/epoch/simulation/simap.go](../../../modules/epoch/simulation/simap.go) |

## go-exported-modules-epoch-types

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `func RegisterCodec(_ *codec.LegacyAmino) {}` | [modules/epoch/types/codec.go](../../../modules/epoch/types/codec.go) |
| `go_exported` | `func RegisterInterfaces(registry cdctypes.InterfaceRegistry)` | [modules/epoch/types/codec.go](../../../modules/epoch/types/codec.go) |
| `go_exported` | `func DefaultEpoch() Epoch` | [modules/epoch/types/epoch.go](../../../modules/epoch/types/epoch.go) |
| `go_exported` | `func (e *Epoch) Validate() error` | [modules/epoch/types/epoch.go](../../../modules/epoch/types/epoch.go) |
| `go_exported` | `ErrParsingPaxEpochQuery = sdkerrors.Register(ModuleName, 2, "Error parsing PaxEpochQuery")` | [modules/epoch/types/errors.go](../../../modules/epoch/types/errors.go) |
| `go_exported` | `ErrEncodingEpoch        = sdkerrors.Register(ModuleName, 4, "Error encoding epoch as JSON")` | [modules/epoch/types/errors.go](../../../modules/epoch/types/errors.go) |
| `go_exported` | `ErrUnknownPaxEpochQuery = sdkerrors.Register(ModuleName, 6, "Error unknown pax epoch query")` | [modules/epoch/types/errors.go](../../../modules/epoch/types/errors.go) |
| `go_exported` | `EventTypeNewEpoch = "new_epoch"` | [modules/epoch/types/events.go](../../../modules/epoch/types/events.go) |
| `go_exported` | `AttributeEpochNumber = "epoch_number"` | [modules/epoch/types/events.go](../../../modules/epoch/types/events.go) |
| `go_exported` | `AttributeEpochTime   = "epoch_time"` | [modules/epoch/types/events.go](../../../modules/epoch/types/events.go) |
| `go_exported` | `AttributeEpochHeight = "epoch_height"` | [modules/epoch/types/events.go](../../../modules/epoch/types/events.go) |
| `go_exported` | `type AccountKeeper interface` | [modules/epoch/types/expected_keepers.go](../../../modules/epoch/types/expected_keepers.go) |
| `go_exported` | `type BankKeeper interface` | [modules/epoch/types/expected_keepers.go](../../../modules/epoch/types/expected_keepers.go) |
| `go_exported` | `func DefaultGenesisTime() time.Time` | [modules/epoch/types/genesis.go](../../../modules/epoch/types/genesis.go) |
| `go_exported` | `func DefaultGenesis() *GenesisState` | [modules/epoch/types/genesis.go](../../../modules/epoch/types/genesis.go) |
| `go_exported` | `func (gs GenesisState) Validate() error` | [modules/epoch/types/genesis.go](../../../modules/epoch/types/genesis.go) |
| `go_exported` | `type EpochHooks interface` | [modules/epoch/types/hooks.go](../../../modules/epoch/types/hooks.go) |
| `go_exported` | `func NewMultiEpochHooks(hooks ...EpochHooks) MultiEpochHooks` | [modules/epoch/types/hooks.go](../../../modules/epoch/types/hooks.go) |
| `go_exported` | `func (h MultiEpochHooks) AfterEpochEnd(ctx sdk.Context, epoch Epoch)` | [modules/epoch/types/hooks.go](../../../modules/epoch/types/hooks.go) |
| `go_exported` | `func (h MultiEpochHooks) BeforeEpochStart(ctx sdk.Context, epoch Epoch)` | [modules/epoch/types/hooks.go](../../../modules/epoch/types/hooks.go) |
| `go_exported` | `ModuleName = "epoch"` | [modules/epoch/types/keys.go](../../../modules/epoch/types/keys.go) |
| `go_exported` | `StoreKey = ModuleName` | [modules/epoch/types/keys.go](../../../modules/epoch/types/keys.go) |
| `go_exported` | `RouterKey = ModuleName` | [modules/epoch/types/keys.go](../../../modules/epoch/types/keys.go) |
| `go_exported` | `QuerierRoute = ModuleName` | [modules/epoch/types/keys.go](../../../modules/epoch/types/keys.go) |
| `go_exported` | `MemStoreKey = "mem_epoch"` | [modules/epoch/types/keys.go](../../../modules/epoch/types/keys.go) |
| `go_exported` | `func ParamKeyTable() paramtypes.KeyTable` | [modules/epoch/types/params.go](../../../modules/epoch/types/params.go) |
| `go_exported` | `func NewParams() Params` | [modules/epoch/types/params.go](../../../modules/epoch/types/params.go) |
| `go_exported` | `func DefaultParams() Params` | [modules/epoch/types/params.go](../../../modules/epoch/types/params.go) |
| `go_exported` | `func (p Params) Validate() error` | [modules/epoch/types/params.go](../../../modules/epoch/types/params.go) |
| `go_exported` | `func (p Params) String() string` | [modules/epoch/types/params.go](../../../modules/epoch/types/params.go) |

<a id="mint"></a>

## mint-events

| Kind | Declaration | Source |
|---|---|---|
| source | — | [modules/mint/types/events.go](../../../modules/mint/types/events.go) |

## mint-params

| Kind | Declaration | Source |
|---|---|---|
| source | — | [api/mint/v1beta1/genesis.proto](../../../api/mint/v1beta1/genesis.proto) |
| source | — | [api/mint/v1beta1/mint.proto](../../../api/mint/v1beta1/mint.proto) |

<a id="mint-release"></a>

<a id="mint-rest"></a>

## mint-storage

| Kind | Declaration | Source |
|---|---|---|
| `storage_key` | `MinterKey = []byte{0x00}` | [modules/mint/types/keys.go](../../../modules/mint/types/keys.go) |

## proto-mint-genesisstate

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message GenesisState {` | [api/mint/v1beta1/genesis.proto](../../../api/mint/v1beta1/genesis.proto) |

## proto-mint-minter

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message Minter {` | [api/mint/v1beta1/mint.proto](../../../api/mint/v1beta1/mint.proto) |

## proto-mint-params

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message Params {` | [api/mint/v1beta1/mint.proto](../../../api/mint/v1beta1/mint.proto) |

## proto-mint-queryminterrequest

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `QueryMinterRequest` | [api/mint/v1beta1/query.proto](../../../api/mint/v1beta1/query.proto) |

## proto-mint-queryminterresponse

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message QueryMinterResponse {` | [api/mint/v1beta1/query.proto](../../../api/mint/v1beta1/query.proto) |

## proto-mint-queryparamsrequest

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `QueryParamsRequest` | [api/mint/v1beta1/query.proto](../../../api/mint/v1beta1/query.proto) |

## proto-mint-queryparamsresponse

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message QueryParamsResponse {` | [api/mint/v1beta1/query.proto](../../../api/mint/v1beta1/query.proto) |

## proto-mint-scheduledtokenrelease

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message ScheduledTokenRelease {` | [api/mint/v1beta1/mint.proto](../../../api/mint/v1beta1/mint.proto) |

## proto-mint-updateminterproposal

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message UpdateMinterProposal {` | [api/mint/v1beta1/gov.proto](../../../api/mint/v1beta1/gov.proto) |

## proto-mint-version2minter

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message Version2Minter {` | [api/mint/v1beta1/mint.proto](../../../api/mint/v1beta1/mint.proto) |

## proto-mint-version2params

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message Version2Params {` | [api/mint/v1beta1/mint.proto](../../../api/mint/v1beta1/mint.proto) |

## proto-mint-version2scheduledtokenrelease

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message Version2ScheduledTokenRelease {` | [api/mint/v1beta1/mint.proto](../../../api/mint/v1beta1/mint.proto) |

## cli-mint-gov-update-minter

| Kind | Declaration | Source |
|---|---|---|
| source | — | [modules/mint/client/cli/tx.go](../../../modules/mint/client/cli/tx.go) |

## cli-mint-query-minter

| Kind | Declaration | Source |
|---|---|---|
| source | — | [modules/mint/client/cli/query.go](../../../modules/mint/client/cli/query.go) |

## cli-mint-query-params

| Kind | Declaration | Source |
|---|---|---|
| source | — | [modules/mint/client/cli/query.go](../../../modules/mint/client/cli/query.go) |

## cli-mint-query-root

| Kind | Declaration | Source |
|---|---|---|
| source | — | [modules/mint/client/cli/query.go](../../../modules/mint/client/cli/query.go) |

## cli-mint-tx-root

| Kind | Declaration | Source |
|---|---|---|
| source | — | [modules/mint/client/cli/tx.go](../../../modules/mint/client/cli/tx.go) |

## go-exported-modules-mint

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `type AppModuleBasic struct` | [modules/mint/module.go](../../../modules/mint/module.go) |
| `go_exported` | `func NewAppModule(cdc codec.Codec, keeper keeper.Keeper, ak types.AccountKeeper) AppModule` | [modules/mint/module.go](../../../modules/mint/module.go) |
| `go_exported` | `func NewProposalHandler(k keeper.Keeper) govtypes.Handler` | [modules/mint/module.go](../../../modules/mint/module.go) |

## go-exported-modules-mint-client-cli

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `func GetQueryCmd() *cobra.Command` | [modules/mint/client/cli/query.go](../../../modules/mint/client/cli/query.go) |
| `go_exported` | `var UpdateMinterHandler = govclient.NewProposalHandler(MsgUpdateMinterProposalCmd, mintrest.UpdateResourceDependencyProposalRESTHandler)` | [modules/mint/client/cli/tx.go](../../../modules/mint/client/cli/tx.go) |

## go-exported-modules-mint-client-rest

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `RegisterRoutes` | [modules/mint/client/rest/rest.go](../../../modules/mint/client/rest/rest.go) |
| `go_exported` | `func UpdateResourceDependencyProposalRESTHandler(clientCtx client.Context) govrest.ProposalRESTHandler` | [modules/mint/client/rest/rest.go](../../../modules/mint/client/rest/rest.go) |

## go-exported-modules-mint-keeper

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `func (k Keeper) InitGenesis(ctx sdk.Context, data *types.GenesisState)` | [modules/mint/keeper/genesis.go](../../../modules/mint/keeper/genesis.go) |
| `go_exported` | `func (k Keeper) ExportGenesis(ctx sdk.Context) *types.GenesisState` | [modules/mint/keeper/genesis.go](../../../modules/mint/keeper/genesis.go) |
| `go_exported` | `func NewQuerier(k Keeper) Querier` | [modules/mint/keeper/grpc_query.go](../../../modules/mint/keeper/grpc_query.go) |
| `go_exported` | `func (k Keeper) Hooks() Hooks` | [modules/mint/keeper/hooks.go](../../../modules/mint/keeper/hooks.go) |
| `go_exported` | `type Keeper struct` | [modules/mint/keeper/keeper.go](../../../modules/mint/keeper/keeper.go) |
| `go_exported` | `func NewKeeper(` | [modules/mint/keeper/keeper.go](../../../modules/mint/keeper/keeper.go) |
| `go_exported` | `func (k *Keeper) SetHooks(h types.MintHooks) *Keeper` | [modules/mint/keeper/keeper.go](../../../modules/mint/keeper/keeper.go) |
| `go_exported` | `func (k Keeper) SetMinter(ctx sdk.Context, minter types.Minter)` | [modules/mint/keeper/keeper.go](../../../modules/mint/keeper/keeper.go) |
| `go_exported` | `func (k Keeper) GetParams(ctx sdk.Context) (params types.Params)` | [modules/mint/keeper/keeper.go](../../../modules/mint/keeper/keeper.go) |
| `go_exported` | `func NewMigrator(keeper Keeper) Migrator` | [modules/mint/keeper/migrations.go](../../../modules/mint/keeper/migrations.go) |

## go-exported-modules-mint-simulation

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `func NewDecodeStore(cdc codec.Codec) func(kvA, kvB kv.Pair) string` | [modules/mint/simulation/decoder.go](../../../modules/mint/simulation/decoder.go) |
| `go_exported` | `func RandomizedGenState(simState *module.SimulationState)` | [modules/mint/simulation/genesis.go](../../../modules/mint/simulation/genesis.go) |

## go-exported-modules-mint-types

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `func RegisterCodec(cdc *codec.LegacyAmino)` | [modules/mint/types/codec.go](../../../modules/mint/types/codec.go) |
| `go_exported` | `func RegisterInterfaces(registry cdctypes.InterfaceRegistry)` | [modules/mint/types/codec.go](../../../modules/mint/types/codec.go) |
| `go_exported` | `type StakingKeeper interface` | [modules/mint/types/expected_keepers.go](../../../modules/mint/types/expected_keepers.go) |
| `go_exported` | `type AccountKeeper interface` | [modules/mint/types/expected_keepers.go](../../../modules/mint/types/expected_keepers.go) |
| `go_exported` | `type BankKeeper interface` | [modules/mint/types/expected_keepers.go](../../../modules/mint/types/expected_keepers.go) |
| `go_exported` | `type EpochKeeper interface` | [modules/mint/types/expected_keepers.go](../../../modules/mint/types/expected_keepers.go) |
| `go_exported` | `func NewGenesisState(minter Minter, params Params) *GenesisState` | [modules/mint/types/genesis.go](../../../modules/mint/types/genesis.go) |
| `go_exported` | `func DefaultGenesisState() *GenesisState` | [modules/mint/types/genesis.go](../../../modules/mint/types/genesis.go) |
| `go_exported` | `func ValidateGenesis(data GenesisState) error` | [modules/mint/types/genesis.go](../../../modules/mint/types/genesis.go) |
| `go_exported` | `func (p *UpdateMinterProposal) ValidateBasic() error` | [modules/mint/types/gov.go](../../../modules/mint/types/gov.go) |
| `go_exported` | `func (p UpdateMinterProposal) String() string` | [modules/mint/types/gov.go](../../../modules/mint/types/gov.go) |
| `go_exported` | `func NewUpdateMinterProposalHandler(title, description string, minter Minter) *UpdateMinterProposal` | [modules/mint/types/gov.go](../../../modules/mint/types/gov.go) |
| `go_exported` | `type MintHooks interface` | [modules/mint/types/hooks.go](../../../modules/mint/types/hooks.go) |
| `go_exported` | `var MinterKey = []byte{0x00}` | [modules/mint/types/keys.go](../../../modules/mint/types/keys.go) |
| `go_exported` | `ModuleName = "mint"` | [modules/mint/types/keys.go](../../../modules/mint/types/keys.go) |
| `go_exported` | `StoreKey = ModuleName` | [modules/mint/types/keys.go](../../../modules/mint/types/keys.go) |
| `go_exported` | `RouterKey = ModuleName` | [modules/mint/types/keys.go](../../../modules/mint/types/keys.go) |
| `go_exported` | `MemStoreKey = "mem_mint"` | [modules/mint/types/keys.go](../../../modules/mint/types/keys.go) |
| `go_exported` | `QuerierRoute = StoreKey` | [modules/mint/types/keys.go](../../../modules/mint/types/keys.go) |
| `go_exported` | `QueryParameters = "parameters"` | [modules/mint/types/keys.go](../../../modules/mint/types/keys.go) |
| `go_exported` | `QueryMinter     = "minter"` | [modules/mint/types/keys.go](../../../modules/mint/types/keys.go) |
| `go_exported` | `TokenReleaseDateFormat = "2006-01-02" //nolint:gosec` | [modules/mint/types/keys.go](../../../modules/mint/types/keys.go) |
| `go_exported` | `func NewMinter(` | [modules/mint/types/minter.go](../../../modules/mint/types/minter.go) |
| `go_exported` | `func InitialMinter() Minter` | [modules/mint/types/minter.go](../../../modules/mint/types/minter.go) |
| `go_exported` | `func DefaultInitialMinter() Minter` | [modules/mint/types/minter.go](../../../modules/mint/types/minter.go) |
| `go_exported` | `func ValidateMinter(minter Minter) error` | [modules/mint/types/minter.go](../../../modules/mint/types/minter.go) |
| `go_exported` | `func (m *Minter) GetEndDateTime() time.Time` | [modules/mint/types/minter.go](../../../modules/mint/types/minter.go) |
| `go_exported` | `func (m *Minter) OngoingRelease() bool` | [modules/mint/types/minter.go](../../../modules/mint/types/minter.go) |
| `go_exported` | `KeyMintDenom            = []byte("MintDenom")` | [modules/mint/types/params.go](../../../modules/mint/types/params.go) |
| `go_exported` | `KeyTokenReleaseSchedule = []byte("TokenReleaseSchedule")` | [modules/mint/types/params.go](../../../modules/mint/types/params.go) |
| `go_exported` | `func ParamKeyTable() paramtypes.KeyTable` | [modules/mint/types/params.go](../../../modules/mint/types/params.go) |
| `go_exported` | `func NewParams(` | [modules/mint/types/params.go](../../../modules/mint/types/params.go) |
| `go_exported` | `func DefaultParams() Params` | [modules/mint/types/params.go](../../../modules/mint/types/params.go) |
| `go_exported` | `func (p Params) String() string` | [modules/mint/types/params.go](../../../modules/mint/types/params.go) |
| `go_exported` | `func (p Version2Params) String() string` | [modules/mint/types/params.go](../../../modules/mint/types/params.go) |
| `go_exported` | `func SortTokenReleaseCalendar(tokenReleaseSchedule []ScheduledTokenRelease) []ScheduledTokenRelease` | [modules/mint/types/params.go](../../../modules/mint/types/params.go) |

<a id="layerx-exchange"></a>

## exchange-legacy-0x1015

| Kind | Declaration | Source |
|---|---|---|
| `precompile_method` | `function cancelOrder(bytes32) nonpayable returns (bytes32)` | [precompiles/layerxexchange/abi.json](../../../precompiles/layerxexchange/abi.json) |
| `precompile_method` | `function depositMargin(bytes32) payable returns (bytes32,bytes32)` | [precompiles/layerxexchange/abi.json](../../../precompiles/layerxexchange/abi.json) |
| `precompile_method` | `function depositMarginToken(address,uint256,bytes32) nonpayable returns (bytes32,bytes32)` | [precompiles/layerxexchange/abi.json](../../../precompiles/layerxexchange/abi.json) |
| `precompile_event` | `event MarginDeposited(bytes32,bytes32,address,bytes32,uint256,bytes32,uint64)` | [precompiles/layerxexchange/abi.json](../../../precompiles/layerxexchange/abi.json) |
| `precompile_event` | `event MarginWithdrawalRequested(bytes32,bytes32,address,bytes32,uint256,uint64)` | [precompiles/layerxexchange/abi.json](../../../precompiles/layerxexchange/abi.json) |
| `precompile_event` | `event OrderCancelRequested(bytes32,bytes32,address,uint64)` | [precompiles/layerxexchange/abi.json](../../../precompiles/layerxexchange/abi.json) |
| `precompile_event` | `event OrderPlaced(bytes32,bytes32,address,uint8,uint256,uint256,uint8,uint64)` | [precompiles/layerxexchange/abi.json](../../../precompiles/layerxexchange/abi.json) |
| `precompile_event` | `event SettlementRequested(bytes32,bytes32,address,uint64)` | [precompiles/layerxexchange/abi.json](../../../precompiles/layerxexchange/abi.json) |
| `precompile_method` | `function getIntent(bytes32) view returns ((bytes32,uint8,uint8,address,uint64,uint64,bytes32,bytes32,string,uint256,bytes32,bytes32,uint8,uint256,uint256,uint8,bytes32,bytes32))` | [precompiles/layerxexchange/abi.json](../../../precompiles/layerxexchange/abi.json) |
| `precompile_method` | `function getMargin(bytes32,bytes32,uint64,bytes) view returns ((uint64,bytes32,bytes32,bytes32,uint256,bool))` | [precompiles/layerxexchange/abi.json](../../../precompiles/layerxexchange/abi.json) |
| `precompile_method` | `function getMarket(bytes32,uint64,bytes) view returns ((uint64,bytes32,bytes,bytes))` | [precompiles/layerxexchange/abi.json](../../../precompiles/layerxexchange/abi.json) |
| `precompile_method` | `function getOrder(bytes32,bytes32,uint64,bytes) view returns ((uint64,bytes32,bytes,bytes))` | [precompiles/layerxexchange/abi.json](../../../precompiles/layerxexchange/abi.json) |
| `precompile_method` | `function getPosition(bytes32,bytes32,uint64,bytes) view returns ((uint64,bytes32,bytes,bytes))` | [precompiles/layerxexchange/abi.json](../../../precompiles/layerxexchange/abi.json) |
| `precompile_method` | `function intentNonce(address) view returns (uint64)` | [precompiles/layerxexchange/abi.json](../../../precompiles/layerxexchange/abi.json) |
| `precompile_method` | `function placeOrder(bytes32,uint8,uint256,uint256,uint8) nonpayable returns (bytes32)` | [precompiles/layerxexchange/abi.json](../../../precompiles/layerxexchange/abi.json) |
| `precompile_method` | `function requestSettlement(bytes32) nonpayable returns (bytes32)` | [precompiles/layerxexchange/abi.json](../../../precompiles/layerxexchange/abi.json) |
| `precompile_method` | `function withdrawMargin(bytes32,bytes32,uint256) nonpayable returns (bytes32)` | [precompiles/layerxexchange/abi.json](../../../precompiles/layerxexchange/abi.json) |

## layerxexchange-errors

| Kind | Declaration | Source |
|---|---|---|
| source | — | [modules/layerxexchange/types/errors.go](../../../modules/layerxexchange/types/errors.go) |

## layerxexchange-events

| Kind | Declaration | Source |
|---|---|---|
| source | — | [modules/layerxexchange/keeper/intents.go](../../../modules/layerxexchange/keeper/intents.go) |

## layerxexchange-genesis

| Kind | Declaration | Source |
|---|---|---|
| source | — | [api/layerxexchange/genesis.proto](../../../api/layerxexchange/genesis.proto) |
| source | — | [api/layerxexchange/params.proto](../../../api/layerxexchange/params.proto) |

## layerxexchange-retirement

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `func (k *Keeper) Retired(ctx sdk.Context) (bool, error)` | [modules/layerxexchange/keeper/keeper.go](../../../modules/layerxexchange/keeper/keeper.go) |
| `go_exported` | `func (k *Keeper) SetUpgradeKeeper(upgrade types.UpgradeKeeper)` | [modules/layerxexchange/keeper/keeper.go](../../../modules/layerxexchange/keeper/keeper.go) |
| `go_exported` | `type UpgradeKeeper interface {` | [modules/layerxexchange/types/expected_keepers.go](../../../modules/layerxexchange/types/expected_keepers.go) |
| `go_exported` | `func KernelCodeText(code lxphost.Code) string` | [precompiles/layerxexchange/errors.go](../../../precompiles/layerxexchange/errors.go) |
| `go_exported` | `func RevertError(err error) error` | [precompiles/layerxexchange/errors.go](../../../precompiles/layerxexchange/errors.go) |
| source | — | [modules/layerxexchange/types/errors.go](../../../modules/layerxexchange/types/errors.go) |

## layerxexchange-storage

| Kind | Declaration | Source |
|---|---|---|
| `storage_key` | `ParamsKey = []byte{0x01}` | [modules/layerxexchange/types/keys.go](../../../modules/layerxexchange/types/keys.go) |
| `storage_key` | `IntentCountKey = []byte{0x02}` | [modules/layerxexchange/types/keys.go](../../../modules/layerxexchange/types/keys.go) |
| `storage_key` | `IntentPrefix = []byte{0x10}` | [modules/layerxexchange/types/keys.go](../../../modules/layerxexchange/types/keys.go) |
| `storage_key` | `OwnerNoncePrefix = []byte{0x20}` | [modules/layerxexchange/types/keys.go](../../../modules/layerxexchange/types/keys.go) |

## msg-layerxexchange-setmarket

| Kind | Declaration | Source |
|---|---|---|
| source | — | [api/layerxexchange/tx.proto](../../../api/layerxexchange/tx.proto) |

## msg-layerxexchange-updateparams

| Kind | Declaration | Source |
|---|---|---|
| source | — | [api/layerxexchange/tx.proto](../../../api/layerxexchange/tx.proto) |

## proto-layerxexchange-eventdeleveraged

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message EventDeleveraged {` | [api/layerxexchange/events.proto](../../../api/layerxexchange/events.proto) |

## proto-layerxexchange-eventfundingsettled

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message EventFundingSettled {` | [api/layerxexchange/events.proto](../../../api/layerxexchange/events.proto) |

## proto-layerxexchange-eventliquidated

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message EventLiquidated {` | [api/layerxexchange/events.proto](../../../api/layerxexchange/events.proto) |

## proto-layerxexchange-eventmargindeposited

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message EventMarginDeposited {` | [api/layerxexchange/events.proto](../../../api/layerxexchange/events.proto) |

## proto-layerxexchange-eventmarginwithdrawalrequested

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message EventMarginWithdrawalRequested {` | [api/layerxexchange/events.proto](../../../api/layerxexchange/events.proto) |

## proto-layerxexchange-eventmarginwithdrawn

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message EventMarginWithdrawn {` | [api/layerxexchange/events.proto](../../../api/layerxexchange/events.proto) |

## proto-layerxexchange-eventoracleaccepted

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message EventOracleAccepted {` | [api/layerxexchange/events.proto](../../../api/layerxexchange/events.proto) |

## proto-layerxexchange-eventordercancelled

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message EventOrderCancelled {` | [api/layerxexchange/events.proto](../../../api/layerxexchange/events.proto) |

## proto-layerxexchange-eventordercancelrequested

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message EventOrderCancelRequested {` | [api/layerxexchange/events.proto](../../../api/layerxexchange/events.proto) |

## proto-layerxexchange-eventorderplaced

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message EventOrderPlaced {` | [api/layerxexchange/events.proto](../../../api/layerxexchange/events.proto) |

## proto-layerxexchange-eventperpsmarketcreated

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message EventPerpsMarketCreated {` | [api/layerxexchange/events.proto](../../../api/layerxexchange/events.proto) |

## proto-layerxexchange-eventperpsmarkethalted

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message EventPerpsMarketHalted {` | [api/layerxexchange/events.proto](../../../api/layerxexchange/events.proto) |

## proto-layerxexchange-eventpositionclosed

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message EventPositionClosed {` | [api/layerxexchange/events.proto](../../../api/layerxexchange/events.proto) |

## proto-layerxexchange-eventpositionincreased

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message EventPositionIncreased {` | [api/layerxexchange/events.proto](../../../api/layerxexchange/events.proto) |

## proto-layerxexchange-eventpositionopened

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message EventPositionOpened {` | [api/layerxexchange/events.proto](../../../api/layerxexchange/events.proto) |

## proto-layerxexchange-eventsettlementrequested

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message EventSettlementRequested {` | [api/layerxexchange/events.proto](../../../api/layerxexchange/events.proto) |

## proto-layerxexchange-eventspotmarketcreated

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message EventSpotMarketCreated {` | [api/layerxexchange/events.proto](../../../api/layerxexchange/events.proto) |

## proto-layerxexchange-eventspotmarkethalted

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message EventSpotMarketHalted {` | [api/layerxexchange/events.proto](../../../api/layerxexchange/events.proto) |

## proto-layerxexchange-eventspotmarketresumed

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message EventSpotMarketResumed {` | [api/layerxexchange/events.proto](../../../api/layerxexchange/events.proto) |

## proto-layerxexchange-eventspotordercancelled

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message EventSpotOrderCancelled {` | [api/layerxexchange/events.proto](../../../api/layerxexchange/events.proto) |

## proto-layerxexchange-eventspotorderplaced

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message EventSpotOrderPlaced {` | [api/layerxexchange/events.proto](../../../api/layerxexchange/events.proto) |

## proto-layerxexchange-genesisstate

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message GenesisState {` | [api/layerxexchange/genesis.proto](../../../api/layerxexchange/genesis.proto) |

## proto-layerxexchange-intent

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message Intent {` | [api/layerxexchange/state.proto](../../../api/layerxexchange/state.proto) |

## proto-layerxexchange-intentkind

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `enum IntentKind {` | [api/layerxexchange/state.proto](../../../api/layerxexchange/state.proto) |

## proto-layerxexchange-intentstatus

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `enum IntentStatus {` | [api/layerxexchange/state.proto](../../../api/layerxexchange/state.proto) |

## proto-layerxexchange-market

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message Market {` | [api/layerxexchange/params.proto](../../../api/layerxexchange/params.proto) |

## proto-layerxexchange-msgsetmarket

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message MsgSetMarket {` | [api/layerxexchange/tx.proto](../../../api/layerxexchange/tx.proto) |

## proto-layerxexchange-msgsetmarketresponse

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `MsgSetMarketResponse` | [api/layerxexchange/tx.proto](../../../api/layerxexchange/tx.proto) |

## proto-layerxexchange-msgupdateparams

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message MsgUpdateParams {` | [api/layerxexchange/tx.proto](../../../api/layerxexchange/tx.proto) |

## proto-layerxexchange-msgupdateparamsresponse

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `MsgUpdateParamsResponse` | [api/layerxexchange/tx.proto](../../../api/layerxexchange/tx.proto) |

## proto-layerxexchange-ownernonce

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message OwnerNonce {` | [api/layerxexchange/state.proto](../../../api/layerxexchange/state.proto) |

## proto-layerxexchange-params

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message Params {` | [api/layerxexchange/params.proto](../../../api/layerxexchange/params.proto) |

## go-exported-modules-layerxexchange

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `type AppModuleBasic struct{}` | [modules/layerxexchange/module.go](../../../modules/layerxexchange/module.go) |
| `go_exported` | `func NewAppModule(k *keeper.Keeper) AppModule` | [modules/layerxexchange/module.go](../../../modules/layerxexchange/module.go) |

## go-exported-modules-layerxexchange-keeper

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `func (k *Keeper) InitGenesis(ctx sdk.Context, gs types.GenesisState)` | [modules/layerxexchange/keeper/genesis.go](../../../modules/layerxexchange/keeper/genesis.go) |
| `go_exported` | `func (k *Keeper) ExportGenesis(ctx sdk.Context) *types.GenesisState` | [modules/layerxexchange/keeper/genesis.go](../../../modules/layerxexchange/keeper/genesis.go) |
| `go_exported` | `func (k *Keeper) RecordDepositMarginIntent(ctx sdk.Context, owner common.Address, ownerAccount sdk.AccAddress,` | [modules/layerxexchange/keeper/intents.go](../../../modules/layerxexchange/keeper/intents.go) |
| `go_exported` | `func (k *Keeper) RecordWithdrawMarginIntent(ctx sdk.Context, owner common.Address, account, assetID [32]byte,` | [modules/layerxexchange/keeper/intents.go](../../../modules/layerxexchange/keeper/intents.go) |
| `go_exported` | `func (k *Keeper) RecordPlaceOrderIntent(ctx sdk.Context, owner common.Address, marketID [32]byte, side uint8,` | [modules/layerxexchange/keeper/intents.go](../../../modules/layerxexchange/keeper/intents.go) |
| `go_exported` | `func (k *Keeper) RecordCancelOrderIntent(ctx sdk.Context, owner common.Address, orderID [32]byte) (types.Intent, error)` | [modules/layerxexchange/keeper/intents.go](../../../modules/layerxexchange/keeper/intents.go) |
| `go_exported` | `func (k *Keeper) RequestSettlement(ctx sdk.Context, owner common.Address, positionID [32]byte) (types.Intent, error)` | [modules/layerxexchange/keeper/intents.go](../../../modules/layerxexchange/keeper/intents.go) |
| `go_exported` | `type Keeper struct` | [modules/layerxexchange/keeper/keeper.go](../../../modules/layerxexchange/keeper/keeper.go) |
| `go_exported` | `func NewKeeper(cdc codec.BinaryCodec, storeKey sdk.StoreKey, custody types.CustodyKeeper, evmKeeper types.EVMKeeper) *Keeper` | [modules/layerxexchange/keeper/keeper.go](../../../modules/layerxexchange/keeper/keeper.go) |
| `go_exported` | `func (k *Keeper) GetParams(ctx sdk.Context) types.Params` | [modules/layerxexchange/keeper/keeper.go](../../../modules/layerxexchange/keeper/keeper.go) |
| `go_exported` | `func (k *Keeper) Custody() types.CustodyKeeper { return k.custody }` | [modules/layerxexchange/keeper/keeper.go](../../../modules/layerxexchange/keeper/keeper.go) |
| `go_exported` | `func (k *Keeper) GetOwnerNonce(ctx sdk.Context, owner common.Address) uint64` | [modules/layerxexchange/keeper/keeper.go](../../../modules/layerxexchange/keeper/keeper.go) |
| `go_exported` | `func (k *Keeper) GetIntent(ctx sdk.Context, intentID [32]byte) (types.Intent, bool)` | [modules/layerxexchange/keeper/keeper.go](../../../modules/layerxexchange/keeper/keeper.go) |
| `go_exported` | `func NewMsgServerImpl(k *Keeper) types.MsgServer { return msgServer{k} }` | [modules/layerxexchange/keeper/msg_server.go](../../../modules/layerxexchange/keeper/msg_server.go) |
| `go_exported` | `type ProvenState struct` | [modules/layerxexchange/keeper/proofs.go](../../../modules/layerxexchange/keeper/proofs.go) |
| `go_exported` | `func (k *Keeper) ProveMarket(ctx sdk.Context, marketID [32]byte, batchNumber uint64, witness []byte) (ProvenState, error)` | [modules/layerxexchange/keeper/proofs.go](../../../modules/layerxexchange/keeper/proofs.go) |
| `go_exported` | `func (k *Keeper) ProveMargin(ctx sdk.Context, account, assetID [32]byte, batchNumber uint64, witness []byte) (ProvenMargin, error)` | [modules/layerxexchange/keeper/proofs.go](../../../modules/layerxexchange/keeper/proofs.go) |

## go-exported-modules-layerxexchange-types

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `func RegisterCodec(cdc *codec.LegacyAmino)` | [modules/layerxexchange/types/codec.go](../../../modules/layerxexchange/types/codec.go) |
| `go_exported` | `func RegisterInterfaces(registry cdctypes.InterfaceRegistry)` | [modules/layerxexchange/types/codec.go](../../../modules/layerxexchange/types/codec.go) |
| `go_exported` | `ErrInvalidParams   = sdkerrors.Register(ModuleName, 2, "invalid params")` | [modules/layerxexchange/types/errors.go](../../../modules/layerxexchange/types/errors.go) |
| `go_exported` | `ErrInvalidMarket   = sdkerrors.Register(ModuleName, 3, "invalid market")` | [modules/layerxexchange/types/errors.go](../../../modules/layerxexchange/types/errors.go) |
| `go_exported` | `ErrUnknownMarket   = sdkerrors.Register(ModuleName, 4, "market is not listed or is disabled")` | [modules/layerxexchange/types/errors.go](../../../modules/layerxexchange/types/errors.go) |
| `go_exported` | `ErrNotMarginAsset  = sdkerrors.Register(ModuleName, 5, "asset is not the margin asset of an enabled market")` | [modules/layerxexchange/types/errors.go](../../../modules/layerxexchange/types/errors.go) |
| `go_exported` | `ErrInvalidIntent   = sdkerrors.Register(ModuleName, 6, "invalid exchange intent")` | [modules/layerxexchange/types/errors.go](../../../modules/layerxexchange/types/errors.go) |
| `go_exported` | `ErrIntentExists    = sdkerrors.Register(ModuleName, 7, "intent already recorded")` | [modules/layerxexchange/types/errors.go](../../../modules/layerxexchange/types/errors.go) |
| `go_exported` | `ErrNotFinalized    = sdkerrors.Register(ModuleName, 8, "batch is not a finalized checkpoint")` | [modules/layerxexchange/types/errors.go](../../../modules/layerxexchange/types/errors.go) |
| `go_exported` | `ErrInvalidProof    = sdkerrors.Register(ModuleName, 9, "LayerX state proof refused")` | [modules/layerxexchange/types/errors.go](../../../modules/layerxexchange/types/errors.go) |
| `go_exported` | `ErrStateMismatch   = sdkerrors.Register(ModuleName, 10, "state proof does not prove the requested entry")` | [modules/layerxexchange/types/errors.go](../../../modules/layerxexchange/types/errors.go) |
| `go_exported` | `ErrInvalidGenesis  = sdkerrors.Register(ModuleName, 11, "invalid genesis")` | [modules/layerxexchange/types/errors.go](../../../modules/layerxexchange/types/errors.go) |
| `go_exported` | `ErrKeeperMissing   = sdkerrors.Register(ModuleName, 12, "layerxexchange keeper is not wired")` | [modules/layerxexchange/types/errors.go](../../../modules/layerxexchange/types/errors.go) |
| `go_exported` | `ErrEvidenceTooLong = sdkerrors.Register(ModuleName, 13, "state proof exceeds the exchange bound")` | [modules/layerxexchange/types/errors.go](../../../modules/layerxexchange/types/errors.go) |
| `go_exported` | `func ErrorCode(err error) string` | [modules/layerxexchange/types/errors.go](../../../modules/layerxexchange/types/errors.go) |
| `go_exported` | `type CustodyKeeper interface` | [modules/layerxexchange/types/expected_keepers.go](../../../modules/layerxexchange/types/expected_keepers.go) |
| `go_exported` | `type EVMKeeper interface` | [modules/layerxexchange/types/expected_keepers.go](../../../modules/layerxexchange/types/expected_keepers.go) |
| `go_exported` | `func DefaultGenesis() *GenesisState` | [modules/layerxexchange/types/genesis.go](../../../modules/layerxexchange/types/genesis.go) |
| `go_exported` | `func (i Intent) Validate() error` | [modules/layerxexchange/types/genesis.go](../../../modules/layerxexchange/types/genesis.go) |
| `go_exported` | `func (gs GenesisState) Validate() error` | [modules/layerxexchange/types/genesis.go](../../../modules/layerxexchange/types/genesis.go) |
| `go_exported` | `ExchangeAddress = "0x0000000000000000000000000000000000001015"` | [modules/layerxexchange/types/ids.go](../../../modules/layerxexchange/types/ids.go) |
| `go_exported` | `func IntentID(chainID *big.Int, owner common.Address, kind IntentKind, nonce uint64) [32]byte` | [modules/layerxexchange/types/ids.go](../../../modules/layerxexchange/types/ids.go) |
| `go_exported` | `ModuleName = "layerxexchange"` | [modules/layerxexchange/types/keys.go](../../../modules/layerxexchange/types/keys.go) |
| `go_exported` | `StoreKey = ModuleName` | [modules/layerxexchange/types/keys.go](../../../modules/layerxexchange/types/keys.go) |
| `go_exported` | `QuerierRoute = ModuleName` | [modules/layerxexchange/types/keys.go](../../../modules/layerxexchange/types/keys.go) |
| `go_exported` | `ParamsKey        = []byte{0x01}` | [modules/layerxexchange/types/keys.go](../../../modules/layerxexchange/types/keys.go) |
| `go_exported` | `IntentCountKey   = []byte{0x02}` | [modules/layerxexchange/types/keys.go](../../../modules/layerxexchange/types/keys.go) |
| `go_exported` | `IntentPrefix     = []byte{0x10}` | [modules/layerxexchange/types/keys.go](../../../modules/layerxexchange/types/keys.go) |
| `go_exported` | `OwnerNoncePrefix = []byte{0x20}` | [modules/layerxexchange/types/keys.go](../../../modules/layerxexchange/types/keys.go) |
| `go_exported` | `func IntentKey(intentID [32]byte) []byte  { return join(IntentPrefix, intentID[:]) }` | [modules/layerxexchange/types/keys.go](../../../modules/layerxexchange/types/keys.go) |
| `go_exported` | `func OwnerNonceKey(owner [20]byte) []byte { return join(OwnerNoncePrefix, owner[:]) }` | [modules/layerxexchange/types/keys.go](../../../modules/layerxexchange/types/keys.go) |
| `go_exported` | `func (m MsgUpdateParams) Type() string { return TypeMsgUpdateParams }` | [modules/layerxexchange/types/msgs.go](../../../modules/layerxexchange/types/msgs.go) |
| `go_exported` | `func (m MsgUpdateParams) ValidateBasic() error` | [modules/layerxexchange/types/msgs.go](../../../modules/layerxexchange/types/msgs.go) |
| `go_exported` | `func (m MsgSetMarket) Type() string { return TypeMsgSetMarket }` | [modules/layerxexchange/types/msgs.go](../../../modules/layerxexchange/types/msgs.go) |
| `go_exported` | `func (m MsgSetMarket) ValidateBasic() error` | [modules/layerxexchange/types/msgs.go](../../../modules/layerxexchange/types/msgs.go) |
| `go_exported` | `func DefaultParams() Params { return Params{Paused: false, TimeInForce: false} }` | [modules/layerxexchange/types/params.go](../../../modules/layerxexchange/types/params.go) |
| `go_exported` | `func (m Market) Validate() error` | [modules/layerxexchange/types/params.go](../../../modules/layerxexchange/types/params.go) |
| `go_exported` | `func (p Params) Validate() error` | [modules/layerxexchange/types/params.go](../../../modules/layerxexchange/types/params.go) |
| `go_exported` | `func (p Params) Market(marketID string) (Market, bool)` | [modules/layerxexchange/types/params.go](../../../modules/layerxexchange/types/params.go) |
| `go_exported` | `func (p Params) MarginAsset(assetID string) bool` | [modules/layerxexchange/types/params.go](../../../modules/layerxexchange/types/params.go) |
| `go_exported` | `PerpsModuleID   uint16 = 6` | [modules/layerxexchange/types/state.go](../../../modules/layerxexchange/types/state.go) |
| `go_exported` | `SideBuy  uint8 = 1` | [modules/layerxexchange/types/state.go](../../../modules/layerxexchange/types/state.go) |
| `go_exported` | `SideSell uint8 = 2` | [modules/layerxexchange/types/state.go](../../../modules/layerxexchange/types/state.go) |
| `go_exported` | `TimeInForceGoodTillCancelled uint8 = 0` | [modules/layerxexchange/types/state.go](../../../modules/layerxexchange/types/state.go) |
| `go_exported` | `TimeInForceImmediateOrCancel uint8 = 1` | [modules/layerxexchange/types/state.go](../../../modules/layerxexchange/types/state.go) |
| `go_exported` | `TimeInForceFillOrKill        uint8 = 2` | [modules/layerxexchange/types/state.go](../../../modules/layerxexchange/types/state.go) |
| `go_exported` | `TimeInForcePostOnly          uint8 = 3` | [modules/layerxexchange/types/state.go](../../../modules/layerxexchange/types/state.go) |
| `go_exported` | `MaxWitnessBytes = 64 * 1024` | [modules/layerxexchange/types/state.go](../../../modules/layerxexchange/types/state.go) |
| `go_exported` | `func MarketStateKey(marketID [32]byte) []byte { return perpsKey("market:", marketID) }` | [modules/layerxexchange/types/state.go](../../../modules/layerxexchange/types/state.go) |
| `go_exported` | `func OrderStateKey(marketID, orderID [32]byte) []byte { return perpsKey("order:", marketID, orderID) }` | [modules/layerxexchange/types/state.go](../../../modules/layerxexchange/types/state.go) |
| `go_exported` | `func PositionStateKey(marketID, positionID [32]byte) []byte` | [modules/layerxexchange/types/state.go](../../../modules/layerxexchange/types/state.go) |
| `go_exported` | `GovernanceModuleID    uint16 = 7` | [modules/layerxexchange/types/state.go](../../../modules/layerxexchange/types/state.go) |
| `go_exported` | `MaxNativeGenesisBytes        = 262144` | [modules/layerxexchange/types/state.go](../../../modules/layerxexchange/types/state.go) |
| `go_exported` | `func GenesisManifestStateKey() []byte { return []byte("genesis/manifest/v1") }` | [modules/layerxexchange/types/state.go](../../../modules/layerxexchange/types/state.go) |
| `go_exported` | `func NativeGenesisCapability(encoded []byte) ([32]byte, bool, error)` | [modules/layerxexchange/types/state.go](../../../modules/layerxexchange/types/state.go) |

## go-exported-precompiles-layerxexchange

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `ExchangeAddress = types.ExchangeAddress` | [precompiles/layerxexchange/layerxexchange.go](../../../precompiles/layerxexchange/layerxexchange.go) |
| `go_exported` | `PrecompileName  = "layerxExchange"` | [precompiles/layerxexchange/layerxexchange.go](../../../precompiles/layerxexchange/layerxexchange.go) |
| `go_exported` | `type PrecompileExecutor struct` | [precompiles/layerxexchange/layerxexchange.go](../../../precompiles/layerxexchange/layerxexchange.go) |
| `go_exported` | `func NewPrecompile(keepers utils.Keepers) (*pcommon.Precompile, error)` | [precompiles/layerxexchange/layerxexchange.go](../../../precompiles/layerxexchange/layerxexchange.go) |
| `go_exported` | `func GetVersioned(latestUpgrade string, keepers utils.Keepers) utils.VersionedPrecompiles` | [precompiles/layerxexchange/setup.go](../../../precompiles/layerxexchange/setup.go) |

<a id="integration-wiring"></a>
<a id="wiring-integration"></a>

See the implementation sources above.
