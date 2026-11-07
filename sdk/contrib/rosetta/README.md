# rosetta

The inherited Cosmos SDK Rosetta CI orchestration is **withdrawn**. Its Compose
manifest and node image recipe are retained only as historical examples in
[`docker-compose.yaml.historical`](docker-compose.yaml.historical) and
[`node/Dockerfile.historical`](node/Dockerfile.historical). The executable paths
`docker-compose.yaml` and `node/Dockerfile` have been removed together; neither
Docker Compose discovery nor the former explicit paths can launch this fixture.

The `test-rosetta` and `rosetta-data` targets in
[`sdk/Makefile`](../../Makefile) still reference the removed paths. They are
unsupported and fail at the missing Dockerfile before building or starting a
node. Removing those callers requires a separate change in the owning SDK
domain. Do not run those targets or pass the historical examples to Docker.

The historical image recipe builds
`./simapp/simd/`, which is absent from its SDK build context. The maintained
node command is [`paxd`](../../../daemon/paxd). The Rosetta API server code
is in [`sdk/server/rosetta`](../../server/rosetta), and `paxd` writes a
`[rosetta]` section into `app.toml` with `enable = false` by default. See the
[configuration defaults](../../server/config/config.go) and
[template](../../server/config/toml.go).

The `simd` network described below is a local test fixture. Paxeer mainnet-beta
is one network with EVM chain id 125 (`0x7d`); its kernel participates in the
network and proposes blocks. This fixture does not describe a mainnet
deployment or demonstrate a passing Rosetta integration for `paxd`.

## Historical fixture notes

The remaining notes describe the inherited example before subsequent fixture
repairs. They are retained for provenance, not as current operating instructions.
The shared native fixture code in `configuration/` is retained; withdrawing the
container target does not certify its integration with a maintained node.

### docker-compose.yaml.historical

Services:

- `cosmos`: a `simd` node with gRPC and the Tendermint RPC exposed
- `rosetta`: `simd rosetta` pointed at the `cosmos` node
- `faucet`: `configuration/faucet.py`, which the construction API test uses to fund accounts
- `test_rosetta`: the `tendermintdev/rosetta-cli:v0.6.7` image running `configuration/run_tests.sh`, which runs `rosetta-cli check:data` and `rosetta-cli check:construction`

### configuration

[`rosetta.json`](configuration/rosetta.json) selects the fixture's `app` /
`network` identifier, loads [`bootstrap.json`](configuration/bootstrap.json)
for balance reconciliation and uses [`transfer.ros`](configuration/transfer.ros)
for construction scenarios. [`run_tests.sh`](configuration/run_tests.sh) waits
for the Rosetta server and runs both checks.
[`faucet.py`](configuration/faucet.py) invokes
[`send_funds.sh`](configuration/send_funds.sh) to fund test accounts.

[`data.sh`](configuration/data.sh) generates a new snapshot; it does not
deterministically reproduce the bundled archive. It deletes `/root/.simapp`,
initializes the `testing` chain, creates a fresh random `fd` key using the
test keyring, funds and stakes that account, starts `simd`, sends a bank
transfer, then writes `/tmp/data.tar.gz`. It prints the new account and
validator addresses but does not update the CLI configuration files.
The `rosetta-data` Makefile target copies the generated archive back to
`node/data.tar.gz`; it depends on the same non-runnable image build.

Replacing the archive requires manually synchronizing the fixture metadata:

- Set the account address and opening balance in `bootstrap.json` from the
  replacement snapshot's actual state, accounting for staking, transfers and
  fees. Copying the genesis allocation alone is insufficient.
- Keep the genesis and funding denominations in `data.sh` and `send_funds.sh`
  consistent with the currency symbol and decimals in `bootstrap.json` and
  `transfer.ros`, including the construction `gas_price`.
- Keep the chain ID used by initialization, transactions and funding aligned,
  and keep the Rosetta blockchain/network identifiers aligned between Compose,
  `rosetta.json` and `transfer.ros`.

These files are currently inconsistent: the generator and funding script use
`stake`, while the bootstrap and construction scenarios use `uhpx` with zero
decimals, and the bootstrap names a different account from the bundled genesis.
Regenerating the archive alone does not resolve those mismatches.

### node

[`data.tar.gz`](node/data.tar.gz) holds a fixed snapshot of `simd` node data,
including genesis, keyring and block history, for message parsing and historical
balance tests. Its bundled keys are test fixtures. `send_funds.sh` selects the
test keyring and still pipes the fixture password `12345678` to `simd`.
The snapshot is fixed; running `data.sh` creates new identities and history.

### rosetta-cli

`rosetta-cli/Dockerfile` builds `rosetta-cli` at tag `v0.6.7` from the
upstream Coinbase repository.
