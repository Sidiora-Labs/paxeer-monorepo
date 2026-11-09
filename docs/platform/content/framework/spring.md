# Spring Boot quickstart

Charge for an endpoint in a Spring Boot service. Two beans; the starter wires the filters.

## Before you start

```text
<dependency>
  <groupId>com.sidiora.layerx</groupId>
  <artifactId>layerx-spring-boot-starter</artifactId>
  <version>0.1.0</version>
</dependency>
```

Configuration is ordinary Spring configuration under the `layerx` prefix, so it binds from `application.yaml`, from the environment, or from whatever property source you already use.

```text
layerx:
  principal: ${LAYERX_PRINCIPAL}
  protected-path: ${LAYERX_PROTECTED_PATH:/paid}
  payment:
    scheme: ${LAYERX_X402_SCHEME}
    network: ${LAYERX_X402_NETWORK}
    price: ${LAYERX_PRICE}
    asset: ${LAYERX_ASSET}
    pay-to: ${LAYERX_PAY_TO}
  authorized-batch:
    batch-id: ${LAYERX_BATCH_ID}
    asset: ${LAYERX_BATCH_ASSET}
    previous-state-root: ${LAYERX_PREVIOUS_STATE_ROOT}
    resulting-state-root: ${LAYERX_RESULTING_STATE_ROOT}
    sequencer-public-key: ${LAYERX_SEQUENCER_PUBLIC_KEY}
  webhook:
    path: ${LAYERX_WEBHOOK_PATH:/layerx/webhooks}
    public-keys:
      ${LAYERX_WEBHOOK_KEY_ID:primary}: ${LAYERX_WEBHOOK_PUBLIC_KEY}
```

The bundled `application.yaml` in the sample carries the full set.

The `authorized-batch` configuration supplies a local compatibility verifier with caller-supplied batch facts, like `layerx receipt verify`. Use independently trusted facts; a successful compatibility check does not establish block commitment.

A receipt is final once its block is committed. The network rungs are `submitted`, `receipted` and `committed`; `checkpoint-finalised` and `done-finalised` are retained API names for the `committed` rung. `settlement-anchored` and `paxeer-finalised` are never reached on the merged network. For finality, authenticate the block header and CometBFT commit, then verify transaction or state inclusion at the relevant height. See [Committed block evidence](https://github.com/Sidiora-Labs/paxeer-monorepo/blob/abc81fd2cead6b2b9d5612c6e09a6ab56f13c1ed/docs/site/docs/concepts/checkpoints.md#committed-block-evidence).

## The integration

```java sample=paid-endpoint-spring
@Bean
LayerXResourceHandler layerXResourceHandler(@Value("${paid.resource-file:./resource.json}") String file) {
    return request -> new LayerXResource("application/json", Files.readAllBytes(Path.of(file)));
}

@Bean
LayerXWebhookEventHandler layerXWebhookEventHandler() {
    return (JsonNode event, String deliveryId) -> record(deliveryId, event);
}
```

That is the whole integration. The auto-configuration supplies the declared config, the authorised batch resolver, the payment authority, the seller middleware, the webhook consumer, and both filter registrations. It backs off from any bean you define yourself, so replacing the fulfilment repository or the delivery store is a matter of declaring one.

| Bean you declare | Effect |
|---|---|
| `LayerXResourceHandler` | Registers the payment gate filter on your protected path |
| `LayerXWebhookEventHandler` | Registers the webhook filter on your webhook path |
| `Fulfillments.FulfillmentRepository` | Replaces the in-memory default |
| `Webhooks.DeliveryStore` | Replaces the in-memory default |

## Before production: replace the defaults

`LayerXAutoConfiguration` defaults the fulfilment repository and the webhook delivery store to in-memory implementations. They are correct and they are not durable. A restart loses the exactly-once fulfilment guarantee and the webhook replay protection with it. Declare durable beans of both types before you take real money.

The starter also runs `PublishedSecretGuard` over the environment at startup and refuses to configure when a declared secret has been copied into a published-prefix variable.

## Run it

```text
cd services/platform/examples/samples/paid-endpoint-spring
mvn -q package
mvn -q spring-boot:run
```

## Enforced by

| Capability | Layer | What that means here |
|---|---|---|
| Offline receipt verification | `protocol` | The filter performs the local compatibility check against configured batch facts; finality requires committed block evidence. |
| Atomic settlement | `protocol` | The payment behind a released resource happened whole or not at all. |
| Receipt-gated resource release | `service` | The filter releases only against a verified receipt. |
| Exactly-once fulfilment | `service` | In-memory by default. Not durable until you declare a durable repository bean. |
| Verified, replay-protected webhooks | `service` | In-memory by default. Not durable until you declare a durable delivery store bean. |
| Refusal to publish a secret | `service` | Startup fails when a declared secret appears under a published prefix. |
