# layerxbridge

This directory holds the chain module for attested transfers between Paxeer and
external chains. The `layerxBridge` precompile calls its keeper to mint bridged
tokenfactory denominations for accepted deposits and burn them for outbound
transfers. Governance controls chain registration, attestors, asset caps and pause
state; the keeper persists consumed deposit events, outstanding supply and outbound
nonces. The module runs within Paxeer's BFT network, where receipts are final when
their block is committed.

## Layout

- [client/](client/) — governance proposal submission through the CLI and REST handler.
- [keeper/](keeper/) — persistent state, signature checks, minting, burning, governance messages, Sidiora registration and genesis import/export.
- [testutil/](testutil/) — application-backed keeper setup, store helpers and attestor signing utilities for tests.
- [types/](types/) — state and genesis types, governance messages and proposals, codecs, events, errors and attestation encodings.
- [module.go](module.go) — module registration, message service wiring and genesis lifecycle.
- [handler.go](handler.go) — atomic execution of the messages carried by a bridge governance proposal.
- [ATTESTATION.md](ATTESTATION.md) — digest layouts, signature rules, denomination mapping and test vectors.

## Build and test

The directory belongs to the root [Go module](../../go.mod) and uses the root
build targets. Run these commands from the repository root:

```sh
make paxd-build
make -f chain.mk test
```

The [Makefile](../../Makefile) target `paxd-build` builds the native libraries and
the node that includes this module. The [chain.mk](../../chain.mk) target `test`
splits the root Go packages into test groups, including the bridge packages, and
runs them with race detection and coverage. Bridge tests cover governance
submission, attestation vectors, transfer limits, replay protection, Sidiora
denominations and persistent genesis recovery.

## Configuration

The module receives its genesis state from the node; its schema is defined in
[types/genesis.go](types/genesis.go). Configuration fields include `params.authority`,
`paused`, `chains`, `attestors`, `assets` and `caps`. Chain records carry `chain_id`,
`vault`, `finality_depth` and `enabled`; the attestor set carries `attestors` and
`threshold`; caps carry `denom`, `max_in_flight` and `max_per_tx`.

Default genesis registers no chains, attestors or caps. The module authority
controls registration, attestor changes, caps and pause state through governance
messages. Sidiora's pair is registered with `MsgRegisterSidioraPair` before its
caps are set. These operations are implemented in
[keeper/msg_server.go](keeper/msg_server.go).

The governance CLI in [client/cli/tx.go](client/cli/tx.go) reads the JSON proposal
file supplied to `layerxbridge-proposal`. The module reads no environment
variables or standalone configuration files.

## Documentation

- [Bridge overview](../../docs/wiki/Bridge.md)
- [Unified network](../../docs/wiki/UnifiedNetwork.md)
- [Module parameters and messages](../../docs/system/chain-modules.md#layerxbridge)
- [Governance](../../docs/system/governance.md)
- [Precompile reference](../../docs/system/precompiles.md#layerxbridge-abijson)
- [Documentation site: bridging and balances](../../docs/site/paxeer-docs/public/markdown/bridge.md)
- [Attestation format](ATTESTATION.md)
