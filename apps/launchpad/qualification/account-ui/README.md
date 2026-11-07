# L31 account UI disclosure and journey checks

This directory covers the account surfaces in `apps/launchpad/web` and the native account journey described in the [mobile architecture](../../mobile/ARCHITECTURE.md). All commands below start at the repository root and reuse launchpad workspace dependencies in `apps/launchpad/web/node_modules`. These commands document runners; they are not evidence that a candidate passed.

## Disclosure primitives and typechecking

For the synthetic disclosure and failure-state checks only:

```sh
mkdir -p launchpad/evidence/production-system/L31/phase1
apps/launchpad/web/node_modules/.bin/vitest run --config apps/launchpad/qualification/account-ui/vitest.config.mts apps/launchpad/qualification/account-ui/disclosure.test.tsx apps/launchpad/qualification/account-ui/rewards-failure.test.tsx
apps/launchpad/web/node_modules/.bin/tsc -p apps/launchpad/qualification/account-ui/tsconfig.json
```

[disclosure.test.tsx](disclosure.test.tsx) exercises the exported `portfolioLabels` and `sharePreview` functions, exact amounts and account-state primitives with synthetic financial inputs. It asserts the current unavailable marker (`—`), partial-history labels and exact selected-field output directly; these modules export no view component. [rewards-failure.test.tsx](rewards-failure.test.tsx) mounts `RewardsView` with deliberately rejected RPC reads and incomplete synthetic NFT enumeration. Generated partial-account labels and anonymous selected-field preview JSON files are illustrations. These checks provide no successful provider response, chain receipt, live browser/service qualification or production evidence. Historical module-resolution failures remain historical evidence, not a result for the current candidate.

The [TypeScript configuration](tsconfig.json) checks disclosure, profile, watchlist, reward, preferences and share integration code against shared generated contracts, including `PortfolioSummary`. This is a scoped strict typecheck, not a complete web/mobile build or release qualification.

## Real account service journeys

[run-phase2.py](run-phase2.py) runs [account-journeys.test.tsx](account-journeys.test.tsx) with the same owned Vitest configuration:

```sh
python3 apps/launchpad/qualification/account-ui/run-phase2.py
```

The runner requires every environment input listed in its `required` array: service origins; real current, other-account and expired credentials; session, pool and asset identities; deposit interval; rebuy token and expected remaining basis; and RPC, USDL and transaction-hash inputs. The journey requires isolated loopback user, gateway and PnL services. Use dedicated qualification accounts and real indexed transactions. Missing inputs fail explicitly.

The journeys exercise profile revision conflicts, watchlist idempotency, sign-out and replacement-session races, private-field exclusion, expired authorization, selected-field share publication, deposit accounting and buy/full-sell/rebuy basis. They include actual service reads and mutations and do not constitute a physical-device or browser-capacity measurement.

The historical entry point is still available:

```sh
mkdir -p launchpad/evidence/production-system/L31/phase1
python3 apps/launchpad/qualification/account-ui/run-check.py
```

It runs **every** `*.test.tsx` in this directory, including real-service journeys, and writes the child exit code to `launchpad/evidence/production-system/L31/phase1/native-exit.json`. Supply the phase-two environment and create the phase-one evidence directory first. Despite that filename, this invokes Node/Vitest, not a native device. Use the targeted commands above when only disclosure primitives are intended. Dependency installation and lock updates are separate workspace work.

All three Python entry points resolve the checkout root from this directory and run their children there, including when invoked by absolute path from another working directory. `run-check.py` passes no test-file filter: its owned configuration discovers all three suites. A failed child remains a failed exit; the JSON file records that child result and is not native qualification evidence.

## Staging browser and physical-device runner

[run-phase4.py](run-phase4.py) coordinates [phase4-browser.mjs](phase4-browser.mjs) and [phase4-native.py](phase4-native.py). After the owner provisions the staging configuration and a fresh private output path outside the checkout, invoke:

```sh
python3 apps/launchpad/qualification/account-ui/run-phase4.py --config /absolute/private/account-staging.json --output /absolute/private/account-run
```

The paths are operator-supplied examples. Configuration must declare schema version 1, staging and `approvedMutationScope: dedicated-qualification-accounts`. It binds the complete current source manifest, web/native artifacts, interface and network revision, measurement provenance and at least two real wallets. The browser child connects to an existing Chromium wallet context over CDP; it does not install a synthetic provider or session. The native child requires a physical Android device, existing Appium session, approved ADB/signature tools, an installed signed release APK and a real request-capture adapter. Its Android evidence cannot qualify iOS.

The runner reconciles UI accounting and watchlists with authoritative snapshots, observes account recovery and supported reward-claim outcomes, and checks concurrent web/native watchlist edits. Unsupported reward settlement remains explicitly unavailable. Raw responses, screenshots, logs and artifact digests belong in the private output directory; preserve credentials and account data in access-controlled evidence storage.

Runner availability is not a passing result. Even a successful phase-four result leaves `qualificationCapacityClaim` false and shared load/restore acceptance deferred to serial task 9.1. The [market UI guide](../market-ui/README.md) describes the corresponding market presentation and measurement boundaries.

The path and command regression proof for audit finding `account-readme-path` is [test_repair_22_33_1026.py](test_repair_22_33_1026.py); its [repair record](repair_22_33_1026.kvx) describes the proof and its limits. Loopback checks of documentation and configuration do not run the service journeys, browser or physical-device qualification above.
