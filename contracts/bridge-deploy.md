# Bridge deployment contract

The bridge moves assets between foreign custody contracts or a Solana custody
program and the native bridge module of the single Paxeer network. Paxeer's
EVM chain ID is **125 (`0x7d`)**; its committed blocks determine receipt
finality. The kernel is part of that network and proposes blocks. The
foreign-chain `finality_depth` setting controls deposit confirmation policy;
it does not create another Paxeer finality layer.

This page describes the tools in [services/bridge/deploy](../services/bridge/deploy/).
It does not record live vault addresses. Use the
[address inventory](address-inventory.md),
[attestation vectors](bridge-vectors.md), and [node RPC](evm-rpc.md) alongside
the deployment records.

## Chainconfig

[chainconfig/config.go](../services/bridge/deploy/chainconfig/config.go) owns
the canonical `config.json` schema and chain registry. EVM files live under
`contracts/bridge/evm/chains/<chain>/config.json`; Solana's file is
[bridge/solana/chains/solana/config.json](bridge/solana/chains/solana/config.json).

| Chain | Foreign chain ID | Native asset |
| --- | --- | --- |
| ethereum | 1 | ETH, 18 decimals |
| base | 8453 | ETH, 18 decimals |
| arbitrum | 42161 | ETH, 18 decimals |
| optimism | 10 | ETH, 18 decimals |
| bnb | 56 | BNB, 18 decimals |
| polygon | 137 | POL, 18 decimals |
| avalanche | 43114 | AVAX, 18 decimals |
| hyperevm | 999 | HYPE, 18 decimals |
| solana | 91600046870081 | wrapped SOL, 9 decimals |

The top-level fields are `chain`, `kind`, `chain_id`, `native`, `environment`,
`owner`, `attestors`, `threshold`, `finality_depth`, `assets`, and optional
`deployer`, `big_blocks`, and `solana`. `environment` contains **variable
names**, not endpoint or key values: `rpc_url`, `deploy_key`, and EVM-only
`explorer_key` or Solana-only `toolchain_bin`.

Each asset has `symbol`, `address`, `asset_id`, `decimals`, `per_tx_cap`, and
`total_cap`. Caps are decimal strings in base units, without leading zeros;
they must be positive, per-transaction cap must not exceed total cap, and both
must fit uint256 for EVM or uint64 for Solana. Asset decimals are 1–18.
The native asset comes first: address zero on EVM, the wrapped SOL mint on
Solana. EVM asset IDs equal their addresses; Solana uses the
[mint mapping](bridge-vectors.md#solana), including Sidiora's fixed exception.

`Load` forbids unknown fields and trailing JSON documents, applies any overlay,
then validates the schema. `Validate` accepts well-formed `PLACEHOLDER:<label>`
identities; `RequireDeployable` rejects placeholder owners, attestors and
program IDs. Attestors are unique and strictly ascending by their 20-byte
addresses, with at most 64 entries and `1 <= threshold <= count`.
`finality_depth` must be positive. Solana commitment is `confirmed` or
`finalized`. HyperEVM requires a `big_blocks` entry and acknowledgement that
the deploying account has been switched to big blocks.

`BRIDGE_DEPLOY_OVERLAY` selects a private JSON file with a `chains` map keyed
by chain name. [overlay.go](../services/bridge/deploy/chainconfig/overlay.go)
defines its permitted replacements: owner, deployer, attestors, threshold,
environment variable names, caps keyed by asset symbol, program ID, and
`big_blocks_acknowledged`. Go loaders and shell tools apply the overlay through
their respective implementations; the result still has to satisfy each
consumer's validation. An overlay does not supply a default credential.

## Scripts

Run these commands from the repository root with the intended environment
supplied.

| Tool | Interface and behavior |
| --- | --- |
| [deploy-evm-chain.sh](../services/bridge/deploy/deploy-evm-chain.sh) | `[--preflight] <chain\|--all>`; builds and broadcasts `PaxeerXVault`, checks chain ID, deployed bytecode, threshold, attestors and caps, then writes a record |
| [verify-evm-chain.sh](../services/bridge/deploy/verify-evm-chain.sh) | `[--preflight] <chain\|--all>`; submits explorer source verification using the recorded vault and constructor arguments |
| [deploy-solana-program.sh](../services/bridge/deploy/deploy-solana-program.sh) | `[--preflight]`; builds, deploys, checks rooted program bytes and authority, initializes and registers configured assets |
| [checklist.sh](../services/bridge/deploy/checklist.sh) | `<chain\|--all>`; compares foreign-chain state and Paxeer bridge views against records and generated governance expectations |

EVM deployment requires `jq`, `forge`, `cast`, `git` and `python3`; broadcasting
also requires `flock`. Its preflight derives the local signer and checks it
against the configured deployer, when one is supplied.

Preflight stops before a chain or explorer call; it is not evidence of a
deployment. EVM `--all` visits the eight chains in the table's order and stops
at the first failure. Checklist `--all` also visits Solana last.

`PAXEER_BRIDGE_DEPLOYMENT_RECORD` names the output record for one deployment;
`PAXEER_BRIDGE_DEPLOYMENT_RECORD_DIR` names an existing directory for `--all`.
`PAXEER_BRIDGE_EVM_CHAINS_ROOT` and `PAXEER_BRIDGE_SOLANA_CHAINS_ROOT` can select
run-local configurations. Endpoint, deployment key and verification key
values arrive through the variables named by each configuration.
The EVM broadcasting command passes its key as a process argument; use an
execution environment where other users cannot read that argument.

Solana additionally needs `PAXEER_BRIDGE_SOLANA_ADMIN_CLI` and the configured
toolchain directory containing `solana`, `solana-keygen` and `cargo-build-sbf`.
The script pins platform tools `v1.56`. An optional
`PAXEER_BRIDGE_SOLANA_PROGRAM_KEYPAIR_FILE` preserves the program ID across
runs; `PAXEER_BRIDGE_SOLANA_EXECUTABLE_WAIT_SECONDS` defaults to 120.
If `solana.program_id` is still a placeholder, the script deploys and records
the program and vault identity, then **fails before initialization**. Fill in
the recorded program ID and supply that same program keypair for the next run.
That intermediate record is not a ready deployment. The generated keypair is
retained beside the record and is never overwritten by a first-deploy run.

## Proposals

[paxeer-bridge-proposals](../services/bridge/deploy/proposals/cmd/paxeer-bridge-proposals/main.go)
generates JSON locally; it does not submit governance transactions. Its
[implementation](../services/bridge/deploy/proposals/proposals.go) accepts:

```text
paxeer-bridge-proposals [-manifest <path>] \
  -authority <bech32> -vault <address> [-readback <path>] \
  [-proposals <directory>] <chain-configuration.json> <output-directory>
```

Supplying `-authority` and `-vault` selects the canonical `chainconfig` loader;
both are required together. For Solana, vault is the 20-byte handle of the
deployed program's vault-authority PDA. The older no-flag input uses a
different flattened `proposals.ChainConfig` schema; a canonical chain file
must use the flags above.

The attestor manifest defaults to
[services/bridge/deploy/attestors.json](../services/bridge/deploy/attestors.json).
The committed set contains one attestor with threshold one. Membership,
ordering and threshold in the selected manifest must match the chain config.
The generator emits native protobuf message bodies for chain registration,
the attestor set, and asset caps, using the bridge module's codec and types.
Files are `01-register-chain.json`, `02-set-attestors.json`, and
`03-set-cap-%02d-%s.json`. With `-proposals`, it also emits
`04-proposal-open-chain.json` and, for Solana's Sidiora flow,
`05-proposal-sidiora-cap.json`. The latter registers the Sidiora pair before
setting its cap; it follows the opening proposal.

`-readback` writes expected chain, owner, vault, attestor, cap and Solana account
values for later comparison. Checklist reads the Paxeer side through
`PAXEER_BRIDGE_PAXEER_RPC_URL` at bridge precompile
`0x0000000000000000000000000000000000001016`, with
`PAXEER_BRIDGE_GOVERNANCE_AUTHORITY` supplying the authority for its generated
bodies. Configure that RPC as a [node root endpoint](evm-rpc.md#http-endpoint).

## Records

EVM records include chain identity, configuration, vault, deployer, code hash,
deployment block, observed/configured owner, ownership state, threshold,
attestors and caps. Ownership can be recorded as `proposed` rather than
`held`; a written file alone does not establish that ownership transfer is
complete. The configured owner must call `acceptOwnership()` on the vault.
The checklist reads back actual state.

The EVM deployer retains a `.<chain>.run` journal and `.<chain>.deploy.lock`
beside the output record. The journal stores the resolved configuration,
signer, broadcast intent and transaction evidence. Repeating deployment
reopens that same run and reads back its vault; it does not automatically
create a replacement vault. Preserve the journal to retain that identity.

Solana records include program ID, genesis hash, program-data account,
upgradeable loader ID, ELF SHA-256 and size, deployment signature/slot, rooted
slot, publisher and upgrade authority, vault-authority PDA and its handle.
A fully initialized record also carries owner, threshold, attestors and
registered assets. Keep keypair files and raw deployment evidence private;
the app receives only the public addresses described next.

## App env

[appenv/appenv.go](../services/bridge/deploy/appenv/appenv.go) and
[appenv/solana.go](../services/bridge/deploy/appenv/solana.go) validate records
before exporting their public app settings:

```text
paxeer-bridge-app-env [--check <env-file>] <records-directory>
```

It reads `.json` records, rejects duplicate chains and an empty directory, and
emits `NEXT_PUBLIC_BRIDGE_VAULT_<UPPERCASE_CHAIN>` for EVM chains plus
`NEXT_PUBLIC_BRIDGE_SOLANA_PROGRAM_ID` for a validated ready Solana record.
`--check` compares existing values; EVM address comparison ignores hex case,
Solana program comparison is exact. This export configures the bridge app,
not the [explorer frontend](explorer-frontend-config.md).

The Solana consumer requires `version: 1` and `phase: "ready"`; it recognizes
`deployed` and `initialized` as earlier phases but refuses them for export.
Ready records must include validated owner, attestors, threshold, and both SOL
and SID registrations. Only records satisfying these requirements are exported.

## CI

The owning shell checks are in
[services/bridge/deploy/tests](../services/bridge/deploy/tests/): checklist,
deployment scripts, EVM and Solana dry runs, documentation and workflow checks.
Explorer indexing configuration is documented [here](explorer-config.md).
