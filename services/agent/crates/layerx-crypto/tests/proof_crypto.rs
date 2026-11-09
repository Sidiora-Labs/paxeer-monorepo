//! Proves: `SendDebit`, `CallOptions`, `DisclosedCall`, `Disclosure` and `KeystoreSigner` -> canonical payer authorizations and EIP-1559 calldata -> `DisclosedCall::from_call`, `Disclosure` validation and `sign_disclosed_call`.
//!
//! The daemon's `human_runtime` consumes prepared calls with `DisclosedCall::from_call`;
//! prepare/disclose and `approval/native_effect` consume canonical bytes with
//! `Disclosure::validate_canonical`; budget, capability/binding and capability/effects
//! validate public semantics before making authorization decisions. These are the
//! same production boundary functions exercised here, without a replacement signer,
//! ABI codec, transport or disclosure decoder.
//!
//! Repairs: `src/signer.rs::sign_disclosed_call` did not bind the signer to the
//! disclosed sender; `src/disclosure.rs::Disclosure::construct_method` returned a
//! method from an altered disclosure. The listed daemon consumers now validate
//! public semantics before using them for purpose, effect and budget decisions.
//! The anchor is stateless: every observation below decodes and validates again.
//! The external OpenSSH key agent is stopped and reopened with its real saved key.

use std::error::Error;
use std::future::Future;
use std::path::PathBuf;
use std::pin::pin;
use std::process::{Child, Command, Stdio};
use std::sync::Arc;
use std::task::{Context, Poll, Wake, Waker};
use std::time::{Duration, Instant, SystemTime, UNIX_EPOCH};

use layerx_crypto::account_derivation::derive_from_seed;
use layerx_crypto::binding::BindMessage;
use layerx_crypto::disclosure::call::{bind_call, ConstructMethod};
use layerx_crypto::disclosure::DisclosureError;
use layerx_crypto::evm_transaction::{assemble, decode_signed, recover_sender};
use layerx_crypto::send::{
    ledger_payment_call, send_context_hash, CallOptions, DisclosedCall, SendCondition, SendDebit,
};
use layerx_crypto::signer::{sign_disclosed_call, KeystoreSigner, LocalSigner, SignError, Signer};

mod support;

type TestResult = Result<(), Box<dyn Error>>;

struct ThreadWake(std::thread::Thread);
impl Wake for ThreadWake {
    fn wake(self: Arc<Self>) {
        self.0.unpark();
    }
}

fn complete<F: Future>(future: F) -> F::Output {
    let mut future = pin!(future);
    let waker = Waker::from(Arc::new(ThreadWake(std::thread::current())));
    let mut context = Context::from_waker(&waker);
    loop {
        match future.as_mut().poll(&mut context) {
            Poll::Ready(value) => return value,
            Poll::Pending => std::thread::park(),
        }
    }
}

fn debit(amount: u128) -> SendDebit {
    SendDebit {
        from: [1; 32],
        to: [2; 32],
        asset: [3; 32],
        amount,
        source_sequence: 9,
        idempotency_key: [4; 32],
        expires_at: 100,
        context_hash: send_context_hash(&[1; 32], &[2; 32], &[3; 32], amount, &[4; 32]),
        conditions: vec![SendCondition {
            kind: 1,
            timestamp: 10,
        }],
        authorization_kind: 1,
        network_id: 17,
        protocol_version: 3,
    }
}

#[test]
fn proof_crypto_producer_to_call_consumer_twice_and_refusals() -> TestResult {
    let account = derive_from_seed(&[7; 64], 0)?;
    let signer = LocalSigner::from_derived_account(&account);
    let from = signer.evm_address().ok_or("missing EVM key")?;
    let options = CallOptions {
        from,
        chain_id: 125,
        nonce: 7,
        max_priority_fee_per_gas: 2,
        max_fee_per_gas: 30,
        gas_limit: 100_000,
    };
    let mut previous = None;
    for amount in [123, 124] {
        let payload = complete(debit(amount).sign(&signer))?;
        let produced: DisclosedCall = ledger_payment_call(&payload, &options)?;
        let mut first = None;
        for _ in 0..2 {
            // Exactly the prepared-call boundary used by human_runtime.
            let consumed = DisclosedCall::from_call(produced.call.clone(), from)?;
            assert_eq!(consumed.disclosure, produced.disclosure);
            assert_eq!(bind_call(&consumed.call, from)?, consumed.disclosure);
            assert_eq!(consumed.disclosure.actor, from);
            assert_eq!(consumed.disclosure.asset, [3; 32]);
            assert_eq!(consumed.disclosure.amounts[0].value, amount);
            assert_eq!(consumed.disclosure.fee_limit, 3_000_000);
            assert_eq!(consumed.disclosure.envelope_sequence(), options.nonce);
            assert_eq!(consumed.disclosure.payload_sequence()?, Some(9));
            assert_eq!(
                consumed.disclosure.signing_digest()?,
                consumed.call.signing_hash()
            );
            assert_eq!(
                consumed.disclosure.construct_method()?,
                Some(ConstructMethod::LedgerSend {
                    payload: payload.clone()
                })
            );
            let signature = complete(sign_disclosed_call(
                &signer,
                &consumed.call,
                &consumed.disclosure,
                from,
            ))?;
            let raw = assemble(&consumed.call, &signature);
            let (decoded, decoded_signature) = decode_signed(&raw)?;
            assert_eq!(decoded, produced.call);
            assert_eq!(recover_sender(&decoded, &decoded_signature)?, from);
            let digest = consumed.disclosure.audit_digest()?;
            if let Some(expected) = first {
                assert_eq!(digest, expected);
            }
            first = Some(digest);
            if let Some(old) = previous {
                assert_ne!(digest, old);
            }
        }
        previous = first;

        assert_producer_refusals(&signer, &produced, &payload, &options)?;
    }
    Ok(())
}

#[test]
fn proof_crypto_canonical_activity_consumer_redecodes_and_refuses_tampering() -> TestResult {
    let account = derive_from_seed(&[5; 64], 0)?;
    let signer = LocalSigner::from_derived_account(&account);
    let from = signer.evm_address().ok_or("missing EVM key")?;
    let options = CallOptions {
        from,
        chain_id: 125,
        nonce: 7,
        max_priority_fee_per_gas: 2,
        max_fee_per_gas: 30,
        gas_limit: 100_000,
    };
    let mut previous = None;
    for amount in [123, 124] {
        let payload = complete(debit(amount).sign(&signer))?;
        let produced = ledger_payment_call(&payload, &options)?;
        let canonical = produced.call.encode_unsigned();
        for _ in 0..2 {
            let call = layerx_crypto::evm_transaction::decode_unsigned(&canonical)?;
            let consumed = bind_call(&call, from)?;
            consumed.validate_canonical(&canonical)?;
            consumed.validate()?;
            assert_eq!(consumed, produced.disclosure);
            assert_eq!(consumed.amounts[0].value, amount);
            assert_eq!(consumed.counterparties[0].account, [1; 32]);
            assert_eq!(consumed.counterparties[1].account, [2; 32]);
            assert_eq!(consumed.payload_sequence()?, Some(9));
            assert_eq!(consumed.envelope_sequence(), 7);
            assert_eq!(
                consumed.construct_method()?,
                Some(ConstructMethod::LedgerSend {
                    payload: payload.clone()
                })
            );
            let signature = complete(sign_disclosed_call(&signer, &call, &consumed, from))?;
            assert_eq!(recover_sender(&call, &signature)?, from);
            if let Some(old) = previous {
                assert_ne!(consumed.audit_digest()?, old);
            }
        }
        previous = Some(produced.disclosure.audit_digest()?);
        let mut altered = produced.disclosure.clone();
        altered.counterparties[1].account = [6; 32];
        assert_eq!(
            altered.validate(),
            Err(DisclosureError::FieldMismatch("counterparties"))
        );
        assert!(altered.validate_canonical(&canonical).is_err());
        assert!(complete(sign_disclosed_call(&signer, &produced.call, &altered, from)).is_err());
        let truncated = &canonical[..canonical.len() - 1];
        assert!(layerx_crypto::evm_transaction::decode_unsigned(truncated).is_err());
        assert_eq!(
            produced.disclosure.validate_canonical(truncated),
            Err(DisclosureError::FieldMismatch("canonical_bytes"))
        );
    }
    Ok(())
}

struct OpenSshAgent {
    child: Child,
}
impl Drop for OpenSshAgent {
    fn drop(&mut self) {
        let _ = self.child.kill();
        let _ = self.child.wait();
    }
}

struct KeyDirectory(PathBuf);
impl Drop for KeyDirectory {
    fn drop(&mut self) {
        let _ = std::fs::remove_dir_all(&self.0);
    }
}

fn open_agent(directory: &KeyDirectory) -> Result<OpenSshAgent, Box<dyn Error>> {
    let socket = directory.0.join("agent.sock");
    if socket.exists() {
        std::fs::remove_file(&socket)?;
    }
    let child = Command::new("ssh-agent")
        .args(["-D", "-a"])
        .arg(&socket)
        .stdout(Stdio::null())
        .stderr(Stdio::null())
        .spawn()?;
    let mut agent = OpenSshAgent { child };
    let deadline = Instant::now() + Duration::from_secs(5);
    while !socket.exists() {
        if agent.child.try_wait()?.is_some() {
            return Err("ssh-agent exited before readiness".into());
        }
        if Instant::now() >= deadline {
            return Err("ssh-agent readiness deadline".into());
        }
        std::thread::sleep(Duration::from_millis(10));
    }
    let status = Command::new("ssh-add")
        .arg(directory.0.join("key"))
        .env("SSH_AUTH_SOCK", &socket)
        .stdout(Stdio::null())
        .stderr(Stdio::null())
        .status()?;
    if !status.success() {
        return Err("ssh-add refused the generated proof key".into());
    }
    Ok(agent)
}

#[test]
fn proof_crypto_real_keystore_transport_survives_agent_reopen_and_refuses_unknown_key() -> TestResult
{
    let unique = SystemTime::now().duration_since(UNIX_EPOCH)?.as_nanos();
    let directory = KeyDirectory(
        std::env::temp_dir().join(format!("lx-proof-{}-{unique}", std::process::id())),
    );
    std::fs::create_dir(&directory.0)?;
    let status = Command::new("ssh-keygen")
        .args(["-q", "-t", "ed25519", "-N", "", "-f"])
        .arg(directory.0.join("key"))
        .stdout(Stdio::null())
        .stderr(Stdio::null())
        .status()?;
    assert!(status.success(), "generate a real OpenSSH Ed25519 key");
    let public = std::fs::read_to_string(directory.0.join("key.pub"))?;
    let blob =
        openssl::base64::decode_block(public.split_whitespace().nth(1).ok_or("missing key blob")?)?;
    assert_eq!(&blob[..19], b"\0\0\0\x0bssh-ed25519\0\0\0\x20");
    let public_key: [u8; 32] = blob[19..].try_into()?;
    let socket = directory.0.join("agent.sock");
    let local = LocalSigner::new([5; 32]);
    let mut chain = [0; 32];
    chain[24..].copy_from_slice(&125u64.to_be_bytes());
    let produced = BindMessage::new(chain, [0xa1; 20], 7);
    let canonical = produced.as_bytes();
    let payment = support::send_call(123, 7);
    let transaction = payment.call.encode_unsigned();
    let request = support::signing::activity_request(&transaction, &payment.disclosure);
    let mut prior_signature = None;
    for _ in 0..2 {
        let agent = open_agent(&directory)?;
        let signer = KeystoreSigner::new(&socket, public_key)?.with_timeout(Duration::from_secs(2));
        assert_eq!(signer.public_key(), public_key);
        assert_eq!(signer.evm_address(), None);
        assert_eq!(
            complete(signer.sign(request)),
            Err(SignError::InvalidDisclosure)
        );
        for _ in 0..2 {
            let consumed = BindMessage::decode(canonical)?;
            assert_eq!(consumed, produced);
            let signature = complete(signer.sign_bind_bytes(consumed.as_bytes()))?;
            consumed.verify(&public_key, signature.as_bytes())?;
            if let Some(expected) = prior_signature {
                assert_eq!(signature, expected);
            }
            prior_signature = Some(signature);
        }
        let unknown = KeystoreSigner::new(&socket, local.public_key())?;
        assert_eq!(
            complete(unknown.sign_bind_bytes(canonical)),
            Err(SignError::KeystoreRefused)
        );
        drop(agent);
        assert_eq!(
            complete(signer.sign(request)),
            Err(SignError::InvalidDisclosure)
        );
        assert_eq!(
            complete(signer.sign_bind_bytes(canonical)),
            Err(SignError::KeystoreUnavailable)
        );
    }
    Ok(())
}

fn assert_producer_refusals(
    signer: &LocalSigner,
    produced: &DisclosedCall,
    payload: &[u8],
    options: &CallOptions,
) -> Result<(), Box<dyn std::error::Error>> {
    let from = options.from;
    let wrong_account = derive_from_seed(&[8; 64], 0)?;
    let wrong_signer = LocalSigner::from_derived_account(&wrong_account);
    assert_eq!(
        complete(sign_disclosed_call(
            &wrong_signer,
            &produced.call,
            &produced.disclosure,
            from,
        )),
        Err(SignError::DisclosureMismatch("signer"))
    );
    let no_evm_key = LocalSigner::new([9; 32]);
    assert_eq!(
        complete(sign_disclosed_call(
            &no_evm_key,
            &produced.call,
            &produced.disclosure,
            from,
        )),
        Err(SignError::EvmKeyUnavailable)
    );

    let mut altered = produced.disclosure.clone();
    altered.amounts[0].value += 1;
    assert_eq!(
        altered.validate(),
        Err(DisclosureError::FieldMismatch("amounts"))
    );
    assert_eq!(
        altered.construct_method(),
        Err(DisclosureError::FieldMismatch("amounts"))
    );
    assert_eq!(
        complete(sign_disclosed_call(signer, &produced.call, &altered, from,)),
        Err(SignError::DisclosureMismatch("call"))
    );
    let mut malformed = produced.call.clone();
    malformed.data.pop();
    assert!(matches!(
        DisclosedCall::from_call(malformed, from),
        Err(DisclosureError::MalformedPayload)
    ));
    let mut malformed_payload = payload.to_vec();
    malformed_payload[4] ^= 1;
    assert!(matches!(
        ledger_payment_call(&malformed_payload, options),
        Err(DisclosureError::MalformedPayload)
    ));
    let mut malformed_raw = assemble(
        &produced.call,
        &complete(sign_disclosed_call(
            signer,
            &produced.call,
            &produced.disclosure,
            from,
        ))?,
    );
    malformed_raw.push(0);
    assert!(decode_signed(&malformed_raw).is_err());

    Ok(())
}
