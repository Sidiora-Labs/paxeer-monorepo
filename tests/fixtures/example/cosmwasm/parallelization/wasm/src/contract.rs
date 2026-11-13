use crate::msg::{BankExecuteMsg, ExecuteMsg, InstantiateMsg};
use cosmwasm_std::{
    entry_point, to_json_binary, DepsMut, Env, MessageInfo, Response, StdError, WasmMsg,
};

#[entry_point]
pub fn instantiate(
    _: DepsMut,
    _env: Env,
    _: MessageInfo,
    _: InstantiateMsg,
) -> Result<Response, StdError> {
    Ok(Response::default())
}

#[entry_point]
pub fn execute(
    _: DepsMut,
    _: Env,
    info: MessageInfo,
    msg: ExecuteMsg,
) -> Result<Response, StdError> {
    match msg {
        ExecuteMsg::Send {
            bank_address,
            destination,
        } => send(info, bank_address, destination),
    }
}

fn send(
    info: MessageInfo,
    bank_address: String,
    destination: String,
) -> Result<Response, StdError> {
    let bank_msg = BankExecuteMsg::Send { destination };
    let msg = WasmMsg::Execute {
        contract_addr: bank_address,
        msg: to_json_binary(&bank_msg).unwrap(),
        funds: info.funds,
    };
    Ok(Response::new().add_message(msg))
}
