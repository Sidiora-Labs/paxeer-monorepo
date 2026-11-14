use crate::require_value::RequireValue as _;
use base64::Engine as _;
use layerx_client::rpc::receipt::{
    lookup_authenticated, lookup_authenticated_with_commit, AuthenticatedReceipt, Lookup,
    LookupContext,
};
use layerx_client::rpc::receipt_commit::ReceiptCommitEvidence;
use layerx_client::runtime_clock::RuntimeClock;
use layerx_client::sequencer::{ReceiptKeyEntry, SequencerEndpoint, SequencerError};
use layerx_crypto::disclosure::call::ledger_selector;
use layerx_crypto::evm_transaction::{
    decode_signed, recover_sender, transaction_hash, Eip1559Call,
};
use layerx_types::clock::{Clock, Deadline};
use layerx_types::rung::Rung;
use serde_json::Value;
use std::collections::BTreeMap;
use std::fs::{self, OpenOptions};
use std::os::unix::fs::{OpenOptionsExt as _, PermissionsExt as _};
use std::path::{Path, PathBuf};
use std::process::{Command, Stdio};
use std::sync::Arc;
use std::thread;
use std::time::Duration;

pub(crate) fn unhex(text: &str) -> Vec<u8> {
    text.strip_prefix("0x")
        .or_fail("hex prefix")
        .as_bytes()
        .chunks_exact(2)
        .map(|pair| {
            u8::from_str_radix(std::str::from_utf8(pair).or_fail("hex text"), 16)
                .or_fail("hex byte")
        })
        .collect()
}

pub(crate) struct BankNode {
    pub(crate) clock: Arc<RuntimeClock>,
    script: PathBuf,
    pub(crate) home: PathBuf,
    pub(crate) endpoint: SequencerEndpoint,
    pub(crate) rpc_url: String,
    pub(crate) verifier: layerx_client::rpc::ChainProofVerifier,
    pub(crate) secret: [u8; 32],
}

impl BankNode {
    pub(crate) fn start() -> Self {
        let root = Path::new(env!("CARGO_MANIFEST_DIR"))
            .ancestors()
            .nth(4)
            .or_fail("repository root");
        let script = root.join("tools/qualification/localnode/v6120-node.sh");
        let environment = std::env::temp_dir()
            .join("lx-localnode")
            .join(format!("move-bank-{}.env", std::process::id()));
        let evidence = layerx_human_test_support::directory("move-bank-node");
        fs::create_dir_all(&evidence).or_fail("node evidence directory");
        fs::set_permissions(&evidence, fs::Permissions::from_mode(0o700))
            .or_fail("private evidence directory");
        let log_path = evidence.join("startup.log");
        let log = OpenOptions::new()
            .create_new(true)
            .write(true)
            .mode(0o600)
            .open(&log_path)
            .or_fail("private node log");
        let status = Command::new("bash")
            .arg(&script)
            .arg("--sequencer")
            .arg("--env-out")
            .arg(&environment)
            .stdout(Stdio::from(log.try_clone().unwrap_or_else(|error| {
                panic!("{}: {error:?}", "node log clone")
            })))
            .stderr(Stdio::from(log))
            .status()
            .or_fail("start genuine disposable sequencer");
        assert!(
            status.success(),
            "node startup failed; evidence: {}",
            log_path.display()
        );
        let text = fs::read_to_string(&environment).or_fail("disposable node environment");
        let fields: BTreeMap<_, _> = text
            .lines()
            .map(|line| line.split_once('=').or_fail("node environment entry"))
            .collect();
        let endpoint = SequencerEndpoint::new(fields["LOCAL_NODE_SEQUENCER_RPC"], &[])
            .or_fail("genuine sequencer endpoint");
        let genesis: Value = serde_json::from_slice(
            &fs::read(Path::new(fields["LOCAL_NODE_HOME"]).join("config/genesis.json"))
                .or_fail("governed genesis"),
        )
        .or_fail("genesis JSON");
        let validators = endpoint
            .validator_set(endpoint.block_number().or_fail("committed height"))
            .or_fail("node validator set");
        let node = Self {
            clock: RuntimeClock::from_environment().or_fail("bank node clock"),
            script,
            home: PathBuf::from(fields["LOCAL_NODE_HOME"]),
            endpoint,
            rpc_url: fields["LOCAL_NODE_SEQUENCER_RPC"].to_owned(),
            verifier: layerx_client::rpc::ChainProofVerifier {
                chain_id: genesis["chain_id"].as_str().or_fail("chain ID").to_owned(),
                validators: layerx_client::rpc::commit::decode_validators(&validators)
                    .or_fail("pinned node validators"),
            },
            secret: unhex(fields["LOCAL_NODE_KEY"])
                .try_into()
                .or_fail("EVM secret width"),
        };
        fs::remove_file(environment).or_fail("remove copied private environment");
        node
    }

    pub(crate) fn receipt_keys(&self) -> Vec<ReceiptKeyEntry> {
        let genesis: Value = serde_json::from_slice(
            &fs::read(self.home.join("config/genesis.json")).or_fail("governed genesis"),
        )
        .or_fail("genesis JSON");
        genesis["app_state"]["sequencer"]["receipt_keys"]
            .as_array()
            .or_fail("governed receipt key list")
            .iter()
            .map(|key| ReceiptKeyEntry {
                key_id: base64::engine::general_purpose::STANDARD
                    .decode(key["key_id"].as_str().or_fail("governed key ID"))
                    .or_fail("base64 key ID")
                    .try_into()
                    .or_fail("key ID width"),
                public_key: base64::engine::general_purpose::STANDARD
                    .decode(key["public_key"].as_str().or_fail("governed public key"))
                    .or_fail("base64 public key")
                    .try_into()
                    .or_fail("public key width"),
                first_height: key["first_height"].as_u64().or_fail("first key height"),
                last_height: key["last_height"].as_u64().or_fail("last key height"),
            })
            .collect()
    }

    pub(crate) fn receipt(
        &self,
        hash: [u8; 32],
        keys: &[ReceiptKeyEntry],
        rung: Rung,
    ) -> AuthenticatedReceipt {
        let mut deadline = Deadline::start(self.clock.as_ref(), Duration::from_secs(60))
            .or_fail("receipt deadline");
        loop {
            let observed = if rung == Rung::Committed {
                self.committed_receipt(hash, keys)
            } else {
                lookup_authenticated(&self.endpoint, hash, LookupContext { receipt_keys: keys })
            };
            match observed.unwrap_or_else(|error| {
                panic!("{}: {error:?}", "authenticate real sequencer receipt")
            }) {
                Lookup::Verified(receipt) if receipt.rung() >= rung => return receipt,
                Lookup::Absent | Lookup::Submitted { .. } | Lookup::Verified(_) => {}
            }
            assert!(
                !deadline
                    .remaining(self.clock.as_ref())
                    .or_fail("receipt clock")
                    .is_zero(),
                "transaction did not reach {rung:?}; evidence: {}",
                self.home.display()
            );
            thread::sleep(Duration::from_millis(10));
        }
    }

    pub(crate) fn committed_receipt(
        &self,
        hash: [u8; 32],
        keys: &[ReceiptKeyEntry],
    ) -> Result<Lookup, layerx_client::rpc::receipt::ReceiptError> {
        let Some(layerx_client::sequencer::SequencerReceipt::Committed {
            block_number: height,
            ..
        }) = self.endpoint.sequencer_receipt(&hash)?
        else {
            return lookup_authenticated(
                &self.endpoint,
                hash,
                LookupContext { receipt_keys: keys },
            );
        };
        let body = serde_json::json!({"jsonrpc":"2.0", "id":1,
            "method":"px_getSequenceBatch", "params":[height.to_string()]});
        let mut response = ureq::post(&self.rpc_url)
            .header("Content-Type", "application/json")
            .send(serde_json::to_vec(&body).or_fail("sequence batch request"))
            .or_fail("node sequence batch");
        let answer: Value = serde_json::from_str(
            &response
                .body_mut()
                .read_to_string()
                .or_fail("sequence batch body"),
        )
        .or_fail("sequence batch JSON");
        assert!(
            answer.get("error").is_none(),
            "sequence batch refused: {answer}"
        );
        let batch = &answer["result"];
        let proof = &batch["proof"];
        let path = layerx_programs::chain_proof::merkle::SimpleProof {
            total: proof["total"]
                .as_u64()
                .or_fail("proof total")
                .try_into()
                .or_fail("bounded proof total"),
            index: proof["index"]
                .as_u64()
                .or_fail("proof index")
                .try_into()
                .or_fail("bounded proof index"),
            leaf_hash: unhex(proof["leafHash"].as_str().or_fail("proof leaf"))
                .try_into()
                .or_fail("leaf hash width"),
            aunts: proof["aunts"]
                .as_array()
                .or_fail("proof aunts")
                .iter()
                .map(|aunt| {
                    unhex(aunt.as_str().or_fail("proof aunt"))
                        .try_into()
                        .or_fail("aunt width")
                })
                .collect(),
        };
        let transaction = unhex(batch["tx"].as_str().or_fail("sequence batch transaction"));
        let activity_ids: Vec<[u8; 32]> = batch["activityIds"]
            .as_array()
            .or_fail("batch activities")
            .iter()
            .map(|id| {
                unhex(id.as_str().or_fail("activity ID"))
                    .try_into()
                    .or_fail("activity width")
            })
            .collect();
        let evidence = ReceiptCommitEvidence {
            verifier: &self.verifier,
            covering_height: height,
            batch_transaction: &transaction,
            transaction_proof: &path,
            activity_ids: &activity_ids,
        };
        lookup_authenticated_with_commit(
            &self.endpoint,
            hash,
            LookupContext { receipt_keys: keys },
            &evidence,
        )
    }
}

impl Drop for BankNode {
    fn drop(&mut self) {
        let status = Command::new("bash")
            .arg(&self.script)
            .arg("--stop")
            .arg(&self.home)
            .stdout(Stdio::null())
            .stderr(Stdio::null())
            .status()
            .or_fail("stop disposable sequencer");
        assert!(status.success(), "disposable sequencer did not stop");
    }
}

pub(crate) fn pending<T>(
    clock: &dyn Clock,
    mut operation: impl FnMut() -> Result<T, SequencerError>,
) -> T {
    let mut deadline = Deadline::start(clock, Duration::from_secs(20)).or_fail("pending deadline");
    loop {
        match operation() {
            Ok(value) => return value,
            Err(SequencerError::Rpc(refusal))
                if refusal.message == "pending block unavailable: sequencer: no block is open"
                    || refusal.message == "sequencer: no block is open" =>
            {
                assert!(
                    !deadline.remaining(clock).or_fail("pending clock").is_zero(),
                    "sequencer did not open a block"
                );
                thread::sleep(Duration::from_millis(10));
            }
            Err(error) => panic!("real sequencer refused bank operation: {error:?}"),
        }
    }
}

pub(crate) fn submit(node: &BankNode, raw: &[u8]) -> [u8; 32] {
    let hash = transaction_hash(raw);
    match pending(node.clock.as_ref(), || {
        match node.endpoint.send_raw_transaction(raw) {
            Err(SequencerError::Rpc(refusal))
                if refusal.message == "sequencer: transaction held at submitted awaiting nonce" =>
            {
                Ok(None)
            }
            result => result.map(Some),
        }
    }) {
        Some(returned) => assert_eq!(returned, hash),
        None => {
            let submitted = node
                .endpoint
                .sequencer_receipt(&hash)
                .or_fail("retained queued transaction")
                .or_fail("queued transaction exists");
            let (call, _) = decode_signed(raw).or_fail("queued signed transaction");
            match submitted {
                layerx_client::sequencer::SequencerReceipt::Submitted { tx_hash, nonce } => {
                    assert_eq!(tx_hash, hash);
                    assert_eq!(nonce, call.nonce);
                }
                layerx_client::sequencer::SequencerReceipt::Receipted(receipt)
                | layerx_client::sequencer::SequencerReceipt::Committed { receipt, .. } => {
                    assert_eq!(receipt.tx_hash, hash);
                    assert_eq!(receipt.nonce, call.nonce);
                }
            }
        }
    }
    hash
}

pub(crate) fn native_pointer(
    node: &BankNode,
    sender: [u8; 20],
    keys: &[ReceiptKeyEntry],
) -> [u8; 20] {
    let mut view = [0; 20];
    view[18..].copy_from_slice(&0x100a_u16.to_be_bytes());
    let mut data = ledger_selector("getNativePointer(string)").to_vec();
    let mut offset = [0; 32];
    offset[31] = 32;
    data.extend(offset);
    let mut length = [0; 32];
    length[31] = 4;
    data.extend(length);
    let mut denom = [0; 32];
    denom[..4].copy_from_slice(b"uhpx");
    data.extend(denom);
    let mut result = pending(node.clock.as_ref(), || {
        node.endpoint.call_pending(&view, &data)
    });
    assert_eq!(result.len(), 96, "native pointer view ABI");
    if result[64..] == [0; 32] {
        let mut registry = [0; 20];
        registry[18..].copy_from_slice(&0x100b_u16.to_be_bytes());
        let mut registration_data = data.clone();
        registration_data[..4].copy_from_slice(&ledger_selector("addNativePointer(string)"));
        let call = Eip1559Call {
            chain_id: 125,
            nonce: pending(node.clock.as_ref(), || node.endpoint.pending_nonce(&sender)),
            max_priority_fee_per_gas: 1,
            max_fee_per_gas: 1_000_000_000_000,
            gas_limit: 5_000_000,
            to: registry,
            data: registration_data,
        };
        let raw = call
            .sign(&node.secret)
            .or_fail("sign native pointer registration");
        let hash = transaction_hash(&raw);
        assert_eq!(submit(node, &raw), hash);
        let receipt = node.receipt(hash, keys, Rung::Committed);
        assert!(
            receipt.execution().success,
            "native pointer registration reverted: {:?}; node evidence: {}",
            String::from_utf8_lossy(&receipt.execution().return_data),
            node.home.display()
        );
        result = pending(node.clock.as_ref(), || {
            node.endpoint.call_pending(&view, &data)
        });
    }
    assert_eq!(result.len(), 96, "registered native pointer view ABI");
    assert_eq!(&result[..12], &[0; 12]);
    assert_eq!(&result[64..95], &[0; 31]);
    assert_eq!(result[95], 1, "native pointer must exist");
    let pointer = result[12..32].try_into().or_fail("native pointer address");
    assert_ne!(pointer, [0; 20]);
    pointer
}

impl BankNode {
    pub(crate) fn rpc(&self, method: &str, params: &Value) -> Result<Value, Value> {
        let body = serde_json::json!({"jsonrpc":"2.0", "id":1, "method":method, "params":params});
        let mut response = ureq::post(&self.rpc_url)
            .header("Content-Type", "application/json")
            .send(serde_json::to_vec(&body).or_fail("node request"))
            .or_fail("node transport");
        let answer: Value = serde_json::from_str(
            &response
                .body_mut()
                .read_to_string()
                .or_fail("node response"),
        )
        .or_fail("node JSON");
        match answer.get("error") {
            Some(error) => Err(error.clone()),
            None => Ok(answer["result"].clone()),
        }
    }

    pub(crate) fn fund(&self, wallet: [u8; 20]) -> [u8; 20] {
        let secret_call = Eip1559Call {
            chain_id: 125,
            nonce: 0,
            max_priority_fee_per_gas: 1,
            max_fee_per_gas: 1_000_000_000_000,
            gas_limit: 1_000_000,
            to: wallet,
            data: Vec::new(),
        };
        let raw = secret_call.sign(&self.secret).or_fail("funding identity");
        let (_, signature) = decode_signed(&raw).or_fail("funding signature");
        let sender = recover_sender(&secret_call, &signature).or_fail("funding sender");
        let keys = self.receipt_keys();
        let pointer = native_pointer(self, sender, &keys);
        let mut call = secret_call;
        call.nonce = pending(self.clock.as_ref(), || self.endpoint.pending_nonce(&sender));
        call.to = pointer;
        call.data = ledger_selector("transfer(address,uint256)").to_vec();
        call.data.extend([0; 12]);
        call.data.extend(wallet);
        let mut amount = [0; 32];
        amount[16..].copy_from_slice(&1_000_000_000_000_u128.to_be_bytes());
        call.data.extend(amount);
        let raw = call.sign(&self.secret).or_fail("fund wallet");
        let receipt = self.receipt(submit(self, &raw), &keys, Rung::Committed);
        assert!(receipt.execution().success, "wallet funding succeeds");
        assert_eq!(
            receipt.execution().return_data,
            [vec![0; 31], vec![1]].concat()
        );
        pointer
    }
}
