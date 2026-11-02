use std::path::PathBuf;

use clap::{Subcommand, ValueEnum};

#[derive(Clone, Copy, Debug, ValueEnum)]
pub enum ChainTier {
    Starter,
    Growth,
    Pro,
}

impl ChainTier {
    pub const fn allowances(self) -> (u64, u64) {
        match self {
            Self::Starter => (500_000, 65_536),
            Self::Growth => (1_500_000, 262_144),
            Self::Pro => (4_000_000, 1_048_576),
        }
    }
}

#[derive(Subcommand)]
pub enum ChainsCommand {
    List,
    Show {
        id_or_name: String,
    },
    Create {
        #[arg(long)]
        name: String,
        #[arg(long, value_enum)]
        tier: ChainTier,
        #[arg(long, value_parser = clap::value_parser!(u16).range(0..=5000))]
        fee_share_bps: u16,
        #[arg(long)]
        evm_key_file: Option<PathBuf>,
    },
}
