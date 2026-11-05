# Chain-125 qualification profiles

All seven JSON profiles in this directory intentionally sign for EVM chain
**125** (`0x7d`). Their `qualification-chain-125-*` metric labels identify that
environment. The historical autobake name describes the workload only; it no
longer selects the retired autobake network. These are qualification workloads
for an operator-controlled, isolated chain-125 deployment, using root RPC URLs.
The local and Docker profiles use ports reachable from the **CLI's** network
namespace. In a container, loopback means that container; select root RPC URLs
reachable on its network or deliberately use host networking where supported.
Copy a profile and select endpoints appropriate to that environment before use.
Do not direct these unbounded workloads at public nodes without authorization.

The executable checks `eth_chainId` on every configured endpoint before any
deployment, prewarming, signing, submission, or transaction-file generation.
It also checks endpoints excluded by `--nodes`. A mismatch, unavailable RPC,
malformed identity, or canceled check aborts the command; each endpoint check
has a five-second timeout. Dry-run still checks identity before signing locally.
There is no automatic chain-ID substitution. For a deliberately different
isolated development chain, copy a profile, explicitly set its `chainId` and
environment label, and keep every endpoint on that chain.

The generator creates fresh random workload accounts and a separate random
deployer on every invocation. It does not import funded keys, persist keys, or
fund accounts. `newAccountRate: 0` fixes the pool during a run but does not fund
it. Live qualification requires an operator-controlled node/harness that provisions
native gas/value balances for **all generated senders and the deployer before
use**; contract workloads also require successful deployment and token allocation.
Without that funding integration these profiles support dry-run generation only,
not a claim of fee-paying network throughput. Matching chain ID alone does not
prove funding, network identity, transaction admission, or successful receipts.
See the funding wiring request in `repair_m823.kvx`.

The autobake transfer profile writes indented JSON to
`evm_transfer_report.json` in the process's current working directory. That
directory must be writable by the operator/container user. The logger does not
create parent directories: prepare any selected report directory before starting.
Report write errors propagate from the CLI. An explicit `--report-path` overrides
the profile; an explicitly empty value disables reporting.

From `tools/loadtest`, after building the executable with the supported toolchain:

```sh
./build/paxload --config profiles/autobake_evm_transfer.json --dry-run
mkdir -p reports
./build/paxload --config profiles/autobake_evm_transfer.json --dry-run \
  --report-path reports/evm-transfer.json
```

In the supported container runtime, mount the profile and a writable report
directory and select the mounted directory as the working directory:

```sh
docker run --rm --network host --user "$(id -u):$(id -g)" \
  --mount "type=bind,src=$PWD/profiles,dst=/profiles,readonly" \
  --mount "type=bind,src=$PWD/reports,dst=/reports" --workdir /reports \
  paxload --config /profiles/autobake_evm_transfer.json --dry-run
```

Create `reports` first and ensure its permissions permit the selected user to
write. Stop a run with SIGINT/SIGTERM to write the final report and evaluate its
performance bounds. Dry-run performance describes local simulation only.
The repair tests execute the production CLI in process, with real signing,
loopback JSON-RPC transport, collection, and JSON reporting from independent local
and mounted-directory layouts; they do not build or launch a container image.
