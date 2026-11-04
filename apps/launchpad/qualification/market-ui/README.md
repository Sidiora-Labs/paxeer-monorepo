# L30 market presentation and journey checks

Market UI source lives in `apps/launchpad/web`. The tree includes presentation primitives, a market session, integrated token layouts and later service/browser runners. This guide describes their inputs and evidence boundaries; it does not claim that release capacity or a production journey has passed. See the [mobile architecture](../../mobile/ARCHITECTURE.md) for the merged chain-125 network boundary and native behavior.

## Presentation fixtures

From the repository root, enter the web application:

```sh
cd apps/launchpad/web
./node_modules/.bin/vitest run src/page-modules/token/market-presentation.test.tsx --reporter=verbose
```

These are React DOM tests in jsdom with synthetic arithmetic and chain-order fixtures. They exercise actual-decimal normalization, 78-digit balances, unavailable quality, malicious identity text, identity disagreement, missing valuation, real versus virtual backing, bounded ranking compatibility, numeric holder sorting, trade deduplication and freshness transitions. They provide no chain receipts, live network journey, browser timing or capacity proof.

The code imports shared generated DTOs and SDK validators rather than copying schemas. [market-session.ts](../../web/src/page-modules/token/market-session.ts) and the [token market layout](../../web/src/widgets/trading/token-market-layout.tsx) are implemented integration paths. The earlier phase-one statement that integration and mounting were wholly pending no longer describes this tree. Each route and control still needs evidence from the candidate actually measured.

## Integrated service and DOM journeys

[phase2-service-journey.ts](phase2-service-journey.ts) exports `observeMarketJourney` and `observeInterruptedMarketJourney`; [phase2-dom.test.tsx](phase2-dom.test.tsx) consumes those observations in jsdom assertions. The service journey reads actual pool details, trade/ranking cursor pages and holders, and requires HTTP 400 for an invalid trade cursor. The interruption journey observes a hidden period, a sequence gap and snapshot/replay recovery with revised statistics.

The DOM suite requires all of these real owned fixture inputs:

- `KINDLE_MARKET_API_ORIGIN`, `KINDLE_MARKET_IDENTITY_ORIGIN` and `KINDLE_MARKET_MEDIA_ORIGIN`;
- `KINDLE_MARKET_STREAM_ORIGIN` and `KINDLE_MARKET_POOL`;
- `KINDLE_MARKET_RANKING_CATEGORY` and `KINDLE_MARKET_EMPTY_CATEGORY`.

Fixtures must contain multiple trade and ranking pages, an empty category, partial holder coverage including balances 100 and 9, and an actual interrupted stream that exposes the required gap and recovery. Missing inputs fail. These service functions are not standalone command-line runners. The web application's default [Vitest configuration](../../web/vitest.config.ts) includes only `src/**/*.test.ts` and `src/**/*.test.tsx`; it does not discover this qualification directory. An executor must explicitly include the owned DOM suite and provide its aliases/dependencies and service environment. Merely running the presentation command above does not execute these journeys.

From the repository root, the phase-two scoped typecheck is:

```sh
apps/launchpad/web/node_modules/.bin/tsc -p apps/launchpad/qualification/market-ui/tsconfig.phase2.json
```

It covers the configured home/new/token/terminal sources, related widgets and owned journey files. It is not a complete application build.

## Browser measurement runner

[phase4-browser.mjs](phase4-browser.mjs) uses [phase4-observer.mjs](phase4-observer.mjs) and the budgets in [phase4-policy.json](phase4-policy.json). Its ordered CLI arguments are `--config <path> --config-sha256 <digest> --output <directory>`. Configuration and digest must identify the exact approved file. Output must be a fresh empty child directory of `launchpad/evidence/production-system/L30/phase4` under the checkout, as enforced by the runner; protect it as raw qualification evidence.

Configuration binds candidate source, interface, topology and workload identities; real trace corpus; Playwright, SDK and accessibility tools; browser executable, device calibration and session; Node/Python executables; heap limits and five hot topics. It also requires a bound backend runner/configuration, fresh backend output and required backend environment keys. The [policy](phase4-policy.json) lists input categories and numeric budgets; the runner's configuration reads and assertions define the exact shape. Use provisioned application/API HTTPS origins and actual service observations.

The runner launches Chromium, captures DOM/network/socket/heap observations, reconciles fills with backend source events, exercises discovery/token/terminal navigation, checks freshness and reconnect behavior, and measures responsive layout and accessibility. Its policy requires at least ten minutes of steady measurement and twenty route cycles. It records `result.json`, raw observations and `artifact-sha256.json`; file existence alone is not proof that its assertions passed.

Even a successful result explicitly records `capacity_qualified: false`, `shared_workload_qualified: false` and final capacity task `9.1`. Shared load/restore and final production acceptance remain deferred. Keep synthetic presentation output, real-service results and browser measurements tied to their actual scope and revision. The [account UI guide](../account-ui/README.md) documents neighboring account, browser and physical Android checks.
