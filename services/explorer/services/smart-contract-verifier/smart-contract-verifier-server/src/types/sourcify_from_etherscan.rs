use crate::proto::VerifyFromEtherscanSourcifyRequest;
use serde::{Deserialize, Serialize};
use smart_contract_verifier::sourcify::api::VerifyFromEtherscanRequest;
use std::ops::Deref;

#[derive(Clone, Debug, Deserialize, Serialize, PartialEq)]
pub struct VerifyFromEtherscanSourcifyRequestWrapper(VerifyFromEtherscanSourcifyRequest);

impl From<VerifyFromEtherscanSourcifyRequest> for VerifyFromEtherscanSourcifyRequestWrapper {
    fn from(inner: VerifyFromEtherscanSourcifyRequest) -> Self {
        Self(inner)
    }
}

impl Deref for VerifyFromEtherscanSourcifyRequestWrapper {
    type Target = VerifyFromEtherscanSourcifyRequest;

    fn deref(&self) -> &Self::Target {
        &self.0
    }
}

impl VerifyFromEtherscanSourcifyRequestWrapper {
    pub fn new(inner: VerifyFromEtherscanSourcifyRequest) -> Self {
        Self(inner)
    }

    pub fn into_inner(self) -> VerifyFromEtherscanSourcifyRequest {
        self.0
    }
}

impl TryFrom<VerifyFromEtherscanSourcifyRequestWrapper> for VerifyFromEtherscanRequest {
    type Error = tonic::Status;

    fn try_from(request: VerifyFromEtherscanSourcifyRequestWrapper) -> Result<Self, Self::Error> {
        let request = request.into_inner();
        Ok(Self {
            address: super::evm_address::EvmAddress::parse(&request.address)?.into_bytes(),
            chain: request.chain,
        })
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use blockscout_display_bytes::Bytes as DisplayBytes;
    use pretty_assertions::assert_eq;
    use std::str::FromStr;

    #[test]
    fn try_into_verification_request() {
        let request = VerifyFromEtherscanSourcifyRequest {
            address: "0x0123456789abcdef0123456789abcdef01234567".to_string(),
            chain: "77".to_string(),
        };

        let verification_request: VerifyFromEtherscanRequest =
            <VerifyFromEtherscanSourcifyRequestWrapper>::from(request)
                .try_into()
                .expect("try_into VerifyFromEtherscanRequest failed");

        let expected = VerifyFromEtherscanRequest {
            address: DisplayBytes::from_str("0x0123456789abcdef0123456789abcdef01234567")
                .unwrap()
                .0,
            chain: "77".to_string(),
        };

        assert_eq!(expected, verification_request);
    }
}
