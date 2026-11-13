/// # Panics
/// Panics if required fixture data is absent, invalid, or an operation fails.
#[must_use]
pub fn live_route() -> (NativeReadRoute, NodeRpcConfig) {
    let block = json!({"number": "0x96", "hash": rpc_hex(&[5; 32]),
        "parentHash": rpc_hex(&[4; 32]), "timestamp": "0x3e8",
        "transactionsRoot": rpc_hex(&[6; 32]), "receiptsRoot": rpc_hex(&[7; 32]),
        "transactions": []});
    let mut exchanges = receipt_exchanges();
    exchanges.extend([
        ("eth_blockNumber", json!([]), json!("0x96")),
        ("eth_chainId", json!([]), json!("0x7d")),
        ("eth_blockNumber", json!([]), json!("0x96")),
        ("eth_chainId", json!([]), json!("0x7d")),
        ("eth_blockNumber", json!([]), json!("0x96")),
        ("eth_getBlockByNumber", json!(["0x96", true]), block.clone()),
        ("eth_getBlockByNumber", json!(["0x96", true]), block),
        ("eth_chainId", json!([]), json!("0x7d")),
        ("px_getHistory", json!([rpc_hex(&receipt_hash()), null, 2]),
            json!({"items": [], "next_cursor": null, "indexedHeight": HEIGHT, "chainHeight": HEIGHT})),
    ]);
    let (config, _server) = serve(exchanges);
    let route = NativeReadRoute::new(endpoint(&config), config.chain_id);
    (route, config)
}
