//! Typed owner adapters for catalogue operations outside the base envelope dispatcher.
//!
//! Each arm decodes the operation `request` strictly into the exact arguments of an existing
//! `HumanOperations` method and calls it on the shared daemon owner. Operations without an
//! existing owner method return `None`, so the dispatcher keeps them unmatched.

use layerx_agent_api::error::{ErrorClass, RequestId, Retriability};
use layerx_agent_api::verify::RungStatus as VerificationStatus;
use serde::Deserialize;
use serde_json::{Map, Value};

use crate::agent_rpc::Rejection;
use crate::agent_rpc_dispatch::{DispatchContext, Dispatched};
use crate::agent_rpc_peer::RpcOwnerContext;
use crate::agent_rpc_wire::Canonical;
use crate::human::HumanRefusal;
use crate::human::{HumanOperationError, HumanOperations, HumanResponse};
use crate::human_runtime::{HumanAuthorityBoundary, SharedAgentOwner};
use crate::session_control::OperationPermit;
use crate::tenant::Operation;
use layerx_agent_api::budget::{AuthorityResponse, BudgetRecord};
use layerx_agent_api::read::Freshness;
type HistorySelector = layerx_agent_api::read::HistorySelector;
use layerx_types::rung::Rung;
use sha2::{Digest, Sha256};

pub(crate) fn sign<A: HumanAuthorityBoundary>(
    owner: &SharedAgentOwner<A>,
    context: &crate::agent_rpc_peer::RpcOwnerContext<'_>,
    request: &serde_json::Map<String, serde_json::Value>,
    ctx: &DispatchContext,
) -> Result<Dispatched, Rejection> {
    use crate::agent_rpc_dispatch::{decode_observation, dispatched, mutation_key};
    use crate::agent_rpc_wire::{decode_wire, SignRequestWire};
    let id = ctx.request_id;
    let typed = decode_wire::<SignRequestWire>(request, id)?.into_request(id)?;
    mutation_key(ctx)?;
    let response = owner
        .lock()
        .and_then(|mut guard| guard.rpc_sign(context, typed));
    dispatched(id, response, decode_observation)
}

pub(crate) fn subscription_create<A: HumanAuthorityBoundary>(
    owner: &SharedAgentOwner<A>,
    context: &RpcOwnerContext<'_>,
    request: &Map<String, Value>,
    ctx: &DispatchContext,
) -> Result<Dispatched, Rejection> {
    use crate::agent_rpc_dispatch::mutation_key;
    let id = ctx.request_id;
    let wire: crate::agent_rpc_wire::SubscriptionCreateWire = decode(request, id)?;
    let typed = wire.into_request(id)?;
    let envelope = crate::human::MutationEnvelope {
        request_id: id.0,
        key: mutation_key(ctx)?,
        body_digest: crate::human_runtime::subscription_create_digest(&typed),
        operation: typed,
    };
    let response = owner
        .lock()
        .and_then(|mut guard| guard.rpc_subscription_create(context, envelope))
        .map_err(|error| owner_error(id, error))?;
    Ok(Dispatched {
        value: subscription_record_value(id, response.bytes())?,
        verification: None,
    })
}

pub(crate) fn subscription_list<A: HumanAuthorityBoundary>(
    owner: &SharedAgentOwner<A>,
    context: &RpcOwnerContext<'_>,
    request: &Map<String, Value>,
    ctx: &DispatchContext,
) -> Result<Dispatched, Rejection> {
    let id = ctx.request_id;
    let wire: crate::agent_rpc_wire::SubscriptionListWire = decode(request, id)?;
    let typed = wire.into_request(id)?;
    let response = owner
        .lock()
        .and_then(|mut guard| guard.rpc_subscription_list(context, typed))
        .map_err(|error| owner_error(id, error))?;
    Ok(Dispatched {
        value: subscription_list_value(id, response.bytes())?,
        verification: None,
    })
}

pub(crate) fn subscription_pause<A: HumanAuthorityBoundary>(
    owner: &SharedAgentOwner<A>,
    context: &RpcOwnerContext<'_>,
    request: &Map<String, Value>,
    ctx: &DispatchContext,
) -> Result<Dispatched, Rejection> {
    let id = ctx.request_id;
    let wire: crate::agent_rpc_wire::SubscriptionTargetWire = decode(request, id)?;
    let typed = wire.into_request(id)?;
    let response = owner
        .lock()
        .and_then(|mut guard| guard.rpc_subscription_pause(context, typed))
        .map_err(|error| owner_error(id, error))?;
    Ok(Dispatched {
        value: subscription_record_value(id, response.bytes())?,
        verification: None,
    })
}

pub(crate) fn subscription_resume<A: HumanAuthorityBoundary>(
    owner: &SharedAgentOwner<A>,
    context: &RpcOwnerContext<'_>,
    request: &Map<String, Value>,
    ctx: &DispatchContext,
) -> Result<Dispatched, Rejection> {
    let id = ctx.request_id;
    let wire: crate::agent_rpc_wire::SubscriptionTargetWire = decode(request, id)?;
    let typed = wire.into_request(id)?;
    let response = owner
        .lock()
        .and_then(|mut guard| guard.rpc_subscription_resume(context, typed))
        .map_err(|error| owner_error(id, error))?;
    Ok(Dispatched {
        value: subscription_record_value(id, response.bytes())?,
        verification: None,
    })
}

pub(crate) fn subscription_delete<A: HumanAuthorityBoundary>(
    owner: &SharedAgentOwner<A>,
    context: &RpcOwnerContext<'_>,
    request: &Map<String, Value>,
    ctx: &DispatchContext,
) -> Result<Dispatched, Rejection> {
    let id = ctx.request_id;
    let wire: crate::agent_rpc_wire::SubscriptionTargetWire = decode(request, id)?;
    let typed = wire.into_request(id)?;
    let response = owner
        .lock()
        .and_then(|mut guard| guard.rpc_subscription_delete(context, typed))
        .map_err(|error| owner_error(id, error))?;
    Ok(Dispatched {
        value: subscription_null_value(id, response.bytes())?,
        verification: None,
    })
}

pub(crate) fn subscription_health<A: HumanAuthorityBoundary>(
    owner: &SharedAgentOwner<A>,
    context: &RpcOwnerContext<'_>,
    request: &Map<String, Value>,
    ctx: &DispatchContext,
) -> Result<Dispatched, Rejection> {
    let id = ctx.request_id;
    let wire: crate::agent_rpc_wire::SubscriptionTargetWire = decode(request, id)?;
    let typed = wire.into_request(id)?;
    let mut guard = owner.lock().map_err(|error| owner_error(id, error))?;
    let response = guard
        .rpc_subscription_health(context, typed.clone())
        .map_err(|error| owner_error(id, error))?;
    let store = guard.session_control.store();
    let store = store
        .lock()
        .map_err(|_| owner_error(id, HumanOperationError::Unavailable))?;
    let positions =
        SubscriptionCursorSnapshot::read(&store, &typed, Some(&context.permit().credential()), id)?;
    let mut value = subscription_health_value(id, response.bytes())?;
    value
        .as_object_mut()
        .ok_or_else(|| malformed(id))?
        .extend(positions.health_fields());
    Ok(Dispatched {
        value,
        verification: None,
    })
}

pub(crate) fn subscription_acknowledge<A: HumanAuthorityBoundary>(
    owner: &SharedAgentOwner<A>,
    context: &RpcOwnerContext<'_>,
    request: &Map<String, Value>,
    ctx: &DispatchContext,
) -> Result<Dispatched, Rejection> {
    let id = ctx.request_id;
    let wire: crate::agent_rpc_wire::CursorAcknowledgementWire = decode(request, id)?;
    let typed = wire.into_request(id)?;
    let mut guard = owner.lock().map_err(|error| owner_error(id, error))?;
    let response = match guard.rpc_subscription_acknowledge(context, typed.clone()) {
        Ok(response) => response,
        Err(HumanOperationError::Refused) => {
            context
                .permit()
                .boundary(&guard.session_control)
                .map_err(|_| owner_error(id, HumanOperationError::Refused))?;
            let target = layerx_agent_api::subscription::SubscriptionTarget {
                scope: typed.scope,
                subscription_id: typed.subscription_id,
            };
            let store = guard.session_control.store();
            let store = store
                .lock()
                .map_err(|_| owner_error(id, HumanOperationError::Unavailable))?;
            let positions = SubscriptionCursorSnapshot::read(
                &store,
                &target,
                Some(&context.permit().credential()),
                id,
            )?;
            return Err(positions.acknowledgement_error(typed.cursor.into(), id));
        }
        Err(error) => return Err(owner_error(id, error)),
    };
    Ok(Dispatched {
        value: subscription_record_value(id, response.bytes())?,
        verification: None,
    })
}

pub struct SubscriptionCursorSnapshot {
    last_delivered: crate::events::subscription::Cursor,
    ingest_watermark: Option<crate::events::subscription::Cursor>,
}

impl SubscriptionCursorSnapshot {
    pub fn read(
        store: &crate::store::Store,
        target: &layerx_agent_api::subscription::SubscriptionTarget,
        credential: Option<&crate::session::SessionCredential>,
        id: RequestId,
    ) -> Result<Self, Rejection> {
        use crate::store::{ObjectKind, StorageClass, TenantId, TenantKey};
        let corrupt = || {
            rejection(
                ErrorClass::InternalFault,
                id,
                "subscription.cursor_state_malformed",
            )
        };
        let refused = || owner_error(id, HumanOperationError::Refused);
        let tenant = TenantId::new(target.scope.tenant.as_str()).map_err(|_| refused())?;
        let key = TenantKey::new(
            tenant.clone(),
            ObjectKind::Subscription,
            target.subscription_id.as_str().as_bytes().to_vec(),
        )
        .map_err(|_| corrupt())?;
        let record = store.get(&key).ok_or_else(refused)?;
        if record.class() != StorageClass::LocalOnly {
            return Err(corrupt());
        }
        let mut reader = Reader {
            bytes: record.bytes(),
            offset: 0,
        };
        let magic = reader.take(4).ok_or_else(corrupt)?;
        if magic == b"LXS2" || magic == b"LXSB" {
            return Err(rejection(
                ErrorClass::PolicyRefusal,
                id,
                "subscription.cursor_reset_required",
            ));
        }
        if magic != b"LXS3" {
            return Err(corrupt());
        }
        for expected in [
            target.subscription_id.as_str(),
            target.scope.tenant.as_str(),
            target.scope.agent.as_str(),
            target.scope.capability.as_str(),
        ] {
            if reader.text().ok_or_else(corrupt)? != expected {
                return Err(refused());
            }
        }
        match reader.u8().ok_or_else(corrupt)? {
            0 if credential.is_none() => {}
            0 => return Err(refused()),
            1 => {
                let session = reader.fixed::<32>().ok_or_else(corrupt)?;
                let token = reader.fixed::<32>().ok_or_else(corrupt)?;
                let generation = reader.u64().ok_or_else(corrupt)?;
                if !credential.is_some_and(|credential| {
                    credential.tenant() == &tenant
                        && credential.session_id().0 == session
                        && credential.token_id() == token
                        && credential.generation() == generation
                }) {
                    return Err(refused());
                }
            }
            _ => return Err(corrupt()),
        }
        reader.text().ok_or_else(corrupt)?;
        let start = subscription_binary_cursor(&mut reader).ok_or_else(corrupt)?;
        let mut last_delivered = subscription_binary_cursor(&mut reader).ok_or_else(corrupt)?;
        if last_delivered < start {
            return Err(corrupt());
        }
        let count = reader.u32().ok_or_else(corrupt)?;
        for _ in 0..count {
            let delivered = subscription_binary_cursor(&mut reader).ok_or_else(corrupt)?;
            if delivered <= last_delivered {
                return Err(corrupt());
            }
            last_delivered = delivered;
        }
        subscription_flag(&mut reader).ok_or_else(corrupt)?;
        match reader.u8().ok_or_else(corrupt)? {
            0 => {}
            1 => {
                reader.take(26).ok_or_else(corrupt)?;
            }
            2 => {
                reader.take(24).ok_or_else(corrupt)?;
            }
            _ => return Err(corrupt()),
        }
        if reader.u8().ok_or_else(corrupt)? != 0 {
            return Err(refused());
        }
        let key = TenantKey::new(
            tenant,
            ObjectKind::Configuration,
            b"event-ingestion-watermark".to_vec(),
        )
        .map_err(|_| corrupt())?;
        let ingest_watermark = match store.get(&key) {
            None => None,
            Some(value) => {
                if value.class() != StorageClass::LocalOnly {
                    return Err(corrupt());
                }
                let mut reader = Reader {
                    bytes: value.bytes(),
                    offset: 0,
                };
                if reader.take(4).ok_or_else(corrupt)? != b"LXEW" {
                    return Err(corrupt());
                }
                let ingested = subscription_flag(&mut reader).ok_or_else(corrupt)?;
                let last = reader.u64().ok_or_else(corrupt)?;
                let next = reader.u64().ok_or_else(corrupt)?;
                let positioned = subscription_flag(&mut reader).ok_or_else(corrupt)?;
                let position = subscription_binary_cursor(&mut reader).ok_or_else(corrupt)?;
                reader.finish().ok_or_else(corrupt)?;
                if (ingested && last.checked_add(1) != Some(next))
                    || (positioned && !ingested)
                    || (!positioned
                        && (position.block_height != 0
                            || position.tx_index != 0
                            || position.log_index != 0))
                {
                    return Err(corrupt());
                }
                positioned.then_some(position)
            }
        };
        Ok(Self {
            last_delivered,
            ingest_watermark,
        })
    }

    pub fn health_fields(&self) -> Map<String, Value> {
        Map::from_iter([
            (
                "last_delivered".into(),
                subscription_position_value(self.last_delivered),
            ),
            (
                "ingest_watermark".into(),
                self.ingest_watermark
                    .map_or(Value::Null, subscription_position_value),
            ),
        ])
    }

    pub fn acknowledgement_error(
        &self,
        cursor: crate::events::subscription::Cursor,
        id: RequestId,
    ) -> Rejection {
        if cursor > self.last_delivered {
            rejection(ErrorClass::PolicyRefusal, id, "subscription.cursor_ahead")
        } else {
            owner_error(id, HumanOperationError::Refused)
        }
    }
}

fn subscription_binary_cursor(
    reader: &mut Reader<'_>,
) -> Option<crate::events::subscription::Cursor> {
    Some(crate::events::subscription::Cursor {
        block_height: reader.u64()?,
        tx_index: reader.u32()?,
        log_index: reader.u32()?,
    })
}

fn subscription_position_value(cursor: crate::events::subscription::Cursor) -> Value {
    serde_json::json!({
        "block_height": dec(cursor.block_height), "tx_index": dec(cursor.tx_index), "log_index": dec(cursor.log_index),
    })
}

fn subscription_payload(
    id: RequestId,
    payload: &[u8],
    decoder: impl FnOnce(&mut Reader<'_>) -> Option<Value>,
) -> Result<Value, Rejection> {
    let mut reader = Reader {
        bytes: payload,
        offset: 0,
    };
    decoder(&mut reader)
        .filter(|_| reader.finish().is_some())
        .ok_or_else(|| rejection(ErrorClass::InternalFault, id, "owner.response_malformed"))
}

fn subscription_decimal(reader: &mut Reader<'_>, id: RequestId) -> Option<u64> {
    crate::agent_rpc_dispatch::decimal_u64(&reader.text()?, id).ok()
}

fn subscription_flag(reader: &mut Reader<'_>) -> Option<bool> {
    match reader.u8()? {
        0 => Some(false),
        1 => Some(true),
        _ => None,
    }
}

fn subscription_optional_decimal(reader: &mut Reader<'_>, id: RequestId) -> Option<Value> {
    match reader.u8()? {
        0 => Some(Value::Null),
        1 => Some(dec(subscription_decimal(reader, id)?)),
        _ => None,
    }
}

fn subscription_scope(reader: &mut Reader<'_>) -> Option<Value> {
    let tenant = reader.text()?;
    let agent = reader.text()?;
    let capability = reader.text()?;
    Some(serde_json::json!({
        "tenant": tenant,
        "agent": agent,
        "capability": capability,
    }))
}

fn subscription_tenant_objects(reader: &mut Reader<'_>) -> Option<Value> {
    let count = reader.u16()?;
    let mut items = Vec::with_capacity(usize::from(count));
    for _ in 0..count {
        let tenant = reader.text()?;
        let value = reader.text()?;
        items.push(serde_json::json!({"tenant": tenant, "value": value}));
    }
    Some(Value::Array(items))
}

fn subscription_activity_types(reader: &mut Reader<'_>, id: RequestId) -> Option<Value> {
    let count = reader.u16()?;
    let mut items = Vec::with_capacity(usize::from(count));
    for _ in 0..count {
        items.push(dec(u16::try_from(subscription_decimal(reader, id)?).ok()?));
    }
    Some(Value::Array(items))
}

fn subscription_result_classes(reader: &mut Reader<'_>) -> Option<Value> {
    let count = reader.u16()?;
    let mut items = Vec::with_capacity(usize::from(count));
    for _ in 0..count {
        items.push(Value::from(i32::from_be_bytes(reader.fixed()?)));
    }
    Some(Value::Array(items))
}

fn subscription_cursor(reader: &mut Reader<'_>, id: RequestId) -> Option<Value> {
    let text = reader.text()?;
    let mut fields = text.strip_prefix('(')?.strip_suffix(')')?.split(", ");
    let block_height = crate::agent_rpc_dispatch::decimal_u64(fields.next()?, id).ok()?;
    let tx_index =
        u32::try_from(crate::agent_rpc_dispatch::decimal_u64(fields.next()?, id).ok()?).ok()?;
    let log_index =
        u32::try_from(crate::agent_rpc_dispatch::decimal_u64(fields.next()?, id).ok()?).ok()?;
    if fields.next().is_some() {
        return None;
    }
    Some(serde_json::json!({
        "block_height": dec(block_height),
        "tx_index": dec(tx_index),
        "log_index": dec(log_index),
    }))
}

fn subscription_record(reader: &mut Reader<'_>, id: RequestId) -> Option<Value> {
    let subscription_id = reader.text()?;
    let scope = subscription_scope(reader)?;
    let agents = subscription_tenant_objects(reader)?;
    let accounts = subscription_tenant_objects(reader)?;
    let modules = subscription_tenant_objects(reader)?;
    let assets = subscription_tenant_objects(reader)?;
    let counterparties = subscription_tenant_objects(reader)?;
    let activity_types = subscription_activity_types(reader, id)?;
    let result_classes = subscription_result_classes(reader)?;
    let start = subscription_cursor(reader, id)?;
    let last_acknowledged = subscription_cursor(reader, id)?;
    let delivery_target = reader.text()?;
    let paused = subscription_flag(reader)?;
    Some(serde_json::json!({
        "subscription_id": subscription_id,
        "scope": scope,
        "filter": {
            "agents": agents,
            "accounts": accounts,
            "modules": modules,
            "assets": assets,
            "counterparties": counterparties,
            "activity_types": activity_types,
            "result_classes": result_classes,
        },
        "start": start,
        "last_acknowledged": last_acknowledged,
        "delivery_target": delivery_target,
        "paused": paused,
    }))
}

fn subscription_record_value(id: RequestId, payload: &[u8]) -> Result<Value, Rejection> {
    subscription_payload(id, payload, |reader| subscription_record(reader, id))
}

fn subscription_list_value(id: RequestId, payload: &[u8]) -> Result<Value, Rejection> {
    subscription_payload(id, payload, |reader| {
        let count = reader.u32()?;
        let mut records = Vec::new();
        for _ in 0..count {
            records.push(subscription_record(reader, id)?);
        }
        Some(Value::Array(records))
    })
}

fn subscription_health_value(id: RequestId, payload: &[u8]) -> Result<Value, Rejection> {
    subscription_payload(id, payload, |reader| {
        let scope = subscription_scope(reader)?;
        let subscription_id = reader.text()?;
        let last_acknowledged = subscription_cursor(reader, id)?;
        let last_delivery_at = subscription_optional_decimal(reader, id)?;
        let pending_backfill = match reader.u8()? {
            0 => Value::Null,
            1 => {
                let missing_first = subscription_decimal(reader, id)?;
                let missing_last = subscription_decimal(reader, id)?;
                let backfill_cursor = subscription_decimal(reader, id)?;
                let backfill_attempted = subscription_flag(reader)?;
                serde_json::json!({
                    "missing_first": dec(missing_first),
                    "missing_last": dec(missing_last),
                    "backfill_cursor": dec(backfill_cursor),
                    "backfill_attempted": backfill_attempted,
                })
            }
            _ => return None,
        };
        Some(serde_json::json!({
            "target": {"scope": scope, "subscription_id": subscription_id},
            "last_acknowledged": last_acknowledged,
            "last_delivery_at": last_delivery_at,
            "pending_backfill": pending_backfill,
        }))
    })
}

fn subscription_null_value(id: RequestId, payload: &[u8]) -> Result<Value, Rejection> {
    subscription_payload(id, payload, |reader| {
        (reader.u8()? == 0).then_some(Value::Null)
    })
}

pub(crate) fn budget_list<A: HumanAuthorityBoundary>(
    owner: &SharedAgentOwner<A>,
    context: &crate::agent_rpc_peer::RpcOwnerContext<'_>,
    request: &serde_json::Map<String, serde_json::Value>,
    ctx: &DispatchContext,
) -> Result<Dispatched, Rejection> {
    let id = ctx.request_id;
    let wire: BudgetListRequestWire = decode(request, id)?;
    let principal = context.principal();
    if principal.tenant.as_str() != wire.tenant
        || principal.agent.as_bytes() != wire.agent_did.as_bytes()
    {
        return Err(rejection(
            ErrorClass::PolicyRefusal,
            id,
            "envelope.coordinate_mismatch",
        ));
    }
    let typed = layerx_agent_api::budget::BudgetList {
        tenant: layerx_agent_api::identity::TenantId::new(wire.tenant)
            .map_err(|_| malformed(id))?,
        agent_did: layerx_agent_api::identity::AgentDid::new(wire.agent_did)
            .map_err(|_| malformed(id))?,
    };
    let mut guard = owner.lock().map_err(|error| owner_error(id, error))?;
    let response = guard.budget_list(context.peer(), typed);
    let listed = owner_payload(id, response, |reader| {
        let count = reader.u16()?;
        let mut budgets = Vec::with_capacity(usize::from(count));
        for _ in 0..count {
            let budget_id: [u8; 32] = reader.fixed()?;
            let agent_id = reader.text()?;
            let mut budget = Map::new();
            budget.insert("budget_id".into(), hexv(&budget_id));
            budget.insert("agent_id".into(), Value::String(agent_id));
            budgets.push(Value::Object(budget));
        }
        let mut out = Map::new();
        out.insert("budgets".into(), Value::Array(budgets));
        Some((Value::Object(out), None))
    })?;
    let mut value = listed.value;
    let budgets = value
        .get_mut("budgets")
        .and_then(Value::as_array_mut)
        .ok_or_else(|| rejection(ErrorClass::InternalFault, id, "owner.response_malformed"))?;
    let mut committed = !budgets.is_empty();
    for budget in budgets {
        let budget_id = budget
            .get("budget_id")
            .and_then(Value::as_str)
            .ok_or_else(|| rejection(ErrorClass::InternalFault, id, "owner.response_malformed"))?;
        let target = layerx_agent_api::budget::BudgetTarget {
            tenant: layerx_agent_api::identity::TenantId::new(principal.tenant.as_str())
                .map_err(|_| malformed(id))?,
            agent_did: layerx_agent_api::identity::AgentDid::new(
                std::str::from_utf8(principal.agent.as_bytes()).map_err(|_| malformed(id))?,
            )
            .map_err(|_| malformed(id))?,
            budget_id: layerx_agent_api::budget::BudgetId::new(budget_id)
                .map_err(|_| malformed(id))?,
        };
        let control = guard.session_control.clone();
        let response = guard
            .budget_state(context, &control, target)
            .map_err(|error| owner_error(id, error))?;
        let (state, verification) = budget_state_operation(&response);
        committed &= verification == Some(VerificationStatus::Achieved(Rung::Committed));
        budget
            .as_object_mut()
            .ok_or_else(|| malformed(id))?
            .insert("state".into(), state);
    }
    Ok(Dispatched {
        value,
        verification: committed.then_some(VerificationStatus::Achieved(Rung::Committed)),
    })
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct BudgetListRequestWire {
    tenant: String,
    agent_did: String,
}

pub(crate) fn budget_reconciliation<A: HumanAuthorityBoundary>(
    owner: &SharedAgentOwner<A>,
    context: &RpcOwnerContext<'_>,
    request: &Map<String, Value>,
    ctx: &DispatchContext,
) -> Result<Dispatched, Rejection> {
    budget_state(owner, context, request, ctx)
}

const fn rejection(class: ErrorClass, request_id: RequestId, reason: &'static str) -> Rejection {
    Rejection {
        class,
        retriability: Retriability::Terminal,
        request_id,
        reason,
    }
}

fn malformed(request_id: RequestId) -> Rejection {
    rejection(
        ErrorClass::ProtocolIncompatibility,
        request_id,
        "envelope.malformed",
    )
}

fn decode<T: for<'de> Deserialize<'de>>(
    request: &Map<String, Value>,
    request_id: RequestId,
) -> Result<T, Rejection> {
    serde_json::from_value(Value::Object(request.clone())).map_err(|error| {
        if error.to_string().starts_with("unknown field") {
            rejection(
                ErrorClass::ProtocolIncompatibility,
                request_id,
                "envelope.unknown_field",
            )
        } else {
            malformed(request_id)
        }
    })
}

fn hex32(text: &str, request_id: RequestId) -> Result<[u8; 32], Rejection> {
    let bytes = text.as_bytes();
    if bytes.len() != 64
        || !bytes
            .iter()
            .all(|byte| matches!(byte, b'0'..=b'9' | b'a'..=b'f'))
    {
        return Err(malformed(request_id));
    }
    let mut out = [0_u8; 32];
    for (index, pair) in bytes.chunks_exact(2).enumerate() {
        let text = std::str::from_utf8(pair).map_err(|_| malformed(request_id))?;
        out[index] = u8::from_str_radix(text, 16).map_err(|_| malformed(request_id))?;
    }
    Ok(out)
}

pub fn capability_operation_error(request_id: RequestId, error: HumanOperationError) -> Rejection {
    match error {
        HumanOperationError::CapabilityRefused(_) => rejection(
            ErrorClass::CapabilityRefusal,
            request_id,
            "capability.exceeds_grant",
        ),
        other => owner_error(request_id, other),
    }
}

fn owner_error(request_id: RequestId, error: HumanOperationError) -> Rejection {
    match error {
        HumanOperationError::Refused => {
            rejection(ErrorClass::PolicyRefusal, request_id, "owner.refused")
        }
        HumanOperationError::CapabilityRefused(dimension) => rejection(
            ErrorClass::CapabilityRefusal,
            request_id,
            match dimension {
                crate::capability::Dimension::Expiry => "capability.expiry",
                crate::capability::Dimension::ActivityType => "capability.activity_type",
                crate::capability::Dimension::Counterparty => "capability.counterparty",
                crate::capability::Dimension::Asset => "capability.asset",
                crate::capability::Dimension::Amount => "capability.amount",
                crate::capability::Dimension::Rate => "capability.rate",
                crate::capability::Dimension::Purpose => "capability.purpose",
            },
        ),

        HumanOperationError::Typed(refusal) => {
            rejection(refusal.class(), request_id, refusal.reason())
        }
        HumanOperationError::Unavailable => Rejection {
            class: ErrorClass::UnavailableCapability,
            retriability: Retriability::Retriable,
            request_id,
            reason: "owner.unavailable",
        },
    }
}

fn level(value: u8) -> Option<Rung> {
    Some(match value {
        0 | 1 => Rung::Submitted,
        2 | 3 => Rung::Receipted,
        4 => Rung::Committed,
        _ => return None,
    })
}

fn hexv(bytes: &[u8]) -> Value {
    Value::String(bytes.iter().map(|byte| format!("{byte:02x}")).collect())
}

fn dec(value: impl ToString) -> Value {
    Value::String(value.to_string())
}

struct Reader<'a> {
    bytes: &'a [u8],
    offset: usize,
}

impl<'a> Reader<'a> {
    fn take(&mut self, length: usize) -> Option<&'a [u8]> {
        let end = self.offset.checked_add(length)?;
        let value = self.bytes.get(self.offset..end)?;
        self.offset = end;
        Some(value)
    }
    fn fixed<const N: usize>(&mut self) -> Option<[u8; N]> {
        self.take(N)?.try_into().ok()
    }
    fn u8(&mut self) -> Option<u8> {
        Some(self.take(1)?[0])
    }
    fn u64(&mut self) -> Option<u64> {
        Some(u64::from_be_bytes(self.fixed()?))
    }
    fn u128(&mut self) -> Option<u128> {
        Some(u128::from_be_bytes(self.fixed()?))
    }
    fn finish(&self) -> Option<()> {
        (self.offset == self.bytes.len()).then_some(())
    }
}

pub(crate) fn approval_list_native<A: HumanAuthorityBoundary>(
    owner: &SharedAgentOwner<A>,
    context: &RpcOwnerContext<'_>,
    request: &Map<String, Value>,
    ctx: &DispatchContext,
) -> Result<Dispatched, Rejection> {
    use crate::agent_rpc_dispatch::dispatched_native;
    use crate::agent_rpc_wire::{
        decode_wire, NativeApprovalListResultV1Wire, NativeApprovalListV1Wire,
    };
    let id = ctx.request_id;
    decode_wire::<NativeApprovalListV1Wire>(request, id)?.into_request(id)?;
    let response = owner
        .lock()
        .and_then(|mut guard| guard.rpc_approval_list_native(context));
    dispatched_native(id, response, NativeApprovalListResultV1Wire::into_result)
}

pub(crate) fn approval_get_native<A: HumanAuthorityBoundary>(
    owner: &SharedAgentOwner<A>,
    context: &RpcOwnerContext<'_>,
    request: &Map<String, Value>,
    ctx: &DispatchContext,
) -> Result<Dispatched, Rejection> {
    use crate::agent_rpc_dispatch::dispatched_native;
    use crate::agent_rpc_wire::{decode_wire, NativeApprovalGetV1Wire, NativeApprovalResultV1Wire};
    let id = ctx.request_id;
    let typed = decode_wire::<NativeApprovalGetV1Wire>(request, id)?.into_request(id)?;
    let response = owner
        .lock()
        .and_then(|mut guard| guard.rpc_approval_get_native(context, typed));
    dispatched_native(id, response, NativeApprovalResultV1Wire::into_result)
}

fn approval_decision_native<A: HumanAuthorityBoundary>(
    owner: &SharedAgentOwner<A>,
    context: &RpcOwnerContext<'_>,
    request: &Map<String, Value>,
    ctx: &DispatchContext,
    grant: bool,
) -> Result<Dispatched, Rejection> {
    use crate::agent_rpc_dispatch::{
        dispatched_native, malformed, mutation_key, native_approval_digest,
    };
    use crate::agent_rpc_wire::{
        decode_wire, NativeApprovalDecisionV1Wire, NativeApprovalResultV1Wire,
    };
    let id = ctx.request_id;
    let typed = decode_wire::<NativeApprovalDecisionV1Wire>(request, id)?.into_request(id)?;
    let envelope = crate::human::MutationEnvelope {
        request_id: id.0,
        key: mutation_key(ctx)?,
        body_digest: native_approval_digest(&typed, grant).map_err(|_| malformed(id))?,
        operation: typed,
    };
    let response = owner
        .lock()
        .and_then(|mut guard| guard.rpc_approval_decide_native(context, envelope, grant));
    dispatched_native(id, response, NativeApprovalResultV1Wire::into_result)
}

pub(crate) fn approval_approve<A: HumanAuthorityBoundary>(
    owner: &SharedAgentOwner<A>,
    context: &crate::agent_rpc_peer::RpcOwnerContext<'_>,
    request: &serde_json::Map<String, serde_json::Value>,
    ctx: &DispatchContext,
) -> Result<Dispatched, Rejection> {
    approval_decision(owner, context, request, ctx, true)
}

pub(crate) fn approval_reject<A: HumanAuthorityBoundary>(
    owner: &SharedAgentOwner<A>,
    context: &crate::agent_rpc_peer::RpcOwnerContext<'_>,
    request: &serde_json::Map<String, serde_json::Value>,
    ctx: &DispatchContext,
) -> Result<Dispatched, Rejection> {
    approval_decision(owner, context, request, ctx, false)
}

fn approval_decision<A: HumanAuthorityBoundary>(
    owner: &SharedAgentOwner<A>,
    context: &crate::agent_rpc_peer::RpcOwnerContext<'_>,
    request: &serde_json::Map<String, serde_json::Value>,
    ctx: &DispatchContext,
    approve: bool,
) -> Result<Dispatched, Rejection> {
    use crate::agent_rpc_dispatch::{
        decimal_u64, decode_decision, dispatched, lower_hex, mutation_key, ApprovalDecisionRequest,
    };
    let id = ctx.request_id;
    if crate::agent_rpc_dispatch::native_variant(request, id)? {
        return approval_decision_native(owner, context, request, ctx, approve);
    }
    let request: ApprovalDecisionRequest = decode(request, id)?;
    let _ = (request.tenant, request.agent);
    let approval_id = hex32(&request.approval_id, id)?;
    let held_digest = hex32(&request.held_digest, id)?;
    let current_sequence = decimal_u64(&request.current_sequence, id)?;
    let key = lower_hex(&mutation_key(ctx)?);
    let response = owner.lock().and_then(|mut guard| {
        if approve {
            guard.rpc_approval_approve(context, approval_id, held_digest, &key, current_sequence)
        } else {
            guard.rpc_approval_reject(context, approval_id, held_digest, &key, current_sequence)
        }
    });
    dispatched(id, response, decode_decision)
}

pub(crate) fn prepare<A: HumanAuthorityBoundary>(
    owner: &SharedAgentOwner<A>,
    context: &crate::agent_rpc_peer::RpcOwnerContext<'_>,
    request: &serde_json::Map<String, serde_json::Value>,
    ctx: &DispatchContext,
) -> Result<Dispatched, Rejection> {
    use crate::agent_rpc_dispatch::{decode_preparation, dispatched, human_prepare, mutation_key};
    let id = ctx.request_id;
    if request.get("variant").and_then(serde_json::Value::as_str) == Some("native_disclosure_v1") {
        use crate::agent_rpc_wire::{decode_wire, Canonical, NativeDisclosureV1Wire};
        let typed = decode_wire::<NativeDisclosureV1Wire>(request, id)?.into_request(id)?;
        let value = owner
            .lock()
            .and_then(|mut guard| guard.rpc_native_disclosure(context, typed))
            .map_err(|error| crate::agent_rpc_dispatch::owner_error(id, error))?;
        return Ok(Dispatched {
            value: value.canonical(),
            verification: None,
        });
    }
    if crate::agent_rpc_dispatch::native_effect_variant(request) {
        use crate::agent_rpc_dispatch::{
            dispatched_native, malformed, native_effect_prepare_digest,
        };
        use crate::agent_rpc_wire::{
            decode_wire, NativeEffectPrepareV1Wire, NativePrepareResultV1Wire,
        };
        let typed = decode_wire::<NativeEffectPrepareV1Wire>(request, id)?.into_request(id)?;
        let envelope = crate::human::MutationEnvelope {
            request_id: id.0,
            key: mutation_key(ctx)?,
            body_digest: native_effect_prepare_digest(&typed).map_err(|_| malformed(id))?,
            operation: typed,
        };
        let response = owner
            .lock()
            .and_then(|mut guard| guard.rpc_prepare_native_effect(context, envelope, false));
        return dispatched_native(id, response, NativePrepareResultV1Wire::into_result);
    }
    if crate::agent_rpc_dispatch::native_registration_variant(request) {
        use crate::agent_rpc_dispatch::{
            dispatched_native, malformed, native_registration_prepare_digest,
        };
        use crate::agent_rpc_wire::{
            decode_wire, NativeEffectPrepareV1Wire, NativePrepareResultV1Wire,
        };
        let typed = decode_wire::<NativeEffectPrepareV1Wire>(request, id)?.into_registration(id)?;
        let envelope = crate::human::MutationEnvelope {
            request_id: id.0,
            key: mutation_key(ctx)?,
            body_digest: native_registration_prepare_digest(&typed).map_err(|_| malformed(id))?,
            operation: typed.request,
        };
        let response = owner
            .lock()
            .and_then(|mut guard| guard.rpc_prepare_native_effect(context, envelope, true));
        return dispatched_native(id, response, NativePrepareResultV1Wire::into_result);
    }
    if crate::agent_rpc_dispatch::native_variant(request, id)? {
        use crate::agent_rpc_dispatch::{dispatched_native, malformed, native_prepare_digest};
        use crate::agent_rpc_wire::{decode_wire, NativePrepareResultV1Wire, NativePrepareV1Wire};
        let typed = decode_wire::<NativePrepareV1Wire>(request, id)?.into_request(id)?;
        let envelope = crate::human::MutationEnvelope {
            request_id: id.0,
            key: mutation_key(ctx)?,
            body_digest: native_prepare_digest(&typed).map_err(|_| malformed(id))?,
            operation: typed,
        };
        let response = owner
            .lock()
            .and_then(|mut guard| guard.rpc_prepare_native(context, envelope));
        return dispatched_native(id, response, NativePrepareResultV1Wire::into_result);
    }
    let typed = human_prepare(decode(request, id)?, id)?;
    let envelope = crate::human::MutationEnvelope {
        request_id: id.0,
        key: mutation_key(ctx)?,
        body_digest: crate::capability::binding::prepare_body_digest(
            crate::human_runtime::prepare_digest(&typed),
            typed.capability_id.as_ref(),
        ),
        operation: typed,
    };
    let response = owner
        .lock()
        .and_then(|mut guard| guard.rpc_prepare(context, envelope));
    dispatched(id, response, decode_preparation)
}

pub(crate) fn submit<A: HumanAuthorityBoundary>(
    owner: &SharedAgentOwner<A>,
    context: &crate::agent_rpc_peer::RpcOwnerContext<'_>,
    request: &serde_json::Map<String, serde_json::Value>,
    ctx: &DispatchContext,
) -> Result<Dispatched, Rejection> {
    use crate::agent_rpc_dispatch::{decode_submitted, dispatched, human_submit, mutation_key};
    let id = ctx.request_id;
    let typed = human_submit(decode(request, id)?, id)?;
    let envelope = crate::human::MutationEnvelope {
        request_id: id.0,
        key: mutation_key(ctx)?,
        body_digest: crate::human_runtime::submit_digest(&typed),
        operation: typed,
    };
    let response = owner
        .lock()
        .and_then(|mut guard| guard.rpc_submit_external(context, envelope));
    dispatched(id, response, decode_submitted)
}

pub(crate) fn session_refresh<A: HumanAuthorityBoundary>(
    owner: &SharedAgentOwner<A>,
    context: &RpcOwnerContext<'_>,
    request: &Map<String, Value>,
    ctx: &DispatchContext,
) -> Result<Dispatched, Rejection> {
    let id = ctx.request_id;
    let request: SessionTargetWire = decode(request, id)?;
    let typed = layerx_agent_api::identity::SessionRefresh {
        session_id: layerx_agent_api::identity::SessionId::new(request.session_id)
            .map_err(|_| malformed(id))?,
        context: session_context(request.context, context, id)?,
    };
    let mut guard = owner.lock().map_err(|error| owner_error(id, error))?;
    let response = guard.rpc_session_refresh(context, typed);
    owner_payload(id, response, |reader| {
        let tenant = reader.text()?;
        let session_id: [u8; 32] = reader.fixed()?;
        let token_id: [u8; 32] = reader.fixed()?;
        let generation = reader.u64()?;
        let expires_at = reader.u64()?;
        let (session, open, record_session_id) = decode_session_record(reader)?;
        let record = session.as_object()?;
        if !open
            || record_session_id != session_id
            || record.get("token_id")? != &hexv(&token_id)
            || record.get("generation")? != &dec(generation)
        {
            return None;
        }
        let mut credential = Map::new();
        credential.insert("tenant".into(), Value::String(tenant));
        credential.insert("session_id".into(), hexv(&session_id));
        credential.insert("token_id".into(), hexv(&token_id));
        credential.insert("generation".into(), dec(generation));
        credential.insert("expires_at".into(), dec(expires_at));
        let mut out = Map::new();
        out.insert("credential".into(), Value::Object(credential));
        out.insert("session".into(), session);
        Some((Value::Object(out), None))
    })
}

pub(crate) fn session_close<A: HumanAuthorityBoundary>(
    owner: &SharedAgentOwner<A>,
    context: &RpcOwnerContext<'_>,
    request: &Map<String, Value>,
    ctx: &DispatchContext,
) -> Result<Dispatched, Rejection> {
    let id = ctx.request_id;
    let request: SessionTargetWire = decode(request, id)?;
    let typed = layerx_agent_api::identity::SessionClose {
        session_id: layerx_agent_api::identity::SessionId::new(request.session_id)
            .map_err(|_| malformed(id))?,
        context: session_context(request.context, context, id)?,
    };
    let mut guard = owner.lock().map_err(|error| owner_error(id, error))?;
    let response = guard.rpc_session_close(context, typed);
    owner_payload(id, response, |reader| {
        let record = decode_session_record(reader)?;
        (!record.1).then_some((record.0, None))
    })
}

pub(crate) fn session_list<A: HumanAuthorityBoundary>(
    owner: &SharedAgentOwner<A>,
    context: &RpcOwnerContext<'_>,
    request: &Map<String, Value>,
    ctx: &DispatchContext,
) -> Result<Dispatched, Rejection> {
    let id = ctx.request_id;
    let request: SessionListWire = decode(request, id)?;
    let typed =
        layerx_agent_api::identity::SessionList(session_context(request.context, context, id)?);
    let mut guard = owner.lock().map_err(|error| owner_error(id, error))?;
    let response = guard.session_list(context.peer(), typed);
    owner_payload(id, response, |reader| {
        let count = reader.u16()?;
        let mut sessions = Vec::with_capacity(usize::from(count));
        let mut previous: Option<[u8; 32]> = None;
        for _ in 0..count {
            let (record, _, session_id) = decode_session_record(reader)?;
            if previous.is_some_and(|last| last >= session_id) {
                return None;
            }
            previous = Some(session_id);
            sessions.push(record);
        }
        let mut out = Map::new();
        out.insert("sessions".into(), Value::Array(sessions));
        Some((Value::Object(out), None))
    })
}

pub(crate) fn wait<A: HumanAuthorityBoundary>(
    owner: &SharedAgentOwner<A>,
    context: &RpcOwnerContext<'_>,
    request: &Map<String, Value>,
    ctx: &DispatchContext,
) -> Result<Dispatched, Rejection> {
    let id = ctx.request_id;
    let typed = wait_request(request, id)?;
    use crate::agent_rpc_dispatch::{submission_key, wait_write_answer};
    let hash = submission_key(typed.submission_ref.as_str(), id)?;
    let started = std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .ok()
        .and_then(|time| u64::try_from(time.as_millis()).ok())
        .ok_or_else(|| malformed(id))?;
    let deadline = typed
        .deadline
        .get()
        .checked_mul(1000)
        .ok_or_else(|| malformed(id))?
        .max(started);
    let mut progress = OwnerWriteProgress {
        owner: owner.clone(),
        peer: context.peer().clone(),
        reference: typed.submission_ref.as_str().to_owned(),
        request_id: id,
        started: std::time::Instant::now(),
        first_observation_ms: started,
        submission: None,
    };
    let result = crate::finality::wait_for_rung(
        &mut progress,
        hash,
        typed.requested_rung,
        started,
        deadline,
        100,
    )
    .map_err(|_| {
        rejection(
            ErrorClass::VerificationFailure,
            id,
            "wait.progress_unavailable",
        )
    })?;
    let submission = progress.submission.ok_or_else(|| malformed(id))?;
    let (value, verification) = wait_write_answer(submission, result, id)?;
    Ok(Dispatched {
        value,
        verification: Some(verification),
    })
}

fn wait_request(
    request: &Map<String, Value>,
    id: RequestId,
) -> Result<layerx_agent_api::track::WaitRequest, Rejection> {
    let request: WaitWire = decode(request, id)?;
    let _ = (request.tenant, request.agent);
    Ok(layerx_agent_api::track::WaitRequest {
        submission_ref: layerx_agent_api::track::SubmissionRef::new(request.submission_ref)
            .map_err(|_| malformed(id))?,
        requested_rung: Rung::from_name(&request.requested_rung).ok_or_else(|| malformed(id))?,
        deadline: layerx_agent_api::generated::TimestampSeconds::parse_decimal(&request.deadline)
            .ok()
            .filter(|value| value.get().to_string() == request.deadline)
            .ok_or_else(|| malformed(id))?,
    })
}

pub(crate) fn read_module_state<A: HumanAuthorityBoundary>(
    owner: &SharedAgentOwner<A>,
    context: &RpcOwnerContext<'_>,
    request: &Map<String, Value>,
    ctx: &DispatchContext,
) -> Result<Dispatched, Rejection> {
    let id = ctx.request_id;
    let request: ModuleStateWire = decode(request, id)?;
    let _ = (request.tenant, request.agent);
    let requested = requested_level(&request.requested_rung, id)?;
    let typed = layerx_agent_api::read::ReadRequest {
        selector: layerx_agent_api::read::ModuleStateSelector {
            module: layerx_agent_api::read::ModuleRef::new(request.module)
                .map_err(|_| malformed(id))?,
            key: layerx_agent_api::prepare::CanonicalBytes::new(lower_hex_bytes(&request.key, id)?)
                .map_err(|_| malformed(id))?,
        },
        requested_rung: requested,
    };
    let mut guard = owner.lock().map_err(|error| owner_error(id, error))?;
    let response = guard.read_module_state(context.peer(), typed);
    read_response(id, response)
}

pub(crate) fn read_history<A: HumanAuthorityBoundary>(
    owner: &SharedAgentOwner<A>,
    context: &RpcOwnerContext<'_>,
    request: &Map<String, Value>,
    ctx: &DispatchContext,
) -> Result<Dispatched, Rejection> {
    let id = ctx.request_id;
    let request: HistoryWire = decode(request, id)?;
    let _ = (request.tenant, request.agent);
    let requested = requested_level(&request.requested_rung, id)?;
    let selector = HistorySelector {
        from_height: canonical_sequence(&request.range.from_height, id)?.0,
        to_height: canonical_sequence(&request.range.to_height, id)?.0,
        cursor: request
            .cursor
            .map(|position| {
                Ok(layerx_agent_api::read::HistoryCursor {
                    block_height: canonical_sequence(&position.block_height, id)?.0,
                    tx_index: canonical_sequence(&position.tx_index, id)?.0,
                    log_index: canonical_sequence(&position.log_index, id)?.0,
                })
            })
            .transpose()?,
        page_limit: request
            .page_limit
            .parse::<u32>()
            .ok()
            .filter(|value| value.to_string() == request.page_limit)
            .ok_or_else(|| malformed(id))?,
    }
    .validate()
    .map_err(|_| malformed(id))?;
    let typed = layerx_agent_api::read::ReadRequest {
        selector,
        requested_rung: requested,
    };
    let response = owner
        .lock()
        .and_then(|mut guard| guard.read_history(context.peer(), typed));
    read_response(id, response)
}

pub(crate) fn availability_fetch<A: HumanAuthorityBoundary>(
    owner: &SharedAgentOwner<A>,
    context: &RpcOwnerContext<'_>,
    request: &Map<String, Value>,
    ctx: &DispatchContext,
) -> Result<Dispatched, Rejection> {
    let id = ctx.request_id;
    let request: AvailabilityWire = decode(request, id)?;
    let _ = (request.tenant, request.agent);
    let requested = requested_level(&request.requested_rung, id)?;
    let typed = layerx_agent_api::availability::AvailabilityRequest {
        selector: request.selector,
        requested_rung: requested,
        maximum_bytes: request
            .maximum_bytes
            .parse::<u64>()
            .ok()
            .filter(|value| value.to_string() == request.maximum_bytes)
            .ok_or_else(|| malformed(id))?,
        maximum_chunks: request
            .maximum_chunks
            .parse::<u32>()
            .ok()
            .filter(|value| value.to_string() == request.maximum_chunks)
            .ok_or_else(|| malformed(id))?,
        deadline: layerx_agent_api::generated::TimestampSeconds::parse_decimal(&request.deadline)
            .ok()
            .filter(|value| value.get().to_string() == request.deadline)
            .ok_or_else(|| malformed(id))?,
    }
    .validate()
    .map_err(|_| malformed(id))?;
    let mut guard = owner.lock().map_err(|error| owner_error(id, error))?;
    let response = guard.availability_fetch(context.peer(), typed);
    read_response(id, response)
}

pub(crate) fn capability_create<A: HumanAuthorityBoundary>(
    owner: &SharedAgentOwner<A>,
    context: &RpcOwnerContext<'_>,
    request: &Map<String, Value>,
    ctx: &DispatchContext,
) -> Result<Dispatched, Rejection> {
    use crate::agent_rpc_dispatch::mutation_key;
    let id = ctx.request_id;
    let wire: crate::agent_rpc_wire::CapabilityCreateWire = decode(request, id)?;
    let typed = wire.into_request(id)?;
    let envelope = crate::human::MutationEnvelope {
        request_id: id.0,
        key: mutation_key(ctx)?,
        body_digest: crate::human_runtime::capability_create_digest(&typed),
        operation: typed,
    };
    let response = owner
        .lock()
        .and_then(|mut guard| guard.rpc_capability_create(context, envelope))
        .map_err(|error| capability_operation_error(id, error))?;
    Ok(Dispatched {
        value: capability_record_value(id, response.bytes())?,
        verification: None,
    })
}

pub(crate) fn capability_attenuate<A: HumanAuthorityBoundary>(
    owner: &SharedAgentOwner<A>,
    context: &RpcOwnerContext<'_>,
    request: &Map<String, Value>,
    ctx: &DispatchContext,
) -> Result<Dispatched, Rejection> {
    use crate::agent_rpc_dispatch::mutation_key;
    let id = ctx.request_id;
    let wire: crate::agent_rpc_wire::CapabilityAttenuateWire = decode(request, id)?;
    let typed = wire.into_request(id)?;
    let envelope = crate::human::MutationEnvelope {
        request_id: id.0,
        key: mutation_key(ctx)?,
        body_digest: crate::human_runtime::capability_attenuate_digest(&typed),
        operation: typed,
    };
    let response = owner
        .lock()
        .and_then(|mut guard| guard.rpc_capability_attenuate(context, envelope))
        .map_err(|error| capability_operation_error(id, error))?;
    Ok(Dispatched {
        value: capability_record_value(id, response.bytes())?,
        verification: None,
    })
}

pub(crate) fn capability_list<A: HumanAuthorityBoundary>(
    owner: &SharedAgentOwner<A>,
    context: &RpcOwnerContext<'_>,
    request: &Map<String, Value>,
    ctx: &DispatchContext,
) -> Result<Dispatched, Rejection> {
    let id = ctx.request_id;
    let wire: crate::agent_rpc_wire::CapabilityListWire = decode(request, id)?;
    let typed = wire.into_request(id)?;
    let response = owner
        .lock()
        .and_then(|mut guard| guard.rpc_capability_list(context, typed))
        .map_err(|error| owner_error(id, error))?;
    Ok(Dispatched {
        value: capability_records_value(id, response.bytes())?,
        verification: None,
    })
}

pub(crate) fn capability_revoke<A: HumanAuthorityBoundary>(
    owner: &SharedAgentOwner<A>,
    context: &RpcOwnerContext<'_>,
    request: &Map<String, Value>,
    ctx: &DispatchContext,
) -> Result<Dispatched, Rejection> {
    use crate::agent_rpc_dispatch::mutation_key;
    let id = ctx.request_id;
    let wire: crate::agent_rpc_wire::CapabilityRevokeWire = decode(request, id)?;
    let typed = wire.into_request(id)?;
    let envelope = crate::human::MutationEnvelope {
        request_id: id.0,
        key: mutation_key(ctx)?,
        body_digest: crate::human_runtime::capability_revoke_digest(&typed),
        operation: typed,
    };
    let response = owner
        .lock()
        .and_then(|mut guard| guard.rpc_capability_revoke(context, envelope))
        .map_err(|error| owner_error(id, error))?;
    Ok(Dispatched {
        value: capability_record_value(id, response.bytes())?,
        verification: None,
    })
}

/// Owner capability payloads are bounded by `HumanResponse::new` (human.rs `MAX_BYTES`,
/// 1 MiB); every count is a u16 and every element is read from the remaining bytes, so a
/// record list holds at most 65 535 records and never more than the payload encodes.
fn capability_payload(
    id: RequestId,
    payload: &[u8],
    decoder: impl FnOnce(&mut Reader<'_>) -> Option<Value>,
) -> Result<Value, Rejection> {
    let mut reader = Reader {
        bytes: payload,
        offset: 0,
    };
    decoder(&mut reader)
        .filter(|_| reader.finish().is_some())
        .ok_or_else(|| rejection(ErrorClass::InternalFault, id, "owner.response_malformed"))
}

/// Authority block: text tenant, text agent_did, text authority_ref, non-empty bytes
/// protocol_authority. Returns the JSON authority and its tenant and agent_did.
fn capability_authority(reader: &mut Reader<'_>) -> Option<(Value, String, String)> {
    let tenant = reader.text()?;
    let agent_did = reader.text()?;
    let authority_ref = reader.text()?;
    let protocol_authority = reader.bytes()?;
    let mut out = Map::new();
    out.insert("tenant".into(), Value::String(tenant.clone()));
    out.insert("agent_did".into(), Value::String(agent_did.clone()));
    out.insert("authority_ref".into(), Value::String(authority_ref));
    out.insert("protocol_authority".into(), hexv(protocol_authority));
    Some((Value::Object(out), tenant, agent_did))
}

fn capability_hex_id(reader: &mut Reader<'_>, id: RequestId) -> Option<(String, [u8; 32])> {
    let text = reader.text()?;
    let bytes = hex32(&text, id).ok()?;
    Some((text, bytes))
}

fn capability_unique<T: Ord + Clone>(values: &[T]) -> Option<()> {
    let set: std::collections::BTreeSet<T> = values.iter().cloned().collect();
    (set.len() == values.len()).then_some(())
}

fn capability_texts(reader: &mut Reader<'_>) -> Option<Vec<String>> {
    let count = reader.u16()?;
    let mut items = Vec::new();
    for _ in 0..count {
        items.push(reader.text()?);
    }
    capability_unique(&items)?;
    Some(items)
}

fn capability_strings(items: &[String]) -> Value {
    Value::Array(
        items
            .iter()
            .map(|item| Value::String(item.clone()))
            .collect(),
    )
}

/// One record: text capability_id (64 lowercase hex), u8 parent tag + text parent_id,
/// u16 count + u16 activity types, u16 count + text counterparties, u16 count + text assets,
/// u16 count + (text asset, u128 amount), u16 count + (u64 window_seconds,
/// u64 maximum_actions), u16 count + text purposes, u64 expiry_seconds, u64 created_at_ms,
/// u64 created_at_sequence, u8 state (0 active, 1 revoked, 2 expired), u8 revoked tag +
/// (u64 revoked_at_ms, u64 revoked_at_sequence). Duplicates, a zero window, a ceiling for
/// an asset outside `assets`, and a state that disagrees with the revoked tag are malformed.
fn capability_record(
    reader: &mut Reader<'_>,
    id: RequestId,
    tenant: &str,
    agent_did: &str,
) -> Option<(Value, [u8; 32])> {
    let (capability_id, capability_bytes) = capability_hex_id(reader, id)?;
    let parent_id = match reader.u8()? {
        0 => Value::Null,
        1 => Value::String(capability_hex_id(reader, id)?.0),
        _ => return None,
    };
    let activity_count = reader.u16()?;
    let mut activity_types = Vec::new();
    for _ in 0..activity_count {
        activity_types.push(reader.u16()?);
    }
    capability_unique(&activity_types)?;
    let counterparties = capability_texts(reader)?;
    let assets = capability_texts(reader)?;
    let amount_count = reader.u16()?;
    let mut amount_assets = Vec::new();
    let mut amount_ceilings = Vec::new();
    for _ in 0..amount_count {
        let asset = reader.text()?;
        let amount = reader.u128()?;
        if !assets.contains(&asset) {
            return None;
        }
        amount_assets.push(asset.clone());
        let mut ceiling = Map::new();
        ceiling.insert("asset".into(), Value::String(asset));
        ceiling.insert("amount".into(), dec(amount));
        amount_ceilings.push(Value::Object(ceiling));
    }
    capability_unique(&amount_assets)?;
    let rate_count = reader.u16()?;
    let mut windows = Vec::new();
    let mut rate_ceilings = Vec::new();
    for _ in 0..rate_count {
        let window_seconds = reader.u64()?;
        let maximum_actions = reader.u64()?;
        if window_seconds == 0 {
            return None;
        }
        windows.push(window_seconds);
        let mut ceiling = Map::new();
        ceiling.insert("window_seconds".into(), dec(window_seconds));
        ceiling.insert("maximum_actions".into(), dec(maximum_actions));
        rate_ceilings.push(Value::Object(ceiling));
    }
    capability_unique(&windows)?;
    let purposes = capability_texts(reader)?;
    let expiry = reader.u64()?;
    let created_at_ms = reader.u64()?;
    let created_at_sequence = reader.u64()?;
    let state = match reader.u8()? {
        0 => "active",
        1 => "revoked",
        2 => "expired",
        _ => return None,
    };
    let (revoked_at_ms, revoked_at_sequence) = match reader.u8()? {
        0 => (Value::Null, Value::Null),
        1 => (dec(reader.u64()?), dec(reader.u64()?)),
        _ => return None,
    };
    if (state == "revoked") == revoked_at_ms.is_null() {
        return None;
    }
    let mut dimensions = Map::new();
    dimensions.insert(
        "activity_types".into(),
        Value::Array(activity_types.into_iter().map(dec).collect()),
    );
    dimensions.insert("counterparties".into(), capability_strings(&counterparties));
    dimensions.insert("assets".into(), capability_strings(&assets));
    dimensions.insert("amount_ceilings".into(), Value::Array(amount_ceilings));
    dimensions.insert("rate_ceilings".into(), Value::Array(rate_ceilings));
    dimensions.insert("purpose_constraints".into(), capability_strings(&purposes));
    dimensions.insert("expiry".into(), dec(expiry));
    let mut out = Map::new();
    out.insert("capability_id".into(), Value::String(capability_id));
    out.insert("parent_id".into(), parent_id);
    out.insert("tenant".into(), Value::String(tenant.to_owned()));
    out.insert("agent_did".into(), Value::String(agent_did.to_owned()));
    out.insert("dimensions".into(), Value::Object(dimensions));
    out.insert("state".into(), Value::String(state.into()));
    out.insert("created_at_ms".into(), dec(created_at_ms));
    out.insert("created_at_sequence".into(), dec(created_at_sequence));
    out.insert("revoked_at_ms".into(), revoked_at_ms);
    out.insert("revoked_at_sequence".into(), revoked_at_sequence);
    Some((Value::Object(out), capability_bytes))
}

/// `AuthorityResponse<CapabilityRecord>`: authority block, then one record.
fn capability_record_value(id: RequestId, payload: &[u8]) -> Result<Value, Rejection> {
    capability_payload(id, payload, |reader| {
        let (authority, tenant, agent_did) = capability_authority(reader)?;
        let (record, _) = capability_record(reader, id, &tenant, &agent_did)?;
        let mut out = Map::new();
        out.insert("authority".into(), authority);
        out.insert("value".into(), record);
        Some(Value::Object(out))
    })
}

/// `AuthorityResponse<CapabilityRecords>`: authority block, then u16 count + records in
/// strictly ascending capability id order.
fn capability_records_value(id: RequestId, payload: &[u8]) -> Result<Value, Rejection> {
    capability_payload(id, payload, |reader| {
        let (authority, tenant, agent_did) = capability_authority(reader)?;
        let count = reader.u16()?;
        let mut records = Vec::new();
        let mut previous: Option<[u8; 32]> = None;
        for _ in 0..count {
            let (record, capability_id) = capability_record(reader, id, &tenant, &agent_did)?;
            if previous.is_some_and(|last| last >= capability_id) {
                return None;
            }
            previous = Some(capability_id);
            records.push(record);
        }
        let mut value = Map::new();
        value.insert("capabilities".into(), Value::Array(records));
        let mut out = Map::new();
        out.insert("authority".into(), authority);
        out.insert("value".into(), Value::Object(value));
        Some(Value::Object(out))
    })
}

/// Body digest of a budget mutation: a per-operation domain over the canonical JSON of the
/// converted request, which binds every signed-authorization carrier field when present.
fn budget_body_digest<T: Canonical>(domain: &[u8], typed: &T) -> [u8; 32] {
    let mut digest = Sha256::new();
    digest.update(domain);
    digest.update(typed.canonical().to_string().as_bytes());
    digest.finalize().into()
}

/// Body digest of a budget create: [`budget_body_digest`] under the create domain, followed by
/// the optional `TextV1` purpose label suffix, which is empty when the label is absent.
pub(crate) fn budget_create_body_digest<T: Canonical>(
    typed: &T,
    purpose: Option<&str>,
) -> [u8; 32] {
    let mut digest = Sha256::new();
    digest.update(b"LayerX/budget/create-body/v1\0");
    digest.update(typed.canonical().to_string().as_bytes());
    digest.update(crate::agent_rpc_wire::budget_create_purpose_suffix(purpose));
    digest.finalize().into()
}

pub(crate) fn budget_create<A: HumanAuthorityBoundary>(
    owner: &SharedAgentOwner<A>,
    context: &RpcOwnerContext<'_>,
    request: &Map<String, Value>,
    ctx: &DispatchContext,
) -> Result<Dispatched, Rejection> {
    use crate::agent_rpc_dispatch::mutation_key;
    use crate::agent_rpc_wire::{decode_wire, BudgetCreateWire};
    let id = ctx.request_id;
    let wire = decode_wire::<BudgetCreateWire>(request, id)?;
    let purpose = wire.purpose().map(str::to_owned);
    let typed = wire.into_request(id)?;
    let envelope = crate::human::MutationEnvelope {
        request_id: id.0,
        key: mutation_key(ctx)?,
        body_digest: budget_create_body_digest(&typed, purpose.as_deref()),
        operation: typed,
    };
    let response = owner
        .lock()
        .and_then(|mut guard| {
            let control = guard.session_control.clone();
            match purpose.as_deref() {
                None => guard.budget_create(context, &control, envelope),
                Some(purpose) => {
                    guard.budget_create_with_purpose(context, &control, envelope, purpose)
                }
            }
        })
        .map_err(|error| owner_error(id, error))?;
    Ok(budget_record_value(&response))
}

pub(crate) fn budget_fund<A: HumanAuthorityBoundary>(
    owner: &SharedAgentOwner<A>,
    context: &RpcOwnerContext<'_>,
    request: &Map<String, Value>,
    ctx: &DispatchContext,
) -> Result<Dispatched, Rejection> {
    use crate::agent_rpc_dispatch::mutation_key;
    use crate::agent_rpc_wire::{decode_wire, BudgetFundWire};
    let id = ctx.request_id;
    let typed = decode_wire::<BudgetFundWire>(request, id)?.into_request(id)?;
    let envelope = crate::human::MutationEnvelope {
        request_id: id.0,
        key: mutation_key(ctx)?,
        body_digest: budget_body_digest(b"LayerX/budget/fund-body/v1\0", &typed),
        operation: typed,
    };
    let response = owner
        .lock()
        .and_then(|mut guard| {
            let control = guard.session_control.clone();
            guard.budget_fund(context, &control, envelope)
        })
        .map_err(|error| owner_error(id, error))?;
    Ok(budget_record_value(&response))
}

pub(crate) fn budget_revoke<A: HumanAuthorityBoundary>(
    owner: &SharedAgentOwner<A>,
    context: &RpcOwnerContext<'_>,
    request: &Map<String, Value>,
    ctx: &DispatchContext,
) -> Result<Dispatched, Rejection> {
    use crate::agent_rpc_dispatch::mutation_key;
    use crate::agent_rpc_wire::{decode_wire, BudgetTargetWire};
    let id = ctx.request_id;
    let typed = decode_wire::<BudgetTargetWire>(request, id)?.into_request(id)?;
    let envelope = crate::human::MutationEnvelope {
        request_id: id.0,
        key: mutation_key(ctx)?,
        body_digest: budget_body_digest(b"LayerX/budget/revoke-body/v1\0", &typed),
        operation: typed,
    };
    let response = owner
        .lock()
        .and_then(|mut guard| {
            let control = guard.session_control.clone();
            guard.budget_revoke(context, &control, envelope)
        })
        .map_err(|error| owner_error(id, error))?;
    Ok(budget_record_value(&response))
}

/// `AuthorityResponse<BudgetRecord>`: a protocol budget reports the level its state was proven
/// at; a daemon limit reports no level and always carries its bypass notice.
fn budget_record_value(response: &AuthorityResponse<crate::human::BudgetState>) -> Dispatched {
    let state = &response.value;
    let (value, verification) =
        budget_record_wire(&state.record, state.proven_head, state.activity_id.as_ref());
    Dispatched {
        value: serde_json::json!({
            "authority": authority_wire(&response.authority),
            "value": value,
        }),
        verification,
    }
}

fn authority_wire(authority: &layerx_agent_api::budget::AuthorityDescription) -> Value {
    serde_json::json!({
        "tenant": authority.tenant.as_str(),
        "agent_did": authority.agent_did.as_str(),
        "authority_ref": authority.authority_ref.as_str(),
        "protocol_authority": hexv(&authority.protocol_authority),
    })
}

/// The one `BudgetRecord` wire encoding and the verification it states.
fn budget_record_wire(
    record: &BudgetRecord,
    head: u64,
    activity_id: Option<&[u8; 32]>,
) -> (Value, Option<VerificationStatus>) {
    let level = crate::agent_rpc_dispatch::rung_name;
    let (mut value, verification) = match record {
        BudgetRecord::Protocol(view) => (
            serde_json::json!({
                "enforcement": "ProtocolBudget",
                "budget_id": hexv(&view.budget_id),
                "owner": hexv(&view.owner),
                "budget_account": hexv(&view.budget_account),
                "asset_id": hexv(&view.asset_id),
                "purpose_hash": hexv(&view.purpose_hash),
                "per_period_limit": dec(view.per_period_limit.0),
                "configured_period_limit": dec(view.configured_period_limit.0),
                "carry_cap": dec(view.carry_cap.0),
                "spent_this_period": dec(view.spent_this_period.0),
                "carried": dec(view.carried.0),
                "period_length_ms": dec(view.period_length_ms),
                "period_start_ms": dec(view.period_start_ms),
                "expiry_ms": dec(view.expiry_ms),
                "revocation_counter": dec(view.revocation_counter),
                "rollover_policy": dec(view.rollover_policy),
                "closed": view.closed,
                "revoked": view.revoked,
                "delegates": view.delegates.iter().map(|delegate| hexv(delegate)).collect::<Vec<_>>(),
                "source_account": view.source_account.as_ref().map(|account| hexv(account)),
                "achieved_rung": view.achieved_rung.name(),
            }),
            Some(VerificationStatus::Achieved(legacy_level(
                view.achieved_rung,
            ))),
        ),
        BudgetRecord::Daemon(view) => (
            serde_json::json!({
                "enforcement": "DaemonLimit",
                "budget_id": hexv(&view.budget_id),
                "asset": hexv(&view.asset),
                "ceiling": dec(view.ceiling.0),
                "consumed": dec(view.consumed.0),
                "expiry_ms": dec(view.expiry_ms),
                "revoked": view.revoked,
                "notice": view.notice(),
            }),
            None,
        ),
    };
    if let Value::Object(fields) = &mut value {
        fields.insert("head".into(), dec(head));
        fields.insert(
            "activity_id".into(),
            activity_id.map_or(Value::Null, |id| hexv(id)),
        );
    }
    (value, verification)
}

/// `budget.state`: the owner reads the budget through the authenticated session; the record
/// keeps its own verification, and the state-proven balance, head and activity id are passed
/// through as the owner proved them.
pub(crate) fn budget_state<A: HumanAuthorityBoundary>(
    owner: &SharedAgentOwner<A>,
    context: &RpcOwnerContext<'_>,
    request: &Map<String, Value>,
    ctx: &DispatchContext,
) -> Result<Dispatched, Rejection> {
    let id = ctx.request_id;
    let typed = crate::agent_rpc_wire::budget_state_request(request, id)?;
    let response = owner
        .lock()
        .and_then(|mut guard| {
            let control = guard.session_control.clone();
            guard.budget_state(context, &control, typed)
        })
        .map_err(|error| owner_error(id, error))?;
    let (value, verification) = budget_state_operation(&response);
    Ok(Dispatched {
        value,
        verification,
    })
}

pub fn budget_state_operation(
    response: &AuthorityResponse<crate::human::BudgetState>,
) -> (Value, Option<VerificationStatus>) {
    let state = &response.value;
    let (record, verification) =
        budget_record_wire(&state.record, state.proven_head, state.activity_id.as_ref());
    (
        serde_json::json!({
            "authority": authority_wire(&response.authority),
            "value": {
                "record": record,
                "balance": dec(state.balance),
                "proven_head": dec(state.proven_head),
                "activity_id": state.activity_id.as_ref().map_or(Value::Null, |id| hexv(id)),
            },
        }),
        verification,
    )
}

fn freshness_value(freshness: &Freshness) -> Value {
    serde_json::json!({
        "chain_head": dec(freshness.chain_head),
        "value_height": dec(freshness.value_height),
        "relative_to": match freshness.relative_to {
            layerx_agent_api::read::RelativeTo::Block {height, hash} =>
                serde_json::json!({"height": dec(height), "hash": hexv(&hash)}),
        },
    })
}

fn legacy_level(rung: Rung) -> Rung {
    rung
}

fn unavailable_operation(id: RequestId, operation: &str, reason: &'static str) -> Rejection {
    let error = layerx_agent_api::error::ApiError::unavailable_capability(id, operation)
        .expect("static operation name");
    Rejection {
        class: error.class,
        retriability: error.retriability,
        request_id: id,
        reason,
    }
}

pub(crate) fn read_proof_bundle<A: HumanAuthorityBoundary>(
    owner: &SharedAgentOwner<A>,
    context: &RpcOwnerContext<'_>,
    request: &Map<String, Value>,
    ctx: &DispatchContext,
) -> Result<Dispatched, Rejection> {
    use crate::agent_rpc_wire::{decode_wire, ProofBundleWire};
    let id = ctx.request_id;
    let typed = decode_wire::<ProofBundleWire>(request, id)?.into_request(id)?;
    let response = owner
        .lock()
        .and_then(|mut guard| guard.rpc_read_proof_bundle(context, typed))
        .map_err(|error| owner_error(id, error))?;
    Ok(Dispatched {
        verification: Some(VerificationStatus::Achieved(response.achieved_rung)),
        value: crate::human_runtime::proof_bundle_operation_value(&response)
            .map_err(|error| owner_error(id, error))?,
    })
}

/// `export.offline`: the fact set is decoded through the shared fact grammar, the requested
/// level is passed to the owner unchanged, and the response is accepted only when it states
/// exactly the requested facts at or above the requested level.
pub(crate) fn export_offline<A: HumanAuthorityBoundary>(
    owner: &SharedAgentOwner<A>,
    context: &RpcOwnerContext<'_>,
    request: &Map<String, Value>,
    ctx: &DispatchContext,
) -> Result<Dispatched, Rejection> {
    use crate::agent_rpc_wire::{decode_wire, ExportOfflineWire};
    let id = ctx.request_id;
    decode_wire::<ExportOfflineWire>(request, id)?.into_request(id)?;
    let _ = (owner, context);
    Err(unavailable_operation(
        id,
        "export.offline",
        "unavailable_capability.export.offline",
    ))
}

/// `project` is the fee projection. The legacy policy payload (`context` +
/// `canonical_intent`) is refused with its typed reason and never reinterpreted.
pub(crate) fn project<A: HumanAuthorityBoundary>(
    owner: &SharedAgentOwner<A>,
    context: &RpcOwnerContext<'_>,
    request: &Map<String, Value>,
    ctx: &DispatchContext,
) -> Result<Dispatched, Rejection> {
    use crate::agent_rpc_wire::{decode_wire, FeeProjectionWire, LegacyProjectWire};
    let id = ctx.request_id;
    if decode_wire::<LegacyProjectWire>(request, id).is_ok() {
        return Err(owner_error(
            id,
            HumanOperationError::Typed(HumanRefusal::Policy(
                crate::policy::PolicyDryRunRefusal::LegacyProjectPayload,
            )),
        ));
    }
    let typed = decode_wire::<FeeProjectionWire>(request, id)?.into_request(id)?;
    let response = owner
        .lock()
        .and_then(|mut guard| guard.fee_projection(context.peer(), typed))
        .map_err(|error| owner_error(id, error))?;
    let fee = &response.projected;
    Ok(Dispatched {
        value: serde_json::json!({
            "projected": {
                "request": {
                    "protocol_activity_type": dec(fee.request.protocol_activity_type),
                    "canonical_bytes": dec(fee.request.canonical_bytes),
                    "execution_units": dec(fee.request.execution_units),
                    "storage_units": dec(fee.request.storage_units),
                },
                "parameter_version": dec(fee.parameter_version),
                "fee": dec(fee.fee.0),
                "canonical_schedule": hexv(fee.canonical_schedule.as_bytes()),
                "snapshot_height": dec(fee.snapshot_height),
                "gas_units": dec(fee.gas_units),
                "denom": fee.denom,
                "snapshot_state_root": hexv(&fee.snapshot_state_root),
            },
            "rationale": response.rationale,
            "observed_freshness": freshness_value(&response.observed_freshness),
        }),
        verification: None,
    })
}

/// `policy.dry_run`: a local restriction evaluated by the owner; the owner bytes are the
/// policy explanation record, decoded strictly. Never a verification level.
pub(crate) fn policy_dry_run<A: HumanAuthorityBoundary>(
    owner: &SharedAgentOwner<A>,
    context: &RpcOwnerContext<'_>,
    request: &Map<String, Value>,
    ctx: &DispatchContext,
) -> Result<Dispatched, Rejection> {
    use crate::agent_rpc_wire::PolicyDryRunShape;
    use crate::policy::{DecisionReason, EvaluationMode, Explanation, Outcome};
    use layerx_agent_api::policy::{PolicyDecisionReason, PolicyOutcome};
    let id = ctx.request_id;
    let malformed_response =
        || rejection(ErrorClass::InternalFault, id, "owner.response_malformed");
    let value = match crate::agent_rpc_wire::policy_dry_run_request(request, id)? {
        PolicyDryRunShape::Typed(typed) => {
            let response = owner
                .lock()
                .and_then(|mut guard| {
                    let control = guard.session_control.clone();
                    guard.policy_dry_run(context, &control, typed)
                })
                .map_err(|error| owner_error(id, error))?;
            let explanation = Explanation::from_machine_bytes(response.bytes())
                .map_err(|_| malformed_response())?;
            if explanation.mode != EvaluationMode::DryRun {
                return Err(malformed_response());
            }
            policy_dry_run_value(
                match explanation.outcome {
                    Outcome::Allow => "allow",
                    Outcome::Deny => "deny",
                },
                &explanation.policy_version,
                &explanation.matched_rules,
                explanation.deciding_rule.as_deref(),
                match explanation.reason {
                    DecisionReason::PermittedByRule => "permitted_by_rule",
                    DecisionReason::ExplicitDeny => "explicit_deny",
                    DecisionReason::ApprovalRequired => "approval_required",
                    DecisionReason::NoPermittingRule => "no_permitting_rule",
                    DecisionReason::InvalidContext => "invalid_context",
                    DecisionReason::EvaluationFailure => "evaluation_failure",
                },
                explanation.authority_statement,
            )
        }
        PolicyDryRunShape::Legacy(legacy) => {
            let response = owner
                .lock()
                .and_then(|mut guard| {
                    let control = guard.session_control.clone();
                    guard.policy_dry_run_legacy(context, &control, legacy)
                })
                .map_err(|error| owner_error(id, error))?;
            let result = response
                .value
                .validate()
                .map_err(|_| malformed_response())?;
            policy_dry_run_value(
                match result.outcome {
                    PolicyOutcome::Allow => "allow",
                    PolicyOutcome::Deny => "deny",
                },
                result.policy_version.as_str(),
                &result.matched_rules,
                result.deciding_rule.as_deref(),
                match result.reason {
                    PolicyDecisionReason::PermittedByRule => "permitted_by_rule",
                    PolicyDecisionReason::ExplicitDeny => "explicit_deny",
                    PolicyDecisionReason::ApprovalRequired => "approval_required",
                    PolicyDecisionReason::NoPermittingRule => "no_permitting_rule",
                    PolicyDecisionReason::InvalidContext => "invalid_context",
                    PolicyDecisionReason::EvaluationFailure => "evaluation_failure",
                },
                &result.authority_statement,
            )
        }
    };
    Ok(Dispatched {
        value,
        verification: None,
    })
}

/// The one seven-key `PolicyDryRunResult` wire encoding.
fn policy_dry_run_value(
    outcome: &str,
    policy_version: &str,
    matched_rules: &[String],
    deciding_rule: Option<&str>,
    reason: &str,
    authority_statement: &str,
) -> Value {
    serde_json::json!({
        "outcome": outcome,
        "policy_version": policy_version,
        "matched_rules": matched_rules,
        "deciding_rule": deciding_rule,
        "reason": reason,
        "mode": "dry_run",
        "authority_statement": authority_statement,
    })
}

/// Reads the layerxprograms registry at pending state without claiming a proof rung.
pub(crate) fn program_discover<A: HumanAuthorityBoundary>(
    owner: &SharedAgentOwner<A>,
    context: &RpcOwnerContext<'_>,
    request: &Map<String, Value>,
    ctx: &DispatchContext,
) -> Result<Dispatched, Rejection> {
    use crate::agent_rpc_wire::{decode_wire, ProgramDiscoverWire};
    let id = ctx.request_id;
    let program = decode_wire::<ProgramDiscoverWire>(request, id)?.into_request(id)?;
    let value = owner
        .lock()
        .and_then(|mut guard| {
            guard.rpc_program_registry_evm(context, program, Operation::ProgramDiscover)
        })
        .map_err(|error| owner_error(id, error))?;
    Ok(Dispatched {
        value,
        verification: None,
    })
}

pub(crate) fn program_interface<A: HumanAuthorityBoundary>(
    owner: &SharedAgentOwner<A>,
    context: &RpcOwnerContext<'_>,
    request: &Map<String, Value>,
    ctx: &DispatchContext,
) -> Result<Dispatched, Rejection> {
    use crate::agent_rpc_wire::{decode_wire, ProgramDiscoverWire};
    let id = ctx.request_id;
    let program = decode_wire::<ProgramDiscoverWire>(request, id)?.into_request(id)?;
    let value = owner
        .lock()
        .and_then(|mut guard| {
            guard.rpc_program_registry_evm(context, program, Operation::ProgramInterface)
        })
        .map_err(|error| owner_error(id, error))?;
    Ok(Dispatched {
        value,
        verification: None,
    })
}

pub(crate) fn program_activity<A: HumanAuthorityBoundary>(
    owner: &SharedAgentOwner<A>,
    context: &RpcOwnerContext<'_>,
    request: &Map<String, Value>,
    ctx: &DispatchContext,
) -> Result<Dispatched, Rejection> {
    use crate::agent_rpc_wire::{decode_wire, ProgramActivityWire};
    let id = ctx.request_id;
    let hash = decode_wire::<ProgramActivityWire>(request, id)?.into_request(id)?;
    let (raw, response) = owner
        .lock()
        .and_then(|mut guard| guard.rpc_program_activity_evm(context, hash))
        .map_err(|error| owner_error(id, error))?;
    let (value, rung) = program_call_answer(id, &raw, response)?;
    Ok(Dispatched {
        value,
        verification: Some(VerificationStatus::Achieved(rung)),
    })
}

pub(crate) fn program_simulate<A: HumanAuthorityBoundary>(
    owner: &SharedAgentOwner<A>,
    context: &RpcOwnerContext<'_>,
    request: &Map<String, Value>,
    ctx: &DispatchContext,
) -> Result<Dispatched, Rejection> {
    let id = ctx.request_id;
    let typed = crate::agent_rpc_wire::program_simulation_request(request, id)?;
    let value = owner
        .lock()
        .and_then(|mut guard| guard.rpc_program_simulate_evm(context, typed))
        .map_err(|error| owner_error(id, error))?;
    Ok(Dispatched {
        value,
        verification: None,
    })
}

pub(crate) fn read_batch<A: HumanAuthorityBoundary>(
    owner: &SharedAgentOwner<A>,
    context: &RpcOwnerContext<'_>,
    request: &Map<String, Value>,
    ctx: &DispatchContext,
) -> Result<Dispatched, Rejection> {
    let id = ctx.request_id;
    if request.contains_key("first_sequence")
        || request.contains_key("last_sequence")
        || request.contains_key("batch_header")
    {
        return Err(rejection(
            ErrorClass::PolicyRefusal,
            id,
            "read.batch_retired",
        ));
    }
    let request: BatchWire = decode(request, id)?;
    let requested = requested_level(&request.requested_rung, id)?;
    canonical_sequence(&request.batch, id)
        .map_err(|_| rejection(ErrorClass::PolicyRefusal, id, "read.batch_retired"))?;
    let typed = layerx_agent_api::read::ReadRequest {
        selector: layerx_agent_api::read::BatchRef::new(request.batch)
            .map_err(|_| malformed(id))?,
        requested_rung: requested,
    };
    let response = owner
        .lock()
        .and_then(|mut guard| guard.read_batch(context.peer(), typed));
    read_response(id, response)
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct SessionContextWire {
    tenant: String,
    agent_did: String,
    authority_ref: String,
    permitted_activity_types: Vec<String>,
    expiry: String,
    client: String,
    policy_version: String,
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct SessionTargetWire {
    session_id: String,
    context: SessionContextWire,
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct SessionListWire {
    context: SessionContextWire,
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct WaitWire {
    #[serde(default)]
    tenant: Option<String>,
    #[serde(default)]
    agent: Option<String>,
    submission_ref: String,
    #[serde(alias = "requested_verification_level")]
    requested_rung: String,
    deadline: String,
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct ModuleStateWire {
    #[serde(default)]
    tenant: Option<String>,
    #[serde(default)]
    agent: Option<String>,
    module: String,
    key: String,
    #[serde(alias = "requested_verification_level")]
    requested_rung: String,
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct HistoryRangeWire {
    first: String,
    last: String,
}

struct HistoryCursorWire {
    block_height: String,
    tx_index: String,
    log_index: String,
}
impl<'de> Deserialize<'de> for HistoryCursorWire {
    fn deserialize<D: serde::Deserializer<'de>>(deserializer: D) -> Result<Self, D::Error> {
        let [block_height, tx_index, log_index] = <[String; 3]>::deserialize(deserializer)?;
        Ok(Self {
            block_height,
            tx_index,
            log_index,
        })
    }
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct HistoryBlockRangeWire {
    from_height: String,
    to_height: String,
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct HistoryWire {
    #[serde(default)]
    tenant: Option<String>,
    #[serde(default)]
    agent: Option<String>,
    range: HistoryBlockRangeWire,
    cursor: Option<HistoryCursorWire>,
    page_limit: String,
    #[serde(alias = "requested_verification_level")]
    requested_rung: String,
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct AvailabilityWire {
    #[serde(default)]
    tenant: Option<String>,
    #[serde(default)]
    agent: Option<String>,
    selector: String,
    #[serde(alias = "requested_verification_level")]
    requested_rung: String,
    maximum_bytes: String,
    maximum_chunks: String,
    deadline: String,
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]

struct BatchWire {
    #[serde(default)]
    tenant: Option<String>,
    #[serde(default)]
    agent: Option<String>,
    batch: String,
    #[serde(alias = "requested_verification_level")]
    requested_rung: String,
}

/// Builds the typed session context and refuses coordinates that differ from the bound
/// principal (tenant.rs `CoordinateMismatch`, mapped to `session.not_authorized` by AMEND 18).
fn session_context(
    wire: SessionContextWire,
    context: &RpcOwnerContext<'_>,
    id: RequestId,
) -> Result<layerx_agent_api::identity::SessionContext, Rejection> {
    use layerx_agent_api::identity::{
        ActivityType, AgentDid, AuthorityRef, ClientId, ExplicitSet, PolicyVersion, SessionContext,
        TenantId,
    };
    let principal = context.principal();
    if wire.tenant != principal.tenant.as_str()
        || wire.agent_did.as_bytes() != principal.agent.as_bytes()
    {
        return Err(rejection(
            ErrorClass::PolicyRefusal,
            id,
            "session.not_authorized",
        ));
    }
    let activity_types = wire
        .permitted_activity_types
        .iter()
        .map(|text| {
            text.parse::<u16>()
                .ok()
                .filter(|value| value.to_string() == *text)
                .map(ActivityType)
                .ok_or_else(|| malformed(id))
        })
        .collect::<Result<Vec<_>, _>>()?;
    SessionContext::new(
        TenantId::new(wire.tenant).map_err(|_| malformed(id))?,
        AgentDid::new(wire.agent_did).map_err(|_| malformed(id))?,
        AuthorityRef::new(wire.authority_ref).map_err(|_| malformed(id))?,
        ExplicitSet::allow(activity_types),
        layerx_agent_api::generated::TimestampSeconds::parse_decimal(&wire.expiry)
            .ok()
            .filter(|value| value.get().to_string() == wire.expiry)
            .ok_or_else(|| malformed(id))?,
        ClientId::new(wire.client).map_err(|_| malformed(id))?,
        PolicyVersion::new(wire.policy_version).map_err(|_| malformed(id))?,
    )
    .map_err(|_| malformed(id))
}

fn canonical_sequence(
    text: &str,
    id: RequestId,
) -> Result<layerx_agent_api::generated::Sequence, Rejection> {
    layerx_agent_api::generated::Sequence::parse_decimal(text)
        .ok()
        .filter(|value| value.get().to_string() == text)
        .ok_or_else(|| malformed(id))
}

fn lower_hex_bytes(text: &str, id: RequestId) -> Result<Vec<u8>, Rejection> {
    let bytes = text.as_bytes();
    if bytes.is_empty()
        || bytes.len() % 2 != 0
        || !bytes
            .iter()
            .all(|byte| matches!(byte, b'0'..=b'9' | b'a'..=b'f'))
    {
        return Err(malformed(id));
    }
    bytes
        .chunks_exact(2)
        .map(|pair| {
            std::str::from_utf8(pair)
                .ok()
                .and_then(|pair| u8::from_str_radix(pair, 16).ok())
                .ok_or_else(|| malformed(id))
        })
        .collect()
}

/// Wire names as the SDKs send them (`program_http.rs` REQUESTED_VERIFICATION, config.rs).
fn requested_level(text: &str, id: RequestId) -> Result<Rung, Rejection> {
    Rung::from_name(text).ok_or_else(|| malformed(id))
}

const fn level_wire(value: Rung) -> &'static str {
    value.name()
}

/// AMEND 28b SESSION RECORD; returns the JSON record, its `open` flag and its session id.
fn decode_session_record(reader: &mut Reader<'_>) -> Option<(Value, bool, [u8; 32])> {
    let session_id: [u8; 32] = reader.fixed()?;
    let token_id: [u8; 32] = reader.fixed()?;
    let agent_did = reader.text()?;
    let generation = reader.u64()?;
    let open = match reader.u8()? {
        0 => false,
        1 => true,
        _ => return None,
    };
    let expiry_sequence = reader.u64()?;
    let sequence = reader.u64()?;
    let mut out = Map::new();
    out.insert("session_id".into(), hexv(&session_id));
    out.insert("token_id".into(), hexv(&token_id));
    out.insert("agent_did".into(), Value::String(agent_did));
    out.insert("generation".into(), dec(generation));
    out.insert("open".into(), Value::Bool(open));
    out.insert("expiry_sequence".into(), dec(expiry_sequence));
    out.insert("sequence".into(), dec(sequence));
    Some((Value::Object(out), open, session_id))
}

fn owner_payload(
    id: RequestId,
    response: Result<HumanResponse, HumanOperationError>,
    decoder: impl FnOnce(&mut Reader<'_>) -> Option<(Value, Option<Rung>)>,
) -> Result<Dispatched, Rejection> {
    let response = response.map_err(|error| owner_error(id, error))?;
    let mut reader = Reader {
        bytes: response.bytes(),
        offset: 0,
    };
    let (value, achieved) = decoder(&mut reader)
        .filter(|_| reader.finish().is_some())
        .ok_or_else(|| rejection(ErrorClass::InternalFault, id, "owner.response_malformed"))?;
    Ok(Dispatched {
        value,
        verification: achieved.map(VerificationStatus::Achieved),
    })
}

impl<'a> Reader<'a> {
    fn u16(&mut self) -> Option<u16> {
        Some(u16::from_be_bytes(self.fixed()?))
    }
    fn u32(&mut self) -> Option<u32> {
        Some(u32::from_be_bytes(self.fixed()?))
    }
    fn bytes(&mut self) -> Option<&'a [u8]> {
        let length = usize::try_from(self.u32()?).ok()?;
        (length != 0).then_some(())?;
        self.take(length)
    }
    fn text(&mut self) -> Option<String> {
        String::from_utf8(self.bytes()?.to_vec()).ok()
    }
    fn opt_bytes(&mut self) -> Option<&'a [u8]> {
        let length = usize::try_from(self.u32()?).ok()?;
        self.take(length)
    }
}

pub(crate) fn program_call<A: HumanAuthorityBoundary>(
    owner: &SharedAgentOwner<A>,
    context: &RpcOwnerContext<'_>,
    request: &Map<String, Value>,
    ctx: &DispatchContext,
) -> Result<Dispatched, Rejection> {
    let id = ctx.request_id;
    let typed = crate::agent_rpc_wire::program_simulation_request(request, id)?;
    let key = crate::agent_rpc_dispatch::mutation_key(ctx)?;
    let (raw, response) = owner
        .lock()
        .and_then(|mut guard| guard.rpc_program_call_evm(context, typed, key))
        .map_err(|error| owner_error(id, error))?;
    let (value, rung) = program_call_answer(id, &raw, response)?;
    Ok(Dispatched {
        value,
        verification: Some(VerificationStatus::Achieved(rung)),
    })
}

pub(crate) fn program_deploy<A: HumanAuthorityBoundary>(
    owner: &SharedAgentOwner<A>,
    context: &RpcOwnerContext<'_>,
    request: &Map<String, Value>,
    ctx: &DispatchContext,
) -> Result<Dispatched, Rejection> {
    program_lifecycle(owner, context, request, ctx, Operation::ProgramDeploy)
}

pub(crate) fn program_upgrade<A: HumanAuthorityBoundary>(
    owner: &SharedAgentOwner<A>,
    context: &RpcOwnerContext<'_>,
    request: &Map<String, Value>,
    ctx: &DispatchContext,
) -> Result<Dispatched, Rejection> {
    program_lifecycle(owner, context, request, ctx, Operation::ProgramUpgrade)
}

pub(crate) fn program_wind_down<A: HumanAuthorityBoundary>(
    owner: &SharedAgentOwner<A>,
    context: &RpcOwnerContext<'_>,
    request: &Map<String, Value>,
    ctx: &DispatchContext,
) -> Result<Dispatched, Rejection> {
    program_lifecycle(owner, context, request, ctx, Operation::ProgramWindDown)
}

fn program_lifecycle<A: HumanAuthorityBoundary>(
    owner: &SharedAgentOwner<A>,
    context: &RpcOwnerContext<'_>,
    request: &Map<String, Value>,
    ctx: &DispatchContext,
    operation: Operation,
) -> Result<Dispatched, Rejection> {
    use crate::agent_rpc_wire::{decode_wire, ProgramLifecycleWire};
    let id = ctx.request_id;
    let typed = decode_wire::<ProgramLifecycleWire>(request, id)?.into_request(id)?;
    let key = crate::agent_rpc_dispatch::mutation_key(ctx)?;
    let raw = typed.signed_activity;
    let response = owner
        .lock()
        .and_then(|mut guard| guard.rpc_program_lifecycle_evm(context, raw.clone(), key, operation))
        .map_err(|error| owner_error(id, error))?;
    let (value, rung) = program_call_answer(id, &raw, response)?;
    Ok(Dispatched {
        value,
        verification: Some(VerificationStatus::Achieved(rung)),
    })
}

pub fn program_call_answer(
    id: RequestId,
    raw: &[u8],
    response: HumanResponse,
) -> Result<(Value, Rung), Rejection> {
    use layerx_crypto::evm_transaction::{decode_signed, transaction_hash};
    let (call, _) = decode_signed(raw).map_err(|_| malformed(id))?;
    if call.to != crate::prepare::evm::PrecompileTarget::Programs.address() || call.data.len() < 4 {
        return Err(malformed(id));
    }
    let (value, rung) = crate::agent_rpc_dispatch::decode_write_answer(id, response)?;
    if value["tx_hash"] != hexv(&transaction_hash(raw)) {
        return Err(rejection(
            ErrorClass::InternalFault,
            id,
            "owner.response_malformed",
        ));
    }
    Ok((value, rung))
}

#[cfg(test)]
mod tests {
    use super::*;

    fn wait_json(rung: &str) -> Map<String, Value> {
        serde_json::json!({
            "submission_ref": "sub-1",
            "requested_rung": rung,
            "deadline": "1700000000",
        })
        .as_object()
        .cloned()
        .unwrap()
    }

    #[test]
    fn freshness_serializes_block_height_coordinates() {
        let value = freshness_value(&Freshness {
            chain_head: 12,
            value_height: 9,
            relative_to: layerx_agent_api::read::RelativeTo::Block {
                height: 9,
                hash: [4; 32],
            },
        });
        assert_eq!(
            value,
            serde_json::json!({
                "chain_head": "12",
                "value_height": "9",
                "relative_to": {"height": "9", "hash": "04".repeat(32)},
            })
        );
    }

    #[test]
    fn history_selector_checks_height_range_and_page_limit() {
        let selector = HistorySelector {
            from_height: 3,
            to_height: 7,
            cursor: Some(layerx_agent_api::read::HistoryCursor {
                block_height: 5,
                tx_index: 2,
                log_index: 1,
            }),
            page_limit: 4,
        };
        assert_eq!(selector.clone().validate(), Ok(selector.clone()));
        let mut inverted = selector.clone();
        inverted.from_height = 8;
        assert!(inverted.validate().is_err());
        let mut zero = selector;
        zero.page_limit = 0;
        assert!(zero.validate().is_err());
    }

    #[test]
    fn wait_request_reads_each_requested_rung_name() {
        for rung in [Rung::Submitted, Rung::Receipted, Rung::Committed] {
            let typed = wait_request(&wait_json(rung.name()), RequestId(7)).unwrap();
            assert_eq!(typed.requested_rung, rung);
            assert_eq!(typed.submission_ref.as_str(), "sub-1");
            assert_eq!(typed.deadline.get(), 1_700_000_000);
            let echoed = serde_json::json!({
                "submission_ref": typed.submission_ref.as_str(),
                "requested_rung": typed.requested_rung.name(),
                "deadline": typed.deadline.get().to_string(),
            });
            assert_eq!(echoed, Value::Object(wait_json(rung.name())));
        }
    }

    #[test]
    fn wait_request_refuses_retired_level_names_and_field() {
        let refusal = wait_request(&wait_json("batch-included"), RequestId(8)).unwrap_err();
        assert_eq!(refusal.reason, "envelope.malformed");
        assert_eq!(refusal.request_id, RequestId(8));
        let mut retired = wait_json("committed");
        retired.insert(
            "requested_verification_level".into(),
            Value::String("BatchIncluded".into()),
        );
        let refusal = wait_request(&retired, RequestId(9)).unwrap_err();
        assert_eq!(refusal.reason, "envelope.unknown_field");
        assert_eq!(refusal.class, ErrorClass::ProtocolIncompatibility);
    }
}

struct OwnerWriteProgress<A> {
    owner: SharedAgentOwner<A>,
    peer: crate::human::HumanPeer,
    reference: String,
    request_id: RequestId,
    started: std::time::Instant,
    first_observation_ms: u64,
    submission: Option<Value>,
}

impl<A: HumanAuthorityBoundary> crate::finality::VerificationProgress for OwnerWriteProgress<A> {
    fn rung_at(
        &mut self,
        hash: [u8; 32],
        observed_at_ms: u64,
    ) -> Result<Rung, crate::finality::FinalityError> {
        use crate::finality::FinalityError;
        if crate::agent_rpc_dispatch::submission_key(&self.reference, self.request_id).ok()
            != Some(hash)
        {
            return Err(FinalityError::ProgressUnavailable);
        }
        let due = std::time::Duration::from_millis(
            observed_at_ms.saturating_sub(self.first_observation_ms),
        );
        if let Some(remaining) = due.checked_sub(self.started.elapsed()) {
            std::thread::sleep(remaining);
        }
        let response = self
            .owner
            .track(&self.peer, &self.reference)
            .map_err(|_| FinalityError::ProgressUnavailable)?;
        let (submission, rung) =
            crate::agent_rpc_dispatch::decode_write_answer(self.request_id, response)
                .map_err(|_| FinalityError::ProgressUnavailable)?;
        self.submission = Some(submission);
        Ok(rung)
    }
}

fn read_response(
    id: RequestId,
    response: Result<HumanResponse, HumanOperationError>,
) -> Result<Dispatched, Rejection> {
    owner_payload(id, response, |reader| {
        let value: Value = serde_json::from_slice(reader.bytes()?).ok()?;
        if !value["achieved_verification_level"].is_null()
            || !value["value"].is_object()
            || !value["freshness"].is_object()
        {
            return None;
        }
        Some((value, None))
    })
}
