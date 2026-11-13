# Android quickstart

Use the human payment API from an Android app with brokered session tokens.

## The credential model

The binding uses the same credential model as [iOS](ios.md):

- The app carries a **publishable configuration** only - service URL, session broker URL, event public keys, timeouts.
- The binding exchanges it for **short-lived session tokens** from a broker you run, re-fetching when a capability refusal says the token is stale.
- Your bearer token stays on your server.

## Before you start

```xml
<dependency>
  <groupId>com.sidiora.layerx</groupId>
  <artifactId>layerx-android</artifactId>
  <version>0.1.0</version>
</dependency>
```

`PublishableConfiguration` accepts declared keys directly, from a JSON file shipped as a resource, or from the environment:

| Declared key | Environment variable |
|---|---|
| `layerx.service_url` | `LAYERX_SERVICE_URL` |
| `layerx.session_broker_url` | `LAYERX_SESSION_BROKER_URL` |
| `layerx.event_public_key.<key-id>` | `LAYERX_EVENT_PUBLIC_KEY_<KEY_ID>` |
| `layerx.event_max_age_seconds` | `LAYERX_EVENT_MAX_AGE_SECONDS` |
| `layerx.request_timeout_seconds` | `LAYERX_REQUEST_TIMEOUT_SECONDS` |
| `layerx.receipt_keys` | `LAYERX_RECEIPT_KEYS_JSON` |

## The integration

```java sample=mobile-payment-android
static LayerXAndroid openLayerX() {
    return LayerXAndroid.create(PublishableConfiguration.ofEnvironment(System.getenv()));
}

static JsonNode pay(LayerXAndroid layerx, ObjectNode move, String paymentKey) {
    var quote = layerx.client().quote(move).toCompletableFuture().join();
    var commit = layerx.mapper().createObjectNode().put("quote_id", quote.path("quote_id").asText());
    return layerx.client().commit(commit, new IdempotencyKey(paymentKey)).toCompletableFuture().join();
}
```

`LayerXAndroid` is `AutoCloseable`; closing it shuts the transport and the session provider down together, so a session token does not outlive the object that fetched it. On devices whose platform provider lacks Ed25519 the binding installs Bouncy Castle at position one, so event verification behaves identically across API levels.

`layerx.client()` provides `version`, `profile`, `activity`, `activityEntry`, `journeys`, `journey`, `quote` and `commit`, plus stream operations. Request methods return `CompletionStage`; `streamSource()` supplies the resumable stream source.

## Receipt checks

Configure `LAYERX_RECEIPT_KEYS_JSON` independently of received proofs before using `gate()`. It verifies signed receipt evidence and matches payment facts before the app treats a journey as paid.

This setting is a JSON array whose entries contain `keyId` and `publicKey` as `0x`-prefixed 32-byte hex strings and `firstHeight` and `lastHeight` as JSON integers. The height range is inclusive. Receipt verification establishes execution evidence; finality also requires committed block evidence.

## Verified push and webhook events

`layerx.consume(rawBody, headerFields, handler)` verifies the envelope against your configured public keys, refuses stale deliveries and claims each delivery under a lease. The default `FileEventDeliveryStore` retains delivery state on disk; Android callers can supply a `Context` to select app-private storage.

## Never ship a secret

`EmbeddedSecretScan` walks a built artifact for embedded credentials. Run it in your release pipeline, with `configuration.exemptScannerValues()` supplying the strings that are legitimately public.

## Run the sample

Set the service and session-broker URLs, at least one event public key, and
`LAYERX_SOURCE`, `LAYERX_DESTINATION`, `LAYERX_AMOUNT`, `LAYERX_CURRENCY` and
`LAYERX_PAYMENT_KEY` before running. The service URL must identify the human
API; local emulator inspection routes do not submit payments.

```sh
cd services/platform/examples/samples/mobile-payment-android
mvn -q package
mvn -q exec:java
```

The sample runs on a plain JVM so it needs no device or emulator, and the API it uses is the one an `Activity` uses.

## Enforced by

| Capability | Layer | What that means here |
|---|---|---|
| Atomic settlement | `protocol` | The committed move applies completely or not at all. |
| Replay refusal | `protocol` | The same signed transaction cannot apply twice with the same sender nonce. |
| Refusal to publish a secret | `service` | The configuration accepts publishable values only, and the bundled scanner checks the built artifact. |
| Verified, replay-protected webhooks | `service` | Event envelopes are signature-checked, age-checked and lease-claimed on device. |
| Idempotent money moves | `service` | `commit` requires the key, so a retry after a dropped connection returns the original journey. |
