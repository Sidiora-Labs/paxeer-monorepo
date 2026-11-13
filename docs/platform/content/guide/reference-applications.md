# Reference applications

Four applications live under `services/platform/examples`: a buyer agent, a paid HTTP API, a merchant shop and a marketplace backed by a kernel program. Their packages and launch commands are listed in `services/platform/examples/reference-apps.json`; each application's `layerx.example.json` declares its profile bindings.

## Build and launch

From the repository root, install and build the JavaScript workspaces:

```sh
npm ci
npm run build
```

The build orders workspaces by their declared dependencies. Configure the selected application's environment before launching it:

```sh
npm run start:beta --workspace @sidiora/layerx-example-buyer-agent
npm run start:beta --workspace @sidiora/layerx-example-paid-api
npm run start:beta --workspace @sidiora/layerx-example-merchant-shop
npm run start:beta --workspace @sidiora/layerx-example-marketplace
```

These are separate applications with different prerequisites; starting them does not provision a node, human-payment service, account or signing key. `merchant-checkout` is a compatibility alias for `merchant-shop`. Local profiles use `start:emulator`; their endpoints and services must also be supplied separately.

## Configuration and evidence

Beta profiles use the mainnet-beta node root on chain ID 125. They select pinned Ed25519 receipt keys through `receiptKeysEnvironment`, rather than a `protocolVersion` setting or static batch authority. The paid API reads `LAYERX_RECEIPT_KEYS_JSON`; the other three profiles read `LAYERX_TESTNET_RECEIPT_KEYS_JSON`. Key IDs, public keys and inclusive height ranges must come from independently trusted configuration.

The buyer also needs its configured human-payment service and credential. The paid API needs the asset, seller, seller account, currency and price bindings declared in its profile; it exposes `GET /paid` on loopback port 8081. The merchant needs its settlement service and webhook verification keys. The marketplace needs a deployed program and the transaction inputs described in its source guide. See the [reference application configuration guide](https://github.com/Sidiora-Labs/paxeer-monorepo/blob/main/services/platform/examples/README.md) for each binding.

Native payment paths bind signed EIP-1559 transaction bytes to signed execution receipts and independently pinned receipt keys. Pending, unknown and refused outcomes remain distinct. A verified signed receipt establishes `receipted`; finality requires authenticated committed-block evidence. See [receipts](receipts.md).

The marketplace program stores listings and consumed receipt digests in program-shared storage. A purchase requires verified, unused receipt evidence matching the listing's asset and price. Receipt consumption, the bounded transfer request, listing deletion and purchase event occur in one atomic program call.

## Enforced by

| Capability | Layer | What that means here |
| --- | --- | --- |
| Receipt-gated resource release | `service` | Seller middleware checks payment evidence before releasing a resource. |
| Durable fulfilment | `service` | Stored payment claims prevent a release from being repeated after an uncertain outcome. |
| Webhook authentication | `service` | The merchant checks signatures and delivery replay claims before handling settlement events. |
| Bounded program transfers | `protocol` | The marketplace uses the transfer authority supplied to its call. |
