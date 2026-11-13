# Local Wasm scripts

These [wasmd](https://github.com/CosmWasm/wasmd) examples drive a standalone
upstream `wasmd` binary. Paxeer embeds the Wasm module in `paxd`; its mainnet
EVM chain ID is 125. The scripts use a separate local Cosmos chain named
`testing`.

## Prerequisites

Install Bash, `jq` and a compatible upstream `wasmd` on `PATH`. The scripts
use CLI forms including `add-genesis-account`, `gentx --amount` and block
broadcast mode (`-b block`).

Use a disposable user environment with a fresh `.wasmd` node home and keyring.
The scripts use the CLI's default home and keyring backend. All shells running
the examples must use that same environment.

[`setup_wasmd.sh`](setup_wasmd.sh) reads `CHAIN_ID`, `MONIKER`, `STAKE_TOKEN`,
`FEE_TOKEN` and `PASSWORD`. The defaults for the chain and tokens are
`testing`, `ustake` and `ucosm`. Set `PASSWORD` to the passphrase for the
local test keyring. Keep the chain and token defaults when using the numbered
examples, whose transactions select `testing` and `ustake` explicitly.

## Start the local node

From the repository root:

```sh
cd wasm/contrib/local
bash setup_wasmd.sh
bash start_node.sh
```

The setup creates and funds the `validator` key and collects its genesis
transaction. [`start_node.sh`](start_node.sh) runs the node in the foreground
and listens for consensus RPC on all interfaces at port 26657. The transaction
examples connect to that port locally.

## Run the examples

In a second shell using the same disposable environment, start at the
repository root:

```sh
cd wasm/contrib/local
bash 01-accounts.sh
bash 02-contracts.sh
```

Run them in that order. [`01-accounts.sh`](01-accounts.sh) creates the key
named `fred` and transfers local stake to it.
[`02-contracts.sh`](02-contracts.sh) uses both test keys. It uploads
`hackatom.wasm`, instantiates and queries a contract, executes it, changes its
admin, migrates it to `burner.wasm`, and clears its admin. The contract fixtures
are in [`x/wasm/keeper/testdata`](../../x/wasm/keeper/testdata). These examples
change chain state and expect a fresh setup.

## Shell script development

Use [ShellCheck](https://www.shellcheck.net/) to check shell scripts and
[shfmt](https://github.com/mvdan/sh) for formatting. See also the
[development tools](../devtools/README.md) and
[protobuf generation guide](../prototools-docker/README.md).
