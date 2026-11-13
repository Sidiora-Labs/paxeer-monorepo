use base64::{engine::general_purpose::STANDARD, Engine as _};
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};

/// Immutable node output: no signing key and no locally generated block roots.
#[derive(Clone)]
pub(crate) struct NodeRecording {
    pub(crate) settlement: Value,
    pub(crate) authorization: Value,
}

impl NodeRecording {
    pub(crate) fn open() -> Self {
        let recording: Value = serde_json::from_str(include_str!(concat!(
            env!("CARGO_MANIFEST_DIR"),
            "/../../../../daemon/committed-fixture/vectors/vector-v4-committed.json"
        )))
        .or_fail("committed paxd recording");
        Self {
            settlement: recording["settlement"].clone(),
            authorization: recording["authorization"].clone(),
        }
    }

    pub(crate) fn receipt(&self) -> &Value {
        &self.settlement["receipt"]
    }

    pub(crate) fn answer(&self, method: &str, params: &Value) -> Result<Value, Value> {
        let s = &self.settlement;
        let receipt = self.receipt();
        let height = receipt["height"].as_str().or_fail("recorded height");
        let hash = &receipt["txHash"];
        let at_transaction = || params == &json!([hash]);
        let at_height = || params[0] == s["execution_header"]["height"] || params[0] == height;
        match method {
            "pax_sequencerReceipt" if at_transaction() => Ok(receipt.clone()),
            "eth_getRawTransactionByHash" if at_transaction() => Ok(s["raw_transaction"].clone()),
            "eth_getTransactionReceipt" if at_transaction() => Ok(json!({
                "transactionHash": hash, "transactionIndex": receipt["index"],
                "blockNumber": receipt["blockNumber"], "blockHash": receipt["blockHash"],
                "status": receipt["status"], "gasUsed": receipt["gasUsed"],
                "logs": receipt["logs"]
            })),
            "px_getReceiptKeys" if params == &json!([]) => {
                let public_key: [u8; 32] =
                    serde_json::from_value(self.authorization["PublicKey"].clone())
                        .or_fail("recorded public key");
                let hex = |bytes: &[u8]| format!("0x{}", layerx_human_service::hex::lower(bytes));
                Ok(json!({"keys": [{
                    "keyId": hex(&Sha256::digest(public_key)), "publicKey": hex(&public_key),
                    "firstHeight": self.authorization["FirstBatchNumber"],
                    "lastHeight": self.authorization["LastBatchNumber"]
                }]}))
            }
            // The recording retains the Comet block, including the real tx-zero batch.
            // Do not manufacture an Ethereum block projection or Ethereum trie roots.
            "block" if at_height() => Ok(json!({
                "block_id": s["execution_commit"]["block_id"],
                "block": {"header": s["execution_header"], "data": {"txs": [
                    s["sequence_batch_proof"]["data"], s["transaction_proof"]["data"]
                ]}}
            })),
            "commit" if at_height() => Ok(json!({"signed_header": {
                "header": s["execution_header"], "commit": s["execution_commit"]
            }})),
            "commit" if params[0] == s["header"]["height"] => Ok(json!({"signed_header": {
                "header": s["header"], "commit": s["commit"]
            }})),
            "validators" if at_height() || params[0] == s["header"]["height"] => {
                Ok(json!({"validators": s["validators"]}))
            }
            "tx" => {
                for (index, proof_name) in ["sequence_batch_proof", "transaction_proof"]
                    .iter()
                    .enumerate()
                {
                    let proof = &s[proof_name];
                    let tx = STANDARD
                        .decode(proof["data"].as_str().or_fail("recorded tx"))
                        .or_fail("node tx base64");
                    let digest = Sha256::digest(tx);
                    let tx_hash =
                        format!("0x{}", layerx_human_service::hex::lower((digest).as_ref()));
                    if params[0] == tx_hash {
                        return Ok(json!({
                            "height": s["execution_header"]["height"], "index": index,
                            "tx": proof["data"], "proof": proof,
                            "tx_result": if index == 1 { s["exec_tx_result"].clone() } else { Value::Null }
                        }));
                    }
                }
                Err(
                    json!({"code": -32001, "message": "transaction is absent from the node recording"}),
                )
            }
            _ => Err(json!({"code": -32001,
                "message": "no matching node-emitted evidence in the committed recording"})),
        }
    }
}

use crate::require_value::RequireValue as _;
