use std::fs;
use std::path::PathBuf;

#[test]
fn gateway_tests_name_no_bridge_withdraw_request() -> Result<(), Box<dyn std::error::Error>> {
    let tests = PathBuf::from(env!("CARGO_MANIFEST_DIR")).join("tests");
    let forbidden = ["Bridge", "WithdrawRequest"].concat();
    let mut directories = vec![tests];
    let mut checked = 0;
    while let Some(directory) = directories.pop() {
        for entry in fs::read_dir(directory)? {
            let entry = entry?;
            let path = entry.path();
            if entry.file_type()?.is_dir() {
                directories.push(path);
            } else if path.extension().is_some_and(|extension| extension == "rs") {
                let source = fs::read_to_string(&path)?;
                assert!(
                    !source.contains(&forbidden),
                    "gateway test {} names the retired bridge withdrawal request",
                    path.display()
                );
                checked += 1;
            }
        }
    }
    assert!(checked > 0, "gateway test sources must be checked");
    Ok(())
}
