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
