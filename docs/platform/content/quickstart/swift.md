# Swift quickstart

Add a payment to a Swift application. Five lines, no protocol vocabulary, no key handling.

## Before you start

Use Swift 5.9 or newer. Configure an authenticated human-payment service that supports
`/v1/moves/quote`, `/v1/moves` and journey reads. The local emulator retires
these payment-write routes. For native node transactions, follow the
[reference applications](../guide/reference-applications.md).

Add the package to your `Package.swift`:

```swift
.package(path: "sdk/platform/swift")
```

| Variable | What it is |
|---|---|
| `LAYERX_API_URL` | The base URL of your human-payment service |
| `LAYERX_API_TOKEN` | A bearer token identifying your account |

## The integration

```swift sample=first-payment-swift
let token = try AccessToken(Data(apiToken.utf8))
let layerx = PlatformClient(transport: try HumanHTTPTransport(baseURL: serviceURL, accessToken: token))
let quote = try await layerx.humanMoveQuote(.object(["source": .string(source), "destination": .string(destination), "money": money]))
guard let quoteID = quote.objectValue?["quote_id"]?.stringValue else { fail("move quote omitted quote_id") }
var journey = try await layerx.humanMoveCommit(.object(["quote_id": .string(quoteID)]), idempotencyKey: paymentKey)
```

`AccessToken` is a locked container: it refuses empty material, redacts itself as `[REDACTED]`, and zeroes its storage on `destroy()` and on deinit. Every call is `async throws`, and `humanMoveCommit` takes the idempotency key as a required argument, so there is no way to spell a money-moving call without one.

`JSONValue` is a closed enum with `objectValue`, `stringValue` and `integerValue` accessors. It has no floating-point case, so an amount cannot be silently turned into a double.

## Run the whole sample

Set `LAYERX_API_URL` and `LAYERX_API_TOKEN` for that service, plus
`LAYERX_SOURCE`, `LAYERX_DESTINATION`, `LAYERX_AMOUNT`, `LAYERX_CURRENCY`
and `LAYERX_PAYMENT_KEY` for the payment. Amounts use integer base units;
reuse the same payment key when reconciling the same operation.

```sh
cd services/platform/examples/samples/first-payment-swift
swift build
swift run FirstPayment
```

The sample polls `humanJourneyGet(pathParameters:)` up to 40 times at 250 ms intervals, emits a JSON report, and destroys the token on the way out.

## If you are building an app, not a server

Do not put a bearer token in a client binary. The iOS binding exists precisely for that case: it holds a publishable configuration only and exchanges it for short-lived session tokens through a broker you run. See the [iOS quickstart](../framework/ios.md).

## Handling refusals

`PlatformSDKError` carries `code`, `retry`, `requestID`, `protocolResultCode` and `retryAfterMilliseconds`. Catch it specifically.

| Code | What to do |
|---|---|
| `idempotencyRequired` | You omitted the key |
| `idempotencyConflict` | Same key, different body |
| `rateLimit` | Wait for `retryAfterMilliseconds` |
| `unknownOutcome` | Do not retry. Resolve by receipt lookup under your key |
| `budgetRefusal` | The payment exceeded a funded budget |

## Enforced by

| Capability | Layer | What that means here |
|---|---|---|
| Atomic settlement | `protocol` | The committed move applies completely or not at all. |
| Replay refusal | `protocol` | A retried submission cannot apply twice. |
| Idempotent money moves | `service` | The commit signature makes the key impossible to omit. |
| Quote then commit | `service` | The quote you committed is the one that executes. |
| Done means verified | `service` | The `done` state is backed by receipt evidence. |
