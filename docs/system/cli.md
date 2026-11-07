# CLI commands

The developer CLI binary is `layerx`. Its command definitions and dispatch live in
[`tools/platform/cli/src/main.rs`](../../tools/platform/cli/src/main.rs), with wallet,
token, chain-registry and workspace subcommands in the same directory. Run
`layerx <command> --help` for flags; `--json` selects machine-readable output.
The catalogue below describes the current handlers, including their availability
limits. Some help strings still describe the pre-merge system.

## Network and transport

Paxeer mainnet-beta is one BFT network, running `paxd`, with EVM chain ID `125`
(`0x7d`). The kernel is part of the network and proposes blocks. Execution receipts
can precede consensus commitment; a receipt is final once its block is committed.
There is no separate LayerX interface or sequencer service in the merged network.

Public node JSON-RPC uses the root path on `https://api1.mainnet-beta.paxeer.network`
through `https://api14.mainnet-beta.paxeer.network` and
`https://api-mainnet-beta.paxeer.network`. The node namespaces are `debug`, `echo`,
`eth`, `net`, `pax`, `test` and `txpool`. Hosted `px_` methods belong to
[`services/platform/hosted/gateway/openrpc.json`](../../services/platform/hosted/gateway/openrpc.json);
they are not an additional node namespace.
`node.hyperpaxeer.com` is the hpx library mirror, not an RPC endpoint.

The router and its `/rpc`, `/rpc/ws` and `/rpc/schema` paths are retired.
The CLI's node transport (`rpc.rs`, `chains.rs`, `programs.rs`, `a2a.rs` and
the subscription transport) requires an HTTP(S) origin at the root path and
refuses any other path. `register` and `faucet` take an explicit hosted
service URL (`--gateway-endpoint` and `--service-endpoint`) and post to exactly
that URL. Local key operations, scaffolding, builds and offline receipt
verification do not require a live network endpoint.

For local network development, use `paxd` with an initialized local genesis, the
merged modules activated, and the node's configured EVM JSON-RPC listener. The
[`v6120-node.sh` local-node helper](../../tools/qualification/localnode/v6120-node.sh)
contains a concrete disposable-node setup, merge activation and precompile checks;
its `--sequencer` mode enables the node's integrated receipt/proposal path. It
writes the listener URL as `LOCAL_NODE_RPC` in its environment file. This is a
local chain setup; the legacy emulator commands below do not start that network.

## Wallets and tokens

Handlers: [`wallet.rs`](../../tools/platform/cli/src/wallet.rs),
[`wallet_derive.rs`](../../tools/platform/cli/src/wallet_derive.rs) and
[`wallet_signing.rs`](../../tools/platform/cli/src/wallet_signing.rs).

- `layerx wallet`: Manage native wallet keys, accounts, transfers, and receipts.
  - `layerx wallet create`: Create and prefund an identity/main account only in the legacy emulator profile. Other transports return `wallet_registration_unavailable` before generating a key.
  - `layerx wallet import`: Import a hexadecimal Ed25519 seed from standard input; this does not register an account.
  - `layerx wallet derive`: Derive an EVM account and native identity from one BIP-39 phrase read from standard input or a file. Private-key export requires `--export-private-keys`; `--bind` attempts an on-chain binding and is subject to the transport limitation above.
  - `layerx wallet list`: List local wallet public metadata.
  - `layerx wallet balance`: Read all accounts of the selected DID.
  - `layerx wallet send`: Submit a transfer with independent identity and source sequences.
  - `layerx wallet history`: Hidden and unavailable; returns `wallet_history_unavailable` because no authenticated DID activity-history backend is published.
  - `layerx wallet watch`: Read one live notification; use receipt verification to establish commitment.
  - `layerx wallet estimate-fee`: Estimate fees from unsigned EIP-1559 transaction bytes at pending.
  - `layerx wallet receipt`: Verify receipt evidence at the requested commitment.
  - `layerx wallet open-account`: Open the selected wallet's account for an asset.
- `layerx token`: Create and manage native tokens.
  - `layerx token create`: Register a natively issued token.
  - `layerx token mint`: Mint units to an existing asset account.
  - `layerx token burn`: Burn units from the selected wallet's asset account.
  - `layerx token transfer`: Transfer token units.
  - `layerx token info`: Read token metadata.
  - `layerx token list`: List one page of registered tokens in ascending asset order.

Network wallet and token operations depend on the configured transport and the
backend's published capabilities. Writes use an EVM key export, fee bounds and
independently pinned receipt policy. The retained wallet `--wait executed` option
maps to a signed `receipted` result; `--wait batched` maps to `committed`.
The transaction wait handler refuses `--wait finalised` with a missing-finality-trust
error. These option names do not introduce another network settlement stage, and
a live notification alone does not prove block commitment.

## Projects, profiles and credentials

- `layerx new`: Scaffold a deterministic Rust program project.
- `layerx workspace`: Inspect or operate on declared workspace modules. Subcommands are `modules`, `doctor`, `install`, `build`, `test` and `all`; build/install/test actions support module selection and `--dry-run`.
- `layerx environment`: Inspect or switch endpoint profiles named `emulator`, `beta` and `production` (`testnet` is a legacy alias for `beta`). These are CLI profile names, not separate network execution domains.
  - `layerx environment list`: List configured endpoint profiles.
  - `layerx environment current`: Show the active endpoint profile.
  - `layerx environment use`: Select a profile, configuring its endpoint when first used.
- `layerx key`: Manage Ed25519 keys in the selected credential store.
  - `layerx key create`: Generate a new Ed25519 seed from operating-system randomness.
  - `layerx key import`: Import a 32-byte hexadecimal Ed25519 seed from standard input.
  - `layerx key list`: List public key metadata without opening secret material.
  - `layerx key show`: Show public metadata for one key.
  - `layerx key default`: Select the default key used by account commands.
  - `layerx key delete`: Permanently delete a key from credential storage.
- `layerx auth`: Manage hosted API tokens in the selected credential store.
  - `layerx auth set`: Read an API token from standard input and save it securely.
  - `layerx auth status`: Report whether a token exists without printing it.
  - `layerx auth delete`: Permanently delete a stored API token.

[`config.rs`](../../tools/platform/cli/src/config.rs) stores public metadata and
profiles at `LAYERX_CONFIG`, or under `XDG_CONFIG_HOME` as `layerx/config.json`,
or at `~/.config/layerx/config.json`. The default profile is the legacy emulator at
`http://127.0.0.1:9402`, protocol network ID `402`; it is not mainnet chain ID `125`.
When configuring a profile with `environment use`, the current implementation
requires `--endpoint`, `--network-id` and either `--sequencer-trust-anchor` or
`--sequencer-trust-anchor-file` together. The emulator profile additionally checks
the advertised signing identity. Beta and production have no default endpoint.

## Chain registry

Sub-chains in the merged network are tenant domains: namespaces in the database
that launch without new infrastructure. The retained CLI registry commands in
[`chains.rs`](../../tools/platform/cli/src/chains.rs) read contract records or
submit a chain-creation transaction; they do not deploy a new node.

- `layerx chains list`: Enumerate chain-registry records.
- `layerx chains show`: Read a registry record by ID or name.
- `layerx chains create`: Submit an EIP-1559 registry transaction on chain ID `125`, using `--name`, `--tier`, `--fee-share-bps` and an EVM key export supplied through `--evm-key-file` or `LAYERX_EVM_KEY_FILE`.

These handlers share the transport migration gap above. Treat generated legacy
`rpcUrl` fields as client output, not as the public node endpoint contract.

## Hosted accounts and payments

- `layerx account`: Create or inspect a developer account.
  - `layerx account create`: Create a hosted account using email, display name and an idempotency key; a nonzero initial amount is refused. In the legacy emulator, it prefunds a local DID account instead.
  - `layerx account get`: Read the active hosted profile or one legacy emulator DID account.
- `layerx register`: Register a self-service identity principal for one local signing key.
- `layerx faucet`: Claim one beta faucet grant for the active identity and local key.
- `layerx payment`: Quote and commit a real test payment through the active endpoint.
  - `layerx payment test`: Request a quote and commit it with a stable idempotency key.

The handlers in [`account.rs`](../../tools/platform/cli/src/account.rs) and
[`payment.rs`](../../tools/platform/cli/src/payment.rs) use hosted REST routes.
`register` calls `px_register`, which creates an identity principal, not a funded
native wallet account; `faucet` calls `px_requestFunds`. These require the hosted
services and their authentication, subject to the transport limitation above.
A public node URL alone does not supply hosted accounts or payment quotes.

## Receipts and programs

- `layerx receipt`: Fetch receipt material or verify a receipt independently and locally.
  - `layerx receipt get`: Fetch the node's `pax_sequencerReceipt` result by its 0x-prefixed, 32-byte transaction hash, subject to the CLI transport limitation.
  - `layerx receipt verify`: Verify a canonical receipt file offline against independently supplied batch ID, asset, previous/resulting state roots and signing public key. This checks receipt evidence; it does not fetch consensus commitment.
- `layerx program`: Build, deploy, and inspect deterministic protocol programs.
  - `layerx program discover`: Discover one active program from the layerxprograms contract.
  - `layerx program interface`: Read or publish a canonical code-bound program interface.
    - `layerx program interface get`
    - `layerx program interface publish`
  - `layerx program build`: Build using a project's toolchain descriptor or release Cargo build for `wasm32-unknown-unknown`, then inspect the artifact. A descriptor's declared lint is run; the Cargo fallback reports that no determinism lint ran.
  - `layerx program bindings`: Generate typed SDK and guest bindings from a verified published interface.
  - `layerx program deploy`: Validate and submit a WASM artifact for receipt-backed deployment.
  - `layerx program upgrade`: Submit a native program upgrade with the expected old code hash and optional migration hook/interface.
  - `layerx program wind-down`
    - `layerx program wind-down route`
    - `layerx program wind-down deprecate`
    - `layerx program wind-down tombstone`
    - `layerx program wind-down exit`
  - `layerx program call`: Sign and submit an EIP-1559 call to a deployed program.
  - `layerx program simulate`: Execute a program call against current state without committing it.
  - `layerx program registry`: Read the protocol registry or mirror source material.
    - `layerx program registry list`: List programs from layerxprograms deployment events.
    - `layerx program registry get`: Read one program's layerxprograms contract record.
    - `layerx program registry mirror-source`: Mirror one program's build plan and source archive into the registry mirror.
    - `layerx program registry verify-source`: Returns source-verification unavailable after reading the deployed record, because `getProgram` exposes no source digest. Mirroring source does not establish source verification.

[`programs.rs`](../../tools/platform/cli/src/programs.rs) implements these program
operations through EVM calls, native lifecycle transactions and registry-mirror
requests. The receipt method name is retained in the merged node; it does not
denote an external executor. Network program operations still have the transport
limitation described above.

## Legacy emulator

These retained commands exercise the pre-merge core harness for local development.
They do not start `paxd`, participate in BFT consensus, or establish mainnet finality.
Their implementation is in [`emulator.rs`](../../tools/platform/cli/src/emulator.rs)
and [`tools/platform/emulator`](../../tools/platform/emulator/src/main.rs).

- `layerx emulator up`: Start the legacy standalone harness through `layerx_platform_emulator::run`, using the required `--sequencer-seed-file` and optional local listen/network/time/prefund settings.
- `layerx emulator provision`: Generate the legacy harness's Ed25519 signing seed and public trust anchor under the CLI profile directory (`emulator/sequencer.seed` and `emulator/sequencer.anchor`). Existing material is refused unless `--force` is supplied. This does not provision a network validator or proposer.

## Agent transports

MCP connects to an enrolled agent daemon using its binding document; that daemon
is an agent credential/tool service. The network executes programs in `paxd`.
See [`install/mcp.rs`](../../tools/platform/cli/src/install/mcp.rs) and
[`mcp.rs`](../../tools/platform/cli/src/mcp.rs) for binding validation and serving.

- `layerx install`: Install and register an agent transport in one command.
  - `layerx install mcp`: Install the model context protocol server bound to an enrolled agent daemon; requires a usable binding and authenticated daemon tool catalogue.
  - `layerx install a2a`: Install a payment-capable agent-to-agent server.
- `layerx mcp`: Serve the model context protocol transport on standard input and output.
  - `layerx mcp serve`: Serve the tool catalogue for one enrolled agent daemon binding.
- `layerx a2a`: Serve the agent-to-agent transport on a loopback endpoint.
  - `layerx a2a serve`: Serve the agent card and task interface for one environment and key.
  - `layerx a2a start`: Start the installed managed A2A runtime.
  - `layerx a2a stop`: Stop the installed managed A2A runtime.
  - `layerx a2a status`: Report the installed managed A2A runtime state.
