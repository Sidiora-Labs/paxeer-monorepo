use std::error::Error;
use std::future::Future;
use std::pin::pin;
use std::sync::Arc;
use std::task::{Context, Poll, Wake, Waker};

use layerx_crypto::account_derivation::derive_from_seed;
use layerx_crypto::binding::{BindAuthorization, BindMessage, ADDR_PRECOMPILE, BIND_SELECTOR};
use layerx_crypto::disclosure::call::{bind_call, reencode, CallFeeContext, ConstructMethod};
use layerx_crypto::disclosure::DisclosureError;
use layerx_crypto::evm_transaction::{recover_sender, Eip1559Call};
use layerx_crypto::send::CallOptions;
use layerx_crypto::signer::{sign_disclosed_call, BindSigner, LocalSigner, SignError, Signer};
use layerx_types::payload::{ActivityType, ModuleId};

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

fn registration() -> Result<(LocalSigner, BindAuthorization, Eip1559Call, [u8; 20]), Box<dyn Error>>
{
    let account = derive_from_seed(&[0x07; 64], 0)?;
    let signer = LocalSigner::from_derived_account(&account);
    let from = signer.evm_address().ok_or("missing EVM address")?;
    let options = CallOptions {
        from,
        chain_id: 125,
        nonce: 9,
        max_priority_fee_per_gas: 2,
        max_fee_per_gas: 30,
        gas_limit: 100_000,
    };
    let mut chain = [0; 32];
    chain[24..].copy_from_slice(&options.chain_id.to_be_bytes());
    let message = BindMessage::new(chain, from, 2);
    let signature = ready(signer.sign_bind(&message))?;
    let authorization = BindAuthorization {
        message,
        public_key: signer.public_key(),
        signature: *signature.as_bytes(),
    };
    let call = authorization.to_call(&options)?;
    Ok((signer, authorization, call, from))
}

#[test]
fn bind_layerx_discloses_the_produced_registration() -> Result<(), Box<dyn Error>> {
    let (_, authorization, call, from) = registration()?;
    assert_eq!(call.to, ADDR_PRECOMPILE);
    assert_eq!(call.data.len(), 164);
    assert_eq!(call.data[..4], BIND_SELECTOR);
    let disclosure = bind_call(&call, from)?;
    let method = disclosure.construct_method()?.ok_or("missing construct")?;
    assert_eq!(
        method,
        ConstructMethod::BindLayerX {
            public_key: authorization.public_key,
            signature: authorization.signature,
        }
    );
    assert_eq!(method.abi_signature(), "bindLayerX(bytes32,bytes)");
    assert_eq!(
        disclosure.activity_type,
        ActivityType::new(ModuleId::Governance, 1)
            .map_err(|_| DisclosureError::MalformedPayload)?
    );
    assert_eq!(disclosure.actor, from);
    assert_eq!(disclosure.authority, from);
    assert!(disclosure.counterparties.is_empty());
    assert!(disclosure.amounts.is_empty());
    assert_eq!(disclosure.asset, [0; 32]);
    assert_eq!(disclosure.fee_limit, 3_000_000);
    assert!(disclosure.onboarding.is_none());
    assert_eq!(disclosure.signing_digest()?, call.signing_hash());
    disclosure.validate()?;
    assert_eq!(reencode(&disclosure)?, call.encode_unsigned());
    let money = disclosure.call_monetary(CallFeeContext::GasOnly)?;
    assert!(money.amounts.is_empty());
    assert_eq!(money.gas_fee_bound.chain_id, call.chain_id);
    assert_eq!(money.gas_fee_bound.amount_wei, disclosure.fee_limit);
    assert_eq!(money.execution_fee_bound, None);
    Ok(())
}

#[test]
fn bind_layerx_refuses_a_32_byte_signature() -> Result<(), Box<dyn Error>> {
    let (_, _, mut call, from) = registration()?;
    call.data[99] = 32;
    call.data.truncate(132);
    assert_eq!(
        bind_call(&call, from),
        Err(DisclosureError::MalformedPayload)
    );
    Ok(())
}

#[test]
fn bind_layerx_refuses_noncanonical_offset_and_length_words() -> Result<(), Box<dyn Error>> {
    let (_, _, call, from) = registration()?;
    for index in 36..100 {
        let mut changed = call.clone();
        changed.data[index] ^= 1;
        assert_eq!(
            bind_call(&changed, from),
            Err(DisclosureError::MalformedPayload),
            "changed ABI word byte {index}"
        );
    }
    Ok(())
}

#[test]
fn bind_layerx_refuses_truncated_and_trailing_bytes() -> Result<(), Box<dyn Error>> {
    let (_, _, call, from) = registration()?;
    for length in 0..call.data.len() {
        let mut changed = call.clone();
        changed.data.truncate(length);
        assert_eq!(
            bind_call(&changed, from),
            Err(DisclosureError::MalformedPayload),
            "truncated calldata length {length}"
        );
    }
    let mut trailing = call;
    trailing.data.push(0);
    assert_eq!(
        bind_call(&trailing, from),
        Err(DisclosureError::MalformedPayload)
    );
    Ok(())
}

#[test]
fn bind_layerx_refuses_other_selectors_at_the_addr_precompile() -> Result<(), Box<dyn Error>> {
    let (_, _, call, from) = registration()?;
    for selector in [[0; 4], [0xa9, 0x05, 0x9c, 0xbb], [0xdd, 0x9a, 0xa6, 0x29]] {
        let mut changed = call.clone();
        changed.data[..4].copy_from_slice(&selector);
        assert_eq!(
            bind_call(&changed, from),
            Err(DisclosureError::MalformedPayload)
        );
    }
    let mut wrong_target = call;
    wrong_target.to[0] = 1;
    assert_eq!(
        bind_call(&wrong_target, from),
        Err(DisclosureError::MalformedPayload)
    );
    Ok(())
}

#[test]
fn bind_layerx_signs_through_the_disclosure_path() -> Result<(), Box<dyn Error>> {
    let (signer, authorization, call, from) = registration()?;
    let disclosure = bind_call(&call, from)?;
    let signature = ready(sign_disclosed_call(&signer, &call, &disclosure, from))?;
    assert_eq!(signature.as_bytes().len(), 65);
    assert!(matches!(signature.as_bytes()[64], 27 | 28));
    let recovered = recover_sender(&call, &signature)?;
    assert_eq!(recovered, from);
    assert_eq!(signature, ready(signer.sign_transaction(&call))?);
    assert_eq!(
        BindAuthorization::from_call(&call, recovered, 2)?,
        authorization
    );
    for index in [4, 100] {
        let mut changed = call.clone();
        changed.data[index] ^= 1;
        assert_eq!(
            ready(sign_disclosed_call(&signer, &changed, &disclosure, from)),
            Err(SignError::DisclosureMismatch("call"))
        );
    }
    let mut changed = disclosure;
    changed.fee_limit += 1;
    assert_eq!(
        ready(sign_disclosed_call(&signer, &call, &changed, from)),
        Err(SignError::DisclosureMismatch("call"))
    );
    Ok(())
}
