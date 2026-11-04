//! The local inspection surface, separate from merged-node payment execution.

pub fn document() -> String {
    serde_json::json!({
        "network_mode": "emulator",
        "routes": [
            {"method": "GET", "path": "/healthz"},
            {"method": "GET", "path": "/v1/surface"},
            {"method": "GET", "path": "/v1/state"},
            {"method": "GET", "path": "/v1/sequencer"},
            {"method": "GET", "path": "/v1/protocol/account-state/head"},
            {"method": "GET", "path": "/v1/dids/{did}/sequence"},
            {"method": "GET", "path": "/v1/receipts/{id}"},
            {"method": "GET", "path": "/v1/evidence/{id}"},
            {"method": "GET", "path": "/v1/journeys/{id}"},
            {"method": "POST", "path": "/__emulator/accounts/prefund"},
            {"method": "POST", "path": "/__emulator/time/set"},
            {"method": "POST", "path": "/__emulator/time/advance"},
            {"method": "POST", "path": "/__emulator/faults"},
            {"method": "GET", "path": "/__emulator/snapshot"},
            {"method": "PUT", "path": "/__emulator/snapshot"}
        ],
        "payments": {
            "supported": false,
            "settlement": "retired",
            "settlement_http_status": 410,
            "code": super::settlement::RETIRED_CODE,
            "detail": super::settlement::RETIRED_DETAIL,
            "replacement": {
                "transport": "merged_node_json_rpc",
                "submit": "eth_sendRawTransaction",
                "receipt": "pax_sequencerReceipt",
                "authority": "ReceiptPayloadAuthority",
                "committed_requires": "independently verified block-commit evidence"
            }
        },
        "historical_imports": {
            "mode": "inspection_only",
            "routes": ["/v1/receipts/{id}", "/v1/evidence/{id}", "/v1/journeys/{id}"],
            "produces_new_payments": false,
            "establishes_merged_finality": false
        },
        "retired_ingress": {
            "http_status": 404,
            "paths": ["/v1/activities", "/v1/moves", "/v1/moves/quote", "/v1/programs", "/v1/programs/*"],
            "replacement": "merged-node transactions via eth_sendRawTransaction"
        }
    }).to_string()
}
