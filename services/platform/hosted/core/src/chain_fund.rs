use layerx_crypto::evm_transaction::transaction_hash;
use layerx_platform_internal::evm_sender::{Eip1559Call, EvmSender};
use serde::{Deserialize, Serialize};
use serde_json::{json, Value};
use std::io::Write;
use std::net::{TcpStream, ToSocketAddrs};
use std::time::Duration;

use super::{hex_decode, hex_encode, parse_node_url, refusal, NodeEndpoint, Response};

pub(super) struct ChainFunding {
    endpoint: NodeEndpoint,
    sender: EvmSender,
    pub(super) chain_id: u64,
}

#[derive(Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
pub(super) struct SignedFunding {
    pub(super) raw: Vec<u8>,
    pub(super) transaction_hash: String,
}

impl SignedFunding {
    pub(super) fn validate(&self) -> Result<(), ()> {
        if self.raw.first() != Some(&2)
            || self.raw.len() < 64
            || self.raw.len() > 1024
            || self.transaction_hash != format!("0x{}", hex_encode(&transaction_hash(&self.raw)))
        {
            return Err(());
        }
        Ok(())
    }
}

impl ChainFunding {
    pub(super) fn from_env() -> Result<Self, &'static str> {
        let endpoint = parse_node_url(
            &std::env::var("LAYERX_CORE_CHAIN_RPC_URL").map_err(|_| "chain_unavailable")?,
        )
        .map_err(|_| "chain_unavailable")?;
        let sender =
            EvmSender::from_env("LAYERX_CORE_FUNDING_SENDER").map_err(|_| "sender_unavailable")?;
        let mut funding = Self {
            endpoint,
            sender,
            chain_id: 0,
        };
        funding.chain_id = quantity(
            &funding
                .rpc("eth_chainId", json!([]))
                .map_err(|_| "chain_unavailable")?,
        )
        .map_err(|_| "chain_unavailable")?
        .try_into()
        .map_err(|_| "chain_unavailable")?;
        if funding.chain_id == 0 {
            return Err("chain_unavailable");
        }
        Ok(funding)
    }

    pub(super) fn readiness(&self) -> Result<Value, &'static str> {
        let chain_id: u64 = quantity(
            &self
                .rpc("eth_chainId", json!([]))
                .map_err(|_| "chain_unavailable")?,
        )
        .map_err(|_| "chain_unavailable")?
        .try_into()
        .map_err(|_| "chain_unavailable")?;
        if chain_id != self.chain_id {
            return Err("chain_unavailable");
        }
        let latest_block: u64 = quantity(
            &self
                .rpc("eth_blockNumber", json!([]))
                .map_err(|_| "chain_unavailable")?,
        )
        .map_err(|_| "chain_unavailable")?
        .try_into()
        .map_err(|_| "chain_unavailable")?;
        Ok(json!({
            "ready": true,
            "chain_id": chain_id,
            "latest_block": latest_block,
            "sender": format!("0x{}", hex_encode(&self.sender.address())),
        }))
    }

    fn rpc(&self, method: &str, params: Value) -> Result<Value, String> {
        let body =
            json!({"jsonrpc": "2.0", "id": 1, "method": method, "params": params}).to_string();
        let address = (self.endpoint.host.as_str(), self.endpoint.port)
            .to_socket_addrs()
            .map_err(|_| "funding endpoint unavailable")?
            .next()
            .ok_or("funding endpoint unavailable")?;
        let mut stream = TcpStream::connect_timeout(&address, Duration::from_secs(3))
            .map_err(|_| "funding endpoint unavailable")?;
        stream
            .set_read_timeout(Some(Duration::from_secs(5)))
            .and_then(|()| stream.set_write_timeout(Some(Duration::from_secs(5))))
            .map_err(|_| "funding deadline unavailable")?;
        write!(stream, "POST / HTTP/1.1\r\nHost: {}:{}\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{}",
            self.endpoint.host, self.endpoint.port, body.len(), body)
            .map_err(|_| "funding request unavailable")?;
        let response = layerx_platform_internal::http::read_http_message(&mut stream, 64 * 1024)?;
        if response
            .headers
            .get("")
            .and_then(|line| line.split_whitespace().nth(1))
            != Some("200")
        {
            return Err("funding RPC HTTP refusal".to_owned());
        }
        let response: Value =
            serde_json::from_slice(&response.body).map_err(|_| "funding RPC malformed")?;
        if response["jsonrpc"] != "2.0" || response["id"] != 1 {
            return Err("funding RPC identity differs".to_owned());
        }
        if !response["error"].is_null() {
            if method == "eth_sendRawTransaction"
                && matches!(
                    response["error"]["message"].as_str(),
                    Some("already known" | "known transaction")
                )
            {
                return Err("funding transaction already known".to_owned());
            }
            return Err("funding RPC refused".to_owned());
        }
        response
            .get("result")
            .cloned()
            .ok_or_else(|| "funding RPC result missing".to_owned())
    }

    pub(super) fn prepare(
        &self,
        public_key: &[u8; 32],
        amount: u64,
    ) -> Result<SignedFunding, Response> {
        let unavailable = || refusal(503, "treasury_unavailable", Some(5));
        let result = self
            .rpc(
                "eth_call",
                json!([{
            "to": "0x0000000000000000000000000000000000001004",
            "data": format!("0xc413bad3{}", hex_encode(public_key)),
        }, "latest"]),
            )
            .map_err(|_| unavailable())?;
        let bound = result
            .as_str()
            .and_then(|text| text.strip_prefix("0x"))
            .and_then(|text| hex_decode(text).ok())
            .filter(|bytes| bytes.len() == 32 && bytes[..12] == [0; 12]);
        let bound = bound.ok_or_else(|| refusal(409, "identity_binding_unavailable", None))?;
        let to: [u8; 20] = bound[12..]
            .try_into()
            .map_err(|_| refusal(409, "identity_binding_unavailable", None))?;
        if to == [0; 20] {
            return Err(refusal(409, "identity_binding_unavailable", None));
        }
        let nonce = quantity(
            &self
                .rpc(
                    "eth_getTransactionCount",
                    json!([
                        format!("0x{}", hex_encode(&self.sender.address())),
                        "pending"
                    ]),
                )
                .map_err(|_| unavailable())?,
        )
        .map_err(|_| unavailable())?
        .try_into()
        .map_err(|_| unavailable())?;
        let gas_price = quantity(
            &self
                .rpc("eth_gasPrice", json!([]))
                .map_err(|_| unavailable())?,
        )
        .map_err(|_| unavailable())?;
        let priority = quantity(
            &self
                .rpc("eth_maxPriorityFeePerGas", json!([]))
                .map_err(|_| unavailable())?,
        )
        .map_err(|_| unavailable())?;
        let max_fee = gas_price
            .checked_mul(2)
            .and_then(|fee| fee.checked_add(priority))
            .ok_or_else(unavailable)?;
        let gas = quantity(
            &self
                .rpc(
                    "eth_estimateGas",
                    json!([{
                        "from": format!("0x{}", hex_encode(&self.sender.address())),
                        "to": format!("0x{}", hex_encode(&to)),
                        "value": format!("0x{amount:x}"), "data": "0x",
                    }]),
                )
                .map_err(|_| unavailable())?,
        )
        .map_err(|_| unavailable())?;
        let call = Eip1559Call {
            chain_id: self.chain_id,
            nonce,
            max_priority_fee_per_gas: priority,
            max_fee_per_gas: max_fee,
            gas_limit: gas.try_into().map_err(|_| unavailable())?,
            to,
            data: Vec::new(),
        };
        // Eip1559Call's shared encoder fixes value to zero. Encode the same
        // type-2 fields with the funding value before asking EvmSender to sign.
        let mut fields = Vec::new();
        for value in [
            u128::from(call.chain_id),
            u128::from(call.nonce),
            call.max_priority_fee_per_gas,
            call.max_fee_per_gas,
            u128::from(call.gas_limit),
        ] {
            integer(value, &mut fields);
        }
        bytes(&call.to, &mut fields);
        integer(u128::from(amount), &mut fields);
        bytes(&call.data, &mut fields);
        fields.push(0xc0);
        let mut preimage = vec![2];
        preimage.extend(list(&fields));
        let signature = self
            .sender
            .sign_prehash(&transaction_hash(&preimage))
            .map_err(|_| unavailable())?;
        integer(u128::from(signature[64] - 27), &mut fields);
        scalar(&signature[..32], &mut fields);
        scalar(&signature[32..64], &mut fields);
        let mut raw = vec![2];
        raw.extend(list(&fields));
        Ok(SignedFunding {
            transaction_hash: format!("0x{}", hex_encode(&transaction_hash(&raw))),
            raw,
        })
    }

    pub(super) fn submit(&self, signed: &SignedFunding) -> Result<(), Response> {
        let result = match self.rpc(
            "eth_sendRawTransaction",
            json!([format!("0x{}", hex_encode(&signed.raw))]),
        ) {
            Ok(result) => result,
            Err(error) if error == "funding transaction already known" => return Ok(()),
            Err(_) => {
                if self.funded(signed)? {
                    return Ok(());
                }
                return Err(refusal(503, "treasury_unavailable", Some(5)));
            }
        };
        if result.as_str() != Some(signed.transaction_hash.as_str()) {
            return Err(refusal(503, "treasury_unavailable", Some(5)));
        }
        Ok(())
    }

    pub(super) fn funded(&self, signed: &SignedFunding) -> Result<bool, Response> {
        let unavailable = || refusal(503, "receipt_unavailable", Some(5));
        let receipt = self
            .rpc(
                "eth_getTransactionReceipt",
                json!([signed.transaction_hash]),
            )
            .map_err(|_| unavailable())?;
        if receipt.is_null() {
            return Ok(false);
        }
        if receipt["transactionHash"].as_str() != Some(signed.transaction_hash.as_str()) {
            return Err(unavailable());
        }
        match quantity(&receipt["status"]).map_err(|_| unavailable())? {
            1 => Ok(true),
            0 => Err(refusal(422, "funding_failed", None)),
            _ => Err(unavailable()),
        }
    }
}

fn quantity(value: &Value) -> Result<u128, String> {
    let value = value
        .as_str()
        .and_then(|value| value.strip_prefix("0x"))
        .filter(|value| !value.is_empty() && (value.len() == 1 || !value.starts_with('0')))
        .filter(|value| value.bytes().all(|byte| byte.is_ascii_hexdigit()))
        .ok_or("invalid funding RPC quantity")?;
    u128::from_str_radix(value, 16).map_err(|_| "funding RPC quantity overflow".to_owned())
}

fn length(size: usize, short: u8, output: &mut Vec<u8>) {
    if size < 56 {
        output.push(short + u8::try_from(size).expect("short RLP length"));
    } else {
        let size = size.to_be_bytes();
        let first = size
            .iter()
            .position(|byte| *byte != 0)
            .expect("nonzero RLP length");
        output.push(short + 55 + u8::try_from(size.len() - first).expect("RLP length width"));
        output.extend_from_slice(&size[first..]);
    }
}

fn bytes(value: &[u8], output: &mut Vec<u8>) {
    if value.len() == 1 && value[0] < 0x80 {
        output.push(value[0]);
    } else {
        length(value.len(), 0x80, output);
        output.extend_from_slice(value);
    }
}

fn scalar(value: &[u8], output: &mut Vec<u8>) {
    let first = value
        .iter()
        .position(|byte| *byte != 0)
        .unwrap_or(value.len());
    bytes(&value[first..], output);
}

fn integer(value: u128, output: &mut Vec<u8>) {
    scalar(&value.to_be_bytes(), output);
}

fn list(payload: &[u8]) -> Vec<u8> {
    let mut output = Vec::new();
    length(payload.len(), 0xc0, &mut output);
    output.extend_from_slice(payload);
    output
}
