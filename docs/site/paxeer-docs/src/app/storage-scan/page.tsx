import Link from 'next/link'
import { DocsLayout } from '@/components/DocsLayout'
import { metadataFor } from '@/lib/seo'

export const metadata = metadataFor('/storage-scan')

export default function StorageScanPage() {
  return <DocsLayout>
    <div className="page-header"><h1 className="page-title">Bounded Storage Scans</h1><p className="page-description">Iterate Paxeer X program state with deterministic ordering, explicit page budgets, scope-bound continuation cursors, and metered canonical results.</p></div>
    <h2>Read a namespace in bounded pages</h2>
    <p>The guest host function <code>layerx_v2.storage_scan_scoped</code> returns ordered key/value pairs matching a byte prefix. Every call declares an entry ceiling and a complete encoded-page byte ceiling. A portable continuation token resumes after the last returned key across activities without retaining a host iterator or allocator handle.</p>
    <p>This function belongs to the frozen ABI v2 namespace, <code>layerx_v2</code>. The runtime declares its nine i32 parameters and i32 result in the versioned ABI manifest. A program must declare an ABI version that includes this namespace.</p>
    <p>Use scans for bounded indexes, application inventories, and paginated state processing. Design keys with stable prefixes and choose page sizes that fit the call&apos;s storage-read budget and guest output buffer. Returning scan data as an activity response also consumes the response output budget. The function scans one authorized namespace.</p>
    <h2>Host-call contract</h2>
    <pre><code>{`storage_scan_scoped(
  raw_selector: i32,
  prefix_pointer: i32, prefix_length: i32,
  cursor_pointer: i32, cursor_length: i32,
  max_entries: i32, max_bytes: i32,
  output_pointer: i32, output_capacity: i32
) -> i32`}</code></pre>
    <table><thead><tr><th>Input</th><th>Contract</th></tr></thead><tbody>
      <tr><td>Selector 1</td><td>Principal-scoped namespace; requires <code>StorageRead</code> and the matching access declaration.</td></tr>
      <tr><td>Selector 2</td><td>Shared program namespace; requires <code>SharedStorageRead</code> and the matching access declaration.</td></tr>
      <tr><td>Prefix</td><td>At most 256 bytes. An empty prefix selects all keys within the namespace.</td></tr>
      <tr><td>Cursor</td><td>Empty starts a scan. A nonempty token must match the same namespace, prefix, and limits.</td></tr>
      <tr><td>Entry ceiling</td><td>Between 1 and 64 entries per page.</td></tr>
      <tr><td>Page byte ceiling</td><td>Between 5 and 67,126,228 complete encoded bytes.</td></tr>
      <tr><td>Output range</td><td>A valid guest-memory buffer large enough for the encoded result.</td></tr>
      <tr><td>Return value</td><td>Nonnegative encoded byte length on success; a negative host status on refusal.</td></tr>
    </tbody></table>
    <p>The host validates the output range before reading the prefix and cursor. It previews and encodes the page, checks output capacity, charges the storage-read meter, then copies bytes to guest memory. A negative length, invalid selector, out-of-memory range, or denied read is rejected before output is written.</p>
    <h2>Decode the canonical page</h2>
    <p>All lengths below are big-endian. Parse only the number of bytes returned by the host, validate each length against the remaining buffer, and require the cursor fields to agree.</p>
    <table><thead><tr><th>Field</th><th>Width</th></tr></thead><tbody>
      <tr><td>Entry count</td><td>u16</td></tr>
      <tr><td>For each entry: key length, key</td><td>u16 followed by key bytes</td></tr>
      <tr><td>For each entry: value length, value</td><td>u32 followed by value bytes</td></tr>
      <tr><td>Has-cursor flag</td><td>u8: 0 or 1</td></tr>
      <tr><td>Cursor length, cursor</td><td>u16 followed by cursor bytes; zero length when no cursor</td></tr>
    </tbody></table>
    <pre><code>{`# Empty terminal page: 5 bytes
00 00 00 00 00

# One entry, key "a", value "b", no continuation: 13 bytes
00 01 00 01 61 00 00 00 01 62 00 00 00`}</code></pre>
    <p>The complete encoding consumes the storage-read budget, including counts, lengths, cursor bytes, and the five-byte empty page. Allocate room for continuation metadata as well as the key/value content. A page with one short entry and a continuation can be larger than a terminal page with several short entries.</p>
    <h2>Resume without changing scan identity</h2>
    <p>The cursor binds version, canonical namespace, prefix, entry ceiling, byte ceiling, and the last returned key. Treat its bytes as an opaque continuation token in application code. Reuse them exactly with the same selector, program, principal where applicable, prefix, and both limits.</p>
    <p>Changing a limit to increase page size requires starting a new scan with an empty cursor. A principal cursor cannot resume in shared scope or under another program. Corrupted bytes, trailing bytes, empty last-key fields, and keys inconsistent with the prefix are invalid. The maximum encoded cursor size is 591 bytes.</p>
    <h3>Pagination workflow</h3>
    <ol><li>Choose the authorized scope and stable prefix. Select both page ceilings and allocate a guest output buffer.</li><li>Call with an empty cursor and check the signed return status before decoding memory.</li><li>Decode the complete page, process its entries, and retain the exact continuation bytes.</li><li>When a cursor is present, invoke again with unchanged scan identity and limits.</li><li>Stop when the page has no continuation cursor.</li></ol>
    <p>Each resumed activity reads its current authorized state. The cursor identifies a position and scan contract; it does not claim that all pages share a frozen historical state. For an application that needs a consistent export, bind the surrounding workflow to its own verified state and mutation policy.</p>
    <h2>Two independent page bounds</h2>
    <p>The scan stops before exceeding the entry count. It also checks the complete encoded size when adding each matching entry. If an entry would exceed the page-byte limit, it is removed and the preceding entries return with a cursor. If no entry can fit, the scan refuses with a bounds status rather than returning an ambiguous empty continuation.</p>
    <p>In a principal-scoped namespace with an empty prefix, three pairs <code>(a,a)</code>, <code>(b,b)</code>, and <code>(c,c)</code> have a 29-byte terminal encoding. With a fourth pair and limits <code>(64, 101)</code>, the first page contains two entries plus continuation and occupies 101 bytes. A 100-byte limit yields one entry plus continuation at 93 bytes. The byte ceiling must allow continuation metadata while the page is assembled.</p>
    <h2>Refusals and unchanged output</h2>
    <table><thead><tr><th>Status</th><th>Meaning</th><th>Response</th></tr></thead><tbody>
      <tr><td><code>-1</code> denied</td><td>Missing matching read authority or access declaration.</td><td>Supply the correct admitted grant and scope.</td></tr>
      <tr><td><code>-2</code> invalid</td><td>Malformed input, invalid selector or limits, or a mismatched cursor.</td><td>Correct the request; restart pagination if identity changes.</td></tr>
      <tr><td><code>-3</code> bounds</td><td>Oversized prefix/cursor, invalid memory range, insufficient output capacity, or no entry fitting the page ceiling.</td><td>Correct memory bounds or start a new scan with suitable limits.</td></tr>
      <tr><td><code>-4</code> meter</td><td>Insufficient admitted storage-read budget for the encoded page.</td><td>Increase the activity budget within admitted ceilings.</td></tr>
    </tbody></table>
    <p>Refusals before charging leave storage-read usage unchanged. A refused charge produces no partial page. The host writes output only after validation and metering succeed, so output bytes remain unchanged on these refusal paths. Never interpret preexisting buffer contents as a fresh page after a negative return.</p>
    <h2>Determinism, writes, and rollback</h2>
    <p>Storage is ordered by program, scope, and key. Within the chosen namespace, entries return in canonical key order independent of insertion order. Equivalent state and scan inputs produce equivalent pages, usage, and evidence.</p>
    <p>A scan sees writes already staged in the same activity because both use the activity&apos;s storage snapshot. A successful activity commits those writes. A later trap discards the staged mutation and returns the runtime-fault outcome without a successful response. Do not treat a page observed inside a failed guest execution as committed state.</p>
    <p>Read <Link href="/programs">Programs</Link> for call budgets, response transport, and receipt verification, or <Link href="/program-workspace">the developer workspace</Link> for conformance and ABI checks.</p>
    <div className="source-note"><strong>Technical references:</strong> programs/crates/layerx-programs-runtime/src/host/scan.rs, src/storage/scan.rs, src/abi/storage_ops.rs, src/abi/manifest.rs, and tests/storage_scan.rs within that runtime crate.</div>
  </DocsLayout>
}
