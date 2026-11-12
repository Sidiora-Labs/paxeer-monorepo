use layerx_crypto::disclosure::call::{self, ConstructMethod};
use layerx_crypto::disclosure::{Disclosure, DisclosureError};
use layerx_crypto::evm_transaction::Eip1559Call;
use layerx_types::payload::{ActivityType, ModuleId};

/// A call disclosure retained with its exact EIP-1559 source and sender.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct CallBinding {
    call: Eip1559Call,
    sender: [u8; 20],
    disclosure: CallDisclosure,
}

#[derive(Clone, Debug, Eq, PartialEq)]
enum CallDisclosure {
    Shared(Box<Disclosure>),
    Consent {
        payload: Vec<u8>,
        action_key: [u8; 32],
    },
}

impl CallBinding {
    /// Re-encodes the retained call using the call-specific source validator.
    ///
    /// # Errors
    /// Returns an error if the retained call or disclosure is invalid.
    pub fn reencode(&self) -> Result<Vec<u8>, DisclosureError> {
        match &self.disclosure {
            CallDisclosure::Shared(disclosure) => {
                // Validate the retained call and sender together. Activity-source
                // validation cannot re-encode an EIP-1559 approval.
                call::validate_call(&self.call, disclosure, self.sender)?;
            }
            CallDisclosure::Consent {
                payload,
                action_key,
            } => {
                validate_consent(&self.call, payload, action_key)?;
                let (actual_payload, actual_key) = lifecycle_payload(&self.call.data)?;
                if actual_payload != payload || actual_key != *action_key {
                    return Err(DisclosureError::FieldMismatch("call"));
                }
            }
        }
        Ok(self.call.encode_unsigned())
    }

    ///
    /// # Errors
    /// Returns an error if the retained call cannot be re-encoded or decoded.
    pub fn construct_method(&self) -> Result<Option<ConstructMethod>, DisclosureError> {
        self.reencode()?;
        match &self.disclosure {
            CallDisclosure::Shared(disclosure) => disclosure.construct_method(),
            CallDisclosure::Consent {
                payload,
                action_key,
            } => Ok(Some(ConstructMethod::AuthLifecycle {
                payload: payload.clone(),
                idempotency_key: *action_key,
            })),
        }
    }

    #[must_use]
    pub fn shared_disclosure(&self) -> Option<&Disclosure> {
        match &self.disclosure {
            CallDisclosure::Shared(disclosure) => Some(disclosure),
            CallDisclosure::Consent { .. } => None,
        }
    }

    /// Encodes the LXKP approval fields bound to the retained call.
    ///
    /// # Errors
    /// Returns an error if the call or disclosure cannot be validated or encoded.
    pub fn disclosure_bytes(&self) -> Result<Vec<u8>, DisclosureError> {
        self.reencode()?;
        match &self.disclosure {
            CallDisclosure::Shared(disclosure) => {
                crate::wire::disclosure(disclosure).map_err(|_| DisclosureError::MalformedPayload)
            }
            CallDisclosure::Consent {
                payload,
                action_key,
            } => {
                // The EVM consent body names account commitments, not a target
                // DID/public key; retain those exact facts in the LXKP v1 view.
                let mut out = vec![1];
                out.extend(
                    ActivityType::new(ModuleId::Governance, 1)
                        .map_err(|_| DisclosureError::MalformedPayload)?
                        .value()
                        .to_be_bytes(),
                );
                for _ in 0..2 {
                    out.extend(20_u32.to_be_bytes());
                    out.extend(self.sender);
                }
                out.extend(2_u32.to_be_bytes());
                out.push(1);
                out.extend(&payload[4..36]);
                out.push(2);
                out.extend(&payload[68..100]);
                out.extend(0_u32.to_be_bytes());
                out.extend(&payload[36..68]);
                out.extend(0_u128.to_be_bytes());
                out.extend(0_u64.to_be_bytes());
                out.extend(u64::MAX.to_be_bytes());
                out.extend(&payload[132..140]);
                out.extend(action_key);
                out.push(0);
                Ok(out)
            }
        }
    }

    ///
    /// # Errors
    /// Returns an error if the lifecycle approval has invalid fields or bounds.
    pub fn validate_for_signing(&self) -> Result<(), DisclosureError> {
        if let Some(ConstructMethod::AuthLifecycle { payload, .. }) = self.construct_method()? {
            if payload.starts_with(&[0x71, 3]) {
                if payload[4..36] == [0; 32]
                    || payload[36..68] == [0; 32]
                    || payload[68..70] == [0; 2]
                {
                    return Err(DisclosureError::MalformedPayload);
                }
                if payload.len() == 86 {
                    let minimum = u64::from_be_bytes(
                        payload[70..78]
                            .try_into()
                            .map_err(|_| DisclosureError::MalformedPayload)?,
                    );
                    let maximum = u64::from_be_bytes(
                        payload[78..86]
                            .try_into()
                            .map_err(|_| DisclosureError::MalformedPayload)?,
                    );
                    if minimum == 0 || maximum < minimum {
                        return Err(DisclosureError::MalformedPayload);
                    }
                }
            }
        }
        Ok(())
    }
}

/// Binds the production KMS call path, including native onboarding consent.
/// Malformed recovery values remain representable for transport, but are
/// refused by the server before it can reserve a nonce or request a signature.
///
/// # Errors
/// Returns an error if the call and sender cannot be bound to a disclosure.
pub fn bind_call(call: &Eip1559Call, sender: [u8; 20]) -> Result<CallBinding, DisclosureError> {
    let lifecycle = call.to[..18] == [0; 18]
        && call.to[18..] == [0x10, 0x1c]
        && call.data.get(..4)
            == Some(call::ledger_selector("recordLifecycle(bytes,bytes32)").as_slice());
    let disclosure = if lifecycle {
        let (payload, action_key) = lifecycle_payload(&call.data)?;
        if payload.starts_with(&[0x71, 1, 3, 5]) {
            validate_consent(call, payload, &action_key)?;
            CallDisclosure::Consent {
                payload: payload.to_vec(),
                action_key,
            }
        } else {
            CallDisclosure::Shared(Box::new(call::bind_call(call, sender)?))
        }
    } else {
        CallDisclosure::Shared(Box::new(call::bind_call(call, sender)?))
    };
    let bound = CallBinding {
        call: call.clone(),
        sender,
        disclosure,
    };
    if bound.reencode()? != call.encode_unsigned() {
        return Err(DisclosureError::FieldMismatch("canonical_bytes"));
    }
    Ok(bound)
}

fn validate_consent(
    call: &Eip1559Call,
    payload: &[u8],
    action_key: &[u8; 32],
) -> Result<(), DisclosureError> {
    if call.to[..18] != [0; 18]
        || call.to[18..] != [0x10, 0x1c]
        || call.data.get(..4)
            != Some(call::ledger_selector("recordLifecycle(bytes,bytes32)").as_slice())
        || payload.len() != 140
        || payload[..4] != [0x71, 1, 3, 5]
        || call.nonce != 0
        || call.max_priority_fee_per_gas != 0
        || call.max_fee_per_gas != 0
        || payload[4..36] == [0; 32]
        || payload[36..68] == [0; 32]
        || payload[68..100] == [0; 32]
        || payload[100..132] == [0; 32]
        || payload[132..140] == [0; 8]
        || payload[100..132] != *action_key
    {
        return Err(DisclosureError::MalformedPayload);
    }
    Ok(())
}

fn lifecycle_payload(data: &[u8]) -> Result<(&[u8], [u8; 32]), DisclosureError> {
    let malformed = || DisclosureError::MalformedPayload;
    let args = data.get(4..).ok_or_else(malformed)?;
    let mut offset = [0; 32];
    offset[31] = 64;
    if args.get(..32) != Some(offset.as_slice()) {
        return Err(malformed());
    }
    let length = args.get(64..96).ok_or_else(malformed)?;
    if length[..24] != [0; 24] {
        return Err(malformed());
    }
    let length = usize::try_from(u64::from_be_bytes(
        length[24..].try_into().map_err(|_| malformed())?,
    ))
    .map_err(|_| malformed())?;
    if !(4..=1024).contains(&length) || args.len() != 96 + length.div_ceil(32) * 32 {
        return Err(malformed());
    }
    if args[96 + length..].iter().any(|byte| *byte != 0) {
        return Err(malformed());
    }
    Ok((
        &args[96..96 + length],
        args[32..64].try_into().map_err(|_| malformed())?,
    ))
}
