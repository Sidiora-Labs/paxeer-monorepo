# Java and Kotlin quickstart

Use the human-payment client from a JVM service. The integration region has six lines; the complete sample supplies configuration and journey polling.

## Before you start

Use JDK 21 and Maven. Configure an authenticated human-payment service that supports
`/v1/moves/quote`, `/v1/moves` and journey reads. The local emulator does not implement
these payment-write routes. For native node transactions, follow the
[reference applications](../guide/reference-applications.md).

Add the SDK to your build:

```xml
<dependency>
  <groupId>com.sidiora.layerx</groupId>
  <artifactId>layerx-sdk</artifactId>
  <version>0.1.0</version>
</dependency>
```

| Variable | What it is |
|---|---|
| `LAYERX_API_URL` | The base URL of your human-payment service |
| `LAYERX_API_TOKEN` | A bearer token identifying your account |

## The integration

```java sample=first-payment-jvm
var credential = new HttpProductionTransport.BearerCredential(new SecretBytes(apiToken.getBytes(StandardCharsets.UTF_8)));
var layerx = new ProductionClient(HttpProductionTransport.create(URI.create(apiUrl), URI.create(apiUrl), credential));
var quote = layerx.human("move.quote", Map.of("source", source, "destination", destination, "money", money),
    JsonNode.class, ProductionClient.Options.none()).toCompletableFuture().join();
var journey = layerx.human("move.commit", Map.of("quote_id", quote.path("quote_id").asText()), JsonNode.class,
    ProductionClient.Options.idempotent(new IdempotencyKey(paymentKey))).toCompletableFuture().join();
```

`SecretBytes` holds the token, redacts itself in `toString` and zeroes its array when closed. `HttpProductionTransport.create` takes the agent-plane and human-plane base URIs; here they are the same endpoint.

Every call returns a `CompletionStage`. The sample joins it because it is a command-line program; in a service, compose the stages instead.

`ProductionClient.Options.idempotent(...)` is required on `move.commit`. `Options.none()` on that operation fails locally with `idempotency-required`.

Kotlin callers get the same API through the `LayerXKotlin` extensions: `human(operation, request, KClass, options)` with `Options` defaulted, plus `protocolAmount`, `idempotencyKey` and `IdempotencyKey.asOptions`.

## Run the whole sample

Set `LAYERX_API_URL` and `LAYERX_API_TOKEN` for that service, plus
`LAYERX_SOURCE`, `LAYERX_DESTINATION`, `LAYERX_AMOUNT`, `LAYERX_CURRENCY`
and `LAYERX_PAYMENT_KEY` for the payment. Amounts use integer base units;
reuse the same payment key when reconciling the same operation.

```sh
cd services/platform/examples/samples/first-payment-jvm
mvn -q package
mvn -q exec:java
```

## Reading responses safely

Use `path` rather than `get` when reading a field out of a response node. `path` returns a missing node; `get` returns `null` and turns a protocol-level absence into a `NullPointerException` three frames away from the cause.

## Handling refusals

`PlatformSdkException` exposes `code()` and `retry()` enum values. `code().wire()` returns the strings in the table. When joining a stage, unwrap `CompletionException` to recover the SDK exception.

| Code | What to do |
|---|---|
| `idempotency-required` | Pass `Options.idempotent(...)` |
| `idempotency-conflict` | Same key, different body |
| `rate-limit` | Wait for the carried retry timing |
| `unknown-outcome` | Reconcile the original journey and its evidence before another payment |
| `budget-refusal` | The service reported a budget or limit refusal; inspect its evidence for the enforcement layer |

## Enforced by

| Capability | Layer | What that means here |
|---|---|---|
| Atomic settlement | `protocol` | The committed move applies completely or not at all. |
| Replay refusal | `protocol` | Rebroadcasting the same signed transaction cannot apply twice. |
| Idempotent money moves | `service` | Repeating the commit with the same key returns the original journey. |
| Quote then commit | `service` | The quote you committed is the one that executes. |
| Done means verified | `service` | The `done` state is backed by receipt evidence. |

The sample reports the service's journey state and evidence references. It does not independently verify receipt bytes or block commitment.
