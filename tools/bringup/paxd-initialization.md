# Native chain initialization

Initialize one chain home with the [initialization script](kernel-genesis.sh):

```bash
tools/bringup/kernel-genesis.sh init "$NODE_HOME" "$MONIKER" "$CHAIN_ID" "$MODE" "$GENESIS_JSON" "$GENESIS_SHA256" "$STOPPED_UNIT"
```

Supply the approved native genesis and its SHA-256. The command checks that the
named systemd service is stopped, requires an empty home, runs `paxd init` with
the explicit mode and chain ID, installs the genesis, and retains identity
fingerprints. `PAXD` selects the binary (default `paxd`), and
`BRINGUP_INSPECT_BIN` selects the inspector (default `bringup-inspect`). The
command leaves the service stopped.

## Retain an existing identity

Stop the home's exclusive service owner and retain the complete home, including
the native node key, validator key, last signing state, and sequencer receipt
key and WAL. Then adopt its identity:

```bash
tools/bringup/kernel-genesis.sh adopt "$NODE_HOME" "$CHAIN_ID"
tools/bringup/kernel-genesis.sh identity "$NODE_HOME" "$CHAIN_ID"
```

Identity checks refuse changed keys or genesis. Signing state may advance
normally; it is validated and never reset. Keep primary and standby receipt
keys and WAL separate. Receipt key changes require the native chain's committed
receipt-key governance and pin update procedure.

## Offline artifact conversion

`offline-render ENV_FILE OUTPUT_DIR` renders signed LXGB genesis artifacts
from an environment file. `offline-regenerate-fixture FIXTURE_DIR` regenerates a signed fixture.
Both operations run offline without starting a node or querying the network.
