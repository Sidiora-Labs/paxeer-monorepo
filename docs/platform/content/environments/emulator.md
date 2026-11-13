# The emulator

The emulator is a local inspection environment around the LayerX core. It
provides account prefunding, clock controls and snapshot export and import.
Payment execution uses a merged-network node and signed EVM transactions.
The emulator does not participate in consensus or establish block commitment.

## Starting it

`layerx emulator provision` generates a sequencer seed and publishes its trust
anchor under the CLI profile directory. Provisioning uses owner-only storage
and is supported on Linux, Android and Apple hosts.

```sh
layerx emulator provision
layerx emulator up --sequencer-seed-file "$HOME/.config/layerx/emulator/sequencer.seed"
```

The seed path above assumes the default profile location. Use the path reported
by provisioning when `LAYERX_CONFIG` or `XDG_CONFIG_HOME` selects another
location.

Defaults are loopback port `9402`, network id `402`, protocol version `2` and
clock `1700000000000` milliseconds. The listener accepts only loopback addresses.
For example:

```sh
layerx emulator up \
  --sequencer-seed-file "$HOME/.config/layerx/emulator/sequencer.seed" \
  --listen 127.0.0.1:9402 \
  --network-id 402 \
  --protocol-version 2 \
  --time-ms 1700000000000 \
  --prefund 'did:layerx:sample,<64-hex-public-key>,1000000'
```

`--protocol-version` accepts `2` or `3`. Repeat `--prefund` to initialize more
accounts. Its format is `did,64-hex-public-key,amount`; amount is a 64-bit
unsigned integer or two unsigned 64-bit words in `high:low` form.

## Binding the local profile

```sh
layerx environment use emulator --endpoint http://127.0.0.1:9402 --network-id 402 \
  --sequencer-trust-anchor-file "$HOME/.config/layerx/emulator/sequencer.anchor"
```

The CLI compares the supplied network identifier and trust anchor with
`GET /v1/sequencer` before saving the profile. This binds the local identity;
it does not provide payment or finality evidence.

## Inspection routes

| Method and path | Purpose |
|---|---|
| `GET /healthz` | Reports `ready` when the core can be inspected |
| `GET /v1/surface` | Lists routes and supported operations |
| `GET /v1/sequencer` | Reports the configured network id and public key |
| `GET /v1/state` | Reports state roots, sequence, batch, clock, cells and accounts |
| `GET /v1/dids/{did}/sequence` | Reads an identity's sequence from local state |
| `GET /v1/journeys/{id}` | Reads journey records present in an imported snapshot |

## Control routes

| Method and path | Purpose |
|---|---|
| `POST /__emulator/accounts/prefund` | Initializes an account balance |
| `POST /__emulator/time/set` | Sets the core clock in milliseconds |
| `POST /__emulator/time/advance` | Advances the core clock by a delta |
| `POST /__emulator/faults` | Configures core fault injection |
| `GET /__emulator/snapshot` | Exports core state and recovery metadata |
| `PUT /__emulator/snapshot` | Imports a validated snapshot |

Fault kinds are `reject`, `drop_receipt` and `corrupt_receipt`. The optional
`count` defaults to `1` and accepts values from `1` to `1000000`. These controls
configure the local core; they do not inject faults into a network node.

Snapshots retain account metadata, stored receipts, program operation records,
move quotes and journeys alongside core state. Import validates the recovered
account identifiers against the restored core. State otherwise resides in the
process and is lost on restart.

## Payment evidence

Use a configured node's root JSON-RPC endpoint to submit signed transactions
through `eth_sendRawTransaction` and obtain signed execution receipts through
`pax_sequencerReceipt`. Verify receipts against independently pinned keys.
For finality, authenticate the block header and CometBFT commit and verify
transaction or state inclusion at the relevant height. See
[Committed block evidence](https://docs.paxeer.app/checkpoints/).
