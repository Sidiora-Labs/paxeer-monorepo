# Existing security regression prerequisite map

[manifest.json](manifest.json) inventories Go test names, source hashes, module
working directories and executable commands. From the repository root, inspect a
fresh inventory with:

```sh
python3 tools/launchpad/production/bootstrap/security/build_manifest.py
```

The generator prints JSON to stdout; it does not update the checked-in manifest
or execute tests. Capture stdout as a candidate and compare it with
`tools/launchpad/production/bootstrap/security/manifest.json`. Before replacing
that checked-in destination, review each row's module working directory, command,
source paths, SHA-256 hashes, test names and dependency prerequisites against the
current tree. Keep `execution_status` as `not_executed_by_inventory`; registry
admission requires this reconciliation and separate execution evidence. This
inventory does not qualify bootstrap task `0.1`
or certify production security. Preserve command exits and Go JSON test events;
skips and zero-case execution must fail qualification.

## Coverage and limits

Paths below are relative to the repository root. Each manifest row supplies
the module working directory and source test inventory for its command.

- `sdk/launchpad/shared/auth`: the inventory lists `webhook_test.go`, whose
  tests cover HMAC signing vectors, round trips, timestamp windows, missing
  signature, bad scheme, wrong secret and tampered payload.
- `sdk/launchpad/shared/http`: API-key rejection, CORS, in-memory/Redis throttling,
  client-IP resolution, 5xx detail masking and scanner responses. The Redis case
  starts a real `redis:7-alpine` container; complete qualification must retain it.
- `services/launchpad/gateway/internal/auth`: wallet login, nonce reuse and
  invalid inputs, missing session, and JWT mint/verify/expiry. Login tests require
  Redis through the gateway helper; JWT-only results cannot prove Redis atomicity.
- Gateway `internal/upload` and `internal/image`: MIME/SVG/size/address handling,
  infected-file rejection and upstream failure. Some tests inject scanners or
  HTTP upstreams; inspect their dependencies before claiming live scanning.
  `internal/serve` covers invalid object keys; `internal/proxy` covers identity
  header stripping and unauthenticated socket rejection.
- `services/launchpad/api/internal/ratelimit`: the generator includes a separate
  API rate-limit group; shared HTTP tests do not substitute for this package.
- `services/launchpad/{stats-workers,pnl-tracker,trading-charts}/internal/httpapi`:
  webhook authentication and dispatch/invalid payload handling. Fixture dispatch
  does not establish provider delivery, queue durability or restart recovery.
- `services/launchpad/{metadata,user}`: signed write workflows with internal
  package fixtures. Inspect each subtest before claiming a route authorization
  matrix.
- `sdk/launchpad/shared/secrets`: environment retrieval, cache expiry/error
  propagation and provider selection. `NewChecked` selects `Options.Provider`,
  then `SECRETS_PROVIDER`, accepting an empty value or `env`. It rejects other
  providers and cache lifetimes outside zero to sixty seconds.

## Scanner behavior and remaining proof

`ScanBuffer` in `sdk/launchpad/shared/http/virusscan.go` fails closed. Host selection
uses `ScanOptions.Host`, then `CLAMAV_HOST`; if both are empty it returns
`UNAVAILABLE` with `clamav-not-configured`. An unreachable
scanner returns `UNAVAILABLE` with `clamav-unreachable`. Port selection uses
`ScanOptions.Port`, then `CLAMAV_PORT`, then 3310. Invalid ports return `ERROR`.
Scans are bounded to 6 MiB and at most five seconds. Only the complete ClamAV
`stream: OK` verdict sets `Clean: true`; infected, interrupted, timed-out and
malformed results never become clean.

`TestScanBufferNotConfigured` and `TestScanBufferUnreachable` assert `Clean: false`,
`UNAVAILABLE`, the specific reason, an empty content hash and zero size.
The gateway upload regression `TestUpload_ScannerUnavailableFailsClosed503`
checks HTTP 503, masked client errors, logged reasons and no forwarding to
media/metadata on unavailable scanning. `TestScanBufferClean` and
`TestScanBufferInfected` use fixed protocol responses from a local server; they
do not execute live ClamAV/EICAR scanning.

`ScanVerifiedBuffer` obtains scanner engine/signature revision before scanning.
Only after a `CLEAN` result does it query the version again; a failed initial
query returns `UNAVAILABLE` with `clamav-version-unavailable`, and a failed
recheck or changed engine/signature revision returns `UNAVAILABLE` with
`clamav-version-changed`. Neither result is clean. To qualify a deployment,
retain real-scanner version and EICAR evidence, timeout/malformed-response and
failed-dependency evidence, and failed-scan storage-denial evidence from the
deployed path, including no object-store writes or metadata side effects.
The forwarding assertion alone does not prove these storage-side-effect bounds.

## Deferred security qualification gaps

The inventory alone cannot establish the following obligations, even where newer
source tests or adapters implement part of them:

1. **Session binding/concurrency:** domain, chain, issued-at/expiry binding and
   simultaneous nonce consumption need execution evidence from the real session
   store; sequential reuse checks are insufficient.
2. **Webhook replay/key rotation:** prove durable committed-once effects, exact
   replay-window boundaries/future timestamps, overlapping keys and revocation.
3. **Trust boundaries:** prove ingress isolation, direct backend restrictions,
   trusted-proxy behavior, private socket authorization and credential rotation.
4. **Upload adversarial content:** prove parser behavior, decompression limits,
   polyglot handling and object-store quarantine lifecycle.
5. **Authorization breadth:** prove route-by-role/object-owner coverage,
   cross-wallet denial, admin boundaries and browser session/CSRF behavior.
6. **Frontend and infrastructure:** this Go inventory cannot establish browser
   XSS/CSP protection, dependency/container advisories or deployment hardening.
7. **Coverage scope:** contract security checks are separate; private fixture
   admission and full application qualification remain coordinator obligations.

## Integration

Register reviewed manifest checks with the central runner only after reconciling
source hashes. Retain per-case JSON events and reject skips, unavailable fixtures,
missing dependencies, stale hashes and zero matching tests. The generator uses
`-run ^TestWebhook` for its webhook groups; full-package groups also include
non-security tests. Container-backed groups share the coordinator's resource
budget. Every inventory row starts `not_executed_by_inventory`.

See the [bootstrap gate](../README.md), [dispatcher](../../dispatcher/README.md),
[integration phases](../../integration/README.md) and
[load evidence limits](../../load/README.md) for the distinct execution paths.
