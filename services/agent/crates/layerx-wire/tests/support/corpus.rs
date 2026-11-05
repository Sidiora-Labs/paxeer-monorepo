use layerx_types::vectors::{Corpus, CorpusError};
use std::path::Path;
use std::process::Command;
use std::sync::atomic::{AtomicU64, Ordering};

pub fn load(root: &Path) -> Result<Corpus, CorpusError> {
    static NEXT_DIRECTORY: AtomicU64 = AtomicU64::new(0);
    let directory = Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("tests/support")
        .join(format!(
            "replay-corpus-{}-{}",
            std::process::id(),
            NEXT_DIRECTORY.fetch_add(1, Ordering::Relaxed)
        ));
    std::fs::create_dir(&directory).unwrap_or_else(|error| panic!("{error}"));
    let codec = directory.join("tests/vectors/codec");
    std::fs::create_dir_all(&codec).unwrap_or_else(|error| panic!("{error}"));
    for relative in [
        "tests/vectors/codec/valid.lxv",
        "tests/vectors/codec/adversarial.lxv",
        "tests/vectors/qualification_replay_10m.digest",
    ] {
        std::fs::copy(root.join(relative), directory.join(relative))
            .unwrap_or_else(|error| panic!("{error}"));
    }
    let generated = Command::new("python3")
        .arg("-B")
        .arg("-c")
        .arg(concat!(
            "import runpy, sys; from pathlib import Path; ",
            "generate = runpy.run_path(sys.argv[1])['main']; ",
            "generate.__globals__['OUTPUT'] = Path(sys.argv[2]); generate()"
        ))
        .arg(root.join("tests/vectors/codec/generate.py"))
        .arg(codec.join("replay_corpus.lxb"))
        .output()
        .unwrap_or_else(|error| panic!("{error}"));
    let corpus = Corpus::load(&directory);
    std::fs::remove_dir_all(&directory).unwrap_or_else(|error| panic!("{error}"));
    assert!(
        generated.status.success(),
        "codec corpus generation failed: {}\n{}",
        String::from_utf8_lossy(&generated.stdout),
        String::from_utf8_lossy(&generated.stderr)
    );
    corpus
}
