use layerx_paxai_worker::discovery::IdentityEvidence;
use layerx_programs_ai_market::{
    codec::decode_chunk_response,
    queries::{read_state_chunk, ReadProof},
};
use serde_json::{json, Value};
use std::fs;
use std::io;
use std::os::unix::fs::PermissionsExt;
use std::path::Path;

/// Publishes real native query chunks to the worker's external authority adapter.
pub fn write(
    directory: &Path,
    state: &[u8],
    roster: &[u8],
    signed_metadata: &[u8],
    proof: &ReadProof,
    owner: &IdentityEvidence,
    identities: &[IdentityEvidence],
) -> io::Result<()> {
    let query = |revision: u64, digest: [u8; 32], offset: u32| {
        let mut payload = revision.to_be_bytes().to_vec();
        payload.extend_from_slice(&digest);
        payload.extend_from_slice(&offset.to_be_bytes());
        payload.extend_from_slice(&8192u16.to_be_bytes());
        payload
    };
    let error = |error| io::Error::other(format!("native snapshot: {error:?}"));
    let mut out = vec![0; 8244];
    let n = read_state_chunk(state, &query(0, [0; 32], 0), &mut out).map_err(error)?;
    let first = decode_chunk_response(&out[..n]).map_err(error)?;
    let (revision, digest, total) = (first.revision, first.digest.bytes(), first.total_bytes);
    let mut chunks = vec![out[..n].to_vec()];
    for offset in (8192..total).step_by(8192) {
        let n =
            read_state_chunk(state, &query(revision, digest, offset), &mut out).map_err(error)?;
        chunks.push(out[..n].to_vec());
    }
    let identity = |evidence: &IdentityEvidence| -> Value {
        json!({
            "principal": evidence.principal.bytes(), "primary_key": evidence.primary_key.0,
            "frozen": evidence.frozen, "execution_height": evidence.execution_height,
        })
    };
    let observation = json!({
        "chunks": chunks, "native_state_root": proof.native_state_root.bytes(),
        "observed_sequence": proof.observed_sequence, "execution_height": proof.execution_height,
        "block_height": proof.execution_height, "block_hash": proof.block_hash.bytes(),
        "app_hash": ([0xC1; 32]), "store_proof_digest": ([0xD3; 32]),
        "sealed_height": proof.execution_height, "owner": identity(owner),
        "identities": identities.iter().map(identity).collect::<Vec<_>>(),
        "roster": roster, "signed_metadata": signed_metadata,
    });
    fs::write(
        directory.join("observation.json"),
        serde_json::to_vec(&observation)?,
    )?;
    let quoted = directory
        .join("observation.json")
        .to_string_lossy()
        .replace('\'', "'\\''");
    let reader = directory.join("authority-reader");
    fs::write(
        &reader,
        format!("#!/bin/sh\ncat >/dev/null\nexec cat '{quoted}'\n"),
    )?;
    fs::set_permissions(reader, fs::Permissions::from_mode(0o700))
}
