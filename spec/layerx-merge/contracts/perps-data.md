# Perps data contract

Source revision: `2a1033aa2`. This contract freezes the cited interface and acceptance blocks of the specification. Statuses include the specification overrides. A source marked planned has line 0; a positive source line is verified in the pinned revision.

The context and decision tables reproduce the rules linked by interface identifiers. An anchor without a linked context gains no additional storage, gas, activation or pause rule here. Acceptance qualification remains UNRUN until its declared gate succeeds.

## auth

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.3.go-perps-data-auth-address-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/auth/auth.go:271</code> | <code>func Address(ctx context.Context) (string, bool)</code> |
| <code>interface.3.go-perps-data-auth-middleware-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/auth/auth.go:259</code> | <code>func Middleware(s *Service, next http.Handler) http.Handler</code> |
| <code>interface.3.go-perps-data-auth-newservice-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/auth/auth.go:61</code> | <code>func NewService(db *sql.DB, secret string, now func() time.Time) (*Service, error)</code> |
| <code>interface.3.go-perps-data-auth-service-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/auth/auth.go:53</code> | <code>type Service struct</code> |
| <code>interface.3.go-perps-data-auth-tokenttl-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/auth/auth.go:29</code> | <code>TokenTTL = 3600 * time.Second</code> |

## cli

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.3.cli-perps-data-perps-explorer-j27</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/perps-data/cmd/perps-explorer/main.go:38</code> | <code>perps-explorer</code> |
| <code>interface.3.cli-perps-data-perps-gateway-j27</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/perps-data/cmd/perps-gateway/main.go:39</code> | <code>perps-gateway</code> |
| <code>interface.3.cli-perps-data-perps-indexer-j27</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/perps-data/cmd/perps-indexer/main.go:61</code> | <code>perps-indexer</code> |
| <code>interface.3.cli-perps-data-perps-loadtest-j27</code> | <code>cli_command</code> | <code>change</code> | <code>services/platform/perps-data/cmd/perps-loadtest/main.go:21</code> | <code>perps-loadtest</code> |
| <code>interface.3.cli-perps-data-perps-portal-j27</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/perps-data/cmd/perps-portal/main.go:35</code> | <code>perps-portal</code> |
| <code>interface.3.cli-perps-data-perps-replay-dry-run-j27</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/perps-data/cmd/perps-replay/main.go:73</code> | <code>perps-replay --dry-run</code> |
| <code>interface.3.cli-perps-data-perps-replay-from-j27</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/perps-data/cmd/perps-replay/main.go:71</code> | <code>perps-replay --from &lt;height&gt; (default 0)</code> |
| <code>interface.3.cli-perps-data-perps-replay-j27</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/perps-data/cmd/perps-replay/main.go:39</code> | <code>perps-replay</code> |
| <code>interface.3.cli-perps-data-perps-replay-to-j27</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/perps-data/cmd/perps-replay/main.go:72</code> | <code>perps-replay --to &lt;height&gt; (default replay.ToCursor)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.3.cli-perps-data-perps-explorer-j27</code> | notes | service binary; started by the deploy image |
| <code>interface.3.cli-perps-data-perps-gateway-j27</code> | notes | service binary; started by the deploy image |
| <code>interface.3.cli-perps-data-perps-indexer-j27</code> | notes | service binary; started by the deploy image |
| <code>interface.3.cli-perps-data-perps-loadtest-j27</code> | notes | service binary; started by the deploy image |
| <code>interface.3.cli-perps-data-perps-portal-j27</code> | notes | service binary; started by the deploy image |
| <code>interface.3.cli-perps-data-perps-replay-j27</code> | notes | service binary; started by the deploy image |

Contract context <code>contract_context.3.services-platform-perps-data-j97</code>:

| Field | Frozen rule |
|---|---|
| interface | 17 surfaces under services/platform/perps-data (1 cli_command, 1 env_var, 8 go_exported, 7 http_route; 16 change, 1 new) frozen at the anchors contracts/perps-data.md#cli, contracts/perps-data.md#configuration, contracts/perps-data.md#explorer, contracts/perps-data.md#gateway, contracts/perps-data.md#loadtest, contracts/perps-data.md#operations and more: perps-loadtest (services/platform/perps-data/cmd/perps-loadtest/main.go:21); PERPS_TRUSTED_PROXY_CIDRS (services/platform/perps-data/internal/ratelimit/ratelimit.go:30); Config.TrustedProxies []netip.Prefix (from PERPS_TRUSTED_PROXY_CIDRS) (services/platform/perps-data/internal/config/config.go (planned)); func Load(service Service) (*Config, error) (services/platform/perps-data/internal/config/config.go:76); type Options struct (services/platform/perps-data/internal/explorer/server.go:38); type Options struct (services/platform/perps-data/internal/gateway/server.go:93); var FeedSymbols = []string{"BTC", "ETH", "SOL", "AVAX", "LINK", "HYPE", "XRP", "ASTER", "T (services/platform/perps-data/loadtest/load.go:34); func FullProfile() Profile (services/platform/perps-data/loadtest/load.go:93); func ClientKey(r *http.Request) string (services/platform/perps-data/internal/ratelimit/ratelimit.go:123); func (s *SpotUsers) StreamItems(ctx context.Context, env delivery.Envelope) ([]SpotStreamI (services/platform/perps-data/internal/rest/spotusers.go:1409); and 7 more listed in interfaces |
| storage | no storage_key item in this context; the services/platform/perps-data items read or write state only through the keepers and stores frozen by the other domain-3 contexts |
| errors | no error_code item in this context; failures surface through the error registries frozen by the other domain-3 contexts |
| events | no event, precompile_event or sol_event item is named by this context; events of the handlers behind these items are frozen by the other domain-3 contexts |
| gas | not metered: no item in this context is a precompile method |
| activation | lands with tasks 3.41.2, 3.41.3, 3.45.2, 3.46.3, 3.46.4, 3.50.1, 3.56.1 in waves 1, 2, 3, 6 |
| invariants | each item keeps the kind, path and signature of its interface block once its landing task passes; a later change of any item needs a new decision of domain 3 |
| pause | no pause parameter governs these items; they are disabled only by reverting the touches of their landing tasks |

## configuration

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.3.env-perps-data-database-url-j27</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/perps-data/internal/config/config.go:34</code> | <code>DATABASE_URL</code> |
| <code>interface.3.env-perps-data-nats-cluster-password-j27</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/perps-data/deploy/nats/nats.conf:18</code> | <code>NATS_CLUSTER_PASSWORD</code> |
| <code>interface.3.env-perps-data-nats-gateway-password-j27</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/perps-data/deploy/nats/nats.conf:37</code> | <code>NATS_GATEWAY_PASSWORD</code> |
| <code>interface.3.env-perps-data-nats-indexer-password-j27</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/perps-data/deploy/nats/nats.conf:29</code> | <code>NATS_INDEXER_PASSWORD</code> |
| <code>interface.3.env-perps-data-nats-max-file-store-j27</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/perps-data/deploy/nats/nats.conf:9</code> | <code>NATS_MAX_FILE_STORE</code> |
| <code>interface.3.env-perps-data-nats-routes-j27</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/perps-data/deploy/nats/nats.conf:20</code> | <code>NATS_ROUTES</code> |
| <code>interface.3.env-perps-data-nats-url-j27</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/perps-data/internal/config/config.go:35</code> | <code>NATS_URL</code> |
| <code>interface.3.env-perps-data-perps-auth-secret-j27</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/perps-data/internal/config/config.go:37</code> | <code>PERPS_AUTH_SECRET</code> |
| <code>interface.3.env-perps-data-perps-chain-id-j27</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/perps-data/internal/config/config.go:31</code> | <code>PERPS_CHAIN_ID</code> |
| <code>interface.3.env-perps-data-perps-comet-urls-j27</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/perps-data/internal/config/config.go:33</code> | <code>PERPS_COMET_URLS</code> |
| <code>interface.3.env-perps-data-perps-data-test-comet-j27</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/perps-data/e2e_test.go:57</code> | <code>PERPS_DATA_TEST_COMET</code> |
| <code>interface.3.env-perps-data-perps-data-test-database-url-j27</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/perps-data/e2e_test.go:55</code> | <code>PERPS_DATA_TEST_DATABASE_URL</code> |
| <code>interface.3.env-perps-data-perps-data-test-rpc-j27</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/perps-data/e2e_test.go:56</code> | <code>PERPS_DATA_TEST_RPC</code> |
| <code>interface.3.env-perps-data-perps-data-test-seed-key-file-j27</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/perps-data/e2e_test.go:58</code> | <code>PERPS_DATA_TEST_SEED_KEY_FILE</code> |
| <code>interface.3.env-perps-data-perps-explorer-tx-base-j27</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/perps-data/internal/config/config.go:38</code> | <code>PERPS_EXPLORER_TX_BASE</code> |
| <code>interface.3.env-perps-data-perps-gateway-url-j27</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/perps-data/internal/config/config.go:39</code> | <code>PERPS_GATEWAY_URL</code> |
| <code>interface.3.env-perps-data-perps-rpc-urls-j27</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/perps-data/internal/config/config.go:32</code> | <code>PERPS_RPC_URLS</code> |
| <code>interface.3.env-perps-data-perps-start-height-j27</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/perps-data/internal/config/config.go:40</code> | <code>PERPS_START_HEIGHT</code> |
| <code>interface.3.env-perps-data-perps-trusted-proxy-cidrs-j27</code> | <code>env_var</code> | <code>change</code> | <code>services/platform/perps-data/internal/ratelimit/ratelimit.go:30</code> | <code>PERPS_TRUSTED_PROXY_CIDRS</code> |
| <code>interface.3.env-perps-data-port-j27</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/perps-data/internal/config/config.go:36</code> | <code>PORT</code> |
| <code>interface.3.go-perps-data-config-config-string-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/config/config.go:189</code> | <code>func (c Config) String() string</code> |
| <code>interface.3.go-perps-data-config-config-trustedproxies-j27</code> | <code>go_exported</code> | <code>new</code> | <code>services/platform/perps-data/internal/config/config.go</code> (planned) | <code>Config.TrustedProxies []netip.Prefix (from PERPS_TRUSTED_PROXY_CIDRS)</code> |
| <code>interface.3.go-perps-data-config-explorer-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/config/config.go:24</code> | <code>Explorer Service = "explorer"</code> |
| <code>interface.3.go-perps-data-config-gateway-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/config/config.go:23</code> | <code>Gateway  Service = "gateway"</code> |
| <code>interface.3.go-perps-data-config-indexer-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/config/config.go:22</code> | <code>Indexer  Service = "indexer"</code> |
| <code>interface.3.go-perps-data-config-load-j27</code> | <code>go_exported</code> | <code>change</code> | <code>services/platform/perps-data/internal/config/config.go:76</code> | <code>func Load(service Service) (*Config, error)</code> |
| <code>interface.3.go-perps-data-config-loadtest-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/config/config.go:27</code> | <code>Loadtest Service = "loadtest"</code> |
| <code>interface.3.go-perps-data-config-portal-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/config/config.go:25</code> | <code>Portal   Service = "portal"</code> |
| <code>interface.3.go-perps-data-config-replay-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/config/config.go:26</code> | <code>Replay   Service = "replay"</code> |
| <code>interface.3.go-perps-data-config-service-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/config/config.go:19</code> | <code>type Service string</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.3.env-perps-data-perps-trusted-proxy-cidrs-j27</code> | notes | read by a package-level var that panics on a malformed value (services/platform/perps-data/internal/ratelimit/ratelimit.go:37-45), outside config.Load |
| <code>interface.3.go-perps-data-config-config-string-j27</code> | notes | 13 callers; the first 12 listed |
| <code>interface.3.go-perps-data-config-load-j27</code> | notes | 15 callers; the first 12 listed |

Contract context <code>contract_context.3.services-platform-perps-data-j97</code>:

| Field | Frozen rule |
|---|---|
| interface | 17 surfaces under services/platform/perps-data (1 cli_command, 1 env_var, 8 go_exported, 7 http_route; 16 change, 1 new) frozen at the anchors contracts/perps-data.md#cli, contracts/perps-data.md#configuration, contracts/perps-data.md#explorer, contracts/perps-data.md#gateway, contracts/perps-data.md#loadtest, contracts/perps-data.md#operations and more: perps-loadtest (services/platform/perps-data/cmd/perps-loadtest/main.go:21); PERPS_TRUSTED_PROXY_CIDRS (services/platform/perps-data/internal/ratelimit/ratelimit.go:30); Config.TrustedProxies []netip.Prefix (from PERPS_TRUSTED_PROXY_CIDRS) (services/platform/perps-data/internal/config/config.go (planned)); func Load(service Service) (*Config, error) (services/platform/perps-data/internal/config/config.go:76); type Options struct (services/platform/perps-data/internal/explorer/server.go:38); type Options struct (services/platform/perps-data/internal/gateway/server.go:93); var FeedSymbols = []string{"BTC", "ETH", "SOL", "AVAX", "LINK", "HYPE", "XRP", "ASTER", "T (services/platform/perps-data/loadtest/load.go:34); func FullProfile() Profile (services/platform/perps-data/loadtest/load.go:93); func ClientKey(r *http.Request) string (services/platform/perps-data/internal/ratelimit/ratelimit.go:123); func (s *SpotUsers) StreamItems(ctx context.Context, env delivery.Envelope) ([]SpotStreamI (services/platform/perps-data/internal/rest/spotusers.go:1409); and 7 more listed in interfaces |
| storage | no storage_key item in this context; the services/platform/perps-data items read or write state only through the keepers and stores frozen by the other domain-3 contexts |
| errors | no error_code item in this context; failures surface through the error registries frozen by the other domain-3 contexts |
| events | no event, precompile_event or sol_event item is named by this context; events of the handlers behind these items are frozen by the other domain-3 contexts |
| gas | not metered: no item in this context is a precompile method |
| activation | lands with tasks 3.41.2, 3.41.3, 3.45.2, 3.46.3, 3.46.4, 3.50.1, 3.56.1 in waves 1, 2, 3, 6 |
| invariants | each item keeps the kind, path and signature of its interface block once its landing task passes; a later change of any item needs a new decision of domain 3 |
| pause | no pause parameter governs these items; they are disabled only by reverting the touches of their landing tasks |

## decode

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.3.go-perps-data-decode-accountlookup-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/decode/registry.go:212</code> | <code>type AccountLookup struct</code> |
| <code>interface.3.go-perps-data-decode-adlexecutedrow-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/decode/perps.go:736</code> | <code>type ADLExecutedRow struct</code> |
| <code>interface.3.go-perps-data-decode-blockinput-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/decode/pricefeed.go:66</code> | <code>type BlockInput struct</code> |
| <code>interface.3.go-perps-data-decode-collateraldepositedrow-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/decode/perps.go:466</code> | <code>type CollateralDepositedRow struct</code> |
| <code>interface.3.go-perps-data-decode-collateraltransferredrow-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/decode/perps.go:483</code> | <code>type CollateralTransferredRow struct</code> |
| <code>interface.3.go-perps-data-decode-collateralwithdrawnrow-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/decode/perps.go:474</code> | <code>type CollateralWithdrawnRow struct</code> |
| <code>interface.3.go-perps-data-decode-columnname-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/decode/registry.go:786</code> | <code>func ColumnName(field string) string</code> |
| <code>interface.3.go-perps-data-decode-decoded-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/decode/registry.go:188</code> | <code>type Decoded struct</code> |
| <code>interface.3.go-perps-data-decode-errstatepruned-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/decode/registry.go:82</code> | <code>ErrStatePruned = errors.New("decode: state pruned on every node")</code> |
| <code>interface.3.go-perps-data-decode-event-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/decode/registry.go:178</code> | <code>type Event struct</code> |
| <code>interface.3.go-perps-data-decode-eventaggregate-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/decode/pricefeed.go:27</code> | <code>EventAggregate        = "pricefeed_aggregate"</code> |
| <code>interface.3.go-perps-data-decode-eventaggregateskipped-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/decode/pricefeed.go:28</code> | <code>EventAggregateSkipped = "pricefeed_aggregate_skipped"</code> |
| <code>interface.3.go-perps-data-decode-fact-common-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/decode/registry.go:158</code> | <code>func (f Fact) Common() Fact { return f }</code> |
| <code>interface.3.go-perps-data-decode-fact-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/decode/registry.go:142</code> | <code>type Fact struct</code> |
| <code>interface.3.go-perps-data-decode-feecollectedrow-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/decode/perps.go:630</code> | <code>type FeeCollectedRow struct</code> |
| <code>interface.3.go-perps-data-decode-feemakerrebate-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/decode/perps.go:33</code> | <code>FeeMakerRebate uint8 = 4</code> |
| <code>interface.3.go-perps-data-decode-feetrading-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/decode/perps.go:32</code> | <code>FeeTrading     uint8 = 0</code> |
| <code>interface.3.go-perps-data-decode-fillrow-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/decode/perps.go:546</code> | <code>type FillRow struct</code> |
| <code>interface.3.go-perps-data-decode-fundingrateupdatedrow-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/decode/perps.go:651</code> | <code>type FundingRateUpdatedRow struct</code> |
| <code>interface.3.go-perps-data-decode-fundingsettledrow-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/decode/perps.go:660</code> | <code>type FundingSettledRow struct</code> |
| <code>interface.3.go-perps-data-decode-indexpricerow-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/decode/pricefeed.go:197</code> | <code>type IndexPriceRow struct</code> |
| <code>interface.3.go-perps-data-decode-insurancedrawrow-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/decode/perps.go:721</code> | <code>type InsuranceDrawRow struct</code> |
| <code>interface.3.go-perps-data-decode-kindrows-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/decode/kinds.go:12</code> | <code>func KindRows() []KindRow</code> |
| <code>interface.3.go-perps-data-decode-liquidationrow-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/decode/perps.go:691</code> | <code>type LiquidationRow struct</code> |
| <code>interface.3.go-perps-data-decode-log-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/decode/registry.go:134</code> | <code>type Log struct</code> |
| <code>interface.3.go-perps-data-decode-markpriceupdatedrow-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/decode/perps.go:642</code> | <code>type MarkPriceUpdatedRow struct</code> |
| <code>interface.3.go-perps-data-decode-moduleaddress-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/decode/registry.go:119</code> | <code>func ModuleAddress(name string) common.Address</code> |
| <code>interface.3.go-perps-data-decode-newregistry-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/decode/registry.go:256</code> | <code>func NewRegistry(metrics *telemetry.Metrics, opts Options) (*Registry, error)</code> |
| <code>interface.3.go-perps-data-decode-openinterestchangedrow-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/decode/perps.go:681</code> | <code>type OpenInterestChangedRow struct</code> |
| <code>interface.3.go-perps-data-decode-options-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/decode/registry.go:110</code> | <code>type Options struct</code> |
| <code>interface.3.go-perps-data-decode-ordercancelledrow-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/decode/perps.go:515</code> | <code>type OrderCancelledRow struct</code> |
| <code>interface.3.go-perps-data-decode-orderexecutedrow-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/decode/perps.go:538</code> | <code>type OrderExecutedRow struct</code> |
| <code>interface.3.go-perps-data-decode-orderexpiredrow-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/decode/perps.go:522</code> | <code>type OrderExpiredRow struct</code> |
| <code>interface.3.go-perps-data-decode-orderoptionsrow-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/decode/perps.go:503</code> | <code>type OrderOptionsRow struct</code> |
| <code>interface.3.go-perps-data-decode-orderplacedrow-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/decode/perps.go:491</code> | <code>type OrderPlacedRow struct</code> |
| <code>interface.3.go-perps-data-decode-ordertriggeredrow-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/decode/perps.go:530</code> | <code>type OrderTriggeredRow struct</code> |
| <code>interface.3.go-perps-data-decode-perpsaddress-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/decode/perps.go:17</code> | <code>var PerpsAddress = common.HexToAddress("0x0000000000000000000000000000000000001015")</code> |
| <code>interface.3.go-perps-data-decode-positionclosedrow-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/decode/perps.go:594</code> | <code>type PositionClosedRow struct</code> |
| <code>interface.3.go-perps-data-decode-positionentrypricechangedrow-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/decode/perps.go:585</code> | <code>type PositionEntryPriceChangedRow struct</code> |
| <code>interface.3.go-perps-data-decode-positionfundingappliedrow-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/decode/perps.go:670</code> | <code>type PositionFundingAppliedRow struct</code> |
| <code>interface.3.go-perps-data-decode-positionmodifiedrow-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/decode/perps.go:575</code> | <code>type PositionModifiedRow struct</code> |
| <code>interface.3.go-perps-data-decode-positionopenedrow-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/decode/perps.go:561</code> | <code>type PositionOpenedRow struct</code> |
| <code>interface.3.go-perps-data-decode-positiontpslsetrow-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/decode/perps.go:606</code> | <code>type PositionTpSlSetRow struct</code> |
| <code>interface.3.go-perps-data-decode-pricefeedaddress-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/decode/pricefeed.go:22</code> | <code>var PricefeedAddress = common.HexToAddress("0x0000000000000000000000000000000000001020")</code> |
| <code>interface.3.go-perps-data-decode-querier-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/decode/registry.go:222</code> | <code>type Querier interface</code> |
| <code>interface.3.go-perps-data-decode-receiptactivation-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/decode/registry.go:52</code> | <code>ReceiptActivation    ReceiptKind = "activation"</code> |
| <code>interface.3.go-perps-data-decode-receiptblockevent-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/decode/registry.go:53</code> | <code>ReceiptBlockEvent    ReceiptKind = "block_event"</code> |
| <code>interface.3.go-perps-data-decode-receiptkind-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/decode/registry.go:46</code> | <code>type ReceiptKind string</code> |
| <code>interface.3.go-perps-data-decode-receiptperpsendblock-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/decode/registry.go:50</code> | <code>ReceiptPerpsEndBlock ReceiptKind = "perps_endblock"</code> |
| <code>interface.3.go-perps-data-decode-receiptspotendblock-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/decode/registry.go:51</code> | <code>ReceiptSpotEndBlock  ReceiptKind = "spot_endblock"</code> |
| <code>interface.3.go-perps-data-decode-receipttx-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/decode/registry.go:49</code> | <code>ReceiptTx            ReceiptKind = "tx"</code> |
| <code>interface.3.go-perps-data-decode-registry-decode-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/decode/registry.go:336</code> | <code>func (r *Registry) Decode(ctx context.Context, q Querier, logs []Log) (*Result, error)</code> |
| <code>interface.3.go-perps-data-decode-registry-decodeblock-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/decode/pricefeed.go:81</code> | <code>func (r *Registry) DecodeBlock(ctx context.Context, q Querier, in BlockInput) (*Result, error)</code> |
| <code>interface.3.go-perps-data-decode-registry-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/decode/registry.go:238</code> | <code>type Registry struct</code> |
| <code>interface.3.go-perps-data-decode-registry-keys-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/decode/registry.go:298</code> | <code>func (r *Registry) Keys() []Key</code> |
| <code>interface.3.go-perps-data-decode-result-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/decode/registry.go:194</code> | <code>type Result struct</code> |
| <code>interface.3.go-perps-data-decode-reverterror-error-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/decode/registry.go:96</code> | <code>func (e *RevertError) Error() string { return "decode: call reverted 0x" + hex.EncodeToString(e.Data) }</code> |
| <code>interface.3.go-perps-data-decode-reverterror-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/decode/registry.go:92</code> | <code>type RevertError struct</code> |
| <code>interface.3.go-perps-data-decode-row-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/decode/registry.go:161</code> | <code>type Row interface</code> |
| <code>interface.3.go-perps-data-decode-seed-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/decode/seed.go:16</code> | <code>type Seed struct</code> |
| <code>interface.3.go-perps-data-decode-spotmarketorderrow-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/decode/spot.go:483</code> | <code>type SpotMarketOrderRow struct</code> |
| <code>interface.3.go-perps-data-decode-spotnativeswaprow-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/decode/spot.go:494</code> | <code>type SpotNativeSwapRow struct</code> |
| <code>interface.3.go-perps-data-decode-spotpooladdress-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/decode/spot.go:19</code> | <code>SpotPoolAddress   = common.HexToAddress("0x0000000000000000000000000000000000001021")</code> |
| <code>interface.3.go-perps-data-decode-spotsimpleswaprow-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/decode/spot.go:504</code> | <code>type SpotSimpleSwapRow struct</code> |
| <code>interface.3.go-perps-data-decode-spottieredfeerow-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/decode/spot.go:524</code> | <code>type SpotTieredFeeRow struct</code> |
| <code>interface.3.go-perps-data-decode-spotvaultaddress-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/decode/spot.go:23</code> | <code>SpotVaultAddress  = common.HexToAddress("0x0000000000000000000000000000000000001025")</code> |
| <code>interface.3.go-perps-data-decode-tradeclose-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/decode/perps.go:22</code> | <code>TradeClose       uint8 = 1</code> |
| <code>interface.3.go-perps-data-decode-tradeliquidation-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/decode/perps.go:23</code> | <code>TradeLiquidation uint8 = 2</code> |
| <code>interface.3.go-perps-data-decode-tradeopen-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/decode/perps.go:21</code> | <code>TradeOpen        uint8 = 0</code> |
| <code>interface.3.go-perps-data-decode-tradesettledrow-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/decode/perps.go:615</code> | <code>type TradeSettledRow struct</code> |
| <code>interface.3.go-perps-data-decode-vaultcreatedrow-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/decode/perps.go:459</code> | <code>type VaultCreatedRow struct</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.3.go-perps-data-decode-fact-common-j27</code> | notes | 16 callers; the first 12 listed |
| <code>interface.3.go-perps-data-decode-fact-j27</code> | notes | 29 callers; the first 12 listed |
| <code>interface.3.go-perps-data-decode-perpsaddress-j27</code> | notes | 67 callers; the first 12 listed |
| <code>interface.3.go-perps-data-decode-result-j27</code> | notes | 17 callers; the first 12 listed |
| <code>interface.3.go-perps-data-decode-reverterror-error-j27</code> | notes | 37 callers; the first 12 listed |
| <code>interface.3.go-perps-data-decode-row-j27</code> | notes | 38 callers; the first 12 listed |

## delivery

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.3.fmt-perps-data-nats-stream-perps-v1-j27</code> | <code>file_format</code> | <code>stable</code> | <code>services/platform/perps-data/internal/stream/layout.go:18</code> | <code>JetStream stream "PERPS_V1", max message 1048576 bytes</code> |
| <code>interface.3.fmt-perps-data-nats-subject-chain-j27</code> | <code>file_format</code> | <code>stable</code> | <code>services/platform/perps-data/internal/stream/layout.go:29</code> | <code>perps.&lt;chain&gt;.chain.&lt;block&#124;tx&#124;rewind&gt;</code> |
| <code>interface.3.fmt-perps-data-nats-subject-control-j27</code> | <code>file_format</code> | <code>stable</code> | <code>services/platform/perps-data/internal/delivery/store.go:125</code> | <code>perps.&lt;chain&gt;.control {"perps_seq","event_id"}</code> |
| <code>interface.3.fmt-perps-data-nats-subject-market-j27</code> | <code>file_format</code> | <code>stable</code> | <code>services/platform/perps-data/internal/stream/layout.go:27</code> | <code>perps.&lt;chain&gt;.&lt;market&gt;.&lt;fill&#124;order&#124;funding&#124;index&#124;liq&#124;mark&#124;oi&#124;market&gt;</code> |
| <code>interface.3.fmt-perps-data-nats-subject-spot-j27</code> | <code>file_format</code> | <code>stable</code> | <code>services/platform/perps-data/internal/stream/layout.go:30</code> | <code>perps.&lt;chain&gt;.spot.&lt;token&gt;.&lt;swap&#124;order&#124;vault&gt;</code> |
| <code>interface.3.fmt-perps-data-nats-subject-user-j27</code> | <code>file_format</code> | <code>stable</code> | <code>services/platform/perps-data/internal/stream/layout.go:28</code> | <code>perps.&lt;chain&gt;.user.&lt;account&gt;.&lt;position&#124;order&#124;fill&#124;margin&#124;funding&#124;ledger&gt;</code> |
| <code>interface.3.go-perps-data-delivery-batchpublisher-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/delivery/relay.go:40</code> | <code>type BatchPublisher interface</code> |
| <code>interface.3.go-perps-data-delivery-controlmessage-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/delivery/store.go:118</code> | <code>func ControlMessage(l Lease) (subject, msgID string, body []byte, err error)</code> |
| <code>interface.3.go-perps-data-delivery-controlskipped-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/delivery/store.go:109</code> | <code>ControlSkipped = "skipped"</code> |
| <code>interface.3.go-perps-data-delivery-decode-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/delivery/envelope.go:155</code> | <code>func Decode(b []byte, p Policy, subject string) (Envelope, error)</code> |
| <code>interface.3.go-perps-data-delivery-delivery-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/delivery/store.go:432</code> | <code>type Delivery interface{ Ack(context.Context) error }</code> |
| <code>interface.3.go-perps-data-delivery-envelope-encode-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/delivery/envelope.go:130</code> | <code>func (e Envelope) Encode(p Policy, subject string) ([]byte, error)</code> |
| <code>interface.3.go-perps-data-delivery-envelope-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/delivery/envelope.go:40</code> | <code>type Envelope struct</code> |
| <code>interface.3.go-perps-data-delivery-errauthorization-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/delivery/envelope.go:25</code> | <code>ErrAuthorization = errors.New("delivery: unapproved subject")</code> |
| <code>interface.3.go-perps-data-delivery-errbrokerdown-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/delivery/relay.go:15</code> | <code>var ErrBrokerDown = errors.New("delivery: broker unreachable")</code> |
| <code>interface.3.go-perps-data-delivery-errinflight-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/delivery/relay.go:23</code> | <code>var ErrInFlight = errors.New("delivery: message still in flight at the broker")</code> |
| <code>interface.3.go-perps-data-delivery-errintegrity-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/delivery/envelope.go:22</code> | <code>ErrIntegrity     = errors.New("delivery: immutable identity conflict")</code> |
| <code>interface.3.go-perps-data-delivery-erroversized-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/delivery/envelope.go:28</code> | <code>ErrOversized = errors.New("delivery: message refused for size")</code> |
| <code>interface.3.go-perps-data-delivery-hash-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/delivery/envelope.go:86</code> | <code>func Hash(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }</code> |
| <code>interface.3.go-perps-data-delivery-headercontrol-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/delivery/store.go:108</code> | <code>HeaderControl  = "Perps-Control"</code> |
| <code>interface.3.go-perps-data-delivery-lease-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/delivery/store.go:93</code> | <code>type Lease struct</code> |
| <code>interface.3.go-perps-data-delivery-maxenvelopebytes-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/delivery/envelope.go:19</code> | <code>const MaxEnvelopeBytes = 1 &lt;&lt; 20</code> |
| <code>interface.3.go-perps-data-delivery-maxretrydelay-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/delivery/store.go:137</code> | <code>const MaxRetryDelay = time.Minute</code> |
| <code>interface.3.go-perps-data-delivery-newstore-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/delivery/store.go:23</code> | <code>func NewStore(db *sql.DB, schema string, policy Policy) (*Store, error)</code> |
| <code>interface.3.go-perps-data-delivery-persistenceack-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/delivery/store.go:189</code> | <code>type PersistenceAck struct</code> |
| <code>interface.3.go-perps-data-delivery-persistentpublisher-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/delivery/relay.go:28</code> | <code>type PersistentPublisher interface</code> |
| <code>interface.3.go-perps-data-delivery-policy-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/delivery/envelope.go:56</code> | <code>type Policy struct</code> |
| <code>interface.3.go-perps-data-delivery-publication-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/delivery/relay.go:45</code> | <code>type Publication struct</code> |
| <code>interface.3.go-perps-data-delivery-quarantine-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/delivery/store.go:263</code> | <code>type Quarantine struct</code> |
| <code>interface.3.go-perps-data-delivery-relay-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/delivery/relay.go:50</code> | <code>type Relay struct</code> |
| <code>interface.3.go-perps-data-delivery-relay-pass-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/delivery/relay.go:73</code> | <code>func (r Relay) Pass(ctx context.Context) error</code> |
| <code>interface.3.go-perps-data-delivery-sourceorder-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/delivery/envelope.go:34</code> | <code>type SourceOrder struct</code> |
| <code>interface.3.go-perps-data-delivery-store-enqueue-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/delivery/store.go:66</code> | <code>func (s *Store) Enqueue(ctx context.Context, tx *sql.Tx, subject string, e Envelope) (int64, error)</code> |
| <code>interface.3.go-perps-data-delivery-store-headage-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/delivery/store.go:376</code> | <code>func (s *Store) HeadAge(ctx context.Context) (time.Duration, error)</code> |
| <code>interface.3.go-perps-data-delivery-store-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/delivery/store.go:17</code> | <code>type Store struct</code> |
| <code>interface.3.go-perps-data-delivery-store-quarantined-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/delivery/store.go:367</code> | <code>func (s *Store) Quarantined(ctx context.Context) (int64, error)</code> |
| <code>interface.3.go-perps-data-delivery-store-retry-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/delivery/store.go:294</code> | <code>func (s *Store) Retry(ctx context.Context, l Lease, class FailureClass, maxAttempts int, delay time.Duration) error</code> |
| <code>interface.3.go-perps-data-natsdriver-consumer-close-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/delivery/natsdriver/driver.go:355</code> | <code>func (c *Consumer) Close()</code> |
| <code>interface.3.go-perps-data-natsdriver-consumer-fetch-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/delivery/natsdriver/driver.go:238</code> | <code>func (c *Consumer) Fetch(ctx context.Context) ([]*Message, error)</code> |
| <code>interface.3.go-perps-data-natsdriver-consumer-run-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/delivery/natsdriver/driver.go:335</code> | <code>func (c *Consumer) Run(ctx context.Context, consume func(context.Context, *Message) error) error</code> |
| <code>interface.3.go-perps-data-natsdriver-consumerconfig-brokerconsumerconfig-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/delivery/natsdriver/driver.go:98</code> | <code>func (c ConsumerConfig) BrokerConsumerConfig() (jetstream.ConsumerConfig, error)</code> |
| <code>interface.3.go-perps-data-natsdriver-consumerconfig-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/delivery/natsdriver/driver.go:84</code> | <code>type ConsumerConfig struct</code> |
| <code>interface.3.go-perps-data-natsdriver-message-ack-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/delivery/natsdriver/driver.go:226</code> | <code>func (m *Message) Ack(ctx context.Context) error</code> |
| <code>interface.3.go-perps-data-natsdriver-message-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/delivery/natsdriver/driver.go:214</code> | <code>type Message struct</code> |
| <code>interface.3.go-perps-data-natsdriver-msgid-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/delivery/natsdriver/driver.go:40</code> | <code>func MsgID(eventID string) (string, error)</code> |
| <code>interface.3.go-perps-data-natsdriver-newconsumer-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/delivery/natsdriver/driver.go:146</code> | <code>func NewConsumer(ctx context.Context, nc *nats.Conn, cfg ConsumerConfig) (*Consumer, error)</code> |
| <code>interface.3.go-perps-data-natsdriver-newpublisher-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/delivery/natsdriver/driver.go:53</code> | <code>func NewPublisher(nc *nats.Conn, stream string) (*Publisher, error)</code> |
| <code>interface.3.go-perps-data-natsdriver-publisher-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/delivery/natsdriver/driver.go:48</code> | <code>type Publisher struct</code> |
| <code>interface.3.go-perps-data-natsdriver-publisher-publish-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/delivery/natsdriver/driver.go:64</code> | <code>func (p *Publisher) Publish(ctx context.Context, subject, eventID string, wire []byte) (delivery.PersistenceAck, error)</code> |
| <code>interface.3.go-perps-data-publish-config-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/publish/publisher.go:118</code> | <code>type Config struct</code> |
| <code>interface.3.go-perps-data-publish-ensurestream-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/publish/publisher.go:43</code> | <code>func EnsureStream(ctx context.Context, nc *nats.Conn, chain uint64) error</code> |
| <code>interface.3.go-perps-data-publish-headerblock-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/publish/publisher.go:29</code> | <code>HeaderBlock = "Perps-Block"</code> |
| <code>interface.3.go-perps-data-publish-headerkind-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/publish/publisher.go:30</code> | <code>HeaderKind  = "Perps-Kind"</code> |
| <code>interface.3.go-perps-data-publish-headerseq-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/publish/publisher.go:28</code> | <code>HeaderSeq   = "Perps-Seq"</code> |
| <code>interface.3.go-perps-data-publish-new-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/publish/publisher.go:163</code> | <code>func New(db *sql.DB, nc *nats.Conn, policy delivery.Policy, cfg Config) (*Publisher, error)</code> |
| <code>interface.3.go-perps-data-publish-publisher-collectors-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/publish/publisher.go:224</code> | <code>func (p *Publisher) Collectors() []prometheus.Collector</code> |
| <code>interface.3.go-perps-data-publish-publisher-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/publish/publisher.go:142</code> | <code>type Publisher struct</code> |
| <code>interface.3.go-perps-data-publish-publisher-ready-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/publish/publisher.go:232</code> | <code>func (p *Publisher) Ready(ctx context.Context) error</code> |
| <code>interface.3.go-perps-data-publish-publisher-run-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/publish/publisher.go:250</code> | <code>func (p *Publisher) Run(ctx context.Context) error</code> |
| <code>interface.3.go-perps-data-stream-chainsubject-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/stream/layout.go:103</code> | <code>func ChainSubject(chain uint64, kind string) (string, error)</code> |
| <code>interface.3.go-perps-data-stream-config-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/stream/layout.go:43</code> | <code>func Config(chain uint64) jetstream.StreamConfig</code> |
| <code>interface.3.go-perps-data-stream-family-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/stream/layout.go:24</code> | <code>type Family string</code> |
| <code>interface.3.go-perps-data-stream-marketsubject-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/stream/layout.go:93</code> | <code>func MarketSubject(chain, market uint64, kind string) (string, error)</code> |
| <code>interface.3.go-perps-data-stream-matches-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/stream/layout.go:133</code> | <code>func Matches(filter, subject string) bool</code> |
| <code>interface.3.go-perps-data-stream-name-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/stream/layout.go:18</code> | <code>const Name = "PERPS_V1"</code> |
| <code>interface.3.go-perps-data-stream-spotsubject-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/stream/layout.go:108</code> | <code>func SpotSubject(chain uint64, token common.Address, kind string) (string, error)</code> |
| <code>interface.3.go-perps-data-stream-subject-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/stream/layout.go:66</code> | <code>func Subject(chain uint64, family Family, entity, kind string) (string, error)</code> |
| <code>interface.3.go-perps-data-stream-usersubject-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/stream/layout.go:98</code> | <code>func UserSubject(chain uint64, account common.Address, kind string) (string, error)</code> |
| <code>interface.3.go-perps-data-stream-validfilter-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/stream/layout.go:116</code> | <code>func ValidFilter(filter string) bool</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.3.fmt-perps-data-nats-subject-chain-j27</code> | notes | kinds at services/platform/perps-data/internal/stream/layout.go:34-37 |
| <code>interface.3.fmt-perps-data-nats-subject-market-j27</code> | notes | kinds at services/platform/perps-data/internal/stream/layout.go:34-37 |
| <code>interface.3.fmt-perps-data-nats-subject-spot-j27</code> | notes | kinds at services/platform/perps-data/internal/stream/layout.go:34-37 |
| <code>interface.3.fmt-perps-data-nats-subject-user-j27</code> | notes | kinds at services/platform/perps-data/internal/stream/layout.go:34-37 |
| <code>interface.3.go-perps-data-delivery-envelope-j27</code> | notes | 48 callers; the first 12 listed |
| <code>interface.3.go-perps-data-delivery-policy-j27</code> | notes | 17 callers; the first 12 listed |
| <code>interface.3.go-perps-data-natsdriver-consumer-close-j27</code> | notes | 39 callers; the first 12 listed |
| <code>interface.3.go-perps-data-natsdriver-consumer-run-j27</code> | notes | 73 callers; the first 12 listed |
| <code>interface.3.go-perps-data-publish-publisher-run-j27</code> | notes | 70 callers; the first 12 listed |
| <code>interface.3.go-perps-data-stream-config-j27</code> | notes | 17 callers; the first 12 listed |
| <code>interface.3.go-perps-data-stream-name-j27</code> | notes | 55 callers; the first 12 listed |

## explorer

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.3.go-perps-data-explorer-action-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/explorer/explorer.go:114</code> | <code>func Action(kind string) (string, error)</code> |
| <code>interface.3.go-perps-data-explorer-actionoracleupdated-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/explorer/explorer.go:33</code> | <code>ActionOracleUpdated       = "Oracle Updated"</code> |
| <code>interface.3.go-perps-data-explorer-actionsealedbatchexecuted-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/explorer/explorer.go:34</code> | <code>ActionSealedBatchExecuted = "Sealed Batch Executed"</code> |
| <code>interface.3.go-perps-data-explorer-cursor-string-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/explorer/explorer.go:217</code> | <code>func (c Cursor) String() string</code> |
| <code>interface.3.go-perps-data-explorer-historyitem-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/explorer/explorer.go:189</code> | <code>type HistoryItem struct</code> |
| <code>interface.3.go-perps-data-explorer-kindpricefeedaggregate-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/explorer/explorer.go:40</code> | <code>KindPricefeedAggregate = "pricefeed_aggregate"</code> |
| <code>interface.3.go-perps-data-explorer-newserver-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/explorer/server.go:62</code> | <code>func NewServer(db *sql.DB, opts Options) (*Server, error)</code> |
| <code>interface.3.go-perps-data-explorer-options-j27</code> | <code>go_exported</code> | <code>change</code> | <code>services/platform/perps-data/internal/explorer/server.go:38</code> | <code>type Options struct</code> |
| <code>interface.3.go-perps-data-explorer-server-servehttp-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/explorer/server.go:95</code> | <code>func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request)</code> |
| <code>interface.3.http-perps-data-explorer-get-api-chain-accounts-address-history-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/explorer/server.go:84</code> | <code>GET /api/chain/accounts/{address}/history</code> |
| <code>interface.3.http-perps-data-explorer-get-api-chain-blocks-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/explorer/server.go:80</code> | <code>GET /api/chain/blocks</code> |
| <code>interface.3.http-perps-data-explorer-get-api-chain-health-j27</code> | <code>http_route</code> | <code>change</code> | <code>services/platform/perps-data/internal/explorer/server.go:82</code> | <code>GET /api/chain/health</code> |
| <code>interface.3.http-perps-data-explorer-get-api-chain-markets-symbol-history-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/explorer/server.go:85</code> | <code>GET /api/chain/markets/{symbol}/history</code> |
| <code>interface.3.http-perps-data-explorer-get-api-chain-transactions-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/explorer/server.go:81</code> | <code>GET /api/chain/transactions</code> |
| <code>interface.3.http-perps-data-explorer-get-api-chain-tx-hash-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/explorer/server.go:86</code> | <code>GET /api/chain/tx/{hash}</code> |
| <code>interface.3.http-perps-data-explorer-get-api-explorer-health-j27</code> | <code>http_route</code> | <code>change</code> | <code>services/platform/perps-data/internal/explorer/server.go:83</code> | <code>GET /api/explorer/health</code> |
| <code>interface.3.http-perps-data-explorer-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/explorer/server.go:87</code> | <code>/ (explorer fallback, 404 JSON)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.3.http-perps-data-explorer-j27</code> | notes | reached over HTTP; no in-repo caller names the literal path |

Contract context <code>contract_context.3.services-platform-perps-data-j97</code>:

| Field | Frozen rule |
|---|---|
| interface | 17 surfaces under services/platform/perps-data (1 cli_command, 1 env_var, 8 go_exported, 7 http_route; 16 change, 1 new) frozen at the anchors contracts/perps-data.md#cli, contracts/perps-data.md#configuration, contracts/perps-data.md#explorer, contracts/perps-data.md#gateway, contracts/perps-data.md#loadtest, contracts/perps-data.md#operations and more: perps-loadtest (services/platform/perps-data/cmd/perps-loadtest/main.go:21); PERPS_TRUSTED_PROXY_CIDRS (services/platform/perps-data/internal/ratelimit/ratelimit.go:30); Config.TrustedProxies []netip.Prefix (from PERPS_TRUSTED_PROXY_CIDRS) (services/platform/perps-data/internal/config/config.go (planned)); func Load(service Service) (*Config, error) (services/platform/perps-data/internal/config/config.go:76); type Options struct (services/platform/perps-data/internal/explorer/server.go:38); type Options struct (services/platform/perps-data/internal/gateway/server.go:93); var FeedSymbols = []string{"BTC", "ETH", "SOL", "AVAX", "LINK", "HYPE", "XRP", "ASTER", "T (services/platform/perps-data/loadtest/load.go:34); func FullProfile() Profile (services/platform/perps-data/loadtest/load.go:93); func ClientKey(r *http.Request) string (services/platform/perps-data/internal/ratelimit/ratelimit.go:123); func (s *SpotUsers) StreamItems(ctx context.Context, env delivery.Envelope) ([]SpotStreamI (services/platform/perps-data/internal/rest/spotusers.go:1409); and 7 more listed in interfaces |
| storage | no storage_key item in this context; the services/platform/perps-data items read or write state only through the keepers and stores frozen by the other domain-3 contexts |
| errors | no error_code item in this context; failures surface through the error registries frozen by the other domain-3 contexts |
| events | no event, precompile_event or sol_event item is named by this context; events of the handlers behind these items are frozen by the other domain-3 contexts |
| gas | not metered: no item in this context is a precompile method |
| activation | lands with tasks 3.41.2, 3.41.3, 3.45.2, 3.46.3, 3.46.4, 3.50.1, 3.56.1 in waves 1, 2, 3, 6 |
| invariants | each item keeps the kind, path and signature of its interface block once its landing task passes; a later change of any item needs a new decision of domain 3 |
| pause | no pause parameter governs these items; they are disabled only by reverting the touches of their landing tasks |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.3.496.1</code> | req | 3496.1 |
| <code>acceptance.3.496.1</code> | scenario | Given a migrated test database with PERPS_DATA_TEST_DATABASE_URL naming a migrated test database with two blocks and three tx_actions rows, when GET /api/chain/health is served, then blocksSeen is 2 and txsSeen is 3 |
| <code>acceptance.3.496.1</code> | test | services/platform/perps-data/internal/explorer/server_test.go::TestChainHealthCounters (planned) |
| <code>acceptance.3.496.1</code> | qualification | UNRUN |
| <code>req.3496</code> | ac_1 | GET /api/chain/health reports blocksSeen and txsSeen equal to perps.chain_counters after inserts, through the status read of services/platform/perps-data/internal/explorer/server.go:220-222. |
| <code>capability.3.45.2</code> | title | Answer explorer chain health from perps.chain_counters |
| <code>capability.3.45.2</code> | reqs | 3496.1 |

## gateway

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.3.go-perps-data-gateway-newserver-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/gateway/server.go:152</code> | <code>func NewServer(ctx context.Context, opts Options) (*Server, error)</code> |
| <code>interface.3.go-perps-data-gateway-options-j27</code> | <code>go_exported</code> | <code>change</code> | <code>services/platform/perps-data/internal/gateway/server.go:93</code> | <code>type Options struct</code> |
| <code>interface.3.go-perps-data-gateway-server-handler-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/gateway/server.go:303</code> | <code>func (s *Server) Handler() http.Handler { return http.HandlerFunc(s.serve) }</code> |
| <code>interface.3.go-perps-data-gateway-server-ready-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/gateway/server.go:409</code> | <code>func (s *Server) Ready() bool { return s.ready.Load() }</code> |
| <code>interface.3.go-perps-data-gateway-server-run-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/gateway/server.go:308</code> | <code>func (s *Server) Run(ctx context.Context) error</code> |
| <code>interface.3.http-perps-data-gateway-get-get-ws-get-ws-markets-get-ws-explorer-get-ws-symbol-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/gateway/server.go:90</code> | <code>GET /{$}, GET /ws, GET /ws/markets, GET /ws/explorer, GET /ws/{symbol} (data sockets, registered at :292)</code> |
| <code>interface.3.http-perps-data-gateway-get-graphql-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/gateway/server.go:288</code> | <code>GET /graphql (graphql-transport-ws upgrade)</code> |
| <code>interface.3.http-perps-data-gateway-get-graphql-ws-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/gateway/server.go:289</code> | <code>GET /graphql-ws</code> |
| <code>interface.3.http-perps-data-gateway-get-metrics-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/gateway/server.go:298</code> | <code>GET /metrics (gateway)</code> |
| <code>interface.3.http-perps-data-gateway-get-v1-ws-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/gateway/server.go:295</code> | <code>GET /v1/ws (spot user socket, gated)</code> |
| <code>interface.3.http-perps-data-gateway-post-api-subgraph-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/gateway/server.go:287</code> | <code>POST /api/subgraph</code> |
| <code>interface.3.http-perps-data-gateway-post-graphql-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/gateway/server.go:286</code> | <code>POST /graphql</code> |
| <code>interface.3.http-perps-data-gateway-post-v2-auth-challenge-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/gateway/server.go:282</code> | <code>POST /v2/auth/challenge</code> |
| <code>interface.3.http-perps-data-gateway-post-v2-auth-refresh-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/gateway/server.go:284</code> | <code>POST /v2/auth/refresh</code> |
| <code>interface.3.http-perps-data-gateway-post-v2-auth-verify-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/gateway/server.go:283</code> | <code>POST /v2/auth/verify</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.3.go-perps-data-gateway-server-run-j27</code> | notes | 21 callers; the first 12 listed |

Contract context <code>contract_context.3.services-platform-perps-data-j97</code>:

| Field | Frozen rule |
|---|---|
| interface | 17 surfaces under services/platform/perps-data (1 cli_command, 1 env_var, 8 go_exported, 7 http_route; 16 change, 1 new) frozen at the anchors contracts/perps-data.md#cli, contracts/perps-data.md#configuration, contracts/perps-data.md#explorer, contracts/perps-data.md#gateway, contracts/perps-data.md#loadtest, contracts/perps-data.md#operations and more: perps-loadtest (services/platform/perps-data/cmd/perps-loadtest/main.go:21); PERPS_TRUSTED_PROXY_CIDRS (services/platform/perps-data/internal/ratelimit/ratelimit.go:30); Config.TrustedProxies []netip.Prefix (from PERPS_TRUSTED_PROXY_CIDRS) (services/platform/perps-data/internal/config/config.go (planned)); func Load(service Service) (*Config, error) (services/platform/perps-data/internal/config/config.go:76); type Options struct (services/platform/perps-data/internal/explorer/server.go:38); type Options struct (services/platform/perps-data/internal/gateway/server.go:93); var FeedSymbols = []string{"BTC", "ETH", "SOL", "AVAX", "LINK", "HYPE", "XRP", "ASTER", "T (services/platform/perps-data/loadtest/load.go:34); func FullProfile() Profile (services/platform/perps-data/loadtest/load.go:93); func ClientKey(r *http.Request) string (services/platform/perps-data/internal/ratelimit/ratelimit.go:123); func (s *SpotUsers) StreamItems(ctx context.Context, env delivery.Envelope) ([]SpotStreamI (services/platform/perps-data/internal/rest/spotusers.go:1409); and 7 more listed in interfaces |
| storage | no storage_key item in this context; the services/platform/perps-data items read or write state only through the keepers and stores frozen by the other domain-3 contexts |
| errors | no error_code item in this context; failures surface through the error registries frozen by the other domain-3 contexts |
| events | no event, precompile_event or sol_event item is named by this context; events of the handlers behind these items are frozen by the other domain-3 contexts |
| gas | not metered: no item in this context is a precompile method |
| activation | lands with tasks 3.41.2, 3.41.3, 3.45.2, 3.46.3, 3.46.4, 3.50.1, 3.56.1 in waves 1, 2, 3, 6 |
| invariants | each item keeps the kind, path and signature of its interface block once its landing task passes; a later change of any item needs a new decision of domain 3 |
| pause | no pause parameter governs these items; they are disabled only by reverting the touches of their landing tasks |

## graphql

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.3.go-perps-data-graphql-feed-deliver-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/graphql/subscriptions.go:69</code> | <code>func (f *Feed) Deliver(subject string, _ delivery.Envelope)</code> |
| <code>interface.3.go-perps-data-graphql-feed-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/graphql/subscriptions.go:33</code> | <code>type Feed struct</code> |
| <code>interface.3.go-perps-data-graphql-newfeed-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/graphql/subscriptions.go:56</code> | <code>func NewFeed(chain uint64, logger *slog.Logger) (*Feed, error)</code> |
| <code>interface.3.go-perps-data-graphql-newschema-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/graphql/schema.go:13</code> | <code>func NewSchema(db *sql.DB) (*graphql.Schema, error)</code> |
| <code>interface.3.go-perps-data-graphql-transportws-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/graphql/transportws.go:39</code> | <code>type TransportWS struct</code> |
| <code>interface.3.go-perps-data-graphql-transportws-servehttp-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/graphql/transportws.go:61</code> | <code>func (t *TransportWS) ServeHTTP(w http.ResponseWriter, r *http.Request)</code> |
| <code>interface.3.http-perps-data-graphql-market-margintiers-j27</code> | <code>http_route</code> | <code>new</code> | <code>services/platform/perps-data/internal/graphql/schema.go</code> (planned) | <code>Market.marginTiers: [MarginTier!]! (lowerBoundUsd, maxLeverage, maintenanceBps, deductionUsd)</code> |
| <code>interface.3.http-perps-data-graphql-query-accountledger-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/graphql/schema.go:565</code> | <code>POST /graphql Query.accountLedger( userAddress: String positionId: String limit: Int offset: Int ): [AccountLedgerEntry!]!</code> |
| <code>interface.3.http-perps-data-graphql-query-collateraltokens-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/graphql/schema.go:545</code> | <code>POST /graphql Query.collateralTokens: [CollateralToken!]!</code> |
| <code>interface.3.http-perps-data-graphql-query-competition-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/graphql/schema.go:899</code> | <code>POST /graphql Query.competition(id: String!): Competition</code> |
| <code>interface.3.http-perps-data-graphql-query-competitions-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/graphql/schema.go:898</code> | <code>POST /graphql Query.competitions: [Competition!]!</code> |
| <code>interface.3.http-perps-data-graphql-query-delegates-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/graphql/schema.go:572</code> | <code>POST /graphql Query.delegates(userAddress: String!): [Delegate!]!</code> |
| <code>interface.3.http-perps-data-graphql-query-enrichedglobalstats-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/graphql/schema.go:637</code> | <code>POST /graphql Query.enrichedGlobalStats: EnrichedGlobalStats!</code> |
| <code>interface.3.http-perps-data-graphql-query-enrichedmarketstats-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/graphql/schema.go:638</code> | <code>POST /graphql Query.enrichedMarketStats(marketId: Int!): EnrichedMarketStats!</code> |
| <code>interface.3.http-perps-data-graphql-query-feeconfig-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/graphql/schema.go:550</code> | <code>POST /graphql Query.feeConfig: FeeConfig</code> |
| <code>interface.3.http-perps-data-graphql-query-fees-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/graphql/schema.go:581</code> | <code>POST /graphql Query.fees( userAddress: String marketId: Int positionId: String feeType: Int limit: Int offset: Int ): [Fee!]!</code> |
| <code>interface.3.http-perps-data-graphql-query-fundingpayments-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/graphql/schema.go:618</code> | <code>POST /graphql Query.fundingPayments( userAddress: String marketId: Int positionId: String limit: Int offset: Int ): [FundingPayment!]!</code> |
| <code>interface.3.http-perps-data-graphql-query-fundingrates-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/graphql/schema.go:531</code> | <code>POST /graphql Query.fundingRates( marketId: Int! limit: Int offset: Int ): [FundingRate!]!</code> |
| <code>interface.3.http-perps-data-graphql-query-globalstats-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/graphql/schema.go:560</code> | <code>POST /graphql Query.globalStats: GlobalStats</code> |
| <code>interface.3.http-perps-data-graphql-query-indexerstatus-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/graphql/schema.go:561</code> | <code>POST /graphql Query.indexerStatus: IndexerStatus</code> |
| <code>interface.3.http-perps-data-graphql-query-keepercycles-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/graphql/schema.go:563</code> | <code>POST /graphql Query.keeperCycles(limit: Int, offset: Int): [KeeperCycle!]!</code> |
| <code>interface.3.http-perps-data-graphql-query-latestprices-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/graphql/schema.go:524</code> | <code>POST /graphql Query.latestPrices: [LatestPrice!]!</code> |
| <code>interface.3.http-perps-data-graphql-query-leaderboard-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/graphql/schema.go:485</code> | <code>POST /graphql Query.leaderboard(limit: Int, offset: Int): [LeaderboardEntry!]!</code> |
| <code>interface.3.http-perps-data-graphql-query-liquidations-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/graphql/schema.go:514</code> | <code>POST /graphql Query.liquidations( userAddress: String marketId: Int limit: Int offset: Int ): [Liquidation!]!</code> |
| <code>interface.3.http-perps-data-graphql-query-market-j27</code> | <code>http_route</code> | <code>change</code> | <code>services/platform/perps-data/internal/graphql/schema.go:521</code> | <code>POST /graphql Query.market(marketId: Int!): Market</code> |
| <code>interface.3.http-perps-data-graphql-query-markets-j27</code> | <code>http_route</code> | <code>change</code> | <code>services/platform/perps-data/internal/graphql/schema.go:522</code> | <code>POST /graphql Query.markets: [Market!]!</code> |
| <code>interface.3.http-perps-data-graphql-query-marketsnapshots-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/graphql/schema.go:626</code> | <code>POST /graphql Query.marketSnapshots( marketId: Int! limit: Int offset: Int ): [MarketSnapshotRecord!]!</code> |
| <code>interface.3.http-perps-data-graphql-query-marketstats-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/graphql/schema.go:559</code> | <code>POST /graphql Query.marketStats(marketId: Int!): MarketStats</code> |
| <code>interface.3.http-perps-data-graphql-query-markpricehistory-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/graphql/schema.go:605</code> | <code>POST /graphql Query.markPriceHistory( marketId: Int! limit: Int offset: Int ): [MarkPriceRecord!]!</code> |
| <code>interface.3.http-perps-data-graphql-query-oisnapshots-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/graphql/schema.go:599</code> | <code>POST /graphql Query.oiSnapshots( marketId: Int! limit: Int offset: Int ): [OiSnapshot!]!</code> |
| <code>interface.3.http-perps-data-graphql-query-order-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/graphql/schema.go:504</code> | <code>POST /graphql Query.order(orderId: String!): Order</code> |
| <code>interface.3.http-perps-data-graphql-query-orderlyfundingrates-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/graphql/schema.go:641</code> | <code>POST /graphql Query.orderlyFundingRates: [OrderlyFundingRate!]!</code> |
| <code>interface.3.http-perps-data-graphql-query-orderlyopeninterests-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/graphql/schema.go:643</code> | <code>POST /graphql Query.orderlyOpenInterests: [OrderlyOpenInterest!]!</code> |
| <code>interface.3.http-perps-data-graphql-query-orderlypricechanges-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/graphql/schema.go:642</code> | <code>POST /graphql Query.orderlyPriceChanges: [OrderlyPriceChange!]!</code> |
| <code>interface.3.http-perps-data-graphql-query-orderlytickers-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/graphql/schema.go:640</code> | <code>POST /graphql Query.orderlyTickers: [OrderlyTicker!]!</code> |
| <code>interface.3.http-perps-data-graphql-query-orders-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/graphql/schema.go:505</code> | <code>POST /graphql Query.orders( userAddress: String marketId: Int status: String orderType: Int limit: Int offset: Int ): [Order!]!</code> |
| <code>interface.3.http-perps-data-graphql-query-poolstate-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/graphql/schema.go:548</code> | <code>POST /graphql Query.poolState(marketId: Int!): PoolState</code> |
| <code>interface.3.http-perps-data-graphql-query-poolstates-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/graphql/schema.go:547</code> | <code>POST /graphql Query.poolStates: [PoolState!]!</code> |
| <code>interface.3.http-perps-data-graphql-query-position-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/graphql/schema.go:486</code> | <code>POST /graphql Query.position(positionId: String!): Position</code> |
| <code>interface.3.http-perps-data-graphql-query-positions-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/graphql/schema.go:487</code> | <code>POST /graphql Query.positions( userAddress: String marketId: Int status: String limit: Int offset: Int ): [Position!]!</code> |
| <code>interface.3.http-perps-data-graphql-query-pricehistory-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/graphql/schema.go:525</code> | <code>POST /graphql Query.priceHistory( marketId: Int! limit: Int offset: Int ): [PriceUpdate!]!</code> |
| <code>interface.3.http-perps-data-graphql-query-protocolevents-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/graphql/schema.go:552</code> | <code>POST /graphql Query.protocolEvents( eventName: String limit: Int offset: Int ): [ProtocolEvent!]!</code> |
| <code>interface.3.http-perps-data-graphql-query-protocolsnapshots-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/graphql/schema.go:632</code> | <code>POST /graphql Query.protocolSnapshots( limit: Int offset: Int ): [ProtocolSnapshotRecord!]!</code> |
| <code>interface.3.http-perps-data-graphql-query-referraltiers-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/graphql/schema.go:897</code> | <code>POST /graphql Query.referralTiers: [ReferralTier!]!</code> |
| <code>interface.3.http-perps-data-graphql-query-spotbatchclearfailures-j27</code> | <code>http_route</code> | <code>retire</code> | <code>services/platform/perps-data/internal/graphql/schema.go:705</code> | <code>POST /graphql Query.spotBatchClearFailures( marketId: String limit: Int offset: Int ): [SpotBatchClearFailure!]!</code> |
| <code>interface.3.http-perps-data-graphql-query-spotbatchclearings-j27</code> | <code>http_route</code> | <code>retire</code> | <code>services/platform/perps-data/internal/graphql/schema.go:665</code> | <code>POST /graphql Query.spotBatchClearings( marketId: String limit: Int offset: Int ): [SpotBatchClearing!]!</code> |
| <code>interface.3.http-perps-data-graphql-query-spotcollateralmovements-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/graphql/schema.go:680</code> | <code>POST /graphql Query.spotCollateralMovements( userAddress: String tokenAddress: String eventType: String limit: Int offset: Int ): [SpotCollateralMovement!]!</code> |
| <code>interface.3.http-perps-data-graphql-query-spotepochsettlements-j27</code> | <code>http_route</code> | <code>retire</code> | <code>services/platform/perps-data/internal/graphql/schema.go:671</code> | <code>POST /graphql Query.spotEpochSettlements(limit: Int, offset: Int): [SpotEpochSettlement!]!</code> |
| <code>interface.3.http-perps-data-graphql-query-spotfastsettlements-j27</code> | <code>http_route</code> | <code>retire</code> | <code>services/platform/perps-data/internal/graphql/schema.go:673</code> | <code>POST /graphql Query.spotFastSettlements( userAddress: String tokenAddress: String limit: Int offset: Int ): [SpotFastSettlement!]!</code> |
| <code>interface.3.http-perps-data-graphql-query-spotkeepercycles-j27</code> | <code>http_route</code> | <code>retire</code> | <code>services/platform/perps-data/internal/graphql/schema.go:703</code> | <code>POST /graphql Query.spotKeeperCycles(limit: Int, offset: Int): [SpotKeeperCycle!]!</code> |
| <code>interface.3.http-perps-data-graphql-query-spotmarket-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/graphql/schema.go:645</code> | <code>POST /graphql Query.spotMarket(marketId: String!): SpotMarket</code> |
| <code>interface.3.http-perps-data-graphql-query-spotmarketmodechanges-j27</code> | <code>http_route</code> | <code>retire</code> | <code>services/platform/perps-data/internal/graphql/schema.go:688</code> | <code>POST /graphql Query.spotMarketModeChanges( marketId: String limit: Int offset: Int ): [SpotMarketModeChange!]!</code> |
| <code>interface.3.http-perps-data-graphql-query-spotmarkets-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/graphql/schema.go:646</code> | <code>POST /graphql Query.spotMarkets(status: String, limit: Int, offset: Int): [SpotMarket!]!</code> |
| <code>interface.3.http-perps-data-graphql-query-spotorder-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/graphql/schema.go:648</code> | <code>POST /graphql Query.spotOrder(orderId: String!): SpotOrder</code> |
| <code>interface.3.http-perps-data-graphql-query-spotorderfills-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/graphql/schema.go:657</code> | <code>POST /graphql Query.spotOrderFills( orderId: String traderAddress: String marketId: String limit: Int offset: Int ): [SpotOrderFill!]!</code> |
| <code>interface.3.http-perps-data-graphql-query-spotorders-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/graphql/schema.go:649</code> | <code>POST /graphql Query.spotOrders( traderAddress: String marketId: String status: String limit: Int offset: Int ): [SpotOrder!]!</code> |
| <code>interface.3.http-perps-data-graphql-query-spotplvs-j27</code> | <code>http_route</code> | <code>retire</code> | <code>services/platform/perps-data/internal/graphql/schema.go:701</code> | <code>POST /graphql Query.spotPLVs(isRegistered: Boolean, limit: Int, offset: Int): [SpotPLV!]!</code> |
| <code>interface.3.http-perps-data-graphql-query-spotpofqupdates-j27</code> | <code>http_route</code> | <code>retire</code> | <code>services/platform/perps-data/internal/graphql/schema.go:694</code> | <code>POST /graphql Query.spotPofqUpdates( entityAddress: String isVault: Boolean limit: Int offset: Int ): [SpotPofqUpdate!]!</code> |
| <code>interface.3.http-perps-data-graphql-query-trades-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/graphql/schema.go:495</code> | <code>POST /graphql Query.trades( userAddress: String marketId: Int positionId: String tradeType: String limit: Int offset: Int ): [Trade!]!</code> |
| <code>interface.3.http-perps-data-graphql-query-tradesettlements-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/graphql/schema.go:590</code> | <code>POST /graphql Query.tradeSettlements( userAddress: String marketId: Int positionId: String tradeType: Int limit: Int offset: Int ): [TradeSettlement!]!</code> |
| <code>interface.3.http-perps-data-graphql-query-tradingaccountevents-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/graphql/schema.go:574</code> | <code>POST /graphql Query.tradingAccountEvents( userAddress: String eventType: String limit: Int offset: Int ): [TradingAccountEvent!]!</code> |
| <code>interface.3.http-perps-data-graphql-query-user-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/graphql/schema.go:892</code> | <code>POST /graphql Query.user(address: String!): User</code> |
| <code>interface.3.http-perps-data-graphql-query-useralerts-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/graphql/schema.go:895</code> | <code>POST /graphql Query.userAlerts(address: String!): [UserAlert!]!</code> |
| <code>interface.3.http-perps-data-graphql-query-userbalances-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/graphql/schema.go:893</code> | <code>POST /graphql Query.userBalances(address: String!): [UserBalance!]!</code> |
| <code>interface.3.http-perps-data-graphql-query-userdailypnl-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/graphql/schema.go:894</code> | <code>POST /graphql Query.userDailyPnl(address: String!, limit: Int): [DailyPnl!]!</code> |
| <code>interface.3.http-perps-data-graphql-query-userreferrals-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/graphql/schema.go:896</code> | <code>POST /graphql Query.userReferrals(address: String!): UserReferrals</code> |
| <code>interface.3.http-perps-data-graphql-query-userstats-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/graphql/schema.go:558</code> | <code>POST /graphql Query.userStats(userAddress: String, address: String): UserStats</code> |
| <code>interface.3.http-perps-data-graphql-query-uservault-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/graphql/schema.go:537</code> | <code>POST /graphql Query.userVault(userAddress: String!): UserVault</code> |
| <code>interface.3.http-perps-data-graphql-query-vaultbalancehistory-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/graphql/schema.go:611</code> | <code>POST /graphql Query.vaultBalanceHistory( tokenAddress: String vaultType: Int limit: Int offset: Int ): [VaultBalanceRecord!]!</code> |
| <code>interface.3.http-perps-data-graphql-query-vaultevents-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/graphql/schema.go:538</code> | <code>POST /graphql Query.vaultEvents( userAddress: String eventType: String limit: Int offset: Int ): [VaultEvent!]!</code> |
| <code>interface.3.http-perps-data-graphql-subscription-indexerstatusupdated-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/graphql/schema.go:879</code> | <code>POST /graphql Subscription.indexerStatusUpdated: IndexerStatus</code> |
| <code>interface.3.http-perps-data-graphql-subscription-latestpricesupdated-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/graphql/schema.go:880</code> | <code>POST /graphql Subscription.latestPricesUpdated: [LatestPrice!]!</code> |
| <code>interface.3.http-perps-data-graphql-subscription-liquidationcreated-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/graphql/schema.go:885</code> | <code>POST /graphql Subscription.liquidationCreated(marketId: Int!): Liquidation</code> |
| <code>interface.3.http-perps-data-graphql-subscription-orderchanged-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/graphql/schema.go:883</code> | <code>POST /graphql Subscription.orderChanged(userAddress: String!): Order</code> |
| <code>interface.3.http-perps-data-graphql-subscription-positionchanged-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/graphql/schema.go:882</code> | <code>POST /graphql Subscription.positionChanged(userAddress: String!): [Position!]!</code> |
| <code>interface.3.http-perps-data-graphql-subscription-priceupdated-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/graphql/schema.go:881</code> | <code>POST /graphql Subscription.priceUpdated(marketId: Int): PriceUpdate</code> |
| <code>interface.3.http-perps-data-graphql-subscription-tradecreated-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/graphql/schema.go:884</code> | <code>POST /graphql Subscription.tradeCreated(marketId: Int!): Trade</code> |
| <code>interface.3.http-perps-data-graphql-subscription-tradeexecuted-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/graphql/schema.go:886</code> | <code>POST /graphql Subscription.tradeExecuted(marketId: Int): TradeExecuted</code> |
| <code>interface.3.http-perps-data-graphql-type-market-j27</code> | <code>http_route</code> | <code>change</code> | <code>services/platform/perps-data/internal/graphql/schema.go:22</code> | <code>type Market { marketId: Int! name: String! symbol: String! maxLeverage: BigDecimal! enabled: Boolean! createdAt: DateTime latestPrice: LatestPrice poolState: PoolState fundingRate: FundingRate }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.3.http-perps-data-graphql-query-collateraltokens-j27</code> | notes | no application operation selects this root |
| <code>interface.3.http-perps-data-graphql-query-feeconfig-j27</code> | notes | no application operation selects this root |
| <code>interface.3.http-perps-data-graphql-query-fundingrates-j27</code> | notes | no application operation selects this root |
| <code>interface.3.http-perps-data-graphql-query-indexerstatus-j27</code> | notes | no application operation selects this root |
| <code>interface.3.http-perps-data-graphql-query-order-j27</code> | notes | no application operation selects this root |
| <code>interface.3.http-perps-data-graphql-query-poolstates-j27</code> | notes | no application operation selects this root |
| <code>interface.3.http-perps-data-graphql-query-position-j27</code> | notes | no application operation selects this root |
| <code>interface.3.http-perps-data-graphql-query-pricehistory-j27</code> | notes | no application operation selects this root |
| <code>interface.3.http-perps-data-graphql-query-protocolevents-j27</code> | notes | no application operation selects this root |
| <code>interface.3.http-perps-data-graphql-type-market-j27</code> | notes | createdAt resolves NULL::timestamptz at services/platform/perps-data/internal/graphql/perps.go:893 |

Contract context <code>contract_context.3.perps-data-gateway-1-j27</code>:

| Field | Frozen rule |
|---|---|
| interface | The gateway GraphQL schema, REST routes and websocket topics of services/platform/perps-data served to the app and the portal |
| storage | Projections of services/platform/perps-data/migrations/0002_projections.sql and the spot leaderboard of 0015 |
| errors | GraphQL validation errors for retired roots; HTTP 404 Unknown symbol for a symbol outside the market set |
| events | Websocket topics of internal/feeds/topics.go per symbol and the markets, chain and spot channels |
| gas | Off-chain service; no gas |
| activation | Served from the gateway deploy; no activation height |
| invariants | Retired roots are refused at validation; every topic symbol is in the market set |
| pause | No pause |

Contract context <code>contract_context.3.perps-data-gateway-2-j27</code>:

| Field | Frozen rule |
|---|---|
| interface | The gateway GraphQL schema, REST routes and websocket topics of services/platform/perps-data served to the app and the portal |
| storage | Projections of services/platform/perps-data/migrations/0002_projections.sql and the spot leaderboard of 0015 |
| errors | GraphQL validation errors for retired roots; HTTP 404 Unknown symbol for a symbol outside the market set |
| events | Websocket topics of internal/feeds/topics.go per symbol and the markets, chain and spot channels |
| gas | Off-chain service; no gas |
| activation | Served from the gateway deploy; no activation height |
| invariants | Retired roots are refused at validation; every topic symbol is in the market set |
| pause | No pause |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.3.490.1</code> | req | 3490.1 |
| <code>acceptance.3.490.1</code> | scenario | Given a migrated test database with PERPS_DATA_TEST_DATABASE_URL naming a migrated test database whose BTC row has sz_decimals 5, tick_size 1000000 and listed_at set, when the markets query runs, then BTC answers szDecimals 5, tickSize 1000000 and createdAt equal to listed_at |
| <code>acceptance.3.490.1</code> | test | services/platform/perps-data/internal/graphql/markets_test.go::TestMarketConfigFields (planned) |
| <code>acceptance.3.490.1</code> | qualification | UNRUN |
| <code>req.3490</code> | ac_1 | The query markets { marketId szDecimals pxDecimals tickSize lotSize tierClass createdAt } served by Resolver.Markets (services/platform/perps-data/internal/graphql/perps.go:920) returns the perps.markets values, with createdAt equal to listed_at instead of the null of services/platform/perps-data/internal/graphql/perps.go:893. |
| <code>acceptance.3.497.1</code> | req | 3497.1 |
| <code>acceptance.3.497.1</code> | scenario | Given a migrated test database with PERPS_DATA_TEST_DATABASE_URL naming a migrated test database with two tiers for BTC, when the markets query asks marginTiers, then BTC answers both tiers, tier 0 first |
| <code>acceptance.3.497.1</code> | test | services/platform/perps-data/internal/graphql/markets_test.go::TestMarketMarginTiers (planned) |
| <code>acceptance.3.497.1</code> | qualification | UNRUN |
| <code>req.3497</code> | ac_1 | The query markets { marketId marginTiers { tier maxLeverage maintenanceBps } } through Resolver.Markets (services/platform/perps-data/internal/graphql/perps.go:920) returns each market's perps.market_tiers rows in tier order. |
| <code>acceptance.3.501.1</code> | req | 3501.1 |
| <code>acceptance.3.501.1</code> | scenario | Given the gateway schema after the retirement, when { spotPLVs { id } } and the seven other retired roots are validated, then each fails with Cannot query field |
| <code>acceptance.3.501.1</code> | test | services/platform/perps-data/internal/graphql/retired_test.go::TestRetiredSpotRoots (planned) |
| <code>acceptance.3.501.1</code> | qualification | UNRUN |
| <code>req.3501</code> | ac_1 | Each of the eight roots removed from services/platform/perps-data/internal/graphql/schema.go:665-705 is refused by schema validation with Cannot query field, and TestOperations (services/platform/perps-data/internal/graphql/operations_test.go:143) still validates every remaining operation. |
| <code>acceptance.3.508.1</code> | req | 3508.1 |
| <code>acceptance.3.508.1</code> | scenario | Given the e2e environment and an indexer that refreshed markets, when the gateway markets query is posted, then each market's tick, lot and tiers equal the chain views |
| <code>acceptance.3.508.1</code> | test | services/platform/perps-data/e2e_test.go::TestEndToEndGraphQLMarkets (planned) |
| <code>acceptance.3.508.1</code> | qualification | UNRUN |
| <code>req.3508</code> | ac_1 | The gateway markets query returns for every market tickSize, lotSize and marginTiers equal to getMarketConfig and getMarginTiers of the test node, the fields typed at services/platform/perps-data/internal/graphql/schema.go:22-32 after task 3.42.9. |
| <code>capability.3.42.4</code> | title | Expose chain decimals, tick, lot, tier class and listing time on GraphQL Market |
| <code>capability.3.42.4</code> | reqs | 3490.1 |
| <code>capability.3.42.9</code> | title | Serve Market.marginTiers from perps.market_tiers |
| <code>capability.3.42.9</code> | reqs | 3497.1 |
| <code>capability.3.43.1</code> | title | Retire the eight spot GraphQL roots that resolve to no facts |
| <code>capability.3.43.1</code> | reqs | 3501.1 |
| <code>capability.3.54.4</code> | title | End-to-end: GraphQL markets equal the chain market config and tiers |
| <code>capability.3.54.4</code> | reqs | 3508.1 |
| <code>resolution.17.608</code> | title | decision.346 binds: Retire the eight phantom spot GraphQL roots |
| <code>resolution.17.608</code> | applies_to | decision.346<br>task.3.43.1<br>task.3.43.3 |
| <code>resolution.17.608</code> | source | decision.346 in fragments/J27/decision.kvx; tasks from affects |

## indexer

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.3.go-perps-data-indexer-deps-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/indexer/indexer.go:85</code> | <code>type Deps struct</code> |
| <code>interface.3.go-perps-data-indexer-indexer-checks-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/indexer/indexer.go:260</code> | <code>func (ix *Indexer) Checks() map[string]httpx.Check</code> |
| <code>interface.3.go-perps-data-indexer-indexer-run-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/indexer/indexer.go:205</code> | <code>func (ix *Indexer) Run(ctx context.Context) error</code> |
| <code>interface.3.go-perps-data-indexer-maxoutboxage-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/indexer/indexer.go:43</code> | <code>MaxOutboxAge = 30 * time.Second</code> |
| <code>interface.3.go-perps-data-indexer-new-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/indexer/indexer.go:136</code> | <code>func New(ctx context.Context, d Deps) (*Indexer, error)</code> |
| <code>interface.3.go-perps-data-source-affected-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/source/follower.go:111</code> | <code>type Affected struct</code> |
| <code>interface.3.go-perps-data-source-block-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/source/follower.go:97</code> | <code>type Block struct</code> |
| <code>interface.3.go-perps-data-source-comet-header-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/source/comet.go:63</code> | <code>func (c *Comet) Header(ctx context.Context, height int64) (*CometHeader, error)</code> |
| <code>interface.3.go-perps-data-source-comet-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/source/comet.go:19</code> | <code>type Comet struct</code> |
| <code>interface.3.go-perps-data-source-follower-fetch-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/source/follower.go:368</code> | <code>func (f *Follower) Fetch(ctx context.Context, h uint64) (*Block, error)</code> |
| <code>interface.3.go-perps-data-source-follower-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/source/follower.go:153</code> | <code>type Follower struct</code> |
| <code>interface.3.go-perps-data-source-follower-run-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/source/follower.go:215</code> | <code>func (f *Follower) Run(ctx context.Context) error</code> |
| <code>interface.3.go-perps-data-source-follower-step-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/source/follower.go:238</code> | <code>func (f *Follower) Step(ctx context.Context) (int, error)</code> |
| <code>interface.3.go-perps-data-source-followeroptions-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/source/follower.go:121</code> | <code>type FollowerOptions struct</code> |
| <code>interface.3.go-perps-data-source-maxlogrange-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/source/rpc.go:36</code> | <code>const MaxLogRange = 500</code> |
| <code>interface.3.go-perps-data-source-newcomet-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/source/comet.go:44</code> | <code>func NewComet(urls []string, timeout time.Duration) (*Comet, error)</code> |
| <code>interface.3.go-perps-data-source-newfollower-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/source/follower.go:162</code> | <code>func NewFollower(ctx context.Context, opts FollowerOptions) (*Follower, error)</code> |
| <code>interface.3.go-perps-data-source-newrpc-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/source/rpc.go:130</code> | <code>func NewRPC(ctx context.Context, opts RPCOptions) (*RPC, error)</code> |
| <code>interface.3.go-perps-data-source-rpc-blocknumber-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/source/rpc.go:416</code> | <code>func (s *RPC) BlockNumber(ctx context.Context) (uint64, error)</code> |
| <code>interface.3.go-perps-data-source-rpc-call-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/source/rpc.go:405</code> | <code>func (s *RPC) Call(ctx context.Context, to common.Address, data []byte) ([]byte, error)</code> |
| <code>interface.3.go-perps-data-source-rpc-callat-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/source/rpc.go:411</code> | <code>func (s *RPC) CallAt(ctx context.Context, to common.Address, data []byte, height uint64) ([]byte, error)</code> |
| <code>interface.3.go-perps-data-source-rpc-callcontract-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/source/rpc.go:444</code> | <code>func (s *RPC) CallContract(ctx context.Context, call ethereum.CallMsg, blockNumber *big.Int) ([]byte, error)</code> |
| <code>interface.3.go-perps-data-source-rpc-close-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/source/rpc.go:204</code> | <code>func (s *RPC) Close()</code> |
| <code>interface.3.go-perps-data-source-rpc-head-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/source/rpc.go:211</code> | <code>func (s *RPC) Head(ctx context.Context) (uint64, error)</code> |
| <code>interface.3.go-perps-data-source-rpc-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/source/rpc.go:111</code> | <code>type RPC struct</code> |
| <code>interface.3.go-perps-data-source-rpc-transport-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/source/rpc.go:685</code> | <code>func (s *RPC) Transport() http.RoundTripper { return poolTransport{s} }</code> |
| <code>interface.3.go-perps-data-source-rpcoptions-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/source/rpc.go:85</code> | <code>type RPCOptions struct</code> |
| <code>interface.3.go-perps-data-source-synthetichash-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/source/follower.go:56</code> | <code>func SyntheticHash(tag string, height uint64) common.Hash</code> |
| <code>interface.3.go-perps-data-source-syntheticmismatcherror-error-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/source/follower.go:144</code> | <code>func (e *SyntheticMismatchError) Error() string</code> |
| <code>interface.3.go-perps-data-source-syntheticmismatcherror-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/source/follower.go:135</code> | <code>type SyntheticMismatchError struct</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.3.go-perps-data-source-follower-run-j27</code> | notes | 48 callers; the first 12 listed |
| <code>interface.3.go-perps-data-source-rpc-close-j27</code> | notes | 34 callers; the first 12 listed |
| <code>interface.3.go-perps-data-source-syntheticmismatcherror-error-j27</code> | notes | 13 callers; the first 12 listed |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.3.499.1</code> | req | 3499.1 |
| <code>acceptance.3.499.1</code> | scenario | Given a migrated test database with only seed rows and the test node, when Check runs before and after Refresh, then it fails first and passes second |
| <code>acceptance.3.499.1</code> | test | services/platform/perps-data/internal/markets/markets_test.go::TestMarketsCheck (planned) |
| <code>acceptance.3.499.1</code> | qualification | UNRUN |
| <code>req.3499</code> | ac_1 | Check fails on the seeded rows of services/platform/perps-data/migrations/0005_markets_seed.sql:1-19 that carry source seed and passes after one Refresh (services/platform/perps-data/internal/markets/markets.go:114). |
| <code>capability.3.42.5</code> | title | Run the periodic market refresh in perps-indexer and gate readiness on it |
| <code>capability.3.42.5</code> | reqs | 3499.1 |

## loadtest

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.3.go-perps-data-loadtest-feedsymbols-j27</code> | <code>go_exported</code> | <code>change</code> | <code>services/platform/perps-data/loadtest/load.go:34</code> | <code>var FeedSymbols = []string{"BTC", "ETH", "SOL", "AVAX", "LINK", "HYPE", "XRP", "ASTER", "TRUMP", "BNB", "SID", "PAX", "TSLA", "NVDA", "GOOGL", "XAU", "NAS100", "SPX500"}</code> |
| <code>interface.3.go-perps-data-loadtest-fullprofile-j27</code> | <code>go_exported</code> | <code>change</code> | <code>services/platform/perps-data/loadtest/load.go:93</code> | <code>func FullProfile() Profile</code> |
| <code>interface.3.go-perps-data-loadtest-report-err-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/loadtest/load.go:210</code> | <code>func (r *Report) Err() error</code> |
| <code>interface.3.go-perps-data-loadtest-run-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/loadtest/load.go:271</code> | <code>func Run(ctx context.Context, socketURL string, p Profile) (*Report, error)</code> |
| <code>interface.3.go-perps-data-loadtest-socketurl-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/loadtest/load.go:149</code> | <code>func SocketURL(gateway string) (string, error)</code> |

Contract context <code>contract_context.3.services-platform-perps-data-j97</code>:

| Field | Frozen rule |
|---|---|
| interface | 17 surfaces under services/platform/perps-data (1 cli_command, 1 env_var, 8 go_exported, 7 http_route; 16 change, 1 new) frozen at the anchors contracts/perps-data.md#cli, contracts/perps-data.md#configuration, contracts/perps-data.md#explorer, contracts/perps-data.md#gateway, contracts/perps-data.md#loadtest, contracts/perps-data.md#operations and more: perps-loadtest (services/platform/perps-data/cmd/perps-loadtest/main.go:21); PERPS_TRUSTED_PROXY_CIDRS (services/platform/perps-data/internal/ratelimit/ratelimit.go:30); Config.TrustedProxies []netip.Prefix (from PERPS_TRUSTED_PROXY_CIDRS) (services/platform/perps-data/internal/config/config.go (planned)); func Load(service Service) (*Config, error) (services/platform/perps-data/internal/config/config.go:76); type Options struct (services/platform/perps-data/internal/explorer/server.go:38); type Options struct (services/platform/perps-data/internal/gateway/server.go:93); var FeedSymbols = []string{"BTC", "ETH", "SOL", "AVAX", "LINK", "HYPE", "XRP", "ASTER", "T (services/platform/perps-data/loadtest/load.go:34); func FullProfile() Profile (services/platform/perps-data/loadtest/load.go:93); func ClientKey(r *http.Request) string (services/platform/perps-data/internal/ratelimit/ratelimit.go:123); func (s *SpotUsers) StreamItems(ctx context.Context, env delivery.Envelope) ([]SpotStreamI (services/platform/perps-data/internal/rest/spotusers.go:1409); and 7 more listed in interfaces |
| storage | no storage_key item in this context; the services/platform/perps-data items read or write state only through the keepers and stores frozen by the other domain-3 contexts |
| errors | no error_code item in this context; failures surface through the error registries frozen by the other domain-3 contexts |
| events | no event, precompile_event or sol_event item is named by this context; events of the handlers behind these items are frozen by the other domain-3 contexts |
| gas | not metered: no item in this context is a precompile method |
| activation | lands with tasks 3.41.2, 3.41.3, 3.45.2, 3.46.3, 3.46.4, 3.50.1, 3.56.1 in waves 1, 2, 3, 6 |
| invariants | each item keeps the kind, path and signature of its interface block once its landing task passes; a later change of any item needs a new decision of domain 3 |
| pause | no pause parameter governs these items; they are disabled only by reverting the touches of their landing tasks |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.3.506.1</code> | req | 3506.1 |
| <code>acceptance.3.506.1</code> | scenario | Given a gateway answering GET /api/markets with BTC, ETH and PAX, when FullProfile is built from that answer, then every subscription names one of the three symbols |
| <code>acceptance.3.506.1</code> | test | services/platform/perps-data/loadtest/load_test.go::TestLoadProfileServedSymbols (planned) |
| <code>acceptance.3.506.1</code> | qualification | UNRUN |
| <code>req.3506</code> | ac_1 | FullProfile (services/platform/perps-data/loadtest/load.go:93) given the GET /api/markets symbols builds subscriptions only for those symbols, so TSLA of the static list at services/platform/perps-data/loadtest/load.go:34 is absent when the gateway does not serve it. |
| <code>capability.3.56.1</code> | title | Load-test the symbols the gateway serves instead of a fixed list |
| <code>capability.3.56.1</code> | reqs | 3506.1 |

## markets

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.3.go-perps-data-markets-check-j27</code> | <code>go_exported</code> | <code>new</code> | <code>services/platform/perps-data/internal/markets/markets.go</code> (planned) | <code>func Check(db *sql.DB, maxAge time.Duration) httpx.Check</code> |
| <code>interface.3.go-perps-data-markets-feeconfig-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/markets/markets.go:161</code> | <code>func FeeConfig(ctx context.Context, db *sql.DB, client *ethclient.Client) error</code> |
| <code>interface.3.go-perps-data-markets-refresh-j27</code> | <code>go_exported</code> | <code>change</code> | <code>services/platform/perps-data/internal/markets/markets.go:114</code> | <code>func Refresh(ctx context.Context, db *sql.DB, client *ethclient.Client) error</code> |
| <code>interface.3.go-perps-data-markets-retry-j27</code> | <code>go_exported</code> | <code>retire</code> | <code>services/platform/perps-data/internal/markets/markets.go:177</code> | <code>func Retry(ctx context.Context, db *sql.DB, client *ethclient.Client, failed func(err error, retryIn time.Duration)) error</code> |
| <code>interface.3.go-perps-data-markets-run-j27</code> | <code>go_exported</code> | <code>new</code> | <code>services/platform/perps-data/internal/markets/markets.go</code> (planned) | <code>func Run(ctx context.Context, db *sql.DB, client *ethclient.Client, every time.Duration, m *telemetry.Metrics) error</code> |
| <code>interface.3.go-perps-data-markets-seed-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/markets/markets.go:56</code> | <code>var Seed = []Market{</code> |
| <code>interface.3.met-perps-data-perps-markets-refresh-failures-total-j27</code> | <code>metric</code> | <code>new</code> | <code>services/platform/perps-data/internal/markets/markets.go</code> (planned) | <code>perps_markets_refresh_failures_total (counter)</code> |
| <code>interface.3.met-perps-data-perps-markets-refresh-height-j27</code> | <code>metric</code> | <code>new</code> | <code>services/platform/perps-data/internal/markets/markets.go</code> (planned) | <code>perps_markets_refresh_height (gauge)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.3.go-perps-data-markets-seed-j27</code> | notes | content of migrations/0005_markets_seed.sql |

Contract context <code>contract_context.3.perps-data-markets-j27</code>:

| Field | Frozen rule |
|---|---|
| interface | perps.markets, perps.fee_config and perps.market_tiers as the market set of every perps-data surface, refreshed from 0x1015 totalMarkets, getMarket, getMarketConfig, getMarginTiers and getFees |
| storage | perps.markets of services/platform/perps-data/migrations/0001_facts.sql:956 with the config columns of 0012, perps.market_tiers of 0014, perps.fee_config row 1 |
| errors | ErrChainValue for an undecodable or out-of-range chain answer; ErrUnknownSymbol for a symbol outside perps.markets |
| events | No chain event announces a listing; perps_markets_refresh_height and perps_markets_refresh_failures_total report the periodic refresh |
| gas | Off-chain service; no gas |
| activation | Seeded by migration 0005 and refreshed from the v6.12.0 height |
| invariants | Every served symbol is a perps.markets row; a refresh writes all markets, fees and tiers in one transaction or none |
| pause | No pause; a failed refresh keeps the previous rows |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.3.486.1</code> | req | 3486.1 |
| <code>acceptance.3.486.1</code> | scenario | Given a migrated test database and the test node of PERPS_DATA_TEST_RPC, when Refresh runs, then every perps.markets row equals getMarketConfig of its id at refreshed_block, decimals included |
| <code>acceptance.3.486.1</code> | test | services/platform/perps-data/internal/markets/markets_test.go::TestRefreshMarketConfig (planned) |
| <code>acceptance.3.486.1</code> | qualification | UNRUN |
| <code>req.3486</code> | ac_1 | After Refresh (services/platform/perps-data/internal/markets/markets.go:114) against the test node every perps.markets row holds sz_decimals, px_decimals, tick_size, lot_size, tier_class and oi_cap_usd equal to getMarketConfig of its id at the refreshed_block height, and a market inserted by the refresh has non-null decimals. |
| <code>acceptance.3.491.1</code> | req | 3491.1 |
| <code>acceptance.3.491.1</code> | scenario | Given a migrated test database and the test node, when Refresh runs twice, then perps.market_tiers equals getMarginTiers of each market once, tier 0 first |
| <code>acceptance.3.491.1</code> | test | services/platform/perps-data/internal/markets/markets_test.go::TestRefreshMarginTiers (planned) |
| <code>acceptance.3.491.1</code> | qualification | UNRUN |
| <code>req.3491</code> | ac_1 | After Refresh (services/platform/perps-data/internal/markets/markets.go:114) perps.market_tiers holds for every market the tiers getMarginTiers returns at the refreshed height, in order, and a second refresh replaces rather than appends them. |
| <code>acceptance.3.495.1</code> | req | 3495.1 |
| <code>acceptance.3.495.1</code> | scenario | Given a migrated test database, the test node and Run with a 50 ms interval, when 500 ms pass, then refreshed_block advanced at least once after the first refresh and the gauge equals it |
| <code>acceptance.3.495.1</code> | test | services/platform/perps-data/internal/markets/markets_test.go::TestRunRefreshesAgain (planned) |
| <code>acceptance.3.495.1</code> | qualification | UNRUN |
| <code>req.3495</code> | ac_1 | Run started with a 50 ms interval refreshes perps.markets at least twice within 500 ms, observed by refreshed_block advancing, and perps_markets_refresh_height equals the last refreshed_block. |
| <code>acceptance.3.500.1</code> | req | 3500.1 |
| <code>acceptance.3.500.1</code> | scenario | Given the e2e environment of PERPS_DATA_TEST_DATABASE_URL and PERPS_DATA_TEST_RPC, when the indexer has refreshed and GET /api/markets is read, then its market count equals totalMarkets and each market_id and symbol pair equals getMarketConfig |
| <code>acceptance.3.500.1</code> | test | services/platform/perps-data/e2e_test.go::TestEndToEndChainMarkets (planned) |
| <code>acceptance.3.500.1</code> | qualification | UNRUN |
| <code>req.3500</code> | ac_1 | GET /api/markets (services/platform/perps-data/internal/rest/markets.go:49) lists totalMarkets markets of the test node, each with market_id and symbol equal to getMarketConfig of that id. |
| <code>acceptance.3.507.1</code> | req | 3507.1 |
| <code>acceptance.3.507.1</code> | scenario | Given the test node at the v6.12 activation seed and a migrated test database, when each Seed market is compared to getMarketConfig, then decimals and oracle symbol agree for all eighteen markets |
| <code>acceptance.3.507.1</code> | test | services/platform/perps-data/internal/markets/markets_test.go::TestSeedMatchesChainConfig (planned) |
| <code>acceptance.3.507.1</code> | qualification | UNRUN |
| <code>req.3507</code> | ac_1 | Every Seed market (services/platform/perps-data/internal/markets/markets.go:56-75) has sz_decimals, px_decimals and oracle_symbol equal to getMarketConfig of its id on the test node. |
| <code>capability.3.42.2</code> | title | Refresh reads getMarketConfig and writes chain decimals and config columns |
| <code>capability.3.42.2</code> | reqs | 3486.1 |
| <code>capability.3.42.3</code> | title | Refresh markets periodically with markets.Run and report height and failures |
| <code>capability.3.42.3</code> | reqs | 3495.1 |
| <code>capability.3.42.6</code> | title | Conformance: the seed migration matches getMarketConfig of the chain seed |
| <code>capability.3.42.6</code> | reqs | 3507.1 |
| <code>capability.3.42.7</code> | title | Write the chain margin tiers into perps.market_tiers on refresh |
| <code>capability.3.42.7</code> | reqs | 3491.1 |
| <code>capability.3.54.1</code> | title | End-to-end: /api/markets serves every chain market by id |
| <code>capability.3.54.1</code> | reqs | 3500.1 |
| <code>resolution.17.613</code> | title | decision.351 binds: Markets are refreshed periodically |
| <code>resolution.17.613</code> | applies_to | decision.351<br>task.3.42.3<br>task.3.42.5 |
| <code>resolution.17.613</code> | source | decision.351 in fragments/J27/decision.kvx; tasks from affects |
| <code>resolution.17.617</code> | title | decision.364 binds: The chain is the authority for perps-data market configuration |
| <code>resolution.17.617</code> | applies_to | decision.364<br>task.3.42.2<br>task.3.42.6 |
| <code>resolution.17.617</code> | source | decision.364 in fragments/J64/decision.kvx; tasks from affects |

## operations

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.3.go-perps-data-httpx-check-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/httpx/health.go:19</code> | <code>type Check func(ctx context.Context) error</code> |
| <code>interface.3.go-perps-data-httpx-healthdeps-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/httpx/health.go:23</code> | <code>type HealthDeps struct</code> |
| <code>interface.3.go-perps-data-httpx-readinesstimeout-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/httpx/health.go:44</code> | <code>const ReadinessTimeout = 2 * time.Second</code> |
| <code>interface.3.go-perps-data-httpx-registerhealth-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/httpx/health.go:31</code> | <code>func RegisterHealth(mux *http.ServeMux, deps HealthDeps)</code> |
| <code>interface.3.go-perps-data-ratelimit-clientkey-j27</code> | <code>go_exported</code> | <code>change</code> | <code>services/platform/perps-data/internal/ratelimit/ratelimit.go:123</code> | <code>func ClientKey(r *http.Request) string</code> |
| <code>interface.3.go-perps-data-ratelimit-limiter-middleware-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/ratelimit/ratelimit.go:145</code> | <code>func (l *Limiter) Middleware(next http.Handler) http.Handler</code> |
| <code>interface.3.go-perps-data-ratelimit-newlimiter-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/ratelimit/ratelimit.go:136</code> | <code>func NewLimiter(maxConcurrent int) *Limiter</code> |
| <code>interface.3.go-perps-data-ratelimit-report-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/ratelimit/ratelimit.go:87</code> | <code>func Report(log *slog.Logger, register func(prometheus.Collector) error) error</code> |
| <code>interface.3.go-perps-data-telemetry-metrics-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/telemetry/metrics.go:26</code> | <code>type Metrics struct</code> |
| <code>interface.3.go-perps-data-telemetry-metrics-recordread-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/telemetry/metrics.go:59</code> | <code>func (m *Metrics) RecordRead(route string, status int) error</code> |
| <code>interface.3.go-perps-data-telemetry-metrics-register-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/telemetry/metrics.go:51</code> | <code>func (m *Metrics) Register(c prometheus.Collector) error</code> |
| <code>interface.3.go-perps-data-telemetry-metrics-servehttp-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/telemetry/metrics.go:86</code> | <code>func (m *Metrics) ServeHTTP(w http.ResponseWriter, r *http.Request)</code> |
| <code>interface.3.go-perps-data-telemetry-metrics-writeprometheus-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/telemetry/metrics.go:68</code> | <code>func (m *Metrics) WritePrometheus(w io.Writer) error</code> |
| <code>interface.3.go-perps-data-telemetry-new-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/telemetry/metrics.go:33</code> | <code>func New(service config.Service) (*Metrics, error)</code> |
| <code>interface.3.http-perps-data-httpx-get-healthz-live-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/httpx/health.go:32</code> | <code>GET /healthz/live</code> |
| <code>interface.3.http-perps-data-httpx-get-healthz-ready-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/httpx/health.go:33</code> | <code>GET /healthz/ready</code> |
| <code>interface.3.http-perps-data-perps-explorer-api-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/cmd/perps-explorer/main.go:85</code> | <code>/api/ (explorer mount)</code> |
| <code>interface.3.http-perps-data-perps-explorer-get-metrics-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/cmd/perps-explorer/main.go:84</code> | <code>GET /metrics (explorer)</code> |
| <code>interface.3.http-perps-data-perps-indexer-get-metrics-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/cmd/perps-indexer/main.go:187</code> | <code>GET /metrics (indexer)</code> |
| <code>interface.3.met-perps-data-perps-gateway-frames-total-j27</code> | <code>metric</code> | <code>stable</code> | <code>services/platform/perps-data/internal/gateway/server.go:178</code> | <code>perps_gateway_frames_total</code> |
| <code>interface.3.met-perps-data-perps-gateway-graphql-duration-seconds-j27</code> | <code>metric</code> | <code>stable</code> | <code>services/platform/perps-data/internal/gateway/server.go:177</code> | <code>perps_gateway_graphql_duration_seconds</code> |
| <code>interface.3.met-perps-data-perps-gateway-resyncs-total-j27</code> | <code>metric</code> | <code>stable</code> | <code>services/platform/perps-data/internal/gateway/server.go:179</code> | <code>perps_gateway_resyncs_total</code> |
| <code>interface.3.met-perps-data-perps-gateway-sockets-j27</code> | <code>metric</code> | <code>stable</code> | <code>services/platform/perps-data/internal/gateway/server.go:175</code> | <code>perps_gateway_sockets</code> |
| <code>interface.3.met-perps-data-perps-gateway-subscriptions-j27</code> | <code>metric</code> | <code>stable</code> | <code>services/platform/perps-data/internal/gateway/server.go:176</code> | <code>perps_gateway_subscriptions</code> |
| <code>interface.3.met-perps-data-perps-http-requests-total-j27</code> | <code>metric</code> | <code>stable</code> | <code>services/platform/perps-data/internal/telemetry/metrics.go:40</code> | <code>perps_http_requests_total</code> |
| <code>interface.3.met-perps-data-perps-indexer-cursor-height-j27</code> | <code>metric</code> | <code>stable</code> | <code>services/platform/perps-data/internal/indexer/indexer.go:164</code> | <code>perps_indexer_cursor_height</code> |
| <code>interface.3.met-perps-data-perps-indexer-enum-unknown-total-j27</code> | <code>metric</code> | <code>stable</code> | <code>services/platform/perps-data/internal/decode/registry.go:272</code> | <code>perps_indexer_enum_unknown_total</code> |
| <code>interface.3.met-perps-data-perps-indexer-head-height-j27</code> | <code>metric</code> | <code>stable</code> | <code>services/platform/perps-data/internal/indexer/indexer.go:160</code> | <code>perps_indexer_head_height</code> |
| <code>interface.3.met-perps-data-perps-indexer-lag-blocks-j27</code> | <code>metric</code> | <code>stable</code> | <code>services/platform/perps-data/internal/indexer/indexer.go:168</code> | <code>perps_indexer_lag_blocks</code> |
| <code>interface.3.met-perps-data-perps-indexer-logs-total-j27</code> | <code>metric</code> | <code>stable</code> | <code>services/platform/perps-data/internal/indexer/indexer.go:172</code> | <code>perps_indexer_logs_total</code> |
| <code>interface.3.met-perps-data-perps-indexer-rewinds-total-j27</code> | <code>metric</code> | <code>stable</code> | <code>services/platform/perps-data/internal/indexer/indexer.go:176</code> | <code>perps_indexer_rewinds_total</code> |
| <code>interface.3.met-perps-data-perps-indexer-unknown-logs-total-j27</code> | <code>metric</code> | <code>stable</code> | <code>services/platform/perps-data/internal/decode/registry.go:268</code> | <code>perps_indexer_unknown_logs_total</code> |
| <code>interface.3.met-perps-data-perps-outbox-broker-down-j27</code> | <code>metric</code> | <code>stable</code> | <code>services/platform/perps-data/internal/publish/publisher.go:198</code> | <code>perps_outbox_broker_down</code> |
| <code>interface.3.met-perps-data-perps-outbox-head-age-seconds-j27</code> | <code>metric</code> | <code>stable</code> | <code>services/platform/perps-data/internal/publish/publisher.go:203</code> | <code>perps_outbox_head_age_seconds</code> |
| <code>interface.3.met-perps-data-perps-outbox-pending-j27</code> | <code>metric</code> | <code>stable</code> | <code>services/platform/perps-data/internal/indexer/indexer.go:180</code> | <code>perps_outbox_pending</code> |
| <code>interface.3.met-perps-data-perps-outbox-quarantined-total-j27</code> | <code>metric</code> | <code>stable</code> | <code>services/platform/perps-data/internal/publish/publisher.go:194</code> | <code>perps_outbox_quarantined_total</code> |
| <code>interface.3.met-perps-data-perps-portal-render-seconds-j27</code> | <code>metric</code> | <code>stable</code> | <code>services/platform/perps-data/internal/portal/server.go:94</code> | <code>perps_portal_render_seconds</code> |
| <code>interface.3.met-perps-data-perps-untrusted-proxy-headers-total-j27</code> | <code>metric</code> | <code>stable</code> | <code>services/platform/perps-data/internal/ratelimit/ratelimit.go:75</code> | <code>perps_untrusted_proxy_headers_total</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.3.go-perps-data-telemetry-metrics-servehttp-j27</code> | notes | 15 callers; the first 12 listed |
| <code>interface.3.go-perps-data-telemetry-new-j27</code> | notes | 23 callers; the first 12 listed |
| <code>interface.3.http-perps-data-perps-explorer-api-j27</code> | notes | reached over HTTP; no in-repo caller names the literal path |
| <code>interface.3.http-perps-data-perps-explorer-get-metrics-j27</code> | notes | reached over HTTP; no in-repo caller names the literal path |
| <code>interface.3.http-perps-data-perps-indexer-get-metrics-j27</code> | notes | reached over HTTP; no in-repo caller names the literal path |
| <code>interface.3.met-perps-data-perps-gateway-frames-total-j27</code> | notes | scraped from GET /metrics |
| <code>interface.3.met-perps-data-perps-gateway-graphql-duration-seconds-j27</code> | notes | scraped from GET /metrics |
| <code>interface.3.met-perps-data-perps-gateway-resyncs-total-j27</code> | notes | scraped from GET /metrics |
| <code>interface.3.met-perps-data-perps-gateway-sockets-j27</code> | notes | scraped from GET /metrics |
| <code>interface.3.met-perps-data-perps-gateway-subscriptions-j27</code> | notes | scraped from GET /metrics |
| <code>interface.3.met-perps-data-perps-http-requests-total-j27</code> | notes | scraped from GET /metrics |
| <code>interface.3.met-perps-data-perps-indexer-cursor-height-j27</code> | notes | scraped from GET /metrics |
| <code>interface.3.met-perps-data-perps-indexer-enum-unknown-total-j27</code> | notes | scraped from GET /metrics |
| <code>interface.3.met-perps-data-perps-indexer-head-height-j27</code> | notes | scraped from GET /metrics |
| <code>interface.3.met-perps-data-perps-indexer-lag-blocks-j27</code> | notes | scraped from GET /metrics |
| <code>interface.3.met-perps-data-perps-indexer-logs-total-j27</code> | notes | scraped from GET /metrics |
| <code>interface.3.met-perps-data-perps-indexer-rewinds-total-j27</code> | notes | scraped from GET /metrics |
| <code>interface.3.met-perps-data-perps-indexer-unknown-logs-total-j27</code> | notes | scraped from GET /metrics |
| <code>interface.3.met-perps-data-perps-outbox-broker-down-j27</code> | notes | scraped from GET /metrics |
| <code>interface.3.met-perps-data-perps-outbox-head-age-seconds-j27</code> | notes | scraped from GET /metrics |
| <code>interface.3.met-perps-data-perps-outbox-pending-j27</code> | notes | scraped from GET /metrics |
| <code>interface.3.met-perps-data-perps-outbox-quarantined-total-j27</code> | notes | scraped from GET /metrics |
| <code>interface.3.met-perps-data-perps-portal-render-seconds-j27</code> | notes | scraped from GET /metrics |
| <code>interface.3.met-perps-data-perps-untrusted-proxy-headers-total-j27</code> | notes | scraped from GET /metrics |

Contract context <code>contract_context.3.services-platform-perps-data-j97</code>:

| Field | Frozen rule |
|---|---|
| interface | 17 surfaces under services/platform/perps-data (1 cli_command, 1 env_var, 8 go_exported, 7 http_route; 16 change, 1 new) frozen at the anchors contracts/perps-data.md#cli, contracts/perps-data.md#configuration, contracts/perps-data.md#explorer, contracts/perps-data.md#gateway, contracts/perps-data.md#loadtest, contracts/perps-data.md#operations and more: perps-loadtest (services/platform/perps-data/cmd/perps-loadtest/main.go:21); PERPS_TRUSTED_PROXY_CIDRS (services/platform/perps-data/internal/ratelimit/ratelimit.go:30); Config.TrustedProxies []netip.Prefix (from PERPS_TRUSTED_PROXY_CIDRS) (services/platform/perps-data/internal/config/config.go (planned)); func Load(service Service) (*Config, error) (services/platform/perps-data/internal/config/config.go:76); type Options struct (services/platform/perps-data/internal/explorer/server.go:38); type Options struct (services/platform/perps-data/internal/gateway/server.go:93); var FeedSymbols = []string{"BTC", "ETH", "SOL", "AVAX", "LINK", "HYPE", "XRP", "ASTER", "T (services/platform/perps-data/loadtest/load.go:34); func FullProfile() Profile (services/platform/perps-data/loadtest/load.go:93); func ClientKey(r *http.Request) string (services/platform/perps-data/internal/ratelimit/ratelimit.go:123); func (s *SpotUsers) StreamItems(ctx context.Context, env delivery.Envelope) ([]SpotStreamI (services/platform/perps-data/internal/rest/spotusers.go:1409); and 7 more listed in interfaces |
| storage | no storage_key item in this context; the services/platform/perps-data items read or write state only through the keepers and stores frozen by the other domain-3 contexts |
| errors | no error_code item in this context; failures surface through the error registries frozen by the other domain-3 contexts |
| events | no event, precompile_event or sol_event item is named by this context; events of the handlers behind these items are frozen by the other domain-3 contexts |
| gas | not metered: no item in this context is a precompile method |
| activation | lands with tasks 3.41.2, 3.41.3, 3.45.2, 3.46.3, 3.46.4, 3.50.1, 3.56.1 in waves 1, 2, 3, 6 |
| invariants | each item keeps the kind, path and signature of its interface block once its landing task passes; a later change of any item needs a new decision of domain 3 |
| pause | no pause parameter governs these items; they are disabled only by reverting the touches of their landing tasks |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.3.488.1</code> | req | 3488.1 |
| <code>acceptance.3.488.1</code> | scenario | Given PERPS_TRUSTED_PROXY_CIDRS set to two CIDRs and then to not-a-cidr, when config.Load(Gateway) runs, then the first returns both prefixes and the second returns an error naming PERPS_TRUSTED_PROXY_CIDRS |
| <code>acceptance.3.488.1</code> | test | services/platform/perps-data/internal/config/config_test.go::TestLoadTrustedProxies (planned) |
| <code>acceptance.3.488.1</code> | qualification | UNRUN |
| <code>req.3488</code> | ac_1 | Load (services/platform/perps-data/internal/config/config.go:76) of the gateway with PERPS_TRUSTED_PROXY_CIDRS = 10.0.0.0/8, 192.168.1.0/24 returns both prefixes in Config.TrustedProxies, and with the value not-a-cidr returns an error naming the variable without panicking. |
| <code>acceptance.3.488.2</code> | req | 3488.2 |
| <code>acceptance.3.488.2</code> | scenario | Given a request from peer 10.1.2.3 with X-Real-IP 203.0.113.9, when ClientKey is called with [10.0.0.0/8] and with an empty list, then it returns ip:203.0.113.9 and ip:10.1.2.3 |
| <code>acceptance.3.488.2</code> | test | services/platform/perps-data/internal/ratelimit/ratelimit_test.go::TestClientKeyTrustedList (planned) |
| <code>acceptance.3.488.2</code> | qualification | UNRUN |
| <code>req.3488</code> | ac_2 | ClientKey (services/platform/perps-data/internal/ratelimit/ratelimit.go:123) honours X-Real-IP only when the TCP peer is inside the list it is passed, and keys by the peer with an empty list, as clientIP of services/platform/perps-data/internal/ratelimit/ratelimit.go does for the env list today. |
| <code>capability.3.50.1</code> | title | Parse PERPS_TRUSTED_PROXY_CIDRS in config.Load instead of a panicking package var |
| <code>capability.3.50.1</code> | reqs | 3488.1<br>3488.2 |

## pnl

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.3.go-perps-data-bigint-add-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/bigint/bigint.go:49</code> | <code>func Add(a, b string) (string, error)</code> |
| <code>interface.3.go-perps-data-bigint-errinvalidraw-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/bigint/bigint.go:24</code> | <code>ErrInvalidRaw = errors.New("bigint: invalid raw integer")</code> |
| <code>interface.3.go-perps-data-bigint-erroverflow-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/bigint/bigint.go:26</code> | <code>ErrOverflow = errors.New("bigint: 256-bit overflow")</code> |
| <code>interface.3.go-perps-data-bigint-int256max-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/bigint/bigint.go:18</code> | <code>Int256Max = "57896044618658097711785492504343953926634992332820282019728792003956564819967"</code> |
| <code>interface.3.go-perps-data-bigint-int256min-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/bigint/bigint.go:19</code> | <code>Int256Min = "-57896044618658097711785492504343953926634992332820282019728792003956564819968"</code> |
| <code>interface.3.go-perps-data-bigint-muldiv-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/bigint/bigint.go:80</code> | <code>func MulDiv(a, b, divisor string) (string, error)</code> |
| <code>interface.3.go-perps-data-bigint-sub-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/bigint/bigint.go:62</code> | <code>func Sub(a, b string) (string, error)</code> |
| <code>interface.3.go-perps-data-bigint-uint256max-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/bigint/bigint.go:14</code> | <code>const Uint256Max = "115792089237316195423570985008687907853269984665640564039457584007913129639935"</code> |
| <code>interface.3.go-perps-data-bigint-validateraw-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/bigint/bigint.go:101</code> | <code>func ValidateRaw(raw string) error</code> |
| <code>interface.3.go-perps-data-bigint-validaterawint256-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/bigint/bigint.go:108</code> | <code>func ValidateRawInt256(raw string) error</code> |
| <code>interface.3.go-perps-data-pnl-entry-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/pnl/pnl.go:539</code> | <code>type Entry struct</code> |
| <code>interface.3.go-perps-data-pnl-fold-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/pnl/pnl.go:93</code> | <code>func Fold(prior Position, t Trade) (Position, error)</code> |
| <code>interface.3.go-perps-data-pnl-history-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/pnl/pnl.go:586</code> | <code>func History(ctx context.Context, q Querier, account common.Address, from int64, below Key, kinds []Kind, limit int) ([]Entry, error)</code> |
| <code>interface.3.go-perps-data-pnl-key-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/pnl/pnl.go:557</code> | <code>type Key struct</code> |
| <code>interface.3.go-perps-data-pnl-kind-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/pnl/pnl.go:509</code> | <code>type Kind string</code> |
| <code>interface.3.go-perps-data-pnl-leaderboard-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/pnl/pnl.go:437</code> | <code>type Leaderboard struct{}</code> |
| <code>interface.3.go-perps-data-pnl-position-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/pnl/pnl.go:52</code> | <code>type Position struct</code> |
| <code>interface.3.go-perps-data-pnl-rebuilddaily-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/pnl/pnl.go:384</code> | <code>func RebuildDaily(ctx context.Context, tx *sql.Tx, account common.Address, day time.Time) (Daily, error)</code> |
| <code>interface.3.go-perps-data-pnl-through-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/pnl/pnl.go:565</code> | <code>func Through(block int64) Key</code> |
| <code>interface.3.go-perps-data-pnl-totalpnl-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/pnl/pnl.go:204</code> | <code>func TotalPnl(realizedPnlUsdl, avgCostBasis, holdings, priceWad string) (string, error)</code> |
| <code>interface.3.go-perps-data-pnl-trade-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/pnl/pnl.go:67</code> | <code>type Trade struct</code> |

## portal

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.3.go-perps-data-portal-newpages-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/portal/pages.go:138</code> | <code>func NewPages(gatewayURL, explorerTxBase string, client *http.Client) (*Pages, error)</code> |
| <code>interface.3.go-perps-data-portal-newserver-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/portal/server.go:79</code> | <code>func NewServer(pages *Pages, gatewayURL string, metrics *telemetry.Metrics, client *http.Client) (*Server, error)</code> |
| <code>interface.3.go-perps-data-portal-server-servehttp-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/portal/server.go:143</code> | <code>func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request)</code> |
| <code>interface.3.http-perps-data-portal-get-accounts-address-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/portal/server.go:122</code> | <code>GET /accounts/{address} (portal account page)</code> |
| <code>interface.3.http-perps-data-portal-get-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/portal/server.go:112</code> | <code>GET /{$} (portal markets page)</code> |
| <code>interface.3.http-perps-data-portal-get-l135-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/portal/server.go:135</code> | <code>GET / (portal unknown page)</code> |
| <code>interface.3.http-perps-data-portal-get-leaderboard-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/portal/server.go:129</code> | <code>GET /leaderboard (portal)</code> |
| <code>interface.3.http-perps-data-portal-get-markets-symbol-j27</code> | <code>http_route</code> | <code>change</code> | <code>services/platform/perps-data/internal/portal/server.go:115</code> | <code>GET /markets/{symbol} (portal market page)</code> |
| <code>interface.3.http-perps-data-portal-get-metrics-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/portal/server.go:111</code> | <code>GET /metrics (portal)</code> |
| <code>interface.3.http-perps-data-portal-get-stats-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/portal/server.go:132</code> | <code>GET /stats (portal)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.3.http-perps-data-portal-get-j27</code> | notes | reached over HTTP; no in-repo caller names the literal path |
| <code>interface.3.http-perps-data-portal-get-l135-j27</code> | notes | reached over HTTP; no in-repo caller names the literal path |
| <code>interface.3.http-perps-data-portal-get-metrics-j27</code> | notes | reached over HTTP; no in-repo caller names the literal path |

Contract context <code>contract_context.3.services-platform-perps-data-j97</code>:

| Field | Frozen rule |
|---|---|
| interface | 17 surfaces under services/platform/perps-data (1 cli_command, 1 env_var, 8 go_exported, 7 http_route; 16 change, 1 new) frozen at the anchors contracts/perps-data.md#cli, contracts/perps-data.md#configuration, contracts/perps-data.md#explorer, contracts/perps-data.md#gateway, contracts/perps-data.md#loadtest, contracts/perps-data.md#operations and more: perps-loadtest (services/platform/perps-data/cmd/perps-loadtest/main.go:21); PERPS_TRUSTED_PROXY_CIDRS (services/platform/perps-data/internal/ratelimit/ratelimit.go:30); Config.TrustedProxies []netip.Prefix (from PERPS_TRUSTED_PROXY_CIDRS) (services/platform/perps-data/internal/config/config.go (planned)); func Load(service Service) (*Config, error) (services/platform/perps-data/internal/config/config.go:76); type Options struct (services/platform/perps-data/internal/explorer/server.go:38); type Options struct (services/platform/perps-data/internal/gateway/server.go:93); var FeedSymbols = []string{"BTC", "ETH", "SOL", "AVAX", "LINK", "HYPE", "XRP", "ASTER", "T (services/platform/perps-data/loadtest/load.go:34); func FullProfile() Profile (services/platform/perps-data/loadtest/load.go:93); func ClientKey(r *http.Request) string (services/platform/perps-data/internal/ratelimit/ratelimit.go:123); func (s *SpotUsers) StreamItems(ctx context.Context, env delivery.Envelope) ([]SpotStreamI (services/platform/perps-data/internal/rest/spotusers.go:1409); and 7 more listed in interfaces |
| storage | no storage_key item in this context; the services/platform/perps-data items read or write state only through the keepers and stores frozen by the other domain-3 contexts |
| errors | no error_code item in this context; failures surface through the error registries frozen by the other domain-3 contexts |
| events | no event, precompile_event or sol_event item is named by this context; events of the handlers behind these items are frozen by the other domain-3 contexts |
| gas | not metered: no item in this context is a precompile method |
| activation | lands with tasks 3.41.2, 3.41.3, 3.45.2, 3.46.3, 3.46.4, 3.50.1, 3.56.1 in waves 1, 2, 3, 6 |
| invariants | each item keeps the kind, path and signature of its interface block once its landing task passes; a later change of any item needs a new decision of domain 3 |
| pause | no pause parameter governs these items; they are disabled only by reverting the touches of their landing tasks |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.3.493.1</code> | req | 3493.1 |
| <code>acceptance.3.493.1</code> | scenario | Given a migrated test database with PERPS_DATA_TEST_DATABASE_URL naming a migrated test database with chain market DOGE id 18, when GET /api/markets/DOGE is served, then market.market_id is 18 |
| <code>acceptance.3.493.1</code> | test | services/platform/perps-data/internal/rest/markets_id_test.go::TestMarketRecordCarriesID (planned) |
| <code>acceptance.3.493.1</code> | qualification | UNRUN |
| <code>req.3493</code> | ac_1 | GET /api/markets/DOGE of the rest markets route (services/platform/perps-data/internal/rest/markets.go:51) answers market.market_id equal to the perps.markets market_id of DOGE, a field MarketRecord (services/platform/perps-data/internal/records/records.go:72) lacks at HEAD. |
| <code>acceptance.3.493.2</code> | req | 3493.2 |
| <code>acceptance.3.493.2</code> | scenario | Given a portal whose gateway answers GET /api/markets/DOGE with market_id 18 and GET /api/markets/NOPE with 404, when GET /markets/DOGE and GET /markets/NOPE are served, then the first renders with market 18 queries and the second answers 404 |
| <code>acceptance.3.493.2</code> | test | services/platform/perps-data/internal/portal/pages_test.go::TestPortalChainMarketPage (planned) |
| <code>acceptance.3.493.2</code> | qualification | UNRUN |
| <code>req.3493</code> | ac_2 | GET /markets/DOGE of the portal renders the page with the market_id 18 the markets answer carries, where the static map of services/platform/perps-data/internal/records/records.go:18-21 has no DOGE, and GET /markets/NOPE answers 404. |
| <code>capability.3.41.3</code> | title | Carry market_id on the market record and retire records.MarketIDs |
| <code>capability.3.41.3</code> | reqs | 3493.1<br>3493.2 |

## records

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.3.go-perps-data-records-decimal-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/records/records.go:585</code> | <code>func Decimal(raw sql.NullString) (*float64, error)</code> |
| <code>interface.3.go-perps-data-records-decimalorzero-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/records/records.go:596</code> | <code>func DecimalOrZero(raw sql.NullString) (float64, error)</code> |
| <code>interface.3.go-perps-data-records-funding8hsql-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/records/records.go:44</code> | <code>Funding8hSQL = "trim_scale(funding_rate * 0.000000000000000008)::text"</code> |
| <code>interface.3.go-perps-data-records-isomillis-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/records/records.go:47</code> | <code>ISOMillis = "2006-01-02T15:04:05.000Z"</code> |
| <code>interface.3.go-perps-data-records-lastfundingratehourlysql-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/records/records.go:41</code> | <code>LastFundingRateHourlySQL = "trim_scale(funding_rate * 0.000000000000000001)::text"</code> |
| <code>interface.3.go-perps-data-records-liquidationfill-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/records/records.go:172</code> | <code>const LiquidationFill = `EXISTS (SELECT 1 FROM perps.liquidations l</code> |
| <code>interface.3.go-perps-data-records-marketids-j27</code> | <code>go_exported</code> | <code>retire</code> | <code>services/platform/perps-data/internal/records/records.go:18</code> | <code>var MarketIDs = map[string]int64</code> |
| <code>interface.3.go-perps-data-records-marketrecord-j27</code> | <code>go_exported</code> | <code>change</code> | <code>services/platform/perps-data/internal/records/records.go:72</code> | <code>type MarketRecord struct</code> |
| <code>interface.3.go-perps-data-records-marketset-j27</code> | <code>go_exported</code> | <code>new</code> | <code>services/platform/perps-data/internal/records/marketset.go</code> (planned) | <code>type MarketSet struct; func NewMarketSet(db *sql.DB, ttl time.Duration) *MarketSet; func (s *MarketSet) ID(ctx context.Context, symbol string) (string, bool, error)</code> |
| <code>interface.3.go-perps-data-records-newsymbol-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/records/records.go:59</code> | <code>func NewSymbol(db *sql.DB) (*Symbol, error)</code> |
| <code>interface.3.go-perps-data-records-perptrade-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/records/records.go:143</code> | <code>type PerpTrade struct</code> |
| <code>interface.3.go-perps-data-records-secondsperhour-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/records/records.go:37</code> | <code>SecondsPerHour = 3600</code> |
| <code>interface.3.go-perps-data-records-spotscale-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/records/records.go:35</code> | <code>SpotScale = "1000000"</code> |
| <code>interface.3.go-perps-data-records-spottokenrank-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/records/records.go:26</code> | <code>var SpotTokenRank = map[string]int{"PAX": 0, "SID": 1}</code> |
| <code>interface.3.go-perps-data-records-symbol-health-j27</code> | <code>go_exported</code> | <code>change</code> | <code>services/platform/perps-data/internal/records/records.go:551</code> | <code>func (s *Symbol) Health(ctx context.Context, symbol string) (Health, error)</code> |
| <code>interface.3.go-perps-data-records-symbol-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/records/records.go:54</code> | <code>type Symbol struct</code> |
| <code>interface.3.go-perps-data-records-symbol-perpstats-j27</code> | <code>go_exported</code> | <code>change</code> | <code>services/platform/perps-data/internal/records/records.go:369</code> | <code>func (s *Symbol) PerpStats(ctx context.Context, symbol string) (PerpStats, error)</code> |
| <code>interface.3.go-perps-data-records-symbol-perptrades-j27</code> | <code>go_exported</code> | <code>change</code> | <code>services/platform/perps-data/internal/records/records.go:504</code> | <code>func (s *Symbol) PerpTrades(ctx context.Context, symbol string, limit int) (Trades[PerpTrade], error)</code> |
| <code>interface.3.go-perps-data-records-symbol-price-j27</code> | <code>go_exported</code> | <code>change</code> | <code>services/platform/perps-data/internal/records/records.go:313</code> | <code>func (s *Symbol) Price(ctx context.Context, symbol string) (Price, error)</code> |
| <code>interface.3.go-perps-data-records-symbol-stats-j27</code> | <code>go_exported</code> | <code>change</code> | <code>services/platform/perps-data/internal/records/records.go:353</code> | <code>func (s *Symbol) Stats(ctx context.Context, symbol string) (Stats, error)</code> |
| <code>interface.3.go-perps-data-records-symbol-trades-j27</code> | <code>go_exported</code> | <code>change</code> | <code>services/platform/perps-data/internal/records/records.go:450</code> | <code>func (s *Symbol) Trades(ctx context.Context, symbol string, limit int) (Trades[SpotTrade], error)</code> |
| <code>interface.3.go-perps-data-records-totals-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/records/records.go:101</code> | <code>type Totals struct</code> |
| <code>interface.3.go-perps-data-records-trades-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/records/records.go:153</code> | <code>type Trades[T any] struct</code> |
| <code>interface.3.go-perps-data-records-usdscale-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/records/records.go:32</code> | <code>USDScale = "1000000000000000000"</code> |
| <code>interface.3.go-perps-data-records-window24h-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/records/records.go:46</code> | <code>Window24h = 24 * time.Hour</code> |

Contract context <code>contract_context.3.perps-data-markets-j27</code>:

| Field | Frozen rule |
|---|---|
| interface | perps.markets, perps.fee_config and perps.market_tiers as the market set of every perps-data surface, refreshed from 0x1015 totalMarkets, getMarket, getMarketConfig, getMarginTiers and getFees |
| storage | perps.markets of services/platform/perps-data/migrations/0001_facts.sql:956 with the config columns of 0012, perps.market_tiers of 0014, perps.fee_config row 1 |
| errors | ErrChainValue for an undecodable or out-of-range chain answer; ErrUnknownSymbol for a symbol outside perps.markets |
| events | No chain event announces a listing; perps_markets_refresh_height and perps_markets_refresh_failures_total report the periodic refresh |
| gas | Off-chain service; no gas |
| activation | Seeded by migration 0005 and refreshed from the v6.12.0 height |
| invariants | Every served symbol is a perps.markets row; a refresh writes all markets, fees and tiers in one transaction or none |
| pause | No pause; a failed refresh keeps the previous rows |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.3.485.1</code> | req | 3485.1 |
| <code>acceptance.3.485.1</code> | scenario | Given a migrated test database with PERPS_DATA_TEST_DATABASE_URL naming a migrated test database with an extra chain market DOGE id 18, when MarketSet.ID is asked for BTC, DOGE and NOPE, then it returns 0 true, 18 true and false |
| <code>acceptance.3.485.1</code> | test | services/platform/perps-data/internal/records/marketset_test.go::TestMarketSetLoadsChainMarkets (planned) |
| <code>acceptance.3.485.1</code> | qualification | UNRUN |
| <code>req.3485</code> | ac_1 | MarketSet.ID returns the market_id of every perps.markets row, including a row with source chain absent from the static map of services/platform/perps-data/internal/records/records.go:18-21, and reports false for an unlisted symbol. |
| <code>acceptance.3.485.2</code> | req | 3485.2 |
| <code>acceptance.3.485.2</code> | scenario | Given a Symbol built with a 10 ms ttl and a market DOGE inserted afterwards, when Symbol.Price is called for DOGE after 20 ms, then it returns the DOGE price row and no ErrUnknownSymbol |
| <code>acceptance.3.485.2</code> | test | services/platform/perps-data/internal/records/marketset_test.go::TestSymbolResolvesInsertedMarket (planned) |
| <code>acceptance.3.485.2</code> | qualification | UNRUN |
| <code>req.3485</code> | ac_2 | Symbol.Price (services/platform/perps-data/internal/records/records.go:313) of a market inserted into perps.markets after NewSymbol (services/platform/perps-data/internal/records/records.go:59) answers its price row once the ttl passed instead of ErrUnknownSymbol (services/platform/perps-data/internal/records/records.go:51). |
| <code>capability.3.41.1</code> | title | Resolve symbols through a MarketSet loaded from perps.markets |
| <code>capability.3.41.1</code> | reqs | 3485.1<br>3485.2 |
| <code>resolution.17.607</code> | title | decision.345 binds: The perps-data market set follows perps.markets |
| <code>resolution.17.607</code> | applies_to | decision.345<br>task.3.41.1<br>task.3.41.3 |
| <code>resolution.17.607</code> | source | decision.345 in fragments/J27/decision.kvx; tasks from affects |

## replay

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.3.go-perps-data-replay-exit-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/replay/replay.go:130</code> | <code>func Exit(err error) int</code> |
| <code>interface.3.go-perps-data-replay-nodesource-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/replay/replay.go:152</code> | <code>type NodeSource struct</code> |
| <code>interface.3.go-perps-data-replay-refusal-error-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/replay/replay.go:71</code> | <code>func (r Refusal) Error() string { return string(r) }</code> |
| <code>interface.3.go-perps-data-replay-refusal-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/replay/replay.go:69</code> | <code>type Refusal string</code> |
| <code>interface.3.go-perps-data-replay-report-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/replay/replay.go:193</code> | <code>type Report struct</code> |
| <code>interface.3.go-perps-data-replay-request-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/replay/replay.go:75</code> | <code>type Request struct</code> |
| <code>interface.3.go-perps-data-replay-runner-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/replay/replay.go:174</code> | <code>type Runner struct</code> |
| <code>interface.3.go-perps-data-replay-runner-run-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/replay/replay.go:216</code> | <code>func (r *Runner) Run(ctx context.Context, req Request) (rep Report, err error)</code> |
| <code>interface.3.go-perps-data-replay-tocursor-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/replay/replay.go:32</code> | <code>const ToCursor int64 = -1</code> |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.3.504.1</code> | req | 3504.1 |
| <code>acceptance.3.504.1</code> | scenario | Given a migrated test database with PERPS_DATA_TEST_DATABASE_URL naming a migrated test database with facts of A and B, when snapshot runs for A's range, then the temporary projection copies hold only A's rows |
| <code>acceptance.3.504.1</code> | test | services/platform/perps-data/internal/replay/replay_test.go::TestReplaySnapshotScoped (planned) |
| <code>acceptance.3.504.1</code> | qualification | UNRUN |
| <code>req.3504</code> | ac_1 | snapshot (services/platform/perps-data/internal/replay/replay.go:432) of a range holding only account A copies no projection row of account B, and the replay of TestReplay (services/platform/perps-data/internal/replay/replay_test.go:274) still reports no diff. |
| <code>capability.3.45.6</code> | title | Copy only the range-keyed projection rows in the replay snapshot |
| <code>capability.3.45.6</code> | reqs | 3504.1 |

## rest

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.3.go-perps-data-rest-accountv2-register-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/rest/accountv2.go:38</code> | <code>func (s *AccountV2) Register(mux *http.ServeMux)</code> |
| <code>interface.3.go-perps-data-rest-markets-register-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/rest/markets.go:48</code> | <code>func (m *Markets) Register(mux *http.ServeMux)</code> |
| <code>interface.3.go-perps-data-rest-newaccountv2-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/rest/accountv2.go:28</code> | <code>func NewAccountV2(db *sql.DB, service *auth.Service, now func() time.Time) (*AccountV2, error)</code> |
| <code>interface.3.go-perps-data-rest-newmarkets-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/rest/markets.go:35</code> | <code>func NewMarkets(db *sql.DB, now func() time.Time) (*Markets, error)</code> |
| <code>interface.3.go-perps-data-rest-newspotusers-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/rest/spotusers.go:72</code> | <code>func NewSpotUsers(db *sql.DB, node SpotNode) (*SpotUsers, error)</code> |
| <code>interface.3.go-perps-data-rest-newsymbol-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/rest/symbol.go:37</code> | <code>func NewSymbol(db *sql.DB) (*Symbol, error)</code> |
| <code>interface.3.go-perps-data-rest-newvenue-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/rest/venue.go:20</code> | <code>func NewVenue(db *sql.DB) (*Venue, error)</code> |
| <code>interface.3.go-perps-data-rest-spotnode-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/rest/spotusers.go:32</code> | <code>type SpotNode interface</code> |
| <code>interface.3.go-perps-data-rest-spotstreamitem-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/rest/spotusers.go:1383</code> | <code>type SpotStreamItem struct</code> |
| <code>interface.3.go-perps-data-rest-spotusers-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/rest/spotusers.go:40</code> | <code>type SpotUsers struct</code> |
| <code>interface.3.go-perps-data-rest-spotusers-register-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/rest/spotusers.go:102</code> | <code>func (s *SpotUsers) Register(mux *http.ServeMux)</code> |
| <code>interface.3.go-perps-data-rest-spotusers-streamitems-j27</code> | <code>go_exported</code> | <code>change</code> | <code>services/platform/perps-data/internal/rest/spotusers.go:1409</code> | <code>func (s *SpotUsers) StreamItems(ctx context.Context, env delivery.Envelope) ([]SpotStreamItem, error)</code> |
| <code>interface.3.go-perps-data-rest-symbol-register-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/rest/symbol.go:48</code> | <code>func (s *Symbol) Register(mux *http.ServeMux)</code> |
| <code>interface.3.go-perps-data-rest-venue-register-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/rest/venue.go:27</code> | <code>func (v *Venue) Register(mux *http.ServeMux)</code> |
| <code>interface.3.http-perps-data-rest-delete-v2-alerts-id-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/rest/accountv2.go:43</code> | <code>DELETE /v2/alerts/{id} (bearer token)</code> |
| <code>interface.3.http-perps-data-rest-get-api-markets-health-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/rest/markets.go:50</code> | <code>GET /api/markets/health</code> |
| <code>interface.3.http-perps-data-rest-get-api-markets-j27</code> | <code>http_route</code> | <code>change</code> | <code>services/platform/perps-data/internal/rest/markets.go:49</code> | <code>GET /api/markets</code> |
| <code>interface.3.http-perps-data-rest-get-api-markets-symbol-j27</code> | <code>http_route</code> | <code>change</code> | <code>services/platform/perps-data/internal/rest/markets.go:51</code> | <code>GET /api/markets/{symbol}</code> |
| <code>interface.3.http-perps-data-rest-get-api-open-interest-and-get-api-markets-open-interest-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/rest/venue.go:30</code> | <code>GET /api/open-interest and GET /api/markets/open-interest</code> |
| <code>interface.3.http-perps-data-rest-get-api-orderbook-symbol-and-get-api-markets-orderbook-symbol-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/rest/venue.go:29</code> | <code>GET /api/orderbook/{symbol} and GET /api/markets/orderbook/{symbol}</code> |
| <code>interface.3.http-perps-data-rest-get-api-symbol-route-j27</code> | <code>http_route</code> | <code>change</code> | <code>services/platform/perps-data/internal/rest/symbol.go:49</code> | <code>GET /api/{symbol}/{route...} (price, stats, perp_stats, trades, perp_trades, health)</code> |
| <code>interface.3.http-perps-data-rest-get-api-ticker-symbol-and-get-api-markets-ticker-symbol-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/rest/venue.go:31</code> | <code>GET /api/ticker/{symbol} and GET /api/markets/ticker/{symbol}</code> |
| <code>interface.3.http-perps-data-rest-get-api-tickers-and-get-api-markets-tickers-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/rest/venue.go:32</code> | <code>GET /api/tickers and GET /api/markets/tickers</code> |
| <code>interface.3.http-perps-data-rest-get-api-totals-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/rest/markets.go:52</code> | <code>GET /api/totals</code> |
| <code>interface.3.http-perps-data-rest-get-v1-leaderboards-kind-j27</code> | <code>http_route</code> | <code>change</code> | <code>services/platform/perps-data/internal/rest/spotusers.go:116</code> | <code>GET /v1/leaderboards/{kind} (kind pnl, volume, win-rate)</code> |
| <code>interface.3.http-perps-data-rest-get-v1-users-address-holdings-positions-orders-trades-trades-stats-closed-legs-pnl-summary-pnl-identity-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/rest/spotusers.go:114</code> | <code>GET /v1/users/{address}/{holdings&#124;positions&#124;orders&#124;trades&#124;trades/stats&#124;closed-legs&#124;pnl/summary&#124;pnl&#124;identity}</code> |
| <code>interface.3.http-perps-data-rest-get-v2-leaderboard-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/rest/accountv2.go:40</code> | <code>GET /v2/leaderboard</code> |
| <code>interface.3.http-perps-data-rest-get-v2-user-address-daily-pnl-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/rest/accountv2.go:39</code> | <code>GET /v2/user/{address}/daily-pnl</code> |
| <code>interface.3.http-perps-data-rest-post-v2-alerts-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/rest/accountv2.go:42</code> | <code>POST /v2/alerts (bearer token)</code> |
| <code>interface.3.http-perps-data-rest-post-v2-competitions-id-join-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/rest/accountv2.go:46</code> | <code>POST /v2/competitions/{id}/join (bearer token)</code> |
| <code>interface.3.http-perps-data-rest-post-v2-referrals-apply-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/rest/accountv2.go:45</code> | <code>POST /v2/referrals/apply (bearer token)</code> |
| <code>interface.3.http-perps-data-rest-post-v2-referrals-generate-j27</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/rest/accountv2.go:44</code> | <code>POST /v2/referrals/generate (bearer token)</code> |

Contract context <code>contract_context.3.services-platform-perps-data-j97</code>:

| Field | Frozen rule |
|---|---|
| interface | 17 surfaces under services/platform/perps-data (1 cli_command, 1 env_var, 8 go_exported, 7 http_route; 16 change, 1 new) frozen at the anchors contracts/perps-data.md#cli, contracts/perps-data.md#configuration, contracts/perps-data.md#explorer, contracts/perps-data.md#gateway, contracts/perps-data.md#loadtest, contracts/perps-data.md#operations and more: perps-loadtest (services/platform/perps-data/cmd/perps-loadtest/main.go:21); PERPS_TRUSTED_PROXY_CIDRS (services/platform/perps-data/internal/ratelimit/ratelimit.go:30); Config.TrustedProxies []netip.Prefix (from PERPS_TRUSTED_PROXY_CIDRS) (services/platform/perps-data/internal/config/config.go (planned)); func Load(service Service) (*Config, error) (services/platform/perps-data/internal/config/config.go:76); type Options struct (services/platform/perps-data/internal/explorer/server.go:38); type Options struct (services/platform/perps-data/internal/gateway/server.go:93); var FeedSymbols = []string{"BTC", "ETH", "SOL", "AVAX", "LINK", "HYPE", "XRP", "ASTER", "T (services/platform/perps-data/loadtest/load.go:34); func FullProfile() Profile (services/platform/perps-data/loadtest/load.go:93); func ClientKey(r *http.Request) string (services/platform/perps-data/internal/ratelimit/ratelimit.go:123); func (s *SpotUsers) StreamItems(ctx context.Context, env delivery.Envelope) ([]SpotStreamI (services/platform/perps-data/internal/rest/spotusers.go:1409); and 7 more listed in interfaces |
| storage | no storage_key item in this context; the services/platform/perps-data items read or write state only through the keepers and stores frozen by the other domain-3 contexts |
| errors | no error_code item in this context; failures surface through the error registries frozen by the other domain-3 contexts |
| events | no event, precompile_event or sol_event item is named by this context; events of the handlers behind these items are frozen by the other domain-3 contexts |
| gas | not metered: no item in this context is a precompile method |
| activation | lands with tasks 3.41.2, 3.41.3, 3.45.2, 3.46.3, 3.46.4, 3.50.1, 3.56.1 in waves 1, 2, 3, 6 |
| invariants | each item keeps the kind, path and signature of its interface block once its landing task passes; a later change of any item needs a new decision of domain 3 |
| pause | no pause parameter governs these items; they are disabled only by reverting the touches of their landing tasks |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.3.489.1</code> | req | 3489.1 |
| <code>acceptance.3.489.1</code> | scenario | Given a migrated test database with PERPS_DATA_TEST_DATABASE_URL naming a migrated test database with chain market DOGE and its price fact, when GET /api/DOGE/price and GET /api/NOPE/price are served, then they answer 200 with the DOGE price and 404 Unknown symbol: NOPE |
| <code>acceptance.3.489.1</code> | test | services/platform/perps-data/internal/rest/symbol_test.go::TestSymbolRoutesChainMarket (planned) |
| <code>acceptance.3.489.1</code> | qualification | UNRUN |
| <code>req.3489</code> | ac_1 | GET /api/DOGE/price served by Register (services/platform/perps-data/internal/rest/symbol.go:49) answers 200 once DOGE is a perps.markets row and 404 Unknown symbol: NOPE for an unlisted symbol. |
| <code>acceptance.3.498.1</code> | req | 3498.1 |
| <code>acceptance.3.498.1</code> | scenario | Given a migrated test database with PERPS_DATA_TEST_DATABASE_URL naming a migrated test database with three perps.spot_leaderboard volume rows, when GET /v1/leaderboards/volume?limit=2 is served, then it answers ranks 1 and 2 in order |
| <code>acceptance.3.498.1</code> | test | services/platform/perps-data/internal/rest/spotusers_test.go::TestSpotLeaderboardProjection (planned) |
| <code>acceptance.3.498.1</code> | qualification | UNRUN |
| <code>req.3498</code> | ac_1 | GET /v1/leaderboards/volume answers the rows of perps.spot_leaderboard kind volume in rank order, without folding legs at request time. |
| <code>acceptance.3.505.1</code> | req | 3505.1 |
| <code>acceptance.3.505.1</code> | scenario | Given a migrated test database with PERPS_DATA_TEST_DATABASE_URL naming a migrated test database and an account with three spot trades, when StreamItems handles a fourth swap, then it returns SpotTrade, SpotPosition, SpotPnlSummary and SpotClosedLeg after exactly one fold |
| <code>acceptance.3.505.1</code> | test | services/platform/perps-data/internal/rest/spotusers_test.go::TestStreamItemsSingleFold (planned) |
| <code>acceptance.3.505.1</code> | qualification | UNRUN |
| <code>req.3505</code> | ac_1 | StreamItems (services/platform/perps-data/internal/rest/spotusers.go:1409) returns for a spot swap the same four items as before with one fold of the account history, counted by the folds counter of spotBoard (services/platform/perps-data/internal/rest/spotusers.go:62). |
| <code>capability.3.41.2</code> | title | Serve the symbol REST routes from the MarketSet |
| <code>capability.3.41.2</code> | reqs | 3489.1 |
| <code>capability.3.46.3</code> | title | Serve the spot leaderboards from the perps.spot_leaderboard projection |
| <code>capability.3.46.3</code> | reqs | 3498.1 |
| <code>capability.3.46.4</code> | title | Fold an account once per spot trade in SpotUsers.StreamItems |
| <code>capability.3.46.4</code> | reqs | 3505.1 |

## storage

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.3.go-perps-data-migrations-fs-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/migrations/embed.go:11</code> | <code>var FS embed.FS</code> |
| <code>interface.3.go-perps-data-store-block-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/store/facts.go:45</code> | <code>type Block struct</code> |
| <code>interface.3.go-perps-data-store-cursor-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/store/facts.go:56</code> | <code>func Cursor(ctx context.Context, db *sql.DB) (uint64, bool, error)</code> |
| <code>interface.3.go-perps-data-store-errleaseheld-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/store/facts.go:73</code> | <code>var ErrLeaseHeld = errors.New("store: writer lease held by another instance")</code> |
| <code>interface.3.go-perps-data-store-fence-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/store/facts.go:80</code> | <code>func Fence(ctx context.Context, tx *sql.Tx, name, holder string) error</code> |
| <code>interface.3.go-perps-data-store-foldspotboard-j27</code> | <code>go_exported</code> | <code>new</code> | <code>services/platform/perps-data/internal/store/spotboard.go</code> (planned) | <code>func FoldSpotBoard(ctx context.Context, tx *sql.Tx, rows []decode.Row) error</code> |
| <code>interface.3.go-perps-data-store-migrate-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/store/migrate.go:60</code> | <code>func Migrate(ctx context.Context, db *sql.DB) error</code> |
| <code>interface.3.go-perps-data-store-open-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/store/migrate.go:45</code> | <code>func Open(databaseURL string) (*sql.DB, error)</code> |
| <code>interface.3.go-perps-data-store-policy-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/store/facts.go:102</code> | <code>func Policy(chain uint64) delivery.Policy</code> |
| <code>interface.3.go-perps-data-store-project-j27</code> | <code>go_exported</code> | <code>change</code> | <code>services/platform/perps-data/internal/store/projections.go:65</code> | <code>func Project(ctx context.Context, tx *sql.Tx, rows []decode.Row) error</code> |
| <code>interface.3.go-perps-data-store-rebuild-j27</code> | <code>go_exported</code> | <code>change</code> | <code>services/platform/perps-data/internal/store/facts.go:1198</code> | <code>func Rebuild(ctx context.Context, tx *sql.Tx, from, to int64) error</code> |
| <code>interface.3.go-perps-data-store-snapshotter-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/store/projections.go:644</code> | <code>type Snapshotter struct{}</code> |
| <code>interface.3.go-perps-data-store-usdl-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/store/projections.go:25</code> | <code>var USDL = common.HexToAddress("0x85FcD13735F4309833A503EE804ea32395851479")</code> |
| <code>interface.3.go-perps-data-store-writeblock-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/store/facts.go:590</code> | <code>func WriteBlock(ctx context.Context, tx *sql.Tx, block Block, rows *decode.Result) error</code> |
| <code>interface.3.go-perps-data-store-writefacts-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/store/facts.go:506</code> | <code>func WriteFacts(ctx context.Context, tx *sql.Tx, block Block, rows *decode.Result) ([]decode.Row, error)</code> |
| <code>interface.3.key-perps-data-table-account-ledger-j27</code> | <code>storage_key</code> | <code>stable</code> | <code>services/platform/perps-data/migrations/0002_projections.sql:178</code> | <code>CREATE VIEW perps.account_ledger AS</code> |
| <code>interface.3.key-perps-data-table-account-margin-j27</code> | <code>storage_key</code> | <code>stable</code> | <code>services/platform/perps-data/migrations/0002_projections.sql:71</code> | <code>CREATE TABLE perps.account_margin</code> |
| <code>interface.3.key-perps-data-table-adl-events-j27</code> | <code>storage_key</code> | <code>stable</code> | <code>services/platform/perps-data/migrations/0001_facts.sql:489</code> | <code>CREATE TABLE perps.adl_events</code> |
| <code>interface.3.key-perps-data-table-auth-challenges-j27</code> | <code>storage_key</code> | <code>stable</code> | <code>services/platform/perps-data/migrations/0004_accounts.sql:1</code> | <code>CREATE TABLE perps.auth_challenges</code> |
| <code>interface.3.key-perps-data-table-backstop-liquidations-j27</code> | <code>storage_key</code> | <code>stable</code> | <code>services/platform/perps-data/migrations/0001_facts.sql:430</code> | <code>CREATE TABLE perps.backstop_liquidations</code> |
| <code>interface.3.key-perps-data-table-blocks-j27</code> | <code>storage_key</code> | <code>stable</code> | <code>services/platform/perps-data/migrations/0001_facts.sql:934</code> | <code>CREATE TABLE perps.blocks</code> |
| <code>interface.3.key-perps-data-table-chain-counters-j27</code> | <code>storage_key</code> | <code>new</code> | <code>services/platform/perps-data/migrations/0013_chain_counters.sql</code> (planned) | <code>CREATE TABLE perps.chain_counters (name text PRIMARY KEY, value bigint NOT NULL)</code> |
| <code>interface.3.key-perps-data-table-collateral-events-j27</code> | <code>storage_key</code> | <code>stable</code> | <code>services/platform/perps-data/migrations/0001_facts.sql:32</code> | <code>CREATE TABLE perps.collateral_events</code> |
| <code>interface.3.key-perps-data-table-competition-entries-j27</code> | <code>storage_key</code> | <code>stable</code> | <code>services/platform/perps-data/migrations/0004_accounts.sql:53</code> | <code>CREATE TABLE perps.competition_entries</code> |
| <code>interface.3.key-perps-data-table-competitions-j27</code> | <code>storage_key</code> | <code>stable</code> | <code>services/platform/perps-data/migrations/0004_accounts.sql:41</code> | <code>CREATE TABLE perps.competitions</code> |
| <code>interface.3.key-perps-data-table-cursors-j27</code> | <code>storage_key</code> | <code>stable</code> | <code>services/platform/perps-data/migrations/0001_facts.sql:943</code> | <code>CREATE TABLE perps.cursors</code> |
| <code>interface.3.key-perps-data-table-delivery-inbox-j27</code> | <code>storage_key</code> | <code>stable</code> | <code>services/platform/perps-data/migrations/0003_delivery.sql:39</code> | <code>CREATE TABLE perps.delivery_inbox</code> |
| <code>interface.3.key-perps-data-table-delivery-outbox-j27</code> | <code>storage_key</code> | <code>stable</code> | <code>services/platform/perps-data/migrations/0003_delivery.sql:1</code> | <code>CREATE TABLE perps.delivery_outbox</code> |
| <code>interface.3.key-perps-data-table-delivery-quarantine-j27</code> | <code>storage_key</code> | <code>stable</code> | <code>services/platform/perps-data/migrations/0003_delivery.sql:48</code> | <code>CREATE TABLE perps.delivery_quarantine</code> |
| <code>interface.3.key-perps-data-table-fee-config-j27</code> | <code>storage_key</code> | <code>stable</code> | <code>services/platform/perps-data/migrations/0001_facts.sql:971</code> | <code>CREATE TABLE perps.fee_config</code> |
| <code>interface.3.key-perps-data-table-feed-keys-j27</code> | <code>storage_key</code> | <code>stable</code> | <code>services/platform/perps-data/migrations/0001_facts.sql:521</code> | <code>CREATE TABLE perps.feed_keys</code> |
| <code>interface.3.key-perps-data-table-fees-j27</code> | <code>storage_key</code> | <code>stable</code> | <code>services/platform/perps-data/migrations/0001_facts.sql:221</code> | <code>CREATE TABLE perps.fees</code> |
| <code>interface.3.key-perps-data-table-fills-j27</code> | <code>storage_key</code> | <code>stable</code> | <code>services/platform/perps-data/migrations/0001_facts.sql:103</code> | <code>CREATE TABLE perps.fills</code> |
| <code>interface.3.key-perps-data-table-funding-payments-j27</code> | <code>storage_key</code> | <code>stable</code> | <code>services/platform/perps-data/migrations/0001_facts.sql:337</code> | <code>CREATE TABLE perps.funding_payments</code> |
| <code>interface.3.key-perps-data-table-funding-rates-j27</code> | <code>storage_key</code> | <code>stable</code> | <code>services/platform/perps-data/migrations/0001_facts.sql:282</code> | <code>CREATE TABLE perps.funding_rates</code> |
| <code>interface.3.key-perps-data-table-funding-settlements-j27</code> | <code>storage_key</code> | <code>stable</code> | <code>services/platform/perps-data/migrations/0001_facts.sql:309</code> | <code>CREATE TABLE perps.funding_settlements</code> |
| <code>interface.3.key-perps-data-table-index-prices-j27</code> | <code>storage_key</code> | <code>stable</code> | <code>services/platform/perps-data/migrations/0001_facts.sql:549</code> | <code>CREATE TABLE perps.index_prices</code> |
| <code>interface.3.key-perps-data-table-insurance-events-j27</code> | <code>storage_key</code> | <code>stable</code> | <code>services/platform/perps-data/migrations/0001_facts.sql:459</code> | <code>CREATE TABLE perps.insurance_events</code> |
| <code>interface.3.key-perps-data-table-keeper-cycles-j27</code> | <code>storage_key</code> | <code>stable</code> | <code>services/platform/perps-data/migrations/0002_projections.sql:167</code> | <code>CREATE TABLE perps.keeper_cycles</code> |
| <code>interface.3.key-perps-data-table-leaderboard-j27</code> | <code>storage_key</code> | <code>change</code> | <code>services/platform/perps-data/migrations/0002_projections.sql:124</code> | <code>CREATE TABLE perps.leaderboard</code> |
| <code>interface.3.key-perps-data-table-liquidations-j27</code> | <code>storage_key</code> | <code>stable</code> | <code>services/platform/perps-data/migrations/0001_facts.sql:398</code> | <code>CREATE TABLE perps.liquidations</code> |
| <code>interface.3.key-perps-data-table-mark-prices-j27</code> | <code>storage_key</code> | <code>stable</code> | <code>services/platform/perps-data/migrations/0001_facts.sql:254</code> | <code>CREATE TABLE perps.mark_prices</code> |
| <code>interface.3.key-perps-data-table-market-snapshots-j27</code> | <code>storage_key</code> | <code>stable</code> | <code>services/platform/perps-data/migrations/0002_projections.sql:144</code> | <code>CREATE TABLE perps.market_snapshots</code> |
| <code>interface.3.key-perps-data-table-market-stats-rolling-j27</code> | <code>storage_key</code> | <code>change</code> | <code>services/platform/perps-data/migrations/0002_projections.sql:108</code> | <code>CREATE TABLE perps.market_stats_rolling</code> |
| <code>interface.3.key-perps-data-table-market-tiers-j27</code> | <code>storage_key</code> | <code>new</code> | <code>services/platform/perps-data/migrations/0014_market_tiers.sql</code> (planned) | <code>CREATE TABLE perps.market_tiers (market_id, tier, lower_bound_usd, max_leverage, maintenance_bps, deduction_usd, refreshed_block)</code> |
| <code>interface.3.key-perps-data-table-markets-j27</code> | <code>storage_key</code> | <code>change</code> | <code>services/platform/perps-data/migrations/0001_facts.sql:956</code> | <code>CREATE TABLE perps.markets</code> |
| <code>interface.3.key-perps-data-table-open-interest-j27</code> | <code>storage_key</code> | <code>stable</code> | <code>services/platform/perps-data/migrations/0001_facts.sql:369</code> | <code>CREATE TABLE perps.open_interest</code> |
| <code>interface.3.key-perps-data-table-order-events-j27</code> | <code>storage_key</code> | <code>stable</code> | <code>services/platform/perps-data/migrations/0001_facts.sql:61</code> | <code>CREATE TABLE perps.order_events</code> |
| <code>interface.3.key-perps-data-table-orders-j27</code> | <code>storage_key</code> | <code>stable</code> | <code>services/platform/perps-data/migrations/0002_projections.sql:23</code> | <code>CREATE TABLE perps.orders</code> |
| <code>interface.3.key-perps-data-table-position-events-j27</code> | <code>storage_key</code> | <code>stable</code> | <code>services/platform/perps-data/migrations/0001_facts.sql:139</code> | <code>CREATE TABLE perps.position_events</code> |
| <code>interface.3.key-perps-data-table-positions-current-j27</code> | <code>storage_key</code> | <code>stable</code> | <code>services/platform/perps-data/migrations/0002_projections.sql:1</code> | <code>CREATE TABLE perps.positions_current</code> |
| <code>interface.3.key-perps-data-table-projection-seeds-j27</code> | <code>storage_key</code> | <code>stable</code> | <code>services/platform/perps-data/migrations/0011_projection_seeds.sql:1</code> | <code>CREATE TABLE perps.projection_seeds</code> |
| <code>interface.3.key-perps-data-table-protocol-events-j27</code> | <code>storage_key</code> | <code>stable</code> | <code>services/platform/perps-data/migrations/0001_facts.sql:878</code> | <code>CREATE TABLE perps.protocol_events</code> |
| <code>interface.3.key-perps-data-table-protocol-snapshots-j27</code> | <code>storage_key</code> | <code>stable</code> | <code>services/platform/perps-data/migrations/0002_projections.sql:155</code> | <code>CREATE TABLE perps.protocol_snapshots</code> |
| <code>interface.3.key-perps-data-table-raw-logs-j27</code> | <code>storage_key</code> | <code>stable</code> | <code>services/platform/perps-data/migrations/0001_facts.sql:850</code> | <code>CREATE TABLE perps.raw_logs</code> |
| <code>interface.3.key-perps-data-table-referral-tiers-j27</code> | <code>storage_key</code> | <code>stable</code> | <code>services/platform/perps-data/migrations/0004_accounts.sql:33</code> | <code>CREATE TABLE perps.referral_tiers</code> |
| <code>interface.3.key-perps-data-table-referrals-j27</code> | <code>storage_key</code> | <code>stable</code> | <code>services/platform/perps-data/migrations/0004_accounts.sql:23</code> | <code>CREATE TABLE perps.referrals</code> |
| <code>interface.3.key-perps-data-table-spot-admin-j27</code> | <code>storage_key</code> | <code>stable</code> | <code>services/platform/perps-data/migrations/0001_facts.sql:753</code> | <code>CREATE TABLE perps.spot_admin</code> |
| <code>interface.3.key-perps-data-table-spot-fee-events-j27</code> | <code>storage_key</code> | <code>stable</code> | <code>services/platform/perps-data/migrations/0001_facts.sql:619</code> | <code>CREATE TABLE perps.spot_fee_events</code> |
| <code>interface.3.key-perps-data-table-spot-leaderboard-j27</code> | <code>storage_key</code> | <code>new</code> | <code>services/platform/perps-data/migrations/0015_spot_leaderboard.sql</code> (planned) | <code>CREATE TABLE perps.spot_leaderboard (kind, rank, account, value, updated_block)</code> |
| <code>interface.3.key-perps-data-table-spot-order-events-j27</code> | <code>storage_key</code> | <code>stable</code> | <code>services/platform/perps-data/migrations/0001_facts.sql:650</code> | <code>CREATE TABLE perps.spot_order_events</code> |
| <code>interface.3.key-perps-data-table-spot-orders-j27</code> | <code>storage_key</code> | <code>stable</code> | <code>services/platform/perps-data/migrations/0002_projections.sql:48</code> | <code>CREATE TABLE perps.spot_orders</code> |
| <code>interface.3.key-perps-data-table-spot-share-transfers-j27</code> | <code>storage_key</code> | <code>stable</code> | <code>services/platform/perps-data/migrations/0001_facts.sql:790</code> | <code>CREATE TABLE perps.spot_share_transfers</code> |
| <code>interface.3.key-perps-data-table-spot-swaps-j27</code> | <code>storage_key</code> | <code>stable</code> | <code>services/platform/perps-data/migrations/0001_facts.sql:580</code> | <code>CREATE TABLE perps.spot_swaps</code> |
| <code>interface.3.key-perps-data-table-spot-token-registry-j27</code> | <code>storage_key</code> | <code>stable</code> | <code>services/platform/perps-data/migrations/0001_facts.sql:724</code> | <code>CREATE TABLE perps.spot_token_registry</code> |
| <code>interface.3.key-perps-data-table-spot-token-seen-j27</code> | <code>storage_key</code> | <code>stable</code> | <code>services/platform/perps-data/migrations/0002_projections.sql:138</code> | <code>CREATE TABLE perps.spot_token_seen</code> |
| <code>interface.3.key-perps-data-table-spot-vault-events-j27</code> | <code>storage_key</code> | <code>stable</code> | <code>services/platform/perps-data/migrations/0001_facts.sql:689</code> | <code>CREATE TABLE perps.spot_vault_events</code> |
| <code>interface.3.key-perps-data-table-trade-settlements-j27</code> | <code>storage_key</code> | <code>stable</code> | <code>services/platform/perps-data/migrations/0001_facts.sql:185</code> | <code>CREATE TABLE perps.trade_settlements</code> |
| <code>interface.3.key-perps-data-table-tx-actions-j27</code> | <code>storage_key</code> | <code>stable</code> | <code>services/platform/perps-data/migrations/0001_facts.sql:905</code> | <code>CREATE TABLE perps.tx_actions</code> |
| <code>interface.3.key-perps-data-table-usdl-transfers-j27</code> | <code>storage_key</code> | <code>stable</code> | <code>services/platform/perps-data/migrations/0001_facts.sql:822</code> | <code>CREATE TABLE perps.usdl_transfers</code> |
| <code>interface.3.key-perps-data-table-user-alerts-j27</code> | <code>storage_key</code> | <code>stable</code> | <code>services/platform/perps-data/migrations/0004_accounts.sql:11</code> | <code>CREATE TABLE perps.user_alerts</code> |
| <code>interface.3.key-perps-data-table-user-pnl-daily-j27</code> | <code>storage_key</code> | <code>stable</code> | <code>services/platform/perps-data/migrations/0002_projections.sql:96</code> | <code>CREATE TABLE perps.user_pnl_daily</code> |
| <code>interface.3.key-perps-data-table-user-stats-j27</code> | <code>storage_key</code> | <code>stable</code> | <code>services/platform/perps-data/migrations/0002_projections.sql:81</code> | <code>CREATE TABLE perps.user_stats</code> |
| <code>interface.3.key-perps-data-table-vaults-j27</code> | <code>storage_key</code> | <code>stable</code> | <code>services/platform/perps-data/migrations/0001_facts.sql:6</code> | <code>CREATE TABLE perps.vaults</code> |
| <code>interface.3.key-perps-data-table-writer-lease-j27</code> | <code>storage_key</code> | <code>stable</code> | <code>services/platform/perps-data/migrations/0001_facts.sql:950</code> | <code>CREATE TABLE perps.writer_lease</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.3.go-perps-data-store-block-j27</code> | notes | 17 callers; the first 12 listed |
| <code>interface.3.go-perps-data-store-migrate-j27</code> | notes | 23 callers; the first 12 listed |
| <code>interface.3.go-perps-data-store-open-j27</code> | notes | 27 callers; the first 12 listed |
| <code>interface.3.go-perps-data-store-usdl-j27</code> | notes | 33 callers; the first 12 listed |
| <code>interface.3.go-perps-data-store-writeblock-j27</code> | notes | 14 callers; the first 12 listed |

Contract context <code>contract_context.3.perps-data-markets-j27</code>:

| Field | Frozen rule |
|---|---|
| interface | perps.markets, perps.fee_config and perps.market_tiers as the market set of every perps-data surface, refreshed from 0x1015 totalMarkets, getMarket, getMarketConfig, getMarginTiers and getFees |
| storage | perps.markets of services/platform/perps-data/migrations/0001_facts.sql:956 with the config columns of 0012, perps.market_tiers of 0014, perps.fee_config row 1 |
| errors | ErrChainValue for an undecodable or out-of-range chain answer; ErrUnknownSymbol for a symbol outside perps.markets |
| events | No chain event announces a listing; perps_markets_refresh_height and perps_markets_refresh_failures_total report the periodic refresh |
| gas | Off-chain service; no gas |
| activation | Seeded by migration 0005 and refreshed from the v6.12.0 height |
| invariants | Every served symbol is a perps.markets row; a refresh writes all markets, fees and tiers in one transaction or none |
| pause | No pause; a failed refresh keeps the previous rows |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.3.481.1</code> | req | 3481.1 |
| <code>acceptance.3.481.1</code> | scenario | Given the embedded migrations with 0012_markets_config.sql, when loadSchema folds every file, then perps.markets lists the seven new columns after refreshed_block with their declared types |
| <code>acceptance.3.481.1</code> | test | services/platform/perps-data/migrations/schema_test.go::TestSchemaFiles (existing) |
| <code>acceptance.3.481.1</code> | qualification | UNRUN |
| <code>req.3481</code> | ac_1 | TestSchemaFiles (services/platform/perps-data/migrations/schema_test.go:473) reads perps.markets with tick_size, lot_size, tier_class, impact_notional_usd, oi_cap_usd, funding_cap_ppm_per_hour and listed_at after refreshed_block, the amounts typed perps.uint256 as the fact_columns rule of services/platform/perps-data/migrations/schema_test.go:497-501 requires. |
| <code>acceptance.3.481.2</code> | req | 3481.2 |
| <code>acceptance.3.481.2</code> | scenario | Given an empty test database with PERPS_DATA_TEST_DATABASE_URL naming a migrated test database, when Migrate applies 0001 through 0012, then information_schema lists the perps.markets columns ending in tick_size, lot_size, tier_class, impact_notional_usd, oi_cap_usd, funding_cap_ppm_per_hour, listed_at |
| <code>acceptance.3.481.2</code> | test | services/platform/perps-data/internal/store/migrate_test.go::TestMigrate (existing) |
| <code>acceptance.3.481.2</code> | qualification | UNRUN |
| <code>req.3481</code> | ac_2 | TestMigrate (services/platform/perps-data/internal/store/migrate_test.go:274) applies every embedded migration to an empty database and reads the perps.markets columns through information_schema with the seven config columns after refreshed_block. |
| <code>acceptance.3.482.1</code> | req | 3482.1 |
| <code>acceptance.3.482.1</code> | scenario | Given a migrated test database with PERPS_DATA_TEST_DATABASE_URL naming a migrated test database, when two blocks and three tx_actions rows are inserted and one block deleted, then perps.chain_counters reads blocks = 1 and tx_actions = 3 |
| <code>acceptance.3.482.1</code> | test | services/platform/perps-data/internal/store/chain_counters_test.go::TestChainCounters (planned) |
| <code>acceptance.3.482.1</code> | qualification | UNRUN |
| <code>req.3482</code> | ac_1 | After Migrate (services/platform/perps-data/internal/store/migrate.go:60) and two inserts into perps.blocks, three into perps.tx_actions and one block delete, perps.chain_counters holds blocks 1 and tx_actions 3, equal to count(*) of each table. |
| <code>acceptance.3.483.1</code> | req | 3483.1 |
| <code>acceptance.3.483.1</code> | scenario | Given a migrated test database with PERPS_DATA_TEST_DATABASE_URL naming a migrated test database, when a tier row is inserted for market 0 and another for market 4096, then the first is stored and the second fails with a foreign-key violation |
| <code>acceptance.3.483.1</code> | test | services/platform/perps-data/internal/store/market_tiers_test.go::TestMarketTiersTable (planned) |
| <code>acceptance.3.483.1</code> | qualification | UNRUN |
| <code>req.3483</code> | ac_1 | After Migrate (services/platform/perps-data/internal/store/migrate.go:60) perps.market_tiers has primary key (market_id, tier), accepts a tier row for seeded market 0 of services/platform/perps-data/migrations/0005_markets_seed.sql:1-19 and refuses a row for market_id 4096 with SQLSTATE 23503. |
| <code>acceptance.3.484.1</code> | req | 3484.1 |
| <code>acceptance.3.484.1</code> | scenario | Given a migrated test database with PERPS_DATA_TEST_DATABASE_URL naming a migrated test database, when the keys of perps.spot_leaderboard and perps.spot_account_legs are read from pg_index, then they are (kind, rank), (kind, account) and (account) |
| <code>acceptance.3.484.1</code> | test | services/platform/perps-data/internal/store/spot_leaderboard_test.go::TestSpotLeaderboardTables (planned) |
| <code>acceptance.3.484.1</code> | qualification | UNRUN |
| <code>req.3484</code> | ac_1 | After Migrate (services/platform/perps-data/internal/store/migrate.go:60) perps.spot_leaderboard has primary key (kind, rank) and unique key (kind, account), and perps.spot_account_legs has primary key account. |
| <code>acceptance.3.487.1</code> | req | 3487.1 |
| <code>acceptance.3.487.1</code> | scenario | Given a migrated test database with PERPS_DATA_TEST_DATABASE_URL naming a migrated test database and one BTC fill at t0, when Project folds a block at t0 + 25 h with no BTC fill, then the BTC row of perps.market_stats_rolling has volume_24h 0 and trades_24h 0 |
| <code>acceptance.3.487.1</code> | test | services/platform/perps-data/internal/store/projections_test.go::TestMarketStatsWindowExpiry (planned) |
| <code>acceptance.3.487.1</code> | qualification | UNRUN |
| <code>req.3487</code> | ac_1 | A market with one fill at t0 reads volume_24h 0 in perps.market_stats_rolling after Project (services/platform/perps-data/internal/store/projections.go:65) of a block at t0 + 25 h that holds no fill of that market. |
| <code>acceptance.3.492.1</code> | req | 3492.1 |
| <code>acceptance.3.492.1</code> | scenario | Given a migrated test database with PERPS_DATA_TEST_DATABASE_URL naming a migrated test database and spot swaps of two accounts, when Project folds the blocks, then perps.spot_leaderboard ranks both accounts with the values the in-process spotBoard fold returns |
| <code>acceptance.3.492.1</code> | test | services/platform/perps-data/internal/store/spotboard_test.go::TestFoldSpotBoard (planned) |
| <code>acceptance.3.492.1</code> | qualification | UNRUN |
| <code>req.3492</code> | ac_1 | After Project (services/platform/perps-data/internal/store/projections.go:65) of two blocks with spot swaps of accounts A and B, perps.spot_leaderboard ranks them by realized and volume with the values the per-request fold of services/platform/perps-data/internal/rest/spotusers.go:62 computes for the same facts. |
| <code>acceptance.3.502.1</code> | req | 3502.1 |
| <code>acceptance.3.502.1</code> | scenario | Given a migrated test database with PERPS_DATA_TEST_DATABASE_URL naming a migrated test database with facts of accounts A and B, when Rebuild runs for a range holding only A's facts, then B's projection rows are unchanged and A's equal a full refold |
| <code>acceptance.3.502.1</code> | test | services/platform/perps-data/internal/store/facts_test.go::TestRebuildScoped (planned) |
| <code>acceptance.3.502.1</code> | qualification | UNRUN |
| <code>req.3502</code> | ac_1 | Rebuild (services/platform/perps-data/internal/store/facts.go:1198) of a range that touches only account A leaves the projection rows of account B byte-identical, including updated_block, and yields for A the rows a fresh fold of all blocks yields. |
| <code>acceptance.3.503.1</code> | req | 3503.1 |
| <code>acceptance.3.503.1</code> | scenario | Given a migrated test database with PERPS_DATA_TEST_DATABASE_URL naming a migrated test database and ten blocks of closes for five accounts, when Project folds them, then perps.leaderboard equals a full Rebuild and rows of untouched accounts keep their updated values |
| <code>acceptance.3.503.1</code> | test | services/platform/perps-data/internal/store/projections_test.go::TestLeaderboardIncremental (planned) |
| <code>acceptance.3.503.1</code> | qualification | UNRUN |
| <code>req.3503</code> | ac_1 | After Project (services/platform/perps-data/internal/store/projections.go:65) of ten blocks perps.leaderboard equals the rows (pnl.Leaderboard{}).Rebuild writes for the same facts, while each block rewrote only the rows of its own accounts. |
| <code>capability.3.42.1</code> | title | Add the chain market-config columns to perps.markets in migration 0012 |
| <code>capability.3.42.1</code> | reqs | 3481.1<br>3481.2 |
| <code>capability.3.42.8</code> | title | Add perps.market_tiers for the chain margin tiers (migration 0014) |
| <code>capability.3.42.8</code> | reqs | 3483.1 |
| <code>capability.3.44.1</code> | title | Expire rolling market stats of markets without fills in the window |
| <code>capability.3.44.1</code> | reqs | 3487.1 |
| <code>capability.3.45.1</code> | title | Maintain block and transaction counters in perps.chain_counters (migration 0013) |
| <code>capability.3.45.1</code> | reqs | 3482.1 |
| <code>capability.3.45.4</code> | title | Rebuild perps.leaderboard only for the accounts a block changes |
| <code>capability.3.45.4</code> | reqs | 3503.1 |
| <code>capability.3.45.5</code> | title | Scope store.Rebuild to the accounts and markets of the rewritten range |
| <code>capability.3.45.5</code> | reqs | 3502.1 |
| <code>capability.3.46.1</code> | title | Add perps.spot_leaderboard for the projected spot boards (migration 0015) |
| <code>capability.3.46.1</code> | reqs | 3484.1 |
| <code>capability.3.46.2</code> | title | Fold spot legs into perps.spot_account_legs and perps.spot_leaderboard in Project |
| <code>capability.3.46.2</code> | reqs | 3492.1 |
| <code>resolution.17.604</code> | title | decision.342 binds: App tests run on node:test with tsx; service tests on go test |
| <code>resolution.17.604</code> | applies_to | decision.342<br>task.3.42.1<br>task.3.48.1 |
| <code>resolution.17.604</code> | source | decision.342 in fragments/J27/decision.kvx; tasks from affects |
| <code>resolution.17.610</code> | title | decision.348 binds: Spot order fills are valued gross |
| <code>resolution.17.610</code> | applies_to | decision.348<br>task.3.46.2 |
| <code>resolution.17.610</code> | source | decision.348 in fragments/J27/decision.kvx; tasks from affects |
| <code>resolution.17.611</code> | title | decision.349 binds: NativeSwap stays excluded from PnL |
| <code>resolution.17.611</code> | applies_to | decision.349<br>task.3.46.2 |
| <code>resolution.17.611</code> | source | decision.349 in fragments/J27/decision.kvx; tasks from affects |

## websocket

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.3.err-perps-data-ws-invalid-json-j27</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/perps-data/internal/ws/protocol.go:34</code> | <code>invalid_json</code> |
| <code>interface.3.err-perps-data-ws-missing-topic-j27</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/perps-data/internal/ws/protocol.go:35</code> | <code>missing_topic</code> |
| <code>interface.3.err-perps-data-ws-subscribe-rate-limit-j27</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/perps-data/internal/ws/protocol.go:38</code> | <code>subscribe_rate_limit</code> |
| <code>interface.3.err-perps-data-ws-too-many-subscriptions-j27</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/perps-data/internal/ws/protocol.go:39</code> | <code>too_many_subscriptions</code> |
| <code>interface.3.err-perps-data-ws-unknown-event-j27</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/perps-data/internal/ws/protocol.go:36</code> | <code>unknown_event</code> |
| <code>interface.3.err-perps-data-ws-unknown-topic-j27</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/perps-data/internal/ws/protocol.go:37</code> | <code>unknown_topic</code> |
| <code>interface.3.go-perps-data-feeds-book-notionallevels-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/feeds/book.go:207</code> | <code>func (b *Book) NotionalLevels(depth int) (asks, bids []Level)</code> |
| <code>interface.3.go-perps-data-feeds-checksumevery-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/feeds/topics.go:50</code> | <code>ChecksumEvery  = 5 * time.Second</code> |
| <code>interface.3.go-perps-data-feeds-frame-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/feeds/topics.go:73</code> | <code>type Frame struct</code> |
| <code>interface.3.go-perps-data-feeds-level-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/feeds/book.go:40</code> | <code>type Level [2]float64</code> |
| <code>interface.3.go-perps-data-feeds-livelevels-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/feeds/book.go:27</code> | <code>LiveLevels = 20</code> |
| <code>interface.3.go-perps-data-feeds-loadbook-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/feeds/book.go:88</code> | <code>func LoadBook(ctx context.Context, db *sql.DB, market string) (*Book, error)</code> |
| <code>interface.3.go-perps-data-feeds-newproviders-j27</code> | <code>go_exported</code> | <code>change</code> | <code>services/platform/perps-data/internal/feeds/topics.go:178</code> | <code>func NewProviders(ctx context.Context, db *sql.DB, logger *slog.Logger) (*Providers, error)</code> |
| <code>interface.3.go-perps-data-feeds-oicontractssql-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/feeds/topics.go:1090</code> | <code>const OIContractsSQL = `(SELECT COALESCE(sum(p.size_usd / p.entry_price), 0) FROM perps.positions_current p</code> |
| <code>interface.3.go-perps-data-feeds-providers-held-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/feeds/topics.go:484</code> | <code>func (p *Providers) Held(topic string) (json.RawMessage, bool)</code> |
| <code>interface.3.go-perps-data-feeds-providers-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/feeds/topics.go:143</code> | <code>type Providers struct</code> |
| <code>interface.3.go-perps-data-feeds-providers-load-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/feeds/topics.go:452</code> | <code>func (p *Providers) Load(ctx context.Context, topic string) error</code> |
| <code>interface.3.go-perps-data-feeds-providers-message-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/feeds/topics.go:593</code> | <code>func (p *Providers) Message(ctx context.Context, subject string, env delivery.Envelope) ([]Frame, []string, error)</code> |
| <code>interface.3.go-perps-data-feeds-providers-run-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/feeds/topics.go:929</code> | <code>func (p *Providers) Run(ctx context.Context, emit func(Frame)) error</code> |
| <code>interface.3.go-perps-data-feeds-providers-snapshot-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/feeds/topics.go:502</code> | <code>func (p *Providers) Snapshot(ctx context.Context, topic string) (json.RawMessage, error)</code> |
| <code>interface.3.go-perps-data-feeds-providers-sockettopics-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/feeds/topics.go:251</code> | <code>func (p *Providers) SocketTopics(path string) ([]string, bool)</code> |
| <code>interface.3.go-perps-data-feeds-providers-subscribed-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/feeds/topics.go:273</code> | <code>func (p *Providers) Subscribed(topic string)</code> |
| <code>interface.3.go-perps-data-feeds-providers-unsubscribed-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/feeds/topics.go:281</code> | <code>func (p *Providers) Unsubscribed(topic string)</code> |
| <code>interface.3.go-perps-data-feeds-topicchainblocks-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/feeds/topics.go:28</code> | <code>TopicChainBlocks   = "chain@blocks"</code> |
| <code>interface.3.go-perps-data-feeds-topicchaintxs-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/feeds/topics.go:29</code> | <code>TopicChainTxs      = "chain@transactions"</code> |
| <code>interface.3.go-perps-data-feeds-topicmarketsall-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/feeds/topics.go:26</code> | <code>TopicMarketsAll    = "markets@all"</code> |
| <code>interface.3.go-perps-data-feeds-topicmarketstotals-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/feeds/topics.go:27</code> | <code>TopicMarketsTotals = "markets@totals"</code> |
| <code>interface.3.go-perps-data-ws-clientframe-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/ws/protocol.go:46</code> | <code>type ClientFrame struct</code> |
| <code>interface.3.go-perps-data-ws-dataframe-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/ws/protocol.go:122</code> | <code>type DataFrame struct</code> |
| <code>interface.3.go-perps-data-ws-defaultlimits-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/ws/protocol.go:140</code> | <code>func DefaultLimits() Limits</code> |
| <code>interface.3.go-perps-data-ws-errorcode-error-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/ws/protocol.go:42</code> | <code>func (c ErrorCode) Error() string { return string(c) }</code> |
| <code>interface.3.go-perps-data-ws-errunknowntopic-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/ws/protocol.go:37</code> | <code>ErrUnknownTopic         ErrorCode = "unknown_topic"</code> |
| <code>interface.3.go-perps-data-ws-event-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/ws/protocol.go:21</code> | <code>type Event string</code> |
| <code>interface.3.go-perps-data-ws-eventsubscribe-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/ws/protocol.go:24</code> | <code>EventSubscribe   Event = "subscribe"</code> |
| <code>interface.3.go-perps-data-ws-eventunsubscribe-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/ws/protocol.go:25</code> | <code>EventUnsubscribe Event = "unsubscribe"</code> |
| <code>interface.3.go-perps-data-ws-frame-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/ws/hub.go:64</code> | <code>type Frame struct</code> |
| <code>interface.3.go-perps-data-ws-hub-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/ws/hub.go:101</code> | <code>type Hub struct</code> |
| <code>interface.3.go-perps-data-ws-hub-leave-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/ws/hub.go:296</code> | <code>func (h *Hub) Leave(s *Socket)</code> |
| <code>interface.3.go-perps-data-ws-hub-listen-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/ws/hub.go:161</code> | <code>func (h *Hub) Listen(fn func(subject string, env delivery.Envelope)) (stop func())</code> |
| <code>interface.3.go-perps-data-ws-hub-listenloss-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/ws/hub.go:181</code> | <code>func (h *Hub) ListenLoss(lost func()) (stop func())</code> |
| <code>interface.3.go-perps-data-ws-hub-run-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/ws/hub.go:320</code> | <code>func (h *Hub) Run(ctx context.Context, nc *nats.Conn) error</code> |
| <code>interface.3.go-perps-data-ws-hub-subscribe-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/ws/hub.go:219</code> | <code>func (h *Hub) Subscribe(s *Socket, f ClientFrame)</code> |
| <code>interface.3.go-perps-data-ws-hub-unsubscribe-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/ws/hub.go:284</code> | <code>func (h *Hub) Unsubscribe(s *Socket, topic string)</code> |
| <code>interface.3.go-perps-data-ws-newhub-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/ws/hub.go:137</code> | <code>func NewHub(provider Provider, chain uint64, snapshotID string, logger *slog.Logger) (*Hub, error)</code> |
| <code>interface.3.go-perps-data-ws-newserver-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/ws/protocol.go:178</code> | <code>func NewServer(snapshotID string, served func(topic string) bool, limits Limits) (*Server, error)</code> |
| <code>interface.3.go-perps-data-ws-parseclientframe-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/ws/protocol.go:55</code> | <code>func ParseClientFrame(raw []byte, served func(topic string) bool) (ClientFrame, error)</code> |
| <code>interface.3.go-perps-data-ws-server-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/ws/protocol.go:159</code> | <code>type Server struct</code> |
| <code>interface.3.go-perps-data-ws-server-servehttp-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/ws/protocol.go:206</code> | <code>func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request)</code> |
| <code>interface.3.go-perps-data-ws-socket-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/ws/protocol.go:261</code> | <code>type Socket struct</code> |
| <code>interface.3.go-perps-data-ws-socket-subscriptions-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/ws/protocol.go:342</code> | <code>func (s *Socket) Subscriptions() int</code> |
| <code>interface.3.go-perps-data-ws-spotuser-closeall-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/ws/spotuser.go:100</code> | <code>func (s *SpotUser) CloseAll()</code> |
| <code>interface.3.go-perps-data-ws-spotuser-deliver-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/ws/spotuser.go:67</code> | <code>func (s *SpotUser) Deliver(stream, address string, data json.RawMessage) error</code> |
| <code>interface.3.go-perps-data-ws-spotuser-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/ws/spotuser.go:24</code> | <code>type SpotUser struct</code> |
| <code>interface.3.go-perps-data-ws-spotuser-servehttp-j27</code> | <code>go_exported</code> | <code>stable</code> | <code>services/platform/perps-data/internal/ws/spotuser.go:108</code> | <code>func (s *SpotUser) ServeHTTP(w http.ResponseWriter, r *http.Request)</code> |
| <code>interface.3.met-perps-data-perps-gateway-markets-j27</code> | <code>metric</code> | <code>new</code> | <code>services/platform/perps-data/internal/feeds/topics.go</code> (planned) | <code>perps_gateway_markets (gauge: symbols in the served market set)</code> |
| <code>interface.3.ws-perps-data-event-ping-j27</code> | <code>ws_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/ws/protocol.go:26</code> | <code>{"event":"ping","topic":"&lt;topic&gt;"}</code> |
| <code>interface.3.ws-perps-data-event-subscribe-j27</code> | <code>ws_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/ws/protocol.go:24</code> | <code>{"event":"subscribe","topic":"&lt;topic&gt;"}</code> |
| <code>interface.3.ws-perps-data-event-unsubscribe-j27</code> | <code>ws_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/ws/protocol.go:25</code> | <code>{"event":"unsubscribe","topic":"&lt;topic&gt;"}</code> |
| <code>interface.3.ws-perps-data-spotuser-streams-j27</code> | <code>ws_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/ws/spotuser.go:22</code> | <code>spot user streams "trade", "position", "pnl", "order", "closed-leg" on GET /v1/ws (limit 50 subscriptions, queue 64)</code> |
| <code>interface.3.ws-perps-data-topic-chain-blocks-j27</code> | <code>ws_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/feeds/topics.go:28</code> | <code>chain@blocks</code> |
| <code>interface.3.ws-perps-data-topic-chain-transactions-j27</code> | <code>ws_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/feeds/topics.go:29</code> | <code>chain@transactions</code> |
| <code>interface.3.ws-perps-data-topic-markets-all-j27</code> | <code>ws_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/feeds/topics.go:26</code> | <code>markets@all</code> |
| <code>interface.3.ws-perps-data-topic-markets-totals-j27</code> | <code>ws_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/feeds/topics.go:27</code> | <code>markets@totals</code> |
| <code>interface.3.ws-perps-data-topic-symbol-market-j27</code> | <code>ws_route</code> | <code>change</code> | <code>services/platform/perps-data/internal/feeds/topics.go:37</code> | <code>&lt;SYMBOL&gt;@market</code> |
| <code>interface.3.ws-perps-data-topic-symbol-orderbook-j27</code> | <code>ws_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/feeds/topics.go:38</code> | <code>&lt;SYMBOL&gt;@orderbook</code> |
| <code>interface.3.ws-perps-data-topic-symbol-perp-orderbook-j27</code> | <code>ws_route</code> | <code>change</code> | <code>services/platform/perps-data/internal/feeds/topics.go:34</code> | <code>&lt;SYMBOL&gt;@perp_orderbook</code> |
| <code>interface.3.ws-perps-data-topic-symbol-perp-stats-j27</code> | <code>ws_route</code> | <code>change</code> | <code>services/platform/perps-data/internal/feeds/topics.go:36</code> | <code>&lt;SYMBOL&gt;@perp_stats</code> |
| <code>interface.3.ws-perps-data-topic-symbol-perp-trade-j27</code> | <code>ws_route</code> | <code>change</code> | <code>services/platform/perps-data/internal/feeds/topics.go:35</code> | <code>&lt;SYMBOL&gt;@perp_trade</code> |
| <code>interface.3.ws-perps-data-topic-symbol-stats-j27</code> | <code>ws_route</code> | <code>change</code> | <code>services/platform/perps-data/internal/feeds/topics.go:40</code> | <code>&lt;SYMBOL&gt;@stats</code> |
| <code>interface.3.ws-perps-data-topic-symbol-trade-j27</code> | <code>ws_route</code> | <code>stable</code> | <code>services/platform/perps-data/internal/feeds/topics.go:39</code> | <code>&lt;SYMBOL&gt;@trade</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.3.go-perps-data-feeds-providers-load-j27</code> | notes | 24 callers; the first 12 listed |
| <code>interface.3.go-perps-data-feeds-providers-run-j27</code> | notes | 35 callers; the first 12 listed |
| <code>interface.3.go-perps-data-ws-hub-run-j27</code> | notes | 51 callers; the first 12 listed |
| <code>interface.3.ws-perps-data-topic-symbol-market-j27</code> | notes | symbol set read once from perps.markets at services/platform/perps-data/internal/feeds/topics.go:174 |
| <code>interface.3.ws-perps-data-topic-symbol-orderbook-j27</code> | notes | spot symbols PAX and SID only (services/platform/perps-data/internal/feeds/topics.go:59) |
| <code>interface.3.ws-perps-data-topic-symbol-perp-orderbook-j27</code> | notes | symbol set read once from perps.markets at services/platform/perps-data/internal/feeds/topics.go:174 |
| <code>interface.3.ws-perps-data-topic-symbol-perp-stats-j27</code> | notes | symbol set read once from perps.markets at services/platform/perps-data/internal/feeds/topics.go:174 |
| <code>interface.3.ws-perps-data-topic-symbol-perp-trade-j27</code> | notes | symbol set read once from perps.markets at services/platform/perps-data/internal/feeds/topics.go:174 |
| <code>interface.3.ws-perps-data-topic-symbol-stats-j27</code> | notes | symbol set read once from perps.markets at services/platform/perps-data/internal/feeds/topics.go:174 |
| <code>interface.3.ws-perps-data-topic-symbol-trade-j27</code> | notes | spot symbols PAX and SID only (services/platform/perps-data/internal/feeds/topics.go:59) |

Contract context <code>contract_context.3.perps-data-gateway-2-j27</code>:

| Field | Frozen rule |
|---|---|
| interface | The gateway GraphQL schema, REST routes and websocket topics of services/platform/perps-data served to the app and the portal |
| storage | Projections of services/platform/perps-data/migrations/0002_projections.sql and the spot leaderboard of 0015 |
| errors | GraphQL validation errors for retired roots; HTTP 404 Unknown symbol for a symbol outside the market set |
| events | Websocket topics of internal/feeds/topics.go per symbol and the markets, chain and spot channels |
| gas | Off-chain service; no gas |
| activation | Served from the gateway deploy; no activation height |
| invariants | Retired roots are refused at validation; every topic symbol is in the market set |
| pause | No pause |

Contract context <code>contract_context.3.perps-data-gateway-3-j27</code>:

| Field | Frozen rule |
|---|---|
| interface | The gateway GraphQL schema, REST routes and websocket topics of services/platform/perps-data served to the app and the portal |
| storage | Projections of services/platform/perps-data/migrations/0002_projections.sql and the spot leaderboard of 0015 |
| errors | GraphQL validation errors for retired roots; HTTP 404 Unknown symbol for a symbol outside the market set |
| events | Websocket topics of internal/feeds/topics.go per symbol and the markets, chain and spot channels |
| gas | Off-chain service; no gas |
| activation | Served from the gateway deploy; no activation height |
| invariants | Retired roots are refused at validation; every topic symbol is in the market set |
| pause | No pause |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.3.494.1</code> | req | 3494.1 |
| <code>acceptance.3.494.1</code> | scenario | Given providers started on a migrated test database with PERPS_DATA_TEST_DATABASE_URL naming a migrated test database and a DOGE row inserted afterwards, when one MarketInterval passes, then subscribing to the DOGE market topic succeeds and perps_gateway_markets is one higher |
| <code>acceptance.3.494.1</code> | test | services/platform/perps-data/internal/feeds/topics_test.go::TestTopicsReloadMarkets (planned) |
| <code>acceptance.3.494.1</code> | qualification | UNRUN |
| <code>req.3494</code> | ac_1 | A market row inserted after NewProviders (services/platform/perps-data/internal/feeds/topics.go:178) becomes a subscribable market topic after one MarketInterval tick and perps_gateway_markets reports the new count. |
| <code>capability.3.41.4</code> | title | Reload the websocket market set on MarketInterval and gauge its size |
| <code>capability.3.41.4</code> | reqs | 3494.1 |
