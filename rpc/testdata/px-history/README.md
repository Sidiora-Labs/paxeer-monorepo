The row recording is copied from `apps/web/e2e/fixtures/px-get-unified-history.json`,
which `unified_history_answer_matches_the_web_fixture` in the gateway history
module records from the real Rust indexer store and API. `responses.json`
projects its original indexer fields, asset records and readiness document into
HTTP responses; the pages retain the original row positions and exact amounts.
The registry record comes from that producer's `indexer()` fixture. The empty
holder envelope is the store's recorded empty-list result asserted by
`px_list_holder_records_pages_the_indexer_holder_route_to_the_end`.

Tests replay these documents over loopback HTTP. Native registry records and
bindings are produced separately by the real node keepers and committed before
RPC reads. Account creation emits no transfer, so those accounts replay the
canonical empty history page. No executable, external endpoint or credential
is needed to replay the records.

The `/mixed/v1/history/` entries are a separate mixed-source recording for
`TestPxNodeRoot_History_ExplorerImportedRows`. Producer revision:
`9e7910e691f796275d37534446e4fb861ccbabd2`. The generator below calls the real
`layerx_indexer::store::Store::commit`, `Store::import_explorer` and
`layerx_indexer::api::route` with GET history requests. Response bodies are
serialized directly from the API; no successful response rows are assembled or
rewritten. `/mixed` is only the replay selector and is not part of the producer
request path.

After integrating task 18.1633.1, these recordings were regenerated with
canonical transaction hashes: the generator hashes its transaction labels
with SHA-256 before committing or importing them. The consumer derives
`tx_hash` from the recorded `tx_id` and retains the producer's `final_basis`
for both indexed and imported rows.

The native account IDs are derived from the same deterministic binding and
asset registration as `pxHistoryCommitNative`: Ed25519 seed `[0x45; 32]`,
asset salt `[0x44, 0, ..., 0]`. Indexed IDs 2 and 10 and imported keys 2 and 10
are self-transfer legs at position `(10, 0, 0)`. Indexed ID 1 belongs to the
committed main account at the same position with ordinal 1; ID 3 belongs to the
owned asset account at height 11. Filler transfers reserve indexed IDs 4–9 in
another account. Later indexed/imported rows force traversal through heights
11, 12 and 13. This exercises source ordering, numeric ordering, ordinal
ordering, position-boundary expansion and native account merging independently.

To regenerate, extract the Rust block below into `src/main.rs` of a temporary
Cargo binary outside the checkout. Give that binary dependencies
`layerx-indexer = { path = "<checkout>/services/platform/hosted/indexer" }` and
`serde_json = "=1.0.151"` and `sha2 = "=0.10.9"`, and run it with `cargo run --manifest-path
<temporary>/Cargo.toml`. Its stdout is an array of recordings. Append that
array to the existing `responses.json`, replacing only prior entries whose
paths start with `/mixed/`; retain every other recording. Generation is an
implementation step, separate from the Go qualification gates. Malformed-ID
subtests mutate one ID of these recordings at replay time and never create
successful history rows.

```rust
use layerx_indexer::{api, store::{Store, TransferRow, Unit}};
use serde_json::{json, Value};
use sha2::{Digest, Sha256};

const EVM: &str = "0xeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee";
const MAIN: &str = "83d594bd43c513bef84f77c2e0509612b22f61cf14ff17a3bf017ce46b0560fe";
const ASSET: &str = "14972cb394ac38e2630607f64480988168b3cac8bffaebd8e8bb4b8cbba6dddb";
const OTHER: &str = "0xffffffffffffffffffffffffffffffffffffffff";

fn leg(height: u64, ordinal: u64, account: &str, direction: &'static str,
       tx: &str, counterparty: &str) -> TransferRow {
    TransferRow {
        height_or_seq: height, tx_index: 0, ordinal,
        kind: "native_transfer".into(), direction, account: account.into(),
        counterparty: Some(counterparty.into()), asset: "evm:native".into(),
        amount: "7".into(), tx_id: format!("0x{:x}", Sha256::digest(tx.as_bytes())),
        decoded: json!({"producer": "mixed-history"}),
    }
}

fn main() -> Result<(), Box<dyn std::error::Error>> {
    let store = Store::open_in_memory()?;
    let mut rows = vec![
        leg(10, 1, MAIN, "in", "native-main-first", OTHER),
        leg(10, 0, EVM, "out", "indexed-self", EVM),
        leg(11, 0, ASSET, "in", "native-asset", OTHER),
    ];
    for id in 4..=9 {
        rows.push(leg(10, 0, OTHER, "out", &format!("filler-{id}"), EVM));
    }
    rows.extend([
        leg(10, 0, EVM, "in", "indexed-self", EVM),
        leg(11, 0, EVM, "in", "indexed-next", OTHER),
        leg(13, 0, MAIN, "in", "native-main-last", OTHER),
    ]);
    store.commit(&Unit {
        chain: "paxeer".into(), position: 13, boundary: 13,
        hash: "mixed-13".into(), parent: "mixed-12".into(), link: "mixed-13".into(),
        transfers: rows, ..Unit::default()
    }, 0)?;
    store.import_explorer(EVM, &[
        (2, leg(10, 0, EVM, "out", "imported-self", EVM)),
        (10, leg(10, 0, EVM, "in", "imported-self", EVM)),
        (11, leg(12, 0, EVM, "in", "imported-next", OTHER)),
    ], &[], true)?;
    let mut recordings = Vec::<Value>::new();
    for account in [EVM, MAIN, ASSET] {
        let path = format!("/v1/history/{account}");
        let mut queries = vec!["limit=1".to_owned(), "limit=50".to_owned()];
        for height in 10..=13 {
            queries.push(format!("cursor={height:016x}0000000000000000&limit=1"));
        }
        for query in queries {
            let response = api::route(&store, "GET", &format!("{path}?{query}"));
            assert_eq!(response.status, 200);
            recordings.push(json!({
                "path": format!("/mixed{path}"), "query": query,
                "status": response.status, "body": serde_json::from_str::<Value>(&response.body)?,
            }));
        }
    }
    println!("{}", serde_json::to_string_pretty(&recordings)?);
    Ok(())
}
```
