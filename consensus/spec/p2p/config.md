# P2P Config

Paxeer's `paxd` process embeds the consensus engine and its peer manager. These
settings control node discovery and connections on the single network; they do
not create another execution or settlement domain. The kernel participates in
this network and proposes blocks. A transaction receipt is final when its block
is committed.

The configuration file is `config/config.toml` under the node home selected by
`paxd start --home <node-home>`. The `[p2p]` keys below come from
[`P2PConfig`](../../config/config.go) and the
[configuration template](../../config/toml.go). Both use hyphenated keys.
Only some settings have CLI flags: the registrations in
[`AddNodeFlags`](../../cmd/tendermint/commands/run_node.go) are included by
[`paxd start`](../../../sdk/server/start.go).

## Seed Mode

The inherited seed crawler settings `p2p.seed_mode` and `p2p.seeds` are retired
configuration recipes for this fork. Use `bootstrap-peers` to introduce peers
and `pex` to exchange addresses. Although `--mode seed` is accepted as a node
mode, it does not select the old crawl, advertise and disconnect behavior: the
current peer manager and PEX reactor do not implement that special seed loop.

## Seeds

Use bootstrap addresses in TOML, with actual node IDs and reachable hosts:

```toml
[p2p]
bootstrap-peers = "0123456789abcdef0123456789abcdef01234567@192.0.2.10:26656"
pex = true
```

The address is an example, not a public bootstrap node. `bootstrap-peers` enters
addresses into the regular peer pool. PEX also learns addresses during the
handshake and through periodic requests. There is no registered `--p2p.seeds`
flag; set `bootstrap-peers` in the file.

## Persistent Peers

```sh
paxd start --home /srv/paxeer \
  --p2p.persistent-peers '0123456789abcdef0123456789abcdef01234567@192.0.2.10:26656'
```

The equivalent file entry is:

```toml
[p2p]
persistent-peers = "0123456789abcdef0123456789abcdef01234567@192.0.2.10:26656"
```

The peer manager maintains a separate persistent pool. After a disconnect the
peer becomes eligible for dialing again. The old one-day retry limit and
`persistent_peers_max_dial_period` recipe do not describe this implementation.
`dial-interval` controls the general dial rate; its configuration default is
`"10s"`. A bootstrap address whose ID is persistent belongs to the persistent
pool as well. Persistent connections do not consume the regular pool limits.

`blocksync-peers` is another comma-separated address list in `[p2p]`. Its entries
are persistent and restrict which peers are eligible for classic block sync;
with an empty list, all connected peers are eligible. This does not configure
Autobahn's validator list or certificate streams.

## Private Peers

Use `--p2p.private-peer-ids` on the CLI or `private-peer-ids` in `[p2p]`:

```toml
[p2p]
private-peer-ids = "0123456789abcdef0123456789abcdef01234567"
```

These IDs are excluded when the peer manager advertises connected peers through
PEX. This is an advertisement policy, not access control or a promise that a
peer's address is hidden from local administrative queries. To dial a private
peer persistently, also provide its address in `persistent-peers`.

## Unconditional Peers

`unconditional-peer-ids` in `[p2p]` lists IDs assigned to the persistent pool,
which is outside the regular inbound and outbound connection limits. Supply a
known address separately when an outbound connection is needed. The registered
CLI spelling is unusually `--p2p.unconditional_peer_ids` (underscores); it differs
from the TOML key. Prefer the file key to avoid that spelling mismatch.

The current regular-pool settings are `max-connections` (default 100) and the
optional `max-outbound-connections`. Node setup derives outbound capacity as
`min(20, (maxConnections + 1) / 2)` when no override is supplied; inbound capacity
is the remaining regular capacity. The old `max_num_inbound_peers` and
`max_num_outbound_peers` names are not current keys.

## Connection Settings and Autobahn

`laddr` defaults to `"tcp://127.0.0.1:26656"`; set an appropriate listening address
for a reachable node. `external-address` supplies the address advertised to
peers. `handshake-timeout` defaults to `"10s"` and `dial-timeout` to `"3s"`.

The compatibility multiplexer uses `flush-throttle-timeout`,
`max-packet-msg-payload-size`, `send-rate` and `recv-rate`. The node configuration
defaults are `"100ms"`, 1000000 bytes, and 20971520 bytes per second in each
direction. They override the lower-level multiplexer constructor's defaults;
see [Connection](./connection.md).

A non-empty top-level `autobahn-config-file` selects Autobahn in
[node setup](../../node/setup.go). Its JSON validator addresses are separate
from PEX and the classic block-sync peer list. Startup requires a local
validator key and rejects a configured remote signer. See the
[Autobahn file schema](../../config/autobahn.go) and the
[message applicability map](./messages/README.md#mode-applicability).
