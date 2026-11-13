# Perps data contract

The [perps-data service](../../../services/platform/perps-data) supplies indexed market data, authenticated APIs and live delivery. Its command, configuration, storage and transport declarations are listed below. Indexed observations do not establish transaction finality; that follows block commitment on the Paxeer network.

The tables identify code declarations and configuration names. Activation, authentication and transport registration are defined by the linked implementations.

## auth

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `func Address(ctx context.Context) (string, bool)` | [services/platform/perps-data/internal/auth/auth.go](../../../services/platform/perps-data/internal/auth/auth.go) |
| `go_exported` | `func Middleware(s *Service, next http.Handler) http.Handler` | [services/platform/perps-data/internal/auth/auth.go](../../../services/platform/perps-data/internal/auth/auth.go) |
| `go_exported` | `func NewService(db *sql.DB, secret string, now func() time.Time) (*Service, error)` | [services/platform/perps-data/internal/auth/auth.go](../../../services/platform/perps-data/internal/auth/auth.go) |
| `go_exported` | `type Service struct` | [services/platform/perps-data/internal/auth/auth.go](../../../services/platform/perps-data/internal/auth/auth.go) |
| `go_exported` | `TokenTTL = 3600 * time.Second` | [services/platform/perps-data/internal/auth/auth.go](../../../services/platform/perps-data/internal/auth/auth.go) |

## cli

| Kind | Declaration | Source |
|---|---|---|
| `cli_command` | `perps-explorer` | [services/platform/perps-data/cmd/perps-explorer/main.go](../../../services/platform/perps-data/cmd/perps-explorer/main.go) |
| `cli_command` | `perps-gateway` | [services/platform/perps-data/cmd/perps-gateway/main.go](../../../services/platform/perps-data/cmd/perps-gateway/main.go) |
| `cli_command` | `perps-indexer` | [services/platform/perps-data/cmd/perps-indexer/main.go](../../../services/platform/perps-data/cmd/perps-indexer/main.go) |
| `cli_command` | `perps-loadtest` | [services/platform/perps-data/cmd/perps-loadtest/main.go](../../../services/platform/perps-data/cmd/perps-loadtest/main.go) |
| `cli_command` | `perps-portal` | [services/platform/perps-data/cmd/perps-portal/main.go](../../../services/platform/perps-data/cmd/perps-portal/main.go) |
| `cli_command` | `perps-replay` | [services/platform/perps-data/cmd/perps-replay/main.go](../../../services/platform/perps-data/cmd/perps-replay/main.go) |

## configuration

| Kind | Declaration | Source |
|---|---|---|
| `env_var` | `DATABASE_URL` | [services/platform/perps-data/internal/config/config.go](../../../services/platform/perps-data/internal/config/config.go) |
| `env_var` | `NATS_CLUSTER_PASSWORD` | [services/platform/perps-data/deploy/nats/nats.conf](../../../services/platform/perps-data/deploy/nats/nats.conf) |
| `env_var` | `NATS_GATEWAY_PASSWORD` | [services/platform/perps-data/deploy/nats/nats.conf](../../../services/platform/perps-data/deploy/nats/nats.conf) |
| `env_var` | `NATS_INDEXER_PASSWORD` | [services/platform/perps-data/deploy/nats/nats.conf](../../../services/platform/perps-data/deploy/nats/nats.conf) |
| `env_var` | `NATS_MAX_FILE_STORE` | [services/platform/perps-data/deploy/nats/nats.conf](../../../services/platform/perps-data/deploy/nats/nats.conf) |
| `env_var` | `NATS_ROUTES` | [services/platform/perps-data/deploy/nats/nats.conf](../../../services/platform/perps-data/deploy/nats/nats.conf) |
| `env_var` | `NATS_URL` | [services/platform/perps-data/internal/config/config.go](../../../services/platform/perps-data/internal/config/config.go) |
| `env_var` | `PERPS_AUTH_SECRET` | [services/platform/perps-data/internal/config/config.go](../../../services/platform/perps-data/internal/config/config.go) |
| `env_var` | `PERPS_CHAIN_ID` | [services/platform/perps-data/internal/config/config.go](../../../services/platform/perps-data/internal/config/config.go) |
| `env_var` | `PERPS_COMET_URLS` | [services/platform/perps-data/internal/config/config.go](../../../services/platform/perps-data/internal/config/config.go) |
| `env_var` | `PERPS_DATA_TEST_COMET` | [services/platform/perps-data/e2e_test.go](../../../services/platform/perps-data/e2e_test.go) |
| `env_var` | `PERPS_DATA_TEST_DATABASE_URL` | [services/platform/perps-data/e2e_test.go](../../../services/platform/perps-data/e2e_test.go) |
| `env_var` | `PERPS_DATA_TEST_RPC` | [services/platform/perps-data/e2e_test.go](../../../services/platform/perps-data/e2e_test.go) |
| `env_var` | `PERPS_DATA_TEST_SEED_KEY_FILE` | [services/platform/perps-data/e2e_test.go](../../../services/platform/perps-data/e2e_test.go) |
| `env_var` | `PERPS_EXPLORER_TX_BASE` | [services/platform/perps-data/internal/config/config.go](../../../services/platform/perps-data/internal/config/config.go) |
| `env_var` | `PERPS_GATEWAY_URL` | [services/platform/perps-data/internal/config/config.go](../../../services/platform/perps-data/internal/config/config.go) |
| `env_var` | `PERPS_RPC_URLS` | [services/platform/perps-data/internal/config/config.go](../../../services/platform/perps-data/internal/config/config.go) |
| `env_var` | `PERPS_START_HEIGHT` | [services/platform/perps-data/internal/config/config.go](../../../services/platform/perps-data/internal/config/config.go) |
| `env_var` | `PERPS_TRUSTED_PROXY_CIDRS` | [services/platform/perps-data/internal/ratelimit/ratelimit.go](../../../services/platform/perps-data/internal/ratelimit/ratelimit.go) |
| `env_var` | `PORT` | [services/platform/perps-data/internal/config/config.go](../../../services/platform/perps-data/internal/config/config.go) |
| `go_exported` | `func (c Config) String() string` | [services/platform/perps-data/internal/config/config.go](../../../services/platform/perps-data/internal/config/config.go) |
| `go_exported` | `Explorer Service = "explorer"` | [services/platform/perps-data/internal/config/config.go](../../../services/platform/perps-data/internal/config/config.go) |
| `go_exported` | `Gateway  Service = "gateway"` | [services/platform/perps-data/internal/config/config.go](../../../services/platform/perps-data/internal/config/config.go) |
| `go_exported` | `Indexer  Service = "indexer"` | [services/platform/perps-data/internal/config/config.go](../../../services/platform/perps-data/internal/config/config.go) |
| `go_exported` | `func Load(service Service) (*Config, error)` | [services/platform/perps-data/internal/config/config.go](../../../services/platform/perps-data/internal/config/config.go) |
| `go_exported` | `Loadtest Service = "loadtest"` | [services/platform/perps-data/internal/config/config.go](../../../services/platform/perps-data/internal/config/config.go) |
| `go_exported` | `Portal   Service = "portal"` | [services/platform/perps-data/internal/config/config.go](../../../services/platform/perps-data/internal/config/config.go) |
| `go_exported` | `Replay   Service = "replay"` | [services/platform/perps-data/internal/config/config.go](../../../services/platform/perps-data/internal/config/config.go) |
| `go_exported` | `type Service string` | [services/platform/perps-data/internal/config/config.go](../../../services/platform/perps-data/internal/config/config.go) |

## decode

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `type AccountLookup struct` | [services/platform/perps-data/internal/decode/registry.go](../../../services/platform/perps-data/internal/decode/registry.go) |
| `go_exported` | `type ADLExecutedRow struct` | [services/platform/perps-data/internal/decode/perps.go](../../../services/platform/perps-data/internal/decode/perps.go) |
| `go_exported` | `type BlockInput struct` | [services/platform/perps-data/internal/decode/pricefeed.go](../../../services/platform/perps-data/internal/decode/pricefeed.go) |
| `go_exported` | `type CollateralDepositedRow struct` | [services/platform/perps-data/internal/decode/perps.go](../../../services/platform/perps-data/internal/decode/perps.go) |
| `go_exported` | `type CollateralTransferredRow struct` | [services/platform/perps-data/internal/decode/perps.go](../../../services/platform/perps-data/internal/decode/perps.go) |
| `go_exported` | `type CollateralWithdrawnRow struct` | [services/platform/perps-data/internal/decode/perps.go](../../../services/platform/perps-data/internal/decode/perps.go) |
| `go_exported` | `func ColumnName(field string) string` | [services/platform/perps-data/internal/decode/registry.go](../../../services/platform/perps-data/internal/decode/registry.go) |
| `go_exported` | `type Decoded struct` | [services/platform/perps-data/internal/decode/registry.go](../../../services/platform/perps-data/internal/decode/registry.go) |
| `go_exported` | `ErrStatePruned = errors.New("decode: state pruned on every node")` | [services/platform/perps-data/internal/decode/registry.go](../../../services/platform/perps-data/internal/decode/registry.go) |
| `go_exported` | `type Event struct` | [services/platform/perps-data/internal/decode/registry.go](../../../services/platform/perps-data/internal/decode/registry.go) |
| `go_exported` | `EventAggregate        = "pricefeed_aggregate"` | [services/platform/perps-data/internal/decode/pricefeed.go](../../../services/platform/perps-data/internal/decode/pricefeed.go) |
| `go_exported` | `EventAggregateSkipped = "pricefeed_aggregate_skipped"` | [services/platform/perps-data/internal/decode/pricefeed.go](../../../services/platform/perps-data/internal/decode/pricefeed.go) |
| `go_exported` | `func (f Fact) Common() Fact { return f }` | [services/platform/perps-data/internal/decode/registry.go](../../../services/platform/perps-data/internal/decode/registry.go) |
| `go_exported` | `type Fact struct` | [services/platform/perps-data/internal/decode/registry.go](../../../services/platform/perps-data/internal/decode/registry.go) |
| `go_exported` | `type FeeCollectedRow struct` | [services/platform/perps-data/internal/decode/perps.go](../../../services/platform/perps-data/internal/decode/perps.go) |
| `go_exported` | `FeeMakerRebate uint8 = 4` | [services/platform/perps-data/internal/decode/perps.go](../../../services/platform/perps-data/internal/decode/perps.go) |
| `go_exported` | `FeeTrading     uint8 = 0` | [services/platform/perps-data/internal/decode/perps.go](../../../services/platform/perps-data/internal/decode/perps.go) |
| `go_exported` | `type FillRow struct` | [services/platform/perps-data/internal/decode/perps.go](../../../services/platform/perps-data/internal/decode/perps.go) |
| `go_exported` | `type FundingRateUpdatedRow struct` | [services/platform/perps-data/internal/decode/perps.go](../../../services/platform/perps-data/internal/decode/perps.go) |
| `go_exported` | `type FundingSettledRow struct` | [services/platform/perps-data/internal/decode/perps.go](../../../services/platform/perps-data/internal/decode/perps.go) |
| `go_exported` | `type IndexPriceRow struct` | [services/platform/perps-data/internal/decode/pricefeed.go](../../../services/platform/perps-data/internal/decode/pricefeed.go) |
| `go_exported` | `type InsuranceDrawRow struct` | [services/platform/perps-data/internal/decode/perps.go](../../../services/platform/perps-data/internal/decode/perps.go) |
| `go_exported` | `func KindRows() []KindRow` | [services/platform/perps-data/internal/decode/kinds.go](../../../services/platform/perps-data/internal/decode/kinds.go) |
| `go_exported` | `type LiquidationRow struct` | [services/platform/perps-data/internal/decode/perps.go](../../../services/platform/perps-data/internal/decode/perps.go) |
| `go_exported` | `type Log struct` | [services/platform/perps-data/internal/decode/registry.go](../../../services/platform/perps-data/internal/decode/registry.go) |
| `go_exported` | `type MarkPriceUpdatedRow struct` | [services/platform/perps-data/internal/decode/perps.go](../../../services/platform/perps-data/internal/decode/perps.go) |
| `go_exported` | `func ModuleAddress(name string) common.Address` | [services/platform/perps-data/internal/decode/registry.go](../../../services/platform/perps-data/internal/decode/registry.go) |
| `go_exported` | `func NewRegistry(metrics *telemetry.Metrics, opts Options) (*Registry, error)` | [services/platform/perps-data/internal/decode/registry.go](../../../services/platform/perps-data/internal/decode/registry.go) |
| `go_exported` | `type OpenInterestChangedRow struct` | [services/platform/perps-data/internal/decode/perps.go](../../../services/platform/perps-data/internal/decode/perps.go) |
| `go_exported` | `type Options struct` | [services/platform/perps-data/internal/decode/registry.go](../../../services/platform/perps-data/internal/decode/registry.go) |
| `go_exported` | `type OrderCancelledRow struct` | [services/platform/perps-data/internal/decode/perps.go](../../../services/platform/perps-data/internal/decode/perps.go) |
| `go_exported` | `type OrderExecutedRow struct` | [services/platform/perps-data/internal/decode/perps.go](../../../services/platform/perps-data/internal/decode/perps.go) |
| `go_exported` | `type OrderExpiredRow struct` | [services/platform/perps-data/internal/decode/perps.go](../../../services/platform/perps-data/internal/decode/perps.go) |
| `go_exported` | `type OrderOptionsRow struct` | [services/platform/perps-data/internal/decode/perps.go](../../../services/platform/perps-data/internal/decode/perps.go) |
| `go_exported` | `type OrderPlacedRow struct` | [services/platform/perps-data/internal/decode/perps.go](../../../services/platform/perps-data/internal/decode/perps.go) |
| `go_exported` | `type OrderTriggeredRow struct` | [services/platform/perps-data/internal/decode/perps.go](../../../services/platform/perps-data/internal/decode/perps.go) |
| `go_exported` | `var PerpsAddress = common.HexToAddress("0x0000000000000000000000000000000000001015")` | [services/platform/perps-data/internal/decode/perps.go](../../../services/platform/perps-data/internal/decode/perps.go) |
| `go_exported` | `type PositionClosedRow struct` | [services/platform/perps-data/internal/decode/perps.go](../../../services/platform/perps-data/internal/decode/perps.go) |
| `go_exported` | `type PositionEntryPriceChangedRow struct` | [services/platform/perps-data/internal/decode/perps.go](../../../services/platform/perps-data/internal/decode/perps.go) |
| `go_exported` | `type PositionFundingAppliedRow struct` | [services/platform/perps-data/internal/decode/perps.go](../../../services/platform/perps-data/internal/decode/perps.go) |
| `go_exported` | `type PositionModifiedRow struct` | [services/platform/perps-data/internal/decode/perps.go](../../../services/platform/perps-data/internal/decode/perps.go) |
| `go_exported` | `type PositionOpenedRow struct` | [services/platform/perps-data/internal/decode/perps.go](../../../services/platform/perps-data/internal/decode/perps.go) |
| `go_exported` | `type PositionTpSlSetRow struct` | [services/platform/perps-data/internal/decode/perps.go](../../../services/platform/perps-data/internal/decode/perps.go) |
| `go_exported` | `var PricefeedAddress = common.HexToAddress("0x0000000000000000000000000000000000001020")` | [services/platform/perps-data/internal/decode/pricefeed.go](../../../services/platform/perps-data/internal/decode/pricefeed.go) |
| `go_exported` | `type Querier interface` | [services/platform/perps-data/internal/decode/registry.go](../../../services/platform/perps-data/internal/decode/registry.go) |
| `go_exported` | `ReceiptActivation    ReceiptKind = "activation"` | [services/platform/perps-data/internal/decode/registry.go](../../../services/platform/perps-data/internal/decode/registry.go) |
| `go_exported` | `ReceiptBlockEvent    ReceiptKind = "block_event"` | [services/platform/perps-data/internal/decode/registry.go](../../../services/platform/perps-data/internal/decode/registry.go) |
| `go_exported` | `type ReceiptKind string` | [services/platform/perps-data/internal/decode/registry.go](../../../services/platform/perps-data/internal/decode/registry.go) |
| `go_exported` | `ReceiptPerpsEndBlock ReceiptKind = "perps_endblock"` | [services/platform/perps-data/internal/decode/registry.go](../../../services/platform/perps-data/internal/decode/registry.go) |
| `go_exported` | `ReceiptSpotEndBlock  ReceiptKind = "spot_endblock"` | [services/platform/perps-data/internal/decode/registry.go](../../../services/platform/perps-data/internal/decode/registry.go) |
| `go_exported` | `ReceiptTx            ReceiptKind = "tx"` | [services/platform/perps-data/internal/decode/registry.go](../../../services/platform/perps-data/internal/decode/registry.go) |
| `go_exported` | `func (r *Registry) Decode(ctx context.Context, q Querier, logs []Log) (*Result, error)` | [services/platform/perps-data/internal/decode/registry.go](../../../services/platform/perps-data/internal/decode/registry.go) |
| `go_exported` | `func (r *Registry) DecodeBlock(ctx context.Context, q Querier, in BlockInput) (*Result, error)` | [services/platform/perps-data/internal/decode/pricefeed.go](../../../services/platform/perps-data/internal/decode/pricefeed.go) |
| `go_exported` | `type Registry struct` | [services/platform/perps-data/internal/decode/registry.go](../../../services/platform/perps-data/internal/decode/registry.go) |
| `go_exported` | `func (r *Registry) Keys() []Key` | [services/platform/perps-data/internal/decode/registry.go](../../../services/platform/perps-data/internal/decode/registry.go) |
| `go_exported` | `type Result struct` | [services/platform/perps-data/internal/decode/registry.go](../../../services/platform/perps-data/internal/decode/registry.go) |
| `go_exported` | `func (e *RevertError) Error() string { return "decode: call reverted 0x" + hex.EncodeToString(e.Data) }` | [services/platform/perps-data/internal/decode/registry.go](../../../services/platform/perps-data/internal/decode/registry.go) |
| `go_exported` | `type RevertError struct` | [services/platform/perps-data/internal/decode/registry.go](../../../services/platform/perps-data/internal/decode/registry.go) |
| `go_exported` | `type Row interface` | [services/platform/perps-data/internal/decode/registry.go](../../../services/platform/perps-data/internal/decode/registry.go) |
| `go_exported` | `type Seed struct` | [services/platform/perps-data/internal/decode/seed.go](../../../services/platform/perps-data/internal/decode/seed.go) |
| `go_exported` | `type SpotMarketOrderRow struct` | [services/platform/perps-data/internal/decode/spot.go](../../../services/platform/perps-data/internal/decode/spot.go) |
| `go_exported` | `type SpotNativeSwapRow struct` | [services/platform/perps-data/internal/decode/spot.go](../../../services/platform/perps-data/internal/decode/spot.go) |
| `go_exported` | `SpotPoolAddress   = common.HexToAddress("0x0000000000000000000000000000000000001021")` | [services/platform/perps-data/internal/decode/spot.go](../../../services/platform/perps-data/internal/decode/spot.go) |
| `go_exported` | `type SpotSimpleSwapRow struct` | [services/platform/perps-data/internal/decode/spot.go](../../../services/platform/perps-data/internal/decode/spot.go) |
| `go_exported` | `type SpotTieredFeeRow struct` | [services/platform/perps-data/internal/decode/spot.go](../../../services/platform/perps-data/internal/decode/spot.go) |
| `go_exported` | `SpotVaultAddress  = common.HexToAddress("0x0000000000000000000000000000000000001025")` | [services/platform/perps-data/internal/decode/spot.go](../../../services/platform/perps-data/internal/decode/spot.go) |
| `go_exported` | `TradeClose       uint8 = 1` | [services/platform/perps-data/internal/decode/perps.go](../../../services/platform/perps-data/internal/decode/perps.go) |
| `go_exported` | `TradeLiquidation uint8 = 2` | [services/platform/perps-data/internal/decode/perps.go](../../../services/platform/perps-data/internal/decode/perps.go) |
| `go_exported` | `TradeOpen        uint8 = 0` | [services/platform/perps-data/internal/decode/perps.go](../../../services/platform/perps-data/internal/decode/perps.go) |
| `go_exported` | `type TradeSettledRow struct` | [services/platform/perps-data/internal/decode/perps.go](../../../services/platform/perps-data/internal/decode/perps.go) |
| `go_exported` | `type VaultCreatedRow struct` | [services/platform/perps-data/internal/decode/perps.go](../../../services/platform/perps-data/internal/decode/perps.go) |

## delivery

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `type BatchPublisher interface` | [services/platform/perps-data/internal/delivery/relay.go](../../../services/platform/perps-data/internal/delivery/relay.go) |
| `go_exported` | `func ControlMessage(l Lease) (subject, msgID string, body []byte, err error)` | [services/platform/perps-data/internal/delivery/store.go](../../../services/platform/perps-data/internal/delivery/store.go) |
| `go_exported` | `ControlSkipped = "skipped"` | [services/platform/perps-data/internal/delivery/store.go](../../../services/platform/perps-data/internal/delivery/store.go) |
| `go_exported` | `func Decode(b []byte, p Policy, subject string) (Envelope, error)` | [services/platform/perps-data/internal/delivery/envelope.go](../../../services/platform/perps-data/internal/delivery/envelope.go) |
| `go_exported` | `type Delivery interface{ Ack(context.Context) error }` | [services/platform/perps-data/internal/delivery/store.go](../../../services/platform/perps-data/internal/delivery/store.go) |
| `go_exported` | `func (e Envelope) Encode(p Policy, subject string) ([]byte, error)` | [services/platform/perps-data/internal/delivery/envelope.go](../../../services/platform/perps-data/internal/delivery/envelope.go) |
| `go_exported` | `type Envelope struct` | [services/platform/perps-data/internal/delivery/envelope.go](../../../services/platform/perps-data/internal/delivery/envelope.go) |
| `go_exported` | `ErrAuthorization = errors.New("delivery: unapproved subject")` | [services/platform/perps-data/internal/delivery/envelope.go](../../../services/platform/perps-data/internal/delivery/envelope.go) |
| `go_exported` | `var ErrBrokerDown = errors.New("delivery: broker unreachable")` | [services/platform/perps-data/internal/delivery/relay.go](../../../services/platform/perps-data/internal/delivery/relay.go) |
| `go_exported` | `var ErrInFlight = errors.New("delivery: message still in flight at the broker")` | [services/platform/perps-data/internal/delivery/relay.go](../../../services/platform/perps-data/internal/delivery/relay.go) |
| `go_exported` | `ErrIntegrity     = errors.New("delivery: immutable identity conflict")` | [services/platform/perps-data/internal/delivery/envelope.go](../../../services/platform/perps-data/internal/delivery/envelope.go) |
| `go_exported` | `ErrOversized = errors.New("delivery: message refused for size")` | [services/platform/perps-data/internal/delivery/envelope.go](../../../services/platform/perps-data/internal/delivery/envelope.go) |
| `go_exported` | `func Hash(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }` | [services/platform/perps-data/internal/delivery/envelope.go](../../../services/platform/perps-data/internal/delivery/envelope.go) |
| `go_exported` | `HeaderControl  = "Perps-Control"` | [services/platform/perps-data/internal/delivery/store.go](../../../services/platform/perps-data/internal/delivery/store.go) |
| `go_exported` | `type Lease struct` | [services/platform/perps-data/internal/delivery/store.go](../../../services/platform/perps-data/internal/delivery/store.go) |
| `go_exported` | `const MaxRetryDelay = time.Minute` | [services/platform/perps-data/internal/delivery/store.go](../../../services/platform/perps-data/internal/delivery/store.go) |
| `go_exported` | `func NewStore(db *sql.DB, schema string, policy Policy) (*Store, error)` | [services/platform/perps-data/internal/delivery/store.go](../../../services/platform/perps-data/internal/delivery/store.go) |
| `go_exported` | `type PersistenceAck struct` | [services/platform/perps-data/internal/delivery/store.go](../../../services/platform/perps-data/internal/delivery/store.go) |
| `go_exported` | `type PersistentPublisher interface` | [services/platform/perps-data/internal/delivery/relay.go](../../../services/platform/perps-data/internal/delivery/relay.go) |
| `go_exported` | `type Policy struct` | [services/platform/perps-data/internal/delivery/envelope.go](../../../services/platform/perps-data/internal/delivery/envelope.go) |
| `go_exported` | `type Publication struct` | [services/platform/perps-data/internal/delivery/relay.go](../../../services/platform/perps-data/internal/delivery/relay.go) |
| `go_exported` | `type Quarantine struct` | [services/platform/perps-data/internal/delivery/store.go](../../../services/platform/perps-data/internal/delivery/store.go) |
| `go_exported` | `type Relay struct` | [services/platform/perps-data/internal/delivery/relay.go](../../../services/platform/perps-data/internal/delivery/relay.go) |
| `go_exported` | `func (r Relay) Pass(ctx context.Context) error` | [services/platform/perps-data/internal/delivery/relay.go](../../../services/platform/perps-data/internal/delivery/relay.go) |
| `go_exported` | `type SourceOrder struct` | [services/platform/perps-data/internal/delivery/envelope.go](../../../services/platform/perps-data/internal/delivery/envelope.go) |
| `go_exported` | `func (s *Store) Enqueue(ctx context.Context, tx *sql.Tx, subject string, e Envelope) (int64, error)` | [services/platform/perps-data/internal/delivery/store.go](../../../services/platform/perps-data/internal/delivery/store.go) |
| `go_exported` | `func (s *Store) HeadAge(ctx context.Context) (time.Duration, error)` | [services/platform/perps-data/internal/delivery/store.go](../../../services/platform/perps-data/internal/delivery/store.go) |
| `go_exported` | `type Store struct` | [services/platform/perps-data/internal/delivery/store.go](../../../services/platform/perps-data/internal/delivery/store.go) |
| `go_exported` | `func (s *Store) Quarantined(ctx context.Context) (int64, error)` | [services/platform/perps-data/internal/delivery/store.go](../../../services/platform/perps-data/internal/delivery/store.go) |
| `go_exported` | `func (s *Store) Retry(ctx context.Context, l Lease, class FailureClass, maxAttempts int, delay time.Duration) error` | [services/platform/perps-data/internal/delivery/store.go](../../../services/platform/perps-data/internal/delivery/store.go) |
| `go_exported` | `func (c *Consumer) Close()` | [services/platform/perps-data/internal/delivery/natsdriver/driver.go](../../../services/platform/perps-data/internal/delivery/natsdriver/driver.go) |
| `go_exported` | `func (c *Consumer) Fetch(ctx context.Context) ([]*Message, error)` | [services/platform/perps-data/internal/delivery/natsdriver/driver.go](../../../services/platform/perps-data/internal/delivery/natsdriver/driver.go) |
| `go_exported` | `func (c *Consumer) Run(ctx context.Context, consume func(context.Context, *Message) error) error` | [services/platform/perps-data/internal/delivery/natsdriver/driver.go](../../../services/platform/perps-data/internal/delivery/natsdriver/driver.go) |
| `go_exported` | `func (c ConsumerConfig) BrokerConsumerConfig() (jetstream.ConsumerConfig, error)` | [services/platform/perps-data/internal/delivery/natsdriver/driver.go](../../../services/platform/perps-data/internal/delivery/natsdriver/driver.go) |
| `go_exported` | `type ConsumerConfig struct` | [services/platform/perps-data/internal/delivery/natsdriver/driver.go](../../../services/platform/perps-data/internal/delivery/natsdriver/driver.go) |
| `go_exported` | `func (m *Message) Ack(ctx context.Context) error` | [services/platform/perps-data/internal/delivery/natsdriver/driver.go](../../../services/platform/perps-data/internal/delivery/natsdriver/driver.go) |
| `go_exported` | `type Message struct` | [services/platform/perps-data/internal/delivery/natsdriver/driver.go](../../../services/platform/perps-data/internal/delivery/natsdriver/driver.go) |
| `go_exported` | `func MsgID(eventID string) (string, error)` | [services/platform/perps-data/internal/delivery/natsdriver/driver.go](../../../services/platform/perps-data/internal/delivery/natsdriver/driver.go) |
| `go_exported` | `func NewConsumer(ctx context.Context, nc *nats.Conn, cfg ConsumerConfig) (*Consumer, error)` | [services/platform/perps-data/internal/delivery/natsdriver/driver.go](../../../services/platform/perps-data/internal/delivery/natsdriver/driver.go) |
| `go_exported` | `func NewPublisher(nc *nats.Conn, stream string) (*Publisher, error)` | [services/platform/perps-data/internal/delivery/natsdriver/driver.go](../../../services/platform/perps-data/internal/delivery/natsdriver/driver.go) |
| `go_exported` | `type Publisher struct` | [services/platform/perps-data/internal/delivery/natsdriver/driver.go](../../../services/platform/perps-data/internal/delivery/natsdriver/driver.go) |
| `go_exported` | `func (p *Publisher) Publish(ctx context.Context, subject, eventID string, wire []byte) (delivery.PersistenceAck, error)` | [services/platform/perps-data/internal/delivery/natsdriver/driver.go](../../../services/platform/perps-data/internal/delivery/natsdriver/driver.go) |
| `go_exported` | `type Config struct` | [services/platform/perps-data/internal/publish/publisher.go](../../../services/platform/perps-data/internal/publish/publisher.go) |
| `go_exported` | `func EnsureStream(ctx context.Context, nc *nats.Conn, chain uint64) error` | [services/platform/perps-data/internal/publish/publisher.go](../../../services/platform/perps-data/internal/publish/publisher.go) |
| `go_exported` | `HeaderBlock = "Perps-Block"` | [services/platform/perps-data/internal/publish/publisher.go](../../../services/platform/perps-data/internal/publish/publisher.go) |
| `go_exported` | `HeaderKind  = "Perps-Kind"` | [services/platform/perps-data/internal/publish/publisher.go](../../../services/platform/perps-data/internal/publish/publisher.go) |
| `go_exported` | `HeaderSeq   = "Perps-Seq"` | [services/platform/perps-data/internal/publish/publisher.go](../../../services/platform/perps-data/internal/publish/publisher.go) |
| `go_exported` | `func New(db *sql.DB, nc *nats.Conn, policy delivery.Policy, cfg Config) (*Publisher, error)` | [services/platform/perps-data/internal/publish/publisher.go](../../../services/platform/perps-data/internal/publish/publisher.go) |
| `go_exported` | `func (p *Publisher) Collectors() []prometheus.Collector` | [services/platform/perps-data/internal/publish/publisher.go](../../../services/platform/perps-data/internal/publish/publisher.go) |
| `go_exported` | `type Publisher struct` | [services/platform/perps-data/internal/publish/publisher.go](../../../services/platform/perps-data/internal/publish/publisher.go) |
| `go_exported` | `func (p *Publisher) Ready(ctx context.Context) error` | [services/platform/perps-data/internal/publish/publisher.go](../../../services/platform/perps-data/internal/publish/publisher.go) |
| `go_exported` | `func (p *Publisher) Run(ctx context.Context) error` | [services/platform/perps-data/internal/publish/publisher.go](../../../services/platform/perps-data/internal/publish/publisher.go) |
| `go_exported` | `func ChainSubject(chain uint64, kind string) (string, error)` | [services/platform/perps-data/internal/stream/layout.go](../../../services/platform/perps-data/internal/stream/layout.go) |
| `go_exported` | `func Config(chain uint64) jetstream.StreamConfig` | [services/platform/perps-data/internal/stream/layout.go](../../../services/platform/perps-data/internal/stream/layout.go) |
| `go_exported` | `type Family string` | [services/platform/perps-data/internal/stream/layout.go](../../../services/platform/perps-data/internal/stream/layout.go) |
| `go_exported` | `func MarketSubject(chain, market uint64, kind string) (string, error)` | [services/platform/perps-data/internal/stream/layout.go](../../../services/platform/perps-data/internal/stream/layout.go) |
| `go_exported` | `func Matches(filter, subject string) bool` | [services/platform/perps-data/internal/stream/layout.go](../../../services/platform/perps-data/internal/stream/layout.go) |
| `go_exported` | `const Name = "PERPS_V1"` | [services/platform/perps-data/internal/stream/layout.go](../../../services/platform/perps-data/internal/stream/layout.go) |
| `go_exported` | `func SpotSubject(chain uint64, token common.Address, kind string) (string, error)` | [services/platform/perps-data/internal/stream/layout.go](../../../services/platform/perps-data/internal/stream/layout.go) |
| `go_exported` | `func Subject(chain uint64, family Family, entity, kind string) (string, error)` | [services/platform/perps-data/internal/stream/layout.go](../../../services/platform/perps-data/internal/stream/layout.go) |
| `go_exported` | `func UserSubject(chain uint64, account common.Address, kind string) (string, error)` | [services/platform/perps-data/internal/stream/layout.go](../../../services/platform/perps-data/internal/stream/layout.go) |
| `go_exported` | `func ValidFilter(filter string) bool` | [services/platform/perps-data/internal/stream/layout.go](../../../services/platform/perps-data/internal/stream/layout.go) |

## explorer

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `func Action(kind string) (string, error)` | [services/platform/perps-data/internal/explorer/explorer.go](../../../services/platform/perps-data/internal/explorer/explorer.go) |
| `go_exported` | `ActionOracleUpdated       = "Oracle Updated"` | [services/platform/perps-data/internal/explorer/explorer.go](../../../services/platform/perps-data/internal/explorer/explorer.go) |
| `go_exported` | `ActionSealedBatchExecuted = "Sealed Batch Executed"` | [services/platform/perps-data/internal/explorer/explorer.go](../../../services/platform/perps-data/internal/explorer/explorer.go) |
| `go_exported` | `func (c Cursor) String() string` | [services/platform/perps-data/internal/explorer/explorer.go](../../../services/platform/perps-data/internal/explorer/explorer.go) |
| `go_exported` | `type HistoryItem struct` | [services/platform/perps-data/internal/explorer/explorer.go](../../../services/platform/perps-data/internal/explorer/explorer.go) |
| `go_exported` | `KindPricefeedAggregate = "pricefeed_aggregate"` | [services/platform/perps-data/internal/explorer/explorer.go](../../../services/platform/perps-data/internal/explorer/explorer.go) |
| `go_exported` | `func NewServer(db *sql.DB, opts Options) (*Server, error)` | [services/platform/perps-data/internal/explorer/server.go](../../../services/platform/perps-data/internal/explorer/server.go) |
| `go_exported` | `type Options struct` | [services/platform/perps-data/internal/explorer/server.go](../../../services/platform/perps-data/internal/explorer/server.go) |
| `go_exported` | `func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request)` | [services/platform/perps-data/internal/explorer/server.go](../../../services/platform/perps-data/internal/explorer/server.go) |
| `http_route` | `GET /api/chain/accounts/{address}/history` | [services/platform/perps-data/internal/explorer/server.go](../../../services/platform/perps-data/internal/explorer/server.go) |
| `http_route` | `GET /api/chain/blocks` | [services/platform/perps-data/internal/explorer/server.go](../../../services/platform/perps-data/internal/explorer/server.go) |
| `http_route` | `GET /api/chain/health` | [services/platform/perps-data/internal/explorer/server.go](../../../services/platform/perps-data/internal/explorer/server.go) |
| `http_route` | `GET /api/chain/markets/{symbol}/history` | [services/platform/perps-data/internal/explorer/server.go](../../../services/platform/perps-data/internal/explorer/server.go) |
| `http_route` | `GET /api/chain/transactions` | [services/platform/perps-data/internal/explorer/server.go](../../../services/platform/perps-data/internal/explorer/server.go) |
| `http_route` | `GET /api/chain/tx/{hash}` | [services/platform/perps-data/internal/explorer/server.go](../../../services/platform/perps-data/internal/explorer/server.go) |
| `http_route` | `GET /api/explorer/health` | [services/platform/perps-data/internal/explorer/server.go](../../../services/platform/perps-data/internal/explorer/server.go) |

## gateway

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `func NewServer(ctx context.Context, opts Options) (*Server, error)` | [services/platform/perps-data/internal/gateway/server.go](../../../services/platform/perps-data/internal/gateway/server.go) |
| `go_exported` | `type Options struct` | [services/platform/perps-data/internal/gateway/server.go](../../../services/platform/perps-data/internal/gateway/server.go) |
| `go_exported` | `func (s *Server) Handler() http.Handler { return http.HandlerFunc(s.serve) }` | [services/platform/perps-data/internal/gateway/server.go](../../../services/platform/perps-data/internal/gateway/server.go) |
| `go_exported` | `func (s *Server) Ready() bool { return s.ready.Load() }` | [services/platform/perps-data/internal/gateway/server.go](../../../services/platform/perps-data/internal/gateway/server.go) |
| `go_exported` | `func (s *Server) Run(ctx context.Context) error` | [services/platform/perps-data/internal/gateway/server.go](../../../services/platform/perps-data/internal/gateway/server.go) |
| `http_route` | `GET /graphql-ws` | [services/platform/perps-data/internal/gateway/server.go](../../../services/platform/perps-data/internal/gateway/server.go) |
| `http_route` | `POST /api/subgraph` | [services/platform/perps-data/internal/gateway/server.go](../../../services/platform/perps-data/internal/gateway/server.go) |
| `http_route` | `POST /graphql` | [services/platform/perps-data/internal/gateway/server.go](../../../services/platform/perps-data/internal/gateway/server.go) |
| `http_route` | `POST /v2/auth/challenge` | [services/platform/perps-data/internal/gateway/server.go](../../../services/platform/perps-data/internal/gateway/server.go) |
| `http_route` | `POST /v2/auth/refresh` | [services/platform/perps-data/internal/gateway/server.go](../../../services/platform/perps-data/internal/gateway/server.go) |
| `http_route` | `POST /v2/auth/verify` | [services/platform/perps-data/internal/gateway/server.go](../../../services/platform/perps-data/internal/gateway/server.go) |

## graphql

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `func (f *Feed) Deliver(subject string, _ delivery.Envelope)` | [services/platform/perps-data/internal/graphql/subscriptions.go](../../../services/platform/perps-data/internal/graphql/subscriptions.go) |
| `go_exported` | `type Feed struct` | [services/platform/perps-data/internal/graphql/subscriptions.go](../../../services/platform/perps-data/internal/graphql/subscriptions.go) |
| `go_exported` | `func NewFeed(chain uint64, logger *slog.Logger) (*Feed, error)` | [services/platform/perps-data/internal/graphql/subscriptions.go](../../../services/platform/perps-data/internal/graphql/subscriptions.go) |
| `go_exported` | `func NewSchema(db *sql.DB) (*graphql.Schema, error)` | [services/platform/perps-data/internal/graphql/schema.go](../../../services/platform/perps-data/internal/graphql/schema.go) |
| `go_exported` | `type TransportWS struct` | [services/platform/perps-data/internal/graphql/transportws.go](../../../services/platform/perps-data/internal/graphql/transportws.go) |
| `go_exported` | `func (t *TransportWS) ServeHTTP(w http.ResponseWriter, r *http.Request)` | [services/platform/perps-data/internal/graphql/transportws.go](../../../services/platform/perps-data/internal/graphql/transportws.go) |
| `http_route` | `type Market {` | [services/platform/perps-data/internal/graphql/schema.go](../../../services/platform/perps-data/internal/graphql/schema.go) |

## indexer

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `type Deps struct` | [services/platform/perps-data/internal/indexer/indexer.go](../../../services/platform/perps-data/internal/indexer/indexer.go) |
| `go_exported` | `func (ix *Indexer) Checks() map[string]httpx.Check` | [services/platform/perps-data/internal/indexer/indexer.go](../../../services/platform/perps-data/internal/indexer/indexer.go) |
| `go_exported` | `func (ix *Indexer) Run(ctx context.Context) error` | [services/platform/perps-data/internal/indexer/indexer.go](../../../services/platform/perps-data/internal/indexer/indexer.go) |
| `go_exported` | `MaxOutboxAge = 30 * time.Second` | [services/platform/perps-data/internal/indexer/indexer.go](../../../services/platform/perps-data/internal/indexer/indexer.go) |
| `go_exported` | `func New(ctx context.Context, d Deps) (*Indexer, error)` | [services/platform/perps-data/internal/indexer/indexer.go](../../../services/platform/perps-data/internal/indexer/indexer.go) |
| `go_exported` | `type Affected struct` | [services/platform/perps-data/internal/source/follower.go](../../../services/platform/perps-data/internal/source/follower.go) |
| `go_exported` | `type Block struct` | [services/platform/perps-data/internal/source/follower.go](../../../services/platform/perps-data/internal/source/follower.go) |
| `go_exported` | `func (c *Comet) Header(ctx context.Context, height int64) (*CometHeader, error)` | [services/platform/perps-data/internal/source/comet.go](../../../services/platform/perps-data/internal/source/comet.go) |
| `go_exported` | `type Comet struct` | [services/platform/perps-data/internal/source/comet.go](../../../services/platform/perps-data/internal/source/comet.go) |
| `go_exported` | `func (f *Follower) Fetch(ctx context.Context, h uint64) (*Block, error)` | [services/platform/perps-data/internal/source/follower.go](../../../services/platform/perps-data/internal/source/follower.go) |
| `go_exported` | `type Follower struct` | [services/platform/perps-data/internal/source/follower.go](../../../services/platform/perps-data/internal/source/follower.go) |
| `go_exported` | `func (f *Follower) Run(ctx context.Context) error` | [services/platform/perps-data/internal/source/follower.go](../../../services/platform/perps-data/internal/source/follower.go) |
| `go_exported` | `func (f *Follower) Step(ctx context.Context) (int, error)` | [services/platform/perps-data/internal/source/follower.go](../../../services/platform/perps-data/internal/source/follower.go) |
| `go_exported` | `type FollowerOptions struct` | [services/platform/perps-data/internal/source/follower.go](../../../services/platform/perps-data/internal/source/follower.go) |
| `go_exported` | `const MaxLogRange = 500` | [services/platform/perps-data/internal/source/rpc.go](../../../services/platform/perps-data/internal/source/rpc.go) |
| `go_exported` | `func NewComet(urls []string, timeout time.Duration) (*Comet, error)` | [services/platform/perps-data/internal/source/comet.go](../../../services/platform/perps-data/internal/source/comet.go) |
| `go_exported` | `func NewFollower(ctx context.Context, opts FollowerOptions) (*Follower, error)` | [services/platform/perps-data/internal/source/follower.go](../../../services/platform/perps-data/internal/source/follower.go) |
| `go_exported` | `func NewRPC(ctx context.Context, opts RPCOptions) (*RPC, error)` | [services/platform/perps-data/internal/source/rpc.go](../../../services/platform/perps-data/internal/source/rpc.go) |
| `go_exported` | `func (s *RPC) BlockNumber(ctx context.Context) (uint64, error)` | [services/platform/perps-data/internal/source/rpc.go](../../../services/platform/perps-data/internal/source/rpc.go) |
| `go_exported` | `func (s *RPC) Call(ctx context.Context, to common.Address, data []byte) ([]byte, error)` | [services/platform/perps-data/internal/source/rpc.go](../../../services/platform/perps-data/internal/source/rpc.go) |
| `go_exported` | `func (s *RPC) CallAt(ctx context.Context, to common.Address, data []byte, height uint64) ([]byte, error)` | [services/platform/perps-data/internal/source/rpc.go](../../../services/platform/perps-data/internal/source/rpc.go) |
| `go_exported` | `func (s *RPC) CallContract(ctx context.Context, call ethereum.CallMsg, blockNumber *big.Int) ([]byte, error)` | [services/platform/perps-data/internal/source/rpc.go](../../../services/platform/perps-data/internal/source/rpc.go) |
| `go_exported` | `func (s *RPC) Close()` | [services/platform/perps-data/internal/source/rpc.go](../../../services/platform/perps-data/internal/source/rpc.go) |
| `go_exported` | `func (s *RPC) Head(ctx context.Context) (uint64, error)` | [services/platform/perps-data/internal/source/rpc.go](../../../services/platform/perps-data/internal/source/rpc.go) |
| `go_exported` | `type RPC struct` | [services/platform/perps-data/internal/source/rpc.go](../../../services/platform/perps-data/internal/source/rpc.go) |
| `go_exported` | `func (s *RPC) Transport() http.RoundTripper { return poolTransport{s} }` | [services/platform/perps-data/internal/source/rpc.go](../../../services/platform/perps-data/internal/source/rpc.go) |
| `go_exported` | `type RPCOptions struct` | [services/platform/perps-data/internal/source/rpc.go](../../../services/platform/perps-data/internal/source/rpc.go) |
| `go_exported` | `func SyntheticHash(tag string, height uint64) common.Hash` | [services/platform/perps-data/internal/source/follower.go](../../../services/platform/perps-data/internal/source/follower.go) |
| `go_exported` | `func (e *SyntheticMismatchError) Error() string` | [services/platform/perps-data/internal/source/follower.go](../../../services/platform/perps-data/internal/source/follower.go) |
| `go_exported` | `type SyntheticMismatchError struct` | [services/platform/perps-data/internal/source/follower.go](../../../services/platform/perps-data/internal/source/follower.go) |

## loadtest

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `func FeedSymbols(ctx context.Context, gateway string) ([]string, error) {` | [services/platform/perps-data/loadtest/load.go](../../../services/platform/perps-data/loadtest/load.go) |
| `go_exported` | `func FullProfile(symbols []string) Profile {` | [services/platform/perps-data/loadtest/load.go](../../../services/platform/perps-data/loadtest/load.go) |
| `go_exported` | `func (r *Report) Err() error` | [services/platform/perps-data/loadtest/load.go](../../../services/platform/perps-data/loadtest/load.go) |
| `go_exported` | `func Run(ctx context.Context, socketURL string, p Profile) (*Report, error)` | [services/platform/perps-data/loadtest/load.go](../../../services/platform/perps-data/loadtest/load.go) |
| `go_exported` | `func SocketURL(gateway string) (string, error)` | [services/platform/perps-data/loadtest/load.go](../../../services/platform/perps-data/loadtest/load.go) |

## markets

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `func Check(db *sql.DB, maxAge time.Duration) httpx.Check` | [services/platform/perps-data/internal/markets/markets.go](../../../services/platform/perps-data/internal/markets/markets.go) |
| `go_exported` | `func FeeConfig(ctx context.Context, db *sql.DB, client *ethclient.Client) error` | [services/platform/perps-data/internal/markets/markets.go](../../../services/platform/perps-data/internal/markets/markets.go) |
| `go_exported` | `func Refresh(ctx context.Context, db *sql.DB, client *ethclient.Client) error` | [services/platform/perps-data/internal/markets/markets.go](../../../services/platform/perps-data/internal/markets/markets.go) |
| `go_exported` | `func Run(ctx context.Context, db *sql.DB, client *ethclient.Client, interval time.Duration, metrics *telemetry.Metrics) error {` | [services/platform/perps-data/internal/markets/markets.go](../../../services/platform/perps-data/internal/markets/markets.go) |
| `go_exported` | `var Seed = []Market{` | [services/platform/perps-data/internal/markets/markets.go](../../../services/platform/perps-data/internal/markets/markets.go) |

## operations

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `type Check func(ctx context.Context) error` | [services/platform/perps-data/internal/httpx/health.go](../../../services/platform/perps-data/internal/httpx/health.go) |
| `go_exported` | `type HealthDeps struct` | [services/platform/perps-data/internal/httpx/health.go](../../../services/platform/perps-data/internal/httpx/health.go) |
| `go_exported` | `const ReadinessTimeout = 2 * time.Second` | [services/platform/perps-data/internal/httpx/health.go](../../../services/platform/perps-data/internal/httpx/health.go) |
| `go_exported` | `func RegisterHealth(mux *http.ServeMux, deps HealthDeps)` | [services/platform/perps-data/internal/httpx/health.go](../../../services/platform/perps-data/internal/httpx/health.go) |
| `go_exported` | `func ClientKey(r *http.Request, trusted []netip.Prefix) string {` | [services/platform/perps-data/internal/ratelimit/ratelimit.go](../../../services/platform/perps-data/internal/ratelimit/ratelimit.go) |
| `go_exported` | `func (l *Limiter) Middleware(next http.Handler) http.Handler` | [services/platform/perps-data/internal/ratelimit/ratelimit.go](../../../services/platform/perps-data/internal/ratelimit/ratelimit.go) |
| `go_exported` | `func NewLimiter(maxConcurrent int) *Limiter` | [services/platform/perps-data/internal/ratelimit/ratelimit.go](../../../services/platform/perps-data/internal/ratelimit/ratelimit.go) |
| `go_exported` | `func Report(log *slog.Logger, register func(prometheus.Collector) error, trusted []netip.Prefix) error {` | [services/platform/perps-data/internal/ratelimit/ratelimit.go](../../../services/platform/perps-data/internal/ratelimit/ratelimit.go) |
| `go_exported` | `type Metrics struct` | [services/platform/perps-data/internal/telemetry/metrics.go](../../../services/platform/perps-data/internal/telemetry/metrics.go) |
| `go_exported` | `func (m *Metrics) RecordRead(route string, status int) error` | [services/platform/perps-data/internal/telemetry/metrics.go](../../../services/platform/perps-data/internal/telemetry/metrics.go) |
| `go_exported` | `func (m *Metrics) Register(c prometheus.Collector) error` | [services/platform/perps-data/internal/telemetry/metrics.go](../../../services/platform/perps-data/internal/telemetry/metrics.go) |
| `go_exported` | `func (m *Metrics) ServeHTTP(w http.ResponseWriter, r *http.Request)` | [services/platform/perps-data/internal/telemetry/metrics.go](../../../services/platform/perps-data/internal/telemetry/metrics.go) |
| `go_exported` | `func (m *Metrics) WritePrometheus(w io.Writer) error` | [services/platform/perps-data/internal/telemetry/metrics.go](../../../services/platform/perps-data/internal/telemetry/metrics.go) |
| `go_exported` | `func New(service config.Service) (*Metrics, error)` | [services/platform/perps-data/internal/telemetry/metrics.go](../../../services/platform/perps-data/internal/telemetry/metrics.go) |
| `http_route` | `GET /healthz/live` | [services/platform/perps-data/internal/httpx/health.go](../../../services/platform/perps-data/internal/httpx/health.go) |
| `http_route` | `GET /healthz/ready` | [services/platform/perps-data/internal/httpx/health.go](../../../services/platform/perps-data/internal/httpx/health.go) |
| `metric` | `perps_gateway_frames_total` | [services/platform/perps-data/internal/gateway/server.go](../../../services/platform/perps-data/internal/gateway/server.go) |
| `metric` | `perps_gateway_graphql_duration_seconds` | [services/platform/perps-data/internal/gateway/server.go](../../../services/platform/perps-data/internal/gateway/server.go) |
| `metric` | `perps_gateway_resyncs_total` | [services/platform/perps-data/internal/gateway/server.go](../../../services/platform/perps-data/internal/gateway/server.go) |
| `metric` | `perps_gateway_sockets` | [services/platform/perps-data/internal/gateway/server.go](../../../services/platform/perps-data/internal/gateway/server.go) |
| `metric` | `perps_gateway_subscriptions` | [services/platform/perps-data/internal/gateway/server.go](../../../services/platform/perps-data/internal/gateway/server.go) |
| `metric` | `perps_http_requests_total` | [services/platform/perps-data/internal/telemetry/metrics.go](../../../services/platform/perps-data/internal/telemetry/metrics.go) |
| `metric` | `perps_indexer_cursor_height` | [services/platform/perps-data/internal/indexer/indexer.go](../../../services/platform/perps-data/internal/indexer/indexer.go) |
| `metric` | `perps_indexer_enum_unknown_total` | [services/platform/perps-data/internal/decode/registry.go](../../../services/platform/perps-data/internal/decode/registry.go) |
| `metric` | `perps_indexer_head_height` | [services/platform/perps-data/internal/indexer/indexer.go](../../../services/platform/perps-data/internal/indexer/indexer.go) |
| `metric` | `perps_indexer_lag_blocks` | [services/platform/perps-data/internal/indexer/indexer.go](../../../services/platform/perps-data/internal/indexer/indexer.go) |
| `metric` | `perps_indexer_logs_total` | [services/platform/perps-data/internal/indexer/indexer.go](../../../services/platform/perps-data/internal/indexer/indexer.go) |
| `metric` | `perps_indexer_rewinds_total` | [services/platform/perps-data/internal/indexer/indexer.go](../../../services/platform/perps-data/internal/indexer/indexer.go) |
| `metric` | `perps_indexer_unknown_logs_total` | [services/platform/perps-data/internal/decode/registry.go](../../../services/platform/perps-data/internal/decode/registry.go) |
| `metric` | `perps_outbox_broker_down` | [services/platform/perps-data/internal/publish/publisher.go](../../../services/platform/perps-data/internal/publish/publisher.go) |
| `metric` | `perps_outbox_head_age_seconds` | [services/platform/perps-data/internal/publish/publisher.go](../../../services/platform/perps-data/internal/publish/publisher.go) |
| `metric` | `perps_outbox_pending` | [services/platform/perps-data/internal/indexer/indexer.go](../../../services/platform/perps-data/internal/indexer/indexer.go) |
| `metric` | `perps_outbox_quarantined_total` | [services/platform/perps-data/internal/publish/publisher.go](../../../services/platform/perps-data/internal/publish/publisher.go) |
| `metric` | `perps_portal_render_seconds` | [services/platform/perps-data/internal/portal/server.go](../../../services/platform/perps-data/internal/portal/server.go) |
| `metric` | `perps_untrusted_proxy_headers_total` | [services/platform/perps-data/internal/ratelimit/ratelimit.go](../../../services/platform/perps-data/internal/ratelimit/ratelimit.go) |
| source | — | [services/platform/perps-data/cmd/perps-explorer/main.go](../../../services/platform/perps-data/cmd/perps-explorer/main.go) |
| source | — | [services/platform/perps-data/cmd/perps-indexer/main.go](../../../services/platform/perps-data/cmd/perps-indexer/main.go) |

## pnl

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `func Add(a, b string) (string, error)` | [services/platform/perps-data/internal/bigint/bigint.go](../../../services/platform/perps-data/internal/bigint/bigint.go) |
| `go_exported` | `ErrInvalidRaw = errors.New("bigint: invalid raw integer")` | [services/platform/perps-data/internal/bigint/bigint.go](../../../services/platform/perps-data/internal/bigint/bigint.go) |
| `go_exported` | `ErrOverflow = errors.New("bigint: 256-bit overflow")` | [services/platform/perps-data/internal/bigint/bigint.go](../../../services/platform/perps-data/internal/bigint/bigint.go) |
| `go_exported` | `Int256Max = "57896044618658097711785492504343953926634992332820282019728792003956564819967"` | [services/platform/perps-data/internal/bigint/bigint.go](../../../services/platform/perps-data/internal/bigint/bigint.go) |
| `go_exported` | `Int256Min = "-57896044618658097711785492504343953926634992332820282019728792003956564819968"` | [services/platform/perps-data/internal/bigint/bigint.go](../../../services/platform/perps-data/internal/bigint/bigint.go) |
| `go_exported` | `func MulDiv(a, b, divisor string) (string, error)` | [services/platform/perps-data/internal/bigint/bigint.go](../../../services/platform/perps-data/internal/bigint/bigint.go) |
| `go_exported` | `func Sub(a, b string) (string, error)` | [services/platform/perps-data/internal/bigint/bigint.go](../../../services/platform/perps-data/internal/bigint/bigint.go) |
| `go_exported` | `const Uint256Max = "115792089237316195423570985008687907853269984665640564039457584007913129639935"` | [services/platform/perps-data/internal/bigint/bigint.go](../../../services/platform/perps-data/internal/bigint/bigint.go) |
| `go_exported` | `func ValidateRaw(raw string) error` | [services/platform/perps-data/internal/bigint/bigint.go](../../../services/platform/perps-data/internal/bigint/bigint.go) |
| `go_exported` | `func ValidateRawInt256(raw string) error` | [services/platform/perps-data/internal/bigint/bigint.go](../../../services/platform/perps-data/internal/bigint/bigint.go) |
| `go_exported` | `type Entry struct` | [services/platform/perps-data/internal/pnl/pnl.go](../../../services/platform/perps-data/internal/pnl/pnl.go) |
| `go_exported` | `func Fold(prior Position, t Trade) (Position, error)` | [services/platform/perps-data/internal/pnl/pnl.go](../../../services/platform/perps-data/internal/pnl/pnl.go) |
| `go_exported` | `func History(ctx context.Context, q Querier, account common.Address, from int64, below Key, kinds []Kind, limit int) ([]Entry, error)` | [services/platform/perps-data/internal/pnl/pnl.go](../../../services/platform/perps-data/internal/pnl/pnl.go) |
| `go_exported` | `type Key struct` | [services/platform/perps-data/internal/pnl/pnl.go](../../../services/platform/perps-data/internal/pnl/pnl.go) |
| `go_exported` | `type Kind string` | [services/platform/perps-data/internal/pnl/pnl.go](../../../services/platform/perps-data/internal/pnl/pnl.go) |
| `go_exported` | `type Leaderboard struct{}` | [services/platform/perps-data/internal/pnl/pnl.go](../../../services/platform/perps-data/internal/pnl/pnl.go) |
| `go_exported` | `type Position struct` | [services/platform/perps-data/internal/pnl/pnl.go](../../../services/platform/perps-data/internal/pnl/pnl.go) |
| `go_exported` | `func RebuildDaily(ctx context.Context, tx *sql.Tx, account common.Address, day time.Time) (Daily, error)` | [services/platform/perps-data/internal/pnl/pnl.go](../../../services/platform/perps-data/internal/pnl/pnl.go) |
| `go_exported` | `func Through(block int64) Key` | [services/platform/perps-data/internal/pnl/pnl.go](../../../services/platform/perps-data/internal/pnl/pnl.go) |
| `go_exported` | `func TotalPnl(realizedPnlUsdl, avgCostBasis, holdings, priceWad string) (string, error)` | [services/platform/perps-data/internal/pnl/pnl.go](../../../services/platform/perps-data/internal/pnl/pnl.go) |
| `go_exported` | `type Trade struct` | [services/platform/perps-data/internal/pnl/pnl.go](../../../services/platform/perps-data/internal/pnl/pnl.go) |

## portal

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `func NewPages(gatewayURL, explorerTxBase string, client *http.Client) (*Pages, error)` | [services/platform/perps-data/internal/portal/pages.go](../../../services/platform/perps-data/internal/portal/pages.go) |
| `go_exported` | `func NewServer(pages *Pages, gatewayURL string, metrics *telemetry.Metrics, client *http.Client) (*Server, error)` | [services/platform/perps-data/internal/portal/server.go](../../../services/platform/perps-data/internal/portal/server.go) |
| `go_exported` | `func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request)` | [services/platform/perps-data/internal/portal/server.go](../../../services/platform/perps-data/internal/portal/server.go) |

## records

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `func Decimal(raw sql.NullString) (*float64, error)` | [services/platform/perps-data/internal/records/records.go](../../../services/platform/perps-data/internal/records/records.go) |
| `go_exported` | `func DecimalOrZero(raw sql.NullString) (float64, error)` | [services/platform/perps-data/internal/records/records.go](../../../services/platform/perps-data/internal/records/records.go) |
| `go_exported` | `Funding8hSQL = "trim_scale(funding_rate * 0.000000000000000008)::text"` | [services/platform/perps-data/internal/records/records.go](../../../services/platform/perps-data/internal/records/records.go) |
| `go_exported` | `ISOMillis = "2006-01-02T15:04:05.000Z"` | [services/platform/perps-data/internal/records/records.go](../../../services/platform/perps-data/internal/records/records.go) |
| `go_exported` | `LastFundingRateHourlySQL = "trim_scale(funding_rate * 0.000000000000000001)::text"` | [services/platform/perps-data/internal/records/records.go](../../../services/platform/perps-data/internal/records/records.go) |
| `go_exported` | ``const LiquidationFill = `EXISTS (SELECT 1 FROM perps.liquidations l`` | [services/platform/perps-data/internal/records/records.go](../../../services/platform/perps-data/internal/records/records.go) |
| `go_exported` | `type MarketRecord struct` | [services/platform/perps-data/internal/records/records.go](../../../services/platform/perps-data/internal/records/records.go) |
| `go_exported` | `func NewMarketSet(db *sql.DB, ttl time.Duration) *MarketSet {` | [services/platform/perps-data/internal/records/marketset.go](../../../services/platform/perps-data/internal/records/marketset.go) |
| `go_exported` | `func NewSymbol(db *sql.DB) (*Symbol, error)` | [services/platform/perps-data/internal/records/records.go](../../../services/platform/perps-data/internal/records/records.go) |
| `go_exported` | `type PerpTrade struct` | [services/platform/perps-data/internal/records/records.go](../../../services/platform/perps-data/internal/records/records.go) |
| `go_exported` | `SecondsPerHour = 3600` | [services/platform/perps-data/internal/records/records.go](../../../services/platform/perps-data/internal/records/records.go) |
| `go_exported` | `SpotScale = "1000000"` | [services/platform/perps-data/internal/records/records.go](../../../services/platform/perps-data/internal/records/records.go) |
| `go_exported` | `var SpotTokenRank = map[string]int{"PAX": 0, "SID": 1}` | [services/platform/perps-data/internal/records/records.go](../../../services/platform/perps-data/internal/records/records.go) |
| `go_exported` | `func (s *Symbol) Health(ctx context.Context, symbol string) (Health, error)` | [services/platform/perps-data/internal/records/records.go](../../../services/platform/perps-data/internal/records/records.go) |
| `go_exported` | `type Symbol struct` | [services/platform/perps-data/internal/records/records.go](../../../services/platform/perps-data/internal/records/records.go) |
| `go_exported` | `func (s *Symbol) PerpStats(ctx context.Context, symbol string) (PerpStats, error)` | [services/platform/perps-data/internal/records/records.go](../../../services/platform/perps-data/internal/records/records.go) |
| `go_exported` | `func (s *Symbol) PerpTrades(ctx context.Context, symbol string, limit int) (Trades[PerpTrade], error)` | [services/platform/perps-data/internal/records/records.go](../../../services/platform/perps-data/internal/records/records.go) |
| `go_exported` | `func (s *Symbol) Price(ctx context.Context, symbol string) (Price, error)` | [services/platform/perps-data/internal/records/records.go](../../../services/platform/perps-data/internal/records/records.go) |
| `go_exported` | `func (s *Symbol) Stats(ctx context.Context, symbol string) (Stats, error)` | [services/platform/perps-data/internal/records/records.go](../../../services/platform/perps-data/internal/records/records.go) |
| `go_exported` | `func (s *Symbol) Trades(ctx context.Context, symbol string, limit int) (Trades[SpotTrade], error)` | [services/platform/perps-data/internal/records/records.go](../../../services/platform/perps-data/internal/records/records.go) |
| `go_exported` | `type Totals struct` | [services/platform/perps-data/internal/records/records.go](../../../services/platform/perps-data/internal/records/records.go) |
| `go_exported` | `type Trades[T any] struct` | [services/platform/perps-data/internal/records/records.go](../../../services/platform/perps-data/internal/records/records.go) |
| `go_exported` | `USDScale = "1000000000000000000"` | [services/platform/perps-data/internal/records/records.go](../../../services/platform/perps-data/internal/records/records.go) |
| `go_exported` | `Window24h = 24 * time.Hour` | [services/platform/perps-data/internal/records/records.go](../../../services/platform/perps-data/internal/records/records.go) |

## replay

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `func Exit(err error) int` | [services/platform/perps-data/internal/replay/replay.go](../../../services/platform/perps-data/internal/replay/replay.go) |
| `go_exported` | `type NodeSource struct` | [services/platform/perps-data/internal/replay/replay.go](../../../services/platform/perps-data/internal/replay/replay.go) |
| `go_exported` | `func (r Refusal) Error() string { return string(r) }` | [services/platform/perps-data/internal/replay/replay.go](../../../services/platform/perps-data/internal/replay/replay.go) |
| `go_exported` | `type Refusal string` | [services/platform/perps-data/internal/replay/replay.go](../../../services/platform/perps-data/internal/replay/replay.go) |
| `go_exported` | `type Report struct` | [services/platform/perps-data/internal/replay/replay.go](../../../services/platform/perps-data/internal/replay/replay.go) |
| `go_exported` | `type Request struct` | [services/platform/perps-data/internal/replay/replay.go](../../../services/platform/perps-data/internal/replay/replay.go) |
| `go_exported` | `type Runner struct` | [services/platform/perps-data/internal/replay/replay.go](../../../services/platform/perps-data/internal/replay/replay.go) |
| `go_exported` | `func (r *Runner) Run(ctx context.Context, req Request) (rep Report, err error)` | [services/platform/perps-data/internal/replay/replay.go](../../../services/platform/perps-data/internal/replay/replay.go) |
| `go_exported` | `const ToCursor int64 = -1` | [services/platform/perps-data/internal/replay/replay.go](../../../services/platform/perps-data/internal/replay/replay.go) |

## rest

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `func (s *AccountV2) Register(mux *http.ServeMux)` | [services/platform/perps-data/internal/rest/accountv2.go](../../../services/platform/perps-data/internal/rest/accountv2.go) |
| `go_exported` | `func (m *Markets) Register(mux *http.ServeMux)` | [services/platform/perps-data/internal/rest/markets.go](../../../services/platform/perps-data/internal/rest/markets.go) |
| `go_exported` | `func NewAccountV2(db *sql.DB, service *auth.Service, now func() time.Time) (*AccountV2, error)` | [services/platform/perps-data/internal/rest/accountv2.go](../../../services/platform/perps-data/internal/rest/accountv2.go) |
| `go_exported` | `func NewMarkets(db *sql.DB, now func() time.Time) (*Markets, error)` | [services/platform/perps-data/internal/rest/markets.go](../../../services/platform/perps-data/internal/rest/markets.go) |
| `go_exported` | `func NewSpotUsers(db *sql.DB, node SpotNode) (*SpotUsers, error)` | [services/platform/perps-data/internal/rest/spotusers.go](../../../services/platform/perps-data/internal/rest/spotusers.go) |
| `go_exported` | `func NewSymbol(db *sql.DB) (*Symbol, error)` | [services/platform/perps-data/internal/rest/symbol.go](../../../services/platform/perps-data/internal/rest/symbol.go) |
| `go_exported` | `func NewVenue(db *sql.DB) (*Venue, error)` | [services/platform/perps-data/internal/rest/venue.go](../../../services/platform/perps-data/internal/rest/venue.go) |
| `go_exported` | `type SpotNode interface` | [services/platform/perps-data/internal/rest/spotusers.go](../../../services/platform/perps-data/internal/rest/spotusers.go) |
| `go_exported` | `type SpotStreamItem struct` | [services/platform/perps-data/internal/rest/spotusers.go](../../../services/platform/perps-data/internal/rest/spotusers.go) |
| `go_exported` | `type SpotUsers struct` | [services/platform/perps-data/internal/rest/spotusers.go](../../../services/platform/perps-data/internal/rest/spotusers.go) |
| `go_exported` | `func (s *SpotUsers) Register(mux *http.ServeMux)` | [services/platform/perps-data/internal/rest/spotusers.go](../../../services/platform/perps-data/internal/rest/spotusers.go) |
| `go_exported` | `func (s *SpotUsers) StreamItems(ctx context.Context, env delivery.Envelope) ([]SpotStreamItem, error)` | [services/platform/perps-data/internal/rest/spotusers.go](../../../services/platform/perps-data/internal/rest/spotusers.go) |
| `go_exported` | `func (s *Symbol) Register(mux *http.ServeMux)` | [services/platform/perps-data/internal/rest/symbol.go](../../../services/platform/perps-data/internal/rest/symbol.go) |
| `go_exported` | `func (v *Venue) Register(mux *http.ServeMux)` | [services/platform/perps-data/internal/rest/venue.go](../../../services/platform/perps-data/internal/rest/venue.go) |
| `http_route` | `GET /api/markets/health` | [services/platform/perps-data/internal/rest/markets.go](../../../services/platform/perps-data/internal/rest/markets.go) |
| `http_route` | `GET /api/markets` | [services/platform/perps-data/internal/rest/markets.go](../../../services/platform/perps-data/internal/rest/markets.go) |
| `http_route` | `GET /api/markets/{symbol}` | [services/platform/perps-data/internal/rest/markets.go](../../../services/platform/perps-data/internal/rest/markets.go) |
| `http_route` | `GET /api/totals` | [services/platform/perps-data/internal/rest/markets.go](../../../services/platform/perps-data/internal/rest/markets.go) |
| `http_route` | `GET /v2/leaderboard` | [services/platform/perps-data/internal/rest/accountv2.go](../../../services/platform/perps-data/internal/rest/accountv2.go) |
| `http_route` | `GET /v2/user/{address}/daily-pnl` | [services/platform/perps-data/internal/rest/accountv2.go](../../../services/platform/perps-data/internal/rest/accountv2.go) |

## storage

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `var FS embed.FS` | [services/platform/perps-data/migrations/embed.go](../../../services/platform/perps-data/migrations/embed.go) |
| `go_exported` | `type Block struct` | [services/platform/perps-data/internal/store/facts.go](../../../services/platform/perps-data/internal/store/facts.go) |
| `go_exported` | `func Cursor(ctx context.Context, db *sql.DB) (uint64, bool, error)` | [services/platform/perps-data/internal/store/facts.go](../../../services/platform/perps-data/internal/store/facts.go) |
| `go_exported` | `var ErrLeaseHeld = errors.New("store: writer lease held by another instance")` | [services/platform/perps-data/internal/store/facts.go](../../../services/platform/perps-data/internal/store/facts.go) |
| `go_exported` | `func Fence(ctx context.Context, tx *sql.Tx, name, holder string) error` | [services/platform/perps-data/internal/store/facts.go](../../../services/platform/perps-data/internal/store/facts.go) |
| `go_exported` | `func FoldSpotBoard(ctx context.Context, tx *sql.Tx, rows []decode.Row) error` | [services/platform/perps-data/internal/store/spotboard.go](../../../services/platform/perps-data/internal/store/spotboard.go) |
| `go_exported` | `func Migrate(ctx context.Context, db *sql.DB) error` | [services/platform/perps-data/internal/store/migrate.go](../../../services/platform/perps-data/internal/store/migrate.go) |
| `go_exported` | `func Open(databaseURL string) (*sql.DB, error)` | [services/platform/perps-data/internal/store/migrate.go](../../../services/platform/perps-data/internal/store/migrate.go) |
| `go_exported` | `func Policy(chain uint64) delivery.Policy` | [services/platform/perps-data/internal/store/facts.go](../../../services/platform/perps-data/internal/store/facts.go) |
| `go_exported` | `func Project(ctx context.Context, tx *sql.Tx, rows []decode.Row) error` | [services/platform/perps-data/internal/store/projections.go](../../../services/platform/perps-data/internal/store/projections.go) |
| `go_exported` | `func Rebuild(ctx context.Context, tx *sql.Tx, from, to int64, recovery ...Recovery) error {` | [services/platform/perps-data/internal/store/facts.go](../../../services/platform/perps-data/internal/store/facts.go) |
| `go_exported` | `type Snapshotter struct{}` | [services/platform/perps-data/internal/store/projections.go](../../../services/platform/perps-data/internal/store/projections.go) |
| `go_exported` | `var USDL = common.HexToAddress("0x85FcD13735F4309833A503EE804ea32395851479")` | [services/platform/perps-data/internal/store/projections.go](../../../services/platform/perps-data/internal/store/projections.go) |
| `go_exported` | `func WriteBlock(ctx context.Context, tx *sql.Tx, block Block, rows *decode.Result) error` | [services/platform/perps-data/internal/store/facts.go](../../../services/platform/perps-data/internal/store/facts.go) |
| `go_exported` | `func WriteFacts(ctx context.Context, tx *sql.Tx, block Block, rows *decode.Result) ([]decode.Row, error)` | [services/platform/perps-data/internal/store/facts.go](../../../services/platform/perps-data/internal/store/facts.go) |
| `storage_key` | `CREATE VIEW perps.account_ledger AS` | [services/platform/perps-data/migrations/0002_projections.sql](../../../services/platform/perps-data/migrations/0002_projections.sql) |
| `storage_key` | `CREATE TABLE perps.account_margin` | [services/platform/perps-data/migrations/0002_projections.sql](../../../services/platform/perps-data/migrations/0002_projections.sql) |
| `storage_key` | `CREATE TABLE perps.adl_events` | [services/platform/perps-data/migrations/0001_facts.sql](../../../services/platform/perps-data/migrations/0001_facts.sql) |
| `storage_key` | `CREATE TABLE perps.auth_challenges` | [services/platform/perps-data/migrations/0004_accounts.sql](../../../services/platform/perps-data/migrations/0004_accounts.sql) |
| `storage_key` | `CREATE TABLE perps.backstop_liquidations` | [services/platform/perps-data/migrations/0001_facts.sql](../../../services/platform/perps-data/migrations/0001_facts.sql) |
| `storage_key` | `CREATE TABLE perps.blocks` | [services/platform/perps-data/migrations/0001_facts.sql](../../../services/platform/perps-data/migrations/0001_facts.sql) |
| `storage_key` | `CREATE TABLE perps.collateral_events` | [services/platform/perps-data/migrations/0001_facts.sql](../../../services/platform/perps-data/migrations/0001_facts.sql) |
| `storage_key` | `CREATE TABLE perps.competition_entries` | [services/platform/perps-data/migrations/0004_accounts.sql](../../../services/platform/perps-data/migrations/0004_accounts.sql) |
| `storage_key` | `CREATE TABLE perps.competitions` | [services/platform/perps-data/migrations/0004_accounts.sql](../../../services/platform/perps-data/migrations/0004_accounts.sql) |
| `storage_key` | `CREATE TABLE perps.cursors` | [services/platform/perps-data/migrations/0001_facts.sql](../../../services/platform/perps-data/migrations/0001_facts.sql) |
| `storage_key` | `CREATE TABLE perps.delivery_inbox` | [services/platform/perps-data/migrations/0003_delivery.sql](../../../services/platform/perps-data/migrations/0003_delivery.sql) |
| `storage_key` | `CREATE TABLE perps.delivery_outbox` | [services/platform/perps-data/migrations/0003_delivery.sql](../../../services/platform/perps-data/migrations/0003_delivery.sql) |
| `storage_key` | `CREATE TABLE perps.delivery_quarantine` | [services/platform/perps-data/migrations/0003_delivery.sql](../../../services/platform/perps-data/migrations/0003_delivery.sql) |
| `storage_key` | `CREATE TABLE perps.fee_config` | [services/platform/perps-data/migrations/0001_facts.sql](../../../services/platform/perps-data/migrations/0001_facts.sql) |
| `storage_key` | `CREATE TABLE perps.feed_keys` | [services/platform/perps-data/migrations/0001_facts.sql](../../../services/platform/perps-data/migrations/0001_facts.sql) |
| `storage_key` | `CREATE TABLE perps.fees` | [services/platform/perps-data/migrations/0001_facts.sql](../../../services/platform/perps-data/migrations/0001_facts.sql) |
| `storage_key` | `CREATE TABLE perps.fills` | [services/platform/perps-data/migrations/0001_facts.sql](../../../services/platform/perps-data/migrations/0001_facts.sql) |
| `storage_key` | `CREATE TABLE perps.funding_payments` | [services/platform/perps-data/migrations/0001_facts.sql](../../../services/platform/perps-data/migrations/0001_facts.sql) |
| `storage_key` | `CREATE TABLE perps.funding_rates` | [services/platform/perps-data/migrations/0001_facts.sql](../../../services/platform/perps-data/migrations/0001_facts.sql) |
| `storage_key` | `CREATE TABLE perps.funding_settlements` | [services/platform/perps-data/migrations/0001_facts.sql](../../../services/platform/perps-data/migrations/0001_facts.sql) |
| `storage_key` | `CREATE TABLE perps.index_prices` | [services/platform/perps-data/migrations/0001_facts.sql](../../../services/platform/perps-data/migrations/0001_facts.sql) |
| `storage_key` | `CREATE TABLE perps.insurance_events` | [services/platform/perps-data/migrations/0001_facts.sql](../../../services/platform/perps-data/migrations/0001_facts.sql) |
| `storage_key` | `CREATE TABLE perps.keeper_cycles` | [services/platform/perps-data/migrations/0002_projections.sql](../../../services/platform/perps-data/migrations/0002_projections.sql) |
| `storage_key` | `CREATE TABLE perps.leaderboard` | [services/platform/perps-data/migrations/0002_projections.sql](../../../services/platform/perps-data/migrations/0002_projections.sql) |
| `storage_key` | `CREATE TABLE perps.liquidations` | [services/platform/perps-data/migrations/0001_facts.sql](../../../services/platform/perps-data/migrations/0001_facts.sql) |
| `storage_key` | `CREATE TABLE perps.mark_prices` | [services/platform/perps-data/migrations/0001_facts.sql](../../../services/platform/perps-data/migrations/0001_facts.sql) |
| `storage_key` | `CREATE TABLE perps.market_snapshots` | [services/platform/perps-data/migrations/0002_projections.sql](../../../services/platform/perps-data/migrations/0002_projections.sql) |
| `storage_key` | `CREATE TABLE perps.market_stats_rolling` | [services/platform/perps-data/migrations/0002_projections.sql](../../../services/platform/perps-data/migrations/0002_projections.sql) |
| `storage_key` | `CREATE TABLE perps.markets` | [services/platform/perps-data/migrations/0001_facts.sql](../../../services/platform/perps-data/migrations/0001_facts.sql) |
| `storage_key` | `CREATE TABLE perps.open_interest` | [services/platform/perps-data/migrations/0001_facts.sql](../../../services/platform/perps-data/migrations/0001_facts.sql) |
| `storage_key` | `CREATE TABLE perps.order_events` | [services/platform/perps-data/migrations/0001_facts.sql](../../../services/platform/perps-data/migrations/0001_facts.sql) |
| `storage_key` | `CREATE TABLE perps.orders` | [services/platform/perps-data/migrations/0002_projections.sql](../../../services/platform/perps-data/migrations/0002_projections.sql) |
| `storage_key` | `CREATE TABLE perps.position_events` | [services/platform/perps-data/migrations/0001_facts.sql](../../../services/platform/perps-data/migrations/0001_facts.sql) |
| `storage_key` | `CREATE TABLE perps.positions_current` | [services/platform/perps-data/migrations/0002_projections.sql](../../../services/platform/perps-data/migrations/0002_projections.sql) |
| `storage_key` | `CREATE TABLE perps.projection_seeds` | [services/platform/perps-data/migrations/0011_projection_seeds.sql](../../../services/platform/perps-data/migrations/0011_projection_seeds.sql) |
| `storage_key` | `CREATE TABLE perps.protocol_events` | [services/platform/perps-data/migrations/0001_facts.sql](../../../services/platform/perps-data/migrations/0001_facts.sql) |
| `storage_key` | `CREATE TABLE perps.protocol_snapshots` | [services/platform/perps-data/migrations/0002_projections.sql](../../../services/platform/perps-data/migrations/0002_projections.sql) |
| `storage_key` | `CREATE TABLE perps.raw_logs` | [services/platform/perps-data/migrations/0001_facts.sql](../../../services/platform/perps-data/migrations/0001_facts.sql) |
| `storage_key` | `CREATE TABLE perps.referral_tiers` | [services/platform/perps-data/migrations/0004_accounts.sql](../../../services/platform/perps-data/migrations/0004_accounts.sql) |
| `storage_key` | `CREATE TABLE perps.referrals` | [services/platform/perps-data/migrations/0004_accounts.sql](../../../services/platform/perps-data/migrations/0004_accounts.sql) |
| `storage_key` | `CREATE TABLE perps.spot_admin` | [services/platform/perps-data/migrations/0001_facts.sql](../../../services/platform/perps-data/migrations/0001_facts.sql) |
| `storage_key` | `CREATE TABLE perps.spot_fee_events` | [services/platform/perps-data/migrations/0001_facts.sql](../../../services/platform/perps-data/migrations/0001_facts.sql) |
| `storage_key` | `CREATE TABLE perps.spot_order_events` | [services/platform/perps-data/migrations/0001_facts.sql](../../../services/platform/perps-data/migrations/0001_facts.sql) |
| `storage_key` | `CREATE TABLE perps.spot_orders` | [services/platform/perps-data/migrations/0002_projections.sql](../../../services/platform/perps-data/migrations/0002_projections.sql) |
| `storage_key` | `CREATE TABLE perps.spot_share_transfers` | [services/platform/perps-data/migrations/0001_facts.sql](../../../services/platform/perps-data/migrations/0001_facts.sql) |
| `storage_key` | `CREATE TABLE perps.spot_swaps` | [services/platform/perps-data/migrations/0001_facts.sql](../../../services/platform/perps-data/migrations/0001_facts.sql) |
| `storage_key` | `CREATE TABLE perps.spot_token_registry` | [services/platform/perps-data/migrations/0001_facts.sql](../../../services/platform/perps-data/migrations/0001_facts.sql) |
| `storage_key` | `CREATE TABLE perps.spot_token_seen` | [services/platform/perps-data/migrations/0002_projections.sql](../../../services/platform/perps-data/migrations/0002_projections.sql) |
| `storage_key` | `CREATE TABLE perps.spot_vault_events` | [services/platform/perps-data/migrations/0001_facts.sql](../../../services/platform/perps-data/migrations/0001_facts.sql) |
| `storage_key` | `CREATE TABLE perps.trade_settlements` | [services/platform/perps-data/migrations/0001_facts.sql](../../../services/platform/perps-data/migrations/0001_facts.sql) |
| `storage_key` | `CREATE TABLE perps.tx_actions` | [services/platform/perps-data/migrations/0001_facts.sql](../../../services/platform/perps-data/migrations/0001_facts.sql) |
| `storage_key` | `CREATE TABLE perps.usdl_transfers` | [services/platform/perps-data/migrations/0001_facts.sql](../../../services/platform/perps-data/migrations/0001_facts.sql) |
| `storage_key` | `CREATE TABLE perps.user_alerts` | [services/platform/perps-data/migrations/0004_accounts.sql](../../../services/platform/perps-data/migrations/0004_accounts.sql) |
| `storage_key` | `CREATE TABLE perps.user_pnl_daily` | [services/platform/perps-data/migrations/0002_projections.sql](../../../services/platform/perps-data/migrations/0002_projections.sql) |
| `storage_key` | `CREATE TABLE perps.user_stats` | [services/platform/perps-data/migrations/0002_projections.sql](../../../services/platform/perps-data/migrations/0002_projections.sql) |
| `storage_key` | `CREATE TABLE perps.vaults` | [services/platform/perps-data/migrations/0001_facts.sql](../../../services/platform/perps-data/migrations/0001_facts.sql) |
| `storage_key` | `CREATE TABLE perps.writer_lease` | [services/platform/perps-data/migrations/0001_facts.sql](../../../services/platform/perps-data/migrations/0001_facts.sql) |
| source | — | [services/platform/perps-data/migrations/0013_chain_counters.sql](../../../services/platform/perps-data/migrations/0013_chain_counters.sql) |
| source | — | [services/platform/perps-data/migrations/0014_market_tiers.sql](../../../services/platform/perps-data/migrations/0014_market_tiers.sql) |
| source | — | [services/platform/perps-data/migrations/0015_spot_leaderboard.sql](../../../services/platform/perps-data/migrations/0015_spot_leaderboard.sql) |

## websocket

| Kind | Declaration | Source |
|---|---|---|
| `error_code` | `invalid_json` | [services/platform/perps-data/internal/ws/protocol.go](../../../services/platform/perps-data/internal/ws/protocol.go) |
| `error_code` | `missing_topic` | [services/platform/perps-data/internal/ws/protocol.go](../../../services/platform/perps-data/internal/ws/protocol.go) |
| `error_code` | `subscribe_rate_limit` | [services/platform/perps-data/internal/ws/protocol.go](../../../services/platform/perps-data/internal/ws/protocol.go) |
| `error_code` | `too_many_subscriptions` | [services/platform/perps-data/internal/ws/protocol.go](../../../services/platform/perps-data/internal/ws/protocol.go) |
| `error_code` | `unknown_event` | [services/platform/perps-data/internal/ws/protocol.go](../../../services/platform/perps-data/internal/ws/protocol.go) |
| `error_code` | `unknown_topic` | [services/platform/perps-data/internal/ws/protocol.go](../../../services/platform/perps-data/internal/ws/protocol.go) |
| `go_exported` | `func (b *Book) NotionalLevels(depth int) (asks, bids []Level)` | [services/platform/perps-data/internal/feeds/book.go](../../../services/platform/perps-data/internal/feeds/book.go) |
| `go_exported` | `ChecksumEvery  = 5 * time.Second` | [services/platform/perps-data/internal/feeds/topics.go](../../../services/platform/perps-data/internal/feeds/topics.go) |
| `go_exported` | `type Frame struct` | [services/platform/perps-data/internal/feeds/topics.go](../../../services/platform/perps-data/internal/feeds/topics.go) |
| `go_exported` | `type Level [2]float64` | [services/platform/perps-data/internal/feeds/book.go](../../../services/platform/perps-data/internal/feeds/book.go) |
| `go_exported` | `LiveLevels = 20` | [services/platform/perps-data/internal/feeds/book.go](../../../services/platform/perps-data/internal/feeds/book.go) |
| `go_exported` | `func LoadBook(ctx context.Context, db *sql.DB, market string) (*Book, error)` | [services/platform/perps-data/internal/feeds/book.go](../../../services/platform/perps-data/internal/feeds/book.go) |
| `go_exported` | `func NewProviders(ctx context.Context, db *sql.DB, logger *slog.Logger, metrics ...*telemetry.Metrics) (*Providers, error) {` | [services/platform/perps-data/internal/feeds/topics.go](../../../services/platform/perps-data/internal/feeds/topics.go) |
| `go_exported` | ``const OIContractsSQL = `(SELECT COALESCE(sum(p.size_usd / p.entry_price), 0) FROM perps.positions_current p`` | [services/platform/perps-data/internal/feeds/topics.go](../../../services/platform/perps-data/internal/feeds/topics.go) |
| `go_exported` | `func (p *Providers) Held(topic string) (json.RawMessage, bool)` | [services/platform/perps-data/internal/feeds/topics.go](../../../services/platform/perps-data/internal/feeds/topics.go) |
| `go_exported` | `type Providers struct` | [services/platform/perps-data/internal/feeds/topics.go](../../../services/platform/perps-data/internal/feeds/topics.go) |
| `go_exported` | `func (p *Providers) Load(ctx context.Context, topic string) error` | [services/platform/perps-data/internal/feeds/topics.go](../../../services/platform/perps-data/internal/feeds/topics.go) |
| `go_exported` | `func (p *Providers) Message(ctx context.Context, subject string, env delivery.Envelope) ([]Frame, []string, error)` | [services/platform/perps-data/internal/feeds/topics.go](../../../services/platform/perps-data/internal/feeds/topics.go) |
| `go_exported` | `func (p *Providers) Run(ctx context.Context, emit func(Frame)) error` | [services/platform/perps-data/internal/feeds/topics.go](../../../services/platform/perps-data/internal/feeds/topics.go) |
| `go_exported` | `func (p *Providers) Snapshot(ctx context.Context, topic string) (json.RawMessage, error)` | [services/platform/perps-data/internal/feeds/topics.go](../../../services/platform/perps-data/internal/feeds/topics.go) |
| `go_exported` | `func (p *Providers) SocketTopics(path string) ([]string, bool)` | [services/platform/perps-data/internal/feeds/topics.go](../../../services/platform/perps-data/internal/feeds/topics.go) |
| `go_exported` | `func (p *Providers) Subscribed(topic string)` | [services/platform/perps-data/internal/feeds/topics.go](../../../services/platform/perps-data/internal/feeds/topics.go) |
| `go_exported` | `func (p *Providers) Unsubscribed(topic string)` | [services/platform/perps-data/internal/feeds/topics.go](../../../services/platform/perps-data/internal/feeds/topics.go) |
| `go_exported` | `TopicChainBlocks   = "chain@blocks"` | [services/platform/perps-data/internal/feeds/topics.go](../../../services/platform/perps-data/internal/feeds/topics.go) |
| `go_exported` | `TopicChainTxs      = "chain@transactions"` | [services/platform/perps-data/internal/feeds/topics.go](../../../services/platform/perps-data/internal/feeds/topics.go) |
| `go_exported` | `TopicMarketsAll    = "markets@all"` | [services/platform/perps-data/internal/feeds/topics.go](../../../services/platform/perps-data/internal/feeds/topics.go) |
| `go_exported` | `TopicMarketsTotals = "markets@totals"` | [services/platform/perps-data/internal/feeds/topics.go](../../../services/platform/perps-data/internal/feeds/topics.go) |
| `go_exported` | `type ClientFrame struct` | [services/platform/perps-data/internal/ws/protocol.go](../../../services/platform/perps-data/internal/ws/protocol.go) |
| `go_exported` | `type DataFrame struct` | [services/platform/perps-data/internal/ws/protocol.go](../../../services/platform/perps-data/internal/ws/protocol.go) |
| `go_exported` | `func DefaultLimits() Limits` | [services/platform/perps-data/internal/ws/protocol.go](../../../services/platform/perps-data/internal/ws/protocol.go) |
| `go_exported` | `func (c ErrorCode) Error() string { return string(c) }` | [services/platform/perps-data/internal/ws/protocol.go](../../../services/platform/perps-data/internal/ws/protocol.go) |
| `go_exported` | `ErrUnknownTopic         ErrorCode = "unknown_topic"` | [services/platform/perps-data/internal/ws/protocol.go](../../../services/platform/perps-data/internal/ws/protocol.go) |
| `go_exported` | `type Event string` | [services/platform/perps-data/internal/ws/protocol.go](../../../services/platform/perps-data/internal/ws/protocol.go) |
| `go_exported` | `EventSubscribe   Event = "subscribe"` | [services/platform/perps-data/internal/ws/protocol.go](../../../services/platform/perps-data/internal/ws/protocol.go) |
| `go_exported` | `EventUnsubscribe Event = "unsubscribe"` | [services/platform/perps-data/internal/ws/protocol.go](../../../services/platform/perps-data/internal/ws/protocol.go) |
| `go_exported` | `type Frame struct` | [services/platform/perps-data/internal/ws/hub.go](../../../services/platform/perps-data/internal/ws/hub.go) |
| `go_exported` | `type Hub struct` | [services/platform/perps-data/internal/ws/hub.go](../../../services/platform/perps-data/internal/ws/hub.go) |
| `go_exported` | `func (h *Hub) Leave(s *Socket)` | [services/platform/perps-data/internal/ws/hub.go](../../../services/platform/perps-data/internal/ws/hub.go) |
| `go_exported` | `func (h *Hub) Listen(fn func(subject string, env delivery.Envelope)) (stop func())` | [services/platform/perps-data/internal/ws/hub.go](../../../services/platform/perps-data/internal/ws/hub.go) |
| `go_exported` | `func (h *Hub) ListenLoss(lost func()) (stop func())` | [services/platform/perps-data/internal/ws/hub.go](../../../services/platform/perps-data/internal/ws/hub.go) |
| `go_exported` | `func (h *Hub) Run(ctx context.Context, nc *nats.Conn) error` | [services/platform/perps-data/internal/ws/hub.go](../../../services/platform/perps-data/internal/ws/hub.go) |
| `go_exported` | `func (h *Hub) Subscribe(s *Socket, f ClientFrame)` | [services/platform/perps-data/internal/ws/hub.go](../../../services/platform/perps-data/internal/ws/hub.go) |
| `go_exported` | `func (h *Hub) Unsubscribe(s *Socket, topic string)` | [services/platform/perps-data/internal/ws/hub.go](../../../services/platform/perps-data/internal/ws/hub.go) |
| `go_exported` | `func NewHub(provider Provider, chain uint64, snapshotID string, logger *slog.Logger) (*Hub, error)` | [services/platform/perps-data/internal/ws/hub.go](../../../services/platform/perps-data/internal/ws/hub.go) |
| `go_exported` | `func NewServer(snapshotID string, served func(topic string) bool, limits Limits) (*Server, error)` | [services/platform/perps-data/internal/ws/protocol.go](../../../services/platform/perps-data/internal/ws/protocol.go) |
| `go_exported` | `func ParseClientFrame(raw []byte, served func(topic string) bool) (ClientFrame, error)` | [services/platform/perps-data/internal/ws/protocol.go](../../../services/platform/perps-data/internal/ws/protocol.go) |
| `go_exported` | `type Server struct` | [services/platform/perps-data/internal/ws/protocol.go](../../../services/platform/perps-data/internal/ws/protocol.go) |
| `go_exported` | `func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request)` | [services/platform/perps-data/internal/ws/protocol.go](../../../services/platform/perps-data/internal/ws/protocol.go) |
| `go_exported` | `type Socket struct` | [services/platform/perps-data/internal/ws/protocol.go](../../../services/platform/perps-data/internal/ws/protocol.go) |
| `go_exported` | `func (s *Socket) Subscriptions() int` | [services/platform/perps-data/internal/ws/protocol.go](../../../services/platform/perps-data/internal/ws/protocol.go) |
| `go_exported` | `func (s *SpotUser) CloseAll()` | [services/platform/perps-data/internal/ws/spotuser.go](../../../services/platform/perps-data/internal/ws/spotuser.go) |
| `go_exported` | `func (s *SpotUser) Deliver(stream, address string, data json.RawMessage) error` | [services/platform/perps-data/internal/ws/spotuser.go](../../../services/platform/perps-data/internal/ws/spotuser.go) |
| `go_exported` | `type SpotUser struct` | [services/platform/perps-data/internal/ws/spotuser.go](../../../services/platform/perps-data/internal/ws/spotuser.go) |
| `go_exported` | `func (s *SpotUser) ServeHTTP(w http.ResponseWriter, r *http.Request)` | [services/platform/perps-data/internal/ws/spotuser.go](../../../services/platform/perps-data/internal/ws/spotuser.go) |
| `ws_route` | `chain@blocks` | [services/platform/perps-data/internal/feeds/topics.go](../../../services/platform/perps-data/internal/feeds/topics.go) |
| `ws_route` | `chain@transactions` | [services/platform/perps-data/internal/feeds/topics.go](../../../services/platform/perps-data/internal/feeds/topics.go) |
| `ws_route` | `markets@all` | [services/platform/perps-data/internal/feeds/topics.go](../../../services/platform/perps-data/internal/feeds/topics.go) |
| `ws_route` | `markets@totals` | [services/platform/perps-data/internal/feeds/topics.go](../../../services/platform/perps-data/internal/feeds/topics.go) |
| `ws_route` | `<SYMBOL>@market` | [services/platform/perps-data/internal/feeds/topics.go](../../../services/platform/perps-data/internal/feeds/topics.go) |
