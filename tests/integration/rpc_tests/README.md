# rpc_tests

Self-contained module for verifying `paxd` EVM JSON-RPC against a real local
**geth** reference node. Everything the suite needs (utilities,
contracts, tooling) lives under `tests/integration/rpc_tests/`, and the module has
its own `package.json`, `tsconfig.json`, and Hardhat compile config so it
installs independently of the other JavaScript suites. It still needs a running
local Paxeer cluster and Docker access for funding and association.

Paxeer mainnet-beta is one BFT network with the kernel participating in block
proposal; committed blocks finalize receipts. Its EVM chain ID is 125 (`0x7d`).
The local fixtures and geth dev chain have their own genesis and chain IDs.
Read the [integration overview](../README.md) for public root-path JSON-RPC
endpoints. This suite deploys contracts and spends test funds on a disposable
local chain.

## Install (one-time)

```bash
cd tests/integration/rpc_tests
npm ci             # installs locked module dependencies (ethers v6, CosmJS, mocha + tsx, Hardhat)
npm run compile    # compiles ./contracts -> ./artifacts (TestERC20, RealGasBurner, SimpleAccount7702)
```

## What this suite proves

The spec files in `eth/` cover method-specific combinations of:

- **Happy path.** The method returns the expected value/shape for valid input.
- **Schema parity.** The response shape on Pax matches geth for the same call.
- **Empty / null handling.** Absent data is represented correctly (`[]`, `null`,
  `0x`, etc.) and never as the wrong empty form.
- **Wrong params / error handling.** Bad input yields the correct JSON-RPC error
  code and message. Cases using `expectSameError` compare code, message and
  data to geth; other cases assert explicit method-specific expectations.

### Reference clients

- **geth `--dev` (primary).** `npm run rpc:geth` starts geth's HTTP server on
  `127.0.0.1:9547`, with APIs `eth,net,web3,debug,txpool` and `--dev.period 0`.
  Bootstrap deploys a mirrored TestERC20 scenario on this independent chain.
  Specs compare declared shapes, errors and selected behavior; chain-specific
  IDs, addresses and block histories are not expected to be identical.
- **Hardhat mainnet fork (optional, manual).** `npm run rpc:fork` starts a
  separate reference for ad-hoc data-shape checks. It requires both
  `ETH_MAINNET_UPSTREAM` and a positive safe-integer `ETH_MAINNET_FORK_BLOCK`.
  See the [fork README](hardhat/README.md). The automated suite still uses geth;
  `RPC_ETH_FORK` does not change `bothProviders()` or the bootstrap reference.

> **Filter & subscription coverage is deliberately Pax-only.** The log-filter and
> WebSocket specs (`eth_getLogs`, `eth_newFilter`, `eth_getFilterChanges`,
> `eth_newBlockFilter`, `eth_subscribe`) assert Pax's *functional* behavior and
> pin each result to Pax's own `eth_getLogs` oracle; they do **not** cross-check
> geth value-for-value. The configured geth command exposes HTTP only. The relevant
> filter specs limit their geth comparisons to error envelopes and handle shapes
> (e.g. `eth_newBlockFilter` id shape,
> `eth_newFilter` malformed-topic error). Do not expect symmetric geth asserts
> across every filter/subscription case.

## Layout

```
tests/integration/rpc_tests/
├── package.json                # module deps + scripts (compile / rpc:* / test:rpc)
├── tsconfig.json               # TypeScript config for the module
├── hardhat.config.ts           # compile-only config: contracts/ -> artifacts/
├── contracts/                  # TestERC20.sol, GasBurner.sol, SimpleAccount7702.sol, cw20_base.wasm
├── .mocharc.bootstrap.json     # runs _start/ sequentially
├── .mocharc.run.json           # mocha config for the spec run (single process)
├── scripts/run-ci.sh           # orchestrator: deps + compile + geth + bootstrap + run + merge
├── config/endpoints.ts         # env-driven endpoints
├── utils/                      # grouped by domain
│   ├── constants.ts            # shared values: HD path, UHPX/WEI_PER_UHPX, staking addr, chain id
│   ├── format.ts               # regex matchers: HEX_QUANTITY / ADDRESS / HASH32 / BLOOM256 / …
│   ├── chainUtils.ts           # providers + raw JSON-RPC + error parity + waitFor + EIP-1559 fee math
│   ├── evmUtils.ts             # EvmAccount + funding + contract deploy + EIP-7702 auth
│   ├── cosmosUtils.ts          # bank query/send + admin funding/association + fee_collector
│   ├── testUtils.ts            # runtime state + claimPool + expectSameError + ERC20 calldata
│   ├── txUtils.ts              # block/tx fixtures + block/receipt/count/raw-tx assertions
│   └── …                       # per-method helpers (feeHistory, gasPrice, logs, storage, subscribe, txLookup, wasm)
├── hardhat/                    # standalone fork config (chainId 1)
├── runtime/                    # gitignored, holds runtime.json
├── _start/
│   └── 00_bootstrap.spec.ts    # one-time setup
└── eth/                        # the actual specs (one dir per RPC namespace)
```

New RPC namespaces just need their own directory of `*.spec.ts` files; the run
config picks up every `**/*.spec.ts` outside `_start/` automatically.

## Runner (recommended)

```bash
cd tests/integration/rpc_tests
# start your local cluster first (e.g. `make docker-cluster-start` from the repo root)
npm run rpc:ci                     # == bash scripts/run-ci.sh
```

`scripts/run-ci.sh` is the single orchestrator, used both locally and by the
`EVM RPC Parity (geth reference)` matrix entry in
`.github/workflows/paxeer-integration-test.yml`. It assumes an EVM RPC is already
reachable (the workflow boots the 4-node cluster; locally you start it yourself),
then end to end:

1. Installs deps (`npm ci`; skip with `SKIP_NPM_CI=true`) and compiles contracts.
2. Waits for the EVM RPC on `:8545` and for the chain to be producing blocks.
3. Installs geth when it is absent (`go install` of go-ethereum at `GETH_VERSION`,
   default `v1.17.0`) and starts the geth `--dev` reference node
   (`npm run rpc:geth`), waiting for `:9547`.
4. Runs `rpc:bootstrap`, then (only if bootstrap passed) `rpc:run`.
5. Merges the per-phase mochawesome JSON into one combined HTML report at
   `reports/merged/rpc-tests.html`.

Cleanup attempts to stop the spawned geth process and listeners on port 9547;
reserve that port for this run. Dependency, compile, readiness, bootstrap and
spec failures return nonzero. A report-merge failure only emits a warning and
can leave a missing report despite a successful test exit.

Knobs: `PAX_EVM_RPC`, `RPC_ETH_GETH`, `PAX_TIMEOUT` (default 300 s),
`GETH_TIMEOUT` (default 120 s), `SKIP_NPM_CI`, `GETH_VERSION`. An existing geth
binary is reused with a version-mismatch warning; the version pin controls an
installation when the binary is absent. `RPC_ETH_GETH` changes the URL polled
and consumed, while `rpc:geth` still starts its fixed local port 9547.

## Reporting

Each phase writes a mochawesome JSON (`reports/new_rpc/bootstrap.json`,
`reports/new_rpc/run.json`). `npm run report:merge` combines them via
`mochawesome-merge` + `mochawesome-report-generator` into a single interactive
report at `reports/merged/rpc-tests.html` (`run-ci.sh` does this for you).

## Running manually

All commands run from `tests/integration/rpc_tests/`.

```bash
# 1. In a dedicated terminal, start the geth reference node. Leave it up.
npm run rpc:geth      # geth --dev on http://localhost:9547  (requires geth on PATH)

# 2. Make sure a local node is up on http://localhost:8545 (for example
#    `make docker-cluster-start` from the repo root).

# 3. (Optional, separate terminal) start a pinned fork for manual checks.
#    Replace the URL and block with a block whose historical state your provider serves.
ETH_MAINNET_UPSTREAM='https://your-ethereum-provider.example/' \
ETH_MAINNET_FORK_BLOCK=21000000 npm run rpc:fork  # http://localhost:9546

# 4. Run the suite (single mocha process).
npm run test:rpc      # bootstrap + run, recommended
# or, piecewise:
npm run rpc:bootstrap # writes runtime/runtime.json
npm run rpc:run       # runs every spec outside _start/ via .mocharc.run.json
```

Both fork variables are required, even for manual checks. Choose a positive
safe-integer block number (at most `9007199254740991`) and keep it pinned across
runs for reproducible reference data. The example uses block `21000000`; replace
it with a block whose historical state your upstream serves. An unset, zero,
negative, fractional or unsafe block fails during configuration loading; the fork
never defaults to the latest block. These requirements apply to `npm run rpc:fork`,
whose package script loads `hardhat/hardhat.config.ts`.

> **Why a single process.** Every spec shares the one chain and the
> bootstrap's funded-account pool, so a parallel run would make specs contend on
> the base fee and reuse pool keys (`claimPool` hands out disjoint slices via a
> module-level cursor, which is only correct in-process). The suite therefore runs
> serially; `report:merge` globs `bootstrap.json` + `run.json` into one report.

Individual files can be run with `mocha`, explicitly loading `tsx`:

```bash
npx mocha --require tsx --timeout 600000 eth/eth_blockNumber.spec.ts
```

…but only after `npm run rpc:bootstrap` has produced `runtime/runtime.json`.

## Configuration

| Variable                | Default                                            |
| ----------------------- | -------------------------------------------------- |
| `PAX_EVM_RPC`           | `http://localhost:8545`                            |
| `PAX_EVM_WS`            | `ws://localhost:8546`                              |
| `PAX_COSMOS_RPC`        | `http://localhost:26657`                           |
| `PAX_REST`              | `http://localhost:1317`                            |
| `RPC_ETH_GETH`          | `http://127.0.0.1:9547` (geth --dev, primary)      |
| `RPC_ETH_FORK`          | `http://127.0.0.1:9546` (manual Hardhat fork)       |
| `ETH_MAINNET_UPSTREAM`  | required for `npm run rpc:fork` (no default — bring your own mainnet RPC URL) |
| `ETH_MAINNET_FORK_BLOCK`| required positive safe integer for `npm run rpc:fork`; no latest default |
| `PAX_ADMIN_MNEMONIC`    | empty; bootstrap generates one when unset |
| `RPC_POLLING_INTERVAL_MS`| `100` (the ethers default of 4 s is too slow for the local chain) |
| `RPC_TESTS_RUNTIME_STATE`| `runtime/runtime.json`                            |

Endpoint defaults come from [`config/endpoints.ts`](config/endpoints.ts).
`RPC_ACCOUNTLESS` is a separate endpoint setting whose current fallback is a
historical service URL; it is not a merged-mainnet endpoint recommendation.
Bootstrap skips admin funding if its EVM balance is already positive; otherwise
it funds and associates the account through Docker. The local helpers use fixed `pax-node-0`/`admin` Docker assumptions, and
CW20 pointer registration uses the in-container URL in
[`utils/constants.ts`](utils/constants.ts). Environment endpoint overrides alone
do not turn this into a remote-only suite. Bootstrap records the actual chain
IDs in `runtime.json`; the local `DEFAULT_EVM_CHAIN_ID` constant is not the
mainnet chain ID.

## Authoring a new spec

Structure every spec into the four sections (happy path / schema matching /
empty-null / wrong params), e.g.:

```ts
import { expect } from 'chai';
import { bothProviders, rawPax, rawGeth } from '../utils/chainUtils';
import { HEX_QUANTITY } from '../utils/format';
import { readRuntimeState, RuntimeState, expectSameError } from '../utils/testUtils';

describe('eth_getBalance', function () {
    this.timeout(60 * 1000);

    const { pax } = bothProviders();
    let runtime: RuntimeState;

    before(() => {
        runtime = readRuntimeState();
    });

    describe('happy path', () => {
        it('returns a canonical hex quantity', async () => {
            const bal = await pax.send('eth_getBalance', [runtime.funded.admin, 'latest']);
            expect(bal).to.match(HEX_QUANTITY);
        });
    });

    describe('wrong params / error handling', () => {
        it('rejects a missing block tag identically to geth', async () => {
            const [s, g] = await Promise.all([
                rawPax('eth_getBalance', [runtime.funded.admin]),
                rawGeth('eth_getBalance', [runtime.funded.admin]),
            ]);
            expectSameError(s, g);
        });
    });
});
```

Rules of the road for new specs:

1. **Read-only fixture file.** Bootstrap is the only writer of `runtime.json`. If
   you need new pre-computed state, add it to the `RuntimeState` interface and
   populate it in `_start/00_bootstrap.spec.ts`.
2. **Pool accounts are single-use.** Claim fresh accounts with
   `claimPool(runtime, provider, count, label)` (testUtils), which hands out a
   disjoint slice of `runtime.funded.pool` on every call; never reuse a pool key
   across specs.
3. **No imports from outside this module** — keep it self-contained.
4. **Negative tests go through `rawPax` / `rawGeth`** to bypass ethers'
   client-side validation, so we assert the *node's* behavior, not ethers'.
5. **geth is the error/schema source of truth.** Assert Pax matches `rawGeth`
   for the comparisons the spec declares, including error data when using
   `expectSameError`. Pax-only methods (`pax_*`) have no geth reference; assert
   their documented Pax behavior explicitly.

For response-kind-only fixture coverage, see the
[`.io` / `.iox` harness](../evm_module/rpc_io_test/RPC_IO_README.md). For dApp
flows spanning EVM and native operations, see the [dApp suite](../dapp_tests/README.md).
