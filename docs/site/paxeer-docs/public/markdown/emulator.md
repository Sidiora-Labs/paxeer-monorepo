<!-- Source: https://docs.paxeer.app/emulator/ -->

# Local Protocol Emulator

Develop Paxeer X applications against real native protocol transitions, local signing identities and verifiable receipts.

## What the emulator provides

The emulator runs a local gateway around the native core transition engine. It provides deterministic development workflows for accounts, payments, canonical activities and Programs, with a sequencer identity that clients explicitly trust. Local prefunding supports repeatable scenarios without moving production value.

| Surface | Purpose |
| --- | --- |
| `GET /healthz` | Check local readiness. |
| `GET /v1/sequencer` | Read the advertised native network ID and sequencer public key. |
| `POST /__emulator/accounts/prefund` | Establish local development funding through emulator account tooling. |
| `GET /v1/state`, `GET /v1/protocol/account-state/head` | Inspect emulator state and the current account-state head. |
| `GET /v1/dids/{did}/sequence` | Read an identity sequence. |
| `POST /__emulator/time/set`, `POST /__emulator/time/advance` | Set or advance local protocol time for deterministic scenarios. |
| `POST /__emulator/faults` | Inject a fault for failure-path testing. |
| `GET` / `PUT /__emulator/snapshot` | Export or import an emulator snapshot. |
| `/v1/settle` | Retired: returns HTTP 410. |
| Native activity and receipt workflows | Exercise signing, state transitions and independently verifiable outcomes. |
| Programs lifecycle and calls | Develop deployment, discovery, simulation and execution journeys. |

## Provision a sequencer identity

```
layerx --json emulator provision
```

Provisioning generates a sequencer seed from operating-system randomness and publishes its public trust anchor. It creates `emulator/sequencer.seed` and `emulator/sequencer.anchor` under the CLI profile directory using owner-only storage and atomic identity publication. Retain the paths returned by the command. The seed is secret; the anchor is the public verification identity.

Existing files cause `sequencer_seed_exists` or `sequencer_trust_anchor_exists`. Use `layerx emulator provision --force` only when deliberately replacing the local identity, then rebind clients to the new anchor.

## Start the gateway

```
layerx emulator up   --listen 127.0.0.1:9402   --network-id 402   --sequencer-seed-file <provisioned-seed-path>
```

Run the gateway in a dedicated terminal. The listener must be loopback because emulator control routes provide local development authority. Network ID 402 is the local default, distinct from Paxeer X EVM chain ID 125. Network ID zero is reserved.

| Option | Use |
| --- | --- |
| `--sequencer-seed-file` | Required provisioned signing seed. |
| `--listen` | Choose an IP socket address on loopback. |
| `--network-id` | Bind activities and client handshake to a local native network. |
| `--protocol-version` | Select the supported protocol version for a compatibility scenario. |
| `--time-ms` | Choose the local protocol time in milliseconds for deterministic scenarios. |
| `--prefund` | Repeat a DID, public key and amount tuple for initial local account funding. |

## Bind the CLI to that identity

```
layerx environment use emulator   --endpoint http://127.0.0.1:9402   --network-id 402   --sequencer-trust-anchor-file <provisioned-anchor-path>
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
layerx key create alice
layerx key default alice
layerx account create --key alice --initial-amount 1000000
layerx key create bob
layerx account create --key bob --initial-amount 0
layerx account get --did <alice-did>
```

Key creation stores secret material in the selected credential store. Account creation in the emulator uses its prefunding route. Read public DIDs and account identifiers from the returned metadata instead of inventing bindings. The wallet convenience flow `layerx wallet create <name>` also registers a local identity and main account.

### Prefund at startup

```
layerx emulator up --sequencer-seed-file <seed-path>   --prefund '<did>,<64-hex-public-key>,1000000'
```

Prefunding binds the supplied DID and 32-byte public key to the local account setup. The amount accepts a decimal low component or an explicit high:low pair. Keep the signing key, public binding and DID consistent across setup and activity construction.

## Exercise payments and receipts

```
layerx --json payment test   --from <alice-account> --to <bob-account>   --currency <asset-id> --amount 1000   --idempotency-key emulator-payment-0001
layerx --json receipt get <0x-prefixed-transaction-hash>
```

Retain the quote, commit outcome, transaction hash and canonical receipt. `receipt get` fetches by 0x-prefixed transaction hash. Reuse the stable idempotency key for the same payment after an uncertain response. Independently verify receipt bytes with `layerx receipt verify`, supplying the batch ID, asset, previous and resulting state roots and trusted sequencer public key. A journey response alone does not replace receipt verification.

## Develop deterministic Programs

```
layerx new local-program
layerx program build --manifest-path local-program/Cargo.toml
layerx program discover <program-id>
layerx program interface get <program-id>
```

Use deploy and upgrade to exercise lifecycle authorization, simulate to obtain non-commit evidence, and call to obtain receipt-bound execution. Preserve current sequences, state roots, fuel limits, validity windows and interface digests. Simulation establishes `committed: false`; execution verifies the returned activity, ABI, sequencer identity and terminal payload before typed success.

## Move the application to a hosted profile

Keep emulator prefunding and local sequencer keys in the development environment. Bind a hosted profile with its own HTTPS endpoint, native network ID and trusted sequencer identity, then establish hosted identity and gateway credentials. A2A installation uses a hosted beta or production profile. MCP installation instead follows its enrolled daemon binding.

Continue with [the CLI reference](https://docs.paxeer.app/cli), [platform API](https://docs.paxeer.app/platform-api), [A2A agents](https://docs.paxeer.app/a2a) and [Programs](https://docs.paxeer.app/wasm-runtime).
