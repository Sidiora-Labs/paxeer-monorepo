#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct ProtocolTime {
    pub seconds: u64,
    pub milliseconds: u64,
    pub height: u64,
    pub block_hash: [u8; 32],
}

impl ProtocolTime {
    pub fn from_block(block: &layerx_client::sequencer::SequencerBlock) -> Self {
        assert_ne!(block.hash, [0; 32]);
        Self {
            seconds: block.timestamp,
            milliseconds: block
                .timestamp
                .checked_mul(1_000)
                .expect("protocol milliseconds"),
            height: block.number,
            block_hash: block.hash,
        }
    }

    pub fn assert_follows(self, previous: Self) {
        assert!(self.height >= previous.height);
        assert!(self.seconds >= previous.seconds);
        if self.height == previous.height {
            assert_eq!(self.block_hash, previous.block_hash);
        }
    }
}
