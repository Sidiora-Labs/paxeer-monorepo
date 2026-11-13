# Storage scan

`layerx_v2` `storage_scan_scoped` is the Candidate-V2 bounded, resumable
storage-scan host function
(`programs/crates/layerx-programs-runtime/src/host/scan.rs`,
`programs/crates/layerx-programs-runtime/src/abi/manifest.rs`).
`host/storage.rs` registers scoped read, write, delete, and drop; it does
not register scan
(`programs/crates/layerx-programs-runtime/src/host/storage.rs`).

Sources:

- `programs/crates/layerx-programs-runtime/src/host/scan.rs`
- `programs/crates/layerx-programs-runtime/src/host/storage.rs`
- `programs/crates/layerx-programs-runtime/src/storage/scan.rs`
- `programs/crates/layerx-programs-runtime/tests/storage_scan.rs`
- `programs/crates/layerx-programs-runtime/src/abi/storage_ops.rs`

---

## Host call

`storage_scan_scoped` is `func_wrap`ped with nine `i32` arguments and an
`i32` return (`programs/crates/layerx-programs-runtime/src/host/scan.rs`).

| Argument | Host decode |
| --- | --- |
| `raw_selector` | `StorageSelector::try_from`: `1` principal, `2` shared; any other value is `AbiError::InvalidEncoding` (`programs/crates/layerx-programs-runtime/src/abi/storage_ops.rs`, `programs/crates/layerx-programs-runtime/src/host/scan.rs`) |
| `prefix_pointer`, `prefix_length` | `read_guest` with ceiling `MAX_STORAGE_KEY_BYTES` `256` (`programs/crates/layerx-programs-runtime/src/host/scan.rs`, `programs/crates/layerx-programs-runtime/src/storage/mod.rs`) |
| `cursor_pointer`, `cursor_length` | `read_guest` with ceiling `MAX_STORAGE_SCAN_CURSOR_BYTES` (`programs/crates/layerx-programs-runtime/src/host/scan.rs`, `programs/crates/layerx-programs-runtime/src/storage/scan.rs`) |
| `max_entries` | `nonnegative` then `u32::try_from`, then `ScanLimits::new` (`programs/crates/layerx-programs-runtime/src/host/scan.rs`) |
| `max_bytes` | `nonnegative` then `u32::try_from`, then `ScanLimits::new` (`programs/crates/layerx-programs-runtime/src/host/scan.rs`) |
| `output_pointer`, `output_capacity` | `validate_output` before any prefix or cursor read (`programs/crates/layerx-programs-runtime/src/host/scan.rs`, `programs/crates/layerx-programs-runtime/src/host/memory.rs`) |

The host takes a namespace selector, a bounded key prefix, and a cursor.
It also decodes `max_entries`, `max_bytes`, `output_pointer`, and `output_capacity`
(`programs/crates/layerx-programs-runtime/src/host/scan.rs`).

Selector `1` binds `CapabilityKey::StorageRead` and the principal
namespace. Selector `2` binds `CapabilityKey::SharedStorageRead` and the
shared namespace
(`programs/crates/layerx-programs-runtime/src/abi/storage_ops.rs`).

`validate_output` maps a negative capacity or a negative pointer (when
capacity is nonzero) to `STATUS_INVALID`. A range that overflows or
extends past guest memory is `STATUS_BOUNDS`
(`programs/crates/layerx-programs-runtime/src/host/memory.rs`,
`programs/crates/layerx-programs-runtime/src/host/mod.rs`).

`read_guest` maps a negative pointer or length to `STATUS_INVALID`, a
length above the declared maximum or a memory read failure to
`STATUS_BOUNDS`
(`programs/crates/layerx-programs-runtime/src/host/memory.rs`).

After decode, the host calls `storage_scan_preview`, then
`encode_for_guest`. If the encoded page is longer than
`output.capacity()`, it returns `STATUS_BOUNDS` without charging and
without writing. Otherwise it charges, writes the encoded page, and
returns the encoded length as `i32`
(`programs/crates/layerx-programs-runtime/src/host/scan.rs`).

### Output buffer and refusal sentinel

`output.write` is the only store into the output range
(`programs/crates/layerx-programs-runtime/src/host/scan.rs`,
`programs/crates/layerx-programs-runtime/src/host/memory.rs`).
Every earlier `return` leaves those guest bytes unchanged.

`scan_status_guest` seeds `keep` at memory 128, stores the host
`i32` at memory 64, then copies four bytes from its `sentinel_pointer`
argument to memory 68
(`programs/crates/layerx-programs-runtime/tests/storage_scan.rs`).
Each status fixture asserts `storage_read_bytes == 0` and response bytes
`expected_status.to_le_bytes()` followed by `keep`
(`programs/crates/layerx-programs-runtime/tests/storage_scan.rs`).

---

## Entry ceiling and page byte ceiling

`ScanLimits::new` refuses `max_entries == 0`, `max_entries > 64`,
`max_bytes < 5`, or `max_bytes > 67_126_228`
(`programs/crates/layerx-programs-runtime/src/storage/scan.rs`).

| Bound | Constant | Value |
| --- | --- | ---: |
| Entry ceiling | `MAX_STORAGE_SCAN_ENTRIES` | 64 |
| Minimum complete page bytes | `MIN_STORAGE_SCAN_PAGE_BYTES` | 5 |
| Page byte ceiling | `MAX_STORAGE_SCAN_BYTES` | 67_126_228 |

`scan_cells` enforces the two ceilings independently. Hitting
`max_entries` before the next match returns the current entries with a
cursor. Before admitting a match, it calculates the complete encoded page
size, including a continuation cursor when another match follows. If that
size exceeds `max_bytes`, an empty page is `ScanCeilingExceeded`;
otherwise the previously accepted entries are returned with their cursor
(`programs/crates/layerx-programs-runtime/src/storage/scan.rs`).

The host test with three `(a,a)/(b,b)/(c,c)` pairs and guest limits
`(64, 101)` returns the 29-byte terminal page. After seeding `(d,d)`,
the same `(64, 101)` limits return the 101-byte two-entry continuation
page. `(64, 100)` returns the 93-byte one-entry continuation page
(`programs/crates/layerx-programs-runtime/tests/storage_scan.rs`).

Each call declares both an entry ceiling and a byte ceiling through
`ScanLimits`. The ABI passes both bounds to the storage scan
(`programs/crates/layerx-programs-runtime/src/abi/storage_ops.rs`);
the runtime enforces them independently as above.

---

## Page encoding

`StorageScan::encode_for_guest`
(`programs/crates/layerx-programs-runtime/src/storage/scan.rs`)
and `expected_page`
(`programs/crates/layerx-programs-runtime/tests/storage_scan.rs`)
use the same field order, big-endian lengths:

| Field | Width | Notes |
| --- | ---: | --- |
| entry count | u16 | number of key/value pairs |
| per entry: key length | u16 | |
| per entry: key | key length | |
| per entry: value length | u32 | |
| per entry: value | value length | |
| has-cursor | u8 | `1` iff cursor bytes are nonempty, else `0` |
| cursor length | u16 | `0` when has-cursor is `0` |
| cursor | cursor length | continuation token, or empty |

`metered_bytes` is that encoding's length
(`programs/crates/layerx-programs-runtime/src/storage/scan.rs`).
`charge_storage_scan` charges it to the storage-read class
(`programs/crates/layerx-programs-runtime/src/abi/storage_ops.rs`).

| Fixture | Bytes | Encoding |
| --- | ---: | --- |
| Empty prefix, empty store, limits `(1, 5)` | 5 | `[0, 0, 0, 0, 0]` (`programs/crates/layerx-programs-runtime/tests/storage_scan.rs`) |
| One `(a, b)` pair, no cursor | 13 | `[0, 1, 0, 1, a, 0, 0, 0, 1, b, 0, 0, 0]` (`programs/crates/layerx-programs-runtime/tests/storage_scan.rs`) |
| Terminal three-entry page `(a,a)/(b,b)/(c,c)`, no cursor | 29 | `expected_page(..., None)` (`programs/crates/layerx-programs-runtime/tests/storage_scan.rs`) |
| Two-entry continuation `(a,a)/(b,b)` plus cursor after `b` | 101 | `expected_page(..., Some(expected_cursor(..., 64, 101, b"b")))` (`programs/crates/layerx-programs-runtime/tests/storage_scan.rs`) |
| One-entry continuation `(a,a)` plus cursor after `a` at `max_bytes` 100 | 93 | `expected_page(..., Some(expected_cursor(..., 64, 100, b"a")))` (`programs/crates/layerx-programs-runtime/tests/storage_scan.rs`) |

---

## Continuation cursor encoding

`ScanCursor::encode`
(`programs/crates/layerx-programs-runtime/src/storage/scan.rs`)
and `expected_cursor`
(`programs/crates/layerx-programs-runtime/tests/storage_scan.rs`)
use this field order, big-endian integers:

| Field | Width | Notes |
| --- | ---: | --- |
| version | u8 | `CURSOR_VERSION` `1` (`programs/crates/layerx-programs-runtime/src/storage/scan.rs`) |
| namespace length | u8 | principal-scoped canonical length is `65` |
| namespace | namespace length | `StorageNamespace::canonical_bytes` (`programs/crates/layerx-programs-runtime/src/storage/namespace.rs`) |
| prefix length | u16 | |
| prefix | prefix length | |
| `max_entries` | u32 | the issuing call's `ScanLimits` |
| `max_bytes` | u32 | the issuing call's `ScanLimits` |
| after length | u16 | |
| after | after length | last key included on the issuing page |

Principal-scoped canonical bytes are program (32), scope tag `0`,
principal (32)
(`programs/crates/layerx-programs-runtime/src/storage/namespace.rs`).
`expected_cursor` therefore starts `[1, 65]`, then those 65 bytes, then
prefix length `0` for an empty prefix
(`programs/crates/layerx-programs-runtime/tests/storage_scan.rs`).

`MAX_STORAGE_SCAN_CURSOR_BYTES` is `1 + 1 + 65 + 2 + 256 + 4 + 4 + 2 + 256`
(`programs/crates/layerx-programs-runtime/src/storage/scan.rs`).

An empty guest cursor starts a scan. A nonempty cursor is
`ScanCursor::decode_for` against this call's namespace, prefix, and
limits (`programs/crates/layerx-programs-runtime/src/storage/scan.rs`).

---

## Refusals

Host status constants
(`programs/crates/layerx-programs-runtime/src/host/mod.rs`):

| Constant | Value |
| --- | ---: |
| `STATUS_DENIED` | -1 |
| `STATUS_INVALID` | -2 |
| `STATUS_BOUNDS` | -3 |
| `STATUS_METER` | -4 |

`error_status` maps scan-related `AbiError` values
(`programs/crates/layerx-programs-runtime/src/host/mod.rs`):

| Host status | Causes | Metered | Output |
| --- | --- | --- | --- |
| -1 `STATUS_DENIED` | missing matching read grant; `AccessDeclaration` | no | sentinel kept |
| -2 `STATUS_INVALID` | `InvalidEncoding` (selector not `1` or `2`); `InvalidScanCursor`; `InvalidScanLimits`; negative pointer/length/`max_entries`/`max_bytes`/`output_capacity` via `nonnegative` | no | sentinel kept |
| -3 `STATUS_BOUNDS` | prefix or cursor longer than the `read_guest` ceiling; output range past memory; encoded page longer than `output_capacity`; `ScanCeilingExceeded` and other `AbiError::Storage(_)` not listed as invalid | no, when returned before `charge_storage_scan` | sentinel kept |
| -4 `STATUS_METER` | `charge_storage_scan` meter refusal | `finish_resource_failure` reports `storage_read_bytes == 0` (`programs/crates/layerx-programs-runtime/src/storage/scan.rs`) | not written; host returns before `output.write` (`programs/crates/layerx-programs-runtime/src/host/scan.rs`) |

Host fixtures that assert unmetered refusal plus unchanged `keep`:

| Case | Status | Test |
| --- | ---: | --- |
| No `StorageRead` grant | -1 | `programs/crates/layerx-programs-runtime/tests/storage_scan.rs` |
| Selector `2` with only principal read | -1 | `programs/crates/layerx-programs-runtime/tests/storage_scan.rs` |
| Output capacity `12` for a 13-byte page | -3 | `programs/crates/layerx-programs-runtime/tests/storage_scan.rs` |
| Output pointer `65535` with capacity `13` | -3 | `programs/crates/layerx-programs-runtime/tests/storage_scan.rs` |
| Prefix `a` with a cursor issued for empty prefix | -2 | `programs/crates/layerx-programs-runtime/tests/storage_scan.rs` |
| Cursor issued for another program's principal namespace | -2 | `programs/crates/layerx-programs-runtime/tests/storage_scan.rs` |
| Cursor with one trailing byte appended | -2 | `programs/crates/layerx-programs-runtime/tests/storage_scan.rs` |

`candidate_scan_refusals_are_unmetered_and_leave_output_sentinel_unchanged`
repeats the grant, selector, capacity, and pointer cases with
`response_write` of the `keep` data bytes and `storage_read_bytes == 0`
(`programs/crates/layerx-programs-runtime/tests/storage_scan.rs`).

`charge_storage_scan` charges the storage-read meter per encoded byte
returned (`programs/crates/layerx-programs-runtime/src/abi/storage_ops.rs`).
The host meters `encode_for_guest` length, including the 5-byte empty page, and only
after the output-capacity check
(`programs/crates/layerx-programs-runtime/src/host/scan.rs`,
`programs/crates/layerx-programs-runtime/tests/storage_scan.rs`).
Refusals above are unmetered.

---

## Cross-scope prefix and limit cursor reuse

`decode_for` returns `InvalidScanCursor` when the cursor namespace,
prefix, or `ScanLimits` differ from the resuming call, when trailing
bytes remain, when `after` is empty, when `after` exceeds
`MAX_STORAGE_KEY_BYTES`, or when `after` does not start with the prefix
(`programs/crates/layerx-programs-runtime/src/storage/scan.rs`).

`candidate_scan_rejects_cross_scope_prefix_and_limit_cursor_reuse`
issues a cursor with empty prefix and `max_entries == 1`, then refuses:

- prefix `a` with that cursor
- empty prefix with `max_entries == 2` and the same cursor
- selector `2` (shared, with `SharedStorageRead`) with that principal
  cursor

Each case meters `storage_read_bytes == 0` and leaves `keep`
(`programs/crates/layerx-programs-runtime/tests/storage_scan.rs`).

---

## Corrupted cursor

`candidate_scan_paginates_across_activities_and_is_insertion_order_independent`
overwrites four cursor bytes with `keep` and runs `scan_status_guest`.
The host returns `-2`, meters `0`, leaves `keep` in the output, and
leaves storage equal to the pre-call clone
(`programs/crates/layerx-programs-runtime/tests/storage_scan.rs`).
The uncorrupted cursor then resumes as the 13-byte `b` page
(`programs/crates/layerx-programs-runtime/tests/storage_scan.rs`).

Appending one byte to a valid cursor is `-2` as in the status fixtures
(`programs/crates/layerx-programs-runtime/tests/storage_scan.rs`).

---

## Same-activity writes and rollback

`Storage::write` inserts into the same `cells` map that `scan` reads
(`programs/crates/layerx-programs-runtime/src/storage/mod.rs`).
`Abi` owns that storage snapshot so a trap discards every write
(`programs/crates/layerx-programs-runtime/src/abi/mod.rs`).

`write_then_scan_guest` writes `(a, b)` then scans. The success path
returns the 13-byte single-entry page and commits `a -> b`
(`programs/crates/layerx-programs-runtime/tests/storage_scan.rs`).
The trap path (`unreachable` after scan, output capacity `12`) is
`V2ActivityOutcome::Failure` with `RefusalClass::RuntimeFault`, no
response, and storage equal to the pre-call clone
(`programs/crates/layerx-programs-runtime/tests/storage_scan.rs`).

---

## Determinism across insertion order

`scan_cells` starts an ordered `BTreeMap` range at the namespace and prefix,
or just after the cursor's last key. It stops when the namespace or prefix
no longer matches. Map addresses sort program-major, scope-tagged, then key
(`programs/crates/layerx-programs-runtime/src/storage/scan.rs`).

Seeding `(b,b)` then `(a,a)` versus `(a,a)` then `(b,b)` produces equal
first-page responses, equal usage, and equal canonical evidence
(`programs/crates/layerx-programs-runtime/tests/storage_scan.rs`).
Scan output is independent of insertion order because `scan_cells` visits
keys in their `BTreeMap` order
(`programs/crates/layerx-programs-runtime/src/storage/scan.rs`).

[Home](../index.md)
