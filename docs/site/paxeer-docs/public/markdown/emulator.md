<!-- Source: https://docs.paxeer.app/emulator/ -->

# Local Protocol Emulator

Inspect local protocol state with explicit signing identities, account prefunding, time controls and snapshots.

## What the emulator provides

The emulator wraps the native core for local inspection and account setup. Its surface document declares the available routes and reports `payments.supported: false`. Local prefunding, time controls and snapshots support repeatable inspection scenarios. Payment and program execution use the merged node.

| Surface | Purpose |
| --- | --- |
| `GET /healthz` | Check local readiness. |
| `GET /v1/sequencer` | Read the advertised native network ID and sequencer public key. |
| `POST /__emulator/accounts/prefund` | Establish local development funding through emulator account tooling. |
| `GET /v1/state`, `GET /v1/protocol/account-state/head` | Inspect local state; the account-state head route returns HTTP 503 with `principal_state_proof_unavailable`. |
| `GET /v1/dids/{did}/sequence` | Read an identity sequence. |
| `POST /__emulator/time/set`, `POST /__emulator/time/advance` | Set or advance local protocol time for deterministic scenarios. |
| `POST /__emulator/faults` | Inject a fault for failure-path testing. |
| `GET` / `PUT /__emulator/snapshot` | Export or import an emulator snapshot. |
| `GET /v1/surface` | Read the route list and declared execution capabilities. |
| Payment execution | Use merged-node `eth_sendRawTransaction` and `pax_sequencerReceipt`. |
| Local receipt reads | Existing receipt entries return HTTP 410; an unknown receipt returns HTTP 404. |

## Provision a sequencer identity

```
layerx --json emulator provision
```

Provisioning generates a sequencer seed from operating-system randomness and publishes its public trust anchor. It creates `emulator/sequencer.seed` and `emulator/sequencer.anchor` under the CLI profile directory using owner-only storage and atomic identity publication. Retain the paths returned by the command. The seed is secret; the anchor is the public verification identity.

Existing files cause `sequencer_seed_exists` or `sequencer_trust_anchor_exists`. Use `layerx emulator provision --force` only when deliberately replacing the local identity, then rebind clients to the new anchor.

## Start the gateway

```
layerx emulator up   --listen <loopback-socket>   --network-id 402   --sequencer-seed-file <provisioned-seed-path>
```

Run the gateway in a dedicated terminal. Set the listener to a loopback socket; its default port is 9402. Emulator control routes provide local development authority. Network ID 402 is the local default, separate from the merged network's EVM chain ID 125. Network ID zero is reserved.

| Option | Use |
| --- | --- |
| `--sequencer-seed-file` | Required provisioned signing seed. |
| `--listen` | Choose an IP socket address on loopback. |
| `--network-id` | Bind activities and client handshake to a local native network. |
| `--protocol-version` | Select protocol version 2 or 3; the default is 2. |
| `--time-ms` | Choose the local protocol time in milliseconds for deterministic scenarios. |
| `--prefund` | Repeat a DID, public key and amount tuple for initial local account funding. |

## Bind the CLI to that identity

```
layerx environment use emulator   --endpoint <emulator-url>   --network-id 402   --sequencer-trust-anchor-file <provisioned-anchor-path>
layerx --json environment current
```

Supply endpoint, network ID and one trust-anchor input together. The CLI waits for the listener and reads `/v1/sequencer`, then checks the network and public identity before accepting the profile. A profile can also use `--sequencer-trust-anchor <hex>` in place of the file. Subsequent selection uses `layerx environment use emulator`.

### Diagnose handshake failures

| Code | Resolution |
| --- | --- |
| `environment_input_missing` | Provide the complete endpoint, network and anchor set. |
| `sequencer_identity_unavailable` | Start the listener and confirm the endpoint and identity route. |
| `network_id_mismatch` | Match client configuration to the running emulator network. |
| `sequencer_trust_anchor_mismatch` | Use the public anchor corresponding to the running sequencer seed. |
| `sequencer_trust_anchor_unreadable`, `sequencer_trust_anchor_malformed` | Correct the anchor path or public-key encoding. |

## Create development accounts

```
layerx key create payer
layerx key default payer
layerx account create --key payer --initial-amount 1000000
layerx key create payee
layerx account create --key payee --initial-amount 0
layerx account get --did <payer-did>
```

Key creation stores secret material in the selected credential store. Account creation in the emulator uses its prefunding route. Read public DIDs and account identifiers from the returned metadata instead of inventing bindings. This account setup is local to the emulator; it does not fund an EVM address on the merged network.

### Prefund at startup

```
layerx emulator up --sequencer-seed-file <seed-path>   --prefund '<did>,<64-hex-public-key>,1000000'
```

Prefunding binds the supplied DID and 32-byte public key to the local account setup. The amount accepts a decimal low component or an explicit high:low pair. Keep the signing key, public binding and DID consistent across setup and activity construction.

## Read merged-network receipts

```
layerx --json --rpc <node-url> receipt get <0x-prefixed-transaction-hash>
```

Point `--rpc` at the merged node's root JSON-RPC origin. `receipt get` calls `pax_sequencerReceipt` with the transaction hash. Preserve the response and independently verify its receipt authority and block-commit evidence. A receipt is final once its block is committed.

## Build programs and query the merged node

```
layerx new local-program
layerx program build --manifest-path local-program/Cargo.toml
layerx --rpc <node-url> program discover <program-id>
layerx --rpc <node-url> program interface get <program-id>
```

Scaffolding and building create local artifacts. Discovery and interface reads query the program precompile through the selected node. Program lifecycle transactions, simulation and calls use the merged network rather than the emulator's inspection routes. Preserve code hashes, ABI versions, interface digests and execution receipts.

## Move the application to a hosted profile

Keep emulator prefunding and local signing keys in the development environment. Configure the node origin for merged-network commands and a hosted profile for authenticated hosted APIs. A2A installation uses a hosted beta or production profile; MCP installation follows its enrolled daemon binding.

Continue with [the CLI reference](https://docs.paxeer.app/cli), [platform API](https://docs.paxeer.app/platform-api), [A2A agents](https://docs.paxeer.app/a2a) and [Programs](https://docs.paxeer.app/wasm-runtime).
