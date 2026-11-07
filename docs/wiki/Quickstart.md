# Quickstart

Connect to Paxeer mainnet-beta, fund a wallet, submit a transaction and check
its committed outcome. Paxeer is one BFT network run by `paxd`, with EVM chain
ID `125` (`0x7d`). The kernel is part of the network and proposes blocks; a
receipt is final when its block is committed. See
[Unified network](UnifiedNetwork.md) for the execution and evidence model.

The public walkthrough below uses node JSON-RPC and an EVM signing wallet.
The [private development section](#private-development-profiles) covers local
credentials, program builds and disposable infrastructure separately.

## 1. Connect to the public network

Use the root HTTPS JSON-RPC endpoint:

```sh
export RPC_URL=https://api-mainnet-beta.paxeer.network

curl --fail-with-body --silent --show-error --max-time 30 "$RPC_URL" \
  --header 'Content-Type: application/json' \
  --data '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
```

Require `result` to be `"0x7d"` before signing. The other public node names
are `https://api1.mainnet-beta.paxeer.network` through
`https://api14.mainnet-beta.paxeer.network`, also at the root path. Compare
`eth_blockNumber` when choosing a current node. A lagging or unavailable node
does not establish that a transaction failed.

The pre-merge router and public `/rpc`, `/rpc/ws` and `/rpc/schema` paths are retired.
`node.hyperpaxeer.com` is the hpx library mirror used by build scripts, not an RPC endpoint.

The node namespace inventory is `debug`, `echo`, `eth`, `net`, `pax`, `px`, `test`
and `txpool`; availability depends on node configuration. The `px` namespace answers from node state; `px_register` and
`px_requestFunds` are operations of the hosted identity and faucet
services declared in `services/platform/hosted/gateway/openrpc.json`. Obtain the service endpoint
from its operator when using that contract. A gateway method is not a promise
that every public node serves it.

## 2. Create and fund a wallet

Connect an EVM signing wallet to chain `125`. Obtain native PAX for gas from
an existing funded account or an operator-provided allocation. There is no
public faucet. Read `eth_getBalance(address, "latest")` to check native funds.
Opening a ledger account or binding a DID does not create a balance.

For applications that need a DID, bind the EVM address through the `addr`
precompile at `0x0000000000000000000000000000000000001004`.
`precompiles/addr/Addr.sol` declares the interface. The DID key signs the
chain-bound message and the EVM owner submits `bindLayerX`; see
[Account binding](UnifiedNetwork.md#account-binding) for the exact bytes and
nonce. A standalone `layerx key create` credential is an Ed25519 key, not a
funded EVM signing wallet.

## 3. Submit a native PAX transfer

For an application using ethers v6 and an injected EIP-1193 wallet, request
approval for the intended recipient and amount:

```ts
import { BrowserProvider, parseEther } from "ethers";

async function transferPax(
  wallet: ConstructorParameters<typeof BrowserProvider>[0],
  recipient: string,
  amountPax: string,
) {
  const provider = new BrowserProvider(wallet);
  if ((await provider.getNetwork()).chainId !== 125n) {
    throw new Error("Connect to Paxeer mainnet-beta (chain 125)");
  }
  await provider.send("eth_requestAccounts", []);
  const signer = await provider.getSigner();
  const tx = await signer.sendTransaction({
    to: recipient,
    value: parseEther(amountPax),
  });
  const receipt = await tx.wait();
  if (receipt === null || receipt.status !== 1) {
    throw new Error(`Transfer did not succeed: ${tx.hash}`);
  }
  return { transactionHash: tx.hash, receipt };
}
```

Pass the browser's wallet provider as `wallet`. The wallet signs locally;
the public node does not hold the user's key. Clients that sign their own
EVM transactions submit the signed bytes with `eth_sendRawTransaction`.
Retain those bytes and their transaction hash before broadcasting.

## 4. Use native ledger assets

Ledger assets and payments execute on the same chain through LayerXLedger at
`0x000000000000000000000000000000000000101a`. Use the ABI in
`precompiles/layerxledger/abi.json`, implemented by
`precompiles/layerxledger/` and `modules/layerxledger/`.

Bind the participating EVM owners to their DIDs first. Wait for a successful
committed receipt after each prerequisite write:

| Step | Call and authority |
| --- | --- |
| Register | The issuer calls `register(assetId, salt, symbol, name, decimals, supplyCap, issuerKind, custodyReference)` with the asset's canonical parameters. |
| Open | Each owner calls `openAccount(assetId)` for its per-asset account. |
| Identify | Read `accountId(owner, assetId)`, where `owner` is the bound DID public key as `bytes32`. |
| Mint | The authorized issuer calls `mint(assetId, accountId, amount)` within the asset's cap and issuance policy. |
| Pay | The source owner calls `payExact(from, to, assetId, amount, expiresAtMs, paymentReference)`. |
| Read | Read `balanceOf(accountId, assetId)`, `getAsset(assetId)` and `nextSequence(accountId, assetId)` with `eth_call`. |

Amounts are integer asset units. For `payExact`, the amount is `uint128`,
expiry is a future Unix timestamp in milliseconds (`uint64`), and
`paymentReference` is `bytes32`. `from` and `to` are per-asset account IDs,
not EVM addresses or DID strings. The keeper selects the next source-account
sequence and emits `PaymentExact`.

The `send(bytes envelope)` entry point accepts a canonical signed SEND
envelope; its encoding is different from `payExact` calldata. Use the ABI
and canonical builders for the chosen entry point. The real-process lifecycle
in `daemon/committed-fixture/vector_v4.go` binds two identities, registers an
asset, opens accounts, mints and pays through EVM calls.

Native gas funding and ledger asset issuance are different operations within
one network. Custody-credit funding and the legacy `lx_sendActivity` payment
flow are retired; do not use their old instructions to fund this lifecycle.

## 5. Fetch the receipt and establish commitment

Set `TX_HASH` to the transaction hash returned by the wallet or node:

```sh
jq -n --arg hash "$TX_HASH" \
  '{jsonrpc:"2.0",id:1,method:"eth_getTransactionReceipt",params:[$hash]}' |
  curl --fail-with-body --silent --show-error --max-time 30 "$RPC_URL" \
    --header 'Content-Type: application/json' --data-binary @-
```

A null result means that node has no receipt available yet. Status `"0x0"`
records failed execution; `"0x1"` records success. Check the transaction hash,
block hash, status and expected effects. Commitment does not turn failed
execution into a successful payment.

Where configured, the optional `pax_sequencerReceipt(transactionHash)` method
reports signed execution progress. Its implementation is
`rpc/sequencer_receipt.go`:

| Rung | Meaning |
| --- | --- |
| `submitted` | A send record exists without a signed execution receipt. |
| `receipted` | A signed execution receipt exists; commitment has not been established. |
| `committed` | A committed block's `MsgSequenceBatch` covers the receipt's sequence. |

A receipt signature alone does not prove block inclusion or BFT commitment.
For independent verification, authenticate the receipt key ranges and chain
trust, then verify the signed transaction, execution-result and sequence-batch
inclusion proofs and validator commits. See
[SDK terminal verification](SdkTerminalVerification.md). The legacy
`layerx receipt verify` authorized-batch check is not the merged-chain
commitment proof.

## 6. Recover an uncertain submission

If a request times out, recover by the existing transaction hash. Query the
receipt and account nonce before authorizing another debit. Rebroadcasting
identical signed bytes preserves the hash; automatically signing a new payment
can charge twice.

Inspect JSON-RPC `error` even when HTTP succeeds. Standard code `-32601`
means the method is unavailable and `-32602` means invalid parameters. The
optional receipt API returns `-32000` when it has neither a local receipt
source nor a configured forwarding endpoint. Keep that absence explicit;
an unavailable receipt is not a successful payment.

## 7. Use supported hosted workflows

For gateway reads, use the operator's endpoint and its published OpenRPC
contract:

| Method | Use |
| --- | --- |
| `px_getNetwork` | Read network identity, EVM chain ID and head information. |
| `px_resolveAccount`, `px_getAccount` | Resolve an EVM address or explicit DID and inspect its binding. |
| `px_getBalances`, `px_listAssets` | Read the declared balance and asset views with their source metadata. |
| `px_getHistory`, `px_getUnifiedHistory` | Read indexed history using the declared cursor and limit parameters. |
| `px_getCapabilities` | Inspect native capability probes before choosing a workflow. |

Keep the original EVM address or DID selector for later lookups; a native
account ID is a different identifier. A bare 64-hex account selector is
refused with `-32001` and
`data.code = "native_account_reverse_resolution_unsupported"`.

For service-managed payments, use [Human journeys](HumanJourneys.md) with the
deployment's authentication and allowance policy. `layerx payment test`
uses `/v1/moves/quote` followed by `/v1/moves` in
`tools/platform/cli/src/payment.rs`; it requires a service implementing that
Human contract. It is not a direct public-node payment command. A service
acknowledgement or history entry is not evidence of a successful committed
debit.

## Private development profiles

These instructions are for a local emulator or an explicitly provisioned
private deployment. Profile names and private network IDs do not configure
the public EVM chain.

### Credentials and a local emulator

Build/install instructions and headless credential-store setup are in the
[CLI README](../../tools/platform/cli/README.md#headless-credential-storage).
With `layerx` on `PATH`, provision and start the local emulator:

```sh
umask 077
export LAYERX_CONFIG="$HOME/.config/layerx/config.json"
mkdir -p "$(dirname "$LAYERX_CONFIG")"
layerx emulator provision
layerx emulator up \
  --sequencer-seed-file "$HOME/.config/layerx/emulator/sequencer.seed"
```

In another terminal using the same config and credential-store settings:

```sh
layerx environment use emulator \
  --endpoint http://localhost:9402 \
  --network-id 402 \
  --sequencer-trust-anchor-file "$HOME/.config/layerx/emulator/sequencer.anchor"
layerx key create quickstart
layerx environment current
```

`tools/platform/cli/src/emulator.rs` requires endpoint, network ID and one
trust-anchor input together when configuring a profile. The local emulator
uses loopback port `9402` and private network ID `402`. Its trust-anchor flag
is emulator configuration, not public-network finality. Credential creation
stores a random Ed25519 seed in the selected backend; the default backend is
the OS keyring. It does not register or fund an account.

For a private hosted session, store the operator-provided token via stdin
with `layerx auth set --environment beta` and inspect availability with
`layerx auth status --environment beta`. The `beta` profile must be configured
for that deployment; a session token does not imply authorization for every
gateway route.

### Build and deploy a program

Scaffold and validate a local WASM artifact:

```sh
layerx new quickstart-program
layerx --json program build --manifest-path quickstart-program/Cargo.toml
```

Use the returned artifact path. The current `program deploy` command takes
that artifact plus `--program-id`, `--evm-key-file` and optional `--fee-limit`;
`--interface` and `--upgrade-authority` are optional. The lifecycle in
`tools/platform/cli/src/programs.rs` encodes a native deployment and signs
an EIP-1559 transaction to the Programs precompile at
`0x000000000000000000000000000000000000101d`.

Deployment requires a funded EVM signer and a configured service transport
supporting that command. Check the deployed capability and current command
help before submission. The old recipe using account-sequence, state-root
and validity-window flags is retired. A local build does not deploy a program
or prove commitment. Continue with [Programs](Programs.md) for lifecycle and
interface details.

### Disposable beta infrastructure

Operators who need the repository's private integration harness can use
`make platform-beta-cluster-up`, which invokes
`services/platform/hosted/tests/beta-cluster.sh up`. Review that script's
inputs and [Beta cluster](BetaCluster.md) before provisioning; this is a
multi-service operator workflow, not a prerequisite for using mainnet-beta.
It builds images and provisions infrastructure and requires the declared
owner inputs and tools.

After a successful `up`, source its generated environment:

```sh
. build/beta-cluster/env
```

The file supplies deployment-specific origins such as `LAYERX_GATEWAY_URL`,
`LAYERX_TESTNET_URL`, `LAYERX_FAUCET_URL` and `LAYERX_PAXEER_BOUNDARY_URL`, plus
CA and token file paths. Use the supplied CA for private HTTPS and keep token
files private. A private faucet, when provisioned, is limited to that
harness; its response does not establish public funding or transaction
commitment. Historical checkpoint/settlement variables in the harness do not
change the merged network's BFT finality.

When finished with the disposable deployment, use
`make platform-beta-cluster-down`. The teardown removes harness resources;
retain any needed local artifacts before running it.

[Getting started](Getting-Started-Beta.md) ·
[Unified network](UnifiedNetwork.md) ·
[SDK terminal verification](SdkTerminalVerification.md) ·
[Home](Home.md)
