# iOS quickstart

Use the human payment API from an iOS app with brokered session tokens.

## The credential model

The binding accepts public configuration and obtains session tokens at runtime:

- The app carries a **publishable configuration** only: a service URL, a session broker URL, one or more event public keys, and two timeouts. All of it is safe in a bundle.
- At call time the binding obtains a **short-lived session token** from a broker you run. The provider caches it in memory while usable. Each request creates an access-token copy and destroys that copy after use. A capability refusal invalidates the cache and triggers one retry with a freshly obtained token.
- Your bearer token stays on your server, where it belongs.

`PublishableConfiguration` accepts declared URL, public-key and timeout settings and rejects unknown declared keys and recognized credential patterns.

## Before you start

```swift
.package(path: "sdk/integrations/ios")
```

| Variable | What it is |
|---|---|
| `LAYERX_SERVICE_URL` | Your human API endpoint |
| `LAYERX_SESSION_BROKER_URL` | The broker that mints session tokens for your users |
| `LAYERX_EVENT_PUBLIC_KEY_<key-id>` | An event signing public key you trust |
| `LAYERX_EVENT_MAX_AGE_SECONDS` | Optional. How stale an event may be |
| `LAYERX_REQUEST_TIMEOUT_SECONDS` | Optional |
| `LAYERX_RECEIPT_KEYS_JSON` | Pinned receipt keys required by `gate()` |

In a real app these come from your build configuration rather than the process environment; `PublishableConfiguration(declaredKeys:)` and `PublishableConfiguration(contentsOfJSONFile:)` exist for exactly that.

## The integration

```swift sample=mobile-payment-ios
let settings = try PublishableConfiguration(environment: ProcessInfo.processInfo.environment)
let layerx = try LayerXMobile(configuration: settings)
let quote = try await layerx.client.quote(.object(["source": .string(source), "destination": .string(destination), "money": money]))
guard let quoteID = quote.objectValue?["quote_id"]?.stringValue else { fail("move quote omitted quote_id") }
var journey = try await layerx.client.commit(.object(["quote_id": .string(quoteID)]), idempotencyKey: paymentKey)
```

`layerx.client` covers the human plane an app actually needs: `version`, `profile`, `activity`, `activityEntry(id:)`, `journeys`, `journey(id:)`, `quote`, `commit(_:idempotencyKey:)` and a resumable event stream. Path values are validated before they reach a URL, so an identifier containing a slash or a query character is refused rather than used to build a request.

## Receipt checks

Configure `LAYERX_RECEIPT_KEYS_JSON` independently of received proofs before using `gate()`. It verifies signed receipt evidence and matches payment facts before the app treats a journey as paid.

This setting is a JSON array whose entries contain `keyId` and `publicKey` as `0x`-prefixed 32-byte hex strings and `firstHeight` and `lastHeight` as JSON integers. The height range is inclusive. Receipt verification establishes execution evidence; finality also requires committed block evidence.

## Verified push and webhook events

`layerx.consume(rawBody:headerFields:handle:)` verifies an event envelope against your configured public keys, refuses it if it is stale, and claims the delivery under a lease. The claim store is pluggable; the default `FileEventDeliveryStore` retains delivery state in application support storage.

## Never ship a secret

The package includes `layerx-ios-secret-scan`, which walks a built product and reports embedded secrets. Run it in your release pipeline. The configuration's `exemptScannerValues` tells the scanner which strings are legitimately public, so the scan is precise rather than noisy.

## Run the sample

Set the service and session-broker URLs, at least one event public key, and
`LAYERX_SOURCE`, `LAYERX_DESTINATION`, `LAYERX_AMOUNT`, `LAYERX_CURRENCY` and
`LAYERX_PAYMENT_KEY` before running. The service URL must identify the human
API; local emulator inspection routes do not submit payments.

```sh
cd services/platform/examples/samples/mobile-payment-ios
swift build
swift run MobilePayment
```

The sample is a command-line executable using the same binding API as an app target.

## Enforced by

| Capability | Layer | What that means here |
|---|---|---|
| Atomic settlement | `protocol` | The committed move applies completely or not at all. |
| Replay refusal | `protocol` | The same signed transaction cannot apply twice with the same sender nonce. |
| Refusal to publish a secret | `service` | The configuration accepts publishable values only, and the bundled scanner checks the built product. |
| Verified, replay-protected webhooks | `service` | Event envelopes are signature-checked, age-checked and lease-claimed on device. |
| Idempotent money moves | `service` | `commit` requires the key, so a retry after a dropped connection returns the original journey. |
