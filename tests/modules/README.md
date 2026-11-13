# Kernel module tests

This directory holds C tests and shared helpers for the native kernel modules
inside the Paxeer network: assets, escrow, budgets, streams, services, oracle
validation, perpetual markets, spot trading and governance. The tests exercise
module codecs, authorization, execution, balances, receipts and state recovery.
It also contains a trading-payload vector exporter and a Python runner for
metered-stream checks.

## Layout

The source files are directly in this directory, with no subdirectories.

| File | Purpose |
| --- | --- |
| `test_asset_registry.c` | Asset registration, pause controls, supply binding, record codecs and restored state. |
| `test_asset_account_grant.c` | Account opening and authority grants through signed activities. |
| `test_asset_state.c` | Asset state roots, balances and supply totals. |
| `test_asset_transfer.c` | Signed native sends and balance reconciliation. |
| `test_asset_deposit.c` | Deposit API refusal and unchanged balances. |
| `test_asset_withdraw.c` | Withdrawal API refusal, record restoration and native supply conservation. |
| `test_asset_withdraw_activity.c` | Shared native activity execution helpers and custody-activity refusal cases. |
| `test_asset_reserve.c` | Reserve codec checks and native supply conservation. |
| `test_asset_payloads.h` | Asset payload encoding and decoding checks. |
| `test_asset_retirement.h` | Shared snapshot restoration and state-proof helpers. |
| `test_budget_create.c` | Budget creation, funding and activity dispatch. |
| `test_budget_period.c` | Period rollover and expiry maintenance. |
| `test_budget_spend.c` | Spending limits and balance updates. |
| `test_budget_delegate.c` | Delegate authorization, spending limits and payload validation. |
| `test_budget_close.c` | Budget lifecycle, revocation, replay and snapshot restoration. |
| `test_escrow_open.c` | Escrow creation and custody funding. |
| `test_escrow_capture.c` | Capture execution, receipts and snapshot restoration. |
| `test_escrow_timeout.c` | Release, timeout handling and sweep capacity. |
| `test_escrow_dispute.c` | Dispute resolution and settlement. |
| `test_escrow_invariants.c` | Escrow custody and balance invariants. |
| `test_escrow_audit.h` | Shared custody, rollback and recovery checks. |
| `test_stream_open.c` | Stream opening, top-ups, rate changes and payload validation. |
| `test_stream_accrue.c` | Accrual boundaries and record codecs. |
| `test_stream_meter.c` | Meter attestations, dispatch, accrual windows, replicas and snapshot recovery. |
| `test_stream_settle.c` | Settlement, refusal cases and metered payments. |
| `test_stream_lifecycle.c` | Pause, resume, close, rate changes and atomicity. |
| `test_service_offer.c` | Offers, agreements and payload validation. |
| `test_service_commit.c` | Task commitments, progress, abandonment and retry authorization. |
| `test_service_attest.c` | Signed execution attestations and idempotency binding. |
| `test_service_deliver.c` | Delivery records and availability checks. |
| `test_service_acceptance.c` | Acceptance, rejection, default outcomes and restored deliveries. |
| `test_service_dispute.c` | Dispute handling, custody preservation and refusal cases. |
| `test_service_helpers.h` | Shared service contexts, activity encoding, signatures and event checks. |
| `test_oracle_intake.c` | Oracle authorization, accepted observations and balance-write restrictions. |
| `test_oracle_checks.c` | Staleness, price bounds, deviation and observation storage. |
| `test_oracle_halt.c` | Market halts, protective actions and committed recovery. |
| `test_perps_market.c` | Perpetual market creation and validation. |
| `test_perps_book.c` | Perpetual order placement, matching and cancellation. |
| `test_perps_position.c` | Margin reservations, position changes and recovery. |
| `test_perps_funding.c` | Funding intervals and settlement. |
| `test_perps_liquidate.c` | Position liquidation and balance accounting. |
| `test_perps_insurance.c` | Insurance and automatic deleveraging. |
| `test_perps_repairs.h` | Shared price, profit-and-loss, margin, funding and liquidation checks. |
| `test_perps_oracle_repairs.h` | Shared oracle-state and halted-market policy checks. |
| `test_spot_book.c` | Spot market creation, matching, cancellation, supply conservation and restored identities. |
| `test_governance_audit.c` | Emergency pause controls, proposal bounds and committed state recovery. |
| `governance_audit.mk` | Build and run rules for the governance checks. |
| `dump_trading_vectors.c` | Canonical perpetual and spot payloads exported as JSON using kernel codecs. |
| `paxeer_x_metered_end_boundary.py` | Runs the prebuilt metered-stream executable and checks its reported cases. |

## Build and test

Run commands from the repository root. The [root Makefile](../../Makefile)
builds the C executables and their kernel library dependencies. Most module
targets also build the Rust programs runtime through `programs-build`.
These builds use a C compiler, Cargo and OpenSSL development libraries.

To build a module executable without running it, use its file target. For
example, the asset registry and metered-stream executables are built with:

```sh
make build/tests/test_asset_registry
make build/tests/test_stream_meter
```

The following Makefile targets build and run the corresponding tests:

| Coverage | Commands |
| --- | --- |
| Assets | `make test-asset-registry test-asset-account-grant test-asset-balance test-asset-transfer test-asset-deposit test-asset-withdraw test-asset-reserve` |
| Escrow | `make test-escrow-open test-escrow-capture test-escrow-timeout test-escrow-dispute test-escrow-invariants` |
| Budgets | `make test-budget-create test-budget-period test-budget-spend test-budget-delegate test-budget-revoke` |
| Streams | `make test-stream-open test-stream-accrual test-stream-meter test-stream-settle test-stream-lifecycle` |
| Services | `make test-service-offer test-service-commit test-service-attest test-service-deliver test-service-acceptance test-service-dispute` |
| Oracle | `make test-oracle-intake test-oracle-bounds test-oracle-failclosed` |
| Perpetual and spot markets | `make test-exchange-native` |

`test-budget-revoke` runs `test_budget_close.c`, `test-stream-accrual` runs
`test_stream_accrue.c`, and `test-oracle-failclosed` runs `test_oracle_halt.c`.
The exchange target includes `test-perps-market`, `test-perps-book`,
`test-perps-margin`, `test-perps-funding`, `test-perps-liquidation`,
`test-perps-insurance` and `test-spot-book`.

Load the governance rules alongside the root Makefile to build and run
`test_governance_audit.c`:

```sh
make -f Makefile -f tests/modules/governance_audit.mk test-governance-audit
```

Build the trading-vector exporter with `make trading-vectors`. The exporter
prints JSON to standard output when run as `build/tools/dump_trading_vectors`.

The metered-stream runner is invoked by
[the stream gate script](../../tools/paxeer-x/gates/3.2.sh) with:

```sh
python3 tests/modules/paxeer_x_metered_end_boundary.py
```

It requires the metered-stream executable to be built and current with its
listed sources, and its output directory to be configured before invocation.

## Configuration

`paxeer_x_metered_end_boundary.py` reads these environment variables:

- `BUILD_DIR`: selects the build directory containing the metered-stream executable.
- `PAXEER_X_EVIDENCE_DIR`: selects an existing private output directory outside the repository.

The C test programs configure their module state in code and read no
configuration files. `test_stream_meter` accepts a snapshot directory as its
first command-line argument and otherwise creates a temporary directory.
The root Makefile accepts `BUILD_DIR`, `CC`, `AR`, `OPT_LEVEL`, `EXTRA_CFLAGS`,
`EXTRA_LDFLAGS` and `RUN_PREFIX` as build or execution overrides.

## Documentation

- [Kernel module registrations and activity types](../../docs/system/kernel-modules.md).
- [Governance proposal types and kernel controls](../../docs/system/governance.md).
- [Documentation site: modules](../../docs/site/paxeer-docs/src/app/modules/page.tsx).
- [Assets](../../docs/site/paxeer-docs/src/app/assets/page.tsx), [agent budgets](../../docs/site/paxeer-docs/src/app/agent-budgets/page.tsx) and [escrow](../../docs/site/paxeer-docs/src/app/escrow/page.tsx).
- [Streams](../../docs/site/paxeer-docs/src/app/streams/page.tsx) and [services](../../docs/site/paxeer-docs/src/app/services/page.tsx).
- [Oracle risk](../../docs/site/paxeer-docs/src/app/oracle-risk/page.tsx), [spot exchange](../../docs/site/paxeer-docs/src/app/spot-exchange/page.tsx) and [governance](../../docs/site/paxeer-docs/src/app/governance/page.tsx).
