# Platform SDKs

This directory holds the Go, JVM, .NET and Swift SDKs for Paxeer X Network,
along with shared schemas, code generators and conformance fixtures. The SDKs
provide Agent and Human API clients, receipt verification, streaming and native
Programs helpers. The LayerX kernel runs inside `paxd`, executes and orders each
transaction and builds each block; validators check and commit it. A receipt is
final when its block is committed. Applications
configure node JSON-RPC and authenticated application services separately.

## Layout

- [conformance/](conformance/) — shared operation catalogues, receipt fixtures, cross-language tests and Go/JVM runners.
- [conformance-runner/](conformance-runner/) — TypeScript assertion adapter and module loader for running shared tests with Node's test runner.
- [dotnet/](dotnet/) — the `LayerX.Sdk` library, generated contracts and .NET tests.
- [generators/](generators/) — the `layerx-platform-sdkgen` Rust crate, Python generators and receipt contract.
- [go/](go/) — the Go SDK module, HTTP transports, receipt verifiers and conformance executable.
- [jvm/](jvm/) — the Java and Kotlin SDK, Maven manifest, examples and tests.
- [schema/](schema/) — mirror verification and PAXAI client contracts.
- [swift/](swift/) — the `LayerXSDK` Swift package, generated contracts and tests.
- [pipeline.kvx](pipeline.kvx) — schema sources, generators and generated output digests.
- [.gitignore](.gitignore) — Rust build output exclusions.

## Build and test

Run these targets from the repository root. They are defined in
[services/platform/Makefile.inc](../../services/platform/Makefile.inc), which the
root [Makefile](../../Makefile) includes.

| Command | Purpose |
| --- | --- |
| `make platform-dependencies-install` | Install platform dependency tooling, SDK/sample dependencies and local JVM artifacts. |
| `make platform-js-install` | Install the npm workspaces used by TypeScript builds and shared tests. |
| `make platform-build-all` | Build the platform services, tools, generators, language SDKs and samples. |
| `make platform-sdk-check` | Check generated SDK contracts and output digests against their sources. |
| `make platform-verify-sdks` | Run shared conformance tests and the language SDK test runners. |

The build uses Cargo, Node/npm, Go, Python, Maven, Swift and the .NET SDK.
Several build and test commands use cached dependencies or disable dependency
resolution, so install dependencies first. Language build commands from the
same Makefile can also be run separately:

```sh
cd sdk/platform/go && GOPROXY=off go build ./...
```

Run each of the following from the repository root:

```sh
mvn -o -q -f sdk/platform/jvm/pom.xml -DskipTests package
dotnet build sdk/platform/dotnet/LayerX.Sdk.csproj --no-restore --configuration Release --nologo
swift build --disable-automatic-resolution --package-path sdk/platform/swift
sh sdk/platform/conformance/run-go.sh "$PWD"
sh sdk/platform/conformance/run-jvm.sh "$PWD"
swift test --disable-automatic-resolution --package-path sdk/platform/swift
```

The Go and JVM scripts check generated outputs before running their language
tests and conformance executables. See the
[conformance guide](conformance/README.md) for the shared vectors and the
[JVM guide](jvm/README.md) for its build and API examples.

## Configuration

Applications supply service origins, credentials, session information and
receipt trust anchors through the language transport and verifier APIs. A
receipt signature authenticates the receipted result; committed verification
also requires independently trusted consensus and inclusion evidence.

On platforms using FoundationNetworking, Swift's `AgentEnvelopeTransport` uses
`URLSessionCertificateAuthorityInfoFile` with the supplied certificate-authority
PEM file.

The [JVM examples](jvm/examples/README.md) read these environment variables:

- Service access: `LAYERX_HUMAN_ORIGIN`, `LAYERX_AGENT_ORIGIN`, `LAYERX_API_KEY`, `LAYERX_TENANT`, `LAYERX_SESSION_ID`, `LAYERX_SESSION_TOKEN_ID`, `LAYERX_SESSION_GENERATION`.
- Native payments: `LAYERX_NODE_RPC`, `LAYERX_EVM_KEY_FILE`, `LAYERX_RECIPIENT`, `LAYERX_AMOUNT`, `LAYERX_MAX_FEE_PER_GAS`, `LAYERX_PRIORITY_FEE_PER_GAS`, `LAYERX_RECEIPT_PINS`, `LAYERX_PAYMENT_JOURNAL`.
- Agent budgets: `LAYERX_AGENT_DID`, `LAYERX_ASSET`, `LAYERX_BUDGET_LIMIT`, `LAYERX_BUDGET_EXPIRY_SECONDS`.

The generators read [pipeline.kvx](pipeline.kvx),
[generators/receipt.kvx](generators/receipt.kvx),
[schema/mirror-v2.kvx](schema/mirror-v2.kvx),
[schema/paxai-v1.kvx](schema/paxai-v1.kvx) and
[conformance/operations.json](conformance/operations.json), together with the
[Agent](../../services/agent/schema/agent-api/) and
[Human](../../services/human/schema/human-api/) schemas.
Build configuration lives in each language manifest: [go.mod](go/go.mod),
[pom.xml](jvm/pom.xml), [LayerX.Sdk.csproj](dotnet/LayerX.Sdk.csproj),
[Package.swift](swift/Package.swift) and [Cargo.toml](generators/Cargo.toml).
The TypeScript runner uses [tsconfig.json](conformance-runner/tsconfig.json).

## Documentation

- [SDK quickstarts](../../docs/wiki/SdkQuickstarts.md): [Go](../../docs/wiki/SdkQuickstartGo.md), [JVM](../../docs/wiki/SdkQuickstartJvm.md), [.NET](../../docs/wiki/SdkQuickstartDotNet.md) and [Swift](../../docs/wiki/SdkQuickstartSwift.md).
- [Receipt verification](../../docs/wiki/SdkTerminalVerification.md).
- [Platform installation guide](../../docs/platform/content/install.md).
- Documentation site: [XDK developer toolkit](../../docs/site/paxeer-docs/public/markdown/xdk.md) and [SDK quickstarts](../../docs/site/paxeer-docs/public/markdown/sdk-quickstarts.md).
