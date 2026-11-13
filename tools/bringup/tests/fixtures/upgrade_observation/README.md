# Upgrade observation fixtures

The systemd show transcripts describe active, inactive and failed service
states for the upgrade observer. `{{PID}}` in the active transcript is replaced
with the observer test process ID, so process identity reads use that process's
`/proc` state and executable bytes.

[`upgrade_peers.py`](upgrade_peers.py) serves separate loopback node and CometBFT
RPC endpoints for each of three units. It loads native RPC responses from the
supplied capture and advances each unit's block height on `eth_blockNumber`.
Status and commit responses use the same height. These endpoints provide peer
responses for the shell transport integration; they do not start chain nodes.

[`xweb-assets.json`](xweb-assets.json) describes the ledger asset metadata used
by the hosted peer.
