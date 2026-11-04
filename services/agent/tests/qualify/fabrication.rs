use std::collections::BTreeSet;
use std::path::Path;
use std::process::Command;

use layerx_agent_api::prepare::CanonicalBytes;
use layerx_agent_api::read::{CheckpointValue, Freshness, RelativeTo, VerifiedRead};
use layerx_agent_api::verify::Rung;
use layerx_sdk::{Client, SdkError};

use crate::hostile_node::{
    agent_hostile_node_harness, owner_record, verify_owner_record, HostileAttack,
};
use serde_json::Value;

fn run(repository: &Path, program: &str, arguments: &[&str]) -> Result<(), String> {
    let output = Command::new(program)
        .args(arguments)
        .current_dir(repository)
        .output()
        .map_err(|error| format!("could not run {program}: {error}"))?;
    if output.status.success() {
        Ok(())
    } else {
        Err(format!(
            "{} {:?} failed: status={} stderr={}",
            program,
            arguments,
            output.status,
            String::from_utf8_lossy(&output.stderr).trim()
        ))
    }
}

fn run_rust_suites(repository: &Path) -> Result<(), String> {
    for (package, test) in [
        ("layerx-proof", "negative"),
        ("layerx-proof", "availability"),
        ("layerx-agentd", "balance"),
        ("layerx-agentd", "history"),
        ("layerx-agentd", "checkpoint"),
        ("layerx-agentd", "finality"),
        ("layerx-agentd", "export"),
        ("layerx-agentd", "gaps"),
        ("layerx-mcp", "read"),
    ] {
        run(
            repository,
            "cargo",
            &[
                "test",
                "--manifest-path",
                "services/agent/Cargo.toml",
                "--locked",
                "-p",
                package,
                "--test",
                test,
            ],
        )?;
    }
    Ok(())
}

/// Convert only independently verified producer material into the major-2
/// read carrier, preserving the exact block hash and value height.
pub fn verified_owner_read(record: &Value) -> Result<VerifiedRead<CheckpointValue>, String> {
    let verified = verify_owner_record(record).map_err(|error| format!("{error:?}"))?;
    let height = u64::try_from(verified.head.height()).map_err(|error| error.to_string())?;
    let canonical = |value: &Value| -> Result<CanonicalBytes, String> {
        CanonicalBytes::new(serde_json::to_vec(value).map_err(|error| error.to_string())?)
            .map_err(|error| format!("canonical evidence: {error:?}"))
    };
    let read = VerifiedRead::new(
        CheckpointValue {
            commit: canonical(&record["commit"])?,
            validator_set: canonical(&record["validators"])?,
            height,
        },
        Rung::Committed,
        Freshness {
            chain_head: height,
            value_height: height,
            relative_to: RelativeTo::Block {
                height,
                hash: verified.head.header_hash(),
            },
        },
    );
    Client::accept_verified_read(Rung::Committed, read)
        .map_err(|error| format!("SDK read: {error:?}"))
}

fn rust_sdk_refuses_unverified() -> Result<(), String> {
    let mut read = verified_owner_read(&owner_record()?)?;
    read.achieved_rung = Rung::Submitted;
    if Client::accept_verified_read(Rung::Receipted, read) == Err(SdkError::UnverifiedRead) {
        Ok(())
    } else {
        Err("Rust SDK surfaced a hostile unverified value".to_owned())
    }
}

pub fn agent_fabrication_evidence_gate() -> Result<(), String> {
    verified_owner_read(&owner_record()?)?;
    rust_sdk_refuses_unverified()?;
    for attack in [
        HostileAttack::AlteredBalance,
        HostileAttack::AlteredReceipt,
        HostileAttack::ResignedReceipt,
    ] {
        let record = crate::hostile_node::attack_record(attack)?;
        if verified_owner_read(&record).is_ok() {
            return Err(format!("SDK accepted hostile {attack:?}"));
        }
    }
    for (field, value) in [
        ("blockHash", Value::from(format!("0x{}", "01".repeat(32)))),
        ("blockNumber", Value::from("0xb")),
    ] {
        let mut record = owner_record()?;
        record["receiptJson"][field] = value;
        if verified_owner_read(&record).is_ok() {
            return Err(format!("SDK accepted hostile {field}"));
        }
    }
    Ok(())
}

fn dynamic_sdks_refuse_unverified(repository: &Path) -> Result<(), String> {
    let typescript = r#"import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {Rung,requireVerified,verifySequencerReceipt,SequencerReceiptVerificationError,layerx_sdk_ts_package} from './services/agent/sdk/typescript/dist/src/index.js';
assert.equal(layerx_sdk_ts_package().contractMajor,2);
const record=JSON.parse(readFileSync('./services/agent/crates/layerx-sdk/src/repair_22_31_227_fixture/committed.json','utf8'));
const bytes=s=>Uint8Array.from(Buffer.from(s.replace(/^0x/,''),'hex'));
const key={keyId:bytes(record.pinnedKey.keyId),publicKey:bytes(record.pinnedKey.publicKey),first_height:10n,last_height:10n};
const receipt=bytes(record.receiptJson.receipt),tx=bytes(record.rawTransaction);
const verified=await verifySequencerReceipt(receipt,tx,[key]);
// Empty compatibility fields claim no retired batch/checkpoint authority.
const read={value:verified,achievedVerificationLevel:Rung.Receipted,freshness:{chainHead:verified.height,latestBatch:'',latestCheckpoint:'',valueSequence:verified.receipt.globalSequence}};
assert.equal(requireVerified(Rung.Receipted,read),read);
assert.throws(()=>requireVerified(Rung.Committed,read),/verification_below_requested/);
assert.throws(()=>requireVerified(Rung.Receipted,{...read,achievedVerificationLevel:null}),/unverified_read/);
for(const index of [receipt.length-1,20]) {
  const altered=receipt.slice(); altered[index]^=1;
  await assert.rejects(()=>verifySequencerReceipt(altered,tx,[key]),e=>e instanceof SequencerReceiptVerificationError && e.check==='bad_signature');
}
await assert.rejects(()=>verifySequencerReceipt(receipt,tx,[{...key,first_height:11n}]),e=>e instanceof SequencerReceiptVerificationError && e.check==='key_out_of_range');
const changedTx=tx.slice();changedTx[changedTx.length-1]^=1;
await assert.rejects(()=>verifySequencerReceipt(receipt,changedTx,[key]),e=>e instanceof SequencerReceiptVerificationError && e.check==='hash_mismatch');"#;
    run(
        repository,
        "node",
        &["--input-type=module", "-e", typescript],
    )?;
    let python = r#"import json
from dataclasses import replace
from pathlib import Path
from layerx_sdk import Rung, VerifiedRead, require_verified, layerx_sdk_py_package
from layerx_sdk.verifier import ReceiptKey, verify_sequencer_receipt, SequencerReceiptVerificationError
assert layerx_sdk_py_package()['contract_major'] == 2
record=json.loads(Path('services/agent/crates/layerx-sdk/src/repair_22_31_227_fixture/committed.json').read_text())
unhex=lambda s: bytes.fromhex(s.removeprefix('0x'))
key=ReceiptKey(unhex(record['pinnedKey']['keyId']),unhex(record['pinnedKey']['publicKey']),10,10)
receipt=unhex(record['receiptJson']['receipt']); tx=unhex(record['rawTransaction'])
verified=verify_sequencer_receipt(receipt,tx,[key])
read=VerifiedRead(verified,Rung.RECEIPTED,verified.height,'','',verified.receipt.global_sequence)
assert require_verified(Rung.RECEIPTED,read) is read
for requested, candidate, expected in [(Rung.COMMITTED,read,'verification_below_requested'),(Rung.RECEIPTED,replace(read,achieved_verification_level=None),'unverified_read')]:
    try: require_verified(requested,candidate)
    except ValueError as error: assert str(error).startswith(expected)
    else: raise AssertionError('hostile rung accepted')
def refuses(data, transaction, keys, expected):
    try: verify_sequencer_receipt(data,transaction,keys)
    except SequencerReceiptVerificationError as error: assert error.check == expected
    else: raise AssertionError('hostile evidence accepted')
for index in [len(receipt)-1,20]:
    altered=bytearray(receipt);altered[index]^=1
    refuses(bytes(altered),tx,[key],'bad_signature')
refuses(receipt,tx,[replace(key,first_height=11)],'key_out_of_range')
altered=bytearray(tx);altered[-1]^=1
refuses(receipt,bytes(altered),[key],'hash_mismatch')"#;
    let output = Command::new("python3")
        .arg("-c")
        .arg(python)
        .current_dir(repository)
        .env("PYTHONPATH", repository.join("services/agent/sdk/python"))
        .output()
        .map_err(|error| format!("could not run Python hostile SDK check: {error}"))?;
    if output.status.success() {
        Ok(())
    } else {
        Err(format!(
            "Python SDK surfaced a hostile unverified value: {}",
            String::from_utf8_lossy(&output.stderr).trim()
        ))
    }
}

/// Runs the hostile-node, independent-evidence and every-surface no-fabrication gate.
///
/// # Errors
///
/// Fails if an attack becomes a result or a required read/stream/MCP/SDK suite fails.
pub fn agent_qualify_fabrication_gate(repository: &Path) -> Result<String, String> {
    let observed = agent_hostile_node_harness()?;
    let expected = BTreeSet::from([
        HostileAttack::AlteredBalance,
        HostileAttack::AlteredReceipt,
        HostileAttack::ResignedReceipt,
        HostileAttack::SubthresholdCertificate,
        HostileAttack::TruncatedProof,
        HostileAttack::ReorderedEvents,
        HostileAttack::WithheldAvailability,
    ]);
    if observed != expected {
        return Err(format!(
            "hostile-node attack coverage drift: observed={observed:?} expected={expected:?}"
        ));
    }
    run_rust_suites(repository)?;
    agent_fabrication_evidence_gate()?;
    dynamic_sdks_refuse_unverified(repository)?;
    Ok(format!(
        "agent_qualify_fabrication_gate passed attacks={} read_surfaces=balance,history,checkpoint,receipt,availability,export streaming=gaps mcp=read sdks=rust,typescript,python",
        observed.len()
    ))
}

#[cfg(test)]
mod tests {
    use super::rust_sdk_refuses_unverified;

    #[test]
    fn rust_sdk_refuses_a_submitted_read() {
        assert_eq!(rust_sdk_refuses_unverified(), Ok(()));
    }
}
