use crate::production_fixture::{Fixture, Result};
use layerx_types::clock::Clock;
use serde_json::{json, Value};
use std::time::Duration;

pub(crate) fn plan(fixture: &Fixture) -> Result<Value> {
    fixture.request(
        "POST",
        "/v1/intents/plan",
        &fixture.document["intent"],
        "intent-plan-evm",
    )
}

pub(crate) fn submission(fixture: &Fixture, plan: &Value) -> Result<Value> {
    let now = fixture.clock.sample(Duration::from_secs(1))?.unix_seconds();
    let deadline = fixture.document["not_after"]
        .as_u64()
        .ok_or("fixture deadline")?;
    let actor = fixture.document["actor"].as_str().ok_or("fixture actor")?;
    let bindings = plan["signing_requirements"]
        .as_array()
        .ok_or("signing requirements")?
        .iter()
        .enumerate()
        .map(|(index, requirement)| {
            json!({
                "leg_index": index, "action_key": requirement["action_key"], "actor": actor,
                "authority": requirement["authority"], "relationship": "self",
                "account_sequence": plan["legs"][index]["nonce"], "not_before": now,
                "not_after": deadline, "fee_limit": plan["legs"][index]["fee"],
            })
        })
        .collect::<Vec<_>>();
    Ok(
        json!({"plan_digest": plan["plan_digest"], "signed_digest": plan["plan_digest"], "bindings": bindings}),
    )
}
