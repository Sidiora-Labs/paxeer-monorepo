# Quickstart

Connect to Paxeer X Network, derive an account, fund its EVM address, send a
native payment and deploy a WASM program. Paxeer is one BFT network with EVM
chain ID `125` (`0x7d`). The kernel runs inside `paxd`, executes transactions
and participates in block proposal. A receipt becomes final when its block
is committed. Sub-chains are tenant domains in the database, not additional
networks to provision for this walkthrough.

Use [Public RPC endpoints](../reference/public-rpc.md) for node connections
and [Checkpoints](../concepts/checkpoints.md) for committed-block evidence.
[Programs](../programs/index.md) describes the WASM runtime, and
[Assets](../concepts/assets.md) explains native asset encodings.

## 1. Connect to a node

The public HTTPS JSON-RPC endpoints are the root of
`https://api-mainnet-beta.paxeer.network` and
`https://api1.mainnet-beta.paxeer.network` through
`https://api14.mainnet-beta.paxeer.network`. Keep several configured and
compare `eth_blockNumber` when choosing a node near the head.

```sh
export RPC_URL=https://api1.mainnet-beta.paxeer.network
curl --fail --silent --show-error --max-time 15 "$RPC_URL" \
  --header 'Content-Type: application/json' \
  --data '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
```

The response's `result` must be `"0x7d"`. Public node namespaces are `debug`,
`echo`, `eth`, `net`, `pax`, `test` and `txpool`; an advertised namespace does
not imply every upstream Geth method is available. WebSocket clients use
root URLs such as `wss://api1.mainnet-beta.paxeer.network`.

The unified router and `/rpc`, `/rpc/ws` and `/rpc/schema` routes are retired.
`node.hyperpaxeer.com` is only the hpx library mirror, never an RPC endpoint.

## 2. Derive an account and fund it

You need the `layerx` CLI from this checkout, Node.js with npm, `curl`, `jq`,
and a Rust toolchain with the `wasm32-unknown-unknown` target for the Programs
step. See [CLI](../platform/cli.md) for the CLI's local tooling. Set
`MNEMONIC_FILE` to the absolute path of your private BIP-39 phrase file.
Run the following from the repository root; generated example files go in a
private temporary directory:

```sh
export LAYERX_PROGRAM_SDK="$PWD/programs/sdk/rust"
QUICKSTART_DIR="$(mktemp -d)"
chmod 700 "$QUICKSTART_DIR"
cd "$QUICKSTART_DIR"
layerx --json wallet derive --mnemonic-file "$MNEMONIC_FILE" \
  --export-private-keys evm-keys.json > account.json
export EVM_KEY_FILE="$PWD/evm-keys.json"
jq '.data | {evm_address, did, layerx_public_key}' account.json
```

`wallet derive` reads the phrase locally. Its JSON envelope has
`kind: "wallet.derived"`; `.data` contains `evm_address`, `did` and
`layerx_public_key`. The new owner-only export contains `evm_private_key`
and `layerx_seed` as well as the public account identifiers. Keep that export
private. Deriving the pair does not bind it on-chain; step 4 does that.
The producer is `tools/platform/cli/src/wallet_derive.rs`.

Have an existing holder transfer native chain funds to `.data.evm_address`.
Those funds pay transaction gas and the native payment below. There is no
public-network faucet promised by this guide. Read the EVM balance at the
latest committed state:

```sh
jq -nc --arg address "$(jq -r '.data.evm_address' account.json)" \
  '{jsonrpc:"2.0",id:1,method:"eth_getBalance",params:[$address,"latest"]}' |
  curl --fail --silent --show-error --max-time 15 "$RPC_URL" \
    --header 'Content-Type: application/json' --data-binary @-
```

The `result` is a hexadecimal quantity of native base units. Funding this
address is an ordinary chain transfer. The former custody-credit executor
at `0x1013` and checkpoint anchor at `0x1014` are retired onboarding paths.

## 3. Prepare a client that submits at the node root

The checked-in CLI has local wallet and program build commands, but its
network transport in `tools/platform/cli/src/rpc.rs` and
`tools/platform/cli/src/programs.rs` still targets the retired prefixed RPC
route. For this walkthrough, submit EIP-1559 transactions with an Ethereum
client configured with `RPC_URL` from step 1.

Install ethers in the temporary directory:

```sh
npm install ethers@6
```

Save the following as `quickstart.mjs` there. It signs locally, stores each
signed transaction before broadcasting, and queries the original hash on
restart. Run only one transaction command at a time for this account.

```javascript
import { readFileSync, writeFileSync, existsSync } from "node:fs";
import { createPrivateKey, createPublicKey, sign } from "node:crypto";
import {
  Contract, FetchRequest, Interface, JsonRpcProvider, Wallet, ZeroHash,
  getAddress, getBytes, hexlify, keccak256, parseEther, randomBytes,
  sha256, toBeHex, zeroPadValue,
} from "ethers";

const url = new URL(process.env.RPC_URL);
if (url.protocol !== "https:" || url.pathname !== "/" || url.search || url.hash) {
  throw new Error("RPC_URL must be the HTTPS node root");
}
const request = new FetchRequest(url.href);
request.timeout = 15000;
const provider = new JsonRpcProvider(request, undefined, { batchMaxCount: 1 });
if (BigInt(await provider.send("eth_chainId", [])) !== 125n) {
  throw new Error("Expected chain ID 125");
}
const [mode, argument] = process.argv.slice(2);
const modes = ["bind", "pay", "deploy", "receipt"];
if (!modes.includes(mode) || (mode === "receipt" && !argument)) {
  throw new Error("Usage: node quickstart.mjs bind|pay <address>|deploy|receipt <file>");
}
const file = mode === "receipt" ? argument : `${mode}.pending.json`;
const addrAddress = "0x0000000000000000000000000000000000001004";
const programsAddress = "0x000000000000000000000000000000000000101d";
const addrABI = [
  "function getUnifiedAccount(address) view returns (address,string,bytes32,bytes32)",
  "function layerXBindNonce(address) view returns (uint64)",
  "function bindLayerX(bytes32,bytes)",
];
const programsABI = [
  "function deploy(bytes32,uint16,uint8,bytes32,bytes32,bytes,bytes)",
  "event ProgramDeployed(bytes32 indexed programId,bytes32 codeHash)",
];
let pending;
if (existsSync(file)) {
  pending = JSON.parse(readFileSync(file, "utf8"));
} else {
  if (mode === "receipt") throw new Error("Saved transaction file does not exist");
  const key = JSON.parse(readFileSync(process.env.EVM_KEY_FILE, "utf8"));
  const wallet = new Wallet(`0x${key.evm_private_key}`, provider);
  if (wallet.address !== getAddress(key.evm_address)) throw new Error("Key/address mismatch");
  const edKey = createPrivateKey({
    key: Buffer.concat([
      Buffer.from("302e020100300506032b657004220420", "hex"),
      Buffer.from(key.layerx_seed, "hex"),
    ]),
    format: "der", type: "pkcs8",
  });
  const publicKey = hexlify(createPublicKey(edKey).export({
    format: "der", type: "spki",
  }).subarray(-32));
  if (key.did !== `did:layerx:${publicKey.slice(2)}`) throw new Error("DID/key mismatch");
  const addr = new Contract(addrAddress, addrABI, provider);
  let transaction;
  let deployment;
  if (mode === "bind") {
    const current = await addr.getUnifiedAccount(wallet.address);
    if (current[2] === publicKey) {
      console.log("Account pair is already bound");
      process.exit(0);
    }
    if (current[2] !== ZeroHash) throw new Error("Address is bound to another DID");
    const nonce = await addr.layerXBindNonce(wallet.address);
    const message = Buffer.concat([
      Buffer.from("LX:PAXEER-BIND:v1"),
      Buffer.from(getBytes(zeroPadValue(toBeHex(125), 32))),
      Buffer.from(getBytes(wallet.address)),
      Buffer.from(getBytes(toBeHex(nonce, 8))),
    ]);
    transaction = {
      to: addrAddress,
      data: addr.interface.encodeFunctionData("bindLayerX", [publicKey, sign(null, message, edKey)]),
    };
  } else if (mode === "pay") {
    transaction = { to: getAddress(argument), value: parseEther("0.001") };
  } else {
    const current = await addr.getUnifiedAccount(wallet.address);
    if (current[2] !== publicKey) throw new Error("Bind this account pair before deployment");
    const built = JSON.parse(readFileSync("build.json", "utf8"));
    if (!built.ok || built.kind !== "program.built") throw new Error("Program build did not succeed");
    const wasm = readFileSync(built.data.artifact);
    if (104 + wasm.length > 524288) throw new Error("Deployment payload exceeds 524288 bytes");
    deployment = { programId: hexlify(randomBytes(32)), codeHash: sha256(wasm) };
    transaction = {
      to: programsAddress,
      data: new Interface(programsABI).encodeFunctionData("deploy", [
        deployment.programId, built.data.abi_version, 0, ZeroHash,
        deployment.codeHash, "0x", wasm,
      ]),
    };
  }
  const populated = await wallet.populateTransaction({ ...transaction, chainId: 125, type: 2 });
  const raw = await wallet.signTransaction(populated);
  pending = { hash: keccak256(raw), raw, deployment };
  writeFileSync(file, JSON.stringify(pending), { mode: 0o600, flag: "wx" });
  console.log(`Saved ${file}; transaction ${pending.hash}`);
  const returned = await provider.send("eth_sendRawTransaction", [raw]);
  if (returned.toLowerCase() !== pending.hash) throw new Error("Broadcast hash mismatch");
}
if (keccak256(pending.raw) !== pending.hash) throw new Error("Saved transaction hash mismatch");
const deadline = Date.now() + 120000;
while (Date.now() < deadline) {
  const receipt = await provider.send("pax_sequencerReceipt", [pending.hash]);
  if (receipt && receipt.txHash.toLowerCase() !== pending.hash) throw new Error("Receipt hash mismatch");
  if (receipt?.rung === "committed") {
    const evm = await provider.send("eth_getTransactionReceipt", [pending.hash]);
    const block = await provider.send("eth_getBlockByNumber", [receipt.blockNumber, false]);
    if (!evm || evm.transactionHash.toLowerCase() !== pending.hash ||
        evm.blockHash !== receipt.blockHash || evm.blockNumber !== receipt.blockNumber ||
        block?.hash !== receipt.blockHash || BigInt(block.number) >
          BigInt(await provider.send("eth_blockNumber", []))) {
      throw new Error("Committed receipt/block mismatch");
    }
    if (BigInt(receipt.status) !== 1n || BigInt(evm.status) !== 1n) {
      throw new Error(`Committed execution failed: ${receipt.vmError}`);
    }
    if (pending.deployment) {
      const iface = new Interface(programsABI);
      const event = evm.logs.filter(log => log.address.toLowerCase() === programsAddress)
        .map(log => iface.parseLog(log)).find(log => log?.name === "ProgramDeployed");
      if (!event || event.args.programId !== pending.deployment.programId ||
          event.args.codeHash !== pending.deployment.codeHash) {
        throw new Error("Deployment event does not match the submitted program");
      }
    }
    writeFileSync(`${file}.receipt.json`, JSON.stringify({ receipt, evm }), { mode: 0o600 });
    console.log(JSON.stringify({ txHash: pending.hash, rung: receipt.rung,
      blockNumber: receipt.blockNumber, blockHash: receipt.blockHash,
      deployment: pending.deployment }));
    process.exit(0);
  }
  await new Promise(resolve => setTimeout(resolve, 1000));
}
throw new Error(`Outcome pending; resume with: node quickstart.mjs receipt ${file}`);
```

The script observes the node's committed result and checks transaction,
block and deployment-event bindings. Independent cryptographic verification
requires the additional evidence described in step 6. An unsupported
`pax_sequencerReceipt` call is an error: obtain a node with receipt forwarding
from the operator rather than interpreting a submission acknowledgement as
finality. `rpc/sequencer_receipt.go` defines that method and its response.

## 4. Bind the account and send a payment

Programs requires an EVM caller bound to its DID. The `bind` command reads
`getUnifiedAccount` and `layerXBindNonce`, signs the chain-specific binding
message with the Ed25519 key, and sends `bindLayerX(bytes32,bytes)` to the
addr precompile at `0x1004` in an EIP-1559 transaction. It refuses to replace
a different DID. The binding layout and ABI are implemented in
`modules/evm/types/layerx_binding.go` and `precompiles/addr/Addr.sol`.

```sh
node quickstart.mjs bind
```

After that transaction commits, set `RECIPIENT` to the intended recipient's
20-byte EVM address. This sends `0.001` native coins, plus transaction gas:

```sh
node quickstart.mjs pay "$RECIPIENT"
```

The script saves `bind.pending.json` and `pay.pending.json` before sending.
Re-running a command with its existing file queries the saved hash without
creating another transaction. A timeout or lost connection leaves the
outcome unknown; resume observation with:

```sh
node quickstart.mjs receipt pay.pending.json
```

If the operator establishes that retransmission is needed, submit the exact
saved `raw` bytes with `eth_sendRawTransaction`. Keep the original nonce and
hash; do not create a replacement payment merely because a reply was lost.
Archive the saved file before intentionally making a different payment.

## 5. Build and deploy a program

Scaffold and build locally, in the temporary directory:

```sh
layerx new quickstart-program
layerx --json program build --manifest-path quickstart-program/Cargo.toml > build.json
jq '.data | {artifact, code_hash, byte_size, abi_version, deterministic_validation}' build.json
node quickstart.mjs deploy
```

The scaffold uses the SDK path set in step 2 and a release profile with
`opt-level = "z"`. `program build` compiles for
`wasm32-unknown-unknown`, validates the selected guest ABI and deterministic
runtime policy, and returns `kind: "program.built"` with the artifact path,
SHA-256 code hash, byte size and ABI version. These producers are
`tools/platform/cli/src/scaffold.rs` and
`tools/platform/cli/src/programs.rs`.

The client calls
`deploy(bytes32,uint16,uint8,bytes32,bytes32,bytes,bytes)` on Programs at
`0x000000000000000000000000000000000000101d`. It uses a new nonzero program
ID, the build's ABI version, immutable policy `0`, zero upgrade authority,
the WASM SHA-256 hash and an empty optional interface. The payload limit is
524288 bytes including the 104-byte envelope for deployment without an
interface. A published interface changes that envelope and must match the
module and ABI; do not substitute a different guest ABI to bypass validation.

Deployment requires the Programs surface to be active and sufficient funds
for fees. `precompiles/layerxprograms/LayerXPrograms.sol`,
`payload_lifecycle.go`, `principal.go` and `method_deploy.go` define the ABI,
payload, caller binding and `ProgramDeployed` event. The size bound is
`LXP_MAX_PAYLOAD_BYTES` in `include/layerx/lxp_protocol.h`.
The output records the program ID and code hash after matching that event
in the committed EVM receipt. Resume interrupted observation with:

```sh
node quickstart.mjs receipt deploy.pending.json
```

The old hosted `/v1/programs/deploy` and `/v1/activities` write routes are
retired; `production_route` in the hosted gateway rejects them.

## 6. Read receipts and establish finality

`pax_sequencerReceipt` takes one `0x`-prefixed 32-byte transaction hash. Its
result is `null` for an unknown hash, or an object with the following rungs:

| Rung | Meaning |
| --- | --- |
| `submitted` | `txHash` and `nonce` identify a known transaction without a signed execution receipt. |
| `receipted` | Signed receipt bytes, `keyId`, `signingKey`, execution `status`, outputs and logs are available before block commitment. |
| `committed` | The receipt is covered by the committed sequence batch; `blockNumber` and `blockHash` identify its covering block. |

Quantities such as `nonce`, `height`, `seq`, `status` and `blockNumber` are
hexadecimal strings. A successful execution has `status: "0x1"`. The helper
keeps the response and EVM receipt in `<saved-file>.receipt.json`.
The `eth_getTransactionReceipt` response and subscription notifications can
be available before commit, so their presence alone is insufficient.

For independent verification, decode the canonical receipt, verify its
signature, bind every decoded field to the transaction and check its
signing key against an independently trusted governed pin and its inclusive
height range. The node implementation and hosted contract name the key-list
method `px_getReceiptKeys`; availability on a particular public endpoint is
not guaranteed. Obtain the trusted pins from the operator. A key returned
alongside an untrusted receipt does not establish its own authority.
`rpc/sequencer_receipt.go` implements
`SequencerReceiptResult.Verify` and the governed key lookup.

Authenticate the CometBFT block header and commit and verify inclusion of
the covering sequence batch. `programs/crates/layerx-programs-registry/src/chain_proof/`
contains chain proof verification. There is no later external settlement or
checkpoint step. The historical authorized-batch flags of
`layerx receipt verify` are not a substitute for this chain evidence.

For streaming consumers, subscribe to `newHeads` or `logs` at the node's root
WebSocket URL. Persist the last processed block number/hash, transaction
index and log index, then backfill a bounded `eth_getLogs` range after
reconnecting. Deduplicate overlapping logs and advance the durable cursor
only after processing them; a subscription ID is not a restart cursor.

## 7. Optional hosted identity and starter credit

Hosted `px_` methods belong to an operator-provided gateway. Obtain its
complete JSON-RPC URL and authentication requirements from the operator;
the public node hostnames do not establish a public hosted gateway.
The method contract is
`services/platform/hosted/gateway/openrpc.json`, with registration and
funding implemented in `src/rpc_register.rs` and `src/rpc_faucet.rs` beside it.

| Method | Positional parameters and result |
| --- | --- |
| `px_register` | Lowercase Ed25519 public key (64 hex) and proof-of-possession signature (128 hex). Returns the principal's `tenant`, `sub`, `allowed_signer_public_keys`, `account` and `audiences`. |
| `px_resolveAccount` | An EVM address or explicit `did:layerx:<64 hex>` selector. Returns the EVM/Paxeer/DID/native-account join and `bound`; unbound halves are null. |
| `px_getAccount` | The same EVM/DID selector. Returns `{account, paxeer}`; `paxeer` contains address, balance, nonce and height when an EVM binding exists. |
| `px_requestFunds` | DID and bound lowercase Ed25519 public key. Requires a Bearer session authorizing that signer; returns `funded`, `funding_id`, `transaction_id`, decimal-string `amount`, `network` and `starter_credit`. |
| `px_getCapabilities` | Empty array. Reports whether each probed precompile surface, including `programs`, is available. |
| `px_getReceiptKeys` | Empty array. Returns `{keys:[{keyId,publicKey,firstHeight,lastHeight}]}`. Key IDs/public keys are `0x`-prefixed; height bounds are JSON integers. |

For `px_register`, sign the SHA-256 digest of three parts: ASCII
`layerx-register-binding-v1`, ASCII `beta` and the raw 32-byte signer public
key. Prefix **each** part with its byte length as an unsigned 64-bit
big-endian integer before hashing. This proof is distinct from the raw
on-chain addr binding message in step 4. Registration is idempotent for the
same key and does not itself provide a Bearer session or pay transaction gas.
Use the operator's identity session flow before requesting starter credit.

Keep the original EVM/DID selector for subsequent reads; a returned
`layerx_account` identifier is not a reverse lookup selector. Starter credit
is a hosted grant with its own funding evidence, not a claim that the EVM
address has enough native gas. Incomplete claims must be queried again
against the same principal/grant rather than replaced with another grant.

Malformed registration parameters return `-32602`; invalid possession proof
returns `-32002`. A pending funding claim returns `-32001` with
`data.code: "faucet_claim_pending"`. Missing receipt pins return `-32003`
with `data.code: "receipt_keys_unavailable"`. An unavailable precompile
surface refuses hosted transaction submission with `-32003` and
`data.code: "surface_unavailable"`; do not treat an error as a zero balance,
a funded account or a completed deployment.

Private test environments may provide their own faucet, credentials and CA.
Keep those operator-supplied settings separate from the public chain-125
connection above. The old network-402 cluster export and checkpoint setup
are retired as public-network onboarding instructions.

[Home](../index.md)
