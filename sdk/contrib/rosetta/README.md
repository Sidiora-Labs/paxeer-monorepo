# Rosetta tooling

The Rosetta API server lives in [`sdk/server/rosetta`](../../server/rosetta).
`paxd start` enables it through the `[rosetta]` section of `app.toml`; `enable`
defaults to `false`. See the [configuration defaults](../../server/config/config.go)
and [template](../../server/config/toml.go).

The [Paxd image guide](node/README.md) describes the explicit root-context
[`Dockerfile.paxd`](node/Dockerfile.paxd). It runs `paxd start` with Rosetta
enabled and requires an initialized, compatible local chain home.

## Local fixture

[`configuration/fixture.json`](configuration/fixture.json) identifies the pinned
[`node/data.tar.gz`](node/data.tar.gz) snapshot by its SHA-256 digest, account,
chain ID, denomination, balance and height. This is `simd` test state, with
public test signing keys. It is separate from a Paxeer node home and must not
be used for public-network signing.

The fixture uses chain ID `testing`, denomination `stake` with zero decimals,
and Rosetta identifiers `app` / `network`. These are fixture values; Paxeer
mainnet-beta has EVM chain ID 125 (`0x7d`) and finality at block commitment.

Build the fixture utility from the repository root:

```sh
go build -o rosetta-fixture ./sdk/contrib/rosetta/configuration/cmd/rosetta-fixture
```

Put the resulting binary on `PATH` for the Python and shell launchers.
[`command.go`](configuration/command.go) supplies three operations:

- `restore`: validates the pinned archive and derives fixture metadata from its
  committed bank state. It installs the snapshot into an absent or empty
  `--home`, writes the same archive to `--output`, and writes `fixture.json`,
  `bootstrap.json` and `transfer.ros` in `--config`. A nonempty home is refused.
- `send`: funds one Cosmos address using the fixture metadata and a local `simd`
  executable with the test keyring. It checks the committed transaction result.
- `serve`: runs the fixture HTTP faucet, which uses the same funding path.

[`data.sh`](configuration/data.sh) invokes `restore`,
[`send_funds.sh`](configuration/send_funds.sh) invokes `send`, and
[`faucet.py`](configuration/faucet.py) invokes `serve`. Restoration reproduces
existing fixture identities and state; it does not generate a new account.

[`rosetta.json`](configuration/rosetta.json) loads
[`bootstrap.json`](configuration/bootstrap.json) for reconciliation and
[`transfer.ros`](configuration/transfer.ros) for construction.
[`run_tests.sh`](configuration/run_tests.sh) waits for a Rosetta server, then
runs `rosetta-cli check:data` and `rosetta-cli check:construction`.
[`rosetta-cli/Dockerfile`](rosetta-cli/Dockerfile) builds CLI tag `v0.6.7`.

## Container entry points

The Compose fixture and `simd` image recipe are retained as
[`docker-compose.yaml.historical`](docker-compose.yaml.historical) and
[`node/Dockerfile.historical`](node/Dockerfile.historical). This checkout has no
`simd` application source. The SDK Makefile's `test-rosetta` and `rosetta-data`
targets refer to absent container entry points and are unsupported. Use the
explicit Paxd image recipe with a fresh compatible chain home for local node
operation.
