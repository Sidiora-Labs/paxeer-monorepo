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

// Reset all files belonging to one temporary journal when a scenario starts
// a distinct history. Removing only the log leaves its anti-rollback seal.
pub fn remove(path: &std::path::Path) -> std::io::Result<()> {
    let parent = path.parent().ok_or(std::io::ErrorKind::InvalidInput)?;
    let name = path
        .file_name()
        .ok_or(std::io::ErrorKind::InvalidInput)?
        .to_string_lossy();
    for item in std::fs::read_dir(parent)? {
        let item = item?;
        let file = item.file_name();
        let file = file.to_string_lossy();
        if file == name
            || file.strip_prefix(name.as_ref()).is_some_and(|suffix| {
                matches!(suffix, ".lock" | ".key" | ".head") || suffix.starts_with(".replay.")
            })
        {
            std::fs::remove_file(item.path())?;
        }
    }
    Ok(())
}
