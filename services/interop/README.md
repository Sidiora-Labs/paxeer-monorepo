# Paxeer X Network interoperability workspace

Adapters at the edge of Paxeer X Network translate external protocols into
evidence consumed by the kernel inside `paxd`. Native ledger and payment
operations execute on the merged chain (EVM chain ID `125`); transaction
finality is BFT block commit. Interop services provide the mirror archive,
attestation bindings and sequencer receipts described below.

This is the Rust workspace in `services/interop/` (members listed in [`Cargo.toml`](Cargo.toml)). It is not a second ledger.

## Where to start

| Surface | What it is | Where it lives |
| --- | --- | --- |
| Gateway | Transport-neutral routes, redaction, adapter host | `crates/layerx-interop-gateway`, executable composition in `crates/layerx-interop-service` |
| MCP / A2A transports | Ingress labels `mcp` and `a2a` next to `http` | Gateway `IngressTransport`; x402 `TransportKind` |
| x402 v2 | Buyer, seller, facilitator over HTTP, MCP, and A2A | `crates/layerx-x402` - [COMPATIBILITY.md](crates/layerx-x402/COMPATIBILITY.md). Commitment extras and metered/subscription grant draws: [`docs/wiki/X402Transport.md`](../../docs/wiki/X402Transport.md) |
| Ethereum / Solana mirrors | Batch archive publication and verification. Pure archives: no vault, no portal, no custody | `crates/layerx-mirror`; contracts in `contracts/ethereum-mirror/` and `contracts/solana-mirror/`; deploy notes in `deploy/mirror/` |
| Ethereum / Solana migration | Source-chain verifiers and the `migration` adapter | `crates/layerx-migrate` - [OPERATIONS.md](crates/layerx-migrate/OPERATIONS.md) |
| Portable receipts | Verify signed `ProtocolReceipt` v2 bytes under independently pinned, height-ranged receipt keys without a live node | `crates/layerx-portable` - [PORTABILITY.md](../../docs/interop/PORTABILITY.md) |
| Bridge relayer | Carries `PaxeerXVault` deposits into the bridge precompile and Paxeer X burns into vault releases, plus the `layerx-bridge-cosign` share transport | `crates/layerx-bridge-relayer`; deploy config in `deploy/bridge-relayer/`; operator runbook in [`contracts/bridge/README.md`](../../contracts/bridge/README.md) |
| Remote signer | Reference signer for the mirror publisher and the bridge relayer | `crates/layerx-mirror-signer`; protocol in [`deploy/mirror/signer-protocol.md`](deploy/mirror/signer-protocol.md) |
| Gas station | Sponsored submission (`paxeer-gas-station` binary) | `crates/layerx-gas-station`; deploy config in `deploy/gas-station/` |
| Web search sidecar | `x-websearch`: paid `/search` and `/fetch`, free `/health` | `crates/x-websearch`; deploy files in `deploy/x-websearch/` and `deploy/search-front/` |
| Other adapters | AP2, UCP, Visa TAP, fiat rails | `crates/layerx-ap2`, `crates/layerx-ucp`, `crates/layerx-visa-tap`, `crates/layerx-fiat` |

There is no standalone `layerx-a2a` crate. A2A is a transport on the gateway and on x402, plus the CLI installer in `tools/platform/cli/`.

## Two MCP surfaces

| Surface | Role |
| --- | --- |
| `services/agent/crates/layerx-mcp` | Tenant- and scope-bound MCP server that routes every call through `layerx-agentd` |
| This workspace + `tools/platform/cli/` | MCP as an interop ingress; `layerx install mcp` / `layerx mcp serve` from the developer CLI |

MCP tool inventory: [`services/agent/crates/layerx-mcp/README.md`](../../services/agent/crates/layerx-mcp/README.md). CLI install copy: [`docs/platform/content/install.md`](../../docs/platform/content/install.md) and [`docs/platform/content/guide/interop.md`](../../docs/platform/content/guide/interop.md).

Interop service deployment inputs, including the required protocol network,
authoritative module registry, server-owned TAP clock skew, explicit trusted
agent status, and per-principal canonical TAP targets, are documented in
[`deploy/gateway/`](deploy/gateway/README.md).

## Mirrors are archives

`layerx-mirror-publisher` and `layerx-mirror-verify` publish and check batch
commitments on Ethereum and Solana. The mirror archive carries verification
material; its contracts hold no vault or custody and do not establish Paxeer
finality. Bridge vaults are separate contracts with their own attestor policy.
Settlement on Paxeer is the committed chain state, whose node lives in the
chain directories at the [Paxeer monorepo root](../../README.md).

Remote signer framing: [`deploy/mirror/signer-protocol.md`](deploy/mirror/signer-protocol.md).

## Attestation bindings and sequencer receipts

The bridge relayer binds deposit attestations to the source chain ID, vault,
transaction hash and log index, recipient, asset and amount. Release bundles
bind the destination chain and vault, Paxeer transaction hash and nonce,
recipient, asset and amount. Signatures cover the corresponding digest and
must meet the destination's distinct-attestor threshold. The read-only
`GET /v1/deposit/{chainId}/{txHash}/{logIndex}` and
`GET /v1/release/{chainId}/{paxeerTxHash}/{nonce}` routes expose observed,
attesting and ready bundles; an unobserved item answers 404. See the
[interop contract](../../spec/layerx-merge/contracts/interop.md#bridge-relayer).

External evidence import preserves protocol presentation bytes and binds the
adapter's typed verification result to their digest and protocol metadata.
Consumers must authenticate the verifier or repeat verification; a digest alone
does not authenticate an attestation. See
[external evidence import](../../docs/interop/PORTABILITY.md#external-evidence-import).

Read `pax_sequencerReceipt` at the node root for the `submitted`, `receipted`
and `committed` rungs. Verify canonical signed `ProtocolReceipt` v2 bytes
against independently pinned receipt keys and their inclusive height windows,
and bind the transaction hash, result and logs to the intended operation.
`submitted` carries no signed execution receipt; `receipted` authenticates
execution but does not prove BFT commit. Finality requires authenticated
block inclusion and validator commit evidence at the `committed` rung.
Portable export uses `layerx-receipt-proof-v2` and retains verification level
`receipted`, even when exported from a committed answer; it cannot establish
block finality on its own. See the
[portable verification guide](../../docs/interop/PORTABILITY.md).

## Migrations (three different things)

| Kind | Location |
| --- | --- |
| ETH / Solana source migration into LayerX | this workspace, `crates/layerx-migrate`; wiki page [`docs/wiki/Migration.md`](../../docs/wiki/Migration.md) |
| LayerX kernel genesis import sections and projections | [`migrations/`](../../migrations/README.md) |
| Paxeer X chain EVM store migrations | `modules/evm/migrations/` (chain-internal; not a LayerX surface) |

## Build and test

From the monorepo root:

```sh
make interop-build
make interop-test
make interop-lint
make interop-test-x402
make interop-test-migration
```

Mirror publisher / verifier binaries:

```sh
cargo build --locked --release --manifest-path services/interop/Cargo.toml --package layerx-mirror --bin layerx-mirror-publisher
cargo build --locked --release --manifest-path services/interop/Cargo.toml --package layerx-mirror --bin layerx-mirror-verify
```

`make mirror-live` and `make mirror-verify-live` build those binaries and run them against live networks through `scripts/qualify-mirror-live.sh` and `scripts/qualify-mirror-verification-live.sh`; they require operator configuration. Example configurations are `deploy/mirror/config.example.json` and `deploy/mirror/verify-config.example.json`.
