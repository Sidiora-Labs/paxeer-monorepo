use std::error::Error;
use std::future::Future;
use std::pin::pin;
use std::sync::Arc;
use std::task::{Context, Poll, Wake, Waker};

use layerx_crypto::account_derivation::derive_from_seed;
use layerx_crypto::disclosure::call::bind_call;
use layerx_crypto::evm_transaction::{recover_sender, Eip1559Call};
use layerx_crypto::signer::{sign_disclosed_call, LocalSigner, SignError, Signer};

struct NoopWake;

impl Wake for NoopWake {
    fn wake(self: Arc<Self>) {}
}

fn ready<F: Future>(future: F) -> F::Output {
    let mut future = pin!(future);
    let waker = Waker::from(Arc::new(NoopWake));
    let mut context = Context::from_waker(&waker);
    match future.as_mut().poll(&mut context) {
        Poll::Ready(value) => value,
        Poll::Pending => panic!("local signer future unexpectedly blocked"),
    }
}

fn call() -> Eip1559Call {
    let mut to = [0; 20];
    to[18..].copy_from_slice(&[0x10, 0x1a]);
    let mut data = vec![0x58, 0xd1, 0x83, 0xa8];
    data.extend_from_slice(&[0x31; 32]);
    let mut revocation_sequence = [0; 32];
    revocation_sequence[31] = 7;
    data.extend_from_slice(&revocation_sequence);
    Eip1559Call {
        chain_id: 125,
        nonce: 3,
        max_priority_fee_per_gas: 2,
        max_fee_per_gas: 30,
        gas_limit: 100_000,
        to,
        data,
    }
}

#[test]
fn sign_disclosed_call_signs_the_eip1559_preimage() -> Result<(), Box<dyn Error>> {
    let account = derive_from_seed(&[0x07; 64], 0)?;
    let signer = LocalSigner::from_derived_account(&account);
    let from = signer.evm_address().ok_or("missing EVM address")?;
    assert_eq!(from, account.evm_address());
    let call = call();
    let disclosure = bind_call(&call, from)?;
    let signature = ready(sign_disclosed_call(&signer, &call, &disclosure, from))?;

    assert_eq!(signature.as_bytes().len(), 65);
    assert!(matches!(signature.as_bytes()[64], 27 | 28));
    assert_eq!(recover_sender(&call, &signature)?, from);
    assert_eq!(signature, ready(signer.sign_transaction(&call))?);

    let mut changed = call.clone();
    changed.nonce += 1;
    assert_ne!(recover_sender(&changed, &signature)?, from);
    Ok(())
}

#[test]
fn sign_disclosed_call_refuses_a_changed_call() -> Result<(), Box<dyn Error>> {
    let account = derive_from_seed(&[0x07; 64], 0)?;
    let signer = LocalSigner::from_derived_account(&account);
    let from = signer.evm_address().ok_or("missing EVM address")?;
    let call = call();
    let disclosure = bind_call(&call, from)?;

    for field in 0..7 {
        let mut changed = call.clone();
        match field {
            0 => changed.chain_id += 1,
            1 => changed.nonce += 1,
            2 => changed.max_priority_fee_per_gas += 1,
            3 => changed.max_fee_per_gas += 1,
            4 => changed.gas_limit += 1,
            5 => changed.data[4] ^= 1,
            _ => changed.data[67] += 1,
        }
        assert_eq!(
            ready(sign_disclosed_call(&signer, &changed, &disclosure, from)),
            Err(SignError::DisclosureMismatch("call")),
            "changed field {field}"
        );
    }

    let mut changed_sender = from;
    changed_sender[0] ^= 1;
    assert_eq!(
        ready(sign_disclosed_call(
            &signer,
            &call,
            &disclosure,
            changed_sender
        )),
        Err(SignError::DisclosureMismatch("call"))
    );
    let mut changed_disclosure = disclosure.clone();
    changed_disclosure.fee_limit += 1;
    assert_eq!(
        ready(sign_disclosed_call(
            &signer,
            &call,
            &changed_disclosure,
            from
        )),
        Err(SignError::DisclosureMismatch("call"))
    );

    let keyless = LocalSigner::new([0x53; 32]);
    assert_eq!(
        ready(sign_disclosed_call(
            &keyless,
            &call,
            &changed_disclosure,
            from
        )),
        Err(SignError::DisclosureMismatch("call"))
    );
    assert_eq!(
        ready(sign_disclosed_call(&keyless, &call, &disclosure, from)),
        Err(SignError::EvmKeyUnavailable)
    );
    let mut unsupported = call.clone();
    unsupported.to[0] = 1;
    assert_eq!(
        ready(sign_disclosed_call(
            &signer,
            &unsupported,
            &disclosure,
            from
        )),
        Err(SignError::InvalidDisclosure)
    );
    Ok(())
}
