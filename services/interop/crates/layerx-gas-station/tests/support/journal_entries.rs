// Inspect entries written by the production journal without treating its
// authenticated envelope as an Entry. Production reopen checks the MAC chain.
use layerx_gas_station::journal::Entry;

pub fn entry(line: &str) -> Result<Entry, serde_json::Error> {
    #[derive(serde::Deserialize)]
    #[serde(deny_unknown_fields)]
    struct Record {
        sequence: u64,
        previous: [u8; 32],
        payload: String,
        tag: [u8; 32],
    }
    let record: Record = serde_json::from_str(line)?;
    assert!(record.sequence > 0);
    assert_ne!(record.tag, [0; 32]);
    if record.sequence == 1 {
        assert_eq!(record.previous, [0; 32]);
    }
    serde_json::from_str(&record.payload)
}
