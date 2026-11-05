"""Offline source/recording qualification; Rust and Phoenix execution is review-only.

No translated client, substitute producer, subprocess, or live service is used.
The Rust integration cases exercise ApiClient when its native suite is qualified.
"""

import hashlib
import json
from pathlib import Path
import re
import tomllib


ROOT = Path(__file__).resolve().parents[3]
PACKAGE = ROOT / "services/explorer/services/libs/blockscout-client"
BACKEND = ROOT / "services/explorer/backend/apps/block_scout_web/lib/block_scout_web"
RECORDINGS = PACKAGE / "crate/tests/recorded/eth_blockscout_com"
NFT_FILE = (
    "_api_v2_tokens_0xD4416b13d2b3a9aBae7AcD5D6C2BbDBE25686401_instances.json"
)
LAST_ID = (
    "115785665443821794889628784622978168165342723005226866880138161782244014770980"
)


def read(path):
    return path.read_text(encoding="utf-8")


def recording(name):
    return json.loads(read(RECORDINGS / name))


def rust_case(name):
    source = read(PACKAGE / "crate/tests/it_works.rs")
    pattern = rf"^(?:async )?fn {re.escape(name)}\(.*?(?=^#\[|\Z)"
    match = re.search(pattern, source, re.M | re.S)
    assert match is not None, f"missing declared test/helper {name}"
    return match.group()


def test_legacy_health_is_versioned_and_keeps_original_decoder_assertions():
    legacy = recording("_api_v1_health.json")
    assert legacy["metadata"] == {
        "compatibility_version": "upstream-v1-2025-02-19",
        "merged_explorer_supported": False,
    }
    assert legacy["request"] == {"method": "GET", "urlPath": "/api/v1/health"}
    assert legacy["response"]["status"] == 200
    body = legacy["response"]["body"]
    assert hashlib.sha256(body.encode()).hexdigest() == (
        "bad67372164a928dbf637a4cac152d12561b2b2473f489d7ec246ea0241b8532"
    )
    decoded = json.loads(body)
    assert decoded["healthy"] is True
    assert decoded["data"]["latest_block_number"] == "21879216"
    case = rust_case("legacy_upstream_health_v1_2025_02_19")
    for fact in (
        '"recorded/eth_blockscout_com/_api_v1_health.json"',
        "blockscout_client::models::V1HealthCheckResponse",
        'assert_eq!(success.healthy, Some(true))',
        '.expect("data is None")',
        ".latest_block_number",
        '"21879216"',
    ):
        assert fact in case
    assert ".health_v1()" not in case


def test_legacy_health_local_router_and_client_declared_refusal():
    router = read(BACKEND / "routers/api_router.ex")
    v1 = router.split('scope "/v1", as: :api_v1 do', 1)[1].split(
        'scope "/health" do', 1
    )[0]
    assert not re.search(r'(?:get|post|match|forward)\s*\(?\s*"/health', v1)
    assert 'get("/", BlockScoutWeb.API.HealthController, :health)' in router.split(
        'scope "/health" do', 1
    )[1].split("\n  end", 1)[0]
    health = read(PACKAGE / "crate/src/apis/health_api.rs")
    implementation = health.split("impl HealthApi for HealthApiClient", 1)[1]
    refusal = implementation.split("async fn health_v1(", 1)[1].split("\n    }", 1)[0]
    assert "Err(Error::UnsupportedCapability(" in refusal
    assert (
        "upstream /api/v1/health is not mounted; use health() at /api/health"
        in refusal
    )
    assert ".request(" not in refusal and ".execute(" not in refusal
    supported = implementation.split("async fn health(", 1)[1].split(
        "async fn health_v1(", 1
    )[0]
    for fact in ('{}/api/health', "reqwest::Method::GET", ".execute(local_var_req)"):
        assert fact in supported
    assert "UnsupportedCapability" in rust_case("health")
    case = rust_case("supported_health_http_and_legacy_refusal")
    for fact in (
        '"/api/health".to_owned()',
        'include_str!("recorded/eth_blockscout_com/_api_health.json")',
        "client.health_api().health_v1().await",
        "Error::UnsupportedCapability(",
        "client.health_api().health().await.unwrap()",
        "server.await.unwrap()",
        "assert_eq!(response.status.as_u16(), 200)",
        "response.try_parse_as_success_model().unwrap()",
        "assert_eq!(typed.healthy, true)",
    ):
        assert fact in case
    assert case.index(".health_v1()") < case.index(".health().await")
    current = recording("_api_health.json")
    assert current["request"] == {"method": "GET", "urlPath": "/api/health"}
    assert current["response"]["status"] == 200
    payload = json.loads(current["response"]["body"])
    assert payload["healthy"] is True
    assert payload["metadata"]["blocks"]["healthy"] is True
    for source in ("db", "cache", "node"):
        assert (
            payload["metadata"]["blocks"]["latest_block"][source]["number"]
            == "23116935"
        )


def test_nft_recording_preserves_full_integer_and_all_other_body_bytes():
    nft = recording(NFT_FILE)
    assert nft["metadata"]["fresh_producer_capture"] is False
    assert (
        nft["metadata"]["capture_kind"]
        == "historical-upstream-with-reconstructed-cursor"
    )
    assert nft["request"] == {
        "method": "GET",
        "urlPath": "/api/v2/tokens/0xD4416b13d2b3a9aBae7AcD5D6C2BbDBE25686401/instances",
    }
    assert nft["response"]["status"] == 200
    assert nft["response"]["headers"] == {"Content-Type": "application/json"}
    body = nft["response"]["body"]
    page = json.loads(body)
    assert page["items"][-1]["id"] == LAST_ID
    cursor = page["next_page_params"]["unique_token"]
    assert type(cursor) is int
    assert cursor == int(LAST_ID) and cursor > 2**64
    exact = '"next_page_params":{"unique_token":' + LAST_ID + "}"
    assert body.count(exact) == 1
    restored_original = body.replace(
        exact, '"next_page_params":{"unique_token":1.157856654438218e+77}'
    )
    assert hashlib.sha256(restored_original.encode()).hexdigest() == (
        "04db983facdfc105d657861ebdb58ecf035e1575fe633db319eea9470baead39"
    )


def test_nft_producer_and_recorder_preserve_raw_response_contract():
    chain = read(BACKEND / "chain.ex")
    assert re.search(
        r'defp paging_params\(%Instance\{token_id: token_id\}\) do\s*'
        r'%\{"unique_token" => Decimal.to_integer\(token_id\)\}\s*end',
        chain,
    )
    assert "Map.merge(paging_params(List.last(list)))" in chain
    controller = read(BACKEND / "controllers/api/v2/token_controller.ex")
    instances = controller.split(
        "def instances(conn, %{address_hash_param: address_hash_string} = params) do", 1
    )[1].split("\n  end", 1)[0]
    for fact in (
        "Instance.address_to_unique_tokens(",
        "unique_tokens_paging_options(params)",
        "split_list_by_page(results_plus_one)",
        "unique_tokens_next_page(token_instances, params)",
        "put_status(200)",
        "render(:token_instances",
        "next_page_params: next_page_params",
    ):
        assert fact in instances
    view = read(BACKEND / "views/api/v2/token_view.ex")
    instances_view = view.split('def render("token_instances.json",', 1)[1].split(
        "\n  end", 1
    )[0]
    assert '"next_page_params" => next_page_params' in instances_view
    recorder = read(PACKAGE / "ts-src/record-stubs.ts")
    assert "const body = await response.text();" in recorder
    assert "status: response.status," in recorder
    assert re.search(r"response:\s*\{\s*status: response.status,\s*body,", recorder)
    assert "response.json(" not in recorder
    assert "JSON.parse(" not in recorder
    assert "fs.outputJson(destination, recording," in recorder


def test_nft_rust_regression_forwards_cursor_through_production_http_client():
    case = rust_case("nft_full_width_cursor_survives_real_client_http")
    assert LAST_ID in case and NFT_FILE in case
    assert case.count(".get_nft_instances(") == 2
    assert re.search(
        r'\blet cursor = first\s*\.next_page_cursor\(\)\s*\.unwrap\(\)'
        r'\s*\.expect\("nonterminal cursor"\);',
        case,
    )
    for fact in (
        'format!("{path}?unique_token={LAST_ID}")',
        "assert_eq!(typed.items.last().unwrap().id, LAST_ID)",
        'vec![("unique_token", LAST_ID)]',
        ".cursor(cursor)",
        "server.await.unwrap()",
        "assert_eq!(forwarded.status.as_u16(), 200)",
        "assert_eq!(forwarded.content, first.content)",
    ):
        assert fact in case
    tokens = read(PACKAGE / "crate/src/apis/tokens_api.rs")
    implementation = tokens.split("impl TokensApi for TokensApiClient", 1)[1]
    instances = implementation.split("async fn get_nft_instances(", 1)[1].split(
        "async fn get_token(", 1
    )[0]
    for fact in (
        '"{}/api/v2/tokens/{address_hash}/instances"',
        "cursor.as_ref().or(self.cursor.as_ref())",
        "cursor.apply(&mut local_var_req)?",
        ".execute(local_var_req).await?",
    ):
        assert fact in instances
    pagination = read(PACKAGE / "crate/src/apis/pagination.rs")
    for fact in (
        'members(&self.content)?',
        'if key == "next_page_params"',
        "PageCursor::from_json(raw)?",
        "_ => raw.to_owned()",
        ".extend_pairs(query)",
    ):
        assert fact in pagination
    assert "parse::<f64>" not in pagination
    assert "pub id: String" in read(PACKAGE / "crate/src/models/nft_instance.rs")


def test_rust_peer_uses_real_transport_and_joins_request_assertions():
    peer = rust_case("recording_peer")
    for fact in (
        'TcpListener::bind("127.0.0.1:0")',
        "listener.accept().await.unwrap()",
        'format!("GET {target} HTTP/1.1")',
        'envelope["response"]["body"].as_str()',
        'envelope["response"]["status"].as_u64()',
        "stream.write_all(response.as_bytes()).await.unwrap()",
        "tokio::time::timeout(Duration::from_secs(10)",
        "reqwest::Client::builder()",
        ".no_proxy()",
        "ApiClient::new(",
        "reqwest_middleware::ClientBuilder::new(http).build()",
    ):
        assert fact in peer
    assert "Stubr" not in peer and "mock" not in peer
    manifest = tomllib.loads(read(PACKAGE / "crate/Cargo.toml"))
    assert manifest["package"]["name"] == "blockscout-client"
    assert manifest["package"]["edition"] == "2021"
    assert "full" in manifest["dev-dependencies"]["tokio"]["features"]


def test_repair_record_names_findings_tests_and_out_of_scope_wiring():
    record = tomllib.loads(read(Path(__file__).with_name("repair_22_33_567.kvx")))
    assert record["meta"]["task"] == "22.33.567"
    for finding in ("legacy-health", "nft-cursor-precision"):
        tests = record["finding"][finding]["tests"]
        assert tests
        for test in tests:
            assert callable(globals()[test]), test
    assert record["proof"]["native_execution"] == "not run by the Python-only gate"
    assert record["proof"]["fresh_producer_capture"] is False
    assert "not run" in record["baseline"]["verification"]
    assert set(record["wiring"]) == {"recorder", "capture", "router"}
    for wiring in record["wiring"].values():
        assert "Domain 14" in wiring["request"]
        assert "services/explorer/" in wiring["request"]
    documentation = read(RECORDINGS / "README.md")
    for fact in (
        "HealthApi::health()",
        "GET /api/health",
        "HealthApi::health_v1()",
        "Error::UnsupportedCapability",
        "upstream-v1-2025-02-19",
        "not a",
        "fresh producer capture",
        LAST_ID,
    ):
        assert fact in documentation
