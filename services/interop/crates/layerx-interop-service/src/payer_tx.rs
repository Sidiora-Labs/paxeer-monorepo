use layerx_crypto::evm_transaction::{
    decode_signed, recover_sender, transaction_hash, EvmTransactionError,
};

const LAYERX_LEDGER: [u8; 20] = [
    0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x10, 0x1a,
];
const PAY_EXACT: [u8; 4] = [0x78, 0xea, 0x11, 0xdb];

#[derive(Clone, Debug, Eq, PartialEq)]
pub(crate) struct PayExactCall {
    pub(crate) from: [u8; 32],
    pub(crate) to: [u8; 32],
    pub(crate) asset: [u8; 32],
    pub(crate) amount: u128,
    pub(crate) expires_at: u64,
    pub(crate) reference: [u8; 32],
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub(crate) enum PayerTransactionRefusal {
    Malformed,
    TargetRefused,
    SelectorRefused,
    SignatureRefused,
}

impl PayerTransactionRefusal {
    pub(crate) const fn code(self) -> &'static str {
        match self {
            Self::Malformed => "transaction_malformed",
            Self::TargetRefused => "transaction_target_refused",
            Self::SelectorRefused => "transaction_selector_refused",
            Self::SignatureRefused => "transaction_signature_refused",
        }
    }
}

impl std::fmt::Display for PayerTransactionRefusal {
    fn fmt(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        formatter.write_str(self.code())
    }
}

impl std::error::Error for PayerTransactionRefusal {}

fn take_rlp<'a>(input: &mut &'a [u8]) -> Result<(bool, &'a [u8]), PayerTransactionRefusal> {
    let malformed = PayerTransactionRefusal::Malformed;
    let (&prefix, rest) = input.split_first().ok_or(malformed)?;
    if prefix < 0x80 {
        let (value, rest) = input.split_at(1);
        *input = rest;
        return Ok((false, value));
    }
    let (list, size) = if prefix < 0xc0 {
        (false, prefix - 0x80)
    } else {
        (true, prefix - 0xc0)
    };
    let (length, rest) = if size < 56 {
        (usize::from(size), rest)
    } else {
        let width = usize::from(size - 55);
        if width > std::mem::size_of::<usize>() {
            return Err(malformed);
        }
        let (bytes, rest) = rest.split_at_checked(width).ok_or(malformed)?;
        if bytes.first() == Some(&0) {
            return Err(malformed);
        }
        let length = bytes
            .iter()
            .fold(0_usize, |length, byte| (length << 8) | usize::from(*byte));
        if length < 56 {
            return Err(malformed);
        }
        (length, rest)
    };
    let (value, rest) = rest.split_at_checked(length).ok_or(malformed)?;
    if !list && matches!(value, [byte] if *byte < 0x80) {
        return Err(malformed);
    }
    *input = rest;
    Ok((list, value))
}

fn transaction_target(raw: &[u8]) -> Result<[u8; 20], PayerTransactionRefusal> {
    let malformed = PayerTransactionRefusal::Malformed;
    let Some((&0x02, mut envelope)) = raw.split_first() else {
        return Err(malformed);
    };
    let (true, mut fields) = take_rlp(&mut envelope)? else {
        return Err(malformed);
    };
    if !envelope.is_empty() {
        return Err(malformed);
    }
    for width in [8, 8, 16, 16, 8] {
        let (false, value) = take_rlp(&mut fields)? else {
            return Err(malformed);
        };
        if value.len() > width || value.first() == Some(&0) {
            return Err(malformed);
        }
    }
    let (false, target) = take_rlp(&mut fields)? else {
        return Err(malformed);
    };
    target.try_into().map_err(|_| malformed)
}

pub(crate) fn decode_pay_exact(
    raw: &[u8],
) -> Result<(PayExactCall, [u8; 20], [u8; 32]), PayerTransactionRefusal> {
    if transaction_target(raw)? != LAYERX_LEDGER {
        return Err(PayerTransactionRefusal::TargetRefused);
    }
    let (transaction, signature) = decode_signed(raw).map_err(|error| match error {
        EvmTransactionError::BadSignature => PayerTransactionRefusal::SignatureRefused,
        _ => PayerTransactionRefusal::Malformed,
    })?;
    if transaction.to != LAYERX_LEDGER {
        return Err(PayerTransactionRefusal::TargetRefused);
    }
    if transaction.data.get(..4) != Some(PAY_EXACT.as_slice()) {
        return Err(PayerTransactionRefusal::SelectorRefused);
    }
    let malformed = PayerTransactionRefusal::Malformed;
    let (words, remainder) = transaction.data[4..].as_chunks::<32>();
    let words: &[[u8; 32]; 6] = words.try_into().map_err(|_| malformed)?;
    if !remainder.is_empty() {
        return Err(malformed);
    }
    if words[3][..16] != [0; 16] || words[4][..24] != [0; 24] {
        return Err(malformed);
    }
    let call = PayExactCall {
        from: words[0],
        to: words[1],
        asset: words[2],
        amount: u128::from_be_bytes(words[3][16..].try_into().map_err(|_| malformed)?),
        expires_at: u64::from_be_bytes(words[4][24..].try_into().map_err(|_| malformed)?),
        reference: words[5],
    };
    let sender = recover_sender(&transaction, &signature)
        .map_err(|_| PayerTransactionRefusal::SignatureRefused)?;
    Ok((call, sender, transaction_hash(raw)))
}

#[cfg(test)]
mod tests {
    use super::{decode_pay_exact, PayExactCall, PayerTransactionRefusal};
    use serde_json::Value;

    type TestResult<T = ()> = Result<T, Box<dyn std::error::Error>>;

    fn fixture(id: &str) -> TestResult<Value> {
        let fixtures: Value = serde_json::from_str(include_str!(
            "../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
        ))?;
        fixtures["cases"]
            .as_array()
            .ok_or("fixture cases must be an array")?
            .iter()
            .find(|case| case["id"] == id)
            .cloned()
            .ok_or_else(|| format!("missing fixture {id}").into())
    }

    fn hex(value: &Value) -> TestResult<Vec<u8>> {
        let encoded = value
            .as_str()
            .and_then(|value| value.strip_prefix("0x"))
            .ok_or("fixture must contain prefixed hex")?;
        if encoded.len() % 2 != 0 {
            return Err("fixture hex must have an even length".into());
        }
        encoded
            .as_bytes()
            .chunks_exact(2)
            .map(|pair| Ok(u8::from_str_radix(std::str::from_utf8(pair)?, 16)?))
            .collect()
    }

    #[test]
    fn decodes_the_recorded_pay_exact_transaction() -> TestResult {
        let fixture = fixture("V1-layerxledger-pay-exact")?;
        let raw = hex(&fixture["rawTransaction"])?;
        let (call, sender, hash) = decode_pay_exact(&raw)?;
        let receipt = &fixture["receiptJson"];
        assert_eq!(sender.as_slice(), hex(&receipt["from"])?);
        assert_eq!(hash.as_slice(), hex(&receipt["txHash"])?);
        let log = &receipt["logs"][0];
        let data = hex(&log["data"])?;
        assert_eq!(
            call,
            PayExactCall {
                from: hex(&log["topics"][1])?
                    .try_into()
                    .map_err(|_| "payer width")?,
                to: hex(&log["topics"][2])?
                    .try_into()
                    .map_err(|_| "payee width")?,
                asset: data[..32].try_into()?,
                amount: 1_000_000,
                expires_at: 1_750_003_600_000,
                reference: hex(&log["topics"][3])?
                    .try_into()
                    .map_err(|_| "reference width")?,
            }
        );
        Ok(())
    }

    #[test]
    fn refuses_a_transaction_to_another_contract() -> TestResult {
        let fixture = fixture("V1-pax-transfer")?;
        let error = match decode_pay_exact(&hex(&fixture["rawTransaction"])?) {
            Err(error) => error,
            Ok(_) => panic!("the PAX transfer must be refused"),
        };
        assert_eq!(error, PayerTransactionRefusal::TargetRefused);
        assert_eq!(error.code(), "transaction_target_refused");
        Ok(())
    }

    #[test]
    fn refuses_a_truncated_transaction_as_malformed() -> TestResult {
        let fixture = fixture("V1-layerxledger-pay-exact")?;
        let mut raw = hex(&fixture["rawTransaction"])?;
        raw.pop().ok_or("transaction fixture must not be empty")?;
        let error = match decode_pay_exact(&raw) {
            Err(error) => error,
            Ok(_) => panic!("truncated RLP must be refused"),
        };
        assert_eq!(error, PayerTransactionRefusal::Malformed);
        assert_eq!(error.code(), "transaction_malformed");
        Ok(())
    }
}
