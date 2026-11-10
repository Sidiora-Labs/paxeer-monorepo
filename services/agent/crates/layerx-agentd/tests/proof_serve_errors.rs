//! Proves: route_request / native_read_response -> HttpPool / response / TCP HTTP -> proof::decode_response.
//!
//! This file is also the host's shared HTTP module: the proof compiles the same
//! listener, request parser, error producers and writer without the host-only MCP
//! dependency graph. No transport is replaced. The error anchor has no durable
//! state; each assertion makes two independent connections. Native error mapping
//! takes the real NativeReadError variants, including ResultTooLarge, rather than
//! constructing wire error messages. Malformed-result checks corrupt bytes of an
//! actual producer response at the consumer's decoding boundary.
//!
//! No runtime defect is repaired: this extracts the existing path without changing
//! its error codes or status semantics.

include!("proof_native_reads.rs");
