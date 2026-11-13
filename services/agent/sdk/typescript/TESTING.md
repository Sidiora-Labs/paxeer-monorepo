# TypeScript SDK tests

Run commands from this package directory after installing the locked dependencies.
The package declares Node 22 or newer; the proof suites additionally need
`node:module.registerHooks` and `node:sqlite` available in the selected runtime.
The cross-language suites also need Python 3 and its
`cryptography` package. All commands build first and propagate failures.

| Command | Coverage and prerequisites |
| --- | --- |
| `npm run test:self-contained` | Every recursive `test/**/*.test.ts` source except the explicitly classified Anvil and case-file suites. Uses committed fixtures, local stores and loopback servers, including `rpc-node`, subscriptions, receipt verification, pending gas nonces, history, native effects, proof suites and restart recovery. No live endpoint or credentials. |
| `npm run test:anvil` | `exchange-v2.test.ts` and `gas-station.test.ts`; requires `anvil` on PATH or `ANVIL_BIN`. These check transaction bytes against a local EVM. |
| `npm test` / `npm run test:ci` | Every discovered suite except the envelope case-file probe, including both Anvil suites. Requires the local prerequisites above and Anvil. Every file runs even if an earlier file fails, and any failure makes the command fail. |
| `npm run test:external-node -- /absolute/path/programs-proof.json` | `ai-market` `--programs-proof` consumer of the Go application/EVM proof transcript produced by `precompiles/layerxprograms`' application proof harness. It consumes recorded calls, simulations, state heads, transfers and refusals. A missing argument or invalid evidence fails. |
| `PAXEER_X_AGENT_ENVELOPE_CASE=/private/case.json npm run test:case-file` | `agent-operation-envelope` harness probe. Requires the endpoints, credentials and TLS files described by the supplied case. Missing input fails. |
| `NATIVE_EFFECT_CROSS_LANGUAGE_REQUEST=/private/request.json npm run test:native-effect-case` | Executes the native effect suite and its cross-language producer-request assertions. Optional `NATIVE_EFFECT_TYPESCRIPT_CANONICAL_OUTPUT` writes its canonical output for the producer harness. Missing input fails. The ordinary native effect assertions also run in `npm test`. |

Discovery follows source files and checks their compiled counterparts before
running them as separate Node processes. It never discovers stale `dist` files.
New `.test.ts` sources enter the default and CI commands automatically; helpers
without that suffix do not. `package.json`'s `sdkTestRequirements` is the complete
list of suites that need Anvil or the private envelope case. Each entry must
exist and appear in only one category.
