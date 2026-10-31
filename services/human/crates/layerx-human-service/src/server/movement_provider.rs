//! Privilege-separated movement planning and external execution boundary.

use std::env;
use std::fs;
use std::io::{Read, Write};
use std::os::unix::fs::{FileTypeExt, MetadataExt};
use std::os::unix::net::UnixStream;
use std::path::{Path, PathBuf};
use std::sync::Arc;
use std::time::Duration;

use rustix::net::sockopt::socket_peercred;

use crate::journeys::MovePlan;
use crate::store::{AgentTenantId, PrincipalId, PrincipalScope, RowKey, Table};
use crate::trace::TraceId;

/// Movement frame revision. Revision 3 carries the native custody precompile
/// shapes: withdrawal material in place of checkpoint proofs, the payable
/// deposit value on a prepared transaction, and the token pointer on a custody
/// request. A revision 2 peer speaks the removed Solidity vault shapes and is
/// refused rather than decoded.
pub const MOVEMENT_PROTOCOL_VERSION: u16 = 3;
const PROTOCOL_VERSION: u16 = MOVEMENT_PROTOCOL_VERSION;
const BODY_VERSION: u8 = 3;

/// Mandatory local transport policy for the movement provider.
#[derive(Clone, Debug)]
pub struct MovementProviderConfig {
    pub socket: PathBuf,
    pub peer_uid: u32,
    pub peer_gid: u32,
    pub maximum_frame_bytes: usize,
    pub deadline: Duration,
}

impl MovementProviderConfig {
    /// Reads a complete configuration. Missing, relative, zero, or overly broad
    /// values refuse startup rather than selecting development defaults.
    /// # Errors
    /// Refuses invalid or unavailable movement authority, transport, or canonical evidence.
    pub fn from_environment() -> Result<Self, MovementProviderError> {
        let socket = required("LAYERX_HUMAN_MOVEMENT_SOCKET").map(PathBuf::from)?;
        if !socket.is_absolute() {
            return Err(MovementProviderError::Configuration);
        }
        let peer_uid = number("LAYERX_HUMAN_MOVEMENT_PEER_UID")?;
        let group_id = number("LAYERX_HUMAN_MOVEMENT_PEER_GID")?;
        let maximum_frame_bytes = number("LAYERX_HUMAN_MOVEMENT_MAX_FRAME_BYTES")?;
        let deadline_seconds: u64 = number("LAYERX_HUMAN_MOVEMENT_DEADLINE_SECONDS")?;
        if maximum_frame_bytes == 0 || deadline_seconds == 0 {
            return Err(MovementProviderError::Configuration);
        }
        Ok(Self {
            socket,
            peer_uid,
            peer_gid: group_id,
            maximum_frame_bytes,
            deadline: Duration::from_secs(deadline_seconds),
        })
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct PlanningContext {
    pub request_anchor: [u8; 32],
    pub account: layerx_types::account::AccountId,
    pub reserve: layerx_types::account::AccountId,
    pub withdrawals_account: layerx_types::account::AccountId,
    pub route: Option<crate::journeys::RouteRequest>,
    pub amount: layerx_types::amount::Amount,
    pub asset: layerx_types::ids::AssetId,
    pub currency: String,
    pub actor: layerx_agent_api::identity::AgentDid,
    pub authority: layerx_agent_api::identity::AuthorityRef,
    pub custody_key: crate::custody::KeyId,
    pub custody_provider_reference: Vec<u8>,
    pub custody_binding_digest: [u8; 32],
    pub wallet: layerx_types::intent::EvmAddress,
    pub network: layerx_types::intent::NetworkId,
    pub protocol_version: u16,
    pub paxeer_chain_id: u64,
    pub account_sequence: u64,
    pub budget_grant: Option<PlanningBudgetGrant>,
    pub fee_limit: u128,
    pub evm_gas_limit: u64,
    pub evm_max_fee_per_gas: u64,
    pub evm_max_priority_fee_per_gas: u64,
    pub not_before: u64,
    pub not_after: u64,
    pub binding_receipt_digest: [u8; 32],
    pub identity_authority_evidence: Vec<u8>,
    pub balance_evidence: Vec<u8>,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct PlanningBudgetGrant {
    pub budget: layerx_types::intent::BudgetId,
    pub grant: layerx_types::intent::AuthorityGrantId,
}

/// Exact caller authority and request bytes used when constructing a plan.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct PlanningRequest {
    pub principal: PrincipalId,
    pub tenant: AgentTenantId,
    pub context: PlanningContext,
    pub operation: String,
    pub idempotency_key: [u8; 32],
    pub canonical_body: Vec<u8>,
    pub trace: TraceId,
    pub now: u64,
}

impl PlanningRequest {
    /// Reconstructs canonical provider fields while enforcing boundary bounds.
    /// # Errors
    /// Refuses absent identity evidence, invalid execution bounds, and malformed request data.
    pub fn from_wire_parts(value: Self) -> Result<Self, MovementProviderError> {
        let Self {
            principal,
            tenant,
            context,
            operation,
            idempotency_key,
            canonical_body,
            trace,
            now,
        } = value;
        if context.paxeer_chain_id == 0
            || context.amount.value() == 0
            || context.asset.bytes() == [0; 32]
            || context.custody_provider_reference.is_empty()
            || context.custody_binding_digest == [0; 32]
            || context.evm_gas_limit == 0
            || context.evm_max_fee_per_gas == 0
            || context.evm_max_priority_fee_per_gas > context.evm_max_fee_per_gas
            || context.reserve.namespace()
                != layerx_types::account::AccountNamespace::SystemTreasury
            || context.withdrawals_account.namespace()
                != layerx_types::account::AccountNamespace::SystemPaxeerWithdrawals
            || context
                .route
                .as_ref()
                .is_some_and(|route| route.asset != context.asset || route.amount != context.amount)
            || context.wallet.bytes() == [0; 20]
            || context.binding_receipt_digest == [0; 32]
            || context.identity_authority_evidence.is_empty()
            || context.balance_evidence.is_empty()
            || context.not_before > now
            || context.not_after <= now
            || !matches!(context.protocol_version, 2 | 3)
            || (operation == "withdraw.start"
                && (context.request_anchor == [0; 32] || context.fee_limit > u128::from(u64::MAX)))
            || operation.is_empty()
            || operation.len() > 128
            || operation.chars().any(char::is_control)
            || idempotency_key == [0; 32]
            || canonical_body.is_empty()
            || canonical_body.len() > 1_048_576
            || now == 0
        {
            return Err(MovementProviderError::ContractViolation);
        }
        Ok(Self {
            principal,
            tenant,
            context,
            operation,
            idempotency_key,
            canonical_body,
            trace,
            now,
        })
    }
}

/// Provider-owned review record. `quote_id` names the durable provider row;
/// the complete plan is returned only with the exact expiry committed there.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct AuthorizedMovePlan {
    pub quote_id: String,
    pub expires_at: u64,
    pub arrival_at: u64,
    pub plan: MovePlan,
}

impl AuthorizedMovePlan {
    /// Encodes the quote envelope and its canonical owner plan.
    #[must_use]
    pub fn canonical_encode(&self) -> Vec<u8> {
        let plan = self.plan.canonical_encode();
        let mut out = vec![1];
        out.extend(
            u16::try_from(self.quote_id.len())
                .unwrap_or_else(|_| unreachable!("validated quote length"))
                .to_be_bytes(),
        );
        out.extend(self.quote_id.as_bytes());
        out.extend(self.expires_at.to_be_bytes());
        out.extend(self.arrival_at.to_be_bytes());
        out.extend(
            u32::try_from(plan.len())
                .unwrap_or_else(|_| unreachable!("validated plan length"))
                .to_be_bytes(),
        );
        out.extend(plan);
        out
    }

    /// Decodes a quote envelope, rejects trailing bytes, and revalidates its plan.
    /// # Errors
    /// Refuses invalid or unavailable movement authority, transport, or canonical evidence.
    pub fn canonical_decode(bytes: &[u8]) -> Result<Self, MovementProviderError> {
        if bytes.is_empty() || bytes.len() > 1_048_576 {
            return Err(MovementProviderError::ContractViolation);
        }
        let mut at = 0usize;
        let mut take = |n: usize| {
            let end = at
                .checked_add(n)
                .ok_or(MovementProviderError::ContractViolation)?;
            let value = bytes
                .get(at..end)
                .ok_or(MovementProviderError::ContractViolation)?;
            at = end;
            Ok::<_, MovementProviderError>(value)
        };
        if take(1)?[0] != 1 {
            return Err(MovementProviderError::ContractViolation);
        }
        let quote_len = u16::from_be_bytes(
            take(2)?
                .try_into()
                .map_err(|_| MovementProviderError::ContractViolation)?,
        ) as usize;
        if !(16..=128).contains(&quote_len) {
            return Err(MovementProviderError::ContractViolation);
        }
        let quote_id = std::str::from_utf8(take(quote_len)?)
            .map_err(|_| MovementProviderError::ContractViolation)?
            .to_owned();
        let expires_at = u64::from_be_bytes(
            take(8)?
                .try_into()
                .map_err(|_| MovementProviderError::ContractViolation)?,
        );
        let arrival_at = u64::from_be_bytes(
            take(8)?
                .try_into()
                .map_err(|_| MovementProviderError::ContractViolation)?,
        );
        let plan_len = u32::from_be_bytes(
            take(4)?
                .try_into()
                .map_err(|_| MovementProviderError::ContractViolation)?,
        ) as usize;
        if plan_len == 0 || plan_len > 1_048_576 {
            return Err(MovementProviderError::ContractViolation);
        }
        let plan = MovePlan::canonical_decode(take(plan_len)?)
            .map_err(|_| MovementProviderError::ContractViolation)?;
        if at != bytes.len() {
            return Err(MovementProviderError::ContractViolation);
        }
        Self::from_wire_parts(quote_id, expires_at, arrival_at, plan)
    }

    /// Constructs one provider-owned quote with a non-empty review window.
    /// # Errors
    /// Refuses invalid or unavailable movement authority, transport, or canonical evidence.
    pub fn from_wire_parts(
        quote_id: String,
        expires_at: u64,
        arrival_at: u64,
        plan: MovePlan,
    ) -> Result<Self, MovementProviderError> {
        quote_row(&quote_id)?;
        if expires_at == 0 || arrival_at == 0 || arrival_at < expires_at {
            return Err(MovementProviderError::ContractViolation);
        }
        Ok(Self {
            quote_id,
            expires_at,
            arrival_at,
            plan,
        })
    }
}

/// Typed provider calls. Economic requests remain native domain objects; they
/// are never converted to an unclassified JSON command.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum MovementProviderRequest {
    PlanMove(PlanningRequest),
    Readiness,
}

/// Exhaustive typed results. A mismatched response discriminant is a contract
/// violation and is never interpreted as success.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum MovementProviderResponse {
    MovePlan(AuthorizedMovePlan),
    Ready,
    Unavailable,
    ContractViolation,
}

/// Authoritative native codec shared by the provider daemon and its client.
/// Implementations must encode every field and validate canonicality, bounds,
/// protocol version, and native proof construction while decoding.
pub trait MovementProviderCodec: Send + Sync {
    /// # Errors
    /// Refuses invalid or unavailable movement authority, transport, or canonical evidence.
    fn encode_request(
        &self,
        request: &MovementProviderRequest,
    ) -> Result<Vec<u8>, MovementProviderError>;
    /// # Errors
    /// Refuses malformed, noncanonical, or unsupported movement frames.
    fn decode_request(
        &self,
        bytes: &[u8],
    ) -> Result<MovementProviderRequest, MovementProviderError>;
    /// # Errors
    /// Refuses invalid or unavailable movement authority, transport, or canonical evidence.
    fn encode_response(
        &self,
        response: &MovementProviderResponse,
    ) -> Result<Vec<u8>, MovementProviderError>;
    /// # Errors
    /// Refuses malformed, noncanonical, or unsupported movement frames.
    fn decode_response(
        &self,
        bytes: &[u8],
    ) -> Result<MovementProviderResponse, MovementProviderError>;
}

/// Canonical native codec. Every nested economic object delegates to its
/// owner module's validated wire representation.
#[derive(Clone, Copy, Debug, Default)]
pub struct NativeMovementCodec;

impl NativeMovementCodec {
    #[must_use]
    pub const fn new() -> Self {
        Self
    }
}

impl MovementProviderCodec for NativeMovementCodec {
    fn encode_request(
        &self,
        request: &MovementProviderRequest,
    ) -> Result<Vec<u8>, MovementProviderError> {
        let mut w = MpWriter::new();
        match request {
            MovementProviderRequest::PlanMove(v) => {
                w.tag(1);
                w.planning(v)?;
            }
            MovementProviderRequest::Readiness => w.tag(14),
        }
        native_frame(w.finish())
    }
    fn decode_request(
        &self,
        bytes: &[u8],
    ) -> Result<MovementProviderRequest, MovementProviderError> {
        let mut r = MpReader::new(bytes)?;
        let value = match r.u8()? {
            1 => MovementProviderRequest::PlanMove(r.planning()?),
            14 => MovementProviderRequest::Readiness,
            _ => return Err(MovementProviderError::ContractViolation),
        };
        r.finish()?;
        if self.encode_request(&value)? != bytes {
            return Err(MovementProviderError::ContractViolation);
        }
        Ok(value)
    }
    fn encode_response(
        &self,
        response: &MovementProviderResponse,
    ) -> Result<Vec<u8>, MovementProviderError> {
        let mut w = MpWriter::new();
        match response {
            MovementProviderResponse::MovePlan(_) => {
                w.plan_response(response)?;
            }
            MovementProviderResponse::Ready => w.tag(14),
            MovementProviderResponse::Unavailable => w.tag(15),
            MovementProviderResponse::ContractViolation => w.tag(16),
        }
        native_frame(w.finish())
    }
    fn decode_response(
        &self,
        bytes: &[u8],
    ) -> Result<MovementProviderResponse, MovementProviderError> {
        let mut r = MpReader::new(bytes)?;
        let value = match r.u8()? {
            1 => MovementProviderResponse::MovePlan(AuthorizedMovePlan::canonical_decode(
                r.blob(1_048_576)?,
            )?),
            14 => MovementProviderResponse::Ready,
            15 => MovementProviderResponse::Unavailable,
            16 => MovementProviderResponse::ContractViolation,
            _ => return Err(MovementProviderError::ContractViolation),
        };
        r.finish()?;
        if self.encode_response(&value)? != bytes {
            return Err(MovementProviderError::ContractViolation);
        }
        Ok(value)
    }
}

struct MpWriter {
    out: Vec<u8>,
}
fn native_frame(bytes: Vec<u8>) -> Result<Vec<u8>, MovementProviderError> {
    if bytes.len() > 1_048_576 {
        Err(MovementProviderError::ContractViolation)
    } else {
        Ok(bytes)
    }
}
impl MpWriter {
    fn new() -> Self {
        Self {
            out: vec![BODY_VERSION],
        }
    }
    fn tag(&mut self, v: u8) {
        self.u8(v);
    }
    fn u8(&mut self, v: u8) {
        self.out.push(v);
    }
    fn u32(&mut self, v: u32) {
        self.out.extend(v.to_be_bytes());
    }
    fn u64(&mut self, v: u64) {
        self.out.extend(v.to_be_bytes());
    }
    fn fixed(&mut self, v: &[u8]) {
        self.out.extend(v);
    }
    fn text(&mut self, v: &str, max: usize) -> Result<(), MovementProviderError> {
        if v.is_empty()
            || v.len() > max
            || v.len() > u16::MAX as usize
            || v.chars().any(char::is_control)
        {
            return Err(MovementProviderError::ContractViolation);
        }
        self.out.extend(
            u16::try_from(v.len())
                .map_err(|_| MovementProviderError::ContractViolation)?
                .to_be_bytes(),
        );
        self.out.extend(v.as_bytes());
        Ok(())
    }
    fn blob(&mut self, v: &[u8], max: usize) -> Result<(), MovementProviderError> {
        if v.is_empty() || v.len() > max || v.len() > u32::MAX as usize {
            return Err(MovementProviderError::ContractViolation);
        }
        self.u32(u32::try_from(v.len()).map_err(|_| MovementProviderError::ContractViolation)?);
        self.out.extend(v);
        Ok(())
    }
    fn planning(&mut self, v: &PlanningRequest) -> Result<(), MovementProviderError> {
        PlanningRequest::from_wire_parts(v.clone())?;
        self.text(v.principal.as_str(), 128)?;
        self.text(v.tenant.as_str(), 255)?;
        self.planning_context(&v.context)?;
        self.text(&v.operation, 128)?;
        self.fixed(&v.idempotency_key);
        self.blob(&v.canonical_body, 1_048_576)?;
        self.text(v.trace.as_str(), 64)?;
        self.u64(v.now);
        Ok(())
    }
    fn planning_context(&mut self, v: &PlanningContext) -> Result<(), MovementProviderError> {
        self.u8(2);
        self.fixed(&v.request_anchor);
        self.text(v.account.canonical(), 512)?;
        self.text(v.reserve.canonical(), 512)?;
        self.text(v.withdrawals_account.canonical(), 512)?;
        match &v.route {
            Some(route) => {
                self.u8(1);
                self.blob(&route.canonical_encode(), 262_144)?;
            }
            None => self.u8(0),
        }
        self.out.extend(v.amount.to_be_bytes());
        self.fixed(&v.asset.bytes());
        self.text(&v.currency, 128)?;
        self.text(v.actor.as_str(), 255)?;
        self.text(v.authority.as_str(), 255)?;
        self.text(v.custody_key.as_str(), 128)?;
        self.blob(&v.custody_provider_reference, 4096)?;
        self.fixed(&v.custody_binding_digest);
        self.fixed(&v.wallet.bytes());
        self.u32(v.network.value());
        self.out.extend(v.protocol_version.to_be_bytes());
        self.u64(v.paxeer_chain_id);
        self.u64(v.account_sequence);
        match &v.budget_grant {
            Some(grant) => {
                self.u8(1);
                self.fixed(&grant.budget.bytes());
                self.fixed(&grant.grant.bytes());
            }
            None => self.u8(0),
        }
        self.out.extend(v.fee_limit.to_be_bytes());
        self.u64(v.evm_gas_limit);
        self.u64(v.evm_max_fee_per_gas);
        self.u64(v.evm_max_priority_fee_per_gas);
        self.u64(v.not_before);
        self.u64(v.not_after);
        self.fixed(&v.binding_receipt_digest);
        self.blob(&v.identity_authority_evidence, 262_144)?;
        self.blob(&v.balance_evidence, 262_144)
    }
    fn execution_identity(
        &mut self,
        identity: &crate::journeys::MovementExecutionIdentity,
    ) -> Result<(), MovementProviderError> {
        if identity.account == [0; 32]
            || identity.plan_id == [0; 32]
            || identity.wallet.bytes() == [0; 20]
        {
            return Err(MovementProviderError::ContractViolation);
        }
        self.text(identity.principal.as_str(), 128)?;
        self.text(identity.tenant.as_str(), 255)?;
        self.fixed(&identity.account);
        self.fixed(&identity.wallet.bytes());
        self.fixed(&identity.plan_id);
        Ok(())
    }
    fn evm_transaction(
        &mut self,
        transaction: &crate::custody::EvmTransaction,
    ) -> Result<(), MovementProviderError> {
        self.tag(17);
        self.u64(transaction.chain_id);
        self.u64(transaction.nonce);
        self.u64(transaction.max_priority_fee_per_gas);
        self.u64(transaction.max_fee_per_gas);
        self.u64(transaction.gas_limit);
        self.fixed(&transaction.to);
        self.fixed(&transaction.value);
        self.blob(&transaction.calldata, 262_144)?;
        Ok(())
    }
    fn plan_response(
        &mut self,
        response: &MovementProviderResponse,
    ) -> Result<(), MovementProviderError> {
        let w = self;
        match response {
            MovementProviderResponse::MovePlan(v) => {
                w.tag(1);
                w.blob(&v.canonical_encode(), 1_048_576)?;
            }
            _ => return Err(MovementProviderError::ContractViolation),
        }
        Ok(())
    }
    fn finish(self) -> Vec<u8> {
        self.out
    }
}
struct MpReader<'a> {
    bytes: &'a [u8],
    at: usize,
}
impl<'a> MpReader<'a> {
    fn new(bytes: &'a [u8]) -> Result<Self, MovementProviderError> {
        if bytes.len() < 2 || bytes.len() > 1_048_576 || bytes[0] != BODY_VERSION {
            return Err(MovementProviderError::ContractViolation);
        }
        Ok(Self { bytes, at: 1 })
    }
    fn take(&mut self, n: usize) -> Result<&'a [u8], MovementProviderError> {
        let end = self
            .at
            .checked_add(n)
            .ok_or(MovementProviderError::ContractViolation)?;
        let v = self
            .bytes
            .get(self.at..end)
            .ok_or(MovementProviderError::ContractViolation)?;
        self.at = end;
        Ok(v)
    }
    fn fixed<const N: usize>(&mut self) -> Result<[u8; N], MovementProviderError> {
        self.take(N)?
            .try_into()
            .map_err(|_| MovementProviderError::ContractViolation)
    }
    fn u8(&mut self) -> Result<u8, MovementProviderError> {
        Ok(self.fixed::<1>()?[0])
    }
    fn u16(&mut self) -> Result<u16, MovementProviderError> {
        Ok(u16::from_be_bytes(self.fixed()?))
    }
    fn u32(&mut self) -> Result<u32, MovementProviderError> {
        Ok(u32::from_be_bytes(self.fixed()?))
    }
    fn u64(&mut self) -> Result<u64, MovementProviderError> {
        Ok(u64::from_be_bytes(self.fixed()?))
    }
    fn u128(&mut self) -> Result<u128, MovementProviderError> {
        Ok(u128::from_be_bytes(self.fixed()?))
    }
    fn text(&mut self, max: usize) -> Result<String, MovementProviderError> {
        let n = self.u16()? as usize;
        if n == 0 || n > max {
            return Err(MovementProviderError::ContractViolation);
        }
        let v = std::str::from_utf8(self.take(n)?)
            .map_err(|_| MovementProviderError::ContractViolation)?;
        if v.chars().any(char::is_control) {
            return Err(MovementProviderError::ContractViolation);
        }
        Ok(v.to_owned())
    }
    fn blob(&mut self, max: usize) -> Result<&'a [u8], MovementProviderError> {
        let n = self.u32()? as usize;
        if n == 0 || n > max {
            return Err(MovementProviderError::ContractViolation);
        }
        self.take(n)
    }
    fn planning(&mut self) -> Result<PlanningRequest, MovementProviderError> {
        PlanningRequest::from_wire_parts(PlanningRequest {
            principal: PrincipalId::new(self.text(128)?)
                .map_err(|_| MovementProviderError::ContractViolation)?,
            tenant: AgentTenantId::new(self.text(255)?)
                .map_err(|_| MovementProviderError::ContractViolation)?,
            context: self.planning_context()?,
            operation: self.text(128)?,
            idempotency_key: self.fixed()?,
            canonical_body: self.blob(1_048_576)?.to_vec(),
            trace: TraceId::parse(&self.text(64)?)
                .map_err(|_| MovementProviderError::ContractViolation)?,
            now: self.u64()?,
        })
    }

    fn planning_context(&mut self) -> Result<PlanningContext, MovementProviderError> {
        if self.u8()? != 2 {
            return Err(MovementProviderError::ContractViolation);
        }
        Ok(PlanningContext {
            request_anchor: self.fixed()?,
            account: layerx_types::account::AccountId::parse(&self.text(512)?)
                .map_err(|_| MovementProviderError::ContractViolation)?,
            reserve: layerx_types::account::AccountId::parse(&self.text(512)?)
                .map_err(|_| MovementProviderError::ContractViolation)?,
            withdrawals_account: layerx_types::account::AccountId::parse(&self.text(512)?)
                .map_err(|_| MovementProviderError::ContractViolation)?,
            route: match self.u8()? {
                0 => None,
                1 => Some(
                    crate::journeys::RouteRequest::canonical_decode(self.blob(262_144)?)
                        .map_err(|_| MovementProviderError::ContractViolation)?,
                ),
                _ => return Err(MovementProviderError::ContractViolation),
            },
            amount: layerx_types::amount::Amount::from_u128(self.u128()?),
            asset: layerx_types::ids::AssetId::new(self.fixed()?),
            currency: self.text(128)?,
            actor: layerx_agent_api::identity::AgentDid::new(self.text(255)?)
                .map_err(|_| MovementProviderError::ContractViolation)?,
            authority: layerx_agent_api::identity::AuthorityRef::new(self.text(255)?)
                .map_err(|_| MovementProviderError::ContractViolation)?,
            custody_key: crate::custody::KeyId::new(self.text(128)?)
                .map_err(|_| MovementProviderError::ContractViolation)?,
            custody_provider_reference: self.blob(4096)?.to_vec(),
            custody_binding_digest: self.fixed()?,
            wallet: layerx_types::intent::EvmAddress::new(self.fixed()?),
            network: layerx_types::intent::NetworkId::new(self.u32()?)
                .map_err(|_| MovementProviderError::ContractViolation)?,
            protocol_version: self.u16()?,
            paxeer_chain_id: self.u64()?,
            account_sequence: self.u64()?,
            budget_grant: match self.u8()? {
                0 => None,
                1 => Some(PlanningBudgetGrant {
                    budget: layerx_types::intent::BudgetId::new(self.fixed()?),
                    grant: layerx_types::intent::AuthorityGrantId::new(self.fixed()?),
                }),
                _ => return Err(MovementProviderError::ContractViolation),
            },
            fee_limit: self.u128()?,
            evm_gas_limit: self.u64()?,
            evm_max_fee_per_gas: self.u64()?,
            evm_max_priority_fee_per_gas: self.u64()?,
            not_before: self.u64()?,
            not_after: self.u64()?,
            binding_receipt_digest: self.fixed()?,
            identity_authority_evidence: self.blob(262_144)?.to_vec(),
            balance_evidence: self.blob(262_144)?.to_vec(),
        })
    }
    fn evm_transaction(&mut self) -> Result<crate::custody::EvmTransaction, MovementProviderError> {
        Ok(crate::custody::EvmTransaction {
            chain_id: self.u64()?,
            nonce: self.u64()?,
            max_priority_fee_per_gas: self.u64()?,
            max_fee_per_gas: self.u64()?,
            gas_limit: self.u64()?,
            to: self.fixed()?,
            value: self.fixed()?,
            calldata: self.blob(262_144)?.to_vec(),
        })
    }
    fn execution_identity(
        &mut self,
    ) -> Result<crate::journeys::MovementExecutionIdentity, MovementProviderError> {
        Ok(crate::journeys::MovementExecutionIdentity {
            principal: PrincipalId::new(self.text(128)?)
                .map_err(|_| MovementProviderError::ContractViolation)?,
            tenant: AgentTenantId::new(self.text(255)?)
                .map_err(|_| MovementProviderError::ContractViolation)?,
            account: self.fixed()?,
            wallet: layerx_types::intent::EvmAddress::new(self.fixed()?),
            plan_id: self.fixed()?,
        })
    }
    fn finish(self) -> Result<(), MovementProviderError> {
        if self.at == self.bytes.len() {
            Ok(())
        } else {
            Err(MovementProviderError::ContractViolation)
        }
    }
}

/// Server-side contract. The daemon owns the real routing, wallet, Paxeer,
/// proof, and settlement implementations behind this exhaustive dispatch.
pub trait MovementProviderService: Send {
    fn dispatch(&mut self, request: MovementProviderRequest) -> MovementProviderResponse;
}

/// Serves one already-accepted provider connection after authenticating its
/// kernel identity. Listener ownership and concurrency admission remain with
/// the provider daemon so it can apply one process-wide bound.
/// # Errors
/// Refuses invalid or unavailable movement authority, transport, or canonical evidence.
pub fn serve_connection(
    mut stream: UnixStream,
    client_uid: u32,
    group_id: u32,
    maximum_frame_bytes: usize,
    deadline: Duration,
    codec: &dyn MovementProviderCodec,
    service: &mut dyn MovementProviderService,
) -> Result<(), MovementProviderError> {
    if maximum_frame_bytes == 0 || deadline.is_zero() {
        return Err(MovementProviderError::Configuration);
    }
    stream
        .set_read_timeout(Some(deadline))
        .map_err(|_| MovementProviderError::Unavailable)?;
    stream
        .set_write_timeout(Some(deadline))
        .map_err(|_| MovementProviderError::Unavailable)?;
    let credentials = socket_peercred(&stream).map_err(|_| MovementProviderError::Unavailable)?;
    if credentials.uid.as_raw() != client_uid || credentials.gid.as_raw() != group_id {
        return Err(MovementProviderError::ContractViolation);
    }
    let mut header = [0_u8; 10];
    stream
        .read_exact(&mut header)
        .map_err(|_| MovementProviderError::Unavailable)?;
    if u16::from_be_bytes([header[0], header[1]]) != PROTOCOL_VERSION {
        return Err(MovementProviderError::ContractViolation);
    }
    let length = usize::try_from(u64::from_be_bytes(
        header[2..10]
            .try_into()
            .map_err(|_| MovementProviderError::ContractViolation)?,
    ))
    .map_err(|_| MovementProviderError::ContractViolation)?;
    if length == 0 || length > maximum_frame_bytes {
        return Err(MovementProviderError::ContractViolation);
    }
    let mut payload = vec![0; length];
    stream
        .read_exact(&mut payload)
        .map_err(|_| MovementProviderError::Unavailable)?;
    let request = codec.decode_request(&payload)?;
    let response = codec.encode_response(&service.dispatch(request))?;
    if response.is_empty() || response.len() > maximum_frame_bytes {
        return Err(MovementProviderError::ContractViolation);
    }
    stream
        .write_all(&PROTOCOL_VERSION.to_be_bytes())
        .map_err(|_| MovementProviderError::Unavailable)?;
    stream
        .write_all(&(response.len() as u64).to_be_bytes())
        .map_err(|_| MovementProviderError::Unavailable)?;
    stream
        .write_all(&response)
        .map_err(|_| MovementProviderError::Unavailable)
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum MovementProviderError {
    Configuration,
    Unavailable,
    ContractViolation,
}

/// Production UDS client and the runtime adapters consumed by durable journeys.
pub struct UnixMovementProvider {
    config: MovementProviderConfig,
    codec: Arc<dyn MovementProviderCodec>,
    execution_authority: Option<Arc<crate::custody::CustodySigner>>,
    planning_authorities: std::collections::BTreeMap<[u8; 32], PlanningRequest>,
}

impl UnixMovementProvider {
    /// # Errors
    /// Refuses invalid or unavailable movement authority, transport, or canonical evidence.
    pub fn new(
        config: MovementProviderConfig,
        codec: Arc<dyn MovementProviderCodec>,
    ) -> Result<Self, MovementProviderError> {
        validate_socket(&config)?;
        Ok(Self {
            config,
            codec,
            execution_authority: None,
            planning_authorities: std::collections::BTreeMap::new(),
        })
    }

    pub fn attach_execution_authority(&mut self, authority: Arc<crate::custody::CustodySigner>) {
        self.execution_authority = Some(authority);
    }

    fn load_execution_authorities(
        &mut self,
        scope: &PrincipalScope<'_>,
    ) -> Result<(), MovementProviderError> {
        self.planning_authorities.clear();
        for key in scope.keys(Table::Journeys) {
            if !key.as_str().starts_with("movement-authority-") {
                continue;
            }
            let row = scope
                .get(Table::Journeys, &key)
                .ok_or(MovementProviderError::ContractViolation)?;
            let MovementProviderRequest::PlanMove(planning) =
                self.codec.decode_request(row.bytes())?
            else {
                return Err(MovementProviderError::ContractViolation);
            };
            if planning.principal != *scope.principal() || planning.tenant != *scope.tenant() {
                return Err(MovementProviderError::ContractViolation);
            }
            self.planning_authorities
                .insert(planning.idempotency_key, planning);
        }
        Ok(())
    }

    /// # Errors
    /// Refuses invalid or unavailable movement authority, transport, or canonical evidence.
    pub fn move_plan(
        &self,
        request: PlanningRequest,
    ) -> Result<AuthorizedMovePlan, MovementProviderError> {
        let context = request.context.clone();
        match self.call(&MovementProviderRequest::PlanMove(request))? {
            MovementProviderResponse::MovePlan(value) => {
                if context.route.as_ref().is_none_or(|route| {
                    crate::journeys::RouteResolver::resolve(route).ok().as_ref()
                        != Some(value.plan.route())
                }) {
                    return Err(MovementProviderError::ContractViolation);
                }
                Ok(value)
            }
            _ => Err(MovementProviderError::ContractViolation),
        }
    }
    /// # Errors
    /// Refuses invalid or unavailable movement authority, transport, or canonical evidence.
    pub fn quote_move(
        &mut self,
        scope: &mut PrincipalScope<'_>,
        request: PlanningRequest,
    ) -> Result<AuthorizedMovePlan, MovementProviderError> {
        let quote = self.move_plan(request)?;
        let key = quote_row(&quote.quote_id)?;
        let bytes = self
            .codec
            .encode_response(&MovementProviderResponse::MovePlan(quote.clone()))?;
        if bytes.len() > self.config.maximum_frame_bytes {
            return Err(MovementProviderError::ContractViolation);
        }
        scope
            .put(Table::Journeys, key, quote.expires_at, bytes)
            .map_err(|_| MovementProviderError::Unavailable)?;
        Ok(quote)
    }
    /// # Errors
    /// Refuses missing, expired, or invalid principal-scoped quotes.
    pub fn load_move_quote(
        &self,
        scope: &PrincipalScope<'_>,
        quote_id: &str,
        commit_key: [u8; 32],
        now: u64,
    ) -> Result<MovePlan, MovementProviderError> {
        let row = scope
            .get(Table::Journeys, &quote_row(quote_id)?)
            .ok_or(MovementProviderError::ContractViolation)?;
        match self.codec.decode_response(row.bytes())? {
            MovementProviderResponse::MovePlan(value)
                if value.quote_id == quote_id && value.expires_at >= now =>
            {
                value
                    .plan
                    .with_idempotency_key(commit_key)
                    .map_err(|_| MovementProviderError::ContractViolation)
            }
            _ => Err(MovementProviderError::ContractViolation),
        }
    }
    #[must_use]
    pub fn ready(&self) -> bool {
        matches!(
            self.call(&MovementProviderRequest::Readiness),
            Ok(MovementProviderResponse::Ready)
        )
    }
    fn call(
        &self,
        request: &MovementProviderRequest,
    ) -> Result<MovementProviderResponse, MovementProviderError> {
        validate_socket(&self.config)?;
        let mut stream = UnixStream::connect(&self.config.socket)
            .map_err(|_| MovementProviderError::Unavailable)?;
        stream
            .set_read_timeout(Some(self.config.deadline))
            .map_err(|_| MovementProviderError::Unavailable)?;
        stream
            .set_write_timeout(Some(self.config.deadline))
            .map_err(|_| MovementProviderError::Unavailable)?;
        let credentials =
            socket_peercred(&stream).map_err(|_| MovementProviderError::Unavailable)?;
        if credentials.uid.as_raw() != self.config.peer_uid
            || credentials.gid.as_raw() != self.config.peer_gid
        {
            return Err(MovementProviderError::ContractViolation);
        }
        let payload = self.codec.encode_request(request)?;
        if payload.is_empty() || payload.len() > self.config.maximum_frame_bytes {
            return Err(MovementProviderError::ContractViolation);
        }
        stream
            .write_all(&PROTOCOL_VERSION.to_be_bytes())
            .map_err(|_| MovementProviderError::Unavailable)?;
        stream
            .write_all(&(payload.len() as u64).to_be_bytes())
            .map_err(|_| MovementProviderError::Unavailable)?;
        stream
            .write_all(&payload)
            .map_err(|_| MovementProviderError::Unavailable)?;
        let mut header = [0_u8; 10];
        stream
            .read_exact(&mut header)
            .map_err(|_| MovementProviderError::Unavailable)?;
        if u16::from_be_bytes([header[0], header[1]]) != PROTOCOL_VERSION {
            return Err(MovementProviderError::ContractViolation);
        }
        let length = usize::try_from(u64::from_be_bytes(
            header[2..10]
                .try_into()
                .map_err(|_| MovementProviderError::ContractViolation)?,
        ))
        .map_err(|_| MovementProviderError::ContractViolation)?;
        if length == 0 || length > self.config.maximum_frame_bytes {
            return Err(MovementProviderError::ContractViolation);
        }
        let mut response = vec![0; length];
        stream
            .read_exact(&mut response)
            .map_err(|_| MovementProviderError::Unavailable)?;
        self.codec.decode_response(&response)
    }
}

fn validate_socket(config: &MovementProviderConfig) -> Result<(), MovementProviderError> {
    let metadata =
        fs::symlink_metadata(&config.socket).map_err(|_| MovementProviderError::Unavailable)?;
    if !metadata.file_type().is_socket()
        || metadata.file_type().is_symlink()
        || metadata.uid() != config.peer_uid
        || metadata.gid() != config.peer_gid
        || metadata.mode() & 0o007 != 0
    {
        return Err(MovementProviderError::ContractViolation);
    }
    let parent = config
        .socket
        .parent()
        .ok_or(MovementProviderError::Configuration)?;
    validate_parent(parent, config.peer_uid, config.peer_gid)
}
fn validate_parent(path: &Path, uid: u32, gid: u32) -> Result<(), MovementProviderError> {
    let metadata = fs::symlink_metadata(path).map_err(|_| MovementProviderError::Unavailable)?;
    if !metadata.is_dir()
        || metadata.file_type().is_symlink()
        || metadata.uid() != uid
        || metadata.gid() != gid
        || metadata.mode() & 0o007 != 0
    {
        Err(MovementProviderError::ContractViolation)
    } else {
        Ok(())
    }
}
fn quote_row(value: &str) -> Result<RowKey, MovementProviderError> {
    if !(16..=128).contains(&value.len())
        || !value.bytes().all(|byte| {
            byte.is_ascii_lowercase() || byte.is_ascii_digit() || byte == b'-' || byte == b'_'
        })
    {
        return Err(MovementProviderError::ContractViolation);
    }
    RowKey::new(format!("movement-quote-{value}"))
        .map_err(|_| MovementProviderError::ContractViolation)
}
fn required(name: &str) -> Result<String, MovementProviderError> {
    env::var(name)
        .ok()
        .filter(|v| !v.is_empty())
        .ok_or(MovementProviderError::Configuration)
}
fn number<T: std::str::FromStr>(name: &str) -> Result<T, MovementProviderError> {
    required(name)?
        .parse()
        .map_err(|_| MovementProviderError::Configuration)
}
#[cfg(test)]
mod protocol_tests {
    use super::*;
    #[test]
    fn prior_movement_shape_is_refused() -> Result<(), String> {
        let codec = NativeMovementCodec::new();
        assert_eq!(
            checked(codec.encode_request(&MovementProviderRequest::Readiness))?,
            vec![3, 14]
        );
        for prior in [1_u8, 2] {
            assert!(codec.decode_request(&[prior, 14]).is_err());
            assert!(codec.decode_response(&[prior, 14]).is_err());
        }
        Ok(())
    }

    fn checked<T, E: std::fmt::Debug>(result: Result<T, E>) -> Result<T, String> {
        result.map_err(|error| format!("{error:?}"))
    }
}
