use serde_json::Value;

/// Displays bank units as observed by the gateway; this evidence never authorizes a native debit.
///
/// # Errors
/// Returns an error when input validation or the requested operation fails.
pub fn balances(snapshot: Value, did: &str, asset: Option<[u8; 32]>) -> Result<Value, String> {
    if snapshot["account"]["layerx_did"] != did {
        return Err("wallet_balance_identity_mismatch: gateway returned another account".into());
    }
    let mut value = layerx_sdk::rpc::BalancesSnapshot::try_from(snapshot)
        .map_err(|error| format!("wallet_balances_unavailable: {error:?}"))?
        .into_value();
    if let Some(asset) = asset {
        let denom = format!("layerx/{}", crate::wallet_gateway::hex(&asset));
        let rows = value["balances"].as_array_mut().ok_or("balances missing")?;
        rows.retain(|row| row["denom"] == denom);
    }
    Ok(value)
}
