# LayerX SDK integrations

This directory holds framework adapters, mobile bindings and examples for applications using the Paxeer X Network. The web adapters mount receipt-gated x402 payments and verified webhooks, the agent adapters connect framework tools to authenticated agent operations, and the mobile bindings use the Human API with brokered sessions. These packages use the network's SDKs and middleware; receipt verification authenticates execution evidence, while finality requires a committed block.

## Layout

- [agents/](agents/) — TypeScript adapters for MCP, A2A, OpenAI, Anthropic, LangChain and the Vercel AI SDK, with runtime examples and tests.
- [android/](android/) — Java mobile binding, event verification, receipt checks, Maven tests and an Android sample application.
- [express/](express/) — Express payment and webhook middleware, an example application and tests.
- [fastapi/](fastapi/) — Python payment and webhook integration, wheel build backend, examples and tests.
- [ios/](ios/) — Swift mobile binding, sample targets, event and receipt verification, artifact scanner and tests.
- [next/](next/) — Next.js route adapters, browser bundle scanner, example application and tests.
- [spring/](spring/) — Spring Boot auto-configuration, payment and webhook filters, durable stores, example application and tests.
- [build-dependencies.mjs](build-dependencies.mjs) — Reads the root npm workspace manifests and builds a selected workspace's dependencies in dependency order.
- [.gitignore](.gitignore) — Excludes Maven target directories.

## Build and test

Run commands from the repository root. Each integration has its own manifest; this directory has no shared package manifest or Makefile. The root [package.json](../../package.json) declares the JavaScript workspaces, and [services/platform/Makefile.inc](../../services/platform/Makefile.inc) supplies the integration targets included by the root Makefile.

```sh
make platform-js-install
make platform-dependencies-install
make platform-test-middleware
```

The install targets prepare JavaScript dependencies, the Python environment, local JVM artifacts and the other platform dependencies. Maven dependencies must be available for recipes that run offline. `platform-test-middleware` builds the Express and Next.js packages, runs their tests and the agent adapter tests, compiles and tests FastAPI, packages the Android binding and builds and tests the Swift binding. The local JVM artifact installation runs the Spring and Android Maven test suites.

For individual packages, use the commands declared by their manifests or the platform Make recipes:

| Package | Commands | Definition |
| --- | --- | --- |
| Agent adapters | `npm run build --workspace @sidiora/layerx-agent-integrations`; `npm test --workspace @sidiora/layerx-agent-integrations` | [agents/package.json](agents/package.json) |
| Express | `npm run build --workspace @sidiora/layerx-express`; `npm test --workspace @sidiora/layerx-express` | [express/package.json](express/package.json) |
| Next.js | `npm run build --workspace @sidiora/layerx-next`; `npm test --workspace @sidiora/layerx-next` | [next/package.json](next/package.json) |
| FastAPI | `PYTHONPATH=services/agent/sdk/python:sdk/integrations/fastapi build/platform-python/bin/python -m pytest -q sdk/integrations/fastapi` | [Platform Make recipes](../../services/platform/Makefile.inc), [pyproject.toml](fastapi/pyproject.toml) |
| Spring and Android | `make platform-install-local-artifacts` | [Platform Make recipes](../../services/platform/Makefile.inc), [Spring pom.xml](spring/pom.xml), [Android pom.xml](android/pom.xml) |
| iOS | `swift build --disable-automatic-resolution --package-path sdk/integrations/ios`; `swift test --package-path sdk/integrations/ios` | [Platform Make recipes](../../services/platform/Makefile.inc), [Package.swift](ios/Package.swift) |

The JavaScript packages' build hooks invoke `build-dependencies.mjs`; their test hooks build the package before running tests. Express and Next.js also run their example tests. FastAPI's [build_backend.py](fastapi/build_backend.py) supplies the wheel backend selected by its manifest.

## Configuration

Configuration belongs to the selected integration and application. The dependency build script reads the root `package.json` and each declared workspace's `package.json`.

**Web adapters.** [Express](express/src/index.ts), [Next.js](next/src/index.ts) and [FastAPI](fastapi/layerx_fastapi/config.py) read these configuration names from an environment mapping:

- Resource and principal: `LAYERX_PRINCIPAL`, `LAYERX_PROTECTED_PATH`, `LAYERX_RESOURCE_URL`, `LAYERX_RESOURCE_DESCRIPTION`, `LAYERX_RESOURCE_MIME_TYPE`, `LAYERX_RESOURCE_SERVICE_NAME`.
- Payment and receipt verification: `LAYERX_X402_SCHEME`, `LAYERX_X402_NETWORK`, `LAYERX_PRICE`, `LAYERX_ASSET`, `LAYERX_PAY_TO`, `LAYERX_PAYMENT_TIMEOUT_SECONDS`, `LAYERX_RECEIPT_KEYS_JSON`.
- Webhooks: `LAYERX_WEBHOOK_PATH`, `LAYERX_WEBHOOK_PUBLIC_KEYS_JSON`, `LAYERX_WEBHOOK_MAX_AGE_MS`, `LAYERX_WEBHOOK_LEASE_MS`.
- Buyer settings: `LAYERX_HUMAN_URL`, `LAYERX_SOURCE`, `LAYERX_TOKEN`.

Express and Next.js also read `LAYERX_PROTOCOL_VERSION`; Next.js reads `LAYERX_PAYMENT_PURPOSE_HASH`. The Express and FastAPI examples read `LAYERX_RESOURCE_FILE` and `LAYERX_FULFILLMENT_DIR`; the Next.js example reads `LAYERX_FULFILLMENT_DIR`. Their resource files are [express/example/resource.json](express/example/resource.json), [fastapi/examples/resource.json](fastapi/examples/resource.json) and [next/example/resource.json](next/example/resource.json). The Express example also reads `PORT`.

**Agent adapters.** [agents/src/config.ts](agents/src/config.ts) declares the following names:

- Transport and identity: `LAYERX_AGENT_RPC_URL`, `LAYERX_AGENT_SURFACE`, `LAYERX_PROTOCOL_VERSION`, `LAYERX_TENANT`, `LAYERX_ACTOR`, `LAYERX_AUTHORITY`.
- Authentication: `LAYERX_SESSION_ID`, `LAYERX_SESSION_TOKEN`, `LAYERX_SESSION_GENERATION`, `LAYERX_GATEWAY_KEY_ID`, `LAYERX_GATEWAY_KEY`, `LAYERX_CLIENT_CERTIFICATE_PEM`, `LAYERX_CLIENT_KEY_PEM`, `LAYERX_TRUSTED_CA_PEM`, `LAYERX_TOKEN`.
- Services and spending: `LAYERX_AGENT_SERVICES_MODULE`, `LAYERX_BUDGET_SERVICE_URL`, `LAYERX_SIGNER_SERVICE_URL`, `LAYERX_RECEIPT_SERVICE_URL`, `LAYERX_RECEIPT_KEYS_JSON`, `LAYERX_FEE_LIMIT`, `LAYERX_MAX_TRACK_POLLS`, `LAYERX_REQUEST_TIMEOUT_MS`.
- Webhooks and A2A: `LAYERX_WEBHOOK_PUBLIC_KEYS_JSON`, `LAYERX_WEBHOOK_MAX_AGE_MS`, `LAYERX_WEBHOOK_LEASE_MS`, `LAYERX_WEBHOOK_DELIVERY_STORE_PATH`, `LAYERX_WEBHOOK_LISTEN_HOST`, `LAYERX_WEBHOOK_LISTEN_PORT`, `LAYERX_A2A_URL`.

The [service provider loader](agents/src/service-providers.ts) imports the module named by `LAYERX_AGENT_SERVICES_MODULE`. The examples additionally read `LAYERX_AGENT_FRAMEWORK`, `LAYERX_SPEND_TOOL_INPUT_JSON`, `LAYERX_WEBHOOK_DELIVERY_PATH`, `LAYERX_WEBHOOK_EXPECT` and `LAYERX_WEBHOOK_PATH`.

**Spring Boot.** [LayerXProperties](spring/src/main/java/com/sidiora/layerx/spring/LayerXProperties.java) binds the `layerx` configuration prefix. The example's [application.yaml](spring/example/src/main/resources/application.yaml) reads the resource and payment names above, plus `LAYERX_STORAGE_DIRECTORY`, `LAYERX_RECEIPT_KEY_ID`, `LAYERX_RECEIPT_PUBLIC_KEY`, `LAYERX_RECEIPT_FIRST_HEIGHT`, `LAYERX_RECEIPT_LAST_HEIGHT`, `LAYERX_WEBHOOK_KEY_ID` and `LAYERX_WEBHOOK_PUBLIC_KEY`. It uses individual receipt and webhook key settings. The example application also reads `example.resource-file` to select its resource file. Configure `layerx.storage-directory` for the default durable stores; `layerx.payment.purpose-hash` supplies the purpose for non-exact payment schemes.

**Mobile bindings.** The [Swift](ios/Sources/LayerXMobile/PublishableConfiguration.swift) and [Java](android/src/main/java/com/sidiora/layerx/android/PublishableConfiguration.java) configuration readers accept declared settings, an application-selected JSON file or an environment mapping:

| Declared setting | Environment name |
| --- | --- |
| `layerx.service_url` | `LAYERX_SERVICE_URL` |
| `layerx.session_broker_url` | `LAYERX_SESSION_BROKER_URL` |
| `layerx.event_public_key.` prefix | `LAYERX_EVENT_PUBLIC_KEY_` prefix |
| `layerx.event_max_age_seconds` | `LAYERX_EVENT_MAX_AGE_SECONDS` |
| `layerx.request_timeout_seconds` | `LAYERX_REQUEST_TIMEOUT_SECONDS` |
| `layerx.receipt_keys` | `LAYERX_RECEIPT_KEYS_JSON` |

Both also accept the pair `LAYERX_EVENT_KEY_ID` and `LAYERX_EVENT_PUBLIC_KEY`. The iOS application reads the `LayerX` and `LayerXSample` dictionaries in [Info.plist](ios/sample-app/Info.plist); the Android application reads string resources from [strings.xml](android/sample-app/app/src/main/res/values/strings.xml). The Java file delivery store reads the system properties `com.sidiora.layerx.deliveryStore` and `user.home` when selecting its storage path.

The mobile command-line samples read `LAYERX_RECEIPT_RELAY_URL`, `LAYERX_SAMPLE_QUOTE_JSON`, `LAYERX_SAMPLE_IDEMPOTENCY_KEY`, `LAYERX_SAMPLE_ASSET`, `LAYERX_SAMPLE_RECIPIENT`, `LAYERX_SAMPLE_AMOUNT` and `LAYERX_SAMPLE_DELIVERY_STORE_PATH`. Event input uses `LAYERX_SAMPLE_WEBHOOK_DELIVERY_PATH`, or `LAYERX_SAMPLE_EVENT_PATH` with `LAYERX_SAMPLE_EVENT_ID`, `LAYERX_SAMPLE_EVENT_TIMESTAMP`, `LAYERX_SAMPLE_EVENT_KEY_ID` and `LAYERX_SAMPLE_EVENT_SIGNATURE`.

## Documentation

- Framework guides: [Express](../../docs/platform/content/framework/express.md), [Next.js](../../docs/platform/content/framework/next.md), [FastAPI](../../docs/platform/content/framework/fastapi.md), [Spring Boot](../../docs/platform/content/framework/spring.md), [iOS](../../docs/platform/content/framework/ios.md) and [Android](../../docs/platform/content/framework/android.md).
- Agent integration concepts: [Agent middleware](../../docs/platform/content/guide/agent-middleware.md) and [Agent API](../../docs/platform/content/reference/agent-api.md).
- Documentation site: [SDK quickstarts](../../docs/site/paxeer-docs/public/markdown/sdk-quickstarts.md), [Agents](../../docs/site/paxeer-docs/public/markdown/agents.md), [MCP tools](../../docs/site/paxeer-docs/public/markdown/mcp.md), [A2A](../../docs/site/paxeer-docs/public/markdown/a2a.md), [Receipts](../../docs/site/paxeer-docs/public/markdown/receipts.md) and [Webhooks](../../docs/site/paxeer-docs/public/markdown/webhooks.md).
