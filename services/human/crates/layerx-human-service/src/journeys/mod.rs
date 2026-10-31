//! Durable, receipt-gated human journeys and their deterministic routing.

mod engine;
mod intent_submit;
mod move_money;
mod observed;
mod resolver;
mod router;
mod wire;

pub use engine::{
    AgentBoundary, AgentBoundaryError, AgentObservation, AgentPreparation, JourneyEngine,
    JourneyError, JourneyKind, JourneyLeg, JourneyPhase, JourneyPlan, JourneyProgress,
    JourneyState, JourneyStatus, NativeJourneyAdmission, NativeJourneyApprovalState,
    NativeJourneyPreview, ReceiptLookup, ReceiptMaterial, VerifiedLegEvidence,
};

pub use intent_submit::{
    authority_label, drive_intent_journey, intent_journey_id, start_kernel_journey,
    start_native_kernel_journey, verify_bindings, BindingExpectation, IntentDriver,
    IntentLegBinding, IntentShape, IntentSubmission, KernelStart, SubmitPlanRequest, SubmitRefusal,
};

pub use move_money::{
    MoveAuthorization, MoveJourney, MoveJourneyError, MoveLegExecution, MoveLegProgress, MovePlan,
    MoveQuote, MoveReceiptReference, MoveStage, MoveStatus,
};

pub use observed::{NetworkObservation, ObservationError, ObservedStateBuilder};

pub use resolver::{
    BudgetCreation, BudgetRoute, ChangeSurface, CustodyRoute, Endpoint, EndpointConstructionError,
    EndpointKind, LimitRefusal, LimitRefusalError, LimitSource, Mechanism, MovementTerm,
    PayerGrantRoute, Relationship, Route, RouteError, RouteLeg, RouteRequest, RouteResolver,
    SendRoute,
};

pub use router::{
    plan, plan_with_advisor, Advice, AdvisedPlan, AllowanceId, AllowanceKind, AllowanceScope,
    Annotation, BalanceEntry, BudgetBinding, CandidateRef, CandidateSet, Constraints,
    CustodyContext, Domain, ExecutableShape, FeeSchedule, LegBinding, LegMechanism, ObservedState,
    PlannedLeg, Refusal, RequiredAuthority, RouteAdvisor, SignedAllowance, SignedPlan,
    SigningRequirement, TopUpRecord, UnifiedIntent, UnifiedPlan,
};

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct MovementExecutionIdentity {
    pub principal: crate::store::PrincipalId,
    pub tenant: crate::store::AgentTenantId,
    pub account: [u8; 32],
    pub wallet: layerx_types::intent::EvmAddress,
    pub plan_id: [u8; 32],
}
