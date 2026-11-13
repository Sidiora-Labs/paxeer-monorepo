# Changelog

## Current package

The umbrella package version is `10.2.6`, declared in [mix.exs](mix.exs).
Its runtime versions are pinned in [.tool-versions](.tool-versions).

The backend indexes Paxeer mainnet-beta, EVM chain ID `125` (`0x7d`), with
`CHAIN_TYPE=paxeer_x` and `ETHEREUM_JSONRPC_VARIANT=paxeer_x`. Paxeer is one
BFT `paxd` network; its kernel proposes blocks, and a receipt is final when
its block commits. The explorer's database can lag the node.

The [backend overview](README.md) describes the native account, transaction
status, signature, price-feed and archive APIs. Transport configuration and
Paxeer response normalization are described in the
[JSON-RPC client reference](apps/ethereum_jsonrpc/README.md).
