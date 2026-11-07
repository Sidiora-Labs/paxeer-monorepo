use std::cmp::Ordering;
use std::collections::BTreeSet;
use std::sync::{Arc, Mutex};

use serde::Serialize;
use tantivy::collector::{Collector, SegmentCollector};
use tantivy::query::{BooleanQuery, Occur, Query, TermQuery};
use tantivy::schema::document::{DeserializeError, DocumentDeserialize, DocumentDeserializer};
use tantivy::schema::{IndexRecordOption, Value as _};
use tantivy::snippet::SnippetGenerator;
use tantivy::tokenizer::TokenStream as _;
use tantivy::{DocAddress, DocId, Score, SegmentOrdinal, SegmentReader, TantivyDocument, Term};

use crate::canonical::{self, CanonicalError, ContentKind};
use crate::content::ContentStore;
use crate::index::{IndexError, WebIndex};
use crate::payment::PaymentGate;
use crate::server::{Request, Response, Route, RouteError, RouteTable};

/// The most results a search returns.
pub const MAX_RESULTS: usize = 10;

/// The longest query accepted, in bytes.
pub const MAX_QUERY_BYTES: usize = 512;

/// The most distinct terms a query is searched with.
pub const MAX_QUERY_TERMS: usize = 32;

/// The longest snippet, in characters.
pub const SNIPPET_CHARS: usize = 150;

/// The media type of the search text in the canonical bytes.
pub const SEARCH_MEDIA_TYPE: &str = "application/json";

#[derive(Debug)]
pub enum SearchError {
    EmptyQuery,
    QueryTooLong,
    TooManyTerms,
    Index(IndexError),
}

impl SearchError {
    #[must_use]
    pub const fn code(&self) -> &'static str {
        match self {
            Self::EmptyQuery => "empty_query",
            Self::QueryTooLong => "query_too_long",
            Self::TooManyTerms => "too_many_terms",
            Self::Index(error) => error.code(),
        }
    }

    #[must_use]
    pub const fn status(&self) -> u16 {
        match self {
            Self::EmptyQuery | Self::QueryTooLong | Self::TooManyTerms => 400,
            Self::Index(_) => 500,
        }
    }
}

impl std::fmt::Display for SearchError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::Index(error) => error.fmt(f),
            _ => f.write_str(self.code()),
        }
    }
}

impl std::error::Error for SearchError {}

impl From<IndexError> for SearchError {
    fn from(error: IndexError) -> Self {
        Self::Index(error)
    }
}

impl From<tantivy::TantivyError> for SearchError {
    fn from(error: tantivy::TantivyError) -> Self {
        Self::Index(IndexError::Tantivy(error))
    }
}

/// One search result as it appears in the canonical bytes: url, title and
/// snippet, in that order.
#[derive(Clone, Debug, Eq, PartialEq, Serialize)]
pub struct SearchResult {
    pub url: String,
    pub title: String,
    pub snippet: String,
}

/// A result with the score it was ranked by. The score orders the results
/// and never enters the canonical bytes.
#[derive(Clone, Debug, PartialEq)]
pub struct ScoredResult {
    pub score: Score,
    pub result: SearchResult,
}

/// The distinct terms of a query as the body field tokenises them.
///
/// # Errors
/// Refuses an over-long query, a query with no terms and one with more than
/// [`MAX_QUERY_TERMS`] distinct terms.
pub fn query_terms(index: &WebIndex, query: &str) -> Result<Vec<String>, SearchError> {
    if query.len() > MAX_QUERY_BYTES {
        return Err(SearchError::QueryTooLong);
    }
    let mut analyzer = index.index().tokenizer_for_field(index.fields().body)?;
    let mut stream = analyzer.token_stream(query);
    let mut terms = BTreeSet::new();
    while stream.advance() {
        terms.insert(stream.token().text.clone());
    }
    if terms.is_empty() {
        return Err(SearchError::EmptyQuery);
    }
    if terms.len() > MAX_QUERY_TERMS {
        return Err(SearchError::TooManyTerms);
    }
    Ok(terms.into_iter().collect())
}

fn build_query(index: &WebIndex, terms: &[String]) -> BooleanQuery {
    let fields = index.fields();
    let clauses: Vec<(Occur, Box<dyn Query>)> = terms
        .iter()
        .flat_map(|term| [fields.title, fields.body].map(|field| (field, term)))
        .map(|(field, term)| {
            let query: Box<dyn Query> = Box::new(TermQuery::new(
                Term::from_field_text(field, term),
                IndexRecordOption::WithFreqs,
            ));
            (Occur::Should, query)
        })
        .collect();
    BooleanQuery::new(clauses)
}

fn stored_text(document: &TantivyDocument, field: tantivy::schema::Field) -> String {
    document
        .get_first(field)
        .and_then(|value| value.as_str())
        .unwrap_or_default()
        .to_owned()
}

fn snippet_text(generator: &SnippetGenerator, body: &str) -> String {
    let snippet = generator.snippet(body);
    let fragment = if snippet.fragment().is_empty() {
        let end = body
            .char_indices()
            .nth(SNIPPET_CHARS)
            .map_or(body.len(), |(offset, _)| offset);
        &body[..end]
    } else {
        snippet.fragment()
    };
    fragment.split_whitespace().collect::<Vec<_>>().join(" ")
}

/// Resource accounting from the same bounded collector used by every search.
#[derive(Clone, Copy, Debug, Default, Eq, PartialEq)]
pub struct SearchStats {
    pub matches_considered: usize,
    pub retained_candidates_peak: usize,
    pub stored_bodies_loaded: usize,
}

struct Candidate {
    score: Score,
    url: String,
    address: DocAddress,
}

impl Candidate {
    fn compare(&self, other: &Self) -> Ordering {
        other
            .score
            .total_cmp(&self.score)
            .then_with(|| self.url.cmp(&other.url))
    }
}

#[derive(Default)]
struct Selection {
    winners: Vec<Candidate>,
    stats: SearchStats,
}

struct BestTen {
    selection: Arc<Mutex<Selection>>,
    url_field: tantivy::schema::Field,
}

struct SegmentBestTen {
    selection: Arc<Mutex<Selection>>,
    store: tantivy::store::StoreReader,
    segment: SegmentOrdinal,
    url_field: tantivy::schema::Field,
    error: Option<tantivy::TantivyError>,
}

// WebIndex::put writes the URL first. Stop deserializing immediately after
// that field, leaving title and body unmaterialized for losing candidates.
struct StoredUrl(tantivy::schema::Field, String);

impl DocumentDeserialize for StoredUrl {
    fn deserialize<'de, D: DocumentDeserializer<'de>>(
        mut deserializer: D,
    ) -> Result<Self, DeserializeError> {
        let (field, url) = deserializer
            .next_field::<String>()?
            .ok_or_else(|| DeserializeError::custom("missing indexed URL"))?;
        Ok(Self(field, url))
    }
}

impl SegmentCollector for SegmentBestTen {
    type Fruit = tantivy::Result<()>;

    fn collect(&mut self, doc: DocId, score: Score) {
        if self.error.is_some() {
            return;
        }
        let result = (|| -> tantivy::Result<()> {
            let mut selection = self.selection.lock().map_err(|_| {
                tantivy::TantivyError::InvalidArgument("search collection poisoned".to_owned())
            })?;
            selection.stats.matches_considered += 1;
            if selection.winners.len() == MAX_RESULTS
                && selection
                    .winners
                    .last()
                    .is_some_and(|worst| score.total_cmp(&worst.score) == Ordering::Less)
            {
                return Ok(());
            }
            let StoredUrl(field, url) = self.store.get::<StoredUrl>(doc)?;
            if field != self.url_field {
                return Err(tantivy::TantivyError::InvalidArgument(
                    "indexed URL is not the first stored field".to_owned(),
                ));
            }
            let candidate = Candidate {
                score,
                url,
                address: DocAddress::new(self.segment, doc),
            };
            if selection.winners.len() < MAX_RESULTS {
                selection.winners.push(candidate);
            } else if candidate.compare(&selection.winners[MAX_RESULTS - 1]) == Ordering::Less {
                selection.winners[MAX_RESULTS - 1] = candidate;
            } else {
                return Ok(());
            }
            selection.winners.sort_by(Candidate::compare);
            selection.stats.retained_candidates_peak = selection
                .stats
                .retained_candidates_peak
                .max(selection.winners.len());
            Ok(())
        })();
        if let Err(error) = result {
            self.error = Some(error);
        }
    }

    fn harvest(self) -> Self::Fruit {
        match self.error {
            Some(error) => Err(error),
            None => Ok(()),
        }
    }
}

impl Collector for BestTen {
    type Fruit = (Vec<Candidate>, SearchStats);
    type Child = SegmentBestTen;

    fn for_segment(
        &self,
        segment: SegmentOrdinal,
        reader: &SegmentReader,
    ) -> tantivy::Result<Self::Child> {
        Ok(SegmentBestTen {
            selection: Arc::clone(&self.selection),
            store: reader.get_store_reader(1)?,
            segment,
            url_field: self.url_field,
            error: None,
        })
    }

    fn requires_scoring(&self) -> bool {
        true
    }

    fn merge_fruits(&self, fruits: Vec<tantivy::Result<()>>) -> tantivy::Result<Self::Fruit> {
        for fruit in fruits {
            fruit?;
        }
        let mut selection = self.selection.lock().map_err(|_| {
            tantivy::TantivyError::InvalidArgument("search collection poisoned".to_owned())
        })?;
        Ok((std::mem::take(&mut selection.winners), selection.stats))
    }
}

/// Searches by descending score and ascending URL, retaining at most ten
/// candidates across all segments and loading bodies only for those winners.
///
/// # Errors
/// Returns query refusals and errors reading the index.
pub fn search(index: &WebIndex, query: &str) -> Result<Vec<ScoredResult>, SearchError> {
    search_with_stats(index, query).map(|(results, _)| results)
}

pub fn search_with_stats(
    index: &WebIndex,
    query: &str,
) -> Result<(Vec<ScoredResult>, SearchStats), SearchError> {
    let terms = query_terms(index, query)?;
    let query = build_query(index, &terms);
    let searcher = index.searcher();
    let fields = index.fields();
    let collector = BestTen {
        selection: Arc::new(Mutex::new(Selection::default())),
        url_field: fields.url,
    };
    let (winners, mut stats) = searcher.search(&query, &collector)?;
    let mut generator = SnippetGenerator::create(&searcher, &query, fields.body)?;
    generator.set_max_num_chars(SNIPPET_CHARS);
    let mut results = Vec::with_capacity(winners.len());
    for candidate in winners {
        let document = searcher.doc::<TantivyDocument>(candidate.address)?;
        stats.stored_bodies_loaded += 1;
        results.push(ScoredResult {
            score: candidate.score,
            result: SearchResult {
                url: candidate.url,
                title: stored_text(&document, fields.title),
                snippet: snippet_text(&generator, &stored_text(&document, fields.body)),
            },
        });
    }
    Ok((results, stats))
}

/// The search text: the compact JSON array of the results with keys url,
/// title and snippet in that order and no score.
///
/// # Errors
/// Returns the error serialising the results.
pub fn search_text(results: &[SearchResult]) -> Result<String, serde_json::Error> {
    serde_json::to_string(results)
}

/// The search canonical bytes: kind 2, the query as the payload, the media
/// type `application/json` and the compact JSON array as the text.
///
/// # Errors
/// Refuses a query longer than a uint32 length can carry and results that
/// do not serialise.
pub fn search_canonical_bytes(
    query: &str,
    results: &[SearchResult],
) -> Result<Vec<u8>, CanonicalError> {
    let text = search_text(results).map_err(|_| CanonicalError::Malformed)?;
    canonical::canonical_bytes(
        ContentKind::Search,
        query.as_bytes(),
        SEARCH_MEDIA_TYPE,
        &text,
    )
}

fn decode_query_component(value: &str) -> Result<String, Response> {
    let mut decoded = Vec::with_capacity(value.len());
    let mut bytes = value.bytes();
    while let Some(byte) = bytes.next() {
        match byte {
            b'%' => {
                let high = bytes.next().and_then(|byte| char::from(byte).to_digit(16));
                let low = bytes.next().and_then(|byte| char::from(byte).to_digit(16));
                let (Some(high), Some(low)) = (high, low) else {
                    return Err(Response::error(400, "malformed_query"));
                };
                decoded.push(((high << 4) | low) as u8);
            }
            b'+' => decoded.push(b' '),
            byte => decoded.push(byte),
        }
    }
    String::from_utf8(decoded).map_err(|_| Response::error(400, "malformed_query"))
}

pub(crate) fn search_query(request: &Request) -> Result<String, Response> {
    let raw = request
        .query
        .as_deref()
        .filter(|query| !query.is_empty())
        .ok_or_else(|| Response::error(400, "missing_query"))?;
    let mut query = None;
    for pair in raw.split('&') {
        let (key, value) = pair
            .split_once('=')
            .ok_or_else(|| Response::error(400, "malformed_query"))?;
        let key = decode_query_component(key)?;
        let value = decode_query_component(value)?;
        if key != "q" {
            return Err(Response::error(400, "unknown_query_parameter"));
        }
        if query.replace(value).is_some() {
            return Err(Response::error(400, "duplicate_query"));
        }
    }
    query
        .filter(|query| !query.is_empty())
        .ok_or_else(|| Response::error(400, "missing_query"))
}

fn validate_search_request(index: &WebIndex, request: &Request) -> Result<(), Response> {
    let query = search_query(request)?;
    query_terms(index, &query)
        .map(|_| ())
        .map_err(|error| Response::error(error.status(), error.code()))
}

/// The `GET /search?q=` resource: searches the index, writes the canonical
/// bytes to the content store and answers with the results and their digest.
#[must_use]
pub fn search_route(index: &WebIndex, store: &ContentStore, request: &Request) -> Response {
    let query = match search_query(request) {
        Ok(query) => query,
        Err(response) => return response,
    };
    let results: Vec<SearchResult> = match search(index, &query) {
        Ok(results) => results.into_iter().map(|scored| scored.result).collect(),
        Err(error) => return Response::error(error.status(), error.code()),
    };
    let canonical = match search_canonical_bytes(&query, &results) {
        Ok(canonical) => canonical,
        Err(error) => return Response::error(500, error.code()),
    };
    let Ok(digest) = store.put(&canonical) else {
        return Response::error(500, "content_store_error");
    };
    Response::json(
        200,
        serde_json::json!({
            "query": query,
            "media_type": SEARCH_MEDIA_TYPE,
            "digest": canonical::digest_hex(&digest),
            "results": results,
        })
        .to_string()
        .into_bytes(),
    )
}

/// Registers `GET /search` behind the payment gate.
///
/// # Errors
/// Refuses a route that already has a handler.
pub fn register(
    routes: &mut RouteTable,
    gate: &Arc<PaymentGate>,
    index: &Arc<WebIndex>,
    store: &Arc<ContentStore>,
) -> Result<(), RouteError> {
    let validation_index = Arc::clone(index);
    let (index, store) = (Arc::clone(index), Arc::clone(store));
    PaymentGate::install_validated(
        gate,
        routes,
        Route::Search,
        move |request: &Request| validate_search_request(&validation_index, request),
        move |request: &Request| search_route(&index, &store, request),
    )
}
