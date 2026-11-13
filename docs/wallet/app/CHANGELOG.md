# Wallet app version

The [package manifest](../../../apps/wallet/package.json) identifies the app as
`@paxeer/wallet-app`, version `0.1.0`. The browser app is served at `/wallet`
and uses the [wallet SDK](../../../sdk/wallet/src/index.ts) for embedded and
injected wallet access.

The app provides portfolio, send, receive, transaction, swap, discovery,
contacts and settings views through its
[shell routes](../../../apps/wallet/src/domains/shell/index.ts). Network
selection distinguishes the root network from tenant domains on the same chain.
Paxeer mainnet-beta uses EVM chain ID `125` (`0x7d`); a transaction is final
when its block is committed.

For setup, configuration and deployment, see the [documentation index](README.md).
