use std::fmt::Debug;

use layerx_client::sequencer::{
    AccountProof, CallRequest, ReceiptKeyEntry, SequencerBlock, SequencerError, SequencerReceipt,
};
use serde_json::Value;

use crate::node_rpc::{NodeRpc, NodeRpcError};

pub trait HumanNode: Send + Sync + Debug {
    fn chain_id(&self) -> Result<u64, NodeRpcError>;
    fn head(&self) -> Result<SequencerBlock, NodeRpcError>;
    fn block(&self, height: u64) -> Result<Option<SequencerBlock>, NodeRpcError>;
    fn receipt_keys(&self) -> Result<Vec<ReceiptKeyEntry>, NodeRpcError>;
    fn bound_address(&self, public_key: &[u8; 32]) -> Result<Option<[u8; 20]>, NodeRpcError>;
    fn pending_nonce(&self, address: &[u8; 20]) -> Result<u64, NodeRpcError>;
    fn call_at(&self, height: u64, to: &[u8; 20], data: &[u8]) -> Result<Vec<u8>, NodeRpcError>;
    fn account_proof(
        &self,
        address: &[u8; 20],
        slots: &[[u8; 32]],
        height: u64,
    ) -> Result<AccountProof, NodeRpcError>;
    fn estimate_gas(&self, call: &CallRequest) -> Result<u64, NodeRpcError>;
    fn send_raw_transaction(&self, raw: &[u8]) -> Result<[u8; 32], NodeRpcError>;
    fn sequencer_receipt(
        &self,
        tx_hash: &[u8; 32],
    ) -> Result<Option<SequencerReceipt>, NodeRpcError>;
    fn history_page(
        &self,
        params: Value,
        maximum_response_bytes: usize,
    ) -> Result<Vec<u8>, NodeRpcError>;

    fn receipt_key_at_head(&self) -> Result<ReceiptKeyEntry, NodeRpcError> {
        let head = self.head()?;
        let mut covering = self
            .receipt_keys()?
            .into_iter()
            .filter(|key| (key.first_height..=key.last_height).contains(&head.number));
        let key = covering.next().ok_or(SequencerError::Malformed)?;
        if covering.next().is_some() || key.public_key == [0; 32] {
            return Err(SequencerError::Malformed.into());
        }
        Ok(key)
    }
}

impl HumanNode for NodeRpc {
    fn chain_id(&self) -> Result<u64, NodeRpcError> {
        Ok(NodeRpc::chain_id(self))
    }
    fn head(&self) -> Result<SequencerBlock, NodeRpcError> {
        self.block_by_number(self.block_number()?)?
            .ok_or(NodeRpcError::Unavailable)
    }
    fn block(&self, height: u64) -> Result<Option<SequencerBlock>, NodeRpcError> {
        self.block_by_number(height)
    }
    fn receipt_keys(&self) -> Result<Vec<ReceiptKeyEntry>, NodeRpcError> {
        NodeRpc::receipt_keys(self)
    }
    fn bound_address(&self, public_key: &[u8; 32]) -> Result<Option<[u8; 20]>, NodeRpcError> {
        NodeRpc::bound_address(self, public_key)
    }
    fn pending_nonce(&self, address: &[u8; 20]) -> Result<u64, NodeRpcError> {
        NodeRpc::pending_nonce(self, address)
    }
    fn call_at(&self, height: u64, to: &[u8; 20], data: &[u8]) -> Result<Vec<u8>, NodeRpcError> {
        NodeRpc::call_at(self, height, to, data)
    }
    fn account_proof(
        &self,
        address: &[u8; 20],
        slots: &[[u8; 32]],
        height: u64,
    ) -> Result<AccountProof, NodeRpcError> {
        NodeRpc::account_proof(self, address, slots, height)
    }
    fn estimate_gas(&self, call: &CallRequest) -> Result<u64, NodeRpcError> {
        NodeRpc::estimate_gas(self, call)
    }
    fn send_raw_transaction(&self, raw: &[u8]) -> Result<[u8; 32], NodeRpcError> {
        NodeRpc::send_raw_transaction(self, raw)
    }
    fn sequencer_receipt(
        &self,
        tx_hash: &[u8; 32],
    ) -> Result<Option<SequencerReceipt>, NodeRpcError> {
        NodeRpc::sequencer_receipt(self, tx_hash)
    }
    fn history_page(
        &self,
        params: Value,
        maximum_response_bytes: usize,
    ) -> Result<Vec<u8>, NodeRpcError> {
        NodeRpc::history_page(self, params, maximum_response_bytes)
    }
}
