# Paxeer X Network for developers

Paxeer X Network moves money between software at machine speed and hands you evidence you can check without trusting the people who ran the transaction. LayerX is the kernel domain inside Paxeer X Network. This site is the developer surface: how to add payments to something you already have, what each guarantee is actually worth, and which layer is holding it up.

## Beta status

This site documents the platform as it exists in the source revision it is built from, not a qualified release. The canonical beta contract records `readiness_claim: false`, functional surfaces at rung `source_present` without release-bound gate records, so **this site does not establish whole-system runtime qualification**. A capability is only a claim about what the code enforces; the receipt verification, Programs, hosted and deployment behaviour below still have to clear their executed gates before it is a beta. Until a release publishes it, the beta artifact manifest is not emitted and no hosted endpoint, registry package or installer described here is asserted live or installable - treat every URL and package coordinate as a declared beta value, not a ready service.

Two numbers bound the integration surface, and they are not the same kind of claim.

- Adding LayerX payments takes **fewer than ten lines** of integration code. The docs build extracts each sample's integration region and fails when it exceeds its declared budget, and the measured count is printed on the [samples page](reference/samples.md).
- Going from a clean machine to a **verified test payment is the five-minute target**, following only what is published here. This is an acceptance target, not a measured result: it is qualified by a clean-profile run recorded as a gate in the beta evidence ledger, and this page does not supply that gate record.

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

Every SDK is generated from the same two schemas in the same build. The idiom differs; the wire behaviour, the error taxonomy and the receipt verification do not.

Those package names are the beta artifact set declared in the canonical beta contract. This checkout does not provide a published artifact manifest that verifies their availability; build the SDK you need from its source directory under `services/agent/sdk`, `services/agent/crates` or `sdk/platform` until a release publishes the coordinate.

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
