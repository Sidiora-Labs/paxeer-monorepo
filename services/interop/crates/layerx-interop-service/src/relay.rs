use layerx_client::sequencer::{SequencerEndpoint, SequencerError, SequencerReceipt};
use layerx_interop_gateway::evidence::{SequencerEvidence, SettlementRung};
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_proof::receipt::{
    verify_sequencer_signature, ReceiptCheck, ReceiptKeys, VerificationFailure,
};
use layerx_proof::sequencer_logs::{verify_logs, SequencerLog, SequencerLogRefusal};

use crate::payer_tx::{decode_pay_exact, PayerTransactionRefusal};

pub(crate) struct ChainRelay<'a> {
    pub(crate) endpoint: &'a SequencerEndpoint,
    pub(crate) receipt_keys: &'a [SequencerAuthorization],
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub(crate) enum RelayOutcome {
    Pending { tx_hash: [u8; 32] },
    Executed(SequencerEvidence),
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub(crate) enum RelayError {
    Transaction(PayerTransactionRefusal),
    Sequencer(SequencerError),
    TransactionHashMismatch,
    ReceiptRejected(VerificationFailure),
    LogsRejected(SequencerLogRefusal),
    Unknown,
}

impl std::fmt::Display for RelayError {
    fn fmt(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::Transaction(error) => error.fmt(formatter),
            error => write!(formatter, "relay refused: {error:?}"),
        }
    }
}

impl std::error::Error for RelayError {}

impl ChainRelay<'_> {
    pub(crate) fn relay(
        &self,
        raw: &[u8],
        expected_sender: [u8; 20],
    ) -> Result<RelayOutcome, RelayError> {
        let (_, sender, tx_hash) = decode_pay_exact(raw).map_err(RelayError::Transaction)?;
        if sender != expected_sender {
            return Err(RelayError::Transaction(
                PayerTransactionRefusal::SignatureRefused,
            ));
        }
        let submitted_hash = self
            .endpoint
            .send_raw_transaction(raw)
            .map_err(RelayError::Sequencer)?;
        if submitted_hash != tx_hash {
            return Err(RelayError::TransactionHashMismatch);
        }
        let answer = self
            .endpoint
            .sequencer_receipt(&tx_hash)
            .map_err(RelayError::Sequencer)?;
        let (executed, rung) = match answer {
            Some(SequencerReceipt::Submitted { .. }) => {
                return Ok(RelayOutcome::Pending { tx_hash });
            }
            Some(SequencerReceipt::Receipted(receipt)) => (receipt, SettlementRung::Receipted),
            Some(SequencerReceipt::Committed {
                receipt,
                block_number,
                block_hash,
            }) => (
                receipt,
                SettlementRung::Committed {
                    block_number,
                    block_hash,
                },
            ),
            None => return Err(RelayError::Unknown),
        };
        let verified = verify_sequencer_signature(
            &executed.receipt,
            ReceiptKeys::HeightRanges(self.receipt_keys),
        )
        .map_err(RelayError::ReceiptRejected)?;
        if verified.protocol().map(|receipt| receipt.activity_id()) != Some(tx_hash) {
            return Err(RelayError::ReceiptRejected(VerificationFailure {
                check: ReceiptCheck::ActivityId,
            }));
        }
        let logs = executed
            .logs
            .into_iter()
            .map(|log| SequencerLog::new(log.address, log.topics, log.data, log.log_index))
            .collect();
        let logs = verify_logs(&verified, logs)
            .map_err(RelayError::LogsRejected)?
            .into_logs();
        Ok(RelayOutcome::Executed(SequencerEvidence {
            transaction_hash: tx_hash,
            signed_receipt: executed.receipt,
            logs,
            rung,
        }))
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use layerx_platform_gateway::http;
    use serde_json::{json, Value};
    use std::net::TcpListener;
    use std::thread;
    use std::time::{Duration, Instant};

    type TestResult<T = ()> = Result<T, Box<dyn std::error::Error>>;

    fn fixture() -> TestResult<Value> {
        let fixtures: Value = serde_json::from_str(include_str!(
            "../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
        ))?;
        fixtures["cases"]
            .as_array()
            .ok_or("fixture cases must be an array")?
            .iter()
            .find(|case| case["id"] == "V1-layerxledger-pay-exact")
            .cloned()
            .ok_or_else(|| "missing recorded payExact fixture".into())
    }

    fn hex(value: &Value) -> TestResult<Vec<u8>> {
        let encoded = value
            .as_str()
            .and_then(|value| value.strip_prefix("0x"))
            .ok_or("expected prefixed fixture hex")?;
        if encoded.len() % 2 != 0 {
            return Err("fixture hex must have an even length".into());
        }
        encoded
            .as_bytes()
            .chunks_exact(2)
            .map(|pair| Ok(u8::from_str_radix(std::str::from_utf8(pair)?, 16)?))
            .collect()
    }

    fn fixed<const N: usize>(value: &Value) -> TestResult<[u8; N]> {
        hex(value)?
            .try_into()
            .map_err(|_| "fixture hex width".into())
    }

    fn pinned_key(fixture: &Value) -> TestResult<SequencerAuthorization> {
        let pin = &fixture["pinnedKey"];
        Ok(SequencerAuthorization::new(
            fixed(&pin["keyId"])?,
            fixed(&pin["publicKey"])?,
            pin["firstHeight"].as_u64().ok_or("first height")?,
            pin["lastHeight"].as_u64().ok_or("last height")?,
        ))
    }

    fn run_relay(
        fixture: &Value,
        receipt_answer: Value,
        receipt_keys: &[SequencerAuthorization],
    ) -> TestResult<Result<RelayOutcome, RelayError>> {
        let listener = TcpListener::bind("127.0.0.1:0")?;
        listener.set_nonblocking(true)?;
        let endpoint = SequencerEndpoint::new(&format!("http://{}", listener.local_addr()?), &[])
            .map_err(|error| format!("loopback endpoint: {error:?}"))?;
        let raw = hex(&fixture["rawTransaction"])?;
        let expected_sender = fixed(&fixture["receiptJson"]["from"])?;
        let raw_json = fixture["rawTransaction"].clone();
        let tx_hash_json = fixture["receiptJson"]["txHash"].clone();
        let server = thread::spawn(move || -> Result<(), String> {
            let deadline = Instant::now() + Duration::from_secs(5);
            for (method, params, result) in [
                (
                    "eth_sendRawTransaction",
                    json!([raw_json]),
                    tx_hash_json.clone(),
                ),
                (
                    "pax_sequencerReceipt",
                    json!([tx_hash_json]),
                    receipt_answer,
                ),
            ] {
                let mut stream = loop {
                    match listener.accept() {
                        Ok((stream, _)) => break stream,
                        Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                            if Instant::now() >= deadline {
                                return Err(format!("timed out waiting for {method}"));
                            }
                            thread::sleep(Duration::from_millis(5));
                        }
                        Err(error) => return Err(error.to_string()),
                    }
                };
                stream
                    .set_read_timeout(Some(Duration::from_secs(5)))
                    .map_err(|error| error.to_string())?;
                stream
                    .set_write_timeout(Some(Duration::from_secs(5)))
                    .map_err(|error| error.to_string())?;
                let request = http::read_request(&mut stream, 64 * 1024)?;
                let rpc: Value =
                    serde_json::from_slice(&request.body).map_err(|error| error.to_string())?;
                assert_eq!(request.method, "POST");
                assert_eq!(rpc["jsonrpc"], "2.0");
                assert_eq!(rpc["method"], method);
                assert_eq!(rpc["params"], params);
                let body = serde_json::to_vec(&json!({
                    "jsonrpc": "2.0", "id": rpc["id"], "result": result,
                }))
                .map_err(|error| error.to_string())?;
                http::write_response(
                    &mut stream,
                    &http::OutgoingResponse {
                        content_type: "application/json".to_owned(),
                        headers: Vec::new(),
                        status: 200,
                        body,
                        retry_after: None,
                    },
                )?;
            }
            Ok(())
        });
        let relay = ChainRelay {
            endpoint: &endpoint,
            receipt_keys,
        };
        assert_eq!(
            relay.relay(&raw, [0; 20]),
            Err(RelayError::Transaction(
                PayerTransactionRefusal::SignatureRefused
            )),
        );
        assert_eq!(
            RelayError::Transaction(PayerTransactionRefusal::SignatureRefused).to_string(),
            "transaction_signature_refused",
        );
        let outcome = relay.relay(&raw, expected_sender);
        server
            .join()
            .map_err(|_| "loopback JSON-RPC listener panicked")??;
        Ok(outcome)
    }

    #[test]
    fn relays_the_recorded_pay_exact_to_receipted_evidence() -> TestResult {
        let fixture = fixture()?;
        let outcome = run_relay(
            &fixture,
            fixture["receiptJson"].clone(),
            &[pinned_key(&fixture)?],
        )??;
        let RelayOutcome::Executed(evidence) = outcome else {
            panic!("the recorded receipted answer must return executed evidence");
        };
        assert_eq!(evidence.rung, SettlementRung::Receipted);
        assert_eq!(
            evidence.transaction_hash,
            fixed(&fixture["receiptJson"]["txHash"])?
        );
        assert_eq!(evidence.signed_receipt, hex(&fixture["signedReceipt"])?);
        assert_eq!(evidence.logs.len(), 1);
        let log = &fixture["receiptJson"]["logs"][0];
        assert_eq!(evidence.logs[0].address(), fixed(&log["address"])?);
        let topics: Vec<[u8; 32]> = log["topics"]
            .as_array()
            .ok_or("log topics")?
            .iter()
            .map(fixed)
            .collect::<TestResult<_>>()?;
        assert_eq!(evidence.logs[0].topics(), topics);
        assert_eq!(evidence.logs[0].data(), hex(&log["data"])?);
        assert_eq!(evidence.logs[0].log_index(), 0);
        Ok(())
    }

    #[test]
    fn a_submitted_answer_is_pending_with_the_tx_hash() -> TestResult {
        let fixture = fixture()?;
        let submitted = json!({
            "rung": "submitted",
            "txHash": fixture["receiptJson"]["txHash"],
            "nonce": fixture["receiptJson"]["nonce"],
        });
        let outcome = run_relay(&fixture, submitted, &[pinned_key(&fixture)?])??;
        assert_eq!(
            outcome,
            RelayOutcome::Pending {
                tx_hash: fixed(&fixture["receiptJson"]["txHash"])?,
            }
        );
        Ok(())
    }

    #[test]
    fn a_receipt_signed_by_an_unpinned_key_is_refused() -> TestResult {
        let fixture = fixture()?;
        let pinned = pinned_key(&fixture)?;
        let other_key = SequencerAuthorization::new(
            [0x7a; 32],
            ed25519_dalek::SigningKey::from_bytes(&[0x7a; 32])
                .verifying_key()
                .to_bytes(),
            pinned.first_batch_number(),
            pinned.last_batch_number(),
        );
        let outcome = run_relay(&fixture, fixture["receiptJson"].clone(), &[other_key])?;
        assert_eq!(
            outcome,
            Err(RelayError::ReceiptRejected(VerificationFailure {
                check: ReceiptCheck::SequencerSignature,
            }))
        );
        Ok(())
    }
}
