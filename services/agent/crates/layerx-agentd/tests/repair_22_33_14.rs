//! Audit 22.30.100: ingest-producer-coverage. The included integration cases
//! exercise recorded node output over real subscription and history clients,
//! durable ingestion, backpressure and reopening the consumer's store.

#[path = "ingest_eth_subscribe.rs"]
mod producer_ingestion;
