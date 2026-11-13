# 0x101D call and simulation response

`call` and `simulateCall` return the same canonical ABI tuple
`(int32 terminalResult, uint8 terminalKind, bytes response)`. The `response`
byte string carries the complete host terminal payload, including authenticated
execution and transfer evidence. `responseCapacity` (at most 1,048,576) applies
to the nested guest response bytes. It must never be applied to the length of
the terminal frame or used to truncate evidence. The Go execution-result
binding is [`DecodeProgramExecutionResult`](response.go).

The binding bounds the entire terminal payload separately at 1,048,576 bytes.
All integer lengths below are unsigned big endian. The ABI tuple and each
terminal or execution wrapper must consume their enclosing input. Trailing
bytes at those boundaries, noncanonical ABI sign extension, offsets, padding,
unknown execution versions and repeated or reordered wrappers are refused.

The outer frame is `LXP/programs/terminal-applied-legs/v1\0`, followed by
`u32 executionLength`, execution bytes, `u32 legsLength`, and applied legs.
There are at most 256 legs of 115 bytes each. Each leg is a zero tag, 32-byte
source, destination and asset, a positive 16-byte amount, and the bytes `00 01`.
Account and asset identifiers must be nonzero. The Merkle leaf is SHA-256 of
`LXP/v1/merkle-leaf\0 || leg`; parents hash
`LXP/v1/merkle-internal\0 || left || right`, duplicating an odd final leaf.
The empty set root is zero.

Consumers must supply a transfer root from an independently verified receipt
or trusted local simulation. Computing an expected root from the untrusted
frame itself does not authenticate transfers. For an executed call, bind the
`ProgramCallOutcome` log at 0x101D to its program, activity, receipt and block
before using its transferSetRoot. Simulation emits no committed log: use its
trusted simulation receipt, never a root from a different execution.

Execution may first carry one
`LXP/program-execution-with-transfer-authority/v2\0` wrapper containing
`u32 executionLength`, execution, `u32 authorizationLength`, nonempty
authorization, and a 32-byte transfer root equal to the verified root.

Next, execution may carry one `LXP/programs/sandbox-settlement/v1\0` wrapper:
`u32 executionLength`, execution, `u32 receiptLength`, a canonical usage receipt
of at most 4096 bytes, and a 32-byte transfer root equal to the verified root.
The receipt uses `LayerX/programs/sandbox/usage-receipt/v4\0`. The binding checks
its program, terminal kind, usage, fee schedule version and execution fee
against the nested execution. It also checks fee arithmetic and requires exactly one
applied leg matching the receipt's settlement destination and positive charge.
`SandboxUsageReceipt` retains the canonical receipt with its SHA-256 digest
appended; that digest alone does not authenticate the receipt.

Execution may then carry one `LXP/program-execution-with-occupancy/v1\0`
wrapper containing `u32 executionLength`, execution, `u32 occupancyLength`,
and occupancy evidence. Empty occupancy evidence is accepted only when the
sandbox receipt records both zero occupancy and zero occupancy fee. Preserve
all attachments for receipt and authority verification. The Go binding checks
their framing and transfer consistency; it does not verify authority signatures
or substitute for receipt authentication.

The nested `LXP/program-execution/v2\0` and `v3\0` formats are historical ABI-1
execution evidence and contain no guest byte response. The `v4\0` format is
ABI 2; `v5\0` is ABI 3 through 5. After the runtime and schedule versions,
typed execution values, usage and optional trace, modern executions carry the
program ID, ABI revision, and an outcome tag. Success (tag zero, terminalKind 1)
carries a nonnegative i32 guest result and `u64 guestLength` plus guest bytes.
Only guestLength is compared with responseCapacity. Failure (tag one, kind 2)
and resource refusal (tag two, kind 3) carry their typed details instead of a
successful guest response. A length-prefixed call graph follows the outcome,
using the domain `LayerX/programs/call-graph/v1\0`; its bytes remain attached
for replay and receipt verification. Outer
terminalResult is zero exactly for kind 1. Non-execution refusal details need
their native typed decoder and are explicitly refused by the execution binding.

The binding retains `TerminalFrame`, `Execution` (including wrappers),
`AppliedLegs`, `GuestResponse` and `SandboxUsageReceipt` separately. A successful
decode never discards evidence.
Host reopen must retain identical state and produce decodable results for both
call and simulation; simulation must leave persistent and transient stores,
events, sequence, balances and logs unchanged.
