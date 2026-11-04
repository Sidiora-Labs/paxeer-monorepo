use cosmwasm_std::testing::{mock_env, mock_info, MockApi, MockQuerier};
use cosmwasm_std::{
    from_binary, Addr, CosmosMsg, Decimal, DepsMut, MemoryStorage, QuerierWrapper, Response,
};
use jupiter::contract::{instantiate, sudo};
use jupiter::msg::{BulkOrderPlacementsResponse, InstantiateMsg, SudoMsg};
use pax_cosmwasm::{OrderType, PaxMsg, PaxQueryWrapper, PositionDirection};

fn bulk_order_responses(heights: &[u64], mars_address: &str) -> Vec<Response<PaxMsg>> {
    let mut storage = MemoryStorage::default();
    let api = MockApi::default();
    let querier = MockQuerier::<PaxQueryWrapper>::new(&[]);
    instantiate(
        DepsMut {
            storage: &mut storage,
            api: &api,
            querier: QuerierWrapper::new(&querier),
        },
        mock_env(),
        mock_info("creator", &[]),
        InstantiateMsg {
            mars_address: Addr::unchecked(mars_address),
        },
    )
    .expect("instantiate must persist the Mars destination");

    heights
        .iter()
        .map(|height| {
            let mut env = mock_env();
            env.block.height = *height;
            let request = SudoMsg::BulkOrderPlacements {
                orders: vec![],
                deposits: vec![],
            };
            let request_bytes = serde_json::to_vec(&request).unwrap();
            let response = sudo(
                DepsMut {
                    storage: &mut storage,
                    api: &api,
                    querier: QuerierWrapper::new(&querier),
                },
                env,
                serde_json::from_slice(&request_bytes).unwrap(),
            )
            .expect("bulk order placement must succeed");

            // Exercise the response codec consumed by the contract caller.
            let response_bytes = serde_json::to_vec(&response).unwrap();
            serde_json::from_slice(&response_bytes).unwrap()
        })
        .collect()
}

#[test]
fn f044_bulk_order_response_between_periodic_orders() {
    for response in bulk_order_responses(&[1, 9, 11, 19, 21], "mars-between-orders") {
        assert!(response.messages.is_empty());
        let data = response.data.expect("bulk-order response data is required");
        assert_eq!(data.as_slice(), br#"{"unsuccessful_orders":[]}"#);
        let decoded: BulkOrderPlacementsResponse = from_binary(&data).unwrap();
        assert!(decoded.unsuccessful_orders.is_empty());
    }
}

#[test]
fn f044_bulk_order_response_survives_periodic_mars_order() {
    let heights = [9, 10, 11, 19, 20, 21];
    let responses = bulk_order_responses(&heights, "mars-periodic-orders");
    let baseline = responses[0]
        .data
        .as_ref()
        .expect("baseline data is required");

    for (height, response) in heights.iter().zip(&responses) {
        let data = response
            .data
            .as_ref()
            .expect("adding the Mars order must preserve bulk-order response data");
        assert_eq!(
            data, baseline,
            "response encoding changed at height {height}"
        );
        let decoded: BulkOrderPlacementsResponse = from_binary(data).unwrap();
        assert!(decoded.unsuccessful_orders.is_empty());

        if height % 10 == 0 {
            assert_eq!(response.messages.len(), 1);
            assert_eq!(
                serde_json::to_value(&response.messages[0].msg).unwrap(),
                serde_json::json!({
                    "custom": {
                        "place_orders": {
                            "orders": [{
                                "price": "120",
                                "quantity": "1",
                                "price_denom": "PAX",
                                "asset_denom": "ATOM",
                                "order_type": 0,
                                "position_direction": 0,
                                "data": "",
                                "status_description": "",
                                "nominal": "0"
                            }],
                            "funds": [],
                            "contract_address": "mars-periodic-orders"
                        }
                    }
                })
            );
            match &response.messages[0].msg {
                CosmosMsg::Custom(PaxMsg::PlaceOrders {
                    funds,
                    orders,
                    contract_address,
                }) => {
                    assert_eq!(contract_address, &Addr::unchecked("mars-periodic-orders"));
                    assert!(funds.is_empty());
                    assert_eq!(orders.len(), 1);
                    let order = &orders[0];
                    assert_eq!(order.price, Decimal::from_ratio(120u128, 1u128));
                    assert_eq!(order.quantity, Decimal::one());
                    assert_eq!(order.price_denom, "PAX");
                    assert_eq!(order.asset_denom, "ATOM");
                    assert_eq!(order.position_direction, PositionDirection::Long);
                    assert_eq!(order.order_type, OrderType::Limit);
                    assert!(order.data.is_empty());
                    assert!(order.status_description.is_empty());
                    assert_eq!(order.nominal, Decimal::zero());
                }
                message => panic!("unexpected periodic message: {:?}", message),
            }
        } else {
            assert!(response.messages.is_empty());
        }
    }
}
