# Solana

PAX against SOL. This page is the Solana leg of the Paxeer X Network bridge: an SPL deposit locked by the custody program in `contracts/bridge/solana` mints its bridged denom on Paxeer X Network, and a burn of that denom on Paxeer X Network is attested over the same outbound digest the EVM vaults release against. Solana is also Sidiora's foreign home. The operator runbook is [bridge runbook](../../../README.md); this page lists what Solana needs that the EVM chains do not.

| Field | Value |
| --- | --- |
| Chain name | `solana` |
| Chain id on the Paxeer side | `91600046870081`, the reserved id `0x0000534f4c414e41`: the ASCII bytes of `SOLANA` left-padded to eight bytes and read big-endian |
| Native coin | `SOL`, 9 decimals, bridged as the wrapped SOL mint |
| Finality depth | `32` slots |
| Commitment | `finalized` |
| Program id | `PLACEHOLDER:program-id` as committed; filled in after the first deployment |
| Attestors | 1 secp256k1 address at threshold 1 in the tracked configuration; attestation encoding is shared with the EVM chains |
| Configuration | `contracts/bridge/solana/chains/solana/config.json` |

## Assets

The configuration registers two assets, in this order. The wrapped SOL mint is first and always registered, and requires custody and Paxeer-side registration before deposits are used.

| Asset | Mint | Asset id | Decimals | Per-transaction cap | Total cap |
| --- | --- | --- | --- | --- | --- |
| SOL | `So11111111111111111111111111111111111111112` | `0xcf996523B5d068A26f0aa8a116602fE5033Ee3A1`, the mint's derived handle | 9 | `5000000000000` (5,000 SOL) | `100000000000000` (100,000 SOL) |
| SID | `5w3wVdJaESaJKyLmStM6Hv9UyUkmZ1b9DLQquAqqpump` | `0x21f7b20a555199fa73A238B1a91FD0f549068fEe`, the id the chain fixes | 6 | `1000000000000` (1,000,000 SID) | `50000000000000` (50,000,000 SID) |

The caps are in base units. A 32-byte Solana key enters the attestation digests as its handle, the last 20 bytes of `keccak256` of the key; `contracts/bridge/ATTESTATION-SOLANA.md` states the whole mapping with worked vectors.

## Sidiora

Sidiora exists on exactly two chains: Paxeer X Network, where SID is the coin at `0x21f7b20a555199fa73A238B1a91FD0f549068fEe` over the `usid` denom with six decimals, and Solana, where it is the SPL mint `5w3wVdJaESaJKyLmStM6Hv9UyUkmZ1b9DLQquAqqpump` with six decimals. Solana is Sidiora's foreign home.

The Solana mint is registered with the asset id `0x21f7b20a555199fa73A238B1a91FD0f549068fEe`, not with its derived handle, because the chain already fixes that pair: `EnsureSidioraDenom` in `modules/layerxbridge/keeper/sidiora.go` records the bridged asset (chain id, `0x21f7b20a555199fa73A238B1a91FD0f549068fEe`, `usid`), so an inbound SID deposit from Solana resolves to the `usid` denom the bridge module already administers. The binding holds both ways: that id is accepted only for that mint and that mint only with that id, and `services/bridge/deploy/deploy-solana-program.sh` and `services/bridge/deploy/chainconfig` refuse any other pairing.

The Paxeer side must hold the pair (`91600046870081`, `0x21f7b20a555199fa73A238B1a91FD0f549068fEe`) against `usid` before Sidiora's cap body is submitted. The runbook's section on the Sidiora pair says why and what to read back.

## Environment variables

The configuration names the first three; neither the configuration nor the script carries a value for any of them.

| Variable | Holds |
| --- | --- |
| `PAXEER_BRIDGE_SOLANA_RPC_URL` | the cluster endpoint, `http` or `https` |
| `PAXEER_BRIDGE_SOLANA_KEYPAIR_FILE` | the path of the publisher keypair file, which pays for the deployment and becomes the program's upgrade authority |
| `PAXEER_BRIDGE_SOLANA_TOOLCHAIN_BIN` | the directory of the pinned Solana toolchain holding `solana`, `solana-keygen` and `cargo-build-sbf` |
| `PAXEER_BRIDGE_DEPLOYMENT_RECORD` | where the deployment record is written |
| `PAXEER_BRIDGE_SOLANA_ADMIN_CLI` | the built `paxeer-x-bridge-solana-admin` binary, the program's admin client that encodes the initialise and register-asset instructions |
| `PAXEER_BRIDGE_SOLANA_PROGRAM_KEYPAIR_FILE` | optional; the program keypair, so a redeployment keeps its id. Required once `solana.program_id` is filled in |
| `PAXEER_BRIDGE_SOLANA_EXECUTABLE_WAIT_SECONDS` | optional; the bound, in seconds, on the wait for the deployed program to become executable, `120` when unset |
| `PAXEER_BRIDGE_SOLANA_CHAINS_ROOT` | optional; a chains root holding `solana/config.json`, in place of `contracts/bridge/solana/chains` |
| `BRIDGE_DEPLOY_OVERLAY` | optional; JSON overlay for owner, attestors, environment names, caps and program id |
| `PAXEER_BRIDGE_ATTESTOR_MANIFEST` | optional; approved signer manifest, defaulting to `services/bridge/deploy/attestors.json` |

## Before running the deploy script

- `owner` requires a base58 Solana public key. Supply it in private configuration or `BRIDGE_DEPLOY_OVERLAY`. Attestors are nonzero 20-byte secp256k1 addresses in strictly ascending order; use the signer set and threshold approved for the bridge.
- `solana.program_id` may stay `PLACEHOLDER:program-id` for the first deployment only. That run deploys, writes a record naming the program id and the vault authority, keeps the program keypair it generated at the record's path with `-program-keypair.json` in place of `.json`, and stops before the initialise step, naming the id. Write the id into the configuration; every later run must deploy with the keypair of that id through `PAXEER_BRIDGE_SOLANA_PROGRAM_KEYPAIR_FILE`. A first run refuses to overwrite a program keypair already at that path.
- `jq`, `python3`, `cast`, `sha256sum` and `flock` are on the `PATH`.
- The toolchain's `cargo-build-sbf` builds `contracts/bridge/solana` with platform tools `v1.56`, passed as `--tools-version` and fetched by `cargo-build-sbf` on first use, because the locked dependency graph, `contracts/bridge/solana/Cargo.lock`, includes crates whose manifests declare edition 2024 and the release the Solana toolchain installs by default refuses them.
- `PAXEER_BRIDGE_SOLANA_ADMIN_CLI` names the admin client, `paxeer-x-bridge-solana-admin` in `contracts/bridge/solana/admin`, built with `cargo build --locked --release --manifest-path contracts/bridge/solana/Cargo.toml -p paxeer-x-bridge-solana-admin`. The script calls it as `initialise` with `--config`, `--url`, `--keypair`, `--program-id`, `--commitment`, `--owner`, `--attestors` and `--threshold`, then once per asset as `register-asset` with `--config`, `--url`, `--keypair`, `--program-id`, `--commitment`, `--mint`, `--asset-id`, `--decimals`, `--per-tx-cap` and `--total-cap`; the client checks every value against the configuration and stops on a disagreement. The script refuses to run, `--preflight` included, until the variable names an executable.

```sh
bash services/bridge/deploy/deploy-solana-program.sh --preflight
bash services/bridge/deploy/deploy-solana-program.sh
```

The run confirms the endpoint answers a genesis hash, builds the program, deploys it, checks that the upgrade authority is the publisher, that the deployed ELF hashes to the built one, that the deployment is rooted and that the program becomes executable, initialises the program, registers both assets in configuration order, and writes a record naming the program id, the program data account, the ELF hash, the vault-authority address with its handle and the rooted slot.

The admin client also runs on its own against the configuration: `apply`, `initialise`, `register-asset`, `set-cap`, `pause`, `unpause`, `register-recipient` and `show`.

## The vault handle

The vault Paxeer X Network registers for Solana is the handle of the program's vault-authority PDA, derived from the single seed `vault-authority` - the seed `contracts/bridge/solana/src/state.rs` declares as `VAULT_SEED` and `contracts/bridge/ATTESTATION-SOLANA.md` specifies. `services/bridge/deploy/deploy-solana-program.sh` derives the address it records as `vault_authority` from that seed, and the `vault_handle` next to it is the last 20 bytes of `keccak256` of that 32-byte address, as `contracts/bridge/vectors/solana.go` computes it. The same address can be derived by hand:

```sh
program_id=$(jq -r .program_id "$PAXEER_BRIDGE_DEPLOYMENT_RECORD")
"$PAXEER_BRIDGE_SOLANA_TOOLCHAIN_BIN/solana" find-program-derived-address "$program_id" string:vault-authority
```

## The program at a glance

The custody program is a raw `solana-program` crate: no Anchor, fixed-width big-endian account and instruction layouts, spl-token moved by CPI, and custody held by the vault-authority PDA. Its instruction set is initialise, propose and accept ownership, set attestors, register asset, set cap, set pause, deposit, register recipient and release. A release pays an attested Paxeer burn out of the vault-authority's token account to the registered recipient, after verifying the attestors' signatures over the outbound digest, and creates a nullifier PDA seeded by the burn so the same burn is never paid twice. A deposit admits the amount against the asset's caps, increments the deposit nonce and writes a receipt account for it; the inbound `logIndex` of that deposit is the nonce, and its inbound `txHash` is `keccak256` of the 64-byte transaction signature.
