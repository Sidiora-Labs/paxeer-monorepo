# Paxeer X Network for developers

Paxeer X Network moves money between software at machine speed and hands you evidence you can check without trusting the people who ran the transaction. LayerX is the kernel domain inside Paxeer X Network. This site is the developer surface: how to add payments to something you already have, what each guarantee is actually worth, and which layer is holding it up.

## Integration surfaces

The SDKs below provide clients for human-payment services and native transaction or receipt operations. Configure the service and trust policy required by the API you use. Package coordinates identify the packages in this repository; use your release's published artifacts or build from the source directories.

The [samples page](reference/samples.md) lists the integration regions and their line counts. The complete applications also supply configuration, transport, error handling and receipt checks.

## Start in your language

| Language | Package | Integration size |
|---|---|---|
| [TypeScript](quickstart/typescript.md) | `@sidiora/layerx-sdk` | 8 lines |
| [Python](quickstart/python.md) | `layerx-sdk` | 7 lines |
| [Go](quickstart/go.md) | `sdk/platform/go` | 9 lines |
| [Java and Kotlin](quickstart/jvm.md) | `com.sidiora.layerx:layerx-sdk` | 6 lines |
| [Swift](quickstart/swift.md) | `LayerXSDK` | 5 lines |
| [C#](quickstart/csharp.md) | `LayerX.Sdk` | 4 lines |
| [Rust](quickstart/rust.md) | `layerx-sdk` and `layerx-proof` | 8 lines |

SDK generation uses the agent and human schemas. Each language package also supplies its own transport and verification support; follow its quickstart for the available operations.

SDK sources are under `services/agent/sdk`, `services/agent/crates` and `sdk/platform`.

## Or start in your framework

| Framework | What you get | Integration size |
|---|---|---|
| [Express](framework/express.md) | A paid route and a verified webhook endpoint | 8 lines |
| [Next.js](framework/next.md) | A paid route handler, plus a bundle scanner that fails a build shipping a secret | 8 lines |
| [FastAPI](framework/fastapi.md) | A paid route and a verified webhook endpoint | 7 lines |
| [Spring Boot](framework/spring.md) | Auto-configured payment gate and webhook filter | 8 lines |
| [iOS](framework/ios.md) | Payments from an app holding no long-lived credential | 5 lines |
| [Android](framework/android.md) | Payments from an app holding no long-lived credential | 8 lines |

## What to read next

- [Money and accounts](concepts/money.md) if you want the model before the code.
- [Paying for things](concepts/paying.md) for the quote-commit-journey shape every surface shares.
- [Receipts and verification](concepts/receipts.md) if the interesting part is proving a payment happened.
- [Who enforces what](concepts/enforcement.md) before you rely on any guarantee in production.

## Enforced by

The four labels on this site are not stylistic. They tell you what survives a compromise of everything above the named layer.

| Capability | Layer | What that means here |
|---|---|---|
| Atomic settlement | `protocol` | A payment applies completely or not at all, whatever your client, the daemon or a gateway does. |
| Offline receipt verification | `protocol` | You can check a settlement claim with no LayerX component in the path. |
| Quote then commit | `service` | The ordering is enforced by `layerx-human-service`; it binds callers of that service. |
| API keys, usage and request logs | `hosted-surface` | An operational control of the hosted deployment, not a protocol property. |

The [enforcement reference](reference/enforcement.md) lists every capability this documentation states, grouped by the layer that holds it.
