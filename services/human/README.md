# Human interface

`services/human/` contains the Rust services for person-facing authentication,
onboarding, custody, money movement, approvals and receipt-backed activity.
Paxeer mainnet-beta is one BFT network, with EVM chain id **125** (`0x7d`).
The kernel is part of that network and proposes blocks. A receipt is final when
its block is committed. Tenant domains are database namespaces, so a sub-chain
does not require a new chain deployment.

## Layout

| Path | Contents |
| --- | --- |
| [`crates`](crates) | Rust workspace members listed below |
| [`schema`](schema) | Human API schema and contract documentation |
| [`tests`](tests) | Workspace documentation and transaction-boundary integration cases |
| [`tools`](tools) | API generation, boundary, copy, schema, UI and dependency checks |
| [`Cargo.toml`](Cargo.toml) | Workspace members, shared dependencies and Rust lint settings |
| [`Cargo.lock`](Cargo.lock) | Locked Rust dependency versions |
| [`clippy.toml`](clippy.toml) | Clippy configuration |
| [`deny.toml`](deny.toml) | Dependency advisory, license, package and source policies |
| [`README.md`](README.md) | Workspace overview and development commands |

The members in [`Cargo.toml`](Cargo.toml) are:

| Path | Contents |
| --- | --- |
| [`crates/layerx-human-service`](crates/layerx-human-service/README.md) | HTTP service and privileged components for authentication, onboarding, journeys, approvals, custody and activity |
| [`crates/layerx-intents`](crates/layerx-intents) | Typed intent compilation, disclosure checks and canonical protocol codecs |
| [`crates/layerx-human-identity-provider`](crates/layerx-human-identity-provider/README.md) | LXIP identity directory and optional tenant-bound wallet-binding reader |
| [`crates/layerx-human-security-provider`](crates/layerx-human-security-provider/README.md) | LXSP authenticator management and verified recovery-receipt disclosure |
| [`crates/layerx-human-kms`](crates/layerx-human-kms/README.md) | LXKP mutual-TLS custody provider and threshold attestor client |
| [`crates/layerx-network-gateway`](crates/layerx-network-gateway) | Typed client for hosted gateway account, balance, asset and network reads |
| [`crates/layerx-human-test-support`](crates/layerx-human-test-support/README.md) | Development helpers for stores, evidence and durable transaction admission |

Related applications and contracts:

| Path | Contents |
| --- | --- |
| [`schema/human-api`](schema/human-api/README.md) | Versioned HTTP/JSON contract consumed by the web application |
| [`apps/web`](../../apps/web) | Human web application and [`@layerx/ui`](../../apps/web/packages/layerx-ui/README.md) components |
| [`apps/wallet`](../../apps/wallet/README.md) | Wallet web application |
| [`services/wallet`](../wallet/README.md) | Wallet gateway, SDK workspace, demo, attestor, ceremony and deployment files |
| [`tools`](tools) | API generation, boundary, copy, schema, UI and dependency checks |

### Transaction and evidence boundaries

The kernel executes inside the chain-125 node; Human does not connect to a
separately running LayerX network or deploy a chain for each tenant.

`layerx-intents` remains the authority for typed protocol intent compilation and
disclosure checks. The [agent runtime adapter](crates/layerx-human-service/src/server/agent_runtime.rs)
checks the prepared precompile target, calldata, nonce and fee ceiling against
the authorized intent before signing. This intent authority is separate from
transaction encoding: shared `layerx-crypto` EIP-1559 codecs and the
[KMS attestor path](crates/layerx-human-kms/src/attestor.rs) encode unsigned calls,
verify signatures and assemble signed type-2 transactions.

[Bank sends](crates/layerx-human-service/src/custody/evm.rs) are zero-value calls
to `0x0000000000000000000000000000000000001001`, using
`send(address,address,string,uint256)`. The disclosure binds sender, recipient,
denomination and amount. Custody validates the authenticated principal, tenant,
wallet and exact plan; KMS reserves the nonce and journals the signed transaction
for recovery. EVM wallets resolve to configured attestor key identities and
public addresses. Their secp256k1 private material is not returned to the Human
service. See the [KMS guide](crates/layerx-human-kms/README.md) for protected
attestor bindings and owner assertions.

The [onboarding sponsor](crates/layerx-human-service/src/server/onboarding_sponsor.rs)
reads a chain-125 registry and requests gas-station sponsorship for
`bindLayerX(bytes32,bytes)` at `0x0000000000000000000000000000000000001004`.
A `submitted` reply remains incomplete. A `receipted` or `committed` reply must
have status `1` and a matching, non-removed `LayerXBound` log for the account and
DID public key before sponsorship reports completion. Completion at `receipted`
does not itself establish block commitment. Configuration is in the
[service guide](crates/layerx-human-service/README.md).

Submission acknowledgement, a verified execution receipt and committed block
evidence are distinct observations. A submitted transaction hash alone does not
prove execution or commitment. [Journey commitment verification](crates/layerx-human-service/src/journeys/bank_commit.rs)
authenticates the governed receipt key and signed execution receipt, then checks
the signed sequence batch, validator quorum and transaction inclusion against
the committed block. An ordinary EVM landing receipt or an RPC rung label alone
cannot establish that proof. Hosted gateway reads are explicitly labeled
`gateway-reported`, not promoted to cryptographically verified evidence.

Node JSON-RPC uses the root path of the configured node origin. The node
[RPC registration](../../rpc/server.go) includes `px` account, history, inference
and kernel methods. Hosted identity registration and faucet requests use
`px_register` and `px_requestFunds` through
[the gateway OpenRPC contract](../platform/hosted/gateway/openrpc.json).
Select the endpoint according to the required method contract.

## Build and test

Run these development targets from the repository root; they are declared in
the [root Makefile](../../Makefile):

```sh
make human-js-install   # agent TypeScript SDK and Human web dependencies
make human-build        # SDK, API checks, Rust workspace and web build
make human-test         # provisioning, Rust workspace and web tests
make human-check        # Rust, boundary, schema and web type checks
make human-lint         # copy, Rust, dependency and web lint checks
make human-check-ui     # UI checks, component build and web checks
```

`human-test` also builds native custody prerequisites and webhooks; its hosted
provisioning tests require the Python dependencies checked by that target.
The workspace test target uses the authenticated runtime clock. The
[clock wrapper](../../tools/runtime/run-with-clock.sh) can also run a focused
suite, for example:

```sh
sh tools/runtime/run-with-clock.sh cargo test --locked \
  --manifest-path services/human/Cargo.toml \
  -p layerx-human-identity-provider --test probe
```

The boundary checker used by `human-check`, `human-lint` and
`human-check-bundle` checks the workspace and web bundle boundaries.

## Configuration

The processes read environment settings and files selected by those settings.
The [HTTP entry point](crates/layerx-human-service/src/main.rs) and
[production composition](crates/layerx-human-service/src/server/production_components.rs)
define the service settings:

| Variables | Purpose |
| --- | --- |
| `LAYERX_HUMAN_BIND`, `LAYERX_HUMAN_LISTENER` | HTTP listener binding and transport selection |
| `LAYERX_HUMAN_TLS_CERT_DER`, `LAYERX_HUMAN_TLS_KEY_DER` | Listener certificate and private-key files |
| `LAYERX_HUMAN_MAX_CONNECTIONS`, `LAYERX_HUMAN_IO_DEADLINE_SECONDS`, `LAYERX_HUMAN_MAX_HEADER_BYTES`, `LAYERX_HUMAN_MAX_BODY_BYTES` | Listener resource limits |
| `LAYERX_HUMAN_REQUESTS_PER_MINUTE`, `LAYERX_HUMAN_MAX_PRINCIPALS` | Per-principal request limits |
| `LAYERX_HUMAN_COMPONENT_SOCKET` | Privileged component socket |
| `LAYERX_HUMAN_STORE_ROOT`, `LAYERX_HUMAN_CUSTODY_ROOT`, `LAYERX_HUMAN_AUTH_INDEX_ROOT` | Durable state directories |
| `LAYERX_HUMAN_TENANCY_DIGEST`, `LAYERX_HUMAN_AUTH_INDEX_KEY`, `LAYERX_HUMAN_STREAM_CURSOR_KEY` | Tenant mapping and authentication bindings |
| `LAYERX_HUMAN_RP_ID`, `LAYERX_HUMAN_RP_NAME`, `LAYERX_HUMAN_WEB_ORIGIN`, `LAYERX_HUMAN_ORIGIN` | Passkey and browser-origin policy |
| `LAYERX_HUMAN_CEREMONY_TTL_SECONDS`, `LAYERX_HUMAN_ASSERTION_TTL_SECONDS`, `LAYERX_HUMAN_SESSION_TTL_SECONDS`, `LAYERX_HUMAN_REFRESH_TTL_SECONDS`, `LAYERX_HUMAN_STEP_UP_TTL_SECONDS` | Authentication lifetimes |
| `LAYERX_HUMAN_AUTH_RATE_ATTEMPTS`, `LAYERX_HUMAN_AUTH_RATE_WINDOW_SECONDS` | Authentication rate policy |
| `LAYERX_HUMAN_AGENT_SOCKET`, `LAYERX_HUMAN_IDENTITY_SOCKET`, `LAYERX_HUMAN_SECURITY_SOCKET` | Agent, identity and security provider connections |
| `LAYERX_HUMAN_IDENTITY_PEER_UID`, `LAYERX_HUMAN_IDENTITY_PEER_GID`, `LAYERX_HUMAN_IDENTITY_MAX_FRAME_BYTES`, `LAYERX_HUMAN_IDENTITY_DEADLINE_SECONDS` | Identity peer authentication and transport limits |
| `LAYERX_HUMAN_SECURITY_MAX_FRAME_BYTES`, `LAYERX_HUMAN_SECURITY_DEADLINE_SECONDS` | Security provider transport limits |
| `LAYERX_HUMAN_NETWORK_ID`, `LAYERX_HUMAN_PROTOCOL_VERSION` | Native transaction network and protocol bindings |
| `LAYERX_HUMAN_EVM_GAS_LIMIT`, `LAYERX_HUMAN_EVM_MAX_FEE_PER_GAS`, `LAYERX_HUMAN_EVM_MAX_PRIORITY_FEE_PER_GAS` | Authorized EVM gas and fee limits |
| `LAYERX_AGENT_NODE_RPC_URL`, `LAYERX_AGENT_NODE_CHAIN_ID` | Node transaction and receipt transport |
| `LAYERX_HUMAN_AGENT_PURPOSE_CATALOG` | Agent purpose catalog file |
| `LAYERX_HUMAN_ONBOARDING_REGISTRY_FILE`, `LAYERX_HUMAN_GAS_STATION_ENDPOINT` | Sponsor registry JSON file and sponsorship endpoint |

The [service guide](crates/layerx-human-service/README.md) describes the
component, retention, signing and agent policies. Remote KMS clients read
`LAYERX_HUMAN_KMS_PROVIDER_REFERENCE`, `LAYERX_HUMAN_KMS_ENDPOINT`,
`LAYERX_HUMAN_KMS_SERVER_NAME`, `LAYERX_HUMAN_KMS_ROOT_CERTIFICATE_DER`,
`LAYERX_HUMAN_KMS_CLIENT_CERTIFICATE_DER`,
`LAYERX_HUMAN_KMS_CLIENT_PRIVATE_KEY_DER`, `LAYERX_HUMAN_KMS_MAX_FRAME_BYTES`,
`LAYERX_HUMAN_KMS_MAX_CONNECTIONS`, `LAYERX_HUMAN_KMS_MAX_STREAMS`,
`LAYERX_HUMAN_KMS_MAX_QUEUED_BYTES` and `LAYERX_HUMAN_KMS_DEADLINE_SECONDS`.
The certificate and private-key settings name DER files.

The component attestor backend reads `LAYERX_HUMAN_ATTESTOR_NODES`,
`LAYERX_HUMAN_ATTESTOR_SIGNERS`, `LAYERX_HUMAN_ATTESTOR_ROOT_CERTIFICATE_DER`,
`LAYERX_HUMAN_ATTESTOR_CLIENT_CERTIFICATE_DER`,
`LAYERX_HUMAN_ATTESTOR_CLIENT_PRIVATE_KEY_DER` and
`LAYERX_HUMAN_ATTESTOR_DEADLINE_SECONDS`.

The providers have their own configuration:

| Provider | Variables and file inputs |
| --- | --- |
| [Identity](crates/layerx-human-identity-provider/README.md) | `LAYERX_HUMAN_IDENTITY_PROVIDER_STATE_ROOT`, `LAYERX_HUMAN_IDENTITY_PROVIDER_SOCKET`, `LAYERX_HUMAN_IDENTITY_PROVIDER_ALLOWED_UID`, `LAYERX_HUMAN_IDENTITY_PROVIDER_DEADLINE_SECONDS`; `LAYERX_HUMAN_IDENTITY_PROVIDER_RECOVERY_POLICY_FILE` selects the recovery policy JSON file |
| [Security](crates/layerx-human-security-provider/README.md) | `LAYERX_HUMAN_SECURITY_PROVIDER_STATE_ROOT`, `LAYERX_HUMAN_SECURITY_PROVIDER_SOCKET`, `LAYERX_HUMAN_SECURITY_PROVIDER_ALLOWED_UID`, `LAYERX_HUMAN_SECURITY_PROVIDER_DEADLINE_SECONDS`; `LAYERX_HUMAN_SECURITY_PROVIDER_TRUST_HISTORY` selects the binary chain trust-root file |
| [KMS](crates/layerx-human-kms/README.md) | `LAYERX_HUMAN_KMS_LISTEN`, `LAYERX_HUMAN_KMS_STATE_DIR`, `LAYERX_HUMAN_KMS_PROVIDER_REFERENCE`, `LAYERX_HUMAN_KMS_DEADLINE_SECONDS`; file inputs are listed below |

The identity provider also reads `LAYERX_HUMAN_IDENTITY_PROVIDER_ASSERTION_JWKS_URL`,
`LAYERX_HUMAN_IDENTITY_PROVIDER_ASSERTION_ISSUER`,
`LAYERX_HUMAN_IDENTITY_PROVIDER_ASSERTION_AUDIENCE`,
`LAYERX_HUMAN_IDENTITY_PROVIDER_ASSERTION_CLOCK_SKEW_SECONDS`,
`LAYERX_HUMAN_IDENTITY_PROVIDER_ASSERTION_REFRESH_INTERVAL_SECONDS` and
`LAYERX_HUMAN_IDENTITY_PROVIDER_ASSERTION_BINDING_PRODUCER_KEY` for identity
assertions. Its optional tenant-bound reader uses
`LAYERX_HUMAN_IDENTITY_PROVIDER_BINDING_SOCKET`,
`LAYERX_HUMAN_IDENTITY_PROVIDER_BINDING_TENANT` and
`LAYERX_HUMAN_IDENTITY_PROVIDER_BINDING_ALLOWED_UIDS`.

The [KMS configuration loader](crates/layerx-human-kms/src/config.rs) reads
`LAYERX_HUMAN_KMS_REGISTRY_FILE` for the chain registry JSON,
`LAYERX_HUMAN_KMS_CLIENT_CA_DER`, `LAYERX_HUMAN_KMS_CLIENT_CERT_DER`,
`LAYERX_HUMAN_KMS_TLS_CERT_DER`, `LAYERX_HUMAN_KMS_TLS_KEY_DER` and
`LAYERX_HUMAN_KMS_EVM_CLIENT_CERT_DER` for TLS material, and
`LAYERX_HUMAN_KMS_SEAL_SECRET_FILE` for the store seal file.
`LAYERX_HUMAN_KMS_ATTESTOR_CONFIG_FILE` selects the principal-bound attestor
JSON configuration; its bindings name owner assertion files, trust anchors,
client certificate chains and a client private-key file. The KMS guide describes
these file formats and ownership requirements.

Production listeners require the authenticated clock bindings
`LAYERX_RUNTIME_CLOCK_SOCKET`, `LAYERX_RUNTIME_CLOCK_PID` and
`LAYERX_RUNTIME_CLOCK_UID`. The development wrapper uses
`LAYERX_RUNTIME_CLOCK_BIN` when provided, otherwise builds the clock supervisor;
`LAYERX_RUNTIME_CLOCK_DIRECTORY` selects its runtime directory. Attaching to an
existing clock requires all three bindings. Provider listener requirements are
in the provider guides above.

## Documentation

- [Human API reference](../../docs/platform/content/reference/human-api.md)
- [Human interface site page](../../docs/site/paxeer-docs/src/app/human-interface/page.tsx)
  and [Markdown page](../../docs/site/paxeer-docs/public/markdown/human-interface.md)
- [Identity](../../docs/site/paxeer-docs/public/markdown/identity.md)
  and [security](../../docs/site/paxeer-docs/public/markdown/security.md)
- [Human HTTP/JSON schema](schema/human-api/README.md)
- [Human service](crates/layerx-human-service/README.md),
  [identity provider](crates/layerx-human-identity-provider/README.md),
  [security provider](crates/layerx-human-security-provider/README.md),
  [KMS provider](crates/layerx-human-kms/README.md) and
  [test support](crates/layerx-human-test-support/README.md)
