# Relayer tests

These scripts exercise IBC token transfers between two local upstream
[wasmd](https://github.com/CosmWasm/wasmd) chains. Install compatible `wasmd`
and `rly` executables, Bash, `jq` and Python 3.11 or later. Python's standard
`tomllib` module is used to read the node configuration. The relayer must
support `config add-chains`, `config add-paths`, `light init`,
`tx relay-packets` and `tx relay-acknowledgements`.

Paxeer embeds its Wasm module in `paxd`; these scripts use the external
upstream `wasmd` executable.

## Initialize the chains

From the repository root:

```sh
cd wasm/contrib/relayer-tests
./init_two_chainz_relayer.sh
```

The script creates `ibc-0` and `ibc-1`, configures the relayer's `demo` path
from [`configs/wasmd`](configs/wasmd), restores local test keys and initializes
light clients. Its node homes are under `data/` and its relayer home is
`.relayer/` in this directory. Initialization replaces those directories and
stops previously recorded example nodes. Use them only for disposable data.

| Chain | Consensus RPC | P2P | Profiling | gRPC |
| --- | --- | --- | --- | --- |
| `ibc-0` | 26657 | 26656 | 6060 | 9090 |
| `ibc-1` | 26557 | 26556 | 6061 | 9091 |

[`one_chain.sh`](one_chain.sh) starts a single node in the foreground with
these positional arguments:

```text
./one_chain.sh BINARY CHAIN_ID CHAIN_DIR RPC_PORT P2P_PORT PROFILING_PORT GRPC_PORT
```

The two-chain script starts it in the background for each chain.

## Transfer tokens

After initialization, run:

```sh
./test_ibc_transfer.sh
```

The [transfer helper](harness.py) links `demo`, sends `1000000test` from
`ibc-0` to `ibc-1`, and relays packets and acknowledgements. It checks the
packet sequence and channel, successful acknowledgement and exact balance
changes. The destination denomination is the IBC voucher for the transferred
token. The transfer operation has a 60-second deadline, and each relayer
command is limited to the remaining time or ten seconds, whichever is shorter.

To stop the example nodes from this directory:

```sh
python3 harness.py stop data
```
