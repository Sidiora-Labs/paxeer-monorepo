use std::env;
use std::fs;
use std::path::{Path, PathBuf};

use layerx_programs::chain_proof::{
    decode::decode_commitment_proof,
    header::ChainHeader,
    proto::{ProtoReader, ProtoValue},
    state::{verify_absent, verify_state},
    validators::{TrustedValidator, ValidatorSet},
    vote::{BlockId, CommitSig, PartSetHeader},
    ChainProofError, ChainProofVerifier, Commit,
};
use layerx_programs_runtime::{programs_fuzz_observation, FuzzTarget as RuntimeFuzzTarget};

#[derive(Clone, Copy, Debug)]
enum FuzzTarget {
    Runtime(RuntimeFuzzTarget),
    ChainProof,
}

/// A heap ceiling far above the runtime's metered module memory bound. A lawful
/// validation, instantiation or execution of a committed corpus seed (and its
/// bounded mutations) stays well under it, so tripping the ceiling can only mean
/// an unbounded-allocation defect - which [`bounded_alloc`] turns into a
/// process abort, i.e. a build-breaking failure of the fuzz gate.
mod bounded_alloc {
    use std::alloc::{GlobalAlloc, Layout, System};
    use std::sync::atomic::{AtomicUsize, Ordering};

    /// Declared upper bound on the live heap the fuzz harness may hold at once.
    pub const MAX_LIVE_HEAP_BYTES: usize = 256 * 1024 * 1024;

    static LIVE_BYTES: AtomicUsize = AtomicUsize::new(0);

    /// System allocator wrapper that aborts the process the moment the live heap
    /// would exceed the declared ceiling, making unbounded allocation a
    /// build-breaking defect rather than a silent memory blow-up.
    pub struct BoundingAllocator;

    #[allow(unsafe_code)]
    unsafe impl GlobalAlloc for BoundingAllocator {
        unsafe fn alloc(&self, layout: Layout) -> *mut u8 {
            let previous = LIVE_BYTES.fetch_add(layout.size(), Ordering::Relaxed);
            if previous.saturating_add(layout.size()) > MAX_LIVE_HEAP_BYTES {
                std::process::abort();
            }
            System.alloc(layout)
        }

        unsafe fn dealloc(&self, ptr: *mut u8, layout: Layout) {
            LIVE_BYTES.fetch_sub(layout.size(), Ordering::Relaxed);
            System.dealloc(ptr, layout);
        }

        unsafe fn alloc_zeroed(&self, layout: Layout) -> *mut u8 {
            let previous = LIVE_BYTES.fetch_add(layout.size(), Ordering::Relaxed);
            if previous.saturating_add(layout.size()) > MAX_LIVE_HEAP_BYTES {
                std::process::abort();
            }
            System.alloc_zeroed(layout)
        }

        unsafe fn realloc(&self, ptr: *mut u8, layout: Layout, new_size: usize) -> *mut u8 {
            if new_size > layout.size() {
                let growth = new_size - layout.size();
                let previous = LIVE_BYTES.fetch_add(growth, Ordering::Relaxed);
                if previous.saturating_add(growth) > MAX_LIVE_HEAP_BYTES {
                    std::process::abort();
                }
            } else {
                LIVE_BYTES.fetch_sub(layout.size() - new_size, Ordering::Relaxed);
            }
            System.realloc(ptr, layout, new_size)
        }
    }
}

#[global_allocator]
static GLOBAL: bounded_alloc::BoundingAllocator = bounded_alloc::BoundingAllocator;

fn decode_hex(text: &str) -> Result<Vec<u8>, String> {
    let compact: Vec<u8> = text
        .bytes()
        .filter(|byte| !byte.is_ascii_whitespace())
        .collect();
    if !compact.len().is_multiple_of(2) {
        return Err("corpus hex has odd length".to_string());
    }
    compact
        .chunks_exact(2)
        .map(|pair| {
            let high = (pair[0] as char)
                .to_digit(16)
                .ok_or_else(|| "invalid corpus hex".to_string())?;
            let low = (pair[1] as char)
                .to_digit(16)
                .ok_or_else(|| "invalid corpus hex".to_string())?;
            u8::try_from((high << 4) | low).map_err(|error| error.to_string())
        })
        .collect()
}

fn target(name: &str) -> Result<FuzzTarget, String> {
    match name {
        "validation" => Ok(FuzzTarget::Runtime(RuntimeFuzzTarget::Validation)),
        "instantiation" => Ok(FuzzTarget::Runtime(RuntimeFuzzTarget::Instantiation)),
        "execution" => Ok(FuzzTarget::Runtime(RuntimeFuzzTarget::Execution)),
        "chain-proof" => Ok(FuzzTarget::ChainProof),
        _ => Err(format!("unknown fuzz target {name}")),
    }
}

fn decode_part_set(bytes: &[u8]) -> Result<PartSetHeader, ChainProofError> {
    let mut parts = PartSetHeader::default();
    let mut reader = ProtoReader::new(bytes);
    let mut previous = 0;
    while let Some(field) = reader.next_field()? {
        if field.number == previous {
            return Err(ChainProofError::NonCanonical);
        }
        previous = field.number;
        match (field.number, field.value) {
            (1, ProtoValue::Varint(total)) => {
                parts.total = u32::try_from(total).map_err(|_| ChainProofError::NonCanonical)?;
            }
            (2, ProtoValue::Bytes(hash)) => parts.hash = hash.to_vec(),
            (1 | 2, _) => return Err(ChainProofError::NonCanonical),
            _ => return Err(ChainProofError::UnknownField),
        }
    }
    reader.finish()?;
    Ok(parts)
}

fn decode_block_id(bytes: &[u8]) -> Result<BlockId, ChainProofError> {
    let mut block = BlockId::default();
    let mut reader = ProtoReader::new(bytes);
    let mut previous = 0;
    while let Some(field) = reader.next_field()? {
        if field.number == previous {
            return Err(ChainProofError::NonCanonical);
        }
        previous = field.number;
        match (field.number, field.value) {
            (1, ProtoValue::Bytes(hash)) => block.hash = hash.to_vec(),
            (2, ProtoValue::Bytes(parts)) => block.part_set_header = decode_part_set(parts)?,
            (1 | 2, _) => return Err(ChainProofError::NonCanonical),
            _ => return Err(ChainProofError::UnknownField),
        }
    }
    reader.finish()?;
    Ok(block)
}

fn decode_commit(bytes: &[u8]) -> Result<Commit, ChainProofError> {
    let mut commit = Commit {
        height: 0,
        round: 0,
        block_id: BlockId::default(),
        signatures: Vec::new(),
    };
    let mut reader = ProtoReader::new(bytes);
    let mut previous = 0;
    while let Some(field) = reader.next_field()? {
        if field.number == previous && field.number != 4 {
            return Err(ChainProofError::NonCanonical);
        }
        previous = field.number;
        match (field.number, field.value) {
            (1, ProtoValue::Varint(height)) => {
                commit.height = i64::from_le_bytes(height.to_le_bytes());
            }
            (2, ProtoValue::Varint(round)) => {
                commit.round = i32::try_from(round).map_err(|_| ChainProofError::NonCanonical)?;
            }
            (3, ProtoValue::Bytes(block)) => commit.block_id = decode_block_id(block)?,
            (4, ProtoValue::Bytes(signature)) => {
                commit.signatures.push(CommitSig::decode(signature)?);
            }
            (1..=4, _) => return Err(ChainProofError::NonCanonical),
            _ => return Err(ChainProofError::UnknownField),
        }
    }
    reader.finish()?;
    Ok(commit)
}

fn decode_signed_header(bytes: &[u8], commit: &[u8]) -> Result<ChainHeader, ChainProofError> {
    let mut reader = ProtoReader::new(bytes);
    let mut header = None;
    let mut previous = 0;
    while let Some(field) = reader.next_field()? {
        if field.number == previous {
            return Err(ChainProofError::NonCanonical);
        }
        previous = field.number;
        match (field.number, field.value) {
            (1, ProtoValue::Bytes(bytes)) => header = Some(ChainHeader::decode(bytes)?),
            (2, ProtoValue::Bytes(bytes)) if bytes == commit => {}
            (2, ProtoValue::Bytes(_)) => return Err(ChainProofError::Mismatch),
            (1 | 2, _) => return Err(ChainProofError::NonCanonical),
            _ => return Err(ChainProofError::UnknownField),
        }
    }
    reader.finish()?;
    header.ok_or(ChainProofError::NonCanonical)
}

fn chain_proof_observation(input: &[u8]) -> Result<i64, ChainProofError> {
    let proof = decode_commitment_proof(input)?;
    let header = decode_signed_header(&proof.signed_header, &proof.commit)?;
    let commit = decode_commit(&proof.commit)?;
    let mut public_key = [0x66; 32];
    public_key[0] = 0x58;
    let verifier = ChainProofVerifier {
        chain_id: "paxeer-chain-proof-fuzz".to_owned(),
        validators: ValidatorSet::new(vec![TrustedValidator::new(public_key, 1)])?,
    };
    let head = verifier.verify_head(&header, &commit)?;
    match &proof.value {
        Some(value) => verify_state(
            &head,
            &proof.store_name,
            &proof.key,
            value,
            &proof.proof_ops,
        ),
        None => verify_absent(&head, &proof.store_name, &proof.key, &proof.proof_ops),
    }
}

fn corpus_files(root: &Path) -> Result<Vec<PathBuf>, String> {
    let entries = fs::read_dir(root).map_err(|error| error.to_string())?;
    let mut files: Vec<PathBuf> = entries
        .filter_map(Result::ok)
        .map(|entry| entry.path())
        .filter(|path| path.extension().is_some_and(|extension| extension == "hex"))
        .collect();
    files.sort();
    if files.is_empty() {
        return Err(format!("empty corpus {}", root.display()));
    }
    Ok(files)
}

fn mutate(seed: &[u8]) -> Vec<Vec<u8>> {
    let mut cases = vec![seed.to_vec(), Vec::new()];
    if !seed.is_empty() {
        for index in 0..seed.len().min(64) {
            let mut flipped = seed.to_vec();
            flipped[index] ^= 0xff;
            cases.push(flipped);
        }
        cases.push(seed[..seed.len() / 2].to_vec());
    }
    cases
}

/// Runs one input through the surface twice, rejecting a non-deterministic
/// outcome. A panic or hang inside the surface fails the process directly; an
/// unbounded allocation aborts through [`bounded_alloc`]; a byte-level
/// divergence between the two observations is reported here as a build-breaking
/// non-determinism defect.
fn check(selected: FuzzTarget, input: &[u8], path: &Path) -> Result<(), String> {
    let deterministic = match selected {
        FuzzTarget::Runtime(target) => {
            programs_fuzz_observation(target, input) == programs_fuzz_observation(target, input)
        }
        FuzzTarget::ChainProof => chain_proof_observation(input) == chain_proof_observation(input),
    };
    if deterministic {
        Ok(())
    } else {
        Err(format!(
            "non-deterministic {selected:?} outcome for input derived from {}",
            path.display()
        ))
    }
}

fn run() -> Result<(), String> {
    let mut arguments = env::args().skip(1);
    let name = arguments
        .next()
        .ok_or_else(|| "missing fuzz target".to_string())?;
    let corpus = arguments
        .next()
        .ok_or_else(|| "missing corpus directory".to_string())?;
    if arguments.next().is_some() {
        return Err("unexpected fuzz argument".to_string());
    }
    let selected = target(&name)?;
    for path in corpus_files(Path::new(&corpus))? {
        let source = fs::read_to_string(&path).map_err(|error| error.to_string())?;
        let seed = decode_hex(&source)?;
        for input in mutate(&seed) {
            check(selected, &input, &path)?;
        }
    }
    Ok(())
}

fn main() {
    if let Err(error) = run() {
        eprintln!("programs fuzz failed: {error}");
        std::process::exit(1);
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn chain_proof_target_never_panics_on_seed_corpus() -> Result<(), String> {
        let selected = target("chain-proof")?;
        assert!(matches!(selected, FuzzTarget::ChainProof));
        let root = Path::new(env!("CARGO_MANIFEST_DIR")).join("corpus/chain-proof");
        let mut count = 0;
        for entry in fs::read_dir(&root).map_err(|error| error.to_string())? {
            let path = entry.map_err(|error| error.to_string())?.path();
            let source = fs::read_to_string(&path).map_err(|error| error.to_string())?;
            let seed = decode_hex(&source)?;
            for input in mutate(&seed) {
                let outcome = std::panic::catch_unwind(|| check(selected, &input, &path));
                assert!(
                    outcome.is_ok(),
                    "chain-proof panicked for {}",
                    path.display()
                );
                outcome.map_err(|_| format!("panic for {}", path.display()))??;
            }
            count += 1;
        }
        assert!(count > 0, "chain-proof corpus must not be empty");
        Ok(())
    }
}
