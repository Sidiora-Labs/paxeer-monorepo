use chainflow_data_gateway::{ChainflowDataGatewayClient, Error, Market, WsScope, WsTokenRequest};
use std::time::Duration;

#[tokio::main]
async fn main() {
    let args: Vec<String> = std::env::args().collect();
    assert_eq!(args.len(), 6);
    let mut builder = ChainflowDataGatewayClient::builder()
        .base_url(&args[2])
        .timeout(Duration::from_secs(5))
        .max_retries(0);
    if args[3] != "unset" {
        builder = builder.rest_api_key(&args[3]);
    }
    if args[4] != "unset" {
        builder = builder.ws_mint_key(&args[4]);
    }
    if args[5] != "unset" {
        builder = builder.ws_token(&args[5]);
    }
    let client = builder.build().expect("SDK client configuration");
    // The Go peer reads the real wire request and then disconnects. It does
    // not impersonate a gateway or manufacture an authentication response.
    match args[1].as_str() {
        "rest" => {
            let result = client.trading_view().list_feed_time(Market::Btc).send().await;
            assert!(matches!(result, Err(Error::Transport(_))), "expected peer disconnect");
        }
        "mint" => {
            let result = client
                .gateway()
                .create_web_socket_token(WsTokenRequest { symbol: WsScope::Btc })
                .send()
                .await;
            assert!(matches!(result, Err(Error::Transport(_))), "expected peer disconnect");
        }
        operation => {
            let resource = client.web_sockets();
            let result = match operation {
                "price" => resource.connect_price_feed(Market::Btc).open().await,
                "markets" => resource.connect_markets().open().await,
                "explorer" => resource.connect_explorer().open().await,
                "alias" => resource.connect_explorer_alias().open().await,
                _ => panic!("unknown operation"),
            };
            assert!(
                matches!(result, Err(Error::WebSocket { response: None, .. })),
                "expected peer disconnect"
            );
        }
    }
}
