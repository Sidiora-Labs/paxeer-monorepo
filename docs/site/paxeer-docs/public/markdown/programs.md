<!-- Source: https://docs.paxeer.app/programs/ -->

# Deterministic Programs

Deploy and call deterministic Paxeer X programs with scoped capabilities, metered execution, governed upgrades and verifiable outputs.

## One execution surface, explicit authority

Programs extend the LayerX kernel domain of Paxeer X with application logic. A program executes a validated WebAssembly artifact under the identity, capability grants, access declarations, and resource ceilings of its invoking activity. The interpreter removes floating-point nondeterminism and enforces structural limits before execution. The same admitted activity and state produce replayable results.

Programs dispatch through kernel module 9. They do not become economic modules and never write balances directly. All monetary effects compile to authenticated `402LXP` transfer sets applied by the kernel. The kernel executes within the Paxeer BFT node on EVM chain ID `125`. A receipt is final when its covering block is committed.

## Application building blocks

| Capability | Application behavior |
| --- | --- |
| Namespaced storage | Read, write, delete, and scan bounded program state; distinguish principal-scoped state from explicitly granted shared state. |
| Program composition | Invoke another program with an explicit callee grant, narrowed capabilities, bounded depth, and response capacity. |
| Native value accounts | Derive and register program-owned accounts for native Assets; stage authorized payments, escrow releases, swaps, and vault movements. |
| Receipt evidence | Consume explicitly granted receipt digests and expose receipt-verified deployment, state, and balance information. |
| Deterministic data and arithmetic | Read admitted context, verify signatures, hash bytes, perform 256-bit integer arithmetic, and consume oracle and web evidence through versioned host imports. |
| Bounded computation | Run the deterministic interpreter, lease isolated sandbox execution, and adjudicate disputed market steps. |

## Deploy, call, upgrade, and retire

### Deploy an artifact and interface

A deployment binds a 32-byte program ID to its WASM artifact, declared SHA-256 code hash, guest ABI version, and upgrade policy. The immutable policy has no upgrade authority. The authority policy names the principal permitted to upgrade. An optional canonical interface describes entry selectors, input and output bounds, and required capability descriptors. Artifact identity is the program ID and code hash together.

Deployment creates version 1 and emits deployment evidence. Retain and verify that receipt before treating a registry entry as proof of deployment. Snapshots retain artifacts so restored state resolves the same code.

### Execute a bounded call

A CALL names the program, guest ABI, entrypoint, canonical calldata, capability grants, access declarations, response capacity, and seven resource-budget dimensions. The runtime verifies the active program record and opens the matching artifact before running guest code. Entrypoint names use letters, digits, underscores, and periods.

The hosted `POST /v1/programs/call` and `POST /v1/programs/simulate` routes carry the same native CALL bytes. Simulation executes against the current head and returns a receipt, terminal, call graph, and signed simulation evidence without committing durable state, sequence, or occupancy. A simulation is a preview against that head; retain the actual execution receipt for settlement evidence.

### Upgrade with an explicit policy

An upgrade must match the recorded authority and current code hash, supply a new artifact and hash, and follow admitted ABI transitions. Optional hook bytes and interface replacement or removal accompany the upgrade. The version counter increments and upgrade evidence binds the old and new hashes. Immutable programs reject upgrades.

### Wind down without stranding value

Lifecycle states are active, deprecated, and tombstoned. Deprecated or tombstoned programs refuse new CALL execution and upgrades. Wind-down requires an authorized exit route for nonzero program-account balances. Confirm the balance-proof requirements in the registry source for the lifecycle transition you use.

## Authority and atomic settlement

Storage access, events, calls, transfers, receipt reads, shared storage, balance views, and program spending are separate grants. A callee cannot obtain authority beyond its parent. Identity fields remain bound; monetary ceilings only narrow. Capability escalation refuses the composition rather than committing a partial transfer set.

A program account is derived from the program ID and an exact seed. Derivation alone grants no ownership, registration, funding, or debit authority. The owner registers the account for an Asset; a principal funds it through an ordinary transfer. A `ProgramSpend` grant separately binds the owner program, seed, source account, Asset, destination, and maximum amount. Only the deriving program frame stages the debit, and repeated transfers consume the ceiling cumulatively.

```
program account = SHA-256(
  "LayerX/programs/program-account/v1\0"
  || program_id32 || seed_length:u32_be || seed
)
```

## Metering and persistent-state occupancy

| Declared ceiling | What consumes it |
| --- | --- |
| CPU fuel | Interpreter instructions and admitted host work. |
| Memory bytes | Peak guest linear memory. |
| Storage read / write bytes | Cumulative state access and mutation. |
| Output values / bytes | Guest return values, response bytes, and refusal transport. |
| Table elements | Peak guest table allocation. |

Admission checks each ceiling and reserves fee coverage before guest execution. Persistent storage also incurs deterministic occupancy measured in protocol batches, separate from the execution meter. A signed responsibility mandate names the payer and caps stored bytes and lifetime charge.

```
byte_batches = recorded_bytes × (to_batch − from_batch)
accrued_fee = byte_batches × occupancy_byte_batch_price
amount_due = prior_arrears + accrued_fee
```

Occupancy settlement becomes final with its covering block commitment and uses the same native transfer primitive. Confirm payer assignment and charge-ceiling dispositions in the runtime occupancy source before relying on them in an application.

## Versioned guest interfaces

Guest ABI, kernel module ABI, and wire protocol are independent version numbers. Select the guest ABI required by the artifact and its imports rather than assuming that the kernel module version describes the guest interface.

| Guest ABI | Host surface added |
| --- | --- |
| 1 | Storage, event emission, program calls, principal transfers, and granted receipt reads. |
| 2 | Responses and refusals; scoped storage and scanning; program transfers and funding; context and balance reads; hashing, signature verification and recovery, and 256-bit integer operations. |
| 3 | `oracle_read` for admitted oracle evidence. |
| 4 | `web_read` for admitted web evidence. |
| 5 | `market_step_adjudicate` for deterministic market-step verdicts. |

External evidence enters through admitted protocol paths. A state transition does not dial out to an arbitrary service. Frozen ABI vectors and manifests preserve the byte contract across implementations.

## Outputs you can verify

Calls return bounded response or refusal bytes and publish terminal evidence, resource usage, call-graph information, and transfer commitments. Verify the activity identity, code and state bindings, terminal, and requested commitment before using a result to release value or advance a workflow. The registry verifies account-tree proofs through the universal subtree and receipt state root before exposing balances.

A granted guest receipt read returns exactly 116 bytes: receipt digest, signed result code, Asset ID, unsigned 128-bit amount, and state root. The digest must match the requested receipt. This lets a merchant program consume authenticated payment evidence instead of trusting a caller-supplied payment description.

## Developer workflow

1. Choose the Rust, C, or AssemblyScript guest SDK and define the application's state, entrypoints, bounds, and capability needs.
2. Compile to WebAssembly, generate the canonical interface, and hash the artifact.
3. Deploy with an immutable or authority upgrade policy; verify the deployment receipt.
4. Register and fund program value accounts where the application holds Assets.
5. Construct a CALL with the minimum grants and sufficient execution and occupancy ceilings. Simulate against the current head.
6. Submit the signed activity and verify the committed terminal and receipt before displaying completion.

Rust reference applications include escrow, naming, LXT-721 NFTs, merchant payments, constant-product swaps, LXT-20 tokens, vaults, and web readers. C and AssemblyScript provide paid-counter examples. EVM, Solana/Anchor, and CosmWasm porting guides map familiar concepts to explicit capabilities and native transfer settlement.

### Example: fixed-supply LXT-20

The LXT-20 reference is an ABI-2 program backed by one native Asset. It exposes `initialize`, `transfer`, `approve`, `transfer_from`, `balance_of`, `allowance`, `total_supply`, and `metadata`. Supply, Asset, and spend ceiling are fixed at build time. Approval of zero revokes an allowance. A recipient must approve at least once, including zero, to register its derived account in token storage, and the deployment authority must separately register that account with the native Programs account module before it can receive backing units. Arbitrary external recipient accounts are refused. This reference has no mint, burn, permit, or nested-call entrypoint.

```
# Canonical initialize request (hex)
4c581400012000000000

# Build a Rust guest from the Programs workspace
cargo build --manifest-path programs/sdk/rust/examples/token-lxt20/Cargo.toml \
  --target wasm32-unknown-unknown --release
```

Continue with [bounded sandbox execution](https://docs.paxeer.app/sandbox), [native Assets](https://docs.paxeer.app/assets), and [payments](https://docs.paxeer.app/payments).

**Technical references:** Paxeer X Programs workspace and guest SDKs; Programs and Sandbox protocol documentation; runtime ABI policy, capability encoding, metering, and receipt-bound registry.
