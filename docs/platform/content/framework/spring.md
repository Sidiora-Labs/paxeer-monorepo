# Spring Boot quickstart

Charge for an endpoint in a Spring Boot service. Two beans; the starter wires the filters.

## Before you start

```xml
<dependency>
  <groupId>com.sidiora.layerx</groupId>
  <artifactId>layerx-spring-boot-starter</artifactId>
  <version>0.1.0</version>
</dependency>
```

Configuration is ordinary Spring configuration under the `layerx` prefix, so it binds from `application.yaml`, from the environment, or from whatever property source you already use.

```yaml
layerx:
  storage-directory: ${LAYERX_STORAGE_DIRECTORY}
  principal: ${LAYERX_PRINCIPAL}
  protected-path: ${LAYERX_PROTECTED_PATH:/paid}
  resource:
    url: ${LAYERX_RESOURCE_URL}
    description: ${LAYERX_RESOURCE_DESCRIPTION:}
    mime-type: ${LAYERX_RESOURCE_MIME_TYPE:application/json}
    service-name: ${LAYERX_RESOURCE_SERVICE_NAME:}
  payment:
    scheme: ${LAYERX_X402_SCHEME}
    network: ${LAYERX_X402_NETWORK}
    price: ${LAYERX_PRICE}
    asset: ${LAYERX_ASSET}
    pay-to: ${LAYERX_PAY_TO}
    timeout-seconds: ${LAYERX_PAYMENT_TIMEOUT_SECONDS:120}
  receipt-keys:
    entries:
      - key-id: ${LAYERX_RECEIPT_KEY_ID}
        public-key: ${LAYERX_RECEIPT_PUBLIC_KEY}
        first-height: ${LAYERX_RECEIPT_FIRST_HEIGHT}
        last-height: ${LAYERX_RECEIPT_LAST_HEIGHT}
  webhook:
    path: ${LAYERX_WEBHOOK_PATH:/layerx/webhooks}
    public-keys:
      ${LAYERX_WEBHOOK_KEY_ID:primary}: ${LAYERX_WEBHOOK_PUBLIC_KEY}
```

`resource.url` and a positive payment timeout are required by `LayerXProperties`; the example configuration supplies the timeout default shown above. Receipt key identifiers and public keys are 32-byte hex values; the height bounds are inclusive. Non-exact schemes also require `layerx.payment.purpose-hash` as a nonzero 64-character lowercase hex string without a prefix.

The `receipt-keys.entries` configuration supplies the independently trusted receipt keys. Signed receipt verification authenticates execution; finality additionally requires committed block evidence.

A receipt is final once its block is committed. The network rungs are `submitted`, `receipted` and `committed`; `checkpoint-finalised` and `done-finalised` are retained API names for the `committed` rung. `settlement-anchored` and `paxeer-finalised` are never reached on the merged network. For finality, authenticate the block header and CometBFT commit, then verify transaction or state inclusion at the relevant height. See [Committed block evidence](https://docs.paxeer.app/checkpoints/).

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

The auto-configuration supplies the declared config, payment authority, seller middleware and webhook consumer when their corresponding beans are absent. It registers the payment filter when a `LayerXResourceHandler` exists and the webhook filter when a `LayerXWebhookEventHandler` exists. Declare fulfilment-repository or delivery-store beans to replace the corresponding default stores.

| Bean you declare | Effect |
|---|---|
| `LayerXResourceHandler` | Registers the payment gate filter on your protected path |
| `LayerXWebhookEventHandler` | Registers the webhook filter on your webhook path |
| `Fulfillments.FulfillmentRepository` | Replaces the default file-backed store |
| `Webhooks.DeliveryStore` | Replaces the default file-backed store |

## Durable storage

`LayerXAutoConfiguration` creates file-backed fulfilment and webhook delivery stores under `layerx.storage-directory`. Configure a private durable directory, or supply repository and delivery-store beans appropriate to the deployment. The default stores retain state across process restarts.

The starter also runs `PublishedSecretGuard` over the environment at startup and refuses to configure when a declared secret has been copied into a published-prefix variable.

## Run it

The integration example includes the receipt-key and storage configuration shown above. Supply its required environment values before starting:

```sh
cd sdk/integrations/spring/example
mvn -q package
mvn -q spring-boot:run
```

## Enforced by

| Capability | Layer | What that means here |
|---|---|---|
| Offline receipt verification | `protocol` | The filter checks signed receipts against pinned keys; finality requires committed block evidence. |
| Atomic settlement | `protocol` | The payment behind a released resource happened whole or not at all. |
| Receipt-gated resource release | `service` | The filter releases only against a verified receipt. |
| Exactly-once fulfilment | `service` | The default repository persists fulfilment records in the configured storage directory. |
| Verified, replay-protected webhooks | `service` | The default delivery store persists signature-checked delivery claims in the configured storage directory. |
| Refusal to publish a secret | `service` | Startup fails when a declared secret appears under a published prefix. |
