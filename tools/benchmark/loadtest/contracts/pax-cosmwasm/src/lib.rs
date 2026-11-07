use cosmwasm_std::{Addr, Coin, CosmosMsg, CustomMsg, CustomQuery, Decimal};
use schemars::JsonSchema;
use serde::{Deserialize, Serialize};
use serde_repr::{Deserialize_repr, Serialize_repr};

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
#[serde(rename_all = "snake_case")]
pub enum PaxMsg {
    PlaceOrders {
        orders: Vec<Order>,
        funds: Vec<Coin>,
        contract_address: Addr,
    },
}

impl CustomMsg for PaxMsg {}

impl From<PaxMsg> for CosmosMsg<PaxMsg> {
    fn from(message: PaxMsg) -> Self {
        CosmosMsg::Custom(message)
    }
}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
pub struct Order {
    pub price: Decimal,
    pub quantity: Decimal,
    pub price_denom: String,
    pub asset_denom: String,
    pub order_type: OrderType,
    pub position_direction: PositionDirection,
    pub data: String,
    pub status_description: String,
    pub nominal: Decimal,
}

#[derive(Serialize_repr, Deserialize_repr, Copy, Clone, Debug, PartialEq, Eq, Hash, JsonSchema)]
#[repr(i32)]
pub enum PositionDirection {
    Long = 0,
    Short = 1,
}

#[derive(Serialize_repr, Deserialize_repr, Copy, Clone, Debug, PartialEq, Eq, Hash, JsonSchema)]
#[repr(i32)]
pub enum OrderType {
    Limit = 0,
    Market = 1,
    Liquidation = 2,
    Fokmarket = 3,
    Fokmarketbyvalue = 4,
}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
#[serde(rename_all = "snake_case")]
pub struct PaxQueryWrapper {
    pub route: PaxRoute,
    pub query_data: PaxQuery,
}

impl CustomQuery for PaxQueryWrapper {}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
#[serde(rename_all = "snake_case")]
pub enum PaxRoute {
    Oracle,
    Dex,
    Epoch,
    Tokenfactory,
}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
#[serde(rename_all = "snake_case")]
pub enum PaxQuery {
    ExchangeRates {},
    OracleTwaps {
        lookback_seconds: i64,
    },
    DexTwaps {
        contract_address: Addr,
        lookback_seconds: u64,
    },
    Epoch {},
    GetOrders {
        contract_address: Addr,
        account: Addr,
    },
    GetOrderById {
        contract_address: Addr,
        price_denom: String,
        asset_denom: String,
        id: u64,
    },
    GetLatestPrice {
        contract_address: Addr,
        price_denom: String,
        asset_denom: String,
    },
    OrderSimulation {
        contract_address: Addr,
        order: Order,
    },
}
