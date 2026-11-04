# Dev scripts

Manual test scripts inherited from upstream [wasmd](https://github.com/CosmWasm/wasmd).
They drive a standalone `wasmd` binary with local chain id `testing`. This repository
vendors the `x/wasm` module for `paxd` and has no `wasm/cmd/wasmd` command. Paxeer's
merged network runs in `paxd`; its mainnet EVM chain id is 125. The local `testing`
chain described here is an upstream development example.

## Prerequisites

Put a compatible upstream `wasmd` build on your `PATH` and install Bash and `jq`.
The scripts use historical CLI forms, including `add-genesis-account`, `gentx` with
`--amount`, and block broadcast mode (`-b block`); a recent upstream release may
require script changes before it can run them.

[`setup_wasmd.sh`](setup_wasmd.sh) defaults to `CHAIN_ID=testing`,
`MONIKER=node001`, `STAKE_TOKEN=ustake`, `FEE_TOKEN=ucosm`, and
`PASSWORD=1234567890`. That password is for disposable local keys only. Keep the
chain and token defaults when using the numbered scripts: their transactions
hardcode `testing` and `ustake`. The key commands use the upstream CLI's default
keyring backend and can prompt for its password.

## Start the local node

Run from the repository root. Choose a fresh, dedicated home directory; use the
same absolute path in every shell so that all commands find the same node
configuration and validator/fred keyring. The `env` prefixes apply that home to
each child process.

```sh
cd wasm/contrib/local
WASMD_EXAMPLE_HOME=/tmp/wasmd-example
mkdir -p "$WASMD_EXAMPLE_HOME"
env HOME="$WASMD_EXAMPLE_HOME" bash setup_wasmd.sh
env HOME="$WASMD_EXAMPLE_HOME" bash start_node.sh
```

[`start_node.sh`](start_node.sh) runs in the foreground and binds the local
consensus RPC to `tcp://0.0.0.0:26657`. Use it on an isolated development machine;
the transaction examples connect to `http://localhost:26657`.

## Run the examples

In a second shell, again starting at the repository root:

```sh
cd wasm/contrib/local
WASMD_EXAMPLE_HOME=/tmp/wasmd-example
env HOME="$WASMD_EXAMPLE_HOME" bash 01-accounts.sh
env HOME="$WASMD_EXAMPLE_HOME" bash 02-contracts.sh
```

Run them in that order: `01-accounts.sh` creates `fred` and transfers local stake;
`02-contracts.sh` uses both `validator` and `fred`. It stores `hackatom.wasm`,
instantiates and queries a contract, changes its admin, then migrates to
`burner.wasm` using fixtures in
[`x/wasm/keeper/testdata`](../../x/wasm/keeper/testdata). These state-changing
examples expect a fresh setup and are not idempotent.

The optional [`03-grpc-queries.sh`](03-grpc-queries.sh) queries `localhost:9090`
after the contract example. It also needs `grpcurl`, an upstream node with gRPC
enabled, and compatible protobuf imports. It refers to `wasm/proto` and
`wasm/third_party/proto`; the latter directory is absent in this tree, so the
script needs its upstream import layout restored before use. With those
prerequisites satisfied, keep the same home when invoking it:

```sh
env HOME="$WASMD_EXAMPLE_HOME" bash 03-grpc-queries.sh
```

[`00-genesis.sh`](00-genesis.sh) is a separate pre-start example, not part of the
sequence above: it expects both keys to exist and includes a hardcoded contract
address. Review it against the chosen upstream version and genesis state first.

## Shell script development

[Use `shellcheck`](https://www.shellcheck.net/) to avoid common mistakes in shell scripts.
[Use `shfmt`](https://github.com/mvdan/sh) to ensure a consistent code formatting.

See also the [devtools limitations](../devtools/README.md) and
[protobuf generation guidance](../prototools-docker/README.md).
