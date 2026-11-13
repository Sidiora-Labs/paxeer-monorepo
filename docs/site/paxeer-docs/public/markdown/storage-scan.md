<!-- Source: https://docs.paxeer.app/storage-scan/ -->

# Bounded Storage Scans

Iterate Paxeer X program state with deterministic ordering, explicit page budgets, scope-bound continuation cursors, and metered canonical results.

## Read a namespace in bounded pages

The guest host function `layerx_v2.storage_scan_scoped` returns ordered key/value pairs matching a byte prefix. Every call declares an entry ceiling and a complete encoded-page byte ceiling. A portable continuation token resumes after the last returned key across activities without retaining a host iterator or allocator handle.

This function belongs to the frozen ABI v2 namespace, `layerx_v2`. The runtime declares its nine i32 parameters and i32 result in the versioned ABI manifest. A program must declare an ABI version that includes this namespace.

Use scans for bounded indexes, application inventories, and paginated state processing. Design keys with stable prefixes and choose page sizes that fit the call's storage-read budget and guest output buffer. Returning scan data as an activity response also consumes the response output budget. The function scans one authorized namespace.

## Host-call contract

```
storage_scan_scoped(
  raw_selector: i32,
  prefix_pointer: i32, prefix_length: i32,
  cursor_pointer: i32, cursor_length: i32,
  max_entries: i32, max_bytes: i32,
  output_pointer: i32, output_capacity: i32
) -> i32
```

| Input | Contract |
| --- | --- |
| Selector 1 | Principal-scoped namespace; requires `StorageRead` and the matching access declaration. |
| Selector 2 | Shared program namespace; requires `SharedStorageRead` and the matching access declaration. |
| Prefix | At most 256 bytes. An empty prefix selects all keys within the namespace. |
| Cursor | Empty starts a scan. A nonempty token must match the same namespace, prefix, and limits. |
| Entry ceiling | Between 1 and 64 entries per page. |
| Page byte ceiling | Between 5 and 67,126,228 complete encoded bytes. |
| Output range | A valid guest-memory buffer large enough for the encoded result. |
| Return value | Nonnegative encoded byte length on success; a negative host status on refusal. |

The host validates the output range before reading the prefix and cursor. It previews and encodes the page, checks output capacity, charges the storage-read meter, then copies bytes to guest memory. A negative length, invalid selector, out-of-memory range, or denied read is rejected before output is written.

## Decode the canonical page

All lengths below are big-endian. Parse only the number of bytes returned by the host, validate each length against the remaining buffer, and require the cursor fields to agree.

| Field | Width |
| --- | --- |
| Entry count | u16 |
| For each entry: key length, key | u16 followed by key bytes |
| For each entry: value length, value | u32 followed by value bytes |
| Has-cursor flag | u8: 0 or 1 |
| Cursor length, cursor | u16 followed by cursor bytes; zero length when no cursor |

```
# Empty terminal page: 5 bytes
00 00 00 00 00

# One entry, key "a", value "b", no continuation: 13 bytes
00 01 00 01 61 00 00 00 01 62 00 00 00
```

The complete encoding consumes the storage-read budget, including counts, lengths, cursor bytes, and the five-byte empty page. Allocate room for continuation metadata as well as the key/value content. A page with one short entry and a continuation can be larger than a terminal page with several short entries.

## Resume without changing scan identity

The cursor binds version, canonical namespace, prefix, entry ceiling, byte ceiling, and the last returned key. Treat its bytes as an opaque continuation token in application code. Reuse them exactly with the same selector, program, principal where applicable, prefix, and both limits.

Changing a limit to increase page size requires starting a new scan with an empty cursor. A principal cursor cannot resume in shared scope or under another program. Corrupted bytes, trailing bytes, empty last-key fields, and keys inconsistent with the prefix are invalid. The maximum encoded cursor size is 591 bytes.

### Pagination workflow

1. Choose the authorized scope and stable prefix. Select both page ceilings and allocate a guest output buffer.
2. Call with an empty cursor and check the signed return status before decoding memory.
3. Decode the complete page, process its entries, and retain the exact continuation bytes.
4. When a cursor is present, invoke again with unchanged scan identity and limits.
5. Stop when the page has no continuation cursor.

Each resumed activity reads its current authorized state. The cursor identifies a position and scan contract; it does not claim that all pages share a frozen historical state. For an application that needs a consistent export, bind the surrounding workflow to its own verified state and mutation policy.

## Two independent page bounds

The scan stops before exceeding the entry count. It also checks the complete encoded size when adding each matching entry. If an entry would exceed the page-byte limit, it is removed and the preceding entries return with a cursor. If no entry can fit, the scan refuses with a bounds status rather than returning an ambiguous empty continuation.

In a principal-scoped namespace with an empty prefix, three pairs `(a,a)`, `(b,b)`, and `(c,c)` have a 29-byte terminal encoding. With a fourth pair and limits `(64, 101)`, the first page contains two entries plus continuation and occupies 101 bytes. A 100-byte limit yields one entry plus continuation at 93 bytes. The byte ceiling must allow continuation metadata while the page is assembled.

## Refusals and unchanged output

| Status | Meaning | Response |
| --- | --- | --- |
| `-1` denied | Missing matching read authority or access declaration. | Supply the correct admitted grant and scope. |
| `-2` invalid | Malformed input, invalid selector or limits, or a mismatched cursor. | Correct the request; restart pagination if identity changes. |
| `-3` bounds | Oversized prefix/cursor, invalid memory range, insufficient output capacity, or no entry fitting the page ceiling. | Correct memory bounds or start a new scan with suitable limits. |
| `-4` meter | Insufficient admitted storage-read budget for the encoded page. | Increase the activity budget within admitted ceilings. |

Refusals before charging leave storage-read usage unchanged. A refused charge produces no partial page. The host writes output only after validation and metering succeed, so output bytes remain unchanged on these refusal paths. Never interpret preexisting buffer contents as a fresh page after a negative return.

## Determinism, writes, and rollback

Storage is ordered by program, scope, and key. Within the chosen namespace, entries return in canonical key order independent of insertion order. Equivalent state and scan inputs produce equivalent pages, usage, and evidence.

A scan sees writes already staged in the same activity because both use the activity's storage snapshot. A successful activity commits those writes. A later trap discards the staged mutation and returns the runtime-fault outcome without a successful response. Do not treat a page observed inside a failed guest execution as committed state.

Read [Programs](https://docs.paxeer.app/programs) for call budgets, response transport, and receipt verification, or [the developer workspace](https://docs.paxeer.app/program-workspace) for conformance and ABI checks.

**Technical references:** programs/crates/layerx-programs-runtime/src/host/scan.rs, src/storage/scan.rs, src/abi/storage_ops.rs, src/abi/manifest.rs, and tests/storage_scan.rs within that runtime crate.
