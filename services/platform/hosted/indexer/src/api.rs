//! The read API: `/healthz`, `/readyz`, `/v1/history/{account}`,
//! `/v1/holders/{address}`, `/v1/assets` and `/v1/assets/{id}`, served over
//! plain HTTP on loopback or over TLS.

use std::collections::HashSet;
use std::net::{SocketAddr, TcpListener, TcpStream};
use std::sync::atomic::{AtomicUsize, Ordering};
use std::sync::Arc;
use std::thread;
use std::time::Duration;

use layerx_platform_internal::http::{json, read_http_message, refusal, write_response, Response};
use rustls::{ServerConfig, ServerConnection, StreamOwned};
use serde::Serialize;
use serde_json::json;

use crate::codec::{hex0x, percent_decode, unhex_fixed};
use crate::store::{NameOrder, NameRecord, Page, Position, Store, MAXIMUM_NAME_RESULTS};
use crate::IndexError;

/// Default page size.
pub const DEFAULT_LIMIT: usize = 50;
/// Largest page size a client may ask for.
pub const MAXIMUM_LIMIT: usize = 500;

const MAXIMUM_REQUEST_BYTES: usize = 16 * 1024;
const MAXIMUM_CONNECTIONS: usize = 256;
const IO_TIMEOUT: Duration = Duration::from_secs(10);

static ACTIVE: AtomicUsize = AtomicUsize::new(0);

/// The record kinds `/v1/holders/{address}` lists.
const HOLDER_KINDS: [&str; 2] = ["budget", "grant"];

/// The cursor form a route pages by.
#[derive(Clone, Copy)]
enum CursorForm {
    /// A positive row id below `i64::MAX`.
    Row,
    /// A history [`Position`] in its 32-hex text form.
    Position,
}

struct Query {
    cursor: Option<u64>,
    position: Option<Position>,
    limit: usize,
    kind: Option<String>,
}

fn parse_query(query: Option<&str>, form: CursorForm, allow_kind: bool) -> Result<Query, Response> {
    let mut parsed = Query {
        cursor: None,
        position: None,
        limit: DEFAULT_LIMIT,
        kind: None,
    };
    let mut seen: Vec<String> = Vec::new();
    for pair in query
        .unwrap_or_default()
        .split('&')
        .filter(|pair| !pair.is_empty())
    {
        let (name, value) = pair.split_once('=').unwrap_or((pair, ""));
        let name = percent_decode(name, true).map_err(|_| refusal(400, "invalid_query", None))?;
        let value = percent_decode(value, true).map_err(|_| refusal(400, "invalid_query", None))?;
        if seen.contains(&name) {
            return Err(refusal(400, "duplicate_query_parameter", None));
        }
        match name.as_str() {
            "cursor" => match form {
                CursorForm::Row => {
                    parsed.cursor = Some(
                        value
                            .parse::<u64>()
                            .ok()
                            .filter(|cursor| *cursor > 0 && i64::try_from(*cursor).is_ok())
                            .ok_or_else(|| refusal(400, "invalid_cursor", None))?,
                    );
                }
                CursorForm::Position => {
                    parsed.position = Some(
                        Position::parse(&value)
                            .ok_or_else(|| refusal(400, "invalid_cursor", None))?,
                    );
                }
            },
            "limit" => {
                parsed.limit = value
                    .parse::<usize>()
                    .ok()
                    .filter(|limit| (1..=MAXIMUM_LIMIT).contains(limit))
                    .ok_or_else(|| refusal(400, "invalid_limit", None))?;
            }
            "kind" if allow_kind => {
                if value.is_empty()
                    || value.len() > 64
                    || !value.bytes().all(|byte| {
                        byte.is_ascii_lowercase() || byte.is_ascii_digit() || byte == b'_'
                    })
                {
                    return Err(refusal(400, "invalid_kind", None));
                }
                parsed.kind = Some(value);
            }
            _ => return Err(refusal(400, "unknown_query_parameter", None)),
        }
        seen.push(name);
    }
    Ok(parsed)
}

fn page(page: Page) -> Response {
    json(
        200,
        &json!({ "version": 1, "items": page.items, "next_cursor": page.next_cursor }),
    )
}

/// A history-cursor page with its freshness: the indexed Paxeer height,
/// the chain height last observed at the source and the readiness document.
fn fresh_page(
    store: &Store,
    readiness: &Readiness,
    page: Result<Page, IndexError>,
    explorer_complete: Option<bool>,
) -> Response {
    let freshness = || -> Result<_, IndexError> {
        Ok((
            store
                .cursor(crate::paxeer::CHAIN)?
                .map(|cursor| cursor.position),
            store
                .source_observation(crate::paxeer::CHAIN)?
                .and_then(|observation| observation.source_head),
            readiness_document(store, readiness)?,
        ))
    };
    match page.and_then(|page| Ok((page, freshness()?))) {
        Ok((page, (indexed_height, chain_height, document))) => {
            let mut document = json!({
                "version": 1, "items": page.items, "next_cursor": page.next_cursor,
                "indexedHeight": indexed_height, "chainHeight": chain_height,
                "freshness": document,
            });
            if let Some(complete) = explorer_complete {
                document["explorerComplete"] = json!(complete);
            }
            json(200, &document)
        }
        Err(error) => failure(&error),
    }
}

fn failure(error: &IndexError) -> Response {
    eprintln!("layerx-indexer request failed: {error}");
    refusal(503, "store_unavailable", Some(1))
}

fn identifier(segment: &str) -> Result<String, Response> {
    let decoded = percent_decode(segment, false).map_err(|_| refusal(400, "invalid_path", None))?;
    if decoded.is_empty()
        || decoded.len() > 256
        || decoded
            .chars()
            .any(|character| character.is_control() || character.is_whitespace())
    {
        return Err(refusal(400, "invalid_path", None));
    }
    Ok(decoded)
}

#[derive(Default)]
struct NameQuery {
    address: Option<String>,
    name: Option<String>,
    only_active: bool,
    owned_by: Option<bool>,
    resolved_to: Option<bool>,
    protocol_ok: bool,
    order: NameOrder,
}

fn parse_name_query(query: Option<&str>, allowed: &[&str]) -> Result<NameQuery, Response> {
    let mut parsed = NameQuery {
        protocol_ok: true,
        ..NameQuery::default()
    };
    let mut seen = Vec::new();
    for pair in query
        .unwrap_or_default()
        .split('&')
        .filter(|pair| !pair.is_empty())
    {
        let (key, value) = pair.split_once('=').unwrap_or((pair, ""));
        let key = percent_decode(key, true).map_err(|_| refusal(400, "invalid_query", None))?;
        let value = percent_decode(value, true).map_err(|_| refusal(400, "invalid_query", None))?;
        if !allowed.contains(&key.as_str()) {
            return Err(refusal(400, "unknown_query_parameter", None));
        }
        if seen.contains(&key) {
            return Err(refusal(400, "duplicate_query_parameter", None));
        }
        match key.as_str() {
            "address" => parsed.address = Some(name_address(&value)?),
            "name" => parsed.name = Some(name_identifier(&value)?),
            "owned_by" | "resolved_to" | "only_active" => {
                let flag = match value.as_str() {
                    "true" => true,
                    "false" => false,
                    _ => return Err(refusal(400, "invalid_query", None)),
                };
                if key == "owned_by" {
                    parsed.owned_by = Some(flag);
                } else if key == "resolved_to" {
                    parsed.resolved_to = Some(flag);
                } else {
                    parsed.only_active = flag;
                }
            }
            "protocols" => parsed.protocol_ok &= value.split(',').any(|id| id == "pax"),
            "protocol_id" => parsed.protocol_ok &= value == "pax",
            "sort" if value == "registration_date" => {}
            "order" => {
                parsed.order = match value.as_str() {
                    "ASC" => NameOrder::Asc,
                    "DESC" => NameOrder::Desc,
                    _ => return Err(refusal(400, "invalid_order", None)),
                }
            }
            _ => return Err(refusal(400, "invalid_query", None)),
        }
        seen.push(key);
    }
    Ok(parsed)
}

fn name_address(address: &str) -> Result<String, Response> {
    unhex_fixed::<20>(address)
        .map(|address| hex0x(&address))
        .map_err(|_| refusal(400, "invalid_address", None))
}

fn name_identifier(name: &str) -> Result<String, Response> {
    if name.is_empty()
        || name.len() > 256
        || name
            .chars()
            .any(|c| c.is_whitespace() || c.is_control() || c == '/')
    {
        return Err(refusal(400, "invalid_name", None));
    }
    Ok(name.to_ascii_lowercase())
}

fn protocol_document() -> serde_json::Value {
    json!({"id": "pax", "short_name": "PNS", "title": "Paxeer Name System", "tld_list": ["pax"]})
}

fn domain_document(record: &NameRecord, detailed: bool) -> serde_json::Value {
    let address = |hash: &Option<String>| hash.as_ref().map(|hash| json!({"hash": hash}));
    let mut document = json!({
        "id": record.node,
        "name": record.name,
        "resolved_address": address(&record.resolved_address),
        "owner": {"hash": record.owner},
        "wrapped_owner": null,
        "registration_date": record.registration_date,
        "expiry_date": record.expiry_date,
        "protocol": protocol_document(),
    });
    if detailed {
        let tokens: Vec<_> = record.token_id.iter().zip(record.registrar.iter()).map(|(id, contract)| {
            json!({"id": id, "contract_hash": contract, "type": "NATIVE_DOMAIN_TOKEN"})
        }).collect();
        document["tokens"] = json!(tokens);
        document["registrant"] = json!(address(&record.registrant));
        document["resolver_address"] = json!(address(&record.resolver));
        document["other_addresses"] = json!({});
        document["stored_offchain"] = json!(false);
    }
    document
}

fn names_route(
    store: &Store,
    path: &str,
    query: Option<&str>,
) -> Result<serde_json::Value, Response> {
    let store_error = |error: IndexError| match error {
        IndexError::NameResultLimit => json(
            422,
            &json!({
                "error": {"code": "names_result_limit_exceeded", "retry": "never",
                    "maximum_results": MAXIMUM_NAME_RESULTS}
            }),
        ),
        error => failure(&error),
    };
    if path == "/api/v1/addresses:lookup" {
        let query = parse_name_query(
            query,
            &[
                "address",
                "owned_by",
                "resolved_to",
                "only_active",
                "sort",
                "order",
                "protocols",
            ],
        )?;
        let address = query
            .address
            .ok_or_else(|| refusal(400, "missing_address", None))?;
        let records = if !query.protocol_ok {
            Vec::new()
        } else if query.owned_by == Some(true) && query.resolved_to == Some(true) {
            let mut records = store
                .names_owned_by(&address, query.only_active, query.order)
                .map_err(store_error)?;
            records.extend(
                store
                    .names_resolving_to(&address, query.only_active, query.order)
                    .map_err(store_error)?,
            );
            let mut nodes = HashSet::new();
            records.retain(|record| nodes.insert(record.node.clone()));
            if records.len() > MAXIMUM_NAME_RESULTS {
                return Err(store_error(IndexError::NameResultLimit));
            }
            records.sort_by(|left, right| {
                let order = (&left.registration_date, &left.node)
                    .cmp(&(&right.registration_date, &right.node));
                match query.order {
                    NameOrder::Asc => order,
                    NameOrder::Desc => order.reverse(),
                }
            });
            records
        } else if query.resolved_to == Some(true) || query.owned_by == Some(false) {
            store
                .names_resolving_to(&address, query.only_active, query.order)
                .map_err(store_error)?
        } else {
            store
                .names_owned_by(&address, query.only_active, query.order)
                .map_err(store_error)?
        };
        let items: Vec<_> = records
            .iter()
            .map(|record| domain_document(record, false))
            .collect();
        return Ok(json!({"items": items, "next_page_params": null}));
    }
    if path == "/api/v1/domains:lookup" {
        let query = parse_name_query(query, &["name", "only_active", "protocols"])?;
        let name = query
            .name
            .ok_or_else(|| refusal(400, "missing_name", None))?;
        if !query.protocol_ok {
            return Ok(json!({"items": [], "next_page_params": null}));
        }
        let record = store
            .name(&name)
            .map_err(store_error)?
            .ok_or_else(|| refusal(404, "domain_not_found", None))?;
        let items = if query.only_active && !record.active {
            Vec::new()
        } else {
            vec![domain_document(&record, false)]
        };
        return Ok(json!({"items": items, "next_page_params": null}));
    }
    if let Some(segment) = path.strip_prefix("/api/v1/domains/") {
        let (segment, events) = segment
            .strip_suffix("/events")
            .map_or((segment, false), |name| (name, true));
        let query = parse_name_query(
            query,
            if events {
                &["order", "protocol_id", "protocols"]
            } else {
                &["protocol_id", "protocols"]
            },
        )?;
        let name = name_identifier(&identifier(segment)?)?;
        if !query.protocol_ok {
            return Err(refusal(404, "domain_not_found", None));
        }
        let record = store
            .name(&name)
            .map_err(store_error)?
            .ok_or_else(|| refusal(404, "domain_not_found", None))?;
        if events {
            let items = store
                .name_events(&record.node, query.order)
                .map_err(store_error)?;
            return Ok(json!({"items": items, "next_page_params": null}));
        }
        return Ok(domain_document(&record, true));
    }
    if let Some(segment) = path.strip_prefix("/api/v1/addresses/") {
        let query = parse_name_query(query, &["protocols"])?;
        let address = name_address(&identifier(segment)?)?;
        if !query.protocol_ok {
            return Ok(json!({"domain": null, "resolved_domains_count": 0}));
        }
        let domain = store.name_for_address(&address).map_err(store_error)?;
        let count = store
            .names_resolving_to(&address, false, NameOrder::Desc)
            .map_err(store_error)?
            .len();
        return Ok(json!({
            "domain": domain.as_ref().map(|record| domain_document(record, true)),
            "resolved_domains_count": count,
        }));
    }
    if path == "/api/v1/protocols" {
        parse_name_query(query, &[])?;
        let mut protocol = protocol_document();
        protocol["description"] = json!("The native name service for Paxeer.");
        return Ok(json!({"items": [protocol]}));
    }
    Err(refusal(404, "not_found", None))
}

/// Answers one request `method` `target` (path plus optional query).
#[must_use]
pub fn route(store: &Store, method: &str, target: &str) -> Response {
    route_with(store, &Readiness::default(), method, target)
}

/// The ingestion sources readiness is judged on.
#[derive(Clone, Debug, Default)]
pub struct Readiness {
    pub sources: Vec<&'static str>,
    pub stall_after_secs: u64,
    /// Process start (Unix seconds): only observations after it count.
    pub started_at: u64,
    /// The explorer API that supplies Paxeer history below the node walk.
    pub explorer: Option<crate::explorer::ExplorerApi>,
}

#[derive(Clone, Debug, Serialize)]
pub struct SourceReadiness {
    pub source: String,
    pub reconciled: bool,
    pub state: &'static str,
    pub last_success_at: Option<u64>,
    pub freshness_secs: Option<u64>,
    pub last_error: Option<String>,
    pub last_error_at: Option<u64>,
    pub consecutive_failures: u64,
    pub source_head: Option<u64>,
    pub indexed_cursor: Option<u64>,
    pub finalized_cursor: Option<u64>,
}

#[derive(Clone, Debug, Serialize)]
pub struct ReadinessDocument {
    pub version: u8,
    pub status: &'static str,
    pub database: &'static str,
    pub started_at: u64,
    pub sources: Vec<SourceReadiness>,
}

const FAILING_AFTER: u64 = 3;

fn unix_now() -> u64 {
    std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .map_or(0, |elapsed| elapsed.as_secs())
}

/// The typed readiness document: database liveness plus the state of every
/// required ingestion source.
///
/// # Errors
/// Returns [`IndexError::Store`] when the database cannot answer.
pub fn readiness_document(
    store: &Store,
    readiness: &Readiness,
) -> Result<ReadinessDocument, IndexError> {
    store.ping()?;
    let current = unix_now();
    let mut worst = if readiness.sources.is_empty() {
        2_usize
    } else {
        0_usize
    };
    let mut sources = Vec::new();
    for &source in &readiness.sources {
        let observation = store.source_observation(source)?;
        let cursor = store.cursor(if source == "comet" { "paxeer" } else { source })?;
        // AI cursors live in the committed projection tables, not the EVM chain-links table.
        let indexed_position = if source == "ai" {
            observation.as_ref().and_then(|o| o.indexed_position)
        } else {
            cursor.as_ref().map(|c| c.position)
        };
        let success = observation.as_ref().and_then(|o| o.last_success_at);
        let failures = observation.as_ref().map_or(0, |o| o.consecutive_failures);
        let (state, rank) = match success {
            _ if !observation.as_ref().is_some_and(|o| o.reconciled) => ("starting", 2),
            Some(at) if at > current => ("stalled", 2),
            Some(at) if at < readiness.started_at => ("starting", 2),
            None => ("starting", 2),
            Some(at) if current.saturating_sub(at) > readiness.stall_after_secs => ("stalled", 2),
            Some(_) if failures >= FAILING_AFTER => ("failing", 1),
            Some(_) if failures > 0 => ("retrying", 1),
            Some(_) if observation.as_ref().and_then(|o| o.source_head) != indexed_position => {
                ("catching_up", 1)
            }
            Some(_) => ("ready", 0),
        };
        worst = worst.max(rank);
        sources.push(SourceReadiness {
            source: source.to_owned(),
            reconciled: observation.as_ref().is_some_and(|o| o.reconciled),
            state,
            last_success_at: success,
            freshness_secs: success.map(|at| current.saturating_sub(at)),
            last_error: observation.as_ref().and_then(|o| o.last_error.clone()),
            last_error_at: observation.as_ref().and_then(|o| o.last_error_at),
            consecutive_failures: failures,
            source_head: observation.as_ref().and_then(|o| o.source_head),
            indexed_cursor: indexed_position,
            finalized_cursor: if source == "ai" {
                indexed_position
            } else {
                cursor.as_ref().and_then(|c| c.finalized_position)
            },
        });
    }
    let status = ["ready", "degraded", "unavailable"][worst];
    Ok(ReadinessDocument {
        version: 1,
        status,
        database: "ok",
        started_at: readiness.started_at,
        sources,
    })
}

/// Answers one request, judging `/readyz` on `readiness`.
#[must_use]
pub fn route_with(store: &Store, readiness: &Readiness, method: &str, target: &str) -> Response {
    if method != "GET" {
        return refusal(405, "method_not_allowed", None);
    }
    let (path, query) = target
        .split_once('?')
        .map_or((target, None), |(path, query)| (path, Some(query)));
    if path.starts_with("/api/v1/") {
        return names_route(store, path, query)
            .map_or_else(|response| response, |body| json(200, &body));
    }
    if path == "/v1/chains" {
        if query.is_some_and(|query| !query.is_empty()) {
            return refusal(400, "unknown_query_parameter", None);
        }
        return store.chains().map_or_else(
            |error| failure(&error),
            |chains| json(200, &json!({"chains": chains})),
        );
    }
    if let Some(segment) = path.strip_prefix("/v1/chains/") {
        let unknown = || json(404, &json!({"error": "unknown_chain"}));
        if let Some(name) = segment.strip_prefix("by-name/") {
            if query.is_some_and(|query| !query.is_empty()) {
                return refusal(400, "unknown_query_parameter", None);
            }
            let name = match identifier(name) {
                Ok(name) => name,
                Err(response) => return response,
            };
            return match store.chain_by_name(&name) {
                Ok(Some(chain)) => json(200, &json!(chain)),
                Ok(None) => unknown(),
                Err(error) => failure(&error),
            };
        }
        let (id, feed) = segment
            .split_once('/')
            .map_or((segment, None), |(id, feed)| (id, Some(feed)));
        let id = match id.parse::<u32>() {
            Ok(id) => id,
            Err(_) => return unknown(),
        };
        let chain = match store.chain(id) {
            Ok(Some(chain)) => chain,
            Ok(None) => return unknown(),
            Err(error) => return failure(&error),
        };
        if let Some(feed @ ("transfers" | "events")) = feed {
            let query = match parse_query(query, CursorForm::Row, true) {
                Ok(query) => query,
                Err(response) => return response,
            };
            return fresh_page(
                store,
                readiness,
                store.chain_page(
                    id,
                    feed == "events",
                    query.cursor,
                    query.limit,
                    query.kind.as_deref(),
                ),
                None,
            );
        }
        if query.is_some_and(|query| !query.is_empty()) {
            return refusal(400, "unknown_query_parameter", None);
        }
        return match feed {
            None => json(200, &json!(chain)),
            Some("summary") => store
                .chain_summary(id)
                .map_or_else(|error| failure(&error), |summary| json(200, &summary)),
            _ => refusal(404, "not_found", None),
        };
    }
    if path == "/healthz" {
        return match store.ping() {
            Ok(()) => json(200, &json!({ "status": "ok" })),
            Err(error) => failure(&error),
        };
    }
    if path == "/readyz" {
        return match readiness_document(store, readiness) {
            Ok(document) => {
                let code = if document.status == "ready" { 200 } else { 503 };
                json(code, &json!(document))
            }
            Err(error) => json(
                503,
                &json!({ "version": 1, "status": "unavailable", "database": "error", "error": error.to_string(), "sources": [] }),
            ),
        };
    }
    if path == "/v1/assets" {
        let query = match parse_query(query, CursorForm::Row, false) {
            Ok(query) => query,
            Err(response) => return response,
        };
        return store
            .assets(query.cursor, query.limit)
            .map_or_else(|error| failure(&error), page);
    }
    if let Some(segment) = path.strip_prefix("/v1/assets/") {
        if query.is_some_and(|query| !query.is_empty()) {
            return refusal(400, "unknown_query_parameter", None);
        }
        let asset = match identifier(segment) {
            Ok(asset) => asset,
            Err(response) => return response,
        };
        return match store.asset(&asset) {
            Ok(Some(document)) => json(200, &json!({ "version": 1, "asset": document })),
            Ok(None) => refusal(404, "asset_not_found", None),
            Err(error) => failure(&error),
        };
    }
    if let Some(segment) = path.strip_prefix("/v1/history/") {
        let account = match identifier(segment) {
            Ok(account) if !account.contains('/') => account.to_ascii_lowercase(),
            Ok(_) => return refusal(404, "not_found", None),
            Err(response) => return response,
        };
        let query = match parse_query(query, CursorForm::Position, true) {
            Ok(query) => query,
            Err(response) => return response,
        };
        let explorer_complete = if let Some(explorer) = &readiness.explorer {
            match explorer.import(store, &account) {
                Ok(complete) => Some(complete),
                Err(error) => {
                    eprintln!("layerx-indexer explorer import failed: {error}");
                    return refusal(503, "explorer_unavailable", Some(1));
                }
            }
        } else {
            None
        };
        return fresh_page(
            store,
            readiness,
            store.merged_history(&account, query.position, query.limit, query.kind.as_deref()),
            explorer_complete,
        );
    }
    if let Some(segment) = path.strip_prefix("/v1/holders/") {
        let holder = match identifier(segment) {
            Ok(holder) if !holder.contains('/') => holder.to_ascii_lowercase(),
            Ok(_) => return refusal(404, "not_found", None),
            Err(response) => return response,
        };
        let query = match parse_query(query, CursorForm::Position, true) {
            Ok(query) => query,
            Err(response) => return response,
        };
        if query
            .kind
            .as_deref()
            .is_some_and(|kind| !HOLDER_KINDS.contains(&kind))
        {
            return refusal(400, "invalid_kind", None);
        }
        return fresh_page(
            store,
            readiness,
            store.holder_records(&holder, query.kind.as_deref(), query.position, query.limit),
            None,
        );
    }
    refusal(404, "not_found", None)
}

fn answer<S: std::io::Read + std::io::Write>(
    store: &Store,
    readiness: &Readiness,
    stream: &mut S,
) -> Result<(), String> {
    let response = match read_http_message(stream, MAXIMUM_REQUEST_BYTES) {
        Ok(request) => {
            let start = request.headers.get("").cloned().unwrap_or_default();
            let mut parts = start.split_whitespace();
            match (parts.next(), parts.next(), parts.next(), parts.next()) {
                (Some(method), Some(target), Some("HTTP/1.1"), None)
                    if target.starts_with('/') && request.headers.contains_key("host") =>
                {
                    route_with(store, readiness, method, target)
                }
                _ => refusal(400, "invalid_request", None),
            }
        }
        Err(_) => refusal(400, "invalid_request", None),
    };
    write_response(stream, &response)?;
    stream.flush().map_err(|error| error.to_string())
}

fn connection(
    store: &Store,
    readiness: &Readiness,
    tls: Option<&Arc<ServerConfig>>,
    tcp: TcpStream,
) -> Result<(), String> {
    tcp.set_read_timeout(Some(IO_TIMEOUT))
        .map_err(|error| error.to_string())?;
    tcp.set_write_timeout(Some(IO_TIMEOUT))
        .map_err(|error| error.to_string())?;
    match tls {
        None => {
            let mut tcp = tcp;
            answer(store, readiness, &mut tcp)
        }
        Some(config) => {
            let server =
                ServerConnection::new(Arc::clone(config)).map_err(|error| error.to_string())?;
            let mut stream = StreamOwned::new(server, tcp);
            answer(store, readiness, &mut stream)?;
            stream.conn.send_close_notify();
            let _ = stream.conn.write_tls(&mut stream.sock);
            Ok(())
        }
    }
}

/// Binds `listen`. Plain HTTP is only admitted on a loopback address.
///
/// # Errors
/// Refuses a non-loopback plaintext listener and bind failures.
pub fn bind(listen: SocketAddr, tls: bool) -> Result<TcpListener, IndexError> {
    if !tls && !listen.ip().is_loopback() {
        return Err(IndexError::Config(
            "plain HTTP listeners must be loopback; set LAYERX_INDEXER_TLS_CERT_DER".to_owned(),
        ));
    }
    TcpListener::bind(listen).map_err(|error| IndexError::Config(format!("bind {listen}: {error}")))
}

/// Serves the API on `listener` until it fails.
pub fn serve(listener: &TcpListener, store: &Arc<Store>, tls: Option<&Arc<ServerConfig>>) {
    serve_with(listener, store, &Arc::new(Readiness::default()), tls);
}

/// Serves the API on `listener`, judging `/readyz` on `readiness`.
pub fn serve_with(
    listener: &TcpListener,
    store: &Arc<Store>,
    readiness: &Arc<Readiness>,
    tls: Option<&Arc<ServerConfig>>,
) {
    for incoming in listener.incoming() {
        let tcp = match incoming {
            Ok(tcp) => tcp,
            Err(error) => {
                eprintln!("layerx-indexer accept failed: {error}");
                continue;
            }
        };
        if ACTIVE
            .fetch_update(Ordering::AcqRel, Ordering::Acquire, |active| {
                (active < MAXIMUM_CONNECTIONS).then_some(active + 1)
            })
            .is_err()
        {
            continue;
        }
        let store = Arc::clone(store);
        let readiness = Arc::clone(readiness);
        let tls = tls.cloned();
        thread::spawn(move || {
            if let Err(error) = connection(&store, &readiness, tls.as_ref(), tcp) {
                eprintln!("layerx-indexer connection failed: {error}");
            }
            ACTIVE.fetch_sub(1, Ordering::AcqRel);
        });
    }
}
