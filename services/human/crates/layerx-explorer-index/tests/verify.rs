use std::path::PathBuf;
use std::process::Command;

use layerx_explorer_index::verify::{EvidenceKind, PastedInclusion, Verifier, VerifyError};
use layerx_proof::inclusion::{InclusionError, SequencerAuthorization};
use layerx_proof::merkle::MerkleError;
use layerx_proof::receipt::{AuthorizedBatch, ReceiptCheck};
use layerx_types::verify::VerificationLevel;

const MAXIMUM_FIXTURE_BYTES: usize = 1_048_576;

struct CoreFixture {
    receipt: Vec<u8>,
    receipt_authority: AuthorizedBatch,
    activity: Vec<u8>,
    proof: Vec<u8>,
    header: Vec<u8>,
    header_signature: [u8; 64],
    sequencer_authority: SequencerAuthorization,
    activity_root: [u8; 32],
}

impl CoreFixture {
    fn load() -> Self {
        let binary = std::env::var_os("LAYERX_EXPLORER_CORE_FIXTURE")
            .map_or_else(default_fixture_binary, PathBuf::from);
        let output = Command::new(&binary)
            .output()
            .unwrap_or_else(|error| panic!("failed to run {}: {error}", binary.display()));
        assert!(
            output.status.success(),
            "core fixture failed with {:?}: {}",
            output.status.code(),
            String::from_utf8_lossy(&output.stderr)
        );
        assert!(output.stdout.len() <= MAXIMUM_FIXTURE_BYTES);
        let mut reader = Reader::new(&output.stdout);
        assert_eq!(reader.array::<4>(), *b"LXEF");
        assert_eq!(reader.byte(), 1);
        let receipt = reader.sized();
        let sequencer_public_key = reader.array();
        let batch_id = reader.array();
        let asset = reader.array();
        let previous_state_root = reader.array();
        let resulting_state_root = reader.array();
        let activity = reader.sized();
        let proof = reader.sized();
        let header = reader.sized();
        let header_signature = reader.array();
        let sequencer_id = reader.array();
        let first_batch = reader.u64();
        let last_batch = reader.u64();
        let activity_root = reader.array();
        assert!(reader.is_empty(), "core fixture has trailing bytes");
        Self {
            receipt,
            receipt_authority: AuthorizedBatch::new(
                batch_id,
                asset,
                previous_state_root,
                resulting_state_root,
                sequencer_public_key,
            ),
            activity,
            proof,
            header,
            header_signature,
            sequencer_authority: SequencerAuthorization::new(
                sequencer_id,
                sequencer_public_key,
                first_batch,
                last_batch,
            ),
            activity_root,
        }
    }

    fn verifier(&self) -> Verifier {
        Verifier::new(vec![self.receipt_authority], vec![self.sequencer_authority])
    }

    fn inclusion(&self) -> PastedInclusion<'_> {
        PastedInclusion {
            kind: EvidenceKind::ActivityInclusion,
            proof_bytes: &self.proof,
            canonical_leaf_bytes: &self.activity,
            named_root: self.activity_root,
            canonical_header_bytes: &self.header,
            header_signature: self.header_signature,
        }
    }
}

struct Reader<'a> {
    bytes: &'a [u8],
    offset: usize,
}

impl<'a> Reader<'a> {
    const fn new(bytes: &'a [u8]) -> Self {
        Self { bytes, offset: 0 }
    }

    fn byte(&mut self) -> u8 {
        self.take(1)[0]
    }

    fn array<const N: usize>(&mut self) -> [u8; N] {
        self.take(N)
            .try_into()
            .unwrap_or_else(|_| panic!("fixture field width mismatch"))
    }

    fn u32(&mut self) -> u32 {
        u32::from_be_bytes(self.array())
    }

    fn u64(&mut self) -> u64 {
        u64::from_be_bytes(self.array())
    }

    fn sized(&mut self) -> Vec<u8> {
        let length = usize::try_from(self.u32())
            .unwrap_or_else(|error| panic!("fixture length overflow: {error}"));
        assert!(length <= MAXIMUM_FIXTURE_BYTES);
        self.take(length).to_vec()
    }

    fn take(&mut self, length: usize) -> &'a [u8] {
        let end = self
            .offset
            .checked_add(length)
            .unwrap_or_else(|| panic!("fixture offset overflow"));
        let bytes = self
            .bytes
            .get(self.offset..end)
            .unwrap_or_else(|| panic!("truncated core fixture"));
        self.offset = end;
        bytes
    }

    fn is_empty(&self) -> bool {
        self.offset == self.bytes.len()
    }
}

#[test]
fn core_receipt_and_inclusion_verify_independently_and_tampering_is_refused() {
    let fixture = CoreFixture::load();
    let verifier = fixture.verifier();
    let receipt = verifier
        .receipt(&fixture.receipt)
        .unwrap_or_else(|error| panic!("core receipt did not verify: {error:?}"));
    assert_eq!(receipt.kind, EvidenceKind::Receipt);
    assert_eq!(receipt.achieved_level, VerificationLevel::SEQUENCER_SIGNED);
    assert!(receipt.receipt_digest.is_some());

    let inclusion = verifier
        .inclusion(&fixture.inclusion())
        .unwrap_or_else(|error| panic!("core inclusion did not verify: {error:?}"));
    assert_eq!(inclusion.kind, EvidenceKind::ActivityInclusion);
    assert_eq!(inclusion.achieved_level, VerificationLevel::BATCH_INCLUDED);
    assert_eq!(inclusion.proof_root, Some(fixture.activity_root));

    let mut altered_receipt = fixture.receipt.clone();
    let last = altered_receipt
        .last_mut()
        .unwrap_or_else(|| panic!("core receipt was empty"));
    *last ^= 1;
    assert_eq!(
        verifier.receipt(&altered_receipt),
        Err(VerifyError::Receipt(ReceiptCheck::SequencerSignature))
    );

    let mut altered_proof = fixture.proof.clone();
    let last = altered_proof
        .last_mut()
        .unwrap_or_else(|| panic!("core proof was empty"));
    *last ^= 1;
    assert_eq!(
        verifier.inclusion(&PastedInclusion {
            proof_bytes: &altered_proof,
            ..fixture.inclusion()
        }),
        Err(VerifyError::Inclusion(InclusionError::Merkle(
            MerkleError::RootMismatch
        )))
    );

    let truncated_proof = fixture
        .proof
        .get(..fixture.proof.len().saturating_sub(1))
        .unwrap_or_else(|| panic!("core proof truncation failed"));
    assert_eq!(
        verifier.inclusion(&PastedInclusion {
            proof_bytes: truncated_proof,
            ..fixture.inclusion()
        }),
        Err(VerifyError::Proof(MerkleError::Encoding))
    );

    let mut altered_signature = fixture.header_signature;
    altered_signature[0] ^= 1;
    assert_eq!(
        verifier.inclusion(&PastedInclusion {
            header_signature: altered_signature,
            ..fixture.inclusion()
        }),
        Err(VerifyError::Inclusion(InclusionError::HeaderSignature))
    );

    let mut altered_root = fixture.activity_root;
    altered_root[0] ^= 1;
    assert_eq!(
        verifier.inclusion(&PastedInclusion {
            named_root: altered_root,
            ..fixture.inclusion()
        }),
        Err(VerifyError::NamedRoot)
    );

    let no_trust = Verifier::new(Vec::new(), Vec::new());
    assert_eq!(
        no_trust.receipt(&fixture.receipt),
        Err(VerifyError::MissingTrust(EvidenceKind::Receipt))
    );
    assert_eq!(
        no_trust.inclusion(&fixture.inclusion()),
        Err(VerifyError::MissingTrust(EvidenceKind::ActivityInclusion))
    );
}

fn default_fixture_binary() -> PathBuf {
    PathBuf::from(env!("CARGO_MANIFEST_DIR"))
        .ancestors()
        .nth(4)
        .unwrap_or_else(|| panic!("explorer crate is outside repository"))
        .join("build/tests/explorer_fixture")
}
