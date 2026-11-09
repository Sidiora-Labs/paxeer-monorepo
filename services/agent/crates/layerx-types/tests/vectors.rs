use std::fs;
use std::path::{Path, PathBuf};
use std::process::Command;

use layerx_types::vectors::checkpoint::{
    checkpoint_vector_path, load_checkpoint_vectors, CHECKPOINT_VECTOR_CASES,
};
use layerx_types::vectors::{
    coverage_report, Corpus, CorpusError, DERIVED_PROTOCOL_VERSION, TYPE_PROTOCOL_VERSIONS,
};

fn repository_root() -> PathBuf {
    Path::new(env!("CARGO_MANIFEST_DIR")).join("../../../..")
}

fn copy_input(target: &Path, relative: &Path) {
    let destination = target.join(relative);
    let Some(parent) = destination.parent() else {
        panic!("corpus input has no parent: {}", destination.display());
    };
    assert!(fs::create_dir_all(parent).is_ok());
    assert!(fs::copy(repository_root().join(relative), &destination).is_ok());
}

fn copy_corpora(target: &Path) {
    for relative in [
        "tests/vectors/codec/valid.lxv",
        "tests/vectors/codec/adversarial.lxv",
        "tests/vectors/qualification_replay_10m.digest",
        "tests/vectors/codec/generate.py",
        "scripts/replica/lxp_replay_fixture.py",
    ] {
        copy_input(target, Path::new(relative));
    }
    let Ok(headers) = fs::read_dir(repository_root().join("include/layerx")) else {
        panic!("kernel activity declarations unreadable");
    };
    for header in headers {
        let Ok(header) = header else {
            panic!("kernel activity declaration entry unreadable");
        };
        if header
            .path()
            .extension()
            .is_some_and(|extension| extension == "h")
        {
            copy_input(
                target,
                &Path::new("include/layerx").join(header.file_name()),
            );
        }
    }
    let Ok(output) = Command::new("python3")
        .arg(target.join("tests/vectors/codec/generate.py"))
        .env("PYTHONDONTWRITEBYTECODE", "1")
        .output()
    else {
        panic!("codec replay producer could not start");
    };
    assert!(
        output.status.success(),
        "codec replay producer failed: {}",
        String::from_utf8_lossy(&output.stderr)
    );
}

fn temporary_repository(name: &str) -> PathBuf {
    let path = std::env::temp_dir().join(format!("layerx-vectors-{name}-{}", std::process::id()));
    let _ = fs::remove_dir_all(&path);
    copy_corpora(&path);
    path
}

#[test]
fn repository_corpora_are_loaded_and_fully_accounted() {
    let Ok(corpus) = Corpus::load(&repository_root()) else {
        panic!("published corpora failed to load");
    };
    assert_eq!(corpus.valid_codec.len(), 4);
    assert_eq!(corpus.adversarial_codec.len(), 7);
    assert_eq!(corpus.replay.protocol_version, DERIVED_PROTOCOL_VERSION);
    let declared_count = corpus.replay.activity_count;
    assert!(declared_count > 0);
    assert_eq!(corpus.replay.activity_types.len(), declared_count);
    assert_eq!(corpus.replay.canonical_activities.len(), declared_count);
    assert_eq!(corpus.replay.expected_receipts.len(), declared_count);
    assert_eq!(corpus.qualification_digests.len(), 9);
    assert!(TYPE_PROTOCOL_VERSIONS
        .iter()
        .all(|(_, version)| *version == DERIVED_PROTOCOL_VERSION));

    let Ok(report) = coverage_report(&corpus) else {
        panic!("published vector class was silently unused");
    };
    assert_eq!(report.exercised.len(), 10);
    assert!(report.unused.is_empty());
    assert_eq!(report.render().lines().count(), 10);
}

#[test]
fn checkpoint_vectors_load_without_the_settlement_document() {
    let repository = std::env::temp_dir().join(format!(
        "layerx-vectors-checkpoint-without-settlement-{}",
        std::process::id()
    ));
    assert!(fs::create_dir(&repository).is_ok());
    for case_name in CHECKPOINT_VECTOR_CASES {
        copy_input(
            &repository,
            &checkpoint_vector_path(Path::new(""), case_name),
        );
    }
    let settlement_document = repository.join("contracts/config/checkpoint-settlement.json");
    assert!(!settlement_document.exists());
    let expected = load_checkpoint_vectors(&repository_root());
    let loaded = load_checkpoint_vectors(&repository);
    assert!(fs::remove_dir_all(&repository).is_ok());

    let Ok(expected) = expected else {
        panic!("published checkpoint vectors failed to load");
    };
    let Ok(loaded) = loaded else {
        panic!("checkpoint vectors failed to load without the settlement document");
    };
    assert_eq!(loaded.len(), CHECKPOINT_VECTOR_CASES.len());
    assert_eq!(
        loaded
            .iter()
            .map(|vector| vector.case_name.as_str())
            .collect::<Vec<_>>(),
        CHECKPOINT_VECTOR_CASES
    );
    assert_eq!(loaded, expected);
}

#[test]
fn unsupported_corpus_version_is_build_breaking() {
    let repository = temporary_repository("version");
    let replay_path = repository.join("tests/vectors/codec/replay_corpus.lxb");
    let Ok(mut replay) = fs::read(&replay_path) else {
        panic!("copied replay corpus unreadable");
    };
    replay[11] = 2;
    assert!(fs::write(&replay_path, replay).is_ok());
    let result = Corpus::load(&repository);
    let _ = fs::remove_dir_all(&repository);
    assert!(matches!(
        result,
        Err(CorpusError::UnsupportedVersion {
            declared: 2,
            supported: DERIVED_PROTOCOL_VERSION,
            ..
        })
    ));
}

#[test]
fn unknown_vector_class_names_the_missing_capability() {
    let repository = temporary_repository("class");
    let valid_path = repository.join("tests/vectors/codec/valid.lxv");
    let Ok(mut valid) = fs::read_to_string(&valid_path) else {
        panic!("copied valid corpus unreadable");
    };
    valid.push_str("future-structure|future-case|00|-1|-\n");
    assert!(fs::write(&valid_path, valid).is_ok());
    let result = Corpus::load(&repository);
    let _ = fs::remove_dir_all(&repository);
    assert_eq!(
        result,
        Err(CorpusError::MissingCapability {
            vector: "future-case".to_owned(),
            capability: "future-structure".to_owned(),
        })
    );
}
