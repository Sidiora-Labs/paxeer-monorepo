# Precompiles

The `precompiles` directory contains the Go EVM precompile implementations, their ABI and embedded data files, versioned legacy implementations, registration code, and tests. `paxd` serves these native capabilities inside the single Paxeer mainnet-beta execution domain (EVM chain ID 125); the merge plan activates the LayerX ledger, payments, authority, programs, chains, and agent-function entries alongside the other registered precompiles, and retained interfaces enforce their activation and retirement rules.

## Layout

- `addr/` — address and account binding precompile.
- `bank/` — bank balance and transfer precompile.
- `common/` — shared ABI, event, error, registration, and pointer helpers.
- `distribution/` — distribution and reward operations.
- `feetoken/` — fee-denomination selection precompile.
- `gov/` — governance precompile and legacy versions.
- `ibc/` — IBC transfers and timeout handling.
- `json/` — JSON extraction helpers for EVM calls.
- `launchpad/` — launchpad market, pool, token, fee, and liquidity surfaces.
- `layerxagent/` — merge-gated agent functions: clearing, EIP-712, oracle aggregation, OROB, POFQ, scheduler, streams, and TEE attestation.
- `layerxanchor/` — retained LayerX anchor interface.
- `layerxauth/` — merge-gated LayerX authority precompile.
- `layerxbridge/` — bridge operations.
- `layerxchains/` — merge-gated LayerX chain and tenant operations.
- `layerxcustody/` — retained LayerX custody interface.
- `layerxexchange/` — LayerX exchange ABI and compatibility implementation; `setup.go` selects `perps/` for current upgrades.
- `layerxledger/` — merge-gated LayerX ledger operations.
- `layerxpay/` — merge-gated LayerX payment operations.
- `layerxprograms/` — merge-gated LayerX program operations.
- `layerxverify/` — retained LayerX verification interface.
- `oracle/` — retained oracle ABI; oracle data queries are disabled.
- `p256/` — P-256 signature verification precompile.
- `perps/` — perpetuals precompile and ABI files.
- `pointer/` — native and CosmWasm token pointer creation.
- `pointerview/` — pointer read-only precompile.
- `pricefeed/` — price-feed precompile and ABI files.
- `solo/` — asset claims through `claim` and `claimSpecific`.
- `spot/` — spot exchange rails and ABI files.
- `staking/` — staking precompile.
- `testutil/` — test utilities.
- `utils/` — shared precompile utilities and keeper interfaces.
- `wasmd/` — CosmWasm execution and query precompile.
- `xweb/` — XWeb request and response precompile.
- `custom_precompiles_e_test.go` — custom precompile constructor error propagation test.
- `merge_gateway_parity_test.go` — merge gateway parity tests.
- `merge_serve_0x0901_test.go` — merge serving test for address `0x0901`.
- `merge_serve_0x0902_test.go` — merge serving test for address `0x0902`.
- `merge_serve_0x0903_test.go` — merge serving test for address `0x0903`.
- `merge_serve_0x0904_test.go` — merge serving test for address `0x0904`.
- `merge_serve_0x0905_test.go` — merge serving test for address `0x0905`.
- `merge_serve_0x0906_test.go` — merge serving test for address `0x0906`.
- `merge_serve_0x0907_test.go` — merge serving test for address `0x0907`.
- `merge_serve_0x0908_test.go` — merge serving test for address `0x0908`.
- `merge_serve_0x101a_test.go` — merge serving test for address `0x101A`.
- `merge_serve_0x101b_test.go` — merge serving test for address `0x101B`.
- `merge_serve_0x101c_test.go` — merge serving test for address `0x101C`.
- `merge_serve_0x101d_test.go` — merge serving test for address `0x101D`.
- `merge_upgrade_test.go` — merge upgrade test.
- `missing_merge_test.go` — missing merge registration test.
- `names_markets_test.go` — precompile names and markets test.
- `precompile_info_e_test.go` — precompile metadata lookup and initialization tests.
- `proof_activation_test.go` — activation proof tests.
- `proof_precompile_go_api_test.go` — Go API precompile proof tests.
- `setup.go` — precompile registration, activation, constructors, and embedded ABI index.
- `setup_retire_test.go` — retirement behavior test.
- `setup_test.go` — registration and setup tests.

## Build and test

The packages belong to the root [Go module](../go.mod). Run these commands from the repository root:

- `make paxd-build` — builds the native dependencies and the node that links the precompiles, using the root [Makefile](../Makefile) and [chain.mk](../chain.mk).
- `make -f chain.mk test` — runs the chain's split Go test suite, which includes the precompile packages.
- `go test -count=1 ./modules/xweb/... ./precompiles/xweb/...` — runs the focused XWeb module and precompile tests, as used in [the XWeb workflow script](../scripts/ci/xweb-workflow-check.sh).

The directory has no separate build manifest or Makefile. The node build prepares the native libraries required by the Go packages.

## Configuration

The precompile sources read no environment variables or external configuration files directly. ABI and pointer data are embedded from files such as `addr/abi.json`, `perps/abi/abi.json`, `pricefeed/abi/abi.json`, `spot/abi/pecor.json`, and `common/pointer_artifacts/manifest.json`; node keepers and activation state are supplied by the registration path in `setup.go`.

## Documentation

- [Precompiles reference](../docs/system/precompiles.md)
- [Documentation-site precompiles page](../docs/site/paxeer-docs/public/markdown/precompiles.md)
- [Documentation-site page source](../docs/site/paxeer-docs/src/app/precompiles/page.tsx)
