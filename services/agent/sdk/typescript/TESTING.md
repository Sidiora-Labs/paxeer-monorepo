# TypeScript SDK tests

Run commands from this package directory after installing the locked dependencies.
Use Node 24.11 or newer (the proof suites use `node:module.registerHooks` and
`node:sqlite`). The existing cross-language suites also need Python 3 and its
`cryptography` package. All commands build first and propagate failures.

| Command | Coverage and prerequisites |
| --- | --- |
| `npm test` / `npm run test:self-contained` | Every recursive `test/**/*.test.ts` source except the explicitly classified Anvil and private case-file suites. Uses committed fixtures, local stores and loopback servers, including `rpc-node`, subscriptions, receipt verification, pending gas nonces, history, native effects, proof suites and restart recovery. No live endpoint or credentials. |
| `npm run test:anvil` | `exchange-v2.test.ts` and `gas-station.test.ts`; requires `anvil` on PATH or `ANVIL_BIN`. These preserve their transaction-byte assertions; Anvil alone does not prove native precompile execution. |
| `npm run test:ci` | Every discovered suite except the private envelope case-file probe, including both Anvil suites. Provision the local prerequisites above and Anvil. Every file runs even if an earlier file fails, and any failure makes the command fail. |
| `npm run test:external-node -- /absolute/path/programs-proof.json` | Existing `ai-market` `--programs-proof` consumer of the real Go application/EVM proof transcript. The producer is `precompiles/layerxprograms`' application proof harness. It must supply the complete recorded calls, simulations, state heads, transfers and refusals. This command consumes node-produced evidence; it does not launch or connect to a node. A missing argument or invalid evidence fails. |
| `PAXEER_X_AGENT_ENVELOPE_CASE=/private/case.json npm run test:case-file` | Existing `agent-operation-envelope` harness probe. Requires the operator's private endpoints, credentials and TLS files described by that case. Missing input fails. This historical probe still contains retired daemon cases; its owning harness must migrate them before claiming current hosted coverage. |
| `NATIVE_EFFECT_CROSS_LANGUAGE_REQUEST=/private/request.json npm run test:native-effect-case` | Executes the native effect suite and its existing cross-language producer-request assertions. Optional `NATIVE_EFFECT_TYPESCRIPT_CANONICAL_OUTPUT` writes its canonical output for the producer harness. Missing input fails. The ordinary native effect assertions also run in `npm test`. |

Discovery follows source files and checks their compiled counterparts before
running them as separate Node processes. It never discovers stale `dist` files.
New `.test.ts` sources enter the default and CI commands automatically; helpers
without that suffix do not. `package.json`'s `sdkTestRequirements` is the complete
list of suites that need Anvil or the private envelope case. Each entry must
exist and appear in only one category. Do not classify a failing offline suite
as an external probe to avoid its assertions.

CI owners must invoke `npm run test:ci` in the SDK lane. The current
`.github/workflows/xweb-test.yml` invokes only `web-search.test.js`; replacing
that invocation and provisioning the prerequisites is an out-of-scope wiring
request recorded in `tools/qualification/audit_repairs/repair_22_33_22.kvx`.
Private case and external-node evidence commands belong in their provisioned
producer/harness lanes, where a failed command must fail the job. This package
change alone does not establish that those workflow edits have been deployed.
